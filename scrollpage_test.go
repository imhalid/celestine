package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func longPage() string {
	var b strings.Builder
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&b, "Paragraf %d: biraz metin, biraz daha metin.\n\n", i)
	}
	return b.String()
}

// The title and the properties are part of the page: they scroll away with
// the text, and the caret goes with it.
func TestPageScrollsAsAWhole(t *testing.T) {
	a, tt := newTestApp(t, longPage())
	tt.SetSize(900, 500)
	tt.Advance(300 * time.Millisecond)
	// the wheel over the text scrolls the page
	tt.Wheel(a.editorR.X+100, 300, 0, 600)
	tt.Advance(300 * time.Millisecond)
	if a.pageScroll.Y < 300 {
		t.Fatalf("the page did not scroll: %v", a.pageScroll.Y)
	}
	if a.pageScroll.Y <= a.headH {
		t.Fatalf("the title did not scroll away: %v, its height %v", a.pageScroll.Y, a.headH)
	}
	// the caret goes to the start, then to the end of a long page, and the
	// page goes after it
	a.doc.SetSelection(0, 0)
	tt.Advance(400 * time.Millisecond)
	tt.Advance(400 * time.Millisecond)
	if a.pageScroll.Y > 1 {
		t.Fatalf("the caret is at the start, the page is at %v", a.pageScroll.Y)
	}
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	tt.Advance(400 * time.Millisecond)
	tt.Advance(400 * time.Millisecond)
	if !a.focused {
		t.Fatal("no focus")
	}
	cr := a.caretWin
	vp := a.scrollR
	if cr.Y < vp.Y || cr.Y+cr.H > vp.Y+vp.H {
		t.Fatalf("the caret %+v is out of the page's room %+v", cr, vp)
	}
	typeEach(tt, "x")
	tt.Advance(300 * time.Millisecond)
	cr = a.caretWin
	if cr.Y < vp.Y || cr.Y+cr.H > vp.Y+vp.H {
		t.Fatalf("typing: the caret %+v is out of %+v", cr, vp)
	}
}

// A long page (1200 paragraphs, three pages linking to it) scrolls as a whole, as any page: the editor is as
// tall as its text from its first frames and never scrolls in itself, its height does not move as the page
// scrolls, and the backlinks are under the text. It was a window on its text past 60000 runes or 800
// paragraphs, scrolling in itself with a bar of its own, the title and backlinks coming and going (#314).
func TestLongPageScrollsAsAWhole(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 1200; i++ {
		fmt.Fprintf(&b, "Paragraf %d: biraz metin, biraz daha metin.\n\n", i)
	}
	a, tt := newTestApp(t, b.String())
	a.clock = tt.Now
	for i := 0; i < 3; i++ { // pages that link to it
		q, err := a.lib.Collections[0].NewPage(nil, fmt.Sprintf("Bağlayan %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if err := q.SaveText("Bkz. [[Sayfa]] üstüne birkaç söz, uzunca bir satır.\n"); err != nil {
			t.Fatal(err)
		}
	}
	a.linksStale()
	if got := len(a.linksTo(a.page, a.clock(), true)); got != 3 {
		t.Fatalf("%d backlinks, want 3", got)
	}
	tt.SetSize(1100, 720)
	settle := func(n int) {
		for i := 0; i < n; i++ {
			tt.Advance(100 * time.Millisecond)
		}
	}
	settle(10)
	paras := a.doc.Text().ParagraphCount()
	whole := func(when string) {
		t.Helper()
		n := len(a.boxes)
		if n == 0 || a.boxes[0].Index != 0 || a.boxes[n-1].Index != paras-1 {
			t.Fatalf("%s: the editor does not hold its whole text: %d boxes of %d paragraphs", when, n, paras)
		}
		if a.boxes[0].Top < 0 {
			t.Fatalf("%s: the editor scrolled in itself: its first paragraph at %v", when, a.boxes[0].Top)
		}
		last := a.boxes[n-1]
		if bottom := last.Top + last.Height; bottom > a.edH {
			t.Fatalf("%s: the text (%v) is taller than the editor (%v)", when, bottom, a.edH)
		}
		if a.linksH == 0 {
			t.Fatalf("%s: no backlinks under the text", when)
		}
	}
	whole("opened")
	h, top := a.edH, a.boxes[0].Top
	if a.pageScroll.MaxY < h/2 {
		t.Fatalf("the page does not scroll as a whole: %v of a text %v tall", a.pageScroll.MaxY, h)
	}
	// the wheel scrolls the page, and nothing in it moves or changes its height
	a.doc.SetSelection(0, 0)
	settle(5)
	prev := a.pageScroll.Y
	for i := 0; i < 8; i++ {
		tt.Wheel(a.editorR.X+100, 400, 0, 2000)
		settle(2)
		whole(fmt.Sprintf("wheel %d", i))
		if a.edH != h || a.boxes[0].Top != top {
			t.Fatalf("wheel %d: the editor %v tall (was %v), its first paragraph at %v (was %v)", i, a.edH, h, a.boxes[0].Top, top)
		}
		if a.pageScroll.Y <= prev && a.pageScroll.Y < a.pageScroll.MaxY {
			t.Fatalf("wheel %d: the page did not scroll on: %v after %v", i, a.pageScroll.Y, prev)
		}
		prev = a.pageScroll.Y
	}
	// the caret at the end: the page takes it into view, and the backlinks are under it
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	settle(10)
	whole("caret at the end")
	if cr, vp := a.caretWin, a.scrollR; cr.Y < vp.Y || cr.Y+cr.H > vp.Y+vp.H {
		t.Fatalf("the caret %+v is out of the page's room %+v", cr, vp)
	}
}

// The header is a region of its own: the page scrolls below it, never under it, and starts as far
// below it as it did; a collection too, and the views with a head of their own leave no room above it.
func TestPageScrollsBelowTheHeader(t *testing.T) {
	a, tt := newTestApp(t, longPage())
	tt.SetSize(900, 500)
	tt.Advance(300 * time.Millisecond)
	if a.scrollR.Y != frameTopH || a.barInset != 0 { // in the window's frame the card has no header: the page starts under its top edge (windowframe.go)
		t.Fatalf("the page scrolls from y=%v under a bar of %v, not below the header", a.scrollR.Y, a.barInset)
	}
	if a.topSpace != pageTop-barH+8 {
		t.Fatalf("the page starts %v below the header, not %v", a.topSpace, pageTop-barH+8)
	}
	tt.Wheel(a.editorR.X+100, frameTopH+10, 0, 300) // just below the top edge
	tt.Advance(300 * time.Millisecond)
	if a.pageScroll.Y < 100 {
		t.Fatalf("the wheel near the top did not scroll the page: %v", a.pageScroll.Y)
	}
	a.coll = a.lib.Collections[0]
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if a.barInset != 0 || a.topSpace != pageTop-barH+8 {
		t.Fatalf("a collection: bar %v, starting %v below", a.barInset, a.topSpace)
	}
	a.tasksOpen = true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if a.barInset != 0 || a.topSpace != 0 {
		t.Fatalf("the tasks have a head of their own: bar %v, top %v", a.barInset, a.topSpace)
	}
}
