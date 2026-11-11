package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
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
		if pe >= ed.Text().Len() { // the line after the fold is gone: one for what it holds, not glued to its line
			ed.Replace(pe, pe, richtext.NewText("\n"))
			ed.FormatParagraphs(pe+1, pe+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		}
		editReplaceKeepIDs(ed, pe+1, pe+1, *ch.hidden)
		ed.Replace(i, i+1, richtext.Text{})
		ed.Select(i, i)
	})
	a.prevText = a.doc.Text() // the chip went because what it held is back: not one to put back (keepHidden)
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

// unfold puts back what every fold and zoom in t holds, as a page is kept with none.
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
				if pe >= t.Len() { // the line after the fold is gone: one for what it holds, not glued to its line
					t = t.Append("\n", richtext.Attributes{})
					t = t.FormatParagraphs(pe+1, pe+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
				}
				t = replaceKeepIDs(t, pe+1, pe+1, unfold(*ch.hidden))
				t = t.Replace(i, i+1, richtext.Text{})
				found = true
				break
			}
			if ch, ok := att.Data.(*chip); ok && ch.kind == "zoom" { // what is above or below the block, in the chip's place
				t = replaceKeepIDs(t, i, i+1, unfold(*ch.hidden))
				found = true
				break
			}
		}
		if !found {
			return t
		}
	}
}

// heldChip is a fold or zoom chip of a text, and where it is.
type heldChip struct {
	ch *chip
	at int
}

// heldChips are the fold and zoom chips of t, the text they hold with them.
func heldChips(t richtext.Text) []heldChip {
	var out []heldChip
	for i, r := range t.Runs(0, t.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && (ch.kind == "fold" || ch.kind == "zoom") && ch.hidden != nil {
				out = append(out, heldChip{ch, i})
			}
		}
	}
	return out
}

// keepHidden puts back what a fold or zoom chip held when an edit took the chip away: Backspace over a fold, a
// line deleted with its chip, ⌘A and a key while zoomed. A chip is one character holding lines of the page, and
// taking it away took them with it, without a word (#128). What a fold held goes back under the line it was on;
// what a zoom held, above or below what is there now. It says whether it put anything back.
func (a *app) keepHidden(prev, now richtext.Text, caret int) bool {
	if !strings.ContainsRune(prev.String(), richtext.ObjectReplacement) {
		return false
	}
	still := map[*richtext.Text]bool{} // the chips there, and those held in their texts: a fold folded into another
	var note func(t richtext.Text)
	note = func(t richtext.Text) {
		for _, h := range heldChips(t) {
			if !still[h.ch.hidden] {
				still[h.ch.hidden] = true
				note(*h.ch.hidden)
			}
		}
	}
	note(now)
	var folds []heldChip
	var top, end *chip
	for _, h := range heldChips(prev) {
		switch {
		case still[h.ch.hidden]:
		case h.ch.kind == "fold":
			folds = append(folds, h)
		case h.ch.top:
			top = h.ch
		default:
			end = h.ch
		}
	}
	if len(folds) == 0 && top == nil && end == nil {
		return false
	}
	paras := paragraphsOf(now)
	grow, from := now.Len()-prev.Len(), min(a.caretAt, caret)
	caretPara, shift := paraIndex(now, caret), 0
	// from the last to the first, so that what goes in does not move where the next goes
	for i := len(folds) - 1; i >= 0; i-- {
		h := folds[i]
		pos := h.at
		if pos >= from {
			pos = max(pos+grow, from)
		}
		k := paraIndex(now, min(max(pos, 0), now.Len()))
		hidden := paragraphsOf(*h.ch.hidden)
		if n := len(hidden); n > 0 && strings.HasSuffix(h.ch.hidden.String(), "\n") && len(hidden[n-1].Runs) == 0 {
			hidden = hidden[:n-1]
		}
		paras = slices.Insert(paras, min(k+1, len(paras)), hidden...)
		if k < caretPara {
			shift += h.ch.hidden.Len()
		}
	}
	if top != nil {
		paras = append(paragraphsOf(*top.hidden), paras...)
		shift += top.hidden.Len() + 1
	}
	if end != nil {
		paras = append(paras, paragraphsOf(*end.hidden)...)
	}
	full := richtext.TextOf(paras...)
	styles := paraStyles(full)
	n := now.Len()
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(0, n, full)
		restyle(ed, 0, styles)
		at := min(caret+shift, full.Len())
		ed.Select(at, at)
	})
	a.notice = locales.BlocksTheLinesHeldWere()
	return true
}
