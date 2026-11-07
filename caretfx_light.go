package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of light, each fixed as it ships:
//
//	Spot ışığı      a soft ring of light round the caret that brightens what is near it (the page is not darkened);
//	                writing fast, it closes in
//	Neon tabela     a neon tube: now and then it flickers off and buzzes back; Enter flashes it
//	Kuzey ışıkları  a curtain of green and violet light that waves over the caret, livelier as it writes
//	Gökkuşağı izi   moving, it leaves a short rainbow behind it

// ---- Spot ışığı ----

type spotlight struct {
	fxBase
	x, y   float64
	placed bool
	rate   float64
	r      float64
}

func newSpotlight() *spotlight { return &spotlight{r: 1} }

func (g *spotlight) reset() { g.placed, g.have = false, false }

func (g *spotlight) shift(dy float32) { g.y += float64(dy); g.prev.Y += dy }

func (g *spotlight) draw(p *ui.Painter, f caretFrame, k kick) {
	_, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	tx, ty := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	a := 1 - math.Pow(0.75, dt*60)
	g.x += (tx - g.x) * a
	g.y += (ty - g.y) * a
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*2)
	want := 1 - 0.45*clamp01(g.rate/10) // writing fast, the ring closes in
	g.r += (want - g.r) * math.Min(1, dt*3)
	R := h * 4.5 * g.r
	warm := ui.Hex("#fef3c7")
	if !f.dark {
		warm = ui.Hex("#fde68a")
	}
	for i, s := range []float64{1, 0.66, 0.36} { // soft rings, lighter toward the middle
		r := R * s
		p.Fill(ui.Rect{X: f32(g.x - r), Y: f32(g.y - r*0.75), W: f32(2 * r), H: f32(1.5 * r)}, withAlpha(warm, []float64{0.07, 0.1, 0.14}[i]), f32(r))
	}
	glassBody(p, f, box, ui.Hex("#fbbf24"), 1, 1.2)
	if math.Hypot(tx-g.x, ty-g.y) > 0.3 || math.Abs(want-g.r) > 0.005 {
		p.AnimationFrame()
	}
}

// ---- Neon tabela ----

type neon struct {
	fxBase
	off   float64 // when it last went out
	dark  float64 // for how long
	next  float64 // when it flickers next
	flash float64
	buzz  motes
}

func newNeon() *neon { return &neon{off: -1e9, next: 2500, flash: -1e9} }

func (g *neon) reset() { g.buzz, g.have = nil, false }

func (g *neon) shift(dy float32) { g.buzz.shift(dy); g.prev.Y += dy }

func (g *neon) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	pink := ui.Hex("#ff3ea5")
	if strings.Contains(k.typed, "\n") {
		g.flash = tn
	}
	if tn > g.next { // a flicker: out for a moment, and back with a buzz
		g.off, g.dark = tn, 60+rnd()*160
		g.next = tn + 2500 + rnd()*5000
		for i := 0; i < 3; i++ {
			g.buzz = append(g.buzz, mote{x: float64(box.X+box.W) + 2, y: float64(box.Y) + rnd()*h, vx: 30 + rnd()*20, vy: (rnd() - 0.5) * 20, t0: tn + g.dark, life: 350, size: h * 0.3})
		}
	}
	lit := 1.0
	if el := tn - g.off; el < g.dark {
		lit = 0.12 + 0.2*math.Max(0, math.Sin(el/9)) // it stutters while it is out
	}
	glow := 1.2 * lit
	if fk := (tn - g.flash) / 350; fk < 1 {
		glow += 2.5 * (1 - fk)
	}
	for i, w := range []float32{8, 5, 3} { // the tube's light round it
		p.StrokePath(rrect(box.X, box.Y, box.W, box.H, boxRadius(box.W, box.H)), w, withAlpha(pink, glow*[]float64{0.08, 0.16, 0.3}[i]))
	}
	glassBody(p, f, box, mix(ui.Hex("#4a0e2e"), pink, lit), 1, glow)
	p.StrokePath(rrect(box.X+1, box.Y+1, box.W-2, box.H-2, boxRadius(box.W, box.H)), 1.2, withAlpha(ui.Hex("#ffd1ec"), lit)) // the tube itself
	g.buzz.stepDraw(p, tn, dt, func(m *mote, age float64) {
		if age < 0 {
			return
		}
		z := &ui.Path{}
		x, y, s := f32(m.x), f32(m.y), f32(m.size)
		z.MoveTo(x, y).LineTo(x+s*0.4, y-s*0.3).LineTo(x+s*0.2, y).LineTo(x+s*0.6, y-s*0.3)
		p.StrokePath(z, 1, withAlpha(pink, 1-age))
	})
	p.AnimationFrame() // it hums
}

// ---- Kuzey ışıkları ----

type aurora struct {
	fxBase
	rate float64
	t    float64
}

func newAurora() *aurora { return &aurora{} }

func (g *aurora) reset() { g.have = false }

func (g *aurora) shift(dy float32) { g.prev.Y += dy }

func (g *aurora) draw(p *ui.Painter, f caretFrame, k kick) {
	_, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*2)
	live := 0.35 + 0.65*clamp01(g.rate/8)
	g.t += dt * (0.6 + 1.6*live)
	cx := float64(box.X + box.W/2)
	base := float64(box.Y) - h*0.2
	green, violet := ui.Hex("#34d399"), ui.Hex("#a78bfa")
	const strips = 14
	for i := 0; i < strips; i++ { // the curtain: strips that wave and breathe, green below fading to violet
		u := float64(i)/(strips-1) - 0.5
		x := cx + u*h*5 + math.Sin(g.t*1.3+float64(i)*0.7)*h*0.35
		top := base - h*(1.4+0.8*math.Sin(g.t*0.9+float64(i)*0.5))*live
		w := h * 0.42
		a := (1 - math.Abs(u)*1.6) * live
		if a <= 0 {
			continue
		}
		p.FillGradient(ui.Rect{X: f32(x - w/2), Y: f32(top), W: f32(w), H: f32(base - top)}, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
			{Color: withAlpha(violet, 0), At: 0},
			{Color: withAlpha(violet, 0.18*a), At: 0.35},
			{Color: withAlpha(green, 0.32*a), At: 0.85},
			{Color: withAlpha(green, 0), At: 1},
		}}, f32(w/2))
	}
	glassBody(p, f, box, mix(green, violet, 0.5+0.5*math.Sin(g.t)), 1, 1)
	p.AnimationFrame() // it waves
}

// ---- Gökkuşağı izi ----

type rainbowTrail struct {
	fxBase
	trail []cometPt
}

func newRainbowTrail() *rainbowTrail { return &rainbowTrail{} }

func (g *rainbowTrail) reset() { g.trail, g.have = nil, false }

func (g *rainbowTrail) shift(dy float32) {
	for i := range g.trail {
		g.trail[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *rainbowTrail) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if n := len(g.trail); n == 0 || math.Hypot(cx-g.trail[n-1].x, cy-g.trail[n-1].y) > 1 {
		g.trail = append(g.trail, cometPt{cx, cy, tn})
	}
	for len(g.trail) > 1 && tn-g.trail[0].t > 650 {
		g.trail = g.trail[1:]
	}
	colors := []string{"#ef4444", "#f97316", "#eab308", "#22c55e", "#3b82f6", "#8b5cf6"}
	band := h * 0.12
	if len(g.trail) > 1 {
		for ci, c := range colors { // six stripes, one over another, along where it went
			off := (float64(ci) - 2.5) * band
			line := &ui.Path{}
			for i, pt := range g.trail {
				if i == 0 {
					line.MoveTo(f32(pt.x), f32(pt.y+off))
				} else {
					line.LineTo(f32(pt.x), f32(pt.y+off))
				}
			}
			age := (tn - g.trail[0].t) / 650
			p.StrokePath(line.Join(ui.RoundJoin).Cap(ui.RoundCap), f32(band), withAlpha(ui.Hex(c), 0.55*(1-age*0.5)))
		}
	}
	glassBody(p, f, box, hsl(tn/8, 0.85, 0.6), 1, 1)
	if len(g.trail) > 1 {
		p.AnimationFrame()
	}
}
