package main

import (
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
	if err := tt.Click("Copy the code"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if want := "func main() {\n\tprintln(\"hi\")\n}"; copied != want {
		t.Fatalf("copied %q, want %q", copied, want)
	}
}
