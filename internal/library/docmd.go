package library

import (
	"encoding/json"
	"strings"
)

// The library's own format of the bodies (Format): page.json's document, its Markdown read and written as the editor
// does (mdpage.go, mdobjects.go), so that the library reads and writes pages alone as the app does (#595). A body that
// is a JSON string is read as its Markdown, as the library's tests once wrote them.

// docFormat is the library's format of the bodies.
type docFormat struct{}

func (docFormat) Markdown(body string) string {
	if d, err := DecodeBody(body); err == nil {
		return d.ExportMarkdown()
	}
	var md string
	if json.Unmarshal([]byte(body), &md) == nil {
		return md
	}
	return ""
}

func (f docFormat) Plain(body string) string {
	if d, err := DecodeBody(body); err == nil {
		return d.Plain()
	}
	return f.Markdown(body)
}

func (f docFormat) Blank(body string) bool {
	if d, err := DecodeBody(body); err == nil {
		return d.Blank()
	}
	return strings.TrimSpace(f.Markdown(body)) == ""
}

func (docFormat) FromMarkdown(md string) string { return EncodeBody(ImportMarkdown(md)) }
