package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A caret's eyes look the way the writing goes: right as it is written, also when a new line starts; left for
// Backspace, right for Delete; as they were when nothing is written (#283).
func TestGazeWant(t *testing.T) {
	for _, c := range []struct {
		name string
		was  float64
		k    kick
		want float64
	}{
		{"written", -1, kick{wrote: 1}, 1},
		{"a new line", -1, kick{wrote: 1, typed: "\n"}, 1},
		{"Backspace", 1, kick{erased: 1, dir: -1}, -1},
		{"Delete", -1, kick{erased: 1, dir: 1}, 1},
		{"nothing written: as it was", -1, kick{}, -1},
	} {
		if got := gazeWant(c.was, c.k); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Kawaii, a caret with a face, looks right while it writes and left once Backspace erases, easing between.
func TestCaretLooksTheWayItGoes(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(700, 420)
	a.setCaret(func() caretKind {
		for k := caretKind(0); k < caretCount(); k++ {
			if caretName(k) == "Kawaii" {
				return k
			}
		}
		t.Fatal("no Kawaii")
		return 0
	}())
	tt.Frame()
	g := a.fxNamed("Kawaii").(*kawaii)
	typeEach(tt, "merhaba")
	tt.Advance(400 * time.Millisecond)
	if g.gaze() < 0.8 {
		t.Fatalf("writing, it looks at %v, not right", g.gaze())
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Advance(400 * time.Millisecond)
	if g.gaze() > -0.8 {
		t.Fatalf("erasing, it looks at %v, not left", g.gaze())
	}
}
