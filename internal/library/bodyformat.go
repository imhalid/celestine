package library

// BodyFormat is how a page's body is read and made. The body is a document (doc.go); the app's editor reads and
// writes its Markdown more fully than the library does (docmd.go), and gives its own before it opens a library.
type BodyFormat interface {
	Markdown(body string) string   // the body as Markdown lines: its tags, tasks and links, what to search for
	Plain(body string) string      // its text alone
	Blank(body string) bool        // it holds no text and no object
	FromMarkdown(md string) string // a body made of Markdown: what comes in from outside
}

// Format is the format of the bodies the library reads and writes: its own (docFormat) until the app gives the
// editor's (#595).
var Format BodyFormat = docFormat{}
