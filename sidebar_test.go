package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"slices"
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

// A double click on a row's name turns it into the field, as the menu's "Yeniden adlandır" does; a click alone does
// not (#266).
func TestRenameOnDoubleClick(t *testing.T) {
	a, tt := newTestApp(t, "x")
	p := a.page
	if err := tt.Click("Sayfa: Sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.renaming != nil {
		t.Fatal("a click alone renames")
	}
	tt.Click("Sayfa: Sayfa")
	tt.Click("Sayfa: Sayfa")
	tt.Advance(50 * time.Millisecond)
	if a.renaming != p {
		t.Fatal("a double click did not rename the row")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Yeni ad")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(50 * time.Millisecond)
	if a.renaming != nil || p.Title != "Yeni ad" {
		t.Fatalf("after Enter: renaming %v, title %q", a.renaming, p.Title)
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
	bin, _ := os.ReadDir(filepath.Join(a.lib.Root, TrashDir))
	if len(bin) != 1 || !strings.HasSuffix(bin[0].Name(), "-cocuk") {
		t.Fatalf("trash: %v", bin)
	}
	if _, err := os.Stat(filepath.Join(a.lib.Root, TrashDir, bin[0].Name(), "torun", PageFileName)); err != nil {
		t.Fatal("the sub-page did not go with it: ", err)
	}
	again, _ := OpenLibrary(a.lib.Root)
	if n := len(again.Collections[0].Pages[0].Children); n != 0 || len(again.Collections) != 1 {
		t.Fatalf("read back: %d children, %d collections (the trash is no collection)", n, len(again.Collections))
	}
}

// A big library with no tree of its own starts with its collections closed,
// but the one of the page open; a small one starts as it did.
func TestBigLibraryStartsClosed(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var cols []*Collection
	for i := 0; i < 4; i++ {
		c, err := lib.NewCollection(fmt.Sprintf("K%d", i))
		if err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	for i := 0; i < 5; i++ {
		cols[0].NewPage(nil, fmt.Sprintf("a%d", i))
	}
	a := newApp(lib)
	a.collapseBigLibrary()
	for _, c := range cols {
		if !c.Open {
			t.Fatalf("a small library closed %s", c.Name)
		}
	}
	for i := 0; i < bigLibraryPages; i++ {
		cols[2].NewPage(nil, fmt.Sprintf("b%d", i))
	}
	a.collapseBigLibrary()
	for _, c := range cols {
		if want := c == a.page.Coll; c.Open != want {
			t.Fatalf("%s: open %v, want %v (the page open is in %s)", c.Name, c.Open, want, a.page.Coll.Name)
		}
	}
}

// TestSidebarEdgeHandle: the sidebar's edge shows nothing while the pointer is away from it, a handle as the pointer
// nears it, plain on the strip that drags it, and in the accent while it is dragged (#459).
func TestSidebarEdgeHandle(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(900, 500)
	a.wframe.on = true
	tt.Advance(400 * time.Millisecond)
	g, ok := tt.Find("sidebar-grip")
	if !ok {
		t.Fatal("no edge strip")
	}
	x, y := g.X+g.W/2, float32(260)
	at := func(px, py float32) color.RGBA { return tt.Image().RGBAAt(int(px), int(py)) }
	tt.Move(x-80, y)
	tt.Advance(400 * time.Millisecond)
	ground := at(x, y)
	tt.Move(x, y)
	tt.Advance(400 * time.Millisecond)
	if on := at(x, y); on == ground {
		t.Fatalf("on the strip the edge shows no handle: %v", on)
	}
	tt.Move(x-80, y)
	tt.Advance(600 * time.Millisecond)
	if off := at(x, y); off != ground {
		t.Fatalf("away from the edge the handle stays: %v, the ground is %v", off, ground)
	}
}

// TestSidebarManyCollections: a library of thousands of collections, all open, builds only the rows near what the
// sidebar shows, at the top and scrolled to the end; the others are gaps, and the list of rows has every one (#581).
func TestSidebarManyCollections(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 800)
	const n = 1500
	for i := range n {
		c, err := a.lib.NewCollection(fmt.Sprintf("C%04d", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.NewPage(nil, fmt.Sprintf("P%04d", i)); err != nil {
			t.Fatal(err)
		}
		c.Open = true
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	want := a.treeCensus().rows
	if want < 2*n || len(a.rows) != want {
		t.Fatalf("%d rows listed, the tree has %d", len(a.rows), want)
	}
	if a.rowsBuilt > 200 {
		t.Fatalf("%d rows built of %d", a.rowsBuilt, want)
	}
	if !tt.HasText("C0001") || tt.HasText("C1499") {
		t.Fatal("not the first collections at the top")
	}
	// to the end of the tree: the last ones are built there
	for range 40 {
		tt.Wheel(100, 400, 0, 40000)
		tt.Advance(500 * time.Millisecond)
		if tt.HasText("C1499") {
			break
		}
	}
	if !tt.HasText("C1499") || a.rowsBuilt > 200 {
		r, _ := tt.Find("C0001")
		t.Fatalf("scrolled to the end: the last collection shown %v, %d rows built, top %v view %v, sidebar %v, C0001 at %v", tt.HasText("C1499"), a.rowsBuilt, a.treeTop, a.treeView, a.sidebarR, r)
	}
}

// TestSidebarCollectionSticks: a collection's row stays at the top of the tree while its pages scroll under it, a
// click there opens the collection, not the page under it, and the next collection takes its place (#354).
func TestSidebarCollectionSticks(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 700)
	c, err := a.lib.NewCollection("Projects")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		if _, err := c.NewPage(nil, fmt.Sprintf("Page %02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	c.Open = true
	d, err := a.lib.NewCollection("Reading")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		if _, err := d.NewPage(nil, fmt.Sprintf("Book %02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	d.Open = true
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	first, _ := tt.Find("Page 00")
	tt.Wheel(100, 400, 0, 300)
	tt.Advance(600 * time.Millisecond)
	head, ok := tt.Find("Projects")
	if !ok || head.Y > first.Y {
		t.Fatalf("scrolled, the collection's row is at %v (%v), above where its first page was (%v)", head, ok, first)
	}
	tt.ClickAt(head.X+20, head.Y+head.H/2)
	tt.Advance(300 * time.Millisecond)
	if a.coll != c || !a.noPage() {
		t.Fatalf("a click on the held row opened %v, page %v", a.coll, a.page)
	}
	// to the end: the next collection has pushed it away, and is held in its place
	tt.Wheel(100, 400, 0, 4000)
	tt.Advance(600 * time.Millisecond)
	if r, ok := tt.Find("Reading"); !ok || r.Y != head.Y {
		t.Fatalf("the next collection at %v (%v), the held row was at %v", r, ok, head)
	}
	if r, ok := tt.Find("Projects"); ok && r.Y+r.H > head.Y {
		t.Fatalf("the first collection is still held at %v", r)
	}
}

// TestContextMenuKey: a row of the tree with the focus opens its menu for Shift+F10 and the menu key, below it, as
// MyGo's own right-click menus do, and Esc closes it (#575).
func TestContextMenuKey(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 600)
	tt.Frame()
	for _, press := range []func(){
		func() { tt.Key(ui.Shift, ui.KeyF10) },
		func() { tt.Key(0, ui.KeyContextMenu) },
	} {
		a.treeFocus = a.page
		tt.Frame()
		tt.Frame()
		press()
		tt.Frame()
		if m := tt.Menu(); !slices.Contains(m, msg("common_open")) {
			t.Fatalf("the row's menu: %v", m)
		}
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		if tt.Menu() != nil {
			t.Fatal("Esc did not close it")
		}
	}
}

// TestTreeKept: what is opened or closed in the tree is kept in the settings within a second, the look at every page
// not made each frame, and at once as the app quits (#204); the census counts the rows shown, not closed branches.
func TestTreeKept(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.clock = tt.Now
	c := a.lib.Collections[0]
	if _, err := c.NewPage(a.page, "Alt"); err != nil {
		t.Fatal(err)
	}
	a.page.Open = false
	tt.Advance(2 * time.Second)
	if rows := a.treeCensus().rows; rows != 2 {
		t.Fatalf("rows %d, want the collection and its page", rows)
	}
	a.page.Open = true
	tt.Advance(100 * time.Millisecond)
	if rows := a.treeCensus().rows; rows != 3 {
		t.Fatalf("rows %d, want the child too", rows)
	}
	tt.Advance(1100 * time.Millisecond)
	if !slices.Contains(a.settings.TreeOpen, a.page.Dir) {
		t.Fatalf("not kept within a second: %v", a.settings.TreeOpen)
	}
	a.page.Open = false
	tt.Frame()
	a.keepTree(true) // as the app quits
	if slices.Contains(a.settings.TreeOpen, a.page.Dir) {
		t.Fatalf("not kept at once: %v", a.settings.TreeOpen)
	}
}
