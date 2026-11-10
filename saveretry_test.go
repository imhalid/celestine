package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// hasErrToast says whether an error toast is up.
func hasErrToast(a *app) bool {
	for _, t := range a.toasts.items {
		if t.kind == toastErr {
			return true
		}
	}
	return false
}

// A failure of something else is an error toast, not the page's save error, and does not hold its autosave back
// (#216). A page left with a save that failed keeps its text, and it is saved when it can be (#129).
func TestFailedSaveKept(t *testing.T) {
	a, _ := newTestApp(t, "ilk\n")
	a.fail(errors.New("kalıcı silinemedi"))
	if a.saveErr != "" || !hasErrToast(a) {
		t.Fatalf("saveErr %q", a.saveErr)
	}
	p := a.page
	other, _ := p.Coll.NewPage(nil, "Öteki")
	a.doc.SetText(parseBody("yazıldı ama kaydedilemedi\n"))
	a.dirty = true
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	dir := p.Dir
	os.Chmod(dir, 0o500) // its folder cannot be written: the save fails
	defer os.Chmod(dir, 0o755)
	a.open(other)
	if !a.unsaved[p] {
		t.Fatal("the page whose save failed is forgotten")
	}
	for range maxDocs + 5 { // many pages opened: the unsaved text is not let go
		q, _ := p.Coll.NewPage(nil, "Sayfa")
		a.open(q)
	}
	if a.docs[p] == nil {
		t.Fatal("the unsaved text was let go")
	}
	os.Chmod(dir, 0o755)
	a.flush()
	if b, _ := p.Text(); !strings.Contains(b, "yazıldı ama kaydedilemedi") || len(a.unsaved) != 0 {
		t.Fatalf("not saved when it could be: %q", b)
	}
}
