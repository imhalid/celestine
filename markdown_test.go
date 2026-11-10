package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

// TestMarkdownKept: what other tools write comes back as it was, after
// ryudit reads and writes it - the promise of plain files.
func TestMarkdownKept(t *testing.T) {
	for _, md := range []string{
		"Merhaba! Nasılsın?",
		"Bir [[Sayfa bağlantısı]] ve [[Başka#Başlık|takma ad]] burada.",
		"Dipnot[^1] ve bir tane daha[^not].\n\n[^1]: Birinci not.\n[^not]: İkinci.",
		"| Ad | Yıl |\n|:---|---:|\n| Dune | 1965 |\n| Anna | 1878 |",
		"<div align=\"center\">\n  <img src=\"logo.png\">\n</div>",
		"Satır <kbd>⌘</kbd> + <kbd>K</kbd> ile.",
		"$$\nx^2 + y^2 = z^2\n$$",
		"> [!note] Başlık\n> İçerik satırı.",
		"```go\nfmt.Println(\"merhaba!\")\n```",
		"~~~python\nprint(1)\n~~~",
		"````md\n```\niç içe\n```\n````",
		"a_b_c ve snake_case_ad",
		"#etiket ile başlayan satır",
		"\\# bu bir başlık değil",
		"Fiyat: 5$ ve %50!",
		"![resim](a.png) ve ![](b.jpg)",
		"Önce\n\n---\n\nSonra",
		"`kod içinde \\! ve \\_ kalsın`",
	} {
		got := strings.TrimSpace(bodyMarkdown(parseBody(md + "\n")))
		want := strings.TrimSpace(md)
		// a fence of tildes comes back as backticks: the same block
		if strings.HasPrefix(want, "~~~") {
			want = "```" + strings.TrimPrefix(strings.TrimSuffix(want, "~~~"), "~~~") + "```"
		}
		if got != want {
			t.Errorf("\n in: %q\nout: %q", want, got)
		}
	}
}

// TestMarkdownEscapesKept: escapes that are needed stay (written the way
// richtext writes them, which reads the same).
func TestMarkdownEscapesKept(t *testing.T) {
	got := strings.TrimSpace(bodyMarkdown(parseBody("Kaçış: \\*yıldız değil\\* ve \\![resim değil]\n")))
	if got != "Kaçış: \\*yıldız değil\\* ve !\\[resim değil\\]" {
		t.Fatalf("%q", got)
	}
}

// TestMarkdownShown: what is kept shows in the editor: a raw block as
// paragraphs of code, named; a code block with its language.
func TestMarkdownShown(t *testing.T) {
	text := parseBody("Önce\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\nx := 1\n```\n\nSonra\n")
	st := styles(text)
	if len(st) != 6 || !st[1].CodeBlock || st[1].ID != rawIDPrefix+"table" || !st[3].CodeBlock || st[3].ID != "" || st[4].ID != codeIDPrefix+"go" {
		t.Fatalf("%q\n%+v", text.String(), st)
	}
	if !strings.Contains(text.String(), "| 1 | 2 |") {
		t.Fatal("the table's text is not there")
	}
}

// TestRawBlockEdited: a raw block edited in the editor is written as edited.
func TestRawBlockEdited(t *testing.T) {
	a, tt := newTestApp(t, "> [!note] Not\n> Metin\n")
	tt.Type(" daha") // at the end of its last line
	tt.Frame()
	a.save()
	if body, _ := a.page.Text(); body != "> [!note] Not\n> Metin daha\n" {
		t.Fatalf("written: %q", body)
	}
}

// Underline, the tags richtext writes for itself, and a "<" that was typed come back as the text they were, not
// as a raw HTML block; HTML the editor does not read is still kept as it is (#124).
func TestOwnTagsNotRawHTML(t *testing.T) {
	under := richtext.Attributes{Underline: true}
	bold := richtext.Attributes{Weight: 700}
	for _, tx := range []richtext.Text{
		richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "bir "}, {Text: "altı çizili", Attributes: under}, {Text: " kelime"}}}),
		richtext.TextOf(
			richtext.Paragraph{Style: richtext.ParagraphStyle{List: richtext.Tasks}, Runs: []richtext.Run{{Text: "bir "}, {Text: "iş", Attributes: under}}},
			richtext.Paragraph{Style: richtext.ParagraphStyle{List: richtext.Tasks}, Runs: []richtext.Run{{Text: "iki"}}},
		),
		richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "Fiyat:", Attributes: bold}, {Text: "100"}}}),
		richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "a <b> c"}}}),
	} {
		md := bodyMarkdown(tx)
		back := parseBody(md)
		if got, want := encodeBody(back), encodeBody(tx); got != want {
			t.Errorf("%q came back as %s, want %s", md, got, want)
		}
	}
	if kind, _ := rawBlock([]string{`<div class="x">kutu</div>`}, 0, true); kind != "html" {
		t.Errorf("a div is %q", kind)
	}
}
