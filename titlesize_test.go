package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A long title wraps rather than being cut at the page's edge, and is smaller the longer it is; it stays one line of
// text: ↵ goes into the text and a pasted break is a space (#268).
func TestLongTitleWraps(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	tt.SetSize(900, 700)
	tt.Advance(300 * time.Millisecond)
	short, ok := tt.Find("Başlık")
	if !ok {
		t.Fatalf("no title: %v", tt.Texts())
	}
	if a.pane().titleW <= 0 || a.titleSize(a.pane().title, a.pane().titleW) != titleSizes[0] {
		t.Fatalf("a short title: room %v, size %v", a.pane().titleW, a.titleSize(a.pane().title, a.pane().titleW))
	}
	// the title is measured at the width the area lays its lines out in: its box less the caret's column
	if a.pane().titleW != short.W-1 {
		t.Fatalf("the title measured at %v in a box %v wide", a.pane().titleW, short.W)
	}
	tt.ClickAt(short.X+20, short.Y+short.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	two := "Cognitive Biases and Attentional Influences on Memory"
	tt.Type(two)
	tt.Advance(100 * time.Millisecond)
	mid, _ := tt.Find("Başlık")
	size := a.titleSize(a.pane().title, a.pane().titleW)
	if a.pane().page.Title != two || size >= titleSizes[0] || mid.H <= short.H {
		t.Fatalf("a long title: %q at %v, %v high (a short one %v)", a.pane().page.Title, size, mid.H, short.H)
	}
	if n := len(a.titleLayout(a.pane().title, size, a.pane().titleW).Lines()); n < 2 {
		t.Fatalf("the long title is on %d line", n)
	}
	tt.Type(strings.Repeat(" and the rest of a very long title", 4))
	tt.Advance(100 * time.Millisecond)
	if s := a.titleSize(a.pane().title, a.pane().titleW); s != titleSizes[len(titleSizes)-1] {
		t.Fatalf("a very long title at %v", s)
	}
	// ↵ goes into the text and leaves no break in the title
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if strings.ContainsAny(a.pane().page.Title, "\n\r") || strings.ContainsAny(a.pane().title, "\n\r") || a.pane().titleFocused {
		t.Fatalf("after ↵: %q, the title focused %v", a.pane().page.Title, a.pane().titleFocused)
	}
	// the area holds the title as it is: what is typed next has no break left in it
	r, _ := tt.Find("Başlık")
	tt.ClickAt(r.X+20, r.Y+12)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyRight) // to its end
	tt.Type("!")
	tt.Advance(100 * time.Millisecond)
	if !strings.HasSuffix(a.pane().page.Title, "long title!") || strings.Contains(a.pane().page.Title, "\n") {
		t.Fatalf("typed after ↵: %q", a.pane().page.Title)
	}
	// a break pasted is a space
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("İki\nsatır")
	tt.Advance(100 * time.Millisecond)
	if a.pane().page.Title != "İki satır" {
		t.Fatalf("a break in the title: %q", a.pane().page.Title)
	}
}
