package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// inkCaretOf is the app with the caret called name chosen, and that caret.
func inkCaretOf(t *testing.T, name string) (*app, *ui.Tester, *inkCaret) {
	a, tt := newTestApp(t, "")
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == name {
			a.caretKind = k
		}
	}
	tt.SetSize(700, 420)
	tt.Frame()
	g, ok := a.fxNamed(name).(*inkCaret)
	if !ok {
		t.Fatalf("no %s caret", name)
	}
	return a, tt, g
}

// The Ink caret: a letter leaves ink, a finished word is locked on with a tag, an erase dissolves into rose ink
// and collapses a box; then it all fades, and the caret asks for no more frames.
func TestInkCaret(t *testing.T) {
	_, tt, g := inkCaretOf(t, "Ink")
	typeEach(tt, "merhaba")
	tt.Advance(30 * time.Millisecond)
	if len(g.cells) == 0 || len(g.ripples) == 0 {
		t.Fatalf("typing left no ink: %d cells, %d ripples", len(g.cells), len(g.ripples))
	}
	typeEach(tt, " ")
	tt.Advance(30 * time.Millisecond)
	if len(g.locks) == 0 || g.locks[len(g.locks)-1].kind != lockWord || g.locks[len(g.locks)-1].tag == "" {
		t.Fatalf("the finished word is not locked on: %+v", g.locks)
	}
	typeEach(tt, ".")
	tt.Advance(30 * time.Millisecond)
	words := 0
	for _, l := range g.locks {
		if l.kind == lockWord {
			words++
		}
	}
	if words != 1 {
		t.Fatalf("a mark after the space locked the word again: %d locks", words)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyBackspace)
	tt.Advance(30 * time.Millisecond)
	rose, gone := 0, false
	for _, c := range g.cells {
		if c.hue == inkRose {
			rose++
		}
	}
	for _, l := range g.locks {
		gone = gone || l.kind == lockGone
	}
	if rose == 0 || !gone {
		t.Fatalf("the erase did not dissolve: %d rose cells, a gone box %v", rose, gone)
	}
	tt.Advance(4 * time.Second)
	if g.alive() {
		t.Fatalf("still going after four seconds: %d cells, %d locks, %d fish", len(g.cells), len(g.locks), len(g.fish))
	}
}

// Lock-on is the brackets alone: a finished word is locked on and no ink is left.
func TestLockOnCaret(t *testing.T) {
	_, tt, g := inkCaretOf(t, "Lock-on")
	typeEach(tt, "merhaba dünya ")
	tt.Advance(30 * time.Millisecond)
	if len(g.locks) == 0 {
		t.Fatal("no lock on the finished words")
	}
	if len(g.cells) != 0 || len(g.ripples) != 0 || len(g.fish) != 0 {
		t.Fatalf("Lock-on left ink: %d cells", len(g.cells))
	}
}

// The tag's pixel font has every letter a tag can hold, five rows of three.
func TestPixelFont(t *testing.T) {
	for _, r := range "0123456789XY:" {
		rows, ok := pixelFont[r]
		if !ok || len(rows) != 5*3+4 {
			t.Fatalf("%q: %q", r, rows)
		}
	}
}
