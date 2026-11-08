package library

import (
	"encoding/json"
	"strings"
)

// BodyFormat is how a page's body is read and made. The body is the editor's own format (body.go), which
// the app knows and the library does not: the app gives its own before it opens a library.
type BodyFormat interface {
	Markdown(body string) string   // the body as Markdown lines: its tags, tasks and links, what to search for
	Plain(body string) string      // its text alone
	Blank(body string) bool        // it holds no text and no object
	FromMarkdown(md string) string // a body made of Markdown: what comes in from outside
}

// Format is the format of the bodies the library reads and writes. Without the app's (a test of the library
// alone) a body is its Markdown, kept as a JSON string.
var Format BodyFormat = markdownFormat{}

type markdownFormat struct{}

func (markdownFormat) Markdown(body string) string {
	var md string
	if json.Unmarshal([]byte(body), &md) != nil {
		return ""
	}
	return md
}

func (f markdownFormat) Plain(body string) string { return f.Markdown(body) }

func (f markdownFormat) Blank(body string) bool { return strings.TrimSpace(f.Markdown(body)) == "" }

func (markdownFormat) FromMarkdown(md string) string {
	b, _ := json.Marshal(md)
	return string(b)
}
