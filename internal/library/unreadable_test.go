package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	items := lib.Trash() // and it can be put back (#456)
	if len(items) != 1 || items[0].Kind != "files" {
		t.Fatalf("trash: %+v", items)
	}
	if _, _, err := lib.Restore(items[0]); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(pic); err != nil || string(b) != "png" {
		t.Fatal("not put back")
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
	if _, err := lib.NewCollection("Good"); err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Bad")
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(c.Dir, CollectionFile) // its folder as the library names it: "bad", on a disk that tells case apart
	if err := os.WriteFile(bad, []byte(`{"name": "Bad", "pages": [`), 0o644); err != nil {
		t.Fatal(err)
	}
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

// A folder library's file is written whole through a temporary file of its own, which does not stay; an empty page
// file is one cut short, not an empty page (#140).
func TestWriteWholeAndEmptyPage(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, PageFileName)
	if err := (OSFS{}).WriteFile(name, []byte(`{"meta":{"title":"x"},"body":[]}`)); err != nil {
		t.Fatal(err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("left beside it: %v", ents)
	}
	if _, _, err := SplitPageChecked([]byte("  ")); err == nil {
		t.Fatal("an empty page file read as a page")
	}
}

// DiffLines compares only what lies between the lines both texts start and end with, and gives what it gave: the
// shared lines kept, the version's as "+", the present's as "-" (#199).
func TestDiffLinesTrimmed(t *testing.T) {
	got := DiffLines([]string{"a", "b", "c", "d"}, []string{"a", "x", "c", "d"})
	want := "=a +b -x =c =d"
	var s []string
	for _, o := range got {
		s = append(s, string(o.Kind)+o.Text)
	}
	if strings.Join(s, " ") != want {
		t.Fatalf("%v, want %s", s, want)
	}
	long := make([]string, 1500)
	for i := range long {
		long[i] = fmt.Sprint("satır ", i)
	}
	edited := slices.Clone(long)
	edited[700] = "değişti"
	if ops := DiffLines(long, edited); len(ops) != 1501 {
		t.Fatalf("%d ops for one line changed in 1 500", len(ops))
	}
}
