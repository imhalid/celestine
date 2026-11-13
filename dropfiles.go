package main

import (
	"path/filepath"
	"strings"

	"github.com/imhalid/celestine/locales"
)

// Files dropped on the window: a Markdown or text file becomes a page - in the
// collection shown, or the open page's, else the first that is not the
// journal's or a saved search - as the import of a folder makes its pages; a
// picture dropped on the text goes into it, and dropped elsewhere on a page
// is its cover.

// dropCollection is the collection a dropped file's page is made in.
func (a *app) dropCollection() *Collection {
	if c := a.currentColl(); c != nil && !c.Journal && !c.Dynamic() {
		return c
	}
	for _, c := range a.lib.Collections {
		if !c.Journal && !c.Dynamic() {
			return c
		}
	}
	return nil
}

// dropFile takes a file dropped on the window; overText is that it was dropped
// on the text of the open page.
func (a *app) dropFile(f string, overText bool) {
	switch strings.ToLower(filepath.Ext(f)) {
	case ".md", ".markdown", ".txt":
		c := a.dropCollection()
		if c == nil {
			a.say(toastWarn, locales.ThereIsNoCollectionToAdd())
			return
		}
		a.flush()
		st := importStats{noWalk: true}
		p, err := a.importFile(c, filepath.Dir(f), f, &st)
		if err != nil {
			a.say(toastErr, locales.CommonDidntWork(err.Error()))
			return
		}
		a.linksStale()
		a.open(p)
		if st.Pictures > 0 {
			a.say(toastOK, locales.PageAddedToWithPictures(p.Title, c.Name, st.Pictures))
		} else {
			a.say(toastOK, locales.PageAddedTo(p.Title, c.Name))
		}
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		switch {
		case overText && a.page != nil && a.doc != nil && !a.noPage():
			a.addImageFile(f)
		case a.page != nil && !a.noPage():
			a.coverFile(a.page, f)
		default:
			a.say(toastWarn, locales.DropThePictureOnAPage())
		}
	default:
		if overText && a.page != nil && a.doc != nil && !a.noPage() { // any other file goes into the page, beside it
			a.addFileFrom(f)
			return
		}
		a.say(toastWarn, locales.PageNotPageType(filepath.Base(f)))
	}
}
