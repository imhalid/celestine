package library

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeMeta writes page p's file with front matter meta and an empty body, as an older client or another hand
// would have left it.
func writeMeta(t *testing.T, p *Page, meta map[string]string) {
	t.Helper()
	src, err := JoinPageChecked(meta, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.FS().WriteFile(p.File(), src); err != nil {
		t.Fatal(err)
	}
}

// pageNamed is the page of the library's first collection titled title.
func pageNamed(t *testing.T, lib *Library, title string) *Page {
	t.Helper()
	for _, p := range lib.Collections[0].All() {
		if p.Title == title {
			return p
		}
	}
	t.Fatalf("no page %q", title)
	return nil
}

// TestReservedPropertyRenamed: a collection property given the id "properties" or "covershift" before those keys
// were reserved gets another id when the library is read, with its values in the pages and its places in the views,
// and they survive a save (#531).
func TestReservedPropertyRenamed(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	books.Properties = append(books.Properties,
		Property{ID: "properties", Name: "Properties", Type: PropText},
		Property{ID: "covershift", Name: "Covershift", Type: PropNumber})
	books.Views = append(books.Views, View{ID: "t", Label: "T", Type: "table", Sort: "prop:properties",
		Columns: []string{"prop:properties", "prop:covershift"}, Filters: []Filter{{Prop: "covershift", Op: "set"}}})
	if err := books.Save(); err != nil {
		t.Fatal(err)
	}
	dune, _ := books.NewPage(nil, "Dune")
	writeMeta(t, dune, map[string]string{"title": "Dune", "created": "2026-11-01T10:00:00Z",
		"properties": "hard cover, signed", "covershift": "7"})

	again, err := OpenLibrary(lib.Root)
	if err != nil {
		t.Fatal(err)
	}
	c := again.Collections[0]
	props, shift := c.PropNamed("Properties"), c.PropNamed("Covershift")
	if props == nil || shift == nil || slices.Contains(reserved, props.ID) || slices.Contains(reserved, shift.ID) {
		t.Fatalf("not renamed: %+v", c.Properties)
	}
	d := pageNamed(t, again, "Dune")
	if d.Value(props) != "hard cover, signed" || d.Value(shift) != "7" || d.CoverShift != 7 {
		t.Fatalf("values: %v, cover shift %d", d.Props, d.CoverShift)
	}
	var view *View
	for i := range c.Views {
		if c.Views[i].ID == "t" {
			view = &c.Views[i]
		}
	}
	if view == nil || view.Sort != "prop:"+props.ID || !slices.Equal(view.Columns, []string{"prop:" + props.ID, "prop:" + shift.ID}) ||
		len(view.Filters) != 1 || view.Filters[0].Prop != shift.ID {
		t.Fatalf("the view still names the old ids: %+v", view)
	}

	// the rename wrote the page: the values under their new ids, none under the reserved keys
	src, err := d.FS().ReadFile(d.File())
	if err != nil {
		t.Fatal(err)
	}
	meta := PageMeta(src)
	if meta[props.ID] != "hard cover, signed" || meta[shift.ID] != "7" || meta["properties"] != "" || meta["covershift"] != "7" {
		t.Fatalf("the file after the rename: %v", meta)
	}

	// a save keeps them, and they are read back from the file
	if err := d.SetValue(props.ID, "hard cover, signed, first edition"); err != nil {
		t.Fatal(err)
	}
	third, _ := OpenLibrary(lib.Root)
	if c := third.Collections[0]; c.Prop(props.ID) == nil || c.Prop(shift.ID) == nil || len(c.Properties) != 2 {
		t.Fatalf("the rename was not kept, or ran again: %+v", c.Properties)
	}
	d = pageNamed(t, third, "Dune")
	if d.Props[props.ID] != "hard cover, signed, first edition" || d.Props[shift.ID] != "7" {
		t.Fatalf("after a save: %v", d.Props)
	}
}

// TestUnreadableOwnPropertiesKept: a "properties" value that is not a page's own properties (the value of a
// property that had the id, or what a newer client wrote) is written back as it was, not dropped (#531).
func TestUnreadableOwnPropertiesKept(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	author, _ := books.AddProperty("Author", PropText, nil)
	dune, _ := books.NewPage(nil, "Dune")
	writeMeta(t, dune, map[string]string{"title": "Dune", "created": "2026-11-01T10:00:00Z", "properties": "hard cover, signed"})

	again, _ := OpenLibrary(lib.Root)
	d := pageNamed(t, again, "Dune")
	if err := d.SetValue(author.ID, "Herbert"); err != nil {
		t.Fatal(err)
	}
	src, err := d.FS().ReadFile(d.File())
	if err != nil {
		t.Fatal(err)
	}
	if meta := PageMeta(src); meta["properties"] != "hard cover, signed" || meta[author.ID] != "Herbert" {
		t.Fatalf("after a save: %v", meta)
	}
	if _, err := d.AddProperty("Mood", PropText, nil); err == nil {
		t.Fatal("a property of its own was added over properties that could not be read")
	}
}

// TestSetValueReservedRefused: a property whose id is a key of the page itself does not take a value it would not
// write (#531).
func TestSetValueReservedRefused(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	dune, _ := books.NewPage(nil, "Dune")
	for _, id := range []string{"properties", "covershift"} {
		books.Properties = append(books.Properties, Property{ID: id, Name: id, Type: PropText}) // as read before #531
		if err := dune.SetValue(id, "hard cover, signed"); err == nil {
			t.Errorf("SetValue(%q) returned nil, and wrote nothing", id)
		}
	}
}

// legacyBooks makes a collection whose properties "Properties" and "Covershift" have the reserved ids they were
// once given, with the pages Dune and Emma holding their values under those keys, and returns the library's root
// and the collection.json as it was then.
func legacyBooks(t *testing.T) (string, []byte) {
	t.Helper()
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	books.Properties = append(books.Properties,
		Property{ID: "properties", Name: "Properties", Type: PropText},
		Property{ID: "covershift", Name: "Covershift", Type: PropText})
	if err := books.Save(); err != nil {
		t.Fatal(err)
	}
	dune, _ := books.NewPage(nil, "Dune")
	emma, _ := books.NewPage(nil, "Emma")
	writeMeta(t, dune, map[string]string{"title": "Dune", "created": "2026-11-01T10:00:00Z",
		"properties": "hard cover, signed", "covershift": "7"})
	writeMeta(t, emma, emmaMeta)
	old, err := books.FS.ReadFile(filepath.Join(books.Dir, CollectionFile))
	if err != nil {
		t.Fatal(err)
	}
	return lib.Root, old
}

// emmaMeta is the front matter of legacyBooks' Emma: a "covershift" that is no cover's position.
var emmaMeta = map[string]string{"title": "Emma", "created": "2026-11-02T10:00:00Z", "properties": "paperback", "covershift": "wide"}

// TestReservedRenameRepeated: when the collection.json of before the rename comes back (an older client syncs it,
// or a conflict keeps the old copy), the next rename takes the ids of the first again: the values stay the
// properties', and a cover position set since stays the cover's (#531).
func TestReservedRenameRepeated(t *testing.T) {
	root, old := legacyBooks(t)
	first, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c := first.Collections[0]
	props, shift := c.PropNamed("Properties").ID, c.PropNamed("Covershift").ID
	if e := pageNamed(t, first, "Emma"); e.Props[shift] != "wide" || e.CoverShift != 0 {
		t.Fatalf("a covershift that is no position: %v, cover shift %d", e.Props, e.CoverShift)
	}
	d := pageNamed(t, first, "Dune")
	d.CoverShift = 30 // set by the user since
	if err := d.saveMeta(); err != nil {
		t.Fatal(err)
	}
	if err := c.FS.WriteFile(filepath.Join(c.Dir, CollectionFile), old); err != nil {
		t.Fatal(err)
	}

	again, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c = again.Collections[0]
	if c.PropNamed("Properties").ID != props || c.PropNamed("Covershift").ID != shift || len(c.Properties) != 2 {
		t.Fatalf("renamed to other ids: %+v, want %s and %s", c.Properties, props, shift)
	}
	d = pageNamed(t, again, "Dune")
	if d.Props[props] != "hard cover, signed" || d.Props[shift] != "7" || d.CoverShift != 30 {
		t.Fatalf("Dune: %v, cover shift %d", d.Props, d.CoverShift)
	}
	if e := pageNamed(t, again, "Emma"); e.Props[props] != "paperback" || e.Props[shift] != "wide" {
		t.Fatalf("Emma: %v", e.Props)
	}
	for k := range d.Props {
		if strings.HasSuffix(k, "-3") {
			t.Fatalf("a third id: %v", d.Props)
		}
	}
}

// TestReservedRenameInterrupted: a rename cut short, with Dune written and Emma and the collection.json not, is
// finished by the next one under the same ids, and none of the values is lost (#531).
func TestReservedRenameInterrupted(t *testing.T) {
	root, old := legacyBooks(t)
	lib, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c := lib.Collections[0]
	props, shift := c.PropNamed("Properties").ID, c.PropNamed("Covershift").ID
	writeMeta(t, pageNamed(t, lib, "Emma"), emmaMeta)
	if err := c.FS.WriteFile(filepath.Join(c.Dir, CollectionFile), old); err != nil {
		t.Fatal(err)
	}

	again, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c = again.Collections[0]
	if c.PropNamed("Properties").ID != props || c.PropNamed("Covershift").ID != shift {
		t.Fatalf("renamed to other ids: %+v, want %s and %s", c.Properties, props, shift)
	}
	d, e := pageNamed(t, again, "Dune"), pageNamed(t, again, "Emma")
	if d.Props[props] != "hard cover, signed" || d.Props[shift] != "7" || e.Props[props] != "paperback" || e.Props[shift] != "wide" {
		t.Fatalf("values: Dune %v, Emma %v", d.Props, e.Props)
	}
}

func TestPageIDPersistenceAndLookup(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	notes, _ := lib.NewCollection("Notes")
	other, _ := lib.NewCollection("Archive")
	p1, err := notes.NewPage(nil, "First")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := notes.NewPage(nil, "Second")
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID == "" || p2.ID == "" || p1.ID == p2.ID {
		t.Fatalf("expected distinct non-empty IDs, got %q and %q", p1.ID, p2.ID)
	}
	origID := p1.ID
	if lib.PageByID(origID) != p1 {
		t.Fatalf("PageByID(%q) = %v, want %v", origID, lib.PageByID(origID), p1)
	}
	if lib.PageByID("") != nil || lib.PageByID("missing") != nil {
		t.Fatal("expected nil for empty or unknown PageID")
	}

	// Reserved "id" property cannot be overwritten via SetValue or AddProperty.
	notes.Properties = append(notes.Properties, Property{ID: "id", Name: "id", Type: PropText})
	if err := p1.SetValue("id", "custom-value"); err == nil {
		t.Fatal("expected SetValue(\"id\", ...) to be rejected")
	}

	// Move across collections and reload from disk; ID stays identical while Dir changes.
	oldDir := p1.Dir
	if err := lib.MovePage(p1, other, nil, 0); err != nil {
		t.Fatal(err)
	}
	if p1.Dir == oldDir || p1.ID != origID {
		t.Fatalf("after MovePage: dir %q (was %q), id %q (want %q)", p1.Dir, oldDir, p1.ID, origID)
	}
	reloaded, err := OpenLibrary(lib.Root)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.PageByID(origID)
	if got == nil || got.Title != "First" || got.Dir != p1.Dir {
		t.Fatalf("reloaded PageByID(%q) = %+v", origID, got)
	}
}
