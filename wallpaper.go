package main

import (
	"bytes"
	"image"
	"image/png"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/wallpaper"
	ryugo "github.com/imhalid/ryu-go"
)

// The desktop's wallpaper as the ground of the window's frame (#606). The glass of macOS (VibrancyGlass) mixes in
// whatever is behind the window, so a bright browser or a dark terminal behind it changed its colors. With Glass on,
// the frame's ground is instead the wallpaper, blurred, where the window stands on its display: the same colors
// whatever is behind, in front or not, and the glass's tint is the wallpaper's color held light or dark enough for
// the text (wallpaper.Clamp). On Linux and Windows the setting Wallpaper behind the frame does the same. A wallpaper
// that cannot be read (a moving one, a sandbox) leaves the frame's own ground (frameGroundOf).

// wallState is the wallpaper as it was last read.
type wallState struct {
	src     wallpaper.Source
	dark    bool           // the appearance it was read for: a desktop may have a light and a dark one
	img     ui.ImageSource // it, small and blurred
	color   ui.Color       // its color, held for the appearance (Clamp)
	ok      bool           // read: img and color are its
	busy    bool           // being read away from the window
	checked time.Time      // when it was last looked for
}

// wallEvery is how often the wallpaper is looked for again: a new one is taken within this.
const wallEvery = 30 * time.Second

// wallFind and wallLoad are how the wallpaper is found and read: a test puts its own.
var (
	wallFind = wallpaper.Find
	wallLoad = wallpaper.Load
)

// wallActive reports whether the frame's ground is the wallpaper: with Glass on macOS, or the setting elsewhere.
func (a *app) wallActive() bool {
	if !a.wframe.on || phone || runtime.GOOS == "js" {
		return false
	}
	return a.settings.Glass && runtime.GOOS == "darwin" || a.settings.WallpaperFrame
}

// syncWallpaper looks for the wallpaper again when the appearance changed or a while has gone, away from the window,
// and takes it in when it is another.
func (a *app) syncWallpaper(dark bool) {
	w := &a.wall
	if !a.wallActive() || w.busy || (w.dark == dark && !w.checked.IsZero() && a.clock().Sub(w.checked) < wallEvery) {
		return
	}
	w.busy, w.checked = true, a.clock()
	was, wasDark, had := w.src, w.dark, w.ok
	go func() {
		defer a.guard()
		src, err := wallFind(dark)
		if err == nil && had && wasDark == dark && src.Path == was.Path && src.Mod.Equal(was.Mod) && src.Color == was.Color {
			a.do(func() error { a.wall.busy = false; return nil }) // the same one
			return
		}
		var img ui.ImageSource
		var col ui.Color
		ok := false
		if err == nil {
			if pic, err := wallLoad(src); err == nil {
				img, col, ok = wallImage(pic), uiColor(wallpaper.Clamp(wallpaper.Color(pic), dark)), true
			}
		}
		a.do(func() error {
			a.wall = wallState{src: src, dark: dark, img: img, color: col, ok: ok && img != nil, checked: a.wall.checked}
			a.glassHasTint = false // the glass's tint is made again, of the wallpaper's color
			return nil
		})
	}()
}

// wallImage is the picture blurred and small, as the window can draw it.
func wallImage(pic image.Image) ui.ImageSource {
	var buf bytes.Buffer
	if png.Encode(&buf, wallpaper.Blurred(pic, 480, 9)) != nil {
		return nil
	}
	img, err := ui.DecodeImage(buf.Bytes())
	if err != nil {
		return nil
	}
	return img
}

// uiColor is c as the window's color.
func uiColor(c interface{ RGBA() (r, g, b, a uint32) }) ui.Color {
	r, g, b, _ := c.RGBA()
	return ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// wallRect is where the window's display is, in the window's own coordinates (r its size): the wallpaper covers the
// display, and the window shows the part of it it stands over. Without a window (a test) it is the window.
func (a *app) wallRect(r ui.Rect) ui.Rect {
	if a.win == nil {
		return r
	}
	wb := a.win.Bounds()
	if wb.Width <= 0 || wb.Height <= 0 {
		return r
	}
	d := mygo.Screen.DisplayMatching(wb).Bounds
	if d.Width <= 0 || d.Height <= 0 {
		return r
	}
	k := r.W / float32(wb.Width) // the window's units per the screen's
	return ui.Rect{X: r.X + float32(d.X-wb.X)*k, Y: r.Y + float32(d.Y-wb.Y)*k, W: float32(d.Width) * k, H: float32(d.Height) * k}
}

// paintWallGround paints the wallpaper as the frame's ground over r, washed toward the window's appearance so that
// the frame's buttons and words read on it, and the frame's tint of the accent over it, as its own ground has.
func (a *app) paintWallGround(p *ui.Painter, r ui.Rect, g frameGround, dark bool) {
	w := &a.wall
	p.Fill(r, w.color, 0)
	p.Clip(r, 0, func() { p.Image(w.img, a.wallRect(r), ui.Cover) })
	// over it, more than half of its color as held for the appearance (Clamp): whatever the picture, the ground's
	// lightness and color stay near those bounds, and the frame's text and hairlines crisp on it
	wash := ui.RGB(255, 255, 255).Alpha(0.18)
	if dark {
		wash = ui.RGB(0, 0, 0).Alpha(0.22)
	}
	p.Fill(r, wash, 0)
	p.Fill(r, w.color.Alpha(0.58), 0)
	clear := g.tint
	clear.A = 0
	p.FillGradient(r, ui.LinearGradient{From: clear, To: g.tint.Alpha(float32(g.tint.A) / 255 * 0.6), Angle: 90, Start: 0.35, End: 1}, 0)
}

// wallTintBase is what the glass's tint is made of: the page's background, mixed with the wallpaper's color when the
// ground is the wallpaper, so that the panels over the glass have its colors too.
func (a *app) wallTintBase() ui.Color {
	if a.wallActive() && a.wall.ok {
		return ryugo.Mix(a.pageBg, a.wall.color, 0.35)
	}
	return a.pageBg
}
