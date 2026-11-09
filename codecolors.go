package main

import (
	"strings"

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

// editorSpans are paragraph p's spans as the page editor draws them: spaced, and a block of code's colored.
func (a *app) editorSpans(p int, st richtext.ParagraphStyle, spans []ui.Span) []ui.Span {
	spans = spaced(spans)
	if !st.CodeBlock || a.codeTheme == nil {
		return spans
	}
	lang, ok := strings.CutPrefix(st.ID, codeIDPrefix)
	if !ok { // a line after the block's first, which holds the language
		lang = a.codeLangOf(p)
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
func (a *app) codeLangOf(p int) string {
	t := a.doc.Text()
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
