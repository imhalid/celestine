package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// checksIn is the boxes of the tasks in the running text of a's page, in order.
func checksIn(a *app) []*chip {
	var out []*chip
	for _, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "check" {
				out = append(out, ch)
			}
		}
	}
	return out
}

// A chip copied shares nothing with the one it was copied from: a table's rows, a fold's hidden text, and the chips
// in that text are their own (#539).
func TestCopyChipOwnsWhatItHolds(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	tb := a.tableChip(newTable(2, 1))
	tb.tbl.setCell(0, 0, "a")
	tc := a.copyChip(tb)
	tc.tbl.setCell(0, 0, "b")
	if tb.tbl == tc.tbl || tb.tbl.cell(0, 0) != "a" || tc.tbl.cell(0, 0) != "b" {
		t.Fatalf("the table's rows are shared: %q %q", tb.tbl.cell(0, 0), tc.tbl.cell(0, 0))
	}
	hidden := a.parse("milk [ ] today\n")
	fold := &chip{kind: "fold", hidden: &hidden, lines: 1}
	fc := a.copyChip(fold)
	if fc.hidden == fold.hidden || fc.hidden.String() != fold.hidden.String() {
		t.Fatal("the fold's hidden text is shared, or not the same")
	}
	inner := func(tx richtext.Text) *chip {
		for _, r := range tx.Runs(0, tx.Len()) {
			if att := r.Attributes.Attachment; att != nil {
				if ch, ok := att.Data.(*chip); ok {
					return ch
				}
			}
		}
		return nil
	}
	if a, b := inner(*fold.hidden), inner(*fc.hidden); a == nil || a == b || b.kind != "check" {
		t.Fatalf("the chip in the fold: %v and %v", a, b)
	}
}

// A line with a task in its running text, copied and pasted in the editor, has a box of its own: ticking the
// pasted one leaves the first as it was. So does the box copied alone, and pasted over a selection (#539).
func TestPastedInlineTaskOwnBox(t *testing.T) {
	a, tt := newTestApp(t, "buy milk [ ] today\n")
	a.clipRead = func() string { return tt.Clipboard() }
	tt.Advance(100 * time.Millisecond)
	if n := len(checksIn(a)); n != 1 {
		t.Fatalf("%d boxes in %s", n, encodeBody(a.doc.Text()))
	}
	s, e := a.doc.Text().ParagraphRange(0)
	a.doc.SetSelection(s, e)
	tt.Key(ui.Cmd, ui.KeyC)
	if a.ownClip.md != "" {
		t.Fatal("the copy was noted as blocks: the editor's own paste is not what is tested")
	}
	a.doc.SetSelection(e, e)
	a.doc.InsertText("\n")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	cs := checksIn(a)
	if len(cs) != 2 || cs[0] == cs[1] {
		t.Fatalf("boxes after the paste: %d, shared %v", len(cs), len(cs) == 2 && cs[0] == cs[1])
	}
	tick := func(ch *chip) {
		t.Helper()
		tt.Frame()
		for _, h := range a.chipHits {
			if h.ch == ch {
				tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
				tt.Frame()
				return
			}
		}
		t.Fatalf("the box is not drawn among %d chips", len(a.chipHits))
	}
	tick(cs[1])
	cs = checksIn(a)
	if len(cs) != 2 || cs[0].target != "" || cs[1].target != "x" {
		t.Fatalf("after ticking the pasted one: %q", bodyMarkdown(a.doc.Text()))
	}

	// the first box alone, pasted over the word "today" of the second line, selected from its end
	first := -1
	for i, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil && att.Data == any(cs[0]) {
			first = i
			break
		}
	}
	a.doc.SetSelection(first, first+1)
	tt.Key(ui.Cmd, ui.KeyC)
	ps, pe := a.doc.Text().ParagraphRange(e + 1) // the second line
	line := a.doc.Text().Slice(ps, pe).String()
	w := ps + len([]rune(line[:strings.Index(line, "today")]))
	a.doc.SetSelection(w+len("today"), w)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	cs = checksIn(a)
	if len(cs) != 3 || cs[0] == cs[2] || cs[1] == cs[2] {
		t.Fatalf("boxes after the box alone was pasted: %d in %q", len(cs), bodyMarkdown(a.doc.Text()))
	}
	tick(cs[2])
	cs = checksIn(a)
	if cs[0].target != "" || cs[1].target != "x" || cs[2].target != "x" {
		t.Fatalf("after ticking the box pasted alone: %q", bodyMarkdown(a.doc.Text()))
	}
	if md := bodyMarkdown(a.doc.Text()); !strings.Contains(md, "buy milk [ ] today") {
		t.Fatalf("the first line changed: %q", md)
	}
}
