package main

import (
	"github.com/egoist/mygo/ui"
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
	if err := tt.Click("Dönüştür…"); err != nil {
		t.Fatal(err, tt.Texts())
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

// The block's menu answers the keys: ↓ ↵ goes into Dönüştür…, Esc goes back
// and then closes; Sil deletes the block.
func TestBlockMenuKeys(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n")
	tt.Frame()
	y := a.editorR.Y + a.boxes[1].Top + 6
	tt.Move(a.editorR.X+30, y)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	tt.Move(g.X+g.W/2, g.Y+g.H/2)
	tt.Frame()
	if err := tt.Click("Blok tutamağı"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if !a.grip.menu {
		t.Fatal("not open")
	}
	tt.Key(0, ui.KeyEnter) // the first: Dönüştür…
	tt.Advance(100 * time.Millisecond)
	if a.grip.view != "turn" {
		t.Fatalf("not in the kinds: %q", a.grip.view)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.grip.view != "" || !a.grip.menu {
		t.Fatalf("Esc did not go back: %+v", a.grip)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.grip.menu {
		t.Fatal("Esc did not close it")
	}
}

// A right click in the margin by a line opens the block's menu, and one on a
// page in the tree opens the page's, in our own design.
func TestContextMenus(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n")
	tt.SetSize(1500, 700)
	tt.Frame()
	tt.Frame()
	y := a.editorR.Y + a.boxes[1].Top + 8
	tt.RightClickAt(a.pageR.X+10, y)
	tt.Advance(200 * time.Millisecond)
	if !a.grip.menu {
		t.Fatalf("no block menu from the margin: %+v", a.grip)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if err := tt.RightClick("Sayfa: Sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if !a.ctx.open {
		t.Fatal("no menu for the page")
	}
	for _, want := range []string{"Aç", "Yeniden adlandır", "Çöpe taşı"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in %v", want, tt.Texts())
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.ctx.open {
		t.Fatal("Esc did not close it")
	}
}
