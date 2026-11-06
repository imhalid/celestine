package main

import (
	"github.com/egoist/mygo/ui"
)

// The caret takes the look of the text it is in: struck through, it is cut in two with nothing
// between; inside a bracket it takes the bracket's shape - round in ( ), square in [ ], cut at its
// corners in { }.

type caretStyle struct {
	strike  bool
	bracket rune // the innermost bracket it is inside: ( [ { or 0
}

const (
	caretGap = 0.2 // of its height, the gap in a struck caret
)

var bracketPairs = map[rune]rune{')': '(', ']': '[', '}': '{'}

// styleAt is the style of the caret at caret: what the next letter written would be, and the bracket it is in.
func (a *app) styleAt(caret int) caretStyle {
	at := a.doc.TypingAttributes()
	st := caretStyle{strike: at.Strikethrough}
	text := a.doc.Text()
	s, _ := text.ParagraphRange(caret)
	if caret > s {
		var open []rune
		for _, r := range text.Slice(s, caret).String() {
			switch r {
			case '(', '[', '{':
				open = append(open, r)
			case ')', ']', '}':
				if n := len(open); n > 0 && open[n-1] == bracketPairs[r] {
					open = open[:n-1]
				}
			}
		}
		if n := len(open); n > 0 {
			st.bracket = open[n-1]
		}
	}
	return st
}

// bracketRadius is the corner of a block in the bracket: round, or square.
func bracketRadius(box ui.Rect, bracket rune) float32 {
	switch bracket {
	case '(':
		return min(box.W, box.H) / 2
	case '[':
		return 0.6
	}
	return boxRadius(box.W, box.H)
}

// shapeBody is the path of a block in its bracket: the rounded box, or cut at the corners in { }.
func shapeBody(box ui.Rect, bracket rune) *ui.Path {
	if bracket == '{' {
		c := min(box.W, box.H) * 0.32
		x, y, w, h := box.X, box.Y, box.W, box.H
		p := &ui.Path{}
		return p.MoveTo(x+c, y).LineTo(x+w-c, y).LineTo(x+w, y+c).LineTo(x+w, y+h-c).LineTo(x+w-c, y+h).LineTo(x+c, y+h).LineTo(x, y+h-c).LineTo(x, y+c).Close()
	}
	return rrect(box.X, box.Y, box.W, box.H, bracketRadius(box, bracket))
}

// styledDraw is draw, cut in two, with the middle left out,
// when it is struck through.
func styledDraw(p *ui.Painter, f caretFrame, box ui.Rect, draw func()) {
	st := f.style
	run := draw
	if st.strike {
		inner := run
		run = func() {
			gap := box.H * caretGap
			half := (box.H - gap) / 2
			p.Clip(ui.Rect{X: box.X - 30, Y: box.Y - 30, W: box.W + 60, H: half + 30}, 0, inner)
			p.Clip(ui.Rect{X: box.X - 30, Y: box.Y + half + gap, W: box.W + 60, H: half + 30}, 0, inner)
		}
	}
	run()
}
