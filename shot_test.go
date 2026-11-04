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
	tt.Click("Görünüm: Tablo")
	tt.Advance(100 * time.Millisecond)
	save("table")
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
