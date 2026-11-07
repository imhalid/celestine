package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// More games and creatures, each fixed as it ships:
//
//	Hayalet         a ghost that comes after the caret: writing fast it catches up ("nom"); an erase sends it off, blue
//	Space Invaders  every word brings an invader down over the line; Enter shoots them all, in bursts
//	Balık           the caret a fish: it beats its tail at a space; at Enter it dives, in bubbles
//	Arı             a bee that buzzes round the caret; at a space it lands on the word just written and comes back
//	Robot           a little robot: its antenna lights at every key; an erase makes it start ("!") with crossed eyes

// ---- Hayalet ----

type ghost struct {
	fxBase
	x, y, vx, vy float64
	placed       bool
	rate         float64
	scared       float64 // when an erase scared it
	nom          motes
	caught       float64
}

func newGhost() *ghost { return &ghost{scared: -1e9, caught: -1e9} }

func (g *ghost) reset() { g.placed, g.nom, g.have = false, nil, false }

func (g *ghost) shift(dy float32) { g.y += float64(dy); g.nom.shift(dy); g.prev.Y += dy }

func (g *ghost) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*2)
	if k.erased > 0 {
		g.scared = tn
	}
	scared := tn-g.scared < 1600
	// where it goes: behind the caret, the nearer the faster the writing; away from it, scared
	gap := h * (3.2 - 2.4*clamp01(g.rate/9))
	tx, ty := float64(box.X)-gap, float64(box.Y)-h*0.15+math.Sin(tn/300)*h*0.08
	if scared {
		tx, ty = float64(box.X)-h*5, float64(box.Y)-h*0.8
	}
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	const w0, z = 7.0, 0.9
	g.vx += (w0*w0*(tx-g.x) - 2*z*w0*g.vx) * dt
	g.vy += (w0*w0*(ty-g.y) - 2*z*w0*g.vy) * dt
	g.x += g.vx * dt
	g.y += g.vy * dt
	if !scared && float64(box.X)-g.x < h*1.2 && tn-g.caught > 900 { // caught up: nom
		g.caught = tn
		g.nom = append(g.nom, mote{x: g.x, y: g.y - h*0.4, vy: -30, t0: tn, life: 800, size: h * 0.5})
	}
	col := ui.Hex("#f87171")
	if scared {
		col = ui.Hex("#3b82f6")
		if int(tn/150)%2 == 0 && tn-g.scared > 1100 {
			col = ui.RGBA(255, 255, 255, 1)
		}
	}
	// the ghost: a dome, a wavy hem, two eyes looking at the caret
	gw, gh := h*0.95, h*1.05
	x0, y0 := g.x-gw/2, g.y
	body := &ui.Path{}
	body.MoveTo(f32(x0), f32(y0+gh*0.45))
	body.CubeTo(f32(x0), f32(y0-gh*0.05), f32(x0+gw), f32(y0-gh*0.05), f32(x0+gw), f32(y0+gh*0.45))
	body.LineTo(f32(x0+gw), f32(y0+gh))
	for i := 0; i < 3; i++ { // the hem, waving
		ph := math.Sin(tn/120+float64(i)) * gh * 0.06
		xa := x0 + gw - float64(i)*gw/3
		body.QuadTo(f32(xa-gw/6), f32(y0+gh*0.82+ph), f32(xa-gw/3), f32(y0+gh))
	}
	body.Close()
	glassFill(p, body, col, col, f.dark, 1)
	look := 1.0
	if scared {
		look = -1
	}
	for _, ox := range []float64{0.32, 0.68} {
		ex, ey := x0+gw*ox, y0+gh*0.42
		er := gw * 0.13
		p.Fill(ui.Rect{X: f32(ex - er), Y: f32(ey - er*1.2), W: f32(2 * er), H: f32(2.4 * er)}, ui.RGBA(255, 255, 255, 1), f32(er))
		pr := er * 0.55
		p.Fill(ui.Rect{X: f32(ex + look*er*0.4 - pr), Y: f32(ey - pr), W: f32(2 * pr), H: f32(2 * pr)}, ui.Hex("#1e3a8a"), f32(pr))
	}
	g.nom.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "nom", f32(m.x), f32(m.y), f32(m.size), withAlpha(col, 1-age), true, 0, 1)
	})
	glassBody(p, f, box, ui.Hex("#facc15"), 1, 0.9)
	p.AnimationFrame() // it floats
}

// ---- Space Invaders ----

var invaderRows = []string{
	"..X.....X..",
	"...X...X...",
	"..XXXXXXX..",
	".XX.XXX.XX.",
	"XXXXXXXXXXX",
	"X.XXXXXXX.X",
	"X.X.....X.X",
	"...XX.XX...",
}

type invader struct {
	x, y, t0 float64
	dead     float64
	col      ui.Color
}

type invaders struct {
	fxBase
	list  []invader
	shots motes
	boom  motes
	fired float64
}

func newInvaders() *invaders { return &invaders{fired: -1e9} }

func (g *invaders) reset() { g.list, g.shots, g.boom, g.have = nil, nil, nil, false }

func (g *invaders) shift(dy float32) {
	for i := range g.list {
		g.list[i].y += float64(dy)
	}
	g.shots.shift(dy)
	g.boom.shift(dy)
	g.prev.Y += dy
}

func (g *invaders) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cell := math.Max(1, h*0.085)
	if strings.Contains(k.typed, " ") && len(g.list) < 10 {
		g.list = append(g.list, invader{x: float64(box.X) - h*0.6 - rnd()*h*3, y: float64(box.Y) - h*1.7 - rnd()*h*0.8, t0: tn, dead: -1,
			col: []ui.Color{ui.Hex("#22c55e"), ui.Hex("#a855f7"), ui.Hex("#06b6d4")}[len(g.list)%3]})
	}
	if strings.Contains(k.typed, "\n") { // fire at them all
		g.fired = tn
		for i := range g.list {
			if g.list[i].dead < 0 {
				g.list[i].dead = tn + 120 + float64(i)*60
				g.shots = append(g.shots, mote{x: float64(box.X + box.W/2), y: float64(box.Y), vx: (g.list[i].x - float64(box.X)) / 0.12, vy: (g.list[i].y - float64(box.Y)) / 0.12, t0: tn, life: 120 + float64(i)*60, col: ui.Hex("#fde047")})
			}
		}
	}
	out := g.list[:0]
	for _, iv := range g.list {
		if iv.dead >= 0 && tn > iv.dead { // it bursts
			for j := 0; j < 10; j++ {
				a := rnd() * 2 * math.Pi
				g.boom = append(g.boom, mote{x: iv.x, y: iv.y, vx: math.Cos(a) * 80, vy: math.Sin(a) * 80, t0: tn, life: 400, size: cell, col: iv.col})
			}
			continue
		}
		// it steps from side to side, in the old way: a jerk, not a glide
		stepx := float64(int(tn/400)%4-2) * cell
		frame := int(tn/400) % 2
		arrive := clamp01((tn - iv.t0) / 300)
		ox, oy := iv.x+stepx, iv.y-(1-arrive)*h
		for r, row := range invaderRows {
			if frame == 1 && r == len(invaderRows)-1 {
				row = ".X.......X." // its legs, the other way
			}
			for c := 0; c < len(row); c++ {
				if row[c] == 'X' {
					p.Fill(ui.Rect{X: f32(ox + float64(c-5)*cell), Y: f32(oy + float64(r-4)*cell), W: f32(cell + 0.3), H: f32(cell + 0.3)}, withAlpha(iv.col, arrive), 0)
				}
			}
		}
		out = append(out, iv)
	}
	g.list = out
	g.shots.stepDraw(p, tn, dt, func(m *mote, age float64) {
		p.Fill(ui.Rect{X: f32(m.x) - 0.75, Y: f32(m.y) - f32(h*0.25), W: 1.5, H: f32(h * 0.5)}, m.col, 0.75)
	})
	g.boom.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x), Y: f32(m.y), W: s, H: s}, withAlpha(m.col, 1-age), 0)
	})
	glassBody(p, f, box, ui.Hex("#22c55e"), 1, 0.9)
	if len(g.list) > 0 || len(g.shots) > 0 || len(g.boom) > 0 {
		p.AnimationFrame()
	}
}

// ---- Balık ----

type fish struct {
	fxBase
	wag     float64 // when it beat its tail
	dive    float64 // when it dived
	bubbles motes
	left    bool
}

func newFish() *fish { return &fish{wag: -1e9, dive: -1e9} }

func (g *fish) reset() { g.bubbles, g.have = nil, false }

func (g *fish) shift(dy float32) { g.bubbles.shift(dy); g.prev.Y += dy }

func (g *fish) draw(p *ui.Painter, f caretFrame, k kick) {
	was := g.prev
	tn, dt, moved, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if moved && was.H > 0 && math.Abs(float64(box.X-was.X)) > 0.5 {
		g.left = box.X < was.X
	}
	if strings.Contains(k.typed, " ") {
		g.wag = tn
	}
	if strings.Contains(k.typed, "\n") {
		g.dive = tn
		for i := 0; i < 6; i++ {
			g.bubbles = append(g.bubbles, mote{x: float64(box.X+box.W/2) + (rnd()-0.5)*h*0.6, y: float64(box.Y), vx: (rnd() - 0.5) * 10, vy: -40 - rnd()*40, t0: tn + float64(i)*60, life: 1000, size: h * (0.08 + rnd()*0.1)})
		}
	}
	if k.wrote > 0 && rnd() < 0.15 {
		g.bubbles = append(g.bubbles, mote{x: float64(box.X + box.W/2), y: float64(box.Y), vy: -30, t0: tn, life: 900, size: h * 0.07})
	}
	blue := ui.Hex("#38bdf8")
	g.bubbles.stepDraw(p, tn, dt, func(m *mote, age float64) {
		if age < 0 {
			return
		}
		m.vx = 8 * math.Sin((tn-m.t0)/150)
		s := f32(m.size)
		p.StrokePath((&ui.Path{}).Circle(f32(m.x), f32(m.y), s), 1, withAlpha(blue, 0.8*(1-age)))
	})
	// the fish: a body round in front, a tail behind that beats
	dy := 0.0
	if dk := (tn - g.dive) / 700; dk < 1 {
		dy = math.Sin(dk*math.Pi) * h * 0.6
	}
	d := 1.0
	if g.left {
		d = -1
	}
	beat := math.Sin(tn/1000*5) * 0.25
	if wk := (tn - g.wag) / 400; wk < 1 {
		beat += math.Sin(wk*math.Pi*4) * 0.9 * (1 - wk)
	}
	bx, by, bw, bh := float64(box.X)-float64(box.W)*0.3, float64(box.Y)+dy+h*0.1, float64(box.W)*1.6, h*0.8
	cx, cy := bx+bw/2, by+bh/2
	body := &ui.Path{}
	body.MoveTo(f32(cx+d*bw/2), f32(cy))
	body.CubeTo(f32(cx+d*bw/2), f32(cy-bh*0.6), f32(cx-d*bw/2), f32(cy-bh*0.55), f32(cx-d*bw/2), f32(cy))
	body.CubeTo(f32(cx-d*bw/2), f32(cy+bh*0.55), f32(cx+d*bw/2), f32(cy+bh*0.6), f32(cx+d*bw/2), f32(cy))
	body.Close()
	tx := cx - d*bw/2
	tail := &ui.Path{}
	tail.MoveTo(f32(tx), f32(cy)).LineTo(f32(tx-d*bh*0.6), f32(cy-bh*0.5+beat*bh*0.4)).LineTo(f32(tx-d*bh*0.6), f32(cy+bh*0.5+beat*bh*0.4)).Close()
	orange := ui.Hex("#fb923c")
	glowAround(p, ui.Rect{X: f32(bx), Y: f32(by), W: f32(bw), H: f32(bh)}, orange, 0.6)
	glassFill(p, tail, orange, ui.Hex("#f97316"), f.dark, 1)
	glassFill(p, body, orange, ui.Hex("#f97316"), f.dark, 1)
	ex := cx + d*bw*0.22
	er := math.Max(1, bh*0.11)
	p.Fill(ui.Rect{X: f32(ex - er), Y: f32(cy - bh*0.18 - er), W: f32(2 * er), H: f32(2 * er)}, faceInk(f.dark, ui.Hex("#1c1917")), f32(er))
	p.AnimationFrame() // it swims
}

// ---- Arı ----

type bee struct {
	fxBase
	land   float64 // when it went to land
	lx, ly float64 // where
	x, y   float64
	placed bool
}

func newBee() *bee { return &bee{land: -1e9} }

func (g *bee) reset() { g.placed, g.have = false, false }

func (g *bee) shift(dy float32) { g.y += float64(dy); g.ly += float64(dy); g.prev.Y += dy }

func (g *bee) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if strings.Contains(k.typed, " ") { // it lands on the word just written
		g.land, g.lx, g.ly = tn, cx-h*1.5, float64(box.Y)-h*0.1
	}
	// a figure of eight round the caret; or to the word, a while, and back
	t := tn / 1000
	tx, ty := cx+math.Sin(t*2.2)*h*1.3, cy-h*0.9+math.Sin(t*4.4)*h*0.45
	if lk := tn - g.land; lk < 900 {
		tx, ty = g.lx, g.ly
	}
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	a := 1 - math.Pow(0.82, dt*60)
	g.x += (tx - g.x) * a
	g.y += (ty - g.y) * a
	s := h * 0.5
	// its wings, beating; its striped body
	flap := math.Abs(math.Sin(tn / 18))
	for _, d := range []float64{-1, 1} {
		w := ui.Rect{X: f32(g.x + d*s*0.2 - s*0.35), Y: f32(g.y - s*0.9 - flap*s*0.3), W: f32(s * 0.7), H: f32(s * (0.6 + flap*0.3))}
		p.Fill(w, ui.RGBA(224, 242, 254, 0.7), w.W/2)
	}
	body := ui.Rect{X: f32(g.x - s*0.75), Y: f32(g.y - s*0.5), W: f32(s * 1.5), H: f32(s)}
	p.Fill(body, ui.Hex("#facc15"), body.H/2)
	for i := 0; i < 2; i++ {
		p.Fill(ui.Rect{X: body.X + body.W*f32(0.3+0.25*float64(i)), Y: body.Y, W: body.W * 0.12, H: body.H}, ui.Hex("#1c1917"), 0)
	}
	p.Fill(ui.Rect{X: body.X + body.W*0.82, Y: body.Y + body.H*0.3, W: max(1, body.H*0.2), H: max(1, body.H*0.2)}, ui.Hex("#1c1917"), body.H*0.1)
	if tn-g.land > 900 && rnd() < 0.3 { // buzz
		z := &ui.Path{}
		bx, by := f32(g.x-s*1.3), f32(g.y)
		z.MoveTo(bx, by).LineTo(bx-f32(s*0.3), by-f32(s*0.2)).LineTo(bx-f32(s*0.6), by)
		p.StrokePath(z, 0.8, withAlpha(ui.Hex("#a16207"), 0.5))
	}
	glassBody(p, f, box, ui.Hex("#fbbf24"), 1, 0.9)
	p.AnimationFrame() // it buzzes
}

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
