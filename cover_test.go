package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// TestCover: a cover is a file beside the page and a line of its front
// matter; it shows over the title, changes, and goes.
func TestCover(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1000, 800)
	p := a.pane().page
	if err := a.setCover(p, []byte("hello")); err == nil {
		t.Fatal("a text for a cover")
	}
	if err := a.setCover(p, pngBytes(300, 100)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Cover, "kapak-") {
		t.Fatalf("cover %q", p.Cover)
	}
	src, _ := os.ReadFile(p.File())
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
	src, _ = os.ReadFile(p.File())
	if p.Cover != CoverNone || !strings.Contains(string(src), `"cover":"none"`) || strings.Contains(string(src), "kapak-") {
		t.Fatalf("not away (the page says it wants none):\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, cur)); err == nil {
		t.Fatal("the file stays")
	}
	// the page menu knows
	var titles []string
	for _, it := range a.pageMenu(p, false) {
		titles = append(titles, it.title)
	}
	if !contains(titles, "page_addCover") {
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
	p := a.pane().page
	if err := p.FS().WriteFile(p.File(), []byte(`{"meta": bozuk`)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Body(); err == nil {
		t.Fatal("no error for a page file that is not JSON")
	}
	p.Cover = "kapak.png"
	a.removeCover(p)
	if !hasErrToast(a) || p.Cover != "kapak.png" {
		t.Fatalf("no error toast, cover %q", p.Cover)
	}
	if b, _ := p.FS().ReadFile(p.File()); string(b) != `{"meta": bozuk` {
		t.Fatalf("the file was written over: %q", b)
	}
	if _, _, err := SplitPageChecked(nil); err == nil { // every page file is written with its meta: an empty one is cut short (#140)
		t.Fatal("an empty file read as a page")
	}
}

// TestCoverFrameless: a picture cover is the picture itself, as tall as it is for the pane's width within its bounds,
// with its buttons over it as the pointer is on it (#478).
func TestCoverFrameless(t *testing.T) {
	a, tt := newTestApp(t, "text\n")
	tt.SetSize(1200, 900)
	p := a.pane().page
	a.open(p)
	if err := a.setCover(p, pngBytes(400, 100)); err != nil { // 4:1
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Frame() // the height follows the width laid out a frame before
	a.mu.Lock()
	w, h := a.coverW, a.coverPicH(p)
	a.mu.Unlock()
	if w <= 0 || h != min(max(w/4, coverMinH), coverMaxH) {
		t.Fatalf("the picture is %v tall at %v wide, not its own height", h, w)
	}
	a.clock = func() time.Time { return time.Now().Add(time.Minute) } // a file of another name, not the picture kept for the first
	if err := a.setCover(p, pngBytes(100, 100)); err != nil {         // square: cut at the most
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	a.mu.Lock()
	h = a.coverPicH(p)
	a.mu.Unlock()
	if h != coverMaxH {
		t.Fatalf("a square picture is %v tall, not cut at %v", h, coverMaxH)
	}
	tt.Move(600, barH+40)
	tt.Advance(400 * time.Millisecond)
	for _, label := range []string{locales.PageChangeCover(), locales.PageCoverMore()} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("no %q over the cover", label)
		}
	}
}

// TestCoverReposition: Reposition in the cover's menu lets the picture be dragged inside its band; letting go writes
// the position to the page's file, and Done ends it (#524).
func TestCoverReposition(t *testing.T) {
	a, tt := newTestApp(t, "text\n")
	tt.SetSize(1200, 900)
	p := a.pane().page
	a.open(p)
	if err := a.setCover(p, pngBytes(100, 100)); err != nil { // square: cropped to the band, room to move
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Frame()
	a.mu.Lock()
	w, h := a.coverW, a.coverPicH(p)
	a.coverFix = p
	a.mu.Unlock()
	tt.Advance(100 * time.Millisecond)
	x, y := coverInset+w/2, a.coverTopOf()+h/2
	tt.Move(x, y)
	tt.Advance(100 * time.Millisecond)
	tt.Press(x, y)
	tt.Advance(50 * time.Millisecond)
	tt.Move(x, y+30) // dragged down: the top of the picture comes into view
	tt.Advance(50 * time.Millisecond)
	tt.Move(x, y+60)
	tt.Advance(50 * time.Millisecond)
	tt.Release(x, y+60)
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	a.mu.Lock()
	shift := p.CoverShift
	a.mu.Unlock()
	if shift >= 0 {
		t.Fatalf("dragged down, the position is %d: the top should show (negative)", shift)
	}
	b, err := p.FS().ReadFile(p.File())
	if err != nil || !strings.Contains(string(b), "covershift") {
		t.Fatalf("the position was not written: %v %q", err, b)
	}
	if got := coverImageRect(ui.Rect{W: 100, H: 50}, 100, 100, -100); got.Y != 0 {
		t.Fatalf("the top shown: the picture starts at %v, not 0", got.Y)
	}
	if got := coverImageRect(ui.Rect{W: 100, H: 50}, 100, 100, 100); got.Y != -50 {
		t.Fatalf("the bottom shown: the picture starts at %v, not -50", got.Y)
	}
	if err := tt.Click(locales.PageCoverDone()); err != nil {
		t.Fatal("no Done while repositioning: ", err)
	}
	tt.Advance(100 * time.Millisecond)
	a.mu.Lock()
	fix := a.coverFix
	a.mu.Unlock()
	if fix != nil {
		t.Fatal("Done did not end it")
	}
}
