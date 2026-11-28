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

// The Living Sky-Paper palette: a continuous, minute-by-minute atmospheric gradient anchored to real local sunrise
// and sunset — crisp airy sky-cotton in the morning, bright open daylight at noon, warm honey-amber golden hour in
// late afternoon, deep shadowed evening blue after dusk, and dark calm night — subtly modulated by live local weather
// (rainy slate, misty haze, overcast velvet, storm depth, snowy crispness) under organic cotton-paper grain.

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

type skyStop struct {
	hour   float32 // 0..24 continuous solar hour
	top    ui.Color
	bottom ui.Color
	accent ui.Color
}

// solarStops builds the 24-hour continuous gradient timeline anchored to local sunrise and sunset hours.
// Every single minute smoothly interpolates between neighboring stops in OKLab — zero abrupt jumps.
func solarStops(sunrise, sunset float32) []skyStop {
	midday := (sunrise + sunset) * 0.5
	morning := sunrise + (midday-sunrise)*0.48
	earlyAft := midday + (sunset-midday)*0.48
	golden := sunset - 0.85  // ~50 min before sunset: unmistakable warm golden hour gradient
	dusk := sunset + 0.35    // ~20 min after sunset: twilight horizon
	evening := sunset + 1.65 // rich deep shadowed evening blue
	lateNight := min(sunset+3.5, 23.0)
	preDawn := max(sunrise-1.4, 3.5)

	return []skyStop{
		{0.0, ui.Hex("#070910"), ui.Hex("#0F1320"), ui.Hex("#D97A66")},       // deep calm night
		{preDawn, ui.Hex("#0F162B"), ui.Hex("#241F36"), ui.Hex("#E88B6E")},   // pre-dawn horizon warmth
		{sunrise, ui.Hex("#DCE3F0"), ui.Hex("#FCE1C6"), ui.Hex("#DC5D3C")},   // dawn: cool sky meeting peach glow
		{morning, ui.Hex("#E4F2FA"), ui.Hex("#F9FEFC"), ui.Hex("#2173B8")},   // morning: crisp airy freshness
		{midday, ui.Hex("#F7FAFD"), ui.Hex("#FFFFFF"), ui.Hex("#2B62B5")},    // bright open daylight
		{earlyAft, ui.Hex("#FAF7EE"), ui.Hex("#FDECD5"), ui.Hex("#CE6228")},  // warm afternoon leaning gold
		{golden, ui.Hex("#F4DEC0"), ui.Hex("#F2B582"), ui.Hex("#BE4620")},    // golden hour: glowing warm gradient
		{dusk, ui.Hex("#243154"), ui.Hex("#7D4E57"), ui.Hex("#F09362")},      // sunset dusk: blue sky & rose horizon
		{evening, ui.Hex("#0F1A36"), ui.Hex("#1E3158"), ui.Hex("#7AA8F5")},   // evening: deep shadowed blue
		{lateNight, ui.Hex("#090D18"), ui.Hex("#12192A"), ui.Hex("#DC7D68")}, // settling into dark night
	}
}

// forcedLightStops keeps bright paper polarity 24/7 when user explicitly forces Light mode.
func forcedLightStops(sunrise, sunset float32) []skyStop {
	midday := (sunrise + sunset) * 0.5
	morning := sunrise + (midday-sunrise)*0.48
	golden := sunset - 0.85
	evening := sunset + 1.5
	return []skyStop{
		{0.0, ui.Hex("#E3E9F3"), ui.Hex("#EDF1F7"), ui.Hex("#476294")},     // moonlit cool paper
		{sunrise, ui.Hex("#E5DFEB"), ui.Hex("#FCE2C8"), ui.Hex("#DC5D3C")}, // dawn peach
		{morning, ui.Hex("#E4F2FA"), ui.Hex("#F9FEFC"), ui.Hex("#2173B8")}, // crisp fresh morning
		{midday, ui.Hex("#F7FAFD"), ui.Hex("#FFFFFF"), ui.Hex("#2B62B5")},  // bright daylight
		{golden, ui.Hex("#F5DFC2"), ui.Hex("#F2B886"), ui.Hex("#BE4620")},  // golden hour gradient
		{evening, ui.Hex("#DFE6F2"), ui.Hex("#ECEAF3"), ui.Hex("#4B64A0")}, // soft blue-dusk paper
	}
}

// forcedDarkStops keeps dark atmospheric polarity 24/7 when user explicitly forces Dark mode.
func forcedDarkStops(sunrise, sunset float32) []skyStop {
	midday := (sunrise + sunset) * 0.5
	morning := sunrise + (midday-sunrise)*0.48
	golden := sunset - 0.85
	evening := sunset + 1.5
	return []skyStop{
		{0.0, ui.Hex("#070910"), ui.Hex("#0F1320"), ui.Hex("#D97A66")},     // deep night
		{sunrise, ui.Hex("#1B1524"), ui.Hex("#342026"), ui.Hex("#F08A6C")}, // dawn rose-plum
		{morning, ui.Hex("#101F2B"), ui.Hex("#1A303E"), ui.Hex("#6AB2F0")}, // fresh deep fjord morning
		{midday, ui.Hex("#181D24"), ui.Hex("#222830"), ui.Hex("#82B2F5")},  // clear charcoal daylight
		{golden, ui.Hex("#241914"), ui.Hex("#3B2418"), ui.Hex("#F28855")},  // warm golden ember
		{evening, ui.Hex("#0F1A36"), ui.Hex("#1E3158"), ui.Hex("#7AA8F5")}, // deep shadowed evening blue
	}
}

func sampleStops(stops []skyStop, h float32) (top, bottom, accent ui.Color) {
	n := len(stops)
	for i := 0; i < n; i++ {
		cur := stops[i]
		next := stops[(i+1)%n]
		h0, h1 := cur.hour, next.hour
		if h1 <= h0 {
			h1 += 24
		}
		hh := h
		if hh < h0 {
			hh += 24
		}
		if hh >= h0 && hh <= h1 {
			k := (hh - h0) / (h1 - h0)
			// Smoothstep eases transitions organically around each solar anchor.
			k = k * k * (3 - 2*k)
			return cur.top.MixOklab(next.top, k), cur.bottom.MixOklab(next.bottom, k), cur.accent.MixOklab(next.accent, k)
		}
	}
	return stops[0].top, stops[0].bottom, stops[0].accent
}

// luminance computes perceptual relative luminance (0..1) of an sRGB ui.Color.
func luminance(c ui.Color) float32 {
	return (0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)) / 255.0
}

// applyWeather modulates the sky-paper gradient according to live weather conditions.
func applyWeather(top, bottom, accent ui.Color, wx weatherInfo, isDark bool) (ui.Color, ui.Color, ui.Color) {
	if !wx.HasWeather || wx.Kind == "" || wx.Kind == weatherClear {
		return top, bottom, accent
	}
	switch wx.Kind {
	case weatherPartly:
		if isDark {
			return top.MixOklab(ui.Hex("#18202C"), 0.16), bottom.MixOklab(ui.Hex("#1E2634"), 0.12), accent
		}
		return top.MixOklab(ui.Hex("#E3E9F0"), 0.22), bottom.MixOklab(ui.Hex("#F2F4F7"), 0.14), accent
	case weatherCloudy:
		if isDark {
			return top.MixOklab(ui.Hex("#191F28"), 0.42), bottom.MixOklab(ui.Hex("#242B36"), 0.34), accent.MixOklab(ui.Hex("#8FA4BD"), 0.35)
		}
		return top.MixOklab(ui.Hex("#D4DCE5"), 0.46), bottom.MixOklab(ui.Hex("#E7ECEF"), 0.38), accent.MixOklab(ui.Hex("#586E87"), 0.38)
	case weatherFog:
		if isDark {
			return top.MixOklab(ui.Hex("#1B2226"), 0.48), bottom.MixOklab(ui.Hex("#283035"), 0.44), accent.MixOklab(ui.Hex("#9BB0B5"), 0.35)
		}
		return top.MixOklab(ui.Hex("#DFE5E6"), 0.52), bottom.MixOklab(ui.Hex("#EEF1F1"), 0.48), accent.MixOklab(ui.Hex("#5A757B"), 0.38)
	case weatherRain:
		if isDark {
			return top.MixOklab(ui.Hex("#0E1826"), 0.52), bottom.MixOklab(ui.Hex("#19293D"), 0.46), accent.MixOklab(ui.Hex("#6EA5D8"), 0.55)
		}
		return top.MixOklab(ui.Hex("#BACADE"), 0.48), bottom.MixOklab(ui.Hex("#DCE5EE"), 0.42), accent.MixOklab(ui.Hex("#356896"), 0.55)
	case weatherStorm:
		if isDark {
			return top.MixOklab(ui.Hex("#090E18"), 0.62), bottom.MixOklab(ui.Hex("#182032"), 0.52), accent.MixOklab(ui.Hex("#8CA4E8"), 0.5)
		}
		return top.MixOklab(ui.Hex("#98A7BC"), 0.55), bottom.MixOklab(ui.Hex("#CCD5E0"), 0.45), accent.MixOklab(ui.Hex("#394E78"), 0.55)
	case weatherSnow:
		if isDark {
			return top.MixOklab(ui.Hex("#141C28"), 0.42), bottom.MixOklab(ui.Hex("#222D3D"), 0.38), accent.MixOklab(ui.Hex("#9EC8F5"), 0.5)
		}
		return top.MixOklab(ui.Hex("#ECF3FA"), 0.58), bottom.MixOklab(ui.Hex("#F9FCFF"), 0.55), accent.MixOklab(ui.Hex("#3D79B8"), 0.45)
	}
	return top, bottom, accent
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

// paperGradientAt returns the continuous minute-by-minute (bgTop, bgBottom, accent, isDark) for now, pref, and wx.
func paperGradientAt(now time.Time, pref string, wx weatherInfo) (top, bottom, accent ui.Color, isDark bool) {
	sunrise, sunset := wx.solarWindow(now)
	t := now.Local()
	h := float32(t.Hour()) + float32(t.Minute())/60.0 + float32(t.Second())/3600.0

	var stops []skyStop
	switch pref {
	case "light":
		stops = forcedLightStops(sunrise, sunset)
	case "dark":
		stops = forcedDarkStops(sunrise, sunset)
	default:
		stops = solarStops(sunrise, sunset)
	}
	top, bottom, accent = sampleStops(stops, h)

	mid := top.MixOklab(bottom, 0.5)
	isDark = luminance(mid) < 0.42
	if pref == "light" {
		isDark = false
	} else if pref == "dark" {
		isDark = true
	}
	top, bottom, accent = applyWeather(top, bottom, accent, wx, isDark)
	return top, bottom, accent, isDark
}

// isDarkFor reports whether the paper at now under pref & wx uses dark polarity (light text & light status bar icons).
func isDarkFor(now time.Time, pref string, wx weatherInfo) bool {
	_, _, _, isDark := paperGradientAt(now, pref, wx)
	return isDark
}

// paletteAt constructs the cohesive frame palette from the living gradient and weather.
func paletteAt(now time.Time, pref string, wx weatherInfo) (palette, bool) {
	bgTop, bgBottom, accent, isDark := paperGradientAt(now, pref, wx)
	bg := bgTop.MixOklab(bgBottom, 0.5)
	if isDark {
		ink := ui.Hex("#F1EFEA")
		return palette{
			bgTop:      bgTop,
			bgBottom:   bgBottom,
			bg:         bg,
			sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.075),
			surface:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.125),
			raised:     bg.MixOklab(ui.Hex("#FFFFFF"), 0.18),
			line:       bg.MixOklab(ui.Hex("#FFFFFF"), 0.16),
			text:       ink,
			muted:      ink.MixOklab(bg, 0.34),
			faint:      ink.MixOklab(bg, 0.54),
			accent:     accent,
			accentSoft: accent.MixOklab(bg, 0.74),
			accentText: ui.Hex("#121214"),
			danger:     ui.Hex("#FF7A6D"),
			toastBg:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.18),
			toastText:  ink,
			scrim:      ui.Hex("#05070D"),
			grainTint:  ui.Hex("#F5F1E8").Alpha(0.055),
		}, true
	}
	ink := ui.Hex("#18191D").MixOklab(bgTop, 0.08)
	return palette{
		bgTop:      bgTop,
		bgBottom:   bgBottom,
		bg:         bg,
		sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.66),
		surface:    bg.MixOklab(ink, 0.065),
		raised:     bg.MixOklab(ink, 0.115),
		line:       bg.MixOklab(ink, 0.135),
		text:       ink,
		muted:      ink.MixOklab(bg, 0.38),
		faint:      ink.MixOklab(bg, 0.56),
		accent:     accent,
		accentSoft: accent.MixOklab(bg, 0.80),
		accentText: ui.Hex("#FFFFFF"),
		danger:     ui.Hex("#C93E36"),
		toastBg:    ink,
		toastText:  bg,
		scrim:      ui.Hex("#121318"),
		grainTint:  ink.Alpha(0.062),
	}, false
}

var (
	pal  palette
	dark bool
)

func init() {
	pal, dark = paletteAt(time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local), "", weatherInfo{})
}

func themeOf(now time.Time, pref string, wx weatherInfo) (*ui.Theme, bool) {
	p, isDark := paletteAt(now, pref, wx)
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
