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

// A block of a newer version goes through Markdown as a mark no reader shows, and comes back as it was.
func TestUnknownBlockThroughMarkdown(t *testing.T) {
	md := bodyMarkdown(decodeBody(newerBody))
	if !strings.Contains(md, blockMarkStart) || strings.Contains(md, "ai-answer") {
		t.Fatalf("Markdown: %q", md)
	}
	back := encodeBody(importMarkdown(md))
	if !strings.Contains(back, newerObj) || !strings.Contains(back, "before") || !strings.Contains(back, "after") {
		t.Fatalf("read back:\n%s", back)
	}
}

// An agent that reads a page and writes it back whole, changed, keeps a block of a newer version (#65).
func TestAgentRewriteKeepsUnknownBlock(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Advance(50 * time.Millisecond)
	if err := a.page.Save(newerBody); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(a.page)
	cs := agent(t, a)
	id := a.idOf(a.page.Dir)
	var read struct{ Markdown string }
	call(t, cs, "read_page", map[string]any{"page": id}, &read)
	call(t, cs, "update_page", map[string]any{"page": id, "markdown": strings.Replace(read.Markdown, "after", "after, changed", 1)}, nil)
	body, _ := a.page.Body()
	if !strings.Contains(body, newerObj) || !strings.Contains(body, "changed") {
		t.Fatalf("rewritten: %s", body)
	}
}
