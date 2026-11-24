package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestLibraryReplace: the search's replace counts what the library has of a plain phrase - any case when it is
// written in small letters, as written with a capital - and replaces it in every page, not inside chips, each kept as a
// version; the notice's Undo puts them back; a search for a tag or a property has no replace (#553).
func TestLibraryReplace(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 800)
	c := a.page.Coll
	mk := func(title, text string) *Page {
		p, _ := c.NewPage(nil, title)
		if err := p.SaveText(text); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pa := mk("A", "the Spice must flow; spice is life\n")
	pb := mk("B", "no spice here? [[spice]] and spice!\n")
	mk("C", "nothing\n")
	a.linksStale()
	a.links.built = time.Time{}
	for q, want := range map[string]bool{"spice": true, `"spice must"`: true, "tag:x": false, "#spice": false, "a OR b": false} {
		if _, ok := replaceable(q); ok != want {
			t.Fatalf("replaceable(%q) = %v", q, ok)
		}
	}
	a.openSearch("spice")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click(locales.ReplaceShow()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pages, n := a.replaceCount("spice"); len(pages) != 2 || n != 4 {
		t.Fatalf("counted %d in %d pages", n, len(pages))
	}
	tt.Type("melange") // into the field, which took the focus as it showed
	tt.Frame()
	if a.repl.with != "melange" {
		t.Fatalf("typed into the field: %q", a.repl.with)
	}
	if err := tt.Click(locales.ReplaceInPages(4, 2)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	ba, _ := pa.Text()
	bb, _ := pb.Text()
	if !strings.Contains(ba, "the melange must flow; melange is life") || !strings.Contains(bb, "no melange here? [[spice]] and melange!") {
		t.Fatalf("replaced: %q, %q", ba, bb)
	}
	last := a.toasts.items[len(a.toasts.items)-1] // the notice, with its Undo
	if last.fn == nil || last.action != locales.CommonUndo2() {
		t.Fatalf("the notice: %+v", last)
	}
	last.fn()
	ba, _ = pa.Text()
	if !strings.Contains(ba, "the Spice must flow; spice is life") {
		t.Fatalf("after Undo: %q", ba)
	}
	// with a capital, as written
	a.replaceIn([]*Page{pa}, "Spice", "Dune")
	if ba, _ = pa.Text(); !strings.Contains(ba, "the Dune must flow; spice is life") {
		t.Fatalf("exact: %q", ba)
	}
}
