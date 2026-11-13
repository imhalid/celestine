package library

import (
	"testing"
)

// TestRestorePageOfTrashedCollection: a page trashed before its collection comes back into that collection, which
// comes back from the trash with it; with the collection gone for good, the page goes to a collection that takes
// pages, never the journal, however the collections are ordered (#481, #482).
func TestRestorePageOfTrashedCollection(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	journal, _ := lib.NewCollection("Journal")
	journal.Journal = true
	if err := journal.Save(); err != nil {
		t.Fatal(err)
	}
	notes, _ := lib.NewCollection("Notes")
	welcome, err := notes.NewPage(nil, "Welcome")
	if err != nil {
		t.Fatal(err)
	}
	pageItem, err := lib.TrashPage(welcome)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.TrashCollection(notes); err != nil {
		t.Fatal(err)
	}
	c, p, err := lib.Restore(pageItem)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.Name != "Notes" || p == nil || p.Coll != c {
		t.Fatalf("restored into %v, want Notes back", c)
	}
	for _, it := range lib.Trash() {
		if it.Kind == "collection" {
			t.Fatalf("the collection is still in the trash: %s", it.Title)
		}
	}
	if len(journal.All()) != 0 {
		t.Fatalf("the journal took pages: %d", len(journal.All()))
	}

	// the collection deleted for good: another collection that takes pages, not the journal
	other, _ := lib.NewCollection("Archive")
	item2, err := lib.TrashPage(p)
	if err != nil {
		t.Fatal(err)
	}
	collItem, err := lib.TrashCollection(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.DeleteForever(collItem); err != nil {
		t.Fatal(err)
	}
	c2, _, err := lib.Restore(item2)
	if err != nil {
		t.Fatal(err)
	}
	if c2 != other {
		t.Fatalf("restored into %v, want Archive (not the journal)", c2.Name)
	}
}

// TestTrashedPages: a collection in the trash shows the pages it holds, sub-pages too, in its tree's order; a page
// item holds none (#481).
func TestTrashedPages(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	notes, _ := lib.NewCollection("Notes")
	a, _ := notes.NewPage(nil, "A")
	notes.NewPage(a, "A child")
	b, _ := notes.NewPage(nil, "B")
	pageItem, err := lib.TrashPage(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.TrashedPages(pageItem); got != nil {
		t.Fatalf("a page item holds %d", len(got))
	}
	item, err := lib.TrashCollection(notes)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, p := range lib.TrashedPages(item) {
		titles = append(titles, p.Title)
	}
	if len(titles) != 2 || titles[0] != "A" || titles[1] != "A child" {
		t.Fatalf("trashed pages %v", titles)
	}
}

// TestRestoreOutOfTrashedCollection: a page of a collection in the trash comes back alone, its sub-pages and text
// with it, to a collection that takes pages; the collection stays in the trash without it (#481).
func TestRestoreOutOfTrashedCollection(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inbox, _ := lib.NewCollection("Inbox")
	notes, _ := lib.NewCollection("Notes")
	a, _ := notes.NewPage(nil, "A")
	a.SaveText("bir\n")
	notes.NewPage(a, "A child")
	notes.NewPage(nil, "B")
	item, err := lib.TrashCollection(notes)
	if err != nil {
		t.Fatal(err)
	}
	trashed := lib.TrashedPages(item)
	coll, p, err := lib.RestoreOutOf(item, trashed[0])
	if err != nil {
		t.Fatal(err)
	}
	if coll != inbox || p.Title != "A" || len(p.Children) != 1 || p.Coll != inbox {
		t.Fatalf("back: %v %+v", coll.Name, p)
	}
	if body, _ := p.Text(); body != "bir\n" {
		t.Fatalf("its text: %q", body)
	}
	left := lib.TrashedPages(item)
	if len(left) != 1 || left[0].Title != "B" || len(lib.Trash()) != 1 {
		t.Fatalf("left in the trash: %d pages, %d items", len(left), len(lib.Trash()))
	}
	again, _ := OpenLibrary(lib.Root)
	if len(again.Collections) != 1 || len(again.Collections[0].Pages) != 1 {
		t.Fatal("not on the disk")
	}
	if _, _, err := lib.RestoreOutOf(item, p); err == nil {
		t.Fatal("a page out of the trash was taken for one in it")
	}
}
