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

// A write from another goroutine while a Batch is open is its own: a Batch that fails takes back what its fn wrote,
// not what the sync wrote meanwhile, which had been told it was written (#526).
func TestBatchKeepsOtherWrites(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.MkdirAll("/c/a")
	st.MkdirAll("/c/b")
	st.WriteFile("/c/a/"+PageFileName, []byte("v1"))
	st.WriteFile("/c/b/"+PageFileName, []byte("mine"))
	watch := func(name string, f func()) {
		t.Helper()
		done := make(chan struct{})
		go func() { defer close(done); f() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: stuck", name)
		}
	}
	read := func(name string) string { b, _ := st.ReadFile(name); return string(b) }

	// the sync writes v2 while the window's batch is open, and the batch fails
	var synced error
	watch("failing batch", func() {
		wrote := make(chan struct{})
		err := st.Batch(func() error {
			if err := st.WriteFile("/c/b/"+PageFileName, []byte("batch")); err != nil {
				return err
			}
			go func() { defer close(wrote); synced = st.WriteFile("/c/a/"+PageFileName, []byte("v2")) }()
			select {
			case <-wrote: // the write went into the batch's transaction
			case <-time.After(200 * time.Millisecond): // the write waits for the batch
			}
			return errors.New("a sibling cannot be read")
		})
		if err == nil {
			t.Error("the batch did not fail")
		}
		<-wrote
	})
	if synced != nil {
		t.Fatal(synced)
	}
	if b := read("/c/a/" + PageFileName); b != "v2" {
		t.Fatalf("the sync's write was taken back: %q", b)
	}
	if b := read("/c/b/" + PageFileName); b != "mine" {
		t.Fatalf("the failed batch's own write was kept: %q", b)
	}

	// what fn does through the store goes into the batch: a save with its copy, a read, a move, a folder, a removal
	ops := func() error {
		if err := st.WriteFile("/c/b/"+PageFileName, []byte("batch text\n")); err != nil {
			return err
		}
		if read("/c/b/"+PageFileName) != "batch text\n" {
			return errors.New("the batch does not read its own write")
		}
		if err := st.Rename("/c/b", "/c/d"); err != nil {
			return err
		}
		if err := st.MkdirAll("/c/e/f"); err != nil {
			return err
		}
		return st.RemoveAll("/c/a")
	}
	watch("failing batch of every kind", func() {
		if err := st.Batch(func() error {
			if err := ops(); err != nil {
				t.Error(err)
			}
			return errors.New("stop")
		}); err == nil {
			t.Error("the batch did not fail")
		}
	})
	if read("/c/a/"+PageFileName) != "v2" || read("/c/b/"+PageFileName) != "mine" {
		t.Fatal("a failed batch was kept")
	}
	if _, err := st.Stat("/c/e"); err == nil {
		t.Fatal("a failed batch's folder was kept")
	}
	watch("batch of every kind", func() {
		if err := st.Batch(ops); err != nil {
			t.Error(err)
		}
	})
	if _, err := st.Stat("/c/a"); err == nil || read("/c/d/"+PageFileName) != "batch text\n" {
		t.Fatal("a batch was not kept")
	}
	if _, err := st.Stat("/c/e/f"); err != nil {
		t.Fatal(err)
	}
	// a panic in fn leaves the store to the others
	func() {
		defer func() { recover() }()
		st.Batch(func() error { panic("boom") })
	}()
	watch("after a panic", func() {
		if err := st.SetMeta("k", []byte("v")); err != nil {
			t.Error(err)
		}
	})
}
