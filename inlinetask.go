package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// A task in the running text (#536) is a box and the words it owns: those after it, up to the next box, the end of
// its line, or its end (#610), an object of its own that Tab puts where the caret is, drawn as a thin bar. While the
// caret is in a task, its words are shown as one, so that it is plain what the task is and where it stops; Tab there
// ends it and the caret goes on outside it, and does not indent the line.

// inlineSegment is what box k of a paragraph's line owns, [from, to), and where its end is in the line (-1 for none):
// the words after it up to the next mark (a box or an end), or, with nothing after it and no end of its own, those
// before it, from the mark before.
func inlineSegment(line []rune, marks []inlineBox, k int) (from, to, end int) {
	b := marks[k]
	from, to, end = b.at+1, len(line), -1
	if k+1 < len(marks) {
		to = marks[k+1].at
		if marks[k+1].end {
			end = to
		}
	}
	if end < 0 && strings.TrimFunc(string(line[from:to]), notWord) == "" {
		from, to = 0, b.at
		if k > 0 {
			from = marks[k-1].at + 1
		}
	}
	return from, to, end
}

// inlineMarks are the boxes and the ends of tasks in paragraph p's line, a rune each of line, which starts at rune
// start of t.
func inlineMarks(t richtext.Text, start int, line []rune) []inlineBox {
	var marks []inlineBox
	for i, r := range line {
		if r != richtext.ObjectReplacement {
			continue
		}
		if att := t.AttributesAt(start + i).Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && (ch.kind == "check" || ch.kind == "checkend") {
				marks = append(marks, inlineBox{at: i, done: ch.target == "x", end: ch.kind == "checkend"})
			}
		}
	}
	return marks
}

// inlineActive is the task in the running text the caret is in: its paragraph, and its box and words, [box, to) of
// the paragraph's runes. The caret is in it from right after its box to the end of its words, its end not included.
type inlineActive struct {
	ok          bool
	p, box, to  int
	ps, end     int // where its paragraph starts in the text, and where its end is in the line (-1: none)
	text        richtext.Text
	atParaEnd   bool // its words run to the end of their line
	caretInLine int
}

// inlineTaskAt is the task in the running text of pn's text that the caret is in; ok false when it is in none, or
// the selection is not a caret.
func (a *app) inlineTaskAt(pn *paneState) inlineActive {
	if pn.doc == nil {
		return inlineActive{}
	}
	anchor, caret := pn.doc.Selection()
	if anchor != caret {
		return inlineActive{}
	}
	t := pn.doc.Text()
	ps, pe := t.ParagraphRange(caret)
	if t.ParagraphStyleAt(ps).CodeBlock || pe-ps > maxCaretParagraph {
		return inlineActive{}
	}
	line := []rune(t.Slice(ps, pe).String())
	if !slices.Contains(line, richtext.ObjectReplacement) {
		return inlineActive{}
	}
	marks := inlineMarks(t, ps, line)
	c := caret - ps
	for k, m := range marks {
		if m.end {
			continue
		}
		to, end := len(line), -1 // what it owns after it, words or not yet
		if k+1 < len(marks) {
			to = marks[k+1].at
			if marks[k+1].end {
				end = to
			}
		}
		if c > m.at && c <= to && !(end >= 0 && c > end) {
			p := a.paraIndex(pn, t, ps)
			return inlineActive{ok: true, p: p, box: m.at, to: to, ps: ps, end: end, text: t, atParaEnd: to == len(line) && end < 0, caretInLine: c}
		}
	}
	return inlineActive{}
}

// paraIndex is the index of the paragraph that starts at rune ps of t.
func (a *app) paraIndex(pn *paneState, t richtext.Text, ps int) int {
	pt := a.paraTableOf(pn, t)
	i, _ := slices.BinarySearch(pt.start, ps)
	return i
}

// inlineTaskKey says which task in the running text the caret is in, for the editor to ask for the spans of the text
// again when it is another (spansVersion): 0 for none.
func (a *app) inlineTaskKey(pn *paneState) int {
	act := a.inlineTaskAt(pn)
	pn.inlineAct = act
	if !act.ok {
		return 0
	}
	return 1 + act.p*7919 + act.box*31 + act.to
}

// inlineActiveSpans shows the task the caret is in as one: a soft band of BetterTask's cyan under its box, its words
// and its end (#613), the box and the end lit with it.
func (a *app) inlineActiveSpans(pn *paneState, p int, spans []ui.Span) []ui.Span {
	act := pn.inlineAct
	if !act.ok || act.p != p || a.theme == nil {
		return spans
	}
	band := mix(a.pageBg, inlineAccent(darkBg(a.pageBg)), 0.13) // opaque: where its pieces meet it is not darker
	to := act.to
	if act.end >= 0 { // the band takes in its end, its closing bracket
		to = act.end + 1
	}
	return restyleRunes(spans, [][2]int{{act.box, to}}, func(s *ui.Span) {
		s.Background, s.Radius = band, 5
	})
}

// inlineChipActive reports whether ch is the box of an active inline task in any pane.
func (a *app) inlineChipActive(ch *chip) bool {
	for i := range a.ps.states {
		act := a.ps.states[i].inlineAct
		if !act.ok || act.box+act.ps >= act.text.Len() {
			continue
		}
		if att := act.text.AttributesAt(act.ps + act.box).Attachment; att != nil && att.Data == any(ch) {
			return true
		}
	}
	return false
}

// tabOutOfInlineTask is Tab in a task in the running text: past its end when it has one, else an end put where the
// caret is (and a space after it at the end of the line), the caret after it. It reports whether the caret was in
// one.
func (a *app) tabOutOfInlineTask(pn *paneState) bool {
	act := a.inlineTaskAt(pn)
	if !act.ok {
		return false
	}
	if act.end >= 0 {
		pn.doc.SetSelection(act.ps+act.end+1, act.ps+act.end+1)
		return true
	}
	caret := act.ps + act.caretInLine
	attrs := act.text.AttributesAt(max(caret-1, act.ps))
	attrs.Attachment = nil
	plain := attrs
	attrs.Attachment = a.attachment(&chip{kind: "checkend"})
	atEnd := act.caretInLine == act.to && act.atParaEnd
	pn.doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(caret, caret, string(richtext.ObjectReplacement), attrs)
		next := caret + 1
		if atEnd {
			ed.ReplaceString(next, next, " ", plain)
			next++
		}
		ed.Select(next, next)
	})
	pn.doc.SetTypingAttributes(plain)
	return true
}

// checkEndW is how wide the end of a task in the running text is in its line.
func checkEndW(size float32) float32 { return max(6, size*0.4) }

// drawCheckEnd draws the end of a task in the running text as its closing bracket, as tall as the letters (#613): in
// BetterTask's cyan while the caret is in its task, faint otherwise.
func (a *app) drawCheckEnd(p *ui.Painter, r ui.Rect, ch *chip) {
	th := a.theme
	if th == nil {
		return
	}
	size := a.fontSize()
	dark := darkBg(a.pageBg)
	h := size * 0.86
	bottom := r.Y + r.H + size*0.1
	col := rgba(148, 163, 184, map[bool]float32{true: 0.5, false: 0.6}[dark])
	for i := range a.ps.states {
		act := a.ps.states[i].inlineAct
		if act.ok && act.end >= 0 && act.ps+act.end < act.text.Len() {
			if att := act.text.AttributesAt(act.ps + act.end).Attachment; att != nil && att.Data == any(ch) {
				col = inlineAccent(dark).Alpha(0.9)
				break
			}
		}
	}
	w := max(1.4, size*0.09)
	x := r.X + r.W/2 + size*0.06
	hook := size * 0.16
	var path ui.Path
	path.MoveTo(x-hook, bottom-h).LineTo(x, bottom-h).LineTo(x, bottom).LineTo(x-hook, bottom)
	p.StrokePath(path.Cap(ui.RoundCap).Join(ui.RoundJoin), w, col)
}

// checkEndLabel is what the end of a task says to assistive technology.
func checkEndLabel() string { return locales.ListsInlineTaskEnd() }
