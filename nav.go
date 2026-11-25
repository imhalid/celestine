package main

// Going back and forward, as a browser does (⌘[ ⌘]): every page, collection
// and the trash shown is a place in the history; one gone since is passed
// over.

// place is something shown: a page or a collection by its folder, or the trash.
type place struct {
	kind string // "page", "coll", "trash"
	dir  string
}

func (a *app) here() place { return a.hereOf(a.pane()) }

func (a *app) hereOf(pn *paneState) place {
	switch {
	case pn.settingsOpen:
		return place{kind: "settings"}
	case pn.statsScreen:
		return place{kind: "stats"}
	case pn.profileScreen:
		return place{kind: "profile"}
	case pn.trashOpen:
		return place{kind: "trash"}
	case pn.searchOpen:
		return place{"search", pn.searchQ}
	case pn.tasksOpen:
		return place{kind: "tasks"}
	case pn.tag != "":
		return place{"tag", pn.tag}
	case pn.coll != nil:
		return place{"coll", pn.coll.Dir}
	case pn.page != nil:
		return place{"page", pn.page.Dir}
	}
	return place{}
}

// visit notes the place now shown in the active pane, unless it was reached by going back or
// forward: what was ahead of it is forgotten.
func (a *app) visit() { a.visitIn(a.pane()) }

func (a *app) visitIn(pn *paneState) {
	a.noteVisitIn(pn)
	if a.going {
		return
	}
	p := a.hereOf(pn)
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

// goTo shows place p in the active pane, if it is still there.
func (a *app) goTo(p place) bool { return a.goToIn(a.pane(), p) }

// goToIn shows place p in pn, if it is still there.
func (a *app) goToIn(pn *paneState, p place) bool {
	a.going = true
	defer func() { a.going = false }()
	if p.kind == "trash" {
		a.openTrashIn(pn)
		return true
	}
	if p.kind == "stats" {
		pn.statsScreen = false
		a.openStatsIn(pn)
		return true
	}
	if p.kind == "profile" {
		pn.profileScreen = false
		a.openProfileIn(pn)
		return true
	}
	if p.kind == "settings" {
		pn.settingsOpen = false
		a.openSettingsIn(pn)
		return true
	}
	if p.kind == "search" {
		a.openSearchIn(pn, p.dir)
		return true
	}
	if p.kind == "tasks" {
		a.openTasksIn(pn)
		return true
	}
	if p.kind == "tag" {
		if a.links.into != nil && len(a.tagPages(p.dir)) == 0 {
			return false
		}
		a.openTagIn(pn, p.dir)
		return true
	}
	for _, c := range a.lib.Collections {
		if p.kind == "coll" && c.Dir == p.dir {
			a.openCollectionIn(pn, c)
			return true
		}
		if p.kind == "page" {
			for _, q := range c.All() {
				if q.Dir == p.dir {
					for up := q.Parent; up != nil; up = up.Parent {
						up.Open = true // seen in the tree
					}
					c.Open = true
					a.openIn(pn, q)
					return true
				}
			}
		}
	}
	return false
}
