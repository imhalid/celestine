//go:build !js

package library

import (
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
