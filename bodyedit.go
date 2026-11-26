package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
)

// The edits made to a page from outside its editor (a task ticked in the Tasks view, a quick note, a tag renamed)
// change its body where they must, and leave the rest as it is: writing the page again from its Markdown loses what
// Markdown cannot say, a task's kind, an alignment, empty lines, a table's filter, a file chip (#123).

// paragraphsOf are the paragraphs of t, their styles and IDs with them.
func paragraphsOf(t richtext.Text) []richtext.Paragraph {
	var out []richtext.Paragraph
	for _, p := range t.Paragraphs() {
		out = append(out, p)
	}
	return out
}

// appendBody is body with the paragraphs of add after its own; an empty body is replaced.
func appendBody(body string, add richtext.Text) string {
	if library.Format.Blank(body) {
		return encodeBody(add)
	}
	return encodeBody(richtext.TextOf(append(paragraphsOf(decodeBody(body)), paragraphsOf(add)...)...))
}

// renameTagInBody is body with tag key written as newName wherever it is typed, outside code, and how many times.
func renameTagInBody(body, key, newName string) (string, int) {
	n := 0
	paras := paragraphsOf(decodeBody(body))
	for i, p := range paras {
		if p.Style.CodeBlock {
			continue
		}
		runs := make([]richtext.Run, len(p.Runs))
		copy(runs, p.Runs)
		for j, r := range runs {
			if r.Attributes.Code || r.Attributes.Attachment != nil || !strings.Contains(r.Text, "#") {
				continue
			}
			runs[j].Text = tagWord.ReplaceAllStringFunc(r.Text, func(m string) string {
				sm := tagWord.FindStringSubmatch(m)
				if tagKey(sm[2]) != key {
					return m
				}
				n++
				return sm[1] + "#" + newName
			})
		}
		paras[i].Runs = runs
	}
	if n == 0 {
		return body, 0
	}
	return encodeBody(richtext.TextOf(paras...)), n
}
