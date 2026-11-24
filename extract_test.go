package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestExtractToPage: the selected blocks go to a new page titled from their heading, under the page, with a link to
// it in their place; one undo brings them back. With no selection the caret's block goes, titled from its line; the
// files the blocks name go with them (#550).
func TestExtractToPage(t *testing.T) {
	a, tt := newTestApp(t, "Intro\n\n# Plan\n\nstep one\n\nstep two\n\nAfter\n")
	tt.SetSize(1100, 700)
	tt.Advance(300 * time.Millisecond)
	p := a.page
	p.Coll.SubPages = true
	text := a.doc.Text().String()
	from := strings.Index(text, "Plan")
	to := strings.Index(text, "step two") + 3
	a.doc.SetSelection(from, to)
	a.extractToPage()
	tt.Frame()
	var np *Page
	for _, q := range p.Children {
		if q.Title == "Plan" {
			np = q
		}
	}
	if np == nil {
		t.Fatalf("no page Plan under %q: %v", p.Title, p.Children)
	}
	body, err := np.Text()
	if err != nil || !strings.Contains(body, "step one") || !strings.Contains(body, "step two") || strings.Contains(body, "Plan") {
		t.Fatalf("the new page's text %q (%v)", body, err)
	}
	md := bodyMarkdown(a.doc.Text())
	if !strings.Contains(md, "[[Plan]]") || strings.Contains(md, "step one") || !strings.Contains(md, "Intro") || !strings.Contains(md, "After") {
		t.Fatalf("the page after: %q", md)
	}
	a.doc.Undo()
	tt.Frame()
	if md := bodyMarkdown(a.doc.Text()); !strings.Contains(md, "# Plan") || !strings.Contains(md, "step two") || strings.Contains(md, "[[Plan]]") {
		t.Fatalf("after undo: %q", md)
	}
	// the caret's block, with no selection
	at := strings.Index(a.doc.Text().String(), "After")
	a.doc.SetSelection(at+2, at+2)
	a.extractToPage()
	tt.Frame()
	found := false
	for _, q := range p.Children {
		found = found || q.Title == "After"
	}
	if md := bodyMarkdown(a.doc.Text()); !found || !strings.Contains(md, "[[After]]") {
		t.Fatalf("the caret's block: found %v, the page %q", found, md)
	}
	// a file the blocks name goes with them
	os.WriteFile(filepath.Join(p.Dir, "notes.pdf"), []byte("%PDF"), 0o600)
	a.carryFiles(p, np, `{"k":"file","ref":"notes.pdf"}`)
	if b, err := os.ReadFile(filepath.Join(np.Dir, "notes.pdf")); err != nil || string(b) != "%PDF" {
		t.Fatalf("the file beside the new page: %q %v", b, err)
	}
}

// TestExtractSplit: a page made from blocks is titled from their first line with words, cut when long; a heading
// there is the title and leaves the body.
func TestExtractSplit(t *testing.T) {
	part := importMarkdown("\n# Heading\n\nbody\n")
	if title, body := extractSplit(part); title != "Heading" || strings.Contains(body.String(), "Heading") || !strings.Contains(body.String(), "body") {
		t.Fatalf("a heading: %q, %q", title, body.String())
	}
	long := strings.Repeat("word ", 30)
	if title, body := extractSplit(importMarkdown(long + "\n\nmore\n")); len([]rune(title)) > extractTitleMax+1 || !strings.HasSuffix(title, "…") || !strings.Contains(body.String(), "word") {
		t.Fatalf("a long line: %q, %q", title, body.String())
	}
}
