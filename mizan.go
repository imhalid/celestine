package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// mizanCaret is Caret FX's Mizan, ported: writing as a dashboard, in Mizan's
// own language. The caret is a small raised control in the selection gradient;
// while you write, a Stat floats over it on Mizan's frosted glass - a live
// sparkline of your pace over the last keys, the pace in words a minute, and a
// change badge against the start of the run - with a border beam running round
// its edge, quicker the faster you write. It follows the caret on Mizan's ease
// and fades when you stop. When the caret jumps (a click, the arrows, a new
// line) a raised pill glides there on Mizan's spring, as Segmented's does.
type mizanCaret struct {
	pace         []float64   // words a minute, smoothed, one per key
	keys         []time.Time // the last keys, for the pace
	base         float64     // the run's opening pace, for the badge; 0 until known
	lastAt       time.Time   // the last key of the run
	openAt       time.Time   // when the Stat opened
	glide        *mizanGlide
	x, y         float64 // where the Stat is, chasing where it is wanted
	placed       bool
	beam         float64 // the beam's place round the edge, 0..1
	prev         ui.Rect
	seen, moving bool
	last         time.Time
}

// shift moves the caret with the content (scrolling) without a glide.
func (m *mizanCaret) shift(dy float32) {
	m.prev.Y += dy
	if m.glide != nil {
		m.glide.from.Y += dy
	}
}

type mizanGlide struct {
	from ui.Rect
	t0   time.Time
}

const (
	mizanPoints   = 28 // the sparkline's length, in keys
	mizanHold     = 1100 * time.Millisecond
	mizanFade     = 450 * time.Millisecond
	mizanIn       = 180 * time.Millisecond
	mizanGlideDur = 250 * time.Millisecond // Segmented's pill: 0.25s var(--spring)
	mizanBeam     = 4 * time.Second        // border-beam's --beam-duration; up to four times as fast at speed
)

var (
	mizanSpring = cubicBezier(0.3, 1.2, 0.4, 1) // --spring: overshoots a little, then settles
	mizanEase   = cubicBezier(0.2, 0.8, 0.2, 1) // --ease
	mizanTop    = ui.Hex("#2d8afc")
	mizanBottom = ui.Hex("#0371e0")
)

// No backdrop blur here (the web Stat is frosted), so the glass is nearly opaque.
// Mizan's tokens for the Stat, by theme: the glass, its edge, text, muted text,
// and the change colors (a faster pace is good).
type mizanTokens struct{ pop, edgeTop, edgeBottom, text, muted, good, bad, shadow ui.Color }

var (
	mizanDark = mizanTokens{
		pop: ui.RGBA(44, 44, 47, 0.97), edgeTop: ui.RGBA(255, 255, 255, 0.16), edgeBottom: ui.RGBA(255, 255, 255, 0.03),
		text: ui.Hex("#f5f5f7"), muted: ui.Hex("#a1a1a6"), good: ui.Hex("#4cd97b"), bad: ui.Hex("#ff6b5e"), shadow: ui.RGBA(0, 0, 0, 0.55),
	}
	mizanLight = mizanTokens{
		pop: ui.RGBA(255, 255, 255, 0.98), edgeTop: ui.RGBA(0, 0, 0, 0.10), edgeBottom: ui.RGBA(0, 0, 0, 0.04),
		text: ui.Hex("#1d1d1f"), muted: ui.Hex("#6e6e73"), good: ui.Hex("#1d7d44"), bad: ui.Hex("#c63527"), shadow: ui.RGBA(0, 0, 0, 0.16),
	}
)

// key is a letter written at now: the pace (five letters a word) over the
// last three seconds of keys goes on the line.
func (m *mizanCaret) key(now time.Time) {
	if now.Sub(m.lastAt) > mizanHold+mizanFade { // a new run
		m.openAt, m.pace, m.keys, m.base, m.placed = now, nil, nil, 0, false
	}
	m.lastAt = now
	m.keys = append(m.keys, now)
	for len(m.keys) > 0 && now.Sub(m.keys[0]) > 3*time.Second {
		m.keys = m.keys[1:]
	}
	if len(m.keys) < 3 { // two keys are no pace yet
		return
	}
	span := now.Sub(m.keys[0]).Seconds()
	if span <= 0 {
		return
	}
	wpm := float64(len(m.keys)-1) / 5 * 60 / span
	prev := wpm
	if len(m.pace) > 0 {
		prev = m.pace[len(m.pace)-1]
	}
	m.pace = append(m.pace, prev+(wpm-prev)*0.35) // smoothed, as a chart of it would be
	if len(m.pace) > mizanPoints {
		m.pace = m.pace[1:]
	}
	if m.base == 0 && len(m.pace) >= 4 {
		m.base = m.pace[len(m.pace)-1]
	}
}

func (m *mizanCaret) draw(p *ui.Painter, f caretFrame) {
	now := p.Now()
	dt := 16.0
	if !m.last.IsZero() {
		dt = math.Min(48, float64(now.Sub(m.last).Microseconds())/1000)
	}
	m.last = now
	r := f.r
	switch {
	case f.wrote > 0:
		for i := 0; i < f.wrote; i++ {
			m.key(now)
		}
	case f.edited:
		m.lastAt = now // Backspace, Enter: still writing, the Stat stays up
	case m.seen && r != m.prev && math.Hypot(float64(r.X-m.prev.X), float64(r.Y-m.prev.Y)) > float64(r.H)*0.5:
		m.glide = &mizanGlide{from: m.prev, t0: now}
	}
	m.prev, m.seen = r, true
	tk := mizanLight
	if f.dark {
		tk = mizanDark
	}
	idle := now.Sub(m.lastAt)
	busy := false

	// The pill gliding from where the caret was: raised, the selection
	// gradient, a sheen on top and the field's ring round it.
	if g := m.glide; g != nil {
		u := float64(now.Sub(g.t0)) / float64(mizanGlideDur)
		if u >= 1 {
			m.glide = nil
		} else {
			busy = true
			s := mizanSpring(u)
			w := boxOf(f).W
			h := lerpf(g.from.H, r.H, s)
			x := lerpf(g.from.X, r.X, s)
			y := lerpf(g.from.Y, r.Y, s)
			a := 1.0
			if u > 0.7 {
				a = 1 - (u-0.7)/0.3
			}
			ring := pill(x-3, y-2, w+6, h+4)
			p.StrokePath(ring, 4, withAlpha(mizanTop, 0.26*a))
			glassFill(p, pill(x, y, w, h), mizanTop, mizanBottom, f.dark, a) // glass, as the caret is
		}
	}

	// The caret: glass, as the slime is - see-through, lit from above, its top's
	// color melting into its bottom's, a rim and a glint. It stays as it is while
	// you write and breathes slowly when you stop.
	breathing := false
	if m.glide == nil {
		a := 1.0
		// it breathes a while when you stop, then stands as it is: the window at rest (#188)
		if idle > 600*time.Millisecond && idle < mizanBreathes {
			a = 0.55 + 0.45*(0.5+0.5*math.Cos(float64(idle-600*time.Millisecond)/float64(time.Second)*math.Pi*2/1.1))
			breathing = true
		}
		c := boxOf(f) // a block, as the other carets are
		styledDraw(p, f, c, func() {
			glowAround(p, c, mizanTop, 0.7*a)
			glassFill(p, restBody(f, c), mizanTop, mizanBottom, f.dark, a) // the jelly's shape, when it stretches it
			glassGloss(p, c, f.dark, a)
		})
	}

	// The Stat, while there is a pace and you have not stopped for long.
	if fade := float64(idle-mizanHold) / float64(mizanFade); fade < 1 && len(m.pace) > 0 {
		busy = true
		show := mizanEase(float64(now.Sub(m.openAt))/float64(mizanIn)) * (1 - easeInOutSine(clamp(fade, 0, 1)))
		m.stat(p, f, tk, show, dt)
	}
	switch {
	case busy:
		p.AnimationFrame()
	case breathing:
		p.AnimationFrameRate(30)
	case idle <= 600*time.Millisecond: // the breath begins then
		p.After(600*time.Millisecond - idle + time.Millisecond)
	}
}

// mizanBreathes is how long the mizan breathes once you stop writing.
const mizanBreathes = time.Minute

// stat draws the Stat over the caret: glass, edge, beam, sparkline, the pace and its badge.
func (m *mizanCaret) stat(p *ui.Painter, f caretFrame, tk mizanTokens, show, dt float64) {
	r := f.r
	wpm := math.Round(m.pace[len(m.pace)-1])
	change := 0.0
	if m.base > 0 {
		change = (wpm - m.base) / m.base
	}
	tone, badge := tk.muted, ""
	if m.base > 0 {
		switch {
		case math.Abs(change) < 0.03:
			badge = "■ 0%"
		case change > 0:
			tone, badge = tk.good, fmt.Sprintf("▲ %d%%", int(math.Round(change*100)))
		default:
			tone, badge = tk.bad, fmt.Sprintf("▼ %d%%", int(math.Round(-change*100)))
		}
	}
	num := ui.Span{Text: fmt.Sprint(wpm), Size: 12, Weight: 600, Color: tk.text, Features: "tnum"}
	unit := ui.Span{Text: "wpm", Size: 11, Weight: 500, Color: tk.muted}
	bdg := ui.Span{Text: badge, Size: 10.5, Weight: 600, Color: tone, Features: "tnum"}
	numW, numH := p.MeasureText(0, num)
	unitW, unitH := p.MeasureText(0, unit)
	badgeW, badgeH := float32(0), float32(0)
	if badge != "" {
		badgeW, badgeH = p.MeasureText(0, bdg)
		badgeW += 12
	}
	const H, sparkW, pad, gap = 26, 46, 9, 7
	W := float32(pad + sparkW + gap + numW + 3 + unitW)
	if badge != "" {
		W += gap + badgeW + 5
	} else {
		W += pad
	}
	// Over the caret, a little to its left; under it when there is no room
	// above; always inside the editor.
	b := f.bounds
	tx, ty := float64(r.X-W*0.35), float64(r.Y-H-8)
	if ty < float64(b.Y)+4 {
		ty = float64(r.Y + r.H + 8)
	}
	tx = clamp(tx, float64(b.X)+6, float64(b.X+b.W-W-6))
	if !m.placed {
		m.x, m.y, m.placed = tx, ty, true // a new run starts where it is wanted
	}
	follow := 1 - math.Pow(1-0.22, dt/16) // Mizan's ease, a frame at a time
	m.x += (tx - m.x) * follow
	m.y += (ty - m.y) * follow
	x, y := float32(math.Round(m.x)), float32(math.Round(m.y+(1-show)*4)) // it rises 4px as it opens, like a tooltip
	a := show

	box := ui.Rect{X: x, Y: y, W: W, H: H}
	p.Shadow(box, min(rad(H/2), H/2), 0, 6, 18, 0, withAlpha(tk.shadow, a))
	p.Fill(box, withAlpha(f.bg, a), min(rad(H/2), H/2)) // nothing of the text shows through
	p.Fill(box, withAlpha(tk.pop, a), min(rad(H/2), H/2))
	edge := pill(x+0.5, y+0.5, W-1, H-1)
	p.StrokePathGradient(edge, 1, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{{Color: withAlpha(tk.edgeTop, a), At: 0}, {Color: withAlpha(tk.edgeBottom, a), At: 1}}})

	// The border beam: a lit stretch of the edge running round it, faster with the pace.
	m.beam = math.Mod(m.beam+dt/float64(mizanBeam.Milliseconds())*(1+math.Min(3, wpm/40)), 1)
	head := mizanTop
	if f.dark {
		head = mix(mizanTop, ui.RGBA(255, 255, 255, 1), 0.55)
	}
	p.StrokePathConicGradient(edge, 1.2, ui.ConicGradient{Angle: float32(m.beam * 360), Stops: []ui.GradientStop{
		{Color: withAlpha(mizanTop, 0), At: 0},
		{Color: withAlpha(mizanTop, 0.45*a), At: 0.06},
		{Color: withAlpha(head, 0.95*a), At: 0.1},
		{Color: withAlpha(mizanTop, 0.45*a), At: 0.14},
		{Color: withAlpha(mizanTop, 0), At: 0.2},
		{Color: withAlpha(mizanTop, 0), At: 1},
	}})

	cx := x + pad
	m.spark(p, cx, y+7, sparkW, H-15, tk, a)
	cx += sparkW + gap
	mid := y + H/2 + 0.5
	p.RichText(cx, mid-numH/2, 0, withSpanAlpha(num, a))
	cx += numW + 3
	p.RichText(cx, mid-unitH/2, 0, withSpanAlpha(unit, a))
	cx += unitW + gap
	if badge != "" { // Badge: its color at 10-16% under it, a pill
		bg := 0.12
		if f.dark {
			bg = 0.16
		}
		p.Fill(ui.Rect{X: cx, Y: y + 5, W: badgeW, H: H - 10}, withAlpha(tone, bg*a), rad((H-10)/2))
		p.RichText(cx+6, mid-badgeH/2, 0, withSpanAlpha(bdg, a))
	}
}

// spark is the Stat's sparkline in the box (x, y, w, h): the grid dots under
// it, the area fading down, the 1.6px line and a dot on the last value
// (Sparkline.tsx).
func (m *mizanCaret) spark(p *ui.Painter, x, y, w, h float32, tk mizanTokens, a float64) {
	for gx := x; gx <= x+w; gx += 4 {
		p.FillPath((&ui.Path{}).Circle(gx, y+h+0.5, 0.5), withAlpha(tk.muted, 0.35*a))
	}
	v := m.pace
	if len(v) < 2 {
		return
	}
	lo, hi := v[0], v[0]
	for _, q := range v {
		lo, hi = math.Min(lo, q), math.Max(hi, q)
	}
	rng, mid := math.Max(hi-lo, 12), (lo+hi)/2
	step := w / (mizanPoints - 1)
	px := func(i int) float32 { return x + float32(i+mizanPoints-len(v))*step }
	py := func(q float64) float32 { return y + h/2 - float32((q-mid)/rng)*h*0.9 }
	line := &ui.Path{}
	area := &ui.Path{}
	for i, q := range v {
		if i == 0 {
			line.MoveTo(px(i), py(q))
			area.MoveTo(px(i), py(q))
		} else {
			line.LineTo(px(i), py(q))
			area.LineTo(px(i), py(q))
		}
	}
	lx, ly := px(len(v)-1), py(v[len(v)-1])
	area.LineTo(lx, y+h).LineTo(px(0), y+h).Close()
	p.FillPathGradient(area, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{{Color: withAlpha(mizanTop, 0.27*a), At: 0}, {Color: withAlpha(mizanTop, 0.015*a), At: 1}}})
	p.StrokePath(line.Join(ui.RoundJoin).Cap(ui.RoundCap), 1.6, withAlpha(mizanTop, a))
	p.FillPath((&ui.Path{}).Circle(lx, ly, 2.2), withAlpha(mizanTop, a))
}

// pill is a rounded rectangle whose ends are half circles, as round as the corners of the app say:
// square at 0, never more than half circles.
func pill(x, y, w, h float32) *ui.Path {
	half := min(w, h) / 2
	r := min(rad(half), half)
	const k = 0.5523 // a quarter circle as a cubic
	pt := &ui.Path{}
	pt.MoveTo(x+r, y).LineTo(x+w-r, y)
	pt.CubeTo(x+w-r+r*k, y, x+w, y+r-r*k, x+w, y+r)
	pt.LineTo(x+w, y+h-r)
	pt.CubeTo(x+w, y+h-r+r*k, x+w-r+r*k, y+h, x+w-r, y+h)
	pt.LineTo(x+r, y+h)
	pt.CubeTo(x+r-r*k, y+h, x, y+h-r+r*k, x, y+h-r)
	pt.LineTo(x, y+r)
	pt.CubeTo(x, y+r-r*k, x+r-r*k, y, x+r, y)
	return pt.Close()
}

// cubicBezier is CSS's cubic-bezier() as a function of time: x is solved for
// the curve's parameter (Newton), y is returned.
func cubicBezier(x1, y1, x2, y2 float64) func(float64) float64 {
	bx := func(t float64) float64 { return 3*x1*t*(1-t)*(1-t) + 3*x2*t*t*(1-t) + t*t*t }
	by := func(t float64) float64 { return 3*y1*t*(1-t)*(1-t) + 3*y2*t*t*(1-t) + t*t*t }
	dbx := func(t float64) float64 { return 3*x1*(1-t)*(1-t) + 6*(x2-x1)*t*(1-t) + 3*(1-x2)*t*t }
	return func(x float64) float64 {
		if x <= 0 {
			return 0
		}
		if x >= 1 {
			return 1
		}
		t := x
		for i := 0; i < 8; i++ {
			d := dbx(t)
			if math.Abs(d) < 1e-6 {
				break
			}
			t = clamp(t-(bx(t)-x)/d, 0, 1)
		}
		return by(t)
	}
}

func easeInOutSine(x float64) float64 { return -(math.Cos(math.Pi*x) - 1) / 2 }

func lerpf(a, b float32, t float64) float32 { return a + (b-a)*float32(t) }

func withSpanAlpha(s ui.Span, a float64) ui.Span {
	s.Color = withAlpha(s.Color, a)
	return s
}
