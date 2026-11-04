package main

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestWaveStrip saves a contact sheet of a shockwave: run with WAVE=file.
func TestWaveStrip(t *testing.T) {
	if !shockwaveOn {
		t.Skip("the shockwave is off")
	}
	d := &doc{text: sample, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820}
	at := ui.UTF16Len(sample[:strings.Index(sample, "click far")])
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: at, End: at}}
	tt := ui.NewTester(d.view, 820, 560)
	tt.Advance(400 * time.Millisecond)
	crop := image.Rect(10, 120, 610, 330)
	sheet := image.NewRGBA(image.Rect(0, 0, crop.Dx()*2, crop.Dy()*4))
	grab := func(k int) {
		x, y := (k%2)*crop.Dx(), (k/2)*crop.Dy()
		draw.Draw(sheet, image.Rect(x, y, x+crop.Dx(), y+crop.Dy()), tt.Image(), crop.Min, draw.Src)
	}
	grab(0)
	tt.Type("x")
	for k, ms := range []int{40, 60, 80, 120, 200, 300, 500} {
		tt.Advance(time.Duration(ms) * time.Millisecond)
		grab(k + 1)
	}
	if !strings.Contains(d.text, "xclick") {
		t.Fatal("typing did not land")
	}
	tt.Advance(2 * time.Second)
	if d.quake.busy() {
		t.Fatal("letters did not settle")
	}
	if out := os.Getenv("WAVE"); out != "" {
		f, _ := os.Create(out)
		png.Encode(f, sheet)
		f.Close()
	}
}
