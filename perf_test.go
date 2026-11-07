package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	st.db.Exec(`PRAGMA synchronous = OFF`)
	lib, err := OpenLibraryIn(st, storeRoot)
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
	lib, err := OpenLibraryIn(st, storeRoot)
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

	t.Logf("%d pages: open %v, first frame %v, search %v, index %v, palette %v, library figures %v, memory +%.0f MB",
		n, open.Round(time.Millisecond), first.Round(time.Millisecond), find.Round(time.Millisecond), index.Round(time.Millisecond),
		pal.Round(time.Millisecond), health.Round(time.Millisecond), mem)
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
