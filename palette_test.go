package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPaletteGoes opens the search (⌘K), finds a page by a few letters of it
// and goes there; runs a command from ⌘P; closes with Esc.
func TestPaletteGoes(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(300 * time.Millisecond)
	if !a.pal.open || a.palCmds() {
		t.Fatalf("not open as the search: %v", tt.Texts())
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
	for _, it := range a.paletteItems("") {
		if it.sub == "Komut" {
			t.Fatalf("a command in the search: %s", it.title)
		}
	}
	// ⌘P is the commands: no pages in it
	tt.Key(0, ui.KeyEscape)
	tt.Advance(400 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Advance(300 * time.Millisecond)
	if !a.pal.open || !a.palCmds() || !tt.HasText("Komutlar") {
		t.Fatalf("not open as the commands: open=%v cmds=%v", a.pal.open, a.palCmds())
	}
	for _, it := range a.paletteItems(">yuzyil") {
		if it.to.kind != "" {
			t.Fatalf("a place among the commands: %s", it.title)
		}
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

// One palette: a ">" first in what is typed in ⌘K makes it the commands, and
// taking it away makes it the pages again.
func TestPaletteIsOneWithCommands(t *testing.T) {
	a, tt, _ := viewsLibrary(t)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(300 * time.Millisecond)
	if a.palCmds() {
		t.Fatal("⌘K starts as the commands")
	}
	tt.Type(">çöp")
	tt.Frame()
	if !a.palCmds() || a.palQuery() != "çöp" {
		t.Fatalf("not the commands: %q", a.pal.query)
	}
	items := a.paletteItems(a.pal.query)
	if len(items) == 0 || items[0].sub != "Komut" {
		t.Fatalf("what the commands list: %+v", items)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(400 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Advance(300 * time.Millisecond)
	if a.pal.query != ">" || !tt.HasText("Komutlar") {
		t.Fatalf("⌘P has no > in it: %q", a.pal.query)
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

// The app's theme: chosen in the commands and the settings, kept, and the accent of the window
// follows it.
func TestAppTheme(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.Frame()
	if a.appTheme().name != "Varsayılan" {
		t.Fatalf("the first theme is %s", a.appTheme().name)
	}
	blue, plainBg := a.theme.Link, a.pageBg
	a.openCommands()
	tt.Frame()
	tt.Type("renk teması gece")
	tt.Frame()
	items := a.paletteItems(a.pal.query)
	if len(items) == 0 || items[0].title != "Renk teması: Gece Yarısı" {
		t.Fatalf("first: %+v", items)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if a.appTheme().name != "Gece Yarısı" || a.settings.AppTheme == 0 {
		t.Fatalf("theme %s", a.appTheme().name)
	}
	if green := a.theme.Link; green == blue {
		t.Fatal("the accent did not change")
	}
	if a.pageBg == plainBg {
		t.Fatal("the page's color did not change")
	}
	// in the dark appearance its dark colors: the page is darker than in the light
	light := a.pageBg
	tt.SetDark(true)
	tt.Frame()
	if luma(a.pageBg) >= luma(light) {
		t.Fatalf("Gece Yarısı dark page %v is not darker than the light %v", a.pageBg, light)
	}
	for i := range appThemes {
		a.setAppTheme(i)
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
		}
	}
	a.settingsOpen, a.settingsTab = true, 0
	tt.Advance(900 * time.Millisecond)
	if !tt.HasText("Renk teması") {
		t.Fatalf("no picker: %v", tt.Texts())
	}
}

// The arrows go down the list, and the list follows: the chosen row is always in the window of 300 points.
func TestPaletteScrollsWithTheArrows(t *testing.T) {
	a, tt, _ := viewsLibrary(t)
	elsewhere := map[string]bool{} // a title the window shows outside the palette too (the sidebar's buttons)
	for _, it := range a.commandItems("") {
		if _, ok := tt.Find(it.title); ok {
			elsewhere[it.title] = true
		}
	}
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Advance(300 * time.Millisecond)
	items := a.paletteItems(a.pal.query)
	if len(items) < 25 {
		t.Skipf("only %d commands", len(items))
	}
	first, ok := tt.Find(items[0].title)
	if !ok {
		t.Fatalf("no first row: %v", tt.Texts())
	}
	for i := 1; i <= 24; i++ {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
		tt.Frame()
		r, ok := tt.Find(items[a.pal.sel].title)
		if !ok {
			t.Fatalf("row %d is not drawn", a.pal.sel)
		}
		if elsewhere[items[a.pal.sel].title] {
			continue
		}
		if r.Y < first.Y-4 || r.Y+r.H > first.Y+300+4 {
			t.Fatalf("row %d at y=%.0f is out of the window [%.0f, %.0f]", a.pal.sel, r.Y, first.Y, first.Y+300)
		}
	}
	// and back up
	for i := 0; i < 24; i++ {
		tt.Key(0, ui.KeyUp)
		tt.Frame()
		tt.Frame()
	}
	if r, ok := tt.Find(items[0].title); !ok || r.Y < first.Y-30 {
		t.Fatalf("the first row did not come back: %v", r)
	}
}

// The palette makes its lines once for a query, not at every frame: the pointer moving over them, a frame asked for,
// cost nothing of it; a letter typed makes them again (#186).
func TestPaletteLinesKept(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Advance(300 * time.Millisecond)
	made := a.pal.made
	if made == 0 || len(a.pal.items) == 0 {
		t.Fatal("no lines")
	}
	for i := range 20 {
		tt.Move(450, float32(160+i*6))
		tt.Frame()
	}
	if a.pal.made != made {
		t.Fatalf("made %d times more as the pointer moved", a.pal.made-made)
	}
	tt.Type("çöp")
	tt.Frame()
	if a.pal.made == made || a.pal.query != ">çöp" {
		t.Fatalf("not made again for the query: %q", a.pal.query)
	}
}
