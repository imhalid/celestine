package wallpaper

import (
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fill is a w by h picture of c, or of two colors in halves.
func fill(w, h int, cs ...color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, cs[x*len(cs)/w])
		}
	}
	return img
}

// Held for a light window, any wallpaper's color is light and soft; for a dark one, deep and soft: the bounds a
// window's text stays crisp within, for a white, a black, a saturated and a busy picture (#606).
func TestClampKeepsTextLegible(t *testing.T) {
	pics := map[string]image.Image{
		"white": fill(10, 10, color.NRGBA{255, 255, 255, 255}),
		"black": fill(10, 10, color.NRGBA{0, 0, 0, 255}),
		"red":   fill(10, 10, color.NRGBA{255, 0, 0, 255}),
		"busy":  fill(40, 10, color.NRGBA{255, 220, 0, 255}, color.NRGBA{0, 40, 200, 255}, color.NRGBA{255, 0, 120, 255}),
	}
	for name, pic := range pics {
		for _, dark := range []bool{false, true} {
			c := Clamp(Color(pic), dark)
			l, a, b := toOKLab(c)
			lo, hi, maxC := 0.855, 0.955, 0.075
			if dark {
				lo, hi, maxC = 0.165, 0.275, 0.085
			}
			if l < lo || l > hi || math.Hypot(a, b) > maxC {
				t.Errorf("%s, dark %v: L %.3f chroma %.3f", name, dark, l, math.Hypot(a, b))
			}
		}
	}
	// the hue stays: a red wallpaper's ground is reddish, a blue one's bluish
	r, b := Clamp(Color(pics["red"]), false), Clamp(Color(fill(4, 4, color.NRGBA{0, 60, 255, 255})), false)
	if r.R <= r.B || b.B <= b.R {
		t.Errorf("the hue went: red %v, blue %v", r, b)
	}
}

// The color of a picture leans toward its colorful part: a gray picture with a colored half is not gray.
func TestColorLeansToTheColorful(t *testing.T) {
	c := Color(fill(20, 4, color.NRGBA{128, 128, 128, 255}, color.NRGBA{0, 140, 255, 255}))
	if c.B < c.R+40 {
		t.Fatalf("%v", c)
	}
}

// Blurred is small and has no sharp edge left where the picture had one.
func TestBlurred(t *testing.T) {
	b := Blurred(fill(1200, 600, color.NRGBA{0, 0, 0, 255}, color.NRGBA{255, 255, 255, 255}), 480, 9)
	if b.Rect.Dx() != 480 || b.Rect.Dy() != 240 {
		t.Fatalf("%v", b.Rect)
	}
	l, r := b.NRGBAAt(238, 120).R, b.NRGBAAt(241, 120).R
	if int(r)-int(l) > 40 {
		t.Fatalf("a sharp edge left: %d to %d", l, r)
	}
}

// A picture Go cannot read is read from its thumbnail as GNOME keeps it, and a desktop with no picture to read but
// a color is that color; with none, there is no wallpaper.
func TestLoadFallbacks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	path := filepath.Join(dir, "wall.jxl")
	os.WriteFile(path, []byte("not a picture Go reads"), 0o644)
	sum := md5.Sum([]byte("file://" + path))
	os.MkdirAll(filepath.Join(dir, "thumbnails", "large"), 0o755)
	f, _ := os.Create(filepath.Join(dir, "thumbnails", "large", hex.EncodeToString(sum[:])+".png"))
	png.Encode(f, fill(8, 8, color.NRGBA{10, 200, 30, 255}))
	f.Close()
	img, err := Load(Source{Path: path})
	if err != nil || img.Bounds().Dx() != 8 {
		t.Fatalf("thumbnail: %v", err)
	}
	if runtime.GOOS == "darwin" { // sips is tried first there
		return
	}
	img, err = Load(Source{Path: filepath.Join(dir, "none.jxl"), Color: "#86b6ef"})
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); c != (color.NRGBA{0x86, 0xb6, 0xef, 255}) {
		t.Fatalf("the desktop's color: %v", c)
	}
	if _, err := Load(Source{}); err != ErrNone {
		t.Fatalf("nothing: %v", err)
	}
}

// On a GNOME desktop the wallpaper is gsettings' picture, the dark one for a dark window when there is one.
func TestFindGNOME(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("GNOME's settings are Linux's")
	}
	dir := t.TempDir()
	light, darkPic := filepath.Join(dir, "l.png"), filepath.Join(dir, "my d.png")
	os.WriteFile(light, nil, 0o644)
	os.WriteFile(darkPic, nil, 0o644)
	defer func(r func(string, ...string) (string, error)) { run = r }(run)
	run = func(name string, args ...string) (string, error) {
		switch args[len(args)-1] {
		case "picture-uri":
			return "'file://" + light + "'", nil
		case "picture-uri-dark":
			return "'file://" + strings.ReplaceAll(darkPic, " ", "%20") + "'", nil
		case "primary-color":
			return "'#112233'", nil
		}
		return "", os.ErrNotExist
	}
	for _, c := range []struct {
		dark bool
		want string
	}{{false, light}, {true, darkPic}} {
		s, err := Find(c.dark)
		if err != nil || s.Path != c.want || s.Color != "#112233" || s.Mod.IsZero() {
			t.Fatalf("dark %v: %+v %v", c.dark, s, err)
		}
	}
}
