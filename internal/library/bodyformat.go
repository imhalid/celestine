package library

// BodyFormat is how a page's body is read and made. The body is a document (doc.go), whose Markdown the library reads
// and writes as the editor does (#595); a test can give another.
type BodyFormat interface {
	Markdown(body string) string   // the body as Markdown lines: its tags, tasks and links, what to search for
	Plain(body string) string      // its text alone
	Blank(body string) bool        // it holds no text and no object
	FromMarkdown(md string) string // a body made of Markdown: what comes in from outside
}

// Format is the format of the bodies the library reads and writes: its own.
var Format BodyFormat = docFormat{}
