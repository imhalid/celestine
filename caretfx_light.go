package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of light, each fixed as it ships:
//
//	Neon tabela  a neon tube: now and then it flickers off and buzzes back; Enter flashes it

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
	idleFrame(p, f) // it hums
}
