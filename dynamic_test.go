package main

import (
	"testing"
	"time"
)

// TestDynamicCollection: a saved search shows the pages the library has for
// its query, from any collection, and takes none of its own.
func TestDynamicCollection(t *testing.T) {
	a, tt := newTestApp(t, "ilk #fikir\n")
	tt.SetSize(1100, 800)
	other, _ := a.lib.NewCollection("Öteki")
	x, _ := other.NewPage(nil, "İkinci")
	x.Save("burada da #fikir var\n")
	y, _ := other.NewPage(nil, "Üçüncü")
	y.Save("fikir değil\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	a.openSearch("#fikir")
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Koleksiyon olarak kaydet"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	c := a.coll
	if c == nil || !c.Dynamic() || c.Query != "#fikir" || c.SubPages {
		t.Fatalf("collection: %+v", c)
	}
	if got := a.filtered(c); len(got) != 2 {
		t.Fatalf("pages: %d", len(got))
	}
	if !tt.HasText("Sayfa: İkinci") && !tt.HasText("İkinci") {
		t.Fatalf("not shown: %v", tt.Texts())
	}
	if tt.HasText("Yeni sayfa") {
		t.Fatal("a button to make a page")
	}
	if _, err := c.NewPage(nil, "elle"); err == nil {
		t.Fatal("a page made in a saved search")
	}
	// the library has it back
	again, _ := OpenLibrary(a.lib.Root)
	found := false
	for _, k := range again.Collections {
		found = found || (k.Query == "#fikir" && k.Icon == "search")
	}
	if !found {
		t.Fatal("not on disk")
	}
	// what the library has now: a page tagged is in
	y.Save("şimdi #fikir\n")
	a.linksStale()
	a.links.built = time.Time{}
	if got := a.filtered(c); len(got) != 3 {
		t.Fatalf("after tagging: %d", len(got))
	}
}
