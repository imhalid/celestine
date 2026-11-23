package main

import (
	"testing"
	"time"
)

// The index keeps what it read of a page while its file is as it was, and reads
// it again once it was written: a link added or taken out shows, one that is
// not touched is not read again.
func TestLinkIndexFollowsWrites(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("K")
	a, _ := c.NewPage(nil, "A")
	b, _ := c.NewPage(nil, "B")
	other, _ := c.NewPage(nil, "Other")
	a.SaveText("bağlantı: [[B]] ve #etiket\n")
	other.SaveText("- [ ] görev\n")
	app := newApp(lib)
	now := time.Now()
	backlinks := func() int { return len(app.linksTo(b, now, true)) }
	app.linksStale()
	if n := backlinks(); n != 1 {
		t.Fatalf("A links to B: %d backlinks", n)
	}
	kept := app.links.pages[other]
	if kept == nil || len(kept.tasks) != 1 {
		t.Fatalf("what was read of Other: %+v", kept)
	}
	// A page written since is read again; one that was not is the same read
	time.Sleep(2 * time.Millisecond)
	if err := a.SaveText("sildim\n"); err != nil {
		t.Fatal(err)
	}
	app.linksStale()
	if n := backlinks(); n != 0 {
		t.Fatalf("A no longer links to B: %d backlinks", n)
	}
	if app.links.pages[other] != kept {
		t.Fatal("Other was read again, its file as it was")
	}
	time.Sleep(2 * time.Millisecond)
	if err := a.SaveText("yine [[b]]\n"); err != nil {
		t.Fatal(err)
	}
	app.linksStale()
	if n := backlinks(); n != 1 {
		t.Fatalf("A links to B again: %d backlinks", n)
	}
	// the text for the search is made when asked for, and follows the writes
	if got := app.plainOf(a).folded; got == "" {
		t.Fatalf("the text of A: %q", got)
	}
	first := app.plainOf(a)
	if app.plainOf(a) != first {
		t.Fatal("the plain text was made again, the file as it was")
	}
	time.Sleep(2 * time.Millisecond)
	a.SaveText("başka\n")
	if app.plainOf(a) == first || app.plainOf(a).folded != fold("başka") {
		t.Fatalf("the text did not follow the write: %q", app.plainOf(a).folded)
	}
}
