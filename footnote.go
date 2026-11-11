package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Footnotes (#308). A reference, [^1] in the text, is a chip of kind "footnote", drawn as a small raised mark; a
// click on it goes to its note. The note, [^1]: … on a line of its own, is a paragraph of the page that starts so,
// written in as any other. Both are written to Markdown as they came.

// footRefInText is a footnote's reference as it is typed: [^1], [^note].
var footRefInText = regexp.MustCompile(`\[\^([^\]\s]+)\]`)

// footnotesIn turns the references in t into chips, outside code, and not the one that starts a note.
func footnotesIn(t richtext.Text) richtext.Text {
	s := t.String()
	if !strings.Contains(s, "[^") {
		return t
	}
	locs := footRefInText.FindAllStringSubmatchIndex(s, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		m := locs[i]
		start := utf8.RuneCountInString(s[:m[0]])
		end := start + utf8.RuneCountInString(s[m[0]:m[1]])
		if t.ParagraphStyleAt(start).CodeBlock || t.AttributesAt(start).Code {
			continue
		}
		if ps, _ := t.ParagraphRange(start); ps == start && strings.HasPrefix(s[m[1]:], ":") { // a note's own
			continue
		}
		ch := &chip{kind: "footnote", target: s[m[2]:m[3]]}
		attrs := t.AttributesAt(start)
		attrs.Attachment = &richtext.Attachment{Name: ch.label(), Data: ch}
		t = t.ReplaceString(start, end, string(richtext.ObjectReplacement), attrs)
	}
	return t
}

const footSize = 10 // the reference's mark: smaller than the text, raised

// footnoteAttachment is a reference drawn: its name, small and raised, in the accent.
func (a *app) footnoteAttachment(ch *chip) *richtext.Attachment {
	l := ui.ShapeRichTextWith([]ui.Span{{Text: ch.target, Size: footSize, Weight: 650}}, a.font, ui.TextShape{})
	w, _ := l.Size()
	return &richtext.Attachment{Width: w + 3, Height: chipBox, Name: ch.label(), Data: ch, Draw: func(p *ui.Painter, r ui.Rect) {
		cl := ryugo.Painting(p).Colors
		top := r.Y + r.H - 0.95*a.fontSize() // up by most of a letter's height
		hit := ui.Rect{X: r.X - 2, Y: top - 2, W: r.W + 4, H: footSize + 6}
		a.chipHits = append(a.chipHits, chipHit{hit, ch})
		col := cl.Accent
		if a.hoverCh == ch {
			p.Fill(hit, cl.Accent.Alpha(0.14), 3)
		}
		p.RichText(r.X+1.5, top, w+2, ui.Span{Text: ch.target, Size: footSize, Weight: 650, Color: col})
	}}
}

// noteAt is where the note of footnote id starts in t, after its "[^id]: ", or -1.
func noteAt(t richtext.Text, id string) int {
	prefix := "[^" + id + "]:"
	for start, p := range t.Paragraphs() {
		var sb strings.Builder
		for _, r := range p.Runs {
			sb.WriteString(r.Text)
			if sb.Len() > len(prefix)+1 {
				break
			}
		}
		if txt := sb.String(); strings.HasPrefix(txt, prefix) {
			n := utf8.RuneCountInString(prefix)
			if strings.HasPrefix(txt[len(prefix):], " ") {
				n++
			}
			return start + n
		}
	}
	return -1
}

// goToNote puts the caret in the note of footnote id, and the page goes to it; without one, it says so.
func (a *app) goToNote(id string) {
	at := noteAt(a.doc.Text(), id)
	if at < 0 {
		a.say(toastWarn, locales.EditorFootnoteMissing(id))
		return
	}
	a.doc.SetSelection(at, at)
	a.refocus, a.revealWant = true, true
}

var iconFootnote = icon(`<path d="M5 18h9M5 13h12M5 8h6"/><path d="M17.5 4.5v5M15.8 6h1.7"/>`)

// chooseFootnote takes the "/…" away, puts a reference at the caret, the next number the page has not used, and its
// note at the end of the page, the caret in the note to write it.
func (a *app) chooseFootnote() {
	s := &a.slash
	_, caret := a.doc.Selection()
	start := s.start
	s.open = false
	t := a.doc.Text()
	n := 1
	for _, r := range t.Runs(0, t.Len()) { // after the numbers used, references and notes alike
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "footnote" {
				if k, err := strconv.Atoi(ch.target); err == nil && k >= n {
					n = k + 1
				}
			}
		}
	}
	for noteAt(t, strconv.Itoa(n)) >= 0 {
		n++
	}
	id := strconv.Itoa(n)
	ch := &chip{kind: "footnote", target: id}
	attrs := t.AttributesAt(max(start-1, 0))
	attrs.Attachment = a.footnoteAttachment(ch)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.ReplaceString(start, start, string(richtext.ObjectReplacement), attrs)
		end := ed.Text().Len()
		note := "[^" + id + "]: "
		if end > 0 && !strings.HasSuffix(ed.Text().String(), "\n") {
			note = "\n" + note
		}
		ed.ReplaceString(end, end, note, richtext.Attributes{})
		e := ed.Text().Len()
		ed.FormatParagraphs(e, e, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		ed.Select(e, e)
	})
	a.refocus, a.revealWant = true, true
}
