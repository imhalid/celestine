package library

import "testing"

// A page's links are written to its new title: any case, an alias kept, an embed too; other links and text stay.
func TestRenameLinks(t *testing.T) {
	md := "See [[Dune]], [[ dune |the book]] and ![[DUNE]].\n[[Dune Messiah]] and [[Arrakis|Dune]] are others; Dune alone is text.\n"
	got, n := RenameLinks(md, "Dune", "Dune (1965)")
	want := "See [[Dune (1965)]], [[Dune (1965)|the book]] and ![[Dune (1965)]].\n[[Dune Messiah]] and [[Arrakis|Dune]] are others; Dune alone is text.\n"
	if n != 3 || got != want {
		t.Fatalf("%d links:\n%s\nwant:\n%s", n, got, want)
	}
	if same, n := RenameLinks("no links here", "Dune", "X"); n != 0 || same != "no links here" {
		t.Fatalf("%d, %q", n, same)
	}
}

// A title with brackets, a bar or a line break cannot be linked to.
func TestLinkableTitle(t *testing.T) {
	for title, want := range map[string]bool{"Dune": true, "A [draft]": false, "a|b": false, "two\nlines": false, " ": false} {
		if got := LinkableTitle(title); got != want {
			t.Errorf("%q: %v", title, got)
		}
	}
}
