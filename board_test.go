package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPropertiesOptIn: a collection has no properties until one is added,
// and no board until it has a choice; values are on disk, front matter we
// don't know is kept.
func TestPropertiesOptIn(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	if len(one.Properties) != 0 || one.View().Type == "board" || len(one.Views) != 2 {
		t.Fatalf("a new collection: %+v %+v", one.Properties, one.Views)
	}
	src, _ := os.ReadFile(ps["A"].file())
	if strings.Contains(string(src), "status") {
		t.Fatalf("a value nobody asked for:\n%s", src)
	}
	if _, err := one.AddProperty("Yazar", PropText, nil); err != nil {
		t.Fatal(err)
	}
	if len(one.Views) != 2 {
		t.Fatal("a board without a choice")
	}
	pr, err := one.AddProperty("Durum", PropChoice, []string{"Yapılacak", " Sürüyor", "Bitti", "Bitti"})
	if err != nil || !equal(pr.Options, []string{"Yapılacak", "Sürüyor", "Bitti"}) {
		t.Fatalf("choice: %v %+v", err, pr)
	}
	if one.Views[2].Type != "board" || one.Views[2].GroupBy != pr.ID {
		t.Fatalf("no board of it: %+v", one.Views)
	}
	if _, err := one.AddProperty("durum", PropText, nil); err == nil {
		t.Fatal("two properties of one name")
	}
	// values
	os.WriteFile(ps["A"].file(), []byte("---\ntitle: A\nkaynak: elle yazıldı\n---\n\nmetin\n"), 0o644)
	again, _ := OpenLibrary(lib.Root)
	var c *Collection
	for _, x := range again.Collections {
		if x.Name == "Bir" {
			c = x
		}
	}
	var a *Page
	for _, p := range c.Pages {
		if p.Title == "A" {
			a = p
		}
	}
	if err := a.SetValue("yazar", "Herbert"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetValue("durum", "sürüyor"); err != nil { // any case
		t.Fatal(err)
	}
	if err := a.SetValue("durum", "Yok böyle"); err == nil {
		t.Fatal("a value not among the options")
	}
	src, _ = os.ReadFile(a.file())
	if !strings.Contains(string(src), "yazar: Herbert\ndurum: Sürüyor\n") || !strings.Contains(string(src), "kaynak: elle yazıldı") {
		t.Fatalf("on disk:\n%s", src)
	}
	if body, _ := a.Body(); body != "metin\n" {
		t.Fatalf("the text: %q", body)
	}
	// taken away: no board, the values stay in the file
	if err := c.RemoveProperty("durum"); err != nil {
		t.Fatal(err)
	}
	if len(c.Views) != 2 {
		t.Fatalf("a board of nothing: %+v", c.Views)
	}
}

// TestStatusesMigrate: a collection of the time before properties gets a
// "Durum" choice of its statuses.
func TestStatusesMigrate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "eski")
	os.MkdirAll(filepath.Join(dir, "a"), 0o755)
	os.WriteFile(filepath.Join(dir, collectionFile), []byte(`{"name":"Eski","subPages":true,"statuses":["Açık","Kapalı"],"views":[{"id":"list","label":"Liste","type":"list"},{"id":"board","label":"Pano","type":"board"}]}`), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "index.md"), []byte("---\ntitle: A\nstatus: Kapalı\n---\n"), 0o644)
	lib, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	c := lib.Collections[0]
	pr := c.Prop("status")
	if pr == nil || pr.Name != "Durum" || !equal(pr.Options, []string{"Açık", "Kapalı"}) || c.Statuses != nil {
		t.Fatalf("migrated: %+v", c.Properties)
	}
	if c.Pages[0].Value(pr) != "Kapalı" || c.Views[1].GroupBy != "status" {
		t.Fatalf("value %q, views %+v", c.Pages[0].Value(pr), c.Views)
	}
	data, _ := os.ReadFile(filepath.Join(dir, collectionFile))
	if strings.Contains(string(data), "statuses") {
		t.Fatalf("not written back: %s", data)
	}
}

// TestBoardOnDisk: a page moved on the board keeps its value and place
// across a reopening; a page without a value is in the first column.
func TestBoardOnDisk(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	one.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	if got := titles(one.Column("durum", "")); !equal(got, []string{"A", "B", "B1", "C"}) {
		t.Fatalf("no value: %v", got)
	}
	one.MoveOnBoard(ps["C"], "durum", "Sürüyor", 0)
	one.MoveOnBoard(ps["A"], "durum", "Sürüyor", 0)
	one.MoveOnBoard(ps["B1"], "durum", "", 0)
	again, _ := OpenLibrary(lib.Root)
	var c *Collection
	for _, x := range again.Collections {
		if x.Name == "Bir" {
			c = x
		}
	}
	if got := titles(c.Column("durum", "Sürüyor")); !equal(got, []string{"A", "C"}) {
		t.Fatalf("under way: %v", got)
	}
	if got := titles(c.Column("durum", "")); !equal(got, []string{"B1", "B"}) {
		t.Fatalf("no value: %v", got)
	}
}

// boardLibrary is viewsLibrary's collection with a "Durum" choice.
func TestBoardDrag(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	tt.Frame()
	if err := tt.Click("Pano"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	from, ok := tt.Find("Kart: Dune")
	if !ok {
		t.Fatalf("no card: %v", tt.Texts())
	}
	to, ok := tt.Find("Sürüyor")
	if !ok {
		t.Fatal("no column")
	}
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1, y1 := to.X+40, to.Y+60
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+(x1-x0)*float32(k)/10, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Advance(time.Second)
	if got := titles(coll.Column("durum", "Sürüyor")); !equal(got, []string{"Dune"}) {
		t.Fatalf("column %v", got)
	}
	if a.coll != coll {
		t.Fatal("the drop opened the page")
	}
	if err := tt.Click("Kart: Anna Karenina"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page == nil || a.page.Title != "Anna Karenina" || a.coll != nil {
		t.Fatal("a click on a card does not open its page")
	}
}

// TestPropertyOnPage adds a choice from the page and gives the page a value.
func TestPropertyOnPage(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	if tt.HasText("Durum") {
		t.Fatal("a property nobody added")
	}
	if err := tt.Click("Özellik ekle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Type("Durum")
	if err := tt.Click("Seçenek listesi"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Type("Yapılacak, Sürüyor, Bitti")
	if err := tt.Click("Ekle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	coll := a.page.Coll
	if pr := coll.Prop("durum"); pr == nil || len(pr.Options) != 3 || a.prop.open {
		t.Fatalf("added: %+v", coll.Properties)
	}
	if err := tt.Click("—"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Bitti"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if a.page.Props["durum"] != "Bitti" {
		t.Fatalf("value %q", a.page.Props["durum"])
	}
	if body, _ := a.page.Body(); body != "gövde\n" {
		t.Fatalf("the text after the value: %q", body)
	}
	// a text property, typed and left
	coll.AddProperty("Yazar", PropText, nil)
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Yazar değeri"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Type("Herbert")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.page.Props["yazar"] != "Herbert" {
		t.Fatalf("text %q", a.page.Props["yazar"])
	}
}
