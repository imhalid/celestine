package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
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
	S *bodyStyle `json:"s,omitempty"`
	R []bodyRun  `json:"r,omitempty"`
}

// bodyStyle is a paragraph's style as the page file has it: the app's own struct, not MyGo's, so that a field
// renamed there does not change the file's format without a word (#132). Its names are those the files have.
type bodyStyle struct {
	Heading   int               `json:"heading,omitempty"`
	List      richtext.ListKind `json:"list,omitempty"`
	Checked   bool              `json:"checked,omitempty"`
	Indent    int               `json:"indent,omitempty"`
	Align     ui.Align          `json:"align,omitempty"`
	Quote     bool              `json:"quote,omitempty"`
	CodeBlock bool              `json:"codeBlock,omitempty"`
	ID        string            `json:"id,omitempty"`
}

func styleOf(st richtext.ParagraphStyle) bodyStyle {
	return bodyStyle{st.Heading, st.List, st.Checked, st.Indent, st.Align, st.Quote, st.CodeBlock, st.ID}
}

func (b bodyStyle) style() richtext.ParagraphStyle {
	return richtext.ParagraphStyle{Heading: b.Heading, List: b.List, Checked: b.Checked, Indent: b.Indent, Align: b.Align, Quote: b.Quote, CodeBlock: b.CodeBlock, ID: b.ID}
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
// "embed", "file", "table", "query" or "image".
type bodyObj struct {
	K      string     `json:"k"`
	Target string     `json:"target,omitempty"` // a page's title, or a day
	Alias  string     `json:"alias,omitempty"`
	Alt    string     `json:"alt,omitempty"` // a picture's
	Ref    string     `json:"ref,omitempty"`
	Cols   []tableCol `json:"cols,omitempty"` // a table's
	Rows   [][]string `json:"rows,omitempty"`
	Q      string     `json:"q,omitempty"` // a table's filter, or a query block's query (queryblock.go)
	// raw is the object as the file has it: one of a kind this version does not know (a newer version wrote
	// it) is kept and written back as it came, never dropped.
	raw []byte
}

// UnmarshalJSON reads an object, keeping it as it came too.
func (o *bodyObj) UnmarshalJSON(b []byte) error {
	type plain bodyObj
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*o = bodyObj(p)
	o.raw = append([]byte(nil), b...)
	return nil
}

// MarshalJSON writes an object; one this version does not know as it came.
func (o *bodyObj) MarshalJSON() ([]byte, error) {
	if o.K == unknownKind && o.raw != nil {
		return o.raw, nil
	}
	type plain bodyObj
	return json.Marshal((*plain)(o))
}

// unknownKind marks an object of a kind this version does not know, in the editor: shown as a placeholder,
// written back as it came.
const unknownKind = "?"

// inlineChipKinds are the chip kinds serialized directly as {k, target, alias} body objects.
var inlineChipKinds = map[string]bool{
	"page":     true,
	"date":     true,
	"embed":    true,
	"file":     true,
	"footnote": true,
}

// bodyObjectDecoders maps every non-inline body object kind handled by attachmentOf to its decoder.
var bodyObjectDecoders = map[string]func(*bodyObj) *richtext.Attachment{
	"table": func(o *bodyObj) *richtext.Attachment {
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
		return &richtext.Attachment{Name: locales.BlocksRawTable(), Data: &chip{kind: "table", tbl: m}}
	},
	"query": func(o *bodyObj) *richtext.Attachment {
		return &richtext.Attachment{Name: locales.CommonQuery(), Data: &chip{kind: "query", target: o.Q}}
	},
	"image": func(o *bodyObj) *richtext.Attachment {
		return &richtext.Attachment{Name: o.Alt, URL: o.Ref}
	},
}

// knownBlockKinds is derived directly from inlineChipKinds and bodyObjectDecoders so blockmark.go
// recognizes every body object kind handled by attachmentOf without maintaining a duplicate list.
var knownBlockKinds = func() map[string]bool {
	m := make(map[string]bool, len(inlineChipKinds)+len(bodyObjectDecoders))
	for k := range inlineChipKinds {
		m[k] = true
	}
	for k := range bodyObjectDecoders {
		m[k] = true
	}
	return m
}()

// encodeBody writes the editor's text as the body: what is folded is put
// back (a fold is the editor's, never the page's).
func encodeBody(t richtext.Text) string {
	t = unfold(t)
	paras := []bodyPara{}
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
					bp.R = append(bp.R, bodyRun{T: string(richtext.ObjectReplacement), O: o})
				}
				continue
			}
			run := bodyRun{T: r.Text, I: a.Italic, U: a.Underline, X: a.Strikethrough, C: a.Code, L: a.Link, G: a.Tag}
			if a.Weight != 0 && a.Weight != 400 {
				run.W = a.Weight
			}
			if n := len(bp.R); n > 0 && bp.R[n-1].O == nil && sameMarks(bp.R[n-1], run) { // one run: the editor makes it one at the next read
				bp.R[n-1].T += run.T
				continue
			}
			if run.T == "" {
				continue
			}
			bp.R = append(bp.R, run)
		}
		paras = append(paras, bp)
	}
	for len(paras) > 0 && paras[len(paras)-1].S == nil && len(paras[len(paras)-1].R) == 0 { // the empty lines that end the text are no part of it
		paras = paras[:len(paras)-1]
	}
	b, err := json.Marshal(paras)
	if err != nil { // not "" and not "[]", which read as an empty page: a body no page file takes, so the save fails (#135)
		return "!" + err.Error()
	}
	return string(b)
}

// sameMarks says whether two runs have the same marks, whatever their text.
func sameMarks(a, b bodyRun) bool {
	a.T, b.T = "", ""
	return a.O == nil && b.O == nil && a == b
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
			return &bodyObj{K: "table", Cols: d.tbl.cols, Rows: d.tbl.rows, Q: d.tbl.Filter}
		case "query":
			return &bodyObj{K: "query", Q: d.target}
		case unknownKind:
			return &bodyObj{K: unknownKind, raw: d.raw}
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

// bodyReadable says why a body is not one decodeBody can read: a value of the wrong type, or a shape of a later
// version. decodeBody gives the empty text for it, which must not be taken for the page's (#125).
// A field this version does not know, in a paragraph, its style or a run, is a newer version's: the page is read
// but not written, as saving would drop it (#132).
func bodyReadable(s string) error {
	var paras []bodyPara
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&paras); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return errors.New("the page was written by a newer version of Celestine: " + err.Error())
		}
		return errors.New("the page's text is in a form this version cannot read: " + err.Error())
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
			p.Style = bp.S.style()
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
	if inlineChipKinds[o.K] {
		ch := &chip{kind: o.K, target: o.Target, alias: o.Alias}
		return &richtext.Attachment{Name: ch.label(), Data: ch}
	}
	if dec := bodyObjectDecoders[o.K]; dec != nil {
		return dec(o)
	}
	if o.raw != nil { // a kind of a newer version: kept, to be written back as it came
		ch := &chip{kind: unknownKind, raw: o.raw}
		return &richtext.Attachment{Name: ch.label(), Data: ch}
	}
	return nil
}

// bodyText is the body as plain text, a line to a paragraph, as Markdown
// reads where it has a mark for it: a view to search in, to find tags and
// tasks and links in. It is not kept anywhere.
func bodyText(body string) string { return bodyMarkdown(decodeBody(body)) }

// appendLine puts a paragraph of text at the end of a body.
func appendLine(body, line string) string {
	// the new paragraph put after the others in the JSON as it is: decoding and encoding it again would drop
	// what this version does not know (#132). A body that cannot be read is refused before (bodyReadable).
	b, _ := json.Marshal(bodyPara{R: []bodyRun{{T: line}}})
	t := strings.TrimSpace(body)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") {
		return "[" + string(b) + "]"
	}
	if strings.TrimSpace(t[1:len(t)-1]) == "" {
		return "[" + string(b) + "]"
	}
	return t[:len(t)-1] + "," + string(b) + "]"
}

// editorFormat is the editor's format, which the library reads its bodies in (internal/library/bodyformat.go).
type editorFormat struct{}

func (editorFormat) Markdown(body string) string   { return bodyText(body) }
func (editorFormat) Plain(body string) string      { return decodeBody(body).PlainText() }
func (editorFormat) FromMarkdown(md string) string { return encodeBody(importMarkdown(md)) }

func (editorFormat) Blank(body string) bool {
	return strings.TrimSpace(strings.ReplaceAll(decodeBody(body).String(), "\n", "")) == ""
}

func init() { library.Format = editorFormat{} }
