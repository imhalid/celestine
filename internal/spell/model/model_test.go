package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// The files are downloaded once, each checked against its size and hash, and not again; a file of another size is
// downloaded again, a changed one is refused and leaves nothing in place, again downloads them all, and Remove takes
// them away (#623).
func TestDownloadOnce(t *testing.T) {
	body := map[string]string{"a.bin": "first file", "b.tsv": "second\tfile\n"}
	saved := Files
	defer func() { Files = saved }()
	Files = nil
	for _, name := range []string{"a.bin", "b.tsv"} {
		sum := sha256.Sum256([]byte(body[name]))
		Files = append(Files, File{name, int64(len(body[name])), hex.EncodeToString(sum[:])})
	}
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		w.Write([]byte(body[strings.TrimPrefix(r.URL.Path, "/")]))
	}))
	defer srv.Close()
	root := t.TempDir()
	dir := Dir(root)
	if Installed(dir) {
		t.Fatal("installed before any download")
	}
	var last int64
	if err := Download(context.Background(), srv.URL+"/", dir, false, func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if !Installed(dir) || asked.Load() != 2 || last != TotalSize() {
		t.Fatalf("installed %v, asked %d, progress %d of %d", Installed(dir), asked.Load(), last, TotalSize())
	}
	if err := Download(context.Background(), srv.URL+"/", dir, false, nil); err != nil || asked.Load() != 2 {
		t.Fatalf("downloaded again: %v, asked %d", err, asked.Load())
	}
	os.WriteFile(filepath.Join(dir, "a.bin"), []byte("cut"), 0o644) // a file of another size: downloaded again
	if err := Download(context.Background(), srv.URL+"/", dir, false, nil); err != nil || asked.Load() != 3 {
		t.Fatalf("a short file: %v, asked %d", err, asked.Load())
	}
	if err := Download(context.Background(), srv.URL+"/", dir, true, nil); err != nil || asked.Load() != 5 {
		t.Fatalf("again: %v, asked %d", err, asked.Load())
	}
	body["b.tsv"] = "second\tfilE\n" // the same size, another hash
	os.Remove(filepath.Join(dir, "b.tsv"))
	if err := Download(context.Background(), srv.URL+"/", dir, false, nil); !errors.Is(err, ErrChanged) {
		t.Fatalf("a changed file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.tsv")); err == nil || Installed(dir) {
		t.Fatal("a changed file was put in place")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
	if err := Remove(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "spell")); !os.IsNotExist(err) {
		t.Fatal("not removed")
	}
}
