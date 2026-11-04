package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// A shockwave from the caret on every edit: a ring runs out from it, and each
// letter it reaches is knocked away from the caret, tipped a little, and eased
// back to its place on a soft spring.
//
// While anything moves, the text is drawn a letter at a time (ui.Shape +
// Painter.Glyphs), each at its own offset; a copy of the layout with its text
// invisible still paints what lies under the letters (inline code's
// background). At rest the layout is drawn whole again.

// shockwaveOn turns the effect on; it is off for now.
const shockwaveOn = false

const (
	waveSpeed  = 720.0 // px/s the ring runs out at
	waveReach  = 230.0 // px: how far it carries
	wavePower  = 210.0 // px/s given to a letter right at the caret
	waveMax    = 5     // waves alive at once; typing fast keeps the newest
	settleFreq = 7.5   // rad/s: the spring that brings a letter home - slow...
	settleDamp = 0.42  // ...and a little bouncy
	jiggleStep = 1.0 / 240
)

// cell is one letter (a grapheme) of the layout, where it stands and how it is drawn.
type cell struct {
	i       int     // its UTF-16 offset in the text
	x, base float32 // its left and its baseline, in the layout
	cx, cy  float32 // its middle
	glyphs  []ui.Glyph
	color   ui.Color
}

// jiggle is how far a letter has been knocked from its place, and its speed.
type jiggle struct{ x, y, a, vx, vy, va float64 }

type wave struct {
	x, y, power float64
	t0          time.Time
	hit         map[int]bool
}

type quake struct {
	cells   []cell
	ghost   *ui.TextLayout // the layout with its text invisible: backgrounds only
	off     map[int]*jiggle
	waves   []wave
	pending float64 // a wave to start at the caret in the next frame, of this power
	last    time.Time
	shaped  map[string][]ui.Glyph
}

func newQuake() *quake {
	return &quake{off: map[int]*jiggle{}, shaped: map[string][]ui.Glyph{}}
}

// build reads the letters of a fresh layout from the spans it was made of.
func (q *quake) build(text string, spans []ui.Span, layout *ui.TextLayout, base ui.Font, width float32) {
	ghost := make([]ui.Span, len(spans))
	for i, s := range spans {
		s.Color = ui.Color{}
		ghost[i] = s
	}
	q.ghost = ui.ShapeRichText(ghost, base, width)
	q.cells = q.cells[:0]
	lines := layout.Lines()
	n := ui.UTF16Len(text)
	span, spanEnd := 0, 0
	if len(spans) > 0 {
		spanEnd = ui.UTF16Len(spans[0].Text)
	}
	for i := 0; i < n; {
		next := layout.NextBoundary(i)
		if next <= i {
			break
		}
		for span < len(spans)-1 && i >= spanEnd {
			span++
			spanEnd += ui.UTF16Len(spans[span].Text)
		}
		a, z := ui.UTF16ByteOffset(text, i), ui.UTF16ByteOffset(text, next)
		s := text[a:z]
		if strings.TrimSpace(s) != "" && len(spans) > 0 {
			sp := spans[span]
			f := ui.Font{Family: sp.Font, Size: sp.Size, Weight: sp.Weight, Italic: sp.Italic}
			if f.Size == 0 {
				f.Size = base.Size
			}
			key := s + "\x00" + f.Family + string(rune(f.Weight)) + string(rune(int(f.Size*10)))
			g, ok := q.shaped[key]
			if !ok {
				g = ui.Shape(s, f)
				q.shaped[key] = g
			}
			c := layout.Caret(i)
			mid := c.Y + c.H/2
			bl, h := c.Y+c.H*0.75, c.H
			for _, l := range lines {
				if mid >= l.Rect.Y && mid <= l.Rect.Y+l.Rect.H {
					bl, h = l.Baseline, l.Ascent
					break
				}
			}
			var w float32
			for _, gl := range g {
				w = max(w, gl.X+gl.Advance)
			}
			q.cells = append(q.cells, cell{i: i, x: c.X, base: bl, cx: c.X + w/2, cy: bl - h*0.35, glyphs: g, color: sp.Color})
		}
		i = next
	}
}

// edited moves the letters' offsets along with an edit of [start, end) that
// left n units in its place, and asks for a wave.
func (q *quake) edited(start, end, n int, power float64) {
	moved := map[int]*jiggle{}
	for k, j := range q.off {
		switch {
		case k < start:
			moved[k] = j
		case k >= end:
			moved[k+n-(end-start)] = j
		}
	}
	q.off = moved
	q.pending = math.Max(q.pending, power)
}

func (q *quake) busy() bool { return len(q.waves) > 0 || len(q.off) > 0 }

// step starts a wave asked for at the caret (cx, cy), lets the rings run,
// knocks the letters they reach, and runs every letter's spring.
func (q *quake) step(now time.Time, cx, cy float64) {
	dt := 0.0
	if !q.last.IsZero() {
		dt = math.Min(now.Sub(q.last).Seconds(), 0.05)
	}
	q.last = now
	if q.pending > 0 {
		q.waves = append(q.waves, wave{x: cx, y: cy, power: q.pending, t0: now, hit: map[int]bool{}})
		if len(q.waves) > waveMax {
			q.waves = q.waves[len(q.waves)-waveMax:]
		}
		q.pending = 0
	}
	alive := q.waves[:0]
	for _, w := range q.waves {
		r := now.Sub(w.t0).Seconds() * waveSpeed
		for _, c := range q.cells {
			if w.hit[c.i] {
				continue
			}
			dx, dy := float64(c.cx)-w.x, float64(c.cy)-w.y
			d := math.Hypot(dx, dy)
			if d > r {
				continue
			}
			w.hit[c.i] = true
			if d >= waveReach {
				continue
			}
			if d < 1 {
				dx, dy, d = 0, -1, 1
			}
			// Letters right by the caret are nudged, not thrown: they would fly into it.
			f := w.power * math.Pow(1-d/waveReach, 1.6) * (0.3 + 0.7*smoothStep((d-4)/34))
			j := q.off[c.i]
			if j == nil {
				j = &jiggle{}
				q.off[c.i] = j
			}
			j.vx += dx / d * f
			j.vy += dy / d * f * 0.8
			j.va += dx / d * f * 0.12 // degrees/s: tipped the way it is pushed
		}
		if r < waveReach+60 {
			alive = append(alive, w)
		}
	}
	q.waves = alive
	k, c := settleFreq*settleFreq, 2*settleDamp*settleFreq
	for key, j := range q.off {
		for left := dt; left > 1e-6; {
			h := math.Min(jiggleStep, left)
			left -= h
			j.vx += (-k*j.x - c*j.vx) * h
			j.vy += (-k*j.y - c*j.vy) * h
			j.va += (-k*j.a - c*j.va) * h
			j.x += j.vx * h
			j.y += j.vy * h
			j.a += j.va * h
		}
		if math.Abs(j.x)+math.Abs(j.y)+math.Abs(j.a) < 0.02 && math.Abs(j.vx)+math.Abs(j.vy)+math.Abs(j.va) < 0.5 {
			delete(q.off, key)
		}
	}
}

// draw paints the text a letter at a time, each at its offset, with the
// layout's backgrounds under them; then the rings over it.
func (q *quake) draw(p *ui.Painter, ox, oy float32, col ui.Color, now time.Time) {
	p.TextLayout(q.ghost, ox, oy, ui.Color{})
	for _, c := range q.cells {
		j := q.off[c.i]
		if j == nil {
			p.Glyphs(c.glyphs, ox+c.x, oy+c.base, c.color)
			continue
		}
		dx, dy := float32(j.x), float32(j.y)
		p.Transform(ui.Rotation(float32(j.a)), ox+c.cx+dx, oy+c.cy+dy, func() {
			p.Glyphs(c.glyphs, ox+c.x+dx, oy+c.base+dy, c.color)
		})
	}
	for _, w := range q.waves {
		r := now.Sub(w.t0).Seconds() * waveSpeed
		u := r / waveReach
		if u >= 1 || r < 2 {
			continue
		}
		a := math.Pow(1-u, 1.5) * math.Min(1, w.power/wavePower)
		ring := (&ui.Path{}).Circle(ox+float32(w.x), oy+float32(w.y), float32(r))
		p.StrokePath(ring, 10*float32(1-u)+2, withAlpha(col, 0.08*a))
		p.StrokePath(ring, 1.5, withAlpha(col, 0.55*a))
	}
}

func smoothStep(x float64) float64 { x = clamp(x, 0, 1); return x * x * (3 - 2*x) }
