package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Carets of nature, each fixed as it ships:
//
//	Bitki           every word opens a leaf on a stem over the caret; Enter opens a flower; erasing drops a leaf
//	Ateş böceği     writing gathers fireflies that circle the caret; resting, they drift off and go out
//	Yağmur damlası  a drop: a space sends a ripple round it, Enter lets a drop fall and splash, a move leaves drips
//	Mevsim          the hour's: dew at dawn, the sun's rays by day, falling leaves at dusk, stars at night

// ---- Bitki ----

type plant struct {
	fxBase
	leaves []float64 // when each leaf opened
	bloom  float64   // when the flower opened; < 0: none
	falls  motes
}

func newPlant() *plant { return &plant{bloom: -1} }

func (g *plant) reset() { g.leaves, g.bloom, g.falls, g.have = nil, -1, nil, false }

func (g *plant) shift(dy float32) { g.falls.shift(dy); g.prev.Y += dy }

func (g *plant) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	leaf, stem := ui.Hex("#22c55e"), ui.Hex("#15803d")
	if g.bloom >= 0 && tn-g.bloom > 1600 { // the flower has been seen: a new stem
		g.leaves, g.bloom = nil, -1
	}
	if n := strings.Count(k.typed, " "); n > 0 && len(g.leaves) < 8 && g.bloom < 0 {
		for i := 0; i < n && len(g.leaves) < 8; i++ {
			g.leaves = append(g.leaves, tn)
		}
	}
	if strings.Contains(k.typed, "\n") && g.bloom < 0 {
		g.bloom = tn
	}
	for i := 0; i < k.erased && len(g.leaves) > 0; i++ { // a leaf falls
		g.leaves = g.leaves[:len(g.leaves)-1]
		g.falls = append(g.falls, mote{x: float64(box.X + box.W/2), y: float64(box.Y) - h*0.2*float64(len(g.leaves)+1), vx: 18, vy: 10,
			spin: 3, t0: tn, life: 1200, size: h * 0.28, grav: 40, col: leaf})
	}
	// the stem grows as tall as its leaves, from the top of the caret
	cx := box.X + box.W/2
	top := float64(box.Y)
	tall := h * (0.25 + 0.22*float64(len(g.leaves)))
	if g.bloom >= 0 {
		tall += h * 0.2
	}
	sway := math.Sin(tn/1000*1.6) * h * 0.06
	if len(g.leaves) > 0 || g.bloom >= 0 {
		st := &ui.Path{}
		st.MoveTo(cx, f32(top)).QuadTo(cx+f32(sway), f32(top-tall/2), cx+f32(sway*1.5), f32(top-tall))
		p.StrokePath(st, max(1.2, box.W*0.14), stem)
	}
	for i, at := range g.leaves {
		grow := clamp01((tn - at) / 280)
		y := top - h*0.22*float64(i+1)
		side := float32(1)
		if i%2 == 1 {
			side = -1
		}
		x := float64(cx) + sway*float64(i+1)/float64(len(g.leaves)+1)
		l := h * 0.3 * easeOut(grow)
		lf := &ui.Path{}
		lf.MoveTo(f32(x), f32(y)).QuadTo(f32(x)+side*f32(l*0.6), f32(y-l*0.5), f32(x)+side*f32(l), f32(y-l*0.15)).
			QuadTo(f32(x)+side*f32(l*0.5), f32(y+l*0.15), f32(x), f32(y)).Close()
		p.FillPath(lf, withAlpha(leaf, 0.95))
	}
	if g.bloom >= 0 { // the flower: petals open round a yellow heart
		k := easeOut(clamp01((tn - g.bloom) / 450))
		fx, fy := float64(cx)+sway*1.5, top-tall
		r := h * 0.22 * k
		for i := 0; i < 6; i++ {
			a := float64(i)/6*2*math.Pi + tn/3000
			px, py := fx+math.Cos(a)*r, fy+math.Sin(a)*r
			pr := r * 0.75
			p.Fill(ui.Rect{X: f32(px - pr), Y: f32(py - pr), W: f32(2 * pr), H: f32(2 * pr)}, ui.Hex("#f472b6"), f32(pr))
		}
		cr := r * 0.6
		p.Fill(ui.Rect{X: f32(fx - cr), Y: f32(fy - cr), W: f32(2 * cr), H: f32(2 * cr)}, ui.Hex("#facc15"), f32(cr))
	}
	g.falls.stepDraw(p, tn, dt, func(m *mote, age float64) {
		m.vx = 18 * math.Sin((tn-m.t0)/180) // it sways down
		s := f32(m.size)
		lf := &ui.Path{}
		lf.MoveTo(0, 0).QuadTo(s*0.6, -s*0.5, s, -s*0.15).QuadTo(s*0.5, s*0.15, 0, 0).Close()
		p.Transform(ui.Rotation(deg(m.ang)).Then(ui.Translation(f32(m.x), f32(m.y))), 0, 0, func() { p.FillPath(lf, withAlpha(m.col, 1-age)) })
	})
	glassBody(p, f, box, leaf, 1, 0.8)
	p.AnimationFrame() // it sways
}

func easeOut(x float64) float64 { x = clamp01(x); return 1 - (1-x)*(1-x)*(1-x) }

// ---- Ateş böceği ----

type firefly struct {
	ang, r, speed, phase float64
	born, leaving        float64
	away                 bool
}

type fireflies struct {
	fxBase
	flies []firefly
	count float64 // letters toward the next one
}

func newFireflies() *fireflies { return &fireflies{} }

func (g *fireflies) reset() { g.flies, g.have = nil, false }

func (g *fireflies) shift(dy float32) { g.prev.Y += dy }

func (g *fireflies) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.count += float64(k.wrote)
	for g.count >= 4 && len(g.flies) < 14 {
		g.count -= 4
		g.flies = append(g.flies, firefly{ang: rnd() * 2 * math.Pi, r: h * (0.9 + rnd()*1.2), speed: (0.6 + rnd()) * map[bool]float64{true: 1, false: -1}[rnd() > 0.5], phase: rnd() * 6, born: tn})
	}
	if g.idle(tn) > 2.2 { // resting: they drift away
		for i := range g.flies {
			if !g.flies[i].away {
				g.flies[i].away, g.flies[i].leaving = true, tn+rnd()*600
			}
		}
	}
	if k.wrote > 0 {
		for i := range g.flies {
			g.flies[i].away = false
		}
	}
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	yellow := ui.Hex("#fde047")
	out := g.flies[:0]
	for _, fl := range g.flies {
		fl.ang += fl.speed * dt
		a := clamp01((tn - fl.born) / 400)
		if fl.away && tn > fl.leaving {
			fl.r += h * 2 * dt
			a *= 1 - clamp01((tn-fl.leaving)/1200)
			if a <= 0 {
				continue
			}
		}
		x := cx + math.Cos(fl.ang)*fl.r
		y := cy + math.Sin(fl.ang*1.3+fl.phase)*fl.r*0.55
		pulse := 0.55 + 0.45*math.Sin(tn/1000*4+fl.phase)
		for i, s := range []float64{5, 3, 1.6} { // its glow, and its light
			al := a * pulse * []float64{0.12, 0.3, 1}[i]
			p.Fill(ui.Rect{X: f32(x - s), Y: f32(y - s), W: f32(2 * s), H: f32(2 * s)}, withAlpha(yellow, al), f32(s))
		}
		out = append(out, fl)
	}
	g.flies = out
	glassBody(p, f, box, mix(yellow, ui.Hex("#a16207"), 0.35), 1, 1.1)
	if len(g.flies) > 0 {
		p.AnimationFrame()
	}
}

// ---- Yağmur damlası ----

type raindrop struct {
	fxBase
	ripples []float64 // when each ripple began
	rx, ry  []float64
	falls   motes // the drop that falls, and its splash
	drips   motes
}

func newRaindrop() *raindrop { return &raindrop{} }

func (g *raindrop) reset() {
	g.ripples, g.rx, g.ry, g.falls, g.drips, g.have = nil, nil, nil, nil, nil, false
}

func (g *raindrop) shift(dy float32) {
	for i := range g.ry {
		g.ry[i] += float64(dy)
	}
	g.falls.shift(dy)
	g.drips.shift(dy)
	g.prev.Y += dy
}

func (g *raindrop) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, moved, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	blue := ui.Hex("#38bdf8")
	cx, base := float64(box.X+box.W/2), float64(box.Y+box.H)
	if strings.Contains(k.typed, " ") {
		g.ripples, g.rx, g.ry = append(g.ripples, tn), append(g.rx, cx), append(g.ry, base)
	}
	if strings.Contains(k.typed, "\n") { // a drop falls from where the caret was
		g.falls = append(g.falls, mote{x: float64(g.prev.X), y: float64(g.prev.Y), vy: 60, t0: tn, life: 520, size: h * 0.3, grav: 1400, col: blue, kind: 1})
	}
	if moved && rnd() < 0.6 {
		g.drips = append(g.drips, mote{x: cx + (rnd()-0.5)*4, y: base - 2, vy: 20, t0: tn, life: 700, size: 1.5 + rnd()*1.5, grav: 120, col: blue})
	}
	out, ox, oy := g.ripples[:0], g.rx[:0], g.ry[:0]
	for i, at := range g.ripples {
		k := (tn - at) / 900
		if k >= 1 {
			continue
		}
		for j := 0; j < 2; j++ { // two rings, one after the other
			kk := k - float64(j)*0.18
			if kk <= 0 {
				continue
			}
			rw, rh := h*1.6*easeOut(kk), h*0.35*easeOut(kk)
			ring := &ui.Path{}
			ring.MoveTo(f32(g.rx[i]+rw), f32(g.ry[i]))
			ring.CubeTo(f32(g.rx[i]+rw), f32(g.ry[i]+rh*1.33), f32(g.rx[i]-rw), f32(g.ry[i]+rh*1.33), f32(g.rx[i]-rw), f32(g.ry[i]))
			ring.CubeTo(f32(g.rx[i]-rw), f32(g.ry[i]-rh*1.33), f32(g.rx[i]+rw), f32(g.ry[i]-rh*1.33), f32(g.rx[i]+rw), f32(g.ry[i]))
			p.StrokePath(ring, 1.2, withAlpha(blue, (1-kk)*0.8))
		}
		out, ox, oy = append(out, at), append(ox, g.rx[i]), append(oy, g.ry[i])
	}
	g.ripples, g.rx, g.ry = out, ox, oy
	var splash []mote
	g.falls.stepDraw(p, tn, dt, func(m *mote, age float64) {
		if m.kind == 1 && age > 0.6 { // it lands: a splash
			m.t0 = -1e9
			for i := 0; i < 6; i++ {
				a := -math.Pi/2 + (rnd()-0.5)*2.2
				splash = append(splash, mote{x: m.x, y: m.y, vx: math.Cos(a) * 90, vy: math.Sin(a) * 120, t0: tn, life: 420, size: 1.8, grav: 600, col: m.col})
			}
			return
		}
		s := f32(m.size)
		if m.kind == 1 {
			p.Fill(ui.Rect{X: f32(m.x) - s/2, Y: f32(m.y) - s, W: s, H: s * 1.4}, withAlpha(m.col, 0.9), s/2)
		} else {
			p.Fill(ui.Rect{X: f32(m.x) - s/2, Y: f32(m.y) - s/2, W: s, H: s}, withAlpha(m.col, 1-age), s/2)
		}
	})
	g.falls = append(g.falls, splash...)
	g.drips.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x) - s/2, Y: f32(m.y), W: s, H: s * 1.3}, withAlpha(m.col, 0.7*(1-age)), s/2)
	})
	// the drop: a box whose top comes to a point
	drop := &ui.Path{}
	x, y, w, hh := box.X, box.Y, box.W, box.H
	drop.MoveTo(x+w/2, y-hh*0.18).CubeTo(x+w*0.55, y+hh*0.12, x+w, y+hh*0.35, x+w, y+hh*0.62).
		CubeTo(x+w, y+hh*0.9, x+w*0.78, y+hh, x+w/2, y+hh).CubeTo(x+w*0.22, y+hh, x, y+hh*0.9, x, y+hh*0.62).
		CubeTo(x, y+hh*0.35, x+w*0.45, y+hh*0.12, x+w/2, y-hh*0.18).Close()
	glowAround(p, box, blue, 0.6)
	glassFill(p, drop, blue, blue, f.dark, 1) // glass, as water is
	p.Fill(ui.Rect{X: x + w*0.25, Y: y + hh*0.4, W: max(1, w*0.16), H: hh * 0.22}, ui.RGBA(255, 255, 255, 0.7), w*0.08)
	if len(g.ripples) > 0 || len(g.falls) > 0 || len(g.drips) > 0 {
		p.AnimationFrame()
	}
}

// ---- Mevsim ----

type season struct {
	fxBase
	leaves motes
	stars  [8][3]float64 // where each star is around the caret, and its phase
	placed bool
	now    func() time.Time
}

func newSeason() *season { return &season{now: time.Now} }

func (g *season) reset() { g.leaves, g.have = nil, false }

func (g *season) shift(dy float32) { g.leaves.shift(dy); g.prev.Y += dy }

// hourPart is the part of the day: 0 dawn, 1 day, 2 dusk, 3 night.
func hourPart(t time.Time) int {
	switch h := t.Hour(); {
	case h >= 5 && h < 9:
		return 0
	case h >= 9 && h < 17:
		return 1
	case h >= 17 && h < 20:
		return 2
	}
	return 3
}

func (g *season) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if !g.placed {
		for i := range g.stars {
			g.stars[i] = [3]float64{(rnd() - 0.5) * 5, (rnd() - 0.8) * 3, rnd() * 6}
		}
		g.placed = true
	}
	var col ui.Color
	switch hourPart(g.now()) {
	case 0: // dawn: dew that glints on it
		col = ui.Hex("#fda4af")
		for i := 0; i < 3; i++ {
			gl := math.Max(0, math.Sin(tn/1000*1.7+float64(i)*2.1))
			x, y := float64(box.X)+float64(box.W)*[]float64{0.2, 0.7, 0.45}[i], float64(box.Y)+h*[]float64{0.15, 0.4, 0.75}[i]
			s := 1.2 + gl*1.5
			p.Fill(ui.Rect{X: f32(x - s), Y: f32(y - s), W: f32(2 * s), H: f32(2 * s)}, ui.RGBA(255, 255, 255, float32(0.4+0.6*gl)), f32(s))
		}
	case 1: // day: the sun's rays, turning
		col = ui.Hex("#fbbf24")
		for i := 0; i < 8; i++ {
			a := float64(i)/8*2*math.Pi + tn/2500
			r0, r1 := h*0.75, h*(0.95+0.15*math.Sin(tn/400+float64(i)))
			ray := &ui.Path{}
			ray.MoveTo(f32(cx+math.Cos(a)*r0), f32(cy+math.Sin(a)*r0)).LineTo(f32(cx+math.Cos(a)*r1), f32(cy+math.Sin(a)*r1))
			p.StrokePath(ray, 1.5, withAlpha(col, 0.7))
		}
	case 2: // dusk: an orange glow, and leaves that fall now and then
		col = ui.Hex("#f97316")
		if rnd() < dt*1.4 || k.wrote > 0 && rnd() < 0.15 {
			g.leaves = append(g.leaves, mote{x: cx + (rnd()-0.5)*h*2, y: cy - h*1.5, vx: 10, vy: 15, spin: 2 + rnd()*2, t0: tn, life: 2200, size: h * 0.25, grav: 10,
				col: []ui.Color{ui.Hex("#ea580c"), ui.Hex("#ca8a04"), ui.Hex("#b91c1c")}[int(rnd()*3)%3]})
		}
		g.leaves.stepDraw(p, tn, dt, func(m *mote, age float64) {
			m.vx = 14 * math.Sin((tn-m.t0)/300)
			s := f32(m.size)
			lf := &ui.Path{}
			lf.MoveTo(0, 0).QuadTo(s*0.6, -s*0.5, s, 0).QuadTo(s*0.6, s*0.5, 0, 0).Close()
			p.Transform(ui.Rotation(deg(m.ang)).Then(ui.Translation(f32(m.x), f32(m.y))), 0, 0, func() { p.FillPath(lf, withAlpha(m.col, 0.9*(1-age))) })
		})
	default: // night: stars that twinkle round it, the caret the moon's
		col = ui.Hex("#c4b5fd")
		for _, st := range g.stars {
			tw := 0.5 + 0.5*math.Sin(tn/1000*2.4+st[2])
			x, y := cx+st[0]*h, cy+st[1]*h
			s := 0.8 + tw*1.3
			star := &ui.Path{}
			star.MoveTo(f32(x), f32(y-s*2)).LineTo(f32(x+s*0.5), f32(y-s*0.5)).LineTo(f32(x+s*2), f32(y)).LineTo(f32(x+s*0.5), f32(y+s*0.5)).
				LineTo(f32(x), f32(y+s*2)).LineTo(f32(x-s*0.5), f32(y+s*0.5)).LineTo(f32(x-s*2), f32(y)).LineTo(f32(x-s*0.5), f32(y-s*0.5)).Close()
			p.FillPath(star, ui.RGBA(255, 255, 240, float32(0.3+0.7*tw)))
		}
	}
	glassBody(p, f, box, col, 1, 1)
	p.AnimationFrame() // the hour's things move
}
