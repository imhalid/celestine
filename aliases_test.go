package main

import (
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestPageAliases: a page's aliases are found by a [[link]] as its title is - the chip opens it, the page counts the
// link among its backlinks - the [[ menu offers them under the page they name, and the palette finds the page by
// one; a page titled so comes before one with it for an alias (#547).
func TestPageAliases(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 700)
	c := a.page.Coll
	also, err := c.AddProperty("Also", PropAliases, nil)
	if err != nil {
		t.Fatal(err)
	}
	ada, _ := c.NewPage(nil, "Ada Lovelace")
	if a.pageTitled("Countess") != nil {
		t.Fatal("found before it has the alias")
	}
	if err := ada.SetValue(also.ID, "Ada, Countess"); err != nil {
		t.Fatal(err)
	}
	if a.pageTitled("countess") != ada || a.pageTitled("Ada Lovelace") != ada {
		t.Fatal("not found by its alias")
	}
	notes, _ := c.NewPage(nil, "Notes")
	if err := notes.SaveText("see [[Countess]]\n"); err != nil {
		t.Fatal(err)
	}
	a.linksStale()
	a.links.built = time.Time{}
	if got := a.linksTo(ada, a.clock(), true); len(got) != 1 || got[0] != notes {
		t.Fatalf("backlinks of the page: %v", got)
	}
	// a page titled so comes first
	countess, _ := c.NewPage(nil, "Countess")
	if a.pageTitled("Countess") != countess {
		t.Fatal("the page titled so is not the one found")
	}
	countess.Title = "Elsewhere" // as a rename leaves it, for the menu below
	PagesGen.Add(1)
	// the [[ menu, under the page it names
	a.slash.start = 0
	found := false
	for _, it := range a.wikiItems("coun") {
		found = found || it.title == "Countess" && it.sub == locales.SlashAliasOf("Ada Lovelace")
	}
	if !found {
		t.Fatalf("the [[ menu has no alias: %v", a.wikiItems("coun"))
	}
	a.linksStale()
	for _, it := range a.placeMatches("countess", 10) {
		if it.to.kind == "page" && it.to.dir == ada.Dir {
			return
		}
	}
	t.Fatalf("the palette does not find the page by its alias: %v", a.placeMatches("countess", 10))
}
