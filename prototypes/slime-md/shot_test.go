package main

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestShots types, jumps and renders frames to PNGs: run with SHOTS=dir.
func TestShots(t *testing.T) {
	dir := os.Getenv("SHOTS")
	d := &doc{text: sample, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820}
	end := ui.UTF16Len(d.text)
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	tt := ui.NewTester(d.view, 820, 560)
	save := func(name string) {
		if dir == "" {
			return
		}
		f, _ := os.Create(fmt.Sprintf("%s/%s.png", dir, name))
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(500 * time.Millisecond)
	save("1-rest")
	tt.Type("Hello")
	tt.Advance(40 * time.Millisecond)
	save("2-typing")
	tt.ClickAt(300, 185) // jump up into the list
	for i, ms := range []int{16, 50, 90, 140, 220} {
		tt.Advance(time.Duration(ms) * time.Millisecond)
		save(fmt.Sprintf("3-jump-%d", i))
	}
	tt.Advance(800 * time.Millisecond)
	save("4-settled")
	if d.text == sample {
		t.Fatal("typing did not reach the document")
	}
}
