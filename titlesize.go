package main

import "github.com/egoist/mygo/ui"

// A page's title wraps onto more lines rather than being cut at the page's edge, and steps down in size as it gets
// longer, as a heading 1 to a heading 2 and a heading 3 (#268): three lines of the largest letters would be a wall.

// titleSizes are the title's sizes, the largest first: the first is for a title that fits on one line, the second for
// one that fits on two at it, the last for any longer.
var titleSizes = [...]float32{32, 26, 22}

// titleWeight is the title's weight at every size.
const titleWeight = 720

// titleSize is the size of title text in a room width wide: the largest it fits in at its number of lines.
func (a *app) titleSize(text string, width float32) float32 {
	if width <= 0 {
		return titleSizes[0]
	}
	for i, size := range titleSizes[:len(titleSizes)-1] {
		if len(a.titleLayout(text, size, width).Lines()) <= i+1 {
			return size
		}
	}
	return titleSizes[len(titleSizes)-1]
}

// titleLayout is title text laid out at size in a room width wide (0: on one line).
func (a *app) titleLayout(text string, size, width float32) *ui.TextLayout {
	if text == "" {
		text = " " // an empty title is a line high
	}
	return ui.ShapeRichTextWith([]ui.Span{{Text: text, Size: size, Weight: titleWeight}}, a.font, ui.TextShape{Width: width})
}

// titleRoom notes the width the title was laid out in, which its size and lines are found at; when it changed, the
// title is laid out again at it. A text area (area, the title being typed) lays its lines out a pixel narrower than its
// box, its last column kept for the caret (egoist/mygo#2203): measured at the whole box, a title that just fits would
// be sized for one line less than it wraps onto, and clipped.
func (a *app) titleRoom(c *ui.Context, title ui.Element, area bool) {
	w := title.LayoutBounds().W
	if area && w > 1 {
		w--
	}
	if w > 0 && w != a.titleW {
		a.titleW = w
		c.Invalidate()
	}
}

// titleLastLine reports whether caret (in runes) in the title is on its last line: ↓ there goes into the text, and on a line
// above it to the line below.
func (a *app) titleLastLine(size float32, caret int) bool {
	l := a.titleLayout(a.title, size, a.titleW)
	lines := l.Lines()
	if len(lines) < 2 {
		return true
	}
	cr := l.CaretAt(ui.TextCaretPosition{Index: units(a.title, caret), Affinity: ui.TextDownstream})
	return cr.Y+cr.H/2 >= lines[len(lines)-1].Rect.Y
}

// titleCaretOf is where the caret is in the title's input, in runes.
func titleCaretOf(title ui.Element) int {
	_, caret := title.TextAnchorCaret()
	return caret
}
