package main

import (
	"github.com/imhalid/celestine/locales"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func styles(t richtext.Text) []richtext.ParagraphStyle {
	var out []richtext.ParagraphStyle
	for _, p := range t.Paragraphs() {
		out = append(out, p.Style)
	}
	return out
}

// TestDividerOnDisk: "---" between paragraphs is a divider, written back as
// it was; under a line of text it underlines a heading, in code it is code.
func TestDividerOnDisk(t *testing.T) {
	md := "Bir\n\n---\n\nİki\n"
	text := parseBody(md)
	st := styles(text)
	if len(st) != 3 || st[1].ID != hrID || text.String() != "Bir\n\nİki" {
		t.Fatalf("parsed: %q %+v", text.String(), st)
	}
	if got := bodyMarkdown(text); strings.TrimSpace(got) != strings.TrimSpace(md) {
		t.Fatalf("written: %q", got)
	}
	if st := styles(parseBody("Başlık\n---\n")); len(st) != 1 || st[0].Heading != 2 {
		t.Fatalf("a heading underlined: %+v", st)
	}
	code := parseBody("```\na\n---\nb\n```\n")
	if !strings.Contains(code.String(), "---") {
		t.Fatalf("in code: %q", code.String())
	}
	if st := styles(parseBody("***\n")); len(st) == 0 || st[0].ID != hrID {
		t.Fatalf("***: %+v", st)
	}
}

func typeEach(tt *ui.Tester, s string) {
	for _, r := range s {
		tt.Type(string(r))
		tt.Frame()
	}
}

// TestDividerShortcut types "---": a divider, and the caret goes on below
// it; what is typed next is under it, and on disk. Undo gives the dashes back.
func TestDividerShortcut(t *testing.T) {
	a, tt := newTestApp(t, "Üst\n")
	tt.Key(ui.Cmd, ui.KeyDown) // the end of the text
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "---")
	typeEach(tt, "alt")
	text := a.doc.Text()
	st := styles(text)
	if len(st) != 3 || st[1].ID != hrID || st[2].ID != "" || text.String() != "Üst\n\nalt" {
		t.Fatalf("after ---: %q %+v", text.String(), st)
	}
	a.save()
	if body, _ := a.page.Text(); body != "Üst\n\n---\n\nalt\n" {
		t.Fatalf("on disk: %q", body)
	}
	// Typing into a divider makes it a paragraph again.
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	typeEach(tt, "x")
	if st := styles(a.doc.Text()); st[1].ID == hrID {
		t.Fatal("still a divider with text in it")
	}
}

// TestCodeShortcut types "```": a block of code at once.
func TestCodeShortcut(t *testing.T) {
	a, tt := newTestApp(t, "")
	typeEach(tt, "```")
	typeEach(tt, "x := 1")
	text := a.doc.Text()
	if st := text.ParagraphStyleAt(0); !st.CodeBlock || text.String() != "x := 1" {
		t.Fatalf("code: %q %+v", text.String(), st)
	}
	a.save()
	if body, _ := a.page.Text(); !strings.Contains(body, "```\nx := 1\n```") {
		t.Fatalf("on disk: %q", body)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got := a.doc.Text().String(); got != "```" {
		t.Fatalf("undo: %q", got)
	}
}

// A block of code has a copy button at its corner, which puts its lines on the clipboard as they are.
func TestCodeBlockCopy(t *testing.T) {
	a, tt := newTestApp(t, "Before.\n\n```go\nfunc main() {\n\tprintln(\"hi\")\n}\n```\n\nAfter.\n")
	copied := ""
	a.clipText = func(s string) { copied = s }
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click(locales.BlocksCopyCode()); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if want := "func main() {\n\tprintln(\"hi\")\n}"; copied != want {
		t.Fatalf("copied %q, want %q", copied, want)
	}
}

// TestDividerTwoKeysInAFrame types "---" with the last two dashes in one frame, as a slow frame or a key held
// brings them: still a divider (#210).
func TestDividerTwoKeysInAFrame(t *testing.T) {
	a, tt := newTestApp(t, "Üst\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "-")
	tt.Type("--") // the tester builds a frame after each event: two letters in one are two in a frame
	tt.Frame()
	if st := styles(a.doc.Text()); len(st) < 2 || st[1].ID != hrID {
		t.Fatalf("not a divider: %q %+v", a.doc.Text().String(), st)
	}
}

// TestUndoNotTypedAgain: an undo that puts back text that typing turns into something is left as it is, and the
// redo is kept: links pasted as chips, ⌘Z gives the text, ⇧⌘Z the chips; a / taken back is no menu (#210).
func TestUndoNotTypedAgain(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	_, caret := a.doc.Selection()
	paste := " [[Bir]] ve [[İki]]"
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(caret, caret, richtext.NewText(paste))
		ed.Select(caret+len([]rune(paste)), caret+len([]rune(paste)))
	})
	tt.Frame()
	tt.Frame()
	if got := chipTargets(a); len(got) != 2 {
		t.Fatalf("no chips to start with: %v", got)
	}
	a.doc.Undo()
	tt.Frame()
	tt.Frame()
	if got := chipTargets(a); len(got) != 0 || !strings.Contains(a.doc.Text().String(), "[[İki]]") || !a.doc.CanRedo() {
		t.Fatalf("the undo was made chips again: %v in %q, redo %v", got, a.doc.Text().String(), a.doc.CanRedo())
	}
	a.doc.Redo()
	tt.Frame()
	if got := chipTargets(a); len(got) != 2 {
		t.Fatalf("the redo: %v in %q", got, a.doc.Text().String())
	}

	b, tb := newTestApp(t, "Önce\n")
	tb.Key(ui.Cmd, ui.KeyDown)
	tb.Key(0, ui.KeyEnter)
	tb.Frame()
	typeEach(tb, "/")
	if !b.slash.open {
		t.Fatal("/ opened no menu")
	}
	tb.Key(0, ui.KeyBackspace)
	tb.Frame()
	b.doc.Undo()
	tb.Frame()
	if b.slash.open {
		t.Fatalf("the / an undo put back opened the menu: %q", b.doc.Text().String())
	}
}
