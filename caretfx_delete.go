package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Cursor-Smith's deleting carets (MIT, see NOTICE.md), each as it ships and with nothing to
// set: Back-man bites the letters you erase, the Shredder cuts them into ribbons, the Portal
// lets them fall through a floor, and Evaporate lifts them away.

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
		drawBoxCursor(p, box, col, 1)
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
		drawBoxCursor(p, box, col, 1)
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
	glowAround(p, box, col, 0.5)
	p.FillPath(pts, withAlpha(col, 0.9))
	b.drawEye(p, f, box, bt)
	p.AnimationFrame()
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

// ---- Shredder ----

type shredLetter struct {
	ch        string
	x, w      float64
	size      float32
	t0        float64
	rush      float64
	rushed    bool
	cut       float64
	top, h    float64
	seed      float64
	joltStamp float64
}

type shredder struct {
	base     time.Time
	letters  []shredLetter
	runT0    float64
	lastKey  float64
	active   bool
	joltT    float64
	joltAmp  float64
	cut      float64
	boxY, bH float64
}

func newShredder() *shredder { return &shredder{} }

func (s *shredder) reset() { s.letters, s.active = nil, false }

func (s *shredder) shift(dy float32) {
	for i := range s.letters {
		s.letters[i].top += float64(dy)
	}
	s.boxY += float64(dy)
}

const (
	shFeed = 260.0
	shFall = 420.0
	shHold = 320.0
	shJolt = 140.0
)

func (s *shredder) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	if s.base.IsZero() {
		s.base = now
	}
	tn := ms(now.Sub(s.base))
	box := boxOf(f)
	col := cursorColor(f.dark)
	if k.erased > 0 && k.gone != "" {
		for i := range s.letters {
			if tn-s.letters[i].t0 < shFeed && !s.letters[i].rushed {
				s.letters[i].rush, s.letters[i].rushed = tn, true
			}
		}
		if s.active && tn-s.lastKey < shHold {
			s.lastKey = tn
		} else {
			s.active, s.runT0, s.lastKey = true, tn, tn
		}
		amp := 0.8
		gl := goneLetters(p, f, k)
		if len(gl) > 1 {
			amp = 1.8
		}
		s.joltT, s.joltAmp = tn, amp
		for _, l := range gl {
			s.letters = append(s.letters, shredLetter{ch: l.ch, x: l.x, w: l.w, size: textSize(f), t0: tn, top: float64(f.r.Y), h: float64(f.r.H), seed: float64(len(l.ch))*0.37 + l.x*0.011 + tn*0.0001})
		}
		s.cut = float64(f.r.X)
	}
	if !s.active && len(s.letters) == 0 {
		drawBoxCursor(p, box, col, 1)
		return
	}
	first, last := tn-s.runT0, tn-s.lastKey
	dash := math.Min(clamp01(first/40), 1)
	if last >= shHold {
		dash = 0
	} else if last >= 230 {
		dash = math.Min(dash, clamp01((shHold-last)/90))
	}
	if last >= shHold && len(s.letters) == 0 {
		s.active = false
		drawBoxCursor(p, box, col, 1)
		return
	}
	// the letters are fed leftward through the cut, and cut into ribbons
	text := p.Theme().Text
	out := s.letters[:0]
	for _, l := range s.letters {
		age := tn - l.t0
		if age >= shFeed+shFall {
			continue
		}
		out = append(out, l)
		s.drawLetter(p, l, tn, age, text)
	}
	s.letters = out
	// the box as blades
	n := int(math.Max(4, math.Min(8, math.Round(float64(box.H)/4))))
	pitch := float64(box.H) / float64(n)
	gap := pitch * 0.42 * dash
	run := 0.0
	if gap > 0 {
		run = math.Mod(tn/1000*6*pitch, pitch)
	}
	jolt := 0.0
	if since := tn - s.joltT; since >= 0 && since < shJolt {
		jolt = s.joltAmp * math.Pow(1-since/shJolt, 2) * math.Sin(since*0.12)
	}
	for i := -1; i < n; i++ {
		top := float64(i)*pitch + gap/2 + run
		a, b := math.Max(0, top), math.Min(float64(box.H), top+pitch-gap)
		if b-a <= 0.01 {
			continue
		}
		dx := jolt
		if (i+n)%2 == 0 {
			dx = jolt * 0.6
		}
		p.Fill(ui.Rect{X: box.X + f32(dx), Y: box.Y + f32(a), W: box.W, H: f32(b - a)}, withAlpha(col, 0.9), 0)
	}
	p.AnimationFrame()
}

func (s *shredder) drawLetter(p *ui.Painter, l shredLetter, tn, age float64, text ui.Color) {
	u := clamp01(age / shFeed)
	if l.rushed {
		u = math.Max(u, clamp01((tn-l.rush)/25))
	}
	e := smoothstep(u)
	dx := -(l.x + l.w - s.cut + 1) * e
	fall := math.Max(0, age-52) / 1000
	dy := 0.5 * 12 * l.h * fall * fall
	alpha := 1 - clamp01((age-shFeed)/shFall)
	cx := l.x + l.w/2 + dx
	cy := l.top + l.h/2
	cut := s.cut
	// the part not through the cut yet
	p.Clip(ui.Rect{X: f32(cut), Y: f32(l.top - 4), W: 400, H: f32(l.h + 8)}, 0, func() {
		glyphAt(p, l.ch, f32(cx), f32(cy), l.size, withAlpha(text, 1), false, 0, 1)
	})
	// five ribbons of what is through
	const ribbons = 5
	band := l.h / ribbons
	for i := 0; i < ribbons; i++ {
		k := math.Mod(math.Abs(math.Sin(float64(len(l.ch))*12.9898+float64(i)*78.233+l.x*0.37+l.t0*0.011)*43758.5453), 1)
		grow := clamp01(fall / 0.12)
		ph := 2 * math.Pi * k
		w := 2 * math.Pi * 3 * (0.8 + 0.4*k) * fall
		sway := 0.3 * l.w * grow * math.Sin(ph+0.6*w)
		tilt := 0.4*grow*math.Sin(1.7*ph+w) - float64(i-2)*0.22*grow*0.6
		flip := 1 - 0.67*grow*(0.5-0.5*math.Cos(2.3*ph+1.3*w))
		dyi := dy * (1 + 0.25*(2*k-1))
		mx, my := cut-l.w/2, l.top+(float64(i)+0.5)*band
		t := ui.Rotation(f32(tilt)).Then(ui.Scaling(1, f32(math.Max(0.05, flip)))).Then(ui.Translation(f32(sway), f32(dyi)))
		clip := ui.Rect{X: f32(cut - l.w - 6), Y: f32(l.top + float64(i)*band + 0.35), W: f32(l.w + 6), H: f32(band - 0.7)}
		p.Transform(t, f32(mx), f32(my), func() {
			p.Clip(clip, 0, func() {
				glyphAt(p, l.ch, f32(cx), f32(cy), l.size, withAlpha(text, alpha), false, 0, 1)
			})
		})
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
		drawBoxCursor(p, box, col, 1)
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
		p.Clip(tint, 0, func() {
			p.Fill(tint, withAlpha(col, 0.55), 0)
		})
	}
	g.letters = out
	bar := ui.Rect{X: f32(floorLeft), Y: f32(floorTop), W: f32(right - floorLeft), H: f32(uh)}
	p.Fill(bar, withAlpha(col, 0.95), f32(uh/2))
	if glow > 0.01 {
		p.Fill(bar, ui.RGBA(255, 255, 255, float32(0.6*glow)), f32(uh/2))
	}
	p.AnimationFrame()
}

// ---- Evaporate ----

type vapor struct {
	ch           string
	x, cy        float64
	size         float32
	h            float64
	t0           float64
	phase, drift float64
	delay        float64
}

type evaporate struct {
	base time.Time
	list []vapor
}

func newEvaporate() *evaporate { return &evaporate{} }

func (e *evaporate) reset() { e.list = nil }

func (e *evaporate) shift(dy float32) {
	for i := range e.list {
		e.list[i].cy += float64(dy)
	}
}

func (e *evaporate) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	if e.base.IsZero() {
		e.base = now
	}
	tn := ms(now.Sub(e.base))
	drawBoxCursor(p, boxOf(f), cursorColor(f.dark), 1)
	if k.erased > 0 && k.gone != "" {
		for i, l := range goneLetters(p, f, k) {
			e.list = append(e.list, vapor{l.ch, l.x + l.w/2, float64(f.r.Y + f.r.H/2), textSize(f), float64(f.r.H), tn, rnd() * 2 * math.Pi, (rnd() - 0.5) * 0.5, float64(i) * 16})
		}
	}
	text := p.Theme().Text
	out := e.list[:0]
	for _, v := range e.list {
		t := (tn - v.t0 - v.delay) / 1100
		if t >= 1 {
			continue
		}
		out = append(out, v)
		u := clamp01(t)
		lift := 1 - (1-u)*(1-u)
		dx := (math.Sin(v.phase+u*3.2)*0.16 + v.drift) * v.h * u
		dy := -1.2 * v.h * lift
		glyphAt(p, v.ch, f32(v.x+dx), f32(v.cy+dy), v.size, withAlpha(text, math.Pow(1-u, 1.3)), false, 0, 1+0.35*u)
	}
	e.list = out
	if len(e.list) > 0 {
		p.AnimationFrame()
	}
}

// ---- tiles ----

func backmanMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	x, y, w, h := r.X+r.W/2-6, r.Y+1.5, float32(12), r.H-3
	pts := &ui.Path{}
	pts.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h*0.35).LineTo(x+w*0.3, y+h*0.5).LineTo(x+w, y+h*0.65).LineTo(x+w, y+h).LineTo(x, y+h).Close()
	p.FillPath(pts, withAlpha(col, 0.9))
	p.Fill(ui.Rect{X: x + w*0.55, Y: y + 1.5, W: 3.4, H: 3.4}, ui.Hex("#222222"), 0.5)
}

func shredderMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	for i := 0; i < 5; i++ {
		p.Fill(ui.Rect{X: r.X + r.W/2 - 6 + float32(i%2)*1.5, Y: r.Y + 1 + float32(i)*4.2, W: 12, H: 3}, withAlpha(col, 0.9), 0)
	}
}

func portalMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 9, Y: r.Y + r.H - 5, W: 18, H: 3}, withAlpha(col, 0.95), 1.5)
	for i := 0; i < 3; i++ {
		p.Fill(ui.Rect{X: r.X + r.W/2 - 5 + float32(i)*4, Y: r.Y + 1 + float32(i)*3, W: 2.5, H: 5}, withAlpha(p.Theme().Text, 0.7-0.2*float64(i)), 0.5)
	}
}

func evaporateMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H - 12, W: 8, H: 11}, withAlpha(col, 0.9), 2)
	for i := 0; i < 4; i++ {
		p.Fill(ui.Rect{X: r.X + r.W/2 - 6 + float32(i)*3.5, Y: r.Y + 7 - float32(i)*2, W: 2, H: 2}, withAlpha(p.Theme().Text, 0.7-0.15*float64(i)), 1)
	}
}
