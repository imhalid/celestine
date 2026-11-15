package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A body's links to a page are written to its new title: chips and embeds, an alias kept, [[links]] in a table's
// cells; code and other pages' links stay.
func TestRelinkBody(t *testing.T) {
	body := encodeBody(importMarkdown("See [[Dune]], [[dune|the book]], [[Arrakis]] and `[[Dune]]`.\n\n![[Dune]]\n\n| a | b |\n| --- | --- |\n| [[Dune]] | x |\n"))
	nb, n := relinkBody(body, "Dune", "Dune (1965)")
	md := bodyText(nb)
	if n != 4 || !strings.Contains(md, "[[Dune (1965)]], [[Dune (1965)|the book]], [[Arrakis]]") ||
		!strings.Contains(md, "`[[Dune]]`") || !strings.Contains(md, "![[Dune (1965)]]") || !strings.Contains(md, `"[[Dune (1965)]]","x"`) {
		t.Fatalf("%d links:\n%s", n, md)
	}
	if same, n := relinkBody(body, "Arrakis2", "X"); n != 0 || same != body {
		t.Fatalf("a body without the links changed: %d", n)
	}
}

// Renaming a page in its title field writes the links to it in another page once the field is left, not at each
// key; the toast's Undo puts them back.
func TestRenameRelinks(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	p := a.page
	other, _ := p.Coll.NewPage(nil, "Beta")
	if err := other.SaveText("Bkz. [[Sayfa]] ve [[sayfa|takma ad]]\n"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	r, ok := tt.Find(msg("common_title"))
	if !ok {
		t.Fatalf("no title: %v", tt.Texts())
	}
	tt.ClickAt(r.X+20, r.Y+r.H/2)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Yeni ad")
	tt.Frame()
	if p.Title != "Yeni ad" {
		t.Fatalf("title %q", p.Title)
	}
	text := func() string { s, _ := other.Text(); return s }
	if got := text(); !strings.Contains(got, "[[Sayfa]]") {
		t.Fatalf("the links were written while the title was typed: %q", got)
	}
	tt.Key(0, ui.KeyEnter) // into the text: the title is left
	tt.Advance(700 * time.Millisecond)
	if got := text(); !strings.Contains(got, "[[Yeni ad]] ve [[Yeni ad|takma ad]]") {
		t.Fatalf("the links were not written: %q", got)
	}
	if !tt.HasText("2 bağlantı güncellendi") {
		t.Fatalf("no toast: %v", tt.Texts())
	}
	if err := tt.Click("Geri al"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	if got := text(); !strings.Contains(got, "[[Sayfa]] ve [[sayfa|takma ad]]") {
		t.Fatalf("undo: %q", got)
	}
	if p.Title != "Yeni ad" {
		t.Fatalf("undo changed the title: %q", p.Title)
	}
}

// Links are kept when the new title is another page's: they would go to that page.
func TestRenameRelinksNotOntoAnother(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	p := a.page
	other, _ := p.Coll.NewPage(nil, "Beta")
	other.SaveText("Bkz. [[Sayfa]]\n")
	p.Coll.NewPage(nil, "Gama")
	tt.Frame()
	p.SetTitle("Gama")
	if n := a.relinkRenamed(p, "Sayfa", false); n != 0 {
		t.Fatalf("%d links written onto another page's title", n)
	}
	if got, _ := other.Text(); !strings.Contains(got, "[[Sayfa]]") {
		t.Fatalf("the link changed: %q", got)
	}
	if !strings.Contains(a.notice, "Gama") {
		t.Fatalf("no notice: %q", a.notice)
	}
}

// The sidebar's rename writes the links too.
func TestSidebarRenameRelinks(t *testing.T) {
	a, tt := newTestApp(t, "x")
	p := a.page
	other, _ := p.Coll.NewPage(nil, "Beta")
	other.SaveText("[[Sayfa]]\n")
	tt.Frame()
	a.renaming, a.renameText = p, "Yeni"
	a.commitRename()
	tt.Frame()
	if got, _ := other.Text(); !strings.Contains(got, "[[Yeni]]") {
		t.Fatalf("not written: %q", got)
	}
}
