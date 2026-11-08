package main

import (
	"slices"
	"testing"
	"time"
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
	if p := a.links.plain[second]; p == nil || p.plain == "" {
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
	lib, err := OpenLibraryIn(st, storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Kitaplar")
	p, _ := c.NewPage(nil, "Dune")
	p.SaveText("Baharat\n")
	c.NewPage(p, "Alt")
	plain, _ := OpenLibraryIn(st, storeRoot)
	st.snapshot()
	snap, err := OpenLibraryIn(st, storeRoot)
	if st.snap.Load() == nil {
		t.Fatal("the open wrote: the snapshot went")
	}
	st.endSnapshot()
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
