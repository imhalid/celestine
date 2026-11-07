package main

import (
	"path/filepath"
	"strings"
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
			a.notice = "Sayfa eklenecek koleksiyon yok"
			return
		}
		a.flush()
		var st importStats
		p, err := a.importFile(c, filepath.Dir(f), f, &st)
		if err != nil {
			a.notice = "Olmadı: " + err.Error()
			return
		}
		a.linksStale()
		a.open(p)
		a.notice = Tf("“%s” sayfası “%s” koleksiyonuna eklendi", p.Title, c.Name) + map[bool]string{true: Tf(" (%d resim)", st.Pictures), false: ""}[st.Pictures > 0]
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		switch {
		case overText && a.page != nil && a.doc != nil && !a.noPage():
			a.addImageFile(f)
		case a.page != nil && !a.noPage():
			a.coverFile(a.page, f)
		default:
			a.notice = "Resmi bir sayfaya bırak"
		}
	default:
		a.notice = "Bu dosya türü sayfa olmaz: " + filepath.Base(f)
	}
}
