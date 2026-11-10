package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of nature, each fixed as it ships:
//
//	Bitki           every word opens a leaf on a stem over the caret; Enter opens a flower; erasing drops a leaf
//	Ateş böceği     writing gathers fireflies that circle the caret; resting, they drift off and go out

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
	idleFrame(p, f) // it sways
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
