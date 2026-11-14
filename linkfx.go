package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The text a link is being made for, and the one just made (#509). While the bar's link field is open the editor
// has given the keyboard to the field and no longer shows its selection: the text the link is for stays tinted,
// with a band of light going across it, so that it is plain which words it goes on. Once the address is set the
// words earn it: a flash of light runs over them, a ring swells out of them and fades, and a few sparks rise.

// linkWonTime is how long the flourish of a link just set lasts.
const linkWonTime = 1100 * time.Millisecond

// rangeRects are where the runes from to to of the text are drawn, in the window, in the paragraphs in view of
// the editor drawn in r.
func (a *app) rangeRects(r ui.Rect, from, to int) []ui.Rect {
	var out []ui.Rect
	if a.doc == nil || to <= from {
		return nil
	}
	for _, b := range a.shown(r) {
		s, e, para := a.paraOfBox(b)
		lo, hi := max(from, s), min(to, e)
		if hi <= lo {
			continue
		}
		var line string
		for _, run := range para.Runs {
			line += run.Text
		}
		l, left := layoutParagraph(para, r.W, a.font, a.theme)
		for _, rc := range l.SelectionRects(ui.TextInputRange{Start: units(line, lo-s), End: units(line, hi-s)}) {
			out = append(out, ui.Rect{X: r.X + left + rc.X, Y: r.Y + b.Top + rc.Y, W: rc.W, H: rc.H})
		}
	}
	return out
}

// bounds is the rectangle around rects.
func bounds(rects []ui.Rect) ui.Rect {
	b := rects[0]
	for _, rc := range rects[1:] {
		x0, y0 := min(b.X, rc.X), min(b.Y, rc.Y)
		x1, y1 := max(b.X+b.W, rc.X+rc.W), max(b.Y+b.H, rc.Y+rc.H)
		b = ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	return b
}

// slantBand is a band w wide, slanted, its middle at x, as tall as box.
func slantBand(box ui.Rect, x, w float32) *ui.Path {
	slant := box.H * 0.45
	return (&ui.Path{}).MoveTo(x-w/2+slant, box.Y).LineTo(x+w/2+slant, box.Y).
		LineTo(x+w/2-slant, box.Y+box.H).LineTo(x-w/2-slant, box.Y+box.H).Close()
}

// drawLinkPending tints the text the bar's link field is open for, a shimmer going across it.
func (a *app) drawLinkPending(p *ui.Painter, r ui.Rect) {
	b := &a.sel
	if !b.link {
		return
	}
	rects := a.rangeRects(r, b.linkSel[0], b.linkSel[1])
	if len(rects) == 0 {
		return
	}
	accent := ryugo.Painting(p).Colors.Accent
	fill := func(c ui.Color) {
		for _, rc := range rects {
			p.Fill(rc, c, rad(3))
		}
	}
	fill(accent.Alpha(0.2))
	if stillNow() {
		return
	}
	box := bounds(rects)
	const period = 1.5
	k := float32(math.Mod(float64(p.Now().UnixMilli())/1000/period, 1))
	x := box.X - 90 + (box.W+180)*k
	for _, w := range []float32{90, 50, 20} { // soft at its edges: the narrower over the wider
		p.ClipPath(slantBand(box, x, w), func() { fill(accent.Alpha(0.13)) })
	}
	p.AnimationFrame()
}

// drawLinkWon draws the flourish over the text of a link just set.
func (a *app) drawLinkWon(p *ui.Painter, r ui.Rect) {
	b := &a.sel
	if b.wonAt.IsZero() {
		return
	}
	t := p.Now().Sub(b.wonAt)
	if t >= linkWonTime || stillNow() {
		b.wonAt = time.Time{}
		return
	}
	p.AnimationFrame()
	rects := a.rangeRects(r, b.wonSel[0], b.wonSel[1])
	if len(rects) == 0 {
		return
	}
	accent := ryugo.Painting(p).Colors.Accent
	gold := ui.Hex("#f5c451")
	white := ui.Hex("#ffffff")
	box := bounds(rects)
	sec := float32(t.Seconds())
	ease := func(k float32) float32 { k = min(max(k, 0), 1); return 1 - (1-k)*(1-k)*(1-k) }

	// the glow the words take, which fades
	if k := sec / 0.7; k < 1 {
		glow := 1 - ease(k)
		for _, rc := range rects {
			p.Shadow(rc, rad(4), 0, 0, 10, 0, accent.Alpha(0.35*glow))
			p.Fill(rc, accent.Alpha(0.26*glow), rad(3))
		}
	}
	// the ring swelling out of them
	if k := sec / 0.65; k < 1 {
		e := ease(k)
		for _, rc := range rects {
			g := 2 + 7*e
			ring := ui.Rect{X: rc.X - g, Y: rc.Y - g*0.6, W: rc.W + 2*g, H: rc.H + 1.2*g}
			p.Stroke(ring, accent.Alpha(0.55*(1-e)), rad(5)+g/2, 1.5)
		}
	}
	// the flash of light running over them, left to right
	if k := (sec - 0.05) / 0.55; k > 0 && k < 1 {
		x := box.X - 40 + (box.W+80)*ease(k)
		fill := func(c ui.Color) {
			for _, rc := range rects {
				p.Fill(rc, c, rad(3))
			}
		}
		for _, w := range []float32{46, 24, 10} {
			p.ClipPath(slantBand(box, x, w), func() { fill(white.Alpha(0.22)) })
		}
	}
	// the sparks: four-pointed stars rising from the words, turning, swelling and gone
	const sparks = 7
	for i := range sparks {
		f := float32(i) / sparks
		start := 0.08 + 0.05*float32(i%3)
		k := (sec - start) / 0.8
		if k <= 0 || k >= 1 {
			continue
		}
		x := box.X + box.W*(0.08+0.84*float32(math.Mod(float64(f)*1.618+0.13, 1)))
		y := box.Y + box.H*0.5 - (14+12*float32(i%2))*ease(k)
		size := (2.5 + 2*float32((i*5)%3)) * float32(math.Sin(float64(k)*math.Pi))
		col := gold
		if i%3 == 1 {
			col = accent
		}
		p.FillPath(star(x+float32(i%2*2-1)*4*k, y, size, k*1.4), col.Alpha(1-k*k))
	}
}

// star is a four-pointed star of radius s about (x, y), turned by turn radians.
func star(x, y, s, turn float32) *ui.Path {
	path := &ui.Path{}
	for i := range 8 {
		ang := float64(turn) + float64(i)*math.Pi/4
		rr := s
		if i%2 == 1 {
			rr = s * 0.3
		}
		px, py := x+rr*float32(math.Sin(ang)), y-rr*float32(math.Cos(ang))
		if i == 0 {
			path.MoveTo(px, py)
		} else {
			path.LineTo(px, py)
		}
	}
	return path.Close()
}
