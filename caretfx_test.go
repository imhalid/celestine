package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Every caret draws through typing, erasing, a jump and a page change, and the
// cycle of ⇧⌘M goes through them all.
func TestCaretKindsDraw(t *testing.T) {
	for k := caretKind(0); k < caretKinds; k++ {
		a, tt := newTestApp(t, "")
		a.caretKind = k
		tt.SetSize(700, 420)
		tt.Frame()
		typeEach(tt, "merhaba dünya")
		tt.Advance(80 * time.Millisecond)
		tt.Key(0, ui.KeyBackspace)
		tt.Key(0, ui.KeyBackspace)
		tt.Advance(80 * time.Millisecond)
		a.doc.SetSelection(0, 0)
		tt.Frame()
		tt.Advance(120 * time.Millisecond)
		a.setCaret((k + 1) % caretKinds)
		if a.caretKind != (k+1)%caretKinds {
			t.Fatalf("the cycle: %d", a.caretKind)
		}
	}
	if len(caretNames) != int(caretKinds) {
		t.Fatal("a caret has no name")
	}
}

// Pacman opens its side to the way it erases, and closes it again.
func TestPacmanOpensWhenErasing(t *testing.T) {
	a, tt := newTestApp(t, "")
	a.caretKind = caretPacman
	tt.SetSize(700, 420)
	tt.Frame()
	typeEach(tt, "merhaba")
	tt.Advance(60 * time.Millisecond)
	if a.pacman.open != 0 {
		t.Fatalf("open while writing: %v", a.pacman.open)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if a.pacman.open <= 0 || a.pacman.dir != -1 {
		t.Fatalf("Backspace: open %v dir %d", a.pacman.open, a.pacman.dir)
	}
	tt.Advance(1200 * time.Millisecond)
	tt.Frame()
	if a.pacman.open != 0 || a.pacman.dir != 1 {
		t.Fatalf("the mouth did not close: open %v dir %d", a.pacman.open, a.pacman.dir)
	}
	a.doc.SetSelection(1, 1)
	tt.Frame()
	tt.Key(0, ui.KeyDelete)
	tt.Frame()
	if a.pacman.open <= 0 || a.pacman.dir != 1 {
		t.Fatalf("Delete: open %v dir %d", a.pacman.open, a.pacman.dir)
	}
}

// The settings show every caret as a tile.
func TestCaretTiles(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.settingsOpen, a.settingsTab = true, 1
	tt.Advance(900 * time.Millisecond)
	for k := caretKind(0); k < caretKinds; k++ {
		if !tt.HasText(caretNames[k]) {
			t.Fatalf("no tile for %s: %v", caretNames[k], tt.Texts())
		}
	}
}
