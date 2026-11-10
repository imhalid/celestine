//go:build !js

package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// A library file in a folder whose name has #, ? or % opens that file, not another (#137), in WAL (#136).
func TestStorePathWithURIChars(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My #1 notes? 50% done")
	os.MkdirAll(dir, 0o755)
	file := filepath.Join(dir, "kitaplik.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetMeta("k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	var mode string
	st.DB.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	st.Close()
	if mode != "wal" {
		t.Fatalf("journal mode %q", mode)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("not the file named")
	}
	ents, _ := os.ReadDir(filepath.Dir(dir))
	if len(ents) != 1 {
		t.Fatalf("a stray file beside the folder: %v", ents)
	}
	st, _ = OpenStore(file)
	defer st.Close()
	if string(st.Meta("k")) != "v" {
		t.Fatal("not read back")
	}
	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := st.Snapshot(snap); err != nil {
		t.Fatal(err)
	}
}

// The smaller points of the store (#143): a folder's Stat, a slug cut at a letter's start, a collection.json that
// does not change with what the app only keeps in memory.
func TestStoreSmallerPoints(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.MkdirAll(StoreRoot + "/Notes/A"); err != nil {
		t.Fatal(err)
	}
	if fi, err := st.Stat(StoreRoot + "/Notes"); err != nil || !fi.IsDir() {
		t.Fatalf("a folder's Stat: %v", err)
	}
	if err := st.WriteFile(StoreRoot+"/Gone/x", []byte("x")); err == nil {
		t.Fatal("a file written into no folder")
	}
	long := strings.Repeat("日本語のタイトル", 10)
	if s := Slug(long); !utf8.ValidString(s) || len(s) > 60 {
		t.Fatalf("slug %q", s)
	}
	lib, _ := OpenLibraryIn(st, StoreRoot)
	c, _ := lib.NewCollection("Günlük")
	before, _ := st.ReadFile(filepath.Join(c.Dir, CollectionFile))
	c.Creating, c.Seen = true, time.Now()
	c.Save()
	after, _ := st.ReadFile(filepath.Join(c.Dir, CollectionFile))
	if string(before) != string(after) {
		t.Fatalf("collection.json changed:\n%s\n%s", before, after)
	}
}

// A reorder is one change and keeps no copy of pages whose text did not change (#138); a removed file's copies go
// with it (#139); a trash item with no page file stays in the trash (#142).
func TestReorderVersionsRestore(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	st.Now = func() time.Time { return now }
	lib, _ := OpenLibraryIn(st, StoreRoot)
	c, _ := lib.NewCollection("Notes")
	var ps []*Page
	for i := range 5 {
		p, _ := c.NewPage(nil, "Sayfa")
		p.SaveText(strings.Repeat("metin ", i+1))
		ps = append(ps, p)
	}
	now = now.Add(time.Hour)
	ps = append([]*Page{ps[4]}, ps[:4]...)
	if err := Renumber(ps); err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if n := len(st.Versions(p.File())); n != 0 {
			t.Fatalf("a reorder kept %d copies of %s", n, p.File())
		}
	}
	// a batch that fails is undone whole
	err = st.Batch(func() error {
		ps[0].SaveText("değişti\n")
		return errors.New("stop")
	})
	if b, _ := ps[0].Text(); err == nil || strings.Contains(b, "değişti") {
		t.Fatalf("a failed batch was kept: %q", b)
	}
	// a removed file's copies go with it
	p := ps[1]
	now = now.Add(time.Hour)
	p.SaveText("yeni metin\n")
	if len(st.Versions(p.File())) == 0 {
		t.Fatal("no copy to begin with")
	}
	st.Remove(p.File())
	if n := len(st.Versions(p.File())); n != 0 {
		t.Fatalf("%d copies left for the next page at that place", n)
	}
	// a trash item with no page file stays in the trash
	item, _ := lib.TrashPage(ps[2])
	st.Remove(filepath.Join(item.Dir, PageFileName))
	if _, _, err := lib.Restore(item); err == nil {
		t.Fatal("restored with no page file")
	}
	if len(lib.Trash()) != 1 {
		t.Fatal("not in the trash any more")
	}
}

// A folder moved or removed takes its tree and its copies with it, in one statement a table, and nothing of a
// folder whose name only starts the same (#200).
func TestStoreSubtreeRange(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, d := range []string{"/a/b", "/a-b", "/a0", "/x"} {
		if err := st.MkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"/a/page.json", "/a/b/page.json", "/a-b/page.json", "/a0/page.json"} {
		st.WriteFile(f, []byte(`{"meta":{},"body":[]}`))
	}
	st.keep("/a/b/page.json", []byte("eski"), time.Now())
	if err := st.Rename("/a", "/x/a"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"/x/a/page.json", "/x/a/b/page.json", "/a-b/page.json", "/a0/page.json"} {
		if _, err := st.ReadFile(f); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if _, err := st.Stat("/a"); err == nil {
		t.Fatal("the old folder stays")
	}
	if len(st.Versions("/x/a/b/page.json")) != 1 {
		t.Fatal("the copies did not move")
	}
	if ents, _ := st.ReadDir("/x/a"); len(ents) != 2 {
		t.Fatalf("children under the new parent: %d", len(ents))
	}
	if err := st.RemoveAll("/x/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadFile("/x/a/b/page.json"); err == nil || len(st.Versions("/x/a/b/page.json")) != 0 {
		t.Fatal("not removed with its copies")
	}
	if _, err := st.ReadFile("/a-b/page.json"); err != nil {
		t.Fatal("a folder of a like name went with it")
	}
}

// The snapshot read as the library opens holds the JSON files only: a picture's or an attachment's bytes are read
// when asked for, not kept for all of them at once (#197).
func TestSnapshotHoldsJSONOnly(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.MkdirAll("/c/p")
	st.WriteFile("/c/p/page.json", []byte(`{"meta":{},"body":[]}`))
	big := make([]byte, 1<<20)
	st.WriteFile("/c/p/resim.png", big)
	st.BeginSnapshot()
	defer st.EndSnapshot()
	sn := st.Snap.Load()
	if sn == nil || sn.nodes["/c/p/resim.png"].data != nil || sn.nodes["/c/p/page.json"].data == nil {
		t.Fatal("the snapshot holds what it should not, or not what it should")
	}
	if b, err := st.ReadFile("/c/p/resim.png"); err != nil || len(b) != len(big) {
		t.Fatalf("the picture: %d bytes, %v", len(b), err)
	}
	if fi, err := st.Stat("/c/p/resim.png"); err != nil || fi.Size() != int64(len(big)) {
		t.Fatalf("its size: %v", err)
	}
}
