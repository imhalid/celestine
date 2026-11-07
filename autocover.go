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
// pyramids, mesas, a city, hills, windmills, a lighthouse, a snowy village, a volcano, a field,
// islands, a neon horizon - in the light of an hour the seed picks too: dawn, morning, noon,
// a golden afternoon or dusk in the light theme; twilight, night or a moonlit night in the
// dark, the moon in its phase of today. The picture is a print of two DIPs to a pixel,
// ordered-dithered, as an old game's; over it, now and then, what lives in it moves, slowly
// and few frames a second: clouds drift, stars twinkle, birds pass, snow falls, the windmills
// turn, the lighthouse sweeps, smoke leaves the volcano, fireflies wander, the sea glints.

const (
	coverNone = "none" // cover: none in the front matter - no cover, not the made one either

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
	sceneHills
	sceneDesert
	sceneMesas
	sceneCity
	sceneWindmills
	sceneLighthouse
	sceneSnow
	sceneVolcano
	sceneField
	sceneIslands
	sceneNeon
	sceneKinds
)

// imageCover reports whether the page has a picture of its own as its cover.
func (p *Page) imageCover() bool { return p.Cover != "" && p.Cover != coverNone }

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
	case p.imageCover():
		return coverKindImage
	case p.Cover == coverNone || a.settings.NoAutoCover:
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
		return coverBandH + barH
	}
	return 0
}

func (a *app) coverRoomOf(p *Page) float32 {
	switch a.coverKind(p) {
	case coverKindAuto:
		return autoRoom + barH
	case coverKindImage:
		return coverRoom + barH
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
	{"Gece", hexRGB("#060a1b"), hexRGB("#0f193a"), hexRGB("#1f2e58"), hexRGB("#f3f1e2"), hexRGB("#34406a"), true, false, hexRGB("#3a4672"), hexRGB("#141c3a")},
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

	// what moves, in the picture's pixels
	clouds    []coverCloud
	stars     []coverStar
	birds     bool
	snow      bool
	glint     bool         // the sea glints where the light lies on it
	glints    [][2]float64 // where
	beam      [2]float64
	mills     [][3]float64 // a windmill's hub, and the length of its blades
	millInk   rgb
	smoke     [2]float64
	fireflies bool
	shooting  bool
	seed      uint64
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
	ac := &autoCover{scene: int(rng.next() % sceneKinds), dark: dark, seed: seed}
	if sceneOf >= 0 {
		ac.scene = sceneOf
	}
	lights := dayLights
	if dark {
		lights = nightLights
	}
	ac.light = int(rng.next() % uint64(len(lights)))
	l := lights[ac.light]
	if ac.scene == sceneNeon { // the neon horizon is at its best late
		if dark {
			ac.light = 0
		} else {
			ac.light = 4
		}
		l = lights[ac.light]
	}
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
		ph := moonPhase(time.Now())
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
		c := base.mul(l.lit)
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

	switch ac.scene {
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
	case sceneHills:
		greens := []string{"#9cc47a", "#7fb064", "#5e9750"}
		for i := 0; i < 3; i++ {
			d := 0.45 + 0.27*float64(i)
			top := ridge(picHz+float64(i)*8, 14-float64(i)*3, 0.004+0.002*float64(i), 3, false, uint64(i)+40)
			c := ground(hexRGB(greens[i]), d)
			cv.land(top, c, c.mix(hexRGB("#2f5a34"), 0.3))
			if i == 1 { // round trees on the middle hill
				for x := rng.in(0, 30); x < picW; x += rng.in(18, 60) {
					t := top(x)
					cv.rect(x-0.5, t-3, x+0.5, t+1, ground(hexRGB("#5a4030"), d), 1)
					cv.disc(x, t-5, rng.in(2.5, 4), ground(hexRGB("#3f7a42"), d+0.1), 1)
				}
			}
			if i == 2 { // a house, its roof red
				hx := rng.in(0.32, 0.68) * picW
				t := top(hx)
				cv.rect(hx-5, t-6, hx+5, t+1, ground(hexRGB("#f2ead8"), 1), 1)
				cv.tri(hx, t-5, 13, 6, ground(hexRGB("#b5463b"), 1))
				win := hexRGB("#4a5a70")
				if l.night {
					win = hexRGB("#ffd27a")
				}
				cv.rect(hx-3, t-4, hx-1, t-2, win, 1)
				cv.rect(hx+1, t-4, hx+3, t-2, win, 1)
			}
		}
		ac.birds = !l.night
		ac.fireflies = l.night
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
	case sceneMesas:
		red := []string{"#c9785a", "#b35e43", "#93452f"}
		for i := 0; i < 3; i++ {
			d := 0.4 + 0.3*float64(i)
			k := uint64(i) + 60
			top := func(x float64) float64 { // flat tops and steep sides: noise stepped
				n := fbm(ns+k, x*(0.006+0.003*float64(i)), 3)
				h := smooth(0.48, 0.56, n)
				return picHz + float64(i)*7 - h*(24-6*float64(i)) - n*3
			}
			c := ground(hexRGB(red[i]), d)
			cv.land(top, c, c.mix(hexRGB("#5a2a1c"), 0.3))
			for x := 0; x < picW; x++ { // a band of paler rock along the mesa
				t := top(float64(x))
				cv.rect(float64(x), t+3, float64(x+1), t+4, c.mix(hexRGB("#f1c9a0"), 0.3), 0.6)
			}
		}
		cv.land(ridge(picHz+22, 2, 0.01, 2, false, 69), ground(hexRGB("#d9a46e"), 1), ground(hexRGB("#a8703f"), 1))
	case sceneCity:
		for i := 0; i < 2; i++ {
			d := 0.45 + 0.4*float64(i)
			c := ground(hexRGB("#4d5872"), d)
			for x := rng.in(-10, 0); x < picW; {
				w := rng.in(6, 16)
				h := rng.in(8, 26) - float64(i)*5
				if rng.f() < 0.08 {
					h += 12 // a tower
				}
				top := picHz + 6 + float64(i)*6 - h
				cv.rect(x, top, x+w, picH, c, 1)
				if rng.f() < 0.25 {
					cv.rect(x+w/2-0.5, top-5, x+w/2+0.5, top, c, 1) // an aerial
				}
				for wy := top + 2; wy < picHz+10; wy += 3 { // windows
					for wx := x + 1.5; wx < x+w-1; wx += 2.5 {
						switch {
						case l.night && rng.f() < 0.35:
							cv.set(int(wx), int(wy), hexRGB("#ffd27a"), 0.9)
						case !l.night:
							cv.set(int(wx), int(wy), c.mix(l.top, 0.35), 0.7)
						}
					}
				}
				x += w + rng.in(0, 3)
			}
		}
		ac.birds = !l.night
	case sceneWindmills:
		top := ridge(picHz+6, 10, 0.004, 3, false, 70)
		cv.land(ridge(picHz, 8, 0.008, 3, false, 71), ground(hexRGB("#8fb56f"), 0.55), ground(hexRGB("#8fb56f"), 0.6))
		cv.land(top, ground(hexRGB("#b9c96a"), 0.95), ground(hexRGB("#7f9a4a"), 1))
		for x := 0; x < picW; x++ { // furrows
			t := top(float64(x))
			for y := int(t) + 3; y < picH; y += 3 {
				cv.set(x, y, ground(hexRGB("#6f8a3e"), 1), 0.35)
			}
		}
		ink := ground(hexRGB("#f1ece2"), 0.95)
		for m := 0; m < 3; m++ {
			x := (0.28 + 0.44*(float64(m)+0.5+rng.in(-0.2, 0.2))/3) * picW
			t := top(x)
			h := rng.in(24, 32)
			for y := int(t - h); y < int(t+1); y++ { // the tower, wider at its foot
				k := (float64(y) - (t - h)) / h
				cv.rect(x-1-k*2, float64(y), x+1+k*2, float64(y+1), ink, 1)
			}
			cv.rect(x-1, t-3, x+1, t+1, ground(hexRGB("#6b4a36"), 1), 1)
			ac.mills = append(ac.mills, [3]float64{x, t - h, h * 0.75})
		}
		ac.millInk = ink
		ac.birds = !l.night
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
	case sceneSnow:
		for i := 0; i < 3; i++ {
			d := 0.45 + 0.27*float64(i)
			top := ridge(picHz+float64(i)*8, 16-float64(i)*4, 0.004+0.002*float64(i), 3, false, uint64(i)+90)
			snow := ground(hexRGB("#eef3fa"), d)
			cv.land(top, snow, snow.mix(hexRGB("#a9bcd8"), 0.4))
			pineC := ground(hexRGB("#2d5340"), d)
			for x := rng.in(0, 20); x < picW; x += rng.in(6, 30) {
				t := top(x)
				h := rng.in(8, 14) + float64(i)*3
				cv.pine(x, t+1, h, pineC)
				cv.tri(x, t+1-h*0.75, h*0.22, h*0.25, snow) // snow on its tip
			}
			if i == 2 { // a cabin, its window lit
				hx := rng.in(0.3, 0.7) * picW
				t := top(hx)
				cv.rect(hx-6, t-6, hx+6, t+1, ground(hexRGB("#7a4a32"), 1), 1)
				cv.tri(hx, t-5, 15, 7, snow)
				cv.rect(hx-2, t-4, hx+1, t-1, hexRGB("#ffcf6e"), 1)
				cv.glow(hx-0.5, t-2.5, 6, hexRGB("#ffcf6e"), 0.3)
				cv.rect(hx+3, t-12, hx+5, t-7, ground(hexRGB("#5a3a2a"), 1), 1) // the chimney
			}
		}
		ac.snow = true
	case sceneVolcano:
		vx := rng.in(0.35, 0.65) * picW
		cone := func(x float64) float64 {
			d := math.Abs(x-vx) / 90
			h := 50 * math.Pow(math.Max(0, 1-d), 1.6)  // a cone, its sides curving out at the foot
			h = math.Min(h, 40) - 3*smooth(0.07, 0, d) // its top cut off, a crater in it
			return picHz + 2 - h - fbm(ns+100, x*0.05, 3)*3
		}
		cv.land(ridge(picHz+2, 10, 0.006, 4, false, 101), ground(hexRGB("#6b6a7a"), 0.45), ground(hexRGB("#6b6a7a"), 0.5))
		cv.land(cone, ground(hexRGB("#5c4a4a"), 0.85), ground(hexRGB("#3a2e30"), 1))
		top := cone(vx)
		if l.night || l.sunLow { // lava's glow at the crater, and a vein down the side
			cv.glow(vx, top, 22, hexRGB("#ff6a2a"), 0.6)
			x := vx
			for y := top; y < picHz; y++ {
				x += math.Sin(y*0.5) * 0.6
				cv.set(int(x), int(y), hexRGB("#ff7a33"), 0.85)
			}
		}
		cv.land(ridge(picHz+16, 3, 0.01, 2, false, 102), ground(hexRGB("#4b5a3f"), 1), ground(hexRGB("#2f3a2a"), 1))
		ac.smoke = [2]float64{vx, top}
	case sceneField:
		lav := rng.f() < 0.5
		rowC, gapC := hexRGB("#9a7fd1"), hexRGB("#6f8f52")
		if !lav {
			rowC, gapC = hexRGB("#e7c063"), hexRGB("#c79a45")
		}
		cv.land(ridge(picHz, 6, 0.006, 3, false, 110), ground(hexRGB("#7fa466"), 0.5), ground(hexRGB("#7fa466"), 0.55))
		vp := rng.in(0.38, 0.62) * picW // the rows run to a point on the horizon
		for y := int(picHz) + 2; y < picH; y++ {
			k := float64(y-int(picHz)) / float64(picH-int(picHz))
			for x := 0; x < picW; x++ {
				u := (float64(x) - vp) / (1 + k*60)
				on := math.Mod(math.Abs(u)+0.25, 1) < 0.55
				c := gapC
				if on {
					c = rowC
				}
				cv.set(x, y, ground(c, 0.7+0.3*k), 1)
			}
		}
		cv.rect(0, picHz+1, picW, picHz+3, ground(gapC, 0.6), 1)
		tx := rng.in(0.3, 0.7) * picW // a lone tree
		cv.rect(tx-1, picHz-10, tx+1, picHz+3, ground(hexRGB("#4a3426"), 0.9), 1)
		cv.disc(tx, picHz-14, 8, ground(hexRGB("#3e6e3a"), 0.9), 1)
		cv.disc(tx-5, picHz-11, 5, ground(hexRGB("#3e6e3a"), 0.9), 1)
		cv.disc(tx+5, picHz-11, 5, ground(hexRGB("#3e6e3a"), 0.9), 1)
		ac.birds = !l.night
		ac.fireflies = l.night
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
	case sceneNeon:
		// the sun striped as it sets behind a grid; mountains in outline
		const sr = 28
		sy = picHz - 12
		for y := int(sy - sr); y < int(sy+sr); y++ {
			c := hexRGB("#ffe27a").mix(hexRGB("#ff4fa3"), clamp01((float64(y)-(sy-sr))/(2*sr)))
			if y > int(sy-8) && (y-int(sy))%4 == 0 { // its stripes
				continue
			}
			for x := int(sx - sr - 1); x < int(sx+sr+1); x++ {
				d := math.Hypot(float64(x)+0.5-sx, float64(y)+0.5-sy)
				cv.set(x, y, c, clamp01(sr-d+0.5))
			}
		}
		cv.glow(sx, sy, 70, hexRGB("#ff7ab8"), 0.35)
		pink := hexRGB("#ff4fa3")
		top := ridge(picHz, 18, 0.008, 4, true, 130)
		cv.land(top, ground(hexRGB("#2a1648"), 0.9), ground(hexRGB("#1a0f30"), 1))
		for x := 0; x < picW; x++ {
			cv.set(x, int(top(float64(x))), pink, 0.8)
		}
		floor := hexRGB("#1b0c33")
		if !dark {
			floor = hexRGB("#3a2266")
		}
		cv.rect(0, picHz, picW, picH, floor, 1)
		for y := int(picHz); y < picH; y++ { // the grid: lines across, closer far off
			k := float64(y-int(picHz)) / float64(picH-int(picHz))
			if math.Mod(math.Sqrt(k)*12, 1) < 0.18 {
				cv.rect(0, float64(y), picW, float64(y+1), pink, 0.75)
			}
		}
		vp := sx
		for i := -60; i <= 60; i++ { // and lines toward the sun's point
			for y := int(picHz); y < picH; y++ {
				k := float64(y-int(picHz)) / float64(picH-int(picHz))
				cv.set(int(vp+float64(i)*(4+k*40)), y, pink, 0.6)
			}
		}
		cv.rect(0, picHz, picW, picHz+1, hexRGB("#ffd1ec"), 0.9)
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

	// the stars that twinkle are those the land does not hide
	seen := ac.stars[:0]
	for _, st := range ac.stars {
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
	if ac.scene == sceneNeon {
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
func (ac *autoCover) paint(pt *ui.Painter, band ui.Rect, fade ui.Color) {
	t := float64(pt.Now().UnixMilli()%(1<<40)) / 1000
	pt.Clip(band, 0, func() {
		// the picture's middle is the band's, so what matters in it is seen at any width; to
		// each side it goes on mirrored
		x0 := band.X + band.W/2 - autoW/2
		k0 := -int(math.Ceil(float64((x0 - band.X) / autoW)))
		for k := k0; x0+float32(k)*autoW < band.X+band.W; k++ {
			ox := x0 + float32(k)*autoW
			flip := k%2 != 0
			pt.ImageRegion(ac.img, ui.Rect{W: picW, H: picH}, ui.Rect{X: ox, Y: band.Y, W: autoW, H: band.H}, ui.ImageOptions{FlipX: flip})
			// the picture's pixel (px, py) is here, on this tile
			at := func(px, py float64) (float32, float32) {
				if flip {
					px = picW - px
				}
				return ox + f32(math.Round(px)*autoPx), band.Y + f32(math.Round(py)*autoPx)
			}
			ac.paintLife(pt, t, at, flip)
		}
		const foot = 96 // how far up the fade reaches
		pt.FillGradient(ui.Rect{X: band.X, Y: band.Y + band.H - foot, W: band.W, H: foot}, ui.LinearGradient{Angle: 180, Oklab: true,
			Stops: []ui.GradientStop{{Color: fade.Alpha(0), At: 0}, {Color: fade.Alpha(0.62), At: 0.5}, {Color: fade, At: 1}}}, 0)
	})
	pt.AnimationFrameRate(coverFPS)
}

// paintLife is what moves in the landscape at time t (seconds), on a tile: at gives where
// a pixel of the picture is.
func (ac *autoCover) paintLife(pt *ui.Painter, t float64, at func(px, py float64) (float32, float32), flip bool) {
	const s = autoPx
	white := rgb{255, 255, 255}
	for _, c := range ac.clouds { // clouds drift, and come round again
		x, y := at(math.Mod(c.x+t*c.speed, picW+c.w)-c.w, c.y)
		w := f32(c.w * s)
		dst := ui.Rect{X: x, Y: y, W: w, H: f32(c.h * s)}
		if flip { // a mirrored tile: its left is where its right was
			dst.X -= w
		}
		pt.ImageRegion(c.img, ui.Rect{W: f32(c.w), H: f32(c.h)}, dst, ui.ImageOptions{FlipX: flip})
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
		k := math.Mod(t, every) / 0.9
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
		k := math.Mod(t+float64(ac.seed%50), cross*1.6) / cross
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
			y := math.Mod(r.f()*picH+t*sp, picH)
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
	if len(ac.mills) > 0 { // the windmills turn
		for i, m := range ac.mills {
			hx, hy := at(m[0], m[1])
			a0 := t*0.9 + float64(i)*1.3
			for b := 0; b < 4; b++ {
				a := a0 + float64(b)*math.Pi/2
				path := &ui.Path{}
				path.MoveTo(hx, hy).LineTo(hx+f32(math.Cos(a)*m[2]*s), hy+f32(math.Sin(a)*m[2]*s))
				pt.StrokePath(path, s, ac.millInk.ui(1))
			}
			pt.Fill(ui.Rect{X: hx - s, Y: hy - s, W: 2 * s, H: 2 * s}, ac.millInk.mix(rgb{60, 50, 40}, 0.5).ui(1), s)
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
	if ac.smoke != [2]float64{} { // smoke leaves the volcano, and goes
		for i := 0; i < 7; i++ {
			k := math.Mod(t/9+float64(i)/7, 1)
			x, y := at(ac.smoke[0]+k*30+math.Sin(k*6+float64(i))*3, ac.smoke[1]-2-k*34)
			r := f32((2 + k*7) * s)
			c := rgb{120, 116, 120}
			if ac.dark {
				c = rgb{70, 66, 76}
			}
			pt.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, c.ui(0.5*(1-k)), r)
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
