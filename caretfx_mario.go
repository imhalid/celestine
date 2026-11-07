package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// ---- Mario: the box in his colors, with an eye ----

// The box is red on top, his cap, and blue below, his overalls, with an eye that looks the
// way the caret last went. At a space he hops a little, and a coin springs out over his head
// and spins up and away.

const (
	marioHopT  = 230.0 // ms a hop takes
	marioCoinT = 520.0 // ms a coin is seen
)

// marioColors are his cap and his overalls, for the dark appearance and the light.
func marioColors(dark bool) (cap, overalls ui.Color) {
	if dark {
		return ui.Hex("#ff4b3e"), ui.Hex("#5b7cff")
	}
	return ui.Hex("#e52521"), ui.Hex("#2b4bd8")
}

type marioCoin struct {
	x, y float64 // where it sprang from: over his head
	t0   float64
}

type mario struct {
	clock
	prev  ui.Rect
	have  bool
	left  bool    // he looks left
	hop   float64 // when the last hop began; < 0: none
	coins []marioCoin
}

func newMario() *mario { return &mario{hop: -1} }

func (m *mario) reset() { m.have, m.coins, m.hop = false, nil, -1 }

func (m *mario) shift(dy float32) {
	m.prev.Y += dy
	for i := range m.coins {
		m.coins[i].y += float64(dy)
	}
}

func (m *mario) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := m.at(p)
	box := boxOf(f)
	h := float64(box.H)
	if m.have && math.Abs(float64(f.r.X-m.prev.X)) > 0.5 {
		m.left = f.r.X < m.prev.X
	}
	m.prev, m.have = f.r, true
	// a space: a hop, and a coin over his head
	if strings.Contains(k.typed, " ") {
		m.hop = tn
		m.coins = append(m.coins, marioCoin{float64(box.X + box.W/2), float64(box.Y), tn})
		if len(m.coins) > 24 {
			m.coins = m.coins[len(m.coins)-24:]
		}
	}
	lift := 0.0
	if m.hop >= 0 {
		t := (tn - m.hop) / marioHopT
		if t >= 1 {
			m.hop = -1
		} else {
			lift = 0.32 * h * 4 * t * (1 - t)
		}
	}
	m.drawCoins(p, tn, h)
	body := box
	body.Y -= f32(lift)
	m.drawBody(p, f, body, lift > 0)
	if m.hop >= 0 || len(m.coins) > 0 {
		p.AnimationFrame()
	}
}

// drawBody is the box, or the jelly's shape of it, see-through like the slime, red above and blue below,
// with its halo and the eye. In the air it is the plain box: the jelly's shape stays on the line.
func (m *mario) drawBody(p *ui.Painter, f caretFrame, box ui.Rect, air bool) {
	cap, overalls := marioColors(f.dark)
	path := shapeBody(box, f.style.bracket)
	if !air && f.jelly != nil {
		path = f.jelly
	}
	b := path.Bounds()
	split := box.Y + box.H*0.45
	styledDraw(p, f, box, func() {
		if air || f.jelly == nil {
			glowAround(p, box, cap, 0.9)
		} else {
			for i, w := range []float32{9, 6, 3.5} {
				p.StrokePath(path, w, withAlpha(cap, 0.9*[]float64{0.035, 0.06, 0.09}[i]))
			}
		}
		// each half as the slime is: see-through, lit from above, with a rim
		half := func(r ui.Rect, col ui.Color) {
			p.Clip(r, 0, func() {
				top := mix(col, ui.RGBA(255, 255, 255, 1), 0.45)
				p.FillPathGradient(path, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
					{Color: withAlpha(top, 0.62), At: 0},
					{Color: withAlpha(col, 0.38), At: 0.55},
					{Color: withAlpha(col, 0.5), At: 1},
				}})
				p.StrokePath(path, 1.25, withAlpha(col, 0.95))
			})
		}
		half(ui.Rect{X: b.X - 2, Y: b.Y - 2, W: b.W + 4, H: split - b.Y + 2}, cap)
		half(ui.Rect{X: b.X - 2, Y: split, W: b.W + 4, H: b.Y + b.H - split + 2}, overalls)
		// a gloss down its upper left side
		a := float32(0.55)
		if !f.dark {
			a = 0.8
		}
		gx := box.X + max(1.6, box.W*0.22)
		gloss := &ui.Path{}
		gloss.MoveTo(gx, box.Y+box.H*0.14).LineTo(gx, box.Y+box.H*0.36)
		p.StrokePath(gloss, 1.4, ui.RGBA(255, 255, 255, a))
		m.drawEye(p, box)
	})
}

// drawEye is a round dark eye with a glint, on the cap's half, toward the way he looks.
func (m *mario) drawEye(p *ui.Painter, b ui.Rect) {
	d := float32(1)
	if m.left {
		d = -1
	}
	r := float32(math.Max(1.5, math.Min(float64(b.W)*0.15, 2.6)))
	ex := b.X + b.W/2 + d*b.W*0.17
	ey := b.Y + b.H*0.24
	p.Fill(ui.Rect{X: ex - r, Y: ey - r, W: 2 * r, H: 2 * r}, ui.Hex("#1a0e08"), r)
	p.Fill(ui.Rect{X: ex - r*0.2, Y: ey - r*0.6, W: r * 0.7, H: r * 0.7}, ui.RGBA(255, 255, 255, 0.9), r*0.35)
}

// drawCoins draws the coins in the air: each springs up, slows, falls back a little and
// fades, turning about its upright as it goes.
func (m *mario) drawCoins(p *ui.Painter, tn, h float64) {
	gold, shine, rim := ui.Hex("#f8b800"), ui.Hex("#fcf0b0"), ui.Hex("#ac7c00")
	out := m.coins[:0]
	for _, c := range m.coins {
		el := tn - c.t0
		if el > marioCoinT {
			continue
		}
		out = append(out, c)
		t := el / marioCoinT
		rise := 1.5 * h * (1 - (1-t)*(1-t)) // up, slowing
		if t > 0.7 {
			rise -= 0.35 * h * smoothstep((t-0.7)/0.3) // and back down a little
		}
		alpha := 1 - smoothstep((t-0.6)/0.4)
		ch := 0.62 * h
		cw := ch * 0.72 * math.Max(0.12, math.Abs(math.Cos(el/1000*14)))
		cx, cy := c.x, c.y-rise-ch/2
		body := ui.Rect{X: f32(cx - cw/2), Y: f32(cy - ch/2), W: f32(cw), H: f32(ch)}
		p.Fill(body, withAlpha(rim, alpha), f32(cw/2))
		in := ui.Rect{X: body.X + body.W*0.18, Y: body.Y + body.H*0.1, W: body.W * 0.64, H: body.H * 0.8}
		p.Fill(in, withAlpha(gold, alpha), in.W/2)
		if cw > ch*0.3 {
			p.Fill(ui.Rect{X: f32(cx - cw*0.08), Y: in.Y + in.H*0.18, W: f32(cw * 0.16), H: in.H * 0.64}, withAlpha(shine, alpha), 0)
		}
	}
	m.coins = out
}
