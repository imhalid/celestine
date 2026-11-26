package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"runtime"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/appicon"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's icon in the look the settings choose (#601): Paper, the icon as it always was; another of the looks of
// internal/appicon, drawn from the same drawing; or Auto, the dark look in a dark window and Paper in a light one.
// The Dock's icon (macOS), the window's (the title bar and the taskbar) and the welcome's logo show it, and change as
// the setting or, in Auto, the window's light or dark does. MyGo's SVG drawing has no filters: the shadow under the
// stickers and the glow behind the carets are drawn here, each part of the drawing blurred as SVG's filters would.

// iconLookIDs are the looks offered, Auto first, each with its name.
var iconLookIDs = []struct{ id, name string }{
	{"auto", "icon_auto"}, {"paper", "icon_paper"}, {"dark", "icon_obsidian"}, {"blue", "icon_blue"},
	{"violet", "icon_violet"}, {"emerald", "icon_emerald"}, {"amber", "icon_amber"}, {"graphite", "icon_graphite"},
}

// iconLook is the look the icon is drawn in now, in a window dark or not.
func (a *app) iconLook(dark bool) appicon.Look { return appicon.Pick(a.settings.AppIcon, dark) }

// iconKey is a look drawn at a size, in the Dock's shape or filling its square.
type iconKey struct {
	id   string
	n    int
	dock bool
}

var (
	iconMu    sync.Mutex
	iconCache = map[iconKey][]byte{}
)

// iconPNG is the icon in look l, n pixels square, as a PNG: in the Dock's shape (a rounded square with room and a
// shadow round it, as macOS's own icons are and scripts/icons.sh draws them) or filling its square (Windows). Paper at
// the sizes the app ships is the file itself.
func iconPNG(l appicon.Look, n int, dock bool) []byte {
	if l.ID == "paper" && dock && n == 512 {
		return appIcon
	}
	if l.ID == "paper" && !dock && n == 256 {
		return winIcon
	}
	k := iconKey{l.ID, n, dock}
	iconMu.Lock()
	b, ok := iconCache[k]
	iconMu.Unlock()
	if ok {
		return b
	}
	var img *image.RGBA
	if dock {
		img = iconDock(l, n)
	} else {
		img = iconArt(l, n)
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		return appIcon
	}
	iconMu.Lock()
	iconCache[k] = buf.Bytes()
	iconMu.Unlock()
	return buf.Bytes()
}

// iconArt is the drawing in look l filling an s-pixel square: the ground, the glow, the stickers' shadow, and the
// stickers with their carets over them.
func iconArt(l appicon.Look, s int) *image.RGBA {
	unit := float64(s) / appicon.GroundUnits
	out := svgPixels(appicon.SVG(l, appicon.PartGround, false), s)
	if l.Glow > 0 {
		glow := svgPixels(appicon.SVG(l, appicon.PartGlow, false), s)
		shade(out, glow, iconRGB(l.Fill[2]), l.Glow, appicon.GlowBlur*unit, 0)
	}
	rims := svgPixels(appicon.SVG(l, appicon.PartRims, false), s)
	shade(out, rims, iconRGB(l.Shadow), l.ShadowOp, appicon.ShadowBlur*unit, appicon.ShadowDY*unit)
	top := svgPixels(appicon.SVG(l, appicon.PartRims|appicon.PartCarets, false), s)
	draw.Draw(out, out.Bounds(), top, image.Point{}, draw.Over)
	return out
}

// iconDock is the drawing in the Dock's shape at n pixels: a rounded square 824/1024 of it, with a soft shadow below.
func iconDock(l appicon.Look, n int) *image.RGBA {
	s := n * 824 / 1024
	m := (n - s) / 2
	r := float64(n) * 185 / 1024
	art := iconArt(l, s)
	mask := image.NewAlpha(image.Rect(0, 0, n, n))
	for y := range s {
		for x := range s {
			mask.Pix[(y+m)*n+x+m] = roundedCover(float64(x)+0.5, float64(y)+0.5, float64(s), r)
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, n, n))
	shade(out, mask, color.Black, 0.30, float64(n)/80+1, float64(n)/100)
	draw.DrawMask(out, image.Rect(m, m, m+s, m+s), art, image.Point{}, mask, image.Pt(m, m), draw.Over)
	return out
}

// roundedCover is how much of the pixel at x, y a rounded square of side s and corner r covers, from 0 to 255, its
// edge smoothed over a pixel.
func roundedCover(x, y, s, r float64) uint8 {
	dx := max(r-x, x-(s-r), 0)
	dy := max(r-y, y-(s-r), 0)
	d := math.Hypot(dx, dy) - r // within the corner's circle: < 0
	if dx == 0 || dy == 0 {
		d = max(-x, x-s, -y, y-s)
	}
	return uint8(255 * min(1, max(0, 0.5-d)))
}

// iconRGB is a color written #RRGGBB, opaque.
func iconRGB(h string) color.NRGBA {
	var v [3]uint8
	for i := range v {
		if len(h) >= 3+2*i {
			var b uint8
			for _, ch := range h[1+2*i : 3+2*i] {
				b <<= 4
				switch {
				case ch >= '0' && ch <= '9':
					b |= uint8(ch - '0')
				case ch >= 'a' && ch <= 'f':
					b |= uint8(ch-'a') + 10
				case ch >= 'A' && ch <= 'F':
					b |= uint8(ch-'A') + 10
				}
			}
			v[i] = b
		}
	}
	return color.NRGBA{v[0], v[1], v[2], 255}
}

// svgPixels draws SVG source src at s pixels square.
func svgPixels(src string, s int) *image.RGBA {
	svg, err := ui.ParseSVG([]byte(src))
	if err != nil {
		return image.NewRGBA(image.Rect(0, 0, s, s))
	}
	return svg.Image(s, s, nil)
}

// shade draws under nothing, over dst, the shape of src's alpha blurred by sigma pixels, moved down dy, in color col
// at opacity op: SVG's feDropShadow, and its feGaussianBlur for a glow.
func shade(dst draw.Image, src image.Image, col color.Color, op, sigma, dy float64) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	a := make([]float32, w*h)
	for y := range h {
		for x := range w {
			_, _, _, al := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			a[y*w+x] = float32(al) / 0xffff
		}
	}
	blur(a, w, h, sigma)
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	shift := int(math.Round(dy))
	for y := range h {
		sy := y - shift
		if sy < 0 || sy >= h {
			continue
		}
		for x := range w {
			mask.Pix[y*w+x] = uint8(255 * min(1, float64(a[sy*w+x])*op))
		}
	}
	draw.DrawMask(dst, dst.Bounds(), image.NewUniform(col), image.Point{}, mask, image.Point{}, draw.Over)
}

// blur blurs a, w by h, by about a gaussian of sigma: three box blurs each way, of the radius whose three passes
// have its variance (a box of radius r has r(r+1)/3).
func blur(a []float32, w, h int, sigma float64) {
	if sigma < 0.5 {
		return
	}
	r := max(1, int(math.Round(math.Sqrt(sigma*sigma+0.25)-0.5)))
	tmp := make([]float32, len(a))
	for range 3 {
		boxLines(a, tmp, h, w, 1, w, r) // each row, along x
		boxLines(tmp, a, w, h, w, 1, r) // each column, along y
	}
}

// boxLines averages each of n lines of src, of length l, over 2r+1 samples into dst; a line's samples are step apart
// and its first is stride after the last line's. What is past a line's ends counts as nothing.
func boxLines(src, dst []float32, n, l, step, stride, r int) {
	k := float32(1) / float32(2*r+1)
	for i := range n {
		base := i * stride
		var sum float32
		for j := 0; j <= r && j < l; j++ {
			sum += src[base+j*step]
		}
		for j := range l {
			dst[base+j*step] = sum * k
			if out := j - r; out >= 0 {
				sum -= src[base+out*step]
			}
			if in := j + r + 1; in < l {
				sum += src[base+in*step]
			}
		}
	}
}

// syncIcon puts the icon in the look it should be in now onto the Dock and the window, when that look is not the one
// they show: as the setting changes, and in Auto as the window goes light or dark. It is drawn away from the window
// and handed to MyGo on the main thread; a test takes it through iconSet.
func (a *app) syncIcon(dark bool) {
	l := a.iconLook(dark)
	if l.ID == a.iconShown || (a.win == nil && a.iconSet == nil) {
		return
	}
	a.iconShown = l.ID
	set := a.iconSet
	if set == nil {
		w := a.win
		set = func(dock, square []byte) {
			mygo.RunOnMain(func() {
				if runtime.GOOS == "windows" {
					w.SetIcon(square) // the title bar and taskbar
				} else {
					w.SetIcon(dock) // the title bar and taskbar (Linux)
				}
				mygo.App.Dock.SetIcon(dock) // the Dock (macOS)
			})
		}
	}
	go func() {
		defer a.guard()
		set(iconPNG(l, 512, true), iconPNG(l, 256, false))
	}()
}

// iconImage is look l's icon in the Dock's shape, n pixels, for the window to show: the welcome's logo, the
// settings' tiles. It is drawn at about the size it is shown at, twice for a sharp display: a big picture made
// small at every frame slowed the settings, as the renderer without a GPU does it (#601).
func (a *app) iconImage(l appicon.Look, n int) ui.ImageSource {
	k := iconKey{l.ID, n, true}
	if img, ok := a.iconImages[k]; ok {
		return img
	}
	img, err := ui.DecodeImage(iconPNG(l, n, true))
	if err != nil {
		return nil
	}
	if a.iconImages == nil {
		a.iconImages = map[iconKey]ui.ImageSource{}
	}
	a.iconImages[k] = img
	return img
}

// setAppIcon chooses the icon's look, mode an ID of iconLookIDs.
func (a *app) setAppIcon(mode string) {
	if mode == "paper" {
		mode = "" // the icon as it always was: nothing to keep
	}
	a.settings.AppIcon = mode
	a.saveSettings()
}

// appIconMode is the mode the settings say, as iconLookIDs names it.
func (a *app) appIconMode() string {
	if a.settings.AppIcon == "" {
		return "paper"
	}
	if _, ok := appicon.ByID(a.settings.AppIcon); !ok && a.settings.AppIcon != "auto" {
		return "paper"
	}
	return a.settings.AppIcon
}

// appIconPicker is the settings' row of the icon's looks, each a tile with the icon in it; Auto's has Paper and the
// dark look side by side.
func (a *app) appIconPicker(c *ui.Context) {
	cl := ryugo.From(c).Colors
	mode := a.appIconMode()
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			settingTitle(c, cl, locales.IconAppIcon())
			ui.Text(c, locales.IconAppIconHelp()).FontSize(12.5).TextColor(cl.Muted)
		})
		ui.Row(c).Gap(14).Wrap().Children(func() {
			for _, it := range iconLookIDs {
				preview := func() {
					ui.Row(c).Width(112).Height(66).Justify(ui.Center).AlignItems(ui.Center).Gap(2).Background(cl.Text.Alpha(0.04)).Children(func() {
						if it.id == "auto" {
							paper, _ := appicon.ByID("paper")
							dark, _ := appicon.ByID("dark")
							ui.Image(c.Key("icon-auto-l"), a.iconImage(paper, 128)).Size(54, 54)
							ui.Image(c.Key("icon-auto-d"), a.iconImage(dark, 128)).Size(54, 54)
							return
						}
						l, _ := appicon.ByID(it.id)
						ui.Image(c.Key("icon-"+it.id), a.iconImage(l, 128)).Size(62, 62)
					})
				}
				if choiceTile(c, "tile-icon-"+it.id, msg(it.name), mode == it.id, preview) {
					a.setAppIcon(it.id)
				}
			}
		})
	})
}
