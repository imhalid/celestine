package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A key stamped into a page's meta leaves its body and the time it was last written as they were, on the disk and
// in a store, read again after the library is opened again; and a sync still sends the page, whose size changed
// (#538).
func TestStampMetaKeepsModified(t *testing.T) {
	old := time.Date(2025, 3, 4, 5, 6, 7, 8000, time.UTC)
	for _, c := range []struct {
		name string
		open func(t *testing.T) (FS, string, func(file string))
	}{
		{"disk", func(t *testing.T) (FS, string, func(string)) {
			return OSFS{}, t.TempDir(), func(file string) {
				if err := os.Chtimes(file, old, old); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"store", func(t *testing.T) (FS, string, func(string)) {
			st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { st.Close() })
			return st, StoreRoot, func(file string) {
				if _, err := st.DB.Exec(`UPDATE nodes SET mtime = ? WHERE path = ?`, old.UnixNano(), Clean(file)); err != nil {
					t.Fatal(err)
				}
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			fsys, root, age := c.open(t)
			lib, err := OpenLibraryIn(fsys, root)
			if err != nil {
				t.Fatal(err)
			}
			coll, err := lib.NewCollection("Journal")
			if err != nil {
				t.Fatal(err)
			}
			p, err := coll.NewPage(nil, "Monday")
			if err != nil {
				t.Fatal(err)
			}
			if err := p.SaveText("words of the day\n"); err != nil {
				t.Fatal(err)
			}
			age(p.File())
			body, _ := p.Body()
			info, _ := fsys.Stat(p.File())
			size := info.Size()

			wrote, err := StampMeta(fsys, p.File(), "celestine:day", "2025-03-04")
			if err != nil || !wrote {
				t.Fatalf("stamped: %v %v", wrote, err)
			}
			if wrote, err := StampMeta(fsys, p.File(), "celestine:day", "2025-03-05"); err != nil || wrote {
				t.Fatalf("stamped twice: %v %v", wrote, err)
			}
			again, err := OpenLibraryIn(fsys, root)
			if err != nil {
				t.Fatal(err)
			}
			q := again.Collections[0].All()[0]
			if q.Props["celestine:day"] != "2025-03-04" || q.Title != "Monday" {
				t.Fatalf("meta read back: %q %v", q.Title, q.Props)
			}
			if !q.Modified.Equal(old) {
				t.Fatalf("modified %v, want %v", q.Modified, old)
			}
			if b, _ := q.Body(); b != body {
				t.Fatalf("body %q, want %q", b, body)
			}
			if info, _ := fsys.Stat(q.File()); info.Size() == size {
				t.Fatal("the file is the same size: a sync would not see it changed")
			}
		})
	}
}

// A sync sends a page whose meta was stamped, though the time it was last written is the one it had.
func TestStampMetaSyncs(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
	d := newDevice(t)
	file := filepath.Join(d.root, "journal", "monday", PageFileName)
	d.write("journal/monday/"+PageFileName, string(JoinPage(map[string]string{"title": "Monday"}, "[]")))
	d.sync(remote)
	if wrote, err := StampMeta(OSFS{}, file, "celestine:day", "2025-03-04"); err != nil || !wrote {
		t.Fatalf("stamped: %v %v", wrote, err)
	}
	if res := d.sync(remote); res.Pushed != 1 {
		t.Fatalf("pushed %d", res.Pushed)
	}
}

// Where a file system cannot keep the time, nothing is written.
func TestStampMetaUnsupported(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, PageFileName)
	src := JoinPage(map[string]string{"title": "A"}, "[]")
	os.WriteFile(file, src, 0o644)
	if wrote, err := StampMeta(plainFS{OSFS{}}, file, "celestine:day", "2025-03-04"); wrote || err == nil {
		t.Fatalf("wrote %v, %v", wrote, err)
	}
	if b, _ := os.ReadFile(file); string(b) != string(src) {
		t.Fatalf("written: %s", b)
	}
}

// plainFS is a file system that cannot keep a file's time.
type plainFS struct{ FS }
