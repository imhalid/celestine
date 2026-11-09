package main

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// A file in the text: any file that is not a picture, put in the page as a chip with its name, the file itself
// copied into the page's folder beside it. A click opens it with the system's program for it; in Markdown it is a
// link to the file.

// addFileFrom copies the file at src into the open page's folder (its name kept, numbered when it is taken) and puts
// its chip in a line of its own after the caret's line.
func (a *app) addFileFrom(src string) {
	p := a.page
	if p == nil || a.doc == nil || a.noPage() {
		a.notice = T("Drop the file on a page")
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		a.notice = err.Error()
		return
	}
	name := freeName(p, filepath.Base(src))
	if err := p.FS().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
		a.saveErr = err.Error()
		return
	}
	ch := &chip{kind: "file", target: name}
	a.putBlock(a.attachment(ch))
}

// pickFileToAttach asks for a file in a dialog and puts it in the page.
func (a *app) pickFileToAttach() {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: T("Choose a file")})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error { a.addFileFrom(paths[0]); return nil })
	}()
}
