package library

import (
	"slices"
	"testing"
	"time"
)

// A collection's own order: pages not yet placed come after the placed ones, by when they were made; moving some of
// them (as a filtered list shows them) moves only those, in the places they had; it is kept in the pages' files and
// read back, and does not touch a board's ranks.
func TestManualOrder(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Notes")
	base := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)
	byName := map[string]*Page{}
	for i, title := range []string{"A", "B", "C", "D"} {
		p, err := c.NewPage(nil, title)
		if err != nil {
			t.Fatal(err)
		}
		p.Created = base.Add(time.Duration(i) * time.Hour)
		p.Rank = 7
		byName[title] = p
	}
	order := func(c *Collection) []string {
		all := c.All()
		slices.SortStableFunc(all, CompareManual)
		var out []string
		for _, p := range all {
			out = append(out, p.Title)
		}
		return out
	}
	if got := order(c); !slices.Equal(got, []string{"A", "B", "C", "D"}) {
		t.Fatalf("not yet placed, by when made: %v", got)
	}
	if err := c.SetManual([]*Page{byName["D"], byName["A"], byName["B"], byName["C"]}); err != nil {
		t.Fatal(err)
	}
	if got := order(c); !slices.Equal(got, []string{"D", "A", "B", "C"}) {
		t.Fatalf("moved: %v", got)
	}
	// a list showing only B and C swaps them; A and D keep their places
	if err := c.SetManual([]*Page{byName["C"], byName["B"]}); err != nil {
		t.Fatal(err)
	}
	if got := order(c); !slices.Equal(got, []string{"D", "A", "C", "B"}) {
		t.Fatalf("a part moved: %v", got)
	}
	again, _ := OpenLibrary(lib.Root)
	if got := order(again.Collections[0]); !slices.Equal(got, []string{"D", "A", "C", "B"}) {
		t.Fatalf("read again: %v", got)
	}
	for _, p := range again.Collections[0].All() {
		if p.Rank != 7 || p.Props["manualrank"] != "" {
			t.Fatalf("%s: rank %d, props %v", p.Title, p.Rank, p.Props)
		}
	}
}
