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

// The One Day palette: organic cotton paper with subtle grain that breathes gently with the hour of the day —
// cool moonlit linen at midnight, blush sunrise at dawn, crisp cotton by morning, golden parchment at sunset —
// paired with deep atmospheric charcoals in dark mode.

//go:embed fonts/Geist.ttf
var geistFont []byte

const fontFamily = "Geist"

func registerFonts() {
	if err := ui.RegisterFont(geistFont, fontFamily); err != nil {
		log.Printf("font %s: %v", fontFamily, err)
	}
}

type palette struct {
	bg         ui.Color // time-of-day living paper canvas
	sheet      ui.Color // elevated sheet surface
	surface    ui.Color // recessed pills and inputs
	raised     ui.Color // pressed surface
	line       ui.Color // hairline tracks and dividers
	text       ui.Color // primary editorial ink
	muted      ui.Color // secondary ink
	faint      ui.Color // whisper ink
	accent     ui.Color // living coral tuned to the hour
	accentSoft ui.Color // soft coral wash
	accentText ui.Color // foreground on accent
	danger     ui.Color // sync errors
	toastBg    ui.Color // dark toast pill background
	toastText  ui.Color // dark toast pill ink
	scrim      ui.Color // sheet backdrop dimming
	grainTint  ui.Color // organic paper grain overlay tint
}

type dayTone struct {
	hour   float32 // 0..24
	bg     ui.Color
	accent ui.Color
}

var (
	lightDayTones = []dayTone{
		{0.0, ui.Hex("#EFEEE8"), ui.Hex("#C85D4E")},  // midnight moonlit linen
		{6.0, ui.Hex("#FBF1E6"), ui.Hex("#E06646")},  // dawn sunrise blush
		{10.0, ui.Hex("#FCF9F2"), ui.Hex("#DF5A43")}, // morning crisp cotton
		{14.5, ui.Hex("#F8F3E6"), ui.Hex("#D9583E")}, // afternoon warm cream
		{18.5, ui.Hex("#F5E8D4"), ui.Hex("#CE5338")}, // golden hour parchment
		{21.5, ui.Hex("#EFEDE5"), ui.Hex("#CA5A4A")}, // evening quiet stone
	}
	darkDayTones = []dayTone{
		{0.0, ui.Hex("#0D0F15"), ui.Hex("#E87C6A")},  // midnight indigo inkstone
		{6.0, ui.Hex("#171316"), ui.Hex("#F2846A")},  // dawn warm plum-charcoal
		{10.0, ui.Hex("#121316"), ui.Hex("#F07A65")}, // morning crisp charcoal
		{14.5, ui.Hex("#151412"), ui.Hex("#ED7B5E")}, // afternoon warm graphite
		{18.5, ui.Hex("#1A1511"), ui.Hex("#F2855C")}, // golden hour espresso
		{21.5, ui.Hex("#101217"), ui.Hex("#EA7B68")}, // evening deep slate
	}
)

// dayPhaseName returns the quiet atmospheric name for the hour of now.
func dayPhaseName(now time.Time) string {
	switch h := now.Local().Hour(); {
	case h >= 5 && h < 8:
		return locales.DayDawn()
	case h >= 8 && h < 12:
		return locales.DayMorning()
	case h >= 12 && h < 17:
		return locales.DayAfternoon()
	case h >= 17 && h < 20:
		return locales.DaySunset()
	case h >= 20 && h < 23:
		return locales.DayEvening()
	default:
		return locales.DayMidnight()
	}
}

// paperToneAt smoothly interpolates the paper background and accent color for time now.
func paperToneAt(now time.Time, isDark bool) (bg, accent ui.Color) {
	stops := lightDayTones
	if isDark {
		stops = darkDayTones
	}
	t := now.Local()
	h := float32(t.Hour()) + float32(t.Minute())/60.0
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
			return cur.bg.MixOklab(next.bg, k), cur.accent.MixOklab(next.accent, k)
		}
	}
	return stops[0].bg, stops[0].accent
}

// paletteAt constructs the complete cohesive palette for time now and appearance isDark.
func paletteAt(now time.Time, isDark bool) palette {
	bg, accent := paperToneAt(now, isDark)
	if isDark {
		return palette{
			bg:         bg,
			sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.055),
			surface:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.10),
			raised:     bg.MixOklab(ui.Hex("#FFFFFF"), 0.15),
			line:       bg.MixOklab(ui.Hex("#FFFFFF"), 0.13),
			text:       ui.Hex("#F0ECE4"),
			muted:      ui.Hex("#9B968D"),
			faint:      ui.Hex("#625E57"),
			accent:     accent,
			accentSoft: accent.MixOklab(bg, 0.78),
			accentText: ui.Hex("#141010"),
			danger:     ui.Hex("#FF7A6D"),
			toastBg:    bg.MixOklab(ui.Hex("#FFFFFF"), 0.14),
			toastText:  ui.Hex("#F2EFE9"),
			scrim:      ui.Hex("#050608"),
			grainTint:  ui.Hex("#F5EFE4").Alpha(0.055),
		}
	}
	return palette{
		bg:         bg,
		sheet:      bg.MixOklab(ui.Hex("#FFFFFF"), 0.65),
		surface:    bg.MixOklab(ui.Hex("#1A1714"), 0.05),
		raised:     bg.MixOklab(ui.Hex("#1A1714"), 0.095),
		line:       bg.MixOklab(ui.Hex("#1A1714"), 0.11),
		text:       ui.Hex("#191716"),
		muted:      ui.Hex("#6B645B"),
		faint:      ui.Hex("#A29A8E"),
		accent:     accent,
		accentSoft: accent.MixOklab(bg, 0.85),
		accentText: ui.Hex("#FFFFFF"),
		danger:     ui.Hex("#C93E36"),
		toastBg:    ui.Hex("#1A191D"),
		toastText:  ui.Hex("#FAF7F2"),
		scrim:      ui.Hex("#141210"),
		grainTint:  ui.Hex("#3E3122").Alpha(0.068),
	}
}

var (
	pal  = paletteAt(time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local), false)
	dark bool
)

func themeOf(now time.Time, isDark bool) *ui.Theme {
	t := ui.LightTheme()
	if isDark {
		t = ui.DarkTheme()
	}
	pal, dark = paletteAt(now, isDark), isDark
	p := pal
	t.Font, t.FontSize = fontFamily, 16
	t.Background, t.Surface, t.Border = p.bg, p.sheet, p.line
	t.SurfaceHover, t.SurfacePressed = p.surface, p.raised
	t.Text, t.TextMuted = p.text, p.muted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText, t.InactiveAccent = p.accent, p.accent, p.accent, p.accentText, p.accent
	t.Danger = p.danger
	t.Selection = p.accent.Alpha(0.22)
	t.Focus = p.accent.Alpha(0.45)
	return t
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
		// Deterministic xorshift32 for fine speckle + soft cotton fiber weave.
		var state uint32 = 0x5EED1927
		next := func() uint32 {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			return state
		}
		// Subtle horizontal & vertical fiber bands so it reads as tactile paper, not harsh digital static.
		rowFiber := make([]int, grainSize)
		colFiber := make([]int, grainSize)
		for i := 0; i < grainSize; i++ {
			rowFiber[i] = int(next()%34) - 17
			colFiber[i] = int(next()%24) - 12
		}
		for y := 0; y < grainSize; y++ {
			rf := rowFiber[y]
			for x := 0; x < grainSize; x++ {
				// Triangular-ish fine grain distribution (sum of two uniform draws) for softer analog tooth.
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

// drawPaperGrain tiles the subtle cotton-paper texture cleanly across rect r using hardware texture sampling.
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
