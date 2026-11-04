package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestShot draws the welcome document: run with SHOT=file.png.
func TestShot(t *testing.T) {
	out := os.Getenv("SHOT")
	if out == "" {
		t.Skip()
	}
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	c := lib.Collections[0]
	for _, title := range []string{"Proje planı", "Okuma listesi"} {
		p, _ := c.NewPage(nil, title)
		c.NewPage(p, "Alt sayfa: "+title)
	}
	a := newApp(lib)
	a.doc.SetSelection(40, 40)
	tt := ui.NewTester(a.view, 1180, 820)
	tt.Advance(400 * time.Millisecond)
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
}

// TestViewShots draws a collection's list and table: run with VIEWS=dir.
func TestViewShots(t *testing.T) {
	dir := os.Getenv("VIEWS")
	if dir == "" {
		t.Skip()
	}
	_, tt, _ := viewsLibrary(t)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(100 * time.Millisecond)
	save("list")
	tt.Click("Tablo")
	tt.Advance(100 * time.Millisecond)
	save("table")
	tt.Click("Pano")
	tt.Advance(time.Second)
	save("board")
	// a card held over the next column
	from, _ := tt.Find("Kart: Dune")
	to, _ := tt.Find("Sürüyor")
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+(to.X+60-x0)*float32(k)/10, y0+(to.Y+50-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Advance(400 * time.Millisecond)
	save("board-drag")
	tt.Release(to.X+60, to.Y+50)
	tt.Advance(time.Second)
	save("board-dropped")
}

// TestFoldShots draws the sidebar open, folding and folded: run with FOLD=dir.
func TestFoldShots(t *testing.T) {
	dir := os.Getenv("FOLD")
	if dir == "" {
		t.Skip()
	}
	lib, _ := OpenLibrary(t.TempDir())
	seed(lib)
	c := lib.Collections[0]
	for _, title := range []string{"Proje planı", "Okuma listesi"} {
		p, _ := c.NewPage(nil, title)
		c.NewPage(p, "Alt sayfa")
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 760)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(time.Second)
	save("fold-0-open")
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Advance(110 * time.Millisecond)
	save("fold-1-mid")
	tt.Advance(time.Second)
	save("fold-2-closed")
}

// TestPolishShots draws the tabs' pill on its way and a row being renamed: run with POLISH=dir.
func TestPolishShots(t *testing.T) {
	dir := os.Getenv("POLISH")
	if dir == "" {
		t.Skip()
	}
	a, tt, _ := viewsLibrary(t)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(time.Second)
	tt.Click("Tablo")
	tt.Advance(90 * time.Millisecond)
	save("pill-mid")
	tt.Advance(time.Second)
	tt.RightClick("Sayfa: Dune")
	tt.ChooseMenuItem("Yeniden adlandır")
	tt.Advance(300 * time.Millisecond)
	_ = a
	save("rename")
}

// TestDragShots draws a drag on its way: a line between rows, then a row lit
// to take the page under it. Run with DRAG=dir.
func TestDragShots(t *testing.T) {
	dir := os.Getenv("DRAG")
	if dir == "" {
		t.Skip()
	}
	lib, ps, _, _ := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 700)
	tt.Advance(time.Second)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	r := a.treeR
	x := r.X + 70
	y0 := r.Y + float32(rowOf(a, ps["C"]))*rowH + rowH/2
	tt.Press(x, y0)
	for _, to := range []struct {
		row  int
		frac float32
		name string
	}{{rowOf(a, ps["A"]), 0.12, "drag-line"}, {rowOf(a, ps["A"]), 0.5, "drag-into"}} {
		y := r.Y + (float32(to.row)+to.frac)*rowH
		for k := 1; k <= 6; k++ {
			tt.Move(x+float32(k)*6, y0+(y-y0)*float32(k)/6)
			tt.Frame()
		}
		y0 = y
		tt.Advance(400 * time.Millisecond)
		save(to.name)
	}
	tt.Release(x+36, y0)
	tt.Advance(120 * time.Millisecond)
	save("drag-land")
	tt.Advance(300 * time.Millisecond)
	save("drag-rise")
}

// TestTrashShots draws the toast after a page is thrown away and the
// trash: run with TRASH=dir.
func TestTrashShots(t *testing.T) {
	dir := os.Getenv("TRASH")
	if dir == "" {
		t.Skip()
	}
	a, tt, coll := viewsLibrary(t)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	a.trashPage(coll.Pages[1])
	tt.Advance(700 * time.Millisecond)
	save("toast")
	a.lib.TrashCollection(coll)
	a.lib.NewCollection("Notlar")
	tt.Click("Çöp")
	tt.Advance(time.Second)
	save("trash")
}
