package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// An automatic cover: a page with no cover of its own gets one made from its seed - the
// time it was made and the name of its folder, which do not change - so that each page has
// a cover of its own, and the same each time. Nothing is stored and nothing is a file. It is
// a still pattern in four tones, drawn from a field (waves, rings, clouds, plasma, stripes,
// metaballs, halftone dots) and dithered (an ordered grid of two sizes, or error diffusion),
// in cells of a few pixels that are drawn as squares, as pixel art is.

const (
	coverNone = "none" // cover: none in the front matter - no cover, not the made one either

	autoCell  = 4   // a cell's side, in DIPs
	autoCols  = 560 // cells across: 2240 DIPs, more than a window is wide
	autoBandH = 184 // the band's height: the room, the title, the foot
	autoRoom  = 100 // above the title, in the band
)

// the kinds of field
const (
	fieldWaves = iota
	fieldRings
	fieldClouds
	fieldPlasma
	fieldStripes
	fieldBlobs
	fieldDots
	fieldKinds
)

// the ways of dithering
const (
	ditherBayer8  = iota // a fine ordered grid
	ditherBayer4         // a coarse one
	ditherDiffuse        // error diffusion: grain
	ditherKinds
)

// imageCover reports whether the page has a picture of its own as its cover.
func (p *Page) imageCover() bool { return p.Cover != "" && p.Cover != coverNone }

// the kinds of cover a page shows
const (
	coverKindNone = iota
	coverKindImage
	coverKindAuto
)

// coverKind is what covers page p: its own picture, the made pattern (unless the page said
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
func (a *app) coverBandOf(p *Page) float32 {
	if a.coverKind(p) == coverKindAuto {
		return autoBandH
	}
	return coverBandH
}

func (a *app) coverRoomOf(p *Page) float32 {
	if a.coverKind(p) == coverKindAuto {
		return autoRoom
	}
	return coverRoom
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

// autoCover is the pattern of a page: its look from the seed, and its pixels.
type autoCover struct {
	field, dither int
	hue           float64
	img           *ui.Bitmap
}

// the Bayer matrices of the ordered dither, 0..n*n-1
var bayer4 = [4][4]int{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}

func bayerAt(n, x, y int) float64 {
	if n == 4 {
		return (float64(bayer4[y&3][x&3]) + 0.5) / 16
	}
	// 8x8, from the 4x4: M8 = 4*M4(x/2... ) kept by the recursive definition
	return (float64(bayer8(x&7, y&7)) + 0.5) / 64
}

func bayer8(x, y int) int {
	v := 0
	for bit := 2; bit >= 0; bit-- { // the recursive Bayer index: interleave the bits of x^y and y
		xb, yb := (x>>bit)&1, (y>>bit)&1
		v = v<<2 | ((xb ^ yb) << 1) | yb
	}
	return v
}

func smooth(a, b, x float64) float64 {
	t := math.Min(math.Max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}

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

// fieldOf and ditherOf force a field and a dither, for tests: -1 leaves them to the seed.
var fieldOf, ditherOf = -1, -1

// newAutoCover makes the pattern of seed, cols by rows of cells.
func newAutoCover(seed uint64, rows int) *autoCover {
	rng := splitmix(seed)
	ac := &autoCover{field: int(rng.next() % fieldKinds), dither: int(rng.next() % ditherKinds), hue: rng.f() * 360}
	if fieldOf >= 0 {
		ac.field = fieldOf
	}
	if ditherOf >= 0 {
		ac.dither = ditherOf
	}
	var k [20]float64
	for i := range k {
		k[i] = rng.f()
	}
	const cols = autoCols
	aspect := float64(cols) / float64(rows) // u runs over 0..aspect, v over 0..1: cells are square
	field := make([]float64, cols*rows)
	for y := 0; y < rows; y++ {
		v := float64(y) / float64(rows)
		for x := 0; x < cols; x++ {
			u := float64(x) / float64(rows)
			var f float64
			switch ac.field {
			case fieldWaves:
				f = 0.5 + 0.28*math.Sin(u*(5+9*k[0])+v*(3+6*k[1])+k[2]*6.28) + 0.22*math.Sin(u*(11+14*k[3])-v*(5+8*k[4])+k[5]*6.28+1.6*math.Sin(v*6+u*2))
			case fieldRings:
				d1 := math.Hypot(u-aspect*(0.2+0.6*k[0]), (v-k[1])*1.0)
				d2 := math.Hypot(u-aspect*(0.2+0.6*k[2]), (v-k[3])*1.0)
				f = 0.5 + 0.5*math.Sin(d1*(24+30*k[4]))*(0.6+0.4*math.Cos(d2*(9+10*k[5])))
			case fieldClouds:
				f = 0
				amp, fr := 0.55, 2.2+2.5*k[0]
				for o := 0; o < 4; o++ {
					f += amp * valueNoise(seed+uint64(o), u*fr+k[1]*40, v*fr+k[2]*40)
					amp *= 0.5
					fr *= 2.05
				}
				f = smooth(0.12, 0.78, f)
			case fieldPlasma:
				f = (math.Sin(u*(6+8*k[0])+k[1]*6.28) + math.Sin(v*(8+8*k[2])+k[3]*6.28) +
					math.Sin((u+v)*(5+7*k[4])+k[5]*6.28) + math.Sin(math.Hypot(u-aspect/2, v-0.5)*(10+12*k[6]))) / 8
				f += 0.5
			case fieldStripes:
				ang := (k[0] - 0.5) * 1.2
				w := math.Sin(u*(3+5*k[1])+v*(2+3*k[2])+k[3]*6.28) * (0.25 + 0.25*k[4])
				t := u*math.Cos(ang) + v*math.Sin(ang) + w
				f = math.Abs(frac(t*(2.2+3*k[5])+k[6])-0.5) * 2
			case fieldBlobs:
				sum := 0.0
				for b := 0; b < 5; b++ {
					cx, cy := aspect*(0.1+0.8*k[b*3]), 0.1+0.8*k[b*3+1]
					r := 0.12 + 0.2*k[b*3+2]
					d := math.Hypot(u-cx, v-cy)
					sum += (r * r) / (d*d + 0.002)
				}
				f = smooth(0.4, 3.2, sum)
			case fieldDots:
				ang := k[0] * math.Pi
				sc := 0.07 + 0.06*k[1]
				ru := (u*math.Cos(ang) + v*math.Sin(ang)) / sc
				rv := (-u*math.Sin(ang) + v*math.Cos(ang)) / sc
				dx, dy := frac(ru)-0.5, frac(rv)-0.5
				d := math.Hypot(dx, dy)
				bigness := 0.25 + 0.5*smooth(0, aspect, u+k[2]*aspect*0.5)
				f = 1 - smooth(bigness-0.18, bigness+0.12, d)
				f = f*0.8 + 0.1
			}
			// the foot is the darkest, where the title lies
			f *= 1 - 0.55*smooth(0.42, 1.0, v)
			field[y*cols+x] = math.Min(math.Max(f, 0), 1)
		}
	}
	// the tones: four, dark to light, in the seed's hue, the lightest turned a little
	tones := [4]color.RGBA{coverTone(ac.hue, 0.10, 0.32), coverTone(ac.hue, 0.18, 0.34), coverTone(ac.hue+10, 0.29, 0.34), coverTone(ac.hue+30, 0.44, 0.36)}
	img := image.NewRGBA(image.Rect(0, 0, cols, rows))
	put := func(x, y, idx int) { img.SetRGBA(x, y, tones[idx]) }
	switch ac.dither {
	case ditherBayer8, ditherBayer4:
		n := 8
		if ac.dither == ditherBayer4 {
			n = 4
		}
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				v := field[y*cols+x] * 3
				base := math.Floor(v)
				idx := int(base)
				if v-base > bayerAt(n, x, y) {
					idx++
				}
				put(x, y, min(idx, 3))
			}
		}
	default: // Floyd-Steinberg, going to and fro
		err := make([]float64, cols*rows)
		copy(err, field)
		for y := 0; y < rows; y++ {
			dir, start := 1, 0
			if y%2 == 1 {
				dir, start = -1, cols-1
			}
			for i := 0; i < cols; i++ {
				x := start + i*dir
				old := math.Min(math.Max(err[y*cols+x], 0), 1)
				idx := int(math.Round(old * 3))
				put(x, y, idx)
				e := old - float64(idx)/3
				add := func(dx, dy int, w float64) {
					nx, ny := x+dx*dir, y+dy
					if nx >= 0 && nx < cols && ny < rows {
						err[ny*cols+nx] += e * w
					}
				}
				add(1, 0, 7.0/16)
				add(-1, 1, 3.0/16)
				add(0, 1, 5.0/16)
				add(1, 1, 1.0/16)
			}
		}
	}
	ac.img = ui.NewBitmap(img).Pixelated()
	return ac
}

func frac(x float64) float64 { return x - math.Floor(x) }

// coverTone is a color of hue h (of 360) at lightness l, soft in its chroma.
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

// autoCoverOf is page p's pattern, made the first time it is wanted and kept: a few at most.
func (a *app) autoCoverOf(p *Page) *autoCover {
	if a.autoCovers == nil || len(a.autoCovers) > 6 {
		a.autoCovers = map[uint64]*autoCover{}
	}
	seed := coverSeed(p)
	ac := a.autoCovers[seed]
	if ac == nil {
		ac = newAutoCover(seed, int(math.Ceil(autoBandH/autoCell)))
		a.autoCovers[seed] = ac
	}
	return ac
}

// paintAutoCover draws the pattern across the band, its cells squares, from the left edge:
// the window growing shows more of it.
func (ac *autoCover) paint(pt *ui.Painter, band ui.Rect) {
	cols := float32(math.Ceil(float64(band.W) / autoCell))
	if cols > autoCols {
		cols = autoCols
	}
	rows := float32(math.Ceil(float64(band.H) / autoCell))
	w, h := ac.img.Size()
	pt.Clip(band, 0, func() {
		pt.ImageRegion(ac.img, ui.Rect{W: cols, H: float32(math.Min(float64(rows), float64(h)))},
			ui.Rect{X: band.X, Y: band.Y, W: cols * autoCell, H: float32(math.Min(float64(rows), float64(h))) * autoCell}, ui.ImageOptions{})
	})
	_ = w
}
