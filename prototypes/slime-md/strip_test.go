package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestStrip saves a contact sheet of a jump, frame by frame: run with STRIP=file.
func TestStrip(t *testing.T) {
	out := os.Getenv("STRIP")
	if out == "" {
		t.Skip()
	}
	d := &doc{text: sample, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820}
	end := ui.UTF16Len(d.text)
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	tt := ui.NewTester(d.view, 820, 560)
	tt.Type("Hi")
	tt.Advance(600 * time.Millisecond)
	crop := image.Rect(20, 160, 360, 340)
	sheet := image.NewRGBA(image.Rect(0, 0, crop.Dx()*3, crop.Dy()*4))
	grab := func(k int) {
		x, y := (k%3)*crop.Dx(), (k/3)*crop.Dy()
		draw.Draw(sheet, image.Rect(x, y, x+crop.Dx(), y+crop.Dy()), tt.Image(), crop.Min, draw.Src)
	}
	grab(0)
	tt.ClickAt(300, 185)
	for k := 1; k < 12; k++ {
		tt.Advance(time.Duration(map[bool]int{true: 16, false: 40}[k < 7]) * time.Millisecond)
		grab(k)
	}
	f, _ := os.Create(out)
	png.Encode(f, sheet)
	f.Close()
	fmt.Println("ok")
}
