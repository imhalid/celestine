package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Cursor-Smith's deleting carets (MIT, see NOTICE.md), each as it ships and with nothing to
// set: Back-man bites the letters you erase, and the Portal lets them fall through a floor.

// goneLetters are the letters that were erased, as they stood: from the caret, to the right,
// nearest to the caret first.
type goneLetter struct {
	ch   string
	x, w float64
}

func goneLetters(p *ui.Painter, f caretFrame, k kick) []goneLetter {
	runes, widths := letterWidths(p, k.gone, textSize(f))
	var out []goneLetter
	off := 0.0
	for i, r := range runes {
		if r != " " {
			out = append(out, goneLetter{r, float64(f.r.X) + off, widths[i]})
		}
		off += widths[i]
	}
	if k.dir >= 0 && k.dir != 0 { // Delete: the nearest is the first
		return out
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { // Backspace: the nearest is the last
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// ---- Back-man ----

const (
	bmChomp   = 140.0 // one bite, ms
	bmGulp    = 220.0
	bmSquint  = 420.0
	bmGrow    = 0.18
	bmHz      = 6.0
	bmDamp    = 0.62
	bmKick    = 26.0
	bmMax     = 0.6
	bmSlide   = 70.0
	bmGulpAt  = 0.6
	bmBigChew = 1.5
)

type bmLetter struct {
	ch      string
	cx, cy  float64
	size    float32
	t0, t1  float64
	mouthDx float64
}

type backman struct {
	base      time.Time
	has       bool
	c0, t     float64
	big       bool
	dir       int
	bend, v   float64
	at        float64
	eye       float64
	shut      bool
	meal      []bmLetter
	lastShape [8][2]float64
}

func newBackman() *backman { return &backman{dir: 1} }

func (b *backman) reset() { b.has, b.meal, b.shut, b.eye, b.bend, b.v = false, nil, false, 0, 0, 0 }

func (b *backman) shift(dy float32) {
	for i := range b.meal {
		b.meal[i].cy += float64(dy)
	}
}

type bite struct {
	open, front, back, squint float64
	end                       float64
	done                      bool
}

func bmBite(tn, tk float64, big bool) bite {
	ch, gulp, grow, sq := bmChomp, bmGulp, bmGrow, bmSquint
	if big {
		ch, gulp, grow, sq = ch*bmBigChew, gulp*1.4, grow*1.9, sq*1.4
	}
	end := math.Max(1, math.Ceil((math.Max(0, tk)+ch)/ch)) * ch
	var bt bite
	bt.end = end
	if tn >= 0 && tn < end {
		bt.open = math.Sin(math.Pi * math.Mod(tn, ch) / ch)
	}
	from := tn - (end - ch) - bmGulpAt*ch
	g, q := from/gulp, from/sq
	hump := func(g, a, b float64) float64 {
		if g <= a || g >= b {
			return 0
		}
		return math.Sin(math.Pi * (g - a) / (b - a))
	}
	bt.front = grow * hump(g, 0, 0.55)
	bt.back = grow * hump(g, 0.35, 1)
	switch {
	case q <= 0 || q >= 1:
	case q < 0.2:
		bt.squint = smoothstep(q / 0.2)
	case q < 0.7:
		bt.squint = 1
	default:
		bt.squint = smoothstep((1 - q) / 0.3)
	}
	bt.done = g >= 1 && q >= 1
	return bt
}

func (b *backman) spring(dt float64) {
	dt = math.Max(0, math.Min(dt/1000, 2))
	steps := int(math.Max(1, math.Ceil(dt*480)))
	h := dt / float64(steps)
	w := 2 * math.Pi * bmHz
	for i := 0; i < steps; i++ {
		b.v += (-w*w*b.bend - 2*bmDamp*w*b.v) * h
		b.bend += b.v * h
		if math.Abs(b.bend) > bmMax {
			b.bend = math.Copysign(bmMax, b.bend)
			if b.v*b.bend > 0 {
				b.v = 0
			}
		}
	}
}

func (b *backman) draw(p *ui.Painter, f caretFrame, k kick) {
	if k.wrote > 0 && k.erased == 0 { // writing, it turns the way the writing goes; erasing, the way it eats (#283)
		b.dir = 1
	}
	now := p.Now()
	if b.base.IsZero() {
		b.base = now
	}
	tn := ms(now.Sub(b.base))
	box := boxOf(f)
	col := cursorColor(f.dark)
	if k.erased > 0 && k.gone != "" {
		b.key(p, f, k, tn)
	}
	if !b.has {
		styledDraw(p, f, box, func() { glassPath(p, restBody(f, box), col, f.dark) })
		return
	}
	bt := bmBite(math.Max(0, tn-b.c0), b.t-b.c0, b.big)
	if bt.squint >= 0.55 {
		b.shut = true
	}
	target := bt.squint
	if b.shut {
		target = 1
	}
	tau := 80.0
	if target > b.eye {
		tau = 30
	}
	b.eye = target + (b.eye-target)*math.Exp(-math.Max(0, tn-b.at)/tau)
	if math.Abs(b.eye-target) < 0.005 {
		b.eye = target
	}
	b.spring(tn - b.at)
	b.at = tn
	out := b.meal[:0]
	for _, m := range b.meal {
		if tn < m.t1 {
			out = append(out, m)
		}
	}
	b.meal = out
	if bt.done && math.Abs(b.bend) < 0.004 && math.Abs(b.v) < 0.05 && len(b.meal) == 0 {
		b.has, b.shut = false, false
		styledDraw(p, f, box, func() { glassPath(p, restBody(f, box), col, f.dark) })
		return
	}
	pts := b.outline(box, bt)
	// the letters slide into the mouth, under the body
	text := p.Theme().Text
	for _, m := range b.meal {
		u := clamp01((tn - m.t0) / math.Max(1, math.Min(m.t1-m.t0, bmSlide)))
		e := 1 - (1-u)*(1-u)
		mx, my := b.lastShape[3][0], b.lastShape[3][1]
		glyphAt(p, m.ch, f32(m.cx+(mx-m.cx)*e), f32(m.cy+(my-m.cy)*e), m.size, text, false, 0, math.Max(0.05, 1-0.8*e))
	}
	styledDraw(p, f, box, func() { glassPath(p, pts, col, f.dark) })
	b.drawEye(p, f, box, bt)
	idleFrame(p, f)
}

// key is a letter erased at tn: the bend gets its kick, a bite is begun or went on, the letters wait.
func (b *backman) key(p *ui.Painter, f caretFrame, k kick, tn float64) {
	big := k.erased > 1
	dir := k.dir
	if dir == 0 {
		dir = -1
	}
	kk := float64(dir) * bmKick
	if big {
		kk *= 1.6
	}
	if b.has {
		b.spring(tn - b.at)
		for i := range b.meal {
			b.meal[i].t1 = math.Min(b.meal[i].t1, tn)
		}
		if big || tn-b.c0 >= bmBite(0, b.t-b.c0, big).end {
			b.c0, b.big = tn, big
		}
		b.t = tn
		b.at = math.Max(b.at, tn)
		b.v += kk
	} else {
		*b = backman{base: b.base, has: true, c0: tn, t: tn, big: big, v: kk, at: tn, dir: dir}
	}
	b.dir = dir
	ch := bmChomp
	if big {
		ch *= bmBigChew
	}
	t1 := b.c0 + math.Ceil((tn-b.c0+0.4*ch)/ch)*ch
	size := textSize(f)
	cy := float64(f.r.Y + f.r.H/2)
	for _, l := range goneLetters(p, f, k) {
		b.meal = append(b.meal, bmLetter{l.ch, l.x + l.w/2, cy, size, tn, t1, 0})
	}
}

// outline is the creature as one path: the box with a soft mouth, its sides bent.
func (b *backman) outline(box ui.Rect, bt bite) *ui.Path {
	w, h := float64(box.W), float64(box.H)
	jaw, depth := 0.36*bt.open, 0.78*bt.open
	type pt struct {
		x, y  float64
		front bool
	}
	ps := [8]pt{{0, 0, false}, {1, 0, true}, {1, 0.5 - jaw, true}, {1 - depth, 0.5, true}, {1, 0.5 + jaw, true}, {1, 1, true}, {0, 1, false}, {0, 0.5, false}}
	f := 1.0
	if b.dir < 0 {
		f = -1
	}
	var xy [8][2]float64
	for i, q := range ps {
		mid := 1 - math.Abs(2*q.y-1)
		sx := q.x
		if f < 0 {
			sx = 1 - q.x
		}
		grow := -f * bt.back
		if q.front {
			grow = f * bt.front
		}
		X := sx + (b.bend+grow)*mid
		xy[i] = [2]float64{float64(box.X) + X*w, float64(box.Y) + q.y*h}
	}
	b.lastShape = xy
	c := float64(boxRadius(box.W, box.H))
	jawPx := jaw * h
	rad := [8]float64{c, c, math.Min(0.24*w, 0.5*jawPx), math.Min(0.14*w, 0.3*jawPx), math.Min(0.24*w, 0.5*jawPx), c, c, 0}
	dist := func(a, b [2]float64) float64 { return math.Hypot(a[0]-b[0], a[1]-b[1]) }
	unit := func(a, b [2]float64) [2]float64 { // from a toward b
		d := dist(a, b)
		if d == 0 {
			return [2]float64{}
		}
		return [2]float64{(b[0] - a[0]) / d, (b[1] - a[1]) / d}
	}
	path := &ui.Path{}
	path.MoveTo(f32((xy[0][0]+xy[1][0])/2), f32((xy[0][1]+xy[1][1])/2))
	for k := 1; k <= 8; k++ {
		i := k % 8
		p0, p, n := xy[(i+7)%8], xy[i], xy[(i+1)%8]
		d := math.Min(rad[i], math.Min(dist(p0, p)/2, dist(n, p)/2))
		if d < 0.05 {
			path.LineTo(f32(p[0]), f32(p[1]))
			continue
		}
		u1, u2 := unit(p, p0), unit(p, n)
		path.LineTo(f32(p[0]+u1[0]*d), f32(p[1]+u1[1]*d))
		path.QuadTo(f32(p[0]), f32(p[1]), f32(p[0]+u2[0]*d), f32(p[1]+u2[1]*d))
	}
	return path.Close()
}

// drawEye is the big square eye, a hole in the body, that squints to a ^ and has a glint.
func (b *backman) drawEye(p *ui.Painter, f caretFrame, box ui.Rect, bt bite) {
	w, h := float64(box.W), float64(box.H)
	fd := 1.0
	if b.dir < 0 {
		fd = -1
	}
	size := math.Min(0.6*w, 0.24*h)
	top := 0.05 * h
	cy := top + size/2
	cx := 0.7*w - size/2
	if fd < 0 {
		cx = w - 0.7*w + size/2
	}
	cx += b.bend * w * (1 - math.Abs(2*cy/h-1))
	ox, oy := float64(box.X), float64(box.Y)
	s := b.eye
	hole := f.bg
	if s >= 0.55 {
		R, a, t := size/2, 0.3*size, math.Max(1, 0.22*size)
		poly := &ui.Path{}
		poly.MoveTo(f32(ox+cx-R), f32(oy+cy+a)).LineTo(f32(ox+cx), f32(oy+cy-a)).LineTo(f32(ox+cx+R), f32(oy+cy+a)).
			LineTo(f32(ox+cx+R), f32(oy+cy+a+t)).LineTo(f32(ox+cx), f32(oy+cy-a+t)).LineTo(f32(ox+cx-R), f32(oy+cy+a+t)).Close()
		p.FillPath(poly, hole)
		return
	}
	hh := size * (1 - 0.75*s/0.55) / 2
	p.Fill(ui.Rect{X: f32(ox + cx - size/2), Y: f32(oy + cy - hh), W: f32(size), H: f32(2 * hh)}, hole, 1)
	if s < 0.25 {
		g, inset := math.Max(1, 0.28*size), 0.16*size
		gx := cx + size/2 - inset - g
		if fd < 0 {
			gx = cx - size/2 + inset
		}
		p.Fill(ui.Rect{X: f32(ox + gx), Y: f32(oy + cy - size/2 + inset), W: f32(g), H: f32(g)}, ui.RGBA(255, 255, 255, 0.95), 0)
	}
}

// ---- Portal ----

type portalLetter struct {
	ch        string
	cx, cy    float64
	half      float64
	ax        float64
	t0        float64
	rush      float64
	rushed    bool
	size      float32
	floorLeft float64
}

type portal struct {
	base    time.Time
	letters []portalLetter
	at      float64
}

func newPortal() *portal { return &portal{} }

func (g *portal) reset() { g.letters, g.at = nil, 0 }

func (g *portal) shift(dy float32) {
	for i := range g.letters {
		g.letters[i].cy += float64(dy)
	}
}

const (
	poFall = 420.0
	poTall = 2.5
	poHold = 200.0
	poRush = 60.0
)

func (g *portal) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	if g.base.IsZero() {
		g.base = now
	}
	tn := ms(now.Sub(g.base))
	box := boxOf(f)
	col := cursorColor(f.dark)
	if k.erased > 0 && k.gone != "" {
		g.at = tn
		for i := range g.letters {
			if !g.letters[i].rushed {
				g.letters[i].rush, g.letters[i].rushed = tn, true
			}
		}
		size := textSize(f)
		gl := goneLetters(p, f, k)
		for _, l := range gl {
			g.letters = append(g.letters, portalLetter{ch: l.ch, cx: l.x + l.w/2, cy: float64(f.r.Y + f.r.H/2), half: 0.3 * float64(size), ax: float64(f.r.X), t0: tn, size: size, floorLeft: float64(f.r.X)})
		}
		g.at = math.Max(g.at, tn+poFall)
	}
	if len(g.letters) == 0 && tn-g.at >= poHold {
		glassBody(p, f, box, col, 1, 0.9)
		return
	}
	uh := math.Max(2, math.Min(4, 0.1*float64(box.H)))
	floorTop := float64(box.Y+box.H) - uh
	floorLeft := float64(box.X)
	text := p.Theme().Text
	glow := 0.0
	right := float64(box.X + box.W)
	out := g.letters[:0]
	for _, l := range g.letters {
		u := clamp01((tn - l.t0) / poFall)
		v := u / 0.85
		if v > 1 {
			v = 1
		}
		if l.rushed {
			v = math.Max(v, clamp01((tn-l.rush)/poRush))
		}
		if u >= 1 || (l.rushed && tn-l.rush >= poRush) {
			continue
		}
		out = append(out, l)
		foot0 := l.cy + l.half
		tall := poTall * l.half
		foot := foot0 + (math.Max(0, floorTop-foot0)+tall+1)*smoothstep(v)
		through := clamp01((foot - floorTop) / tall)
		x := l.cx + (floorLeft - l.ax)
		if through > 0 && through < 1 {
			glow = math.Max(glow, math.Sin(math.Pi*through))
		}
		span := smoothstep(u/0.15) * (1 - smoothstep((u-0.85)/0.15))
		right = math.Max(right, float64(box.X+box.W)+math.Max(0, x+l.half-float64(box.X+box.W))*span)
		cy := foot - l.half
		above := ui.Rect{X: f32(x - 40), Y: f32(cy - 80), W: 80, H: f32(floorTop - (cy - 80))}
		p.Clip(above, 0, func() { glyphAt(p, l.ch, f32(x), f32(cy), l.size, text, false, 0, 1) })
		band := 0.45 * poTall * l.half
		tint := ui.Rect{X: f32(x - 40), Y: f32(floorTop - band), W: 80, H: f32(band)}
		p.Clip(tint, 0, func() { // the letter itself, lit, where it goes through: not a block
			glyphAt(p, l.ch, f32(x), f32(cy), l.size, withAlpha(col, 0.9), false, 0, 1)
		})
	}
	g.letters = out
	bar := ui.Rect{X: f32(floorLeft), Y: f32(floorTop), W: f32(right - floorLeft), H: f32(uh)}
	p.Fill(bar, withAlpha(col, 0.95), f32(uh/2))
	if glow > 0.01 {
		p.Fill(bar, ui.RGBA(255, 255, 255, float32(0.6*glow)), f32(uh/2))
	}
	idleFrame(p, f)
}
