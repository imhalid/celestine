package main

import (
	_ "embed"
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/mobile/locales"
)

// The frame's palette is built on the sky (sky.go): the paper's gradient is the sky's, and its ink, surfaces and
// accent are set against it so they read on every hour and weather, under organic cotton-paper grain.

//go:embed fonts/Geist.ttf
var geistFont []byte

const fontFamily = "Geist"

func registerFonts() {
	if err := ui.RegisterFont(geistFont, fontFamily); err != nil {
		log.Printf("font %s: %v", fontFamily, err)
	}
}

type palette struct {
	bgTop      ui.Color // top of the living sky-paper gradient
	bgBottom   ui.Color // bottom of the living sky-paper gradient
	bg         ui.Color // blended midpoint paper tone (for solid fallbacks)
	sheet      ui.Color // elevated sheet surface
	surface    ui.Color // recessed pills and inputs
	raised     ui.Color // pressed surface
	line       ui.Color // hairline tracks and dividers
	text       ui.Color // primary editorial ink
	muted      ui.Color // secondary ink
	faint      ui.Color // whisper ink
	accent     ui.Color // living accent tuned to the minute & weather
	accentSoft ui.Color // soft accent wash
	accentText ui.Color // foreground on accent
	danger     ui.Color // sync errors
	toastBg    ui.Color // toast pill background
	toastText  ui.Color // toast pill ink
	scrim      ui.Color // sheet backdrop dimming
	grainTint  ui.Color // organic paper grain overlay tint
}

// dayPhaseName returns the quiet atmospheric phase name anchored to local sunrise/sunset.
func dayPhaseName(now time.Time, wx weatherInfo) string {
	sunrise, sunset := wx.solarWindow(now)
	t := now.Local()
	h := float32(t.Hour()) + float32(t.Minute())/60.0
	switch {
	case h >= sunrise-0.6 && h < sunrise+1.5:
		return locales.DayDawn()
	case h >= sunrise+1.5 && h < 12.0:
		return locales.DayMorning()
	case h >= 12.0 && h < sunset-1.35:
		return locales.DayAfternoon()
	case h >= sunset-1.35 && h < sunset+0.55:
		return locales.DaySunset()
	case h >= sunset+0.55 && h < min(sunset+3.5, 23.0):
		return locales.DayEvening()
	default:
		return locales.DayMidnight()
	}
}

// paperGradientAt returns the sky's zenith, horizon, accent and polarity at now, for pref and wx.
func paperGradientAt(now time.Time, pref string, wx weatherInfo) (top, bottom, accent ui.Color, isDark bool) {
	s := skyAt(now, pref, wx)
	return s.top, s.bottom, s.accent, s.dark
}

// isDarkFor reports whether the paper at now under pref & wx uses dark polarity (light text & light status bar icons).
func isDarkFor(now time.Time, pref string, wx weatherInfo) bool {
	return skyAt(now, pref, wx).dark
}

// paletteAt builds the frame's palette on the sky at now: its ink is set against the sky's lightest or darkest
// end, so text, secondary text and the whisper labels keep their contrast on every sky.
func paletteAt(now time.Time, pref string, wx weatherInfo) (palette, bool) {
	return paletteOf(skyAt(now, pref, wx))
}

func paletteOf(s sky) (palette, bool) {
	bg := s.top.MixOklab(s.bottom, 0.5)
	tl, bl := toLCH(s.top), toLCH(s.bottom)
	hue := tl.H
	ink := func(l, c float64) ui.Color { return lch{l, c, hue}.color() }
	if s.dark {
		edge := max(tl.L, bl.L)
		text := lch{0.95, 0.012, 85}.color() // warm cream, as a page under a lamp
		return palette{
			bgTop:      s.top,
			bgBottom:   s.bottom,
			bg:         bg,
			sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.07),
			surface:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.11),
			raised:     bg.MixOklab(ui.Hex("#FFFFFF"), 0.17),
			line:       bg.MixOklab(ui.Hex("#FFFFFF"), 0.15),
			text:       text,
			muted:      reach(lch{min(edge+0.4, 0.83), 0.02, hue}, s, 4.8),
			faint:      reach(lch{min(edge+0.3, 0.72), 0.025, hue}, s, 3.2),
			accent:     s.accent,
			accentSoft: s.accent.MixOklab(bg, 0.76),
			accentText: ui.Hex("#141317"),
			danger:     ui.Hex("#FF8272"),
			toastBg:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.17),
			toastText:  text,
			scrim:      ui.Hex("#04060C"),
			grainTint:  ui.Hex("#F5F1E8").Alpha(0.05),
		}, true
	}
	edge := min(tl.L, bl.L)
	text := ink(0.24, 0.018)
	return palette{
		bgTop:      s.top,
		bgBottom:   s.bottom,
		bg:         bg,
		sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.66),
		surface:    bg.MixOklab(text, 0.065),
		raised:     bg.MixOklab(text, 0.115),
		line:       bg.MixOklab(text, 0.13),
		text:       text,
		muted:      reach(lch{max(edge-0.4, 0.38), 0.025, hue}, s, 4.8),
		faint:      reach(lch{max(edge-0.29, 0.5), 0.03, hue}, s, 3.2),
		accent:     s.accent,
		accentSoft: s.accent.MixOklab(bg, 0.8),
		accentText: ui.Hex("#FFFFFF"),
		danger:     ui.Hex("#C23C34"),
		toastBg:    text,
		toastText:  bg,
		scrim:      ui.Hex("#121318"),
		grainTint:  text.Alpha(0.058),
	}, false
}

var (
	pal    palette
	dark   bool
	curSky sky // the sky of the frame being built, drawn by drawSky
)

func init() {
	curSky = skyAt(time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local), "", weatherInfo{})
	pal, dark = paletteOf(curSky)
}

func themeOf(now time.Time, pref string, wx weatherInfo) (*ui.Theme, bool) {
	curSky = skyAt(now, pref, wx)
	p, isDark := paletteOf(curSky)
	pal, dark = p, isDark
	t := ui.LightTheme()
	if isDark {
		t = ui.DarkTheme()
	}
	t.Font, t.FontSize = fontFamily, 16
	t.Background, t.Surface, t.Border = p.bg, p.sheet, p.line
	t.SurfaceHover, t.SurfacePressed = p.surface, p.raised
	t.Text, t.TextMuted = p.text, p.muted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText, t.InactiveAccent = p.accent, p.accent, p.accent, p.accentText, p.accent
	t.Danger = p.danger
	t.Selection = p.accent.Alpha(0.22)
	t.Focus = p.accent.Alpha(0.45)
	return t, isDark
}

func textTheme() *richtext.Theme {
	return &richtext.Theme{
		Text:                pal.text,
		Muted:               pal.muted,
		Link:                pal.accent,
		Caret:               pal.accent,
		Selection:           pal.accent.Alpha(0.22),
		CodeBackground:      pal.surface,
		CodeBlockBackground: pal.surface,
		ParagraphSpacing:    0.65,
	}
}

// Organic tileable cotton-paper grain bitmap, synthesized once lazily on first frame for 60 FPS GPU tiling.
var (
	grainOnce sync.Once
	grainBmp  *ui.Bitmap
)

const grainSize = 192

func paperGrainBitmap() *ui.Bitmap {
	grainOnce.Do(func() {
		img := image.NewNRGBA(image.Rect(0, 0, grainSize, grainSize))
		var state uint32 = 0x5EED1927
		next := func() uint32 {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			return state
		}
		rowFiber := make([]int, grainSize)
		colFiber := make([]int, grainSize)
		for i := 0; i < grainSize; i++ {
			rowFiber[i] = int(next()%34) - 17
			colFiber[i] = int(next()%24) - 12
		}
		for y := 0; y < grainSize; y++ {
			rf := rowFiber[y]
			for x := 0; x < grainSize; x++ {
				n1 := int(next() & 0x7F)
				n2 := int(next() & 0x7F)
				v := n1 + n2 + rf + colFiber[x]
				if v < 0 {
					v = 0
				} else if v > 255 {
					v = 255
				}
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(v)})
			}
		}
		grainBmp = ui.NewBitmap(img)
	})
	return grainBmp
}

func drawPaperGrain(p *ui.Painter, r ui.Rect) {
	bmp := paperGrainBitmap()
	if bmp == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	const tile = float32(grainSize)
	srcFull := ui.Rect{X: 0, Y: 0, W: tile, H: tile}
	opt := ui.ImageOptions{Tint: pal.grainTint}
	for y := r.Y; y < r.Y+r.H; y += tile {
		h := min(tile, r.Y+r.H-y)
		for x := r.X; x < r.X+r.W; x += tile {
			w := min(tile, r.X+r.W-x)
			src := srcFull
			if w < tile || h < tile {
				src = ui.Rect{X: 0, Y: 0, W: w, H: h}
			}
			p.ImageRegion(bmp, src, ui.Rect{X: x, Y: y, W: w, H: h}, opt)
		}
	}
}

const (
	side     = 26
	sheetPad = 20
	sheetR   = 28
	pressIn  = 0.965
)

func sheetAnim() ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.04}}
}

func fadeAnim() ui.Animation {
	return ui.Animation{Duration: 190 * time.Millisecond, Ease: ui.EaseOut}
}

func pressable(e ui.Element) ui.Element {
	target := float32(1)
	if e.Pressed() {
		target = pressIn
	}
	s := e.Animate("press", target, 110*time.Millisecond)
	return e.Scale(s, s)
}

func shadow(e ui.Element) ui.Element {
	a := float32(0.14)
	if dark {
		a = 0.48
	}
	return e.Shadow(0, 8, 28, -6, ui.RGB(0, 0, 0).Alpha(a))
}
