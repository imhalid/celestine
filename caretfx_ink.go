package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of ink and paper, each fixed as it ships:
//
//	Dolma kalem  a nib: what it writes leaves a wet line of ink under it that dries; quick writing spatters; erasing sheds crumbs
//	Daktilo      every key jolts it; Enter sends the carriage back with a ding
//	Mühür        at every space it presses down and leaves an ink ring that sets and fades

// ---- Dolma kalem ----

type inkLine struct {
	x0, x1, y, t0 float64
}

type pen struct {
	fxBase
	lines  []inkLine
	splats motes
	crumbs motes
	rate   float64
}

func newPen() *pen { return &pen{} }

func (g *pen) reset() { g.lines, g.splats, g.crumbs, g.have = nil, nil, nil, false }

func (g *pen) shift(dy float32) {
	for i := range g.lines {
		g.lines[i].y += float64(dy)
	}
	g.splats.shift(dy)
	g.crumbs.shift(dy)
	g.prev.Y += dy
}

func (g *pen) draw(p *ui.Painter, f caretFrame, k kick) {
	was := g.prev
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	ink := ui.Hex("#1e3a8a")
	if f.dark {
		ink = ui.Hex("#93c5fd")
	}
	base := float64(box.Y+box.H) + 1.5
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*4)
	if k.wrote > 0 && was.H > 0 && math.Abs(float64(was.Y-box.Y)) < 2 { // under what was just written
		g.lines = append(g.lines, inkLine{x0: float64(was.X), x1: float64(box.X), y: base, t0: tn})
		if g.rate > 9 && rnd() < 0.5 { // writing fast: it spatters
			for i := 0; i < 3; i++ {
				g.splats = append(g.splats, mote{x: float64(box.X) + (rnd()-0.5)*h, y: base + (rnd()-0.3)*h*0.4, t0: tn, life: 2400, size: 0.8 + rnd()*1.8, col: ink})
			}
		}
	}
	if k.erased > 0 { // the eraser's crumbs
		for i := 0; i < 4; i++ {
			g.crumbs = append(g.crumbs, mote{x: float64(box.X) + rnd()*h*0.5, y: float64(box.Y) + h*0.5, vx: (rnd() - 0.5) * 60, vy: -30 - rnd()*30, t0: tn, life: 700, size: 1 + rnd()*1.5, grav: 400,
				col: ui.Hex("#f9a8d4")})
		}
	}
	out := g.lines[:0]
	for _, l := range g.lines {
		age := (tn - l.t0) / 5000
		if age >= 1 {
			continue
		}
		wet := 1 - clamp01((tn-l.t0)/1600) // wet, it is dark and glossy; dry, it is lighter, then gone
		a := (0.35 + 0.55*wet) * (1 - smoothstep((age-0.6)/0.4))
		th := f32(1.2 + 0.8*wet)
		p.Fill(ui.Rect{X: f32(l.x0), Y: f32(l.y) - th/2, W: f32(l.x1 - l.x0), H: th}, withAlpha(ink, a), th/2)
		if wet > 0.3 {
			p.Fill(ui.Rect{X: f32(l.x0), Y: f32(l.y) - th/2, W: f32(l.x1 - l.x0), H: max(0.5, th*0.3)}, ui.RGBA(255, 255, 255, float32(0.35*wet)), 0)
		}
		out = append(out, l)
	}
	g.lines = out
	g.splats.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s, W: 2 * s, H: 2 * s}, withAlpha(m.col, 0.7*(1-age*age)), s)
	})
	g.crumbs.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x), Y: f32(m.y), W: s, H: s}, withAlpha(m.col, 1-age), 0.5)
	})
	// the nib: the box, its lower end a point, a slit down its middle
	x, y, w, hh := box.X, box.Y, box.W, box.H
	nib := &ui.Path{}
	nib.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+hh*0.62).LineTo(x+w/2, y+hh).LineTo(x, y+hh*0.62).Close()
	glowAround(p, box, ink, 0.5)
	glassFill(p, nib, ink, ink, f.dark, 1) // glass, as the slime is
	glassGloss(p, box, f.dark, 1)
	p.Fill(ui.Rect{X: x + w/2 - 0.5, Y: y + hh*0.35, W: 1, H: hh * 0.55}, withAlpha(f.bg, 0.9), 0)
	p.Fill(ui.Rect{X: x + w/2 - 1.2, Y: y + hh*0.32, W: 2.4, H: 2.4}, withAlpha(f.bg, 0.9), 1.2)
	if len(g.lines) > 0 || len(g.splats) > 0 || len(g.crumbs) > 0 {
		p.AnimationFrame()
	}
}

// ---- Daktilo ----

type typewriter struct {
	fxBase
	jolt  float64 // when a key last struck
	ret   float64 // when the carriage went back
	retX  float64 // from where
	dings motes
}

func newTypewriter() *typewriter { return &typewriter{jolt: -1e9, ret: -1e9} }

func (g *typewriter) reset() { g.dings, g.have = nil, false }

func (g *typewriter) shift(dy float32) { g.dings.shift(dy); g.prev.Y += dy }

func (g *typewriter) draw(p *ui.Painter, f caretFrame, k kick) {
	was := g.prev
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	metal := ui.Hex("#57534e")
	if f.dark {
		metal = ui.Hex("#d6d3d1")
	}
	if k.wrote > 0 {
		g.jolt = tn
	}
	if strings.Contains(k.typed, "\n") && was.H > 0 { // the carriage goes back: ding
		g.ret, g.retX = tn, float64(was.X)
		g.dings = append(g.dings, mote{x: float64(box.X) + h, y: float64(box.Y) - h*0.2, vy: -25, t0: tn, life: 900, size: h * 0.5})
	}
	jk := (tn - g.jolt) / 90
	b := box
	if jk < 1 { // the strike: down, and back
		b.Y += f32(math.Sin(jk*math.Pi) * h * 0.08)
		for _, dx := range []float64{-1, 1} { // a tick each side
			x := float64(b.X+b.W/2) + dx*float64(b.W)*0.9
			p.Fill(ui.Rect{X: f32(x) - 0.5, Y: b.Y - f32(h*0.15), W: 1, H: f32(h * 0.2)}, withAlpha(metal, 0.6*(1-jk)), 0)
		}
	}
	if rk := (tn - g.ret) / 380; rk < 1 { // the carriage: a streak from where the line ended to where the next begins
		e := easeOut(rk)
		x := g.retX + (float64(box.X)-g.retX)*e
		p.Fill(ui.Rect{X: f32(math.Min(x, g.retX)), Y: box.Y + box.H*0.45, W: f32(math.Abs(g.retX - x)), H: max(1, box.H*0.1)}, withAlpha(metal, 0.35*(1-rk)), 1)
		b.X = f32(x)
	}
	g.dings.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "ding!", f32(m.x), f32(m.y), f32(m.size), withAlpha(metal, 1-age), true, 0, 1+0.2*age)
	})
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, metal, 1, 0.6)
	if jk < 1 || tn-g.ret < 380 || len(g.dings) > 0 {
		p.AnimationFrame()
	}
}

// ---- Mühür ----

type stampMark struct {
	x, y, t0, ang float64
}

type stamp struct {
	fxBase
	marks []stampMark
	press float64
}

func newStamp() *stamp { return &stamp{press: -1e9} }

func (g *stamp) reset() { g.marks, g.have = nil, false }

func (g *stamp) shift(dy float32) {
	for i := range g.marks {
		g.marks[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *stamp) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	red := ui.Hex("#dc2626")
	if strings.Contains(k.typed, " ") {
		g.press = tn
		g.marks = append(g.marks, stampMark{x: float64(box.X) - h*0.4, y: float64(box.Y + box.H/2), t0: tn, ang: (rnd() - 0.5) * 0.5})
		if len(g.marks) > 12 {
			g.marks = g.marks[1:]
		}
	}
	out := g.marks[:0]
	for _, m := range g.marks {
		age := (tn - m.t0) / 2200
		if age >= 1 {
			continue
		}
		set := easeOut(clamp01((tn - m.t0) / 160))
		r := h * 0.55 * (1.15 - 0.15*set) // it lands a little wide, and settles
		a := 0.38 * (1 - smoothstep((age-0.5)/0.5))
		ring := &ui.Path{}
		const n = 28
		for i := 0; i <= n; i++ { // a ring, a little uneven, as ink is
			t := float64(i) / n * 2 * math.Pi
			rr := r * (1 + 0.05*math.Sin(t*5+m.ang*9))
			x, y := m.x+math.Cos(t+m.ang)*rr, m.y+math.Sin(t+m.ang)*rr
			if i == 0 {
				ring.MoveTo(f32(x), f32(y))
			} else {
				ring.LineTo(f32(x), f32(y))
			}
		}
		p.StrokePath(ring, f32(math.Max(1.2, h*0.08)), withAlpha(red, a))
		glyphAt(p, "✓", f32(m.x), f32(m.y), f32(h*0.55), withAlpha(red, a), true, m.ang*57, 1)
		out = append(out, m)
	}
	g.marks = out
	b := box
	if pk := (tn - g.press) / 220; pk < 1 { // pressed down: squashed, then up
		s := math.Sin(pk * math.Pi)
		b.H *= f32(1 - 0.25*s)
		b.Y += box.H - b.H
		b.X -= f32(s * 1.5)
		b.W += f32(s * 3)
	}
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, red, 1, 0.8)
	if len(g.marks) > 0 {
		p.AnimationFrame()
	}
}
