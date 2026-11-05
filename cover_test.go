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
	if !strings.Contains(string(src), "cover: kapak-") {
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
	if body, _ := p.Body(); body != "metin\n" {
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
	if p.Cover != "" || strings.Contains(string(src), "cover") {
		t.Fatalf("not away:\n%s", src)
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
