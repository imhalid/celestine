package main

import (
	"image/png"
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
	if len(one.Properties) != 0 || one.View().Type == "board" || len(one.Views) != 3 {
		t.Fatalf("a new collection: %+v %+v", one.Properties, one.Views)
	}
	src, _ := os.ReadFile(ps["A"].File())
	if strings.Contains(string(src), "status") {
		t.Fatalf("a value nobody asked for:\n%s", src)
	}
	if _, err := one.AddProperty("Yazar", PropText, nil); err != nil {
		t.Fatal(err)
	}
	if len(one.Views) != 3 {
		t.Fatal("a board without a choice")
	}
	pr, err := one.AddProperty("Durum", PropChoice, []string{"Yapılacak", " Sürüyor", "Bitti", "Bitti"})
	if err != nil || !equal(pr.Options, []string{"Yapılacak", "Sürüyor", "Bitti"}) {
		t.Fatalf("choice: %v %+v", err, pr)
	}
	if one.Views[3].Type != "board" || one.Views[3].GroupBy != pr.ID {
		t.Fatalf("no board of it: %+v", one.Views)
	}
	if _, err := one.AddProperty("durum", PropText, nil); err == nil {
		t.Fatal("two properties of one name")
	}
	// values
	writePageFile(t, ps["A"].File(), map[string]string{"title": "A", "kaynak": "elle yazıldı"}, "metin\n")
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
	src, _ = os.ReadFile(a.File())
	if m := PageMeta(src); m["yazar"] != "Herbert" || m["durum"] != "Sürüyor" || m["kaynak"] != "elle yazıldı" {
		t.Fatalf("on disk:\n%s", src)
	}
	if body, _ := a.Text(); body != "metin\n" {
		t.Fatalf("the text: %q", body)
	}
	// taken away: no board, the values stay in the file
	if err := c.RemoveProperty("durum"); err != nil {
		t.Fatal(err)
	}
	if len(c.Views) != 3 {
		t.Fatalf("a board of nothing: %+v", c.Views)
	}
}

// TestStatusesMigrate: a collection of the time before properties gets a
// "Durum" choice of its statuses.
func TestStatusesMigrate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "eski")
	os.MkdirAll(filepath.Join(dir, "a"), 0o755)
	os.WriteFile(filepath.Join(dir, CollectionFile), []byte(`{"name":"Eski","subPages":true,"statuses":["Açık","Kapalı"],"views":[{"id":"list","label":"Liste","type":"list"},{"id":"board","label":"Pano","type":"board"}]}`), 0o644)
	writePageFile(t, filepath.Join(dir, "a", PageFileName), map[string]string{"title": "A", "status": "Kapalı"}, "")
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
	data, _ := os.ReadFile(filepath.Join(dir, CollectionFile))
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

// TestPropertyOnPage adds a choice from the page, to that page alone (#487), and gives the page a value.
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
	if err := tt.Click("Seçenek ekle"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Type("Yapılacak,Sürüyor,Bitti,")
	if err := tt.Click("Ekle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	coll := a.page.Coll
	if pr := a.page.Prop("durum"); pr == nil || len(pr.Options) != 3 || a.prop.open || coll.Prop("durum") != nil {
		t.Fatalf("added: %+v, the collection's %+v", a.page.Own, coll.Properties)
	}
	if err := tt.Click("Durum değeri"); err != nil {
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
	if body, _ := a.page.Text(); body != "gövde\n" {
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

// TestBoardCardMoveMenu: a card's menu lists the groups; the one chosen takes
// the card, and the group it is in is marked.
func TestBoardCardMoveMenu(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	tt.Frame()
	if err := tt.Click("Pano"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.RightClick("Kart: Dune"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Grubu değiştir"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Durum yok") || !tt.HasText("şimdi burada") {
		t.Fatalf("the groups: %v", tt.Texts())
	}
	// the names are in the board too: the menu's own item is chosen
	if err := tt.ChooseMenuItem("Grubu değiştir", "Bitti"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(300 * time.Millisecond)
	if got := titles(coll.Column("durum", "Bitti")); !equal(got, []string{"Dune"}) {
		t.Fatalf("column %v", got)
	}
	if a.coll != coll {
		t.Fatal("the menu opened the page")
	}
}

// A board's cards come in three sizes: small, with no cover and one line of title; the middle; and large, wider, every
// card showing what it says.
func TestBoardCardSize(t *testing.T) {
	_, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Bitti"})
	tt.Frame()
	if err := tt.Click("Pano"); err != nil {
		t.Fatal(err)
	}
	width := func(size string) float32 {
		t.Helper()
		coll.View().CardSize = size
		tt.Frame()
		tt.Advance(time.Second)
		r, ok := tt.Find("Kart: Dune")
		if !ok {
			t.Fatalf("no card: %v", tt.Texts())
		}
		return r.W
	}
	small, mid, large := width("small"), width(""), width("large")
	if !(small < mid && mid < large) {
		t.Fatalf("cards: small %v, middle %v, large %v", small, mid, large)
	}
	if !tt.HasText("Çöl gezegeni Arrakis.") {
		t.Fatal("a large card does not show the start of its text")
	}
}

// TestNumberPropertyRefusesLetters: a number takes digits, a sign and one decimal mark as it is typed, and nothing
// else (#487); a choice's value is drawn as a badge. SHOT=file.png draws the properties.
func TestNumberPropertyRefusesLetters(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	p := a.page
	if _, err := p.AddProperty("Sayfa sayısı", PropNumber, nil); err != nil {
		t.Fatal(err)
	}
	mood, _ := p.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Bitti"})
	p.SetValue(mood.ID, "Bitti")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Sayfa sayısı değeri"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Type("rr1a1,5,x")
	tt.Frame()
	if got := *a.propBuf[p.Dir+"|sayfa-sayisi"]; got != "11,5" {
		t.Fatalf("typed: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.Props["sayfa-sayisi"] != "11,5" || a.propErr != "" {
		t.Fatalf("value %q, error %q", p.Props["sayfa-sayisi"], a.propErr)
	}
	if file := os.Getenv("SHOT"); file != "" {
		f, _ := os.Create(file)
		png.Encode(f, tt.Image())
		f.Close()
	}
}
