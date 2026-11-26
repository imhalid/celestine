package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The pages read away from the window fill the links' index and the search's text: once they are in,
// a page's backlinks and a word in its text are found without the window reading the pages.
func TestPrewarmFillsTheIndexes(t *testing.T) {
	a, tt := newTestApp(t, "Birinci [[İkinci]] #etiket\n")
	coll := a.pane().page.Coll
	second, _ := coll.NewPage(nil, "İkinci")
	second.SaveText("benzersizkelime burada\n")
	a.prewarm()
	for deadline := time.Now().Add(5 * time.Second); ; {
		a.mu.Lock()
		w := a.prewarming
		a.mu.Unlock()
		if !w {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still reading")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e := a.links.pages[a.pane().page]; e == nil || len(e.links) != 1 || e.links[0] != "ikinci" || len(e.tags) == 0 {
		t.Fatalf("links' index: %+v", e)
	}
	if p := a.links.plain[second]; p == nil || p.folded == "" {
		t.Fatalf("plain text: %+v", p)
	}
	tt.Frame()
	if got := a.linksTo(second, a.clock(), true); len(got) != 1 || got[0] != a.pane().page {
		t.Fatalf("backlinks of İkinci: %v", got)
	}
}

// Opening a library from its file reads the table once: the folders and pages are the same as when
// each is read by itself.
func TestStoreSnapshotOpensTheSame(t *testing.T) {
	file := t.TempDir() + "/k.db"
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Kitaplar")
	p, _ := c.NewPage(nil, "Dune")
	p.SaveText("Baharat\n")
	c.NewPage(p, "Alt")
	plain, _ := OpenLibraryIn(st, StoreRoot)
	st.BeginSnapshot()
	snap, err := OpenLibraryIn(st, StoreRoot)
	if st.Snap.Load() == nil {
		t.Fatal("the open wrote: the snapshot went")
	}
	st.EndSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	names := func(l *Library) []string {
		var out []string
		for _, c := range l.Collections {
			for _, p := range c.All() {
				out = append(out, c.Name+"/"+p.Title)
			}
		}
		return out
	}
	if a, b := names(plain), names(snap); len(a) != 2 || !slices.Equal(a, b) {
		t.Fatalf("plain %v, snapshot %v", a, b)
	}
}

// The pages are read once the window's first frame is built, not before: the reading would take the cores the first
// frame needs (#581).
func TestPrewarmWaitsForTheFirstFrame(t *testing.T) {
	a := newApp(testLibrary(t, "Birinci [[İkinci]]\n"))
	a.noSound = true
	t.Cleanup(a.stamping.Wait)
	a.prewarm()
	time.Sleep(300 * time.Millisecond)
	a.mu.Lock()
	read := len(a.links.pages)
	a.mu.Unlock()
	if read != 0 {
		t.Fatalf("%d pages read before the first frame", read)
	}
	tt := ui.NewTester(a.view, 900, 1400)
	tt.Frame()
	for deadline := time.Now().Add(prewarmWait / 2); ; { // read for the frame, not for the wait running out
		a.mu.Lock()
		w := a.prewarming
		a.mu.Unlock()
		if !w {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not read after the first frame")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.links.pages[a.pane().page] == nil {
		t.Fatal("the page was not read")
	}
}

// A library's pages are read in parts, not all their files at once, and every page is read (#196).
func TestPrewarmInParts(t *testing.T) {
	was := prewarmChunk
	prewarmChunk = 3
	defer func() { prewarmChunk = was }()
	a := syncApp(t, t.TempDir())
	c, _ := a.lib.NewCollection("Notlar")
	var pages []*Page
	for i := range 10 {
		p, _ := c.NewPage(nil, fmt.Sprintf("Sayfa %d", i))
		p.SaveText(fmt.Sprintf("kelime%d\n", i))
		pages = append(pages, p)
	}
	close(a.firstFrame) // as the first frame does
	a.prewarm()
	for deadline := time.Now().Add(5 * time.Second); ; {
		a.mu.Lock()
		w := a.prewarming
		a.mu.Unlock()
		if !w {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still reading")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for i, p := range pages {
		if e := a.links.plain[p]; e == nil || !strings.Contains(e.folded, fmt.Sprintf("kelime%d", i)) {
			t.Fatalf("page %d not read: %+v", i, e)
		}
	}
}

func waitPrewarm(t *testing.T, a *app) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		a.mu.Lock()
		w := a.prewarming
		a.mu.Unlock()
		if !w {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("still prewarming")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPrewarmUsesSQLitePageIndexCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Kitaplar")
	p1, _ := c.NewPage(nil, "Birinci")
	p1.SaveText("- [ ] Oku [[2026-11-30]] #okuma [[İkinci]]\n")
	p2, _ := c.NewPage(nil, "İkinci")
	p2.SaveText("gizliarama kelimesi\n")

	// First cold start populates page_index (all misses).
	a1 := newApp(lib)
	close(a1.firstFrame)
	a1.prewarm()
	waitPrewarm(t, a1)
	if a1.links.warmMisses != 2 || a1.links.warmHits != 0 {
		t.Fatalf("first prewarm hits=%d misses=%d, want 0 hits 2 misses", a1.links.warmHits, a1.links.warmMisses)
	}

	// Second cold open hydrates entirely from SQLite page_index cache without re-parsing.
	lib2, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	a2 := newApp(lib2)
	close(a2.firstFrame)
	a2.prewarm()
	waitPrewarm(t, a2)
	if a2.links.warmHits != 2 || a2.links.warmMisses != 0 {
		t.Fatalf("cached prewarm hits=%d misses=%d, want 2 hits 0 misses", a2.links.warmHits, a2.links.warmMisses)
	}
	q1, q2 := lib2.PageByID(p1.ID), lib2.PageByID(p2.ID)
	if e := a2.links.pages[q1]; e == nil || !slices.Contains(e.links, "ikinci") || len(e.tasks) != 1 || len(e.tags) != 1 {
		t.Fatalf("hydrated pageIdx mismatch: %+v", e)
	}
	if pl := a2.links.plain[q2]; pl == nil || !strings.Contains(pl.folded, "gizliarama") {
		t.Fatalf("hydrated plainIdx mismatch: %+v", pl)
	}

	// Updating one page invalidates only that page; unchanged peer still hits cache.
	q2.SaveText("yenidenyazilan kelime\n")
	lib3, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	a3 := newApp(lib3)
	close(a3.firstFrame)
	a3.prewarm()
	waitPrewarm(t, a3)
	if a3.links.warmHits != 1 || a3.links.warmMisses != 1 {
		t.Fatalf("after one edit hits=%d misses=%d, want 1 hit 1 miss", a3.links.warmHits, a3.links.warmMisses)
	}
	if pl := a3.links.plain[lib3.PageByID(p2.ID)]; pl == nil || !strings.Contains(pl.folded, "yenidenyazilan") {
		t.Fatalf("re-indexed plainIdx mismatch: %+v", pl)
	}
}

// A page written while the app is open is read again for the index, and what was read is kept in the library file's
// page_index as the index is made: the next start reads none of its pages, the edited one included. A new page is
// kept the same way (#594).
func TestPageIndexKeptAfterMidSessionEdits(t *testing.T) {
	file := filepath.Join(t.TempDir(), "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Kitaplar")
	p1, _ := c.NewPage(nil, "Birinci")
	p1.SaveText("- [ ] Oku #okuma [[İkinci]]\n")
	p2, _ := c.NewPage(nil, "İkinci")
	p2.SaveText("ilk kelime\n")

	a := newApp(lib)
	close(a.firstFrame)
	a.prewarm()
	waitPrewarm(t, a)
	a.linksTo(nil, a.clock(), true)

	// mid-session: one page written, one made
	p2.SaveText("yeniyazilan kelime #yeni\n")
	p3, _ := c.NewPage(nil, "Üçüncü")
	p3.SaveText("ucuncusayfa\n")
	a.mu.Lock()
	a.linksStale()
	a.linksTo(nil, a.clock().Add(time.Minute), true)
	a.mu.Unlock()
	a.pageIndexWrites.Wait()

	lib2, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	b := newApp(lib2)
	close(b.firstFrame)
	b.prewarm()
	waitPrewarm(t, b)
	if b.links.warmMisses != 0 || b.links.warmHits != 3 {
		t.Fatalf("after the edits were kept: hits=%d misses=%d, want 3 hits 0 misses", b.links.warmHits, b.links.warmMisses)
	}
	q2 := lib2.PageByID(p2.ID)
	if pl := b.links.plain[q2]; pl == nil || !strings.Contains(pl.folded, "yeniyazilan") {
		t.Fatalf("the edited page's text from the cache: %+v", pl)
	}
	if e := b.links.pages[q2]; e == nil || len(e.tags) != 1 || e.tags[0] != "yeni" {
		t.Fatalf("the edited page's index from the cache: %+v", e)
	}

	// a page written after it was read is not kept with what was read of it
	p2.SaveText("sonraki\n")
	a.mu.Lock()
	a.links.toKeep = map[*Page]bool{p2: true}
	a.keepPageIndexes(true)
	a.mu.Unlock()
	lib3, _ := OpenLibraryIn(st, StoreRoot)
	d := newApp(lib3)
	close(d.firstFrame)
	d.prewarm()
	waitPrewarm(t, d)
	if d.links.warmMisses != 1 {
		t.Fatalf("a page written since it was read: misses=%d, want 1", d.links.warmMisses)
	}
}
