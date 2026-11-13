package library

import "testing"

// TestOwnProperties: a property added on a page is that page's alone, read back from its file with its value; a
// choice of its own has no board to move on; taken away, its value goes with it; the collection's are every page's
// (#487).
func TestOwnProperties(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	books, _ := lib.NewCollection("Books")
	shared, _ := books.AddProperty("Author", PropText, nil)
	dune, _ := books.NewPage(nil, "Dune")
	emma, _ := books.NewPage(nil, "Emma")
	mood, err := dune.AddProperty("Mood", PropChoice, []string{"Calm", "Dark"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dune.AddProperty("author", PropText, nil); err == nil {
		t.Fatal("a page's own property took the name of its collection's")
	}
	if len(books.Properties) != 1 || len(emma.Properties()) != 1 || len(dune.Properties()) != 2 {
		t.Fatalf("the page's property went elsewhere: %d in the collection, %d on Emma", len(books.Properties), len(emma.Properties()))
	}
	if err := dune.SetValue(mood.ID, "dark"); err != nil {
		t.Fatal(err)
	}
	if err := dune.SetValue(shared.ID, "Herbert"); err != nil {
		t.Fatal(err)
	}
	if dune.Rank != 0 || dune.Props[mood.ID] != "Dark" {
		t.Fatalf("own choice: rank %d, %q", dune.Rank, dune.Props[mood.ID])
	}
	if err := emma.SetValue(mood.ID, "Calm"); err == nil {
		t.Fatal("another page took a value of a property it doesn't have")
	}
	again, _ := OpenLibrary(lib.Root)
	d := again.Collections[0].Pages[0]
	if d.Title != "Dune" {
		d = again.Collections[0].Pages[1]
	}
	if pr := d.PropNamed("Mood"); pr == nil || !d.IsOwn(pr.ID) || d.Value(pr) != "Dark" || len(pr.Options) != 2 {
		t.Fatalf("read back: %+v %v", pr, d.Props)
	}
	if _, kept := d.Props["properties"]; kept {
		t.Fatal("the own properties' meta was taken for a value")
	}
	if err := d.EditProperty(mood.ID, "Feel", nil); err != nil || d.Prop(mood.ID).Name != "Feel" {
		t.Fatalf("rename: %v", err)
	}
	if err := d.RemoveProperty(mood.ID); err != nil {
		t.Fatal(err)
	}
	if d.Prop(mood.ID) != nil || d.Props[mood.ID] != "" || d.Props[shared.ID] != "Herbert" {
		t.Fatalf("removed: %v", d.Props)
	}
	// a property the collection gets later takes no id a page has of its own
	own, _ := d.AddProperty("Year", PropNumber, nil)
	if pr, _ := again.Collections[0].AddProperty("year ", PropText, nil); pr == nil || pr.ID == own.ID {
		t.Fatalf("the collection's new property took the page's id: %+v", pr)
	}
}
