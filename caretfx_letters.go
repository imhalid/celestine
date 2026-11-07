package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets that move the letters round them, as they are drawn (the text itself is not changed): each letter it
// moves is covered where it stands and drawn where the caret has it.
//
//	Dalga        every key sends a wave along the line from the caret: the letters rise and fall as it passes
//	Rüzgâr       writing fast, the letters behind it bend in the wind and straighten as it calms; Enter is a gust
//	Yerçekimi    left a while, the last word's letters start to wobble and sag; writing again, they spring back
//	Domino       at a space the word before it falls over letter by letter, and stands up again
//	Jöle satır   the line quivers like jelly, most beside the caret
//	Kar küresi   what is erased turns to snow and drifts down; some of it settles on the line

// moveLetter draws a letter of the line moved by (dx, dy), turned by angle (radians) and scaled, in col: its own
// place covered first.
func moveLetter(p *ui.Painter, f caretFrame, gl nearGlyph, dx, dy, angle, scale float64, col ui.Color) {
	p.Fill(ui.Rect{X: gl.r.X - 0.5, Y: gl.r.Y - 0.5, W: gl.r.W + 1, H: gl.r.H + 1}, f.bg, 0)
	glyphAt(p, gl.ch, gl.r.X+gl.r.W/2+f32(dx), gl.r.Y+gl.r.H/2+f32(dy), gl.size, col, false, angle*180/math.Pi, scale) // glyphAt turns by degrees
}

// ---- Dalga ----

type wave struct {
	fxBase
	waves []float64 // when each wave left the caret
	wx    []float64 // and from where
}

func newWave() *wave { return &wave{} }

func (g *wave) reset() { g.waves, g.wx, g.have = nil, nil, false }

func (g *wave) shift(dy float32) { g.prev.Y += dy }

func (g *wave) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.wrote > 0 {
		g.waves, g.wx = append(g.waves, tn), append(g.wx, float64(box.X))
		if len(g.waves) > 8 {
			g.waves, g.wx = g.waves[1:], g.wx[1:]
		}
	}
	const life, speed = 1100.0, 0.55 // ms, px a ms
	keep, kx := g.waves[:0], g.wx[:0]
	for i, t0 := range g.waves {
		if tn-t0 < life {
			keep, kx = append(keep, t0), append(kx, g.wx[i])
		}
	}
	g.waves, g.wx = keep, kx
	cyan := ui.Hex("#06b6d4")
	if len(g.waves) > 0 && f.near != nil {
		text := p.Theme().Text
		for _, gl := range f.near(80) {
			gx := float64(gl.r.X + gl.r.W/2)
			lift := 0.0
			for i, t0 := range g.waves {
				age := tn - t0
				front := age * speed
				d := math.Abs(gx-g.wx[i]) - front
				lift += math.Exp(-d*d/(2*h*h*0.35)) * (1 - age/life) // a bump that travels out and dies down
			}
			if lift > 0.02 {
				moveLetter(p, f, gl, 0, -lift*h*0.35, 0, 1, mix(text, cyan, clamp01(lift)*0.6))
			}
		}
	}
	glassBody(p, f, box, cyan, 1, 0.9)
	if len(g.waves) > 0 {
		p.AnimationFrame()
	}
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

// ---- Yerçekimi ----

type gravity struct {
	fxBase
	sag   float64 // how far the wobble has gone: 0..1
	vsag  float64
	burst float64 // when writing again sprang them back
}

func newGravity() *gravity { return &gravity{burst: -1e9} }

func (g *gravity) reset() { g.sag, g.vsag, g.have = 0, 0, false }

func (g *gravity) shift(dy float32) { g.prev.Y += dy }

func (g *gravity) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	want := clamp01((g.idle(tn) - 3) / 4) // after three seconds of rest it starts to give
	if k.wrote > 0 && g.sag > 0.1 {
		g.burst = tn
	}
	// on a spring: slow to sag, quick to spring back, a bounce past it
	w0, z := 4.0, 0.9
	if want < g.sag {
		w0, z = 18, 0.35
	}
	g.vsag += (w0*w0*(want-g.sag) - 2*z*w0*g.vsag) * dt
	g.sag += g.vsag * dt
	purple := ui.Hex("#8b5cf6")
	if f.near != nil && math.Abs(g.sag) > 0.01 {
		text := p.Theme().Text
		letters := f.near(30)
		// the last word: the letters before the caret back to the space
		var word []nearGlyph
		for i := len(letters) - 1; i >= 0; i-- {
			gl := letters[i]
			if gl.off >= 0 {
				continue
			}
			if len(word) > 0 && word[len(word)-1].off-gl.off > 1 {
				break
			}
			word = append(word, gl)
		}
		for i, gl := range word {
			ph := tn/1000*7 + float64(i)*1.3
			wob := math.Sin(ph) * g.sag * 0.35
			drop := g.sag * h * 0.25 * float64(i+1) / float64(len(word))
			moveLetter(p, f, gl, 0, math.Max(-h, drop), wob, 1, text)
		}
	}
	glassBody(p, f, box, purple, 1, 0.8)
	if math.Abs(g.sag) > 0.005 || math.Abs(g.vsag) > 0.01 || want > 0 {
		p.AnimationFrame()
	}
}

// ---- Domino ----

type domino struct {
	fxBase
	falls []float64 // when each word began to fall
	fy    []float64 // on which line
	fx    []float64 // where its caret was
}

func newDomino() *domino { return &domino{} }

func (g *domino) reset() { g.falls, g.fy, g.fx, g.have = nil, nil, nil, false }

func (g *domino) shift(dy float32) {
	for i := range g.fy {
		g.fy[i] += float64(dy)
	}
	g.prev.Y += dy
}

func (g *domino) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	if strings.Contains(k.typed, " ") {
		g.falls, g.fy, g.fx = append(g.falls, tn), append(g.fy, float64(box.Y)), append(g.fx, float64(box.X))
	}
	const each, down = 55.0, 520.0 // ms between letters, and a letter's fall and rise
	orange := ui.Hex("#f97316")
	keep, ky, kx := g.falls[:0], g.fy[:0], g.fx[:0]
	if f.near != nil && len(g.falls) > 0 {
		text := p.Theme().Text
		letters := f.near(40)
		for i, t0 := range g.falls {
			alive := false
			// the word before where its space was: the letters left of that x on its line, back to a gap
			var word []nearGlyph
			for j := len(letters) - 1; j >= 0; j-- {
				gl := letters[j]
				if math.Abs(float64(gl.r.Y)-g.fy[i]) > 2 || float64(gl.r.X) >= g.fx[i] {
					continue
				}
				if len(word) > 0 && float64(word[len(word)-1].r.X-(gl.r.X+gl.r.W)) > 2 {
					break
				}
				word = append(word, gl)
			}
			for n := len(word) - 1; n >= 0; n-- { // from the first letter on
				gl := word[n]
				order := float64(len(word) - 1 - n)
				u := (tn - t0 - order*each) / down
				if u < 0 || u > 1 {
					if u < 0 {
						alive = true
					}
					continue
				}
				alive = true
				ang := math.Sin(u*math.Pi) * 1.15 // over to the right, and up again
				// it turns about its foot at the right: its middle goes round that corner
				px, py := float64(gl.r.X+gl.r.W), float64(gl.r.Y+gl.r.H)
				mx, my := float64(gl.r.X+gl.r.W/2)-px, float64(gl.r.Y+gl.r.H/2)-py
				rx := mx*math.Cos(ang) - my*math.Sin(ang)
				ry := mx*math.Sin(ang) + my*math.Cos(ang)
				moveLetter(p, f, gl, rx-mx, ry-my, ang, 1, mix(text, orange, math.Sin(u*math.Pi)*0.5))
			}
			if alive || tn-t0 < down {
				keep, ky, kx = append(keep, t0), append(ky, g.fy[i]), append(kx, g.fx[i])
			}
		}
	}
	g.falls, g.fy, g.fx = keep, ky, kx
	glassBody(p, f, box, orange, 1, 0.8)
	if len(g.falls) > 0 {
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

// ---- Kar küresi ----

type snowGlobe struct {
	fxBase
	flakes motes
	pile   motes // settled on the line, melting slowly
}

func newSnowGlobe() *snowGlobe { return &snowGlobe{} }

func (g *snowGlobe) reset() { g.flakes, g.pile, g.have = nil, nil, false }

func (g *snowGlobe) shift(dy float32) { g.flakes.shift(dy); g.pile.shift(dy); g.prev.Y += dy }

func (g *snowGlobe) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	white := ui.Hex("#e0f2fe")
	if !f.dark {
		white = ui.Hex("#7dd3fc")
	}
	if k.erased > 0 && k.gone != "" { // the letters erased: each a flake, the letter first, then snow
		for _, l := range goneLetters(p, f, k) {
			settle := rnd() < 0.35
			g.flakes = append(g.flakes, mote{x: l.x + l.w/2, y: float64(box.Y + box.H/2), vx: (rnd() - 0.5) * 20, vy: 10 + rnd()*15, spin: (rnd() - 0.5) * 3,
				t0: tn, life: 2600 + rnd()*900, size: h * (0.08 + rnd()*0.06), grav: 18, col: white, kind: map[bool]int{true: 1, false: 0}[settle]})
		}
	}
	text := p.Theme().Text
	var settled []mote
	g.flakes.stepDraw(p, tn, dt, func(m *mote, age float64) {
		m.vx = 14 * math.Sin((tn-m.t0)/400+m.size*10)
		if m.kind == 1 && m.y > float64(box.Y+box.H)+2 { // it lands on the line, and stays a while
			settled = append(settled, mote{x: m.x, y: float64(box.Y+box.H) + 1, t0: tn, life: 6000, size: m.size, col: m.col})
			m.t0 = -1e9
			return
		}
		turn := clamp01((tn - m.t0) / 500) // a letter for a moment, then a flake
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s, W: 2 * s, H: 2 * s}, withAlpha(m.col, turn*(1-age)), s)
		if turn < 1 {
			glyphAt(p, "❄", f32(m.x), f32(m.y), f32(h*0.4), withAlpha(text, 1-turn), false, 0, 1)
		}
	})
	g.pile = append(g.pile, settled...)
	g.pile.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size * 1.2)
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s*0.6, W: 2 * s, H: s * 1.2}, withAlpha(m.col, 0.9*(1-age*age)), s*0.6)
	})
	glassBody(p, f, box, mix(white, ui.Hex("#38bdf8"), 0.4), 1, 0.9)
	if len(g.flakes) > 0 || len(g.pile) > 0 {
		p.AnimationFrame()
	}
}
