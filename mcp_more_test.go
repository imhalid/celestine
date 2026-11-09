package main

import (
	"context"
	"testing"

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
