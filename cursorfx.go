package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Effects of the mouse pointer, as the carets are effects of the text's caret (#287): chosen in the settings,
// off by default. Each goes with the system's pointer, which stays as it is, and is drawn over the window in
// an overlay the pointer passes through, so a click lands where it would without it.
//
//	Follower      a second pointer that comes after the real one on a spring
//	Trail         a line of the last places, fading
//	Ripple        a ring where the button went down
//	Paper plane   follows a little behind, turned the way it flies (cursorfx_vehicles.go)
//	Spider        walks to the pointer on its eight legs, its silk behind it
//	Rocket        homes in on the pointer; a click locks it on and it strikes

// cursorFX is an effect of the pointer: told where the pointer is each frame, it moves and draws itself.
type cursorFX interface {
	step(m pointerAt, dt float64) bool // moves it on dt seconds; true while it still moves and wants frames
	draw(p *ui.Painter, m pointerAt)
}

// pointerAt is the pointer of a frame, in the window's DIPs.
type pointerAt struct {
	x, y  float64
	in    bool // over the window
	press bool // the button went down since the last frame
	col   ui.Color
	dark  bool
}

// cursorPreset is an effect of the pointer: its name, and how to make one.
type cursorPreset struct {
	name string
	make func() cursorFX
}

// cursorPresets are the effects of the pointer, in the order the settings list them.
var cursorPresets []cursorPreset

func init() {
	cursorPresets = []cursorPreset{
		{"Follower", func() cursorFX { return &follower{} }},
		{"Trail", func() cursorFX { return &trail{} }},
		{"Ripple", func() cursorFX { return &ripples{} }},
		{"Paper plane", func() cursorFX { return &paperPlane{} }},
		{"Spider", func() cursorFX { return &spider{} }},
		{"Rocket", func() cursorFX { return &rocket{} }},
	}
}

// cursorFXNamed makes the effect called name, or nil: "" is none.
func cursorFXNamed(name string) cursorFX {
	for _, c := range cursorPresets {
		if c.name == name {
			return c.make()
		}
	}
	return nil
}

// cursorState is the effect of the pointer the settings chose, and what the frames told it.
type cursorState struct {
	name   string
	fx     cursorFX
	at     pointerAt
	press  bool      // a press seen by a build, for the next draw
	last   time.Time // the last draw
	moving bool      // the effect still moved at the last draw, and asked for another
}

// cursorFXView puts the chosen effect of the pointer over the window. The root tells where the pointer is
// and whether it went down there, without taking it; on a phone, with no pointer, there is nothing to draw.
func (a *app) cursorFXView(c *ui.Context, root ui.Element) {
	s := &a.cursor
	if a.settings.CursorFX != s.name {
		s.name, s.fx, s.last = a.settings.CursorFX, cursorFXNamed(a.settings.CursorFX), time.Time{}
	}
	if s.fx == nil || phone {
		return
	}
	x, y, over := root.PointerPosition()
	s.at.x, s.at.y, s.at.in = float64(x), float64(y), over
	if root.PressedInside() {
		s.press = true
	}
	w, h := a.winW, a.winH
	if w <= 0 {
		w, h = c.Size()
	}
	ui.Overlay(c, func() {
		ui.Box(c.Key("cursor-fx")).Absolute().Left(0).Top(0).Width(w).Height(h).PassThrough().Role(ui.RoleNone).
			Draw(func(p *ui.Painter, r ui.Rect) {
				now := p.Now()
				dt := dtOf(now, s.last)
				s.last = now
				m := s.at
				m.press, s.press = s.press, false
				m.col, m.dark = p.Theme().Accent, p.Theme().Dark
				if s.moving = s.fx.step(m, dt); s.moving {
					p.AnimationFrame()
				}
				s.fx.draw(p, m)
			})
	})
}

// ---- what they share ----

// spring2 is a point on a spring to a target: stiff k, damped by d.
type spring2 struct {
	x, y, vx, vy float64
	set          bool
}

// to moves it dt toward (tx, ty), and reports whether it still moves.
func (s *spring2) to(tx, ty, k, d, dt float64) bool {
	if !s.set {
		s.x, s.y, s.set = tx, ty, true
		return false
	}
	ax := k*(tx-s.x) - d*s.vx
	ay := k*(ty-s.y) - d*s.vy
	s.vx += ax * dt
	s.vy += ay * dt
	s.x += s.vx * dt
	s.y += s.vy * dt
	if math.Hypot(tx-s.x, ty-s.y) < 0.3 && math.Hypot(s.vx, s.vy) < 3 {
		s.x, s.y, s.vx, s.vy = tx, ty, 0, 0
		return false
	}
	return true
}

// turnTo turns angle a toward b (radians) by the shortest way, at rate per second.
func turnTo(a, b, rate, dt float64) float64 {
	d := math.Remainder(b-a, 2*math.Pi)
	step := rate * dt
	if math.Abs(d) <= step {
		return b
	}
	return a + math.Copysign(step, d)
}

// turned is (x, y) turned by ang about the origin and moved to (cx, cy).
func turned(cx, cy, x, y, ang float64) (float32, float32) {
	s, c := math.Sincos(ang)
	return f32(cx + x*c - y*s), f32(cy + x*s + y*c)
}

// shape is a closed path through pts (x, y pairs about the origin), turned by ang and moved to (cx, cy).
func shape(cx, cy, ang float64, pts ...float64) *ui.Path {
	path := &ui.Path{}
	for i := 0; i+1 < len(pts); i += 2 {
		x, y := turned(cx, cy, pts[i], pts[i+1], ang)
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return path.Close()
}

// slant is a straight line from (x0, y0) to (x1, y1) at any slant, which Painter.Line is not: it draws only the
// level and the upright.
func slant(p *ui.Painter, x0, y0, x1, y1, width float32, col ui.Color) {
	p.StrokePolyline([][2]float32{{x0, y0}, {x1, y1}}, width, col)
}

// ink is the color drawn on the page in the dark or the light: near white, or near black.
func ink(dark bool) ui.Color {
	if dark {
		return ui.Hex("#f4f4f5")
	}
	return ui.Hex("#18181b")
}

// ---- Follower ----

// follower is a second pointer, in the accent, that comes after the real one on a soft spring: it overshoots
// a little as it stops.
type follower struct{ s spring2 }

func (f *follower) step(m pointerAt, dt float64) bool {
	return f.s.to(m.x+10, m.y+12, 90, 11, dt)
}

func (f *follower) draw(p *ui.Painter, m pointerAt) {
	if !m.in || !f.s.set {
		return
	}
	// the arrow of a pointer, its tip at the point
	arrow := shape(f.s.x, f.s.y, 0, 0, 0, 0, 17, 4.5, 13, 7.5, 19.5, 10.5, 18.2, 7.6, 11.8, 13, 11.8)
	p.FillPath(shape(f.s.x+1, f.s.y+2, 0, 0, 0, 0, 17, 4.5, 13, 7.5, 19.5, 10.5, 18.2, 7.6, 11.8, 13, 11.8), ui.RGBA(0, 0, 0, 0.18))
	p.FillPath(arrow, withAlpha(m.col, 0.85))
	p.StrokePath(arrow, 1.2, ui.RGBA(255, 255, 255, 0.9))
}

// ---- Trail ----

// trail is the pointer's last places, a line that thins and fades behind it.
type trail struct {
	pts    []trailPt
	now    float64
	lx, ly float64 // where the pointer was last
}

type trailPt struct{ x, y, t float64 }

// trailLife is how long a place of the trail lasts, in seconds.
const trailLife = 0.32

func (tr *trail) step(m pointerAt, dt float64) bool {
	tr.now += dt
	if m.in && math.Hypot(m.x-tr.lx, m.y-tr.ly) > 0.5 {
		tr.pts = append(tr.pts, trailPt{m.x, m.y, tr.now})
		tr.lx, tr.ly = m.x, m.y
	}
	i := 0
	for i < len(tr.pts) && tr.now-tr.pts[i].t > trailLife {
		i++
	}
	tr.pts = tr.pts[i:]
	return len(tr.pts) > 0 // until the last place is gone, or it would draw a line from there at the next move
}

func (tr *trail) draw(p *ui.Painter, m pointerAt) {
	for i := 1; i < len(tr.pts); i++ {
		a, b := tr.pts[i-1], tr.pts[i]
		left := 1 - (tr.now-b.t)/trailLife
		if left <= 0 {
			continue
		}
		slant(p, f32(a.x), f32(a.y), f32(b.x), f32(b.y), f32(1+5*left), withAlpha(m.col, 0.7*left))
	}
}

// ---- Ripple ----

// ripples are rings where the button went down: each widens and fades, with a few sparks flung out.
type ripples struct{ rings []ripple }

type ripple struct{ x, y, age float64 }

// rippleLife is how long a ring lasts, in seconds.
const rippleLife = 0.55

func (r *ripples) step(m pointerAt, dt float64) bool {
	if m.press {
		r.rings = append(r.rings, ripple{x: m.x, y: m.y})
	}
	out := r.rings[:0]
	for _, g := range r.rings {
		if g.age += dt; g.age < rippleLife {
			out = append(out, g)
		}
	}
	r.rings = out
	return len(r.rings) > 0
}

func (r *ripples) draw(p *ui.Painter, m pointerAt) {
	for _, g := range r.rings {
		t := g.age / rippleLife
		ease := 1 - math.Pow(1-t, 3)
		rad := 4 + 26*ease
		p.StrokePath((&ui.Path{}).Circle(f32(g.x), f32(g.y), f32(rad)), f32(2.5*(1-t)+0.5), withAlpha(m.col, 0.8*(1-t)))
		for i := 0; i < 8; i++ { // and the sparks, between the ring and a little past it
			x0, y0 := turned(g.x, g.y, rad*0.55, 0, float64(i)*math.Pi/4)
			x1, y1 := turned(g.x, g.y, rad*0.85, 0, float64(i)*math.Pi/4)
			slant(p, x0, y0, x1, y1, 1.6, withAlpha(m.col, 0.9*(1-t)))
		}
	}
}
