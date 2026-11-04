package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestShot draws the welcome document: run with SHOT=file.png.
func TestShot(t *testing.T) {
	out := os.Getenv("SHOT")
	if out == "" {
		t.Skip()
	}
	d := newDoc(parseMarkdown(welcome)...)
	d.setCaret(Pos{3, 9})
	e := newEditor(d, "/tmp/belge.md")
	tt := ui.NewTester(e.view, 960, 900)
	tt.Advance(300 * time.Millisecond)
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
}
