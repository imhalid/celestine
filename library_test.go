package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLibraryOnDisk(t *testing.T) {
	root := t.TempDir()
	lib, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Okuma Listesi")
	a, _ := c.NewPage(nil, "Çalışma: Ünite 1")
	b, _ := c.NewPage(a, "Alt Sayfa")
	twin, _ := c.NewPage(nil, "Çalışma: Ünite 1") // the same title: a folder of its own
	if err := b.SaveText("- bir\n- iki\n"); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(c.Dir) != "okuma-listesi" || filepath.Base(a.Dir) != "calisma-unite-1" || filepath.Base(twin.Dir) != "calisma-unite-1-2" {
		t.Fatalf("folders: %s %s %s", c.Dir, a.Dir, twin.Dir)
	}
	if filepath.Dir(b.Dir) != a.Dir {
		t.Fatalf("a sub-page lives in its parent's folder: %s", b.Dir)
	}
	// Renaming keeps the folder.
	if err := a.SetTitle("Yeni ad: \"tırnaklı\""); err != nil {
		t.Fatal(err)
	}
	// Read back from disk, it is the same tree.
	again, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Collections) != 1 || again.Collections[0].Name != "Okuma Listesi" {
		t.Fatalf("collections: %+v", again.Collections)
	}
	pages := again.Collections[0].Pages
	if len(pages) != 2 || pages[0].Title != "Yeni ad: \"tırnaklı\"" || pages[0].Dir != a.Dir || len(pages[0].Children) != 1 {
		t.Fatalf("pages: %+v", pages)
	}
	sub := pages[0].Children[0]
	if sub.Title != "Alt Sayfa" || sub.Parent != pages[0] || sub.Depth() != 1 {
		t.Fatalf("sub-page: %+v", sub)
	}
	if body, _ := sub.Text(); body != "- bir\n- iki\n" {
		t.Fatalf("body: %q", body)
	}
	src, _ := os.ReadFile(filepath.Join(a.Dir, PageFileName))
	if m := PageMeta(src); m["title"] != "Yeni ad: \"tırnaklı\"" || m["created"] == "" {
		t.Fatalf("meta:\n%s", src)
	}
	// A folder without a collection.json is not a collection.
	os.Mkdir(filepath.Join(root, "baska"), 0o755)
	if again, _ := OpenLibrary(root); len(again.Collections) != 1 {
		t.Fatal("a stray folder became a collection")
	}
}

// TestSidebarMakesSubPage clicks a page's + and gets a sub-page, open, on disk.
func TestSidebarMakesSubPage(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	a.clock = tt.Now // the autosave's delay, in the tester's time
	parent := a.pane().page
	if err := tt.Click("Ekle: Sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(50 * time.Millisecond)
	if len(parent.Children) != 1 || a.pane().page != parent.Children[0] || a.pane().page.Title != "Adsız" {
		t.Fatalf("sub-page: %+v, open %v", parent.Children, a.pane().page)
	}
	if _, err := os.Stat(filepath.Join(parent.Dir, "adsiz", PageFileName)); err != nil {
		t.Fatal(err)
	}
	// The new title is typed straight away, and saved once the typing pauses, with the page (#192).
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Toplantı notları")
	tt.Advance(50 * time.Millisecond)
	if a.pane().page.Title != "Toplantı notları" {
		t.Fatalf("title: %q", a.pane().page.Title)
	}
	if again, _ := OpenLibrary(a.lib.Root); again.Collections[0].Pages[0].Children[0].Title == "Toplantı notları" {
		t.Fatal("the title was written at once, at a key")
	}
	tt.Advance(a.saveAfter() + 100*time.Millisecond)
	if again, _ := OpenLibrary(a.lib.Root); again.Collections[0].Pages[0].Children[0].Title != "Toplantı notları" {
		t.Fatal("the title is not on disk")
	}
}

// TestSwitchingPagesSaves edits a page, opens another: the edit is on disk.
func TestSwitchingPagesSaves(t *testing.T) {
	a, tt := newTestApp(t, "ilk")
	first := a.pane().page
	tt.Type("!")
	tt.Frame()
	other, _ := first.Coll.NewPage(nil, "İkinci")
	a.open(other)
	tt.Frame()
	if body, _ := first.Text(); !strings.Contains(body, "!") {
		t.Fatalf("not saved on leaving: %q", body)
	}
	a.open(first) // its document, undo and all, is kept
	if !strings.Contains(a.pane().doc.Text().String(), "!") {
		t.Fatal("the edit is gone on coming back")
	}
}
