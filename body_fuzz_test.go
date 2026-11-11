package main

import (
	"encoding/json"
	"strings"
	"testing"
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

func FuzzParseBody(f *testing.F) {
	for _, s := range []string{"# a\n\n- [*] b\n\n```go\nx\n```\n---\n", "[[_x]] [^1] <u>u</u> $$x$$\r\n", "| a |\n|---|\n| b |"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if len(md) > 4096 {
			return
		}
		body := encodeBody(parseBody(md))
		if !json.Valid([]byte(body)) {
			t.Fatalf("not JSON from %q", md)
		}
	})
}
