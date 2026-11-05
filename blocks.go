package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Blocks richtext doesn't make on its own, made here:
//
//   - A divider: "---" typed in an empty paragraph turns it into a line
//     across the page, and the caret goes on below it. richtext has no
//     such paragraph, so a divider is an empty paragraph we name hrID
//     (ParagraphStyle.ID, which stays with it as it is edited) and draw a
//     line over; on disk it is Markdown's "---". Typing into it makes it a
//     paragraph again, and Backspace after it takes it away.
//   - A block of code at the third backtick: "```" typed in an empty
//     paragraph turns it into code at once. (richtext makes one only with
//     Enter after "```".)

const hrID = "ryudit:hr"

// blockShortcuts turns the paragraph the caret is at the end of into a
// divider or a block of code when what was just typed makes it "---" or
// "```". A divider with text typed into it is a paragraph again.
func (a *app) blockShortcuts(text richtext.Text, caret int, typed bool) bool {
	s, e := text.ParagraphRange(caret)
	st := text.ParagraphStyleAt(s)
	if st.ID == hrID && s != e {
		a.doc.Edit(func(ed *richtext.Edit) {
			ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { p.ID = "" })
		})
		return true
	}
	if !typed || caret != e || st.CodeBlock || st.Quote || st.List != richtext.NoList || st.Heading > 0 {
		return false
	}
	switch text.Slice(s, e).String() {
	case "---", "***", "___":
		a.doc.Edit(func(ed *richtext.Edit) {
			ed.ReplaceString(s, e, "\n", richtext.Attributes{})
			ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
			ed.FormatParagraphs(s+1, s+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
			ed.Select(s+1, s+1)
		})
	case "```":
		a.doc.Edit(func(ed *richtext.Edit) {
			ed.ReplaceString(s, e, "", richtext.Attributes{})
			ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{CodeBlock: true, Indent: p.Indent} })
			ed.Select(s, s)
		})
	default:
		return false
	}
	a.doc.SetTypingAttributes(richtext.Attributes{})
	return true
}

// drawDividers draws over the editor what richtext doesn't: the dividers
// in view, a hairline across the column in the middle of their line; and
// the name of a block of code's language, or of a raw block's kind, at its
// top right corner.
func (a *app) drawDividers(p *ui.Painter, r ui.Rect) {
	text := a.doc.Text()
	th := p.Theme()
	for _, b := range a.boxes {
		if b.Start > text.Len() {
			continue
		}
		st := text.ParagraphStyleAt(b.Start)
		if st.ID == hrID {
			if s, e := text.ParagraphRange(b.Start); s == e {
				y := r.Y + b.Top + b.Height/2
				p.Fill(ui.Rect{X: r.X, Y: y - 0.5, W: r.W, H: 1}, th.Border, 0.5)
			}
			continue
		}
		label := ""
		if lang, ok := strings.CutPrefix(st.ID, codeIDPrefix); ok {
			label = lang
		} else if kind, ok := strings.CutPrefix(st.ID, rawIDPrefix); ok {
			label = rawNames[kind] + " · Markdown"
		}
		if !st.CodeBlock || !(strings.HasPrefix(st.ID, codeIDPrefix) || strings.HasPrefix(st.ID, rawIDPrefix)) {
			continue
		}
		// a block right after another: a gap between them, which richtext
		// draws as one
		if b.Start > 0 && text.ParagraphStyleAt(b.Start-1).CodeBlock {
			p.Fill(ui.Rect{X: r.X - 2, Y: r.Y + b.Top - 2, W: r.W + 4, H: 2}, th.Background, 0)
		}
		if label == "" {
			continue
		}
		span := ui.Span{Text: label, Size: 11, Weight: 560, Color: th.TextMuted.Alpha(0.75)}
		w, h := p.MeasureText(200, span)
		p.RichText(r.X+r.W-w-10, r.Y+b.Top+max(0, (b.Height-h)/2), w+1, span)
	}
}
