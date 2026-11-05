package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPaletteGoes opens the palette, finds a page by a few letters of it
// and goes there; runs a command; closes with Esc.
func TestPaletteGoes(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Advance(300 * time.Millisecond)
	if !a.pal.open || !tt.HasText("Komutlar") {
		t.Fatalf("not open: %v", tt.Texts())
	}
	tt.Type("yuzyil") // accents don't matter
	tt.Frame()
	if items := a.paletteItems(a.pal.query); len(items) == 0 || items[0].title != "Yüzyıllık Yalnızlık" {
		t.Fatalf("first: %+v", items)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if a.pal.open || a.page == nil || a.page.Title != "Yüzyıllık Yalnızlık" {
		t.Fatalf("went to %v", a.here())
	}
	// empty, it has the places last shown
	a.openPalette()
	tt.Frame()
	if items := a.paletteItems(""); len(items) == 0 || items[0].to != (place{"coll", coll.Dir}) {
		t.Fatalf("recent: %+v", items[0])
	}
	tt.Type("çöp")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !a.trashOpen {
		t.Fatal("the command did not run")
	}
	a.openPalette()
	tt.Advance(300 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(500 * time.Millisecond)
	if a.pal.open {
		t.Fatal("Esc did not close it")
	}
}

// TestMatchScores: whole words at the start first, letters in order last.
func TestMatchScores(t *testing.T) {
	if match("dune", "Notlar: Dune") <= match("dune", "Dunedin günlüğü kitap") && match("dune", "Dune") <= match("dune", "Notlar: Dune") {
		t.Fatal("order")
	}
	if match("ypl", "Yapılacaklar") <= 0 || match("xyz", "Yapılacaklar") >= 0 {
		t.Fatal("letters in order")
	}
	if match("İSTANBUL", "istanbul notları") < 0 || match("ozellik", "Özellik ekle") < 0 {
		t.Fatal("case and accents")
	}
}

// TestFindInPage finds, steps through and replaces.
func TestFindInPage(t *testing.T) {
	a, tt := newTestApp(t, "Bir elma, iki ELMA.\n\nÜç elma, dört armut.\n")
	a.doc.SetSelection(0, 0)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Advance(200 * time.Millisecond)
	if !a.find.open {
		t.Fatal("not open")
	}
	tt.Type("elma")
	tt.Frame()
	if len(a.find.matches) != 3 || !tt.HasText("1/3") {
		t.Fatalf("matches %v", a.find.matches)
	}
	if s, e := a.doc.Selection(); s != 4 || e != 8 {
		t.Fatalf("selected %d-%d", s, e)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.find.cur != 1 || !tt.HasText("2/3") {
		t.Fatalf("next: %d", a.find.cur)
	}
	tt.Key(ui.Shift, ui.KeyEnter)
	tt.Frame()
	if a.find.cur != 0 {
		t.Fatalf("back: %d", a.find.cur)
	}
	a.find.replace, a.find.with = true, "armut"
	tt.Frame()
	if err := tt.Click("Tümünü değiştir"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.doc.Text().String(); got != "Bir armut, iki armut.\nÜç armut, dört armut." || len(a.find.matches) != 0 {
		t.Fatalf("replaced: %q", got)
	}
	a.doc.Undo() // one step undoes it
	tt.Frame()
	if got := a.doc.Text().String(); got != "Bir elma, iki ELMA.\nÜç elma, dört armut." {
		t.Fatalf("undo: %q", got)
	}
}
