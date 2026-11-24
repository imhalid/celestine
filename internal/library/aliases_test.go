package library

import (
	"slices"
	"testing"
)

// TestAliases: a page's aliases are the values of its properties of the aliases kind, its collection's and its own,
// each once whatever its case; they are kept as a list, and a view's filter finds one whole (#547).
func TestAliases(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("People")
	also, err := c.AddProperty("Also", PropAliases, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := c.NewPage(nil, "Ada Lovelace")
	gen := PagesGen.Load()
	if err := p.SetValue(also.ID, " Ada ,  Countess,ada "); err != nil {
		t.Fatal(err)
	}
	if PagesGen.Load() == gen {
		t.Fatal("an alias set is not a change of what links resolve to")
	}
	if v := p.Props[also.ID]; v != "Ada, Countess" {
		t.Fatalf("kept as %q", v)
	}
	p.Own = append(p.Own, Property{ID: "nick", Name: "Nick", Type: PropAliases})
	p.Props["nick"] = "AL, countess"
	if got := p.Aliases(); !slices.Equal(got, []string{"Ada", "Countess", "AL"}) {
		t.Fatalf("aliases %v", got)
	}
	f := Filter{Prop: also.ID, Op: "contains", Value: "countess"}
	if !f.match(c, p) || (Filter{Prop: also.ID, Op: "contains", Value: "Count"}).match(c, p) {
		t.Fatal("a filter on an alias finds it whole")
	}
	if !IsList(PropAliases) || !IsList(PropTags) || IsList(PropText) {
		t.Fatal("IsList")
	}
}
