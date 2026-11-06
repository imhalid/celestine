package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Folding. richtext has no paragraph that is not shown, so a block folded
// has what is under its line taken out of the text and held in a chip at
// the end of the line ("⋯ 5 satır"); a click on the chip, or the block's
// menu, puts it back, and the file is written with it back (withoutChips),
// so a fold is never in the file. A fold is the editor's, not the page's: it
// is not remembered when the page is closed.

// blockHasKids reports whether the block at at has lines under it to fold.
func blockHasKids(t richtext.Text, at int) bool {
	_, he := t.ParagraphRange(at)
	hs, _ := t.ParagraphRange(at)
	_, be := blockRange(t, hs)
	return be > he
}

// foldOf is the fold chip on the line at at, or nil.
func foldOf(t richtext.Text, at int) *chip {
	s, e := t.ParagraphRange(at)
	for _, r := range t.Runs(s, e) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "fold" {
				return ch
			}
		}
	}
	return nil
}

// fold takes what is under the line at at out, behind a chip.
func (a *app) fold(at int) {
	t := a.doc.Text()
	hs, he := t.ParagraphRange(at)
	if foldOf(t, at) != nil || !blockHasKids(t, at) {
		return
	}
	_, be := blockRange(t, hs)
	if be >= t.Len() { // the block ends the text: a line after it to hold the end
		n := t.Len()
		a.doc.Edit(func(ed *richtext.Edit) {
			ed.Replace(n, n, richtext.NewText("\n"))
			ed.FormatParagraphs(n+1, n+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		})
		t = a.doc.Text()
	}
	hidden := t.Slice(he+1, be+1)
	ch := &chip{kind: "fold", hidden: &hidden, lines: strings.Count(hidden.String(), "\n")}
	attrs := t.AttributesAt(hs)
	attrs.Attachment = a.attachment(ch)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(he+1, be+1, richtext.Text{})
		ed.ReplaceString(he, he, string(richtext.ObjectReplacement), attrs)
		ed.Select(he, he)
	})
}

// unfold puts back what the chip holds.
func (a *app) unfold(ch *chip) {
	i := a.chipPos(ch)
	if i < 0 || ch.hidden == nil {
		return
	}
	_, pe := a.doc.Text().ParagraphRange(i)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(pe+1, pe+1, *ch.hidden)
		ed.Replace(i, i+1, richtext.Text{})
		ed.Select(i, i)
	})
}

// toggleFold folds the block the caret is in, or unfolds it.
func (a *app) toggleFold(at int) {
	if ch := foldOf(a.doc.Text(), at); ch != nil {
		a.unfold(ch)
		return
	}
	a.fold(at)
}

var iconFoldOpen = icon(`<path d="m6.5 9.5 5.5 5.5 5.5-5.5"/>`)

var _ = ui.Rect{}

// unfold puts back what every fold in t holds, as a page is kept with none.
func unfold(t richtext.Text) richtext.Text {
	for {
		found := false
		for i, r := range t.Runs(0, t.Len()) {
			att := r.Attributes.Attachment
			if att == nil {
				continue
			}
			if ch, ok := att.Data.(*chip); ok && ch.kind == "fold" {
				_, pe := t.ParagraphRange(i)
				t = t.Replace(pe+1, pe+1, unfold(*ch.hidden))
				t = t.Replace(i, i+1, richtext.Text{})
				found = true
				break
			}
		}
		if !found {
			return t
		}
	}
}
