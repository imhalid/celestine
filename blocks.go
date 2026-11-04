package main

import (
	"regexp"
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

// hrToken stands for a divider while richtext reads or writes Markdown:
// letters only, which Markdown leaves as they are.
const hrToken = "RYUDITDIVIDER7F3A"

// hrLine is a thematic break: three or more -, * or _, spaces between allowed.
var hrLine = regexp.MustCompile(`^ {0,3}(?:(?:- *){3,}|(?:\* *){3,}|(?:_ *){3,})$`)

// parseBody reads a page's Markdown, its dividers too.
func parseBody(md string) richtext.Text {
	lines := strings.Split(md, "\n")
	fence, found := false, false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if fence || !hrLine.MatchString(l) {
			continue
		}
		// "---" right under a line of text underlines it as a heading
		if i > 0 && strings.TrimSpace(lines[i-1]) != "" && lines[i-1] != hrToken && strings.Contains(l, "-") {
			continue
		}
		lines[i] = hrToken
		found = true
	}
	if !found {
		return richtext.ParseMarkdown(md)
	}
	// each divider a paragraph of its own
	var sb strings.Builder
	for i, l := range lines {
		if l == hrToken {
			sb.WriteString("\n" + hrToken + "\n")
		} else {
			sb.WriteString(l)
		}
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	t := richtext.ParseMarkdown(sb.String())
	for _, at := range starts(t) {
		s, e := t.ParagraphRange(at)
		if t.Slice(s, e).String() != hrToken {
			continue
		}
		t = t.ReplaceString(s, e, "", richtext.Attributes{})
		t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
	}
	return t
}

// bodyMarkdown writes a page's text as Markdown, its dividers as "---".
func bodyMarkdown(t richtext.Text) string {
	found := false
	for _, at := range starts(t) {
		s, e := t.ParagraphRange(at)
		if t.ParagraphStyleAt(s).ID == hrID && s == e {
			t = t.ReplaceString(s, e, hrToken, richtext.Attributes{})
			t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
			found = true
		}
	}
	md := t.Markdown()
	if !found {
		return md
	}
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		if l == hrToken {
			lines[i] = "---"
		}
	}
	return strings.Join(lines, "\n")
}

// starts is where t's paragraphs start, last first: to change them from
// the end, the places of those before staying put.
func starts(t richtext.Text) []int {
	var out []int
	for at := range t.Paragraphs() {
		out = append([]int{at}, out...)
	}
	return out
}

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

// drawDividers draws the dividers in view: a hairline across the column,
// in the middle of their line.
func (a *app) drawDividers(p *ui.Painter, r ui.Rect) {
	text := a.doc.Text()
	col := p.Theme().Border
	for _, b := range a.boxes {
		if b.Start > text.Len() || text.ParagraphStyleAt(b.Start).ID != hrID {
			continue
		}
		if s, e := text.ParagraphRange(b.Start); s != e {
			continue
		}
		y := r.Y + b.Top + b.Height/2
		p.Fill(ui.Rect{X: r.X, Y: y - 0.5, W: r.W, H: 1}, col, 0.5)
	}
}
