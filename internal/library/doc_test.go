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

// The library reads and writes a page's Markdown on its own, as the editor did (#595): its objects (links to pages and
// days, a card, a footnote, a task in the running text, a table, a query), its tasks of a kind, its held blocks, and the
// marks of its runs; what it writes it reads back the same.
func TestPageMarkdownAlone(t *testing.T) {
	md := "# Başlık\n\nbir [[Dune|D]] ve [[2026-11-26]] not[^1]\n\n- [*] yıldız\n- [ ] yap #iş\n- [x] bitti\n\nsatır [ ] içi\n\n" +
		"![[Gömülü]]\n\n```go\nfmt.Println()\n```\n\n```query\n#proje\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[^1]: not"
	d := ImportMarkdown(md)
	kinds := map[string]int{}
	for _, p := range d {
		for _, r := range p.R {
			if r.O != nil {
				kinds[r.O.K]++
			}
		}
	}
	for _, k := range []string{"page", "date", "footnote", "check", "embed", "query", "table"} {
		if kinds[k] != 1 {
			t.Errorf("%s objects: %d, want 1 (%v)", k, kinds[k], kinds)
		}
	}
	if st := d[2].S; st == nil || st.ID != TaskIDPrefix+"star" {
		t.Fatalf("the star task: %+v", st)
	}
	out := d.ExportMarkdown()
	if again := ImportMarkdown(out).ExportMarkdown(); again != out {
		t.Fatalf("written:\n%s\nread and written again:\n%s", out, again)
	}
	for _, want := range []string{"[[Dune|D]]", "[[2026-11-26]]", "- [*] yıldız", "satır [ ] içi", "![[Gömülü]]", "```go\nfmt.Println()\n```", "```query\n#proje\n```", "```celestine-table\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q not written in:\n%s", want, out)
		}
	}
	marks := Doc{{R: []Run{{T: "kalın", W: 700}, {T: " "}, {T: "eğik", I: true}, {T: " "}, {T: "kod", C: true}, {T: " "}, {T: "site", L: "https://x.y"}}}}
	if got := marks.PageMarkdown(); got != "**kalın** *eğik* `kod` [site](https://x.y)" {
		t.Fatal(got)
	}
}

// A query block right after the lines of another block of code takes its own lines, not those (#608).
func TestQueryAfterCodeKeepsTheCode(t *testing.T) {
	d := ImportMarkdown("<img src=\"a.png\">\nbir satır\nikinci satır\n\n```query\n#proje @todo\n```\n")
	var q string
	var code []string
	for _, p := range d {
		for _, r := range p.R {
			if r.O != nil && r.O.K == "query" {
				q = r.O.Q
			}
		}
		if p.S != nil && p.S.CodeBlock {
			code = append(code, p.text())
		}
	}
	if q != "#proje @todo" || strings.Join(code, "|") != "<img src=\"a.png\">|bir satır|ikinci satır" {
		t.Fatalf("query %q, code %q", q, code)
	}
}

// Without the app, the library's format is its own document: what it saves is page.json's document, and it reads its
// Markdown, its text and whether it is blank from it; a body that is a JSON string is read as its Markdown.
func TestLibraryFormatIsTheDocument(t *testing.T) {
	f := docFormat{}
	body := f.FromMarkdown("# T\n\n- [ ] yap [[P]]")
	if !strings.HasPrefix(body, "[") || BodyReadable(body) != nil {
		t.Fatalf("not a document: %s", body)
	}
	if got := f.Markdown(body); got != "# T\n\n- [ ] yap [[P]]" {
		t.Fatal(got)
	}
	if got := f.Plain(body); got != "T\nyap " {
		t.Fatalf("%q", got)
	}
	if f.Blank(body) || !f.Blank(`[{"r":[{"t":"  "}]}]`) || !f.Blank("[]") {
		t.Fatal("blank")
	}
	if got := f.Markdown(`"eski biçim"`); got != "eski biçim" {
		t.Fatal(got)
	}
}

// A task in the running text can end before its line does: its end is an object of the page, written as a comment no
// other reader shows right after its words, read back from it, and kept as text in code (#610).
func TestInlineTaskEnd(t *testing.T) {
	md := "Buy [ ] milk<!--/task--> and more, [x] call Ana<!--/task--> too\n\n`code <!--/task-->`"
	d := ImportMarkdown(md)
	var kinds []string
	for _, r := range d[0].R {
		if r.O != nil {
			kinds = append(kinds, r.O.K)
		}
	}
	if strings.Join(kinds, ",") != "check,checkend,check,checkend" {
		t.Fatalf("objects: %v", kinds)
	}
	if strings.Contains(d[1].text(), "") || !strings.Contains(d[1].text(), "<!--/task-->") {
		t.Fatalf("in code: %q", d[1].text())
	}
	if got := d.ExportMarkdown(); got != md {
		t.Fatalf("written:\n%s\nnot:\n%s", got, md)
	}
	if body := EncodeBody(d); !strings.Contains(body, `"k":"checkend"`) || BodyReadable(body) != nil {
		t.Fatalf("body: %s", body)
	}
	if p, _, _ := strings.Cut(Format.Plain(EncodeBody(d)), "\n"); strings.Contains(p, "task-->") {
		t.Fatalf("plain: %q", p)
	}
}
