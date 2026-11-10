package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

// ⌘C and ⌘V inside the app keep what the blocks are: a code block's language, a task's kind, a divider. The
// editor's own copy would lose them; a paste of text copied elsewhere is the editor's as before.
func TestCopyPasteKeepsBlocks(t *testing.T) {
	src := "```go\nfmt.Println(1)\n```\n\n- [!] urgent\n\n---\n\nend\n"
	a, tt := newTestApp(t, src)
	a.clipRead = func() string { return tt.Clipboard() }
	tt.Advance(100 * time.Millisecond)
	n := a.doc.Text().Len()
	a.doc.SetSelection(0, n)
	tt.Key(ui.Cmd, ui.KeyC)
	if a.ownClip.md == "" || tt.Clipboard() == "" {
		t.Fatalf("not noted: %+v, clipboard %q", a.ownClip, tt.Clipboard())
	}
	a.doc.SetSelection(n, n)
	a.doc.InsertText("\n") // a new line to paste on
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	a.save()
	md := a.currentMarkdown(a.page)
	for _, want := range []string{"```go", "- [!] urgent", "---"} {
		if strings.Count(md, want) != 2 {
			t.Fatalf("%q not kept by the paste:\n%s", want, md)
		}
	}
	// copied in another app: the editor pastes it
	tt.SetClipboard("plain words")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	a.save()
	if md := a.currentMarkdown(a.page); !strings.Contains(md, "plain words") || strings.Count(md, "```go") != 2 {
		t.Fatalf("another app's copy: %q", md)
	}
}

// ⌘V of text with Markdown blocks makes the blocks, as a file dropped does: a table, a fenced code block with its
// language, a heading; plain text stays as it is (#307).
func TestPasteMarkdownBlocks(t *testing.T) {
	a, tt := newTestApp(t, "\n")
	tt.Frame()
	md := "## Önemli olan\n\n| Model | Ne çalışır |\n| :--- | :--- |\n| Anlık | Her şey |\n\n```rust\nlet x = 1;\n```\n\nBir paragraf.\n"
	a.clipRead = func() string { return md }
	if !a.clipKey(ui.KeyV, ui.Cmd) {
		t.Fatal("⌘V of Markdown was left to the editor's plain paste")
	}
	tx := a.doc.Text()
	heading, code, table := false, false, false
	for _, p := range tx.Paragraphs() {
		heading = heading || p.Style.Heading == 2
		code = code || p.Style.ID == codeIDPrefix+"rust"
		for _, r := range p.Runs {
			if att := r.Attributes.Attachment; att != nil {
				if ch, ok := att.Data.(*chip); ok && ch.kind == "table" {
					table = true
				}
			}
		}
	}
	if !heading || !code || !table {
		t.Fatalf("heading %v, code with its language %v, table %v:\n%s", heading, code, table, bodyMarkdown(tx))
	}
	a.clipRead = func() string { return "just words, no blocks" }
	if a.clipKey(ui.KeyV, ui.Cmd) {
		t.Fatal("plain text was taken from the editor")
	}
}
