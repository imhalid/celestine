package main

import (
	"runtime"
	"unicode"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// unit is how far a caret step goes.
type unit int

const (
	byChar unit = iota
	byWord      // Alt (Option) on macOS, Ctrl elsewhere
	byLine      // the visual line's edge: Cmd on macOS, Home/End
	byDoc       // the document's start or end
)

// unitOf reads the step from the modifiers of an arrow or delete key, the way
// each platform's text fields do.
func unitOf(m ui.Modifiers) unit {
	if runtime.GOOS == "darwin" {
		switch {
		case m&ui.Super != 0:
			return byLine
		case m&ui.Alt != 0:
			return byWord
		}
		return byChar
	}
	if m&ui.Ctrl != 0 {
		return byWord
	}
	return byChar
}

// step moves from the UTF-16 offset i by one unit, back (dir < 0) or forward.
func (d *doc) step(i, dir int, u unit) int {
	switch u {
	case byWord:
		return d.wordEdge(i, dir)
	case byLine:
		return d.lineEdge(i, dir)
	case byDoc:
		if dir < 0 {
			return 0
		}
		return ui.UTF16Len(d.text)
	}
	if dir < 0 {
		return d.layout.PreviousBoundary(i)
	}
	return d.layout.NextBoundary(i)
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '\'' }

// wordEdge skips what is not a word, then the word: backward to its start,
// forward to its end, as macOS's Option+arrows do. A newline stops it, so a
// word step never leaves the line it began on without first landing at its edge.
func (d *doc) wordEdge(i, dir int) int {
	u := utf16.Encode([]rune(d.text))
	at := func(k int) rune { // the rune whose code units start at k
		if k+1 < len(u) && utf16.IsSurrogate(rune(u[k])) {
			return utf16.DecodeRune(rune(u[k]), rune(u[k+1]))
		}
		return rune(u[k])
	}
	if dir > 0 {
		k := i
		if k < len(u) && at(k) == '\n' {
			return k + 1
		}
		for k < len(u) && !isWord(at(k)) && at(k) != '\n' {
			k++
		}
		for k < len(u) && isWord(at(k)) {
			k++
		}
		return k
	}
	k := i
	before := func(k int) rune {
		if k >= 2 && utf16.IsSurrogate(rune(u[k-1])) {
			return at(k - 2)
		}
		return rune(u[k-1])
	}
	back := func(k int) int {
		if k >= 2 && utf16.IsSurrogate(rune(u[k-1])) {
			return k - 2
		}
		return k - 1
	}
	if k > 0 && before(k) == '\n' {
		return k - 1
	}
	for k > 0 && !isWord(before(k)) && before(k) != '\n' {
		k = back(k)
	}
	for k > 0 && isWord(before(k)) {
		k = back(k)
	}
	return k
}

// lineEdge is the start or end of the visual line the caret is on: a wrapped
// paragraph has several, and Cmd+arrow goes to the edge of the one in view.
func (d *doc) lineEdge(i, dir int) int {
	c := d.layout.Caret(i)
	mid := c.Y + c.H/2
	for _, l := range d.layout.Lines() {
		if mid < l.Rect.Y || mid > l.Rect.Y+l.Rect.H {
			continue
		}
		if dir < 0 {
			return l.Start
		}
		end := l.End
		if s, _ := d.TextForRange(ui.TextInputRange{Start: max(end-1, l.Start), End: end}); s == "\n" || (s == " " && end < ui.UTF16Len(d.text)) {
			end-- // before the newline, or the space a soft wrap hangs at the line's end
		}
		return end
	}
	return i
}
