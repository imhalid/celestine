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

// Back and forward find a page by its ID: moved to another collection, under another page, or renamed, a page
// visited is still gone back to; a place kept without an ID is found by its folder as before (#593).
func TestBackForwardFollowsMovedPages(t *testing.T) {
	lib, ps, one, two := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 900)
	tt.Advance(time.Second)
	for _, n := range []string{"A", "B", "C"} {
		if ps[n].ID == "" {
			t.Fatalf("%s has no ID", n)
		}
	}
	a.open(ps["A"])
	a.open(ps["B"])
	a.open(ps["C"])
	tt.Frame()
	oldA, oldB := ps["A"].Dir, ps["B"].Dir
	if err := lib.MovePage(ps["A"], two, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := lib.MovePage(ps["B"], one, ps["C"], 0); err != nil {
		t.Fatal(err)
	}
	if err := ps["B"].SetTitle("Bee"); err != nil {
		t.Fatal(err)
	}
	if ps["A"].Dir == oldA || ps["B"].Dir == oldB {
		t.Fatal("the moves did not change the folders")
	}
	if !a.step(-1) || a.pane().page != ps["B"] {
		t.Fatalf("back to B, moved under C and renamed: %v", a.here())
	}
	if !a.step(-1) || a.pane().page != ps["A"] {
		t.Fatalf("back to A, moved to another collection: %v", a.here())
	}
	if !a.step(1) || a.pane().page != ps["B"] {
		t.Fatalf("forward to B: %v", a.here())
	}
	// a place from before IDs: by its folder
	a.hist = []visited{{place: place{"page", ps["C"].Dir}}, {place: place{"page", ps["A"].Dir}}}
	a.histAt = 1
	if !a.step(-1) || a.pane().page != ps["C"] {
		t.Fatalf("a place without an ID: %v", a.here())
	}
}
