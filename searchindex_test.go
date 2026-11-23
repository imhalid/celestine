package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// match as it was before it was taken apart for speed: the folded form must score what it scored.
func matchOld(q, s string) int {
	q, s = foldText(strings.TrimSpace(q)), foldText(s)
	if q == "" {
		return 0
	}
	score := 0
	all := true
	for _, w := range strings.Fields(q) {
		i := strings.Index(s, w)
		if i < 0 {
			all = false
			break
		}
		score += 100 - min(i, 60)
		if i == 0 || s[i-1] == ' ' || s[i-1] == '-' || s[i-1] == ':' {
			score += 40
		}
	}
	if all {
		if strings.HasPrefix(s, q) {
			score += 80
		}
		return score + 200
	}
	j, gaps, last := 0, 0, -1
	qr := []rune(strings.ReplaceAll(q, " ", ""))
	for i, r := range []rune(s) {
		if j < len(qr) && r == qr[j] {
			if last >= 0 {
				gaps += i - last - 1
			}
			last = i
			j++
		}
	}
	if j < len(qr) {
		return -1
	}
	return max(1, 100-gaps*4)
}

func TestMatchFoldedParity(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	alpha := []rune("abcçdefgğhıijklmnoöprsştuüvyzABCİÖŞ 12-:")
	word := func(n int) string {
		r := make([]rune, n)
		for i := range r {
			r[i] = alpha[rng.Intn(len(alpha))]
		}
		return string(r)
	}
	for i := 0; i < 4000; i++ {
		q, s := word(1+rng.Intn(5)), word(rng.Intn(30))
		if got, want := match(q, s), matchOld(q, s); got != want {
			t.Fatalf("match(%q, %q) = %d, was %d", q, s, got, want)
		}
		// the mask never throws out what matches
		if matchOld(q, s) >= 0 && letterMask(foldText(strings.TrimSpace(q)))&^letterMask(foldText(s)) != 0 {
			t.Fatalf("the mask drops %q in %q", q, s)
		}
	}
}

func indexApp(t testing.TB, pages int) *app {
	t.Helper()
	_, lib := bigLibrary(t, pages)
	a := newApp(lib)
	for !a.sx.warm(a, time.Second) {
	}
	return a
}

// The index finds what the scan finds, in the same order with the same scores, and follows the pages
// that are written, made and gone.
func TestIndexAgreesWithScan(t *testing.T) {
	a := indexApp(t, 400)
	queries := []string{"nadirkelime", "sayfa 12", "metin", "ıö", "bir iki üç", `"iki üç"`, "başlık:sayfa", "koleksiyon:koleksiyon sayfa", "#etiket3 sayfa", "yok yok yok", "s", "sayfa 399",
		"nadirkelime OR sayfa 12", "metin -nadirkelime", "(sayfa 1 VEYA sayfa 2) NOT nadirkelime", "@todo sayfa 3", "-#etiket3 iki"}
	compare := func(label string) {
		t.Helper()
		for _, q := range queries {
			a.sx.ready = true
			got := a.searchPages(q, 0, false)
			a.sx.ready = false
			want := a.searchPages(q, 0, false)
			a.sx.ready = true
			if len(got) != len(want) {
				t.Fatalf("%s: %q: %d hits, the scan %d", label, q, len(got), len(want))
			}
			for i := range got {
				if got[i].page != want[i].page || got[i].score != want[i].score {
					t.Fatalf("%s: %q: hit %d is %s (%d), the scan's %s (%d)", label, q, i, got[i].page.Title, got[i].score, want[i].page.Title, want[i].score)
				}
			}
		}
	}
	compare("as made")
	// a page written
	p := a.lib.Collections[0].Pages[0]
	if err := p.SaveText("ayrıca benzersizsözcük ve nadirkelime\n"); err != nil {
		t.Fatal(err)
	}
	if hits := a.searchPages("benzersizsözcük", 10, false); len(hits) != 1 || hits[0].page != p {
		t.Fatalf("a written page: %d hits", len(hits))
	}
	compare("written")
	// a page made, and one gone
	np, _ := a.lib.Collections[1].NewPage(nil, "Yepyeni sayfa")
	np.SaveText("tazesözcük\n")
	if hits := a.searchPages("tazesözcük", 10, false); len(hits) != 1 {
		t.Fatalf("a new page: %d hits", len(hits))
	}
	if _, err := a.lib.TrashPage(np); err != nil {
		t.Fatal(err)
	}
	if hits := a.searchPages("tazesözcük", 10, false); len(hits) != 0 {
		t.Fatalf("a page in the trash is found: %d hits", len(hits))
	}
	compare("made and gone")
}

// A keystroke of the palette, in a library of pages, is a millisecond or so: the names by their letters,
// the texts by the index. PERF_PAGES=10000 for the big one.
func TestIndexedKeystrokes(t *testing.T) {
	n := perfPages()
	a := indexApp(t, n)
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Frame()
	queries := []string{"s", "sa", "say", "sayf", "sayfa", "sayfa 1", "sayfa 12", "nad", "nadirk", "nadirkelime", "bir iki", "üç dört beş"}
	a.paletteItems("sayfa") // the places are made the first time
	var worst time.Duration
	var total time.Duration
	runs := 0
	for rep := 0; rep < 5; rep++ {
		for _, q := range queries {
			t0 := time.Now()
			items := a.paletteItems(q)
			d := time.Since(t0)
			if rep > 0 {
				total += d
				runs++
				worst = max(worst, d)
			}
			if len(items) == 0 {
				t.Fatalf("%q found nothing", q)
			}
		}
	}
	avg := total / time.Duration(runs)
	t.Logf("%d pages: a keystroke %v on average, %v at worst", n, avg, worst)
	scale := max(float64(n)/1000, 1)
	if limit := time.Duration(float64(3*time.Millisecond) * scale); avg > limit {
		t.Errorf("a keystroke took %v on average: over %v", avg, limit)
	}
	_ = fmt.Sprint
}
