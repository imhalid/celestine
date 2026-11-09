package library

import (
	"slices"
	"testing"
)

// A view's filters: each comparison on each type, all of them together, a filter on a property that is gone letting
// every page through, and Normalize dropping it.
func TestViewFilters(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Books")
	state, _ := c.AddProperty("State", PropChoice, []string{"To read", "Read"})
	pagesN, _ := c.AddProperty("Pages", PropNumber, nil)
	on, _ := c.AddProperty("On", PropDate, nil)
	author, _ := c.AddProperty("Author", PropText, nil)
	mk := func(title string, vals map[string]string) {
		p, err := c.NewPage(nil, title)
		if err != nil {
			t.Fatal(err)
		}
		for id, v := range vals {
			if err := p.SetValue(id, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("Dune", map[string]string{state.ID: "Read", pagesN.ID: "412", on.ID: "2026-03-01", author.ID: "Frank Herbert"})
	mk("Anna Karenina", map[string]string{state.ID: "To read", pagesN.ID: "864", author.ID: "Lev Tolstoy"})
	mk("Untitled", nil)
	shown := func(fs ...Filter) []string {
		t.Helper()
		v := View{Filters: fs}
		var out []string
		for _, p := range c.All() {
			if v.Match(c, p) {
				out = append(out, p.Title)
			}
		}
		slices.Sort(out)
		return out
	}
	for _, tc := range []struct {
		f    []Filter
		want []string
	}{
		{nil, []string{"Anna Karenina", "Dune", "Untitled"}},
		{[]Filter{{Prop: state.ID, Op: "is", Value: "read"}}, []string{"Dune"}},
		{[]Filter{{Prop: state.ID, Op: "isNot", Value: "Read"}}, []string{"Anna Karenina", "Untitled"}},
		{[]Filter{{Prop: state.ID, Op: "empty"}}, []string{"Untitled"}},
		{[]Filter{{Prop: pagesN.ID, Op: "gt", Value: "500"}}, []string{"Anna Karenina"}},
		{[]Filter{{Prop: pagesN.ID, Op: "lt", Value: "500"}}, []string{"Dune"}},
		{[]Filter{{Prop: on.ID, Op: "lt", Value: "2026-06-01"}}, []string{"Dune"}},
		{[]Filter{{Prop: author.ID, Op: "contains", Value: "tolst"}}, []string{"Anna Karenina"}},
		{[]Filter{{Prop: author.ID, Op: "notEmpty"}, {Prop: pagesN.ID, Op: "gt", Value: "500"}}, []string{"Anna Karenina"}},
		{[]Filter{{Prop: "title", Op: "contains", Value: "un"}}, []string{"Dune", "Untitled"}},
		{[]Filter{{Prop: "gone", Op: "is", Value: "x"}}, []string{"Anna Karenina", "Dune", "Untitled"}},
	} {
		if got := shown(tc.f...); !slices.Equal(got, tc.want) {
			t.Errorf("%+v: %v, want %v", tc.f, got, tc.want)
		}
	}
	c.Views[0].Filters = []Filter{{Prop: author.ID, Op: "notEmpty"}, {Prop: "title", Op: "contains", Value: "a"}}
	if err := c.RemoveProperty(author.ID); err != nil {
		t.Fatal(err)
	}
	if f := c.Views[0].Filters; len(f) != 1 || f[0].Prop != "title" {
		t.Fatalf("a filter on a removed property is kept: %+v", f)
	}
}
