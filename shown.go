package main

import (
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// The editor is as tall as its text laid out so far (editorHeight), and the
// page scrolls as a whole, so a.boxes holds every paragraph laid out - on a
// long page thousands, a frame or two of them below the fold. What is drawn
// over the editor (dividers, tags, tables, matches) needs only those the page
// shows, and each of them costs several reads of the text: shown is these.

// shown is the boxes of a.boxes in the page's view, and a margin of half a
// screen to each side, for r, the editor's rectangle in the window.
func (a *app) shown(r ui.Rect) []richtext.ParagraphBox {
	n := len(a.boxes)
	if n == 0 || a.scrollR.H <= 0 {
		return a.boxes
	}
	margin := a.scrollR.H/2 + 200
	lo, hi := a.scrollR.Y-margin-r.Y, a.scrollR.Y+a.scrollR.H+margin-r.Y // in the editor's own y
	first := sort.Search(n, func(i int) bool { return a.boxes[i].Top+a.boxes[i].Height >= lo })
	last := first
	for last < n && a.boxes[last].Top <= hi {
		last++
	}
	return a.boxes[first:last]
}

// hugePageRunes and hugePageParagraphs are the length of a page, in runes and in
// paragraphs, over either of which the editor is a
// window as high as the page's room and scrolls the text in it. Under it the
// editor is as tall as its text (editorHeight) and the page scrolls as a whole,
// the title and the properties with it; but every paragraph in the editor's
// height is laid out and painted each frame, and its height grows by the room
// beyond the last paragraph laid out until the whole text is in - on a page of
// thousands of lines, seconds a frame. A document also begins with its caret at
// the end, which a window shorter than the text scrolls itself to: a huge page
// opens with the caret at its start.
const hugePageRunes, hugePageParagraphs = 60000, 800

// hugePage reports whether t is long by either measure; both are known without reading it.
func hugePage(t richtext.Text) bool {
	return t.Len() > hugePageRunes || t.ParagraphCount() > hugePageParagraphs
}

// windowEditor moves the editor of a huge page up as the title above it folds
// away, and down as it comes back.
var windowEditor = ui.ElementTransition{Animation: branch.Animation, Position: true}

// trackWindow notes whether a huge page, which scrolls in its window, is
// scrolled from the top: then its title and properties give way (pageBody)
// and the text has the whole room, as in a page that scrolls as a whole they
// would scroll away; and they come back when the text is at the top again.
func (a *app) trackWindow() {
	if !hugePage(a.doc.Text()) || len(a.boxes) == 0 {
		a.winScrolled = false
		return
	}
	b := a.boxes[0]
	scrolled := b.Index > 0
	if !scrolled {
		a.winTop = max(a.winTop, b.Top)
		scrolled = b.Top < a.winTop-1
	}
	if scrolled != a.winScrolled {
		a.winScrolled, a.winFlip = scrolled, a.clock()
	}
}

// edgeFades lays a soft fade over each edge of a sideways scroll that has more
// beyond it - at the right while there is more to the right, at the left once it
// is scrolled - so that what is cut at the edge reads as going on, not as cut.
// They let the pointer through. s is the scroll's state (TrackScroll); the page's
// own background is what they fade to.
func (a *app) edgeFades(c *ui.Context, s *ui.ScrollState) {
	const w = 60
	bg := a.bg(c)
	for _, side := range []struct {
		key  string
		on   bool
		left bool
	}{{"fade-left", s.X > 1, true}, {"fade-right", s.X < s.MaxX-1, false}} {
		e := ui.Box(c.Key(side.key)).Absolute().Top(0).Bottom(0).Width(w).PassThrough()
		if side.left {
			e.Left(0)
			e.LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true, From: bg, To: bg.Alpha(0)})
		} else {
			e.Right(0)
			e.LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true, From: bg.Alpha(0), To: bg})
		}
		e.Opacity(e.AnimateTo("o", map[bool]float32{true: 1, false: 0}[side.on], fadeQuick))
	}
}
