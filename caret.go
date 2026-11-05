package main

import (
	"math"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// MyGo's rich text editor draws its own caret, a line one pixel wide, and
// does not tell where it is. Ours are drawn over it - its own is made
// invisible (theme.Caret) - so the place is found here: the editor says which
// paragraphs are in view and where (Document.VisibleParagraphs), and the
// paragraph holding the caret is laid out again the way the editor lays it
// out, from the same theme, font and width. caret_test.go checks the result
// against the caret the editor reports to input methods, for every kind of
// paragraph, so a change in MyGo that moves it fails a test.
//
// This mirrors richtext v0.7.4's paraFont, look.span and insets (ui/richtext/view.go);
// the editor is an EditorBase, which has no padding.

// invisible is a caret color the editor takes as set (it is not the zero
// Color, which would take the default) and draws as nothing.
var invisible = ui.RGBA(255, 255, 255, 0)

// caretBox is where the caret of doc is, in the coordinates of the editor's
// element (as r of its Draw): its box, the line's letters' height rather
// than its leading, and the width of the letter after it. ok is false when
// the caret is out of view.
type caretBox struct {
	r    ui.Rect
	full ui.Rect // the caret as the editor has it, the line's full height (tests compare it)
	cw   float32
	top  float32 // the top of its paragraph's box
	para int     // the start of the paragraph
}

// locate finds the caret of doc in an editor of width w, from the boxes of
// the paragraphs in view (doc.VisibleParagraphs), with the base font and theme
// the editor was given.
func locate(doc *richtext.Document, boxes []richtext.ParagraphBox, w float32, base ui.Font, th *richtext.Theme) (caretBox, bool) {
	if len(boxes) == 0 {
		return caretBox{}, false
	}
	_, caret := doc.Selection()
	text := doc.Text()
	bi := -1
	for i, b := range boxes {
		if b.Start <= caret && (i+1 == len(boxes) || caret < boxes[i+1].Start) {
			bi = i
			break
		}
	}
	if bi < 0 {
		return caretBox{}, false
	}
	box := boxes[bi]
	start, end := text.ParagraphRange(box.Start)
	if caret > end { // past the last paragraph in view
		return caretBox{}, false
	}
	var para richtext.Paragraph
	for _, p := range text.Slice(start, end).Paragraphs() {
		para = p
		break
	}
	para.Style = text.ParagraphStyleAt(start)
	l, left := layoutParagraph(para, w, base, th)
	// The caret's place in the layout, in UTF-16 units of the paragraph's text.
	var line string
	for _, r := range para.Runs {
		line += r.Text
	}
	at := caret - start
	units := 0
	for i, r := range line {
		if utf8.RuneCountInString(line[:i]) >= at {
			break
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	c := l.CaretAt(ui.TextCaretPosition{Index: units, Affinity: ui.TextDownstream})
	full := c
	mid := c.Y + c.H/2
	for _, ln := range l.Lines() {
		if mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H && ln.Ascent > 0 {
			c.Y, c.H = ln.Baseline-ln.Ascent, ln.Ascent+ln.Descent
			break
		}
	}
	cw := c.H * 0.5
	// the caret is as wide as the letter after it - but not over a chip,
	// which is one object, wider than a word: it would swallow it
	onChip := caret < text.Len() && text.Slice(caret, caret+1).String() == string(richtext.ObjectReplacement)
	if next := l.NextBoundary(units); next > units && !onChip {
		if nc := l.CaretAt(ui.TextCaretPosition{Index: next}); nc.X > full.X && math.Abs(float64(nc.Y-full.Y)) < 0.5 {
			cw = nc.X - full.X
		}
	}
	c.X += left
	c.Y += box.Top
	full.X += left
	full.Y += box.Top
	return caretBox{r: c, full: full, cw: cw, top: box.Top, para: box.Start}, true
}

// layoutParagraph lays a paragraph out as the editor does, and returns how
// far in from the left it starts.
func layoutParagraph(p richtext.Paragraph, w float32, base ui.Font, th *richtext.Theme) (*ui.TextLayout, float32) {
	st := p.Style
	size := base.Size
	indent := size * th.Indent
	left := float32(st.Indent) * indent
	right := float32(0)
	if st.List != richtext.NoList {
		left += indent
	}
	if st.Quote {
		left += indent * 0.75
	}
	if st.CodeBlock {
		left += size * th.CodeBlockPadding[0]
		right += size * th.CodeBlockPadding[0]
	}
	width := max(w-left-right, 1)
	var spans []ui.Span
	for _, r := range p.Runs {
		spans = append(spans, spanOf(r.Text, r.Attributes, st, size, width, th))
	}
	f := base
	switch {
	case st.Heading > 0:
		h := th.Headings[min(max(st.Heading, 1), 6)-1]
		f.Size *= h.Size
		f.Weight = h.Weight
	case st.CodeBlock:
		f.Family = th.CodeFont
		f.Size *= th.CodeSize
	}
	if len(spans) == 0 {
		spans = []ui.Span{{Text: ""}}
	}
	return ui.ShapeRichTextWith(spans, f, ui.TextShape{Width: width, Align: st.Align}), left
}

// spanOf is the editor's look.span: the geometry of a run (colors leave the
// layout as it is, so only what changes sizes and fonts matters here).
func spanOf(text string, a richtext.Attributes, st richtext.ParagraphStyle, size, width float32, th *richtext.Theme) ui.Span {
	s := ui.Span{Text: text, Font: a.Font, Size: a.Size, Weight: a.Weight, Italic: a.Italic,
		Underline: a.Underline, Strikethrough: a.Strikethrough}
	if a.Code {
		s.Font = th.CodeFont
		if s.Size == 0 {
			s.Size = size * th.CodeSize
		}
		s.Radius = max(3, s.Size/4)
	}
	if at := a.Attachment; at != nil {
		w, h := at.Width, at.Height
		if at.Image != nil && (w <= 0 || h <= 0) {
			iw, ih := ui.ImageSize(at.Image)
			switch {
			case w <= 0 && h <= 0:
				w, h = iw, ih
			case w <= 0 && ih > 0:
				w = h * iw / ih
			case h <= 0 && iw > 0:
				h = w * ih / iw
			}
		}
		w, h = max(w, 1), max(h, 1)
		if w > width {
			w, h = width, h*width/w
		}
		s.ObjectWidth, s.ObjectHeight = w, h
	}
	return s
}
