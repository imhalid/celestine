package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// viewsLibrary is a collection of pages made and changed at known times.
func viewsLibrary(t *testing.T) (*app, *ui.Tester, *Collection) {
	t.Helper()
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	coll, _ := lib.NewCollection("Kitaplar")
	now := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local)
	mk := func(parent *Page, title, body string, made, changed time.Time) *Page {
		p, err := coll.NewPage(parent, title)
		if err != nil {
			t.Fatal(err)
		}
		p.Created = made
		if err := p.Save(body); err != nil {
			t.Fatal(err)
		}
		p.Modified = changed
		return p
	}
	day := 24 * time.Hour
	dune := mk(nil, "Dune", "Çöl gezegeni **Arrakis**.", now.Add(-40*day), now.Add(-2*time.Hour))
	mk(dune, "Notlar: Dune", "Baharat her şeydir.", now.Add(-30*day), now.Add(-26*time.Hour))
	mk(nil, "Anna Karenina", "Mutlu aileler birbirine benzer.", now.Add(-10*day), now.Add(-5*day))
	mk(nil, "Yüzyıllık Yalnızlık", "Albay Aureliano Buendía.", now.Add(-400*day), now.Add(-90*day))
	a := newApp(lib)
	a.clock = func() time.Time { return now }
	tt := ui.NewTester(a.view, 1180, 900)
	if err := tt.Click("Koleksiyon: Kitaplar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.coll != coll {
		t.Fatal("clicking the collection does not open its page")
	}
	return a, tt, coll
}

func titles(ps []*Page) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Title)
	}
	return out
}

func TestListView(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if v := coll.View(); v.Type != "list" || v.Sort != "modified" || !v.Desc {
		t.Fatalf("default view: %+v", v)
	}
	pages := a.filtered(coll)
	sortPages(pages, coll.View())
	want := []string{"Dune", "Notlar: Dune", "Anna Karenina", "Yüzyıllık Yalnızlık"}
	if got := titles(pages); !equal(got, want) {
		t.Fatalf("newest first: %v", got)
	}
	// A page opens from the list, and a sub-page is in it with its parent.
	if err := tt.Click("Aç: Notlar: Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.coll != nil || a.page.Title != "Notlar: Dune" {
		t.Fatalf("open from the list: coll %v page %v", a.coll, a.page)
	}
}

func TestSearch(t *testing.T) {
	a, _, coll := viewsLibrary(t)
	a.search = "baharat" // in the text, not the title
	if got := titles(a.filtered(coll)); !equal(got, []string{"Notlar: Dune"}) {
		t.Fatalf("search: %v", got)
	}
	a.search = "KARENINA"
	if got := titles(a.filtered(coll)); !equal(got, []string{"Anna Karenina"}) {
		t.Fatalf("search ignores case: %v", got)
	}
}

func TestTableViewSortsAndRemembers(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Tablo"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if coll.View().Type != "table" {
		t.Fatal("the table is not shown")
	}
	if err := tt.Click("Oluşturulma"); err != nil { // Ryugo's table: the first click sorts up, oldest first
		t.Fatal(err)
	}
	tt.Frame()
	if v := coll.View(); v.Sort != "created" || v.Desc {
		t.Fatalf("first click: %+v", v)
	}
	pages := a.filtered(coll)
	sortPages(pages, coll.View())
	if got := titles(pages); !equal(got, []string{"Yüzyıllık Yalnızlık", "Dune", "Notlar: Dune", "Anna Karenina"}) {
		t.Fatalf("by created, oldest first: %v", got)
	}
	if err := tt.Click("Oluşturulma"); err != nil { // again: the other way
		t.Fatal(err)
	}
	tt.Frame()
	if v := coll.View(); !v.Desc || v.Sort != "created" {
		t.Fatalf("second click: %+v", v)
	}
	// The choice is in collection.json.
	data, _ := os.ReadFile(filepath.Join(coll.Dir, collectionFile))
	var saved Collection
	json.Unmarshal(data, &saved)
	if saved.ActiveView != "table" || saved.Views[1].Sort != "created" || !saved.Views[1].Desc {
		t.Fatalf("collection.json: %s", data)
	}
	again, _ := OpenLibrary(a.lib.Root)
	if v := again.Collections[0].View(); v.ID != "table" || v.Sort != "created" {
		t.Fatalf("after reopening: %+v", v)
	}
}

func TestNewPageFromCollection(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Yeni sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.coll != nil || a.page == nil || a.page.Title != "Adsız" || len(coll.Pages) != 4 {
		t.Fatalf("new page: coll %v page %+v", a.coll, a.page)
	}
}

func TestTimeWords(t *testing.T) {
	now := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local)
	cases := []struct {
		t          time.Time
		ago, group string
	}{
		{now.Add(-30 * time.Second), "şimdi", "Bugün"},
		{now.Add(-5 * time.Minute), "5 dk önce", "Bugün"},
		{now.Add(-3 * time.Hour), "3 sa önce", "Bugün"},
		{now.Add(-20 * time.Hour), "dün", "Dün"},
		{now.Add(-4 * 24 * time.Hour), "4 gün önce", "Son 7 gün"},
		{time.Date(2026, 10, 10, 9, 0, 0, 0, time.Local), "10 Eki 2026", "Son 30 gün"},
		{time.Date(2026, 3, 2, 9, 0, 0, 0, time.Local), "2 Mar 2026", "Mar 2026"},
		{time.Date(2024, 3, 2, 9, 0, 0, 0, time.Local), "2 Mar 2024", "2024"},
	}
	for _, c := range cases {
		if got := ago(c.t, now); got != c.ago {
			t.Errorf("ago(%v) = %q, want %q", c.t, got, c.ago)
		}
		if got := dateGroup(c.t, now); got != c.group {
			t.Errorf("dateGroup(%v) = %q, want %q", c.t, got, c.group)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A table's columns are as wide in every row as in the header, whatever the
// titles: the values stand under their names. (The rows once took the width of
// their own contents, and the columns after the title drifted by its length.)
func TestTableColumnsAlign(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Gezi")
	pr, _ := c.AddProperty("Ülke", "text", nil)
	for title, ulke := range map[string]string{"Edirne": "Türkiye", "Bo": "Bulgaristan", "Kapadokya ve çevresindeki vadiler": "Gürcistan", "Prag": "Çekya"} {
		p, _ := c.NewPage(nil, title)
		p.SetValue(pr.ID, ulke)
	}
	for _, v := range c.Views {
		if v.Type == "table" {
			c.SetView(v.ID)
		}
	}
	for _, extra := range []string{"Bütçe", "Ziyaret", "Not"} {
		c.AddProperty(extra, "text", nil)
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 900, 760)
	a.do(func() error { a.openCollection(c); return nil })
	tt.Advance(600 * time.Millisecond)
	head, ok := tt.Find("Ülke")
	if !ok {
		t.Fatalf("no header: %v", tt.Texts())
	}
	first := float32(-1)
	for _, v := range []string{"Türkiye", "Bulgaristan", "Gürcistan", "Çekya"} {
		cell, ok := tt.Find(v)
		if !ok {
			t.Fatalf("no cell %q: %v", v, tt.Texts())
		}
		if first < 0 {
			first = cell.X
		}
		if math.Abs(float64(cell.X-first)) > 1 {
			t.Errorf("%q starts at x=%v, the others at %v", v, cell.X, first)
		}
		if math.Abs(float64(cell.X-head.X)) > 20 { // the header's own inset
			t.Errorf("%q starts at x=%v, its column's header at %v", v, cell.X, head.X)
		}
	}
}

// In the table a click on a row opens its page, and while rows are being chosen
// it chooses it instead; the boxes of the table are the app's choice too.
func TestTableRowOpensOrChooses(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.Open = false // the sidebar does not list the pages: the table has the only rows with these titles
	if err := tt.Click("Tablo"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	a.bulk.on = true
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Anna Karenina"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.coll == nil || len(a.bulk.sel) != 1 {
		t.Fatalf("a click on a row while choosing: coll %v, chosen %d", a.coll, len(a.bulk.sel))
	}
	for p := range a.bulk.sel {
		if p.Title != "Anna Karenina" {
			t.Fatalf("chose %q", p.Title)
		}
	}
	a.bulkClear()
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Anna Karenina"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.coll != nil || a.page == nil || a.page.Title != "Anna Karenina" {
		t.Fatalf("a click on a row: coll %v page %v (of %v)", a.coll, a.page, coll.Name)
	}
}
