package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Carets of games and creatures, each fixed as it ships:
//
//	Tamagotchi  a pet fed by what is written: it grows and brightens, hearts at every hundred letters; it wilts when left

// fxBase is what most of these carets keep: their own time, where the caret was, when something last happened.
type fxBase struct {
	clock
	prev   ui.Rect
	have   bool
	last   float64 // ms, the last frame
	active float64 // ms, when something was last written, erased or moved
	look   float64 // where its eyes look, -1 left to 1 right, eased toward want (gaze)
	want   float64
}

// step moves a caret's base to this frame: the time now and since the last frame (s), whether the caret moved
// and how far.
func (b *fxBase) step(p *ui.Painter, f caretFrame, k kick) (tn, dt float64, moved bool, dist float64) {
	tn = b.at(p)
	dt = math.Min(0.1, math.Max(0.001, (tn-b.last)/1000))
	b.last = tn
	if b.have {
		moved, dist = movedBy(b.prev, f.r)
	}
	b.want = gazeWant(b.want, k)
	b.look += (b.want - b.look) * (1 - math.Exp(-dt/0.09))
	b.prev, b.have = f.r, true
	if moved || k.wrote > 0 || k.erased > 0 {
		b.active = tn
	}
	return
}

// gazeWant is the way a caret's eyes look after this frame (#283): the way the writing goes, right as it is
// written (a line wrapped or a new one started too); the way it is erased, left for Backspace, right for Delete;
// else as they looked. Not the way the caret moved on the screen: the page scrolls and the column settles under it.
func gazeWant(was float64, k kick) float64 {
	switch {
	case k.erased > 0 && k.dir < 0:
		return -1
	case k.erased > 0, k.wrote > 0:
		return 1
	}
	return was
}

// gaze is where the eyes look, -1 left to 1 right.
func (b *fxBase) gaze() float32 { return float32(b.look) }

// idle is how long nothing happened, in seconds.
func (b *fxBase) idle(tn float64) float64 { return (tn - b.active) / 1000 }

func movedBy(prev, cur ui.Rect) (bool, float64) {
	d := math.Hypot(float64(cur.X-prev.X), float64(cur.Y-prev.Y))
	return d > 0.5, d
}

// mote is a small thing that flies, fades and falls: a leaf, a crumb, a spark, a drop.
type mote struct {
	x, y, vx, vy, spin, ang float64
	t0, life, size, grav    float64
	col                     ui.Color
	kind                    int
}

type motes []mote

// stepDraw moves the motes to tn, draws each with draw, and keeps the ones still alive.
func (ms *motes) stepDraw(p *ui.Painter, tn, dt float64, draw func(m *mote, age float64)) {
	out := (*ms)[:0]
	for _, m := range *ms {
		age := (tn - m.t0) / m.life
		if age >= 1 {
			continue
		}
		m.vy += m.grav * dt
		m.x += m.vx * dt
		m.y += m.vy * dt
		m.ang += m.spin * dt
		draw(&m, age)
		out = append(out, m)
	}
	*ms = out
}

func (ms *motes) shift(dy float32) {
	for i := range *ms {
		(*ms)[i].y += float64(dy)
	}
}

// ---- Tamagotchi ----

type pet struct {
	fxBase
	fed    float64 // what it has eaten: letters, slowly digested
	total  int     // letters written to it, for the hearts
	hearts motes
	bob    float64
}

func newPet() *pet { return &pet{fed: 40} }

func (g *pet) reset() { g.have = false }

func (g *pet) shift(dy float32) { g.hearts.shift(dy); g.prev.Y += dy }

func (g *pet) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	if k.wrote > 0 {
		g.fed = math.Min(400, g.fed+float64(k.wrote))
		was := g.total / 100
		g.total += k.wrote
		if g.total/100 > was { // a hundred letters: hearts
			for i := 0; i < 4; i++ {
				g.hearts = append(g.hearts, mote{x: float64(box.X + box.W/2), y: float64(box.Y), vx: (rnd() - 0.5) * 50, vy: -50 - rnd()*30, t0: tn, life: 1300, size: float64(box.H) * 0.4})
			}
		}
	}
	if g.idle(tn) > 2 { // left alone, it digests, and wilts
		g.fed = math.Max(0, g.fed-dt*4)
	}
	well := g.fed / 400
	grow := f32(1 + 0.45*well)
	g.bob += dt * (1.5 + 3*well)
	bob := f32(math.Sin(g.bob)) * box.H * 0.05
	w, h := box.W*grow, box.H*grow
	b := ui.Rect{X: box.X + box.W/2 - w/2, Y: box.Y + box.H - h + bob, W: w, H: h}
	col := hsl(300-120*well, 0.35+0.5*well, 0.62)
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, col, 0.95, 0.6+0.8*well)
	ink := faceInk(f.dark, ui.Hex("#1f1530"))
	er := max(1, b.W*0.11)
	for _, ox := range []float32{0.3, 0.7} {
		ex, ey := b.X+b.W*ox+g.gaze()*b.W*0.07, b.Y+b.H*0.38
		p.Fill(ui.Rect{X: ex - er, Y: ey - er, W: 2 * er, H: 2 * er}, ink, er)
	}
	mouth := &ui.Path{}
	mx, my, mw := b.X+b.W/2, b.Y+b.H*0.66, b.W*0.24
	curve := b.H * f32(0.12*(well*2-1)) // a smile when fed, a frown when not
	mouth.MoveTo(mx-mw, my).QuadTo(mx, my+curve, mx+mw, my)
	p.StrokePath(mouth, max(1, b.W*0.07), ink)
	g.hearts.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "♥", f32(m.x), f32(m.y), f32(m.size), withAlpha(ui.Hex("#f43f5e"), 1-age), true, 0, 1+0.3*age)
	})
	idleFrame(p, f) // it breathes
}

// deg is an angle in radians as a rotation takes it, in degrees.
func deg(rad float64) float32 { return float32(rad * 180 / math.Pi) }
