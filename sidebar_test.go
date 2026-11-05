package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSidebarFolds: ⌘\ folds the sidebar on a spring - on its way in
// between, never past - and brings it back.
func TestSidebarFolds(t *testing.T) {
	a, tt := newTestApp(t, "x")
	tt.Advance(100 * time.Millisecond)
	if math.Abs(float64(a.sidebarShown-sidebarW)) > 0.5 {
		t.Fatalf("open: %v", a.sidebarShown)
	}
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Advance(80 * time.Millisecond)
	if mid := a.sidebarShown; mid <= 0 || mid >= sidebarW {
		t.Fatalf("folding should be under way: %v", mid)
	}
	low := a.sidebarShown
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		if a.sidebarShown < -0.5 || a.sidebarShown > low+0.5 {
			t.Fatalf("went past or back: %v after %v", a.sidebarShown, low)
		}
		low = a.sidebarShown
	}
	if a.sidebarShown > 0.5 {
		t.Fatalf("folded: %v", a.sidebarShown)
	}
	if err := tt.Click("Kenar çubuğu (⌘\\)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(800 * time.Millisecond)
	if math.Abs(float64(a.sidebarShown-sidebarW)) > 0.5 {
		t.Fatalf("back: %v", a.sidebarShown)
	}
}

// TestBranchFolds: a page's chevron hides its children, and they come back.
func TestBranchFolds(t *testing.T) {
	a, tt := newTestApp(t, "x")
	parent := a.page
	if _, err := parent.Coll.NewPage(parent, "Çocuk"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second) // the branch has unfolded
	if err := tt.Click("Sayfa: Çocuk"); err != nil {
		t.Fatal("the child row is not shown: ", err)
	}
	parent.Open = false
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click("Sayfa: Çocuk"); err == nil {
		t.Fatal("the child row is still there once its branch folded")
	}
}

// TestRenameInPlace: the menu's "Yeniden adlandır" turns the row into a
// field; Enter keeps the name typed, Escape the old one.
func TestRenameInPlace(t *testing.T) {
	a, tt := newTestApp(t, "x")
	p := a.page
	if err := tt.RightClick("Sayfa: Sayfa"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Yeniden adlandır"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(50 * time.Millisecond)
	if a.renaming != p {
		t.Fatal("the row is not being renamed")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Yeni ad")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(50 * time.Millisecond)
	if a.renaming != nil || p.Title != "Yeni ad" || a.title != "Yeni ad" {
		t.Fatalf("after Enter: renaming %v, title %q, field %q", a.renaming, p.Title, a.title)
	}
	if again, _ := OpenLibrary(a.lib.Root); again.Collections[0].Pages[0].Title != "Yeni ad" {
		t.Fatal("the name is not on disk")
	}
	// Escape keeps the old name.
	tt.RightClick("Sayfa: Yeni ad")
	tt.Click("Yeniden adlandır")
	tt.Advance(50 * time.Millisecond)
	tt.Type("bozuk")
	tt.Key(0, ui.KeyEscape)
	tt.Advance(50 * time.Millisecond)
	if a.renaming != nil || p.Title != "Yeni ad" {
		t.Fatalf("after Escape: %q", p.Title)
	}
}

// TestTrashPage: "Çöpe taşı" moves the page's folder, its sub-pages with it,
// into the library's trash; the open page closes to its parent.
func TestTrashPage(t *testing.T) {
	a, tt := newTestApp(t, "x")
	parent := a.page
	child, _ := parent.Coll.NewPage(parent, "Çocuk")
	grand, _ := parent.Coll.NewPage(child, "Torun")
	a.open(grand)
	tt.Frame()
	tt.Advance(time.Second)
	if err := tt.RightClick("Sayfa: Çocuk"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Çöpe taşı"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(50 * time.Millisecond)
	if len(parent.Children) != 0 || a.page != parent {
		t.Fatalf("after trash: children %d, open %v", len(parent.Children), a.page.Title)
	}
	if _, err := os.Stat(child.Dir); !os.IsNotExist(err) {
		t.Fatal("the folder is still in place")
	}
	bin, _ := os.ReadDir(filepath.Join(a.lib.Root, trashDir))
	if len(bin) != 1 || !strings.HasSuffix(bin[0].Name(), "-cocuk") {
		t.Fatalf("trash: %v", bin)
	}
	if _, err := os.Stat(filepath.Join(a.lib.Root, trashDir, bin[0].Name(), "torun", "index.md")); err != nil {
		t.Fatal("the sub-page did not go with it: ", err)
	}
	again, _ := OpenLibrary(a.lib.Root)
	if n := len(again.Collections[0].Pages[0].Children); n != 0 || len(again.Collections) != 1 {
		t.Fatalf("read back: %d children, %d collections (the trash is no collection)", n, len(again.Collections))
	}
}
