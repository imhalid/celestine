package main

import (
	"testing"
	"time"
)

func TestGripMenuAndDrag(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n\nÜç\n")
	tt.Advance(300 * time.Millisecond)
	r := a.editorR
	box := func(s string) (float32, float32) {
		start := at(a.doc, s)
		for _, b := range a.boxes {
			if b.Start == start {
				return r.Y + b.Top, b.Height
			}
		}
		t.Fatalf("no box for %q", s)
		return 0, 0
	}
	y, h := box("İki")
	tt.Move(r.X+40, y+h/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok || a.grip.para != at(a.doc, "İki") {
		t.Fatalf("no grip: %v %d", ok, a.grip.para)
	}
	// the menu: turn it into a heading
	tt.Move(g.X+g.W/2, g.Y+g.H/2)
	tt.Frame()
	if err := tt.Click("Blok tutamağı"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Başlık 1"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.doc); got != "Bir\n\n# İki\n\nÜç" {
		t.Fatalf("turned: %q", got)
	}
	// drag "Bir" below "Üç"
	y, h = box("Bir")
	tt.Move(r.X+40, y+h/2)
	tt.Frame()
	tt.Frame()
	g, _ = tt.Find("Blok tutamağı")
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	yu, hu := box("Üç")
	y1 := yu + hu + 10
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	if !a.grip.dragging {
		t.Fatal("no drag")
	}
	tt.Release(x0, y1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.doc); got != "# İki\n\nÜç\n\nBir" {
		t.Fatalf("dragged: %q", got)
	}
}
