package main

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's own scroll bar, for every scroll area, in place of MyGo's (whose thumb is always a pill
// and does nothing): a jelly, as the carets are.
//
//	at rest        a thin bar in the muted color, which fades away a while after the last scroll
//	scrolling      it lights up in the accent, and the faster the page goes the more it stretches
//	               back toward where it came from, on a spring, and snaps back as it stops
//	under pointer  it widens, and shows two grip dots; a click on the track jumps there
//	dragged        wider still, with a bubble beside it saying how far down the page is
//
// An area that scrolls sideways (the board) has the same bar along its bottom (scrolledSideways).
//
// Where an area's top or bottom right corner is a rounded corner of its pane's card, the track bends with it
// (curvedTrack): its end bends a little way round the corner, as the corner does, rather than standing straight out
// over it.
//
// Its corners follow the corner setting: at 0 it is a square bar. MyGo's bar is turned off
// (Scrollbars(false)); the bar is painted over the area (DrawOver), and a strip at the area's right
// edge, an absolute child of the area (which stays where it is as the content scrolls), takes the pointer.

const (
	funBarRest  = 4.0  // its width at rest
	funBarHover = 6.0  // under the pointer
	funBarDrag  = 7.0  // dragged
	funBarStrip = 14.0 // the strip that takes the pointer
	funBarInset = 3.0  // from the area's edge
	funBarMin   = 28.0 // the shortest the thumb gets
	funBarShow  = 1100 * time.Millisecond
)

// funBar is the state of one area's bar.
type funBar struct {
	st      *ui.ScrollState // the area's scroll: its own, or one the view keeps
	own     ui.ScrollState
	horiz   bool    // it scrolls sideways: the bar runs along the bottom
	view    float32 // the area's length along the bar (its height, or its width), last frame
	box     ui.Rect // the area, as it was painted
	lastOff float32
	vel     float64 // how fast it scrolls, px a second, smoothed
	stretch float64 // how far the thumb stretches back, px
	w       float64 // its width now, on its way to what the state asks
	alpha   float64
	moved   time.Time // when it last scrolled
	last    time.Time
	hover   bool // the pointer is on the strip
	over    bool // the pointer is over the area
	drag    bool
	grab    float32 // where on the thumb it was taken
	cardR   float32 // the radius of the corners of the pane's card, 0 where the panes are not cards
}

// bar is the bar of the area named key, made the first time.
func (a *app) bar(key string, st *ui.ScrollState) *funBar {
	if a.bars == nil {
		a.bars = map[string]*funBar{}
	}
	key = a.ps.key(a.ps.cur) + "/" + key // each pane its own
	b := a.bars[key]
	if b == nil {
		b = &funBar{w: funBarRest}
		a.bars[key] = b
	}
	if st == nil {
		st = &b.own
	}
	b.st = st
	return b
}

// scrolled builds the scroll area el (ui.Scroll, sized as the view wants it) with children, under
// the name key, with the app's bar. st is the scroll the view keeps, or nil.
func (a *app) scrolled(c *ui.Context, key string, st *ui.ScrollState, el ui.Element, children func()) ui.Element {
	return a.scrolledOn(c, key, st, el, children, false)
}

// scrolledSideways is scrolled for an area that scrolls sideways (ui.ScrollHorizontal): the bar runs
// along its bottom.
func (a *app) scrolledSideways(c *ui.Context, key string, st *ui.ScrollState, el ui.Element, children func()) ui.Element {
	return a.scrolledOn(c, key, st, el, children, true)
}

// scrolledList is scrolled for a ui.List, whose rows are its children: list builds the list in wrap, a box
// it fills, and the strip that takes the pointer stands over it there.
func (a *app) scrolledList(c *ui.Context, key string, st *ui.ScrollState, wrap ui.Element, list func() ui.Element) ui.Element {
	b := a.bar(key, st)
	b.horiz, b.cardR = false, a.cardRadius(c)
	wrap.Children(func() {
		el := list().TrackScroll(b.st).Scrollbars(false)
		b.over = el.Hovered()
		el.DrawOver(func(p *ui.Painter, r ui.Rect) { a.paintBar(p, b, r) })
		b.view = el.LayoutBounds().H
		a.barStrip(c, key, b)
	})
	return wrap
}

func (a *app) scrolledOn(c *ui.Context, key string, st *ui.ScrollState, el ui.Element, children func(), horiz bool) ui.Element {
	b := a.bar(key, st)
	b.horiz, b.cardR = horiz, a.cardRadius(c)
	el.TrackScroll(b.st).Scrollbars(false)
	b.over = el.Hovered()
	el.DrawOver(func(p *ui.Painter, r ui.Rect) { a.paintBar(p, b, r) })
	el.Children(func() {
		children()
		a.barStrip(c, key, b)
	})
	if r := el.LayoutBounds(); horiz {
		b.view = r.W
	} else {
		b.view = r.H
	}
	return el
}

// off and reach are how far the area is scrolled along the bar, and how far it goes.
func (b *funBar) off() float32 {
	if b.horiz {
		return b.st.X
	}
	return b.st.Y
}

func (b *funBar) reach() float32 {
	if b.horiz {
		return b.st.MaxX
	}
	return b.st.MaxY
}

// thumb is where the thumb is along the area, and how long, at rest: from the area's top (or left).
func (b *funBar) thumb() (y, h float32) {
	track := b.view - 2*funBarInset
	content := b.view + b.reach()
	if b.reach() <= 0.5 || track <= 0 {
		return 0, 0
	}
	h = max(funBarMin, track*b.view/content)
	h = min(h, track)
	y = funBarInset + (track-h)*min(1, max(0, b.off()/b.reach()))
	return y, h
}

// barStrip is the strip at the area's right edge (or its bottom) that takes the pointer: in the content,
// placed as the area's absolute children are, against the area itself, so it stays in view as the content
// scrolls.
func (a *app) barStrip(c *ui.Context, key string, b *funBar) {
	if b.reach() <= 0.5 || b.view <= 0 {
		b.hover, b.drag = false, false
		return
	}
	s := ui.Box(c.Key(key + ":bar")).Absolute().Cursor(ui.CursorDefault).Label(locales.ThemesScrollBar()).Role(ui.RoleSlider)
	if b.horiz {
		s.Left(0).Bottom(0).Height(funBarStrip).Width(b.view)
	} else {
		s.Right(0).Top(0).Width(funBarStrip).Height(b.view)
	}
	b.hover = s.Hovered()
	px, py, over := s.PointerPosition()
	dx, dy, pressed := s.Dragged()
	if b.horiz { // along the bar
		py, dy = px, dx
	}
	ty, th := b.thumb()
	switch {
	case pressed && !b.drag: // taken: on the thumb, it is held where it was taken; on the track, it jumps there
		b.drag = true
		if over && (py < ty || py > ty+th) {
			b.grab = th / 2
			b.jumpTo(py - b.grab)
		} else {
			b.grab = py - ty
		}
	case pressed:
		if dy != 0 {
			b.jumpTo(ty + dy)
		}
	default:
		b.drag = false
	}
	if b.drag { // the view follows the thumb; hovering, the paint does the rest (#189)
		c.Invalidate()
	}
}

// jumpTo scrolls so that the thumb starts at y, from the area's top (or left).
func (b *funBar) jumpTo(y float32) {
	_, th := b.thumb()
	travel := b.view - 2*funBarInset - th
	if travel <= 0 {
		return
	}
	v := min(1, max(0, (y-funBarInset)/travel)) * b.reach()
	if b.horiz {
		b.st.X = v
	} else {
		b.st.Y = v
	}
	b.moved = time.Now()
}

// paintBar paints the bar over the area r.
func (a *app) paintBar(p *ui.Painter, b *funBar, r ui.Rect) {
	b.box = r
	if b.reach() <= 0.5 {
		return
	}
	now := p.Now()
	dt := max(dtOf(now, b.last), 0.001) // painted twice in a frame: no time went by, and the speed is not infinite
	b.last = now
	if d := b.off() - b.lastOff; math.Abs(float64(d)) > 0.01 {
		b.vel += (float64(d)/dt - b.vel) * 0.5
		b.moved = now
	} else {
		b.vel *= math.Exp(-dt * 14)
	}
	b.lastOff = b.off()
	ty, th := b.thumb()
	// the stretch: back toward where it came from, as long as the speed asks, on a spring
	want := clampf(math.Abs(b.vel)/1800, 0, 1) * float64(th) * 0.7
	if b.drag { // in the hand it stays more itself
		want *= 0.35
	}
	b.stretch += (want - b.stretch) * (1 - math.Exp(-dt*18))
	wantW := funBarRest
	switch {
	case b.drag:
		wantW = funBarDrag
	case b.hover:
		wantW = funBarHover
	}
	b.w += (wantW - b.w) * (1 - math.Exp(-dt*20))
	if math.Abs(wantW-b.w) < 0.05 { // there: a value that only comes closer asks for frames for ever (#189)
		b.w = wantW
	}
	active := b.drag || b.hover || now.Sub(b.moved) < funBarShow
	wantA := 0.0
	switch {
	case active:
		wantA = 1
	case b.over:
		wantA = 0.55
	}
	b.alpha += (wantA - b.alpha) * (1 - math.Exp(-dt*10))
	if math.Abs(wantA-b.alpha) < 0.01 {
		b.alpha = wantA
	}
	if b.alpha < 0.01 && !active && !b.over {
		b.alpha = 0
		return
	}
	muted := p.Theme().TextMuted
	accent := a.barAccent(p)
	lit := clamp01(1 - float64(now.Sub(b.moved))/float64(funBarShow))
	if b.drag || b.hover {
		lit = 1
	}
	col := mix(muted, accent, lit)
	if !b.horiz {
		if top, bottom := a.cardCorners(r, b.cardR); top > 0 || bottom > 0 {
			a.paintCurvedBar(p, b, r, curvedTrack(r, top, bottom), col, lit, accent, wantA, wantW)
			return
		}
	}
	// along the bar and across it: where it starts and how long, how thick; then laid on the area
	start, along := r.Y, r.H
	if b.horiz {
		start, along = r.X, r.W
	}
	// squashed as it stretches: the jelly keeps its volume
	w := float32(b.w * (1 - 0.25*clamp01(b.stretch/float64(th+1))))
	y, h := start+ty, th
	if b.vel > 0 { // going down (or right): it trails behind
		y -= f32(b.stretch)
	}
	h += f32(b.stretch)
	y = max(y, start+funBarInset)
	h = min(h, start+along-funBarInset-y)
	body := ui.Rect{X: r.X + r.W - funBarInset - w, Y: y, W: w, H: h}
	if b.horiz {
		body = ui.Rect{X: y, Y: r.Y + r.H - funBarInset - w, W: h, H: w}
	}
	rr := rad(w / 2)
	p.FillGradient(body, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: withAlpha(mix(col, ui.RGBA(255, 255, 255, 1), 0.25*lit), 0.9*b.alpha), At: 0},
		{Color: withAlpha(col, 0.9*b.alpha), At: 1},
	}}, rr)
	if w > 6.5 { // a gloss along its side, and two grip dots in its middle
		glint, dot := ui.RGBA(255, 255, 255, float32(0.3*b.alpha*lit)), ui.RGBA(255, 255, 255, float32(0.75*b.alpha*lit))
		d := w * 0.26
		if b.horiz {
			p.Fill(ui.Rect{X: body.X + 3, Y: body.Y + w*0.22, W: max(0, h-6), H: max(1, w*0.18)}, glint, rad(w*0.09))
			for _, k := range []float32{-1, 1} {
				p.Fill(ui.Rect{X: body.X + h/2 + k*d*1.1 - d/2, Y: body.Y + w/2 - d/2, W: d, H: d}, dot, rad(d/2))
			}
		} else {
			p.Fill(ui.Rect{X: body.X + w*0.22, Y: body.Y + 3, W: max(1, w*0.18), H: max(0, h-6)}, glint, rad(w*0.09))
			for _, k := range []float32{-1, 1} {
				p.Fill(ui.Rect{X: body.X + w/2 - d/2, Y: body.Y + h/2 + k*d*1.1 - d/2, W: d, H: d}, dot, rad(d/2))
			}
		}
	}
	if b.drag { // the bubble: how far along it is, beside the thumb (above it, sideways)
		pct := strconv.Itoa(int(math.Round(float64(b.off()/b.reach())*100))) + "%"
		span := ui.Span{Text: pct, Size: 11, Weight: 650, Color: ui.RGBA(255, 255, 255, 1)}
		tw, tht := p.MeasureText(0, span)
		bw, bh := tw+14, tht+6
		bx, by := body.X-bw-8, start+ty+th/2-bh/2
		if b.horiz {
			bx, by = start+ty+th/2-bw/2, body.Y-bh-8
		}
		p.Fill(ui.Rect{X: bx, Y: by, W: bw, H: bh}, withAlpha(accent, 0.95*b.alpha), rad(bh/2))
		p.RichText(bx+7, by+3, tw+4, span)
	}
	// a frame while something moves: its light fading after a scroll, its width, its alpha, its stretch; resting
	// under the pointer, lit, it is drawn as it is and asks for none
	fading := !b.drag && !b.hover && now.Sub(b.moved) < funBarShow
	if b.drag || fading || b.alpha != wantA || b.w != wantW || b.stretch > 0.3 {
		p.AnimationFrame()
	}
}

// barAccent is the accent the bar lights up in.
func (a *app) barAccent(p *ui.Painter) ui.Color {
	if t := p.Theme(); t != nil && t.Accent != (ui.Color{}) {
		return t.Accent
	}
	return ui.Hex("#3b82f6")
}

// cardRadius is the radius of the corners of the pane's card: the window's frame makes the panes cards, a lone pane
// one of the frame's and the Dock's areas its own.
func (a *app) cardRadius(c *ui.Context) float32 {
	switch {
	case !a.wframe.on:
		return 0
	case a.ps.layout == nil:
		return rad(a.cardR())
	}
	return ryugo.From(c).R(ryugo.RadiusControl)
}

// cardCorners are the radii of area r's top and bottom right corners where they are a pane card's, of radius R:
// where r's right edge is the card's, and its top (or its bottom) too. Zero where they are not.
func (a *app) cardCorners(r ui.Rect, R float32) (top, bottom float32) {
	if R <= 0 {
		return 0, 0
	}
	near := func(x, y float32) bool { return math.Abs(float64(x-y)) <= 1.5 }
	for _, pr := range a.ps.rects {
		if !near(r.X+r.W, pr.X+pr.W) || r.Y < pr.Y-1.5 || r.Y+r.H > pr.Y+pr.H+1.5 {
			continue
		}
		if near(r.Y, pr.Y) {
			top = R
		}
		if near(r.Y+r.H, pr.Y+pr.H) {
			bottom = R
		}
	}
	return top, bottom
}

// funBarLine is how far the line the curved thumb runs along is from the area's edges: its middle, so that it
// widens to both sides and still keeps off the edge at its widest.
const funBarLine = funBarInset + funBarDrag/2

// funBarBend is how far round a corner the track bends, in radians: enough that its end follows the corner, not so
// far that it runs along the edge.
const funBarBend = 0.7

// track is the path a bar runs along, in the area: a little way round the top corner's arc, down the right side,
// and a little way round the bottom corner's; a corner of radius 0 leaves its arc out. Each part is a stretch of
// its length, in DIPs.
type track struct {
	x, top, bottom float32 // the right side's line, and where the line starts and ends going down
	rt, rb         float32 // the arcs' radii, 0 where there is none
	arcT, mid      float32 // the lengths of the top arc and the side
	arcB           float32
}

// curvedTrack is the track of a bar in area r whose top and bottom right corners are rounded by top and bottom
// (0 for a square one): the arcs are as round as the corners, moved in to the bar's line, which keeps the widest
// thumb inside the corner (its outer edge is R+w/2-√2·funBarLine from the corner's middle, less than R).
func curvedTrack(r ui.Rect, top, bottom float32) track {
	t := track{x: r.X + r.W - funBarLine}
	radius := func(R float32) float32 {
		if R < 2 {
			return 0
		}
		return R
	}
	t.rt, t.rb = radius(top), radius(bottom)
	t.top, t.bottom = r.Y+funBarInset, r.Y+r.H-funBarInset
	if t.rt > 0 {
		t.top = r.Y + funBarLine
	}
	if t.rb > 0 {
		t.bottom = r.Y + r.H - funBarLine
	}
	t.arcT, t.arcB = funBarBend*t.rt, funBarBend*t.rb
	t.mid = max(0, (t.bottom-t.rb)-(t.top+t.rt))
	return t
}

// length is the track's whole length.
func (t track) length() float32 { return t.arcT + t.mid + t.arcB }

// topArc and bottomArc are the points of the corners' circles at angle a: 0 is the side, the top's going up
// toward the top edge and the bottom's down toward the bottom edge.
func (t track) topArc(a float64) (x, y float32) {
	return t.x - t.rt + t.rt*float32(math.Cos(a)), t.top + t.rt - t.rt*float32(math.Sin(a))
}

func (t track) bottomArc(a float64) (x, y float32) {
	return t.x - t.rb + t.rb*float32(math.Cos(a)), t.bottom - t.rb + t.rb*float32(math.Sin(a))
}

// at is the point s along the track.
func (t track) at(s float32) (x, y float32) {
	s = min(max(s, 0), t.length())
	switch {
	case s < t.arcT:
		return t.topArc(funBarBend - float64(s/t.rt))
	case s <= t.arcT+t.mid:
		return t.x, t.top + t.rt + (s - t.arcT)
	}
	return t.bottomArc(float64((s - t.arcT - t.mid) / t.rb))
}

// path is the track as a path to stroke: its arcs as cubic curves, each control point along the circle's tangent
// by 4/3·tan(bend/4) of the radius, as an arc that short is drawn.
func (t track) path() *ui.Path {
	k := float32(4.0 / 3 * math.Tan(funBarBend/4))
	p := new(ui.Path)
	if t.rt > 0 {
		x0, y0 := t.topArc(funBarBend)
		s, c := float32(math.Sin(funBarBend)), float32(math.Cos(funBarBend))
		p.MoveTo(x0, y0).CubeTo(x0+k*t.rt*s, y0+k*t.rt*c, t.x, t.top+t.rt-k*t.rt, t.x, t.top+t.rt)
	} else {
		p.MoveTo(t.x, t.top)
	}
	p.LineTo(t.x, t.bottom-t.rb)
	if t.rb > 0 {
		x1, y1 := t.bottomArc(funBarBend)
		s, c := float32(math.Sin(funBarBend)), float32(math.Cos(funBarBend))
		p.CubeTo(t.x, t.bottom-t.rb+k*t.rb, x1+k*t.rb*s, y1-k*t.rb*c, x1, y1)
	}
	return p
}

// paintCurvedBar paints bar b along track tr, in area r: the thumb's part of the track, as long as the area's part
// of the content is of all of it, and where the scroll is along it; stretched back as the straight one is, in col.
func (a *app) paintCurvedBar(p *ui.Painter, b *funBar, r ui.Rect, tr track, col ui.Color, lit float64, accent ui.Color, wantA, wantW float64) {
	L := tr.length()
	if L <= 0 {
		return
	}
	content := b.view + b.reach()
	th := min(L, max(funBarMin, L*b.view/content))
	from := (L - th) * min(1, max(0, b.off()/b.reach()))
	st := f32(min(b.stretch, float64(th)))
	if b.vel > 0 { // going down: it trails behind
		from -= st
	}
	to := min(L, from+th+st)
	from = max(0, from)
	w := float32(b.w * (1 - 0.25*clamp01(b.stretch/float64(th+1))))
	path := tr.path().Trim(from/L, to/L)
	_, y0 := tr.at(from)
	_, y1 := tr.at(to)
	p.StrokePathGradient(path, w, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: withAlpha(mix(col, ui.RGBA(255, 255, 255, 1), 0.25*lit), 0.9*b.alpha), At: 0},
		{Color: withAlpha(col, 0.9*b.alpha), At: 1},
	}})
	if w > 6.5 { // two grip dots in its middle
		mx, my := tr.at((from + to) / 2)
		dot := ui.RGBA(255, 255, 255, float32(0.75*b.alpha*lit))
		d := w * 0.26
		_, my2 := tr.at((from+to)/2 + 1) // along the track there, the dots one above the other or side by side
		dx, dy := float32(0), float32(1)
		if math.Abs(float64(my2-my)) < 0.5 {
			dx, dy = 1, 0
		}
		for _, k := range []float32{-1, 1} {
			p.Fill(ui.Rect{X: mx + k*d*1.1*dx - d/2, Y: my + k*d*1.1*dy - d/2, W: d, H: d}, dot, rad(d/2))
		}
	}
	if b.drag { // the bubble, beside the thumb's middle
		pct := strconv.Itoa(int(math.Round(float64(b.off()/b.reach())*100))) + "%"
		span := ui.Span{Text: pct, Size: 11, Weight: 650, Color: ui.RGBA(255, 255, 255, 1)}
		tw, tht := p.MeasureText(0, span)
		bw, bh := tw+14, tht+6
		bx, by := tr.x-w/2-bw-8, (y0+y1)/2-bh/2
		p.Fill(ui.Rect{X: bx, Y: by, W: bw, H: bh}, withAlpha(accent, 0.95*b.alpha), rad(bh/2))
		p.RichText(bx+7, by+3, tw+4, span)
	}
	fading := !b.drag && !b.hover && p.Now().Sub(b.moved) < funBarShow
	if b.drag || fading || b.alpha != wantA || b.w != wantW || b.stretch > 0.3 {
		p.AnimationFrame()
	}
}
