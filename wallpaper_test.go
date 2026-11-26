package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/wallpaper"
)

// With the setting on, the frame's ground is the wallpaper: read away from the window, drawn under the frame washed
// toward the appearance, and read again, the dark one, when the window goes dark. One that cannot be read leaves the
// frame's own ground (#606). WALL_SHOT_DIR keeps a picture of it.
func TestWallpaperGround(t *testing.T) {
	var mu sync.Mutex
	asked := []bool{}
	defer func(f func(bool) (wallpaper.Source, error), l func(wallpaper.Source) (image.Image, error)) {
		wallFind, wallLoad = f, l
	}(wallFind, wallLoad)
	wallFind = func(dark bool) (wallpaper.Source, error) {
		mu.Lock()
		asked = append(asked, dark)
		mu.Unlock()
		return wallpaper.Source{Path: map[bool]string{false: "light", true: "dark"}[dark]}, nil
	}
	wallLoad = func(s wallpaper.Source) (image.Image, error) {
		c := color.NRGBA{255, 60, 40, 255} // a red wallpaper; its dark one blue
		if s.Path == "dark" {
			c = color.NRGBA{20, 60, 220, 255}
		}
		img := image.NewNRGBA(image.Rect(0, 0, 64, 40))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
		return img, nil
	}
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.settings.Theme = 1
	a.settings.WallpaperFrame = true
	tt.Frame()
	wait := func() {
		for range 200 {
			a.mu.Lock()
			busy := a.wall.busy
			a.mu.Unlock()
			if !busy {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		tt.Frame()
		tt.Frame()
	}
	wait()
	if !a.wall.ok || a.wall.dark {
		t.Fatalf("not read: %+v", a.wall)
	}
	pixel := func() color.NRGBA { // the frame's ground, at its left edge under the sidebar's buttons
		return color.NRGBAModel.Convert(tt.Image().At(3, 350)).(color.NRGBA)
	}
	if p := pixel(); int(p.R) <= int(p.B)+20 {
		t.Fatalf("the ground is not the red wallpaper's: %v", p)
	}
	if dir := os.Getenv("WALL_SHOT_DIR"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "wall-light.png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
	if b := a.wallTintBase(); b == a.pageBg {
		t.Fatal("the glass's tint does not take the wallpaper's color")
	}
	a.settings.Theme = 2 // dark: its own wallpaper
	tt.Frame()
	wait()
	if !a.wall.dark || !a.wall.ok {
		t.Fatalf("dark: %+v", a.wall)
	}
	if p := pixel(); int(p.B) <= int(p.R)+10 || int(p.R)+int(p.G)+int(p.B) > 3*110 {
		t.Fatalf("the dark ground is not the blue wallpaper's, deep: %v", p)
	}
	mu.Lock()
	if len(asked) != 2 || asked[0] || !asked[1] {
		t.Fatalf("looked for %v", asked)
	}
	mu.Unlock()
	tt.Frame() // nothing changed: not looked for again before wallEvery
	mu.Lock()
	if len(asked) != 2 {
		t.Fatalf("looked for again: %v", asked)
	}
	mu.Unlock()

	wallLoad = func(wallpaper.Source) (image.Image, error) { return nil, errors.New("a moving wallpaper") }
	a.settings.Theme = 1
	tt.Frame()
	wait()
	if a.wall.ok {
		t.Fatal("a wallpaper that cannot be read is taken")
	}
	if p := pixel(); int(p.R) > int(p.B)+20 { // in int: a blue of 236 and over made 20 more wrap round in uint8
		t.Fatalf("still the old wallpaper: %v", p)
	}
	a.settings.WallpaperFrame = false
	if a.wallActive() {
		t.Fatal("the setting off, still the wallpaper")
	}
}
