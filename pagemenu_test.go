package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPageMoreMenu: the "…" of the window opens what the page offers: a copy
// of it, right after it with its sub-pages and pictures, its link, its Markdown.
func TestPageMoreMenu(t *testing.T) {
	a, tt := newTestApp(t, "gövde\n")
	p := a.page
	sub, _ := p.Coll.NewPage(p, "Alt")
	sub.Save("altın metni\n")
	if err := a.addImage("foto.png", pngBytes(20, 20)); err != nil {
		t.Fatal(err)
	}
	a.save()
	tt.Frame()
	var copied string
	a.clipText = func(s string) { copied = s }

	if err := tt.Click("Sayfa menüsü"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Bağlantıyı kopyala"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	if copied != "[["+p.Title+"]]" {
		t.Fatalf("link: %q", copied)
	}
	tt.Click("Sayfa menüsü")
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Markdown'u kopyala"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copied, "gövde") || !strings.Contains(copied, "![foto](resim-") {
		t.Fatalf("markdown: %q", copied)
	}

	tt.Click("Sayfa menüsü")
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Çoğalt"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	pages := a.lib.Collections[0].Pages
	if len(pages) != 2 || pages[0] != p || pages[1].Title != p.Title+" (kopya)" || a.page != pages[1] {
		t.Fatalf("pages: %d, open %v", len(pages), a.page.Title)
	}
	cp := pages[1]
	if len(cp.Children) != 1 || cp.Children[0].Title != "Alt" || cp.Children[0].Parent != cp {
		t.Fatalf("sub-pages: %+v", cp.Children)
	}
	if body, _ := cp.Children[0].Body(); body != "altın metni\n" {
		t.Fatalf("sub-page text %q", body)
	}
	if imgs, _ := filepath.Glob(filepath.Join(cp.Dir, "resim-*.png")); len(imgs) != 1 {
		t.Fatalf("pictures of the copy: %v", imgs)
	}
	if body, _ := cp.Body(); !strings.Contains(body, "gövde") {
		t.Fatalf("copy's text %q", body)
	}
	// read back, it is still there
	again, _ := OpenLibrary(a.lib.Root)
	if len(again.Collections[0].Pages) != 2 {
		t.Fatalf("read back: %d", len(again.Collections[0].Pages))
	}
}

// TestPageExport: the page's file, and the pictures it names, are written together.
func TestPageExport(t *testing.T) {
	a, _ := newTestApp(t, "gövde\n")
	if err := a.addImage("foto.png", pngBytes(20, 20)); err != nil {
		t.Fatal(err)
	}
	a.save()
	dir := t.TempDir()
	out := filepath.Join(dir, "sayfa.md")
	if err := a.writeExport(a.page, a.currentMarkdown(a.page), out); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(out)
	imgs, _ := filepath.Glob(filepath.Join(dir, "resim-*.png"))
	if !strings.Contains(string(md), "gövde") || len(imgs) != 1 {
		t.Fatalf("md %q, pictures %v", md, imgs)
	}
}
