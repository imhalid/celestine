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
	for _, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
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
		t.Fatalf("%d boxes in %s", n, encodeBody(a.pane().doc.Text()))
	}
	s, e := a.pane().doc.Text().ParagraphRange(0)
	a.pane().doc.SetSelection(s, e)
	tt.Key(ui.Cmd, ui.KeyC)
	if a.ownClip.md != "" || a.ownClip.rich.Len() == 0 {
		t.Fatalf("the copy was not noted as text with chips: %+v", a.ownClip)
	}
	a.pane().doc.SetSelection(e, e)
	a.pane().doc.InsertText("\n")
	before := a.pane().doc.Text().String()
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	cs := checksIn(a)
	if len(cs) != 2 || cs[0] == cs[1] {
		t.Fatalf("boxes after the paste: %d, shared %v", len(cs), len(cs) == 2 && cs[0] == cs[1])
	}
	// one undo takes the paste back whole; a redo brings it back with its own box
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got := a.pane().doc.Text().String(); got != before || len(checksIn(a)) != 1 {
		t.Fatalf("one undo left %q, want %q", got, before)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Frame()
	cs = checksIn(a)
	if len(cs) != 2 || cs[0] == cs[1] {
		t.Fatalf("boxes after the redo: %d in %q", len(cs), a.pane().doc.Text().String())
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
		t.Fatalf("after ticking the pasted one: %q", bodyMarkdown(a.pane().doc.Text()))
	}

	// the first box alone, pasted over the word "today" of the second line, selected from its end
	first := -1
	for i, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil && att.Data == any(cs[0]) {
			first = i
			break
		}
	}
	a.pane().doc.SetSelection(first, first+1)
	tt.Key(ui.Cmd, ui.KeyC)
	ps, pe := a.pane().doc.Text().ParagraphRange(e + 1) // the second line
	line := a.pane().doc.Text().Slice(ps, pe).String()
	w := ps + len([]rune(line[:strings.Index(line, "today")]))
	a.pane().doc.SetSelection(w+len("today"), w)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	cs = checksIn(a)
	if len(cs) != 3 || cs[0] == cs[2] || cs[1] == cs[2] {
		t.Fatalf("boxes after the box alone was pasted: %d in %q", len(cs), bodyMarkdown(a.pane().doc.Text()))
	}
	tick(cs[2])
	cs = checksIn(a)
	if cs[0].target != "" || cs[1].target != "x" || cs[2].target != "x" {
		t.Fatalf("after ticking the box pasted alone: %q", bodyMarkdown(a.pane().doc.Text()))
	}
	if md := bodyMarkdown(a.pane().doc.Text()); !strings.Contains(md, "buy milk [ ] today") {
		t.Fatalf("the first line changed: %q", md)
	}
}

// A block duplicated (⌘D) has chips of its own: ticking the copy's task leaves the first (#539).
func TestDuplicatedInlineTaskOwnBox(t *testing.T) {
	a, tt := newTestApp(t, "buy milk [ ] today\n")
	tt.Advance(100 * time.Millisecond)
	a.pane().doc.SetSelection(2, 2)
	tt.Key(ui.Cmd, ui.KeyD)
	tt.Frame()
	cs := checksIn(a)
	if len(cs) != 2 || cs[0] == cs[1] {
		t.Fatalf("boxes after ⌘D: %d in %q", len(cs), bodyMarkdown(a.pane().doc.Text()))
	}
	a.toggleCheckChip(cs[1])
	tt.Frame()
	if cs = checksIn(a); cs[0].target != "" || cs[1].target != "x" {
		t.Fatalf("after ticking the copy: %q", bodyMarkdown(a.pane().doc.Text()))
	}
}

// A fold pasted holds chips of its own in what it hides (#539).
func TestPastedFoldOwnsItsChips(t *testing.T) {
	a, tt := newTestApp(t, "title\n")
	a.clipRead = func() string { return tt.Clipboard() }
	tt.Advance(100 * time.Millisecond)
	hidden := a.parse("milk [ ] today\n")
	fold := &chip{kind: "fold", hidden: &hidden, lines: 1}
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(5, 5, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: a.attachment(fold)})
	})
	tt.Frame()
	s, e := a.pane().doc.Text().ParagraphRange(0)
	a.pane().doc.SetSelection(s, e)
	tt.Key(ui.Cmd, ui.KeyC)
	a.pane().doc.SetSelection(e, e)
	a.pane().doc.InsertText("\n")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	var folds []*chip
	for _, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "fold" {
				folds = append(folds, ch)
			}
		}
	}
	if len(folds) != 2 || folds[0] == folds[1] || folds[0].hidden == folds[1].hidden {
		t.Fatalf("folds after the paste: %d", len(folds))
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
	if x, y := inner(*folds[0].hidden), inner(*folds[1].hidden); x == nil || x == y {
		t.Fatal("the folds' hidden tasks share a box")
	}
}
