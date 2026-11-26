package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Two panes: a page in each, each written in on its own, the one in use following the user.
func TestTwoPanesDifferentPages(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	first := a.pane().page
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
	if a.ps.active != 1 || a.pane().page != second {
		t.Fatalf("the new pane is not in use: active %d page %v", a.ps.active, a.pane().page)
	}
	if a.ps.states[0].page != first {
		t.Fatalf("the first pane lost its page: %v", a.ps.states[0].page)
	}
	// write in the pane in use
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "abc")
	tt.Advance(1200 * time.Millisecond)
	if got := a.pane().doc.Text().String(); got != "Öteki"+"abc" {
		t.Fatalf("second page: %q", got)
	}
	if got := a.docs[first].Text().String(); got != "Birinci" {
		t.Fatalf("the first page was written in: %q", got)
	}
	// go to the first pane, and write there
	a.usePane(0)
	tt.Advance(300 * time.Millisecond)
	if a.pane().page != first {
		t.Fatalf("not in the first: %v", a.pane().page)
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "xyz")
	tt.Advance(1200 * time.Millisecond)
	if got := a.pane().doc.Text().String(); got != "Birinci"+"xyz" {
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
	a.openInSide(place{"page", a.pane().page.Dir})
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 || a.ps.states[0].page != a.pane().page {
		t.Fatalf("panes %d", a.paneCount())
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, "hey")
	tt.Advance(400 * time.Millisecond)
	if a.ps.states[0].doc != a.pane().doc {
		t.Fatal("two texts for one page")
	}
}

// A screen that is not a page - the settings - is a pane of its own, and a pane closes.
func TestPaneForAScreen(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	page := a.pane().page
	a.openInSide(place{kind: "settings"})
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("Görünüm") {
		t.Fatalf("the page and the settings are not both there: %v", tt.Texts())
	}
	if !a.pane().settingsOpen {
		t.Fatal("the pane in use is not the settings")
	}
	a.closePane(a.ps.active)
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 1 || a.pane().page != page || a.pane().settingsOpen {
		t.Fatalf("after closing: %d panes, page %v, settings %v", a.paneCount(), a.pane().page, a.pane().settingsOpen)
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
	first := a.pane().page
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
	r := a.panesR // the window's right edge target: the middle of the panes' right edge
	x1, y1 := r.X+r.W-12-15, r.Y+r.H/2
	holdTo(tt, x0, y0, x1, y1)
	if a.carry.drop == nil || a.ps.layout == nil {
		t.Fatalf("not offered to the Dock: pos %v panes %v", a.drag.pos, r)
	}
	tt.Release(x1, y1)
	tt.Advance(600 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("panes: %d", a.paneCount())
	}
	if a.pane().page != second || a.ps.states[0].page != first {
		t.Fatalf("not where it was dropped: in use %v, first pane %v", a.pane().page, a.ps.states[0].page)
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
	for _, it := range a.pageMenu(a.pane().page, false) {
		if it.title == "openInASidePane" {
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
	for _, it := range a.collectionMenu(a.pane().page.Coll) {
		if it.title == "openInASidePane" {
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
	for _, w := range []string{"Yan bölmede aç: bu ekran", locales.SwitchPane(), "Bölmeyi kapat"} {
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
	firstDir := a.pane().page.Dir
	second, _ := a.pane().page.Coll.NewPage(nil, "İkinci")
	second.SaveText("Öteki\n")
	a.openInSide(place{"page", second.Dir})
	tt.Advance(300 * time.Millisecond)
	secondDir := a.pane().page.Dir
	a.reloadLibrary()
	tt.Advance(300 * time.Millisecond)
	if a.pane().page == nil || a.pane().page.Dir != secondDir || a.pane().doc == nil {
		t.Fatalf("the pane in use lost its page: %v", a.pane().page)
	}
	other := a.ps.states[1-a.ps.active]
	if other.page == nil || other.page.Dir != firstDir || other.doc == nil {
		t.Fatalf("the other pane lost its page: %+v", other.page)
	}
}

// More than two panes: Ryugo's Dock lays them out, each shows its page, and a pane closed (here by the
// app, there by the user in the Dock) is gone from both.
func TestManyPanes(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1600, 900)
	tt.Frame()
	first := a.pane().page
	var titles []string
	for _, name := range []string{"İkinci", "Üçüncü", "Dördüncü"} {
		p, err := first.Coll.NewPage(nil, name)
		if err != nil {
			t.Fatal(err)
		}
		p.SaveText(name + "\n")
		titles = append(titles, name)
		a.openAtPane(a.paneCount(), place{"page", p.Dir})
		tt.Advance(300 * time.Millisecond)
	}
	if a.paneCount() != 4 || len(a.ps.ids) != 4 || len(a.ps.layout.PaneIDs()) != 4 {
		t.Fatalf("panes %d, ids %d, in the Dock %d", a.paneCount(), len(a.ps.ids), len(a.ps.layout.PaneIDs()))
	}
	for _, name := range titles {
		if !tt.HasText(name) {
			t.Errorf("%q is not shown in %v", name, tt.Texts())
		}
	}
	// every pane has a place of its own
	seen := map[ui.Rect]bool{}
	for _, id := range a.ps.ids {
		r := a.ps.rects[id]
		if r.W < 100 || r.H < 100 || seen[r] {
			t.Fatalf("pane %s at %v", id, r)
		}
		seen[r] = true
	}
	// the user closes a tab in the Dock: the pane goes from the app too
	gone := a.ps.ids[1]
	a.ps.layout.RemovePane(gone)
	tt.Frame()
	tt.Frame()
	if a.paneCount() != 3 || a.ps.index(gone) >= 0 {
		t.Fatalf("after the Dock closed one: %d panes, %v", a.paneCount(), a.ps.ids)
	}
	// the app closes the pane in use; the one after it takes the use
	a.closePane(a.ps.active)
	tt.Frame()
	if a.paneCount() != 2 || a.ps.layout == nil || len(a.ps.layout.PaneIDs()) != 2 {
		t.Fatalf("after closing: %d panes", a.paneCount())
	}
	a.closePane(a.ps.active)
	tt.Frame()
	if a.paneCount() != 1 || a.ps.layout != nil {
		t.Fatalf("with one left the Dock goes: %d panes, layout %v", a.paneCount(), a.ps.layout)
	}
	if a.pane().page == nil {
		t.Fatal("the pane left shows nothing")
	}
}

// The panes are kept in the settings and come back as they were: how many, in what arrangement, each showing
// what it showed, and the one in use.
func TestPanesComeBack(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1600, 900)
	tt.Frame()
	first := a.pane().page
	var pages []*Page
	for _, name := range []string{"İkinci", "Üçüncü"} {
		p, _ := first.Coll.NewPage(nil, name)
		p.SaveText(name + "\n")
		pages = append(pages, p)
		a.openAtPane(a.paneCount(), place{"page", p.Dir})
		tt.Advance(300 * time.Millisecond)
	}
	a.openAtPane(0, place{"coll", first.Coll.Dir}) // the first pane shows the collection
	tt.Advance(300 * time.Millisecond)
	a.usePane(1)
	tt.Advance(300 * time.Millisecond)
	a.keepPanes()
	if a.settings.Panes == "" {
		t.Fatal("nothing kept")
	}
	// a window opened again from the same settings and library
	b := newApp(a.lib)
	b.settings.Panes = a.settings.Panes
	b.ps = paneSet{states: []*paneState{newPaneState()}}
	b.restorePaneLayout()
	if b.paneCount() != 3 || len(b.ps.layout.PaneIDs()) != 3 {
		t.Fatalf("panes: %d", b.paneCount())
	}
	if b.ps.active != 1 {
		t.Fatalf("pane in use: %d", b.ps.active)
	}
	if b.ps.states[0].coll == nil || b.ps.states[0].coll.Dir != first.Coll.Dir {
		t.Fatalf("the first pane does not show the collection: %+v", b.ps.states[0].coll)
	}
	if b.pane().page == nil || b.pane().page.Dir != pages[0].Dir {
		t.Fatalf("the pane in use shows %v", b.here())
	}
	if b.ps.states[2].page == nil || b.ps.states[2].page.Dir != pages[1].Dir {
		t.Fatalf("the third pane shows %v", b.ps.states[2].page)
	}
	// one pane again clears it
	a.closePane(0)
	a.closePane(0)
	tt.Frame()
	a.keepPanes()
	if a.settings.Panes != "" {
		t.Fatalf("still kept with one pane: %s", a.settings.Panes)
	}
}

// A pane goes into a window of its own: out of the main window's tree, into a window of the layout,
// with what it showed; the main window keeps a pane, and the overlays of the text are not drawn in it
// while the pane in use is elsewhere.
func TestTearOffPane(t *testing.T) {
	a, tt := newTestApp(t, "merhaba\n")
	tt.SetSize(1000, 700)
	tt.Advance(200 * time.Millisecond)
	page := a.pane().page
	a.mu.Lock()
	a.tearOffPane()
	a.mu.Unlock()
	tt.Advance(300 * time.Millisecond)
	l := a.ps.layout
	if l == nil || len(l.Windows) != 2 {
		t.Fatalf("no window of its own: %+v", l)
	}
	if a.paneCount() != 2 {
		t.Fatalf("%d panes", a.paneCount())
	}
	if !a.activeElsewhere() {
		t.Fatalf("the pane in use is in %q", a.paneWindow(a.ps.active))
	}
	if a.paneWindow(1-a.ps.active) != "main" {
		t.Fatal("the main window has no pane")
	}
	if a.pane().page != page {
		t.Fatalf("the pane torn off shows %v, not the page", a.pane().page)
	}
	// once more does nothing: it is in its own window already
	a.mu.Lock()
	a.tearOffPane()
	a.mu.Unlock()
	if len(a.ps.layout.Windows) != 2 {
		t.Fatalf("torn off twice: %d windows", len(a.ps.layout.Windows))
	}
}

// holdTo presses at x0, y0 and moves to x1, y1 in steps, a frame each, holding.
func holdTo(tt *ui.Tester, x0, y0, x1, y1 float32) {
	tt.Press(x0, y0)
	for k := 1; k <= 12; k++ {
		f := float32(k) / 12
		tt.Move(x0+(x1-x0)*f, y0+(y1-y0)*f)
		tt.Frame()
	}
	for range 3 { // the Dock aims a frame behind the pointer
		tt.Move(x1, y1)
		tt.Frame()
	}
}

// A page let go on the compass's middle over a pane opens in a pane beside it, never on top of it (the panes have no
// tabs to show one under another by, #468); a collection
// dragged over the panes and let go on the window's bottom edge target opens in a pane along it; let go
// back over the sidebar, nothing opens and the Dock goes again.
func TestDragIntoDock(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	first := a.pane().page
	second, _ := first.Coll.NewPage(nil, "İkinci")
	second.SaveText("Öteki\n")
	a.open(first)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	row, ok := tt.Find("İkinci")
	if !ok {
		t.Fatalf("no row: %v", tt.Texts())
	}
	r := a.panesR
	// the compass's middle: the middle of the pane, under its header
	holdTo(tt, row.X+row.W/2, row.Y+row.H/2, r.X+r.W/2, r.Y+r.H/2+14)
	tt.Release(r.X+r.W/2, r.Y+r.H/2+14)
	tt.Advance(600 * time.Millisecond)
	if a.paneCount() != 2 || a.pane().page != second {
		t.Fatalf("panes %d, in use %v", a.paneCount(), a.pane().page)
	}
	ids := a.ps.layout.PaneIDs()
	if len(ids) != 2 {
		t.Fatalf("Dock panes %v", ids)
	}
	if ar := a.ps.layout.Windows[0].Root; ar.IsArea() || len(ar.Children) != 2 {
		t.Fatalf("not two areas side by side: %+v", ar)
	}
	// a collection, onto the bottom edge
	coll := first.Coll
	crow, ok := tt.Find("Koleksiyon: " + coll.Name)
	if !ok {
		t.Fatalf("no collection row: %v", tt.Texts())
	}
	r = a.panesR
	bx, by := r.X+r.W/2, r.Y+r.H-12-15
	holdTo(tt, crow.X+crow.W/2, crow.Y+crow.H/2, bx, by)
	tt.Release(bx, by)
	tt.Advance(600 * time.Millisecond)
	if a.paneCount() != 3 || a.pane().coll != coll {
		t.Fatalf("panes %d, collection in use %v", a.paneCount(), a.pane().coll)
	}
	if root := a.ps.layout.Windows[0].Root; root.Dir != ryugo.DockColumn || len(root.Children) != 2 {
		t.Fatalf("not along the bottom: %+v", root)
	}
	if len(a.lib.Collections) == 0 || a.lib.Collections[0] != coll {
		t.Fatal("the collections' order changed")
	}
}

// With one pane, a page dragged over the panes and back to the sidebar opens nothing, and the Dock goes.
func TestDragIntoDockAndBack(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	second, _ := a.pane().page.Coll.NewPage(nil, "İkinci")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	row, _ := tt.Find("İkinci")
	r := a.panesR
	holdTo(tt, row.X+row.W/2, row.Y+row.H/2, r.X+r.W/2, r.Y+r.H/2)
	if a.ps.layout == nil {
		t.Fatal("no Dock for the drag")
	}
	holdTo(tt, r.X+r.W/2, r.Y+r.H/2, row.X+row.W/2, row.Y+row.H/2+60)
	tt.Release(row.X+row.W/2, row.Y+row.H/2+60)
	tt.Advance(600 * time.Millisecond)
	if a.paneCount() != 1 || a.ps.layout != nil {
		t.Fatalf("panes %d, Dock %v", a.paneCount(), a.ps.layout != nil)
	}
	_ = second
}

// A page thrown away while another pane shows it: that pane shows its collection, not a page that is gone
// (written to, it would make its folder again).
func TestTrashedPageLeavesOtherPane(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	first := a.pane().page
	second, err := first.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	second.SaveText("Öteki\n")
	a.openInSide(place{"page", second.Dir}) // pane 1 in use, showing second
	tt.Advance(300 * time.Millisecond)
	a.usePane(0)
	tt.Advance(300 * time.Millisecond)
	a.trashPage(second)
	if st := a.ps.states[1]; st.page != nil || st.doc != nil || st.coll != first.Coll {
		t.Fatalf("the other pane still shows it: page %v coll %v", st.page, st.coll)
	}
	tt.Advance(300 * time.Millisecond) // drawn with no page in it: no failure
	if _, err := os.Stat(second.Dir); !os.IsNotExist(err) {
		t.Fatalf("the folder of the page is there again: %v", err)
	}
}

// The same page in two panes, written in one: in the other, when it is used, the text is not a paste.
func TestSamePageOtherPaneResync(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	p := a.pane().page
	a.openInSide(place{"page", p.Dir})
	tt.Advance(300 * time.Millisecond)
	a.usePane(0)
	tt.Advance(300 * time.Millisecond)
	if err := a.writeBody(p, "Yeni metin"); err != nil { // as an agent does, the other pane not in use
		t.Fatal(err)
	}
	a.usePane(1)
	if v := a.pane().doc.Version(); a.pane().version != v || a.pane().length != a.pane().doc.Text().Len() {
		t.Fatalf("pane 1 not up with the text: version %d/%d length %d/%d", a.pane().version, v, a.pane().length, a.pane().doc.Text().Len())
	}
	a.usePane(0)
	if v := a.pane().doc.Version(); a.pane().version != v {
		t.Fatalf("pane 0 not up with the text: version %d/%d", a.pane().version, v)
	}
}

// The texts of the pages opened are kept, up to a number: a session through hundreds of pages does not hold them
// all, and what was let go is read again from its file, written as it was left.
func TestDocsAreBounded(t *testing.T) {
	a, _ := newTestApp(t, "ilk\n")
	c := a.lib.Collections[0]
	var pages []*Page
	for i := 0; i < maxDocs+30; i++ {
		p, err := c.NewPage(nil, fmt.Sprintf("S%d", i))
		if err != nil {
			t.Fatal(err)
		}
		p.SaveText(fmt.Sprintf("metin %d\n", i))
		pages = append(pages, p)
	}
	a.open(pages[0])
	a.pane().doc.SetText(importMarkdown("değişti\n")) // written in, and left: saved as it goes
	a.pane().dirty = true
	for _, p := range pages[1:] {
		a.open(p)
	}
	if len(a.docs) > maxDocs {
		t.Fatalf("%d texts kept, at most %d", len(a.docs), maxDocs)
	}
	if a.docs[a.pane().page] == nil || a.docs[pages[len(pages)-1]] == nil {
		t.Fatal("the page shown has no text")
	}
	if a.docs[pages[0]] != nil {
		t.Fatal("the longest unused is kept")
	}
	a.open(pages[0]) // read again from its file
	if got := a.pane().doc.Text().String(); !strings.Contains(got, "değişti") {
		t.Fatalf("what was written is lost: %q", got)
	}
}

// Each pane reads its own editor's paragraphs as it is built: one not in use follows the window narrowing, its
// text wrapping to more lines, as the one in use does, and its height with them (#190).
func TestPaneBoxesOwn(t *testing.T) {
	a, tt := newTestApp(t, strings.Repeat("Uzunca bir paragraf, bölme daraldıkça daha çok satıra kırılır. ", 8)+"\n\nİkinci paragraf.\n")
	tt.SetSize(1600, 800)
	tt.Frame()
	first := a.pane().page
	a.openInSide(place{"page", first.Dir})
	tt.Advance(400 * time.Millisecond)
	if a.paneCount() != 2 || a.ps.active != 1 {
		t.Fatalf("panes %d, active %d", a.paneCount(), a.ps.active)
	}
	before := a.ps.states[0].boxes
	if len(before) == 0 {
		t.Fatal("the pane not in use has no boxes")
	}
	tt.SetSize(1000, 800)
	tt.Advance(400 * time.Millisecond)
	after := a.ps.states[0].boxes
	if len(after) == 0 || after[0].Height <= before[0].Height {
		t.Fatalf("the pane not in use kept its boxes as the window narrowed: %v, then %v", before[0].Height, firstH(after))
	}
	// the two editors of one page each compare what they painted with what was read for them: no frame at each
	// refresh (60 a second) for a move that is not; the caret's blink and the timers are a few
	tt.Advance(3 * time.Second)
	v := a.frameSeq
	tt.Advance(time.Second)
	if n := a.frameSeq - v; n > 30 {
		t.Fatalf("%d views in a second at rest", n)
	}
}

func firstH(bs []richtext.ParagraphBox) float32 {
	if len(bs) == 0 {
		return 0
	}
	return bs[0].Height
}

// TestRestoreTagPaneColdIndex checks that restoring a tag pane before the link index
// has finished warming keeps the tag destination instead of dropping it as empty.
func TestRestoreTagPaneColdIndex(t *testing.T) {
	a, tt := newTestApp(t, "#fikir notu\n")
	tt.SetSize(1100, 700)
	tt.Advance(200 * time.Millisecond)
	a.openInSide(place{"tag", "fikir"})
	tt.Advance(200 * time.Millisecond)
	a.keepPanes()
	if a.settings.Panes == "" {
		t.Fatal("nothing kept in settings.Panes")
	}

	b := newApp(a.lib)
	b.settings.Panes = a.settings.Panes
	b.ps = paneSet{states: []*paneState{newPaneState()}}
	b.prewarming = true // simulate startup prewarm running before index is built
	b.links = linkIndex{}
	b.restorePaneLayout()
	if b.pane().tag != "fikir" {
		t.Fatalf("active restored pane tag = %q, want %q", b.pane().tag, "fikir")
	}
}

// TestBlockDragLeavesOtherPane: with one page in two panes, a block dragged in one opens room in that pane's text
// alone; the other stays still, and shows the block moved once it is dropped (#480).
func TestBlockDragLeavesOtherPane(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n\nÜç\n\nDört\n")
	tt.SetSize(1400, 800)
	tt.Frame()
	a.openInSide(place{"page", a.pane().page.Dir})
	tt.Advance(400 * time.Millisecond)
	if a.paneCount() != 2 || a.ps.active != 1 {
		t.Fatalf("panes %d, active %d", a.paneCount(), a.ps.active)
	}
	tops := func(bs []richtext.ParagraphBox) []float32 {
		var out []float32
		for _, b := range bs {
			out = append(out, b.Top)
		}
		return out
	}
	still := tops(a.ps.states[0].boxes)
	r := a.pane().editorR
	var before float32
	for _, b := range a.pane().boxes {
		if b.Index == 2 {
			before = b.Top
		}
	}
	b0 := a.pane().boxes[0]
	tt.Move(r.X+40, r.Y+b0.Top+b0.Height/2)
	tt.Frame()
	tt.Frame()
	g, ok := tt.Find("Blok tutamağı")
	if !ok {
		t.Fatal("no grip")
	}
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	y1 := r.Y + before + 2
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Advance(500 * time.Millisecond)
	if !a.pane().grip.dragging || a.pane().grip.toIdx != 2 {
		t.Fatalf("aim: dragging %v, toIdx %d", a.pane().grip.dragging, a.pane().grip.toIdx)
	}
	other := a.ps.states[0]
	if other.grip.dragging || len(other.grip.rooms) != 0 {
		t.Fatalf("the other pane drags too: %v, rooms %v", other.grip.dragging, other.grip.rooms)
	}
	if got := tops(other.boxes); !slices.Equal(got, still) {
		t.Fatalf("the other pane's text moved: %v, was %v", got, still)
	}
	tt.Release(x0, y1+1)
	tt.Advance(400 * time.Millisecond)
	if got := mdOf(a.pane().doc); got != "İki\n\nBir\n\nÜç\n\nDört" || a.ps.states[0].doc != a.pane().doc {
		t.Fatalf("dropped: %q", got)
	}
}

func TestPaneRestoreByPageIDAcrossMove(t *testing.T) {
	a, tt := newTestApp(t, "İlk sayfa\n")
	c1 := a.pane().page.Coll
	c2, _ := a.lib.NewCollection("Hedef")
	p2, _ := c1.NewPage(nil, "Taşınacak")
	p2.SaveText("içerik burada\n")
	if p2.ID == "" {
		t.Fatal("expected generated Page.ID")
	}
	a.openInSide(place{"page", p2.Dir})
	tt.Frame()
	if a.paneCount() != 2 || a.ps.states[1].page != p2 {
		t.Fatalf("expected side pane on %v, got %v", p2, a.ps.states[1].page)
	}
	snap := a.panesSnapshot()
	if snap == "" {
		t.Fatal("expected non-empty panes snapshot")
	}

	// Move p2 to c2, changing its Dir while preserving its Page.ID.
	if err := a.lib.MovePage(p2, c2, nil, 0); err != nil {
		t.Fatal(err)
	}
	refs := a.paneRefs()
	a.restorePanes(refs)
	if a.ps.states[1].page == nil || a.ps.states[1].page.ID != p2.ID {
		t.Fatalf("restorePanes lost moved page: got %v want ID %q", a.ps.states[1].page, p2.ID)
	}

	// Simulate fresh restart reading snapshot from settings after the move.
	a2 := newApp(a.lib)
	a2.settings.Panes = snap
	a2.restorePaneLayout()
	if len(a2.ps.states) != 2 || a2.ps.states[1].page == nil || a2.ps.states[1].page.ID != p2.ID {
		t.Fatalf("restorePaneLayout lost moved page: states=%v", a2.ps.states)
	}
}

func TestPanesPersistThroughSettingsStoreReload(t *testing.T) {
	file := t.TempDir() + "/panes.db"
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Notlar")
	p1, _ := c.NewPage(nil, "Bir")
	p1.SaveText("bir\n")
	p2, _ := c.NewPage(nil, "İki")
	p2.SaveText("iki\n")

	a := newApp(lib)
	a.settingsStore = st
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	a.openInSide(place{"page", p2.Dir})
	tt.Advance(200 * time.Millisecond)
	a.usePane(0)
	tt.Frame()
	a.keepPanes()
	a.saveSettings()

	// Replicate main() startup order: newApp(lib2) runs before settingsStore & loadSettings().
	lib2, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	b := newApp(lib2)
	b.settingsStore = st
	b.loadSettings()
	b.restorePaneLayout()
	if b.paneCount() != 2 || b.ps.active != 0 {
		t.Fatalf("expected 2 panes with active=0 after reload, got count=%d active=%d", b.paneCount(), b.ps.active)
	}
	if b.ps.states[1].page == nil || b.ps.states[1].page.ID != p2.ID {
		t.Fatalf("expected second pane to restore %q, got %v", p2.ID, b.ps.states[1].page)
	}
}

// With panes open, a frame in which neither the layout nor what a pane shows nor the pane in use changed makes no
// snapshot of them, typing and scrolling included; a change of any of them is kept as before (#592).
func TestPanesSnapshotOnlyWhenChanged(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1600, 900)
	tt.Frame()
	first := a.pane().page
	p, _ := first.Coll.NewPage(nil, "İkinci")
	p.SaveText("İkinci\n")
	a.openAtPane(a.paneCount(), place{"page", p.Dir})
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 || a.settings.Panes == "" {
		t.Fatalf("panes %d, kept %q", a.paneCount(), a.settings.Panes)
	}
	n := a.ps.snaps
	for range 20 { // at rest, the caret blinking
		tt.Advance(50 * time.Millisecond)
	}
	typeEach(tt, "abc")
	tt.Advance(100 * time.Millisecond)
	if a.ps.snaps != n {
		t.Fatalf("%d snapshots in frames that changed none of the panes", a.ps.snaps-n)
	}
	was := a.settings.Panes
	a.usePane(0) // the pane in use
	tt.Advance(300 * time.Millisecond)
	if a.ps.snaps == n || a.settings.Panes == was {
		t.Fatal("the pane in use changed and was not kept")
	}
	n, was = a.ps.snaps, a.settings.Panes
	a.openAtPane(1, place{"coll", first.Coll.Dir}) // what a pane shows
	tt.Advance(300 * time.Millisecond)
	if a.ps.snaps == n || a.settings.Panes == was {
		t.Fatal("a pane went elsewhere and was not kept")
	}
	n = a.ps.snaps
	a.ps.rev++ // the Dock's OnChange: an edge let go
	tt.Frame()
	if a.ps.snaps != n+1 {
		t.Fatalf("the layout changed: %d snapshots, want 1", a.ps.snaps-n)
	}
}
