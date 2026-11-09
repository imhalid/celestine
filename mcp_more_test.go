package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestUnderHeading: text goes at the end of the heading's section, before the next heading of its level or above,
// and a heading inside code is not one.
func TestUnderHeading(t *testing.T) {
	body := "# Kitap\n\n## Notlar\n\n- bir\n\n### Alt\n\nalt\n\n```\n## Yalan\n```\n\n## Sonra\n\nson\n"
	got, err := underHeading(body, "notlar", "- iki")
	if err != nil {
		t.Fatal(err)
	}
	want := "# Kitap\n\n## Notlar\n\n- bir\n\n### Alt\n\nalt\n\n```\n## Yalan\n```\n\n- iki\n\n## Sonra\n\nson\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if got, _ = underHeading(body, "## Sonra", "yeni"); got != "# Kitap\n\n## Notlar\n\n- bir\n\n### Alt\n\nalt\n\n```\n## Yalan\n```\n\n## Sonra\n\nson\n\nyeni" {
		t.Fatalf("at the end: %q", got)
	}
	if _, err := underHeading(body, "Yalan", "x"); err == nil {
		t.Fatal("a heading in code was found")
	}
}

// TestMCPMoreTools: an agent puts text under a heading, finds pages by a property, reads the day's page, and brings a
// page back from the trash.
func TestMCPMoreTools(t *testing.T) {
	a, _ := newTestApp(t, "ilk metin")
	cs := agent(t, a)

	call(t, cs, "create_collection", map[string]any{"name": "Okumalar", "properties": []map[string]any{{"name": "Durum", "type": "choice", "options": []string{"Okunacak", "Okundu"}}}}, nil)
	var dune, emma struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": "okumalar", "title": "Dune", "markdown": "## Notlar\n\n- çöl\n\n## Alıntılar\n\nkorku\n", "properties": map[string]string{"Durum": "Okundu"}}, &dune)
	call(t, cs, "create_page", map[string]any{"collection": "okumalar", "title": "Emma", "properties": map[string]string{"Durum": "Okunacak"}}, &emma)

	call(t, cs, "insert_under_heading", map[string]any{"page": dune.ID, "heading": "Notlar", "markdown": "- baharat"}, nil)
	var read struct{ Markdown string }
	call(t, cs, "read_page", map[string]any{"page": dune.ID}, &read)
	if read.Markdown != "## Notlar\n\n- çöl\n- baharat\n\n## Alıntılar\n\nkorku\n" {
		t.Fatalf("under heading: %q", read.Markdown)
	}

	var found []struct{ ID string }
	call(t, cs, "query_pages", map[string]any{"where": []map[string]any{{"property": "Durum", "op": "is", "value": "Okunacak"}}}, &found)
	if len(found) != 1 || found[0].ID != emma.ID {
		t.Fatalf("query: %+v", found)
	}
	call(t, cs, "query_pages", map[string]any{"collection": "okumalar", "where": []map[string]any{{"property": "title", "op": "contains", "value": "dun"}}}, &found)
	if len(found) != 1 || found[0].ID != dune.ID {
		t.Fatalf("query by title: %+v", found)
	}

	var day struct{ ID, Title string }
	call(t, cs, "get_daily_note", nil, &day)
	if day.ID == "" {
		t.Fatal("no day's page")
	}
	var again struct{ ID string }
	call(t, cs, "get_daily_note", map[string]any{"date": a.clock().Format("2006-01-02")}, &again)
	if again.ID != day.ID {
		t.Fatalf("today twice: %s %s", day.ID, again.ID)
	}
	res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_daily_note", Arguments: map[string]any{"date": "1999-01-01"}})
	if !res.IsError {
		t.Fatal("a past day without a page was made")
	}

	call(t, cs, "trash_page", map[string]any{"page": emma.ID}, nil)
	var trash []trashInfo
	call(t, cs, "list_trash", nil, &trash)
	if len(trash) != 1 || trash[0].Title != "Emma" {
		t.Fatalf("trash: %+v", trash)
	}
	// the collection read-only: the page stays in the trash
	a.settings.MCPAccess = map[string]string{"okumalar": mcpRead}
	res, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "restore_page", Arguments: map[string]any{"item": trash[0].ID}})
	if !res.IsError {
		t.Fatal("restored into a read-only collection")
	}
	a.settings.MCPAccess = nil
	var back struct{ ID string }
	call(t, cs, "restore_page", map[string]any{"item": trash[0].ID}, &back)
	if back.ID != emma.ID || len(a.lib.Trash()) != 0 {
		t.Fatalf("restored: %q, trash %d", back.ID, len(a.lib.Trash()))
	}
}

// TestAgentEditsUndo: what an agent wrote and made shows in the status bar, and one click puts the pages back as they
// were and the made ones in the trash.
func TestAgentEditsUndo(t *testing.T) {
	a, tt := newTestApp(t, "ilk metin")
	cs := agent(t, a)
	var colls []collInfo
	call(t, cs, "list_collections", nil, &colls)
	var pages []pageInfo
	call(t, cs, "list_pages", map[string]any{"collection": colls[0].ID}, &pages)
	first := pages[0]
	call(t, cs, "update_page", map[string]any{"page": first.ID, "title": "Başka", "append": "ajanın satırı"}, nil)
	var made struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": colls[0].ID, "title": "Yeni", "markdown": "x"}, &made)
	tt.Frame()
	if !tt.HasText("Ajan 2 sayfayı değiştirdi") {
		t.Fatal("the status bar does not say what the agent changed")
	}
	if err := tt.Click("Ajanın değişikliklerini geri al"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	var read struct{ Title, Markdown string }
	call(t, cs, "read_page", map[string]any{"page": first.ID}, &read)
	if read.Title != first.Title || strings.Contains(read.Markdown, "ajanın") {
		t.Fatalf("not put back: %+v", read)
	}
	if res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "read_page", Arguments: map[string]any{"page": made.ID}}); !res.IsError {
		t.Fatal("the made page is still there")
	}
	if tt.HasText("Ajan 2 sayfayı değiştirdi") || a.agentChanged() != 0 {
		t.Fatal("still said")
	}
}

// run_command keeps what agents may do with a collection: a command that changes the open page is refused when
// its collection is read-only to agents, and runs when they may write it (#66).
func TestRunCommandKeepsAccess(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	tt.Advance(50 * time.Millisecond)
	cs := agent(t, a)
	p := a.page
	key := filepath.Base(p.Coll.Dir)
	a.settings.MCPAccess = map[string]string{key: mcpRead}
	res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "run_command", Arguments: map[string]any{"command": "Çöpe taşı: Sayfa"}})
	if !res.IsError || !a.inLibrary(p) {
		t.Fatal("a read-only collection's page was thrown away")
	}
	// a command that changes nothing runs
	call(t, cs, "run_command", map[string]any{"command": "Sayfada bul"}, nil)
	a.settings.MCPAccess = map[string]string{key: mcpWrite}
	call(t, cs, "run_command", map[string]any{"command": "Çöpe taşı: Sayfa"}, nil)
	if a.inLibrary(p) {
		t.Fatal("not thrown away with write access")
	}
}

// help, open_page, update_collection, a page as a resource and the prompts (#68).
func TestMCPHelpResourcesPrompts(t *testing.T) {
	a, tt := newTestApp(t, "ilk metin")
	tt.Advance(50 * time.Millisecond)
	cs := agent(t, a)
	ctx := context.Background()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "help"})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "query language") {
		t.Fatalf("help: %v", err)
	}
	call(t, cs, "create_collection", map[string]any{"name": "Tarifler"}, nil)
	var made struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": "Tarifler", "title": "Menemen", "markdown": "Domates, biber, yumurta."}, &made)
	call(t, cs, "open_page", map[string]any{"page": made.ID}, nil)
	if a.page == nil || a.page.Title != "Menemen" {
		t.Fatal("open_page did not open it")
	}
	f := false
	call(t, cs, "update_collection", map[string]any{"collection": "Tarifler", "name": "Yemekler", "pageNoun": "Tarif", "subPages": f}, nil)
	c, _ := a.findColl("tarifler", mcpRead)
	if c.Name != "Yemekler" || c.PageNoun != "Tarif" || c.SubPages {
		t.Fatalf("update_collection: %+v", c)
	}
	// the page as a resource; one of a hidden collection is not there
	r, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: pageURIPrefix + made.ID})
	if err != nil || !strings.Contains(r.Contents[0].Text, "Domates") {
		t.Fatalf("resource: %v %+v", err, r)
	}
	a.settings.MCPAccess = map[string]string{"tarifler": mcpOff}
	if _, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: pageURIPrefix + made.ID}); err == nil {
		t.Fatal("a hidden collection's page read as a resource")
	}
	a.settings.MCPAccess = nil
	// the prompts
	ps, err := cs.ListPrompts(ctx, nil)
	if err != nil || len(ps.Prompts) != 2 {
		t.Fatalf("prompts: %v %+v", err, ps)
	}
	p, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "summarize_page", Arguments: map[string]string{"page": made.ID}})
	if err != nil || !strings.Contains(p.Messages[0].Content.(*mcp.TextContent).Text, "yumurta") {
		t.Fatalf("summarize_page: %v", err)
	}
	w, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "weekly_review"})
	if err != nil || !strings.Contains(w.Messages[0].Content.(*mcp.TextContent).Text, a.clock().Format(time.DateOnly)) {
		t.Fatalf("weekly_review: %v", err)
	}
}
