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
// a cover of its own, and the same each time. Nothing is stored and nothing is a file.
//
// It is a mosaic: square tiles, each of the color a soft field gives at its place - a slow
// gradient and a little cloud - with a little jitter of its own, in two hues near each
// other from the seed. How the tiles lie is the seed's too: even tiles with a thin gap,
// big tiles, tiles lit from a corner, or tiles merged into larger blocks with an outline.
// There is a palette for the light theme (pale) and one for the dark (deep): the same
// pattern, in colors that suit the page.

const (
	coverNone = "none" // cover: none in the front matter - no cover, not the made one either

	autoW     = 2240 // the picture's width, in DIPs: more than a window is wide
	autoBandH = 184  // the band's height: the room, the title, the foot
	autoRoom  = 100  // above the title, in the band

	// the cover begins at the top of the window, under the title bar, so its picture is
	// as tall as the bar and the band
	autoFullH = autoBandH + barH
)

// the ways the tiles lie
const (
	mosaicEven   = iota // even tiles, a thin gap between
	mosaicBig           // big tiles, no gap, a hair of a line
	mosaicLit           // tiles lit from a corner: a light edge, a dark one
	mosaicBlocks        // tiles merged into larger blocks, outlined
	mosaicKinds
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

// autoCover is the mosaic of a page: how it lies, and its pixels.
type autoCover struct {
	style int
	theme int // the index of its theme, of coverThemes
	hue   float64
	img   *ui.Bitmap
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

// styleOf forces how the tiles lie, for tests: -1 leaves it to the seed.
var styleOf = -1

// mosaicPalette is the two colors the field runs between, in the light theme (pale) or the
// dark (deep), and the color that shows between tiles.
func mosaicPalette(h1, h2, sat float64, dark bool) (a, b, gap [3]float64) {
	s := func(v float64) float64 { return math.Min(v*sat, 1) } // a theme is more or less saturated
	if dark {
		// bright at one end, deep at the other, and clear in between: a jewel, not mud
		return rgbOf(coverTone(h1, 0.46, s(0.38))), rgbOf(coverTone(h2, 0.17, s(0.36))), rgbOf(coverTone(h2, 0.09, s(0.30)))
	}
	// near white at one end, a clear color at the other
	return rgbOf(coverTone(h1, 0.955, s(0.55))), rgbOf(coverTone(h2, 0.79, s(0.46))), rgbOf(coverTone(h1, 0.99, s(0.4)))
}

// A cover theme is a pair of hues the mosaic runs between (the near-white or bright end,
// and the clear or deep one) and how saturated it is: a family of colors that go together.
// The seed picks one for a page.
type coverTheme struct {
	name   string
	h1, h2 float64 // hue, of 360, at the two ends
	sat    float64 // 1 is as the palette says
}

var coverThemes = []coverTheme{
	{"Okyanus", 212, 232, 1.0},
	{"Deniz", 183, 203, 1.0},
	{"Orman", 140, 112, 0.95},
	{"Nane", 165, 148, 0.95},
	{"Gün batımı", 28, 342, 1.0},
	{"Gül", 346, 322, 1.0},
	{"Lavanta", 262, 286, 1.0},
	{"Şeftali", 14, 36, 1.0},
	{"Kum", 44, 28, 0.6},
	{"Taş", 214, 224, 0.28},
	{"Gece", 238, 266, 1.0},
}

func rgbOf(c color.RGBA) [3]float64 { return [3]float64{float64(c.R), float64(c.G), float64(c.B)} }

// newAutoCover makes the mosaic of seed for a light or a dark page; the seed picks its
// theme, unless th is given (the app's theme, whose accent the covers then go with).
func newAutoCover(seed uint64, dark bool, th *coverTheme) *autoCover {
	rng := splitmix(seed)
	ac := &autoCover{style: int(rng.next() % mosaicKinds)}
	pick := int(rng.f() * float64(len(coverThemes)))
	if styleOf >= 0 {
		ac.style = styleOf
	}
	var k [16]float64
	for i := range k {
		k[i] = rng.f()
	}
	// the theme's two hues, each moved a little by the seed, so that two pages of a theme differ
	if th == nil {
		th = &coverThemes[pick]
	}
	h1, h2 := th.h1+(k[0]-0.5)*18, th.h2+(k[1]-0.5)*18
	ac.theme, ac.hue = pick, h1
	cA, cB, gapC := mosaicPalette(h1, h2, th.sat, dark)

	// the field: a slow wave across, a little cloud, and a lean from top to bottom
	theta := k[2] * math.Pi
	lambda := 800 + 900*k[3]
	lean := (k[4] - 0.5) * 0.9
	cloudScale := 1 / (150 + 120*k[5])
	field := func(x, y float64) float64 {
		g := 0.5 + 0.5*math.Sin((x*math.Cos(theta)+y*math.Sin(theta))/lambda*2*math.Pi+k[6]*6.28)
		n := 0.6*valueNoise(seed, x*cloudScale, y*cloudScale) + 0.4*valueNoise(seed+7, x*cloudScale*2.3, y*cloudScale*2.3)
		f := math.Min(math.Max(0.46*g+0.4*n+lean*(y/autoFullH-0.5)+0.1, 0), 1)
		// the foot goes to the end that is the page's own: near white in the light theme,
		// the deep end in the dark; the title lies there, and it blends into the page
		foot := smooth(0.40, 1, y/autoFullH) * 0.75
		if dark {
			return f + (1-f)*foot
		}
		return f * (1 - foot)
	}
	lerp := func(f float64) [3]float64 {
		return [3]float64{cA[0] + (cB[0]-cA[0])*f, cA[1] + (cB[1]-cA[1])*f, cA[2] + (cB[2]-cA[2])*f}
	}
	img := image.NewRGBA(image.Rect(0, 0, autoW, autoFullH))
	px := func(x, y int, c [3]float64, alpha float64) { // c over what is there, at alpha
		if x < 0 || y < 0 || x >= autoW || y >= autoFullH {
			return
		}
		o := img.PixOffset(x, y)
		for i := 0; i < 3; i++ {
			img.Pix[o+i] = uint8(float64(img.Pix[o+i])*(1-alpha) + c[i]*alpha)
		}
		img.Pix[o+3] = 255
	}
	rect := func(x0, y0, x1, y1 int, c [3]float64, alpha float64) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				px(x, y, c, alpha)
			}
		}
	}
	white, black := [3]float64{255, 255, 255}, [3]float64{0, 0, 0}
	edgeLight, edgeDark := white, black
	lightA, darkA := 0.50, 0.12 // how strong a light edge and a dark one are
	if dark {
		lightA, darkA = 0.16, 0.38
	}
	rect(0, 0, autoW, autoFullH, gapC, 1)

	switch ac.style {
	case mosaicEven, mosaicBig, mosaicLit:
		t := 20 + int(26*k[7]) // the tile's side
		gap := 2
		jit := 0.11
		switch ac.style {
		case mosaicBig:
			t, gap, jit = 64+int(70*k[7]), 1, 0.16
		case mosaicLit:
			t, gap, jit = 18+int(14*k[7]), 1, 0.09
		}
		for ty := 0; ty*t < autoFullH; ty++ {
			for tx := 0; tx*t < autoW; tx++ {
				x0, y0 := tx*t, ty*t
				cx, cy := float64(x0+t/2), float64(y0+t/2)
				jr := splitmix(seed ^ uint64(tx)*0x9e3779b97f4a7c15 ^ uint64(ty)*0xc2b2ae3d27d4eb4f)
				f := math.Min(math.Max(field(cx, cy)+(jr.f()-0.5)*2*jit, 0), 1)
				rect(x0, y0, x0+t-gap, y0+t-gap, lerp(f), 1)
				if ac.style == mosaicLit {
					rect(x0, y0, x0+t-gap, y0+1, edgeLight, lightA)           // the top edge is lit,
					rect(x0, y0, x0+1, y0+t-gap, edgeLight, lightA)           // and the left
					rect(x0, y0+t-gap-1, x0+t-gap, y0+t-gap, edgeDark, darkA) // the bottom is in shade,
					rect(x0+t-gap-1, y0, x0+t-gap, y0+t-gap, edgeDark, darkA) // and the right
				}
			}
		}
	default: // blocks: tiles merged into larger squares, each outlined
		t := 14 + int(10*k[7])
		cols, rows := (autoW+t-1)/t, (autoFullH+t-1)/t
		used := make([]bool, cols*rows)
		free := func(cx, cy, n int) bool {
			if cx+n > cols || cy+n > rows {
				return false
			}
			for y := cy; y < cy+n; y++ {
				for x := cx; x < cx+n; x++ {
					if used[y*cols+x] {
						return false
					}
				}
			}
			return true
		}
		big := 0.15 + 0.25*k[8] // how much of the mosaic is in larger blocks
		for cy := 0; cy < rows; cy++ {
			for cx := 0; cx < cols; cx++ {
				if used[cy*cols+cx] {
					continue
				}
				r := splitmix(seed ^ uint64(cx)*0x9e3779b97f4a7c15 ^ uint64(cy)*0xc2b2ae3d27d4eb4f ^ 0x55)
				n := 1
				switch roll := r.f(); {
				case roll < big*0.35:
					n = 3
				case roll < big:
					n = 2
				}
				for n > 1 && !free(cx, cy, n) {
					n--
				}
				for y := cy; y < cy+n; y++ {
					for x := cx; x < cx+n; x++ {
						used[y*cols+x] = true
					}
				}
				x0, y0, s := cx*t, cy*t, n*t
				f := math.Min(math.Max(field(float64(x0+s/2), float64(y0+s/2))+(r.f()-0.5)*0.14, 0), 1)
				rect(x0, y0, x0+s, y0+s, lerp(f), 1)
				// the outline: a light line on the top and the left, a darker one on the others
				rect(x0, y0, x0+s, y0+1, edgeLight, lightA*0.8)
				rect(x0, y0, x0+1, y0+s, edgeLight, lightA*0.8)
				rect(x0, y0+s-1, x0+s, y0+s, edgeDark, darkA*0.8)
				rect(x0+s-1, y0, x0+s, y0+s, edgeDark, darkA*0.8)
			}
		}
	}
	ac.img = ui.NewBitmap(img).Pixelated()
	return ac
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

// autoCoverOf is page p's mosaic for a light or a dark page, made the first time it is
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

// paint draws the mosaic across the band, one DIP to a pixel of it, its edges crisp; past
// the picture's width it goes on mirrored, so that there is no seam. Its foot fades into
// the page: fade is the page's own color.
func (ac *autoCover) paint(pt *ui.Painter, band ui.Rect, fade ui.Color) {
	pt.Clip(band, 0, func() {
		for i, x := 0, float32(0); x < band.W; i, x = i+1, x+autoW {
			w := float32(math.Min(autoW, float64(band.W-x)))
			src := ui.Rect{W: w, H: autoFullH}
			if i%2 == 1 { // mirrored: the part shown is the picture's end
				src.X = autoW - w
			}
			pt.ImageRegion(ac.img, src, ui.Rect{X: band.X + x, Y: band.Y, W: w, H: band.H}, ui.ImageOptions{FlipX: i%2 == 1})
		}
		const foot = 76 // how far up the fade reaches
		pt.FillGradient(ui.Rect{X: band.X, Y: band.Y + band.H - foot, W: band.W, H: foot}, ui.LinearGradient{Angle: 180, Oklab: true,
			Stops: []ui.GradientStop{{Color: fade.Alpha(0), At: 0}, {Color: fade.Alpha(0.55), At: 0.6}, {Color: fade, At: 1}}}, 0)
	})
}

func smooth(a, b, x float64) float64 {
	t := math.Min(math.Max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
