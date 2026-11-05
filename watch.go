package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// Watching the library for changes made elsewhere: another editor, a
// script, a sync (iCloud, Dropbox, git) or an agent writing the files.
// Every watchEvery the disk is compared with what the app holds: the
// folders of its collections and pages, and when each file was last read
// or written here. Our own writes leave the two the same; anything else is
// a change from outside, and the app takes it:
//   - a page's text changed: its document is read again, unless it has
//     edits not yet saved - then both are kept, the outside version next
//     to the page as "index (çakışma …).md", ours saved over it;
//   - pages or collections came, went or moved, or a collection.json
//     changed: the library is read again, the open page and folds kept;
//   - the open page was deleted with edits not yet saved: it is written
//     back, so that nothing typed is lost.

const watchEvery = 1500 * time.Millisecond

// watch checks the disk every watchEvery, for as long as the app runs.
func (a *app) watch() {
	for range time.Tick(watchEvery) {
		a.do(func() error { a.checkDisk(); return nil })
	}
}

// diskState is the library as the disk has it.
type diskState struct {
	colls map[string]time.Time // collection folders: their collection.json's time
	pages map[string]time.Time // page folders: their index.md's time
}

func scanLibrary(root string) diskState {
	d := diskState{colls: map[string]time.Time{}, pages: map[string]time.Time{}}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		info, err := os.Stat(filepath.Join(dir, collectionFile))
		if err != nil {
			continue
		}
		d.colls[dir] = info.ModTime()
		scanPages(dir, d.pages)
	}
	return d
}

func scanPages(dir string, into map[string]time.Time) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		info, err := os.Stat(filepath.Join(pdir, "index.md"))
		if err != nil {
			continue
		}
		into[pdir] = info.ModTime()
		scanPages(pdir, into)
	}
}

// checkDisk takes the changes made to the library from outside.
func (a *app) checkDisk() {
	disk := scanLibrary(a.lib.Root)
	reload := len(disk.colls) != len(a.lib.Collections)
	pages := 0
	var changed []*Page
	for _, c := range a.lib.Collections {
		if t, ok := disk.colls[c.Dir]; !ok || !t.Equal(c.seen) {
			reload = true
		}
		for _, p := range c.All() {
			pages++
			t, ok := disk.pages[p.Dir]
			switch {
			case !ok:
				reload = true
			case !t.Equal(p.Modified):
				changed = append(changed, p)
			}
		}
	}
	if pages != len(disk.pages) {
		reload = true
	}
	// the open page first: what is typed in it is never lost
	if p := a.page; p != nil {
		if _, ok := disk.pages[p.Dir]; !ok && a.dirty {
			if err := os.MkdirAll(p.Dir, 0o755); err == nil {
				a.save()
				a.notice = "“" + p.Title + "” dışarıda silinmişti; yazdıkların kaybolmasın diye geri yazıldı."
				return // the next look sees it back
			}
		}
	}
	for _, p := range changed {
		a.takeOutside(p)
	}
	if reload {
		a.reloadLibrary()
	}
}

// takeOutside takes the text of page p as it is on disk now.
func (a *app) takeOutside(p *Page) {
	src, err := os.ReadFile(p.file())
	if err != nil {
		return
	}
	meta, body := splitFrontMatter(string(src))
	p.Modified = modTime(p.file())
	if t := meta["title"]; t != "" {
		p.Title = t
		if p == a.page && !a.dirty {
			a.title = t
		}
	}
	p.readMeta(meta)
	doc := a.docs[p]
	if doc == nil {
		return
	}
	if p == a.page && a.dirty {
		// both kept: theirs next to the page, ours over it
		name := "index (çakışma " + time.Now().Format("2006-01-02 15-04-05") + ").md"
		if err := writeAtomic(filepath.Join(p.Dir, name), src); err != nil {
			a.saveErr = err.Error()
			return
		}
		a.save()
		a.notice = "“" + p.Title + "” dışarıda da değişti: senin yazdıkların kaydedildi, öteki sürüm yanına “" + name + "” olarak kondu."
		return
	}
	doc.SetText(a.parsePage(p, body))
	if p == a.page {
		_, caret := doc.Selection()
		a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
		a.lastSave = a.pageMarkdown(p, doc.Text())
	}
}

// reloadLibrary reads the library again from disk, keeping what is open
// and folded, and the documents of pages still there.
func (a *app) reloadLibrary() {
	a.linksStale()
	fresh, err := OpenLibrary(a.lib.Root)
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	open := map[string]bool{}
	docs := map[string]*richtext.Document{}
	for _, c := range a.lib.Collections {
		open[c.Dir] = c.Open
		for _, p := range c.All() {
			open[p.Dir] = p.Open
			if d := a.docs[p]; d != nil {
				docs[p.Dir] = d
			}
		}
	}
	pageDir, collDir := "", ""
	if a.page != nil {
		pageDir = a.page.Dir
	}
	if a.coll != nil {
		collDir = a.coll.Dir
	}
	*a.lib = *fresh
	a.docs = map[*Page]*richtext.Document{}
	a.page, a.coll = nil, nil
	for _, c := range a.lib.Collections {
		if o, ok := open[c.Dir]; ok {
			c.Open = o
		}
		if c.Dir == collDir {
			a.coll = c
		}
		for _, p := range c.All() {
			if o, ok := open[p.Dir]; ok {
				p.Open = o
			}
			if d := docs[p.Dir]; d != nil {
				a.docs[p] = d
			}
			if p.Dir == pageDir {
				a.page = p
			}
		}
	}
	// what was being done to what is gone is let go
	a.drag = dragState{}
	a.renaming = nil
	a.prop.open = false
	a.loadTrash()
	if a.page != nil {
		a.doc = a.docs[a.page]
		return
	}
	a.doc = nil
	if pageDir != "" && a.coll == nil && !a.trashOpen { // the open page is gone: its collection, or the first
		for _, c := range a.lib.Collections {
			a.openCollection(c)
			break
		}
	}
}
