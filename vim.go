package main

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// The motions of Vim in the normal mode, in the text, when the setting asks for them (Ayarlar ▸ Editör ▸
// "Vim hareketleri"): h j k l, w b e, 0 $, gg G, x, dd, yy, p, o O, A I. They work where there are no
// rows to move over, that is, on a page. Without the setting the normal mode still has the rest of the
// keys, and the text is edited with the keys it always was.

func (a *app) vimOn() bool {
	return a.settings.VimKeys && a.page != nil && a.doc != nil && !a.noPage() && len(a.kb.rows()) == 0
}

func vimIsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// vimClass is the class of a letter of a word, as Vim has them: the letters of a script written without spaces
// are each a word of their own class, so w stops where hiragana turns to kanji, and in a run of ideographs at
// each of them, rather than taking the whole sentence as one word (#206). The rest of the letters are one class.
func vimClass(r rune) int {
	switch {
	case unicode.Is(unicode.Han, r):
		return 0x4e00
	case unicode.Is(unicode.Hiragana, r):
		return 0x3040
	case unicode.Is(unicode.Katakana, r):
		return 0x30a0
	case unicode.Is(unicode.Hangul, r):
		return 0xac00
	}
	return 2
}

// vimSame says whether b goes on the word that a is in: a letter of the same class, and an ideograph never.
func vimSame(a, b rune) bool {
	return vimIsWord(a) && vimIsWord(b) && vimClass(a) == vimClass(b) && vimClass(a) != 0x4e00
}

// vimWordStart is the start of the next word from i, or n.
func vimWordStart(rs []rune, i int) int {
	n := len(rs)
	if i >= n {
		return n
	}
	if vimIsWord(rs[i]) {
		for i++; i < n && vimSame(rs[i-1], rs[i]); i++ {
		}
	} else if !unicode.IsSpace(rs[i]) {
		for i < n && !vimIsWord(rs[i]) && !unicode.IsSpace(rs[i]) {
			i++
		}
	}
	for i < n && unicode.IsSpace(rs[i]) && rs[i] != '\n' {
		i++
	}
	if i < n && rs[i] == '\n' {
		return i + 1
	}
	return i
}

// vimWordBack is the start of the word before i, or 0.
func vimWordBack(rs []rune, i int) int {
	i = min(i, len(rs))
	for i > 0 && (unicode.IsSpace(rs[i-1])) {
		i--
	}
	if i == 0 {
		return 0
	}
	if vimIsWord(rs[i-1]) {
		for i--; i > 0 && vimSame(rs[i-1], rs[i]); i-- {
		}
	} else {
		for i > 0 && !vimIsWord(rs[i-1]) && !unicode.IsSpace(rs[i-1]) {
			i--
		}
	}
	return i
}

// vimWordEnd is the last letter of the word at or after i.
func vimWordEnd(rs []rune, i int) int {
	n := len(rs)
	i++
	for i < n && unicode.IsSpace(rs[i]) {
		i++
	}
	if i >= n {
		return max(n-1, 0)
	}
	if vimIsWord(rs[i]) {
		for i+1 < n && vimSame(rs[i], rs[i+1]) {
			i++
		}
	} else {
		for i+1 < n && !unicode.IsSpace(rs[i+1]) && !vimIsWord(rs[i+1]) {
			i++
		}
	}
	return i
}

// vimKey does a key of the page's normal mode, n times; it reports that it took the key.
func (a *app) vimKey(r rune, n int) bool {
	if !a.vimOn() {
		return false
	}
	d := a.doc
	t := d.Text()
	_, caret := d.Selection()
	// the text's runes, for the word motions alone: made at every key, they were the whole page's (#198)
	var rs []rune
	runes := func() []rune {
		if rs == nil {
			rs = []rune(t.String())
		}
		return rs
	}
	total := t.Len()
	ps, pe := t.ParagraphRange(caret)
	put := func(i int) {
		i = min(max(i, 0), total)
		d.SetSelection(i, i)
		a.revealWant = true
	}
	if strings.ContainsRune("xpoOAI", r) {
		a.kb.again.r, a.kb.again.n, a.kb.again.pair = r, n, ""
	}
	// steps goes n letters on from i, or back, as the eye has them (vimStep), in the paragraph
	steps := func(i, dir int) int {
		for k := 0; k < n; k++ {
			i = a.vimStep(ps, pe, i, dir)
		}
		return i
	}
	switch r {
	case 'h', kbLeft:
		put(steps(caret, -1))
	case 'l', kbRight:
		put(steps(caret, 1))
	case 'j', kbDown, 'k', kbUp:
		dir := 1
		if r == 'k' || r == kbUp {
			dir = -1
		}
		at, x := caret, float32(-1)
		for k := 0; k < n; k++ {
			at, x = a.vimLine(at, dir, x)
		}
		put(at)
	case 'w':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordStart(runes(), i)
		}
		put(i)
	case 'b':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordBack(runes(), i)
		}
		put(i)
	case 'e':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordEnd(runes(), i)
		}
		put(i)
	case '0':
		put(ps)
	case '$':
		put(pe)
	case 'G':
		put(total)
	case 'x':
		if caret < pe {
			e := steps(caret, 1)
			d.Edit(func(ed *richtext.Edit) { ed.Replace(caret, e, richtext.Text{}) })
		}
	case 'p':
		if a.vimReg.Len() > 0 {
			ins := a.vimReg
			d.Edit(func(ed *richtext.Edit) {
				ed.Replace(pe, pe, ins.Insert(0, richtext.NewText("\n")))
				ed.Select(pe+1, pe+1)
			})
			a.revealWant = true
		}
	case 'o', 'O':
		at := pe
		if r == 'O' {
			at = ps
		}
		d.Edit(func(ed *richtext.Edit) {
			ed.ReplaceString(at, at, "\n", richtext.Attributes{})
			if r == 'O' {
				ed.Select(at, at)
			} else {
				ed.Select(at+1, at+1)
			}
		})
		a.kbLeave()
	case 'A':
		put(pe)
		a.kbLeave()
	case 'I':
		put(ps)
		a.kbLeave()
	default:
		return false
	}
	return true
}

// vimLaid is the paragraph from ps to pe laid out as the editor lays it out, and its text, for the motions that
// go by what is seen; ok is false where it cannot be: no editor yet, or a paragraph too long to lay out again.
func (a *app) vimLaid(ps, pe int) (l *ui.TextLayout, line string, ok bool) {
	if a.editorR.W <= 0 || pe-ps > maxCaretParagraph || a.theme == nil {
		return nil, "", false
	}
	t := a.doc.Text()
	var para richtext.Paragraph
	for _, p := range t.Slice(ps, pe).Paragraphs() {
		para = p
		break
	}
	para.Style = t.ParagraphStyleAt(ps)
	for _, r := range para.Runs {
		line += r.Text
	}
	l, _ = longMemo.layout(a.doc, ps, pe-ps, para, a.editorR.W, a.font, a.theme)
	return l, line, l != nil
}

// vimStep is the place one letter on from i, or back (dir), in the paragraph from ps to pe, as the eye has the
// letters: a family of emoji, a flag, a letter and its mark are one, which h, l and x do not cut in two (#206).
func (a *app) vimStep(ps, pe, i, dir int) int {
	if l, line, ok := a.vimLaid(ps, pe); ok {
		u := units(line, i-ps)
		if dir > 0 {
			u = l.NextBoundary(u)
		} else {
			u = l.PreviousBoundary(u)
		}
		i = ps + runesIn(line, u)
	} else {
		i += dir
	}
	return min(max(i, ps), pe)
}

// vimLine is the place a line down from i, or up (dir), as the paragraph wraps, at x across the line, or where
// i is when x is below 0; it gives the x it kept, for the next line of a count. A long paragraph is walked line
// by line, not jumped over (#206); where there is no layout, the line is the paragraph, at the same column.
func (a *app) vimLine(i, dir int, x float32) (int, float32) {
	t := a.doc.Text()
	ps, pe := t.ParagraphRange(i)
	l, line, ok := a.vimLaid(ps, pe)
	if !ok {
		col := i - ps
		switch {
		case dir > 0 && pe < t.Len():
			ps, pe = t.ParagraphRange(pe + 1)
		case dir < 0 && ps > 0:
			ps, pe = t.ParagraphRange(ps - 1)
		default:
			return i, x
		}
		return min(ps+col, pe), x
	}
	c := l.CaretAt(ui.TextCaretPosition{Index: units(line, i-ps), Affinity: ui.TextDownstream})
	if x < 0 {
		x = c.X
	}
	lines := l.Lines()
	at := 0
	for k, ln := range lines {
		if mid := c.Y + c.H/2; mid >= ln.Rect.Y && mid <= ln.Rect.Y+ln.Rect.H {
			at = k
		}
	}
	if k := at + dir; k >= 0 && k < len(lines) { // a line of the same paragraph
		return ps + runesIn(line, l.IndexAt(ui.Point{X: x, Y: lines[k].Rect.Y + lines[k].Rect.H/2})), x
	}
	switch { // the paragraph after or before: its first line, or its last
	case dir > 0 && pe < t.Len():
		ps, pe = t.ParagraphRange(pe + 1)
	case dir < 0 && ps > 0:
		ps, pe = t.ParagraphRange(ps - 1)
	default:
		return i, x
	}
	l, line, ok = a.vimLaid(ps, pe)
	if !ok {
		return ps, x
	}
	lines = l.Lines()
	if len(lines) == 0 {
		return ps, x
	}
	ln := lines[0]
	if dir < 0 {
		ln = lines[len(lines)-1]
	}
	return min(ps+runesIn(line, l.IndexAt(ui.Point{X: x, Y: ln.Rect.Y + ln.Rect.H/2})), pe), x
}

// runesIn is the rune of line that u UTF-16 units of it reach, as units is the other way.
func runesIn(line string, u int) int {
	n := 0
	for _, r := range line {
		if u <= 0 {
			break
		}
		if r > 0xFFFF {
			u -= 2
		} else {
			u--
		}
		n++
	}
	return n
}

// vimPair does the second key of dd and yy: the paragraph is deleted, or taken into the register.
func (a *app) vimPair(first, second rune) bool {
	if !a.vimOn() || first != second || (first != 'd' && first != 'y') {
		return false
	}
	d := a.doc
	t := d.Text()
	_, caret := d.Selection()
	ps, pe := t.ParagraphRange(caret)
	a.vimReg = t.Slice(ps, pe)
	if first == 'd' {
		a.kb.again.r, a.kb.again.n, a.kb.again.pair = 'd', 1, "dd"
	}
	if first == 'y' {
		a.say(toastOK, locales.ParagraphCopied())
		return true
	}
	end := pe + 1
	d.Edit(func(ed *richtext.Edit) {
		if end > t.Len() { // the last: its newline is the one before
			ed.Replace(max(ps-1, 0), t.Len(), richtext.Text{})
		} else {
			ed.Replace(ps, end, richtext.Text{})
		}
	})
	return true
}
