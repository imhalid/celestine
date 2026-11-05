package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSplitTitle(t *testing.T) {
	for _, c := range []struct{ md, title, rest, head string }{
		{"# Sayfa\n\nMetin\n", "Sayfa", "Metin\n", "# Sayfa\n\n"},
		{"\n# sayfa  \nMetin\n", "Sayfa", "Metin\n", "\n# sayfa  \n"},
		{"# Başka\n\nMetin\n", "Sayfa", "# Başka\n\nMetin\n", ""},
		{"Metin\n# Sayfa\n", "Sayfa", "Metin\n# Sayfa\n", ""},
		{"## Sayfa\n", "Sayfa", "## Sayfa\n", ""},
		{"", "Sayfa", "", ""},
	} {
		rest, head := splitTitle(c.md, c.title)
		if rest != c.rest || head != c.head {
			t.Errorf("%q: rest %q head %q", c.md, rest, head)
		}
	}
}

// The heading that is the page's title is not shown in the text, and is
// written back as it was.
func TestTitleHeadingHidden(t *testing.T) {
	a, tt := newTestApp(t, "# Sayfa\n\nMetin\n")
	tt.Frame()
	if got := mdOf(a.doc); got != "Metin" {
		t.Fatalf("shown: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "x")
	tt.Advance(1500 * time.Millisecond)
	tt.Frame()
	a.flush()
	data, err := os.ReadFile(filepath.Join(a.page.Dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Sayfa\n\nMetinx") {
		t.Fatalf("on disk: %q", data)
	}
}

// "[[Page]]" typed becomes a chip as the last bracket is typed.
func TestTypedLinkBecomesChip(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, " [[Alfa]]")
	tt.Frame()
	tt.Frame()
	if got := mdOf(a.doc); got != "Önce [[Alfa]]" {
		t.Fatalf("written: %q", got)
	}
	n := 0
	for _, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.target == "Alfa" {
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("chips: %d", n)
	}
	typeEach(tt, "x")
	if got := mdOf(a.doc); got != "Önce [[Alfa]]x" {
		t.Fatalf("after: %q", got)
	}
}

// A property is moved by its grip; the row open is the one outlined.
func TestMoveItem(t *testing.T) {
	l := []string{"a", "b", "c", "d"}
	if got := moveItem(append([]string{}, l...), 0, 3); strings.Join(got, "") != "bcad" {
		t.Fatalf("down: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 3, 1); strings.Join(got, "") != "adbc" {
		t.Fatalf("up: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 1, 2); strings.Join(got, "") != "abcd" {
		t.Fatalf("same place: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 0, 4); strings.Join(got, "") != "bcda" {
		t.Fatalf("to the end: %v", got)
	}
}
