package main

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"
)

// Dragging pages in the sidebar's tree. A page's row, pressed and moved a few
// DIPs, starts a drag: it dims in place and a ghost of it follows the
// pointer. Where the pointer is on a row decides where the page goes: on its
// top or bottom edge, before or after it (a line shows where); in its
// middle, under it as a sub-page (the row lights up); on a collection's row,
// to the end of that collection's top pages. Letting go moves the page, its
// folder and its sub-pages with it (Library.MovePage).
//
// The rows are all rowH high, one under another, so the row under the pointer
// and where on it are read from the pointer's place in the tree.

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
	line   float32 // the y of the line showing the place, or -1
	into   int     // the row lit up for "under it" or "into the collection", or -1
	depth  int     // how far in the line starts
}

type dragState struct {
	page    *Page
	pressed *Page   // the row pressed, and how far the pointer went since
	moved   float32 // (a drag starts once it went dragStart DIPs)
	x, y    float32 // the pointer, in the tree
	over    bool
	target  *dropTarget
	dropped int // frames left in which a row's click is the end of a drag, not a click
}

const dragStart = 4

// dragRow follows a press on a page's row r: it starts a drag once the
// pointer went far enough, and drops the page when the pointer lets go.
func (a *app) dragRow(r ui.Element, p *Page) {
	d := &a.drag
	if dx, dy, ok := r.Dragged(); ok && (dx != 0 || dy != 0) {
		if d.pressed != p {
			d.pressed, d.moved = p, 0
		}
		d.moved += float32(math.Hypot(float64(dx), float64(dy)))
		if d.page == nil && d.moved >= dragStart {
			d.page = p
		}
	}
	if _, _, ended := r.DragEnded(); ended && d.pressed == p {
		if d.page != nil {
			a.dropPage()
			d.dropped = 2
		}
		d.page, d.pressed, d.target = nil, nil, nil
	}
}

// aim works out where the dragged page would go, from the pointer.
func (a *app) aim() {
	d := &a.drag
	d.target = nil
	if d.page == nil || !d.over || len(a.rows) == 0 {
		return
	}
	i := int(d.y / rowH)
	if i < 0 || i >= len(a.rows) {
		return
	}
	frac := d.y/rowH - float32(i)
	e := a.rows[i]
	if e.coll != nil { // onto a collection: the end of its top pages
		d.target = &dropTarget{coll: e.coll, index: len(e.coll.Pages), line: -1, into: i}
		return
	}
	q := e.page
	if d.page.Contains(q) {
		return // not onto itself, nor under itself
	}
	siblings := q.Coll.Pages
	if q.Parent != nil {
		siblings = q.Parent.Children
	}
	at := slices.Index(siblings, q)
	depth := q.Depth() + 1
	switch {
	case frac < 0.3:
		d.target = &dropTarget{coll: q.Coll, parent: q.Parent, index: at, line: float32(i) * rowH, into: -1, depth: depth}
	case frac > 0.7 || !q.Coll.SubPages:
		d.target = &dropTarget{coll: q.Coll, parent: q.Parent, index: at + 1, line: float32(i+1) * rowH, into: -1, depth: depth}
	default:
		d.target = &dropTarget{coll: q.Coll, parent: q, index: len(q.Children), line: -1, into: i}
	}
}

// dropPage moves the dragged page where the pointer let it go.
func (a *app) dropPage() {
	d := &a.drag
	t := d.target
	if t == nil {
		return
	}
	if err := a.lib.MovePage(d.page, t.coll, t.parent, t.index); err != nil {
		a.saveErr = err.Error()
		return
	}
	if t.parent != nil {
		t.parent.Open = true
	}
	t.coll.Open = true
}

// drawDrag draws over the tree: the place the page would go - a line with a
// dot at its start, or a lit row - and the ghost of the row under the pointer.
func (a *app) drawDrag(p *ui.Painter, r ui.Rect) {
	a.treeR = r
	d := &a.drag
	if d.page == nil {
		return
	}
	th := p.Theme()
	if t := d.target; t != nil {
		if t.into >= 0 {
			box := ui.Rect{X: r.X + 8, Y: r.Y + float32(t.into)*rowH, W: r.W - 16, H: rowH}
			p.Fill(box, th.Accent.Alpha(0.14), 7)
			p.Stroke(box, th.Accent, 7, 1.5)
		} else {
			x := r.X + 8 + 4 + float32(t.depth)*indentW + 18
			y := r.Y + t.line
			p.Fill(ui.Rect{X: x, Y: y - 1, W: r.X + r.W - 12 - x, H: 2}, th.Accent, 1)
			p.StrokePath((&ui.Path{}).Circle(x-3, y, 3), 1.5, th.Accent)
		}
	}
	if !d.over {
		return
	}
	// The ghost: the row, raised, following the pointer down the tree and
	// staying in it across, so the sidebar's edge never cuts it.
	ghost := ui.Rect{X: r.X + 34, Y: r.Y + d.y + 8, W: r.W - 44, H: rowH} // below the pointer: the row it is over stays in sight
	p.Shadow(ghost, 8, 0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.18))
	p.Fill(ghost, th.Background.Alpha(0.86), 8) // the row under it shows through
	p.Stroke(ghost, th.Border, 8, 1)
	p.RichText(ghost.X+12, ghost.Y+7, ghost.W-24, ui.Span{Text: d.page.Title, Size: 13.5, Color: th.Text})
}
