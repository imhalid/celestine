package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// A new page's title field is empty and focused, "Untitled" its placeholder: what is typed is the title, with nothing
// to clear first. Left with nothing typed, the page is "Untitled" and the field says it; a page renamed keeps its
// title in the field (#598).
func TestNewPageTitleStartsEmpty(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1100, 800)
	c := a.pane().page.Coll
	a.newPage(c, nil)
	tt.Frame() // made from outside a frame: a window at rest draws none of its own
	tt.Advance(300 * time.Millisecond)
	pn := a.pane()
	p := pn.page
	if pn.title != "" || !pn.blank || !pn.titleFocused || !untitled(p.Title) {
		t.Fatalf("new page: field %q, blank %v, focused %v, title %q", pn.title, pn.blank, pn.titleFocused, p.Title)
	}
	if got := a.paneTitleOf(pn); got != locales.CommonUntitled() {
		t.Fatalf("the pane says %q while the field is empty", got)
	}
	typeEach(tt, "Plan")
	tt.Frame()
	if pn.title != "Plan" || p.Title != "Plan" {
		t.Fatalf("typed: field %q, page %q", pn.title, p.Title)
	}

	// another, left with nothing typed: ↵ goes into the text, and the page and its field are "Untitled"
	a.newPage(c, nil)
	tt.Frame() // made from outside a frame: a window at rest draws none of its own
	tt.Advance(300 * time.Millisecond)
	pn = a.pane()
	q := pn.page
	if q == p || pn.title != "" {
		t.Fatalf("second new page: field %q", pn.title)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if pn.titleFocused || pn.blank || pn.title != locales.CommonUntitled() || !untitled(q.Title) {
		t.Fatalf("left blank: focused %v, blank %v, field %q, title %q", pn.titleFocused, pn.blank, pn.title, q.Title)
	}

	// typed and then cleared: the page is "Untitled" again
	a.newPage(c, nil)
	tt.Frame() // made from outside a frame: a window at rest draws none of its own
	tt.Advance(300 * time.Millisecond)
	pn = a.pane()
	typeEach(tt, "X")
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if !untitled(pn.page.Title) {
		t.Fatalf("cleared: title %q", pn.page.Title)
	}

	// a rename keeps the title in the field
	a.open(p)
	a.pane().fresh = true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if a.pane().title != "Plan" || a.pane().blank {
		t.Fatalf("rename: field %q, blank %v", a.pane().title, a.pane().blank)
	}
}
