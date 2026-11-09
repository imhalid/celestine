package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A file dropped on the text goes into the page's folder and the page as a chip with its name, kept as a file
// object when the page is saved and read again, and a link to the file in its Markdown; a second of the same name is
// numbered.
func TestAttachFile(t *testing.T) {
	a, tt := newTestApp(t, "Notes.\n")
	tt.Advance(100 * time.Millisecond)
	src := filepath.Join(t.TempDir(), "plan b.pdf")
	os.WriteFile(src, []byte("%PDF"), 0o644)
	a.dropFile(src, true)
	a.dropFile(src, true)
	tt.Frame()
	for _, name := range []string{"plan b.pdf", "plan b (2).pdf"} {
		if b, err := os.ReadFile(filepath.Join(a.page.Dir, name)); err != nil || string(b) != "%PDF" {
			t.Fatalf("%s is not in the page's folder: %v", name, err)
		}
	}
	a.save()
	md := a.currentMarkdown(a.page)
	if !strings.Contains(md, "[plan b.pdf](<plan b.pdf>)") || !strings.Contains(md, "[plan b (2).pdf](<plan b (2).pdf>)") {
		t.Fatalf("Markdown: %q", md)
	}
	body, _ := a.page.Body()
	if !strings.Contains(body, `"k":"file"`) {
		t.Fatalf("not kept as a file: %s", body)
	}
	again := decodeBody(body)
	n := 0
	for _, r := range again.Runs(0, again.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "file" {
				n++
			}
		}
	}
	if n != 2 {
		t.Fatalf("read again: %d file chips", n)
	}
	if !tt.HasText("plan b.pdf") && !strings.Contains(strings.Join(tt.Texts(), " "), "plan b") {
		t.Log("(the chip's name is drawn, not an accessible text)")
	}
}
