package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// TestChipsKept: links read as chips and written back as they were.
func TestChipsKept(t *testing.T) {
	a, _ := newTestApp(t, "")
	md := "Bkz. [[Dune]], [[Dune|kitap]] ve [[2026-11-05]]; `[[kodda]]` kalır."
	tx := a.parse(md + "\n")
	if n := strings.Count(tx.String(), string(richtext.ObjectReplacement)); n != 3 {
		t.Fatalf("chips: %d in %q", n, tx.String())
	}
	if got := strings.TrimSpace(bodyMarkdown(tx)); got != md {
		t.Fatalf("written: %q", got)
	}
}

// chipAt is the center of the first chip drawn whose label is label.
func chipAt(t *testing.T, a *app, label string) (float32, float32) {
	t.Helper()
	for _, h := range a.chipHits {
		if h.ch.label() == label {
			return h.r.X + h.r.W/2, h.r.Y + h.r.H/2
		}
	}
	t.Fatalf("no chip %q among %d", label, len(a.chipHits))
	return 0, 0
}

// TestAtMenu links a page, a date and sets a choice from the @ menu; a
// click on a chip opens its page, or makes it.
func TestAtMenu(t *testing.T) {
	a, tt := newTestApp(t, "")
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	coll := a.pane().page.Coll
	dune, _ := coll.NewPage(nil, "Dune")
	coll.AddProperty("Durum", PropChoice, []string{"Açık", "Bitti"})
	tt.Frame()
	typeEach(tt, "Oku @dun")
	tt.Frame()
	if !a.slash.open || a.slash.sign != "@" {
		t.Fatalf("not open: %+v", a.slash)
	}
	items := a.menuItems(a.slashQuery())
	if len(items) == 0 || items[0].title != "Dune" {
		t.Fatalf("items: %+v", items)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "ve @yar")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "@bitti")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := strings.TrimSpace(bodyMarkdown(a.pane().doc.Text())); got != "Oku [[Dune]] ve [[2026-11-06]]" {
		t.Fatalf("text: %q", got)
	}
	if a.pane().page.Props["durum"] != "Bitti" {
		t.Fatalf("choice: %+v", a.pane().page.Props)
	}
	// a click on the page's chip opens it
	tt.Advance(100 * time.Millisecond)
	x, y := chipAt(t, a, "Dune")
	tt.ClickAt(x, y)
	tt.Frame()
	if a.pane().page != dune {
		t.Fatalf("opened %v", a.here())
	}
	// a link to a page not there makes it
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	tt.Frame()
	a.pane().doc.SetSelection(a.pane().doc.Text().Len(), a.pane().doc.Text().Len())
	a.insertChip(a.pane().doc.Text().Len(), &chip{kind: "page", target: "Yeni fikir"})
	tt.Advance(100 * time.Millisecond)
	x, y = chipAt(t, a, "Yeni fikir")
	tt.ClickAt(x, y)
	tt.Frame()
	if a.pane().page == nil || a.pane().page.Title != "Yeni fikir" || a.pane().page.Coll != coll {
		t.Fatalf("made: %v", a.here())
	}
}

// A chip is as wide as its text: sized by a letter's width taken as a constant
// it was long at its end (narrow letters) or short of it (wide ones).
func TestChipIsAsWideAsItsText(t *testing.T) {
	a, _ := newTestApp(t, "x")
	narrow := a.attachment(chipOf("illliiilllii iiilll"))
	wide := a.attachment(chipOf("MMMWWW MMMWWW"))
	text := func(label string) float32 {
		w, _ := ui.ShapeRichTextWith([]ui.Span{{Text: label, Size: chipSize, Weight: 520}}, a.font, ui.TextShape{}).Size()
		return w
	}
	for _, c := range []struct {
		label string
		got   float32
	}{{"illliiilllii iiilll", narrow.Width}, {"MMMWWW MMMWWW", wide.Width}} {
		tw := text(c.label)
		if c.got < tw+2*(chipPad-1) || c.got > tw+2*chipPad+2 {
			t.Errorf("chip %q is %v wide, its text %v: want it %v..%v", c.label, c.got, tw, tw+2*(chipPad-1), tw+2*chipPad+2)
		}
	}
	if narrow.Width >= wide.Width {
		t.Errorf("narrow letters (%v) are not narrower than wide ones (%v)", narrow.Width, wide.Width)
	}
}
