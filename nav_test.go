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
	first := a.page
	a.openCollection(one)
	a.open(ps["B"])
	a.open(ps["C"])
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	tt.Frame()
	if a.page != ps["B"] || a.coll != nil {
		t.Fatalf("back: %v", a.here())
	}
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	tt.Frame()
	if a.coll != one {
		t.Fatalf("back to the collection: %v", a.here())
	}
	if err := tt.Click("İleri (⌘])"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != ps["B"] || a.coll != nil {
		t.Fatalf("forward: %v", a.here())
	}
	// B thrown away: back from C goes past it
	a.open(ps["C"])
	a.trashPage(ps["B"])
	a.step(-1)
	if a.coll != one {
		t.Fatalf("past a page gone: %v", a.here())
	}
	a.step(-1)
	if a.page != first {
		t.Fatalf("to the first: %v", a.here())
	}
	if a.canStep(-1) {
		t.Fatal("back from the first")
	}
}
