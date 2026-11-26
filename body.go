package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
)

// A page's body is its own format, not Markdown: a list of paragraphs, each with the style the editor gives it (a
// heading, a list's item, a task, a quote, a block of code, a divider, a callout) and its runs of text, each with its
// marks. What richtext has no run for - a link to a page, a day, a page's card, a table, a picture - is an object in
// a run of its own, typed, that the file keeps whole. The document and its JSON are the library's (internal/library
// doc.go, #595): this file turns them into the editor's text and back, and the editor dresses the objects to be drawn
// afterwards (dress, in chips.go).

// The library's document, by the names this file and relink.go have always used.
type (
	bodyPara  = library.Para
	bodyStyle = library.ParaStyle
	bodyRun   = library.Run
	bodyObj   = library.Obj
)

func styleOf(st richtext.ParagraphStyle) bodyStyle {
	return bodyStyle{Heading: st.Heading, List: int(st.List), Checked: st.Checked, Indent: st.Indent, Align: int(st.Align),
		Quote: st.Quote, CodeBlock: st.CodeBlock, ID: st.ID}
}

// paraStyle is the editor's style of a paragraph of the page file.
func paraStyle(b bodyStyle) richtext.ParagraphStyle {
	return richtext.ParagraphStyle{Heading: b.Heading, List: richtext.ListKind(b.List), Checked: b.Checked, Indent: b.Indent,
		Align: ui.Align(b.Align), Quote: b.Quote, QuoteDepth: b.QuoteDepth, CodeBlock: b.CodeBlock, ID: b.ID}
}

// unknownKind marks an object of a kind this version does not know, in the editor: shown as a placeholder,
// written back as it came.
const unknownKind = library.UnknownKind

// inlineChipKinds are the chip kinds serialized directly as {k, target, alias} body objects.
var inlineChipKinds = map[string]bool{
	"page":     true,
	"date":     true,
	"embed":    true,
	"file":     true,
	"footnote": true,
	"check":    true, // a task in the running text, its target "x" when done (listmarks.go)
}

// bodyObjectDecoders maps every non-inline body object kind handled by attachmentOf to its decoder.
var bodyObjectDecoders = map[string]func(*bodyObj) *richtext.Attachment{
	"table": func(o *bodyObj) *richtext.Attachment {
		if len(o.Cols) == 0 {
			return nil
		}
		m := &tableModel{Cols: o.Cols, Rows: o.Rows, Filter: o.Q}
		for i, r := range m.Rows {
			for len(r) < len(m.Cols) {
				r = append(r, "")
			}
			m.Rows[i] = r[:len(m.Cols)]
		}
		return &richtext.Attachment{Name: locales.BlocksRawTable(), Data: &chip{kind: "table", tbl: m}}
	},
	"query": func(o *bodyObj) *richtext.Attachment {
		return &richtext.Attachment{Name: locales.CommonQuery(), Data: &chip{kind: "query", target: o.Q}}
	},
	"image": func(o *bodyObj) *richtext.Attachment {
		return &richtext.Attachment{Name: o.Alt, URL: o.Ref}
	},
}

// encodeBody writes the editor's text as the body: what is folded is put
// back (a fold is the editor's, never the page's).
func encodeBody(t richtext.Text) string {
	t = unfoldText(t)
	var d library.Doc
	for _, p := range t.Paragraphs() {
		bp := bodyPara{}
		if p.Style != (richtext.ParagraphStyle{}) {
			st := styleOf(p.Style)
			bp.S = &st
		}
		for _, r := range p.Runs {
			a := r.Attributes
			if at := a.Attachment; at != nil {
				if o := objOf(at); o != nil {
					bp.R = append(bp.R, bodyRun{O: o})
				}
				continue
			}
			run := bodyRun{T: r.Text, I: a.Italic, U: a.Underline, X: a.Strikethrough, C: a.Code, L: a.Link, G: a.Tag}
			if a.Weight != 0 && a.Weight != 400 {
				run.W = a.Weight
			}
			bp.R = append(bp.R, run)
		}
		d = append(d, bp)
	}
	return library.EncodeBody(d)
}

// objOf is the object an attachment is, or nil for one the page does not keep.
func objOf(at *richtext.Attachment) *bodyObj {
	switch d := at.Data.(type) {
	case *chip:
		if inlineChipKinds[d.kind] {
			return &bodyObj{K: d.kind, Target: d.target, Alias: d.alias}
		}
		switch d.kind {
		case "table":
			return &bodyObj{K: "table", Cols: d.tbl.Cols, Rows: d.tbl.Rows, Q: d.tbl.Filter}
		case "query":
			return &bodyObj{K: "query", Q: d.target}
		case unknownKind:
			return &bodyObj{K: unknownKind, Raw: d.raw}
		}
		return nil
	case *imageData:
		return &bodyObj{K: "image", Alt: at.Name, Ref: at.URL}
	case nil:
		if at.URL != "" { // a picture not yet dressed
			return &bodyObj{K: "image", Alt: at.Name, Ref: at.URL}
		}
	}
	return nil
}

// bodyReadable says why a body is not one decodeBody can read (library.BodyReadable): decodeBody gives the empty
// text for it, which must not be taken for the page's (#125, #132).
func bodyReadable(s string) error { return library.BodyReadable(s) }

// decodeBody reads a body into text, its objects as attachments that carry
// what they are and are not yet drawn. A body that cannot be read is empty.
func decodeBody(s string) richtext.Text {
	paras, err := library.DecodeBody(s)
	if err != nil || len(paras) == 0 {
		return richtext.Text{}
	}
	return textOf(paras)
}

// textOf is the library's document as the editor's text: its objects as attachments that carry what they are and
// are not yet drawn, in the marks of the text they are in.
func textOf(paras library.Doc) richtext.Text {
	out := make([]richtext.Paragraph, 0, len(paras))
	for _, bp := range paras {
		p := richtext.Paragraph{}
		if bp.S != nil {
			p.Style = paraStyle(*bp.S)
		}
		for _, r := range bp.R {
			a := richtext.Attributes{Italic: r.I, Underline: r.U, Strikethrough: r.X, Code: r.C, Link: r.L, Tag: r.G, Weight: r.W}
			if r.O != nil {
				att := attachmentOf(r.O)
				if att == nil {
					continue
				}
				a.Attachment = att
				r.T = string(richtext.ObjectReplacement)
			}
			p.Runs = append(p.Runs, richtext.Run{Text: r.T, Attributes: a})
		}
		out = append(out, p)
	}
	return richtext.TextOf(out...)
}

// docObjectsOf is the editor's text as the library's document with its objects, as a page's Markdown is written from
// it (library.Doc.ExportMarkdown): each chip the object the file has for it (objOf) and anything else drawn a picture
// of its name and its URL, each in the marks of the text it is in, and the depths of quotes kept. Folds and zooms
// are the editor's alone: unfoldText first.
func docObjectsOf(t richtext.Text) library.Doc {
	var d library.Doc
	for _, p := range t.Paragraphs() {
		bp := bodyPara{}
		if p.Style != (richtext.ParagraphStyle{}) {
			st := styleOf(p.Style)
			st.QuoteDepth = p.Style.QuoteDepth
			bp.S = &st
		}
		for _, r := range p.Runs {
			a := r.Attributes
			run := bodyRun{T: r.Text, I: a.Italic, U: a.Underline, X: a.Strikethrough, C: a.Code, L: a.Link, G: a.Tag}
			if a.Weight != 0 && a.Weight != 400 {
				run.W = a.Weight
			}
			if at := a.Attachment; at != nil {
				run.T, run.O = library.ObjectChar, objOf(at)
				if run.O == nil {
					run.O = &bodyObj{K: "image", Alt: at.Name, Ref: at.URL}
				}
			}
			bp.R = append(bp.R, run)
		}
		d = append(d, bp)
	}
	return d
}

// attachmentOf is the attachment of an object: the data of it, to be drawn later.
func attachmentOf(o *bodyObj) *richtext.Attachment {
	if inlineChipKinds[o.K] {
		ch := &chip{kind: o.K, target: o.Target, alias: o.Alias}
		return &richtext.Attachment{Name: ch.label(), Data: ch}
	}
	if dec := bodyObjectDecoders[o.K]; dec != nil {
		return dec(o)
	}
	if o.Raw != nil { // a kind of a newer version: kept, to be written back as it came
		ch := &chip{kind: unknownKind, raw: o.Raw}
		return &richtext.Attachment{Name: ch.label(), Data: ch}
	}
	return nil
}

// bodyText is the body as plain text, a line to a paragraph, as Markdown
// reads where it has a mark for it: a view to search in, to find tags and
// tasks and links in. It is not kept anywhere.
func bodyText(body string) string { return bodyMarkdown(decodeBody(body)) }

// appendLine puts a paragraph of text at the end of a body (library.AppendLine).
func appendLine(body, line string) string { return library.AppendLine(body, line) }
