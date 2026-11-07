package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Carets after Cursor-Smith (https://github.com/Sadsnake1/cursor-smith, MIT, see
// NOTICE.md), each fixed as it ships, with nothing to set: the phosphor ghosts of the CRT,
// drawn by this app's painter, and the jelly that can be put on any caret.

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

// ---- Jelly ----

// smear is the jelly: the box's two faces follow where the caret goes on springs, the leading one
// quick and the trailing one slow, so the box stretches toward where it came from and snaps back.
// body gives it, for whatever caret is chosen.
type smear struct {
	lx, ly, lvx, lvy float64 // the leading face
	tx, ty           float64 // the trailing face
	dx, dy           float64 // the way it moves, held when it stops
	pcx, pcy         float64
	placed           bool
	last             time.Time
}

func (s *smear) reset() { s.placed = false }

func (s *smear) shift(dy float32) {
	s.ly += float64(dy)
	s.ty += float64(dy)
	s.pcy += float64(dy)
}

// faces moves the springs to now, and gives the four corners of the stretched box, and
// whether it is at rest (then the box is the caret's own).
func (s *smear) faces(now time.Time, f caretFrame) (c [4][2]float64, settled bool) {
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
	offs := [4][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}}
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
		return c, true
	}
	// the tail thins toward the line it travels
	reach := clamp01((maxLag - 4) / 20)
	for i := range offs {
		offx, offy := c[i][0]-cx, c[i][1]-cy
		along := offx*s.dx + offy*s.dy
		px, py := offx-along*s.dx, offy-along*s.dy
		k := 0.0
		if maxLag > 0 {
			k = lag[i] / maxLag * reach * 0.7
		}
		c[i][0] -= px * k
		c[i][1] -= py * k
	}
	return c, false
}

// body is the caret's own body as the jelly makes it: the box stretched toward where it was, its
// corners rounded; nil when it is at rest, and the caret is the plain box. The carets draw
// their body with it (glassBody), so the jelly is the caret, not a layer under it.
func (s *smear) body(p *ui.Painter, f caretFrame) *ui.Path {
	c, settled := s.faces(p.Now(), f)
	if settled {
		return nil
	}
	p.AnimationFrame()
	r := float64(boxRadius(max(f.cw, 6), f.r.H))
	dist := func(a, b [2]float64) float64 { return math.Hypot(a[0]-b[0], a[1]-b[1]) }
	toward := func(a, b [2]float64, d float64) (float32, float32) { // d from a toward b
		l := dist(a, b)
		if l == 0 {
			return f32(a[0]), f32(a[1])
		}
		return f32(a[0] + (b[0]-a[0])*d/l), f32(a[1] + (b[1]-a[1])*d/l)
	}
	path := &ui.Path{}
	for i := 0; i < 4; i++ {
		prev, cur, next := c[(i+3)%4], c[i], c[(i+1)%4]
		d := math.Min(r, math.Min(dist(prev, cur), dist(cur, next))/2)
		x0, y0 := toward(cur, prev, d)
		x1, y1 := toward(cur, next, d)
		if i == 0 {
			path.MoveTo(x0, y0)
		} else {
			path.LineTo(x0, y0)
		}
		path.QuadTo(f32(cur[0]), f32(cur[1]), x1, y1)
	}
	return path.Close()
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

func clampf(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ---- CRT: phosphor ghosts, glitch on jumps and on Enter ----

// crt leaves faded copies of the box where it was, and breaks it up on a jump, and on Enter.
type crt struct {
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

func newCRT() *crt { return &crt{} }

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
			if dist >= crtJump { // a jump: the box breaks up
				c.glitch.on, c.glitch.at, c.glitch.dist, c.glitch.seed = true, now, dist, uint32(now.UnixNano()>>10)
			}
		}
		c.prev = T
	}
	switch { // Enter breaks the box up as a jump does; a space, a little less
	case strings.Contains(k.typed, "\n"):
		c.glitch.on, c.glitch.at, c.glitch.dist, c.glitch.seed = true, now, crtJump, uint32(now.UnixNano()>>10)
	case strings.Contains(k.typed, " "):
		c.glitch.on, c.glitch.at, c.glitch.dist, c.glitch.seed = true, now, crtJump*0.5, uint32(now.UnixNano()>>10)
	}
	for len(c.ghosts) > crtGhosts*6 {
		c.ghosts = c.ghosts[1:]
	}
	if len(c.ghosts) > crtGhosts {
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
		glassFill(p, rrect(r.X, r.Y, r.W, r.H, boxRadius(r.W, r.H)), col, col, f.dark, a*0.8) // a ghost of glass
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
	glassBody(p, f, box, col, 1, 0.9)
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
