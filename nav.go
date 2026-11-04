package main

import (
	"runtime"
	"unicode"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '\'' }

// wordEdge skips what is not a word, then the word, from UTF-16 offset i:
// back to its start (dir < 0) or on to its end. A line break stops it.
func wordEdge(text string, i, dir int) int {
	u := utf16.Encode([]rune(text))
	at := func(k int) rune {
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
	back := func(k int) int {
		if k >= 2 && utf16.IsSurrogate(rune(u[k-1])) {
			return k - 2
		}
		return k - 1
	}
	before := func(k int) rune { return at(back(k)) }
	k := i
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

// wordAt is the word around offset i, for a double click.
func wordAt(text string, i int) (int, int) {
	a, z := wordEdge(text, min(i+1, ui.UTF16Len(text)), -1), wordEdge(text, i, 1)
	if a > i {
		a = i
	}
	return a, max(z, i)
}

// unit is how far an arrow key goes with its modifiers.
type unit int

const (
	byChar unit = iota
	byWord      // Option on macOS, Ctrl elsewhere
	byLine      // the visual line's edge: Cmd on macOS
)

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
