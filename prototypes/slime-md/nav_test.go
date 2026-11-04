package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func newDoc(text string, at int) (*doc, *ui.Tester) {
	d := &doc{text: text, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820}
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: at, End: at}}
	return d, ui.NewTester(d.view, 820, 400)
}

func TestWordAndLineNav(t *testing.T) {
	word, line := ui.Alt, ui.Super // macOS; the test machine is darwin
	if unitOf(ui.Alt) != byWord {
		word, line = ui.Ctrl, 0
	}
	d, tt := newDoc("hello, wörld 😀 foo\nnext line", 0)
	caret := func() int { return d.selection.Caret() }
	want := func(step string, n int) {
		t.Helper()
		if caret() != n {
			t.Fatalf("%s: caret %d, want %d", step, caret(), n)
		}
	}
	tt.Key(word, ui.KeyRight)
	want("word right", 5) // end of "hello"
	tt.Key(word, ui.KeyRight)
	want("word right over punctuation", 12) // end of "wörld"
	tt.Key(word, ui.KeyRight)
	want("word right over emoji", 19) // end of "foo" (😀 is 2 UTF-16 units)
	tt.Key(word, ui.KeyRight)
	want("word right stops at newline", 20)
	tt.Key(word, ui.KeyLeft)
	want("word left back over newline", 19)
	tt.Key(word, ui.KeyLeft)
	want("word left to word start", 16)
	tt.Key(ui.Shift|word, ui.KeyLeft)
	if r := d.selection.Range; r.Start != 7 || r.End != 16 {
		t.Fatalf("shift+word left selects %v", r)
	}
	tt.Key(0, ui.KeyHome)
	want("home", 0)
	tt.Key(0, ui.KeyEnd)
	want("end stops before the newline", 19)
	if line != 0 {
		tt.Key(line, ui.KeyLeft)
		want("cmd left", 0)
		tt.Key(line, ui.KeyRight)
		want("cmd right", 19)
	}
	tt.Key(ui.Cmd, ui.KeyDown)
	want("cmd down", ui.UTF16Len(d.text))
	tt.Key(word, ui.KeyBackspace)
	if d.text != "hello, wörld 😀 foo\nnext " {
		t.Fatalf("word backspace left %q", d.text)
	}
	tt.Key(ui.Cmd, ui.KeyUp)
	want("cmd up", 0)
}
