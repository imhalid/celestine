package main

import (
	"sort"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// The editor is as tall as its text (editorHeight), however long, and the page
// scrolls as a whole, so a.boxes holds every paragraph - on a long page
// thousands, most of them out of view. What is drawn
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

// paraTable is the paragraphs of a text, each with the rune it starts at, read in one pass. A paragraph found
// by its rune is read from the start of the text, so asking it for each paragraph in view, every frame, made a
// frame of a long page cost as much as its length times its paragraphs (#312); the table is read once for each
// version of the text and its paragraphs are found by the index a box carries.
type paraTable struct {
	text  richtext.Text
	start []int
	paras []richtext.Paragraph
}

// paraTableOf is the table of t, read again only when t is another text.
func (a *app) paraTableOf(t richtext.Text) *paraTable {
	pt := &a.paras
	if pt.start != nil && pt.text.Same(t) {
		return pt
	}
	pt.text, pt.start, pt.paras = t, pt.start[:0], pt.paras[:0]
	for s, p := range t.Paragraphs() {
		pt.start = append(pt.start, s)
		pt.paras = append(pt.paras, p)
	}
	return pt
}

// paraOfBox is the paragraph of box b in the document's text: where it starts and ends (before its newline),
// its style and its runs. A box laid out before the text last changed may not be its paragraph any more: then
// it is read by its rune, as it was before the table.
func (a *app) paraOfBox(b richtext.ParagraphBox) (s, e int, para richtext.Paragraph) {
	t := a.doc.Text()
	pt := a.paraTableOf(t)
	if i := b.Index; i >= 0 && i < len(pt.start) && pt.start[i] == b.Start {
		para = pt.paras[i]
		s, e = b.Start, b.Start
		for _, r := range para.Runs {
			e += utf8.RuneCountInString(r.Text)
		}
		return s, e, para
	}
	s, e = t.ParagraphRange(min(b.Start, t.Len()))
	for _, pp := range t.Slice(s, e).Paragraphs() {
		para = pp
		break
	}
	para.Style = t.ParagraphStyleAt(s)
	return s, e, para
}

// edgeFades lays a soft fade over each edge of a sideways scroll that has more
// beyond it - at the right while there is more to the right, at the left once it
// is scrolled - so that what is cut at the edge reads as going on, not as cut.
// They let the pointer through. s is the scroll's state (TrackScroll). Each starts
// at the edge in the page's own background, so that nothing marks where it begins,
// turns to it with a little of the theme's accent, and fades out: the cards going
// under it take the theme's color, not a dark shadow's.
func (a *app) edgeFades(c *ui.Context, s *ui.ScrollState) {
	const w = 60
	bg := a.bg(c)
	tint := bg
	if acc := c.Theme().Accent; acc != (ui.Color{}) {
		tint = mix(bg, acc, 0.16)
	}
	stops := []ui.GradientStop{{Color: bg, At: 0}, {Color: tint.Alpha(0.85), At: 0.3}, {Color: tint.Alpha(0), At: 1}}
	for _, side := range []struct {
		key  string
		on   bool
		left bool
	}{{"fade-left", s.X > 1, true}, {"fade-right", s.X < s.MaxX-1, false}} {
		e := ui.Box(c.Key(side.key)).Absolute().Top(0).Bottom(0).Width(w).PassThrough()
		if side.left {
			e.Left(0).LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true, Stops: stops})
		} else {
			e.Right(0).LinearGradient(ui.LinearGradient{Angle: 270, Oklab: true, Stops: stops})
		}
		e.Opacity(e.AnimateTo("o", map[bool]float32{true: 1, false: 0}[side.on], fadeQuick))
	}
}

// locateRune is locateAt for the rune at in the page's editor, its paragraph from the text's table: the caret is
// found every frame, and on a long page reading its paragraph from the start of the text was a part of each (#314).
func (a *app) locateRune(at int, w float32) (caretBox, bool) {
	boxes := a.boxes
	for i, b := range boxes {
		if b.Start <= at && (i+1 == len(boxes) || at < boxes[i+1].Start) {
			s, e, para := a.paraOfBox(b)
			return locateIn(a.doc, at, b, s, e, para, w, a.font, a.theme)
		}
	}
	return caretBox{}, false
}

// chipInText reports whether ch is an object of the page's text, from the text's table: a table out of view is not
// drawn, and is told apart this way from one taken away.
func (a *app) chipInText(ch *chip) bool {
	if ch == nil || a.doc == nil {
		return false
	}
	pt := a.paraTableOf(a.doc.Text())
	for _, p := range pt.paras {
		for _, r := range p.Runs {
			if at := r.Attributes.Attachment; at != nil && at.Data == ch {
				return true
			}
		}
	}
	return false
}
