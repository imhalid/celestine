package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestBackForward goes back and forward through pages and a collection,
// passing over a page thrown away since.
func TestBackForward(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 900)
	tt.Advance(time.Second)
	first := a.pane().page
	a.openCollection(one)
	a.open(ps["B"])
	a.open(ps["C"])
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	tt.Frame()
	if a.pane().page != ps["B"] || a.pane().coll != nil {
		t.Fatalf("back: %v", a.here())
	}
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	tt.Frame()
	if a.pane().coll != one {
		t.Fatalf("back to the collection: %v", a.here())
	}
	if err := tt.Click("İleri (⌘])"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pane().page != ps["B"] || a.pane().coll != nil {
		t.Fatalf("forward: %v", a.here())
	}
	// B thrown away: back from C goes past it
	a.open(ps["C"])
	a.trashPage(ps["B"])
	a.step(-1)
	if a.pane().coll != one {
		t.Fatalf("past a page gone: %v", a.here())
	}
	a.step(-1)
	if a.pane().page != first {
		t.Fatalf("to the first: %v", a.here())
	}
	if a.canStep(-1) {
		t.Fatal("back from the first")
	}
}

// Back and forward without a [ on the keyboard: ⌘ and the key in its place, ğ on Turkish Q, and the
// mouse's side buttons (#207).
func TestBackForwardAnyLayout(t *testing.T) {
	lib, ps, _, _ := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 900)
	tt.Advance(time.Second)
	a.open(ps["B"])
	a.open(ps["C"])
	tt.Frame()
	tt.KeyAt(ui.Cmd, ui.KeyBracketLeft) // ⌘ğ
	tt.Frame()
	if a.pane().page != ps["B"] {
		t.Fatalf("⌘ and the key of [: %v", a.here())
	}
	tt.Key(0, ui.KeyForward)
	tt.Frame()
	if a.pane().page != ps["C"] {
		t.Fatalf("the mouse's forward: %v", a.here())
	}
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if a.pane().page != ps["B"] {
		t.Fatalf("the mouse's back: %v", a.here())
	}
}
