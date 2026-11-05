package main

import (
	"regexp"
	"strings"
	"unicode/utf8"

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
		// a callout is tinted by its kind
		if kind, first := rawKindAt(text, b.Start); kind == "callout" {
			if col, ok := calloutTint(first); ok {
				p.Fill(ui.Rect{X: r.X, Y: r.Y + b.Top - 1, W: r.W, H: b.Height + 2}, col.Alpha(0.10), 0)
				p.Fill(ui.Rect{X: r.X, Y: r.Y + b.Top - 1, W: 3, H: b.Height + 2}, col.Alpha(0.85), 0)
			}
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
			p.Fill(ui.Rect{X: r.X - 2, Y: r.Y + b.Top - 2, W: r.W + 4, H: 2}, a.pageBg, 0)
		}
		if label == "" {
			continue
		}
		span := ui.Span{Text: label, Size: 11, Weight: 560, Color: th.TextMuted.Alpha(0.75)}
		w, h := p.MeasureText(200, span)
		p.RichText(r.X+r.W-w-10, r.Y+b.Top+max(0, (b.Height-h)/2), w+1, span)
	}
}

// A rawTemplate is a block put in from the Ekle… menu: Markdown richtext
// has no blocks for, a table and a callout, made as the raw blocks a page
// read from a file has - so they are edited as Markdown and written as it is.
type rawTemplate struct {
	kind, name, sub string
	lines           []string
	caret           int // the line the caret goes to
	glyph           *ui.SVG
}

var (
	iconTable   = icon(`<rect x="4.5" y="5.5" width="15" height="13" rx="2"/><path d="M4.5 10h15M4.5 14h15M10 5.5v13"/>`)
	iconCallout = icon(`<rect x="4.5" y="5.5" width="15" height="13" rx="3"/><path d="M12 9v4M12 15.4v.1"/>`)
)

var rawTemplates = []rawTemplate{
	{"table", "Tablo", "satırlar ve sütunlar", []string{"| Sütun 1 | Sütun 2 | Sütun 3 |", "| --- | --- | --- |", "|  |  |  |"}, 2, iconTable},
	{"callout", "Not kutusu", "[!note]", []string{"> [!note] Not", "> Metni buraya yaz."}, 1, iconCallout},
	{"callout", "Bilgi kutusu", "[!info]", []string{"> [!info] Bilgi", "> Metni buraya yaz."}, 1, iconCallout},
	{"callout", "İpucu kutusu", "[!tip]", []string{"> [!tip] İpucu", "> Metni buraya yaz."}, 1, iconCallout},
	{"callout", "Uyarı kutusu", "[!warning]", []string{"> [!warning] Uyarı", "> Metni buraya yaz."}, 1, iconCallout},
}

// insertRaw puts a raw block at start (the empty line the caret is on, or
// under the line it is in), with an empty paragraph after it to write on.
func (a *app) insertRaw(start int, r rawTemplate) (first int) {
	t := a.doc.Text()
	ps, pe := t.ParagraphRange(min(start, t.Len()))
	at, prefix := ps, ""
	if pe > ps { // a line with text: the block goes under it
		at, prefix = pe, "\n"
	}
	body := strings.Join(r.lines, "\n")
	first = at + utf8.RuneCountInString(prefix)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(at, at, richtext.NewText(prefix+body+"\n"))
		pos := first
		for i, ln := range r.lines {
			ed.FormatParagraphs(pos, pos, func(p *richtext.ParagraphStyle) {
				*p = richtext.ParagraphStyle{CodeBlock: true}
				if i == 0 {
					p.ID = rawIDPrefix + r.kind
				}
			})
			pos += utf8.RuneCountInString(ln) + 1
		}
		ed.FormatParagraphs(pos, pos, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		// the caret in the line asked for, at its end
		c := first
		for i := 0; i < r.caret; i++ {
			c += utf8.RuneCountInString(r.lines[i]) + 1
		}
		if r.kind == "table" {
			c += 2 // into the first cell
		} else {
			c += utf8.RuneCountInString(r.lines[r.caret])
		}
		ed.Select(c, c)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
	return first
}

var calloutKind = regexp.MustCompile(`\[!(\w+)\]`)

// rawKindAt is the kind of the raw block the paragraph at s is in, and the
// text of the block's first line.
func rawKindAt(t richtext.Text, s int) (kind, first string) {
	for s >= 0 {
		st := t.ParagraphStyleAt(s)
		if !st.CodeBlock {
			return "", ""
		}
		if k, ok := strings.CutPrefix(st.ID, rawIDPrefix); ok {
			ps, pe := t.ParagraphRange(s)
			return k, t.Slice(ps, pe).String()
		}
		if strings.HasPrefix(st.ID, codeIDPrefix) || s == 0 {
			return "", ""
		}
		s, _ = t.ParagraphRange(s - 1)
	}
	return "", ""
}

// calloutTint is the color of a callout of the kind the line names.
func calloutTint(line string) (ui.Color, bool) {
	m := calloutKind.FindStringSubmatch(line)
	if m == nil {
		return ui.Color{}, false
	}
	switch strings.ToLower(m[1]) {
	case "warning", "caution", "attention":
		return ui.Hex("#f5a524"), true
	case "danger", "error", "bug", "failure":
		return ui.Hex("#f31260"), true
	case "tip", "success", "check", "done", "hint":
		return ui.Hex("#17c964"), true
	case "question", "help", "faq":
		return ui.Hex("#a855f7"), true
	}
	return ui.Hex("#3b82f6"), true // note, info, abstract, todo, quote, …
}
