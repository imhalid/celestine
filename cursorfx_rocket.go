package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The rocket, the pointer's effect (#287): a place, a speed and a heading of its own, steered toward the
// pointer.

// ---- Rocket ----

// rocket homes in on the pointer: it steers its speed toward it, as a guided missile does, so it curves into a
// pointer that moves, and comes to rest beside it. Close, a reticle closes on the pointer; a click locks it on
// the place clicked, which it keeps however the pointer goes on, and the rocket strikes there at full thrust,
// bursts, and comes back from below.
type rocket struct {
	x, y, vx, vy, ang float64
	set, locked       bool
	lock              float64 // seconds since the lock
	lx, ly            float64 // where it is locked on: the place clicked
	bx, by            float64 // where it burst last
	puffs             []puff
	flash             float64 // what is left of a burst's flash, 1 to 0
	debris            sparks
	flicker, spin     float64 // the flame's flicker, and the reticle's turn
}

// puff is a puff of smoke: left behind the rocket, or thrown out by a burst, it drifts, swells and thins.
type puff struct{ x, y, vx, vy, age, life, size float64 }

func (r *rocket) step(m pointerAt, dt float64) bool {
	if !r.set {
		r.x, r.y, r.ang, r.set = m.x-220, m.y+220, -math.Pi/4, true
	}
	if m.press && m.in && !r.locked { // locked on, it is set on its place: another click does not move it
		r.locked, r.lock, r.lx, r.ly = true, 0, m.x, m.y
	}
	tx, ty := m.x+20, m.y+22
	cruise, force := 280.0, 900.0
	if r.locked {
		tx, ty = r.lx, r.ly
		cruise, force = 760, 3000
		r.lock += dt
	}
	dx, dy := tx-r.x, ty-r.y
	d := math.Hypot(dx, dy)
	if r.locked && d < 9 { // a hit: it bursts, and a new one comes from below
		r.burst(m.col)
		r.x, r.y, r.vx, r.vy, r.ang = r.lx-240, r.ly+260, 0, 0, -math.Pi/4
		tx, ty = m.x+20, m.y+22 // and the new one comes to the pointer
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
	r.spin += dt
	if speed > 40 && (len(r.puffs) == 0 || math.Hypot(r.x-r.puffs[len(r.puffs)-1].x, r.y-r.puffs[len(r.puffs)-1].y) > 5) {
		bx, by := turned(r.x, r.y, -15, (rnd()-0.5)*3, r.ang)
		back := 20 + rnd()*20 // the smoke is blown back, and spreads a little
		vx, vy := turned(0, 0, -back, (rnd()-0.5)*24, r.ang)
		r.puffs = append(r.puffs, puff{float64(bx), float64(by), float64(vx), float64(vy), 0, 0.5 + rnd()*0.3, 2.2 + rnd()*1.6})
	}
	out := r.puffs[:0]
	for _, pf := range r.puffs {
		if pf.age += dt; pf.age < pf.life {
			pf.x += pf.vx * dt
			pf.y += pf.vy * dt
			pf.vx *= 1 - math.Min(1, dt*3)
			pf.vy = pf.vy*(1-math.Min(1, dt*3)) - 8*dt // and rises, warm
			out = append(out, pf)
		}
	}
	r.puffs = out
	r.flash = math.Max(0, r.flash-dt*2.2)
	r.debris.step(dt)
	return speed > 0 || r.locked || len(r.puffs) > 0 || r.flash > 0 || len(r.debris.list) > 0
}

// burst is the rocket's end where it was locked on: a fireball, sparks of the accent and of fire, and smoke.
func (r *rocket) burst(col ui.Color) {
	r.bx, r.by, r.flash, r.locked = r.lx, r.ly, 1, false
	r.debris.burst(r.bx, r.by, 22, col, 260)
	r.debris.burst(r.bx, r.by, 16, ui.Hex("#ff9f1c"), 230)
	r.debris.burst(r.bx, r.by, 8, ui.Hex("#fff3b0"), 170)
	for i := 0; i < 10; i++ {
		a := float64(i)*math.Pi/5 + rnd()*0.4
		sp := 40 + rnd()*50
		r.puffs = append(r.puffs, puff{r.bx, r.by, math.Cos(a) * sp, math.Sin(a) * sp, 0, 0.7 + rnd()*0.4, 4 + rnd()*3})
	}
}

func (r *rocket) draw(p *ui.Painter, m pointerAt) {
	if !r.set {
		return
	}
	smoke := ui.Hex("#94a3b8")
	if m.dark {
		smoke = ui.Hex("#cbd5e1")
	}
	for _, pf := range r.puffs {
		t := pf.age / pf.life
		p.FillPath((&ui.Path{}).Circle(f32(pf.x), f32(pf.y), f32(pf.size*(1+1.6*t))), withAlpha(smoke, 0.3*(1-t)*(1-t)))
	}
	r.reticle(p, m)
	r.fireball(p)
	r.debris.draw(p)
	speed := math.Hypot(r.vx, r.vy)
	x, y := f32(r.x), f32(r.y)
	at := func(lx, ly float32) (float32, float32) { return x + lx, y + ly } // a point of the rocket, drawn turned about its middle
	p.Transform(ui.Rotation(f32(r.ang*180/math.Pi)), x, y, func() {
		if speed > 8 {
			r.flame(p, x, y, speed)
		}
		body := &ui.Path{} // a hull and an ogive nose, its tail a little round
		body.MoveTo(at(-11, -4.2))
		body.LineTo(at(3, -4.2))
		body.QuadTo(x+11, y-3.8, x+16, y)
		body.QuadTo(x+11, y+3.8, x+3, y+4.2)
		body.LineTo(at(-11, 4.2))
		body.QuadTo(x-12.6, y, x-11, y-4.2)
		body.Close()
		edge := withAlpha(ink(m.dark), 0.45)
		// the shadow under it, then the fins: swept back, in the accent, darker at their edge
		sx, sy := turned(0, 0, 1.5, 2.5, -r.ang) // down and to the right on the window, whichever way it faces
		p.Transform(ui.Translation(sx, sy), 0, 0, func() { p.FillPath(body, ui.RGBA(0, 0, 0, 0.14)) })
		for _, s := range []float32{-1, 1} {
			fin := &ui.Path{}
			fin.MoveTo(at(-4, 3.5*s))
			fin.QuadTo(x-8, y+9*s, x-14, y+10.5*s)
			fin.LineTo(at(-11.5, 3.5*s))
			fin.Close()
			p.FillPathGradient(fin, ui.LinearGradient{Angle: 90, Oklab: true, Stops: []ui.GradientStop{{Color: m.col, At: 0}, {Color: mix(m.col, ui.Hex("#000000"), 0.35), At: 1}}})
			p.StrokePath(fin, 0.8, edge)
		}
		// the hull, lit from one side; a band of the accent, the nose in red, a gleam along it
		p.FillPathGradient(body, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
			{Color: ui.Hex("#ffffff"), At: 0.1}, {Color: ui.Hex("#e2e8f0"), At: 0.5}, {Color: ui.Hex("#94a3b8"), At: 1}}})
		p.ClipPath(body, func() {
			p.Fill(ui.Rect{X: x - 8, Y: y - 5, W: 2.4, H: 10}, m.col, 0)
			p.FillPathGradient(rrect(x+7, y-5, 10, 10, 0), ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
				{Color: ui.Hex("#fb7185"), At: 0}, {Color: ui.Hex("#dc2626"), At: 0.55}, {Color: ui.Hex("#991b1b"), At: 1}}})
			p.Fill(ui.Rect{X: x - 10, Y: y - 3.1, W: 15, H: 1}, ui.RGBA(255, 255, 255, 0.7), 0.5)
		})
		p.StrokePath(body, 0.9, edge)
		// its window: a rim, the glass, a glint
		p.FillPath((&ui.Path{}).Circle(x+1.5, y, 2.6), ui.Hex("#cbd5e1"))
		p.FillPathGradient((&ui.Path{}).Circle(x+1.5, y, 1.8), ui.LinearGradient{Angle: 135, Oklab: true, Stops: []ui.GradientStop{
			{Color: mix(m.col, ui.Hex("#ffffff"), 0.55), At: 0}, {Color: mix(m.col, ui.Hex("#000000"), 0.4), At: 1}}})
		p.FillPath((&ui.Path{}).Circle(x+0.9, y-0.7, 0.55), ui.RGBA(255, 255, 255, 0.9))
	})
}

// flame is the rocket's fire out of its tail, drawn in its own turned frame: a glow, then three tongues, each
// shorter and hotter, that flicker and grow with the speed.
func (r *rocket) flame(p *ui.Painter, x, y float32, speed float64) {
	long := f32(7 + math.Min(16, speed/45) + 2.2*math.Sin(r.flicker) + 1.3*math.Sin(r.flicker*2.7+1))
	tail := x - 11
	p.FillRadialGradient(ui.Rect{X: tail - long - 6, Y: y - 10, W: long + 12, H: 20}, ui.RadialGradient{Center: ui.Point{X: 0.75, Y: 0.5}, Radius: long, Stops: []ui.GradientStop{
		{Color: ui.RGBA(255, 159, 28, 0.35), At: 0}, {Color: ui.RGBA(255, 159, 28, 0), At: 1}}}, 0)
	tongue := func(l, w float32, col ui.Color) {
		t := &ui.Path{}
		t.MoveTo(tail, y-w)
		t.QuadTo(tail-l*0.45, y-w*1.15, tail-l, y)
		t.QuadTo(tail-l*0.45, y+w*1.15, tail, y+w)
		t.Close()
		p.FillPath(t, col)
	}
	tongue(long, 3.4, ui.RGBA(239, 68, 68, 0.85))
	tongue(long*0.75, 2.5, ui.Hex("#ff9f1c"))
	tongue(long*0.42, 1.5, ui.Hex("#fff7d6"))
}

// fireball is the burst's light where the rocket struck: a ball of fire that swells and fades, and the ring of
// its blast going out ahead of it.
func (r *rocket) fireball(p *ui.Painter) {
	if r.flash <= 0 {
		return
	}
	t := 1 - r.flash // 0 at the hit, 1 gone
	x, y := f32(r.bx), f32(r.by)
	rad := f32(8 + 22*math.Sqrt(t))
	p.FillRadialGradient(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, ui.RadialGradient{Radius: rad, Stops: []ui.GradientStop{
		{Color: withAlpha(ui.Hex("#fffbe6"), r.flash), At: 0},
		{Color: withAlpha(ui.Hex("#ffb347"), 0.85*r.flash), At: 0.35},
		{Color: withAlpha(ui.Hex("#ef4444"), 0.5*r.flash), At: 0.7},
		{Color: ui.RGBA(239, 68, 68, 0), At: 1}}}, rad)
	ring := f32(10 + 46*(1-math.Pow(1-t, 3)))
	p.StrokePath((&ui.Path{}).Circle(x, y, ring), f32(2.5*r.flash+0.3), withAlpha(ui.Hex("#ffd27a"), 0.8*r.flash))
}

// reticle closes on the pointer as the rocket comes near: four corners, turning, that tighten with the
// distance, and a broken ring; locked on, they are red and shut on the place clicked, with a dot in the middle.
func (r *rocket) reticle(p *ui.Painter, m pointerAt) {
	cx, cy := m.x, m.y
	if r.locked {
		cx, cy = r.lx, r.ly
	} else if !m.in {
		return
	}
	d := math.Hypot(cx-r.x, cy-r.y)
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
	ring := (&ui.Path{}).Circle(f32(cx), f32(cy), f32(size+5)).Dash(f32(-r.spin*14), 5, 4)
	p.StrokePath(ring, 1, withAlpha(col, 0.55))
	for i := 0; i < 4; i++ {
		a := spin + float64(i)*math.Pi/2
		x0, y0 := turned(cx, cy, size, -6, a)
		xc, yc := turned(cx, cy, size, 0, a)
		x1, y1 := turned(cx, cy, size-6, 0, a)
		p.StrokePolyline([][2]float32{{x0, y0}, {xc, yc}, {x1, y1}}, 1.8, col)
	}
	if r.locked {
		p.FillPath((&ui.Path{}).Circle(f32(cx), f32(cy), 1.6), col)
		label := ui.Span{Text: "LOCK", Size: 9, Weight: 750, Color: ui.Hex("#ffffff")}
		w, h := p.MeasureText(0, label)
		box := ui.Rect{X: f32(cx + size*0.6), Y: f32(cy-size-6) - h, W: w + 8, H: h + 2} // over the ring's upper right
		p.Fill(box, ui.Hex("#ef4444"), 3)
		p.RichText(box.X+4, box.Y+1, w+4, label)
	}
}
