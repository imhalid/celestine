package library

import (
	"strings"
	"testing"

	"github.com/imhalid/celestine/internal/table"
)

// A body is read and written back as it was: its paragraphs' styles, its runs' marks, its objects, and an object of
// a newer version's kind as it came (#595).
func TestBodyCodecRoundTrip(t *testing.T) {
	body := `[{"s":{"heading":1},"r":[{"t":"Başlık"}]},` +
		`{"r":[{"t":"bir "},{"t":"kalın","w":700},{"t":"￼","o":{"k":"page","target":"Dune","alias":"D"}}]},` +
		`{"s":{"list":3,"checked":true,"id":"celestine:task:star"},"r":[{"t":"iş"}]},` +
		`{"r":[{"t":"￼","o":{"k":"table","cols":[{"name":"Ad","kind":"number"}],"rows":[["1"]]}}]},` +
		`{"r":[{"t":"￼","o":{"k":"hologram","beam":42}}]}]`
	d, err := DecodeBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if got := EncodeBody(d); got != body {
		t.Fatalf("written back:\n%s\nnot:\n%s", got, body)
	}
	if o := d[4].R[0].O; o.Known() || !strings.Contains(string(o.Raw), "beam") {
		t.Fatalf("the newer object: %+v", o)
	}
	if d[3].R[0].O.Cols[0] != (table.Col{Name: "Ad", Kind: "number"}) {
		t.Fatalf("the table's column: %+v", d[3].R[0].O.Cols)
	}
}

// Written, runs of the same marks side by side are one, empty ones go, and so do the empty lines that end the
// document; a plain style is no style.
func TestEncodeBodyTidies(t *testing.T) {
	d := Doc{
		{S: &ParaStyle{}, R: []Run{{T: "a"}, {T: "b"}, {T: ""}, {T: "c", W: 700}}},
		{},
		{},
	}
	if got := EncodeBody(d); got != `[{"r":[{"t":"ab"},{"t":"c","w":700}]}]` {
		t.Fatal(got)
	}
}

// A body this version cannot read says why: a field of a newer version's, or another shape; AppendLine adds a line
// without reading the rest, what this version does not know kept.
func TestBodyReadableAndAppend(t *testing.T) {
	if err := BodyReadable(`[{"r":[{"t":"x","glow":1}]}]`); err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Fatalf("a newer field: %v", err)
	}
	if err := BodyReadable(`{"r":1}`); err == nil {
		t.Fatal("another shape read")
	}
	if err := BodyReadable(`[{"r":[{"t":"x"}]}]`); err != nil {
		t.Fatal(err)
	}
	if got := AppendLine(`[{"r":[{"t":"x","glow":1}]}]`, "y"); got != `[{"r":[{"t":"x","glow":1}]},{"r":[{"t":"y"}]}]` {
		t.Fatal(got)
	}
	if got := AppendLine("", "y"); got != `[{"r":[{"t":"y"}]}]` {
		t.Fatal(got)
	}
}

// The library reads and writes a document's Markdown on its own: headings, lists, tasks, quotes, code, links to
// pages and days, embeds and boxes in the text, and the marks of its runs.
func TestDocMarkdown(t *testing.T) {
	md := "# Başlık\nbir [[Dune|D]] ve [[2026-11-26]]\n- madde\n  - alt\n1. bir\n2. iki\n- [ ] yap #iş\n- [x] bitti\n> alıntı\n![[Gömülü]]\nsatır [x] içi\n```go\nfmt.Println()\n```"
	d := DocFromMarkdown(md)
	if got := d.Markdown(); got != md {
		t.Fatalf("written back:\n%s\nnot:\n%s", got, md)
	}
	if d[1].R[1].O.K != "page" || d[1].R[1].O.Alias != "D" || d[1].R[3].O.K != "date" {
		t.Fatalf("links: %+v", d[1].R)
	}
	if st := d[6].S; st.List != ListTasks || st.Checked || !d[7].S.Checked {
		t.Fatalf("tasks: %+v %+v", d[6].S, d[7].S)
	}
	if st := d[3].S; st.List != ListBullets || st.Indent != 1 {
		t.Fatalf("the nested item: %+v", st)
	}
	if d[11].S == nil || !d[11].S.CodeBlock || d[11].S.ID != "celestine:code:go" {
		t.Fatalf("code: %+v", d[11].S)
	}
	marks := Doc{{R: []Run{{T: "kalın", W: 700}, {T: " "}, {T: "eğik", I: true}, {T: " "}, {T: "kod", C: true}, {T: " "}, {T: "site", L: "https://x.y"}}}}
	if got := marks.Markdown(); got != "**kalın** *eğik* `kod` [site](https://x.y)" {
		t.Fatal(got)
	}
}

// Without the app, the library's format is its own document: what it saves is page.json's document, and it reads its
// Markdown, its text and whether it is blank from it; a body that is a JSON string is read as its Markdown.
func TestLibraryFormatIsTheDocument(t *testing.T) {
	f := docFormat{}
	body := f.FromMarkdown("# T\n- [ ] yap [[P]]")
	if !strings.HasPrefix(body, "[") || BodyReadable(body) != nil {
		t.Fatalf("not a document: %s", body)
	}
	if got := f.Markdown(body); got != "# T\n- [ ] yap [[P]]" {
		t.Fatal(got)
	}
	if got := f.Plain(body); got != "T\nyap "+ObjectChar {
		t.Fatalf("%q", got)
	}
	if f.Blank(body) || !f.Blank(`[{"r":[{"t":"  "}]}]`) || !f.Blank("[]") {
		t.Fatal("blank")
	}
	if got := f.Markdown(`"eski biçim"`); got != "eski biçim" {
		t.Fatal(got)
	}
}
