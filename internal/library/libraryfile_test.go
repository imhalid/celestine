package library

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The library is in the data folder, unless one is already beside the program (a portable copy) where it can be
// written; never inside an app bundle, from which one is moved out, from the bundle an update left too (#230).
func TestLibraryFileIn(t *testing.T) {
	dir, data := t.TempDir(), filepath.Join(t.TempDir(), "celestine")
	if got := libraryFileIn(dir, data); got != filepath.Join(data, "kitaplik.db") {
		t.Fatalf("a program's folder with no library: %s", got)
	}
	if st, err := os.Stat(data); err != nil {
		t.Fatalf("the data folder is made: %v", err)
	} else if runtime.GOOS != "windows" && st.Mode().Perm() != 0o700 {
		t.Fatalf("the data folder mode = %v, want 0700", st.Mode().Perm())
	}
	os.WriteFile(filepath.Join(dir, "kitaplik.db"), []byte("taşınabilir"), 0o644)
	if got := libraryFileIn(dir, data); got != filepath.Join(dir, "kitaplik.db") {
		t.Fatalf("a portable library beside the program: %s", got)
	}
	if got := libraryFileIn(dir, ""); got != filepath.Join(dir, "kitaplik.db") {
		t.Fatalf("no data folder known: %s", got)
	}

	apps := t.TempDir()
	exe := filepath.Join(apps, "celestine.app", "Contents", "MacOS")
	os.MkdirAll(exe, 0o755)
	os.WriteFile(filepath.Join(exe, "kitaplik.db"), []byte("pakette"), 0o644)
	os.WriteFile(filepath.Join(exe, "kitaplik.db-wal"), []byte("günlük"), 0o644)
	data2 := filepath.Join(t.TempDir(), "celestine")
	got := libraryFileIn(exe, data2)
	if got != filepath.Join(data2, "kitaplik.db") {
		t.Fatalf("inside a bundle: %s", got)
	}
	if b, _ := os.ReadFile(got); string(b) != "pakette" {
		t.Fatal("the library was not moved out of the bundle")
	}
	if b, _ := os.ReadFile(got + "-wal"); string(b) != "günlük" {
		t.Fatal("its journal was not moved with it")
	}
	if _, err := os.Stat(filepath.Join(exe, "kitaplik.db")); err == nil {
		t.Fatal("still in the bundle")
	}

	// an update: the new bundle is empty, the old one beside it holds the library
	old := filepath.Join(apps, "celestine.app.old", "Contents", "MacOS")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "kitaplik.db"), []byte("eskide"), 0o644)
	data3 := filepath.Join(t.TempDir(), "celestine")
	if b, _ := os.ReadFile(libraryFileIn(exe, data3)); string(b) != "eskide" {
		t.Fatal("the library in the bundle an update left was not taken")
	}
	// one already in the data folder is never written over
	os.WriteFile(filepath.Join(old, "kitaplik.db"), []byte("başka"), 0o644)
	if b, _ := os.ReadFile(libraryFileIn(exe, data3)); string(b) != "eskide" {
		t.Fatal("the data folder's library was replaced")
	}

	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("a folder that cannot be written needs a user that is not root, on Unix")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if got := libraryFileIn(dir, data); got != filepath.Join(data, "kitaplik.db") {
		t.Fatalf("a portable library that cannot be written: %s", got)
	}
}
