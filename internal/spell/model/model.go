// Package model keeps tinyfix-tr's data on the device (#623): the spell checker's files are not in the app, they are
// downloaded from Hugging Face once, on demand, at a pinned revision, checked against their known sizes and hashes,
// and kept in the app's folder, where they are read from then on, never downloaded again unless they are deleted. The
// settings delete them, or download them again over the ones there.
package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Revision is the revision of imhalid/tinyfix-tr on Hugging Face whose files are downloaded: the npm package's of the
// same engine, so that what the Go port does is what it was measured against. A new revision is a new folder.
const Revision = "d1e88233bc0c64e697a996833a21de144d4f1c18"

// BaseURL is where the revision's files are downloaded from.
const BaseURL = "https://huggingface.co/imhalid/tinyfix-tr/resolve/" + Revision + "/"

// File is one of the model's files: its name, its size in bytes, and the SHA-256 of its bytes at Revision.
type File struct {
	Name   string
	Size   int64
	SHA256 string
}

// Files are the files the engine reads: the lexicon, the morphology and the known misspellings. The context model
// (context.onnx) joins them when the engine reads it.
var Files = []File{
	{"lexicon.bin", 5587966, "cfeb4b8c7e195e63875ea8ab8bbfc5afd28c3c9b31072990dd811097630ce670"},
	{"morph.bin", 337652, "72040edbf47168150c2922f022e8b8d90a87e4860e438a24ab0dfe2eacbcccfb"},
	{"known.tsv", 353276, "87881d027a20413aff3b25674ac16bed642e17326610611d8d7ad311da4c4dc6"},
}

// TotalSize is the size of all the files, in bytes.
func TotalSize() int64 {
	var n int64
	for _, f := range Files {
		n += f.Size
	}
	return n
}

// Dir is the folder of the model's revision under root, the app's own folder (os.UserConfigDir's celestine): one
// folder for each revision, under one for the spell checker, which Remove takes away whole.
func Dir(root string) string { return filepath.Join(root, "spell", Revision) }

// Installed reports whether every file is in dir, at its size. Their hashes were checked as they were downloaded, and
// are not read again each time the app opens.
func Installed(dir string) bool {
	for _, f := range Files {
		st, err := os.Stat(filepath.Join(dir, f.Name))
		if err != nil || st.Size() != f.Size {
			return false
		}
	}
	return true
}

// Progress is told how much of the download is done, in bytes, of total.
type Progress func(done, total int64)

// client is the HTTP client of the downloads: no download takes longer than this, a stalled one is given up.
var client = &http.Client{Timeout: 5 * time.Minute}

// Download puts the files in dir, downloading those not there at their size (all of them when again is set), each to
// a file of its own beside, renamed over its place once its size and hash are those of Files: a download cut short or
// changed on the way leaves nothing in place. base is BaseURL but in a test.
func Download(ctx context.Context, base, dir string, again bool, progress Progress) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	total, done := TotalSize(), int64(0)
	for _, f := range Files {
		path := filepath.Join(dir, f.Name)
		if st, err := os.Stat(path); err == nil && st.Size() == f.Size && !again {
			done += f.Size
			if progress != nil {
				progress(done, total)
			}
			continue
		}
		if err := fetch(ctx, base+f.Name, path, f, func(n int64) {
			if progress != nil {
				progress(done+n, total)
			}
		}); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		done += f.Size
	}
	return nil
}

// ErrChanged is what Download says of a file whose size or hash is not the pinned one.
var ErrChanged = errors.New("the downloaded file is not the expected one")

// fetch downloads url to path as f, telling each step's bytes to step.
func fetch(ctx context.Context, url, path string, f File, step func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "celestine") // a generic name, nothing of the user's
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+f.Name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // gone once renamed, or left by a failure
	h := sha256.New()
	var n int64
	buf := make([]byte, 64<<10)
	for {
		k, rerr := resp.Body.Read(buf)
		if k > 0 {
			if n+int64(k) > f.Size {
				tmp.Close()
				return ErrChanged
			}
			if _, werr := tmp.Write(buf[:k]); werr != nil {
				tmp.Close()
				return werr
			}
			h.Write(buf[:k])
			n += int64(k)
			step(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		return ErrChanged
	}
	return os.Rename(tmp.Name(), path)
}

// Remove deletes the spell checker's files under root, every revision's.
func Remove(root string) error { return os.RemoveAll(filepath.Join(root, "spell")) }
