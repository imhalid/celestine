package main

import (
	"testing"
	"time"
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
