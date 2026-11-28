package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The sky behind Today's paper: one family of colors across the day, anchored to the real sunrise and sunset,
// that each weather turns to a mood of its own (a hue, a saturation and a lightness in OKLCH) instead of washing it
// toward grey. Over the colors lie the sky's lights and weather: the sun's glow on its arc, the moon in its real
// phase and the stars by night, soft clouds, a band of mist, fine rain, slow snow, a far flash in a storm.

// sky is what the paper's sky looks like at one moment: its colors, its polarity and the lights and weather drawn
// over them.
type sky struct {
	top, bottom ui.Color // the zenith and the horizon of the gradient
	accent      ui.Color
	dark        bool
	kind        weatherKind // the weather drawn, "" when none is known

	sunX, sunY float32  // the sun's place on its arc, in fractions of the sky's box
	sunGlow    ui.Color // its light
	sunA       float32  // how strong its light is, 0 at night and under heavy cloud
	moonA      float32  // how much the moon shows, 0 by day
	starsA     float32  // how much the stars show, 0 by day and under cloud
	moonAge    float32  // the moon's phase, 0 new, 0.5 full, 1 new again
}

// skyAnchor is the clear sky at one moment of the day; the sky between two anchors is mixed in OKLab.
type skyAnchor struct {
	hour                      float32
	top, bottom, accent, glow ui.Color
	glowA                     float32
}

// skyAnchors builds the clear day's anchors around the local sunrise and sunset hours.
func skyAnchors(sunrise, sunset float32) []skyAnchor {
	midday := (sunrise + sunset) * 0.5
	morning := sunrise + (midday-sunrise)*0.45
	afternoon := midday + (sunset-midday)*0.5
	return []skyAnchor{
		{0, ui.Hex("#090D1B"), ui.Hex("#141A2E"), ui.Hex("#E8B978"), ui.Hex("#C8D4FF"), 0},                                 // night: ink-blue, a candle's accent
		{max(sunrise-1.1, 3), ui.Hex("#10173A"), ui.Hex("#3A2E57"), ui.Hex("#F2A88A"), ui.Hex("#F4A285"), 0.32},            // before dawn: violet, a warm rim
		{sunrise + 0.05, ui.Hex("#C9D1EC"), ui.Hex("#F8D7C5"), ui.Hex("#C65A45"), ui.Hex("#FFC29A"), 0.62},                 // dawn: periwinkle over peach
		{morning, ui.Hex("#B4D6F3"), ui.Hex("#F2F7F5"), ui.Hex("#2C6FBE"), ui.Hex("#FFF0CC"), 0.5},                         // morning: washed blue, fresh
		{midday, ui.Hex("#A3CBF0"), ui.Hex("#F3F7FA"), ui.Hex("#2A60C2"), ui.Hex("#FFFBE8"), 0.55},                         // noon: open sky, white light
		{afternoon, ui.Hex("#B1CDEA"), ui.Hex("#FAEEDA"), ui.Hex("#AE5E2A"), ui.Hex("#FFE3B4"), 0.55},                      // afternoon: the light turns warm
		{max(sunset-0.8, afternoon+0.1), ui.Hex("#C3C4DD"), ui.Hex("#F7C08C"), ui.Hex("#AE4520"), ui.Hex("#FFA862"), 0.72}, // golden hour: lilac over honey
		{sunset, ui.Hex("#BAB6D6"), ui.Hex("#F4AE92"), ui.Hex("#AC4038"), ui.Hex("#FF9670"), 0.68},                         // sunset: lavender over coral
		{sunset + 0.5, ui.Hex("#1A2046"), ui.Hex("#5B3C5E"), ui.Hex("#F49C7C"), ui.Hex("#EE8468"), 0.4},                    // dusk: indigo over plum
		{min(sunset+1.6, 23.7), ui.Hex("#0D1530"), ui.Hex("#1C2950"), ui.Hex("#F1A56E"), ui.Hex("#9FB4F0"), 0},             // evening: deep blue, lamps on
		{min(sunset+3.5, 23.9), ui.Hex("#090D1B"), ui.Hex("#141A2E"), ui.Hex("#E8B978"), ui.Hex("#C8D4FF"), 0},             // night again
	}
}

// mood is how one weather turns the clear sky. Each color keeps part of its own tint (keep) and takes the
// weather's (tint), mixed in OKLab so a warm sky cools through linen and slate, never through green or pink. The
// zenith takes the whole mood; the horizon only part of it (horizon), so the hour's warmth still shows low down.
type mood struct {
	tintH, tintC  float64 // the weather's own tint: a hue in degrees and a chroma
	keep          float64 // how much of the clear sky's tint is kept
	horizon       float64 // how much of the mood the horizon takes, 0..1
	lLight, lDark float64 // lightness moved on a light and on a dark sky
	flatten       float64 // how far the zenith and the horizon come to the same lightness
	glow          float32 // how much of the sun's light comes through
	stars         float32 // how much of the stars show
	accH, accC    float64 // the accent's tint
	accKeep       float64 // how much of its own tint the accent keeps; 0 leaves the accent as it is
	heavy         bool    // the sky darkens even by day, as under a storm
}

func moodOf(k weatherKind) mood {
	switch k {
	case weatherPartly:
		return mood{tintH: 235, tintC: 0.006, keep: 0.88, horizon: 0.7, lLight: 0.005, flatten: 0.1, glow: 0.75, stars: 0.55}
	case weatherCloudy:
		return mood{tintH: 250, tintC: 0.02, keep: 0.35, horizon: 0.75, lLight: -0.02, lDark: 0.03, flatten: 0.5, glow: 0.16,
			accH: 250, accC: 0.05, accKeep: 0.6}
	case weatherFog:
		return mood{tintH: 222, tintC: 0.014, keep: 0.3, horizon: 0.9, lLight: 0.015, lDark: 0.04, flatten: 0.8, glow: 0.42,
			accH: 215, accC: 0.07, accKeep: 0.4}
	case weatherRain:
		return mood{tintH: 245, tintC: 0.04, keep: 0.4, horizon: 0.55, lLight: -0.045, lDark: -0.005, flatten: 0.25, glow: 0.08,
			accH: 240, accC: 0.09, accKeep: 0.35}
	case weatherStorm:
		return mood{tintH: 275, tintC: 0.05, keep: 0.45, horizon: 0.85, lLight: -0.1, lDark: 0.02, flatten: 0.2,
			accH: 290, accC: 0.1, accKeep: 0.35, heavy: true}
	case weatherSnow:
		return mood{tintH: 230, tintC: 0.025, keep: 0.45, horizon: 0.7, lLight: -0.015, lDark: 0.05, flatten: 0.4, glow: 0.38,
			accH: 225, accC: 0.08, accKeep: 0.4}
	}
	return mood{keep: 1, horizon: 1, glow: 1, stars: 1}
}

// The lightness each polarity keeps, so the ink on it always reads: a light sky is never darker than lightMin, a
// dark one never lighter than darkMax, and the minutes around sunrise and sunset turn over quickly between them.
const (
	lightMin  = 0.82
	darkMax   = 0.4
	darkBelow = 0.6 // a sky whose mean lightness is under this is dark
)

// skyAt returns the sky at now under the appearance pref ("", "light", "dark") and the weather wx.
func skyAt(now time.Time, pref string, wx weatherInfo) sky {
	sunrise, sunset := wx.solarWindow(now)
	t := now.Local()
	h := float32(t.Hour()) + float32(t.Minute())/60 + float32(t.Second())/3600

	anchors := skyAnchors(sunrise, sunset)
	a, b, k := bracket(anchors, h)
	top := a.top.MixOklab(b.top, k)
	bottom := a.bottom.MixOklab(b.bottom, k)
	accent := a.accent.MixOklab(b.accent, k)
	glow := a.glow.MixOklab(b.glow, k)
	glowA := a.glowA + (b.glowA-a.glowA)*k

	kind := weatherKind("")
	if wx.HasWeather {
		kind = wx.Kind
	}
	m := moodOf(kind)
	tl, bl := toLCH(top), toLCH(bottom)
	dark := (tl.L+bl.L)/2 < darkBelow
	dayLight := !dark
	switch pref {
	case "light":
		if dark {
			tl, bl = lighten(tl), lighten(bl)
		}
		dark = false
	case "dark":
		if !dark {
			tl, bl = darken(tl), darken(bl)
		}
		dark = true
	default:
		if m.heavy && !dark {
			tl, bl = overcast(tl), overcast(bl)
			dark = true
		}
	}
	tl, bl = m.apply(tl, bl, dark)
	if pref == "" {
		dark = (tl.L+bl.L)/2 < darkBelow
	}
	tl, bl = keepPolarity(tl, dark), keepPolarity(bl, dark)

	al := toLCH(accent)
	if m.accKeep > 0 {
		al = al.tinted(m.accKeep, m.accH, m.accC)
		al.C = max(al.C, 0.07)
	}
	if dark {
		al.L = clamp(al.L, 0.76, 0.86)
	} else {
		al.L = clamp(al.L, 0.44, 0.58)
	}

	s := sky{top: tl.color(), bottom: bl.color(), dark: dark, kind: kind, sunGlow: glow}
	s.accent = reach(al, s, 3)

	// The sun walks an arc from the horizon at the bottom left, high over the middle at noon, to the bottom right.
	day := (h - sunrise) / max(sunset-sunrise, 1)
	arc := clamp32(day, 0, 1)
	s.sunX = 0.14 + 0.72*arc
	s.sunY = 1.02 - 0.92*float32(math.Sin(math.Pi*float64(arc)))
	lit := ramp(h, sunrise-1.4, sunrise-0.8) * (1 - ramp(h, sunset+1.0, sunset+1.5))
	s.sunA = glowA * m.glow * lit
	if dark && dayLight {
		s.sunA *= 0.3 // a dark paper by day keeps only a hint of the sun
	}
	night := ramp(h, sunset+0.6, sunset+1.8) + (1 - ramp(h, sunrise-1.6, sunrise-0.5))
	night = clamp32(night, 0, 1)
	if dark && pref == "dark" && night == 0 {
		night = 0.35 // a dark paper by day keeps a faint moon and stars
	}
	if !dark {
		night = 0
	}
	s.starsA = night * m.stars
	s.moonA = night * min(1, m.stars*1.4+0.15)
	s.moonAge = moonAge(now)
	return s
}

// bracket returns the anchors around hour h and how far h is between them, eased.
func bracket(anchors []skyAnchor, h float32) (skyAnchor, skyAnchor, float32) {
	n := len(anchors)
	for i := 0; i < n; i++ {
		cur, next := anchors[i], anchors[(i+1)%n]
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
			return cur, next, k * k * (3 - 2*k)
		}
	}
	return anchors[0], anchors[0], 0
}

// apply turns a clear sky's zenith and horizon into this weather's.
func (m mood) apply(top, bottom lch, dark bool) (lch, lch) {
	mean := (top.L + bottom.L) / 2
	shift := m.lLight
	if dark {
		shift = m.lDark
	}
	turn := func(c lch, k float64) lch {
		w := c.tinted(m.keep, m.tintH, m.tintC)
		w.L += (mean-c.L)*m.flatten + shift
		return c.mix(w, k)
	}
	return turn(top, 1), turn(bottom, m.horizon)
}

// tinted keeps part of c's tint and adds the tint of hue h and chroma ch, in OKLab.
func (c lch) tinted(keep, h, ch float64) lch {
	a, b := c.ab()
	a = a*keep + ch*math.Cos(h*math.Pi/180)
	b = b*keep + ch*math.Sin(h*math.Pi/180)
	return fromLab(c.L, a, b)
}

// mix mixes c toward o by k, in OKLab.
func (c lch) mix(o lch, k float64) lch {
	a0, b0 := c.ab()
	a1, b1 := o.ab()
	return fromLab(c.L+(o.L-c.L)*k, a0+(a1-a0)*k, b0+(b1-b0)*k)
}

func (c lch) ab() (a, b float64) {
	return c.C * math.Cos(c.H*math.Pi/180), c.C * math.Sin(c.H*math.Pi/180)
}

func fromLab(l, a, b float64) lch {
	h := math.Atan2(b, a) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return lch{l, math.Hypot(a, b), h}
}

// lighten turns a night color into a moonlit paper of the same hue, for a light appearance at night.
func lighten(c lch) lch {
	c.L = 0.87 + c.L*0.24
	c.C *= 0.55
	return c
}

// darken turns a day color into a deep sky of the same hue, for a dark appearance by day.
func darken(c lch) lch {
	c.L = 0.16 + max(c.L-0.8, 0)*0.6
	c = c.tinted(0.45, 262, 0.035) // toward the night's blue, so a warm hour turns ember-dark, not brown
	return c
}

// overcast turns a day color into the dim sky under a storm: darker than a dark appearance's day, as a storm
// still lets some light through.
func overcast(c lch) lch {
	c.L = 0.25 + max(c.L-0.8, 0)*0.7
	c.C *= 0.7
	return c
}

// keepPolarity keeps c on its side of the lightness its ink needs.
func keepPolarity(c lch, dark bool) lch {
	if dark {
		c.L = min(c.L, darkMax)
	} else {
		c.L = max(c.L, lightMin)
	}
	return c
}

// moonAge returns the moon's phase at t, 0 at new moon and 0.5 at full, from a known new moon and the mean
// synodic month.
func moonAge(t time.Time) float32 {
	const synodic = 29.530588853 * 24 * float64(time.Hour)
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	f := math.Mod(float64(t.Sub(ref)), synodic) / synodic
	if f < 0 {
		f++
	}
	return float32(f)
}

func ramp(h, from, to float32) float32 { return clamp32((h-from)/(to-from), 0, 1) }

func clamp32(v, lo, hi float32) float32 { return max(lo, min(hi, v)) }

func clamp(v, lo, hi float64) float64 { return max(lo, min(hi, v)) }

// lch is a color in OKLCH: lightness 0..1, chroma, hue in degrees.
type lch struct{ L, C, H float64 }

func srgbToLinear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linearToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func toLCH(c ui.Color) lch {
	r, g, b := srgbToLinear(c.R), srgbToLinear(c.G), srgbToLinear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	h := math.Atan2(B, A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return lch{L, math.Hypot(A, B), h}
}

// linear returns c in linear sRGB, and whether it is inside the sRGB gamut.
func (c lch) linear() (r, g, b float64, ok bool) {
	A := c.C * math.Cos(c.H*math.Pi/180)
	B := c.C * math.Sin(c.H*math.Pi/180)
	l := c.L + 0.3963377774*A + 0.2158037573*B
	m := c.L - 0.1055613458*A - 0.0638541728*B
	s := c.L - 0.0894841775*A - 1.2914855480*B
	l, m, s = l*l*l, m*m*m, s*s*s
	r = 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g = -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	b = -0.0041960863*l - 0.7034186147*m + 1.7076953130*s
	const e = 1e-4
	ok = r >= -e && r <= 1+e && g >= -e && g <= 1+e && b >= -e && b <= 1+e
	return
}

// color returns c in sRGB, its chroma lowered until it fits so its hue and lightness stay.
func (c lch) color() ui.Color {
	c.L = clamp(c.L, 0, 1)
	r, g, b, ok := c.linear()
	if !ok {
		lo, hi := 0.0, c.C
		for i := 0; i < 18; i++ {
			c.C = (lo + hi) / 2
			if _, _, _, in := c.linear(); in {
				lo = c.C
			} else {
				hi = c.C
			}
		}
		c.C = lo
		r, g, b, _ = c.linear()
	}
	to := func(v float64) uint8 { return uint8(math.Round(clamp(linearToSRGB(clamp(v, 0, 1)), 0, 1) * 255)) }
	return ui.RGB(to(r), to(g), to(b))
}

// relLum is a color's relative luminance as WCAG counts it.
func relLum(c ui.Color) float64 {
	return 0.2126*srgbToLinear(c.R) + 0.7152*srgbToLinear(c.G) + 0.0722*srgbToLinear(c.B)
}

// contrast is the WCAG contrast ratio of two colors, 1 to 21.
func contrast(a, b ui.Color) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance is an sRGB color's rough perceptual lightness, 0..1, by its gamma-encoded channels.
func luminance(c ui.Color) float32 {
	return (0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)) / 255.0
}

// reach returns ink c, moved away from the sky's lightness only as far as it must go to stand at least ratio
// against both ends of sky s, so each ink keeps its designed tone wherever that already reads.
func reach(c lch, s sky, ratio float64) ui.Color {
	reads := func(c lch) bool {
		col := c.color()
		return contrast(col, s.top) >= ratio && contrast(col, s.bottom) >= ratio
	}
	if reads(c) {
		return c.color()
	}
	step := -0.01
	if s.dark {
		step = 0.01
	}
	for i := 0; i < 80 && !reads(c); i++ {
		c.L = clamp(c.L+step, 0, 1)
	}
	return c.color()
}
