package appicon

import (
	"encoding/xml"
	"strings"
	"testing"
)

// Every look is drawn by the one drawing: each SVG is well formed, has the same shapes, and differs only in color.
func TestLooksShareTheDrawing(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range Looks {
		if seen[l.ID] {
			t.Fatalf("look %q twice", l.ID)
		}
		seen[l.ID] = true
		for _, filters := range []bool{true, false} {
			s := SVG(l, PartAll, filters)
			if err := xml.Unmarshal([]byte(s), new(struct{})); err != nil {
				t.Fatalf("%s: not well formed: %v", l.ID, err)
			}
			if got := strings.Count(s, `rx="105"`); got != 2 {
				t.Fatalf("%s: %d rims", l.ID, got)
			}
			if got := strings.Count(s, `fill="url(#fill)"`); got != 2 {
				t.Fatalf("%s: %d carets", l.ID, got)
			}
			if strings.Contains(s, "<filter") != filters {
				t.Fatalf("%s: filters %v in %s", l.ID, filters, s)
			}
		}
	}
	for _, id := range []string{"paper", "dark", "blue"} { // the icon and the brand's two (#521)
		if !seen[id] {
			t.Fatalf("no %s look", id)
		}
	}
	if len(Looks) < 6 {
		t.Fatalf("%d looks", len(Looks))
	}
	if Looks[0].ID != "paper" {
		t.Fatal("Paper is not the first look")
	}
}

// Auto follows the window: the dark look in a dark one, Paper in a light one; a look chosen is that look whatever
// the window, and a mode there is no look of is Paper.
func TestPick(t *testing.T) {
	for _, c := range []struct {
		mode string
		dark bool
		want string
	}{{"auto", true, "dark"}, {"auto", false, "paper"}, {"", true, "paper"}, {"blue", true, "blue"}, {"blue", false, "blue"},
		{"violet", false, "violet"}, {"nonesuch", true, "paper"}} {
		if got := Pick(c.mode, c.dark).ID; got != c.want {
			t.Errorf("Pick(%q, dark %v) = %q, want %q", c.mode, c.dark, got, c.want)
		}
	}
}

// Without filters, the parts are drawn alone: a renderer that blurs them itself asks for each.
func TestParts(t *testing.T) {
	l, _ := ByID("dark")
	if s := SVG(l, PartGround, false); strings.Contains(s, "rx=") || !strings.Contains(s, "url(#ground)") {
		t.Fatalf("the ground alone: %s", s)
	}
	if s := SVG(l, PartGlow, false); strings.Count(s, "rx=\"65\"") != 2 || strings.Contains(s, "url(#fill)") {
		t.Fatalf("the glow alone: %s", s)
	}
	p, _ := ByID("paper")
	if s := SVG(p, PartGlow, false); strings.Contains(s, "rx=") {
		t.Fatal("Paper has a glow")
	}
}
