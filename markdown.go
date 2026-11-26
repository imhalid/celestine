package main

import (
	"slices"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/locales"
)

// A page's Markdown is the library's (internal/library mdpage.go, #595): it reads and writes it on its document,
// holding what Markdown's codec would lose (a table, HTML, math, a callout, a code block's language), the kinds of
// tasks, the tasks in the running text and lists' looks. The app reads it into the editor's text (textOf) and writes
// it from it (pageDoc), its chips but a task in the running text their text by then.

// The library's names for what Markdown has no style for, and its lines, by the names the app has used.
const (
	codeIDPrefix = library.CodeIDPrefix // a code block's first paragraph: its language after it
	rawIDPrefix  = library.RawIDPrefix  // a raw block's first paragraph: its kind after it
)

var (
	delimRow     = library.DelimRow
	callout      = library.Callout
	listItemLine = library.ListItemLine
	taskBoxLine  = library.TaskBoxLine
	refDefX      = library.RefDefX
	listMarkLine = library.ListMarkLine
)

// listMarkStart starts the comment over a list that says its look.
const listMarkStart = textnorm.ListMarkStart

// rawNames are what a raw block is called over it, in the editor.
var rawNames = map[string]func() string{"table": locales.BlocksRawTable, "html": func() string { return "HTML" }, "math": locales.BlocksRawMath,
	"footnote": locales.BlocksRawFootnote, "callout": locales.BlocksRawCallout}

// parseBody reads a page's Markdown into the editor's text.
func parseBody(md string) richtext.Text { return textOf(library.PageFromMarkdown(md)) }

// bodyMarkdown writes the editor's text as the page's Markdown.
func bodyMarkdown(t richtext.Text) string { return pageDoc(t).PageMarkdown() }

// rawBlock reports whether a block Markdown's codec would lose starts at line i, and its lines (library.RawBlockOf).
func rawBlock(lines []string, i int, paraStart bool) (string, int) {
	return library.RawBlockOf(lines, i, paraStart, true)
}

// inlineTaskSpots are the tasks in the running text of a page's Markdown, in their order (library.InlineTaskSpots).
func inlineTaskSpots(md string) []library.TaskSpot { return library.InlineTaskSpots(md) }

// inlineBoxes, inCode and isWord are the library's.
var (
	inlineBoxes = library.InlineBoxes
	inCode      = library.InCode
	isWord      = library.IsWord
)

// starts is where t's paragraphs start, last first: to change them from
// the end, the places of those before staying put.
func starts(t richtext.Text) []int {
	var out []int
	for at := range t.Paragraphs() {
		out = append(out, at)
	}
	slices.Reverse(out) // put each first, it was a copy of all the others: 46 ms at 10 000 paragraphs (#198)
	return out
}
