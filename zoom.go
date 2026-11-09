package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// Zooming into a block: the page shows only that block and what is under it (a heading's section, a list item's
// children). richtext shows no part of a text alone, so what is above the block and what is below it are taken out
// behind two chips, one at the top and one at the bottom, as a fold is; a click on either, or Zoom out, puts the
// page back as it was a level up. The file is always written whole (withoutChips), and a zoom is not remembered
// when the page is closed. Zooming in again, inside a zoom, goes a level deeper.

// zoomIn zooms into the block holding rune at.
func (a *app) zoomIn(at int) {
	t := a.doc.Text()
	bs, be := blockRange(t, at)
	n := t.Len()
	if bs == 0 && be >= n { // the block is the whole page
		return
	}
	_, caret := a.doc.Selection()
	plain := func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} }
	a.doc.Edit(func(ed *richtext.Edit) {
		if be < n {
			below := t.Slice(be+1, n)
			ch := &chip{kind: "zoom", hidden: &below, lines: strings.Count(below.String(), "\n") + 1}
			ed.Replace(be+1, n, richtext.Text{})
			ed.ReplaceString(be+1, be+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: a.attachment(ch)})
			ed.FormatParagraphs(be+1, be+1, plain)
		}
		if bs > 0 {
			above := t.Slice(0, bs-1)
			ch := &chip{kind: "zoom", top: true, hidden: &above, lines: strings.Count(above.String(), "\n") + 1}
			ed.Replace(0, bs-1, richtext.Text{})
			ed.ReplaceString(0, 0, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: a.attachment(ch)})
			ed.FormatParagraphs(0, 0, plain)
		}
		c := min(max(caret, bs), be) - bs
		if bs > 0 {
			c += 2
		}
		ed.Select(c, c)
	})
}

// zoomed reports whether the page is zoomed into a block.
func (a *app) zoomed() bool {
	if a.doc == nil {
		return false
	}
	top, end, _, _ := zoomChips(a.doc.Text())
	return top != nil || end != nil
}

// zoomChips are the chips of the zoom shown: the one at the top, the one at the bottom, and where they are.
func zoomChips(t richtext.Text) (top, end *chip, ti, ei int) {
	ti, ei = -1, -1
	if t.Len() == 0 {
		return
	}
	if ch := chipIn(t, 0); ch != nil && ch.kind == "zoom" && ch.top {
		top, ti = ch, 0
	}
	if i := t.Len() - 1; i > 0 {
		if ch := chipIn(t, i); ch != nil && ch.kind == "zoom" && !ch.top {
			end, ei = ch, i
		}
	}
	return
}

// chipIn is the chip at rune i of t, or nil.
func chipIn(t richtext.Text, i int) *chip {
	if att := t.AttributesAt(i).Attachment; att != nil {
		if ch, ok := att.Data.(*chip); ok {
			return ch
		}
	}
	return nil
}

// zoomOut goes a level up: the page as it was before the last zoom.
func (a *app) zoomOut() {
	t := a.doc.Text()
	top, end, ti, ei := zoomChips(t)
	if top == nil && end == nil {
		return
	}
	_, caret := a.doc.Selection()
	a.doc.Edit(func(ed *richtext.Edit) {
		// a chip is a paragraph of its own: the text it holds takes its place
		if end != nil {
			editReplaceKeepIDs(ed, ei, ei+1, *end.hidden)
		}
		if top != nil {
			editReplaceKeepIDs(ed, ti, ti+1, *top.hidden)
			caret += top.hidden.Len() - 1
		}
		caret = min(max(caret, 0), ed.Text().Len())
		ed.Select(caret, caret)
	})
}

// replaceKeepIDs is t.Replace(s, e, with), the paragraphs put in keeping their IDs: Replace gives them none, and
// the app's IDs say what a block is (a code block's language, a task's kind, a divider). The first paragraph of
// with keeps its ID only when it starts a paragraph, not when it joins the one at s.
func replaceKeepIDs(t richtext.Text, s, e int, with richtext.Text) richtext.Text {
	ps, _ := t.ParagraphRange(s)
	r := t.Replace(s, e, with)
	for off, p := range with.Paragraphs() {
		if p.Style.ID == "" || off == 0 && ps != s {
			continue
		}
		id := p.Style.ID
		r = r.FormatParagraphs(s+off, s+off, func(st *richtext.ParagraphStyle) { st.ID = id })
	}
	return r
}

// editReplaceKeepIDs is replaceKeepIDs in an edit of a document.
func editReplaceKeepIDs(ed *richtext.Edit, s, e int, with richtext.Text) {
	ps, _ := ed.Text().ParagraphRange(s)
	ed.Replace(s, e, with)
	for off, p := range with.Paragraphs() {
		if p.Style.ID == "" || off == 0 && ps != s {
			continue
		}
		id := p.Style.ID
		ed.FormatParagraphs(s+off, s+off, func(st *richtext.ParagraphStyle) { st.ID = id })
	}
}
