package main

import (
	"context"
	"path/filepath"
	"slices"
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
	if !tt.HasText("Ajan 2 değişiklik yaptı") {
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
	if tt.HasText("Ajan 2 değişiklik yaptı") || a.agentChanged() != 0 {
		t.Fatal("still said")
	}
}

// TestAgentOpsUndo: what an agent did through the tools besides writing - a page moved and one thrown away, a
// collection made, a property added, a collection renamed, a page brought back - is taken back by the one click too,
// the last first, and the record follows the pages when the library is read again (#180).
func TestAgentOpsUndo(t *testing.T) {
	a, tt := newTestApp(t, "ilk metin")
	coll := a.lib.Collections[0]
	second, _ := coll.NewPage(nil, "İkinci")
	third, _ := coll.NewPage(nil, "Üçüncü")
	third.SaveText("üçüncünün metni\n")
	order := func() []string {
		var out []string
		for _, p := range coll.Pages {
			out = append(out, p.Title)
		}
		return out
	}
	was := order()
	cs := agent(t, a)
	call(t, cs, "move_page", map[string]any{"page": a.idOf(third.Dir), "index": 0}, nil)
	call(t, cs, "update_page", map[string]any{"page": a.idOf(coll.Pages[0].Dir), "append": "ajanın satırı"}, nil)
	call(t, cs, "trash_page", map[string]any{"page": a.idOf(second.Dir)}, nil)
	var made struct{ ID string }
	call(t, cs, "create_collection", map[string]any{"name": "Ajanınki"}, &made)
	call(t, cs, "add_property", map[string]any{"collection": a.idOf(coll.Dir), "name": "Durum", "type": "text"}, nil)
	call(t, cs, "update_collection", map[string]any{"collection": a.idOf(coll.Dir), "name": "Yeni ad"}, nil)
	if slices.Equal(order(), was) || coll.Name != "Yeni ad" || len(a.lib.Collections) != 2 || len(coll.Properties) == 0 {
		t.Fatalf("the agent's changes did not happen: %v %q %d", order(), coll.Name, len(a.lib.Collections))
	}
	dir := coll.Dir
	a.reloadLibrary() // as a folder library is read again: other pages
	coll = a.collAt(dir)
	tt.Frame()
	if n := a.agentChanged(); n < 6 {
		t.Fatalf("the record after a reload: %d", n)
	}
	a.undoAgent()
	coll = a.collAt(dir)
	if got := order(); !slices.Equal(got, was) {
		t.Fatalf("the order: %v, was %v", got, was)
	}
	if coll.Name == "Yeni ad" || len(coll.Properties) != 0 || len(a.lib.Collections) != 1 {
		t.Fatalf("the collections: %q, %d properties, %d collections", coll.Name, len(coll.Properties), len(a.lib.Collections))
	}
	for _, p := range coll.All() {
		if text, _ := p.Text(); strings.Contains(text, "ajanın") {
			t.Fatalf("%s still has the agent's line", p.Title)
		}
	}
	if a.agentChanged() != 0 {
		t.Fatal("still said")
	}
}

// run_command keeps what agents may do with a collection: a command that changes the open page is refused when
// its collection is read-only to agents, and runs when they may write it (#66).
func TestRunCommandKeepsAccess(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	tt.Advance(50 * time.Millisecond)
	cs := agent(t, a)
	p := a.pane().page
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

type spyKeychain struct {
	data map[string]string
	sets int
}

func (k *spyKeychain) Get(name string) (string, error) {
	return k.data[name], nil
}

func (k *spyKeychain) Set(name, secret string) error {
	k.sets++
	if k.data == nil {
		k.data = map[string]string{}
	}
	k.data[name] = secret
	return nil
}

func (k *spyKeychain) Delete(name string) error {
	delete(k.data, name)
	return nil
}

func TestRunCommandDispatchesUIAndRecoversPanic(t *testing.T) {
	a, _ := newTestApp(t, "ilk metin")
	a.crashDir = t.TempDir()
	uiCalls := 0
	a.runOnUI = func(fn func()) {
		uiCalls++
		fn()
	}
	cs := agent(t, a)

	call(t, cs, "run_command", map[string]any{"command": "Sayfada bul"}, nil)
	if uiCalls != 1 {
		t.Fatalf("runOnUI calls = %d, want 1", uiCalls)
	}

	a.invalidate = func() { panic("ui boom") }
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "run_command",
		Arguments: map[string]any{"command": "Sayfada bul"},
	})
	if err != nil {
		t.Fatalf("transport error instead of tool error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError on UI panic")
	}
	if uiCalls != 2 {
		t.Fatalf("runOnUI calls after panic = %d, want 2", uiCalls)
	}
	a.mu.Lock()
	msg, crashPath := a.crashMsg, a.crashPath
	a.mu.Unlock()
	if msg != "ui boom" || crashPath == "" {
		t.Fatalf("expected crash note and path, got msg=%q path=%q", msg, crashPath)
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
	if a.pane().page == nil || a.pane().page.Title != "Menemen" {
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

func TestMCPMoveTargetByDirJournalReorderAndHiddenCommands(t *testing.T) {
	a, _ := newTestApp(t, "ilk metin")
	cs := agent(t, a)
	ctx := context.Background()

	// Two destination collections sharing the same Name must be resolved by Dir, not Name (#171).
	destRW, err := a.lib.NewCollection("Ortak")
	if err != nil {
		t.Fatal(err)
	}
	destRO, err := a.lib.NewCollection("Ortak")
	if err != nil {
		t.Fatal(err)
	}
	a.settings.MCPAccess = map[string]string{
		filepath.Base(destRO.Dir): mcpRead,
	}

	var matched int
	for _, it := range a.commands() {
		if target := a.mcpMoveTarget(it); target != nil {
			matched++
			if target == destRW {
				if err := a.mcpMayChange(it); err != nil {
					t.Fatalf("writable duplicate-name target denied: %v", err)
				}
			}
			if target == destRO {
				if err := a.mcpMayChange(it); err == nil {
					t.Fatal("read-only duplicate-name target allowed via name collision")
				}
			}
		}
	}
	if matched < 2 {
		t.Fatalf("expected move targets for both Ortak collections, matched %d", matched)
	}

	// Hidden collection + open hidden page must not leak names or titles in list_commands (#171).
	hiddenColl, err := a.lib.NewCollection("GizliKoleksiyon")
	if err != nil {
		t.Fatal(err)
	}
	hiddenColl.SubPages = true
	hiddenPage, err := hiddenColl.NewPage(nil, "GizliSayfaBaşlığı")
	if err != nil {
		t.Fatal(err)
	}
	a.settings.MCPAccess[filepath.Base(hiddenColl.Dir)] = mcpOff
	a.pane().coll = hiddenColl
	a.pane().page = hiddenPage

	var cmds []struct {
		Title string `json:"title"`
	}
	call(t, cs, "list_commands", nil, &cmds)
	for _, cmd := range cmds {
		if strings.Contains(cmd.Title, "GizliSayfaBaşlığı") || strings.Contains(cmd.Title, "GizliKoleksiyon") {
			t.Fatalf("list_commands leaked hidden name/title: %q", cmd.Title)
		}
	}

	// move_page allows reordering within a journal, but rejects moving into a journal or dynamic collection (#181).
	journalColl, err := a.lib.NewCollection("Günlük")
	if err != nil {
		t.Fatal(err)
	}
	journalColl.Journal = true
	j1, err := a.makeDay(journalColl, a.clock().AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	j2, err := a.makeDay(journalColl, a.clock())
	if err != nil {
		t.Fatal(err)
	}
	dynColl, err := a.lib.NewCollection("Dinamik")
	if err != nil {
		t.Fatal(err)
	}
	dynColl.Query = "#etiket"

	call(t, cs, "move_page", map[string]any{"page": a.idOf(j2.Dir), "index": 0}, nil)
	if journalColl.Pages[0] != j2 || journalColl.Pages[1] != j1 {
		t.Fatalf("journal reorder failed: got %q, %q", journalColl.Pages[0].Title, journalColl.Pages[1].Title)
	}

	normalPage, err := destRW.NewPage(nil, "Normal")
	if err != nil {
		t.Fatal(err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "move_page",
		Arguments: map[string]any{"page": a.idOf(normalPage.Dir), "collection": a.idOf(journalColl.Dir)},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("expected error moving a page into a journal collection")
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "move_page",
		Arguments: map[string]any{"page": a.idOf(normalPage.Dir), "collection": a.idOf(dynColl.Dir)},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("expected error moving a page into a dynamic collection")
	}
}

// TestMCPSeesTrashCollectionAccess verifies that mcpSeesTrash uses mcpOff consistently with mcpAccess,
// including treating an empty string override like default/visible access.
func TestMCPSeesTrashCollectionAccess(t *testing.T) {
	a, _ := newTestApp(t, "x")
	tr := &TrashItem{Kind: "collection", From: "Tarifler"}

	a.settings.MCPAccess = map[string]string{"Tarifler": mcpOff}
	if a.mcpSeesTrash(tr) {
		t.Fatal("mcpSeesTrash should be false when collection access is mcpOff")
	}

	a.settings.MCPAccess = map[string]string{"Tarifler": ""}
	if !a.mcpSeesTrash(tr) {
		t.Fatal("mcpSeesTrash should match mcpAccess != mcpOff for empty access override")
	}

	a.settings.MCPAccess = map[string]string{"*": mcpOff}
	if a.mcpSeesTrash(tr) {
		t.Fatal("mcpSeesTrash should be false when wildcard access is mcpOff")
	}
}
