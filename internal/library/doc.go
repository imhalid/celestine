package library

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/imhalid/celestine/internal/table"
)

// A page's body as the library knows it (#595): a typed document of paragraphs, each with its style and its runs of
// text, each run with its marks or an object (a link to a page, a day, a table, a picture…) in a run of its own. It is
// what page.json keeps, read and written here without the editor: the app turns it into its editor's text and back
// (body.go), and dresses the objects to be drawn. The JSON is the file's, and the names in it are the files':
//
//	[{"s":{"heading":1},"r":[{"t":"Başlık"}]},
//	 {"r":[{"t":"bir "},{"t":"kalın","w":700},{"t":"\uFFFC","o":{"k":"page","target":"Dune"}}]}]

// A Doc is a page's body: its paragraphs, in order.
type Doc []Para

// A Para is a paragraph: its style, nil for a plain one, and its runs.
type Para struct {
	S *ParaStyle `json:"s,omitempty"`
	R []Run      `json:"r,omitempty"`
}

// ParaStyle is a paragraph's style as the page file has it.
type ParaStyle struct {
	Heading   int    `json:"heading,omitempty"`
	List      int    `json:"list,omitempty"` // 0 none, then the editor's kinds of list: bullets, numbers, tasks
	Checked   bool   `json:"checked,omitempty"`
	Indent    int    `json:"indent,omitempty"`
	Align     int    `json:"align,omitempty"`
	Quote     bool   `json:"quote,omitempty"`
	CodeBlock bool   `json:"codeBlock,omitempty"`
	ID        string `json:"id,omitempty"`
	// QuoteDepth is how many quotations deep a quote is, when more than one: what Markdown read in says, which the
	// page file does not keep.
	QuoteDepth int `json:"-"`
}

// A Run is a piece of a paragraph with the same marks, or an object.
type Run struct {
	T string `json:"t,omitempty"`
	W int    `json:"w,omitempty"` // weight, when not the normal one
	I bool   `json:"i,omitempty"`
	U bool   `json:"u,omitempty"`
	X bool   `json:"x,omitempty"` // struck through
	C bool   `json:"c,omitempty"` // code
	L string `json:"l,omitempty"` // link
	G string `json:"g,omitempty"` // the app's own mark
	O *Obj   `json:"o,omitempty"`
}

// The kinds of list a paragraph is an item of, as the page file numbers them (the editor's).
const (
	ListNone = iota
	ListBullets
	ListNumbers
	ListTasks
)

// text is the paragraph's text, its objects as the character they stand as.
func (p Para) text() string {
	var b strings.Builder
	for _, r := range p.R {
		if r.O != nil {
			b.WriteString(ObjectChar)
			continue
		}
		b.WriteString(r.T)
	}
	return b.String()
}

// ObjectChar is the character an object stands as in its run's text.
const ObjectChar = "\uFFFC"

// An Obj is what takes the place of one character: K is its kind, "page", "date", "embed", "file", "footnote",
// "check", "table", "query" or "image", or one of a newer version, which is kept as it came (Raw).
type Obj struct {
	K      string      `json:"k"`
	Target string      `json:"target,omitempty"` // a page's title, a day, a file's name, a check's "x" when done
	Alias  string      `json:"alias,omitempty"`
	Alt    string      `json:"alt,omitempty"` // a picture's
	Ref    string      `json:"ref,omitempty"`
	Cols   []table.Col `json:"cols,omitempty"` // a table's
	Rows   [][]string  `json:"rows,omitempty"`
	Q      string      `json:"q,omitempty"` // a table's filter, or a query block's query
	// Raw is the object as the file has it: one of a kind this version does not know (a newer version wrote it) is
	// kept and written back as it came, never dropped. Unknown marks it so (UnknownKind).
	Raw []byte `json:"-"`
}

// UnknownKind marks an object of a kind this version does not know: written back as it came (Obj.Raw).
const UnknownKind = "?"

// UnmarshalJSON reads an object, keeping it as it came too.
func (o *Obj) UnmarshalJSON(b []byte) error {
	type plain Obj
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*o = Obj(p)
	o.Raw = append([]byte(nil), b...)
	return nil
}

// MarshalJSON writes an object; one this version does not know (of a kind it does not read, or the editor's
// placeholder for one) as it came.
func (o *Obj) MarshalJSON() ([]byte, error) {
	if !o.Known() && o.Raw != nil {
		return o.Raw, nil
	}
	type plain Obj
	return json.Marshal((*plain)(o))
}

// KnownObjKinds are the kinds of object this version reads.
var KnownObjKinds = map[string]bool{"page": true, "date": true, "embed": true, "file": true, "footnote": true, "check": true, "checkend": true,
	"table": true, "query": true, "image": true}

// Known reports whether the object is of a kind this version reads.
func (o *Obj) Known() bool { return KnownObjKinds[o.K] }

// DecodeBody reads a body. One that cannot be read is the empty Doc and an error: it must not be taken for the
// page's (BodyReadable says why).
func DecodeBody(s string) (Doc, error) {
	var d Doc
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil, err
	}
	for i := range d { // a table's rows as long as its columns, and a table without columns none, as the editor reads them
		rs := d[i].R[:0]
		for _, r := range d[i].R {
			if o := r.O; o != nil && o.K == "table" {
				if len(o.Cols) == 0 {
					continue
				}
				for k, row := range o.Rows {
					for len(row) < len(o.Cols) {
						row = append(row, "")
					}
					o.Rows[k] = row[:len(o.Cols)]
				}
			}
			rs = append(rs, r)
		}
		d[i].R = mergeRuns(rs)
	}
	return d, nil
}

// EncodeBody writes a Doc as a body: the empty lines that end it are no part of it, and runs of the same marks
// side by side are one. A Doc that cannot be written is a body no page file takes, so that its save fails (#135).
func EncodeBody(d Doc) string {
	out := make(Doc, 0, len(d))
	for _, p := range d {
		q := Para{S: p.S}
		if q.S != nil && *q.S == (ParaStyle{}) {
			q.S = nil
		}
		for _, r := range p.R {
			if r.O != nil {
				if r.O.K == "image" && r.O.Ref == "" { // a picture shown from data the file does not keep (ReadHTML)
					continue
				}
				r = Run{T: ObjectChar, O: r.O} // nor the marks of the text an object is in
				q.R = append(q.R, r)
				continue
			}
			if r.T == "" {
				continue
			}
			if n := len(q.R); n > 0 && q.R[n-1].O == nil && sameMarks(q.R[n-1], r) {
				q.R[n-1].T += r.T
				continue
			}
			q.R = append(q.R, r)
		}
		out = append(out, q)
	}
	for len(out) > 0 && out[len(out)-1].S == nil && len(out[len(out)-1].R) == 0 {
		out = out[:len(out)-1]
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "!" + err.Error()
	}
	return string(b)
}

// sameMarks says whether two runs have the same marks, whatever their text.
func sameMarks(a, b Run) bool {
	a.T, b.T = "", ""
	return a.O == nil && b.O == nil && a == b
}

// BodyReadable says why a body is not one this version can read: a value of the wrong type, or a shape of a later
// version. A field this version does not know, in a paragraph, its style or a run, is a newer version's: the page is
// read but not written, as saving would drop it (#132).
func BodyReadable(s string) error {
	var d Doc
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return errors.New("the page was written by a newer version of Celestine: " + err.Error())
		}
		return errors.New("the page's text is in a form this version cannot read: " + err.Error())
	}
	if _, err := dec.Token(); err != io.EOF { // something after the document, which reading it would drop
		return errors.New("the page's text is in a form this version cannot read: more after its paragraphs")
	}
	return nil
}

// AppendLine puts a paragraph of text at the end of a body, after the others in the JSON as it is: decoding and
// encoding it again would drop what this version does not know (#132). A body that cannot be read is refused before.
func AppendLine(body, line string) string {
	b, _ := json.Marshal(Para{R: []Run{{T: line}}})
	t := strings.TrimSpace(body)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") || strings.TrimSpace(t[1:len(t)-1]) == "" {
		return "[" + string(b) + "]"
	}
	return t[:len(t)-1] + "," + string(b) + "]"
}

// Plain is the document's text alone, a line to a paragraph, its objects left out: what the editor's text of it says
// as plain text, without the editor.
func (d Doc) Plain() string { return strings.ReplaceAll(d.text(), ObjectChar, "") }

// text is the document's text, a line to a paragraph, an object as the one character it stands as.
func (d Doc) text() string {
	var b strings.Builder
	for i, p := range d {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.text())
	}
	return b.String()
}

// Blank reports whether the document holds no text and no object.
func (d Doc) Blank() bool { return strings.TrimSpace(strings.ReplaceAll(d.text(), "\n", "")) == "" }
