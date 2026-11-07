package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Carets of motion, each fixed as it ships:
//
//	Yay              the box on a spring after the caret: a coil between them, a bounce past where it goes
//	Kuyruklu yıldız  a bright head with a tail that grows with the speed of writing and moving, shedding sparkles
//	Mıknatıs         iron filings round the caret: drawn in and lined up as it writes, pushed away as it erases,
//	                 loose when it rests (the letters themselves are the text's: they are not moved)

// ---- Yay ----

type spring struct {
	fxBase
	x, y, vx, vy float64
	placed       bool
}

func newSpring() *spring { return &spring{} }

func (g *spring) reset() { g.placed, g.have = false, false }

func (g *spring) shift(dy float32) { g.y += float64(dy); g.prev.Y += dy }

func (g *spring) draw(p *ui.Painter, f caretFrame, k kick) {
	_, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	tx, ty := float64(box.X), float64(box.Y)
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	if k.wrote > 0 { // a key: a little kick up
		g.vy -= 90
	}
	const w0, z = 15.0, 0.28 // a soft spring, little damping: it bounces
	steps := int(math.Ceil(dt * 240))
	hh := dt / float64(steps)
	for i := 0; i < steps; i++ {
		g.vx += (w0*w0*(tx-g.x) - 2*z*w0*g.vx) * hh
		g.vy += (w0*w0*(ty-g.y) - 2*z*w0*g.vy) * hh
		g.x += g.vx * hh
		g.y += g.vy * hh
	}
	dx, dy := g.x-tx, g.y-ty
	settled := math.Hypot(dx, dy) < 0.3 && math.Hypot(g.vx, g.vy) < 3
	if settled {
		g.x, g.y, g.vx, g.vy = tx, ty, 0, 0
	}
	steel := ui.Hex("#94a3b8")
	if math.Hypot(dx, dy) > 1 { // the coil, from where the caret is to where the box is
		ax, ay := float64(box.X+box.W/2), float64(box.Y+box.H)
		bx, by := g.x+float64(box.W)/2, g.y+float64(box.H)
		l := math.Hypot(bx-ax, by-ay)
		nx, ny := -(by-ay)/l, (bx-ax)/l
		coil := &ui.Path{}
		coil.MoveTo(f32(ax), f32(ay))
		const turns = 7
		for i := 1; i <= turns*2; i++ {
			t := float64(i) / (turns * 2)
			s := map[bool]float64{true: 1, false: -1}[i%2 == 0] * float64(box.W) * 0.6
			if i == turns*2 {
				s = 0
			}
			coil.LineTo(f32(ax+(bx-ax)*t+nx*s), f32(ay+(by-ay)*t+ny*s))
		}
		p.StrokePath(coil, 1.2, withAlpha(steel, 0.9))
	}
	b := ui.Rect{X: f32(g.x), Y: f32(g.y), W: box.W, H: box.H}
	fr := f
	if !settled {
		fr.jelly = nil
	}
	glassBody(p, fr, b, ui.Hex("#06b6d4"), 1, 0.9)
	if !settled {
		p.AnimationFrame()
	}
}

// ---- Kuyruklu yıldız ----

type cometPt struct{ x, y, t float64 }

type cometFX struct {
	fxBase
	trail  []cometPt
	speed  float64 // how fast it goes: writing and moving, smoothed
	sparks motes
}

func newComet() *cometFX { return &cometFX{} }

func (g *cometFX) reset() { g.trail, g.sparks, g.have = nil, nil, false }

func (g *cometFX) shift(dy float32) {
	for i := range g.trail {
		g.trail[i].y += float64(dy)
	}
	g.sparks.shift(dy)
	g.prev.Y += dy
}

func (g *cometFX) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, dist := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	g.speed += ((dist/dt)/600 + float64(k.wrote)*0.25/dt/20 - g.speed) * math.Min(1, dt*5)
	g.speed *= math.Exp(-dt * 1.5)
	g.trail = append(g.trail, cometPt{cx, cy, tn})
	keep := 250 + 900*clamp01(g.speed) // the faster, the longer the tail lasts
	for len(g.trail) > 2 && tn-g.trail[0].t > keep {
		g.trail = g.trail[1:]
	}
	if len(g.trail) > 120 {
		g.trail = g.trail[len(g.trail)-120:]
	}
	cyan, white := ui.Hex("#67e8f9"), ui.RGBA(255, 255, 255, 1)
	if !f.dark { // on a light page, deeper: a pale tail would not show
		cyan, white = ui.Hex("#0891b2"), ui.Hex("#22d3ee")
	}
	// the tail: dots from old to new, growing and brightening toward the head
	n := len(g.trail)
	for i, pt := range g.trail {
		t := float64(i+1) / float64(n)
		r := h * 0.08 * (0.3 + 2.2*t*t) * (0.4 + clamp01(g.speed))
		a := t * t * 0.8
		p.Fill(ui.Rect{X: f32(pt.x - r), Y: f32(pt.y - r), W: f32(2 * r), H: f32(2 * r)}, withAlpha(mix(cyan, white, t*t), a), f32(r))
	}
	if g.speed > 0.15 && rnd() < g.speed { // it sheds sparkles
		g.sparks = append(g.sparks, mote{x: cx + (rnd()-0.5)*h*0.5, y: cy + (rnd()-0.5)*h*0.5, vx: (rnd() - 0.5) * 40, vy: (rnd() - 0.5) * 40, t0: tn, life: 500, size: 1 + rnd()*1.5, col: cyan})
	}
	g.sparks.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size * (1 - age))
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s, W: 2 * s, H: 2 * s}, withAlpha(white, 1-age), s)
	})
	glassBody(p, f, box, mix(cyan, white, 0.4), 1, 1.4+clamp01(g.speed))
	if n > 2 || len(g.sparks) > 0 || g.speed > 0.01 {
		p.AnimationFrame()
	}
}

// ---- Mıknatıs ----

type filing struct {
	ang, r, rot, base float64 // where it is round the caret, how it lies, and its own rest
}

type magnet struct {
	fxBase
	fil  []filing
	pull float64 // + drawn in, - pushed out
}

func newMagnet() *magnet {
	m := &magnet{}
	for i := 0; i < 26; i++ {
		a := float64(i)/26*2*math.Pi + rnd()*0.2
		m.fil = append(m.fil, filing{ang: a, r: 1.9 + rnd()*1.2, rot: rnd() * math.Pi, base: rnd() * math.Pi})
	}
	return m
}

func (g *magnet) reset() { g.have = false }

func (g *magnet) shift(dy float32) { g.prev.Y += dy }

func (g *magnet) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.wrote > 0 {
		g.pull = math.Min(1, g.pull+0.25*float64(k.wrote))
	}
	if k.erased > 0 {
		g.pull = math.Max(-1, g.pull-0.5*float64(k.erased))
	}
	g.pull *= math.Exp(-dt * 0.9)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	grey := ui.Hex("#71717a")
	if f.dark {
		grey = ui.Hex("#a1a1aa")
	}
	for i := range g.fil {
		fl := &g.fil[i]
		wantR := 2.3 - 0.9*g.pull // drawn in, or pushed out
		fl.r += (wantR + 0.4*math.Sin(fl.base*3) - fl.r) * math.Min(1, dt*4)
		align := clamp01(math.Abs(g.pull) * 1.5) // pulled, a filing points at the caret; loose, it lies as it fell
		want := fl.ang*align + fl.base*(1-align)
		if g.pull > 0 {
			want = fl.ang
		}
		fl.rot += (want - fl.rot) * math.Min(1, dt*6*(0.3+align))
		x, y := cx+math.Cos(fl.ang)*fl.r*h, cy+math.Sin(fl.ang)*fl.r*h*0.7
		l := h * 0.22
		dx, dy := math.Cos(fl.rot)*l/2, math.Sin(fl.rot)*l/2
		ln := &ui.Path{}
		ln.MoveTo(f32(x-dx), f32(y-dy)).LineTo(f32(x+dx), f32(y+dy))
		p.StrokePath(ln, 1.3, withAlpha(grey, 0.45+0.45*align))
	}
	// the magnet: red melting into blue, a faint field round it as it pulls
	if g.pull > 0.05 {
		for _, s := range []float64{1.2, 1.8} {
			r := h * s * (1 + 0.1*math.Sin(tn/300))
			p.Fill(ui.Rect{X: f32(cx - r), Y: f32(cy - r*0.7), W: f32(2 * r), H: f32(1.4 * r)}, withAlpha(ui.Hex("#ef4444"), 0.04*g.pull), f32(r))
		}
	}
	glowAround(p, box, ui.Hex("#ef4444"), 0.5)
	glassFill(p, shapeBody(box, f.style.bracket), ui.Hex("#ef4444"), ui.Hex("#3b82f6"), f.dark, 1) // glass: red melting into blue
	glassGloss(p, box, f.dark, 1)
	if math.Abs(g.pull) > 0.01 {
		p.AnimationFrame()
	}
}
