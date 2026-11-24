package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSnippets: an abbreviation typed as a word and ended by a space is put out as its text, which one undo takes
// back to what was typed; a longer word that begins with it is left; a snippet of lines is Markdown, with the caret
// where its {{cursor}} is (#554).
func TestSnippets(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.settings.Snippets = []Snippet{{Abbr: "brb", Text: "be right back"}, {Abbr: ";mtg", Text: "## Agenda\n\n- {{cursor}}\n"}}
	tt.Advance(300 * time.Millisecond)
	end := a.doc.Text().Len()
	a.doc.SetSelection(end, end)
	a.refocus = true
	tt.Frame()
	tt.Type(" brbx brb")
	tt.Type(" ")
	tt.Frame()
	if got := a.doc.Text().String(); !strings.Contains(got, "brbx be right back ") {
		t.Fatalf("after the space: %q", got)
	}
	a.doc.Undo()
	tt.Frame()
	if got := a.doc.Text().String(); !strings.HasSuffix(got, "brbx brb ") {
		t.Fatalf("after an undo: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Type(";mtg")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	md := bodyMarkdown(a.doc.Text())
	if !strings.Contains(md, "## Agenda") || strings.Contains(md, ";mtg") || strings.Contains(a.doc.Text().String(), cursorMark) {
		t.Fatalf("the snippet of lines: %q", md)
	}
	_, caret := a.doc.Selection()
	if ps, _ := a.doc.Text().ParagraphRange(caret); a.doc.Text().ParagraphStyleAt(ps).List == 0 {
		t.Fatalf("the caret is not in the list item: %d in %q", caret, a.doc.Text().String())
	}
}
