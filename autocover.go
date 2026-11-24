package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

// An automatic cover: a page with no cover of its own gets one made from its seed - the
// time it was made and the name of its folder, which do not change - so that each page has
// a cover of its own, and the same each time. Nothing is stored and nothing is a file.
//
// It is a landscape: mountains, a lake under them, the sea, a forest, dunes and their
// pyramids, a lighthouse, islands, a beach seen from above (and in autocover_scenes.go a river,
// space and a sea under the moon) - in the light of an hour the seed picks too: dawn, morning, noon,
// a golden afternoon or dusk in the light theme; twilight, night or a moonlit night in the
// dark, the moon in its phase of today. The picture is a print of two DIPs to a pixel,
// ordered-dithered, as an old game's; over it, now and then, what lives in it moves, slowly
// and few frames a second: clouds drift, stars twinkle, birds pass, snow falls, the
// lighthouse sweeps, fireflies wander, the sea glints.

const (
	autoW     = 2240 // the picture's width, in DIPs: more than a window is wide
	autoBandH = 184  // the band's height: the room, the title, the foot
	autoRoom  = 100  // above the title, in the band

	// the cover begins at the top of the window, under the title bar, so its picture is
	// as tall as the bar and the band
	autoFullH = autoBandH + barH

	autoPx = 2 // DIPs to a pixel of the picture
	picW   = autoW / autoPx
	picH   = autoFullH / autoPx
	picHz  = picH * 54 / 100 // the horizon, about where the title begins
)

// the landscapes
const (
	sceneMountains = iota
	sceneLake
	sceneSea
	sceneForest
	sceneDesert
	sceneLighthouse
	sceneIslands
	sceneBeach
	sceneRiver // the rest are in autocover_scenes.go
	sceneSpace
	sceneMoonSea
	sceneKinds
)

// the kinds of cover a page shows
const (
	coverKindNone = iota
	coverKindImage
	coverKindAuto
)

// coverKind is what covers page p: its own picture, the made landscape (unless the page said
// none, or the setting is off), or nothing.
func (a *app) coverKind(p *Page) int {
	switch {
	case p == nil:
		return coverKindNone
	case p.ImageCover():
		return coverKindImage
	case p.Cover == CoverNone || !a.settings.AutoCover:
		return coverKindNone
	}
	return coverKindAuto
}

// coverBandOf is the band's height for page p, and coverRoomOf the room above its title.
// The cover begins at the top of the window, under the title bar (the bar is clear glass over
// it), so both count the bar's height; the title stands where it would below a bar.
func (a *app) coverBandOf(p *Page) float32 {
	switch a.coverKind(p) {
	case coverKindAuto:
		return autoFullH
	case coverKindImage:
		return a.coverTopOf() + a.coverPicH(p)
	}
	return 0
}

func (a *app) coverRoomOf(p *Page) float32 {
	switch a.coverKind(p) {
	case coverKindAuto:
		return autoRoom + barH
	case coverKindImage:
		return a.coverTopOf() + a.coverPicH(p) + coverGapT
	}
	return 0
}

// coverSeed is what makes a page's cover its own.
func coverSeed(p *Page) uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.Created.UTC().Format("2006-01-02T15:04:05.000000000")))
	h.Write([]byte{0})
	h.Write([]byte(filepath.Base(p.Dir)))
	return h.Sum64()
}

// splitmix is the seed's stream of numbers: the same seed, the same ones.
type splitmix uint64

func (s *splitmix) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// f is the next number, from 0 up to 1.
func (s *splitmix) f() float64 { return float64(s.next()>>11) / (1 << 53) }

// in is the next number from lo up to hi.
func (s *splitmix) in(lo, hi float64) float64 { return lo + (hi-lo)*s.f() }

// valueNoise is lattice noise at (x, y), smooth, 0..1, from the seed.
func valueNoise(seed uint64, x, y float64) float64 {
	hash := func(ix, iy int) float64 {
		h := splitmix(seed ^ uint64(int64(ix))*0x9e3779b185ebca87 ^ uint64(int64(iy))*0xc2b2ae3d27d4eb4f)
		return h.f()
	}
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(ix), y-float64(iy)
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := hash(ix, iy), hash(ix+1, iy)
	c, d := hash(ix, iy+1), hash(ix+1, iy+1)
	return a + (b-a)*sx + (c-a)*sy + (a-b-c+d)*sx*sy
}

// fbm is noise of a few octaves along a line, 0..1: a ridge's outline.
func fbm(seed uint64, x float64, octaves int) float64 {
	v, amp, sum := 0.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		v += amp * valueNoise(seed+uint64(i)*131, x, float64(i)*7.3)
		sum += amp
		x *= 2.03
		amp *= 0.5
	}
	return v / sum
}

// A cover theme is the hue an app theme's covers lean toward (h1; h2 and sat are what its
// accent gives besides): the sky of every page's landscape takes a little of it.
type coverTheme struct {
	name   string
	h1, h2 float64 // hue, of 360
	sat    float64
}

// rgb is a color of the picture, 0..255 a channel, as it is mixed before it is dithered.
type rgb [3]float64

func hexRGB(s string) rgb {
	c := ui.Hex(s)
	return rgb{float64(c.R), float64(c.G), float64(c.B)}
}

func rgbOf(c color.RGBA) rgb { return rgb{float64(c.R), float64(c.G), float64(c.B)} }

func (a rgb) mix(b rgb, t float64) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// mul lights a by b, as a surface of color a in light of color b.
func (a rgb) mul(b rgb) rgb { return rgb{a[0] * b[0] / 255, a[1] * b[1] / 255, a[2] * b[2] / 255} }

func (a rgb) ui(alpha float64) ui.Color {
	c8 := func(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }
	return ui.RGBA(c8(a[0]), c8(a[1]), c8(a[2]), float32(alpha))
}

// An hour's light: the sky from its top to the horizon, the sun or the moon, the light that
// falls on the land, and the haze far things go into.
type coverLight struct {
	name                 string
	top, mid, low        rgb
	sun                  rgb
	lit                  rgb  // the land is its own color in this light
	night                bool // the moon and the stars, lit windows
	sunLow               bool // the sun near the horizon: long light, a path on the water
	cloudLit, cloudShade rgb
}

var dayLights = []coverLight{
	{"Şafak", hexRGB("#7f8fd0"), hexRGB("#e7b3c6"), hexRGB("#fcd9b6"), hexRGB("#fff0d4"), hexRGB("#f0d8d6"), false, true, hexRGB("#ffe7de"), hexRGB("#c99db6")},
	{"Sabah", hexRGB("#5d9fe6"), hexRGB("#a8d2f4"), hexRGB("#e6f3fb"), hexRGB("#fffbea"), hexRGB("#ffffff"), false, false, hexRGB("#ffffff"), hexRGB("#c4d7ea")},
	{"Öğle", hexRGB("#3f8de6"), hexRGB("#88c1f1"), hexRGB("#d6ebfa"), hexRGB("#ffffff"), hexRGB("#ffffff"), false, false, hexRGB("#ffffff"), hexRGB("#bcd2ea")},
	{"İkindi", hexRGB("#5a92d4"), hexRGB("#b9d4e9"), hexRGB("#f7e3b9"), hexRGB("#fff0c4"), hexRGB("#fbe9c8"), false, false, hexRGB("#fff6e2"), hexRGB("#d2c3b4")},
	{"Akşam", hexRGB("#4c4f9c"), hexRGB("#d47c9e"), hexRGB("#f9b679"), hexRGB("#ffd690"), hexRGB("#c99aa6"), false, true, hexRGB("#ffc9a2"), hexRGB("#8e6a96")},
}

var nightLights = []coverLight{
	{"Alacakaranlık", hexRGB("#151b3e"), hexRGB("#3b326e"), hexRGB("#94557a"), hexRGB("#ffd9a8"), hexRGB("#5a4a78"), true, false, hexRGB("#6d5d93"), hexRGB("#2c2650")},
	{"night", hexRGB("#060a1b"), hexRGB("#0f193a"), hexRGB("#1f2e58"), hexRGB("#f3f1e2"), hexRGB("#34406a"), true, false, hexRGB("#3a4672"), hexRGB("#141c3a")},
	{"Mehtap", hexRGB("#0b1530"), hexRGB("#1b2d57"), hexRGB("#3a5585"), hexRGB("#fbf8e8"), hexRGB("#5a6e9a"), true, false, hexRGB("#6f83ad"), hexRGB("#26365e")},
}

// the picture, as it is drawn: a pixel's color, mixed
type canvas struct {
	w, h int
	px   []rgb
}

func newCanvas(w, h int) *canvas { return &canvas{w: w, h: h, px: make([]rgb, w*h)} }

func (cv *canvas) set(x, y int, c rgb, a float64) {
	if x < 0 || y < 0 || x >= cv.w || y >= cv.h || a <= 0 {
		return
	}
	p := &cv.px[y*cv.w+x]
	if a >= 1 {
		*p = c
		return
	}
	*p = p.mix(c, a)
}

func (cv *canvas) get(x, y int) rgb {
	x, y = max(0, min(cv.w-1, x)), max(0, min(cv.h-1, y))
	return cv.px[y*cv.w+x]
}

// rect fills from (x0, y0) up to (x1, y1).
func (cv *canvas) rect(x0, y0, x1, y1 float64, c rgb, a float64) {
	for y := int(math.Round(y0)); y < int(math.Round(y1)); y++ {
		for x := int(math.Round(x0)); x < int(math.Round(x1)); x++ {
			cv.set(x, y, c, a)
		}
	}
}

// disc is a round thing of radius r, its edge soft by a pixel.
func (cv *canvas) disc(cx, cy, r float64, c rgb, a float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			cv.set(x, y, c, a*clamp01(r-d+0.5))
		}
	}
}

// glow is light round a point, falling off to nothing at r.
func (cv *canvas) glow(cx, cy, r float64, c rgb, a float64) {
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) / r
			if d < 1 {
				cv.set(x, y, c, a*(1-d)*(1-d))
			}
		}
	}
}

// land fills below the outline top(x), from its color at the outline to bottom's at the
// foot, its edge soft.
func (cv *canvas) land(top func(x float64) float64, c, bottom rgb) {
	for x := 0; x < cv.w; x++ {
		t := top(float64(x) + 0.5)
		for y := max(0, int(t)); y < cv.h; y++ {
			k := clamp01((float64(y) - t) / math.Max(1, float64(cv.h)-t))
			cv.set(x, y, c.mix(bottom, k), clamp01(float64(y)+1-t))
		}
	}
}

// tri is a pine, or anything that narrows upward: its foot's middle at (cx, by).
func (cv *canvas) tri(cx, by, w, h float64, c rgb) {
	for y := int(by - h); y < int(by); y++ {
		k := (by - float64(y)) / h // 1 at the tip
		hw := w / 2 * (1 - k)
		cv.rect(cx-hw, float64(y), cx+hw+0.5, float64(y+1), c, 1)
	}
}

// pine is a tree of two tiers, on a trunk.
func (cv *canvas) pine(cx, by, h float64, c rgb) {
	cv.rect(cx-0.5, by-h*0.25, cx+0.5, by, c.mix(rgb{40, 28, 20}, 0.4), 1)
	cv.tri(cx, by-h*0.15, h*0.62, h*0.6, c)
	cv.tri(cx, by-h*0.45, h*0.45, h*0.55, c)
}

// sky is the sky's gradient, from the top to the horizon and on below it.
func (cv *canvas) sky(l *coverLight, tint rgb, tintA float64) {
	for y := 0; y < cv.h; y++ {
		k := float64(y) / picHz
		var c rgb
		if k < 0.55 {
			c = l.top.mix(l.mid, smooth(0, 0.55, k))
		} else {
			c = l.mid.mix(l.low, smooth(0.55, 1, math.Min(k, 1)))
		}
		c = c.mix(tint, tintA*(1-math.Min(k, 1)))
		for x := 0; x < cv.w; x++ {
			cv.px[y*cv.w+x] = c
		}
	}
}

// moon is the moon in phase ph (0 new, 0.5 full), lit from the right as it waxes.
func (cv *canvas) moon(cx, cy, r, ph float64, c, sky rgb) {
	cv.glow(cx, cy, r*4, c, 0.18)
	lit := math.Cos(ph * 2 * math.Pi) // 1 new, -1 full: where the terminator lies
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			if d > r+0.5 {
				continue
			}
			edge := clamp01(r - d + 0.5)
			half := math.Sqrt(math.Max(0, r*r-dy*dy))
			u := dx / math.Max(half, 0.01) // -1..1 across the disc at this row
			if ph > 0.5 {
				u = -u // waning: lit from the left
			}
			on := clamp01((u - lit) * half * 0.6)
			col := sky.mix(c, 0.12).mix(c, on)
			if on > 0.5 && valueNoise(77, float64(x)*0.5, float64(y)*0.5) > 0.62 { // seas on it
				col = col.mix(sky, 0.18)
			}
			cv.set(x, y, col, edge)
		}
	}
}

// moonPhase is the moon's phase on day t: 0 new, 0.5 full.
func moonPhase(t time.Time) float64 {
	const synodic = 29.530588853
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC) // a new moon
	d := t.Sub(ref).Hours() / 24
	return math.Mod(math.Mod(d, synodic)+synodic, synodic) / synodic
}

// bayer8 is the ordered dither's threshold map.
var bayer8 = func() (m [64]float64) {
	b := [64]int{
		0, 32, 8, 40, 2, 34, 10, 42, 48, 16, 56, 24, 50, 18, 58, 26,
		12, 44, 4, 36, 14, 46, 6, 38, 60, 28, 52, 20, 62, 30, 54, 22,
		3, 35, 11, 43, 1, 33, 9, 41, 51, 19, 59, 27, 49, 17, 57, 25,
		15, 47, 7, 39, 13, 45, 5, 37, 63, 31, 55, 23, 61, 29, 53, 21,
	}
	for i, v := range b {
		m[i] = (float64(v)+0.5)/64 - 0.5
	}
	return
}()

// ditherLevels is how many steps a channel has after the dither: few enough to show it.
const ditherLevels = 14

// image is the canvas, dithered.
func (cv *canvas) image() *image.RGBA {
	w, h := cv.w, cv.h
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	step := 255.0 / (ditherLevels - 1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cv.px[y*cv.w+x]
			o := img.PixOffset(x, y)
			th := bayer8[(y%8)*8+x%8]
			for i := 0; i < 3; i++ {
				v := math.Round(c[i]/step+th) * step
				img.Pix[o+i] = uint8(math.Max(0, math.Min(255, v)))
			}
			img.Pix[o+3] = 255
		}
	}
	return img
}

// autoCover is the landscape of a page: what it is, its picture, and what moves in it.
type autoCover struct {
	scene int
	light int // of dayLights, or of nightLights in the dark
	dark  bool
	img   *ui.Bitmap

	// what moves, drawn into lifeCv at the step lifeTick of coverFPS a second, and shown as lifeBm
	lifeCv   *ui.Canvas
	lifeBm   *ui.Bitmap
	lifeTick float64

	// what moves, in the picture's pixels
	clouds    []coverCloud
	stars     []coverStar
	birds     bool
	snow      bool
	glint     bool         // the sea glints where the light lies on it
	glints    [][2]float64 // where
	beam      [2]float64
	shore     []float64 // the beach's: where the dry sand meets the water, a column's
	sand      rgb       // the wet sand the waves run up
	shallow   rgb
	fireflies bool
	shooting  bool
	seed      uint64

	season    int       // of the day it is made: 0 spring, 1 summer, 2 autumn, 3 winter
	green     bool      // it has green in it, that the season turns
	noSky     bool      // seen from above, or under water: no sky, no clouds, no stars
	noWeather bool      // no snow, leaves or petals of the season
	ownSky    bool      // its scene paints its own sky, and its own stars
	leaves    bool      // autumn's leaves fall
	petals    bool      // spring's petals drift
	life      coverLife // what moves in the scenes of autocover_scenes.go
}

type coverCloud struct {
	img        *ui.Bitmap
	x, y, w, h float64
	speed      float64 // pixels a second
}

type coverStar struct{ x, y, b, ph float64 }

// sceneOf forces the landscape, for tests: -1 leaves it to the seed.
var sceneOf = -1

// newAutoCover makes the landscape of seed for a light or a dark page; th, if given, is the
// app's theme, whose hue the sky takes a little of.
func newAutoCover(seed uint64, dark bool, th *coverTheme) *autoCover {
	ac, img := drawAutoCover(seed, dark, th)
	ac.img = ui.NewBitmap(img).Pixelated()
	return ac
}

// drawAutoCover is newAutoCover's landscape, and its picture.
func drawAutoCover(seed uint64, dark bool, th *coverTheme) (*autoCover, *image.RGBA) {
	rng := splitmix(seed)
	ac := &autoCover{scene: sceneOfSeed(seed), dark: dark, seed: seed, season: seasonOf(coverNow())}
	rng.next()
	if sceneOf >= 0 {
		ac.scene = sceneOf
	}
	lights := dayLights
	if dark {
		lights = nightLights
	}
	ac.light = int(rng.next() % uint64(len(lights)))
	l := lights[ac.light]
	var tint rgb
	tintA := 0.0
	if th != nil {
		tint, tintA = rgbOf(coverTone(th.h1, 0.6, 0.6)), 0.18
	}
	cv := newCanvas(picW, picH)
	cv.sky(&l, tint, tintA)
	var skyAt [picH]rgb
	for y := range skyAt {
		skyAt[y] = cv.get(0, y)
	}

	// the sun, or the moon and the stars
	sx := rng.in(0.3, 0.7) * picW
	sy := rng.in(0.18, 0.5) * picHz
	if l.sunLow {
		sy = rng.in(0.62, 0.8) * picHz
	}
	shine := 1.0 // how bright the light on the water is: the moon's by its phase
	if l.night {
		ph := moonPhase(coverNow())
		shine = 0.15 + 0.85*(1-math.Cos(ph*2*math.Pi))/2
		for i := 0; i < 70; i++ {
			s := coverStar{x: rng.f() * picW, y: rng.f() * picHz * 0.85, b: rng.in(0.3, 1), ph: rng.f() * 6.28}
			ac.stars = append(ac.stars, s)
			cv.set(int(s.x), int(s.y), hexRGB("#ffffff"), s.b*0.35)
		}
		cv.moon(sx, sy, rng.in(5, 8), ph, l.sun, l.mid)
		ac.shooting = true
	} else {
		cv.glow(sx, sy, picHz*0.9, l.sun, 0.35)
		cv.disc(sx, sy, rng.in(5, 8), l.sun, 1)
	}

	haze := l.low
	ground := func(base rgb, depth float64) rgb { // the land at depth (0 far, 1 near), in this light
		c := ac.seasonal(base).mul(l.lit)
		if l.night {
			c = c.mix(hexRGB("#0b1230"), 0.55)
		}
		return haze.mix(c, depth)
	}
	waterLine := float64(picHz)
	ns := seed

	ridge := func(base float64, amp, freq float64, oct int, sharp bool, k uint64) func(float64) float64 {
		return func(x float64) float64 {
			n := fbm(ns+k, x*freq, oct)
			if sharp {
				n = 1 - math.Abs(2*n-1)
				n = n * n
			}
			return base - amp*n
		}
	}

	reflect := func(water rgb) { // the land above the horizon, mirrored in the water below, rippled
		hz := int(waterLine)
		for y := hz; y < picH; y++ {
			d := y - hz
			src := hz - 1 - d
			for x := 0; x < picW; x++ {
				wob := int(math.Round(math.Sin(float64(y)*1.7+float64(x)*0.05) * float64(d) * 0.08))
				c := cv.get(x+wob, src)
				cv.set(x, y, c.mix(water, 0.35+0.5*float64(d)/float64(picH-hz)), 1)
			}
		}
		for i := 0; i < 90; i++ { // ripples catching the light
			y := waterLine + 2 + rng.f()*float64(picH-hz-2)
			x := rng.f() * picW
			cv.rect(x, y, x+rng.in(3, 12), y+1, l.low.mix(l.sun, 0.4), 0.35)
		}
	}
	sea := func() {
		water := l.top.mix(l.low, 0.35).mix(hexRGB("#1b4d7a"), 0.25)
		if l.night {
			water = l.top.mix(l.mid, 0.4)
		}
		cv.rect(0, waterLine, picW, picH, water, 1)
		for y := int(waterLine); y < picH; y++ { // the water lighter far off, toward the horizon's haze
			k := float64(y-int(waterLine)) / float64(picH-int(waterLine))
			cv.rect(0, float64(y), picW, float64(y+1), l.low, 0.55*(1-k))
		}
		for x := sx - 30; x < sx+30; x += 1 { // the sun's (the moon's) path on the water
			for y := int(waterLine) + 1; y < picH; y += 2 {
				spread := 4 + float64(y-int(waterLine))*0.9
				if math.Abs(x-sx) < spread && valueNoise(ns+5, x*0.4, float64(y)*0.9) > 0.55 {
					cv.set(int(x), y, l.sun, 0.55*shine)
				}
			}
		}
		cv.rect(0, waterLine, picW, waterLine+1, l.low.mix(l.sun, 0.3), 0.6)
		ac.glint = true
	}

	k := &kit{cv: cv, rng: &rng, l: &l, ac: ac, ns: ns, sx: sx, ridge: ridge, sea: sea, water: &waterLine}
	switch ac.scene {
	default:
		k.draw(ac.scene)
	case sceneMountains:
		rock := hexRGB("#6e7d9e")
		for i := 0; i < 4; i++ {
			d := 0.3 + float64(i)*0.23
			top := ridge(picHz+float64(i)*5-4, picHz*(0.75-0.13*float64(i)), 0.006+0.004*float64(i), 5, i < 2, uint64(i))
			cv.land(top, ground(rock, d), ground(rock.mix(hexRGB("#3c4a68"), 0.5), d))
			if i < 2 { // snow on the far peaks
				snowline := picHz * (0.42 + 0.12*float64(i))
				for x := 0; x < picW; x++ {
					t := top(float64(x))
					for y := int(t); float64(y) < snowline-valueNoise(ns+9, float64(x)*0.2, 0)*4; y++ {
						cv.set(x, y, ground(hexRGB("#f4f7fb"), d+0.2), 0.9)
					}
				}
			}
		}
		cv.land(ridge(picHz+16, 4, 0.01, 3, false, 9), ground(hexRGB("#6f9a5a"), 1), ground(hexRGB("#3f6e44"), 1))
		ac.birds = !l.night
	case sceneLake:
		rock := hexRGB("#68789b")
		for i := 0; i < 2; i++ {
			d := 0.35 + float64(i)*0.3
			top := ridge(waterLine, picHz*(0.7-0.25*float64(i)), 0.006+0.006*float64(i), 5, i == 0, uint64(i)+20)
			cv.land(top, ground(rock, d), ground(rock, d))
		}
		forest := ground(hexRGB("#2f5d3e"), 0.95)
		for x := 0.0; x < picW; x += rng.in(3, 7) { // pines along the far shore
			if valueNoise(ns+3, x*0.02, 0) > 0.45 {
				cv.pine(x, waterLine+1, rng.in(5, 11), forest)
			}
		}
		reflect(l.top.mix(hexRGB("#2a5a7a"), 0.4))
		ac.birds = !l.night
	case sceneSea:
		waterLine = picHz - 4
		sea()
		for i := 0; i < 2; i++ { // sails far off
			bx := rng.in(0.2, 0.8) * picW
			sail := ground(hexRGB("#f7f3ea"), 0.85)
			cv.tri(bx, waterLine-1, 4, 7, sail)
			cv.rect(bx-3, waterLine-1, bx+3, waterLine+1, ground(hexRGB("#5a4636"), 0.9), 1)
		}
		ac.birds = !l.night
	case sceneForest:
		cv.land(ridge(picHz-4, picHz*0.45, 0.005, 5, true, 29), ground(hexRGB("#6e7d9e"), 0.3), ground(hexRGB("#6e7d9e"), 0.35))
		for i := 0; i < 4; i++ {
			d := 0.3 + 0.23*float64(i)
			c := ground(hexRGB("#2c5a3c"), d)
			base := picHz - 6 + float64(i)*9
			cv.land(ridge(base+2, 3, 0.01, 2, false, uint64(i)+30), c, c)
			for x := rng.in(-4, 4); x < picW; x += rng.in(3, 6) - float64(i)*0.4 {
				h := rng.in(9, 16) + float64(i)*4
				cv.pine(x, base+3+rng.f()*2, h, c)
			}
			cv.rect(0, base-6, picW, base+4, haze, 0.18*(1-float64(i)/4)) // mist between the rows
		}
		ac.fireflies = l.night
		ac.birds = !l.night

	case sceneDesert:
		sand := []string{"#e9c78f", "#dcae6c", "#c9924f"}
		for p := 0; p < 3; p++ { // pyramids far off
			px, w := rng.in(0.25, 0.75)*picW, rng.in(26, 48)
			by := picHz + 2.0
			for y := int(by - w*0.6); y < int(by); y++ {
				k := (by - float64(y)) / (w * 0.6)
				hw := w / 2 * (1 - k)
				cv.rect(px-hw, float64(y), px, float64(y+1), ground(hexRGB("#e8c48c"), 0.55), 1)
				cv.rect(px, float64(y), px+hw, float64(y+1), ground(hexRGB("#b98a55"), 0.55), 1)
			}
		}
		for i := 0; i < 3; i++ {
			d := 0.5 + 0.25*float64(i)
			top := ridge(picHz+4+float64(i)*9, 9-float64(i)*2, 0.005+0.003*float64(i), 2, false, uint64(i)+50)
			c := ground(hexRGB(sand[i]), d)
			cv.land(top, c.mix(l.sun, 0.15), c.mix(hexRGB("#8a5a33"), 0.25))
		}

	case sceneLighthouse:
		waterLine = picHz + 2
		sea()
		left := rng.f() < 0.5
		cliff := func(x float64) float64 { // a headland from one side, ending short of the middle
			u := x / picW
			if !left {
				u = 1 - u
			}
			return waterLine + 30*smooth(0.43, 0.47, u) - 22*smooth(0.45, 0.34, u) - fbm(ns+80, x*0.05, 3)*4*smooth(0.45, 0.36, u)
		}
		cv.land(cliff, ground(hexRGB("#6d7a5e"), 0.9), ground(hexRGB("#4a4f40"), 1))
		lx := 0.38 * picW
		if !left {
			lx = picW - lx
		}
		lt := cliff(lx)
		for y := int(lt - 22); y < int(lt); y++ { // the tower, striped
			k := (float64(y) - (lt - 22)) / 22
			c := hexRGB("#f6f2ea")
			if (y/4)%2 == 0 {
				c = hexRGB("#c4423a")
			}
			cv.rect(lx-2-k, float64(y), lx+2+k, float64(y+1), ground(c, 0.95), 1)
		}
		lamp := lt - 25
		cv.rect(lx-2, lamp-1, lx+2, lamp+3, hexRGB("#fff1b0"), 1)
		cv.rect(lx-3, lamp-2, lx+3, lamp-1, ground(hexRGB("#3a3a3a"), 1), 1)
		if l.night || l.sunLow {
			cv.glow(lx, lamp+1, 14, hexRGB("#fff1b0"), 0.6)
			ac.beam = [2]float64{lx, lamp + 1}
		}
		ac.birds = !l.night

	case sceneIslands:
		waterLine = picHz
		sea()
		for i := 0; i < 3; i++ {
			ix, w := rng.in(0.22, 0.78)*picW, rng.in(30, 70)
			h := rng.in(5, 12)
			c := ground(hexRGB("#4e8a52"), 0.7+0.1*float64(i))
			for x := ix - w/2; x < ix+w/2; x++ {
				u := (x - ix) / (w / 2)
				t := waterLine - h*(1-u*u) - fbm(ns+uint64(i)+120, x*0.1, 2)*2
				cv.rect(x, t, x+1, waterLine+1, c, 1)
			}
			cv.rect(ix-w/2, waterLine, ix+w/2, waterLine+1, ground(hexRGB("#efe0b4"), 0.8), 1) // its beach
			for p := 0; p < 2; p++ {                                                           // palms
				px := ix + rng.in(-w/4, w/4)
				u := (px - ix) / (w / 2)
				base := waterLine - h*(1-u*u)
				lean := rng.in(-0.25, 0.25)
				trunk := ground(hexRGB("#6b5236"), 0.9)
				tx := px
				for y := base; y > base-12; y-- {
					cv.set(int(tx), int(y), trunk, 1)
					tx += lean
				}
				leaf := ground(hexRGB("#2f6b3a"), 0.95)
				for a := 0.0; a < 6.28; a += 1.05 { // fronds, drooping
					for r := 0.0; r < 7; r++ {
						cv.set(int(tx+math.Cos(a)*r), int(base-12+math.Sin(a)*r*0.4+r*r*0.06), leaf, 1)
					}
				}
			}
		}
		ac.birds = !l.night
	case sceneBeach:
		// seen from above, from the land at the top to the sea at the foot: grass and trees,
		// dry sand, the wet sand the waves run up, and the water, shallow and clear, then deep
		ac.noSky = true
		lit := rgb{255, 255, 255}.mix(l.lit, 0.35) // seen from above, the hour's light only tints it
		in := func(hex string) rgb {
			c := ac.seasonal(hexRGB(hex)).mul(lit)
			if l.night {
				c = c.mul(rgb{120, 135, 175}) // moonlight: darker, and blue
			}
			return c
		}
		dry, wet := in("#efe4d2"), in("#d9cbb6")
		shallow, deep := in("#8fd6cf"), in("#1f8580")
		grass, leaf := in("#6d9a45"), in("#5f9a40")
		shade := in("#bfae98")
		ac.sand, ac.shallow = wet, shallow
		ac.shore = make([]float64, picW)
		for x := range ac.shore {
			fx := float64(x)
			ac.shore[x] = 49 + 3*math.Sin(fx*0.011+1) + 4*fbm(ns+140, fx*0.02, 3)
		}
		for x := 0; x < picW; x++ {
			sh := ac.shore[x]
			for y := 0; y < picH; y++ {
				fy := float64(y)
				var c rgb
				switch {
				case fy < sh-7:
					c = dry
				case fy < sh:
					c = dry.mix(wet, smooth(sh-7, sh-4, fy))
				default: // the water deepens as it goes out
					k := clamp01((fy - sh) / 46)
					c = shallow.mix(deep, k*k*0.9+k*0.1)
					// light on the bottom: a net of bright lines, as the sun through ripples
					n := valueNoise(ns+150, float64(x)*0.09, fy*0.16) + 0.5*valueNoise(ns+151, float64(x)*0.21, fy*0.33)
					if math.Abs(n-0.75) < 0.035 {
						c = c.mix(hexRGB("#ffffff"), 0.22*(1-k*0.6))
					}
				}
				cv.set(x, y, c, 1)
			}
		}
		// the land: grass along the top, its edge ragged, and trees on the sand under it
		edge := func(x float64) float64 { return 9 + 7*fbm(ns+160, x*0.03, 3) }
		for x := 0; x < picW; x++ {
			e := edge(float64(x))
			for y := 0; y < int(e)+2; y++ {
				cv.set(x, y+2, shade, 0.35) // its shadow on the sand
			}
			for y := 0; y < int(e); y++ {
				cv.set(x, y, grass.mix(hexRGB("#3f6e2c"), 0.25*valueNoise(ns+161, float64(x)*0.3, float64(y)*0.3)), 1)
			}
		}
		tree := func(cx, cy, r float64, palm bool) {
			cv.disc(cx+r*0.5, cy+r*0.45, r, shade, 0.55) // its shadow, down and to the right
			if palm {                                    // fronds from the middle, a star
				for a := 0.0; a < 6.28; a += 6.28 / 7 {
					for t := 0.0; t < r*1.2; t += 0.5 {
						w := 1.0 - t/(r*1.4)
						px, py := cx+math.Cos(a)*t, cy+math.Sin(a)*t
						cv.disc(px, py, w, leaf.mix(hexRGB("#2f5a24"), 0.2), 1)
					}
				}
				cv.disc(cx, cy, 1, hexRGB("#7a5a3a").mix(leaf, 0.3), 1)
				return
			}
			cv.disc(cx, cy, r, leaf.mix(hexRGB("#2c5420"), 0.45), 1) // a round crown: dark at its rim,
			cv.disc(cx-r*0.15, cy-r*0.15, r*0.78, leaf, 1)
			cv.disc(cx-r*0.35, cy-r*0.35, r*0.38, leaf.mix(hexRGB("#c8e68a"), 0.35), 1) // lit from the top left
		}
		for x := rng.in(0, 6); x < picW; x += rng.in(7, 18) {
			e := edge(x)
			if rng.f() < 0.6 {
				tree(x, e+rng.in(-2, 2), rng.in(5, 8.5), false)
			}
			if rng.f() < 0.45 { // a few out on the sand, smaller, palms among them
				tree(x+rng.in(-4, 4), e+rng.in(8, 18), rng.in(3, 5), rng.f() < 0.5)
			}
		}
		ac.birds = !l.night
		ac.fireflies = false

	}

	if ac.glint { // where the light lies on the water, and nothing stands on it
		for len(ac.glints) < 26 {
			x, y := sx+rng.in(-26, 26), rng.in(waterLine+3, picH-6)
			if c := cv.get(int(x), int(y)); c[2] > c[0]+4 {
				ac.glints = append(ac.glints, [2]float64{x, y})
			} else if rng.f() < 0.1 {
				break
			}
		}
	}

	if ac.green && !ac.noWeather { // the season's weather, where there is green for it
		switch ac.season {
		case 0:
			ac.petals = true
		case 2:
			ac.leaves = true
		case 3:
			ac.snow = true
		}
	}
	if ac.noSky {
		ac.stars, ac.shooting = nil, false
	}

	// the stars that twinkle are those the land does not hide
	seen := ac.stars[:0]
	for _, st := range ac.stars {
		if ac.ownSky {
			seen = append(seen, st)
			continue
		}
		if c, want := cv.get(int(st.x)+1, int(st.y)), skyAt[int(st.y)]; math.Abs(c[0]-want[0])+math.Abs(c[1]-want[1])+math.Abs(c[2]-want[2]) < 60 {
			seen = append(seen, st)
		}
	}
	ac.stars = seen

	// clouds of the sky, made once, that drift
	n := 2 + int(rng.next()%4)
	if l.night {
		n = 1 + int(rng.next()%2)
	}
	if ac.noSky || ac.ownSky { // a scene from above or with its own sky: no clouds
		n = 0
	}
	for i := 0; i < n; i++ {
		ac.clouds = append(ac.clouds, makeCloud(&rng, &l, picHz))
	}

	return ac, cv.image()
}

// makeCloud is a puffy cloud: round lumps on a flat foot, lit from above, shaded below.
func makeCloud(rng *splitmix, l *coverLight, hz float64) coverCloud {
	w, h := int(rng.in(28, 60)), 0
	h = int(float64(w)*0.42) + 2
	cv := newCanvas(w, h)
	key := rgb{255, 0, 255}
	for i := range cv.px {
		cv.px[i] = key
	}
	foot := float64(h) - 1
	type lump struct{ cx, cy, r float64 }
	var lumps []lump
	n := 3 + int(rng.next()%3)
	for i := 0; i < n; i++ { // round lumps, overlapping, the middle ones the biggest
		u := 0.18 + 0.64*float64(i)/float64(n-1) + rng.in(-0.04, 0.04)
		r := float64(w) * (0.13 + 0.12*(1-math.Abs(u-0.5)*2)) * rng.in(0.85, 1.15)
		lumps = append(lumps, lump{u * float64(w), foot - r*0.75, r})
	}
	base := float64(h) * 0.32 // a flat foot under them, its ends round
	top := float64(h)
	for _, l := range lumps {
		top = math.Min(top, l.cy-l.r)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			in := false
			for _, l := range lumps {
				if math.Hypot(fx-l.cx, fy-l.cy) <= l.r {
					in = true
					break
				}
			}
			if !in && fy >= foot-base && fy <= foot { // the foot, a long round-ended bar
				r := base / 2
				cx := math.Max(r, math.Min(float64(w)-r, fx))
				in = math.Hypot(fx-cx, fy-(foot-r)) <= r
			}
			if !in {
				continue
			}
			k := clamp01((fy - top) / (foot - top))
			cv.set(x, y, l.cloudLit.mix(l.cloudShade, smooth(0.45, 1, k)), 1)
		}
	}
	img := cv.image()
	for i := 0; i < len(img.Pix); i += 4 { // the key color is clear
		if img.Pix[i] > 240 && img.Pix[i+1] < 20 && img.Pix[i+2] > 240 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 0, 0, 0
		}
	}
	return coverCloud{
		img: ui.NewBitmap(img).Pixelated(), w: float64(w), h: float64(h),
		x: rng.f() * picW, y: rng.in(0.08, 0.55) * hz, speed: rng.in(0.6, 1.8),
	}
}

// coverTone is a color of hue h (of 360) at lightness l, with saturation s.
func coverTone(h, l, s float64) color.RGBA {
	h = math.Mod(math.Mod(h, 360)+360, 360)
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

// autoCoverOf is page p's landscape for a light or a dark page, made the first time it is
// wanted and kept: a few at most.
func (a *app) autoCoverOf(p *Page, dark bool) *autoCover {
	if a.autoCovers == nil || len(a.autoCovers) > 8 {
		a.autoCovers = map[uint64]*autoCover{}
	}
	key := coverSeed(p) ^ uint64(a.settings.AppTheme%len(appThemes))<<50
	if dark {
		key ^= 1 << 63
	}
	ac := a.autoCovers[key]
	if ac == nil {
		ac = newAutoCover(coverSeed(p), dark, a.appTheme().coverTheme(dark))
		a.autoCovers[key] = ac
	}
	return ac
}

// coverFPS is how often what moves in a cover is drawn again: slow things, few frames.
const coverFPS = 8

// paint draws the landscape across the band, a pixel of it two DIPs, crisp; past the
// picture's width it goes on mirrored, so that there is no seam. Over it, what moves; its
// foot fades into the page, where the title lies: fade is the page's own color.
//
// What moves is drawn into a canvas of the picture's size coverFPS times a second, and each
// frame shows the canvas, as it shows the picture: a frame drawn for a key, the caret or a
// scroll drew the clouds, the waves, the stars and the birds of every tile anew, some 3 ms
// of the CPU's frame and a third of a key's in a big library (#204).
func (ac *autoCover) paint(pt *ui.Painter, band ui.Rect, fade ui.Color) {
	t := float64(pt.Now().UnixMilli()%(1<<40)) / 1000
	if fx.still { // reduced motion: the landscape as it is at one moment, still (#221)
		t = 0
	}
	if tick := math.Floor(t * coverFPS); ac.lifeCv == nil || tick != ac.lifeTick {
		if ac.lifeCv == nil {
			ac.lifeCv = ui.NewCanvas(autoW, autoFullH)
			ac.lifeBm = ac.lifeCv.Bitmap().Pixelated()
		}
		ac.lifeTick = tick
		at := func(px, py float64) (float32, float32) {
			return f32(math.Round(px) * autoPx), f32(math.Round(py) * autoPx)
		}
		ac.lifeCv.Draw(pt, func(cp *ui.Painter) { ac.paintLife(cp, tick/coverFPS, at) })
	}
	pt.Clip(band, 0, func() {
		// the picture's middle is the band's, so what matters in it is seen at any width; to
		// each side it goes on mirrored
		x0 := band.X + band.W/2 - autoW/2
		k0 := -int(math.Ceil(float64((x0 - band.X) / autoW)))
		for k := k0; x0+float32(k)*autoW < band.X+band.W; k++ {
			ox := x0 + float32(k)*autoW
			tile := ui.Rect{X: ox, Y: band.Y, W: autoW, H: band.H}
			opts := ui.ImageOptions{FlipX: k%2 != 0}
			pt.ImageRegion(ac.img, ui.Rect{W: picW, H: picH}, tile, opts)
			pt.ImageRegion(ac.lifeBm, ui.Rect{W: autoW, H: autoFullH}, tile, opts)
		}
		const foot = 96 // how far up the fade reaches
		pt.FillGradient(ui.Rect{X: band.X, Y: band.Y + band.H - foot, W: band.W, H: foot}, ui.LinearGradient{Angle: 180, Oklab: true,
			Stops: []ui.GradientStop{{Color: fade.Alpha(0), At: 0}, {Color: fade.Alpha(0.62), At: 0.5}, {Color: fade, At: 1}}}, 0)
	})
	if !fx.still {
		pt.AnimationFrameRate(coverFPS)
	}
}

// wrap is x modulo m, for x and m not below zero: what math.Mod gives then, without its slow loop. The moving parts
// of a cover wrap a position for every column of every tile at every frame, and math.Mod was half of their cost (#204).
func wrap(x, m float64) float64 { return x - m*math.Floor(x/m) }

// paintLife is what moves in the landscape at time t (seconds), in its canvas: at gives where
// a pixel of the picture is.
func (ac *autoCover) paintLife(pt *ui.Painter, t float64, at func(px, py float64) (float32, float32)) {
	const s = autoPx
	white := rgb{255, 255, 255}
	ac.paintScene(pt, t, at)
	for _, c := range ac.clouds { // clouds drift, and come round again
		x, y := at(wrap(c.x+t*c.speed, picW+c.w)-c.w, c.y)
		w := f32(c.w * s)
		pt.ImageRegion(c.img, ui.Rect{W: f32(c.w), H: f32(c.h)}, ui.Rect{X: x, Y: y, W: w, H: f32(c.h * s)}, ui.ImageOptions{})
	}
	for i, st := range ac.stars { // stars twinkle
		if i%2 == 1 {
			continue
		}
		a := st.b * (0.45 + 0.55*math.Pow(0.5+0.5*math.Sin(t*1.3+st.ph*3), 3))
		x, y := at(st.x, st.y)
		pt.Fill(ui.Rect{X: x, Y: y, W: s, H: s}, white.ui(a), 0)
	}
	if ac.shooting { // now and then a star falls
		const every = 11.0
		k := wrap(t, every) / 0.9
		if k < 1 {
			r := splitmix(ac.seed ^ uint64(t/every))
			x0, y0 := r.f()*picW, r.in(0.05, 0.35)*picHz
			hx, hy := x0+k*60, y0+k*18
			for j := 0.0; j < 10; j++ {
				x, y := at(hx-j*3, hy-j*0.9)
				pt.Fill(ui.Rect{X: x, Y: y, W: s, H: s}, white.ui((1-j/10)*(1-k)), 0)
			}
		}
	}
	if ac.birds { // a few birds, flapping, once across in a while
		const cross = 70.0
		k := wrap(t+float64(ac.seed%50), cross*1.6) / cross
		if k < 1 {
			r := splitmix(ac.seed ^ 0xb1)
			by := r.in(0.15, 0.45) * picHz
			for b := 0; b < 4; b++ {
				bx := k*(picW+80) - 40 - float64(b)*7
				byy := by + float64(b%2)*3 + math.Sin(t*0.7+float64(b))*1.5
				flap := math.Sin(t*6+float64(b)*1.7) > 0
				ink := rgb{40, 44, 60}.ui(0.7)
				x, y := at(bx, byy)
				pt.Fill(ui.Rect{X: x, Y: y, W: s, H: s}, ink, 0)
				wy := float32(-s)
				if !flap {
					wy = 0
				}
				pt.Fill(ui.Rect{X: x - s, Y: y + wy, W: s, H: s}, ink, 0)
				pt.Fill(ui.Rect{X: x + s, Y: y + wy, W: s, H: s}, ink, 0)
			}
		}
	}
	if ac.snow { // snow falls, swaying
		r := splitmix(ac.seed ^ 0x5a0)
		for i := 0; i < 90; i++ {
			fx, sp, ph := r.f()*picW, r.in(3, 7), r.f()*6.28
			y := wrap(r.f()*picH+t*sp, picH)
			x := fx + math.Sin(t*0.8+ph)*3
			px, py := at(x, y)
			pt.Fill(ui.Rect{X: px, Y: py, W: s, H: s}, white.ui(0.85), 0)
		}
	}
	for i, g := range ac.glints { // the water glints where the light lies on it
		if math.Sin(t*2.1+float64(i)*2.7) > 0.7 {
			x, y := at(g[0], g[1])
			pt.Fill(ui.Rect{X: x, Y: y, W: s * 2, H: s}, white.ui(0.75), 0)
		}
	}
	if ac.beam != [2]float64{} { // the lighthouse sweeps the sea
		lx, ly := at(ac.beam[0], ac.beam[1])
		a := math.Sin(t*0.6) * 1.2
		for _, side := range []float64{0, math.Pi} {
			d := a + side
			r := float64(picW) * 0.45 * s
			path := &ui.Path{}
			path.MoveTo(lx, ly).
				LineTo(lx+f32(math.Cos(d-0.05)*r), ly+f32(math.Sin(d-0.05)*r*0.12)).
				LineTo(lx+f32(math.Cos(d+0.05)*r), ly+f32(math.Sin(d+0.05)*r*0.12)).Close()
			pt.FillPathGradient(path, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{Color: rgb{255, 241, 176}.ui(0.38 * math.Abs(math.Cos(d))), At: 0}, {Color: rgb{255, 241, 176}.ui(0), At: 1},
			}})
		}
	}
	if ac.shore != nil { // the waves run up the sand and draw back, their foam ahead of them
		const col = 2 // pixels a column
		foam := rgb{255, 255, 255}
		for x := 0; x < picW; x += col {
			sh := ac.shore[x]
			// how far up it runs here now: a slow swell, a little ahead along the beach
			up := 3 + 3*math.Sin(t*0.9-float64(x)*0.012) + 1.2*math.Sin(t*1.7+float64(x)*0.05)
			front := math.Round(sh - up)
			px, py := at(float64(x), front)
			h := f32((sh - front) * s)
			if h > 0 {
				pt.Fill(ui.Rect{X: px, Y: py, W: col * s, H: h}, ac.shallow.ui(0.72), 0) // the water over the sand
			}
			pt.Fill(ui.Rect{X: px, Y: py - s, W: col * s, H: 2 * s}, foam.ui(0.92), 0) // its edge, foam
			// lace behind it, that breaks up and joins
			lace := valueNoise(ac.seed^0x1ace, float64(x)*0.15, t*0.4)
			if lace > 0.45 {
				pt.Fill(ui.Rect{X: px, Y: py + 2*s, W: col * s, H: s}, foam.ui(0.6), 0)
			}
			if lace < 0.3 {
				pt.Fill(ui.Rect{X: px, Y: py + 4*s, W: col * s, H: s}, foam.ui(0.4), 0)
			}
			// the swell itself, a line of light a little out, coming in
			k := wrap(t*0.25+float64(x)*0.0005, 1)
			_, wy := at(float64(x), sh+4+(1-k)*14)
			pt.Fill(ui.Rect{X: px, Y: wy, W: col * s, H: s}, foam.ui(0.28*math.Sin(k*math.Pi)), 0)
		}
		r := splitmix(ac.seed ^ 0xf15)
		ink := rgb{18, 70, 72}.ui(0.55)
		for i := 0; i < 14; i++ { // fish, a few on their own and a shoal, slowly along
			fx, fy, v := r.f()*picW, r.in(66, 92), r.in(1.5, 4)
			if i >= 8 { // the shoal
				fx, fy, v = 400+float64(i%3)*5, 74+float64(i/3)*3, 3
			}
			x := wrap(fx+t*v, picW+20) - 10
			y := fy + math.Sin(t*0.6+float64(i))*1.5
			hx, hy := at(x, y)
			tx, _ := at(x-2, y)
			pt.Fill(ui.Rect{X: min(hx, tx), Y: hy, W: 3 * s, H: s}, ink, 0)
			wag := float32(0)
			if math.Sin(t*5+float64(i)) > 0 {
				wag = s
			}
			tail, _ := at(x-3, y)
			pt.Fill(ui.Rect{X: tail, Y: hy - s/2 + wag - s/2, W: s, H: s}, ink, 0)
		}
	}
	if ac.fireflies { // fireflies wander low, and blink
		r := splitmix(ac.seed ^ 0xf1f)
		for i := 0; i < 24; i++ {
			fx, fy, ph := r.f()*picW, r.in(picHz-6, picH-30), r.f()*6.28
			x, y := at(fx+math.Sin(t*0.4+ph)*8, fy+math.Cos(t*0.5+ph*2)*4)
			a := math.Max(0, math.Sin(t*1.5+ph*4))
			pt.Fill(ui.Rect{X: x - s, Y: y - s, W: 3 * s, H: 3 * s}, rgb{220, 255, 120}.ui(0.18*a), 2*s)
			pt.Fill(ui.Rect{X: x, Y: y, W: s, H: s}, rgb{235, 255, 150}.ui(0.95*a), 0)
		}
	}
}

func smooth(a, b, x float64) float64 {
	t := math.Min(math.Max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
