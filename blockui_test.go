package main

import (
	"github.com/egoist/mygo/ui"
	"slices"
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

// A block dragged opens room where it would go: the paragraph there and those
// under it slide down (the editor's Space) while the pointer aims at it, the
// room shuts when it aims elsewhere, and let go, the block is there and no
// room is left open.
func TestBlockDragOpensRoom(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n\nÜç\n\nDört\n")
	tt.Advance(300 * time.Millisecond)
	r := a.editorR
	topOf := func(i int) float32 {
		for _, b := range a.boxes {
			if b.Index == i {
				return b.Top
			}
		}
		t.Fatalf("no box %d", i)
		return 0
	}
	before := topOf(2) // Üç
	b0 := a.boxes[0]
	tt.Move(r.X+40, r.Y+b0.Top+b0.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	y1 := r.Y + before + 2 // the top half of Üç: before it
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Advance(500 * time.Millisecond)
	if !a.grip.dragging || a.grip.toIdx != 2 {
		t.Fatalf("aim: dragging %v, toIdx %d", a.grip.dragging, a.grip.toIdx)
	}
	if room := a.blockRoom(2); room < a.grip.roomH-1 {
		t.Fatalf("room %.1f, want %.1f", room, a.grip.roomH)
	}
	if moved := topOf(2) - before; moved < a.grip.roomH-1 {
		t.Fatalf("Üç moved %.1f, want %.1f", moved, a.grip.roomH)
	}
	// the aim holds though the room moved Üç down: it is read without the room
	tt.Move(x0, y1+1)
	tt.Frame()
	if a.grip.toIdx != 2 {
		t.Fatalf("aim moved with the room: %d", a.grip.toIdx)
	}
	tt.Release(x0, y1+1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.doc); got != "İki\n\nBir\n\nÜç\n\nDört" {
		t.Fatalf("dragged: %q", got)
	}
	if len(a.grip.rooms) != 0 {
		t.Fatalf("rooms left open: %v", a.grip.rooms)
	}
}

// The block's menu is MyGo's, drawn in the window (#305): Dönüştür… is a submenu of the kinds, and choosing one turns
// the block; Esc closes it.
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
	if !a.grip.menu || !slices.Contains(tt.Menu(), "Dönüştür…") {
		t.Fatalf("not open: %v %v", a.grip.menu, tt.Menu())
	}
	if err := tt.ChooseMenuItem("Dönüştür…", "Başlık 1"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(200 * time.Millisecond)
	if a.grip.menu || a.doc.Text().ParagraphStyleAt(a.boxes[1].Start).Heading != 1 {
		t.Fatalf("not turned: menu %v, style %+v", a.grip.menu, a.doc.Text().ParagraphStyleAt(a.boxes[1].Start))
	}
	if err := tt.Click("Blok tutamağı"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.grip.menu || tt.Menu() != nil {
		t.Fatal("Esc did not close it")
	}
}

// A block dragged to the bottom edge of a long page and held there scrolls
// the page down, and let go it goes where the pointer is then.
func TestGripDragScrollsAtEdge(t *testing.T) {
	text := ""
	for i := 0; i < 80; i++ {
		text += "Satır " + string(rune('A'+i%26)) + string(rune('a'+i/26)) + "\n\n"
	}
	a, tt := newTestApp(t, text)
	tt.Advance(300 * time.Millisecond)
	if a.pageScroll.MaxY <= 0 {
		t.Fatal("the page does not scroll")
	}
	r, b := a.editorR, a.boxes[0]
	tt.Move(r.X+40, r.Y+b.Top+b.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	y1 := a.scrollR.Y + a.scrollR.H - 4
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	if !a.grip.dragging {
		t.Fatal("no drag")
	}
	for k := 0; k < 30; k++ { // held still at the edge
		tt.Frame()
	}
	if a.pageScroll.Y < 200 {
		t.Fatalf("scrolled only %v", a.pageScroll.Y)
	}
	tt.Release(x0, y1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.doc); got[:len("Satır Aa")] == "Satır Aa" {
		t.Fatalf("not moved: %q", got[:40])
	}
}
