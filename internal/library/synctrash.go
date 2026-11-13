package library

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// What a sync takes away from here, because the remote no longer has it, is not deleted: it goes into the
// trash (trashDir), where it waits trashDays to be put back (see Library.PurgeTrash). A page deleted on
// another device already comes here in the trash, moved: what it was is then the same bytes as a file in the
// trash, and is not kept twice.

// TrashDays is how long what is in the trash stays, before it is deleted for good.
const TrashDays = 30

// LocalTrash starts the name of a folder of the trash that is this device's alone: the sync leaves it out, both
// ways. A sync keeps there the files it takes away from a page that stays (an attachment, a picture): each device
// the removal comes to keeps them for itself, and none uploads them again as something new (#159, #456).
const LocalTrash = "here-"

// localTrash is whether name, a path from the library's root, is in a folder of the trash that is not synced.
func localTrash(name string) bool {
	return strings.HasPrefix(name, TrashDir+"/"+LocalTrash)
}

// syncTrash puts the files of gone, which a pass was about to delete, into the trash: a folder of it for
// each folder they were in, with a note of where it was, as Library.trash makes. A file that is the same
// bytes as one already in the trash is deleted outright. A file that cannot be kept is not deleted, and
// is returned with the error. kept are the hashes of the files in the trash.
//
// A folder whose page (or collection) file is among gone is a page (or collection) of the trash. Files taken
// from a folder that stays are an item of kind "files" for it, put back there (Library.Restore), or with its
// page when the page is put back. here is whether the removal was made on this device (TrashFiles): its item is
// synced like the rest of the trash; one a sync brings is kept in a folder of LocalTrash.
func syncTrash(fsys FS, root string, gone []string, hash map[string]string, kept map[string]bool, now time.Time, here bool) (removed []string, failed map[string]error) {
	type folder struct {
		to    string
		item  TrashItem
		names []string
	}
	abs := func(n string) string { return filepath.Join(root, filepath.FromSlash(n)) }
	bin := filepath.Join(root, TrashDir)
	folders := map[string]*folder{}
	failed = map[string]error{}
	goneSet := make(map[string]bool, len(gone))
	for _, n := range gone {
		goneSet[n] = true
	}
	for _, n := range gone {
		dir := path.Dir(n)
		if strings.HasPrefix(n, TrashDir+"/") { // it is the trash
			if err := removeFile(fsys, abs(n)); err != nil {
				failed[n] = err
				continue
			}
			removed = append(removed, n)
			continue
		}
		if kept[hash[n]] { // it is in the trash already, moved there from the other device
			if !localIs(fsys, abs(n), hash[n]) { // written since the pass looked
				continue
			}
			if err := removeFile(fsys, abs(n)); err != nil {
				failed[n] = err
				continue
			}
			removed = append(removed, n)
			continue
		}
		b, err := fsys.ReadFile(abs(n))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) { // not here after all: nothing to keep
				removed = append(removed, n)
				continue
			}
			failed[n] = err
			continue
		}
		if h := hash[n]; h != "" && HashOf(b) != h { // written since the pass looked: it is not taken away, the next pass sees it changed here
			continue
		}
		f := folders[dir]
		if f == nil {
			kind := "page"
			switch {
			case dir == ".": // files of the library's top: kept together, put back where they were (#147)
				kind = "files"
			case !goneSet[dir+"/"+PageFileName] && !goneSet[dir+"/"+CollectionFile]: // files of a folder that stays
				kind = "files"
			case !strings.Contains(dir, "/"):
				kind = "collection"
			}
			if err := fsys.MkdirAll(bin); err != nil {
				failed[n] = err
				continue
			}
			base := path.Base(dir)
			if dir == "." {
				base = "files"
			}
			name := now.Format("20060102-150405") + "-sync-" + base
			if kind == "files" && !here {
				name = LocalTrash + name
			}
			to, err := UniqueDir(fsys, bin, name)
			if err != nil {
				failed[n] = err
				continue
			}
			f = &folder{to: to, item: TrashItem{Kind: kind, Title: path.Base(dir), From: filepath.FromSlash(dir), Trashed: now.Truncate(time.Second)}}
			switch {
			case dir == ".":
				f.item.Title, f.item.From = "Files of the library", ""
			case kind == "files":
				f.item.Where = folderTitle(fsys, abs(dir))
			case kind == "page":
				f.item.Where = path.Dir(dir)
			}
			folders[dir] = f
		}
		if err := fsys.WriteFile(filepath.Join(f.to, path.Base(n)), b); err != nil {
			failed[n] = err
			continue
		}
		switch path.Base(n) {
		case PageFileName:
			if t := PageMeta(b)["title"]; t != "" && f.item.Kind == "page" {
				f.item.Title = t
			}
		case CollectionFile:
			var c Collection
			if json.Unmarshal(b, &c) == nil && c.Name != "" && f.item.Kind == "collection" {
				f.item.Title = c.Name
			}
		}
		if err := removeFile(fsys, abs(n)); err != nil {
			failed[n] = err
			continue
		}
		f.names = append(f.names, path.Base(n))
		removed = append(removed, n)
	}
	for dir, f := range folders { // the notes, when the folders are full: their titles are known now
		if f.item.Kind == "files" && dir != "." && len(f.names) > 0 { // named by what is in it: the page is where it was
			f.item.Title = strings.Join(f.names, ", ")
		}
		data, _ := json.MarshalIndent(f.item, "", "  ")
		// not written: the files are in the trash without a note, and Trash() reads them as pages from nowhere
		fsys.WriteFile(filepath.Join(f.to, TrashNote), data)
	}
	// the folders they were in, if nothing is left in them: an empty one keeps its name from a page made there later
	for _, n := range removed {
		for dir := path.Dir(n); dir != "." && dir != "/" && dir != TrashDir; dir = path.Dir(dir) {
			ents, err := fsys.ReadDir(abs(dir))
			if err != nil || len(ents) > 0 || fsys.Remove(abs(dir)) != nil {
				break
			}
		}
	}
	return removed, failed
}

// folderTitle is the title of the page (or the name of the collection) in folder dir, or the folder's name.
func folderTitle(fsys FS, dir string) string {
	if b, err := fsys.ReadFile(filepath.Join(dir, PageFileName)); err == nil {
		if t := PageMeta(b)["title"]; t != "" {
			return t
		}
	}
	if b, err := fsys.ReadFile(filepath.Join(dir, CollectionFile)); err == nil {
		var c Collection
		if json.Unmarshal(b, &c) == nil && c.Name != "" {
			return c.Name
		}
	}
	return filepath.Base(dir)
}

// removeFile deletes a file, which is not an error when it is gone already.
func removeFile(fsys FS, name string) error {
	if err := fsys.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// TrashFiles moves files of page p's folder, by name, into the trash, as a sync's removals go: a folder of the
// trash for them, under the page's name, from which they can be taken back for 30 days (#126).
func (l *Library) TrashFiles(p *Page, names []string, now time.Time) error {
	rel, err := filepath.Rel(l.Root, p.Dir)
	if err != nil {
		return err
	}
	gone := make([]string, len(names))
	for i, n := range names {
		gone[i] = path.Join(filepath.ToSlash(rel), n)
	}
	_, failed := syncTrash(l.FS, l.Root, gone, map[string]string{}, map[string]bool{}, now, true)
	for _, err := range failed {
		return err
	}
	return nil
}

// restoreFiles puts the files of item, of kind "files", back in the folder they were taken from: the library's
// top, or a page's folder. When that folder is not in the library (its page is in the trash), nothing moves: the
// files come back with the page. A file of the same name there is not written over, and nothing moves either.
func (l *Library) restoreFiles(item *TrashItem) error {
	to := filepath.Join(l.Root, item.From)
	if fi, err := l.FS.Stat(to); err != nil || !fi.IsDir() {
		return errors.New(item.Where + ": the page is not in the library; put it back first, and its files come with it")
	}
	ents, err := l.FS.ReadDir(item.Dir)
	if err != nil {
		return err
	}
	for _, e := range ents { // all checked first: none moves when one cannot
		if e.IsDir() || e.Name() == TrashNote {
			continue
		}
		if _, err := l.FS.Stat(filepath.Join(to, e.Name())); err == nil {
			return errors.New(e.Name() + ": a file of that name is there already")
		}
	}
	for _, e := range ents {
		if e.IsDir() || e.Name() == TrashNote {
			continue
		}
		if err := l.FS.Rename(filepath.Join(item.Dir, e.Name()), filepath.Join(to, e.Name())); err != nil {
			return err
		}
	}
	return l.FS.RemoveAll(item.Dir)
}

// filesBackWith puts back, into to, the files the trash keeps for the folder from (items of kind "files"), as the
// page that was there comes back from the trash into to. A file of a name that is there already stays in the trash.
func (l *Library) filesBackWith(from, to string) {
	if from == "" || from == "." {
		return
	}
	for _, it := range l.Trash() {
		if it.Kind != "files" || it.From != from {
			continue
		}
		ents, err := l.FS.ReadDir(it.Dir)
		if err != nil {
			continue
		}
		left := false
		for _, e := range ents {
			if e.IsDir() || e.Name() == TrashNote {
				continue
			}
			if _, err := l.FS.Stat(filepath.Join(to, e.Name())); err == nil || l.FS.Rename(filepath.Join(it.Dir, e.Name()), filepath.Join(to, e.Name())) != nil {
				left = true
			}
		}
		if !left {
			l.FS.RemoveAll(it.Dir)
		}
	}
}
