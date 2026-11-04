package main

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestMizanStrip writes at a rising pace with the Mizan caret, jumps, and
// stops; it saves a contact sheet with MIZAN=file.
func TestMizanStrip(t *testing.T) {
	d := &doc{text: sample, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820, caret: caretMizan}
	end := ui.UTF16Len(d.text)
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	tt := ui.NewTester(d.view, 820, 560)
	tt.Advance(300 * time.Millisecond)
	crop := image.Rect(0, 250, 520, 380)
	var frames []*image.RGBA
	grab := func() {
		f := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
		draw.Draw(f, f.Bounds(), tt.Image(), crop.Min, draw.Src)
		frames = append(frames, f)
	}
	words := "the quick brown fox jumps over"
	for i, ch := range words {
		tt.Type(string(ch))
		gap := 190 - i*5 // faster and faster
		tt.Advance(time.Duration(max(gap, 60)) * time.Millisecond)
		if i == 8 || i == 18 || i == len(words)-1 {
			grab()
		}
	}
	if len(d.mizan.pace) < 10 {
		t.Fatalf("pace has %d points", len(d.mizan.pace))
	}
	if d.mizan.pace[len(d.mizan.pace)-1] <= d.mizan.base {
		t.Fatalf("the pace did not rise: %.0f from %.0f", d.mizan.pace[len(d.mizan.pace)-1], d.mizan.base)
	}
	tt.ClickAt(200, 125) // a jump up the page: the pill glides
	tt.Advance(80 * time.Millisecond)
	if d.mizan.glide == nil {
		t.Fatal("no glide after a jump")
	}
	crop = image.Rect(0, 100, 520, 230)
	grab()
	tt.Advance(3 * time.Second)
	if d.mizan.glide != nil {
		t.Fatal("the glide did not end")
	}
	grab()
	if out := os.Getenv("MIZAN"); out != "" {
		sheet := image.NewRGBA(image.Rect(0, 0, 520*2, 130*3))
		for i, f := range frames {
			x, y := (i%2)*520, (i/2)*130
			draw.Draw(sheet, image.Rect(x, y, x+520, y+130), f, image.Point{}, draw.Src)
		}
		fo, _ := os.Create(out)
		png.Encode(fo, sheet)
		fo.Close()
	}
}
