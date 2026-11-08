package main

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The caret's playground: a field under the caret tiles of the settings, to write in and see the chosen caret
// as it is in the text - what is written, erased, a space, the word just written (the field's own letters and
// words, for the carets that move letters or count words). Enter is a new line there, as in the text: the
// field holds one line, and Enter clears it; past its width the first words go, so the line never scrolls.

const tryText = "Buraya yaz, careti dene: kelime kelime "

// caretTry is the playground's state.
type caretTry struct {
	text    string
	was     string // the text as the caret last saw it
	seen    bool
	pending kick    // what happened since, for the caret to take
	fieldW  float32 // the field's width, as last drawn
}

// tryKick is what happened to the field's text since the caret last saw it: what was written (a space, an
// Enter) or erased, found from the two texts by what they share at their start and their end.
func tryKick(was, now string) kick {
	a, b := []rune(was), []rune(now)
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	gone, put := a[pre:len(a)-suf], b[pre:len(b)-suf]
	var k kick
	if len(put) > 0 {
		k.wrote, k.typed = len(put), string(put)
	}
	if len(gone) > 0 {
		k.erased, k.gone, k.dir = len(gone), string(gone), -1
	}
	k.edited = len(put) > 0 || len(gone) > 0
	return k
}

// caretPlayground draws the field, and the caret in it.
func (a *app) caretPlayground(c *ui.Context) {
	cl := ryugo.From(c).Colors
	ct := &a.caretTry
	if !ct.seen {
		ct.text, ct.was, ct.seen = tryText, tryText, true
	}
	size := a.fontSize()
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, T("Burada dene")).FontSize(13).FontWeight(620).TextColor(cl.Text2)
		in := ui.TextInputBase(c.Key("caret-try"), &ct.text).Label(T("Careti dene")).FontSize(size).Padding(0, 14).
			FillWidth().Height(size*2.6).Background(cl.Card).Radius(rad(12)).Border(1, cl.Hairline).FocusRing(false).CaretWidth(0)
		a.fieldMenu(in, &ct.text, fieldOpts{oneLine: true})
		if in.Submitted() { // Enter: a new line in the text, here a clean field, with the caret's Enter
			ct.pending.typed += "\n"
			ct.text, ct.was = "", ""
		}
		if ct.text != ct.was {
			// the field holds what fits: past its width, the first words go, so it never scrolls under the caret
			for ct.fieldW > 0 {
				w, _ := ui.ShapeRichTextWith([]ui.Span{{Text: ct.text, Size: size}}, a.font, ui.TextShape{}).Size()
				sp := strings.IndexByte(ct.text, ' ')
				if w < ct.fieldW-28-size*4 || sp < 0 { // room at the end for what the caret does there
					break
				}
				ct.was = strings.TrimPrefix(ct.was, ct.text[:sp+1]) // not an erase: the caret is not told of it
				ct.text = ct.text[sp+1:]
			}
			k := tryKick(ct.was, ct.text)
			ct.pending.wrote += k.wrote
			ct.pending.typed += k.typed
			ct.pending.erased += k.erased
			ct.pending.gone = k.gone + ct.pending.gone
			ct.pending.dir = k.dir
			ct.pending.edited = ct.pending.edited || k.edited
			ct.was = ct.text
		}
		focused := in.Focused()
		_, caret := in.TextAnchorCaret()
		anchor, _ := in.TextAnchorCaret()
		text := ct.text
		in.DrawOver(func(p *ui.Painter, r ui.Rect) {
			ct.fieldW = r.W
			left, top := r.X+14, r.Y
			cr, cw := a.inputCaret(text, caret, size, 0, left, top, r.H)
			if !focused { // not being written in: a still caret, at the end, so it can be seen
				cr, cw = a.inputCaret(text, utf8.RuneCountInString(text), size, 0, left, top, r.H)
			}
			k := ct.pending
			ct.pending = kick{}
			a.nearFn = func(n int) []nearGlyph { return a.tryLetters(text, caret, size, left, top, r.H, n) }
			a.wordsFn = &tryWords{a: a, text: text, caret: caret, size: size, left: left, top: top, h: r.H}
			a.paintCaret(p, cr, cw, r, k, focused && anchor != caret)
			a.nearFn, a.wordsFn = nil, nil
		})
		ui.Text(c, T("Boşluk, Enter, silme ve hızlı yazma, her caret kendi tepkisini verir.")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
	})
}

// tryLetters are the field's letters within n of its caret, as nearLetters gives the text's.
func (a *app) tryLetters(text string, caret int, size, left, top, h float32, n int) []nearGlyph {
	l := ui.ShapeRichTextWith([]ui.Span{{Text: text, Size: size}}, a.font, ui.TextShape{})
	_, lh := l.Size()
	dy := top + max(0, (h-lh)/2)
	runes := []rune(text)
	at := func(i int) ui.Rect {
		return l.CaretAt(ui.TextCaretPosition{Index: units(text, i), Affinity: ui.TextDownstream})
	}
	var out []nearGlyph
	for i := max(0, caret-n); i < min(len(runes), caret+n); i++ {
		if r := runes[i]; r == ' ' || r == '\t' {
			continue
		}
		c0, c1 := at(i), at(i+1)
		if c1.X <= c0.X {
			continue
		}
		gy, gh := c0.Y, c0.H
		for _, ln := range l.Lines() {
			if mid := c0.Y + c0.H/2; mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H && ln.Ascent > 0 {
				gy, gh = ln.Baseline-ln.Ascent, ln.Ascent+ln.Descent
				break
			}
		}
		out = append(out, nearGlyph{ch: string(runes[i]), r: ui.Rect{X: left + c0.X, Y: dy + gy, W: c1.X - c0.X, H: gh}, off: i - caret, size: gh / 1.25})
	}
	return out
}

// tryWords is the field's text as a caret asks it of the page's (caretWords).
type tryWords struct {
	a                  *app
	text               string
	caret              int
	size, left, top, h float32
}

func (t *tryWords) last() string {
	runes := []rune(t.text)
	i := min(t.caret, len(runes))
	for i > 0 && !isWordRune(runes[i-1]) {
		i--
	}
	j := i
	for j > 0 && isWordRune(runes[j-1]) {
		j--
	}
	return string(runes[j:i])
}

func (t *tryWords) paraWords() int { return len(strings.Fields(t.text)) }

func (t *tryWords) find(word string) ([]ui.Rect, int) {
	hits := wordAt(foldRunes(t.text), foldRunes(word))
	if len(hits) == 0 {
		return nil, 0
	}
	l := ui.ShapeRichTextWith([]ui.Span{{Text: t.text, Size: t.size}}, t.a.font, ui.TextShape{})
	_, lh := l.Size()
	dy := t.top + max(0, (t.h-lh)/2)
	n := utf8.RuneCountInString(word)
	var rects []ui.Rect
	for _, at := range hits {
		for _, rc := range l.SelectionRects(ui.TextInputRange{Start: units(t.text, at), End: units(t.text, at+n)}) {
			rects = append(rects, ui.Rect{X: t.left + rc.X, Y: dy + rc.Y, W: rc.W, H: rc.H})
		}
	}
	return rects, len(hits)
}
