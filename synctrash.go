package main

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

// trashDays is how long what is in the trash stays, before it is deleted for good.
const trashDays = 30

// syncTrash puts the files of gone, which a pass was about to delete, into the trash: a folder of it for
// each folder they were in, with a note of where it was, as Library.trash makes. A file that is the same
// bytes as one already in the trash is deleted outright. A file that cannot be kept is not deleted, and
// is returned with the error. kept are the hashes of the files in the trash.
func syncTrash(fsys FS, root string, gone []string, hash map[string]string, kept map[string]bool, now time.Time) (removed []string, failed map[string]error) {
	type folder struct {
		to   string
		item TrashItem
	}
	abs := func(n string) string { return filepath.Join(root, filepath.FromSlash(n)) }
	bin := filepath.Join(root, trashDir)
	folders := map[string]*folder{}
	failed = map[string]error{}
	for _, n := range gone {
		dir := path.Dir(n)
		if dir == "." || strings.HasPrefix(n, trashDir+"/") { // no folder to keep it in, or it is the trash
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
		if h := hash[n]; h != "" && hashOf(b) != h { // written since the pass looked: it is not taken away, the next pass sees it changed here
			continue
		}
		f := folders[dir]
		if f == nil {
			if err := fsys.MkdirAll(bin); err != nil {
				failed[n] = err
				continue
			}
			to, err := uniqueDir(fsys, bin, now.Format("20060102-150405")+"-sync-"+path.Base(dir))
			if err != nil {
				failed[n] = err
				continue
			}
			kind := "page"
			if !strings.Contains(dir, "/") {
				kind = "collection"
			}
			f = &folder{to: to, item: TrashItem{Kind: kind, Title: path.Base(dir), From: filepath.FromSlash(dir), Trashed: now.Truncate(time.Second)}}
			if kind == "page" {
				f.item.Where = path.Dir(dir)
			}
			folders[dir] = f
		}
		if err := fsys.WriteFile(filepath.Join(f.to, path.Base(n)), b); err != nil {
			failed[n] = err
			continue
		}
		switch path.Base(n) {
		case pageFileName:
			if t := pageMeta(b)["title"]; t != "" && f.item.Kind == "page" {
				f.item.Title = t
			}
		case collectionFile:
			var c Collection
			if json.Unmarshal(b, &c) == nil && c.Name != "" && f.item.Kind == "collection" {
				f.item.Title = c.Name
			}
		}
		if err := removeFile(fsys, abs(n)); err != nil {
			failed[n] = err
			continue
		}
		removed = append(removed, n)
	}
	for _, f := range folders { // the notes, when the folders are full: their titles are known now
		data, _ := json.MarshalIndent(f.item, "", "  ")
		// not written: the files are in the trash without a note, and Trash() reads them as pages from nowhere
		fsys.WriteFile(filepath.Join(f.to, trashNote), data)
	}
	// the folders they were in, if nothing is left in them: an empty one keeps its name from a page made there later
	for _, n := range removed {
		for dir := path.Dir(n); dir != "." && dir != "/" && dir != trashDir; dir = path.Dir(dir) {
			ents, err := fsys.ReadDir(abs(dir))
			if err != nil || len(ents) > 0 || fsys.Remove(abs(dir)) != nil {
				break
			}
		}
	}
	return removed, failed
}

// removeFile deletes a file, which is not an error when it is gone already.
func removeFile(fsys FS, name string) error {
	if err := fsys.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
