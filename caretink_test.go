package main

import (
	"image/color"
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
	_, tt, g := inkCaretOf(t, "ink")
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
	tt.Advance(time.Second) // the typing's ink, still going, would draw over the rose
	tt.Key(0, ui.KeyBackspace)
	tt.Frame() // the erase's own frame: a fish of another hue sinks through the rose and inks over it soon after
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
	_, tt, g := inkCaretOf(t, "lockOn")
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

// TestCaretTakesTheAccent: the carets are drawn in the theme's accent, not in the green they shipped in (#213): the
// slime, the CRT's and the torch's leave no green on a page of the light theme, which has none of its own.
func TestCaretTakesTheAccent(t *testing.T) {
	green := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		r, g, b = r>>8, g>>8, b>>8
		return g > r+40 && g > b+30
	}
	for _, name := range []string{"Slime", "CRT", "torch"} {
		a, tt := newTestApp(t, "metin\n")
		found := false
		for k := caretKind(0); k < caretCount(); k++ {
			if caretName(k) == name {
				a.caretKind, found = k, true
			}
		}
		if !found {
			t.Fatalf("no caret %s", name)
		}
		tt.SetSize(700, 420)
		tt.Advance(200 * time.Millisecond)
		img, n := tt.Image(), 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if green(img.At(x, y)) {
					n++
				}
			}
		}
		if n > 0 {
			t.Errorf("%s: %d green pixels", name, n)
		}
	}
}
