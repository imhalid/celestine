package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Every caret draws through typing, erasing, a jump and a page change, and the
// cycle of ⇧⌘M goes through them all.
func TestCaretKindsDraw(t *testing.T) {
	for k := caretKind(0); k < caretCount(); k++ {
		a, tt := newTestApp(t, "")
		a.caretKind = k
		tt.SetSize(700, 420)
		tt.Frame()
		typeEach(tt, "merhaba dünya Ali")
		tt.Key(0, ui.KeyEnter)
		typeEach(tt, "bir iki ")
		tt.Advance(80 * time.Millisecond)
		tt.Key(0, ui.KeyBackspace)
		tt.Key(0, ui.KeyBackspace)
		tt.Advance(80 * time.Millisecond)
		a.doc.SetSelection(0, 0)
		tt.Frame()
		tt.Advance(120 * time.Millisecond)
		a.setCaret((k + 1) % caretCount())
		if a.caretKind != (k+1)%caretCount() {
			t.Fatalf("the cycle: %d", a.caretKind)
		}
	}
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == "" {
			t.Fatalf("caret %d has no name", k)
		}
	}
}

// Pacman opens its side to the way it erases, and closes it again.
func TestPacmanOpensWhenErasing(t *testing.T) {
	a, tt := newTestApp(t, "")
	a.caretKind = caretFirstFX
	pac := a.fxNamed("Pacman").(*pacman)
	tt.SetSize(700, 420)
	tt.Frame()
	typeEach(tt, "merhaba")
	tt.Advance(60 * time.Millisecond)
	if pac.open != 0 {
		t.Fatalf("open while writing: %v", pac.open)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if pac.open <= 0 || pac.dir != -1 {
		t.Fatalf("Backspace: open %v dir %d", pac.open, pac.dir)
	}
	tt.Advance(1200 * time.Millisecond)
	tt.Frame()
	if pac.open != 0 || pac.dir != 1 {
		t.Fatalf("the mouth did not close: open %v dir %d", pac.open, pac.dir)
	}
	a.doc.SetSelection(1, 1)
	tt.Frame()
	tt.Key(0, ui.KeyDelete)
	tt.Frame()
	if pac.open <= 0 || pac.dir != 1 {
		t.Fatalf("Delete: open %v dir %d", pac.open, pac.dir)
	}
}

// The settings show every caret as a tile.
func TestCaretTiles(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.settingsOpen, a.settingsTab = true, 1
	tt.Advance(900 * time.Millisecond)
	for k := caretKind(0); k < caretCount(); k++ {
		if !tt.HasText(caretName(k)) {
			t.Fatalf("no tile for %s: %v", caretName(k), tt.Texts())
		}
	}
}
