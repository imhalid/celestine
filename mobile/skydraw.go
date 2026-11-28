package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// skyMotion says whether the sky's weather moves: only in a live window, when the system does not ask for less
// motion, and not while writing, so the page holds still under the pen. Set each frame by the view.
var skyMotion bool

// skyInset is the safe-area inset above the page, set each frame by Today: the moon and the falling weather are
// placed by the page's layout in dp from where the page starts, not by the height of the screen.
var skyInset float32

// skyHeld is the sky's time when it last moved, so the weather holds still where it was when it stops.
var skyHeld float64

// The frame rates the sky asks for while it moves: enough for each weather to look smooth, and no more.
const (
	fpsRain   = 24
	fpsSnow   = 20
	fpsClouds = 12
	fpsStars  = 6
)

// drawSky paints the frame's sky (curSky) into r: the gradient, the sun's or the moon's light, the stars, and the
// weather.
func drawSky(p *ui.Painter, r ui.Rect) {
	s := curSky
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: s.top, At: 0},
		{Color: s.top.MixOklab(s.bottom, 0.42), At: 0.38},
		{Color: s.bottom, At: 1},
	}}, 0)

	motion := skyMotion && !p.Preferences().ReduceMotion
	t := skyHeld
	if motion {
		t = float64(p.Now().UnixMilli()%86_400_000) / 1000
		skyHeld = t
	}
	fps := float32(0)
	ask := func(f float32) { fps = max(fps, f) }

	if s.sunA > 0.004 {
		sunGlow(p, r, s)
	}
	if s.starsA > 0.01 {
		stars(p, r, s, t)
		ask(fpsStars)
	}
	if s.moonA > 0.01 {
		moon(p, r, s)
	}

	switch s.kind {
	case weatherPartly:
		clouds(p, r, s, t, 3, 0.9, 0.5)
		ask(fpsClouds)
	case weatherCloudy:
		clouds(p, r, s, t, 7, 1.25, 0.62)
		ask(fpsClouds)
	case weatherFog:
		mist(p, r, s, t)
		ask(fpsClouds)
	case weatherRain:
		clouds(p, r, s, t, 5, 1.15, 0.5)
		rain(p, r, s, t, 70, 1)
		ask(fpsRain)
	case weatherStorm:
		clouds(p, r, s, t, 6, 1.3, 0.55)
		rain(p, r, s, t, 110, 1.35)
		if motion {
			flash(p, r, t)
		}
		ask(fpsRain)
	case weatherSnow:
		clouds(p, r, s, t, 4, 1.1, 0.42)
		snow(p, r, s, t)
		ask(fpsSnow)
	}
	if motion && fps > 0 {
		p.AnimationFrameRate(fps)
	}
}

// sunGlow is the sun's light, a wide soft bloom from where the sun stands on its arc.
func sunGlow(p *ui.Painter, r ui.Rect, s sky) {
	cx, cy := r.X+r.W*s.sunX, r.Y+r.H*s.sunY
	rad := r.W * 1.15
	a := s.sunA
	p.FillRadialGradient(r, ui.RadialGradient{
		Center: ui.Point{X: (cx - r.X) / r.W, Y: (cy - r.Y) / r.H},
		Radius: rad,
		Oklab:  true,
		Stops: []ui.GradientStop{
			{Color: s.sunGlow.Alpha(a), At: 0},
			{Color: s.sunGlow.Alpha(a * 0.55), At: 0.28},
			{Color: s.sunGlow.Alpha(a * 0.18), At: 0.62},
			{Color: s.sunGlow.Alpha(0), At: 1},
		},
	}, 0)
}

// moonX and moonDY place the moon between the day's numeral and the date: across the middle, and moonDY below
// the page's top, level with the numeral.
const (
	moonX  = 0.5
	moonDY = 98
	moonR  = 9
)

// moon is the moon in its real phase, with a cool halo and the dark part's faint earthshine.
func moon(p *ui.Painter, r ui.Rect, s sky) {
	cx, cy := r.X+r.W*moonX, r.Y+skyInset+moonDY
	a := s.moonA
	halo := ui.Hex("#C8D4FF")
	p.FillRadialGradient(ui.Rect{X: cx - 160, Y: cy - 160, W: 320, H: 320}, ui.RadialGradient{
		Center: ui.Point{X: 0.5, Y: 0.5}, Radius: 160, Oklab: true,
		Stops: []ui.GradientStop{
			{Color: halo.Alpha(0.16 * a), At: 0},
			{Color: halo.Alpha(0.05 * a), At: 0.4},
			{Color: halo.Alpha(0), At: 1},
		},
	}, 0)
	face := ui.Hex("#F3EEDC")
	p.FillPath(new(ui.Path).Circle(cx, cy, moonR), face.Alpha(0.1*a))
	if lit := moonLit(cx, cy, moonR, s.moonAge); lit != nil {
		p.FillPath(lit, face.Alpha(0.92*a))
	}
}

// moonLit returns the lit part of a moon of radius rad at (cx, cy) at phase age, or nil near new moon.
func moonLit(cx, cy, rad float32, age float32) *ui.Path {
	k := float32(math.Cos(2 * math.Pi * float64(age))) // 1 new, 0 half, -1 full
	if k > 0.985 {
		return nil
	}
	side := float32(1) // waxing: the right side is lit
	if age > 0.5 {
		side = -1
	}
	const n = 28
	path := new(ui.Path)
	for i := 0; i <= n; i++ { // the lit limb, top to bottom
		th := math.Pi * (float64(i)/n - 0.5)
		x := cx + side*rad*float32(math.Cos(th))
		y := cy + rad*float32(math.Sin(th))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	for i := n; i >= 0; i-- { // the terminator, bottom to top
		th := math.Pi * (float64(i)/n - 0.5)
		x := cx + side*rad*k*float32(math.Cos(th))
		y := cy + rad*float32(math.Sin(th))
		path.LineTo(x, y)
	}
	return path.Close()
}

// hash returns a steady pseudo-random number in [0, 1) for i and a salt, so the sky's pieces keep their places
// from frame to frame.
func hash(i, salt int) float32 {
	x := uint32(i)*0x9E3779B1 ^ uint32(salt)*0x85EBCA77
	x ^= x >> 15
	x *= 0x2C1B3C6D
	x ^= x >> 12
	x *= 0x297A2D39
	x ^= x >> 15
	return float32(x>>8) / float32(1<<24)
}

// stars are a scatter of small lights in the upper sky that slowly breathe.
func stars(p *ui.Painter, r ui.Rect, s sky, t float64) {
	const n = 52
	for i := 0; i < n; i++ {
		x := r.X + r.W*hash(i, 1)
		fy := hash(i, 2) * hash(i, 2) * 0.62
		y := r.Y + r.H*fy
		if math.Abs(float64(x-(r.X+r.W*moonX))) < 22 && math.Abs(float64(y-(r.Y+skyInset+moonDY))) < 22 {
			continue
		}
		size := 0.55 + hash(i, 3)*0.95
		tw := float32(0.72 + 0.28*math.Sin(t*(0.6+float64(hash(i, 4))*1.4)+float64(hash(i, 5))*6.28))
		a := s.starsA * (0.25 + 0.6*hash(i, 6)) * (1 - fy*1.2) * tw
		if a <= 0.01 {
			continue
		}
		p.Fill(ui.Rect{X: x - size, Y: y - size, W: size * 2, H: size * 2}, ui.Hex("#F4F1FF").Alpha(a), size)
	}
}

// cloudColor is the clouds' color on this sky: lit white by day, moonlit grey-blue at night, slate in rain.
func cloudColor(s sky) ui.Color {
	if s.dark {
		return s.top.MixOklab(ui.Hex("#A9B5D2"), 0.26)
	}
	switch s.kind {
	case weatherRain, weatherStorm:
		return s.top.MixOklab(ui.Hex("#FFFFFF"), 0.35)
	}
	return s.top.MixOklab(ui.Hex("#FFFFFF"), 0.78)
}

// clouds draws n soft clouds drifting across the upper sky, each a few overlapping blooms.
func clouds(p *ui.Painter, r ui.Rect, s sky, t float64, n int, scale, alpha float32) {
	col := cloudColor(s)
	if s.dark {
		alpha *= 0.55
	}
	puffs := [5][3]float32{{-0.95, 0.18, 0.55}, {-0.38, -0.12, 0.78}, {0.22, -0.22, 0.9}, {0.82, 0.08, 0.62}, {0.05, 0.28, 0.72}}
	for i := 0; i < n; i++ {
		base := (70 + 50*hash(i, 11)) * scale
		span := r.W + 4*base
		speed := 2.5 + 3.5*hash(i, 12)
		x := r.X - 2*base + float32(math.Mod(float64((float32(i)+0.6*hash(i, 13))/float32(n)*span)+t*float64(speed), float64(span)))
		y := r.Y + r.H*(0.03+0.24*hash(i, 14))
		for j, pf := range puffs {
			rad := base * pf[2] * (0.85 + 0.3*hash(i*7+j, 15))
			px, py := x+pf[0]*base*1.1, y+pf[1]*base*0.8
			a := alpha * (0.55 + 0.45*hash(i*7+j, 16))
			p.FillRadialGradient(ui.Rect{X: px - rad, Y: py - rad, W: rad * 2, H: rad * 2}, ui.RadialGradient{
				Center: ui.Point{X: 0.5, Y: 0.5}, Radius: rad, Oklab: true,
				Stops: []ui.GradientStop{
					{Color: col.Alpha(a), At: 0},
					{Color: col.Alpha(a * 0.78), At: 0.45},
					{Color: col.Alpha(0), At: 1},
				},
			}, 0)
		}
	}
}

// mist is three soft bands of haze that breathe in and out across the page.
func mist(p *ui.Painter, r ui.Rect, s sky, t float64) {
	col := ui.Hex("#FFFFFF")
	a := float32(0.6)
	if s.dark {
		col, a = s.top.MixOklab(ui.Hex("#B4C0CC"), 0.4), 0.24
	}
	for i, fy := range []float32{0.1, 0.4, 0.72} {
		breathe := float32(0.75 + 0.25*math.Sin(t*0.18+float64(i)*2.1))
		h := r.H * 0.2
		y := r.Y + r.H*fy + float32(math.Sin(t*0.07+float64(i)))*10
		p.FillGradient(ui.Rect{X: r.X, Y: y, W: r.W, H: h}, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
			{Color: col.Alpha(0), At: 0},
			{Color: col.Alpha(a * breathe), At: 0.5},
			{Color: col.Alpha(0), At: 1},
		}}, 0)
	}
}

// rain is n fine slanted streaks falling, fading as they come down toward the text.
func rain(p *ui.Painter, r ui.Rect, s sky, t float64, n int, weight float32) {
	col := s.top.MixOklab(ui.Hex("#22324E"), 0.62)
	base := float32(0.2)
	if s.dark {
		col, base = ui.Hex("#C9D6F2"), 0.17
	}
	const slant = 0.2 // horizontal per vertical
	for i := 0; i < n; i++ {
		l := (14 + 16*hash(i, 21)) * weight
		speed := 520 + 300*hash(i, 22)
		span := r.H + l
		fy := float32(math.Mod(float64(hash(i, 23)*span)+t*float64(speed), float64(span))) - l
		x := r.X + float32(math.Mod(float64(r.W*hash(i, 24)+fy*slant), float64(r.W)))
		y := r.Y + fy
		depth := 0.5 + 0.5*hash(i, 25)
		a := base * depth * weight * veil(fy-skyInset)
		if a <= 0.01 {
			continue
		}
		p.Line(x, y, x+l*slant, y+l, 0.8+0.5*depth, col.Alpha(min(a, 0.4)))
	}
}

// snow is slow flakes that sway as they fall.
func snow(p *ui.Painter, r ui.Rect, s sky, t float64) {
	col, a0 := ui.Hex("#FFFFFF"), float32(0.92)
	shade := s.top.MixOklab(ui.Hex("#5A6E8C"), 0.5)
	if s.dark {
		a0 = 0.78
	}
	const n = 60
	for i := 0; i < n; i++ {
		size := 1.1 + 2.1*hash(i, 31)
		speed := 16 + 30*hash(i, 32)*size/3
		fy := float32(math.Mod(float64(hash(i, 33)*r.H)+t*float64(speed), float64(r.H)))
		x := r.X + r.W*hash(i, 34) + float32(math.Sin(t*(0.4+float64(hash(i, 35)))+float64(i)))*12
		y := r.Y + fy
		a := a0 * (0.55 + 0.45*hash(i, 36)) * veil(fy-skyInset)
		if !s.dark {
			p.Fill(ui.Rect{X: x - size, Y: y - size + 0.6, W: size * 2, H: size * 2}, shade.Alpha(0.16*a), size)
		}
		soft := size * 2 // a flake is a bright core in a soft rim, as seen through the air
		p.FillRadialGradient(ui.Rect{X: x - soft, Y: y - soft, W: soft * 2, H: soft * 2}, ui.RadialGradient{
			Center: ui.Point{X: 0.5, Y: 0.5}, Radius: soft,
			Stops: []ui.GradientStop{
				{Color: col.Alpha(a), At: 0},
				{Color: col.Alpha(a * 0.85), At: 0.38},
				{Color: col.Alpha(0), At: 1},
			},
		}, 0)
	}
}

// flash is a far lightning flash, a double pulse every so often, lighting the sky from above.
func flash(p *ui.Painter, r ui.Rect, t float64) {
	const every = 11.3
	u := math.Mod(t, every)
	var k float64
	switch {
	case u < 0.14:
		k = 1 - u/0.14
	case u > 0.24 && u < 0.5:
		k = 0.7 * (1 - (u-0.24)/0.26)
	default:
		return
	}
	at := 0.2 + 0.6*float32(hash(int(t/every), 41))
	p.FillRadialGradient(r, ui.RadialGradient{
		Center: ui.Point{X: at, Y: 0.02}, Radius: r.W * 1.3, Oklab: true,
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#E9E6FF").Alpha(float32(0.32 * k)), At: 0},
			{Color: ui.Hex("#E9E6FF").Alpha(0), At: 1},
		},
	}, 0)
}

// veil is how much of the falling weather shows y dp below the page's top: all of it above the date, little
// where the text is written, so rain and snow fall in the sky and not between the words.
func veil(y float32) float32 {
	k := clamp32((y-110)/160, 0, 1)
	k = k * k * (3 - 2*k)
	return 1 - 0.82*k
}
