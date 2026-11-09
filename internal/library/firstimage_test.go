package library

import "testing"

// FirstImage is the first picture a page's text shows, not an embedded page, and it follows the page as it changes.
func TestFirstImage(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Notes")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Trip")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ md, want string }{
		{"Nothing to see.\n", ""},
		{"![[Other page]]\n\nA line.\n", ""},
		{"Words.\n\n![the sea](resim-1.png)\n\n![later](resim-2.png)\n", "resim-1.png"},
		{"![[Other page]]\n![a hill](<my pictures/hill.jpg>)\n", "my pictures/hill.jpg"},
		{"![out](https://example.com/a.png \"title\")\n", "https://example.com/a.png"},
	} {
		if err := p.SaveText(tc.md); err != nil {
			t.Fatal(err)
		}
		if got := p.FirstImage(); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.md, got, tc.want)
		}
	}
}
