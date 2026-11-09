package main

import (
	"strings"
	"testing"
	"time"
)

// Copy as Markdown, plain text and HTML take the selection, or the whole page with none; paste as Markdown makes
// blocks of what the clipboard has, paste as plain text puts it word for word.
func TestCopyAndPasteAs(t *testing.T) {
	a, tt := newTestApp(t, "# Heading\n\nSome **bold** words.\n")
	copied := ""
	a.clipText = func(s string) { copied = s }
	tt.Advance(100 * time.Millisecond)
	a.copyAs("markdown")
	if copied != "# Heading\n\nSome **bold** words." {
		t.Fatalf("markdown: %q", copied)
	}
	a.copyAs("plain")
	if copied != "Heading\nSome bold words." {
		t.Fatalf("plain: %q", copied)
	}
	a.copyAs("html")
	if !strings.Contains(copied, "<h1>Heading</h1>") || !strings.Contains(copied, "<b>bold</b>") && !strings.Contains(copied, "<strong>bold</strong>") {
		t.Fatalf("html: %q", copied)
	}
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	a.doc.InsertText("\n") // a new line to paste on
	clip := "- one\n- two"
	a.clipRead = func() string { return clip }
	a.pasteAs("markdown")
	tt.Frame()
	a.save()
	if md := a.currentMarkdown(a.page); !strings.Contains(md, "- one\n- two") {
		t.Fatalf("pasted as Markdown: %q", md)
	}
	n = a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	a.doc.InsertText("\n")
	clip = "# not a heading"
	a.pasteAs("plain")
	tt.Frame()
	if md := a.currentMarkdown(a.page); !strings.Contains(md, "\\# not a heading") {
		t.Fatalf("pasted as plain text: %q", md)
	}
}
