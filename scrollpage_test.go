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
