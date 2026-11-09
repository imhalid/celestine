package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// newerBody is a page written by a newer version: an object of a kind this one does not know, with fields it does
// not know either.
const newerBody = `[{"r":[{"t":"before "},{"t":"` + string(richtext.ObjectReplacement) + `","o":{"k":"ai-answer","model":"x-1","answer":"hello","at":[1,2]}},{"t":" after"}]}]`

const newerObj = `{"k":"ai-answer","model":"x-1","answer":"hello","at":[1,2]}`

// An object of a kind this version does not know is read as a placeholder and written back as it came.
func TestBodyKeepsUnknownObjects(t *testing.T) {
	text := decodeBody(newerBody)
	if !strings.Contains(text.String(), "before") || !strings.Contains(text.String(), "after") {
		t.Fatalf("read: %q", text.String())
	}
	out := encodeBody(text)
	if !strings.Contains(out, newerObj) {
		t.Fatalf("written back:\n%s\nwant the object\n%s", out, newerObj)
	}
	if md := bodyMarkdown(text); strings.Contains(md, "ai-answer") || !strings.Contains(md, "before") {
		t.Fatalf("Markdown: %q", md)
	}
}

// A page with a block of a newer version, opened and edited elsewhere and saved, still has the block.
func TestEditingKeepsUnknownObjects(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Advance(50 * time.Millisecond)
	p := a.page
	if err := p.Save(newerBody); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(p)
	tt.Frame()
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	a.doc.InsertText(" more")
	tt.Frame()
	a.save()
	body, err := p.Body()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, newerObj) || !strings.Contains(body, "more") {
		t.Fatalf("saved: %s", body)
	}
}
