package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestRestoreOnDisk: a page thrown away goes back under its page, its
// sub-pages and text with it; a collection goes back in the library; a page
// whose page is gone goes to the top of its collection; deleted for good,
// it is gone from the disk.
func TestRestoreOnDisk(t *testing.T) {
	lib, ps, one, two := dragLibrary(t)
	ps["B1"].SaveText("bir\n")
	item, err := lib.TrashPage(ps["B"])
	if err != nil {
		t.Fatal(err)
	}
	trash := lib.Trash()
	if len(trash) != 1 || trash[0].Title != "B" || trash[0].Kind != "page" || trash[0].From != filepath.Join("bir", "b") {
		t.Fatalf("trash: %+v", trash[0])
	}
	coll, p, err := lib.Restore(item)
	if err != nil {
		t.Fatal(err)
	}
	if coll != one || p.Title != "B" || p.Parent != nil || !equal(titles(one.Pages), []string{"A", "C", "B"}) {
		t.Fatalf("back: %v", titles(one.Pages))
	}
	if len(p.Children) != 1 || p.Children[0].Coll != one {
		t.Fatal("the sub-page did not come back with it")
	}
	if body, _ := p.Children[0].Text(); body != "bir\n" {
		t.Fatalf("its text: %q", body)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, TrashNote)); !os.IsNotExist(err) {
		t.Fatal("the note came back too")
	}
	if len(lib.Trash()) != 0 {
		t.Fatal("still in the trash")
	}
	// A sub-page whose page is thrown away after it: to the top.
	b1 := p.Children[0]
	sub, _ := lib.TrashPage(b1)
	lib.TrashPage(p)
	if _, q, err := lib.Restore(sub); err != nil || q.Parent != nil || q.Coll != one {
		t.Fatalf("orphan: %v %+v", err, q)
	}
	// A collection.
	citem, _ := lib.TrashCollection(two)
	if len(lib.Collections) != 1 {
		t.Fatal("the collection is still there")
	}
	if c, _, err := lib.Restore(citem); err != nil || c.Name != "İki" || len(lib.Collections) != 2 {
		t.Fatalf("collection back: %v", err)
	}
	// For good.
	left := lib.Trash()
	if len(left) != 1 {
		t.Fatalf("left: %d", len(left))
	}
	if err := lib.DeleteForever(left[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(left[0].Dir); !os.IsNotExist(err) || len(lib.Trash()) != 0 {
		t.Fatal("not deleted")
	}
	again, _ := OpenLibrary(lib.Root)
	if len(again.Collections) != 2 {
		t.Fatalf("read back: %d collections", len(again.Collections))
	}
}

// TestTrashUndoAndView: the toast's "Geri al" puts a page back and opens
// it; thrown away again, the trash shows it and puts it back.
func TestTrashUndoAndView(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	page := a.pane().page
	child, _ := page.Coll.NewPage(page, "Çocuk")
	a.open(child)
	tt.Advance(time.Second)
	tt.RightClick("Sayfa: Çocuk")
	if err := tt.Click("Çöpe taşı"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(700 * time.Millisecond) // the toast rose
	if len(page.Children) != 0 {
		t.Fatal("not thrown away")
	}
	if err := tt.Click("Geri al"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	if len(page.Children) != 1 || a.pane().page != page.Children[0] {
		t.Fatalf("undo: %d children", len(page.Children))
	}
	tt.Advance(5 * time.Second) // the toast goes
	tt.RightClick("Sayfa: Çocuk")
	tt.Click("Çöpe taşı")
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Çöp"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !a.pane().trashOpen || !tt.HasText("Çocuk") {
		t.Fatalf("trash view: %v", tt.Texts())
	}
	if err := tt.Click("Geri yükle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if len(page.Children) != 1 || len(a.trashItems) != 0 || !tt.HasText("Çöp boş") {
		t.Fatalf("restored from the view: %d, %v", len(page.Children), tt.Texts())
	}
}

// TestTrashShowsCollectionPages: a collection in the trash says how many pages went with it, and unfolds them by
// that line; each comes back alone by its button (#481).
func TestTrashShowsCollectionPages(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if _, err := a.lib.TrashCollection(coll); err != nil {
		t.Fatal(err)
	}
	a.lib.Collections = slices.DeleteFunc(a.lib.Collections, func(c *Collection) bool { return c == coll })
	a.pane().coll, a.pane().page = nil, nil
	a.openTrash()
	tt.Advance(500 * time.Millisecond)
	n := locales.CommonPageCount(4)
	if !tt.HasText(n) || tt.HasText("Yüzyıllık Yalnızlık") {
		t.Fatalf("no folded count of the pages: %v", tt.Texts())
	}
	if err := tt.Click(n); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	for _, title := range []string{"Dune", "Notlar: Dune", "Anna Karenina", "Yüzyıllık Yalnızlık"} {
		if !tt.HasText(title) {
			t.Fatalf("%s not shown under the collection: %v", title, tt.Texts())
		}
	}
	// one of them back alone, to the collection there is; the collection stays in the trash without it
	other, _ := a.lib.NewCollection("Gelen")
	if err := tt.Click(locales.TrashRestoreAlone("Anna Karenina")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if len(other.Pages) != 1 || other.Pages[0].Title != "Anna Karenina" || len(a.trashItems) != 1 || len(a.trashPages[a.trashItems[0].Dir]) != 3 {
		t.Fatalf("restored alone: %d pages in Gelen, %d items", len(other.Pages), len(a.trashItems))
	}
}
