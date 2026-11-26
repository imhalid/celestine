package library

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
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

// OSFS is the disk.
type OSFS struct{}

func (OSFS) ReadFile(n string) ([]byte, error)       { return os.ReadFile(n) }
func (OSFS) ReadDir(n string) ([]fs.DirEntry, error) { return os.ReadDir(n) }
func (OSFS) Mkdir(n string) error                    { return os.Mkdir(n, 0o755) }
func (OSFS) MkdirAll(n string) error                 { return os.MkdirAll(n, 0o755) }
func (OSFS) Rename(a, b string) error                { return os.Rename(a, b) }
func (OSFS) Remove(n string) error                   { return os.Remove(n) }
func (OSFS) RemoveAll(n string) error                { return os.RemoveAll(n) }
func (OSFS) Stat(n string) (fs.FileInfo, error)      { return os.Stat(n) }

// WriteFile goes through a temporary file, so a crash never leaves half of it.
func (OSFS) WriteFile(name string, data []byte) error {
	return writeWhole(name, data, 0o644, time.Time{})
}

// WriteFileKeepTime writes over name as WriteFile does, and gives the new file the time the old one was last
// written before it takes the name: what looks at the folder never sees it written now (TimeKeeper).
func (OSFS) WriteFileKeepTime(name string, data []byte) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	return writeWhole(name, data, 0o644, info.ModTime())
}

// WriteFilePrivate writes a file whole, as WriteFile does, for its owner alone: the settings hold keys and tokens.
func WriteFilePrivate(name string, data []byte) error {
	return writeWhole(name, data, 0o600, time.Time{})
}

// tempPattern is the pattern of writeWhole's temporary file for a file named base, as os.CreateTemp takes it: a
// hidden ".<base>.<digits>.tmp" beside it. IsTempFile knows the names it makes, so the two cannot disagree.
func tempPattern(base string) string { return "." + base + ".*.tmp" }

// IsTempFile reports whether name (a base name) is one of writeWhole's temporary files. What walks a folder library
// (the sync, the search for conflicts) leaves it out: it is renamed to its file or removed in a moment (#542).
func IsTempFile(name string) bool {
	if !strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".tmp") {
		return false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".tmp")
	i := strings.LastIndexByte(mid, '.')
	if i <= 0 || i == len(mid)-1 { // a base before the dot, and os.CreateTemp's random digits after it
		return false
	}
	for _, c := range mid[i+1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// writeWhole writes data to name through a temporary file of its own beside it, on the disk before it takes the
// name, and the folder's entry after: a power cut leaves the old file or the new one, never an empty one, and two
// writers do not share a temporary name (#140). Given a time, mt, the file takes it as when it was last written.
func writeWhole(name string, data []byte, perm os.FileMode, mt time.Time) error {
	dir := filepath.Dir(name)
	f, err := os.CreateTemp(dir, tempPattern(filepath.Base(name)))
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, perm)
	}
	if err == nil && !mt.IsZero() {
		err = os.Chtimes(tmp, mt, mt)
	}
	if err == nil {
		err = os.Rename(tmp, name)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(dir); err == nil { // the rename itself on the disk (not on every system: no error then)
		d.Sync()
		d.Close()
	}
	return nil
}

// StoreRoot is the root of a store's tree.
const StoreRoot = "/"

func Clean(name string) string {
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

// CopyTree copies the folder srcRoot of src, all that is under it, to dstRoot
// of dst: a folder library into a file (the first time), and back (an export).
// A link to a file is copied as the file; a link to a folder (it may point
// back up the tree), a link to nothing and what is neither file nor folder are
// left, so that one odd entry never stops a whole copy.
func CopyTree(src FS, srcRoot string, dst FS, dstRoot string) error {
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
			if err := CopyTree(src, from, dst, to); err != nil {
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

// IsLibraryFolder reports whether dir holds a library: a folder with a
// collection in it.
func IsLibraryFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), CollectionFile)); err == nil {
				return true
			}
		}
	}
	return false
}

// LibraryFile is where the library is kept when none is named: the user's data folder. A library already beside the
// program (kitaplik.db there: a portable copy the user set up) stays there, and so does one a program `go run` built
// uses, which is the working folder's. Never inside an app bundle, which an update or a new copy of the app replaces
// with the library in it (#230); one found there is moved out once.
func LibraryFile() string {
	dir := "."
	if exe, err := os.Executable(); err == nil && !strings.Contains(filepath.ToSlash(exe), "/go-build") {
		dir = filepath.Dir(exe)
	}
	return libraryFileIn(dir, dataDir())
}

// libraryFileIn is where the library is for a program in dir, with data the user's data folder ("" when unknown).
func libraryFileIn(dir, data string) string {
	beside := filepath.Join(dir, "kitaplik.db")
	if data == "" {
		return beside
	}
	if inBundle(dir) {
		os.MkdirAll(data, 0o700)
		_ = os.Chmod(data, 0o700)
		to := filepath.Join(data, "kitaplik.db")
		// an update keeps what it replaced beside the bundle, as name.app.old, for a day: the bundle's Contents, and
		// before #627 the whole bundle
		old := strings.Replace(filepath.ToSlash(dir), ".app/Contents/MacOS", ".app.old/MacOS", 1)
		older := strings.Replace(filepath.ToSlash(dir), ".app/Contents/MacOS", ".app.old/Contents/MacOS", 1)
		for _, from := range []string{beside, filepath.Join(filepath.FromSlash(old), "kitaplik.db"), filepath.Join(filepath.FromSlash(older), "kitaplik.db")} {
			if moveLibrary(from, to) {
				break
			}
		}
		return to
	}
	if dir == "." || fileExists(beside) && writable(dir) {
		return beside
	}
	os.MkdirAll(data, 0o700)
	_ = os.Chmod(data, 0o700)
	return filepath.Join(data, "kitaplik.db")
}

// inBundle says whether dir is a macOS app bundle's program folder.
func inBundle(dir string) bool {
	return strings.Contains(filepath.ToSlash(filepath.Clean(dir))+"/", ".app/Contents/MacOS/")
}

func fileExists(name string) bool {
	st, err := os.Stat(name)
	return err == nil && st.Mode().IsRegular()
}

// moveLibrary moves the library file from, with the journal SQLite keeps beside it, to to, when there is one at
// from and none at to; it says whether it did. Across disks it is copied, and the copy is checked before the
// original goes.
func moveLibrary(from, to string) bool {
	if !fileExists(from) || fileExists(to) {
		return false
	}
	for _, ext := range []string{"", "-wal", "-shm"} {
		if !fileExists(from + ext) {
			continue
		}
		if os.Rename(from+ext, to+ext) == nil {
			continue
		}
		b, err := os.ReadFile(from + ext)
		if err != nil || os.WriteFile(to+ext, b, 0o600) != nil {
			os.Remove(to + ext)
			return false
		}
		if got, err := os.ReadFile(to + ext); err != nil || len(got) != len(b) {
			return false
		}
		os.Remove(from + ext)
	}
	return true
}

// writable is whether a file can be made in dir: the library's file and the
// journal SQLite keeps beside it.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".celestine-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// dataDir is the user's folder for the app's data: ~/.local/share/celestine on
// Linux (or $XDG_DATA_HOME), the config folder elsewhere.
func dataDir() string {
	if runtime.GOOS == "linux" {
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, AppName)
		}
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".local", "share", AppName)
		}
		return ""
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, AppName)
	}
	return ""
}

// Batcher is a file system whose writes can be made one change: all of them or none.
// fn receives a transaction-scoped FS to run those reads and writes on.
type Batcher interface {
	Batch(fn func(tx FS) error) error
}

// TimeKeeper is a file system that can write a file over and leave it the time it was last written: a change to a
// page that is not a change of its text (a key the app keeps in its meta) does not move it in the order of the
// pages changed last (#538).
type TimeKeeper interface {
	WriteFileKeepTime(name string, data []byte) error
}

// WriteFileKeepTime writes data over the file name in fsys and leaves it the time it was last written, where fsys
// can (TimeKeeper); errors.ErrUnsupported where it cannot. The file must be there.
func WriteFileKeepTime(fsys FS, name string, data []byte) error {
	if k, ok := fsys.(TimeKeeper); ok {
		return k.WriteFileKeepTime(name, data)
	}
	return errors.ErrUnsupported
}

// StampMeta keeps value under key in the meta of the page file at file, its body and the time it was last written
// left as they were (WriteFileKeepTime). It reports whether it wrote: a page that has a value for key already, or
// whose file cannot be read as a page, is left alone. Its size changes, so a sync still sends it (localFiles).
func StampMeta(fsys FS, file, key, value string) (bool, error) {
	src, err := fsys.ReadFile(file)
	if err != nil {
		return false, err
	}
	meta, body, err := SplitPageChecked(src)
	if err != nil {
		return false, nil // a file that cannot be read is not written over (#125)
	}
	if meta[key] != "" {
		return false, nil
	}
	meta[key] = value
	out, err := JoinPageChecked(meta, body)
	if err != nil {
		return false, err
	}
	if err := WriteFileKeepTime(fsys, file, out); err != nil {
		return false, err
	}
	return true, nil
}

// PageIndexTask is the persisted form of a parsed Markdown task for a cached page index.
type PageIndexTask struct {
	Line   int    `json:"l"`
	Done   bool   `json:"d,omitempty"`
	Text   string `json:"t"`
	Due    int64  `json:"u,omitempty"`
	Every  string `json:"e,omitempty"`
	Kind   string `json:"k,omitempty"`
	Inl    int    `json:"i,omitempty"`
	Col    int    `json:"c,omitempty"`
	Folded string `json:"f,omitempty"`
}

// PageIndex is the derived per-page index payload cached by path, mtime and byte size.
type PageIndex struct {
	Path   string          `json:"-"`
	MTime  int64           `json:"-"`
	Size   int64           `json:"-"`
	Tasks  []PageIndexTask `json:"t,omitempty"`
	Tags   []string        `json:"g,omitempty"`
	Links  []string        `json:"l,omitempty"`
	Words  int             `json:"w"`
	Empty  bool            `json:"e,omitempty"`
	Folded string          `json:"x,omitempty"`
}

// PrewarmRow is one node returned by ReadPrewarmFrom: either a fresh cached Index blob or raw page Src.
type PrewarmRow struct {
	MTime int64
	Size  int64
	Index []byte
	Src   []byte
}

// sealedBatcher is a Batcher whose Batch other goroutines' reads and writes wait for: the store of a file, which
// has one connection. The browser's store only holds its writes back.
type sealedBatcher interface {
	Batcher
	sealedBatch()
}

// BatchSeals reports whether a Batch of fsys keeps other goroutines out while it runs: what it reads in it is
// what it writes over, whoever else writes.
func BatchSeals(fsys FS) bool {
	_, ok := fsys.(sealedBatcher)
	return ok
}

// InBatch runs fn, whose writes go to tx, as one change of fsys when it can be one: a reorder that writes every
// sibling is not left half done (#138).
func InBatch(fsys FS, fn func(tx FS) error) error {
	if b, ok := fsys.(Batcher); ok {
		return b.Batch(fn)
	}
	return fn(fsys)
}
