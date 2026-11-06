package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Cursor-Smith's carets that go on round the box (MIT, see NOTICE.md), each as it ships and
// with nothing to set: the speed demon and the torch.

// moved reports whether the caret box moved since the last frame, and by how much.
func moved(prev, cur ui.Rect) (bool, float64) {
	d := math.Hypot(float64(cur.X-prev.X), float64(cur.Y-prev.Y))
	return d > 0.5, d
}

// squareMote is a square spark of the speed demon.
type squareMote struct {
	x, y, vx, vy, size float64
	t0, life           float64
	col                ui.Color
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
	drawBody(p, f, box, col, 1, glow)
	if g.heat > 0 || len(g.sparks) > 0 {
		p.AnimationFrame()
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
	drawBody(p, f, box, cursorColor(f.dark), 1, 0.9)
	if math.Hypot(tx-g.x, ty-g.y) > 0.25 {
		p.AnimationFrame()
	}
}
