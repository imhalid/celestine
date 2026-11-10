package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The effects of the pointer that go after it on their own (#287): a spider and a rocket. Each has
// a place, a speed and a heading of its own, and turns toward where it goes by the shortest way.

// ---- Spider ----

// spider walks to the pointer and waits under it, on eight legs that step in turn as it goes; it lays a thread
// of silk behind it, pinned every so often, the last pins only.
type spider struct {
	x, y, ang float64
	gait      float64 // how far it walked, which moves the legs
	pins      [][2]float64
	set       bool
}

// spiderPins is how many pins of its silk are kept.
const spiderPins = 7

func (sp *spider) step(m pointerAt, dt float64) bool {
	tx, ty := m.x+6, m.y+30
	if !sp.set {
		sp.x, sp.y, sp.ang, sp.set = tx, ty+60, -math.Pi/2, true
		sp.pins = append(sp.pins, [2]float64{sp.x, sp.y})
	}
	dx, dy := tx-sp.x, ty-sp.y
	d := math.Hypot(dx, dy)
	if d < 1.5 {
		return false
	}
	sp.ang = turnTo(sp.ang, math.Atan2(dy, dx), 9, dt)
	speed := math.Min(420, d*5) // it slows as it arrives
	// it walks the way it faces: a turn first, then on
	facing := math.Max(0, math.Cos(math.Remainder(math.Atan2(dy, dx)-sp.ang, 2*math.Pi)))
	step := math.Min(d, speed*dt*facing)
	sp.x += math.Cos(sp.ang) * step
	sp.y += math.Sin(sp.ang) * step
	sp.gait += step + 2*dt // turning on the spot, it still shuffles
	if last := sp.pins[len(sp.pins)-1]; math.Hypot(sp.x-last[0], sp.y-last[1]) > 80 {
		sp.pins = append(sp.pins, [2]float64{sp.x, sp.y})
		if len(sp.pins) > spiderPins {
			sp.pins = sp.pins[1:]
		}
	}
	return true
}

func (sp *spider) draw(p *ui.Painter, m pointerAt) {
	if !sp.set {
		return
	}
	body := ink(m.dark)
	// the silk: pin to pin, and to the spider, the old end fainter
	for i := range sp.pins {
		a := sp.pins[i]
		bx, by := sp.x, sp.y
		if i+1 < len(sp.pins) {
			bx, by = sp.pins[i+1][0], sp.pins[i+1][1]
		}
		slant(p, f32(a[0]), f32(a[1]), f32(bx), f32(by), 0.8, withAlpha(body, 0.1+0.25*float64(i+1)/float64(len(sp.pins))))
		p.Fill(ui.Rect{X: f32(a[0] - 1.2), Y: f32(a[1] - 1.2), W: 2.4, H: 2.4}, withAlpha(body, 0.3), 1.2)
	}
	// the legs: four a side, each a thigh out from the body and a shin bent on from the knee, the front ones
	// forward and the back ones back; a side's legs swing in turn, the two sides opposite
	for side := -1.0; side <= 1; side += 2 {
		for i, out := range []float64{0.45, 1.05, 1.75, 2.35} { // a leg's angle from straight ahead
			swing := 0.3 * math.Sin(sp.gait*0.16+float64(i)*math.Pi/2+(side+1)*math.Pi/2)
			bend := -0.5
			if i >= 2 {
				bend = 0.5
			}
			hipX, hipY := 1.5-float64(i)*1.2, side*2.2
			thigh, shin := side*(out+swing), side*(out+swing+bend)
			kx, ky := hipX+7*math.Cos(thigh), hipY+7*math.Sin(thigh)
			fx, fy := kx+9*math.Cos(shin), ky+9*math.Sin(shin)
			x0, y0 := turned(sp.x, sp.y, hipX, hipY, sp.ang)
			x1, y1 := turned(sp.x, sp.y, kx, ky, sp.ang)
			x2, y2 := turned(sp.x, sp.y, fx, fy, sp.ang)
			slant(p, x0, y0, x1, y1, 1.4, body)
			slant(p, x1, y1, x2, y2, 1.1, body)
		}
	}
	ax, ay := turned(sp.x, sp.y, -7, 0, sp.ang)
	p.FillPath((&ui.Path{}).Circle(ax, ay, 6.5), body)
	p.FillPath((&ui.Path{}).Circle(f32(sp.x), f32(sp.y), 4.2), body)
	for _, s := range []float64{-1, 1} { // its eyes, in the accent
		ex, ey := turned(sp.x, sp.y, 2.6, s*1.6, sp.ang)
		p.FillPath((&ui.Path{}).Circle(ex, ey, 1.1), m.col)
	}
}

// ---- Rocket ----

// rocket homes in on the pointer: it steers its speed toward it, as a guided missile does, so it curves into a
// pointer that moves, and comes to rest beside it. Close, a reticle closes on the pointer; a click locks it on,
// and the rocket strikes the pointer at full thrust, bursts, and comes back from below.
type rocket struct {
	x, y, vx, vy, ang float64
	set, locked       bool
	lock              float64 // seconds since the lock
	puffs             []puff
	flash             float64 // what is left of a burst's flash, 1 to 0
	debris            sparks
	flicker           float64
}

// puff is a puff of the rocket's smoke: where it was left, and how long ago.
type puff struct{ x, y, age float64 }

func (r *rocket) step(m pointerAt, dt float64) bool {
	if !r.set {
		r.x, r.y, r.ang, r.set = m.x-220, m.y+220, -math.Pi/4, true
	}
	if m.press && m.in {
		r.locked, r.lock = true, 0
	}
	tx, ty := m.x+20, m.y+22
	cruise, force := 280.0, 900.0
	if r.locked {
		tx, ty = m.x, m.y
		cruise, force = 760, 3000
		r.lock += dt
	}
	dx, dy := tx-r.x, ty-r.y
	d := math.Hypot(dx, dy)
	if r.locked && d < 9 { // a hit: it bursts, and a new one comes from below
		r.debris.burst(m.x, m.y, 26, m.col, 260)
		r.debris.burst(m.x, m.y, 12, ui.Hex("#ff9f1c"), 200)
		r.flash, r.locked = 1, false
		r.x, r.y, r.vx, r.vy, r.ang = m.x-240, m.y+260, 0, 0, -math.Pi/4
		dx, dy, d = tx-r.x, ty-r.y, math.Hypot(tx-r.x, ty-r.y)
	}
	want := cruise
	if !r.locked && d < 90 {
		want = cruise * d / 90 // it slows as it arrives
	}
	var wx, wy float64
	if d > 0 {
		wx, wy = dx/d*want, dy/d*want
	}
	sx, sy := wx-r.vx, wy-r.vy
	if s := math.Hypot(sx, sy); s > force*dt { // it can turn only so hard
		sx, sy = sx/s*force*dt, sy/s*force*dt
	}
	r.vx += sx
	r.vy += sy
	r.x += r.vx * dt
	r.y += r.vy * dt
	speed := math.Hypot(r.vx, r.vy)
	if speed > 8 {
		r.ang = math.Atan2(r.vy, r.vx)
	}
	if !r.locked && d < 1 && speed < 6 {
		r.x, r.y, r.vx, r.vy, speed = tx, ty, 0, 0, 0
	}
	r.flicker += dt * 30
	if speed > 40 && (len(r.puffs) == 0 || math.Hypot(r.x-r.puffs[len(r.puffs)-1].x, r.y-r.puffs[len(r.puffs)-1].y) > 6) {
		bx, by := turned(r.x, r.y, -12, 0, r.ang)
		r.puffs = append(r.puffs, puff{x: float64(bx), y: float64(by)})
	}
	out := r.puffs[:0]
	for _, pf := range r.puffs {
		if pf.age += dt; pf.age < 0.45 {
			out = append(out, pf)
		}
	}
	r.puffs = out
	r.flash = math.Max(0, r.flash-dt*3)
	r.debris.step(dt)
	return speed > 0 || r.locked || len(r.puffs) > 0 || r.flash > 0 || len(r.debris.list) > 0
}

func (r *rocket) draw(p *ui.Painter, m pointerAt) {
	if !r.set {
		return
	}
	smoke := ui.Hex("#9ca3af")
	for _, pf := range r.puffs {
		t := pf.age / 0.45
		p.FillPath((&ui.Path{}).Circle(f32(pf.x), f32(pf.y), f32(2+5*t)), withAlpha(smoke, 0.35*(1-t)))
	}
	r.reticle(p, m)
	if r.flash > 0 {
		p.StrokePath((&ui.Path{}).Circle(f32(m.x), f32(m.y), f32(6+30*(1-r.flash))), f32(3*r.flash), withAlpha(ui.Hex("#ff9f1c"), r.flash))
	}
	r.debris.draw(p)
	x, y, a := r.x, r.y, r.ang
	if speed := math.Hypot(r.vx, r.vy); speed > 8 { // the flame, as long as the thrust
		long := 6 + math.Min(14, speed/50) + 2*math.Sin(r.flicker)
		p.FillPath(shape(x, y, a, -10, -3.2, -10-long, 0, -10, 3.2), ui.Hex("#ff9f1c"))
		p.FillPath(shape(x, y, a, -10, -1.6, -10-long*0.55, 0, -10, 1.6), ui.Hex("#fff3b0"))
	}
	hull := ui.Hex("#e5e7eb")
	if !m.dark {
		hull = ui.Hex("#f8fafc")
	}
	edge := withAlpha(ink(m.dark), 0.5)
	fins := shape(x, y, a, -10, -3.5, -13, -8, -5, -3.5, -5, 3.5, -13, 8, -10, 3.5)
	p.FillPath(fins, m.col)
	body := shape(x, y, a, -10, -3.5, 6, -3.5, 13, 0, 6, 3.5, -10, 3.5)
	p.FillPath(body, hull)
	p.StrokePath(body, 1, edge)
	p.FillPath(shape(x, y, a, 6, -3.5, 13, 0, 6, 3.5), ui.Hex("#ef4444")) // its nose
	wx, wy := turned(x, y, 1, 0, a)
	p.FillPath((&ui.Path{}).Circle(wx, wy, 1.6), m.col) // and its window
}

// reticle closes on the pointer as the rocket comes near: four corners, turning, that tighten with the
// distance; locked on, they are red and shut.
func (r *rocket) reticle(p *ui.Painter, m pointerAt) {
	if !m.in {
		return
	}
	d := math.Hypot(m.x-r.x, m.y-r.y)
	near := clamp01(1 - (d-30)/170) // 0 far, 1 close
	col := withAlpha(m.col, 0.75*near)
	size, spin := 26-10*near, near*math.Pi/4
	if r.locked {
		t := clamp01(r.lock / 0.18)
		size, spin, col = 26-14*t, math.Pi/4+t*math.Pi/4, ui.Hex("#ef4444")
	}
	if near <= 0 && !r.locked {
		return
	}
	for i := 0; i < 4; i++ {
		a := spin + float64(i)*math.Pi/2
		x0, y0 := turned(m.x, m.y, size, -6, a)
		xc, yc := turned(m.x, m.y, size, 0, a)
		x1, y1 := turned(m.x, m.y, size-6, 0, a)
		p.StrokePolyline([][2]float32{{x0, y0}, {xc, yc}, {x1, y1}}, 1.8, col)
	}
	if r.locked {
		p.Text(f32(m.x+size+4), f32(m.y-size-4), "LOCK", 9, ui.Hex("#ef4444"))
	}
}
