package main

import (
	"encoding/json"

	"github.com/egoist/mygo/ui/richtext"
)

// A page's body is its own format, not Markdown: a list of paragraphs, each
// with the style the editor gives it (a heading, a list's item, a task, a
// quote, a block of code, a divider, a callout) and its runs of text, each
// with its marks. What richtext has no run for - a link to a page, a day, a
// page's card, a table, a picture - is an object in a run of its own,
// typed, that this file keeps whole:
//
//	[{"s":{"heading":1},"r":[{"t":"Başlık"}]},
//	 {"r":[{"t":"bir "},{"t":"kalın","w":700},{"t":"￼","o":{"k":"page","target":"Dune"}}]},
//	 {"r":[{"t":"￼","o":{"k":"table","cols":[{"name":"Ad"}],"rows":[["elma"]]}}]}]
//
// The body is read and written without the app (no window, no fonts): the
// editor dresses its objects to be drawn afterwards (dress, in chips.go).

type bodyPara struct {
	S *richtext.ParagraphStyle `json:"s,omitempty"`
	R []bodyRun                `json:"r,omitempty"`
}

type bodyRun struct {
	T string   `json:"t,omitempty"`
	W int      `json:"w,omitempty"` // weight, when not the normal one
	I bool     `json:"i,omitempty"`
	U bool     `json:"u,omitempty"`
	X bool     `json:"x,omitempty"` // struck through
	C bool     `json:"c,omitempty"` // code
	L string   `json:"l,omitempty"` // link
	G string   `json:"g,omitempty"` // the app's own mark (richtext's Tag)
	O *bodyObj `json:"o,omitempty"`
}

// bodyObj is what takes the place of one character: k is "page", "date",
// "embed", "table" or "image".
type bodyObj struct {
	K      string     `json:"k"`
	Target string     `json:"target,omitempty"` // a page's title, or a day
	Alias  string     `json:"alias,omitempty"`
	Alt    string     `json:"alt,omitempty"` // a picture's
	Ref    string     `json:"ref,omitempty"`
	Cols   []tableCol `json:"cols,omitempty"` // a table's
	Rows   [][]string `json:"rows,omitempty"`
	Q      string     `json:"q,omitempty"` // a table's filter
}

// encodeBody writes the editor's text as the body: what is folded is put
// back (a fold is the editor's, never the page's).
func encodeBody(t richtext.Text) string {
	t = unfold(t)
	paras := []bodyPara{}
	for _, p := range t.Paragraphs() {
		bp := bodyPara{}
		if p.Style != (richtext.ParagraphStyle{}) {
			st := p.Style
			bp.S = &st
		}
		for _, r := range p.Runs {
			a := r.Attributes
			if at := a.Attachment; at != nil {
				if o := objOf(at); o != nil {
					bp.R = append(bp.R, bodyRun{T: string(richtext.ObjectReplacement), O: o})
				}
				continue
			}
			run := bodyRun{T: r.Text, I: a.Italic, U: a.Underline, X: a.Strikethrough, C: a.Code, L: a.Link, G: a.Tag}
			if a.Weight != 0 && a.Weight != 400 {
				run.W = a.Weight
			}
			bp.R = append(bp.R, run)
		}
		paras = append(paras, bp)
	}
	for len(paras) > 0 && paras[len(paras)-1].S == nil && len(paras[len(paras)-1].R) == 0 { // the empty lines that end the text are no part of it
		paras = paras[:len(paras)-1]
	}
	b, _ := json.Marshal(paras)
	return string(b)
}

// objOf is the object an attachment is, or nil for one the page does not keep.
func objOf(at *richtext.Attachment) *bodyObj {
	switch d := at.Data.(type) {
	case *chip:
		switch d.kind {
		case "page", "date", "embed":
			return &bodyObj{K: d.kind, Target: d.target, Alias: d.alias}
		case "table":
			return &bodyObj{K: "table", Cols: d.tbl.cols, Rows: d.tbl.rows, Q: d.tbl.Filter}
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

// decodeBody reads a body into text, its objects as attachments that carry
// what they are and are not yet drawn. A body that cannot be read is empty.
func decodeBody(s string) richtext.Text {
	var paras []bodyPara
	if json.Unmarshal([]byte(s), &paras) != nil || len(paras) == 0 {
		return richtext.Text{}
	}
	out := make([]richtext.Paragraph, 0, len(paras))
	for _, bp := range paras {
		p := richtext.Paragraph{}
		if bp.S != nil {
			p.Style = *bp.S
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

// attachmentOf is the attachment of an object: the data of it, to be drawn later.
func attachmentOf(o *bodyObj) *richtext.Attachment {
	switch o.K {
	case "page", "date", "embed":
		ch := &chip{kind: o.K, target: o.Target, alias: o.Alias}
		return &richtext.Attachment{Name: ch.label(), Data: ch}
	case "table":
		if len(o.Cols) == 0 {
			return nil
		}
		m := &tableModel{cols: o.Cols, rows: o.Rows, Filter: o.Q}
		for i, r := range m.rows {
			for len(r) < len(m.cols) {
				r = append(r, "")
			}
			m.rows[i] = r[:len(m.cols)]
		}
		return &richtext.Attachment{Name: "Tablo", Data: &chip{kind: "table", tbl: m}}
	case "image":
		return &richtext.Attachment{Name: o.Alt, URL: o.Ref}
	}
	return nil
}

// bodyText is the body as plain text, a line to a paragraph, as Markdown
// reads where it has a mark for it: a view to search in, to find tags and
// tasks and links in. It is not kept anywhere.
func bodyText(body string) string { return bodyMarkdown(decodeBody(body)) }

// appendLine puts a paragraph of text at the end of a body.
func appendLine(body, line string) string {
	var paras []bodyPara
	if json.Unmarshal([]byte(body), &paras) != nil {
		paras = nil
	}
	paras = append(paras, bodyPara{R: []bodyRun{{T: line}}})
	b, _ := json.Marshal(paras)
	return string(b)
}
