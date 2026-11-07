package main

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/richtext"
)

// The motions of Vim in the normal mode, in the text, when the setting asks for them (Ayarlar ▸ Editör ▸
// "Vim hareketleri"): h j k l, w b e, 0 $, gg G, x, dd, yy, p, o O, A I. They work where there are no
// rows to move over, that is, on a page. Without the setting the normal mode still has the rest of the
// keys, and the text is edited with the keys it always was.

func (a *app) vimOn() bool {
	return a.settings.VimKeys && a.page != nil && a.doc != nil && !a.noPage() && len(a.kb.rows()) == 0
}

func vimIsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// vimWordStart is the start of the next word from i, or n.
func vimWordStart(rs []rune, i int) int {
	n := len(rs)
	if i >= n {
		return n
	}
	if vimIsWord(rs[i]) {
		for i < n && vimIsWord(rs[i]) {
			i++
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
		for i > 0 && vimIsWord(rs[i-1]) {
			i--
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
	k := vimIsWord(rs[i])
	for i+1 < n && !unicode.IsSpace(rs[i+1]) && vimIsWord(rs[i+1]) == k {
		i++
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
	rs := []rune(t.String())
	total := len(rs)
	ps, pe := t.ParagraphRange(min(caret, max(total-1, 0)))
	put := func(i int) {
		i = min(max(i, 0), total)
		d.SetSelection(i, i)
		a.revealWant = true
	}
	if strings.ContainsRune("xpoOAI", r) {
		a.kb.again.r, a.kb.again.n, a.kb.again.pair = r, n, ""
	}
	switch r {
	case 'h', kbLeft:
		put(max(caret-n, ps))
	case 'l', kbRight:
		put(min(caret+n, pe))
	case 'j', kbDown, 'k', kbUp:
		down := r == 'j' || r == kbDown
		col := caret - ps
		at := ps
		for i := 0; i < n; i++ {
			if down {
				if pe >= total {
					break
				}
				at = pe + 1
			} else {
				if at <= 0 {
					break
				}
				at, _ = t.ParagraphRange(at - 1)
			}
			ps2, pe2 := t.ParagraphRange(at)
			at, ps, pe = ps2, ps2, pe2
		}
		put(min(at+col, pe))
	case 'w':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordStart(rs, i)
		}
		put(i)
	case 'b':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordBack(rs, i)
		}
		put(i)
	case 'e':
		i := caret
		for k := 0; k < n; k++ {
			i = vimWordEnd(rs, i)
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
			e := min(caret+n, pe)
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

// vimPair does the second key of dd and yy: the paragraph is deleted, or taken into the register.
func (a *app) vimPair(first, second rune) bool {
	if !a.vimOn() || first != second || (first != 'd' && first != 'y') {
		return false
	}
	d := a.doc
	t := d.Text()
	_, caret := d.Selection()
	ps, pe := t.ParagraphRange(min(caret, max(t.Len()-1, 0)))
	a.vimReg = t.Slice(ps, pe)
	if first == 'd' {
		a.kb.again.r, a.kb.again.n, a.kb.again.pair = 'd', 1, "dd"
	}
	if first == 'y' {
		a.notice = "Paragraf kopyalandı"
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
