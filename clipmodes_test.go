package main

import (
	"image"
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

func TestPasteDrawsLinksAndTables(t *testing.T) {
	md := "A link [[Dune]] here.\n\n| A | B |\n| :--- | :--- |\n| 1 | 2 |\n\n---\n"
	assertDressedAndDrawn := func(name string, paste func(a *app)) {
		t.Helper()
		a, tt := newTestApp(t, "")
		tt.Frame()
		before := *tt.Image()
		before.Pix = append([]byte(nil), before.Pix...)
		paste(a)
		tt.Frame()
		tt.Frame()

		var linkDrawn, tableDrawn bool
		for _, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
			att := r.Attributes.Attachment
			if att == nil {
				continue
			}
			ch, ok := att.Data.(*chip)
			if !ok {
				continue
			}
			if ch.kind == "page" && ch.target == "Dune" {
				if att.Draw == nil || att.Width <= 1 || att.Height <= 1 {
					t.Fatalf("%s: pasted link attachment not dressed: draw=%v size=%vx%v", name, att.Draw != nil, att.Width, att.Height)
				}
				linkDrawn = true
			}
			if ch.kind == "table" {
				if att.Draw == nil || att.Width <= 1 || att.Height <= 1 {
					t.Fatalf("%s: pasted table attachment not dressed: draw=%v size=%vx%v", name, att.Draw != nil, att.Width, att.Height)
				}
				tableDrawn = true
			}
		}
		if !linkDrawn || !tableDrawn {
			t.Fatalf("%s: linkDrawn=%v tableDrawn=%v", name, linkDrawn, tableDrawn)
		}
		after := tt.Image()
		box := image.Rect(int(a.editorR.X), int(a.editorR.Y), int(a.editorR.X+a.editorR.W), int(a.editorR.Y+a.editorR.H))
		if d := pixelsDiffer(&before, after, box, image.Rectangle{}); d < 40 {
			t.Fatalf("%s: expected pasted link and table to draw pixels, got %d", name, d)
		}
	}

	assertDressedAndDrawn("pasteAsMarkdown", func(a *app) {
		a.clipRead = func() string { return md }
		a.pasteAs("markdown")
	})
	assertDressedAndDrawn("pasteOwn", func(a *app) {
		a.ownClip = ownClip{plain: "own", md: md}
		a.clipRead = func() string { return "own" }
		if !a.pasteOwn() {
			t.Fatal("pasteOwn returned false")
		}
	})
	assertDressedAndDrawn("contextMenuPaste", func(a *app) {
		a.clipRead = func() string { return md }
		for _, it := range a.editorItems(0) {
			if it.title == "common_paste" && it.run != nil {
				it.run()
				return
			}
		}
		t.Fatal("common_paste item not found in editorItems")
	})
}
