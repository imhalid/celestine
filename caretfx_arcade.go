package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// More games and creatures, each fixed as it ships:
//
//	Robot  a little robot: its antenna lights at every key; an erase makes it start ("!") with crossed eyes

// ---- Robot ----

type robot struct {
	fxBase
	blink float64 // when its antenna last lit
	oops  float64 // when an erase startled it
	bang  motes
}

func newRobot() *robot { return &robot{blink: -1e9, oops: -1e9} }

func (g *robot) reset() { g.bang, g.have = nil, false }

func (g *robot) shift(dy float32) { g.bang.shift(dy); g.prev.Y += dy }

func (g *robot) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.wrote > 0 {
		g.blink = tn
	}
	if k.erased > 0 {
		if tn-g.oops > 500 {
			g.bang = append(g.bang, mote{x: float64(box.X + box.W/2), y: float64(box.Y) - h*0.6, vy: -20, t0: tn, life: 700, size: h * 0.6})
		}
		g.oops = tn
	}
	steel := ui.Hex("#94a3b8")
	// its antenna: a stalk and a light at its top that blinks at a key
	ax := box.X + box.W/2
	p.Fill(ui.Rect{X: ax - 0.6, Y: box.Y - f32(h*0.3), W: 1.2, H: f32(h * 0.3)}, steel, 0)
	lit := clamp01(1 - (tn-g.blink)/220)
	lr := f32(math.Max(1.4, h*0.09))
	red := ui.Hex("#ef4444")
	glowAround(p, ui.Rect{X: ax - lr, Y: box.Y - f32(h*0.3) - lr, W: 2 * lr, H: 2 * lr}, red, lit*1.5)
	p.Fill(ui.Rect{X: ax - lr, Y: box.Y - f32(h*0.3) - lr, W: 2 * lr, H: 2 * lr}, mix(ui.Hex("#7f1d1d"), red, lit), lr)
	glassBody(p, f, box, steel, 1, 0.8)
	ink := faceInk(f.dark, ui.Hex("#0f172a"))
	oops := tn-g.oops < 700
	er := max(1, box.W*0.13)
	for _, ox := range []float32{0.3, 0.7} { // square eyes; crossed when startled
		ex, ey := box.X+box.W*ox, box.Y+box.H*0.36
		if oops {
			x := &ui.Path{}
			x.MoveTo(ex-er, ey-er).LineTo(ex+er, ey+er).MoveTo(ex+er, ey-er).LineTo(ex-er, ey+er)
			p.StrokePath(x, max(0.8, er*0.5), ink)
		} else {
			p.Fill(ui.Rect{X: ex - er, Y: ey - er, W: 2 * er, H: 2 * er}, ui.Hex("#22d3ee"), 0.5)
		}
	}
	// its mouth: a grille
	for i := 0; i < 3; i++ {
		p.Fill(ui.Rect{X: box.X + box.W*f32(0.28+0.18*float64(i)), Y: box.Y + box.H*0.68, W: max(0.8, box.W*0.08), H: box.H * 0.12}, withAlpha(ink, 0.7), 0)
	}
	g.bang.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "!", f32(m.x), f32(m.y), f32(m.size), withAlpha(red, 1-age), true, 0, 1+0.3*(1-age))
	})
	if lit > 0 || oops || len(g.bang) > 0 {
		p.AnimationFrame()
	}
}
