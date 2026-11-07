package main

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// A page's made cover is its own: the same page always gets the same one, other pages
// get others, and every landscape and every hour turns up.
func TestAutoCoverSeeds(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("K")
	scenes := map[int]bool{}
	days, nights := map[int]bool{}, map[int]bool{}
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
		if a.scene != b.scene || a.light != b.light || len(a.clouds) != len(b.clouds) {
			t.Fatal("a seed does not give the same cover")
		}
		d := newAutoCover(s, true, nil)
		if d.scene != a.scene {
			t.Fatal("the light and the dark cover of a page are not the same landscape")
		}
		if d.noSky && len(d.stars) > 0 || !a.ownSky && len(a.stars) != 0 { // from above, no sky; by day, none but a sky of its own
			t.Fatalf("stars: %d at night, %d by day", len(d.stars), len(a.stars))
		}
		days[a.light], nights[d.light] = true, true
	}
	for seed := uint64(1); seed < 3000; seed++ {
		scenes[sceneOfSeed(seed*0x9e3779b97f4a7c15)] = true
	}
	if len(scenes) != sceneKinds {
		t.Fatalf("%d of %d landscapes in 3000 pages: %v", len(scenes), sceneKinds, scenes)
	}
	if len(days) != len(dayLights) || len(nights) != len(nightLights) {
		t.Fatalf("hours: %d of %d by day, %d of %d at night", len(days), len(dayLights), len(nights), len(nightLights))
	}
	p, _ := c.NewPage(nil, "Ad")
	was := coverSeed(p)
	p.Title = "Başka bir ad"
	if coverSeed(p) != was {
		t.Fatal("a rename changed the page's cover")
	}
}

// Every landscape is drawn, light and dark, in every season, at any width - past the
// picture's too - and what moves in it goes on, at a few frames a second.
func TestAutoCoverDraws(t *testing.T) {
	defer func() { sceneOf, coverNow = -1, time.Now }()
	a, tt := newTestApp(t, "metin\n")
	for scene := 0; scene < sceneKinds; scene++ {
		sceneOf = scene
		month := time.Month(1 + scene*3%12)
		coverNow = func() time.Time { return time.Date(2026, month, 10, 12, 0, 0, 0, time.UTC) }
		for i, dark := range []bool{false, true} {
			a.autoCovers = nil
			tt.SetDark(dark)
			tt.SetSize([]int{900, 2600}[(i+scene)%2], 500) // narrower and wider than the picture
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
		}
	}
}

// The moon's phase is today's: new on a known new moon, full half a month on.
func TestMoonPhase(t *testing.T) {
	newMoon := time.Date(2024, 1, 11, 11, 57, 0, 0, time.UTC)
	if ph := moonPhase(newMoon); ph > 0.01 && ph < 0.99 {
		t.Fatalf("a new moon's phase: %v", ph)
	}
	if ph := moonPhase(newMoon.Add(354 * time.Hour)); math.Abs(ph-0.5) > 0.02 {
		t.Fatalf("a full moon's phase: %v", ph)
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
	if p.Cover != coverNone || !strings.Contains(string(src), `"cover":"none"`) || a.coverKind(p) != coverKindNone {
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
