package main

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
)

// Tasks in the running text (#536) are read from and written to Markdown by the library (internal/library mdpage.go,
// #595); in the editor's text each is a chip of kind "check" (listmarks.go).

// checkChips are where the tasks in the running text of t are, in their order, out of code, and whether each is done.
func checkChips(t richtext.Text) (at []int, done []bool) {
	for s, para := range t.Paragraphs() {
		if para.Style.CodeBlock {
			continue
		}
		pos := s
		for _, r := range para.Runs {
			if att := r.Attributes.Attachment; att != nil {
				if c, ok := att.Data.(*chip); ok && c.kind == "check" {
					at, done = append(at, pos), append(done, c.target == "x")
				}
			}
			pos += utf8.RuneCountInString(r.Text)
		}
	}
	return at, done
}
