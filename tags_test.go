package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTagsOf(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"merhaba #fikir ve #Proje/alt.", []string{"fikir", "Proje/alt"}},
		{"# Başlık\n## Alt\nmetin", nil},
		{"(#x) a#b #1 #-", nil},
		{"```\n#include\n```\n`#kod` ama #gerçek", []string{"gerçek"}},
		{"#Fikir ve #fikir", []string{"Fikir"}},
		{"#çalışma #ölçü", []string{"çalışma", "ölçü"}},
	} {
		got := tagsOf(tc.in)
		if !equal(got, tc.want) {
			t.Errorf("%q: %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestTagView: a tag in a page's text lists in the palette and the side
// panel, is drawn as a pill that opens it, and its view lists its pages.
func TestTagView(t *testing.T) {
	a, tt := newTestApp(t, "bir #fikir burada\n")
	tt.SetSize(1100, 800)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.Save("başka #Fikir ve #plan\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if len(a.tagHits)+len(a.tagSeen) == 0 {
		t.Fatal("the tag is not drawn")
	}
	all := a.allTags()
	if len(all) != 2 || all[0].key != "fikir" || len(all[0].pages) != 2 || all[1].key != "plan" {
		t.Fatalf("tags: %+v", all)
	}
	found := false
	for _, it := range a.paletteItems("fikir") {
		found = found || it.title == "#fikir"
	}
	if !found {
		t.Fatal("not in the palette")
	}
	// the side panel lists the page's tags
	a.settings.PanelOpen = true
	tt.Advance(600 * time.Millisecond)
	if !tt.HasText("ETİKETLER") {
		t.Fatalf("no tags in the panel: %v", tt.Texts())
	}
	// a click on the tag in the text opens its view
	h := a.tagSeen[0]
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(300 * time.Millisecond)
	if a.tag != "fikir" || !tt.HasText("#fikir") || !tt.HasText("Sayfa: Öteki") {
		t.Fatalf("tag %q: %v", a.tag, tt.Texts())
	}
	// a page of it opens; back returns to the tag
	if err := tt.Click("Sayfa: Öteki"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if a.tag != "" || a.page != other {
		t.Fatalf("page %v tag %q", a.page.Title, a.tag)
	}
	a.step(-1)
	tt.Advance(100 * time.Millisecond)
	if a.tag != "fikir" {
		t.Fatalf("back: %q", a.tag)
	}
}

// TestTagMenu: a # typed in a line offers the tags there are; ↵ takes one,
// or the new name typed; at the start of a line it is a heading's, and offers nothing.
func TestTagMenu(t *testing.T) {
	a, tt := newTestApp(t, "ilk\n")
	tt.SetSize(1100, 800)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.Save("başka #fikir ve #plan\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	a.doc.SetSelection(3, 3)
	tt.Frame()
	typeEach(tt, " gün ")
	tt.Type("#")
	tt.Advance(100 * time.Millisecond)
	if !a.slash.open || a.slash.sign != "#" || !tt.HasText("#fikir") || !tt.HasText("#plan") {
		t.Fatalf("no menu: open %v %v", a.slash.open, tt.Texts())
	}
	typeEach(tt, "fi")
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("#plan") {
		t.Fatalf("not narrowed: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if got := a.doc.Text().String(); got != "ilk gün #fikir \n" && got != "ilk gün #fikir " {
		t.Fatalf("text %q", got)
	}
	if a.slash.open {
		t.Fatal("the menu stays")
	}
	// a name that is new
	typeEach(tt, "ve #yeni")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Yeni etiket: #yeni") {
		t.Fatalf("no new: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if got := a.doc.Text().String(); !strings.Contains(got, "#fikir ve #yeni ") {
		t.Fatalf("text %q", got)
	}
	// a space in the name closes it
	typeEach(tt, "de #fi bir")
	tt.Advance(100 * time.Millisecond)
	if a.slash.open {
		t.Fatal("open after a space")
	}
}

// TestHashAtLineStart: # at the start of a line is a heading's: no menu.
func TestHashAtLineStart(t *testing.T) {
	a, tt := newTestApp(t, "")
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.Save("#fikir\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	typeEach(tt, "# Başlık")
	tt.Advance(100 * time.Millisecond)
	if a.slash.open {
		t.Fatal("a menu on a heading's #")
	}
	if st := a.doc.Text().ParagraphStyleAt(0); st.Heading != 1 {
		t.Fatalf("not a heading: %+v %q", st, a.doc.Text().String())
	}
}
