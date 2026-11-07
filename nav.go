package main

// Going back and forward, as a browser does (⌘[ ⌘]): every page, collection
// and the trash shown is a place in the history; one gone since is passed
// over.

// place is something shown: a page or a collection by its folder, or the trash.
type place struct {
	kind string // "page", "coll", "trash"
	dir  string
}

func (a *app) here() place {
	switch {
	case a.settingsOpen:
		return place{kind: "settings"}
	case a.statsScreen:
		return place{kind: "stats"}
	case a.trashOpen:
		return place{kind: "trash"}
	case a.searchOpen:
		return place{"search", a.searchQ}
	case a.tasksOpen:
		return place{kind: "tasks"}
	case a.tag != "":
		return place{"tag", a.tag}
	case a.coll != nil:
		return place{"coll", a.coll.Dir}
	case a.page != nil:
		return place{"page", a.page.Dir}
	}
	return place{}
}

// visit notes the place now shown, unless it was reached by going back or
// forward: what was ahead of it is forgotten.
func (a *app) visit() {
	a.noteVisit()
	if a.going {
		return
	}
	p := a.here()
	if p.kind == "" || (a.histAt >= 0 && a.histAt < len(a.hist) && a.hist[a.histAt] == p) {
		return
	}
	a.hist = append(a.hist[:a.histAt+1], p)
	a.histAt = len(a.hist) - 1
}

// step goes back (-1) or forward (1) to the nearest place still there.
func (a *app) step(dir int) bool {
	for i := a.histAt + dir; i >= 0 && i < len(a.hist); i += dir {
		if a.goTo(a.hist[i]) {
			a.histAt = i
			return true
		}
	}
	return false
}

func (a *app) canStep(dir int) bool {
	i := a.histAt + dir
	return i >= 0 && i < len(a.hist)
}

// goTo shows place p, if it is still there.
func (a *app) goTo(p place) bool {
	a.going = true
	defer func() { a.going = false }()
	if p.kind == "trash" {
		a.openTrash()
		return true
	}
	if p.kind == "stats" {
		a.statsScreen = false
		a.openStats()
		return true
	}
	if p.kind == "settings" {
		a.settingsOpen = false
		a.openSettings()
		return true
	}
	if p.kind == "search" {
		a.openSearch(p.dir)
		return true
	}
	if p.kind == "tasks" {
		a.openTasks()
		return true
	}
	if p.kind == "tag" {
		if len(a.tagPages(p.dir)) == 0 {
			return false
		}
		a.openTag(p.dir)
		return true
	}
	for _, c := range a.lib.Collections {
		if p.kind == "coll" && c.Dir == p.dir {
			a.openCollection(c)
			return true
		}
		if p.kind == "page" {
			for _, q := range c.All() {
				if q.Dir == p.dir {
					for up := q.Parent; up != nil; up = up.Parent {
						up.Open = true // seen in the tree
					}
					c.Open = true
					a.open(q)
					return true
				}
			}
		}
	}
	return false
}
