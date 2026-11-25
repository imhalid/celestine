package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestHeadingLinks: [[Page#Heading]] is a link to the page, said "Page › Heading", which a click opens at that
// heading; the [[ menu offers a page's headings after a "#"; the page counts it among its backlinks; a rename keeps
// the heading; and a title with a "#" of its own is the page's (#549).
func TestHeadingLinks(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 800)
	c := a.pane().page.Coll
	dune, _ := c.NewPage(nil, "Dune")
	dune.SaveText("intro\n\n## Plot\n\nstory\n\n## Ecology\n\nsand\n")
	cs, _ := c.NewPage(nil, "C#")
	notes, _ := c.NewPage(nil, "Notes")
	notes.SaveText("see [[Dune#Ecology]] and [[C#]]\n")
	a.linksStale()
	a.links.built = time.Time{}
	if a.pageTitled("Dune#Ecology") != dune || a.pageTitled("C#") != cs || a.pageTitled("Dune#Nowhere") != dune {
		t.Fatal("not resolved to the page")
	}
	if p, h := splitHeading("C#"); p != "C#" || h != "" {
		t.Fatalf("C#: %q %q", p, h)
	}
	if l := (&chip{kind: "page", target: "Dune#Ecology"}).label(); l != locales.LinksPageHeading("Dune", "Ecology") {
		t.Fatalf("the chip says %q", l)
	}
	if got := a.linksTo(dune, a.clock(), true); len(got) != 1 || got[0] != notes {
		t.Fatalf("the backlinks: %v", got)
	}
	// the [[ menu, after a "#"
	items := a.wikiItems("Dune#eco")
	if len(items) == 0 || items[0].title != locales.LinksPageHeading("Dune", "Ecology") {
		t.Fatalf("the [[ menu: %+v", items)
	}
	// a click on the chip opens the page at its heading
	a.open(notes)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	var hit *chipHit
	for i := range a.chipSeen {
		if a.chipSeen[i].ch.target == "Dune#Ecology" {
			hit = &a.chipSeen[i]
		}
	}
	if hit == nil {
		t.Fatal("the chip is not drawn")
	}
	tt.ClickAt(hit.r.X+hit.r.W/2, hit.r.Y+hit.r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	_, caret := a.pane().doc.Selection()
	if a.pane().page != dune {
		t.Fatalf("opened %v", a.pane().page)
	}
	if ps, pe := a.pane().doc.Text().ParagraphRange(caret); a.pane().doc.Text().Slice(ps, pe).String() != "Ecology" {
		t.Fatalf("the caret is in %q", a.pane().doc.Text().Slice(ps, pe).String())
	}
	// a rename keeps the heading
	body, _ := notes.Body()
	if nb, n := relinkBody(body, map[string]string{"dune": "Dune (novel)"}); n != 1 || !strings.Contains(nb, `Dune (novel)#Ecology`) {
		t.Fatalf("relinked %d: %s", n, nb)
	}
}
