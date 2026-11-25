package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

func findPage(t *testing.T) (*app, func()) {
	t.Helper()
	var b strings.Builder
	b.WriteString("Kenar iğne burada\n\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "Satır %d dolgu dolgu dolgu\n\n", i)
	}
	b.WriteString("Son iğne en altta\n")
	a, tt := newTestApp(t, b.String())
	tt.SetSize(1200, 700)
	tt.Advance(400 * time.Millisecond)
	a.pane().pageScroll.Y = 0
	tt.Advance(200 * time.Millisecond)
	return a, func() {
		tt.Frame()
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
	}
}

// ⌘F drops a bar over the page's top right: the page under it does not move.
func TestFindBarLiesOverThePage(t *testing.T) {
	a, tt := newTestApp(t, "Birinci satır\n\nİkinci satır iğne\n")
	tt.SetSize(1200, 700)
	tt.Advance(400 * time.Millisecond)
	before, ok := tt.Find("Özellik ekle") // the page's own row, under the title
	if !ok {
		t.Fatalf("no row: %v", tt.Texts())
	}
	a.openFind()
	a.pane().find.query = "iğne"
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !a.pane().find.open || len(a.pane().find.matches) != 1 {
		t.Fatalf("not found: open %v, matches %v", a.pane().find.open, a.pane().find.matches)
	}
	after, ok := tt.Find("Özellik ekle")
	if !ok || after.Y != before.Y {
		t.Fatalf("the page moved: %v -> %v", before, after)
	}
	bar, ok := tt.Find("0/0")
	_ = bar
	if ok {
		t.Fatal("a count of no matches shown for a match")
	}
}

// Stepping to a match out of sight scrolls the page to it, with the focus in the find bar.
func TestFindStepScrollsToTheMatch(t *testing.T) {
	a, settle := findPage(t)
	a.openFind()
	a.pane().find.query = "iğne"
	settle()
	if len(a.pane().find.matches) != 2 {
		t.Fatalf("matches %v", a.pane().find.matches)
	}
	a.pane().pageScroll.Y = 0
	settle()
	a.stepMatch(a.pane(), 1) // the match at the end, far below
	a.stepMatch(a.pane(), 1)
	a.pane().find.cur = len(a.pane().find.matches) - 1
	a.showMatch(a.pane())
	settle()
	if a.pane().pageScroll.Y < 500 {
		t.Fatalf("the page did not go to the match: scrolled %v of %v", a.pane().pageScroll.Y, a.pane().pageScroll.MaxY)
	}
	a.pane().find.cur = 0
	a.showMatch(a.pane())
	settle()
	if a.pane().pageScroll.Y > 200 {
		t.Fatalf("the page did not come back to the first match: %v", a.pane().pageScroll.Y)
	}
}

// A found line far down a long page: the cover scrolls away with the title, and does not stay at the top of the
// window over the lines (the margin is the page's own color all the way down).
func TestLongPageCoverGoesWhenScrolled(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 1200; i++ {
		fmt.Fprintf(&b, "Paragraf %d: bir araç ne kadar güçlüyse yanlış kullanımın sonucu da o kadar büyüktür.\n\n", i)
	}
	b.WriteString("Bizans sırları\n")
	a, tt := newTestApp(t, b.String())
	a.settings.AutoCover = true // a made cover, off by default
	tt.SetSize(1100, 800)
	tt.Advance(500 * time.Millisecond)
	a.gotoPos(strings.Index(a.pane().doc.Text().String(), "Bizans"))
	for i := 0; i < 20; i++ {
		tt.Advance(150 * time.Millisecond)
		tt.Frame()
		if a.pane().pageScroll.Y > autoFullH+a.pane().headH {
			// the top of the window is as it is with no cover at all: the card's own ground, which is a gradient in
			// the window's frame, so it is set against itself and not against the pixels lower down
			with := tt.Image().RGBAAt(1060, 60)
			a.settings.AutoCover = false
			tt.Frame()
			if without := tt.Image().RGBAAt(1060, 60); with != without {
				t.Fatalf("the cover is over the lines: %v at the top, %v with no cover", with, without)
			}
			return
		}
	}
	t.Fatal("the page never scrolled to the line")
}

// A query with a capital or a mark is found as it is written, and a replace of all takes only that; in lower case
// without marks it finds every way of writing it (#212).
func TestFindExactWhenWrittenSo(t *testing.T) {
	a, tt := newTestApp(t, "on ön ON Ön\n")
	tt.Frame()
	for q, want := range map[string]int{"on": 4, "ön": 1, "ON": 1, "Ön": 1} {
		a.pane().find.query, a.pane().find.ver = q, 0
		a.findAll(a.pane())
		if len(a.pane().find.matches) != want {
			t.Errorf("%q: %d matches, want %d", q, len(a.pane().find.matches), want)
		}
	}
	a.pane().find.open, a.pane().find.query, a.pane().find.with, a.pane().find.ver = true, "ön", "x", 0
	a.findAll(a.pane())
	a.replaceMatch(a.pane(), true)
	if got := a.pane().doc.Text().String(); got != "on x ON Ön" {
		t.Fatalf("replace all of ön: %q", got)
	}
}

// What find does not go into, a fold's lines and a table's cells, is counted, and the bar says so (#212).
func TestFindCountsWhatIsHidden(t *testing.T) {
	a, tt := newTestApp(t, "# Başlık\n\nelma bir\n\nelma iki\n\n# Son\n\nelma\n")
	tt.SetSize(1100, 800)
	tt.Frame()
	a.fold(at(a.pane().doc, "Başlık"))
	tt.Frame()
	ch := a.insertTableOf(a.pane().doc.Text().Len(), newTable(2, 1))
	ch.tbl.setCell(0, 1, "kırmızı elma")
	tt.Frame()
	a.openFind()
	a.pane().find.query = "elma"
	tt.Frame()
	if len(a.pane().find.matches) != 1 || a.pane().find.hidden != 3 {
		t.Fatalf("matches %d, hidden %d", len(a.pane().find.matches), a.pane().find.hidden)
	}
	if !tt.HasText(locales.FindHiddenMore(3)) {
		t.Fatal("the bar does not say what is hidden")
	}
}
