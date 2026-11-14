package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRecentLibraryPaths: a folder and a library's file are libraries to open from the recent documents, a page's
// Markdown is not, and the library open is told apart from another (#359).
func TestRecentLibraryPaths(t *testing.T) {
	dir := t.TempDir()
	db, md := filepath.Join(dir, "kitaplik.db"), filepath.Join(dir, "note.md")
	for _, f := range []string{db, md} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !isLibraryPath(dir) || !isLibraryPath(db) || isLibraryPath(md) || isLibraryPath(filepath.Join(dir, "gone.db")) {
		t.Fatal("what is a library to open is wrong")
	}
	if !sameFile(db, filepath.Join(dir, ".", "kitaplik.db")) || sameFile(db, md) || sameFile(db, "") {
		t.Fatal("the library open is not told apart")
	}
}
