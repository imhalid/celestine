package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

func TestGripMenuAndDrag(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n\nÜç\n")
	tt.Advance(300 * time.Millisecond)
	r := a.pane().editorR
	box := func(s string) (float32, float32) {
		start := at(a.pane().doc, s)
		for _, b := range a.pane().boxes {
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
	if !ok || a.pane().grip.para != at(a.pane().doc, "İki") {
		t.Fatalf("no grip: %v %d", ok, a.pane().grip.para)
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
	if got := mdOf(a.pane().doc); got != "Bir\n\n# İki\n\nÜç" {
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
	if !a.pane().grip.dragging {
		t.Fatal("no drag")
	}
	tt.Release(x0, y1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.pane().doc); got != "# İki\n\nÜç\n\nBir" {
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
	y := a.pane().editorR.Y + a.pane().boxes[1].Top + 8
	tt.RightClickAt(a.pane().pageR.X+10, y)
	tt.Advance(200 * time.Millisecond)
	if !a.pane().grip.menu {
		t.Fatalf("no block menu from the margin: %+v", a.pane().grip)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if err := tt.RightClick("Sayfa: Sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if tt.Menu() == nil {
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
	r := a.pane().editorR
	topOf := func(i int) float32 {
		for _, b := range a.pane().boxes {
			if b.Index == i {
				return b.Top
			}
		}
		t.Fatalf("no box %d", i)
		return 0
	}
	before := topOf(2) // Üç
	b0 := a.pane().boxes[0]
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
	if !a.pane().grip.dragging || a.pane().grip.toIdx != 2 {
		t.Fatalf("aim: dragging %v, toIdx %d", a.pane().grip.dragging, a.pane().grip.toIdx)
	}
	if room := a.blockRoom(a.pane(), 2); room < a.pane().grip.roomH-1 {
		t.Fatalf("room %.1f, want %.1f", room, a.pane().grip.roomH)
	}
	if moved := topOf(2) - before; moved < a.pane().grip.roomH-1 {
		t.Fatalf("Üç moved %.1f, want %.1f", moved, a.pane().grip.roomH)
	}
	// the aim holds though the room moved Üç down: it is read without the room
	tt.Move(x0, y1+1)
	tt.Frame()
	if a.pane().grip.toIdx != 2 {
		t.Fatalf("aim moved with the room: %d", a.pane().grip.toIdx)
	}
	tt.Release(x0, y1+1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.pane().doc); got != "İki\n\nBir\n\nÜç\n\nDört" {
		t.Fatalf("dragged: %q", got)
	}
	if len(a.pane().grip.rooms) != 0 {
		t.Fatalf("rooms left open: %v", a.pane().grip.rooms)
	}
}

// The block's menu is MyGo's, drawn in the window (#305): Dönüştür… is a submenu of the kinds, and choosing one turns
// the block; Esc closes it.
func TestBlockMenuKeys(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n")
	tt.Frame()
	y := a.pane().editorR.Y + a.pane().boxes[1].Top + 6
	tt.Move(a.pane().editorR.X+30, y)
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
	if !a.pane().grip.menu || !slices.Contains(tt.Menu(), "Dönüştür…") {
		t.Fatalf("not open: %v %v", a.pane().grip.menu, tt.Menu())
	}
	if err := tt.ChooseMenuItem("Dönüştür…", "Başlık 1"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(200 * time.Millisecond)
	if a.pane().grip.menu || a.pane().doc.Text().ParagraphStyleAt(a.pane().boxes[1].Start).Heading != 1 {
		t.Fatalf("not turned: menu %v, style %+v", a.pane().grip.menu, a.pane().doc.Text().ParagraphStyleAt(a.pane().boxes[1].Start))
	}
	if err := tt.Click("Blok tutamağı"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.pane().grip.menu || tt.Menu() != nil {
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
	if a.pane().pageScroll.MaxY <= 0 {
		t.Fatal("the page does not scroll")
	}
	r, b := a.pane().editorR, a.pane().boxes[0]
	tt.Move(r.X+40, r.Y+b.Top+b.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	y1 := a.pane().scrollR.Y + a.pane().scrollR.H - 4
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	if !a.pane().grip.dragging {
		t.Fatal("no drag")
	}
	for k := 0; k < 30; k++ { // held still at the edge
		tt.Frame()
	}
	if a.pane().pageScroll.Y < 200 {
		t.Fatalf("scrolled only %v", a.pane().pageScroll.Y)
	}
	tt.Release(x0, y1)
	tt.Advance(300 * time.Millisecond)
	if got := mdOf(a.pane().doc); got[:len("Satır Aa")] == "Satır Aa" {
		t.Fatalf("not moved: %q", got[:40])
	}
}

func TestGripDragClearedOnModal(t *testing.T) {
	a, tt := newTestApp(t, "Birinci blok\n\nİkinci blok\n\nÜçüncü blok\n")
	tt.Advance(300 * time.Millisecond)
	r, b := a.pane().editorR, a.pane().boxes[0]
	tt.Move(r.X+40, r.Y+b.Top+b.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	tt.Press(x0, y0)
	tt.Move(x0, y0+40)
	tt.Frame()
	if !a.pane().grip.dragging {
		t.Fatal("expected active grip drag before opening modal")
	}
	a.openCommands()
	tt.Frame()
	tt.Frame()
	if a.pane().grip.dragging || a.pane().grip.pressed {
		t.Fatalf("modal should clear grip drag state, got dragging=%v pressed=%v", a.pane().grip.dragging, a.pane().grip.pressed)
	}
	tt.Release(x0, y0+40)
	tt.Frame()
	if got := mdOf(a.pane().doc); !strings.HasPrefix(got, "Birinci blok") {
		t.Fatalf("cancelled grip drag should not reorder blocks, got %q", got)
	}
}

func TestBlockMenuPaletteKeepsGripAndSelBar(t *testing.T) {
	a, tt := newTestApp(t, "Birinci blok\n\nİkinci blok\n")
	tt.Advance(300 * time.Millisecond)
	r, b0, b1 := a.pane().editorR, a.pane().boxes[0], a.pane().boxes[1]

	// 1. Open the block menu on the first paragraph.
	tt.Move(r.X+40, r.Y+b0.Top+b0.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find(locales.BlocksBlockHandle())
	if !ok {
		t.Fatal("grip not shown on paragraph hover")
	}
	tt.ClickAt(g.X+g.W/2, g.Y+g.H/2)
	tt.Frame()
	tt.Frame()
	if !a.pane().grip.menu {
		t.Fatal("expected block menu to open")
	}

	// 2. Open the command palette and close it again.
	a.openPalette()
	tt.Frame()
	if !a.pal.open {
		t.Fatal("expected palette to open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.pal.open {
		t.Fatal("expected Escape to close palette")
	}

	// 3. Move pointer onto the second paragraph: the grip must reappear and the selection bar must work.
	tt.Move(r.X+40, r.Y+b1.Top+b1.Height/2)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find(locales.BlocksBlockHandle()); !ok {
		t.Fatalf("grip missing after block menu + palette round-trip (menu=%v para=%d)", a.pane().grip.menu, a.pane().grip.para)
	}
	tt.ClickAt(r.X+40, r.Y+b1.Top+b1.Height/2)
	tt.Frame()
	a.pane().doc.SetSelection(b1.Start, b1.Start+len("İkinci"))
	tt.Frame()
	if _, ok := tt.Find(locales.SelectionBold()); !ok {
		t.Fatalf("selection format bar missing after block menu + palette round-trip (menu=%v)", a.pane().grip.menu)
	}
}

// TestBlockDropIsSmooth (#457): a block let go takes the room it opened, so at that moment nothing on the page moves; then
// the hole it left where it was, and the room it did not fill, shut on a spring, every paragraph going a little each
// frame, never jumping; and the block comes to its colors. Down (a tall block, two lines) and up (a short one, in a
// room taller than it).
func TestBlockDropIsSmooth(t *testing.T) {
	const text = "Bir uzun paragraf, iki satıra yayılacak kadar uzun bir metin bu: biraz daha, biraz daha, biraz daha yazı yazı yazı yazı yazı.\n\nİki\n\nÜç\n\nDört\n\nBeş\n"
	for _, c := range []struct {
		name     string
		from, to int
		want     string
	}{
		{"down", 0, 3, "İki\n\nÜç\n\nBir"},
		{"up", 3, 1, "Bir"},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, tt := newTestApp(t, text)
			tt.SetSize(700, 900)
			tt.Advance(300 * time.Millisecond)
			r := a.pane().editorR
			var fy, ty float32
			for _, b := range a.pane().boxes {
				if b.Index == c.from {
					fy = b.Top + b.Height/2
				}
				if b.Index == c.to {
					ty = b.Top + 2
				}
			}
			tt.Move(r.X+40, r.Y+fy)
			tt.Frame()
			tt.Frame()
			g, ok := tt.Find("Blok tutamağı")
			if !ok {
				t.Fatal("no grip")
			}
			x0, y0 := g.X+g.W/2, g.Y+g.H/2
			y1 := r.Y + ty
			tt.Press(x0, y0)
			for k := 1; k <= 10; k++ {
				tt.Move(x0, y0+(y1-y0)*float32(k)/10)
				tt.Frame()
			}
			tt.Advance(600 * time.Millisecond)
			// where each paragraph is, by its first letters
			tops := func() map[string]float32 {
				m := map[string]float32{}
				txt := a.pane().doc.Text()
				for _, b := range a.pane().boxes {
					s, e := txt.ParagraphRange(b.Start)
					m[txt.Slice(s, min(e, s+3)).String()] = b.Top
				}
				return m
			}
			moved := a.pane().doc.Text().Slice(a.pane().grip.from[0], a.pane().grip.from[0]+3).String()
			was := tops()
			tt.Release(x0, y1)
			tt.Frame()
			now := tops()
			for k, y := range was {
				if k != moved && abs32(now[k]-y) > 1 {
					t.Fatalf("let go, %q jumped from %.1f to %.1f", k, y, now[k])
				}
			}
			for f := 0; f < 60; f++ {
				prev := tops()
				tt.Advance(16 * time.Millisecond)
				for k, y := range tops() {
					if d := abs32(y - prev[k]); d > 14 {
						t.Fatalf("frame %d: %q went %.1f in one frame", f, k, d)
					}
				}
			}
			if len(a.pane().grip.settle) != 0 || !a.pane().grip.landAt.IsZero() {
				t.Fatalf("still settling after a second: %v, landed at %v", a.pane().grip.settle, a.pane().grip.landAt)
			}
			if got := mdOf(a.pane().doc); !strings.Contains(got, c.want) {
				t.Fatalf("dropped: %q", got)
			}
		})
	}
}

// TestBlockLayKeepsChips: the block's preview draws what is in its lines that is not text, a page's chip here, in
// the place the editor draws it: without it the preview has a hole where the chip is (#457).
func TestBlockLayKeepsChips(t *testing.T) {
	a, tt := newTestApp(t, "Bu kitabı [[Sayfa]] ile birlikte okumak isterim.\n")
	tt.Advance(100 * time.Millisecond)
	l := a.blockLay(a.pane(), 0, a.pane().editorR.W)
	if len(l.objs) != 1 || l.objs[0].att.Draw == nil {
		t.Fatalf("the chip is not in the block's layout: %+v", l.objs)
	}
	if r := l.objs[0].r; r.W < 10 || r.X < 20 {
		t.Fatalf("the chip's box %+v is not after \"Bu kitabı \"", r)
	}
}
