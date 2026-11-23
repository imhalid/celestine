package main

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
)

// Blocks: a paragraph and what is under it - a list item and the items
// indented deeper after it, a heading and what follows it up to the next
// heading of its level or above, a run of code or of a quotation. A block
// moves, is duplicated and deleted whole (blockui.go).

// under reports whether a paragraph of style p belongs to the block of a
// paragraph of style parent, right before it.
func under(parent, p richtext.ParagraphStyle) bool {
	switch {
	case parent.Heading > 0:
		return p.Heading == 0 || p.Heading > parent.Heading
	case parent.CodeBlock:
		return p.CodeBlock && !strings.HasPrefix(p.ID, codeIDPrefix) && !strings.HasPrefix(p.ID, rawIDPrefix)
	case parent.Quote:
		return p.Quote
	case parent.List != richtext.NoList:
		return p.Indent > parent.Indent
	}
	return false
}

// blockHeadOf is the start of the first paragraph of the block the paragraph at start is in,
// when that is a run of code or of a quotation: its lines are one block.
func blockHeadOf(t richtext.Text, start int) int {
	s, _ := t.ParagraphRange(min(start, t.Len()))
	for s > 0 {
		cur := t.ParagraphStyleAt(s)
		if !cur.CodeBlock && !cur.Quote {
			break
		}
		ps, _ := t.ParagraphRange(s - 1)
		if prev := t.ParagraphStyleAt(ps); (prev.CodeBlock || prev.Quote) && under(prev, cur) {
			s = ps
			continue
		}
		break
	}
	return s
}

// blockRange is the block starting at the paragraph holding rune at, or the run of code or quotation it is a
// line of: from its start to the end of its last paragraph (without the newline after), the paragraphs under
// it with it. A paragraph inside a heading's section is a block of its own; one inside a list item's is its
// own item's.
func blockRange(t richtext.Text, at int) (int, int) {
	s, e := t.ParagraphRange(blockHeadOf(t, at))
	st := t.ParagraphStyleAt(s)
	if e >= t.Len() || !under(st, t.ParagraphStyleAt(e+1)) {
		return s, e // a block of one paragraph, the most of them
	}
	// A block of more is walked in one pass: finding each paragraph in the tree on its own costs a page of
	// thousands of paragraphs a second when the caret is in a heading over all of them (#506).
	for ps, p := range t.Paragraphs() {
		if ps <= e {
			continue
		}
		if !under(st, p.Style) {
			break
		}
		// a heading's block runs on through what is under the paragraphs in it
		e = ps + paraLen(p)
	}
	return s, e
}

// paraLen is the length of a paragraph in runes, without its newline.
func paraLen(p richtext.Paragraph) int {
	n := 0
	for _, r := range p.Runs {
		n += utf8.RuneCountInString(r.Text)
	}
	return n
}

// prevBlock is the block right before the one starting at s: the nearest
// block ending just before it, or ok false at the start.
func prevBlock(t richtext.Text, s int) (int, int, bool) {
	if s == 0 {
		return 0, 0, false
	}
	for q := s - 1; q >= 0; {
		qs, _ := t.ParagraphRange(q)
		if bs, be := blockRange(t, qs); be == s-1 {
			// the outermost: a heading above a list holds the list
			for qs > 0 {
				ps, _ := t.ParagraphRange(qs - 1)
				if os, oe := blockRange(t, ps); oe == s-1 && os < bs {
					bs = os
				}
				qs = ps
			}
			return bs, be, true
		}
		q = qs - 1
	}
	return 0, 0, false
}

// swapBlocks puts block [b0, b1) before block [a0, a1), which ends just
// before it, as a text to replace [a0, b1) with.
func swapBlocks(t richtext.Text, a0, a1, b0, b1 int) richtext.Text {
	y := t.Slice(b0, b1).Append("\n", richtext.Attributes{})
	n := y.Len()
	y = y.Replace(n, n, t.Slice(a0, a1))
	st := t.ParagraphStyleAt(a0)
	return y.FormatParagraphs(n, n, func(p *richtext.ParagraphStyle) { *p = st })
}

// moveBlock moves the block holding the caret up (-1) or down (1), past
// the block next to it; the caret moves with it.
func moveBlock(doc *richtext.Document, dir int) bool {
	t := doc.Text()
	_, caret := doc.Selection()
	s, e := blockRange(t, caret)
	var a0, a1, b0, b1 int
	if dir < 0 {
		ps, pe, ok := prevBlock(t, s)
		if !ok {
			return false
		}
		a0, a1, b0, b1 = ps, pe, s, e
	} else {
		if e >= t.Len() {
			return false
		}
		ns, ne := blockRange(t, e+1)
		a0, a1, b0, b1 = s, e, ns, ne
	}
	moved := swapBlocks(t, a0, a1, b0, b1)
	off := caret - s
	styles := paraStyles(t)
	i0, i1, j0, j1 := paraIndex(t, a0), paraIndex(t, a1), paraIndex(t, b0), paraIndex(t, b1)
	order := append(slices.Clone(styles[j0:j1+1]), styles[i0:i1+1]...)
	doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(a0, b1, moved)
		restyle(ed, i0, order)
		if dir < 0 {
			ed.Select(a0+off, a0+off)
		} else {
			at := a0 + (b1 - b0) + 1 + off
			ed.Select(at, at)
		}
	})
	return true
}

// moveBlockTo moves block [s, e) to before the paragraph starting at to
// (to == t.Len()+1: to the end), and returns where it starts there.
func moveBlockTo(doc *richtext.Document, s, e, to int) int {
	t := doc.Text()
	if to >= s && to <= e+1 {
		return s // onto itself
	}
	blk := t.Slice(s, e)
	n := e - s
	at := s
	styles := paraStyles(t)
	i0, i1 := paraIndex(t, s), paraIndex(t, e)
	j := len(styles) // the paragraph it goes before
	if to <= t.Len() {
		j = paraIndex(t, to)
	}
	moved := slices.Clone(styles[i0 : i1+1])
	rest := slices.Delete(slices.Clone(styles), i0, i1+1)
	k := j
	if j > i1 {
		k -= i1 + 1 - i0
	}
	order := slices.Insert(rest, k, moved...)
	lo := min(i0, k)
	doc.Edit(func(ed *richtext.Edit) {
		defer func() { restyle(ed, lo, order[lo:]) }()
		if to > e { // down: insert first, then take it out from above
			if to > t.Len() { // at the end
				end := t.Len()
				ed.Replace(end, end, blk.Insert(0, richtext.NewText("\n")))
				st := t.ParagraphStyleAt(s)
				ed.FormatParagraphs(end+1, end+1, func(p *richtext.ParagraphStyle) { *p = st })
			} else {
				ed.Replace(to, to, blk.Append("\n", richtext.Attributes{}))
				st := t.ParagraphStyleAt(s)
				ed.FormatParagraphs(to, to, func(p *richtext.ParagraphStyle) { *p = st })
			}
			ed.Replace(s, e+1, richtext.Text{}) // with its newline
			at = to - n - 1
			if to > t.Len() {
				at = t.Len() - n
			}
		} else { // up: take it out, then insert
			end := min(e+1, t.Len())
			if e >= t.Len() { // the last block: the newline before it goes
				ed.Replace(s-1, e, richtext.Text{})
			} else {
				ed.Replace(s, end, richtext.Text{})
			}
			ed.Replace(to, to, blk.Append("\n", richtext.Attributes{}))
			st := t.ParagraphStyleAt(s)
			ed.FormatParagraphs(to, to, func(p *richtext.ParagraphStyle) { *p = st })
			at = to
		}
		ed.Select(at, at)
	})
	return at
}

// duplicateBlock puts a copy of the block holding rune at right after it, its chips made their own by own (copyChipsIn:
// a task duplicated has a box of its own, #539); nil copies the text as it is.
func duplicateBlock(doc *richtext.Document, at int, own func(richtext.Text) richtext.Text) {
	t := doc.Text()
	s, e := blockRange(t, at)
	blk := t.Slice(s, e)
	if own != nil {
		blk = own(blk)
	}
	st := t.ParagraphStyleAt(s)
	styles := paraStyles(t)
	i0, i1 := paraIndex(t, s), paraIndex(t, e)
	order := slices.Concat(styles[i0:i1+1], styles[i0:])
	doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(e, e, blk.Insert(0, richtext.NewText("\n")))
		ed.FormatParagraphs(e+1, e+1, func(p *richtext.ParagraphStyle) { *p = st })
		restyle(ed, i0, order)
		ed.Select(e+1, e+1)
	})
}

// deleteBlock takes the block holding rune at away, with its newline: the blocks chosen are taken the same way
// (deleteRange).
func deleteBlock(doc *richtext.Document, at int) {
	s, e := blockRange(doc.Text(), at)
	deleteRange(doc, s, e)
}

// turnInto gives the paragraph holding rune at the kind of block named.
func turnInto(doc *richtext.Document, at int, kind string) {
	t := doc.Text()
	s, _ := t.ParagraphRange(at)
	doc.Edit(func(ed *richtext.Edit) {
		ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) {
			indent := p.Indent
			*p = richtext.ParagraphStyle{}
			switch kind {
			case "h1":
				p.Heading = 1
			case "h2":
				p.Heading = 2
			case "h3":
				p.Heading = 3
			case "bullets":
				p.List, p.Indent = richtext.Bullets, indent
			case "numbers":
				p.List, p.Indent = richtext.Numbers, indent
			case "tasks":
				p.List, p.Indent = richtext.Tasks, indent
			default:
				if id, ok := strings.CutPrefix(kind, "task:"); ok { // a kind of task (taskkinds.go)
					p.List, p.Indent, p.ID = richtext.Tasks, indent, taskIDPrefix+id
				}
			case "quote":
				p.Quote = true
			case "code":
				p.CodeBlock = true
			}
		})
	})
}

// paraStyles is the style of each paragraph of t, in order.
func paraStyles(t richtext.Text) []richtext.ParagraphStyle {
	var out []richtext.ParagraphStyle
	for _, p := range t.Paragraphs() {
		out = append(out, p.Style)
	}
	return out
}

// paraIndex is the index of the paragraph holding rune at of t.
func paraIndex(t richtext.Text, at int) int {
	i := -1
	for s := range t.Paragraphs() {
		if s > at {
			break
		}
		i++
	}
	return max(i, 0)
}

// restyle gives the paragraphs of the edit's text from index from the styles, in order. A block moved or copied
// with Replace keeps its text but not what its paragraphs are, which the app keeps in their IDs (a divider, a task's
// kind, a code block's language), and the paragraph beside it may take them instead (#127): the styles are put back
// as they were, in the new order.
func restyle(ed *richtext.Edit, from int, styles []richtext.ParagraphStyle) {
	i := 0
	for s := range ed.Text().Paragraphs() {
		if k := i - from; k >= 0 && k < len(styles) {
			st := styles[k]
			ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = st })
		}
		i++
	}
}
