package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// typeAll types s a letter at a time, as a person does, with Enter as "\n".
func typeAll(d *Doc, s string) {
	for _, r := range s {
		if r == '\n' {
			d.enter(false)
			continue
		}
		d.typeText(string(r))
	}
}

func kinds(d *Doc) []Kind {
	var out []Kind
	for _, b := range d.Blocks {
		out = append(out, b.Kind)
	}
	return out
}

func TestBlockRulesHideMarks(t *testing.T) {
	cases := []struct {
		typed string
		kind  Kind
		text  string
	}{
		{"# Başlık", Heading1, "Başlık"},
		{"## İki", Heading2, "İki"},
		{"### Üç", Heading3, "Üç"},
		{"- madde", Bullet, "madde"},
		{"* madde", Bullet, "madde"},
		{"1. bir", Numbered, "bir"},
		{"[] iş", Todo, "iş"},
		{"> söz", Quote, "söz"},
	}
	for _, c := range cases {
		d := newDoc()
		typeAll(d, c.typed)
		b := d.Blocks[0]
		if b.Kind != c.kind || b.Text != c.text {
			t.Errorf("%q: got kind %d text %q, want %d %q", c.typed, b.Kind, b.Text, c.kind, c.text)
		}
	}
	d := newDoc()
	typeAll(d, "[x] bitti")
	if b := d.Blocks[0]; b.Kind != Todo || !b.Checked || b.Text != "bitti" {
		t.Errorf("[x]: %+v", b)
	}
	d = newDoc()
	typeAll(d, "---")
	if k := kinds(d); len(k) != 2 || k[0] != Divider || k[1] != Paragraph || d.caret().B != 1 {
		t.Errorf("---: %v caret %v", k, d.caret())
	}
	d = newDoc()
	typeAll(d, "```fmt.Println()")
	if b := d.Blocks[0]; b.Kind != Code || b.Text != "fmt.Println()" {
		t.Errorf("```: %+v", b)
	}
	d = newDoc()
	typeAll(d, "a # b") // not at the start: stays text
	if b := d.Blocks[0]; b.Kind != Paragraph || b.Text != "a # b" {
		t.Errorf("mid-line #: %+v", b)
	}
}

func TestInlineRulesHideMarks(t *testing.T) {
	cases := []struct {
		typed, text string
		kind        MarkKind
		s, e        int
	}{
		{"bir **kalın** söz", "bir kalın söz", Bold, 4, 9},
		{"bir __kalın__ söz", "bir kalın söz", Bold, 4, 9},
		{"bir *eğik* söz", "bir eğik söz", Italic, 4, 8},
		{"bir _eğik_ söz", "bir eğik söz", Italic, 4, 8},
		{"bir `kod` söz", "bir kod söz", InlineCode, 4, 7},
		{"bir ~~çizik~~ söz", "bir çizik söz", Strike, 4, 9},
	}
	for _, c := range cases {
		d := newDoc()
		typeAll(d, c.typed)
		b := d.Blocks[0]
		if b.Text != c.text {
			t.Errorf("%q: text %q, want %q", c.typed, b.Text, c.text)
			continue
		}
		if len(b.Marks) != 1 || b.Marks[0] != (Mark{c.kind, c.s, c.e}) {
			t.Errorf("%q: marks %+v", c.typed, b.Marks)
		}
	}
	// What is not emphasis stays as typed.
	for _, s := range []string{"2 * 3 * 4", "snake_case_name", "a ** b", "fiyat *"} {
		d := newDoc()
		typeAll(d, s)
		if b := d.Blocks[0]; b.Text != s || len(b.Marks) != 0 {
			t.Errorf("%q became %q %+v", s, b.Text, b.Marks)
		}
	}
	// After a closing mark the next letters are plain.
	d := newDoc()
	typeAll(d, "**a** b")
	if b := d.Blocks[0]; b.Text != "a b" || len(b.Marks) != 1 || b.Marks[0].E != 1 {
		t.Errorf("after bold: %q %+v", b.Text, b.Marks)
	}
}

func TestUndoGivesTheMarksBack(t *testing.T) {
	d := newDoc()
	typeAll(d, "# ")
	if d.Blocks[0].Kind != Heading1 {
		t.Fatal("no heading")
	}
	d.Undo()
	if b := d.Blocks[0]; b.Kind != Paragraph || b.Text != "# " {
		t.Fatalf("undo: %+v", b)
	}
	d.Redo()
	if d.Blocks[0].Kind != Heading1 || d.Blocks[0].Text != "" {
		t.Fatal("redo")
	}
}

func TestEnterAndBackspace(t *testing.T) {
	d := newDoc()
	typeAll(d, "- bir\niki\n")
	if k := kinds(d); len(k) != 3 || k[0] != Bullet || k[1] != Bullet || k[2] != Bullet {
		t.Fatalf("list goes on: %v", k)
	}
	d.enter(false) // Enter on an empty item ends the list
	if k := kinds(d); len(k) != 3 || k[2] != Paragraph {
		t.Fatalf("empty item: %v", k)
	}
	typeAll(d, "son")
	d.setCaret(Pos{1, 0})
	d.backspace(false) // a list item's start: it becomes a paragraph
	if d.Blocks[1].Kind != Paragraph {
		t.Fatalf("backspace kind: %v", kinds(d))
	}
	d.backspace(false) // then joins the block above
	if len(d.Blocks) != 2 || d.Blocks[0].Text != "biriki" || d.caret() != (Pos{0, 3}) {
		t.Fatalf("join: %q %v", d.Blocks[0].Text, d.caret())
	}
	// A heading split in the middle: the second half is a paragraph.
	d = newDoc()
	typeAll(d, "# başlık")
	d.setCaret(Pos{0, 3})
	d.enter(false)
	if k := kinds(d); k[0] != Heading1 || k[1] != Paragraph || d.Blocks[1].Text != "lık" {
		t.Fatalf("heading split: %v %q", k, d.Blocks[1].Text)
	}
	// Tasks: a new task is unchecked.
	d = newDoc()
	typeAll(d, "[x] a\n")
	if b := d.Blocks[1]; b.Kind != Todo || b.Checked {
		t.Fatalf("new task: %+v", b)
	}
	// Code: Enter is a line; on an empty last line it leaves.
	d = newDoc()
	typeAll(d, "```a\nb\n\n")
	if k := kinds(d); len(k) != 2 || k[0] != Code || d.Blocks[0].Text != "a\nb" || k[1] != Paragraph {
		t.Fatalf("code exit: %v %q", k, d.Blocks[0].Text)
	}
}

func TestMarksMoveWithEdits(t *testing.T) {
	d := newDoc()
	typeAll(d, "ab **cd** ef")
	d.setCaret(Pos{0, 0})
	typeAll(d, "XY")
	b := d.Blocks[0]
	if b.Text != "XYab cd ef" || b.Marks[0] != (Mark{Bold, 5, 7}) {
		t.Fatalf("shift: %q %+v", b.Text, b.Marks)
	}
	// Typing inside bold is bold; ⌘B at a caret turns it off for what comes next.
	d.setCaret(Pos{0, 6})
	typeAll(d, "z")
	if b.Marks[0] != (Mark{Bold, 5, 8}) {
		t.Fatalf("inside: %+v", b.Marks)
	}
	d.toggleMark(Bold)
	typeAll(d, "q")
	if b.Text != "XYab czqd ef" || b.Marks[0] != (Mark{Bold, 5, 7}) || len(b.Marks) != 2 {
		t.Fatalf("stored off: %q %+v", b.Text, b.Marks)
	}
	// Selection across blocks: format, then delete joins them.
	d = newDoc()
	typeAll(d, "bir\niki")
	d.Sel = Selection{Pos{0, 1}, Pos{1, 2}}
	d.toggleMark(Italic)
	if !d.Blocks[0].hasMark(Italic, 1, 3) || !d.Blocks[1].hasMark(Italic, 0, 2) {
		t.Fatal("format across blocks")
	}
	d.record("delete")
	d.deleteSelection()
	if len(d.Blocks) != 1 || d.Blocks[0].Text != "bi" {
		t.Fatalf("delete across: %q", d.Blocks[0].Text)
	}
}

func TestMarkdownRoundTrip(t *testing.T) {
	src := strings.Join([]string{
		"# Başlık",
		"",
		"Bir **kalın** ve _eğik_, `kod` ile ~~çizik~~ ve **_iç içe_** söz.",
		"",
		"Satır  ",
		"kırılır.",
		"",
		"- bir",
		"  - iç",
		"- [ ] görev",
		"- [x] bitti",
		"1. ilk",
		"2. ikinci",
		"",
		"> alıntı",
		"",
		"```go",
		"fmt.Println(\"**değil**\")",
		"```",
		"",
		"---",
		"",
		`2 \* 3 ve snake\_case`,
		"",
	}, "\n")
	blocks := parseMarkdown(src)
	want := []Kind{Heading1, Paragraph, Paragraph, Bullet, Bullet, Todo, Todo, Numbered, Numbered, Quote, Code, Divider, Paragraph}
	if len(blocks) != len(want) {
		t.Fatalf("blocks: %d, want %d: %v", len(blocks), len(want), kinds(&Doc{Blocks: blocks}))
	}
	for i, k := range want {
		if blocks[i].Kind != k {
			t.Errorf("block %d: kind %d, want %d", i, blocks[i].Kind, k)
		}
	}
	if b := blocks[1]; b.Text != "Bir kalın ve eğik, kod ile çizik ve iç içe söz." {
		t.Errorf("inline text: %q", b.Text)
	}
	if blocks[2].Text != "Satır\nkırılır." {
		t.Errorf("hard break: %q", blocks[2].Text)
	}
	if blocks[4].Depth != 1 || !blocks[6].Checked || blocks[5].Checked {
		t.Errorf("depth/checked: %+v %+v", blocks[4], blocks[6])
	}
	if blocks[10].Text != `fmt.Println("**değil**")` || blocks[10].Lang != "go" {
		t.Errorf("code: %+v", blocks[10])
	}
	if blocks[12].Text != "2 * 3 ve snake_case" || len(blocks[12].Marks) != 0 {
		t.Errorf("escapes: %q %+v", blocks[12].Text, blocks[12].Marks)
	}
	// Written out and read back, nothing changes.
	again := parseMarkdown(toMarkdown(blocks))
	if len(again) != len(blocks) {
		t.Fatalf("round trip: %d blocks, want %d\n%s", len(again), len(blocks), toMarkdown(blocks))
	}
	for i := range blocks {
		a, b := blocks[i], again[i]
		if a.Kind != b.Kind || a.Text != b.Text || a.Depth != b.Depth || a.Checked != b.Checked || len(a.Marks) != len(b.Marks) {
			t.Errorf("round trip block %d:\n %+v\n %+v\n%s", i, a, b, toMarkdown(blocks))
		}
	}
}

// TestEditorTyping drives the real view: keys and text input through MyGo's
// Tester, a frame at a time.
func TestEditorTyping(t *testing.T) {
	d := newDoc()
	e := newEditor(d, "")
	tt := ui.NewTester(e.view, 900, 600)
	keys := func(s string) { // a key at a time, as a keyboard sends them
		for _, r := range s {
			if r == '\n' {
				tt.Key(0, ui.KeyEnter)
			} else {
				tt.Type(string(r))
			}
		}
	}
	keys("# Merhaba\n- bir **iki** üç\ndört")
	tt.Advance(100 * time.Millisecond)
	if k := kinds(d); len(k) != 3 || k[0] != Heading1 || k[1] != Bullet || k[2] != Bullet {
		t.Fatalf("kinds %v", k)
	}
	for _, b := range d.Blocks {
		if strings.ContainsAny(b.Text, "#*") {
			t.Fatalf("a mark is left in the text: %q", b.Text)
		}
	}
	if d.Blocks[1].Text != "bir iki üç" || !d.Blocks[1].hasMark(Bold, 4, 7) {
		t.Fatalf("bold: %q %+v", d.Blocks[1].Text, d.Blocks[1].Marks)
	}
	tt.Key(ui.Cmd, ui.KeyZ) // the run of typing "dört"
	tt.Key(0, ui.KeyUp)     // into the list item above
	if d.caret().B != 1 {
		t.Fatalf("up: %v", d.caret())
	}
	tt.Key(0, ui.KeyTab) // the first item cannot nest: there is no item above it
	if d.Blocks[1].Depth != 1 {
		t.Fatalf("tab: depth %d", d.Blocks[1].Depth)
	}
	md := toMarkdown(d.Blocks)
	if !strings.Contains(md, "# Merhaba") || !strings.Contains(md, "**iki**") {
		t.Fatalf("markdown:\n%s", md)
	}
}
