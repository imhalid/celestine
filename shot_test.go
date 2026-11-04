package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// TestShot draws the welcome document: run with SHOT=file.png.
func TestShot(t *testing.T) {
	out := os.Getenv("SHOT")
	if out == "" {
		t.Skip()
	}
	a := newApp(richtext.NewDocument(richtext.ParseMarkdown(welcome)), "/tmp/belge.md")
	a.doc.SetSelection(120, 120)
	tt := ui.NewTester(a.view, 960, 820)
	tt.Advance(400 * time.Millisecond)
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
}
