package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets that move the letters round them, as they are drawn (the text itself is not changed): each letter it
// moves is covered where it stands and drawn where the caret has it.
//
//	Rüzgâr       writing fast, the letters behind it bend in the wind and straighten as it calms; Enter is a gust
//	Jöle satır   the line quivers like jelly, most beside the caret

// moveLetter draws a letter of the line moved by (dx, dy), turned by angle (radians) and scaled, in col: its own
// place covered first.
func moveLetter(p *ui.Painter, f caretFrame, gl nearGlyph, dx, dy, angle, scale float64, col ui.Color) {
	p.Fill(ui.Rect{X: gl.r.X - 0.5, Y: gl.r.Y - 0.5, W: gl.r.W + 1, H: gl.r.H + 1}, f.bg, 0)
	glyphAt(p, gl.ch, gl.r.X+gl.r.W/2+f32(dx), gl.r.Y+gl.r.H/2+f32(dy), gl.size, col, false, angle*180/math.Pi, scale) // glyphAt turns by degrees
}

// ---- Rüzgâr ----

type wind struct {
	fxBase
	rate  float64 // letters a second, smoothed: how hard it blows
	gust  float64 // when Enter blew one
	gustX float64
	gustY float64
	lines motes
}

func newWind() *wind { return &wind{gust: -1e9} }

func (g *wind) reset() { g.lines, g.have = nil, false }

func (g *wind) shift(dy float32) { g.lines.shift(dy); g.gustY += float64(dy); g.prev.Y += dy }

func (g *wind) draw(p *ui.Painter, f caretFrame, k kick) {
	was := g.prev
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*2.5)
	blow := clamp01((g.rate - 3) / 8)
	if strings.Contains(k.typed, "\n") && was.H > 0 { // a gust sweeps the line just ended
		g.gust, g.gustX, g.gustY = tn, float64(was.X), float64(was.Y+was.H/2)
	}
	grey := ui.Hex("#94a3b8")
	if blow > 0.05 && rnd() < blow*0.3 { // streaks of wind
		g.lines = append(g.lines, mote{x: float64(box.X) - rnd()*h*6, y: float64(box.Y) + rnd()*h, vx: 260 + rnd()*120, t0: tn, life: 450, size: h * (0.8 + rnd()), col: grey})
	}
	g.lines.stepDraw(p, tn, dt, func(m *mote, age float64) {
		l := &ui.Path{}
		l.MoveTo(f32(m.x), f32(m.y)).QuadTo(f32(m.x+m.size/2), f32(m.y-2), f32(m.x+m.size), f32(m.y))
		p.StrokePath(l, 1, withAlpha(m.col, 0.35*math.Sin(age*math.Pi)))
	})
	if f.near != nil && (blow > 0.02 || tn-g.gust < 900) {
		text := p.Theme().Text
		for _, gl := range f.near(40) {
			if gl.off >= 0 {
				continue
			}
			n := float64(-gl.off)
			bend := blow * (1 - n/40) * 0.3        // radians, leaning with the wind (to the right)
			if gk := (tn - g.gust) / 900; gk < 1 { // the gust: a front from the left that bends what it passes
				gx := float64(gl.r.X)
				front := g.gustX - 200 + gk*float64(box.X+800-float32(g.gustX))
				d := (gx - front) / (h * 3)
				bend += 0.5 * math.Exp(-d*d) * (1 - gk)
			}
			if bend > 0.01 {
				moveLetter(p, f, gl, bend*h*0.35, 0, bend, 1, text)
			}
		}
	}
	glassBody(p, f, box, mix(grey, ui.Hex("#38bdf8"), blow), 1, 0.8)
	if blow > 0.01 || len(g.lines) > 0 || tn-g.gust < 900 {
		p.AnimationFrame()
	}
}

// ---- Jöle satır ----

type jellyLine struct {
	fxBase
	energy float64
}

func newJellyLine() *jellyLine { return &jellyLine{} }

func (g *jellyLine) reset() { g.energy, g.have = 0, false }

func (g *jellyLine) shift(dy float32) { g.prev.Y += dy }

func (g *jellyLine) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.energy = math.Min(1, g.energy+0.35*float64(k.wrote+k.erased))
	g.energy *= math.Exp(-dt * 3)
	pink := ui.Hex("#f472b6")
	if f.near != nil && g.energy > 0.01 {
		text := p.Theme().Text
		for _, gl := range f.near(25) {
			n := math.Abs(float64(gl.off) + 0.5)
			fall := math.Exp(-n / 6)
			y := math.Sin(tn/1000*28+float64(gl.off)*0.9) * g.energy * fall * h * 0.12
			sq := 1 + math.Sin(tn/1000*28+float64(gl.off)*0.9+1.4)*g.energy*fall*0.08
			moveLetter(p, f, gl, 0, y, 0, sq, text)
		}
	}
	glassBody(p, f, box, pink, 1, 0.8)
	if g.energy > 0.01 {
		p.AnimationFrame()
	}
}
