package main

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
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
// Its corners follow the corner setting: at 0 it is a square bar. MyGo's bar is turned off
// (Scrollbars(false)); the bar is painted over the area (DrawOver), and a strip at the area's right
// edge, an absolute child of the area (which stays where it is as the content scrolls), takes the pointer.

const (
	funBarRest  = 4.0  // its width at rest
	funBarHover = 8.0  // under the pointer
	funBarDrag  = 10.0 // dragged
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

func (a *app) scrolledOn(c *ui.Context, key string, st *ui.ScrollState, el ui.Element, children func(), horiz bool) ui.Element {
	b := a.bar(key, st)
	b.horiz = horiz
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
	dt := dtOf(now, b.last)
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
	if w > 5 { // a gloss along its side, and two grip dots in its middle
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
