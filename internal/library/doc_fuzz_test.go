package library

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/imhalid/celestine/internal/textnorm"
)

// FuzzBodyRoundTrip: a body this version reads, read and written, is a readable body that a second pass leaves as
// it is, and an object of a newer version is kept (#595, brought from the app's).
func FuzzBodyRoundTrip(f *testing.F) {
	for _, s := range []string{
		`[{"s":{"heading":1},"r":[{"t":"Başlık"}]},{"r":[{"t":"a "},{"t":"b","w":700},{"t":"￼","o":{"k":"page","target":"Dune"}}]}]`,
		`[{"r":[{"t":"￼","o":{"k":"table","cols":[{"name":"A"}],"rows":[["1","2"],[]]}}]}]`,
		`[{"r":[{"t":"￼","o":{"k":"future","what":1}}]},{},{}]`, `[{"s":{"list":3,"checked":true,"id":"celestine:task:star"}}]`,
		`[]`, `[{"r":[{"t":""},{"t":"x","i":true},{"t":"y","i":true}]}]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if BodyReadable(s) != nil {
			return
		}
		d, err := DecodeBody(s)
		if err != nil {
			t.Fatal(err)
		}
		once := EncodeBody(d)
		if !json.Valid([]byte(once)) || BodyReadable(once) != nil {
			t.Fatalf("its own body is not readable: %q", once)
		}
		d2, _ := DecodeBody(once)
		if twice := EncodeBody(d2); twice != once {
			t.Fatalf("a second pass changed it:\n%s\n%s\nfrom %s", once, twice, s)
		}
		if strings.Contains(s, `"future"`) && !strings.Contains(once, `"future"`) {
			t.Fatalf("an object of a newer version was dropped: %s", once)
		}
	})
}

var linkInMD = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)

// keptOf is what a round trip keeps of a page's Markdown: its words with a letter or a digit (its Markdown made plain,
// as the search reads it), its tasks in the running text, and its links.
func keptOf(md string) string {
	var b strings.Builder
	for _, w := range strings.Fields(textnorm.Plain(md)) {
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			b.WriteString(w + " ")
		}
	}
	for _, s := range InlineTaskSpots(md) {
		b.WriteString(map[bool]string{true: "\n[x]", false: "\n[ ]"}[s.Done])
	}
	for _, m := range linkInMD.FindAllStringSubmatch(md, -1) {
		b.WriteString("\nlink " + m[1])
	}
	return b.String()
}

// FuzzMarkdownRoundTrip: a page's Markdown read and written, then read and written again, keeps its words, its tasks
// in the running text and its links (#544, #595, brought from the app's).
func FuzzMarkdownRoundTrip(f *testing.F) {
	for _, s := range []string{
		"# Başlık\n\nmetin [[Dune|D]] ve [[2026-11-26]]\n\n- [*] yıldız\n- [ ] yap\n\nsatır [ ] içi [x] bitti",
		"```go\nx\n```\n---\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```query\n#p\n```", "> [!note] c\n> d\n\n$$\nx\n$$",
		"<!--celestine:list num=roman-->\n1. a\n2. b", "a[^1]\n\n[^1]: n", "![[Kart]]", "**[[A]]** _b_ ~~c~~ `d` <u>e</u>",
		"1. [ ] n\n2. [x] m", "- a\n    - b\n        - c", "<div>h</div>\n\ntext", "[x]: http://u\n\nsee [x]",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if len(md) > 4096 {
			return
		}
		once := ImportMarkdown(md).ExportMarkdown()
		twice := ImportMarkdown(once).ExportMarkdown()
		if a, b := keptOf(once), keptOf(twice); a != b {
			t.Fatalf("read and written again, it changed:\n%s\nfrom\n%s\n(once %q, twice %q, from %q)", b, a, once, twice, md)
		}
	})
}

// FuzzReadMarkdown: any Markdown reads into a body this version reads, through the codec and the page's reading.
func FuzzReadMarkdown(f *testing.F) {
	for _, s := range []string{"# a\n\n- [*] b", "" + "9x", "- -1 a", "\xff\xfe", "<p>x</p><img src=x>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if body := EncodeBody(ImportMarkdown(md)); BodyReadable(body) != nil {
			t.Fatalf("%q made an unreadable body: %s", md, body)
		}
	})
}
