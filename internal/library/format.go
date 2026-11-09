package library

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// FormatVersion is the format of the library this app writes: its collections, pages and what they keep. It
// goes up by one when a library written by this app could not be read right by an app of before; an app does
// not write a library of a format newer than its own, and says to update instead.
const FormatVersion = 1

// FormatFile is where a folder library keeps its format: a dot-file at its root, which OpenLibraryIn does not
// take for a collection and the sync leaves out (each device's app writes its own). A library in one file keeps
// it in the file's meta, under FormatKey.
const (
	FormatFile = ".ryudit-format"
	FormatKey  = "format"
)

// NewerFormatError is the error of opening a library written by a newer app, in a format this one does not
// know: nothing of it is written.
type NewerFormatError struct {
	Library int // the library's format
	App     int // this app's: FormatVersion
}

func (e *NewerFormatError) Error() string {
	return fmt.Sprintf("the library is in format %d, newer than this app's (%d): update Ryudit to open it", e.Library, e.App)
}

// metaKeeper is a disk that keeps small things beside its files: the library file (Store).
type metaKeeper interface {
	Meta(key string) []byte
	SetMeta(key string, value []byte) error
}

// LibraryFormat is the format the library at root of fsys says it is written in, 0 when it says none.
func LibraryFormat(fsys FS, root string) int {
	var data []byte
	if m, ok := fsys.(metaKeeper); ok {
		data = m.Meta(FormatKey)
	}
	if data == nil { // a folder library; or one taken into a file (CopyTree), its folder's file with it
		data, _ = fsys.ReadFile(filepath.Join(root, FormatFile))
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// newerFormat is a *NewerFormatError when the library at root of fsys is of a format newer than this app's. It
// is asked before anything is written, the library's folder made too.
func newerFormat(fsys FS, root string) error {
	if n := LibraryFormat(fsys, root); n > FormatVersion {
		return &NewerFormatError{Library: n, App: FormatVersion}
	}
	return nil
}

// markFormat writes this app's format in a library that says none or an older one. One of this app's format is
// not written: a re-read writes nothing.
func markFormat(fsys FS, root string) error {
	if LibraryFormat(fsys, root) == FormatVersion {
		return nil
	}
	data := []byte(strconv.Itoa(FormatVersion) + "\n")
	if m, ok := fsys.(metaKeeper); ok {
		return m.SetMeta(FormatKey, data)
	}
	return fsys.WriteFile(filepath.Join(root, FormatFile), data)
}
