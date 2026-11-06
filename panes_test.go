package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Two panes: a page in each, each written in on its own, the one in use following the user.
func TestTwoPanesDifferentPages(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	first := a.page
	second, err := first.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	second.SaveText("Öteki\n")
	a.openInSide(place{"page", second.Dir})
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("panes: %d", a.paneCount())
	}
	if a.ps.active != 1 || a.page != second {
		t.Fatalf("the new pane is not in use: active %d page %v", a.ps.active, a.page)
	}
	if a.ps.states[0].page != first {
		t.Fatalf("the first pane lost its page: %v", a.ps.states[0].page)
	}
	// write in the pane in use
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "abc")
	tt.Advance(1200 * time.Millisecond)
	if got := a.doc.Text().String(); got != "Öteki"+"abc" {
		t.Fatalf("second page: %q", got)
	}
	if got := a.docs[first].Text().String(); got != "Birinci" {
		t.Fatalf("the first page was written in: %q", got)
	}
	// go to the first pane, and write there
	a.usePane(0)
	tt.Advance(300 * time.Millisecond)
	if a.page != first {
		t.Fatalf("not in the first: %v", a.page)
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "xyz")
	tt.Advance(1200 * time.Millisecond)
	if got := a.doc.Text().String(); got != "Birinci"+"xyz" {
		t.Fatalf("first page: %q", got)
	}
	if got := a.docs[second].Text().String(); got != "Öteki"+"abc" {
		t.Fatalf("the second page changed: %q", got)
	}
	// what was written is on disk
	if body, _ := second.Body(); body == "" {
		t.Fatal("the second page was not saved")
	}
}

// The same page in two panes is one text: written in one, shown in the other.
func TestTwoPanesSamePage(t *testing.T) {
	a, tt := newTestApp(t, "Aynı\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	a.openInSide(place{"page", a.page.Dir})
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 || a.ps.states[0].page != a.page {
		t.Fatalf("panes %d", a.paneCount())
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "hey")
	tt.Advance(400 * time.Millisecond)
	if a.ps.states[0].doc != a.doc {
		t.Fatal("two texts for one page")
	}
}

// A screen that is not a page - the settings - is a pane of its own, and a pane closes.
func TestPaneForAScreen(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	page := a.page
	a.openInSide(place{kind: "settings"})
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("Görünüm") {
		t.Fatalf("the page and the settings are not both there: %v", tt.Texts())
	}
	if !a.settingsOpen {
		t.Fatal("the pane in use is not the settings")
	}
	a.closePane(a.ps.active)
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 1 || a.page != page || a.settingsOpen {
		t.Fatalf("after closing: %d panes, page %v, settings %v", a.paneCount(), a.page, a.settingsOpen)
	}
	if tt.HasText("Görünüm") {
		t.Fatal("the settings stayed")
	}
}

// A page held in the sidebar and let go over the right of the content opens in a pane of its own.
func TestDragToSide(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	first := a.page
	second, _ := first.Coll.NewPage(nil, "İkinci")
	second.SaveText("Öteki\n")
	a.open(first)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	row, ok := tt.Find("İkinci")
	if !ok {
		t.Fatalf("no row: %v", tt.Texts())
	}
	x0, y0 := row.X+row.W/2, row.Y+row.H/2
	tt.Press(x0, y0)
	for k := 1; k <= 12; k++ {
		tt.Move(x0+float32(k)*90, y0+float32(k)*3)
		tt.Frame()
	}
	if !a.drag.sideOn {
		t.Fatalf("not aimed at the side: pos %v zone %d", a.drag.pos, a.sideZone())
	}
	tt.Release(x0+12*90, y0+36)
	tt.Advance(600 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("panes: %d", a.paneCount())
	}
	if a.page != second || a.ps.states[0].page != first {
		t.Fatalf("not where it was dropped: in use %v, first pane %v", a.page, a.ps.states[0].page)
	}
	// the page did not move in the tree
	if len(first.Coll.Pages) != 2 || first.Coll.Pages[0] != first {
		t.Fatalf("the tree changed: %v", first.Coll.Pages)
	}
}

// The context menus of a page and a collection open it in a pane; the commands of the palette too.
func TestOpenInPaneMenus(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	found := false
	for _, it := range a.pageMenu(a.page, false) {
		if it.title == "Yan bölmede aç" {
			found = true
			it.run()
		}
	}
	if !found {
		t.Fatal("no \"Yan bölmede aç\" in the page's menu")
	}
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("panes: %d", a.paneCount())
	}
	found = false
	for _, it := range a.collectionMenu(a.page.Coll) {
		if it.title == "Yan bölmede aç" {
			found = true
		}
	}
	if !found {
		t.Fatal("no \"Yan bölmede aç\" in the collection's menu")
	}
	have := map[string]bool{}
	for _, it := range a.commands() {
		have[it.title] = true
	}
	for _, w := range []string{"Yan bölmede aç: bu ekran", "Bölmeler arası geç", "Bölmeyi kapat"} {
		if !have[w] {
			t.Errorf("no command %q", w)
		}
	}
}

// The library read again from disk leaves each pane on its page.
func TestPanesSurviveReload(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	firstDir := a.page.Dir
	second, _ := a.page.Coll.NewPage(nil, "İkinci")
	second.SaveText("Öteki\n")
	a.openInSide(place{"page", second.Dir})
	tt.Advance(300 * time.Millisecond)
	secondDir := a.page.Dir
	a.reloadLibrary()
	tt.Advance(300 * time.Millisecond)
	if a.page == nil || a.page.Dir != secondDir || a.doc == nil {
		t.Fatalf("the pane in use lost its page: %v", a.page)
	}
	other := a.ps.states[1-a.ps.active]
	if other.page == nil || other.page.Dir != firstDir || other.doc == nil {
		t.Fatalf("the other pane lost its page: %+v", other.page)
	}
}
