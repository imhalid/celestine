package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// A dynamic collection is a saved search: it has no pages of its own, and
// shows, in the views a collection has, the pages of the library its query
// finds (the search's: words, "phrases", başlık:, koleksiyon:, #etiket). It is
// made from the search ("Koleksiyon olarak kaydet"), its query is changed in
// its settings, and what it shows is always what the library has now.

// Dynamic reports whether the collection is a saved search.
func (c *Collection) Dynamic() bool { return strings.TrimSpace(c.Query) != "" }

// saveSearch makes a collection of the search q, and opens it.
func (a *app) saveSearch(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	name := q
	if r := []rune(name); len(r) > 36 {
		name = string(r[:36]) + "…"
	}
	c, err := a.lib.NewCollection(name)
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	c.Query, c.SubPages, c.Icon = q, false, "search"
	if err := c.save(); err != nil {
		a.saveErr = err.Error()
	}
	a.openCollection(c)
}

// saveSearchButton is the search view's button to keep the query as a collection.
func (a *app) saveSearchButton(c *ui.Context) {
	if strings.TrimSpace(a.searchQ) == "" {
		return
	}
	if ryugo.Button(c.Key("search-save"), "Koleksiyon olarak kaydet", ryugo.ButtonOptions{Icon: iconSearch, Size: ryugo.Sm}).Clicked() {
		a.saveSearch(a.searchQ)
	}
}
