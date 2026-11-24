package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// The libraries opened lately, in the system's recent documents (#359): MyGo's AddRecentDocument puts the library
// the app opened - its file, or the folder of a folder library - in the Dock's menu on macOS, the jump list on
// Windows and the recently used files on Linux. Choosing one there opens it: the app comes to the front when it is
// the library open, and starts again with the other one when it is not, as a library is chosen on the command line.

// rememberLibrary records the open library with the system; from WhenReady.
func (a *app) rememberLibrary() {
	if a.libPath == "" {
		return
	}
	if err := mygo.App.AddRecentDocument(a.libPath); err != nil {
		log.Printf("recent libraries: %v", err)
	}
}

// watchRecent opens a library chosen from the system's recent documents; from main before Run, so a choice that
// launched the app is told too.
func (a *app) watchRecent() {
	mygo.App.OnOpenFile(func(path string) {
		if !isLibraryPath(path) {
			return
		}
		if sameFile(path, a.libPath) {
			a.bringToFront()
			return
		}
		a.do(func() error { a.flush(); return nil }) // what is written goes to its file before the app starts again
		a.do(func() error { a.relaunch(path); return nil })
	})
}

// isLibraryPath reports whether path is a library the app opens: a folder of pages, or a library's file.
func isLibraryPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir() || strings.EqualFold(filepath.Ext(path), ".db")
}

// sameFile reports whether a and b are one file or folder.
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ia, ea := os.Stat(a)
	ib, eb := os.Stat(b)
	return ea == nil && eb == nil && os.SameFile(ia, ib)
}
