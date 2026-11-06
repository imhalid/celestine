package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Carets after Cursor-Smith (https://github.com/Sadsnake1/cursor-smith, MIT, see
// NOTICE.md), each fixed as it ships, with nothing to set: the cursor's own look -
// the smear, the energy beam and the aurora, the phosphor ghosts of the CRT and
// the neon - drawn by this app's painter.

// cursorColor is the color of Cursor-Smith's box cursor, for the dark appearance and the light.
func cursorColor(dark bool) ui.Color {
	if dark {
		return ui.Hex("#31edae")
	}
	return ui.Hex("#147133")
}

// rrect is a rounded rectangle as a path.
func rrect(x, y, w, h, r float32) *ui.Path {
	r = max(0, min(r, min(w, h)/2))
	if r < 0.1 {
		pt := &ui.Path{}
		return pt.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h).LineTo(x, y+h).Close()
	}
	const k = 0.5523
	pt := &ui.Path{}
	pt.MoveTo(x+r, y).LineTo(x+w-r, y)
	pt.CubeTo(x+w-r+r*k, y, x+w, y+r-r*k, x+w, y+r)
	pt.LineTo(x+w, y+h-r)
	pt.CubeTo(x+w, y+h-r+r*k, x+w-r+r*k, y+h, x+w-r, y+h)
	pt.LineTo(x+r, y+h)
	pt.CubeTo(x+r-r*k, y+h, x, y+h-r+r*k, x, y+h-r)
	pt.LineTo(x, y+r)
	pt.CubeTo(x, y+r-r*k, x+r-r*k, y, x+r, y)
	return pt.Close()
}

// boxRadius is the corner of a box caret: a quarter of its short side.
func boxRadius(w, h float32) float32 { return min(w, h) * 0.25 }

// glowAround paints the soft halo of a box: wider and fainter passes under it.
func glowAround(p *ui.Painter, r ui.Rect, col ui.Color, alpha float64) {
	for _, k := range []float32{10, 7.5, 5, 2.5} {
		p.Fill(ui.Rect{X: r.X - k, Y: r.Y - k, W: r.W + 2*k, H: r.H + 2*k}, withAlpha(col, alpha*0.12*(1-float64(k)/12)), boxRadius(r.W, r.H)+k)
	}
}

// ---- Smear ----

// smear is Jell-O: the box stretches toward where it came from and snaps back, on a spring.
type smear struct {
	lx, ly, lvx, lvy float64 // the leading face
	tx, ty           float64 // the trailing face
	dx, dy           float64 // the way it moves, held when it stops
	pcx, pcy         float64
	placed           bool
	last             time.Time
}

func newSmear() *smear { return &smear{} }

func (s *smear) reset() { s.placed = false }

func (s *smear) shift(dy float32) {
	s.ly += float64(dy)
	s.ty += float64(dy)
	s.pcy += float64(dy)
}

func (s *smear) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	dt := dtOf(now, s.last)
	s.last = now
	T := f.r
	w, h := float64(max(f.cw, 6)), float64(T.H)
	tx, ty := float64(T.X), float64(T.Y)
	cx, cy := tx+w/2, ty+h/2
	if !s.placed {
		s.lx, s.ly, s.tx, s.ty, s.pcx, s.pcy, s.placed = tx, ty, tx, ty, cx, cy, true
	}
	if d := math.Hypot(cx-s.pcx, cy-s.pcy); d > 0.01 {
		s.dx, s.dy = (cx-s.pcx)/d, (cy-s.pcy)/d
		s.pcx, s.pcy = cx, cy
	}
	const stiff, trailStiff, damp = 0.65, 0.15, 0.4
	wL := 2 + stiff*38
	wT := 2 + trailStiff*38
	zeta := 0.15 + damp*1.15
	steps := int(math.Min(16, math.Max(1, math.Ceil(dt*240))))
	hh := dt / float64(steps)
	for i := 0; i < steps; i++ {
		s.lvx += (wL*wL*(tx-s.lx) - 2*zeta*wL*s.lvx) * hh
		s.lx += s.lvx * hh
		s.lvy += (wL*wL*(ty-s.ly) - 2*zeta*wL*s.lvy) * hh
		s.ly += s.lvy * hh
		f := 1 - math.Exp(-wT*hh)
		s.tx += (s.lx - s.tx) * f
		s.ty += (s.ly - s.ty) * f
	}
	for _, v := range []*float64{&s.lx, &s.ly, &s.tx, &s.ty, &s.lvx, &s.lvy} { // a stray value is a reset
		if math.IsNaN(*v) || math.IsInf(*v, 0) {
			s.lx, s.ly, s.tx, s.ty, s.lvx, s.lvy = tx, ty, tx, ty, 0, 0
		}
	}
	capLen := 150.0 // how far a face may stray
	for _, q := range [][2]*float64{{&s.lx, &s.ly}, {&s.tx, &s.ty}} {
		if d := math.Hypot(*q[0]-tx, *q[1]-ty); d > capLen {
			*q[0], *q[1] = tx+(*q[0]-tx)*capLen/d, ty+(*q[1]-ty)*capLen/d
		}
	}
	col := cursorColor(f.dark)
	offs := [4][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}}
	var c [4][2]float64
	maxLag := 0.0
	var lag [4]float64
	for i, o := range offs {
		sx, sy := -1.0, -1.0
		if o[0] > 0 {
			sx = 1
		}
		if o[1] > 0 {
			sy = 1
		}
		ax, ay := math.Abs(s.dx), math.Abs(s.dy)
		hx, wy := math.Pow(h*ax, 4), math.Pow(w*ay, 4)
		u := 0.5
		if hx+wy > 0 {
			u = hx / (hx + wy)
		}
		front := sx*sgn(s.dx)*u + sy*sgn(s.dy)*(1-u)
		fr := (1 - front) / 2
		c[i] = [2]float64{s.lx + (s.tx-s.lx)*fr + o[0], s.ly + (s.ty-s.ly)*fr + o[1]}
		lag[i] = math.Hypot(c[i][0]-(tx+o[0]), c[i][1]-(ty+o[1]))
		maxLag = math.Max(maxLag, lag[i])
	}
	if maxLag < 0.5 && math.Hypot(s.lvx, s.lvy) < 8 { // settled: the plain box
		s.lx, s.ly, s.tx, s.ty, s.lvx, s.lvy = tx, ty, tx, ty, 0, 0
		box := ui.Rect{X: T.X, Y: T.Y, W: f32(w), H: T.H}
		glowAround(p, box, col, 0.9)
		p.Fill(box, withAlpha(col, 0.9), boxRadius(box.W, box.H))
		return
	}
	// the tail thins toward the line it travels
	reach := clamp01((maxLag - 4) / 20)
	for i, o := range offs {
		offx, offy := c[i][0]-cx, c[i][1]-cy
		along := offx*s.dx + offy*s.dy
		px, py := offx-along*s.dx, offy-along*s.dy
		k := 0.0
		if maxLag > 0 {
			k = lag[i] / maxLag * reach * 0.7
		}
		c[i][0] -= px * k
		c[i][1] -= py * k
		_ = o
	}
	path := &ui.Path{}
	path.MoveTo(f32(c[0][0]), f32(c[0][1]))
	for i := 1; i < 4; i++ {
		path.LineTo(f32(c[i][0]), f32(c[i][1]))
	}
	path.Close()
	for i, wd := range []float32{9, 6, 3.5} {
		p.StrokePath(path, wd, withAlpha(col, []float64{0.04, 0.07, 0.1}[i]))
	}
	p.FillPath(path, withAlpha(col, 0.9))
	p.AnimationFrame()
}

func sgn(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// ---- Energy beam and Aurora ----

// energy fills the box with a light that runs along it: a bright band and a dark one move up
// through it, the colors drifting. The aurora does it with a ramp of colors, warped.
type energy struct {
	aurora bool
	t0     time.Time
}

func newEnergy(aurora bool) *energy { return &energy{aurora: aurora} }

func (e *energy) reset()        {}
func (e *energy) shift(float32) {}

var auroraRamp = []ui.Color{ui.Hex("#31edae"), ui.Hex("#4aa3ff"), ui.Hex("#c792ea"), ui.Hex("#f5a623")}

// ramp samples the colors as a cycle.
func ramp(pos float64) ui.Color {
	n := float64(len(auroraRamp))
	pos = pos - math.Floor(pos)
	i := pos * n
	a, b := auroraRamp[int(i)%len(auroraRamp)], auroraRamp[(int(i)+1)%len(auroraRamp)]
	return mix(a, b, i-math.Floor(i))
}

func (e *energy) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	if e.t0.IsZero() {
		e.t0 = now
	}
	t := now.Sub(e.t0).Seconds()
	box := ui.Rect{X: f.r.X, Y: f.r.Y, W: f32(math.Max(float64(f.cw), 6)), H: f.r.H}
	base := cursorColor(f.dark)
	n := 8
	if e.aurora {
		n = 20
	}
	stops := make([]ui.GradientStop, 0, n+1)
	for i := 0; i <= n; i++ {
		pos := float64(i) / float64(n) // 0 at the bottom, 1 at the top
		var col ui.Color
		punch := 1.0
		if e.aurora {
			warp := math.Sin(pos*3.1+t*0.85)*0.26 + math.Sin(pos*5.7-t*0.55)*0.14 + math.Sin(pos*1.3+t*1.25)*0.20
			near := ramp(pos - t*0.3 + warp)
			far := ramp(pos*0.45 + t*0.17 + 0.37)
			col = mix(near, far, (0.5+0.5*math.Sin(pos*2.3+t*0.7))*0.6)
			punch = 0.45
		} else {
			col = base
		}
		pulse := 0.5 + 0.5*math.Sin((pos-t*0.6)*2*math.Pi)
		r, g, b := float64(col.R), float64(col.G), float64(col.B)
		if pulse > 0.5 {
			k := (pulse - 0.5) * 2 * 0.55 * punch
			r, g, b = r+(255-r)*k, g+(255-g)*k, b+(255-b)*k
		} else {
			k := (0.5 - pulse) * 2 * 0.45 * punch
			r, g, b = r-r*k, g-g*k, b-b*k
		}
		if !e.aurora { // the hue drifts, a little, from one stop to the next
			r += math.Sin(t*0.7+pos*6) * 14
			g += math.Sin(t*0.7+pos*6+2.1) * 14
			b += math.Sin(t*0.7+pos*6+4.2) * 14
		}
		stops = append(stops, ui.GradientStop{Color: ui.RGBA(uint8(clampf(r, 0, 255)), uint8(clampf(g, 0, 255)), uint8(clampf(b, 0, 255)), 0.9), At: float32(1 - pos)})
	}
	glowAround(p, box, base, 0.9)
	p.FillPathGradient(rrect(box.X, box.Y, box.W, box.H, boxRadius(box.W, box.H)), ui.LinearGradient{Angle: 180, Oklab: true, Stops: stops})
	p.AnimationFrame()
}

func clampf(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ---- CRT: phosphor ghosts, glitch on jumps; neon ----

// crt leaves faded copies of the box where it was, and breaks it up on a jump. Neon makes the
// ghosts glowing tubes, bridged across a jump.
type crt struct {
	neon   bool
	ghosts []crtGhost
	prev   ui.Rect
	placed bool
	glitch struct {
		on      bool
		at      time.Time
		dist    float64
		seed    uint32
		x, y, w float64
		h       float64
	}
}

type crtGhost struct {
	x, y, w, h float64
	at         time.Time
}

func newCRT(neon bool) *crt { return &crt{neon: neon} }

func (c *crt) reset() { c.ghosts, c.placed, c.glitch.on = nil, false, false }

func (c *crt) shift(dy float32) {
	for i := range c.ghosts {
		c.ghosts[i].y += float64(dy)
	}
	c.prev.Y += dy
}

const (
	crtFade    = 450 * time.Millisecond
	crtJump    = 120.0 // a move of this many px is a jump
	crtGhosts  = 10
	crtGlitchT = 220 * time.Millisecond
)

func glitchNoise(a, b, c uint32) float64 {
	n := a*374761393 + b*668265263 + c*2246822519
	n = (n ^ (n >> 13)) * 1274126177
	return float64((n^(n>>16))&0xffffffff) / 4294967296.0
}

func (c *crt) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	T := f.r
	w := float64(max(f.cw, 6))
	if !c.placed {
		c.prev, c.placed = T, true
	}
	if T != c.prev {
		dist := math.Hypot(float64(T.X-c.prev.X), float64(T.Y-c.prev.Y))
		if dist > 0.5 {
			c.ghosts = append(c.ghosts, crtGhost{float64(c.prev.X), float64(c.prev.Y), w, float64(c.prev.H), now})
			if c.neon && dist >= crtJump && k.erased == 0 { // the leap, bridged
				steps := int(math.Min(crtGhosts, dist/8))
				for i := 1; i <= steps; i++ {
					s := float64(i) / float64(steps+1)
					c.ghosts = append(c.ghosts, crtGhost{float64(c.prev.X) + (float64(T.X)-float64(c.prev.X))*s, float64(c.prev.Y) + (float64(T.Y)-float64(c.prev.Y))*s, w, float64(c.prev.H), now.Add(time.Duration(s * 10 * float64(time.Millisecond)))})
				}
			}
			if !c.neon && dist >= crtJump { // a jump: the box breaks up
				c.glitch.on, c.glitch.at, c.glitch.dist, c.glitch.seed = true, now, dist, uint32(now.UnixNano()>>10)
			}
		}
		c.prev = T
	}
	for len(c.ghosts) > crtGhosts*6 {
		c.ghosts = c.ghosts[1:]
	}
	if len(c.ghosts) > crtGhosts && !c.neon {
		c.ghosts = c.ghosts[len(c.ghosts)-crtGhosts:]
	}
	col := cursorColor(f.dark)
	out := c.ghosts[:0]
	for _, g := range c.ghosts {
		age := float64(now.Sub(g.at)) / float64(crtFade)
		if age >= 1 {
			continue
		}
		out = append(out, g)
		a := 1 - age
		r := ui.Rect{X: f32(g.x), Y: f32(g.y), W: f32(g.w), H: f32(g.h)}
		if c.neon {
			halo := withAlpha(col, math.Min(1, 2*a)*0.5)
			for _, k := range []float32{9, 5} {
				p.Fill(ui.Rect{X: r.X - k, Y: r.Y - k, W: r.W + 2*k, H: r.H + 2*k}, withAlpha(halo, 0.08*float64(halo.A)/255*4), rad(4)+k)
			}
			p.Fill(r, withAlpha(mix(col, ui.RGBA(255, 255, 255, 1), 0.35), math.Min(1, a*1.3)), boxRadius(r.W, r.H))
			if r.W >= 3 {
				fw := max(1, r.W*0.34)
				p.Fill(ui.Rect{X: r.X + (r.W-fw)/2, Y: r.Y, W: fw, H: r.H}, ui.RGBA(255, 255, 255, float32(math.Min(1, 1.6*a))), 1)
			}
		} else {
			p.Fill(r, withAlpha(col, a*0.9), boxRadius(r.W, r.H))
		}
	}
	c.ghosts = out
	if len(c.ghosts) > 0 || c.glitch.on {
		p.AnimationFrame()
	}
	box := ui.Rect{X: T.X, Y: T.Y, W: f32(w), H: T.H}
	if c.glitch.on {
		el := now.Sub(c.glitch.at)
		pr := float64(el) / float64(crtGlitchT)
		if pr >= 1 {
			c.glitch.on = false
		} else {
			c.drawGlitch(p, box, col, pr, el)
			return
		}
	}
	glowAround(p, box, col, 0.9)
	p.Fill(box, withAlpha(col, 0.9), boxRadius(box.W, box.H))
}

// drawGlitch is the box in slices that slip sideways, with red and blue fringes and a scanline.
func (c *crt) drawGlitch(p *ui.Painter, box ui.Rect, col ui.Color, pr float64, el time.Duration) {
	reach := 0.8
	if c.glitch.dist >= crtJump {
		reach = math.Min(2.2, 0.7+c.glitch.dist/420)
	}
	env := (1 - pr) * (1 - pr)
	bucket := uint32(el / (45 * time.Millisecond))
	amp := 14 * reach * env
	ab := 3.2 * env
	S := int(math.Min(12, math.Max(3, math.Round(float64(box.H)/3))))
	sh := float64(box.H) / float64(S)
	seed := c.glitch.seed
	for i := 0; i < S; i++ {
		n1 := glitchNoise(seed, uint32(i), bucket)
		n2 := glitchNoise(seed+101, uint32(i), bucket)
		n3 := glitchNoise(seed+977, uint32(i), bucket)
		if n3 < 0.13*env {
			continue
		}
		d := (n1 - 0.5) * 2
		dx := d * d * d * amp
		sw := math.Max(1, float64(box.W)*(1+(n2-0.5)*0.55*env))
		sx := float64(box.X) + dx - (sw-float64(box.W))/2
		sy := float64(box.Y) + float64(i)*sh
		for _, ch := range []struct {
			off float64
			c   ui.Color
		}{{-ab, ui.RGBA(col.R, 0, 0, 1)}, {0, ui.RGBA(0, col.G, 0, 1)}, {ab, ui.RGBA(0, 0, col.B, 1)}} {
			p.Fill(ui.Rect{X: f32(sx + ch.off), Y: f32(sy), W: f32(sw), H: f32(sh + 0.5)}, withAlpha(mix(ch.c, col, 0.45), 0.6), 0)
		}
	}
	if glitchNoise(seed+5501, 0, bucket) < 0.55 {
		ly := float64(box.Y) + glitchNoise(seed+31, 1, bucket)*float64(box.H)
		lw := float64(box.W) * (1.2 + glitchNoise(seed+77, 2, bucket)*1.6)
		p.Fill(ui.Rect{X: f32(float64(box.X) - (lw-float64(box.W))/2), Y: f32(ly), W: f32(lw), H: float32(math.Max(1, 0.06*float64(box.H)))}, ui.RGBA(255, 255, 255, float32(0.55*env)), 0)
	}
	p.AnimationFrame()
}

// ---- tiles ----

func smearMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	path := &ui.Path{}
	path.MoveTo(r.X+4, r.Y+10).LineTo(r.X+22, r.Y+3).LineTo(r.X+31, r.Y+3).LineTo(r.X+13, r.Y+19).LineTo(r.X+4, r.Y+19).Close()
	p.FillPath(path, withAlpha(col, 0.85))
}

func energyMock(p *ui.Painter, r ui.Rect, dark bool) {
	x, y := r.X+r.W/2-5, r.Y+1
	base := cursorColor(dark)
	p.FillPathGradient(rrect(x, y, 10, r.H-2, 3), ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: mix(base, ui.RGBA(255, 255, 255, 1), 0.55), At: 0}, {Color: base, At: 0.45}, {Color: mix(base, ui.RGBA(0, 0, 0, 1), 0.4), At: 1}}})
}

func auroraMock(p *ui.Painter, r ui.Rect, dark bool) {
	x, y := r.X+r.W/2-5, r.Y+1
	p.FillPathGradient(rrect(x, y, 10, r.H-2, 3), ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: ui.Hex("#c792ea"), At: 0}, {Color: ui.Hex("#4aa3ff"), At: 0.35}, {Color: ui.Hex("#31edae"), At: 0.7}, {Color: ui.Hex("#f5a623"), At: 1}}})
}

func crtMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	for i := 0; i < 5; i++ {
		p.Fill(ui.Rect{X: r.X + 2 + float32(i)*6, Y: r.Y + 2, W: 7, H: r.H - 4}, withAlpha(col, 0.14+0.16*float64(i)), 2)
	}
}

func neonMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	for i := 0; i < 4; i++ {
		x := r.X + 4 + float32(i)*7
		a := 0.25 + 0.2*float64(i)
		p.Fill(ui.Rect{X: x - 3, Y: r.Y, W: 13, H: r.H}, withAlpha(col, 0.07*a), 6)
		p.Fill(ui.Rect{X: x, Y: r.Y + 3, W: 7, H: r.H - 6}, withAlpha(mix(col, ui.RGBA(255, 255, 255, 1), 0.35), a), 2)
		p.Fill(ui.Rect{X: x + 2.4, Y: r.Y + 3, W: 2.4, H: r.H - 6}, ui.RGBA(255, 255, 255, float32(a)), 1)
	}
}
