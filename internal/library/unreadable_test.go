package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A page marked unreadable is never written over, and TrashFiles keeps what it takes in the trash (#125, #126).
func TestUnreadableAndTrashFiles(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Notes")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Trip")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p.File())
	p.Unreadable = "cut short"
	if err := p.SaveText("over it\n"); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("saved: %v", err)
	}
	if after, _ := os.ReadFile(p.File()); string(after) != string(before) {
		t.Fatal("the file changed")
	}
	pic := filepath.Join(p.Dir, "resim-1.png")
	os.WriteFile(pic, []byte("png"), 0o644)
	if err := lib.TrashFiles(p, []string{"resim-1.png"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pic); err == nil {
		t.Fatal("still here")
	}
	if got, _ := filepath.Glob(filepath.Join(lib.Root, TrashDir, "*", "resim-1.png")); len(got) != 1 {
		t.Fatal("not in the trash")
	}
	if _, err := os.Stat(p.File()); err != nil {
		t.Fatal("the page went with it")
	}
}
