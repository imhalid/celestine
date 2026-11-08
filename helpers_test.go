package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

// pageMarkdown is the text as plain lines, as the tests compare it.
func (a *app) pageMarkdown(p *Page, t richtext.Text) string { return bodyMarkdown(t) }

// writePageFile writes a page file by hand, as another process would.
func writePageFile(t testing.TB, path string, meta map[string]string, md string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, JoinPage(meta, encodeBody(importMarkdown(md))), 0o644); err != nil {
		t.Fatal(err)
	}
}
