package main

import (
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Dragging pages in the sidebar's tree, the way Ryugo's drag and drop moves
// its cards (ryu-go/dnd.go), carried over to a tree. A page's row, pressed
// and moved a few DIPs, lifts off its place: a copy of it follows the
// pointer, a little larger, leaning into the motion, and its place - with
// the sub-pages showing under it - closes. Where the pointer is on a row
// decides where the page goes: on its top or bottom edge, before or after
// it, and the rows slide apart to make room there, an inset well at the
// depth it would take; in its middle, under it as a sub-page, and the row
// glows, a cloud's shadow of the accent drifting across it; on a
// collection's row, to the end of that collection's top pages. Let go, the
// page moves (Library.MovePage, its folder and sub-pages with it) and the
// copy flies into its new place; a light runs up the sides of the row it
// went into.
//
// The rows are all rowH high, one under another, so the row under the
// pointer and where on it are read from the pointer's place in the tree,
// with the dragged rows left out: the rows below them have closed their room.

// Ryugo's springs of a drag: rows making room settle without a wobble, the
// page landing with a hint of one.
var (
	springShift = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.06}}
	springLand  = ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond, Bounce: 0.16}}
	springLift  = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.2}}
	springTilt  = ui.Animation{Spring: ui.SpringOptions{Duration: 240 * time.Millisecond}}
	fadeOver    = ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.EaseOut}
	flashRun    = ui.Animation{Duration: 1500 * time.Millisecond, Ease: ui.EaseInOut}
)

// treeEntry is a row of the tree as laid out this frame.
type treeEntry struct {
	coll *Collection
	page *Page
}

// dropTarget is where a dragged page would go.
type dropTarget struct {
	coll   *Collection
	parent *Page
	index  int
	slot   int // the room opens before this row of the tree without the dragged rows, or -1
	into   int // the row (of a.rows) that glows for "under it" or "into the collection", or -1
	depth  int // how far in the well of the room starts
}

type dragState struct {
	page    *Page // the page held
	pressed *Page // the row pressed, and how far the pointer went since
	moved   float32
	gen     int // pick-ups so far: each has its own copy

	x, y float32 // the pointer, in the tree
	over bool
	grab ui.Point // where the pointer holds the row, from its top left
	pos  ui.Point // the pointer, in window DIPs
	w    float32  // the row's width
	vx   float32  // the pointer's sideways speed, which the copy leans with

	from, n int // the dragged rows of the last frame: the first, and how many
	target  *dropTarget
	dropped int // frames left in which a row's click is the end of a drag, not a click

	// Let go: the move applies before the next frame builds the tree
	// (pending), in which the rows take their places at once (settle).
	pending *dropTarget
	settle  bool
	// The copy on its way from where it was let go (land) to the page's row
	// (dest, once that frame laid it out).
	landing *Page
	land    ui.Point
	tilt    float32
	dest    ui.Rect
	destOK  bool
	// The row the light runs up as the page lands, and whether it starts now.
	flash   any
	flashGo bool
}

const dragStart = 4

// dragRow follows a press on a page's row r: a press that went a few DIPs
// picks the page up, and letting go drops it.
func (a *app) dragRow(c *ui.Context, r ui.Element, p *Page) {
	d := &a.drag
	dx, dy, pressed := r.Dragged()
	mine := d.page == p
	switch {
	case pressed && mine:
		b := r.LayoutBounds()
		px, py, _ := r.PointerPosition()
		d.pos = ui.Point{X: b.X + px, Y: b.Y + py}
		d.vx, _ = r.DragVelocity()
	case pressed && d.page == nil && d.landing == nil:
		if d.pressed != p {
			d.pressed, d.moved = p, 0
		}
		if d.moved += abs32(dx) + abs32(dy); d.moved > dragStart {
			b := r.LayoutBounds()
			px, py, _ := r.PointerPosition()
			d.page, d.gen, d.from, d.target = p, d.gen+1, -1, nil
			d.grab, d.w = ui.Point{X: px, Y: py}, b.W
			d.pos = ui.Point{X: b.X + px, Y: b.Y + py}
			mine = true
		}
	case !pressed && d.pressed == p:
		d.pressed, d.moved = nil, 0
	}
	if mine && !pressed {
		if a.letCarry() { // let go over the panes: the Dock places it, and nothing moves in the tree
			d.page, d.pressed, d.target = nil, nil, nil
			d.dropped = 2
			return
		}
		// let go: the copy stays where it is until the page's row is laid out
		d.land = ui.Point{X: d.pos.X - d.grab.X, Y: d.pos.Y - d.grab.Y}
		d.pending, d.landing, d.destOK = d.target, p, false
		d.page, d.pressed, d.target = nil, nil, nil
		d.dropped = 2
		c.AnimationFrame()
	}
}

// settleDrop applies the drop let go in the last frame, before the tree builds.
func (a *app) settleDrop() {
	d := &a.drag
	d.settle = false
	t := d.pending
	if t == nil {
		return
	}
	d.pending, d.settle = nil, true
	p := d.landing
	if err := a.lib.MovePage(p, t.coll, t.parent, t.index); err != nil {
		a.fail(err)
		return
	}
	t.coll.Open = true
	d.flash = any(p)
	switch {
	case t.parent != nil && t.slot < 0:
		t.parent.Open = true
		d.flash = any(t.parent)
	case t.parent != nil:
		t.parent.Open = true
	case t.slot < 0:
		d.flash = any(t.coll)
	}
}

// moving is whether rows are being dragged, or a page is landing: their
// branches don't clip, the rows moving out of them.
func (a *app) moving() bool { return a.drag.page != nil || a.drag.landing != nil }

// placeRow moves row r, the i-th of the tree, to make room for the page
// held and to close the room it left; it hides the rows held, and the
// page's row until its copy landed, and lights the row the page would go
// into.
func (a *app) placeRow(c *ui.Context, r ui.Element, i int, item any) {
	d := &a.drag
	held := d.page != nil && d.from >= 0 && i >= d.from && i < d.from+d.n
	var s float32
	if d.page != nil && d.from >= 0 && !held {
		k := i
		if i >= d.from+d.n {
			s -= float32(d.n) * rowH
			k -= d.n
		}
		if t := d.target; t != nil && t.slot >= 0 && k >= t.slot {
			s += rowH
		}
	}
	if d.settle {
		r.Jump("shift", 0)
	}
	r.Translate(0, r.AnimateTo("shift", s, springShift))
	if held || (d.landing != nil && item == any(d.landing)) {
		r.Opacity(0)
	}
	if p, ok := item.(*Page); ok && d.landing == p && d.pending == nil && !d.settle {
		if b := r.LayoutBounds(); b.W > 0 {
			d.dest, d.destOK = b, true
		}
	}
	// the row it would go into glows, as Ryugo's column does
	lit := d.page != nil && d.target != nil && d.target.into == i
	over := r.AnimateTo("over", b2f(lit), fadeOver)
	t := ryugo.From(c)
	if over > 0.001 {
		cl := t.Colors
		r.Background(ryugo.Mix(cl.Well, cl.ColorAccent500, 0.08*over)).Border(1, ryugo.Mix(cl.WellBorder, cl.ColorAccent500, 0.55*over))
		cloud(t, r, over)
	}
	// as the page lands, the light runs up the sides of the row it went to
	if d.flashGo && item == d.flash {
		r.Jump("flash", 0)
		d.flashGo, d.flash = false, nil
	}
	rise(t, r, r.AnimateTo("flash", 1, flashRun))
}

// aim works out where the dragged page would go, from the pointer.
func (a *app) aim() {
	a.aimTree()
	d := &a.drag
	switch {
	case d.page != nil && a.carryDock(place{"page", d.page.Dir}, d.page.Title, iconPage, d.pos): // over the panes: the Dock's, not the tree's
		d.target = nil
	case d.page == nil && !a.cdrag.active && !a.paneDrag.active: // nor a pane carried by its bar (panebar.go)
		a.dropCarry()
	}
}

// aimTree is where the page held would go in the tree.
func (a *app) aimTree() {
	d := &a.drag
	d.target = nil
	if d.page == nil {
		return
	}
	// the dragged rows: the page and its sub-pages showing under it
	d.from, d.n = -1, 0
	for j, e := range a.rows {
		if e.page != nil && d.page.Contains(e.page) {
			if d.from < 0 {
				d.from = j
			}
			d.n++
		}
	}
	if d.from < 0 || !d.over || d.y < 0 {
		return
	}
	rest := slices.Delete(slices.Clone(a.rows), d.from, d.from+d.n)
	k := int(d.y / rowH)
	if k >= len(rest) {
		return
	}
	frac := d.y/rowH - float32(k)
	row := k // the row, among all of them
	if k >= d.from {
		row += d.n
	}
	e := rest[k]
	// the row below opens a branch of this one
	opens := k+1 < len(rest) && rest[k+1].page != nil &&
		((e.coll != nil && rest[k+1].page.Coll == e.coll && rest[k+1].page.Parent == nil) ||
			(e.page != nil && rest[k+1].page.Parent == e.page))
	if e.coll != nil {
		if frac < 0.3 && k > 0 { // the end of the collection above
			prev := rest[k-1].coll
			if prev == nil {
				prev = rest[k-1].page.Coll
			}
			d.target = &dropTarget{coll: prev, index: len(prev.Pages), slot: k, into: -1, depth: 1}
			return
		}
		if frac > 0.7 && opens { // before its first page
			d.target = &dropTarget{coll: e.coll, index: 0, slot: k + 1, into: -1, depth: 1}
			return
		}
		// onto a collection: the end of its top pages
		d.target = &dropTarget{coll: e.coll, index: len(e.coll.Pages), slot: -1, into: row}
		return
	}
	q := e.page
	siblings := q.Coll.Pages
	if q.Parent != nil {
		siblings = q.Parent.Children
	}
	at := slices.Index(siblings, q)
	depth := q.Depth() + 1
	switch {
	case frac < 0.3:
		d.target = &dropTarget{coll: q.Coll, parent: q.Parent, index: at, slot: k, into: -1, depth: depth}
	case frac > 0.7 && opens: // before its first sub-page
		d.target = &dropTarget{coll: q.Coll, parent: q, index: 0, slot: k + 1, into: -1, depth: depth + 1}
	case frac > 0.7 || !q.Coll.SubPages:
		d.target = &dropTarget{coll: q.Coll, parent: q.Parent, index: at + 1, slot: k + 1, into: -1, depth: depth}
	default:
		d.target = &dropTarget{coll: q.Coll, parent: q, index: len(q.Children), slot: -1, into: row}
	}
}

// well paints, under the rows, the inset well of the room the rows made:
// at the depth the page would take, sliding along as the room moves.
func (a *app) well(c *ui.Context, tree ui.Element) {
	d := &a.drag
	t := ryugo.From(c)
	cl := t.Colors
	on, y, x := float32(0), float32(0), float32(0)
	if tg := d.target; d.page != nil && tg != nil && tg.slot >= 0 {
		on, y, x = 1, float32(tg.slot)*rowH, float32(tg.depth)*indentW
	}
	shown := tree.AnimateTo("well-a", on, fadeOver)
	if on == 1 && shown < 0.02 { // it comes where the room opens, not from the last place
		tree.Jump("well-y", y)
		tree.Jump("well-x", x)
	}
	wy := tree.AnimateTo("well-y", y, springShift)
	wx := tree.AnimateTo("well-x", x, springShift)
	tree.Draw(func(p *ui.Painter, r ui.Rect) {
		a.treeR = r
		if shown < 0.01 {
			return
		}
		box := ui.Rect{X: r.X + 8 + wx, Y: r.Y + wy + 3, W: r.W - 16 - wx, H: rowH - 6}
		p.Fill(box, ryugo.Mix(cl.Well, cl.ColorAccent500, 0.06).Alpha(shown), rad(7))
		p.Stroke(box, ryugo.Mix(cl.WellBorder, cl.ColorAccent500, 0.45).Alpha(shown), 7, 1)
	})
}

// ghost is the copy of the row held, lifted off the tree: it follows the
// pointer, a little larger, leaning into the motion; let go, it flies to
// the page's row and settles into it.
func (a *app) ghost(c *ui.Context) {
	d := &a.drag
	p := d.page
	if p == nil {
		p = d.landing
	}
	if p == nil {
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	ui.Overlay(c, func() {
		c.Key("ghost:" + strconv.Itoa(d.gen))
		g := ui.Row(c).Absolute().Width(d.w).Height(rowH).Padding(0, 10, 0, 12).Gap(6).AlignItems(ui.Center).
			Radius(7).PassThrough().Cursor(ui.CursorGrabbing)
		picked := ui.Local(g, "picked", func() bool { return false })
		at := ui.Point{X: d.pos.X - d.grab.X, Y: d.pos.Y - d.grab.Y}
		if !*picked {
			*picked = true
			g.Jump("lift", 0)
			g.Jump("x", at.X)
			g.Jump("y", at.Y)
		}
		var x, y, lift, tilt float32
		switch {
		case d.page != nil: // held
			g.Jump("x", at.X)
			g.Jump("y", at.Y)
			x, y = at.X, at.Y
			lift = g.AnimateTo("lift", 1, springLift)
			tilt = g.AnimateTo("tilt", min(max(d.vx*0.0016, -2.5), 2.5), springTilt)
			d.tilt = tilt
		case !d.destOK: // let go: it waits where it is for the row's place
			g.Jump("x", d.land.X)
			g.Jump("y", d.land.Y)
			x, y = d.land.X, d.land.Y
			lift, tilt = g.AnimateTo("lift", 1, springLand), g.AnimateTo("tilt", d.tilt, springTilt)
		default: // and flies there
			x = g.AnimateTo("x", d.dest.X, springLand)
			y = g.AnimateTo("y", d.dest.Y, springLand)
			lift = g.AnimateTo("lift", 0, springLand)
			tilt = g.AnimateTo("tilt", 0, springLand)
			if abs32(x-d.dest.X) < 0.5 && abs32(y-d.dest.Y) < 0.5 && lift < 0.02 {
				d.landing, d.destOK, d.flashGo = nil, false, d.flash != nil
			}
			c.AnimationFrame()
		}
		s := 1 + 0.03*lift
		g.Left(x).Top(y).Scale(s, s).Rotate(tilt).
			Background(ryugo.Mix(cl.ControlBottom, cl.ControlTop, 0.5)).Border(1, cl.ControlBorder.Alpha(lift))
		if lift > 0.01 {
			t.Shadows.ShadowPop.Apply(g)
		}
		g.Children(func() {
			ui.Box(c).Width(14)
			ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted)
			ui.Text(c, p.Title).FontSize(13.5).MaxLines(1).Grow(1).TextColor(cl.Text)
		})
	})
}

// cloud paints the glow of the row a page is held over, at strength over:
// three soft blots of the accent drift across it, each along its own smooth
// noise, as Ryugo's column glows.
func cloud(t *ryugo.Tokens, e ui.Element, over float32) {
	accent := t.Colors.ColorAccent500
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		s := float64(p.Now().UnixMilli()%1_000_000) / 1000
		if stillNow() { // reduced motion: the blots stay where they are
			s = 0
		}
		for i := range 3 {
			k := float64(i)*17.3 + 3.1
			cx := 0.5 + 0.42*noise(s*0.22+k)
			cy := 0.5 + 0.45*noise(s*0.17+k+40)
			rad := max(r.W, r.H) * float32(0.42+0.12*noise(s*0.3+k+80))
			al := over * float32(0.16+0.05*noise(s*0.4+k+120))
			p.FillRadialGradient(r, ui.RadialGradient{Center: ui.Point{X: float32(cx), Y: float32(cy)}, Radius: rad, Oklab: true,
				Stops: []ui.GradientStop{{Color: accent.Alpha(al), At: 0}, {Color: accent.Alpha(al * 0.45), At: 0.45}, {Color: accent.Alpha(0), At: 1}}}, 7)
		}
		p.AnimationFrame()
	})
}

// rise paints the light that runs up a row's sides as a page lands in it,
// at f from 0 to 1: two sparks leave the middle of its bottom edge, round
// its lower corners and climb its sides, each with a fading tail, to meet
// at the top as they fade out.
func rise(t *ryugo.Tokens, e ui.Element, f float32) {
	if f <= 0 || f >= 1 {
		return
	}
	light := ryugo.Mix(t.Colors.ColorAccent400, ui.RGB(255, 255, 255), 0.35)
	e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		r = ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}
		fade := min(1, (1-f)*2.5) * min(1, f*8)
		const tail, n = 0.28, 7
		for _, left := range []bool{true, false} {
			path := halfBorder(r, 7, left)
			for j := range n {
				from := max(0, f-tail*float32(n-j)/n)
				seg := *path
				p.StrokePath(seg.Trim(from, f), 3.5, light.Alpha(0.05*fade))
				seg = *path
				p.StrokePath(seg.Trim(from, f), 1, light.Alpha(0.3*fade))
			}
		}
	})
}

// halfBorder is half of r's rounded border, from the middle of its bottom
// edge round the left (or right) side to the middle of its top edge.
func halfBorder(r ui.Rect, radius float32, left bool) *ui.Path {
	rad := min(radius, r.W/2, r.H/2)
	cx, x, sx := r.X+r.W/2, r.X, float32(1)
	if !left {
		x, sx = r.X+r.W, -1
	}
	top, bottom := r.Y, r.Y+r.H
	var p ui.Path
	p.MoveTo(cx, bottom).LineTo(x+sx*rad, bottom).
		QuadTo(x, bottom, x, bottom-rad).LineTo(x, top+rad).
		QuadTo(x, top, x+sx*rad, top).LineTo(cx, top)
	return &p
}

// noise is smooth noise in [-1, 1]: values at whole numbers, eased between them.
func noise(x float64) float64 {
	i := math.Floor(x)
	f := x - i
	u := f * f * (3 - 2*f)
	h := func(n float64) float64 {
		v := math.Sin(n*127.1+311.7) * 43758.5453
		return v - math.Floor(v)
	}
	return (h(i)*(1-u)+h(i+1)*u)*2 - 1
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
