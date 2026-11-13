package library

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The page, tags and file properties: tags are trimmed and each kept once, a page is its folders from the root and
// found again when it moved, a file is a name in the page's folder and nothing else; a filter on tags finds one
// whole tag.
func TestPropTypes(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	notes, _ := lib.NewCollection("Notes")
	tags, _ := books.AddProperty("Tags", PropTags, nil)
	about, _ := books.AddProperty("About", PropPage, nil)
	file, _ := books.AddProperty("File", PropFile, nil)
	dune, _ := books.NewPage(nil, "Dune")
	idea, _ := notes.NewPage(nil, "Desert idea")
	if err := dune.SetValue(tags.ID, " sci-fi, Classic ,  , SCI-FI,desert "); err != nil {
		t.Fatal(err)
	}
	if got := dune.Props[tags.ID]; got != "sci-fi, Classic, desert" {
		t.Fatalf("tags: %q", got)
	}
	if err := dune.SetValue(about.ID, "/"+idea.Path()+"/"); err != nil {
		t.Fatal(err)
	}
	if got := dune.Props[about.ID]; got != idea.Path() || lib.PageAt(got) != idea {
		t.Fatalf("page: %q -> %v", got, lib.PageAt(got))
	}
	if err := os.Rename(idea.Dir, filepath.Join(filepath.Dir(idea.Dir), "..", "books", filepath.Base(idea.Dir))); err != nil {
		t.Fatal(err)
	}
	again, _ := OpenLibrary(lib.Root)
	if p := again.PageAt(dune.Props[about.ID]); p == nil || p.Title != "Desert idea" {
		t.Fatalf("the moved page is not found by its folder's name: %v", p)
	}
	for _, bad := range []string{"../x.pdf", "a/b.pdf", ".."} {
		if err := dune.SetValue(file.ID, bad); err == nil {
			t.Errorf("%q taken as a file of the page", bad)
		}
	}
	if err := dune.SetValue(file.ID, "plan.pdf"); err != nil {
		t.Fatal(err)
	}
	for op, want := range map[string]bool{"classic": true, "class": false} {
		v := View{Filters: []Filter{{Prop: tags.ID, Op: "contains", Value: op}}}
		if v.Match(books, dune) != want {
			t.Errorf("tag filter %q: %v", op, !want)
		}
	}
}

// TestDefaultViewsEnglish: a new collection's views are named in English, the project's language; the app
// shows them in the window's (#490).
func TestDefaultViewsEnglish(t *testing.T) {
	var got []string
	for _, v := range defaultViews() {
		got = append(got, v.Label)
	}
	if fmt.Sprint(got) != "[List Table Calendar]" {
		t.Fatalf("default views %v", got)
	}
}
