package main

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
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
	view    float32 // the area's height, last frame
	box     ui.Rect // the area, as it was painted
	lastY   float32
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
	b := a.bar(key, st)
	el.TrackScroll(b.st).Scrollbars(false)
	b.over = el.Hovered()
	el.DrawOver(func(p *ui.Painter, r ui.Rect) { a.paintBar(p, b, r) })
	el.Children(func() {
		children()
		a.barStrip(c, key, b)
	})
	b.view = el.LayoutBounds().H
	return el
}

// thumb is where the thumb is along the area, and how long, at rest: from the area's top.
func (b *funBar) thumb() (y, h float32) {
	track := b.view - 2*funBarInset
	content := b.view + b.st.MaxY
	if b.st.MaxY <= 0.5 || track <= 0 {
		return 0, 0
	}
	h = max(funBarMin, track*b.view/content)
	h = min(h, track)
	y = funBarInset + (track-h)*min(1, max(0, b.st.Y/b.st.MaxY))
	return y, h
}

// barStrip is the strip at the area's right edge that takes the pointer: in the content, placed as
// the area's absolute children are, against the area itself, so it stays in view as the content scrolls.
func (a *app) barStrip(c *ui.Context, key string, b *funBar) {
	if b.st.MaxY <= 0.5 || b.view <= 0 {
		b.hover, b.drag = false, false
		return
	}
	s := ui.Box(c.Key(key + ":bar")).Absolute().Right(0).Top(0).Width(funBarStrip).Height(b.view).
		Cursor(ui.CursorDefault).Label(T("Kaydırma çubuğu")).Role(ui.RoleSlider)
	b.hover = s.Hovered()
	_, py, over := s.PointerPosition()
	ty, th := b.thumb()
	_, dy, pressed := s.Dragged()
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
	if b.drag || b.hover {
		c.Invalidate()
	}
}

// jumpTo scrolls so that the thumb starts at y, from the area's top.
func (b *funBar) jumpTo(y float32) {
	_, th := b.thumb()
	travel := b.view - 2*funBarInset - th
	if travel <= 0 {
		return
	}
	b.st.Y = min(1, max(0, (y-funBarInset)/travel)) * b.st.MaxY
	b.moved = time.Now()
}

// paintBar paints the bar over the area r.
func (a *app) paintBar(p *ui.Painter, b *funBar, r ui.Rect) {
	b.box = r
	if b.st.MaxY <= 0.5 {
		return
	}
	now := p.Now()
	dt := dtOf(now, b.last)
	b.last = now
	if d := b.st.Y - b.lastY; math.Abs(float64(d)) > 0.01 {
		b.vel += (float64(d)/dt - b.vel) * 0.5
		b.moved = now
	} else {
		b.vel *= math.Exp(-dt * 14)
	}
	b.lastY = b.st.Y
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
	active := b.drag || b.hover || now.Sub(b.moved) < funBarShow
	wantA := 0.0
	switch {
	case active:
		wantA = 1
	case b.over:
		wantA = 0.55
	}
	b.alpha += (wantA - b.alpha) * (1 - math.Exp(-dt*10))
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
	// squashed as it stretches: the jelly keeps its volume
	w := float32(b.w * (1 - 0.25*clamp01(b.stretch/float64(th+1))))
	x := r.X + r.W - funBarInset - w
	y, h := r.Y+ty, th
	if b.vel > 0 { // going down: it trails above
		y -= f32(b.stretch)
	}
	h += f32(b.stretch)
	y = max(y, r.Y+funBarInset)
	h = min(h, r.Y+r.H-funBarInset-y)
	body := ui.Rect{X: x, Y: y, W: w, H: h}
	rr := rad(w / 2)
	p.FillGradient(body, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: withAlpha(mix(col, ui.RGBA(255, 255, 255, 1), 0.25*lit), 0.9*b.alpha), At: 0},
		{Color: withAlpha(col, 0.9*b.alpha), At: 1},
	}}, rr)
	if w > 5 { // a gloss down its left, and two grip dots in its middle
		p.Fill(ui.Rect{X: x + w*0.22, Y: y + 3, W: max(1, w*0.18), H: max(0, h-6)}, ui.RGBA(255, 255, 255, float32(0.3*b.alpha*lit)), rad(w*0.09))
		d := w * 0.26
		for _, k := range []float32{-1, 1} {
			p.Fill(ui.Rect{X: x + w/2 - d/2, Y: y + h/2 + k*d*1.1 - d/2, W: d, H: d}, ui.RGBA(255, 255, 255, float32(0.75*b.alpha*lit)), rad(d/2))
		}
	}
	if b.drag { // the bubble: how far down the page is
		pct := strconv.Itoa(int(math.Round(float64(b.st.Y/b.st.MaxY)*100))) + "%"
		span := ui.Span{Text: pct, Size: 11, Weight: 650, Color: ui.RGBA(255, 255, 255, 1)}
		tw, tht := p.MeasureText(0, span)
		bw, bh := tw+14, tht+6
		bx, by := x-bw-8, r.Y+ty+th/2-bh/2
		p.Fill(ui.Rect{X: bx, Y: by, W: bw, H: bh}, withAlpha(accent, 0.95*b.alpha), rad(bh/2))
		p.RichText(bx+7, by+3, tw+4, span)
	}
	if active || b.alpha > 0.01 && b.alpha < 0.99 || b.stretch > 0.3 || math.Abs(b.w-wantW) > 0.05 {
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
