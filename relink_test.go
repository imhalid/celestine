package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// A body's links to a page are written to its new title: chips and embeds, an alias kept, [[links]] in a table's
// cells; code and other pages' links stay.
func TestRelinkBody(t *testing.T) {
	body := encodeBody(importMarkdown("See [[Dune]], [[dune|the book]], [[Arrakis]] and `[[Dune]]`.\n\n![[Dune]]\n\n| a | b |\n| --- | --- |\n| [[Dune]] | x |\n"))
	nb, n := relinkBody(body, map[string]string{"dune": "Dune (1965)"})
	md := bodyText(nb)
	if n != 4 || !strings.Contains(md, "[[Dune (1965)]], [[Dune (1965)|the book]], [[Arrakis]]") ||
		!strings.Contains(md, "`[[Dune]]`") || !strings.Contains(md, "![[Dune (1965)]]") || !strings.Contains(md, `"[[Dune (1965)]]","x"`) {
		t.Fatalf("%d links:\n%s", n, md)
	}
	if same, n := relinkBody(body, map[string]string{"arrakis2": "X"}); n != 0 || same != body {
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

// The [[links]] typed in a callout or a footnote, held blocks whose text is read as text, are written too; those in
// a block of code or of math are not (#560).
func TestRelinkBodyHeldBlocks(t *testing.T) {
	md := "> [!note] See [[Dune]]\n> and [[Dune|it]] again\n\n```\n[[Dune]] in code\n```\n\n$$\n[[Dune]]\n$$\n\nText [[Dune]].\n"
	body := encodeBody(importMarkdown(md))
	nb, n := relinkBody(body, map[string]string{"dune": "Dune (1965)"})
	got := bodyText(nb)
	if n != 3 || !strings.Contains(got, "[!note] See [[Dune (1965)]]") || !strings.Contains(got, "[[Dune (1965)|it]]") ||
		!strings.Contains(got, "[[Dune]] in code") || !strings.Contains(got, "$$\n[[Dune]]\n$$") || !strings.Contains(got, "Text [[Dune (1965)]].") {
		t.Fatalf("%d links:\n%s", n, got)
	}
}

// A page named for the first time is not renamed from "Untitled", in either language: the links to "Untitled" stay.
func TestRenameFromUntitledKeepsLinks(t *testing.T) {
	for _, l := range languages {
		a, tt := newTestApp(t, "Metin\n")
		p := a.page
		name := locales.In(l.code).CommonUntitled()
		other, _ := p.Coll.NewPage(nil, "Beta")
		other.SaveText("See [[" + name + "]]\n")
		p.SetTitle("Named")
		tt.Frame()
		if n := a.relinkRenamed(p, name, false); n != 0 {
			t.Fatalf("%s: %d links to %q written", l.code, n, name)
		}
		if got, _ := other.Text(); !strings.Contains(got, "[["+name+"]]") {
			t.Fatalf("%s: the link changed: %q", l.code, got)
		}
	}
}

// A rename being typed when the app went (killed, or crashed) is kept in the settings, and its links are written at
// the next start; one being typed in a pane that is closed is ended with it (#560).
func TestRenameEndedLater(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	p := a.page
	other, _ := p.Coll.NewPage(nil, "Beta")
	other.SaveText("See [[Sayfa]]\n")
	tt.Frame()
	a.titleTyped(p, "Yeni")
	if a.settings.PendingRenames[p.Dir] != "Sayfa" {
		t.Fatalf("the rename is not kept: %v", a.settings.PendingRenames)
	}
	// the app went: the next one finds the rename in its settings
	a.retitle = titleEdit{}
	a.finishRenames()
	tt.Frame()
	if got, _ := other.Text(); !strings.Contains(got, "[[Yeni]]") {
		t.Fatalf("not written at the start: %q", got)
	}
	if len(a.settings.PendingRenames) != 0 {
		t.Fatalf("still kept: %v", a.settings.PendingRenames)
	}

	// typed in a side pane, which is closed before the field is left
	tt.SetSize(1400, 800)
	third, _ := p.Coll.NewPage(nil, "Gama")
	other.SaveText("See [[Gama]]\n")
	a.openInSide(place{"page", third.Dir})
	tt.Advance(300 * time.Millisecond)
	a.titleTyped(third, "Delta")
	a.closePane(a.ps.active)
	tt.Advance(300 * time.Millisecond)
	if got, _ := other.Text(); !strings.Contains(got, "[[Delta]]") {
		t.Fatalf("not written when its pane closed: %q", got)
	}
}

// A journal's new format retitles its days, and the links to them follow, one notice for all (#560).
func TestJournalFormatRelinks(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	a.clock = func() time.Time { return time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local) }
	day, err := a.todayPage()
	if err != nil {
		t.Fatal(err)
	}
	c := day.Coll
	c.JournalFormat = "DD.MM.YYYY" // from one format of its own to another
	a.retitleJournal(c)
	tt.Frame()
	other, _ := a.lib.Collections[0].NewPage(nil, "Beta")
	other.SaveText("See [[04.03.2026]]\n")
	tt.Frame()
	c.JournalFormat = "MM/DD/YYYY"
	a.retitleJournal(c)
	tt.Frame()
	if day.Title != "03/04/2026" {
		t.Fatalf("the day's title: %q", day.Title)
	}
	if got, _ := other.Text(); !strings.Contains(got, "[[03/04/2026]]") {
		t.Fatalf("the link to the day: %q", got)
	}
}
