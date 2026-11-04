package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// slime is a caret made of jelly, ported from Caret FX's Slime (Thymer plugin):
// a rectangle the width of the letter under it whose base stays put while its
// body is a chain of springs, so the top lags and rocks. At rest it sways and
// breathes; each step kicks it the way it went; a new line or a jump squashes
// it flat and draws it out from where it was, its foot arriving first.
type slime struct {
	m, vm, j, vj, t, vt float64 // the spine's three joints, sideways
	sx, vx, sy, vy      float64 // scale
	ax, ay, avx, avy    float64 // the foot's offset from the caret, on its journey
	bx, by, bvx, bvy    float64 // the top's offset, chasing the foot's
	last                time.Time
	prev                ui.Rect
	placed              bool
	flying              bool // on its way to a new line or place: it squashes when it lands
}

const slimeStep = 1.0 / 240 // springs are integrated in steps this small: every frame rate wobbles alike

func newSlime() *slime { return &slime{sx: 1, sy: 1} }

// moved kicks the jelly when the caret went from prev to cur.
func (s *slime) moved(prev, cur ui.Rect) {
	dx, dy := float64(cur.X-prev.X), float64(cur.Y-prev.Y)
	dist := math.Hypot(dx, dy)
	if dist < 0.2 {
		return
	}
	dir := 1.0
	if dx < 0 {
		dir = -1
	}
	h := float64(cur.H)
	if math.Abs(dy) > h*0.6 || dist > h*2.5 { // a new line or a jump: lands flat, then springs up
		s.flying = true // it stretches as it goes; the squash comes on landing (draw)
		s.vy += 2.5
		s.vx -= 1.5
		s.vm += dir * 110
		s.vj += dir * 130
		s.vt += dir * 160
		s.ax -= dx // it stays where it was and is drawn across to the caret
		s.ay -= dy
		s.bx -= dx
		s.by -= dy
		return
	}
	s.vm += dir * 45
	s.vj += dir * 60
	s.vt += dir * 80
	s.sx = math.Min(1.35, s.sx+0.16)
	s.sy = math.Max(0.7, s.sy-0.1)
}

// shift moves the jelly with the content (scrolling) without kicking it.
func (s *slime) shift(dx, dy float32) { s.prev.X += dx; s.prev.Y += dy }

func (s *slime) step(now time.Time, dt, idle float64, w float64) {
	ms := float64(now.UnixNano()) / 1e6
	drive := idle * w * (0.13*math.Sin(ms*0.0057) + 0.05*math.Sin(ms*0.0093+1)) // two slow waves out of step
	breathe := 1 + idle*0.03*math.Sin(ms*0.0057+1)
	const (
		wm, zm     = 20.0, 0.2
		wj, wt, zt = 24.0, 30.0, 0.16
		ws, zs     = 26.0, 0.2
		wf, wb, zf = 24.0, 21.0, 0.8
	)
	for left := math.Min(dt, 0.05); left > 1e-6; {
		h := math.Min(slimeStep, left)
		left -= h
		s.vm += (-wm*wm*(s.m-drive) - 2*zm*wm*s.vm) * h
		s.m += s.vm * h
		s.vj += (-wj*wj*(s.j-s.m) - 2*zt*wj*s.vj) * h
		s.j += s.vj * h
		s.vt += (-wt*wt*(s.t-s.j) - 2*zt*wt*s.vt) * h
		s.t += s.vt * h
		s.vx += (-ws*ws*(s.sx-1) - 2*zs*ws*s.vx) * h
		s.sx += s.vx * h
		s.vy += (-ws*ws*(s.sy-breathe) - 2*zs*ws*s.vy) * h
		s.sy += s.vy * h
		s.avx += (-wf*wf*s.ax - 2*zf*wf*s.avx) * h // the foot comes in first...
		s.ax += s.avx * h
		s.avy += (-wf*wf*s.ay - 2*zf*wf*s.avy) * h
		s.ay += s.avy * h
		s.bvx += (-wb*wb*(s.bx-s.ax) - 2*zf*wb*s.bvx) * h // ...and the top after it
		s.bx += s.bvx * h
		s.bvy += (-wb*wb*(s.by-s.ay) - 2*zf*wb*s.bvy) * h
		s.by += s.bvy * h
	}
}

// draw paints the jelly on the caret's text box r (absolute: the letter's
// ascent to descent, not the line's leading), cw wide, and asks for the next
// frame: it never rests.
func (s *slime) draw(p *ui.Painter, r ui.Rect, cw float32, col ui.Color, dark bool) {
	now := p.Now()
	if !s.placed {
		s.prev, s.last, s.placed = r, now, true
	} else if r != s.prev {
		s.moved(s.prev, r)
		s.prev = r
	}
	dt := now.Sub(s.last).Seconds()
	s.last = now
	W := math.Max(float64(cw), 7)
	H := float64(r.H)
	s.step(now, dt, 1, W)
	// Landing: the foot is home and coming in fast - it splats, then springs up.
	if s.flying && math.Hypot(s.ax, s.ay) < 0.25*H {
		s.flying = false
		s.vy -= 11
		s.vx += 8
	}
	defer p.AnimationFrame()

	j := s.jelly(float64(r.X)+W/2, float64(r.Y)+H, W, H)
	path := j.outline()
	if path == nil {
		return
	}
	// A soft glow round it (stacked strokes: paths have no blur), the jelly
	// itself lit from above, its rim, and a gloss down its upper left side.
	glow := 1.0
	if !dark {
		glow = 0.6
	}
	for i, w := range []float32{9, 6, 3.5} {
		p.StrokePath(path, w, withAlpha(col, glow*[]float64{0.035, 0.06, 0.09}[i]))
	}
	top := mix(col, ui.RGBA(255, 255, 255, 1), 0.45)
	p.FillPathGradient(path, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: withAlpha(top, 0.62), At: 0},
		{Color: withAlpha(col, 0.38), At: 0.55},
		{Color: withAlpha(col, 0.5), At: 1},
	}})
	p.StrokePath(path, 1.25, withAlpha(col, 0.95))
	if g := j.gloss(); g != nil {
		a := float32(0.55)
		if !dark {
			a = 0.8
		}
		p.StrokePath(g, 1.4, ui.RGBA(255, 255, 255, a))
	}
}

type pt struct{ x, y float64 }

// jelly is the slime as a distance field: a body standing on the caret, and on
// a journey a drop of it left behind and a strand between them. The three are
// melted together with a smooth minimum, so the neck between them is goo, not
// a seam; the outline is where the field crosses zero.
type jelly struct {
	fx, fy, hh, hw, bulge float64 // the body: its foot's middle, height, half width, squash bulge
	sway                  [4]float64
	rt, rb                float64 // corner radii, top and foot
	travel                bool
	a, b                  pt      // the strand, from the body's middle to the drop's
	ra, rbk, neck         float64 // its radius at each end, and how far its middle thins
	thw, thh              float64 // the drop's half size
	k                     float64 // how softly the parts melt together
}

func (s *slime) jelly(fx, fy, W, H float64) jelly {
	hh := H * s.sy
	ws := clamp(s.sx*math.Sqrt(1/math.Max(s.sy, 0.3)), 0.75, 1.6) // squashed jelly spreads
	lim := W * 1.1
	j := jelly{
		fx: fx + s.ax, fy: fy + s.ay, hh: hh, hw: W * ws / 2,
		bulge: clamp((1-s.sy)*W*0.7, -W*0.3, W*0.45),
		sway:  [4]float64{0, clamp(s.m, -lim, lim), clamp(s.j, -lim, lim), clamp(s.t, -lim, lim)},
		k:     math.Max(2.5, H*0.18),
	}
	j.rb = math.Min(2.5, j.hw*0.5)
	j.rt = math.Min(j.hw*0.85, hh*0.3)
	// The drop left behind lags the foot, but the jelly stretches only so far:
	// a drop pulled long, never a line drawn across the text.
	vx, vy := s.bx-s.ax, s.by-s.ay
	d := math.Hypot(vx, vy)
	if maxL := 1.7 * H; d > maxL {
		vx, vy, d = vx*maxL/d, vy*maxL/d, maxL
	}
	if d > 1 {
		tail := clamp(1/(1+d/(2.5*H)), 0.5, 1)
		j.travel = true
		j.a = pt{j.fx + catmull(j.sway, 0.5), j.fy - hh/2}
		j.b = pt{j.a.x + vx, j.a.y + vy}
		j.thw, j.thh = j.hw*tail, hh/2*tail
		j.ra = math.Min(j.hw, hh/2) * 0.95
		j.rbk = math.Min(j.thw, j.thh) * 0.9
		j.neck = clamp(d/(d+2*H), 0, 0.5)
	}
	return j
}

// at is the field at (x, y): negative inside the jelly.
func (j jelly) at(x, y float64) float64 {
	u := clamp((j.fy-y)/j.hh, 0, 1)
	cx := j.fx + catmull(j.sway, u)
	hw := j.hw + j.bulge*math.Sin(math.Pi*u)
	rc := j.rb
	if y < j.fy-j.hh/2 {
		rc = j.rt
	}
	d := roundBox(x-cx, y-(j.fy-j.hh/2), hw, j.hh/2, rc)
	if !j.travel {
		return d
	}
	drop := roundBox(x-j.b.x, y-j.b.y, j.thw, j.thh, math.Min(j.thw, j.thh)*0.7)
	ex, ey := j.b.x-j.a.x, j.b.y-j.a.y
	t := clamp(((x-j.a.x)*ex+(y-j.a.y)*ey)/(ex*ex+ey*ey), 0, 1)
	rr := (j.ra + (j.rbk-j.ra)*t) * (1 - j.neck*math.Sin(math.Pi*t))
	strand := math.Hypot(x-j.a.x-ex*t, y-j.a.y-ey*t) - math.Max(rr, 0.8)
	return smin(d, smin(strand, drop, j.k), j.k)
}

// roundBox is the signed distance to a box of half size (hx, hy) with corners
// rounded by rc, from a point (x, y) relative to its middle.
func roundBox(x, y, hx, hy, rc float64) float64 {
	rc = math.Min(rc, math.Min(hx, hy))
	qx, qy := math.Abs(x)-hx+rc, math.Abs(y)-hy+rc
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rc
}

// smin is a smooth minimum: where two shapes come within k of each other they
// flow together instead of meeting at a crease.
func smin(a, b, k float64) float64 {
	h := clamp(0.5+0.5*(b-a)/k, 0, 1)
	return b + (a-b)*h - k*h*(1-h)
}

// outline traces the field's zero line (marching squares on a 1px grid over
// the jelly's bounds) and smooths it into a path; nil when there is nothing.
func (j jelly) outline() *ui.Path {
	const st = 1.0
	m := j.hw + math.Abs(j.bulge) + 4
	x0, x1 := j.fx-m-math.Abs(j.sway[3])-2, j.fx+m+math.Abs(j.sway[3])+2
	y0, y1 := j.fy-j.hh-4, j.fy+4
	if j.travel {
		x0, x1 = math.Min(x0, j.b.x-j.thw-4), math.Max(x1, j.b.x+j.thw+4)
		y0, y1 = math.Min(y0, j.b.y-j.thh-4), math.Max(y1, j.b.y+j.thh+4)
	}
	nx, ny := int((x1-x0)/st)+1, int((y1-y0)/st)+1
	if nx < 2 || ny < 2 || nx*ny > 200000 {
		return nil
	}
	w := nx + 1
	g := make([]float64, w*(ny+1))
	for yi := 0; yi <= ny; yi++ {
		for xi := 0; xi <= nx; xi++ {
			g[yi*w+xi] = j.at(x0+float64(xi)*st, y0+float64(yi)*st)
		}
	}
	// Edge keys: 2*node for the edge going right from a node, 2*node+1 going down.
	point := func(key int) pt {
		n, down := key/2, key%2 == 1
		xi, yi := n%w, n/w
		a := g[n]
		var b float64
		if down {
			b = g[n+w]
		} else {
			b = g[n+1]
		}
		t := a / (a - b)
		if down {
			return pt{x0 + float64(xi)*st, y0 + (float64(yi)+t)*st}
		}
		return pt{x0 + (float64(xi)+t)*st, y0 + float64(yi)*st}
	}
	adj := map[int][]int{}
	link := func(a, b int) { adj[a] = append(adj[a], b); adj[b] = append(adj[b], a) }
	for yi := 0; yi < ny; yi++ {
		for xi := 0; xi < nx; xi++ {
			n := yi*w + xi
			c := 0
			for bit, v := range [4]float64{g[n], g[n+1], g[n+w+1], g[n+w]} {
				if v < 0 {
					c |= 1 << bit
				}
			}
			top, bottom, left, right := 2*n, 2*(n+w), 2*n+1, 2*(n+1)+1
			mid := (g[n]+g[n+1]+g[n+w]+g[n+w+1])/4 < 0
			switch c {
			case 1, 14:
				link(left, top)
			case 2, 13:
				link(top, right)
			case 3, 12:
				link(left, right)
			case 4, 11:
				link(right, bottom)
			case 6, 9:
				link(top, bottom)
			case 7, 8:
				link(left, bottom)
			case 5:
				if mid {
					link(top, right)
					link(bottom, left)
				} else {
					link(left, top)
					link(right, bottom)
				}
			case 10:
				if mid {
					link(left, top)
					link(right, bottom)
				} else {
					link(top, right)
					link(bottom, left)
				}
			}
		}
	}
	path := &ui.Path{}
	seen := map[int]bool{}
	any := false
	for start := range adj {
		if seen[start] {
			continue
		}
		var loop []pt
		prev, cur := -1, start
		for !seen[cur] {
			seen[cur] = true
			loop = append(loop, point(cur))
			next := -1
			for _, q := range adj[cur] {
				if q != prev && !seen[q] {
					next = q
					break
				}
			}
			if next < 0 {
				break
			}
			prev, cur = cur, next
		}
		if len(loop) < 4 {
			continue
		}
		// Through the midpoints, each grid point a control: no corner of the grid shows.
		n := len(loop)
		midpt := func(a, b pt) pt { return pt{(a.x + b.x) / 2, (a.y + b.y) / 2} }
		m0 := midpt(loop[n-1], loop[0])
		path.MoveTo(f(m0.x), f(m0.y))
		for i := 0; i < n; i++ {
			q, e := loop[i], midpt(loop[i], loop[(i+1)%n])
			path.QuadTo(f(q.x), f(q.y), f(e.x), f(e.y))
		}
		path.Close()
		any = true
	}
	if !any {
		return nil
	}
	return path
}

// gloss is a short highlight just inside the body's upper left side, where
// light from above catches its curve; none on a body too thin to hold one.
func (j jelly) gloss() *ui.Path {
	if j.hw < 3 || j.hh < 8 {
		return nil
	}
	path := (&ui.Path{}).Cap(ui.RoundCap)
	for i := 0; i <= 8; i++ {
		u := 0.42 + 0.42*float64(i)/8
		hw := j.hw + j.bulge*math.Sin(math.Pi*u)
		x, y := j.fx+catmull(j.sway, u)-hw*0.5, j.fy-j.hh*u
		if i == 0 {
			path.MoveTo(f(x), f(y))
		} else {
			path.LineTo(f(x), f(y))
		}
	}
	return path
}

// catmull reads the sway of the spine's joints (at 0, 1/3, 2/3 and 1 of its
// height) at u, through a smooth curve rather than a bent line.
func catmull(v [4]float64, u float64) float64 {
	x := u * 3
	i := int(math.Min(x, 2.999))
	t := x - float64(i)
	at := func(k int) float64 { return v[max(0, min(3, k))] }
	p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
	return 0.5 * (2*p1 + (-p0+p2)*t + (2*p0-5*p1+4*p2-p3)*t*t + (-p0+3*p1-3*p2+p3)*t*t*t)
}

func f(v float64) float32 { return float32(v) }

func clamp(v, lo, hi float64) float64 { return math.Min(hi, math.Max(lo, v)) }

func withAlpha(c ui.Color, a float64) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(a*float64(c.A)/255))
}

func mix(a, b ui.Color, t float64) ui.Color {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return ui.RGBA(l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 1)
}
