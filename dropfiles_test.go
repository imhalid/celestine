package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDropFiles: a Markdown file dropped on the window is a page; a picture is
// the text's, or - elsewhere on a page - its cover.
func TestDropFiles(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1100, 800)
	dir := t.TempDir()
	md := filepath.Join(dir, "toplantı.md")
	os.WriteFile(md, []byte("---\ntags: [iş]\n---\n\n# Haftalık toplantı\n\nkararlar burada\n"), 0o644)
	png := filepath.Join(dir, "foto.png")
	os.WriteFile(png, pngBytes(40, 20), 0o644)
	tt.Frame()
	before := len(a.pane().page.Coll.Pages)
	// on the sidebar: not over the text
	tt.DropFiles(60, 300, md)
	tt.Advance(300 * time.Millisecond)
	if len(a.pane().page.Coll.Pages) != before+1 || a.pane().page.Title != "Haftalık toplantı" {
		t.Fatalf("pages %d, open %q", len(a.pane().page.Coll.Pages), a.pane().page.Title)
	}
	if body, _ := a.pane().page.Text(); !strings.Contains(body, "kararlar burada") || !strings.Contains(body, "#iş") {
		t.Fatalf("body %q", body)
	}
	// a picture over the text goes in the text
	r := a.pane().editorR
	tt.DropFiles(r.X+r.W/2, r.Y+r.H/2, png)
	tt.Advance(200 * time.Millisecond)
	if n, _ := pictures(a); n != 1 || a.pane().page.Cover != "" {
		t.Fatalf("pictures %d, cover %q", n, a.pane().page.Cover)
	}
	// and one elsewhere is the cover
	tt.DropFiles(60, 300, png)
	tt.Advance(200 * time.Millisecond)
	if a.pane().page.Cover == "" {
		t.Fatal("no cover")
	}
	// something else is said so
	tt.DropFiles(60, 300, filepath.Join(dir, "x.exe"))
	tt.Advance(100 * time.Millisecond)
}
