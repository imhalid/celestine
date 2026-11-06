package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// touchLater writes file with content, dated a moment later than now, as
// another program would: two writes in one clock tick look like one.
func touchLater(t *testing.T, file, content string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	os.Chtimes(file, later, later)
}

// TestWatchOwnWrites: what the app writes itself is no change from outside.
func TestWatchOwnWrites(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	page, lib := a.page, *a.lib
	tt.Type("x")
	tt.Advance(time.Second) // saved
	a.page.Coll.NewPage(nil, "İkinci")
	a.checkDisk()
	if a.page != page || len(a.lib.Collections) != len(lib.Collections) || a.lib.Collections[0] != lib.Collections[0] || a.notice != "" {
		t.Fatal("the app's own writes were taken as changes from outside")
	}
}

// TestWatchTextChanged: a page changed outside shows its new text, and its
// new title; the open page too, when nothing is typed in it.
func TestWatchTextChanged(t *testing.T) {
	a, tt := newTestApp(t, "eski")
	p := a.page
	touchLater(t, p.file(), string(joinPage(map[string]string{"title": "Yeni ad"}, encodeBody(importMarkdown("Dışarıdan yazıldı.\n")))))
	a.checkDisk()
	tt.Frame()
	if got := a.doc.Text().String(); got != "Dışarıdan yazıldı." || p.Title != "Yeni ad" || a.title != "Yeni ad" {
		t.Fatalf("text %q, title %q", got, p.Title)
	}
	tt.Advance(2 * time.Second) // nothing saved over it
	if body, _ := p.Text(); body != "Dışarıdan yazıldı.\n" {
		t.Fatalf("on disk: %q", body)
	}
}

// TestWatchConflict: a page changed outside while edits typed in it are not
// yet saved keeps both.
func TestWatchConflict(t *testing.T) {
	a, tt := newTestApp(t, "ortak")
	p := a.page
	tt.Type("benim ")
	tt.Frame()
	if !a.dirty {
		t.Fatal("nothing unsaved")
	}
	touchLater(t, p.file(), string(joinPage(map[string]string{"title": "Sayfa"}, encodeBody(importMarkdown("onların\n")))))
	a.checkDisk()
	if body, _ := p.Text(); body != "ortakbenim \n" && !strings.Contains(body, "benim") {
		t.Fatalf("ours lost: %q", body)
	}
	matches, _ := filepath.Glob(filepath.Join(p.Dir, "page (çakışma *).json"))
	if len(matches) != 1 {
		t.Fatalf("no conflict copy: %v", matches)
	}
	if theirs, _ := os.ReadFile(matches[0]); !strings.Contains(string(theirs), "onların") {
		t.Fatalf("theirs: %q", theirs)
	}
	if a.notice == "" {
		t.Fatal("not told")
	}
	a.notice = ""
	a.checkDisk() // and settled: no conflict again
	if a.notice != "" {
		t.Fatal("a conflict again")
	}
}

// TestWatchPagesCome: pages made and removed outside come and go in the
// tree; the open page and its document stay.
func TestWatchPagesCome(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	page, doc := a.page, a.doc
	coll := page.Coll
	dir := filepath.Join(coll.Dir, "disaridan")
	os.MkdirAll(dir, 0o755)
	writePageFile(t, filepath.Join(dir, pageFileName), map[string]string{"title": "Dışarıdan"}, "merhaba")
	a.checkDisk()
	tt.Frame()
	if !tt.HasText("Dışarıdan") {
		t.Fatalf("the page from outside is not in the tree: %v", tt.Texts())
	}
	if a.page == nil || a.page.Dir != page.Dir || a.doc != doc {
		t.Fatal("the open page was lost")
	}
	os.RemoveAll(dir)
	a.checkDisk()
	tt.Frame()
	if tt.HasText("Dışarıdan") {
		t.Fatal("a page removed outside is still in the tree")
	}
}

// TestWatchOpenDeleted: the open page deleted outside with edits not yet
// saved is written back.
func TestWatchOpenDeleted(t *testing.T) {
	a, tt := newTestApp(t, "metin")
	p := a.page
	tt.Type("!")
	tt.Frame()
	os.RemoveAll(p.Dir)
	a.checkDisk()
	if body, err := p.Text(); err != nil || !strings.Contains(body, "metin") {
		t.Fatalf("not written back: %q %v", body, err)
	}
	a.checkDisk()
	if a.page == nil || a.page.Dir != p.Dir {
		t.Fatal("the page is not open")
	}
}
