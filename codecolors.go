package main

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

// How the editor's paragraphs are drawn beyond their attributes (richtext.EditorOptions.Spans): a block of code
// colored by its language, as MyGo's code editor colors it, and the letters spaced as the settings say. Colors leave
// the layout as it is; the spacing does not, so the app's own layout of a paragraph (layoutParagraph, for the caret)
// spaces its letters the same way.

// textSpacing is how far apart the letters of the text are, in DIPs, from the settings (LetterSpacing): the editor's
// spans and the caret's layout both take it.
var textSpacing float32

// letterSpacings are the settings' steps of LetterSpacing, as parts of the text's size.
var letterSpacings = []struct {
	name string
	em   float32
}{{"Off", 0}, {"Slight", 0.02}, {"Wide", 0.05}, {"Wider", 0.08}}

// spacingOf is the spacing of the letters of text of size, for setting step.
func spacingOf(step int, size float32) float32 {
	if step <= 0 || step >= len(letterSpacings) {
		return 0
	}
	return letterSpacings[step].em * size
}

// spaced gives the text's spans the spacing of the letters; an object (a chip, a picture) is left as it is.
func spaced(spans []ui.Span) []ui.Span {
	if textSpacing == 0 {
		return spans
	}
	for i := range spans {
		if spans[i].ObjectWidth == 0 {
			spans[i].LetterSpacing = textSpacing
		}
	}
	return spans
}

// linkRoom leaves the room of its icon before each web link of paragraph p (linkcards.go), in the spans the editor
// draws it with.
func (a *app) linkRoom(pn *paneState, p int, spans []ui.Span) []ui.Span {
	if !linkIcons || pn.doc == nil {
		return spans
	}
	t := pn.doc.Text()
	pt := a.paraTableOf(pn, t) // p is the paragraph's number, not a rune of it (#509)
	if p < 0 || p >= len(pt.start) {
		return spans
	}
	start := pt.start[p]
	return splitLinkRoom(spans, a.font.Size, func(at int) string {
		attrs := t.AttributesAt(start + at)
		if attrs.Attachment != nil {
			return ""
		}
		return attrs.Link
	})
}

// splitLinkRoom gives the room of its icon to the first word of each web link of the spans of a paragraph, its
// own span: a span's PaddingStart is taken at the start of each line's piece of it, and a link that wraps would have
// the room, and its icon, on each of its lines (#509). linkAt is the link of the paragraph's rune at, "" for none;
// size is the text's when a span has none of its own.
func splitLinkRoom(spans []ui.Span, size float32, linkAt func(at int) string) []ui.Span {
	var out []ui.Span // made when a span is split
	at := 0
	for i, s := range spans {
		n := utf8.RuneCountInString(s.Text)
		link := ""
		if n > 0 {
			link = linkAt(at)
		}
		ls := s.Size
		if ls == 0 {
			ls = size
		}
		pad := float32(0)
		if link != "" && (at == 0 || linkAt(at-1) != link) {
			pad = linkIconPad(link, ls)
		}
		at += n
		if pad == 0 {
			if out != nil {
				out = append(out, s)
			}
			continue
		}
		if out == nil {
			out = append(make([]ui.Span, 0, len(spans)+4), spans[:i]...)
		}
		head, tail := s, s
		head.PaddingStart = pad
		k := strings.IndexAny(s.Text, " \t") + 1 // the first word, with its space: the letters of a word are shaped together
		if s.ObjectWidth != 0 || k <= 0 || k >= len(s.Text) {
			out = append(out, head)
			continue
		}
		head.Text, tail.Text = s.Text[:k], s.Text[k:]
		out = append(out, head, tail)
	}
	if out == nil {
		return spans
	}
	return out
}

// editorSpans are paragraph p's spans as the page editor draws them: spaced, and a block of code's colored.
func (a *app) editorSpans(pn *paneState, p int, st richtext.ParagraphStyle, spans []ui.Span) []ui.Span {
	// the room of a link's icon, the last: what splits or colors the spans before would take it with them
	return a.linkRoom(pn, p, a.paintSpans(pn, p, st, spaced(spans)))
}

// paintSpans are paragraph p's spans colored and struck as the page editor draws them.
func (a *app) paintSpans(pn *paneState, p int, st richtext.ParagraphStyle, spans []ui.Span) []ui.Span {
	if st.List == richtext.Tasks && st.Checked {
		for i := range spans { // a fold's chip on a task done is not struck through (#494)
			if spans[i].Text == string(richtext.ObjectReplacement) {
				spans[i].Strikethrough = false
			}
		}
		if out, ok := a.strikeSpans(p, st, spans); ok { // its line still being drawn
			return out
		}
	}
	if pn.grip.dragging && p >= pn.grip.paras[0] && p <= pn.grip.paras[1] { // the block dragged (blockui.go)
		return a.blockSpans(pn, p, spans)
	}
	if !st.CodeBlock {
		spans = a.inlineDoneSpans(pn, p, spans)                                                 // the words of a task in the running text, done (#599)
		spans = a.inlineActiveSpans(pn, p, spans)                                               // and the one the caret is in, shown as one (#610)
		spans = a.tagStatusSpans(st, spans)                                                     // a task's status tag as a pill of its color (#597)
		if k, ok := taskKindOf(st); ok && k.id == "canceled" && !st.Checked && a.theme != nil { // a task canceled: faint
			for i := range spans {
				if spans[i].ObjectWidth == 0 {
					spans[i].Color = a.theme.Muted
				}
			}
		}
		if out, ok := a.lapseSpans(pn, p, spans); ok { // the words in the colors of their days
			return out
		}
	}
	if !st.CodeBlock || a.codeTheme == nil {
		return spans
	}
	lang, ok := strings.CutPrefix(st.ID, codeIDPrefix)
	if !ok { // a line after the block's first, which holds the language
		lang = a.codeLangOf(pn, p)
	}
	l := editor.LanguageNamed(lang)
	if l == nil || l.Tokenizer == nil {
		return spans
	}
	var line strings.Builder
	for _, s := range spans {
		line.WriteString(s.Text)
	}
	toks, _ := l.Tokenizer.Tokenize(line.String(), 0, nil) // a line on its own: a string or comment of many lines is colored by its lines
	return colorSpans(spans, toks, a.codeTheme)
}

// inlineDoneSpans strikes and mutes the words of each task done in paragraph p's running text: those its box owns
// as the task view reads them (inlineTasksOf), the words after it up to the next box, or before it when none come
// after. Other chips in them, and the words of a task not done, stay as they are (#599).
func (a *app) inlineDoneSpans(pn *paneState, p int, spans []ui.Span) []ui.Span {
	if pn.doc == nil || a.theme == nil {
		return spans
	}
	objects := false
	for _, s := range spans {
		if s.ObjectWidth != 0 {
			objects = true
			break
		}
	}
	if !objects {
		return spans
	}
	t := pn.doc.Text()
	pt := a.paraTableOf(pn, t)
	if p < 0 || p >= len(pt.start) {
		return spans
	}
	var line []rune
	for _, s := range spans {
		line = append(line, []rune(s.Text)...)
	}
	start := pt.start[p]
	var boxes []inlineBox
	for i, r := range line {
		if r != richtext.ObjectReplacement {
			continue
		}
		if att := t.AttributesAt(start + i).Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && (ch.kind == "check" || ch.kind == "checkend") {
				boxes = append(boxes, inlineBox{at: i, done: ch.target == "x", end: ch.kind == "checkend"})
			}
		}
	}
	segs := inlineDoneSegments(line, boxes)
	if len(segs) == 0 {
		return spans
	}
	ink := a.theme.Muted
	return restyleRunes(spans, segs, func(s *ui.Span) {
		if s.ObjectWidth == 0 && strings.TrimSpace(s.Text) != "" {
			s.Strikethrough, s.Color = true, ink
		}
	})
}

// inlineBox is a task's box in a paragraph's running text: the rune it is, and whether it is done.
type inlineBox struct {
	at   int
	done bool
	end  bool // not a box: where the task before it ends (#610)
}

// inlineDoneSegments are the runes of line, [from, to), that the boxes done own, as inlineTasksOf reads a task's
// words: from after the box to the next box, or from the box before (or the start) to it when no word comes after.
// The spaces and stops at their ends are left out.
func inlineDoneSegments(line []rune, boxes []inlineBox) [][2]int {
	var out [][2]int
	for k, b := range boxes {
		if !b.done || b.end {
			continue
		}
		from, to, _ := inlineSegment(line, boxes, k)
		for from < to && (notWord(line[from]) || line[from] == richtext.ObjectReplacement) {
			from++
		}
		for to > from && (notWord(line[to-1]) || line[to-1] == richtext.ObjectReplacement) {
			to--
		}
		if from < to {
			out = append(out, [2]int{from, to})
		}
	}
	return out
}

// restyleRunes splits spans where the ranges of runes segs start and end, and gives each piece inside one to f.
func restyleRunes(spans []ui.Span, segs [][2]int, f func(*ui.Span)) []ui.Span {
	out := make([]ui.Span, 0, len(spans)+2*len(segs))
	at := 0
	for _, s := range spans {
		rs := []rune(s.Text)
		if len(rs) == 0 {
			out = append(out, s)
			continue
		}
		end := at + len(rs)
		cuts := []int{at, end}
		for _, g := range segs {
			for _, c := range g {
				if c > at && c < end {
					cuts = append(cuts, c)
				}
			}
		}
		slices.Sort(cuts)
		cuts = slices.Compact(cuts)
		for i := 0; i+1 < len(cuts); i++ {
			piece := s
			piece.Text = string(rs[cuts[i]-at : cuts[i+1]-at])
			for _, g := range segs {
				if cuts[i] >= g[0] && cuts[i+1] <= g[1] {
					f(&piece)
					break
				}
			}
			out = append(out, piece)
		}
		at = end
	}
	return out
}

// colorSpans splits spans where the tokens of their text start and end, and colors each piece as its token is.
func colorSpans(spans []ui.Span, toks []editor.Token, th *editor.Theme) []ui.Span {
	out := make([]ui.Span, 0, len(spans)+len(toks))
	pos, t := 0, 0 // where in the line the span being cut starts, and the token it is in
	for _, s := range spans {
		text := s.Text
		for len(text) > 0 {
			for t < len(toks) && toks[t].End <= pos {
				t++
			}
			piece := s
			n := len(text)
			if t < len(toks) && toks[t].End-pos < n {
				n = toks[t].End - pos
			}
			piece.Text = text[:n]
			if s.ObjectWidth == 0 {
				if t < len(toks) {
					piece.Color = th.TokenColor(toks[t].Kind)
				} else {
					piece.Color = th.Text
				}
			}
			out = append(out, piece)
			text, pos = text[n:], pos+n
		}
	}
	return out
}

// codeLangs are the languages of the page's paragraphs, by index: a block of code's for each of its lines ("" for a
// paragraph of no block of code), read once for a text.
type codeLangs struct {
	text  richtext.Text
	langs []string
}

// codeLangOf is the language of the block of code paragraph p is in: its first line's.
func (a *app) codeLangOf(pn *paneState, p int) string {
	t := pn.doc.Text()
	cl := &a.codeLangs
	if cl.langs == nil || cl.text != t {
		cl.text, cl.langs = t, cl.langs[:0]
		lang := ""
		for _, para := range t.Paragraphs() {
			st := para.Style
			switch l, ok := strings.CutPrefix(st.ID, codeIDPrefix); {
			case st.CodeBlock && ok:
				lang = l
			case !st.CodeBlock:
				lang = ""
			}
			cl.langs = append(cl.langs, lang)
		}
	}
	if p < 0 || p >= len(cl.langs) {
		return ""
	}
	return cl.langs[p]
}
