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

// A huge page (20000 lines) is a window: the editor is as high as the page's
// room and scrolls the text in it. Each frame lays out and paints every
// paragraph in the editor's height, and that height once grew frame after frame
// until the whole text was in (seconds a frame); and a document begins with its
// caret at the end, which a window shorter than its text scrolls itself to. It
// opens at its top, scrolls on to the end, and the caret goes there and back.
func TestHugePageIsAWindow(t *testing.T) {
	var b strings.Builder
	const lines = 20000
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "- Madde %d: biraz metin, biraz daha metin.\n", i)
	}
	a, tt := newTestApp(t, b.String())
	a.clock = tt.Now // the time the frames run on
	tt.SetSize(900, 700)
	settle := func(n int) {
		for i := 0; i < n; i++ {
			tt.Advance(100 * time.Millisecond)
		}
	}
	inView := func(what string) (first, last int) {
		t.Helper()
		n := len(a.boxes)
		if n == 0 || n > 100 {
			t.Fatalf("%s: %d paragraphs laid out of %d", what, n, lines)
		}
		return a.boxes[0].Index, a.boxes[n-1].Index
	}
	settle(20)
	if first, _ := inView("opened"); first != 0 {
		t.Fatalf("opened at paragraph %d, not at its top", first)
	}
	if an, ca := a.doc.Selection(); an != 0 || ca != 0 {
		t.Fatalf("opened with the caret at %d-%d, not at its start", an, ca)
	}
	if a.winScrolled || a.headH == 0 {
		t.Fatalf("at the top the title is shown: scrolled %v, its height %v", a.winScrolled, a.headH)
	}
	if a.pageScroll.MaxY > 1 {
		t.Fatalf("the page around the window scrolls too (%v), a second bar", a.pageScroll.MaxY)
	}
	// the wheel takes the text on, and what is in view is all that is laid out
	prev := 0
	for i := 0; i < 5; i++ {
		tt.Wheel(a.editorR.X+100, 300, 0, 3000)
		settle(3)
		first, _ := inView("scrolled")
		if first <= prev {
			t.Fatalf("the wheel did not take the text on: %d after %d", first, prev)
		}
		prev = first
	}
	if !a.winScrolled || a.headH != 0 {
		t.Fatalf("scrolled, the title gives way: scrolled %v, its height %v", a.winScrolled, a.headH)
	}
	if a.pageScroll.MaxY > 1 {
		t.Fatalf("the page around the window scrolls too (%v), a second bar", a.pageScroll.MaxY)
	}
	// the caret to the end: the last line is in view, nothing more is laid out
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	settle(30)
	if _, last := inView("caret at the end"); last != lines-1 {
		t.Fatalf("the end of the page is not in view: last paragraph %d of %d", last, lines)
	}
	// and back to the start
	a.doc.SetSelection(0, 0)
	settle(30)
	if first, _ := inView("caret at the start"); first != 0 {
		t.Fatalf("the start of the page is not in view: first paragraph %d", first)
	}
	if a.winScrolled || a.headH == 0 {
		t.Fatalf("back at the top the title comes back: scrolled %v, its height %v", a.winScrolled, a.headH)
	}
}

// A page of many short lines is a window too, though it is short in runes.
func TestManyLinesPageIsAWindow(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 5000; i++ {
		fmt.Fprintf(&b, "- satır %d\n", i)
	}
	a, tt := newTestApp(t, b.String())
	tt.SetSize(900, 700)
	for i := 0; i < 20; i++ {
		tt.Advance(100 * time.Millisecond)
	}
	if n := len(a.boxes); n == 0 || n > 100 {
		t.Fatalf("%d paragraphs laid out of 5000", n)
	}
	if !hugePage(a.doc.Text()) {
		t.Fatal("5000 lines are not a huge page")
	}
}

// The page scrolls from the top of the window, under the title bar, which lies
// over it: the text is not cut off below an empty band, and the wheel works
// over the bar too; at the top the title stands where it did.
func TestPageScrollsUnderTheTitleBar(t *testing.T) {
	a, tt := newTestApp(t, longPage())
	tt.SetSize(900, 500)
	tt.Advance(300 * time.Millisecond)
	if a.scrollR.Y != 0 || a.barInset != barH {
		t.Fatalf("the page scrolls from y=%v under a bar of %v, not from the top", a.scrollR.Y, a.barInset)
	}
	if a.topSpace != pageTop+8 {
		t.Fatalf("the page starts %v below the top, where it did: %v", a.topSpace, pageTop+8)
	}
	tt.Wheel(a.editorR.X+100, 40, 0, 300) // over the top of the page, below the strip that moves the window
	tt.Advance(300 * time.Millisecond)
	if a.pageScroll.Y < 100 {
		t.Fatalf("the wheel near the top did not scroll the page: %v", a.pageScroll.Y)
	}
	// a collection, with no page, keeps its bar above it
	a.coll = a.lib.Collections[0]
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if a.barInset != 0 || a.topSpace != 0 {
		t.Fatalf("a collection is under a bar of its own: %v %v", a.barInset, a.topSpace)
	}
}
