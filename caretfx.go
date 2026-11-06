package main

import (
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// More carets than the slime and the mizan, in their own file: each is told what
// happened since the last frame (a kick: letters written, letters erased and
// which way, whether the text changed) and draws itself where the caret is.
//
//	Pacman        the slime's own body, in yellow, with an eye; when you erase, its side opens
//	              a little, a triangle, to the way it erases: left for Backspace, right for Delete
//	Kuyruklu      a comet: the caret leaves a tail of light where it has been

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
	caretPacman
	caretComet
	caretKinds
)

// caretNames are what the carets are called.
var caretNames = [caretKinds]string{"Slime", "Mizan", "Pacman", "Kuyruklu yıldız"}

// kick is what happened to the text since the caret was last drawn.
type kick struct {
	wrote  int    // letters written
	edited bool   // the text changed in another way, too
	erased int    // letters taken away
	dir    int    // -1: Backspace, took what was behind the caret; 1: Delete, what was ahead
	gone   string // what was erased, as far as it is known
}

// eraseOf notes letters erased from prev, between from and to, going dir.
func (a *app) eraseOf(prev richtext.Text, from, to, dir int) {
	a.erased += to - from
	a.eraseDir = dir
	if from >= 0 && to <= prev.Len() && to > from {
		s := prev.Slice(from, to).String()
		s = strings.Map(func(r rune) rune {
			if r == '\n' || r == richtext.ObjectReplacement {
				return -1
			}
			return r
		}, s)
		a.gone = s + a.gone
		if r := []rune(a.gone); len(r) > 40 {
			a.gone = string(r[len(r)-40:])
		}
	}
}

func f32(v float64) float32 { return float32(v) }

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// dtOf is the time since last, in seconds, at most 50 ms: a frame after a pause is not a leap.
func dtOf(now, last time.Time) float64 {
	if last.IsZero() {
		return 0.016
	}
	return math.Min(0.05, now.Sub(last).Seconds())
}

// ---- particles ----

type spark struct {
	x, y, vx, vy float64
	life, max    float64 // seconds left, and in all
	size         float64
	col          ui.Color
}

type sparks struct{ list []spark }

// burst throws n sparks from (x, y).
func (s *sparks) burst(x, y float64, n int, col ui.Color, power float64) {
	for i := 0; i < n; i++ {
		ang := -math.Pi/2 + (rand.Float64()-0.5)*math.Pi*1.4
		sp := power * (0.4 + rand.Float64())
		life := 0.35 + rand.Float64()*0.45
		s.list = append(s.list, spark{x, y, math.Cos(ang) * sp, math.Sin(ang) * sp, life, life, 1.6 + rand.Float64()*1.8, col})
	}
	if len(s.list) > 160 {
		s.list = s.list[len(s.list)-160:]
	}
}

// step moves them, and drops the spent.
func (s *sparks) step(dt float64) {
	out := s.list[:0]
	for _, sp := range s.list {
		sp.life -= dt
		if sp.life <= 0 {
			continue
		}
		sp.vy += 520 * dt // they fall
		sp.x += sp.vx * dt
		sp.y += sp.vy * dt
		out = append(out, sp)
	}
	s.list = out
}

func (s *sparks) draw(p *ui.Painter) {
	for _, sp := range s.list {
		a := clamp01(sp.life / sp.max)
		p.Fill(ui.Rect{X: float32(sp.x - sp.size/2), Y: float32(sp.y - sp.size/2), W: float32(sp.size), H: float32(sp.size)}, withAlpha(sp.col, a), rad(2))
	}
}

// ---- Pacman ----

// pacman is the slime, yellow, with an eye. Erasing opens a triangle in its side - the
// side it erases toward - which closes again.
type pacman struct {
	body *slime
	open float64 // how open the mouth is, 0..1
	hold float64 // seconds it stays open
	dir  int     // -1 looks left (Backspace), 1 right
	last time.Time
}

func newPacman() *pacman { return &pacman{body: newSlime(), dir: 1} }

func (g *pacman) reset() { g.body.placed, g.open, g.hold, g.dir = false, 0, 0, 1 }

func (g *pacman) shift(dy float32) { g.body.shift(0, dy) }

func (g *pacman) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	dt := dtOf(now, g.last)
	g.last = now
	if k.erased > 0 {
		g.dir = k.dir
		if g.dir == 0 {
			g.dir = -1
		}
		g.hold = 0.16 + 0.03*math.Min(float64(k.erased), 8)
		g.open = math.Max(g.open, 0.35) // the mouth is there at once
	}
	if g.hold > 0 {
		g.hold -= dt
		g.open += (1 - g.open) * (1 - math.Exp(-dt*34))
	} else {
		g.open = math.Max(0, g.open-dt*5)
		if g.open == 0 {
			g.dir = 1
		}
	}
	body := ui.Hex("#ffd60a")
	if !f.dark {
		body = ui.Hex("#eab308")
	}
	g.body.drawWith(p, f.r, f.cw, body, f.dark, func(b ui.Rect) { g.face(p, b) })
}

// face draws the eye, and the mouth when it is open, on the jelly's box b.
func (g *pacman) face(p *ui.Painter, b ui.Rect) {
	ink := ui.Hex("#2a1d00")
	d := float32(g.dir)
	cx := b.X + b.W/2
	// the mouth: a triangle from the middle of the body to its side, opening with the erase
	if g.open > 0.02 {
		half := b.H * 0.2 * f32(g.open)
		my := b.Y + b.H*0.62
		edge := b.X + b.W
		if d < 0 {
			edge = b.X
		}
		tri := &ui.Path{}
		tri.MoveTo(cx, my).LineTo(edge+d*1.5, my-half).LineTo(edge+d*1.5, my+half).Close()
		p.FillPath(tri, withAlpha(ink, 0.8))
	}
	// the eye
	r := float32(math.Max(1.5, math.Min(float64(b.W)*0.15, 2.6)))
	ex := cx + d*b.W*0.17
	ey := b.Y + b.H*0.3
	p.Fill(ui.Rect{X: ex - r, Y: ey - r, W: 2 * r, H: 2 * r}, ink, r)
	p.Fill(ui.Rect{X: ex - r*0.2, Y: ey - r*0.6, W: r * 0.7, H: r * 0.7}, ui.RGBA(255, 255, 255, 0.9), r*0.35)
}

// ---- the comet ----

type comet struct {
	trail    []cometPt
	last     time.Time
	boost    float64 // 1 on a letter, falling: the head swells
	prev     ui.Rect
	placed   bool
	sparkles sparks
}

type cometPt struct {
	x, y float64
	at   time.Time
}

func newComet() *comet { return &comet{} }

func (m *comet) reset() { m.trail, m.placed = nil, false }

func (m *comet) shift(dy float32) {
	for i := range m.trail {
		m.trail[i].y += float64(dy)
	}
	m.prev.Y += dy
}

const cometTail = 520 * time.Millisecond

func (m *comet) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	dt := dtOf(now, m.last)
	m.last = now
	r := f.r
	hx, hy := float64(r.X), float64(r.Y)+float64(r.H)/2
	if !m.placed {
		m.trail, m.placed = nil, true
	}
	if n := len(m.trail); n == 0 || math.Hypot(m.trail[n-1].x-hx, m.trail[n-1].y-hy) > 0.5 {
		m.trail = append(m.trail, cometPt{hx, hy, now})
	} else {
		m.trail[n-1].at = now // it rests: its tail shortens into it
	}
	for len(m.trail) > 1 && now.Sub(m.trail[0].at) > cometTail {
		m.trail = m.trail[1:]
	}
	if k.wrote > 0 {
		m.boost = 1
		m.sparkles.burst(hx, hy, 2*k.wrote, p.Theme().Accent, 120)
	}
	m.boost = math.Max(0, m.boost-dt*5)
	m.sparkles.step(dt)
	m.prev = r

	a, b := ui.Hex("#22d3ee"), ui.Hex("#e879f9") // cyan to magenta along the tail
	// the tail: segments, fading and thinning with age
	for i := 1; i < len(m.trail); i++ {
		p0, p1 := m.trail[i-1], m.trail[i]
		age := clamp01(float64(now.Sub(p1.at)) / float64(cometTail))
		fade := 1 - age
		col := mix(a, b, age)
		seg := &ui.Path{}
		seg.MoveTo(f32(p0.x), f32(p0.y)).LineTo(f32(p1.x), f32(p1.y))
		seg.Cap(ui.RoundCap)
		w := float32(float64(r.H) * (0.04 + 0.2*fade))
		p.StrokePath(seg, w*2.4, withAlpha(col, 0.05*fade))
		p.StrokePath(seg, w, withAlpha(col, 0.5*fade))
	}
	// the head: a bar with a glow
	hw := float32(2.4 + 2.2*m.boost)
	head := ui.Rect{X: f32(hx) - hw/2, Y: r.Y, W: hw, H: r.H}
	for i, w := range []float32{12, 7, 3.5} {
		p.Fill(ui.Rect{X: head.X - w/2, Y: head.Y - w/2, W: head.W + w, H: head.H + w}, withAlpha(a, []float64{0.05, 0.09, 0.16}[i]), 5)
	}
	p.Fill(head, mix(a, ui.RGBA(255, 255, 255, 1), 0.35), 1.5)
	m.sparkles.draw(p)
	p.AnimationFrame()
}
