package main

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
)

// depths are the quote depths of d's paragraphs, which the page file does not keep.
func depths(d library.Doc) string {
	var b strings.Builder
	for _, p := range d {
		if p.S != nil && p.S.QuoteDepth > 1 {
			b.WriteString("q")
			b.WriteByte(byte('0' + p.S.QuoteDepth))
		}
		b.WriteByte(',')
	}
	return b.String()
}

// mdCorpus is Markdown of every kind the readers meet: the hand-written cases, then random mixes of them.
func mdCorpus(n int) []string {
	cases := []string{
		"", "plain", "a\nb", "a\n\nb", "# H1", "###### H6 #", "# closing ##", "Setext\n===", "Setext\n---",
		"- a\n- b\n  - c\n    - d", "1. a\n2. b\n   1. c", "3) x", "- [ ] t\n- [x] d\n  - [ ] sub", "* star\n+ plus",
		"- item\n\n  second para\n\n  ```\n  code\n  ```", "- ```\n  code first\n  ```", "-\n- empty first",
		"> q\n> > qq\n> > > qqq", "> - quoted item\n>   - nested", "> # quoted heading", "> ```\n> code\n> ```",
		"```go\nfunc a() {}\n```", "~~~\ntilde\n~~~", "    indented code", "```\n```", "``` \nx\n```",
		"**b** *i* ~~s~~ `c` [l](http://x.y) <u>u</u>", "***bi***", "**b *bi* b**", "_u_ __b__", "a*b*c", "a**b**c",
		"**foo.**bar", "*a **b** c*", "[link **bold**](u)", "[a](<u v>)", "[a](javascript:alert(1))", "<https://auto.link>",
		"www.example.com", "![alt](pic.png)", "![a **b**](x.png \"t\")", "text ![i](p.png) more", "**bold ![i](p.png) bold**",
		"line  \nbreak", "line\\\nbreak", "soft\nbreak", "<b>b</b><i>i</i><s>s</s><del>d</del><ins>ins</ins><br>x",
		"<div>\nhtml block\n</div>", "<p>para <b>bold</b></p>", "<ul><li>a</li><li>b</li></ul>", "<!-- comment -->",
		"| a | b |\n|---|---|\n| 1 | 2 |", "a | b\n--|--\n1 | 2", "[^1] ref\n\n[^1]: note", "[^a]: note\n    more",
		"\\# not heading", "\\- not item", "1\\. not item", "&amp; &copy; &#38; &#x26;", "\\*\\_\\[\\]", "a\\\\b",
		"`` code ` tick ``", "` `", "`  a  `", "***", "---", "___", "- - -", "Tab\tin", "\ttab start", "  two spaces",
		"emoji 😀 ünïcödé İ ı ğ", "\u00a0nbsp", "a\u0000b", "\xff\xfe bad utf8", "[[wiki link]]", "[[a|b]]", "#tag",
		"[ ] box [x] done", "1. [ ] numbered task", "- [*] star kind", "- [!] imp", "<u>under **bold**</u>",
		"*a*b*c*", "**a**b**c**", "~~a~~b", "a~~b~~c", "trailing   ", "> ", ">", "- ", "1.", "#", "#hashtag start",
		"=== start", "+ plus item", "> > \n> > x", "<details><summary>s</summary>body</details>", "<img src=\"a.png\" alt=\"A\">",
		"<pre>pre\ntext</pre>", "<h2 style=\"text-align:center\">c</h2>", "<span style=\"font-weight:bold\">b</span>",
		"<p align=\"right\">r</p>", "<input type=\"checkbox\" checked> task", "<table><tr><td>a</td><td>b</td></tr></table>",
	}
	out := append([]string(nil), cases...)
	rng := rand.New(rand.NewSource(595))
	for range n {
		k := 1 + rng.Intn(5)
		var parts []string
		for range k {
			parts = append(parts, cases[rng.Intn(len(cases))])
		}
		sep := []string{"\n", "\n\n", " ", ""}[rng.Intn(4)]
		out = append(out, strings.Join(parts, sep))
	}
	return out
}

// The library reads Markdown as the editor's reader did, to the last run: richtext.ParseMarkdown and
// library.ReadMarkdown make the same page file of every text of the corpus, and the same depths of quotes (#595).
func TestMarkdownReaderParity(t *testing.T) {
	bad := 0
	for _, md := range mdCorpus(4000) {
		want := docObjectsOf(richtext.ParseMarkdown(md))
		got := library.ReadMarkdown(md)
		dw, dg := strings.TrimRight(depths(want), ","), strings.TrimRight(depths(got), ",")
		if w, g := library.EncodeBody(want), library.EncodeBody(got); w != g || dw != dg {
			if bad++; bad <= 8 {
				t.Errorf("%q:\nrichtext %s %s\nlibrary  %s %s", md, w, dw, g, dg)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d texts read otherwise", bad)
	}
}

// The library writes a document as the editor's writer did: Text.Markdown and Doc.WriteMarkdown write the same
// Markdown of every text the corpus reads into (#595).
func TestMarkdownWriterParity(t *testing.T) {
	bad := 0
	for _, md := range mdCorpus(4000) {
		text := richtext.ParseMarkdown(md)
		want, got := text.Markdown(), docObjectsOf(text).WriteMarkdown()
		if want != got {
			if bad++; bad <= 8 {
				t.Errorf("%q:\nrichtext %q\nlibrary  %q", md, want, got)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d texts written otherwise", bad)
	}
}

// FuzzMarkdownParity holds the library's reader and writer to the editor's on what the fuzzer makes of the corpus.
func FuzzMarkdownParity(f *testing.F) {
	for _, md := range mdCorpus(40) {
		f.Add(md)
	}
	f.Fuzz(func(t *testing.T, md string) {
		text := richtext.ParseMarkdown(md)
		want, got := docObjectsOf(text), library.ReadMarkdown(md)
		if w, g := library.EncodeBody(want), library.EncodeBody(got); w != g {
			t.Fatalf("%q read:\nrichtext %s\nlibrary  %s", md, w, g)
		}
		if w, g := strings.TrimRight(depths(want), ","), strings.TrimRight(depths(got), ","); w != g {
			t.Fatalf("%q quote depths: richtext %s, library %s", md, w, g)
		}
		if w, g := text.Markdown(), want.WriteMarkdown(); w != g {
			t.Fatalf("%q written:\nrichtext %q\nlibrary  %q", md, w, g)
		}
	})
}
