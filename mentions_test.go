package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestUnlinkedMentions: the section under a page, opened, lists the pages that name it as a whole word without a link
// - by its title or an alias, not inside another word nor in a page that links to it already - and Link makes the
// first mention a link, which the notice's Undo takes back (#548).
func TestUnlinkedMentions(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 1000)
	c := a.page.Coll
	also, _ := c.AddProperty("Also", PropAliases, nil)
	dune, _ := c.NewPage(nil, "Dune")
	dune.SetValue(also.ID, "Arrakis")
	mk := func(title, text string) *Page {
		p, _ := c.NewPage(nil, title)
		if err := p.SaveText(text); err != nil {
			t.Fatal(err)
		}
		return p
	}
	reads := mk("Reading", "I read dune last year, and `Dune` in code.\n")
	planet := mk("Planet", "Arrakis is a desert.\n")
	mk("Dunes", "the dunes of the coast\n")
	mk("Linked", "see [[Dune]] and Dune again\n")
	a.linksStale()
	a.links.built = time.Time{}
	a.open(dune)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click(locales.MentionsTitle()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	got := map[string]bool{}
	for _, m := range a.mentions.found {
		got[m.page.Title] = true
	}
	if len(got) != 2 || !got["Reading"] || !got["Planet"] {
		t.Fatalf("the mentions: %v", got)
	}
	var rd mention
	for _, m := range a.mentions.found {
		if m.page == reads {
			rd = m
		}
	}
	a.linkMention(dune, rd)
	body, _ := reads.Text()
	if !strings.Contains(body, "read [[Dune]] last year") || !strings.Contains(body, "`Dune`") {
		t.Fatalf("linked: %q", body)
	}
	if got := a.linksTo(dune, a.clock(), true); len(got) != 2 {
		t.Fatalf("the backlinks after: %v", got)
	}
	// by its alias, as written; the notice's Undo puts the page back
	a.linkMention(dune, mention{page: planet, name: "Arrakis"})
	if body, _ := planet.Text(); !strings.Contains(body, "[[Arrakis]] is a desert") {
		t.Fatalf("by the alias: %q", body)
	}
	if a.pageTitled("Arrakis") != dune {
		t.Fatal("the alias link does not resolve")
	}
	last := a.toasts.items[len(a.toasts.items)-1]
	if last.action != locales.CommonUndo() || last.fn == nil {
		t.Fatalf("the notice: %+v", last)
	}
	last.fn()
	if body, _ := planet.Text(); strings.Contains(body, "[[") {
		t.Fatalf("after Undo: %q", body)
	}
}
