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

// A collection.json that cannot be read leaves its collection out, named in Damaged, and the rest of the library
// opens; its folder is left as it is (#134).
func TestDamagedCollectionLeftOut(t *testing.T) {
	root := t.TempDir()
	lib, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Good", "Bad"} {
		if _, err := lib.NewCollection(n); err != nil {
			t.Fatal(err)
		}
	}
	bad := filepath.Join(root, "Bad", CollectionFile)
	os.WriteFile(bad, []byte(`{"name": "Bad", "pages": [`), 0o644)
	lib, err = OpenLibrary(root)
	if err != nil {
		t.Fatalf("the library did not open: %v", err)
	}
	if len(lib.Collections) != 1 || lib.Collections[0].Name != "Good" || len(lib.Damaged) != 1 {
		t.Fatalf("collections %d, damaged %v", len(lib.Collections), lib.Damaged)
	}
	if b, _ := os.ReadFile(bad); string(b) != `{"name": "Bad", "pages": [` {
		t.Fatal("the damaged file was changed")
	}
}

// A body that is not JSON is refused, not written as an empty file (#135).
func TestSaveRefusesInvalidBody(t *testing.T) {
	lib, _ := OpenLibrary(t.TempDir())
	c, _ := lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Trip")
	before, _ := os.ReadFile(p.File())
	if err := p.Save("!json: unsupported value"); err == nil {
		t.Fatal("saved")
	}
	if after, _ := os.ReadFile(p.File()); string(after) != string(before) {
		t.Fatal("the file changed")
	}
}
