package main

import (
	"context"
	"log"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Watching the library for changes made elsewhere: another editor, a
// script, a sync (iCloud, Dropbox, git) or an agent writing the files.
// When the system says something changed (watch), the disk is compared
// with what the app holds: the folders of its collections and pages, and
// when each file was last read or written here. Our own writes leave the two the same; anything else is
// a change from outside, and the app takes it:
//   - a page's text changed: its document is read again, unless it has
//     edits not yet saved - then both are kept, the outside version next
//     to the page as "page (çakışma …).json", ours saved over it;
//   - pages or collections came, went or moved, or a collection.json
//     changed: the library is read again, the open page and folds kept;
//   - the open page was deleted with edits not yet saved: it is written
//     back, so that nothing typed is lost.

// watchEvery is how often the library is looked at where the system does not tell what changes in it: a library of
// another kind of storage, a system without file watching, or one whose watch ended (too many files, its folder gone).
var watchEvery = 1500 * time.Millisecond

// watch follows the disk for as long as ctx lasts: through the system's watch of a folder library
// (mygo.WatchFiles, #344), which says at once that something changed and costs nothing while nothing does, else
// every watchEvery. Either way what changed is read by scanLibrary and taken by applyDisk, which tell the app's own
// writes from others: an event only says when to look.
func (a *app) watch(ctx context.Context) {
	for ctx.Err() == nil {
		a.mu.Lock()
		fsys, root := a.lib.FS, a.lib.Root
		a.mu.Unlock()
		if onDisk(fsys) && a.watchFolder(ctx, fsys, root) {
			continue // the library is another now: watched again
		}
		if !a.pollDisk(ctx, fsys, root) {
			return
		}
	}
}

// watchFolder watches the folder library at root until ctx ends or the library is another, which it reports.
// It returns false at once where the system cannot watch it, and once its watch ended, so that the library is
// polled instead.
func (a *app) watchFolder(ctx context.Context, fsys FS, root string) bool {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w, err := mygo.WatchFiles(wctx, root, mygo.FileWatchOptions{
		Recursive:    true,
		PollInterval: watchEvery, // where the system has no watch
		// what scanLibrary passes over: the trash, the app's own folders, hidden files
		Ignore: func(p string, _ bool) bool { return strings.HasPrefix(path.Base(p), ".") },
	})
	if err != nil {
		log.Printf("celestine: watching the library every %v instead: %v", watchEvery, err)
		return false
	}
	changed := make(chan error)
	go func() {
		defer a.guard()
		for {
			_, err := w.Next(wctx)
			select {
			case changed <- err:
			case <-wctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	a.scanDisk(fsys, root) // what changed before the watch began
	tick := time.NewTicker(watchEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case err := <-changed:
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("celestine: watching the library every %v instead: %v", watchEvery, err)
				}
				return false
			}
			a.scanDisk(fsys, root)
		case <-tick.C: // nothing read: only whether the library is still this one
			if !a.sameLibrary(fsys, root) {
				return true
			}
		}
	}
}

// pollDisk looks at the library every watchEvery until ctx ends, reporting false then, or the library is another
// that the system can watch.
func (a *app) pollDisk(ctx context.Context, fsys FS, root string) bool {
	tick := time.NewTicker(watchEvery) // one scan at a time: a tick that comes during one is dropped
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
		if !a.sameLibrary(fsys, root) {
			return true
		}
		if _, ok := fsys.(*Store); ok {
			continue // nothing else reads or writes a library's file: no one to wait for
		}
		a.scanDisk(fsys, root)
	}
}

// onDisk says whether fsys is the computer's own disk, which the system can watch: a folder library's.
func onDisk(fsys FS) bool {
	if c, ok := fsys.(*countFS); ok {
		fsys = c.FS
	}
	_, ok := fsys.(OSFS)
	return ok
}

// sameLibrary says whether the app's library is still the one in fsys at root.
func (a *app) sameLibrary(fsys FS, root string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lib.FS == fsys && a.lib.Root == root
}

// scanDisk reads the library in fsys at root, the slow part, outside the lock: no frame waits for the disk (#193);
// then takes what changed, if the library is still that one.
func (a *app) scanDisk(fsys FS, root string) {
	disk := scanLibrary(fsys, root)
	a.do(func() error {
		if a.lib.FS == fsys && a.lib.Root == root {
			a.applyDisk(disk, true)
		}
		return nil
	})
}

// diskReload says whether disk shows the tree changed: a collection or a page there or gone, a collection.json
// written.
func (a *app) diskReload(disk diskState) bool {
	if len(disk.colls) != len(a.lib.Collections) {
		return true
	}
	pages := 0
	for _, c := range a.lib.Collections {
		if t, ok := disk.colls[c.Dir]; !ok || !t.Equal(c.Seen) {
			return true
		}
		for _, p := range c.All() {
			pages++
			if _, ok := disk.pages[p.Dir]; !ok {
				return true
			}
		}
	}
	return pages != len(disk.pages)
}

// diskState is the library as the disk has it.
type diskState struct {
	colls map[string]time.Time // collection folders: their collection.json's time
	pages map[string]time.Time // page folders: their index.md's time
}

func scanLibrary(fsys FS, root string) diskState {
	d := diskState{colls: map[string]time.Time{}, pages: map[string]time.Time{}}
	entries, _ := fsys.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		info, err := fsys.Stat(filepath.Join(dir, CollectionFile))
		if err != nil {
			continue
		}
		d.colls[dir] = info.ModTime()
		scanPages(fsys, dir, d.pages)
	}
	return d
}

func scanPages(fsys FS, dir string, into map[string]time.Time) {
	entries, _ := fsys.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		info, err := fsys.Stat(filepath.Join(pdir, PageFileName))
		if err != nil {
			continue
		}
		into[pdir] = info.ModTime()
		scanPages(fsys, pdir, into)
	}
}

// checkDisk takes the changes made to the library from outside.
func (a *app) checkDisk() { a.applyDisk(scanLibrary(a.lib.FS, a.lib.Root), false) }

// applyDisk takes the changes disk shows. A scan made outside the lock (late) may be older than what the app wrote
// since: what it says changed is looked at again first, and a reload it calls for is checked with a scan made now,
// so that a page saved meanwhile is not taken for one changed outside.
func (a *app) applyDisk(disk diskState, late bool) {
	if late {
		if a.diskReload(disk) {
			disk = scanLibrary(a.lib.FS, a.lib.Root)
		} else {
			fresh := diskState{colls: disk.colls, pages: map[string]time.Time{}}
			for d, t := range disk.pages {
				fresh.pages[d] = t
			}
			for _, c := range a.lib.Collections {
				for _, p := range c.All() {
					if t, ok := disk.pages[p.Dir]; ok && !t.Equal(p.Modified) {
						fresh.pages[p.Dir] = ModTime(a.lib.FS, p.File())
					}
				}
			}
			disk = fresh
		}
	}
	reload := len(disk.colls) != len(a.lib.Collections)
	pages := 0
	var changed []*Page
	for _, c := range a.lib.Collections {
		if t, ok := disk.colls[c.Dir]; !ok || !t.Equal(c.Seen) {
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
	// the open pages first: what is typed in them is never lost
	for _, st := range a.ps.states {
		if p := st.page; p != nil && st.dirty {
			if _, ok := disk.pages[p.Dir]; !ok {
				if err := a.lib.FS.MkdirAll(p.Dir); err == nil {
					a.savePane(st)
					a.say(toastOK, locales.PageRestoredAfterDelete(p.Title))
					return // the next look sees it back
				}
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
	src, err := p.FS().ReadFile(p.File())
	if err != nil {
		return
	}
	meta, body, bad := SplitPageChecked(src)
	if bad == nil {
		bad = bodyReadable(body)
	}
	p.Modified = ModTime(p.FS(), p.File())
	// what was shown of a page that could not be read is not the page: nothing of it is kept over the file
	var ours *paneState
	for _, st := range a.ps.states {
		if st.page == p && st.dirty && p.Unreadable == "" {
			ours = st
			break
		}
	}
	if bad != nil && ours == nil { // a file that cannot be read now: its meta and text are not taken for
		// the page's, it is shown read-only, and nothing is written over it (#125)
		p.Unreadable = bad.Error()
		return
	}
	if t := meta["title"]; bad == nil && t != "" {
		p.Title = t
		PagesChanged()
		for _, st := range a.ps.states {
			if st.page == p && !st.dirty {
				st.title = t
			}
		}
	}
	if bad == nil {
		p.ReadMeta(meta)
		SavesGen.Add(1) // its values may be others: what is kept from them is made again
		// it reads again: written to again, whether it is open or not, or its numbering, values and moves on a
		// board would be refused until it was opened (#541)
		p.Unreadable = ""
	}
	doc := a.docs[p]
	if doc == nil {
		return
	}
	if ours != nil {
		// both kept: theirs next to the page, ours over it
		name := "page (çakışma " + time.Now().Format("2006-01-02 15-04-05") + ").json"
		if err := p.FS().WriteFile(filepath.Join(p.Dir, name), src); err != nil {
			a.fail(err)
			return
		}
		a.savePane(ours)
		a.say(toastOK, locales.PageChangedOutside(p.Title, name))
		return
	}
	doc.SetText(a.parsePage(p, body))
	for _, st := range a.ps.states {
		if st.page == p {
			st.dirty, a.saveErr = false, ""
			_, caret := doc.Selection()
			st.version, st.length, st.caretAt = doc.Version(), doc.Text().Len(), caret
			st.lastSave, st.lastWords = encodeBody(doc.Text()), len(strings.Fields(doc.Text().PlainText()))
		}
	}
}

// reloadLibrary reads the library again from disk, keeping what is open
// and folded, and the documents of pages still there.
func (a *app) reloadLibrary() {
	a.linksStale()
	fresh, err := OpenLibraryIn(a.lib.FS, a.lib.Root)
	if err != nil {
		a.fail(err)
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
	refs := a.paneRefs()
	activePageDir := refs[a.ps.active].page
	*a.lib = *fresh
	a.docs = map[*Page]*richtext.Document{}
	a.docUse = nil // the pages are new ones: what is kept is listed again below
	for _, c := range a.lib.Collections {
		if o, ok := open[c.Dir]; ok {
			c.Open = o
		}
		for _, p := range c.All() {
			if o, ok := open[p.Dir]; ok {
				p.Open = o
			}
			if d := docs[p.Dir]; d != nil {
				a.docs[p] = d
				a.docUse = append(a.docUse, p)
			}
		}
	}
	a.restorePanes(refs)
	a.agentRemap() // what the agents changed, followed to the new pages (#180)
	// what was being done to what is gone is let go
	a.drag = dragState{}
	a.renaming = nil
	a.prop.open = false
	a.loadTrash()
	if pn := a.pane(); pn.page == nil && activePageDir != "" && !a.noPage() { // the open page is gone: its collection, or the first
		for _, c := range a.lib.Collections {
			a.openCollection(c)
			break
		}
	}
}
