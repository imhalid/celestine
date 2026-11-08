package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// The library keeps its pages as a tree of folders and files; where that tree
// is kept is an FS: the disk (a folder, as before: tests, and a folder given
// on the command line) or a Store, one file holding all of it, which nothing
// else reads and which grows with the library.

// FS is the little of a file system the library uses. Names are slash paths
// from the library's root; errors are those of os (fs.ErrExist, fs.ErrNotExist).
type FS interface {
	ReadFile(name string) ([]byte, error)
	// WriteFile writes a file whole, or not at all: a crash never leaves half of it.
	WriteFile(name string, data []byte) error
	ReadDir(name string) ([]fs.DirEntry, error)
	Mkdir(name string) error
	MkdirAll(name string) error
	Rename(oldName, newName string) error
	Remove(name string) error
	RemoveAll(name string) error
	Stat(name string) (fs.FileInfo, error)
}

// osFS is the disk.
type osFS struct{}

func (osFS) ReadFile(n string) ([]byte, error)       { return os.ReadFile(n) }
func (osFS) ReadDir(n string) ([]fs.DirEntry, error) { return os.ReadDir(n) }
func (osFS) Mkdir(n string) error                    { return os.Mkdir(n, 0o755) }
func (osFS) MkdirAll(n string) error                 { return os.MkdirAll(n, 0o755) }
func (osFS) Rename(a, b string) error                { return os.Rename(a, b) }
func (osFS) Remove(n string) error                   { return os.Remove(n) }
func (osFS) RemoveAll(n string) error                { return os.RemoveAll(n) }
func (osFS) Stat(n string) (fs.FileInfo, error)      { return os.Stat(n) }

// WriteFile goes through a temporary file, so a crash never leaves half of it.
func (osFS) WriteFile(name string, data []byte) error {
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

// writeFilePrivate writes a file whole, as WriteFile does, for its owner alone: the settings hold keys and tokens.
func writeFilePrivate(name string, data []byte) error {
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	os.Chmod(tmp, 0o600) // a leftover from before was made with more
	return os.Rename(tmp, name)
}

// storeRoot is the root of a store's tree.
const storeRoot = "/"

func clean(name string) string {
	n := path.Clean("/" + filepath.ToSlash(name))
	return n
}

func pathErr(op, name string, err error) error { return &fs.PathError{Op: op, Path: name, Err: err} }

// info is a node, as fs.FileInfo and fs.DirEntry.
type info struct {
	name  string
	size  int64
	dir   bool
	mtime time.Time
}

func (i info) Name() string { return i.name }
func (i info) Size() int64  { return i.size }
func (i info) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (i info) ModTime() time.Time         { return i.mtime }
func (i info) IsDir() bool                { return i.dir }
func (i info) Sys() any                   { return nil }
func (i info) Type() fs.FileMode          { return i.Mode().Type() }
func (i info) Info() (fs.FileInfo, error) { return i, nil }

// copyTree copies the folder srcRoot of src, all that is under it, to dstRoot
// of dst: a folder library into a file (the first time), and back (an export).
// A link to a file is copied as the file; a link to a folder (it may point
// back up the tree), a link to nothing and what is neither file nor folder are
// left, so that one odd entry never stops a whole copy.
func copyTree(src FS, srcRoot string, dst FS, dstRoot string) error {
	if err := dst.MkdirAll(dstRoot); err != nil {
		return err
	}
	entries, err := src.ReadDir(srcRoot)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from, to := filepath.Join(srcRoot, e.Name()), filepath.Join(dstRoot, e.Name())
		switch {
		case e.IsDir():
			if err := copyTree(src, from, dst, to); err != nil {
				return err
			}
		case strings.HasSuffix(e.Name(), ".tmp"):
		case !e.Type().IsRegular() && !linksToFile(src, from, e):
		default:
			data, err := src.ReadFile(from)
			if err != nil {
				return err
			}
			if err := dst.WriteFile(to, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// linksToFile reports whether the entry e, at path, is a link to a regular file.
func linksToFile(fsys FS, path string, e fs.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := fsys.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isLibraryFolder reports whether dir holds a library: a folder with a
// collection in it.
func isLibraryFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), collectionFile)); err == nil {
				return true
			}
		}
	}
	return false
}

// libraryFile is where the library is kept when none is named: a file next
// to the program, so that the two move together.
func libraryFile() string {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	return filepath.Join(dir, "kitaplik.db")
}
