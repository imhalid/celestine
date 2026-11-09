package library

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLibraryFileIn(t *testing.T) {
	dir, data := t.TempDir(), filepath.Join(t.TempDir(), "ryudit")
	if got := libraryFileIn(dir, data); got != filepath.Join(dir, "kitaplik.db") {
		t.Fatalf("a folder that can be written: %s", got)
	}
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("a folder that cannot be written needs a user that is not root, on Unix")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if got := libraryFileIn(dir, data); got != filepath.Join(data, "kitaplik.db") {
		t.Fatalf("a folder that cannot be written: %s", got)
	}
	if _, err := os.Stat(data); err != nil {
		t.Fatalf("the data folder is made: %v", err)
	}
	if got := libraryFileIn(dir, ""); got != filepath.Join(dir, "kitaplik.db") {
		t.Fatalf("no data folder known: %s", got)
	}
}
