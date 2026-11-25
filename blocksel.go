package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Blocks chosen several at a time, from the keyboard: in normal mode (Esc) V chooses the block the caret is in, and
// then j and k (or ↓ ↑) take in the blocks after or before it; d deletes them, y copies them as Markdown, J and K
// move them down and up past a block, Esc lets them go. The blocks are drawn tinted under the text.

// blockSel is the blocks chosen: the first paragraph of the block it began at, and of the one it reaches to.
type blockSel struct {
	on           bool
	anchor, head int
}

// blockSelRange is the text the chosen blocks take, from the start of the first to the end of the last.
func (a *app) blockSelRange(pn *paneState) (int, int) {
	t := pn.doc.Text()
	as, ae := blockRange(t, min(a.bsel.anchor, t.Len()))
	hs, he := blockRange(t, min(a.bsel.head, t.Len()))
	return min(as, hs), max(ae, he)
}

// startBlockSel chooses the block the caret is in.
func (a *app) startBlockSel() {
	pn := a.pane()
	if pn.doc == nil || a.noPage() {
		return
	}
	_, at := pn.doc.Selection()
	s, _ := blockRange(pn.doc.Text(), at)
	a.bsel = blockSel{on: true, anchor: s, head: s}
	if !a.kb.normal {
		a.kbEnter() // the keys are the choice's
	}
}

// blockSelKey does what key r does to the blocks chosen, and reports whether it did anything.
func (a *app) blockSelKey(r rune) bool {
	pn := a.pane()
	if !a.bsel.on || pn.doc == nil || a.noPage() {
		a.bsel.on = false
		return false
	}
	t := pn.doc.Text()
	s, e := a.blockSelRange(pn)
	switch r {
	case 0x1b: // Esc
		a.bsel.on = false
	case 'j', kbDown: // take in the next block
		if _, he := blockRange(t, a.bsel.head); he < t.Len() {
			a.bsel.head, _ = blockRange(t, he+1)
		}
	case 'k', kbUp: // the one before
		if ps, _, ok := prevBlock(t, a.bsel.head); ok {
			a.bsel.head = ps
		}
	case 'y':
		a.copyText(strings.TrimSuffix(bodyMarkdown(t.Slice(s, e)), "\n"))
		a.say(toastOK, locales.BlocksTheBlocksWereCopied())
		a.bsel.on = false
	case 'd', 'x':
		deleteRange(pn.doc, s, e)
		a.bsel.on = false
	case 'J': // past the block after them
		if e < t.Len() {
			_, ne := blockRange(t, e+1)
			at := moveBlockTo(pn.doc, s, e, min(ne+1, pn.doc.Text().Len()+1))
			a.reselect(at, e-s)
		}
	case 'K': // past the block before them
		if ps, _, ok := prevBlock(t, s); ok {
			at := moveBlockTo(pn.doc, s, e, ps)
			a.reselect(at, e-s)
		}
	default:
		return false
	}
	return true
}

// reselect chooses the blocks of n runes that start at at: where the chosen blocks went.
func (a *app) reselect(at, n int) {
	t := a.pane().doc.Text()
	a.bsel.anchor = at
	a.bsel.head, _ = blockRange(t, blockHeadOf(t, min(at+n, t.Len())))
	if a.bsel.head < at {
		a.bsel.head = at
	}
}

// deleteRange takes the paragraphs from s to e away, with the newline after them (or before, at the end).
func deleteRange(doc *richtext.Document, s, e int) {
	t := doc.Text()
	doc.Edit(func(ed *richtext.Edit) {
		switch {
		case e < t.Len():
			ed.Replace(s, e+1, richtext.Text{})
			ed.Select(s, s)
		case s > 0:
			ed.Replace(s-1, e, richtext.Text{})
			ed.Select(s-1, s-1)
		default:
			ed.Replace(s, e, richtext.Text{})
			ed.FormatParagraphs(0, 0, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		}
	})
}

// drawBlockSel tints the chosen blocks, across the column, under the text.
func (a *app) drawBlockSel(pn *paneState, p *ui.Painter, r ui.Rect) {
	if !a.bsel.on || a.isInactivePane(pn) || pn.doc == nil {
		return
	}
	s, e := a.blockSelRange(pn)
	top, bottom := float32(-1), float32(0)
	for _, b := range a.shown(pn, r) {
		if b.Start < s || b.Start > e {
			continue
		}
		if top < 0 {
			top = b.Top
		}
		bottom = b.Top + b.Height
	}
	if top < 0 {
		return
	}
	acc := p.Theme().Accent
	p.Fill(ui.Rect{X: r.X - 8, Y: r.Y + top - 2, W: r.W + 16, H: bottom - top + 4}, acc.Alpha(0.12), rad(6))
	p.Fill(ui.Rect{X: r.X - 8, Y: r.Y + top - 2, W: 3, H: bottom - top + 4}, acc.Alpha(0.7), rad(1.5))
}

// openBlockMenu opens the menu of the block the caret is in, as its grip's click does: from the keyboard.
func (a *app) openBlockMenu() {
	pn := a.pane()
	if pn.doc == nil || a.noPage() {
		return
	}
	_, at := pn.doc.Selection()
	s, _ := blockRange(pn.doc.Text(), at)
	g := &pn.grip
	g.para = s
	g.menu = true
	if a.frame != nil {
		a.frame()
	}
}

// openBlockBeside shows the page in the next pane at the block the caret is in.
func (a *app) openBlockBeside() {
	pn := a.pane()
	if pn.doc == nil || a.noPage() {
		return
	}
	_, at := pn.doc.Selection()
	s, _ := blockRange(pn.doc.Text(), at)
	pl := a.here()
	if phone {
		a.gotoPos(s)
		return
	}
	a.openInSide(pl)
	if a.ps.want >= 0 && a.ps.want < len(a.ps.states) {
		a.gotoPosIn(a.ps.states[a.ps.want], s)
	}
}
