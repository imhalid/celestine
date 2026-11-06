package main

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// A page's made cover is its own: the same page always gets the same one, other pages
// get others, and every way the tiles lie turns up.
func TestAutoCoverSeeds(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("K")
	styles := map[int]bool{}
	themes := map[int]bool{}
	seen := map[uint64]bool{}
	for i := 0; i < 60; i++ {
		p, _ := c.NewPage(nil, "Sayfa")
		p.Created = p.Created.Add(time.Duration(i) * 1000)
		s := coverSeed(p)
		if seen[s] {
			t.Fatalf("two pages have seed %v", s)
		}
		seen[s] = true
		a, b := newAutoCover(s, false, nil), newAutoCover(s, false, nil)
		if a.style != b.style || a.hue != b.hue || a.theme != b.theme {
			t.Fatal("a seed does not give the same cover")
		}
		if d := newAutoCover(s, true, nil); d.style != a.style || d.hue != a.hue || d.theme != a.theme {
			t.Fatal("the light and the dark cover of a page are not the same pattern")
		}
		styles[a.style] = true
		themes[a.theme] = true
	}
	if len(styles) != mosaicKinds {
		t.Fatalf("%d of %d styles in 60 pages: %v", len(styles), mosaicKinds, styles)
	}
	if len(themes) != len(coverThemes) {
		t.Fatalf("%d of %d themes in 60 pages", len(themes), len(coverThemes))
	}
	p, _ := c.NewPage(nil, "Ad")
	was := coverSeed(p)
	p.Title = "Başka bir ad"
	if coverSeed(p) != was {
		t.Fatal("a rename changed the page's cover")
	}
}

// Every style is drawn, light and dark, at any width - past the picture's too.
func TestAutoCoverDraws(t *testing.T) {
	defer func() { styleOf = -1 }()
	for style := 0; style < mosaicKinds; style++ {
		styleOf = style
		a, tt := newTestApp(t, "metin\n")
		a.autoCovers = nil
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			for _, w := range []int{900, 420, 130, 2600} {
				tt.SetSize(w, 500)
				tt.Frame()
			}
		}
	}
}

// A page with no cover of its own is covered by the made one; "remove the cover" makes
// the page say none (and the file is as the page's own, read again); the menu can bring
// it back; the setting turns the made ones off.
func TestAutoCoverKinds(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	p := a.page
	if a.coverKind(p) != coverKindAuto || a.coverBandOf(p) != autoFullH {
		t.Fatalf("a page with no cover: kind %d, band %v", a.coverKind(p), a.coverBandOf(p))
	}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if a.topSpace != 0 { // a cover begins at the very top of the window, behind the bar
		t.Fatalf("the page does not start at the top for its cover: %v", a.topSpace)
	}
	a.removeCover(p)
	src, _ := os.ReadFile(p.file())
	if p.Cover != coverNone || !strings.Contains(string(src), "cover: none") || a.coverKind(p) != coverKindNone {
		t.Fatalf("removed: %q, kind %d\n%s", p.Cover, a.coverKind(p), src)
	}
	again, _ := OpenLibrary(a.lib.Root)
	if got := again.Collections[0].Pages[0]; got.Cover != coverNone || a.coverKind(got) != coverKindNone {
		t.Fatalf("read again: %q", got.Cover)
	}
	var titles []string
	for _, it := range a.pageMoreMenu(p) {
		titles = append(titles, it.title)
	}
	if !contains(titles, "Otomatik kapağı geri getir") || contains(titles, "Kapağı kaldır") {
		t.Fatalf("menu of a page with none: %v", titles)
	}
	a.restoreAutoCover(p)
	src, _ = os.ReadFile(p.file())
	if p.Cover != "" || strings.Contains(string(src), "cover") || a.coverKind(p) != coverKindAuto {
		t.Fatalf("brought back: %q\n%s", p.Cover, src)
	}
	// a picture of its own is the cover, and the setting off leaves the others bare
	if err := a.setCover(p, pngBytes(300, 100)); err != nil {
		t.Fatal(err)
	}
	if a.coverKind(p) != coverKindImage || !p.imageCover() {
		t.Fatalf("a picture: kind %d", a.coverKind(p))
	}
	a.removeCover(p)
	a.restoreAutoCover(p)
	a.settings.NoAutoCover = true
	if a.coverKind(p) != coverKindNone {
		t.Fatal("the setting off still gives a made cover")
	}
}

// The seed picks a page's theme, its two hues moved a little by the seed; the picks differ
// from page to page and cover the themes.
func TestAutoCoverThemes(t *testing.T) {
	picked := map[int]bool{}
	for seed := uint64(1); seed < 80; seed++ {
		ac := newAutoCover(seed*104729, true, nil)
		picked[ac.theme] = true
		if d := math.Abs(ac.hue - coverThemes[ac.theme].h1); d > 9.01 {
			t.Fatalf("%s: a page's hue is %v from the theme's", coverThemes[ac.theme].name, d)
		}
	}
	if len(picked) < len(coverThemes)-1 {
		t.Fatalf("the seed picks %d of %d themes in 80 pages", len(picked), len(coverThemes))
	}
}
