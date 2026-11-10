package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A library of many pages, to measure what a big one costs: how long it takes to open, to show the
// first frame, to find a word, and the memory it holds. PERF_PAGES sets the count (10000 for the full
// measure); by default a smaller library is made so the test stays quick, with budgets scaled to it.

func perfPages() int {
	if s := os.Getenv("PERF_PAGES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 1500
}

func bigLibrary(tb testing.TB, n int) (*Store, *Library) {
	st, err := OpenStore(filepath.Join(tb.TempDir(), "big.db"))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { st.Close() })
	st.DB.Exec(`PRAGMA synchronous = OFF`)
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		tb.Fatal(err)
	}
	var colls []*Collection
	for i := 0; i < 5; i++ {
		c, err := lib.NewCollection("Koleksiyon " + strconv.Itoa(i))
		if err != nil {
			tb.Fatal(err)
		}
		colls = append(colls, c)
	}
	for i := 0; i < n; i++ {
		c := colls[i%len(colls)]
		p, err := c.NewPage(nil, "Sayfa "+strconv.Itoa(i))
		if err != nil {
			tb.Fatal(err)
		}
		body := fmt.Sprintf("Sayfa %d için metin. Bir iki üç dört beş altı yedi sekiz dokuz on.\n\n[[Sayfa %d]] #etiket%d\n\n- [] görev %d\n", i, (i+1)%n, i%40, i)
		if i%250 == 0 {
			body += "nadirkelime\n"
		}
		if err := p.SaveText(body); err != nil {
			tb.Fatal(err)
		}
	}
	return st, lib
}

func heapMB() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc) / (1 << 20)
}

// TestPerfBudget measures the costs of a big library and holds them to a budget.
func TestPerfBudget(t *testing.T) {
	n := perfPages()
	scale := float64(n) / 1000
	st, _ := bigLibrary(t, n)
	base := heapMB()

	t0 := time.Now()
	lib, err := OpenLibraryIn(st, StoreRoot)
	open := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Frame()
	first := time.Since(t0) - open
	mem := heapMB() - base

	t0 = time.Now()
	hits := a.searchPages("nadirkelime", 10, false)
	find := time.Since(t0)
	if len(hits) == 0 {
		t.Fatal("search found nothing")
	}
	t0 = time.Now()
	a.linksStale()
	a.linksTo(a.page, a.clock(), true)
	index := time.Since(t0)
	t0 = time.Now()
	items := a.paletteItems("sayfa 12")
	pal := time.Since(t0)
	if len(items) == 0 {
		t.Fatal("palette found nothing")
	}
	t0 = time.Now()
	ls := a.libraryStats()
	health := time.Since(t0)
	// a key in the text, and the frames it makes: the same keys in a library of ten pages and in the big one, by turns,
	// both warmed up first; what is held is that a key costs about the same in any library (#203, issue #70)
	sa, stt := keyTester(t, 10)
	warm(a, tt)
	var bigKeys, smallKeys []time.Duration
	for i := 0; i < 21; i++ {
		bigKeys = append(bigKeys, timeKey(tt))
		smallKeys = append(smallKeys, timeKey(stt))
	}
	_ = sa
	key, smallKey := median(bigKeys), median(smallKeys)
	t.Logf("%d pages: open %v, first frame %v, search %v, index %v, palette %v, library figures %v, a key %v, memory +%.0f MB",
		n, open.Round(time.Millisecond), first.Round(time.Millisecond), find.Round(time.Millisecond), index.Round(time.Millisecond),
		pal.Round(time.Millisecond), health.Round(time.Millisecond), key.Round(100*time.Microsecond), mem)
	t.Logf("a key in a library of 10 pages: %v", smallKey.Round(100*time.Microsecond))
	_ = ls

	// budgets, per thousand pages; generous, so that a slow machine does not fail them and a real
	// regression - a loop that went quadratic - does
	check := func(name string, got time.Duration, perK time.Duration) {
		if limit := time.Duration(float64(perK) * max(scale, 1)); got > limit {
			t.Errorf("%s took %v: over the budget of %v", name, got, limit)
		}
	}
	check("open the library", open, 1500*time.Millisecond)
	check("the first frame", first, 3*time.Second)
	check("a search", find, 2*time.Second)
	check("the link index", index, 2*time.Second)
	check("a palette query", pal, 600*time.Millisecond)
	check("the library figures", health, 4*time.Second)
	if key > smallKey*3/2+5*time.Millisecond { // a ratio: a slower machine is slower at both
		t.Errorf("a key in the big library took %v, in a small one %v: over one and a half times", key, smallKey)
	}
	if os.Getenv("PERF_STRICT") == "1" && key > 60*time.Millisecond { // the absolute budget, on a machine known (in the headless, software painter)
		t.Errorf("a key in the text took %v: over the budget of 60ms", key)
	}
	if limit := 200 * max(scale, 1); mem > limit {
		t.Errorf("the library holds %.0f MB: over the budget of %.0f MB", mem, limit)
	}
}

func BenchmarkSearchBigLibrary(b *testing.B) {
	_, lib := bigLibrary(b, perfPages())
	a := newApp(lib)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.searchPages("nadirkelime", 10, false)
	}
}

func BenchmarkPaletteBigLibrary(b *testing.B) {
	_, lib := bigLibrary(b, perfPages())
	a := newApp(lib)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.paletteItems("sayfa 12")
	}
}

// BenchmarkKeyBigLibrary is a key in the text of a big library, and its frame.
func BenchmarkKeyBigLibrary(b *testing.B) {
	st, _ := bigLibrary(b, perfPages())
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		b.Fatal(err)
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 800)
	warm(a, tt) // the indexes are read in, a little a frame: not in the keys timed
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Type("x")
		tt.Frame()
	}
}

// keyTester is an app with a library of n pages, warmed up, for keys to be timed in.
func keyTester(t *testing.T, n int) (*app, *ui.Tester) {
	st, _ := bigLibrary(t, n)
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 800)
	warm(a, tt)
	return a, tt
}

// warm runs frames until the search index has read every page in, as it does a little a frame: the keys timed after
// carry none of it.
func warm(a *app, tt *ui.Tester) {
	for i := 0; i < 2000 && !(a.sx != nil && a.sx.ready); i++ {
		tt.Advance(20 * time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		tt.Advance(50 * time.Millisecond)
	}
}

// timeKey is how long a key and the frames it makes take.
func timeKey(tt *ui.Tester) time.Duration {
	t0 := time.Now()
	tt.Type("x")
	tt.Frame()
	return time.Since(t0)
}

// median is the middle of ds: one sample a busy machine slowed does not move it.
func median(ds []time.Duration) time.Duration {
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[len(s)/2]
}
