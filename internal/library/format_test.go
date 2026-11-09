package library

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestFormatFolder: a new folder library is written in this app's format, one that says an older format is
// brought up to it, and one of a newer format is refused and left as it was.
func TestFormatFolder(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenLibrary(root); err != nil {
		t.Fatal(err)
	}
	if got := LibraryFormat(OSFS{}, root); got != FormatVersion {
		t.Fatalf("a new library: format %d, want %d", got, FormatVersion)
	}

	old := t.TempDir()
	os.WriteFile(filepath.Join(old, FormatFile), []byte("0\n"), 0o644)
	if _, err := OpenLibrary(old); err != nil {
		t.Fatal(err)
	}
	if got := LibraryFormat(OSFS{}, old); got != FormatVersion {
		t.Fatalf("an older library: format %d, want %d", got, FormatVersion)
	}

	newer := t.TempDir()
	mark := []byte(strconv.Itoa(FormatVersion+1) + "\n")
	os.WriteFile(filepath.Join(newer, FormatFile), mark, 0o644)
	os.Mkdir(filepath.Join(newer, "notes"), 0o755) // a collection that needs a migration: not to be made
	os.WriteFile(filepath.Join(newer, "notes", CollectionFile), []byte(`{"name":"Notes"}`), 0o644)
	_, err := OpenLibrary(newer)
	var nf *NewerFormatError
	if !errors.As(err, &nf) || nf.Library != FormatVersion+1 || nf.App != FormatVersion {
		t.Fatalf("a newer library: %v, want a *NewerFormatError", err)
	}
	if b, _ := os.ReadFile(filepath.Join(newer, FormatFile)); string(b) != string(mark) {
		t.Fatalf("the newer library's format was written: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(newer, "notes", CollectionFile)); string(b) != `{"name":"Notes"}` {
		t.Fatalf("the newer library's collection was written: %s", b)
	}
}

// TestFormatStore: the same for a library in one file, which keeps its format in its meta.
func TestFormatStore(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := OpenLibraryIn(st, StoreRoot); err != nil {
		t.Fatal(err)
	}
	if got := LibraryFormat(st, StoreRoot); got != FormatVersion {
		t.Fatalf("a new library: format %d, want %d", got, FormatVersion)
	}
	if _, err := st.ReadFile(filepath.Join(StoreRoot, FormatFile)); err == nil {
		t.Fatal("a library file keeps its format in its meta, not in a file")
	}

	mark := []byte(strconv.Itoa(FormatVersion+1) + "\n")
	st.SetMeta(FormatKey, mark)
	writes := st.Writes.Load()
	_, err = OpenLibraryIn(st, StoreRoot)
	var nf *NewerFormatError
	if !errors.As(err, &nf) || nf.Library != FormatVersion+1 {
		t.Fatalf("a newer library: %v, want a *NewerFormatError", err)
	}
	if string(st.Meta(FormatKey)) != string(mark) || st.Writes.Load() != writes {
		t.Fatalf("the newer library was written: format %q, %d writes after %d", st.Meta(FormatKey), st.Writes.Load(), writes)
	}
}

// TestFormatNotSynced: the format file of a folder library is not pushed: each device's app writes its own.
func TestFormatNotSynced(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenLibrary(root); err != nil {
		t.Fatal(err)
	}
	local, err := PristineOf(OSFS{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := local[FormatFile]; ok {
		t.Fatal("the format file is among the files to sync")
	}
}
