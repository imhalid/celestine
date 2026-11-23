package main

import (
	"fmt"
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
	coll := a.page.Coll
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
	if e := a.links.pages[a.page]; e == nil || len(e.links) != 1 || e.links[0] != "ikinci" || len(e.tags) == 0 {
		t.Fatalf("links' index: %+v", e)
	}
	if p := a.links.plain[second]; p == nil || p.folded == "" {
		t.Fatalf("plain text: %+v", p)
	}
	tt.Frame()
	if got := a.linksTo(second, a.clock(), true); len(got) != 1 || got[0] != a.page {
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
	if a.links.pages[a.page] == nil {
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
