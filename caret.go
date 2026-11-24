package main

import (
	"math"
	"strings"
	"unicode"
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
// This mirrors MyGo's richtext paraFont, look.span and insets (ui/richtext/view.go), the
// version go.mod requires, which caret_test.go holds it to; the editor is an EditorBase, which
// has no padding.

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
	_, caret := doc.Selection()
	return locateAt(doc, caret, boxes, w, base, th)
}

// locateAt is locate for the rune caret, wherever the selection is.
func locateAt(doc *richtext.Document, caret int, boxes []richtext.ParagraphBox, w float32, base ui.Font, th *richtext.Theme) (caretBox, bool) {
	if len(boxes) == 0 {
		return caretBox{}, false
	}
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
	var para richtext.Paragraph
	if end-start <= maxCaretParagraph {
		for _, p := range text.Slice(start, end).Paragraphs() {
			para = p
			break
		}
		para.Style = text.ParagraphStyleAt(start)
	}
	return locateIn(doc, caret, box, start, end, para, w, base, th)
}

// locateIn is locateAt in the paragraph of box, from start to end, para, as the caller has it already: a box's
// from the text's table (paraOfBox) is not read again from the start of the text, which for a mark drawn over
// each paragraph in view of a long page was most of a frame (#314).
func locateIn(doc *richtext.Document, caret int, box richtext.ParagraphBox, start, end int, para richtext.Paragraph, w float32, base ui.Font, th *richtext.Theme) (caretBox, bool) {
	if caret > end { // past the last paragraph in view
		return caretBox{}, false
	}
	if end-start > maxCaretParagraph { // one line of a megabyte: laying it out again, for a caret, takes minutes
		return caretBox{}, false
	}
	l, left := longMemo.layout(doc, start, end-start, para, w, base, th)
	var line string
	for _, r := range para.Runs {
		line += r.Text
	}
	u := units(line, caret-start) // in UTF-16 units of the paragraph's text, as the layout has them
	c := l.CaretAt(ui.TextCaretPosition{Index: u, Affinity: ui.TextDownstream})
	full := c
	mid := c.Y + c.H/2
	// a line holding a block (a table, a card, a picture) is as tall as it: its caret is
	// as tall as a letter, on the baseline, not as the block
	tall := false
	for _, r := range para.Runs {
		if at := r.Attributes.Attachment; at != nil && at.Height > base.Size*2 {
			tall = true
		}
	}
	for _, ln := range l.Lines() {
		if mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H && ln.Ascent > 0 {
			c.Y, c.H = ln.Baseline-ln.Ascent, ln.Ascent+ln.Descent
			if tall {
				c.Y, c.H = ln.Baseline-base.Size*0.95, base.Size*1.25
			}
			break
		}
	}
	cw := c.H * 0.5
	// the caret is as wide as the letter after it - but not over a chip,
	// which is one object, wider than a word: it would swallow it
	onChip := runeIn(line, caret-start) == richtext.ObjectReplacement // read from the paragraph, not from the start of the text (#314)
	if next := l.NextBoundary(u); next > u && !onChip {
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
	paraLayouts++
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
	var links []string // the link of each run, none for an attachment
	for _, r := range p.Runs {
		spans = append(spans, spanOf(r.Text, r.Attributes, st, size, width, th))
		link := r.Attributes.Link
		if r.Attributes.Attachment != nil {
			link = ""
		}
		links = append(links, link)
	}
	if linkIcons { // the room of a link's icon, as the editor's spans leave it (linkRoom)
		spans = splitLinkRoom(spans, size, func(at int) string {
			for i, r := range p.Runs {
				n := utf8.RuneCountInString(r.Text)
				if at < n {
					return links[i]
				}
				at -= n
			}
			return ""
		})
	}
	f := base
	switch {
	case st.Heading > 0:
		h := th.Headings[min(max(st.Heading, 1), 6)-1]
		f.Size *= h.Size
		f.Weight = h.Weight
		if h.LineHeight > 0 {
			f.LineHeight = h.LineHeight
		}
	case st.CodeBlock:
		f.Family = th.CodeFont
		f.Size *= th.CodeSize
	}
	if len(spans) == 0 {
		spans = []ui.Span{{Text: ""}}
	}
	spans = spaced(spans) // as the editor's (codecolors.go)
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

// paraLayouts counts the paragraphs laid out by layoutParagraph, for the tests of what is kept.
var paraLayouts int

// longMemo keeps the layout of the last long paragraph laid out for a caret.
// The caret is found every frame, and a paragraph is laid out again for it each
// time: for a paragraph of a few hundred runes that is nothing, for one of
// half a megabyte (an article pasted as a single line) seconds a frame. It is
// kept for as long as the text, its place, the width and the look stay as
// they were: the caret moving changes none of them.
var longMemo paraMemo

// longParagraphRunes is the length from which a paragraph's layout is kept.
const longParagraphRunes = 2000

// maxCaretParagraph is the length over which a paragraph gets no caret of ours.
const maxCaretParagraph = 30000

type paraMemo struct {
	doc   *richtext.Document
	ver   uint64
	start int
	w     float32
	base  ui.Font
	th    layLook
	style richtext.ParagraphStyle
	l     *ui.TextLayout
	left  float32
}

func (m *paraMemo) layout(doc *richtext.Document, start, runes int, p richtext.Paragraph, w float32, base ui.Font, th *richtext.Theme) (*ui.TextLayout, float32) {
	if runes < longParagraphRunes {
		return shortMemo.layout(doc, start, p, w, base, th)
	}
	ver, look := doc.Version(), lookOf(th)
	if m.l != nil && m.doc == doc && m.ver == ver && m.start == start && m.w == w && m.base == base && m.th == look && m.style == p.Style {
		return m.l, m.left
	}
	l, left := layoutParagraph(p, w, base, th)
	*m = paraMemo{doc, ver, start, w, base, look, p.Style, l, left}
	return l, left
}

// shortMemo keeps the layouts of the short paragraphs laid out for a caret or a mark over them (a task's box, a
// tag's pill, a link's place, the whitespace marks, the find's hits), for one version of the text: on a long page the
// boxes of the tasks in view were each laid out twice a frame, most of a frame, in every frame between two keys as in
// the one after a key (#314), and a page of tags some 60 µs a paragraph in view, each frame (#204).
var shortMemo shortLayouts

type shortLayouts struct {
	doc  *richtext.Document
	ver  uint64
	w    float32
	base ui.Font
	th   layLook
	m    map[int]shortLaid // by the rune the paragraph starts at
}

// layLook is what lays a paragraph out besides its text, width and font (layoutParagraph): the editor's theme, made
// anew each frame, so that a layout is kept by these rather than by the theme, whether links have the room of their
// icons, the spacing of the letters, and the sizes of the objects that change in place (sizedGen).
type layLook struct {
	indent, codeSize float32
	codeFont         string
	codePad          [2]float32
	headings         [6]richtext.HeadingStyle
	linkIcons        bool
	spacing          float32
	sized            uint64
}

// sizedGen counts the objects of a text whose size changed in place, not by an edit (a query block's rows): a layout
// kept from before is not that of the text any longer.
var sizedGen uint64

func lookOf(th *richtext.Theme) layLook {
	return layLook{th.Indent, th.CodeSize, th.CodeFont, th.CodeBlockPadding, th.Headings, linkIcons, textSpacing, sizedGen}
}

// paraLayout is the paragraph para of the open page, starting at rune s, laid out at width w as the editor lays it
// out, kept for this version of the text.
func (a *app) paraLayout(s int, para richtext.Paragraph, w float32) (*ui.TextLayout, float32) {
	n := 0
	for _, r := range para.Runs {
		n += utf8.RuneCountInString(r.Text)
	}
	return longMemo.layout(a.doc, s, n, para, w, a.font, a.theme)
}

type shortLaid struct {
	style richtext.ParagraphStyle
	l     *ui.TextLayout
	left  float32
}

func (s *shortLayouts) layout(doc *richtext.Document, start int, p richtext.Paragraph, w float32, base ui.Font, th *richtext.Theme) (*ui.TextLayout, float32) {
	ver, look := doc.Version(), lookOf(th)
	if s.m == nil || s.doc != doc || s.ver != ver || s.w != w || s.base != base || s.th != look || len(s.m) > 1024 {
		*s = shortLayouts{doc, ver, w, base, look, map[int]shortLaid{}}
	}
	if e, ok := s.m[start]; ok && e.style == p.Style {
		return e.l, e.left
	}
	l, left := layoutParagraph(p, w, base, th)
	s.m[start] = shortLaid{p.Style, l, left}
	return l, left
}

// inputCaret is where the caret is in a text input that shows text of one
// size on one line, at (left, top) of a box height high, the text in the
// middle of it: its place and the width of the letter after it. The input's
// own caret is hidden (CaretWidth(0)); ours is drawn as the page's.
func (a *app) inputCaret(text string, caret int, size float32, weight int, left, top, height float32) (ui.Rect, float32) {
	return a.inputCaretIn(text, caret, size, weight, 0, left, top, height)
}

// inputCaretIn is inputCaret in a text wrapped at width, as the page's title is (0: one line).
func (a *app) inputCaretIn(text string, caret int, size float32, weight int, width, left, top, height float32) (ui.Rect, float32) {
	l := ui.ShapeRichTextWith([]ui.Span{{Text: text, Size: size, Weight: weight}}, a.font, ui.TextShape{Width: width})
	u := units(text, caret)
	c := l.CaretAt(ui.TextCaretPosition{Index: u, Affinity: ui.TextDownstream})
	full := c
	mid := c.Y + c.H/2
	for _, ln := range l.Lines() {
		if mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H && ln.Ascent > 0 {
			c.Y, c.H = ln.Baseline-ln.Ascent, ln.Ascent+ln.Descent
			break
		}
	}
	cw := c.H * 0.5
	if next := l.NextBoundary(u); next > u {
		if nc := l.CaretAt(ui.TextCaretPosition{Index: next}); nc.X > full.X && math.Abs(float64(nc.Y-full.Y)) < 0.5 {
			cw = nc.X - full.X
		}
	}
	_, lh := l.Size()
	c.X += left
	c.Y += top + max(0, (height-lh)/2)
	return c, cw
}

// nearGlyph is a letter on the caret's line, where it is in the window, for a caret that moves letters (the
// magnet): the letter and its box, and how far it is from the caret, in letters (- before it, + after).
type nearGlyph struct {
	ch   string
	r    ui.Rect
	off  int
	size float32
}

// nearLetters are the letters of the caret's line within n of it, either side, as laid out in an editor whose
// box is at (x, y) and w wide. Spaces and objects (a chip, a table) are not letters.
func nearLetters(doc *richtext.Document, boxes []richtext.ParagraphBox, x, y, w float32, base ui.Font, th *richtext.Theme, n int) []nearGlyph {
	_, caret := doc.Selection()
	text := doc.Text()
	var box richtext.ParagraphBox
	found := false
	for i, b := range boxes {
		if b.Start <= caret && (i+1 == len(boxes) || caret < boxes[i+1].Start) {
			box, found = b, true
			break
		}
	}
	if !found {
		return nil
	}
	start, end := text.ParagraphRange(box.Start)
	if caret > end || end-start > maxCaretParagraph {
		return nil
	}
	var para richtext.Paragraph
	for _, p := range text.Slice(start, end).Paragraphs() {
		para = p
		break
	}
	para.Style = text.ParagraphStyleAt(start)
	l, left := longMemo.layout(doc, start, end-start, para, w, base, th)
	var line string
	for _, r := range para.Runs {
		line += r.Text
	}
	runes := []rune(line)
	at := func(i int) ui.Rect {
		return l.CaretAt(ui.TextCaretPosition{Index: units(line, i), Affinity: ui.TextDownstream})
	}
	home := at(caret - start)
	var out []nearGlyph
	for i := max(0, caret-start-n); i < min(len(runes), caret-start+n); i++ {
		r := runes[i]
		if r == ' ' || r == '\t' || r == richtext.ObjectReplacement || r == '\n' {
			continue
		}
		c0, c1 := at(i), at(i+1)
		if math.Abs(float64(c0.Y-home.Y)) > 0.5 || c1.X <= c0.X { // on another line, or not laid out in a row
			continue
		}
		gy, gh := c0.Y, c0.H
		for _, ln := range l.Lines() {
			if mid := c0.Y + c0.H/2; mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H && ln.Ascent > 0 {
				gy, gh = ln.Baseline-ln.Ascent, ln.Ascent+ln.Descent
				break
			}
		}
		out = append(out, nearGlyph{ch: string(r), r: ui.Rect{X: x + left + c0.X, Y: y + box.Top + gy, W: c1.X - c0.X, H: gh}, off: i - (caret - start), size: gh / 1.25})
	}
	return out
}

// caretText is what a caret may ask of the text round it, while it is painted: the word just written, how
// many words its paragraph has, and where a word is in the text and how many times. The page's text has it
// (caretWords), and the field the settings try a caret in (caretplay.go).
type caretText interface {
	last() string
	paraWords() int
	find(word string) (rects []ui.Rect, total int)
}

// caretWords is what a caret may ask of the page's text round it, while it is painted: the word just written, how
// many words its paragraph has, and where a word is in the text (the repeats caret, the word count caret).
type caretWords struct {
	doc     *richtext.Document
	boxes   []richtext.ParagraphBox
	x, y, w float32
	font    ui.Font
	th      *richtext.Theme
}

// isWordRune reports whether r is part of a word: a letter or a digit.
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// last is the word before the caret: the one just written, past the space or mark after it.
func (c *caretWords) last() string {
	_, caret := c.doc.Selection()
	text := c.doc.Text()
	from := max(0, caret-64)
	runes := []rune(text.Slice(from, caret).String())
	i := len(runes)
	for i > 0 && !isWordRune(runes[i-1]) {
		i--
	}
	j := i
	for j > 0 && isWordRune(runes[j-1]) {
		j--
	}
	return string(runes[j:i])
}

// paraWords is how many words the caret's paragraph has.
func (c *caretWords) paraWords() int {
	_, caret := c.doc.Selection()
	text := c.doc.Text()
	s, e := text.ParagraphRange(caret)
	if e-s > maxCaretParagraph {
		return 0
	}
	return len(strings.Fields(text.Slice(s, e).PlainText()))
}

// wordAt reports the starts of word, a whole word, folded, in runes.
func wordAt(runes, word []rune) []int {
	var out []int
	for i := 0; i+len(word) <= len(runes); i++ {
		if (i > 0 && isWordRune(runes[i-1])) || (i+len(word) < len(runes) && isWordRune(runes[i+len(word)])) {
			continue
		}
		match := true
		for k, r := range word {
			if runes[i+k] != r {
				match = false
				break
			}
		}
		if match {
			out = append(out, i)
		}
	}
	return out
}

// find is where word is, as a whole word in any case: its boxes in the paragraphs in view, in the window, and how
// many times it is in the whole text.
func (c *caretWords) find(word string) (rects []ui.Rect, total int) {
	w := foldRunes(word)
	if len(w) == 0 {
		return nil, 0
	}
	text := c.doc.Text()
	total = len(wordAt(foldRunes(text.PlainText()), w))
	for _, b := range c.boxes {
		s, e := text.ParagraphRange(b.Start)
		if e-s > maxCaretParagraph {
			continue
		}
		var para richtext.Paragraph
		for _, p := range text.Slice(s, e).Paragraphs() {
			para = p
			break
		}
		var line string
		for _, r := range para.Runs {
			line += r.Text
		}
		hits := wordAt(foldRunes(line), w)
		if len(hits) == 0 {
			continue
		}
		para.Style = text.ParagraphStyleAt(s)
		l, left := longMemo.layout(c.doc, s, e-s, para, c.w, c.font, c.th)
		for _, at := range hits {
			for _, rc := range l.SelectionRects(ui.TextInputRange{Start: units(line, at), End: units(line, at+len(w))}) {
				rects = append(rects, ui.Rect{X: c.x + left + rc.X, Y: c.y + b.Top + rc.Y, W: rc.W, H: rc.H})
			}
		}
	}
	return rects, total
}

// runeIn is the rune at i of s, counted in runes, or 0 past its end.
func runeIn(s string, i int) rune {
	for _, r := range s {
		if i == 0 {
			return r
		}
		i--
	}
	return 0
}
