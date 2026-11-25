package main

import (
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Ctrl+L, Ctrl+E and Ctrl+R leave the paragraph as it is off macOS: the editor would align it (#280).
func TestNoAlignKeys(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the keys align nothing on macOS")
	}
	a, tt := newTestApp(t, "hello world\n")
	tt.Advance(100 * time.Millisecond)
	a.pane().refocus = true
	tt.Frame()
	a.pane().doc.SetSelection(2, 2)
	tt.Frame()
	for _, k := range []ui.Key{ui.KeyR, ui.KeyE, ui.KeyL} {
		tt.Key(ui.Cmd, k)
		tt.Frame()
		if al := a.pane().doc.Text().ParagraphStyleAt(2).Align; al != 0 {
			t.Fatalf("key %v aligned the paragraph: %v", k, al)
		}
	}
	if a.pane().doc.Text().String() != "hello world" {
		t.Fatalf("the text changed: %q", a.pane().doc.Text().String())
	}
}
