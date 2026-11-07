package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of games and creatures, each fixed as it ships:
//
//	Tetris      every letter drops a block under the line; Enter clears the row in a flash; erasing throws the last one up
//	Yılan       a snake whose tail grows as it writes and follows the caret, and draws back when it rests or erases
//	Kedi        a cat: it naps when nothing is written, pricks its ears when typing is quick, runs after a caret that jumps
//	Tamagotchi  a pet fed by what is written: it grows and brightens, hearts at every hundred letters; it wilts when left

// fxBase is what most of these carets keep: their own time, where the caret was, when something last happened.
type fxBase struct {
	clock
	prev   ui.Rect
	have   bool
	last   float64 // ms, the last frame
	active float64 // ms, when something was last written, erased or moved
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
	b.prev, b.have = f.r, true
	if moved || k.wrote > 0 || k.erased > 0 {
		b.active = tn
	}
	return
}

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

// ---- Tetris ----

var tetrisColors = []string{"#22d3ee", "#facc15", "#a855f7", "#22c55e", "#ef4444", "#3b82f6", "#f97316"}

type tetrisBlock struct {
	x, y, vy, land float64
	col            ui.Color
	t0             float64
	landed, gone   bool
	goneAt         float64
}

type tetris struct {
	fxBase
	blocks []tetrisBlock
	next   int
	flash  float64 // when the row was cleared
	up     motes   // blocks thrown up by erasing
}

func newTetris() *tetris { return &tetris{flash: -1e9} }

func (g *tetris) reset() { g.blocks, g.up, g.have = nil, nil, false }

func (g *tetris) shift(dy float32) {
	for i := range g.blocks {
		g.blocks[i].y += float64(dy)
		g.blocks[i].land += float64(dy)
	}
	g.up.shift(dy)
	g.prev.Y += dy
}

func (g *tetris) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	s := math.Max(4, h*0.32) // a block's side
	for i := 0; i < k.wrote && i < 4; i++ {
		g.blocks = append(g.blocks, tetrisBlock{x: float64(box.X) - float64(i)*s, y: float64(box.Y) - 2.5*h, land: float64(box.Y+box.H) + 1.5,
			col: ui.Hex(tetrisColors[g.next%len(tetrisColors)]), t0: tn})
		g.next++
	}
	if strings.Contains(k.typed, "\n") { // the row is full: it flashes, and goes
		g.flash = tn
		for i := range g.blocks {
			if !g.blocks[i].gone {
				g.blocks[i].gone, g.blocks[i].goneAt = true, tn
			}
		}
	}
	for i := 0; i < k.erased && len(g.blocks) > 0; i++ { // the last block is thrown up
		b := g.blocks[len(g.blocks)-1]
		g.blocks = g.blocks[:len(g.blocks)-1]
		g.up = append(g.up, mote{x: b.x, y: b.y, vx: (rnd() - 0.5) * 80, vy: -180 - rnd()*80, spin: (rnd() - 0.5) * 10, t0: tn, life: 600, size: s, grav: 700, col: b.col})
	}
	if len(g.blocks) > 60 {
		g.blocks = g.blocks[len(g.blocks)-60:]
	}
	out := g.blocks[:0]
	for _, b := range g.blocks {
		if b.gone && tn-b.goneAt > 260 {
			continue
		}
		if !b.landed {
			b.vy += 2200 * dt
			b.y += b.vy * dt
			if b.y >= b.land {
				b.y, b.landed = b.land, true
			}
		}
		a := 1.0
		if age := (tn - b.t0) / 9000; age > 0.7 { // an old row fades on its own
			a = 1 - (age-0.7)/0.3
		}
		col := b.col
		if b.gone {
			k := (tn - b.goneAt) / 260
			col, a = mix(b.col, ui.RGBA(255, 255, 255, 1), math.Min(1, k*2)), a*(1-k)
		}
		if a <= 0 {
			continue
		}
		r := ui.Rect{X: f32(b.x), Y: f32(b.y), W: f32(s - 1), H: f32(s - 1)}
		p.Fill(r, withAlpha(col, a), rad(1.5))
		p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: max(1, r.H*0.28)}, ui.RGBA(255, 255, 255, float32(0.35*a)), 0)
		out = append(out, b)
	}
	g.blocks = out
	g.up.stepDraw(p, tn, dt, func(m *mote, age float64) {
		r := ui.Rect{X: f32(m.x), Y: f32(m.y), W: f32(m.size - 1), H: f32(m.size - 1)}
		p.Transform(ui.Rotation(deg(m.ang)), r.X+r.W/2, r.Y+r.H/2, func() { p.Fill(r, withAlpha(m.col, 1-age), rad(1.5)) })
	})
	col := ui.Hex(tetrisColors[g.next%len(tetrisColors)]) // the body is the next block's color
	glow := 0.9
	if fl := (tn - g.flash) / 300; fl < 1 {
		col, glow = mix(col, ui.RGBA(255, 255, 255, 1), 1-fl), 2
	}
	glassBody(p, f, box, col, 1, glow)
	if len(g.blocks) > 0 || len(g.up) > 0 || tn-g.flash < 300 {
		p.AnimationFrame()
	}
}

// ---- Yılan ----

type snakePt struct{ x, y float64 }

type snake struct {
	fxBase
	trail  []snakePt // where the head has been, the newest last
	segs   float64   // how long it is, in segments
	tongue float64   // when it last flicked its tongue
	left   bool
}

func newSnake() *snake { return &snake{segs: 2, tongue: -1e9} }

func (g *snake) reset() { g.trail, g.have, g.segs = nil, false, 2 }

func (g *snake) shift(dy float32) {
	for i := range g.trail {
		g.trail[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *snake) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, moved, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	gap := h * 0.24 // between two segments: less than one's width, so they touch
	// the body slithers just under the line, not over the letters; the head is the caret
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H)+h*0.14
	if moved && g.have && len(g.trail) > 0 {
		g.left = cx < g.trail[len(g.trail)-1].x
	}
	// the path the head went along, a point every so often
	if n := len(g.trail); n == 0 {
		g.trail = append(g.trail, snakePt{cx, cy})
	} else if last := g.trail[n-1]; math.Hypot(cx-last.x, cy-last.y) > gap/3 {
		// the caret moves a letter at a time: the path between is filled in, so the body stays whole
		d := math.Hypot(cx-last.x, cy-last.y)
		steps := int(math.Min(60, d/(gap/3)))
		for i := 1; i <= steps; i++ {
			t := float64(i) / float64(steps)
			g.trail = append(g.trail, snakePt{last.x + (cx-last.x)*t, last.y + (cy-last.y)*t})
		}
	}
	g.segs = math.Min(26, g.segs+float64(k.wrote)*0.6)
	if k.erased > 0 {
		g.segs = math.Max(1, g.segs-float64(k.erased))
		g.tongue = tn
	}
	if g.idle(tn) > 1.2 { // resting, it draws its tail in
		g.segs = math.Max(1, g.segs-dt*6)
	}
	keep := int(g.segs*3) + 4
	if len(g.trail) > keep {
		g.trail = g.trail[len(g.trail)-keep:]
	}
	// the body: segments along the trail, from the tail, thinning toward it
	green, dark := ui.Hex("#4ade80"), ui.Hex("#15803d")
	n := int(g.segs)
	for i := n; i >= 1; i-- {
		at := len(g.trail) - 1 - i*3
		if at < 0 {
			continue
		}
		pt := g.trail[at]
		r := h * 0.16 * (1 - 0.45*float64(i)/math.Max(1, float64(n)))
		col := green
		if i%2 == 0 {
			col = mix(green, dark, 0.35)
		}
		p.Fill(ui.Rect{X: f32(pt.x - r), Y: f32(pt.y - r), W: f32(2 * r), H: f32(2 * r)}, withAlpha(col, 0.85), f32(r))
	}
	glassBody(p, f, box, green, 1, 0.9)
	// the head's eyes, toward where it goes, and its tongue when it erases
	d := float32(1)
	if g.left {
		d = -1
	}
	er := max(1.2, box.W*0.13)
	for _, oy := range []float32{0.28, 0.52} {
		ex, ey := box.X+box.W/2+d*box.W*0.18, box.Y+box.H*oy
		p.Fill(ui.Rect{X: ex - er, Y: ey - er, W: 2 * er, H: 2 * er}, ui.RGBA(255, 255, 255, 1), er)
		p.Fill(ui.Rect{X: ex - er*0.5 + d*er*0.3, Y: ey - er*0.5, W: er, H: er}, ui.Hex("#052e16"), er/2)
	}
	if tk := (tn - g.tongue) / 260; tk < 1 {
		l := float32(math.Sin(tk*math.Pi)) * box.W * 1.2
		x0 := box.X + box.W/2 + d*box.W/2
		y0 := box.Y + box.H*0.72
		t := &ui.Path{}
		t.MoveTo(x0, y0).LineTo(x0+d*l, y0).MoveTo(x0+d*l, y0).LineTo(x0+d*(l+3), y0-2).MoveTo(x0+d*l, y0).LineTo(x0+d*(l+3), y0+2)
		p.StrokePath(t, 1.2, ui.Hex("#e11d48"))
	}
	if n > 1 || tn-g.tongue < 260 {
		p.AnimationFrame()
	}
}

// ---- Kedi ----

type cat struct {
	fxBase
	x, y, vx, vy float64 // where its body is: on a spring after the caret
	placed       bool
	perk         float64 // how pricked its ears are
	rate         float64 // letters a second, smoothed
	left         bool
}

func newCat() *cat { return &cat{} }

func (g *cat) reset() { g.placed, g.have = false, false }

func (g *cat) shift(dy float32) { g.y += float64(dy); g.prev.Y += dy }

func (g *cat) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	tx, ty := float64(box.X), float64(box.Y)
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	if math.Abs(tx-g.x) > 0.5 {
		g.left = tx < g.x
	}
	// it runs after the caret: quick, with a little overshoot, its legs a blur while it runs
	const w0, z = 26.0, 0.75
	g.vx += (w0*w0*(tx-g.x) - 2*z*w0*g.vx) * dt
	g.vy += (w0*w0*(ty-g.y) - 2*z*w0*g.vy) * dt
	g.x += g.vx * dt
	g.y += g.vy * dt
	speed := math.Hypot(g.vx, g.vy)
	running := speed > 60
	if !running && math.Hypot(tx-g.x, ty-g.y) < 0.3 {
		g.x, g.y, g.vx, g.vy = tx, ty, 0, 0
	}
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*3)
	want := clamp01(g.rate / 8)
	g.perk += (want - g.perk) * math.Min(1, dt*6)
	asleep := g.idle(tn) > 4
	b := ui.Rect{X: f32(g.x), Y: f32(g.y), W: box.W, H: box.H}
	orange := ui.Hex("#fb923c")
	if f.dark {
		orange = ui.Hex("#fdba74")
	}
	// its ears, its tail, its body, its face
	earH := b.H * f32(0.22+0.16*g.perk)
	if asleep {
		earH = b.H * 0.16
	}
	ear := &ui.Path{}
	ear.MoveTo(b.X, b.Y+2).LineTo(b.X+b.W*0.18, b.Y-earH).LineTo(b.X+b.W*0.45, b.Y+2).Close()
	ear.MoveTo(b.X+b.W*0.55, b.Y+2).LineTo(b.X+b.W*0.82, b.Y-earH).LineTo(b.X+b.W, b.Y+2).Close()
	p.FillPath(ear, orange)
	sway := float32(math.Sin(tn/1000*2.2)) * b.H * 0.12
	if asleep {
		sway *= 0.3
	}
	side := b.X + b.W
	d := float32(1)
	if g.left {
		side, d = b.X, -1
	}
	tail := &ui.Path{}
	tail.MoveTo(side, b.Y+b.H*0.85).CubeTo(side+d*b.H*0.4, b.Y+b.H*0.9, side+d*b.H*0.5+sway, b.Y+b.H*0.3, side+d*b.H*0.25+sway, b.Y+b.H*0.1)
	p.StrokePath(tail, max(1.5, b.W*0.22), orange)
	fr := caretFrame{r: ui.Rect{X: b.X, Y: b.Y, W: f.r.W, H: f.r.H}, cw: f.cw, dark: f.dark, bg: f.bg, bounds: f.bounds, style: f.style}
	if running {
		fr.jelly = nil
	} else {
		fr.jelly = f.jelly
	}
	glassBody(p, fr, b, orange, 1, 0.8)
	if running { // legs, a blur under it
		for i := 0; i < 3; i++ {
			lx := b.X + b.W*float32(0.2+0.3*float64(i)) - d*float32(i)*2
			p.Fill(ui.Rect{X: lx, Y: b.Y + b.H, W: max(1, b.W*0.12), H: b.H * 0.18}, withAlpha(orange, 0.5), 0)
		}
	}
	ink := faceInk(f.dark, ui.Hex("#431407"))
	ey := b.Y + b.H*0.34
	for _, ox := range []float32{0.3, 0.7} {
		ex := b.X + b.W*ox
		if asleep {
			p.Fill(ui.Rect{X: ex - b.W*0.12, Y: ey, W: b.W * 0.24, H: max(1, b.H*0.04)}, ink, 0)
		} else {
			er := max(1, b.W*0.1)
			p.Fill(ui.Rect{X: ex - er, Y: ey - er*1.3, W: 2 * er, H: 2.6 * er}, ink, er)
		}
	}
	if asleep { // z's rise from it
		for i := 0; i < 3; i++ {
			ph := math.Mod(tn/1600+float64(i)/3, 1)
			zx := float64(b.X+b.W) + float64(d)*(4+ph*10) + float64(i)*2
			zy := float64(b.Y) - ph*float64(b.H)*1.2
			glyphAt(p, "z", f32(zx), f32(zy), b.H*f32(0.3+0.2*ph), withAlpha(orange, 1-ph), true, 0, 1)
		}
	}
	p.AnimationFrame() // its tail sways, always
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
		ex, ey := b.X+b.W*ox, b.Y+b.H*0.38
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
	p.AnimationFrame() // it breathes
}

// deg is an angle in radians as a rotation takes it, in degrees.
func deg(rad float64) float32 { return float32(rad * 180 / math.Pi) }
