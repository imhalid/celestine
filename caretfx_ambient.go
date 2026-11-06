package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Cursor-Smith's carets that go on round the box (MIT, see NOTICE.md), each as it ships and
// with nothing to set: the pixel trail, stardust, the speed demon, hot-head's flames, the torch.

// moved reports whether the caret box moved since the last frame, and by how much.
func moved(prev, cur ui.Rect) (bool, float64) {
	d := math.Hypot(float64(cur.X-prev.X), float64(cur.Y-prev.Y))
	return d > 0.5, d
}

// jitter is col with each channel moved by up to ±amp/2.
func jitter(col ui.Color, amp float64) ui.Color {
	ch := func(v uint8) uint8 { return uint8(clampf(float64(v)+(rnd()-0.5)*amp, 0, 255)) }
	return ui.RGBA(ch(col.R), ch(col.G), ch(col.B), 1)
}

// ---- pixel trail ----

type squareMote struct {
	x, y, vx, vy, size float64
	t0, life           float64
	col                ui.Color
}

type pixelTrail struct {
	clock
	list []squareMote
	prev ui.Rect
	have bool
}

func newPixelTrail() *pixelTrail { return &pixelTrail{} }

func (g *pixelTrail) reset() { g.list, g.have = nil, false }

func (g *pixelTrail) shift(dy float32) {
	for i := range g.list {
		g.list[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *pixelTrail) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	box := boxOf(f)
	if g.have {
		if mv, dist := moved(g.prev, f.r); mv {
			// a puff from where it was
			aw := float64(max(g.prev.W, box.W))
			n := 6 + int(rnd()*6)
			for i := 0; i < n; i++ {
				g.list = append(g.list, squareMote{x: float64(g.prev.X) + rnd()*aw, y: float64(g.prev.Y) + rnd()*float64(g.prev.H), vx: (rnd() - 0.5) * 20, size: 4 * (0.65 + rnd()*0.7), t0: tn, life: 400, col: jitter(col, 70)})
			}
			if dist >= 60 { // a long jump: a streak of squares along the way
				puffs := int(math.Min(12, math.Floor(dist/14)))
				cx0, cy0 := float64(g.prev.X+box.W/2), float64(g.prev.Y+g.prev.H/2)
				cx1, cy1 := float64(f.r.X+box.W/2), float64(f.r.Y+f.r.H/2)
				for i := 1; i <= puffs; i++ {
					s := float64(i) / float64(puffs+1)
					for j := 0; j < 2; j++ {
						g.list = append(g.list, squareMote{x: cx0 + (cx1-cx0)*s + (rnd()-0.5)*6, y: cy0 + (cy1-cy0)*s + (rnd()-0.5)*float64(box.H)*0.7, vx: (rnd() - 0.5) * 14, size: 4 * 0.8 * (0.65 + rnd()*0.7), t0: tn, life: 400, col: jitter(col, 70)})
					}
				}
			}
		}
	}
	g.prev, g.have = f.r, true
	out := g.list[:0]
	for _, m := range g.list {
		el := tn - m.t0
		if el > m.life {
			continue
		}
		out = append(out, m)
		t := el / m.life
		p.Fill(ui.Rect{X: f32(m.x + m.vx*el/1000), Y: f32(m.y), W: f32(m.size), H: f32(m.size)}, withAlpha(m.col, 1-t*t), 0)
	}
	g.list = out
	drawBoxCursor(p, box, col, 1)
	if len(g.list) > 0 {
		p.AnimationFrame()
	}
}

// ---- stardust ----

type mote struct {
	x, y, vy, sway, swaySpeed, phase, twinkle, size float64
	t0, life                                        float64
	col                                             ui.Color
}

type stardust struct {
	clock
	list      []mote
	lastAct   float64
	lastSpawn float64
	prev      ui.Rect
	have      bool
}

func newStardust() *stardust { return &stardust{} }

func (g *stardust) reset() { g.list, g.have = nil, false }

func (g *stardust) shift(dy float32) {
	for i := range g.list {
		g.list[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *stardust) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	box := boxOf(f)
	if (g.have && f.r != g.prev) || k.wrote > 0 || k.erased > 0 || k.edited {
		g.lastAct = tn
	}
	g.prev, g.have = f.r, true
	if tn-g.lastAct >= 2000 && tn-g.lastSpawn >= 320 && len(g.list) < 24 {
		g.lastSpawn = tn
		g.list = append(g.list, mote{x: float64(box.X) + rnd()*float64(box.W), y: float64(box.Y) + rnd()*float64(box.H)*0.6, vy: -(8 + rnd()*14), sway: 2 + rnd()*5, swaySpeed: 0.6 + rnd()*0.9, phase: rnd() * 2 * math.Pi, twinkle: 2 + rnd()*3, size: 1 + rnd()*1.5, t0: tn, life: (2.2 + rnd()*2.2) * 1000, col: jitter(col, 50)})
	}
	out := g.list[:0]
	for _, m := range g.list {
		e := (tn - m.t0) / 1000
		t := e / (m.life / 1000)
		if t >= 1 {
			continue
		}
		out = append(out, m)
		env := 1 - (t-0.2)/0.8
		if t < 0.2 {
			env = t / 0.2
		}
		a := env * (0.72 + 0.28*math.Sin(e*m.twinkle+m.phase))
		if a <= 0.01 {
			continue
		}
		p.Fill(ui.Rect{X: f32(m.x + math.Sin(e*m.swaySpeed+m.phase)*m.sway), Y: f32(m.y + m.vy*e), W: f32(m.size), H: f32(m.size)}, withAlpha(m.col, a), 0)
	}
	g.list = out
	drawBoxCursor(p, box, col, 1)
	p.AnimationFrame() // waiting for the quiet that makes the dust
}

// ---- speed demon ----

type speedDemon struct {
	clock
	heat      float64
	last      float64
	lastSpawn float64
	sparks    []squareMote
	prev      ui.Rect
	have      bool
}

func newSpeedDemon() *speedDemon { return &speedDemon{} }

func (g *speedDemon) reset() { g.heat, g.sparks, g.have = 0, nil, false }

func (g *speedDemon) shift(dy float32) {
	for i := range g.sparks {
		g.sparks[i].y += float64(dy)
	}
	g.prev.Y += dy
}

var heatStops = []struct {
	at  float64
	col ui.Color
}{{0, ui.Hex("#8a8a8a")}, {0.3, ui.Hex("#d9a441")}, {0.55, ui.Hex("#ff8c1a")}, {0.8, ui.Hex("#ff5a1f")}, {1, ui.Hex("#fff4e0")}}

func heatColor(h float64) ui.Color {
	h = clamp01(h)
	for i := 1; i < len(heatStops); i++ {
		if h <= heatStops[i].at {
			a, b := heatStops[i-1], heatStops[i]
			return mix(a.col, b.col, (h-a.at)/(b.at-a.at))
		}
	}
	return heatStops[len(heatStops)-1].col
}

func (g *speedDemon) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	dt := math.Min(0.1, (tn-g.last)/1000)
	g.last = tn
	box := boxOf(f)
	if k.wrote > 0 {
		g.heat = math.Min(1, g.heat+0.08*float64(k.wrote))
	}
	if g.have && k.wrote == 0 {
		if mv, d := moved(g.prev, f.r); mv {
			g.heat = math.Min(1, g.heat+math.Min(0.12, d/900))
		}
	}
	g.prev, g.have = f.r, true
	g.heat *= math.Exp(-dt / 1.6)
	if g.heat < 0.01 {
		g.heat = 0
	}
	if g.heat >= 0.4 && len(g.sparks) < 120 {
		gap := 70 - 55*g.heat
		if tn-g.lastSpawn >= gap {
			g.lastSpawn = tn
			n := 1 + int(g.heat*3)
			for i := 0; i < n; i++ {
				c := heatColor(math.Min(1, g.heat+0.1))
				c = ui.RGBA(uint8(clampf(float64(c.R)+(rnd()-0.5)*40, 0, 255)), uint8(clampf(float64(c.G)+(rnd()-0.5)*30, 0, 255)), uint8(clampf(float64(c.B)+(rnd()-0.5)*20, 0, 255)), 1)
				g.sparks = append(g.sparks, squareMote{x: float64(box.X) + rnd()*float64(box.W), y: float64(box.Y) + rnd()*float64(box.H)*0.4, vx: (rnd() - 0.5) * 12, vy: -20 - rnd()*30 - g.heat*20, size: 1.5 + rnd()*2, t0: tn, life: 400, col: c})
			}
		}
	}
	out := g.sparks[:0]
	for _, s := range g.sparks {
		el := tn - s.t0
		if el > s.life {
			continue
		}
		out = append(out, s)
		t := el / s.life
		p.Fill(ui.Rect{X: f32(s.x + s.vx*el/1000), Y: f32(s.y + s.vy*el/1000), W: f32(s.size), H: f32(s.size)}, withAlpha(s.col, 1-t*t), 0)
	}
	g.sparks = out
	col := heatColor(g.heat)
	glow := 0.9 * (1 + 1.6*g.heat)
	glowAround(p, box, col, glow)
	p.Fill(box, withAlpha(col, 0.9), boxRadius(box.W, box.H))
	if g.heat > 0 || len(g.sparks) > 0 {
		p.AnimationFrame()
	}
}

// ---- hot-head ----

// flameShapes are the pixel-art shapes a chunk of flame goes through as it shrinks, rows from the top.
var flameShapes = [16][]string{
	{"11", "11", "11", "11"}, {"011", "111", "111"}, {"110", "111", "111"}, {"11", "11", "11"},
	{"01", "11", "11"}, {"10", "11", "11"}, {"111", "111"}, {"1", "1", "1", "1"},
	{"11", "11"}, {"1", "1", "1"}, {"01", "11"}, {"10", "11"},
	{"111"}, {"11"}, {"1", "1"}, {"1"},
}

type flameChunk struct {
	x, y, vx, vy  float64
	life, life0   float64
	shape0        int
	flip          bool
	temp, lift    float64
	amp, freq, ph float64
	so            float64
	age           float64
	spark, fine   bool
}

type flameMark struct {
	x, y, t float64
}

type hotHead struct {
	clock
	chunks []flameChunk
	marks  []flameMark
	last   float64
	warm   float64
	prev   ui.Rect
	have   bool
}

func newHotHead() *hotHead { return &hotHead{} }

func (g *hotHead) reset() { g.chunks, g.marks, g.have = nil, nil, false }

func (g *hotHead) shift(dy float32) {
	for i := range g.chunks {
		g.chunks[i].y += float64(dy)
	}
	for i := range g.marks {
		g.marks[i].y += float64(dy)
	}
	g.prev.Y += dy
}

// flameColor is the fire's palette at index j of 0..16: the caret's own color, then amber, orange, deep orange.
func flameColor(base ui.Color, u float64) ui.Color {
	stops := []ui.Color{base, hsvColor(44, 0.88, 0.92), hsvColor(30, 0.88, 0.92), hsvColor(12, 0.84, 0.92), hsvColor(26, 0.20, 1)}
	pos := []float64{0, 0.12, 0.42, 0.72, 1}
	for i := 0; i < 4; i++ {
		if u <= pos[i+1] {
			t := (u - pos[i]) / (pos[i+1] - pos[i])
			return mix(stops[i], stops[i+1], 0.7*t+0.3*easeSine(t))
		}
	}
	return stops[4]
}

func hsvColor(h, s, v float64) ui.Color {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch int(h / 60) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	default:
		r, g, b = 0, c, x
	}
	return ui.RGBA(uint8((r+m)*255+0.5), uint8((g+m)*255+0.5), uint8((b+m)*255+0.5), 1)
}

func (g *hotHead) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	dt := math.Min(0.1, math.Max(0.001, (tn-g.last)/1000))
	g.last = tn
	box := boxOf(f)
	col := cursorColor(f.dark)
	cw := float64(box.W)
	fs := float64(textSize(f))
	px := math.Max(2, math.Min(5, math.Round(fs/4.5)))
	// the text the caret has been on keeps burning
	cx := float64(box.X) + cw/2
	if g.have {
		if mv, _ := moved(g.prev, f.r); mv {
			if n := len(g.marks); n > 0 && math.Abs(g.marks[n-1].x-cx) < 0.5*cw && math.Abs(g.marks[n-1].y-float64(box.Y)) < 2 {
				g.marks[n-1].t = tn
			} else {
				g.marks = append(g.marks, flameMark{cx, float64(box.Y), tn})
			}
			if math.Abs(float64(f.r.Y-g.prev.Y)) < 2 {
				dx := cx - float64(g.prev.X+box.W/2)
				for i := 1; i <= int(math.Min(24, math.Abs(dx)/cw)); i++ {
					s := float64(i) * cw / math.Max(1, math.Abs(dx))
					g.marks = append(g.marks, flameMark{cx - dx*s, float64(box.Y), tn - s*0.35*720})
				}
			}
		}
	}
	g.prev, g.have = f.r, true
	if k.wrote > 0 || k.erased > 0 {
		g.warm = math.Min(1, g.warm+0.14*math.Max(1, float64(k.wrote+k.erased)))
		if n := len(g.marks); n == 0 || math.Abs(g.marks[n-1].x-cx) >= 0.5*cw {
			g.marks = append(g.marks, flameMark{cx, float64(box.Y), tn})
		} else {
			g.marks[n-1].t = tn
		}
	}
	g.warm *= math.Exp(-dt * 1000 / 1200)
	const linger = 720.0
	om := g.marks[:0]
	W := 0.0
	for _, m := range g.marks {
		if s := 1 - (tn-m.t)/linger; s > 0 {
			om = append(om, m)
			W += s
		}
	}
	if len(om) > 64 {
		om = om[len(om)-64:]
	}
	g.marks = om
	// new flame, from the marks, more the faster it is written
	if W > 0 && len(g.chunks) < 70 {
		typed := 0.15 + 0.85*g.warm
		n := 70 * dt * (1 + 2.0/6) * math.Min(2.4, 0.55+W*0.45) * typed
		cnt := int(n)
		if rnd() < n-float64(cnt) {
			cnt++
		}
		for ; cnt > 0 && len(g.chunks) < 70; cnt-- {
			// a mark, in proportion to how fresh it is
			r := rnd() * W
			mk := g.marks[0]
			for _, m := range g.marks {
				r -= 1 - (tn-m.t)/linger
				mk = m
				if r <= 0 {
					break
				}
			}
			s := 1 - (tn-mk.t)/linger
			spark := rnd() < 0.3
			lh := float64(box.H)
			x0 := mk.x + (rnd()*2-1)*(2*cw*0.5) + (rnd()-0.5)*0.5*cw
			y0 := mk.y + math.Max(0, lh-fs)/2 + fs*0.06 + (0.02-rnd()*0.09)*lh
			ang := -math.Pi/2 + (rnd()*2-1)*0.45
			mag := 6 * math.Sqrt(rnd()) * cw * 0.44
			life0 := 620 * (0.25 + 0.75*math.Pow(rnd(), 2.2)) * (0.7 + 0.3*s)
			c := flameChunk{x: x0, y: y0, vx: mag * math.Cos(ang), vy: mag * math.Sin(ang), life: life0, life0: life0,
				shape0: int(clampf(math.Round((1-life0/620)*15+(rnd()-0.5)*3), 0, 15)), flip: rnd() < 0.5,
				temp: math.Min(1, (0.55+rnd()*0.45)*(0.45+0.55*s)), lift: 0.44, amp: 0.3 * (0.6 + 0.4*rnd()), freq: 1.3 * (0.75 + 0.5*rnd()), ph: rnd() * 2 * math.Pi}
			if spark {
				c.spark = true
				c.life = 620 * (0.15 + 0.65*rnd()) * (0.5 + 0.5*s)
				c.life0 = c.life
				c.lift = 0.44 * 1.9
				c.temp = 0.5 + rnd()*0.5
				c.fine = rnd() < 0.4
			}
			g.chunks = append(g.chunks, c)
		}
	}
	// physics, then paint, the oldest first
	damp := math.Pow(0.8, dt*1000/17)
	out := g.chunks[:0]
	for _, c := range g.chunks {
		c.vy = (c.vy + (-85*c.lift+62*0.5*(rnd()-0.5))*cw*dt) * damp
		c.vx = c.vx*damp + 62*0.15*(rnd()-0.5)*cw*dt
		c.x += c.vx * dt
		c.y += c.vy * dt
		c.age += dt * 1000
		so := c.amp * cw * math.Min(1, c.age/260) * math.Sin(2*math.Pi*c.freq*c.age/1000+c.ph)
		c.x += so - c.so
		c.so = so
		c.life -= dt * 1000
		if c.life <= 0 {
			continue
		}
		out = append(out, c)
		g.paintChunk(p, c, col, px, box)
	}
	g.chunks = out
	drawBoxCursor(p, box, col, 1)
	if len(g.chunks) > 0 || len(g.marks) > 0 {
		p.AnimationFrame()
	}
}

func (g *hotHead) paintChunk(p *ui.Painter, c flameChunk, base ui.Color, px float64, box ui.Rect) {
	frac := clamp01(c.life / 620)
	quant := func(v float64, l float64) float64 { return math.Round(v*l) / l }
	temp := quant(c.temp*math.Pow(frac, 0.55), 3) * 0.5
	pi := math.Round(temp * 16)
	col := flameColor(base, pi/16)
	const opa = 0.8
	cell := func(x, y float64) (float64, float64) { return math.Floor(x/px) * px, math.Floor(y/px) * px }
	if c.spark {
		q := quant(frac, 3)
		if q <= 0 {
			return
		}
		a, sz := 0.2+0.5*q, math.Max(1, math.Round(px*0.85))
		if c.fine {
			a, sz = 0.15+0.35*q, math.Max(1, math.Round(px*0.5))
		}
		gx, gy := cell(c.x, c.y)
		p.Fill(ui.Rect{X: f32(gx), Y: f32(gy), W: f32(sz), H: f32(sz)}, withAlpha(col, a*opa), 0)
		return
	}
	if frac <= 0.22 {
		q := quant(frac/0.22, 3)
		gx, gy := cell(c.x, c.y)
		p.Fill(ui.Rect{X: f32(gx), Y: f32(gy), W: f32(math.Max(1, math.Round(px*0.85))), H: f32(math.Max(1, math.Round(px*0.85)))}, withAlpha(col, (0.15+0.25*q)*opa), 0)
		return
	}
	gone := math.Pow(1-clamp01(c.life/c.life0), 1.6)
	idx := int(math.Min(15, float64(c.shape0)+math.Floor(gone*float64(16-c.shape0))))
	rows := flameShapes[idx]
	area := 0 // the cells it has
	for _, r := range rows {
		for _, ch := range r {
			if ch == '1' {
				area++
			}
		}
	}
	stage := 1.0
	switch {
	case area >= 6:
		stage = 3
	case area >= 4:
		stage = 2
	case area >= 2:
		stage = 1.5
	}
	q := quant((frac-0.22)/0.78, 3)
	if q <= 0 {
		return
	}
	a := 0.45 + 0.25*q
	if stage >= 3 {
		a = 0.70 + 0.25*q
	} else if stage >= 2 {
		a = 0.60 + 0.25*q
	}
	gx, gy := math.Floor(c.x/px), math.Floor(c.y/px)
	w := len(rows[0])
	for ri, r := range rows {
		for i := 0; i < len(r); i++ {
			if r[i] != '1' {
				continue
			}
			ci := i
			if c.flip {
				ci = w - 1 - i
			}
			x, y := (gx+float64(ci))*px, (gy-float64(len(rows)-1-ri))*px
			rect := ui.Rect{X: f32(x), Y: f32(y), W: f32(px), H: f32(px)}
			if rect.X < box.X+box.W && rect.X+rect.W > box.X && rect.Y < box.Y+box.H && rect.Y+rect.H > box.Y { // never over the caret itself
				continue
			}
			p.Fill(rect, withAlpha(col, a*opa), 0)
		}
	}
}

// ---- torch ----

type torch struct {
	clock
	x, y   float64
	placed bool
	last   float64
}

func newTorch() *torch { return &torch{} }

func (g *torch) reset() { g.placed = false }

func (g *torch) shift(dy float32) { g.y += float64(dy) }

func (g *torch) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	dt := math.Min(0.1, math.Max(0.001, (tn-g.last)/1000))
	g.last = tn
	box := boxOf(f)
	tx, ty := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if !g.placed {
		g.x, g.y, g.placed = tx, ty, true
	}
	a := 1 - math.Pow(0.78, dt*60)
	g.x += (tx - g.x) * a
	g.y += (ty - g.y) * a
	if math.Abs(tx-g.x) < 0.25 && math.Abs(ty-g.y) < 0.25 {
		g.x, g.y = tx, ty
	}
	b := f.bounds
	if b.W > 0 && b.H > 0 {
		const R, dark = 250.0, 0.7
		cen := ui.Point{X: f32((g.x - float64(b.X)) / float64(b.W)), Y: f32((g.y - float64(b.Y)) / float64(b.H))}
		if cen == (ui.Point{}) {
			cen.X = 1e-6
		}
		black := ui.RGBA(0, 0, 0, 1)
		p.FillRadialGradient(b, ui.RadialGradient{Center: cen, Radius: R, Stops: []ui.GradientStop{
			{Color: withAlpha(black, 0), At: 0}, {Color: withAlpha(black, 0.52*dark), At: 0.4}, {Color: withAlpha(black, 0.91*dark), At: 0.7}, {Color: withAlpha(black, dark), At: 1}}}, 0)
		warm := ui.Hex("#ff963c")
		p.FillRadialGradient(b, ui.RadialGradient{Center: cen, Radius: R * 0.6, Stops: []ui.GradientStop{
			{Color: withAlpha(warm, 0.04), At: 0}, {Color: withAlpha(warm, 0.012), At: 0.45}, {Color: withAlpha(warm, 0), At: 0.75}}}, 0)
	}
	drawBoxCursor(p, box, cursorColor(f.dark), 1)
	if math.Hypot(tx-g.x, ty-g.y) > 0.25 {
		p.AnimationFrame()
	}
}

// ---- tiles ----

func pixelTrailMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	for i := 0; i < 9; i++ {
		p.Fill(ui.Rect{X: r.X + 3 + float32(i)*3.2, Y: r.Y + 5 + float32((i*7)%11), W: 3.5, H: 3.5}, withAlpha(col, 0.2+0.08*float64(i)), 0)
	}
	p.Fill(ui.Rect{X: r.X + 32, Y: r.Y + 2, W: 5, H: r.H - 4}, withAlpha(col, 0.9), 1.5)
}

func stardustMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H - 12, W: 8, H: 11}, withAlpha(col, 0.9), 2)
	for i := 0; i < 6; i++ {
		p.Fill(ui.Rect{X: r.X + r.W/2 - 8 + float32(i)*3, Y: r.Y + 2 + float32((i*5)%9), W: 1.8, H: 1.8}, withAlpha(col, 0.35+0.1*float64(i)), 0)
	}
}

func speedMock(p *ui.Painter, r ui.Rect, dark bool) {
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + 5, W: 8, H: r.H - 6}, withAlpha(heatColor(0.85), 0.95), 2)
	for i := 0; i < 5; i++ {
		p.Fill(ui.Rect{X: r.X + r.W/2 - 6 + float32(i)*3, Y: r.Y + 1 + float32((i*3)%5), W: 2, H: 2}, withAlpha(heatColor(0.6+0.08*float64(i)), 0.9), 0)
	}
}

func hotHeadMock(p *ui.Painter, r ui.Rect, dark bool) {
	base := cursorColor(dark)
	for i, rows := range [][]string{flameShapes[1], flameShapes[8], flameShapes[10]} {
		for ri, row := range rows {
			for ci, ch := range row {
				if ch == '1' {
					p.Fill(ui.Rect{X: r.X + 6 + float32(i)*9 + float32(ci)*3, Y: r.Y + r.H - 4 - float32(len(rows)-ri)*3 - 3, W: 3, H: 3}, withAlpha(flameColor(base, 0.2+0.25*float64(i)), 0.85), 0)
				}
			}
		}
	}
	p.Fill(ui.Rect{X: r.X + 2, Y: r.Y + r.H - 4, W: r.W - 4, H: 3}, withAlpha(base, 0.5), 1)
}

func torchMock(p *ui.Painter, r ui.Rect, dark bool) {
	p.FillRadialGradient(r, ui.RadialGradient{Radius: r.H * 1.1, Stops: []ui.GradientStop{
		{Color: ui.RGBA(0, 0, 0, 0), At: 0}, {Color: ui.RGBA(0, 0, 0, 0.5), At: 0.5}, {Color: ui.RGBA(0, 0, 0, 0.8), At: 1}}}, 4)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 2, Y: r.Y + 4, W: 4, H: r.H - 8}, withAlpha(cursorColor(dark), 0.95), 1.5)
}
