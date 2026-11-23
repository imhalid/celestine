package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// The body's round trip, body → editor's text → body, and what it gives up on the way (#132). One pass may change a
// body only as this table says; a second pass changes nothing.
//
//	what goes in                                   what comes out
//	empty paragraphs at the end                    dropped: no part of the text
//	"w": 400 (the normal weight)                   dropped
//	a paragraph style of all zero values           dropped ("s" left out)
//	a run with no text and no object               dropped
//	an object of a kind this version knows         its fields as this version has them
//	an object of a kind it does not know           kept as it came
//	a field it does not know, outside an object    the page is not written (bodyReadable)
var bodySeeds = []string{
	`[]`,
	`[{"r":[{"t":"bir"}]}]`,
	`[{"s":{"heading":1},"r":[{"t":"Başlık"}]},{"r":[{"t":"metin","i":true,"w":700}]},{},{}]`,
	`[{"s":{"list":3,"checked":true,"id":"celestine:task:star"},"r":[{"t":"iş"}]}]`,
	`[{"r":[{"t":"￼","o":{"k":"page","target":"Öteki"}}]}]`,
	`[{"r":[{"t":"￼","o":{"k":"future","what":[1,2]}}]}]`,
	`[{"s":{"codeBlock":true,"id":"celestine:code:go"},"r":[{"t":"x := 1"}]},{"s":{"codeBlock":true},"r":[{"t":"y"}]}]`,
	`[{"r":[{"t":"a","w":400},{"t":""},{"t":"b","l":"https://x.y","u":true,"x":true,"c":true,"g":"m"}]}]`,
	`[{"r":[{"t":"Buy "},{"t":"￼","o":{"k":"check"}},{"t":" milk "},{"t":"￼","o":{"k":"check","target":"x"}}]}]`,
	`[{"s":{"list":3,"id":"celestine:list:num=roman\u0026task=number"},"r":[{"t":"a"}]},{"s":{"list":3,"checked":true},"r":[{"t":"b"}]}]`,
	`[{"s":{"list":1,"id":"celestine:list:mark=%E2%AD%90"},"r":[{"t":"a"}]},{"s":{"list":1,"indent":1,"id":"celestine:list:mark=icon%3Aarrow"},"r":[{"t":"b"}]}]`,
	`[{"r":[{"t":"a note"},{"t":"￼","o":{"k":"footnote","target":"1"}}]},{"s":{"codeBlock":true,"id":"celestine:raw:footnote"},"r":[{"t":"[^1]: the note"}]}]`,
	`[{"s":{"list":3},"r":[{"t":"parent"}]},{"s":{"list":3,"indent":1,"checked":true},"r":[{"t":"child"}]},{"s":{"list":3,"indent":2},"r":[{"t":"grandchild [[2026-03-04]]"}]}]`,
}

// markdownSeeds are pages of Markdown with what the round trip has to keep: tasks of every kind, in quotes, in
// the running text and in HTML a raw block holds, footnotes, and links.
var markdownSeeds = []string{
	"# a\n\n- [*] b\n\n```go\nx\n```\n---\n",
	"- [ ] open\n- [x] done\n- [!] important\n- [?] question\n- [/] half\n- [e] event\n",
	"> - [ ] a quoted task\n> - [x] and a done one\n",
	"<div>\n- [ ] in HTML\n</div>\n\n- [ ] after it\n",
	"a note[^1] and [[Page|its name]]\n\n[^1]: the note, with [[Other]]\n",
	"Buy [ ] milk and [x] bread, call [[Ana]] [ ] today.\n\n- [ ] a line [ ] with one\n",
	"1. [ ] numbered\n2. [x] tasks\n   - [ ] nested [[2026-03-04]] every:week\n",
	"| a | [ ] b |\n|---|---|\n| [x] c | d |\n\n> [!note] a callout [ ] held\n",
}

func FuzzBodyRoundTrip(f *testing.F) {
	for _, s := range bodySeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if bodyReadable(s) != nil {
			return
		}
		once := encodeBody(decodeBody(s))
		if !json.Valid([]byte(once)) {
			t.Fatalf("not JSON: %q from %q", once, s)
		}
		if bodyReadable(once) != nil {
			t.Fatalf("its own body is not readable: %q", once)
		}
		if twice := encodeBody(decodeBody(once)); twice != once {
			t.Fatalf("a second pass changed it:\n%s\n%s\nfrom %s", once, twice, s)
		}
		if strings.Contains(s, `"future"`) && !strings.Contains(once, `"future"`) {
			t.Fatalf("an object of a newer version was dropped: %s", once)
		}
	})
}

// A field this version does not know makes the page one it reads and does not write.
func TestBodyUnknownField(t *testing.T) {
	for _, s := range []string{
		`[{"s":{"heading":1,"future":true},"r":[{"t":"x"}]}]`,
		`[{"r":[{"t":"x","color":"red"}]}]`,
		`[{"r":[{"t":"x"}],"later":1}]`,
	} {
		if bodyReadable(s) == nil {
			t.Errorf("%s is taken as this version's", s)
		}
	}
	// a line put at the end keeps what is there as it is
	body := `[{"r":[{"t":"￼","o":{"k":"future","what":1}}]}]`
	if got := appendLine(body, "#etiket"); !strings.HasPrefix(got, body[:len(body)-1]+",") || !strings.Contains(got, "#etiket") {
		t.Fatalf("appendLine: %s", got)
	}
}

// The round trip of a page's Markdown, read and written again: once written by this version, a page's Markdown
// read and written once more keeps its words, its tasks (the lines' and those in the running text, their states,
// kinds and words) and its links (#544). It would have caught #529 and #536.
func FuzzMarkdownRoundTrip(f *testing.F) {
	for _, s := range markdownSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if len(md) > 4096 {
			return
		}
		once := bodyMarkdown(parseBody(md))
		twice := bodyMarkdown(parseBody(once))
		if a, b := roundTripKept(once), roundTripKept(twice); a != b {
			t.Fatalf("read and written again, it changed:\n%s\nfrom\n%s\n(once %q, twice %q, from %q)", b, a, once, twice, md)
		}
	})
}

// roundTripKept is what the round trip keeps of a page's Markdown md: its words, with a letter or a digit (a page of
// marks alone, "> * > *", may be nested otherwise by richtext the second time), its tasks and its links. A task with
// no words is not kept, and leaves its box as text: richtext writes it "- [x] ", which GFM, and richtext's reader,
// read as no task (lostBox).
func roundTripKept(md string) string {
	var b strings.Builder
	for _, w := range strings.Fields(plainText(md)) {
		if lostBox.MatchString(w) {
			continue
		}
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			b.WriteString(w + " ")
		}
	}
	for _, k := range pageTasks(nil, md) {
		fmt.Fprintf(&b, "\ntask %d %v %q %s", k.inl, k.done, k.text, k.kind)
	}
	for _, m := range wikiInText.FindAllStringSubmatch(md, -1) {
		fmt.Fprintf(&b, "\nlink %s", m[1])
	}
	return b.String()
}

// lostBox is the box of a task with no words, as the round trip leaves it: text, escaped or not.
var lostBox = regexp.MustCompile(`^\\?\[[xX ]\\?\]$`)

// The bodies of the seeds keep their words, tasks and links through Markdown and back (#544).
func TestBodyMarkdownKeeps(t *testing.T) {
	for _, s := range bodySeeds {
		if bodyReadable(s) != nil {
			continue
		}
		once := bodyMarkdown(decodeBody(s))
		twice := bodyMarkdown(parseBody(once))
		if a, b := roundTripKept(once), roundTripKept(twice); a != b {
			t.Errorf("%s:\n%s\nbecame\n%s", s, a, b)
		}
	}
}

func FuzzParseBody(f *testing.F) {
	for _, s := range []string{"# a\n\n- [*] b\n\n```go\nx\n```\n---\n", "[[_x]] [^1] <u>u</u> $$x$$\r\n", "| a |\n|---|\n| b |",
		"Buy [ ] milk [x] \\[ \\] `[x]` [x](u) [[a [x]]]\n\n1. [ ] a\n2. [x] b",
		"- a\n    <!--celestine:list mark=%E2%AD%90-->\n    - b\n> <!--celestine:list num=roman-->\n> 1. c\n<!--celestine:list -->\n",
		"\uE002\uE004" + "9\uE005\uE006x\uE005", "- \uE004-1\uE005 a"} {
		f.Add(s)
	}
	for _, s := range markdownSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if len(md) > 4096 {
			return
		}
		tx := parseBody(md)
		body := encodeBody(tx)
		if !json.Valid([]byte(body)) {
			t.Fatalf("not JSON from %q", md)
		}
		bodyMarkdown(tx) // and written back, its tasks and lists' looks too (listmd.go), without a panic
	})
}
