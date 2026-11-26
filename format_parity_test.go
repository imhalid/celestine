package main

import (
	"strings"
	"testing"

	"github.com/imhalid/celestine/internal/library"
)

// The library reads and writes bodies alone (library.Format, its own) as the editor does: the body the editor makes
// of every Markdown of the corpus, and the Markdown, plain text and blankness it reads of each body, are the
// library's (#595). The app no longer gives the library a format of its own.
func TestLibraryFormatIsTheEditors(t *testing.T) {
	lib := library.Format
	bad := 0
	check := func(what, md, w, g string) {
		if w != g {
			if bad++; bad <= 8 {
				t.Errorf("%s of %q:\neditor  %q\nlibrary %q", what, md, w, g)
			}
		}
	}
	for _, md := range pageCorpus(t, 2000) {
		body := encodeBody(importMarkdown(md))
		check("body", md, body, lib.FromMarkdown(md))
		check("markdown", md, bodyText(body), lib.Markdown(body))
		check("plain", md, decodeBody(body).PlainText(), lib.Plain(body))
		blank := strings.TrimSpace(strings.ReplaceAll(decodeBody(body).String(), "\n", "")) == ""
		if blank != lib.Blank(body) {
			check("blank", md, "editor", "library")
		}
	}
	if bad > 0 {
		t.Errorf("%d differences", bad)
	}
}
