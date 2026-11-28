package main

import (
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// TestSkyShots renders Today at ten moments of the day under each weather into one contact sheet, to look at the
// sky as a whole when its colors change. It runs only when SKY_SHOTS names a folder: SKY_PREF sets the appearance
// ("light", "dark"), SKY_SHEET opens the Settings sheet over the page, SKY_FULL writes each frame at full size, and
// SKY_PICK (frame names, comma-separated) lays the named frames side by side in detail.png.
func TestSkyShots(t *testing.T) {
	out := os.Getenv("SKY_SHOTS")
	if out == "" {
		t.Skip("SKY_SHOTS not set")
	}
	hours := []float64{5.5, 7.4, 9.5, 12.5, 15.0, 16.3, 17.2, 18.4, 20.5, 23.5}
	kinds := []weatherKind{weatherClear, weatherPartly, weatherCloudy, weatherFog, weatherRain, weatherStorm, weatherSnow}
	if os.Getenv("SKY_PREF") != "" {
		hours = []float64{7.4, 12.5, 16.3, 20.5}
	}
	const w, h = 412, 860
	const sw, sh = w / 2, h / 2
	sheet := image.NewRGBA(image.Rect(0, 0, sw*len(hours), sh*len(kinds)))
	for yi, k := range kinds {
		for xi, hr := range hours {
			a, tt := newTestApp(t)
			a.settings.Theme = os.Getenv("SKY_PREF")
			base := time.Date(2026, 11, 28, 0, 0, 0, 0, time.Local)
			now := base.Add(time.Duration(hr * float64(time.Hour)))
			a.clock = func() time.Time { return now }
			a.wx.info = weatherInfo{Kind: k, TempC: 9, HasWeather: true, SunriseHr: 7.2, SunsetHr: 16.8,
				SolarDay: "2026-11-28", City: "Istanbul"}
			a.showToday()
			a.ed.doc.SetText(textOf(library.Doc{
				{S: &library.ParaStyle{Heading: 2}, R: []library.Run{{T: "Sabah"}}},
				{R: []library.Run{{T: "Kahveyi balkonda içtim, hava "}, {T: "serin", W: 700}, {T: " ama berrak. Bugün yavaş başlamak istiyorum."}}},
				{S: &library.ParaStyle{List: library.ListTasks, Checked: true}, R: []library.Run{{T: "Faturayı öde"}}},
				{S: &library.ParaStyle{List: library.ListTasks}, R: []library.Run{{T: "Annemi ara"}}},
				{S: &library.ParaStyle{List: library.ListTasks}, R: []library.Run{{T: "Okur için PDF seçimi"}}},
				{R: []library.Run{{T: "Akşam yürüyüşe çıkarsam sahile kadar inerim."}}},
			}))
			tt.Frame()
			tt.Frame()
			if os.Getenv("SKY_SHEET") != "" {
				a.sheet = sheetSettings
				tt.Frame()
				tt.Advance(time.Second)
			}
			img := tt.Image()
			small := image.NewRGBA(image.Rect(0, 0, sw, sh))
			b := img.Bounds()
			for y := 0; y < sh; y++ {
				for x := 0; x < sw; x++ {
					small.Set(x, y, img.At(b.Min.X+x*b.Dx()/sw, b.Min.Y+y*b.Dy()/sh))
				}
			}
			draw.Draw(sheet, image.Rect(xi*sw, yi*sh, (xi+1)*sw, (yi+1)*sh), small, image.Point{}, draw.Src)
			if os.Getenv("SKY_FULL") != "" {
				f, _ := os.Create(filepath.Join(out, string(k)+"-"+now.Format("1504")+".png"))
				png.Encode(f, img)
				f.Close()
			}
		}
	}
	if picks := os.Getenv("SKY_PICK"); picks != "" {
		names := strings.Split(picks, ",")
		det := image.NewRGBA(image.Rect(0, 0, w*len(names), h))
		for i, n := range names {
			f, err := os.Open(filepath.Join(out, n+".png"))
			if err != nil {
				t.Fatal(err)
			}
			im, _ := png.Decode(f)
			f.Close()
			draw.Draw(det, image.Rect(i*w, 0, (i+1)*w, h), im, im.Bounds().Min, draw.Src)
		}
		f, _ := os.Create(filepath.Join(out, "detail.png"))
		png.Encode(f, det)
		f.Close()
	}
	name := "sheet" + os.Getenv("SKY_PREF") + ".png"
	f, err := os.Create(filepath.Join(out, name))
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, sheet)
	f.Close()
}

// TestSkyStormMotion renders the storm moving (SKY_SHOTS=<dir>): three frames a moment apart, and the frame of a
// lightning flash, into storm-motion.png.
func TestSkyStormMotion(t *testing.T) {
	out := os.Getenv("SKY_SHOTS")
	if out == "" {
		t.Skip("SKY_SHOTS not set")
	}
	a, tt := newTestApp(t)
	now := time.Date(2026, 11, 28, 15, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	a.wx.info = weatherInfo{Kind: weatherStorm, HasWeather: true, SunriseHr: 7.2, SunsetHr: 16.8, SolarDay: "2026-11-28"}
	a.skyLive = true
	a.showToday()
	tt.Frame()
	const w, h = 412, 860
	strip := image.NewRGBA(image.Rect(0, 0, w*4, h))
	for i := 0; i < 3; i++ {
		tt.Advance(400 * time.Millisecond)
		img := tt.Image()
		draw.Draw(strip, image.Rect(i*w, 0, (i+1)*w, h), img, img.Bounds().Min, draw.Src)
	}
	for i := 0; i < 600; i++ { // up to 24 s, for the flash's next peak
		tt.Advance(40 * time.Millisecond)
		if u := math.Mod(skyHeld, 11.3); u < 0.06 {
			img := tt.Image()
			draw.Draw(strip, image.Rect(3*w, 0, 4*w, h), img, img.Bounds().Min, draw.Src)
			break
		}
	}
	f, err := os.Create(filepath.Join(out, "storm-motion.png"))
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, strip)
	f.Close()
}
