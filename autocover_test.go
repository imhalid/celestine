package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// A page's made cover is its own: the same page always gets the same one, other pages
// get others, and every field and every dither turns up.
func TestAutoCoverSeeds(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("K")
	fields, dithers := map[int]bool{}, map[int]bool{}
	seen := map[uint64]bool{}
	for i := 0; i < 60; i++ {
		p, _ := c.NewPage(nil, "Sayfa")
		p.Created = p.Created.Add(time.Duration(i) * 1000)
		s := coverSeed(p)
		if seen[s] {
			t.Fatalf("two pages have seed %v", s)
		}
		seen[s] = true
		a, b := newAutoCover(s, 46), newAutoCover(s, 46)
		if a.field != b.field || a.dither != b.dither || a.hue != b.hue {
			t.Fatal("a seed does not give the same cover")
		}
		fields[a.field], dithers[a.dither] = true, true
	}
	if len(fields) != fieldKinds || len(dithers) != ditherKinds {
		t.Fatalf("fields %d of %d, dithers %d of %d in 60 pages", len(fields), fieldKinds, len(dithers), ditherKinds)
	}
	p, _ := c.NewPage(nil, "Ad")
	was := coverSeed(p)
	p.Title = "Başka bir ad"
	if coverSeed(p) != was {
		t.Fatal("a rename changed the page's cover")
	}
}

// Every field, in every dither, is drawn at any width.
func TestAutoCoverDraws(t *testing.T) {
	for f := 0; f < fieldKinds; f++ {
		for d := 0; d < ditherKinds; d++ {
			a, tt := newTestApp(t, "metin\n")
			a.autoCovers = map[uint64]*autoCover{coverSeed(a.page): newAutoCoverOf(uint64(f*131+d*17+5), f, d)}
			for _, w := range []int{900, 420, 130} {
				tt.SetSize(w, 500)
				tt.Frame()
			}
		}
	}
}

// newAutoCoverOf is a cover of the seed in the given field and dither.
func newAutoCoverOf(seed uint64, field, dither int) *autoCover {
	fieldOf, ditherOf = field, dither
	defer func() { fieldOf, ditherOf = -1, -1 }()
	return newAutoCover(seed, 46)
}

// A page with no cover of its own is covered by the made one; "remove the cover" makes
// the page say none (and the file is as the page's own, read again); the menu can bring
// it back; the setting turns the made ones off.
func TestAutoCoverKinds(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	p := a.page
	if a.coverKind(p) != coverKindAuto || a.coverBandOf(p) != autoBandH {
		t.Fatalf("a page with no cover: kind %d, band %v", a.coverKind(p), a.coverBandOf(p))
	}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if a.topSpace != barH { // a cover starts under the bar
		t.Fatalf("the page does not start under the bar for its cover: %v", a.topSpace)
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
