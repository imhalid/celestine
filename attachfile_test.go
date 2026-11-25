package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
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
		if b, err := os.ReadFile(filepath.Join(a.pane().page.Dir, name)); err != nil || string(b) != "%PDF" {
			t.Fatalf("%s is not in the page's folder: %v", name, err)
		}
	}
	a.save()
	md := a.currentMarkdown(a.pane().page)
	if !strings.Contains(md, "[plan b.pdf](<plan b.pdf>)") || !strings.Contains(md, "[plan b (2).pdf](<plan b (2).pdf>)") {
		t.Fatalf("Markdown: %q", md)
	}
	body, _ := a.pane().page.Body()
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

// TestAttachedFileStoreCopy: a file of the one-file library is opened as a copy in the user's cache, the folders
// only the user's (0700) and the copy too (0600); a name that leaves the page's folder is never opened; the copies are
// gone when the app clears them, at its start and its quit (#166).
func TestAttachedFileStoreCopy(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // os.UserCacheDir on macOS
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Notes")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Home")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.FS().WriteFile(filepath.Join(p.Dir, "secret.pdf"), []byte("%PDF-store")); err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.noSound = true
	type call struct {
		path   string
		reveal bool
	}
	opened := make(chan call, 8)
	a.shellOpen = func(path string, reveal bool) { opened <- call{path, reveal} }
	for _, bad := range []string{"", ".", "..", "../secret.pdf", "/etc/passwd"} {
		a.openAttachedFile(p, bad, false)
	}
	a.openAttachedFile(p, "secret.pdf", true)
	var got call
	select {
	case got = <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("the copy was never opened")
	}
	if !got.reveal || filepath.Base(got.path) != "secret.pdf" || !strings.HasPrefix(got.path, openedCopies()) {
		t.Fatalf("opened %+v, not a copy in %s", got, openedCopies())
	}
	if b, err := os.ReadFile(got.path); err != nil || string(b) != "%PDF-store" {
		t.Fatalf("the copy holds %q (%v)", b, err)
	}
	for _, d := range []string{openedCopies(), filepath.Dir(got.path)} {
		if info, err := os.Stat(d); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s: %v %v", d, info.Mode().Perm(), err)
		}
	}
	if info, err := os.Stat(got.path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the copy: %v %v", info.Mode().Perm(), err)
	}
	a.openAttachedFile(p, "secret.pdf", false) // opened again: the same copy, written over
	select {
	case again := <-opened:
		if again.path != got.path {
			t.Fatalf("a second copy: %s, then %s", got.path, again.path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the copy was not opened again")
	}
	select {
	case extra := <-opened:
		t.Fatalf("a name that leaves the page was opened: %+v", extra)
	default:
	}
	clearOpenedCopies()
	if _, err := os.Stat(openedCopies()); !os.IsNotExist(err) {
		t.Fatalf("the copies are still there: %v", err)
	}
}

// TestAddFileFromInvalidNameShowsNotice: a file whose name cannot be a page's file is not put in, and the notice says why.
func TestAddFileFromInvalidNameShowsNotice(t *testing.T) {
	a, _ := newTestApp(t, "Notes.\n")
	a.addFileFrom("..")
	want := locales.CommonInvalidAttachmentName("..")
	if a.notice != want || a.noticeTone == nil || *a.noticeTone != toastErr {
		t.Fatalf("notice = (%v, %q), want (%v, %q)", a.noticeTone, a.notice, toastErr, want)
	}
}
