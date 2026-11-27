package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/locales"
)

// A file in the text: any file that is not a picture, put in the page as a chip with its name, the file itself
// copied into the page's folder beside it. A click opens it with the system's program for it; in Markdown it is a
// link to the file.

// safeAttachName is whether name, a file's name as a page keeps it, stays in the page's folder: not empty, not the
// folder itself, no "..", no root and no drive. A name that does not is kept in the page as it was written, and is
// never opened, revealed or removed (#166).
func safeAttachName(name string) bool {
	if name == "" || !filepath.IsLocal(name) || !SafeName(filepath.ToSlash(name)) {
		return false
	}
	return filepath.Clean(name) != "."
}

// validAttachFileName is safeAttachName for a name that is one file, with no folder in it: a file put in a page.
func validAttachFileName(base string) bool {
	return !strings.ContainsAny(base, `/\`) && safeAttachName(base)
}

// openedCopies is the folder that the one-file library's files are copied into to be opened by the system's
// programs, one folder a page: in the user's cache, not the shared temporary folder, and emptied when the app starts
// and when it quits.
func openedCopies() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "celestine", "opened")
}

// clearOpenedCopies removes the copies of files opened from the one-file library. At the start it also takes what a
// session that did not end as it should left behind.
func clearOpenedCopies() { os.RemoveAll(openedCopies()) }

// shellOpenSoon is runShellOpen away from the window, with a panic in it logged rather than lost (guard).
func (a *app) shellOpenSoon(path string, reveal bool) {
	go func() {
		defer a.guard()
		a.runShellOpen(path, reveal)
	}()
}

// runShellOpen gives path to the system: opened with its program, or shown in its folder. MyGo makes the call on
// the main thread itself (onMain), from any goroutine.
func (a *app) runShellOpen(path string, reveal bool) {
	if a.shellOpen != nil { // a test sees what would be opened
		a.shellOpen(path, reveal)
		return
	}
	if reveal {
		mygo.Shell.ShowItemInFolder(path)
		return
	}
	mygo.Shell.OpenPath(path)
}

// openAttachedFile opens the file name of page p with the system's program, or shows it in its folder. A name that
// leaves the page's folder is refused before anything is touched. A file of the one-file library has no place on the
// disk: it is copied out first, away from the window, and the copy is opened; what is changed in that copy is not
// in the library, and the notice says so.
func (a *app) openAttachedFile(p *Page, name string, reveal bool) {
	if p == nil || !safeAttachName(name) {
		return
	}
	file := filepath.Join(p.Dir, name)
	if a.store() == nil {
		a.runShellOpen(file, reveal)
		return
	}
	fsys := p.FS()
	sum := sha256.Sum256([]byte(p.Dir))
	dir := filepath.Join(openedCopies(), hex.EncodeToString(sum[:8]))
	go func() {
		defer a.guard()
		path, err := copyOut(fsys, file, dir)
		if err != nil {
			a.do(func() error { a.fail(err); return nil })
			return
		}
		a.runShellOpen(path, reveal)
		a.do(func() error { a.say(toastInfo, locales.PageOpenedCopy()); return nil })
	}()
}

// copyOut writes file of fsys into dir under its own name, the folders made only for the user (0700) and the copy
// readable only by them (0600), and gives the copy's path.
func copyOut(fsys FS, file, dir string) (string, error) {
	data, err := fsys.ReadFile(file)
	if err != nil {
		return "", err
	}
	for _, d := range []string{filepath.Dir(dir), dir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
		if err := os.Chmod(d, 0o700); err != nil {
			return "", err
		}
	}
	path := filepath.Join(dir, filepath.Base(file))
	os.Remove(path) // a copy opened before, or a link put in its place, is not written through
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// addFileFrom copies the file at src into the open page's folder (its name kept, numbered when it is taken) and puts
// its chip in a line of its own after the caret's line.
func (a *app) addFileFrom(src string) {
	p := a.pane().page
	if p == nil || a.pane().doc == nil || a.noPage() {
		a.say(toastInfo, locales.PageDropFilePage())
		return
	}
	if a.pageLocked(p) {
		a.sayLocked()
		return
	}
	base := filepath.Base(src)
	if !validAttachFileName(base) {
		a.say(toastErr, locales.CommonInvalidAttachmentName(base))
		return
	}
	a.readThen(src, maxFileBytes, func(data []byte, err error) { // a big file away from the window (#177)
		if a.pane().page != p || a.pane().doc == nil {
			return
		}
		if a.pageLocked(p) {
			a.sayLocked()
			return
		}
		if err != nil {
			a.say(toastErr, err.Error())
			return
		}
		name := freeName(p, base)
		if err := p.FS().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
			a.fail(err)
			return
		}
		a.putBlock(a.attachment(&chip{kind: "file", target: name}))
	})
}

// pickFileToAttach asks for a file in a dialog and puts it in the page.
func (a *app) pickFileToAttach() {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: locales.CommonChooseFile()})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error { a.addFileFrom(paths[0]); return nil })
	}()
}
