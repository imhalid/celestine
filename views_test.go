package main

import (
	"encoding/json"
	"fmt"
	"github.com/imhalid/celestine/locales"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
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
		if err := p.SaveText(body); err != nil {
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
	if err := tt.Click(locales.ViewsOpenLabel("Notlar: Dune")); err != nil {
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
	if err := tt.Click(msg("slash_table")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if coll.View().Type != "table" {
		t.Fatal("the table is not shown")
	}
	if err := tt.Click(msg("common_created")); err != nil { // Ryugo's table: the first click sorts up, oldest first
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
	data, _ := os.ReadFile(filepath.Join(coll.Dir, CollectionFile))
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

// Relative times, dates and a list's headings are in the window's language (#302).
func TestTimeWords(t *testing.T) {
	withLang(t, "tr")
	now := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local)
	cases := []struct {
		t          time.Time
		ago, group string
	}{
		{now.Add(-30 * time.Second), "şimdi", "Bugün"},
		{now.Add(-5 * time.Minute), "5 dk. önce", "Bugün"},
		{now.Add(-3 * time.Hour), "3 sa. önce", "Bugün"},
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

// Views with nothing to show say so with Ryugo's empty state, and what to do.
func TestEmptyStates(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Boş")
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 760)
	a.do(func() error { a.openCollection(c); return nil })
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("Bu koleksiyon boş") {
		t.Fatalf("an empty collection: %v", tt.Texts())
	}
	a.search = "yokböyle"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Eşleşen sayfa yok") || !tt.HasText("Aramayı temizle") {
		t.Fatalf("a search with no hits: %v", tt.Texts())
	}
	if err := tt.Click("Aramayı temizle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.search != "" || !tt.HasText("Bu koleksiyon boş") {
		t.Fatalf("search not cleared: %q", a.search)
	}
	a.coll, a.searchOpen = nil, true
	a.searchQ = ""
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Kitaplıkta ara") {
		t.Fatalf("the search page with no words: %v", tt.Texts())
	}
	a.searchQ = "hiçbiryerdeyok"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Eşleşen sayfa yok") {
		t.Fatalf("a search of the library with no hits: %v", tt.Texts())
	}
}

// A gallery shows the pages as cards that wrap, each with its title and the start of its text; a card opens its page.
func TestGalleryView(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.Views = append(coll.Views, View{ID: "gal", Label: "Galeri", Type: "gallery", Sort: "title"})
	if err := coll.SetView("gal"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	for _, want := range []string{"Dune", "Anna Karenina", "Yüzyıllık Yalnızlık", "Mutlu aileler birbirine benzer."} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in the gallery: %v", want, tt.Texts())
		}
	}
	dune, _ := tt.Find(locales.ViewsOpenLabel("Dune"))
	anna, _ := tt.Find(locales.ViewsOpenLabel("Anna Karenina"))
	if dune.W != galleryCard || anna.Y != dune.Y || anna.X == dune.X {
		t.Fatalf("the cards are not side by side: %+v %+v", dune, anna)
	}
	if out := os.Getenv("GALSHOT"); out != "" {
		f, _ := os.Create(out)
		png.Encode(f, tt.Image())
		f.Close()
	}
	if err := tt.Click(locales.ViewsOpenLabel("Anna Karenina")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page == nil || a.page.Title != "Anna Karenina" {
		t.Fatalf("the card did not open its page: %v", a.page)
	}
}

// A list grouped by a choice shows its pages under the choice's options, in their order, those with none last; a
// heading folds its group, and the fold is kept in the view. A list by a choice that is gone is by date again.
func TestListGroupedByChoice(t *testing.T) {
	_, tt, coll := viewsLibrary(t)
	pr, err := coll.AddProperty("Durum", PropChoice, []string{"Okunacak", "Okundu"})
	if err != nil {
		t.Fatal(err)
	}
	for title, s := range map[string]string{"Dune": "Okundu", "Anna Karenina": "Okunacak"} {
		for _, p := range coll.All() {
			if p.Title == title {
				if err := p.SetValue(pr.ID, s); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	coll.View().GroupBy = pr.ID
	tt.Advance(400 * time.Millisecond)
	y := func(label string) float32 {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q: %v", label, tt.Texts())
		}
		return r.Y
	}
	if !(y(locales.ViewsCollapse()+"Okunacak") < y(locales.ViewsOpenLabel("Anna Karenina")) && y(locales.ViewsOpenLabel("Anna Karenina")) < y(locales.ViewsCollapse()+"Okundu") &&
		y(locales.ViewsCollapse()+"Okundu") < y(locales.ViewsOpenLabel("Dune")) && y(locales.ViewsOpenLabel("Dune")) < y(locales.ViewsCollapse()+locales.ViewsNoValue("Durum")) && y(locales.ViewsCollapse()+locales.ViewsNoValue("Durum")) < y(locales.ViewsOpenLabel("Yüzyıllık Yalnızlık"))) {
		t.Fatalf("the groups are not in the options' order: %v", tt.Texts())
	}
	if err := tt.Click(locales.ViewsCollapse() + "Okundu"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find(locales.ViewsOpenLabel("Dune")); ok || !tt.HasText("Okundu") {
		t.Fatal("the folded group still shows its pages")
	}
	if !slices.Contains(coll.View().Collapsed, "Okundu") {
		t.Fatalf("the fold is not kept: %v", coll.View().Collapsed)
	}
	if err := coll.RemoveProperty(pr.ID); err != nil {
		t.Fatal(err)
	}
	if g := coll.View().GroupBy; g != "" {
		t.Fatalf("still grouped by a property that is gone: %q", g)
	}
}

// A table's columns stay as the user leaves them: a width dragged, a column moved or hidden goes into the view and is
// saved once still (not on each frame of the drag), and comes back on the table of a library opened again.
func TestTableColumnsRemembered(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Tablo"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	v := coll.View()
	tu := a.tableOf(coll, v)
	tu.st.Widths = map[string]float32{"created": 220}
	tu.st.Order = []string{"modified", "created"}
	tu.st.Hidden = map[string]bool{"parent": true}
	tt.Frame()
	saved := func() View {
		data, _ := os.ReadFile(filepath.Join(coll.Dir, CollectionFile))
		var c Collection
		json.Unmarshal(data, &c)
		for _, sv := range c.Views {
			if sv.ID == v.ID {
				return sv
			}
		}
		return View{}
	}
	if v.Widths["created"] != 220 || saved().Widths != nil {
		t.Fatalf("taken into the view %v, or saved at once %v", v.Widths, saved().Widths)
	}
	tt.Advance(layoutSettle + 200*time.Millisecond)
	sv := saved()
	if sv.Widths["created"] != 220 || !slices.Equal(sv.Order, []string{"modified", "created"}) || !slices.Equal(sv.Hidden, []string{"parent"}) {
		t.Fatalf("not saved once still: %+v", sv)
	}
	again, _ := OpenLibrary(a.lib.Root)
	b := newApp(again)
	ac := again.Collections[0]
	tt2 := ui.NewTester(b.view, 1180, 900)
	b.openCollection(ac)
	tt2.Frame()
	st := b.tableOf(ac, ac.View()).st
	if st.Widths["created"] != 220 || !st.Hidden["parent"] || !slices.Equal(st.Order, []string{"modified", "created"}) {
		t.Fatalf("the table opened again: %+v", st)
	}
}

// A view's filters: the view shows only the pages that meet them, its toolbar says how many there are and opens them
// in its settings, where one is added; a view whose filters leave nothing says so.
func TestViewFiltersShown(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	v := coll.View()
	v.Filters = []Filter{{Prop: "title", Op: "contains", Value: "dune"}}
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find(locales.ViewsOpenLabel("Anna Karenina")); ok || !tt.HasText("Dune") || !tt.HasText("Notlar: Dune") {
		t.Fatalf("the filter does not hold: %v", tt.Texts())
	}
	v.Filters = []Filter{{Prop: "title", Op: "is", Value: "nothing"}}
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText(locales.ViewsNoPageMeetsThis()) {
		t.Fatalf("no word that the filters leave nothing: %v", tt.Texts())
	}
	v.Filters = []Filter{{Prop: "title", Op: "contains", Value: "dune"}}
	tt.Advance(400 * time.Millisecond)
	if err := tt.Click(locales.ViewsN1Filter()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if !a.cs.open || a.cs.tab != 2 || a.cs.view != v.ID || !tt.HasText(locales.CollectionAddFilter()) {
		t.Fatalf("the view's settings are not open at its filters: open %v tab %d view %q", a.cs.open, a.cs.tab, a.cs.view)
	}
	if err := tt.Click(locales.CollectionAddFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d := a.cs.draft.View(); len(d.Filters) != 2 {
		t.Fatalf("no filter added to the draft: %+v", d.Filters)
	}
	if out := os.Getenv("FILTERSHOT"); out != "" {
		tt.Advance(400 * time.Millisecond)
		f, _ := os.Create(out)
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// A list in the collection's own order: a row dragged to the top stays there, in the pages' files; a row's arrow
// opens its page.
func TestManualListDrag(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	coll.View().Sort = "manual"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	order := func() []string {
		ps := coll.All()
		sortPages(ps, coll.View())
		return titles(ps)
	}
	before := order()
	last := before[len(before)-1]
	from, ok := tt.Find("Sırala: " + last)
	if !ok {
		t.Fatalf("no row for %s: %v", last, tt.Texts())
	}
	to, _ := tt.Find("Sırala: " + before[0])
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1, y1 := to.X+to.W/2, to.Y+4
	tt.Press(x0, y0)
	for k := 1; k <= 12; k++ {
		tt.Move(x0+(x1-x0)*float32(k)/12, y0+(y1-y0)*float32(k)/12)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Advance(time.Second)
	if got := order(); got[0] != last {
		t.Fatalf("not first after the drag: %v (was %v)", got, before)
	}
	again, _ := OpenLibrary(a.lib.Root)
	ps := again.Collections[0].All()
	slices.SortStableFunc(ps, CompareManual)
	if ps[0].Title != last {
		t.Fatalf("not kept: %v", titles(ps))
	}
	if err := tt.Click(locales.ViewsOpenLabel(before[0])); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page == nil || a.page.Title != before[0] || a.coll != nil {
		t.Fatal("a row's arrow does not open its page")
	}
}

// A table's foot says what the view asks under its columns; grouped by a choice its rows are under the options'
// headings, a heading folds its group and the fold is kept; in its own order a row dragged by its grip stays where
// it was let go.
func TestTableFootGroupsAndOrder(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	pr, _ := coll.AddProperty("Durum", PropChoice, []string{"Okunacak", "Okundu"})
	pages, _ := coll.AddProperty("Sayfa sayısı", PropNumber, nil)
	vals := map[string][2]string{"Dune": {"Okundu", "412"}, "Anna Karenina": {"Okunacak", "864"}, "Yüzyıllık Yalnızlık": {"Okundu", "417"}}
	for _, p := range coll.All() {
		if v, ok := vals[p.Title]; ok {
			p.SetValue(pr.ID, v[0])
			p.SetValue(pages.ID, v[1])
		}
	}
	if err := tt.Click("Tablo"); err != nil {
		t.Fatal(err)
	}
	v := coll.View()
	v.Columns = []string{"prop:" + pr.ID, "prop:" + pages.ID}
	v.Calcs = map[string]string{"title": "count", "prop:" + pages.ID: "sum"}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Adet 4") || !tt.HasText("Toplam 1.693") && !tt.HasText("Toplam 1693") {
		t.Fatalf("the foot: %v", tt.Texts())
	}
	v.GroupBy = pr.ID
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	// in the table (after its last header), as the texts come: each group's heading, then its rows
	texts := tt.Texts()
	from := slices.Index(texts, "Sayfa sayısı")
	want := []string{"Okunacak", "Anna Karenina", "Okundu", "Dune", "Yüzyıllık Yalnızlık", locales.ViewsNoValue("Durum"), "Notlar: Dune"}
	at := 0
	for _, s := range texts[max(from, 0):] {
		if at < len(want) && s == want[at] {
			at++
		}
	}
	if from < 0 || at < len(want) {
		t.Fatalf("not in their groups, in the options' order (found %d of %v): %v", at, want, texts)
	}
	tu := a.tableOf(coll, v)
	tu.st.Collapsed = map[string]bool{"Okundu": true}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !slices.Equal(v.Collapsed, []string{"Okundu"}) {
		t.Fatalf("the fold is not kept: %v", v.Collapsed)
	}
	// its own order: the first row dragged by its grip below the second
	v.GroupBy, v.Sort, v.Desc = "", "manual", false
	tu.st.Collapsed = nil
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	before := viewOrder(coll, v)
	grip, ok := tt.Find("Move row")
	if !ok {
		t.Fatalf("no grip to move a row by: %v", tt.Texts())
	}
	x0, y0 := grip.X+grip.W/2, grip.Y+grip.H/2
	_, row := ryugo.TableSize(ryugo.Md)
	y1 := y0 + row
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Release(x0, y1)
	tt.Advance(time.Second)
	if after := viewOrder(coll, v); after[1] != before[0] || after[0] != before[1] {
		t.Fatalf("the row dragged is not where it was let go: %v, was %v", after, before)
	}
}

// viewOrder is coll's pages in view v's order.
func viewOrder(coll *Collection, v *View) []string {
	ps := coll.All()
	sortPages(ps, v)
	return titles(ps)
}

// A view's pages are filtered and sorted once, not at every frame, and made again when a page changes (#194); a
// dynamic collection's search is made once too (#187).
func TestViewPagesKept(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	made := a.viewCache.made
	for i := range 10 {
		tt.Move(500, float32(200+i*20))
		tt.Frame()
	}
	if a.viewCache.made != made {
		t.Fatalf("made %d times more in ten frames", a.viewCache.made-made)
	}
	coll.Pages[0].SetTitle("Ab")
	tt.Frame()
	if a.viewCache.made == made {
		t.Fatal("not made again when a page changed")
	}
	dyn, err := a.lib.NewCollection("Aranan")
	if err != nil {
		t.Fatal(err)
	}
	dyn.Query = "baharat"
	dyn.Save()
	a.openCollection(dyn)
	tt.Frame()
	searches := a.searchMade
	for range 10 {
		tt.Frame()
	}
	if a.searchMade != searches {
		t.Fatalf("searched %d times more in ten frames", a.searchMade-searches)
	}
	if got := titles(a.viewPages(dyn, dyn.View())); len(got) != 1 || got[0] != "Notlar: Dune" {
		t.Fatalf("the dynamic collection: %v", got)
	}
}

// TestViewLabelInWindowLanguage: a view that still has its type's name, in any of the window's languages, is
// shown by its type's name in the window's language; a name of its own is shown as it is (#490).
func TestViewLabelInWindowLanguage(t *testing.T) {
	defer setLang(lang)
	setLang("en")
	for _, c := range []struct {
		v    View
		want string
	}{
		{View{Type: "list", Label: "Liste"}, "List"},
		{View{Type: "table", Label: "Table"}, "Table"},
		{View{Type: "calendar", Label: "Takvim"}, "Calendar"},
		{View{Type: "board", Label: "Pano"}, "Board"},
		{View{Type: "list", Label: ""}, "List"},
		{View{Type: "table", Label: "Reading list"}, "Reading list"},
		{View{Type: "board", Label: "Tablo"}, "Tablo"}, // another type's name is a name of its own
	} {
		if got := viewLabel(c.v); got != c.want {
			t.Errorf("en %q (%s) = %q, want %q", c.v.Label, c.v.Type, got, c.want)
		}
	}
	setLang("tr")
	if got := viewLabel(View{Type: "list", Label: "List"}); got != "Liste" {
		t.Errorf("tr List = %q, want Liste", got)
	}
}

// TestColumnsMenu: the table's Columns menu checks the columns the view shows; hiding one takes it out of the
// view's columns, and showing it again brings it back, even when the table itself had hidden it (#490).
func TestColumnsMenu(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Tablo"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	v := coll.View()
	shown := func(key string) bool {
		for _, it := range a.columnsMenu(coll, v) {
			if it.title == columnName(coll, key) {
				return it.glyph == iconCheck
			}
		}
		t.Fatalf("no %s in the menu", key)
		return false
	}
	if !shown("created") {
		t.Fatal("created should be shown at first")
	}
	a.showColumn(coll, v, "created", false)
	if shown("created") || slices.Contains(viewColumns(coll, v), "created") {
		t.Fatalf("created still shown: %v", viewColumns(coll, v))
	}
	v.Hidden = append(v.Hidden, "created")
	a.showColumn(coll, v, "created", true)
	if !shown("created") || slices.Contains(v.Hidden, "created") {
		t.Fatalf("created not back: columns %v hidden %v", viewColumns(coll, v), v.Hidden)
	}
	items := a.columnsMenu(coll, v)
	if items[len(items)-1].title != locales.CommonAddProperty() {
		t.Fatal("the menu does not end with Add a property")
	}
}

// columnName is the name the Columns menu gives the column of key.
func columnName(coll *Collection, key string) string {
	for _, col := range allColumns(coll) {
		if col.key == key {
			return col.label
		}
	}
	return key
}

// A list of hundreds of pages builds viewStep rows, and its last row shows more as it comes into sight; it does not
// grow by itself while the reader is not at the end (#194).
func TestListShowsInSteps(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	for i := range 450 {
		if _, err := coll.NewPage(nil, fmt.Sprintf("Kitap %03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	tt.SetSize(1100, 800)
	a.openCollection(coll)
	for range 5 {
		tt.Advance(100 * time.Millisecond)
	}
	if a.more.n != viewStep {
		t.Fatalf("it grew by itself, the reader at the top: %d", a.more.n)
	}
	if !tt.HasText(locales.ViewsShowMore(len(coll.All()) - viewStep)) {
		t.Fatalf("no row for more: %v", tt.Texts()[:10])
	}
	for range 60 { // to the end of what is shown
		tt.Wheel(600, 400, 0, 2000)
		tt.Advance(100 * time.Millisecond)
		if a.more.n > viewStep {
			break
		}
	}
	if a.more.n <= viewStep {
		t.Fatal("the end of the list was reached and no more was shown")
	}
	a.search = "Kitap 01"
	tt.Advance(100 * time.Millisecond)
	if a.more.n != viewStep {
		t.Fatalf("a new search did not start again from the first rows: %d", a.more.n)
	}
}

// TestCollectionColorIsItsAccent: a collection with a color has its view in that accent - its new page button takes
// a hue near the collection's, not the window's blue - and one without keeps the window's (#358).
func TestCollectionColorIsItsAccent(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 600)
	coll := a.lib.Collections[0]
	a.openCollection(coll)
	button := func() color.RGBA {
		t.Helper()
		tt.Frame()
		tt.Advance(400 * time.Millisecond)
		r, ok := tt.Find(newPageLabel(coll))
		if !ok {
			t.Fatalf("no new page button: %v", tt.Texts())
		}
		return tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
	}
	plain := button()
	coll.Color = "c3"
	tinted := button()
	if plain == tinted {
		t.Fatalf("the button is %v with the collection's color and without", tinted)
	}
	// warmer: the collection's c3 is an amber, the window's accent a blue
	if int(tinted.R)-int(tinted.B) <= int(plain.R)-int(plain.B) {
		t.Fatalf("the button %v is not the collection's color (without it %v)", tinted, plain)
	}
}

// A list grouped by a choice shows viewStep pages too, and more as the reader gets to the end; each group's heading
// counts all its pages (#194).
func TestGroupedListShowsInSteps(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	pr, err := coll.AddProperty("Durum", PropChoice, []string{"Okunacak", "Okundu"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 300 {
		p, err := coll.NewPage(nil, fmt.Sprintf("Kitap %03d", i))
		if err != nil {
			t.Fatal(err)
		}
		p.SetValue(pr.ID, []string{"Okunacak", "Okundu"}[i%2])
	}
	tt.SetSize(1100, 800)
	a.openCollection(coll)
	coll.View().GroupBy = pr.ID
	tt.Advance(300 * time.Millisecond)
	total := len(coll.All())
	if !tt.HasText(locales.ViewsShowMore(total - viewStep)) {
		t.Fatalf("no row for more: %v", tt.Texts()[:12])
	}
	if !tt.HasText("150") {
		t.Fatal("a group's heading does not count all its pages")
	}
	a.more.n = total
	tt.Advance(100 * time.Millisecond)
	if tt.HasText(locales.ViewsShowMore(total - viewStep)) {
		t.Fatal("still a row for more when all show")
	}
}
