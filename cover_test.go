package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCover: a cover is a file beside the page and a line of its front
// matter; it shows over the title, changes, and goes.
func TestCover(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1000, 800)
	p := a.page
	if err := a.setCover(p, []byte("hello")); err == nil {
		t.Fatal("a text for a cover")
	}
	if err := a.setCover(p, pngBytes(300, 100)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Cover, "kapak-") {
		t.Fatalf("cover %q", p.Cover)
	}
	src, _ := os.ReadFile(p.file())
	if !strings.Contains(string(src), `"cover":"kapak-`) {
		t.Fatalf("front matter:\n%s", src)
	}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText("Kapak resmi") && len(tt.Texts()) == 0 {
		t.Fatal("not shown")
	}
	// read again
	again, _ := OpenLibrary(a.lib.Root)
	if got := again.Collections[0].Pages[0].Cover; got != p.Cover {
		t.Fatalf("read back %q", got)
	}
	// the body is still the body
	if body, _ := p.Text(); body != "metin\n" {
		t.Fatalf("body %q", body)
	}
	// changed: the old file goes
	old := p.Cover
	a.clock = func() time.Time { return time.Now().Add(2 * time.Second) }
	if err := a.setCover(p, pngBytes(50, 50)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, old)); err == nil || p.Cover == old {
		t.Fatalf("the old cover stays: %q -> %q", old, p.Cover)
	}
	// and taken away
	cur := p.Cover
	a.removeCover(p)
	src, _ = os.ReadFile(p.file())
	if p.Cover != coverNone || !strings.Contains(string(src), `"cover":"none"`) || strings.Contains(string(src), "kapak-") {
		t.Fatalf("not away (the page says it wants none):\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, cur)); err == nil {
		t.Fatal("the file stays")
	}
	// the page menu knows
	var titles []string
	for _, it := range a.pageMoreMenu(p) {
		titles = append(titles, it.title)
	}
	if !contains(titles, "Kapak resmi ekle…") {
		t.Fatalf("menu: %v", titles)
	}
}

// TestCoverOnCards: a page with a cover has it as a picture on its card and its row.
func TestCoverOnCards(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Bitti"})
	var dune *Page
	for _, p := range coll.All() {
		if p.Title == "Dune" {
			dune = p
		}
	}
	if err := a.setCover(dune, pngBytes(400, 200)); err != nil {
		t.Fatal(err)
	}
	tt.SetSize(1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	for _, view := range []string{"Liste", "Pano"} {
		if err := tt.Click(view); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		px := 0
		img := tt.Image()
		// the picture's own colors (blue and green ramps) are nowhere else on the page
		for y := 0; y < img.Bounds().Dy(); y += 2 {
			for x := 252; x < img.Bounds().Dx(); x += 2 {
				if r, g, b, _ := img.At(x, y).RGBA(); b>>8 > 190 && r>>8 < 120 && g>>8 < 120 {
					px++
				}
			}
		}
		if px < 40 {
			t.Fatalf("%s: no picture drawn (%d px)", view, px)
		}
	}
}

// A page file that is not JSON is not written over by what only wants to change the cover: it is told, and left.
func TestCorruptPageIsLeftAlone(t *testing.T) {
	a, _ := newTestApp(t, "x")
	p := a.page
	if err := p.fs().WriteFile(p.file(), []byte(`{"meta": bozuk`)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Body(); err == nil {
		t.Fatal("no error for a page file that is not JSON")
	}
	p.Cover = "kapak.png"
	a.removeCover(p)
	if a.saveErr == "" || p.Cover != "kapak.png" {
		t.Fatalf("saveErr %q cover %q", a.saveErr, p.Cover)
	}
	if b, _ := p.fs().ReadFile(p.file()); string(b) != `{"meta": bozuk` {
		t.Fatalf("the file was written over: %q", b)
	}
	if _, body, err := splitPageChecked(nil); err != nil || body != "[]" {
		t.Fatalf("an empty file is an empty page: %q %v", body, err)
	}
}
