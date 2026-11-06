package main

import (
	"strings"
	"testing"
	"time"
)

// TestBulk: rows chosen, then done to all: a tag, a value, a move, the trash.
func TestBulk(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	other, _ := a.lib.NewCollection("Öteki")
	tt.SetSize(1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	if err := tt.Click("Liste"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Seç"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if !a.bulk.on || !tt.HasText("Satırlara tıklayarak seç") {
		t.Fatalf("not on: %v", tt.Texts())
	}
	if err := tt.Click("Aç: Dune"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Aç: Anna Karenina"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.coll != coll || len(a.bulk.sel) != 2 || !tt.HasText("2 seçili") {
		t.Fatalf("chosen %d, coll open %v", len(a.bulk.sel), a.coll == coll)
	}
	pages := a.bulkPages(a.filtered(coll))
	// a tag
	a.bulkAddTag(pages, "#okunacak")
	for _, p := range pages {
		if body, _ := p.Text(); !strings.Contains(body, "#okunacak") {
			t.Fatalf("%s: %q", p.Title, body)
		}
	}
	a.bulkAddTag(pages, "okunacak") // again: not twice
	if body, _ := pages[0].Text(); strings.Count(body, "#okunacak") != 1 {
		t.Fatalf("twice: %q", body)
	}
	// a value of a choice
	for _, it := range a.bulkChoices(coll, pages) {
		if it.title == "Durum: Bitti" {
			it.run()
		}
	}
	for _, p := range pages {
		if p.Value(coll.Prop("durum")) != "Bitti" {
			t.Fatalf("%s: %q", p.Title, p.Value(coll.Prop("durum")))
		}
	}
	// a move
	for _, it := range a.bulkMoveMenu(pages) {
		if it.title == "Öteki" {
			it.run()
		}
	}
	a.runLater()
	if len(other.Pages) != 2 {
		t.Fatalf("moved: %d", len(other.Pages))
	}
	// the trash
	a.bulkTrash(other.Pages[:1])
	if len(other.Pages) != 1 || len(a.lib.Trash()) == 0 {
		t.Fatalf("trash: %d", len(other.Pages))
	}
	if a.bulk.on {
		t.Fatal("the bar stays")
	}
}
