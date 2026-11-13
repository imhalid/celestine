package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The trash: what was thrown away, the last first, each to put back where
// it was or to delete for good. Its row is at the bottom of the sidebar; a
// page or collection just thrown away is also offered back by a toast.

// loadTrash reads what is in the trash.
func (a *app) loadTrash() {
	a.trashItems = a.lib.Trash()
	// what each collection took with it, read once here and not in every frame
	a.trashPages = map[string][]*Page{}
	for _, it := range a.trashItems {
		if ps := a.lib.TrashedPages(it); len(ps) > 0 {
			a.trashPages[it.Dir] = ps
		}
	}
}

// purgeTrash deletes what has been in the trash for trashDays: at the start, after a sync, when the trash is opened.
func (a *app) purgeTrash() {
	if a.lib.PurgeTrash(a.clock()) > 0 && a.trashItems != nil {
		a.loadTrash()
	}
}

// trashDaysLeft is the whole days an item has left in the trash.
func trashDaysLeft(item *TrashItem, now time.Time) int {
	left := item.Trashed.Add(TrashDays * 24 * time.Hour).Sub(now)
	return max(0, int((left+24*time.Hour-1)/(24*time.Hour)))
}

// openTrash shows the trash instead of a page.
func (a *app) openTrash() {
	a.flush()
	a.trashOpen, a.coll, a.tag, a.tasksOpen, a.searchOpen, a.settingsOpen = true, nil, "", false, false, false
	a.statsScreen, a.profileScreen = false, false
	a.purgeTrash()
	a.loadTrash()
	a.visit()
}

// restore puts item back and opens it.
func (a *app) restore(item *TrashItem) {
	coll, p, err := a.lib.Restore(item)
	if err != nil {
		a.fail(err)
		return
	}
	a.loadTrash()
	if a.trashOpen {
		a.say(toastOK, locales.TrashRestored(item.Title))
		return
	}
	switch {
	case p != nil:
		a.open(p)
	case coll != nil:
		a.openCollection(coll)
	default: // files of the library's top: nothing to open
		a.say(toastOK, locales.TrashRestored(item.Title))
	}
}

// restoreOutOf puts page p of collection item back without the collection, and says so.
func (a *app) restoreOutOf(item *TrashItem, p *Page) {
	if _, _, err := a.lib.RestoreOutOf(item, p); err != nil {
		a.fail(err)
		return
	}
	a.loadTrash()
	a.say(toastOK, locales.TrashRestored(p.Title))
}

// trashRow is the foot of the sidebar, under its list: the tasks and the trash side by side, each with how much is in
// it. It stays at the bottom, as short as the list is (#489).
func (a *app) trashRow(c *ui.Context) {
	t := c.Theme()
	if a.trashItems == nil {
		a.loadTrash()
		if a.trashItems == nil {
			a.trashItems = []*TrashItem{}
		}
	}
	ui.Box(c.Key("nav-divider")).Height(1).Background(t.Border.Alpha(0.6)).Margin(0, 12, 0, 12)
	ui.Row(c.Key("trash-row")).Padding(6, 8, 10, 8).Gap(4).AlignItems(ui.Center).Children(func() {
		a.tasksRow(c)
		r := ui.Row(c.Key("trash-btn")).Grow(1).Basis(0).MinWidth(0).Height(rowH).Padding(0, 8, 0, 8).Gap(7).Radius(rad(7)).AlignItems(ui.Center).
			Label(locales.CommonTrash()).Role(ui.RoleButton).Transition(rowMove)
		st := navRowState(a.trashOpen, r.Hovered())
		textC, iconC := sideRowLook(c, r, st)
		r.Children(func() {
			ui.Icon(c, iconTrash).Width(15).Height(15).TextColor(iconC)
			ui.Text(c, locales.CommonTrash()).FontSize(14).TextColor(textC).MaxLines(1).Grow(1).MinWidth(0)
			if n := len(a.trashItems); n > 0 {
				ryugo.Badge(c, strconv.Itoa(n), ryugo.ToneNeutral)
			}
		})
		a.kbMark(r, "nav:trash", false, a.openTrash)
		if r.Clicked() {
			a.openTrash()
		}
	})
}

// trashView lists what is in the trash.
func (a *app) trashView(c *ui.Context) {
	t := c.Theme()
	now := a.clock()
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, locales.CommonTrash()).FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		ui.Text(c, locales.TrashThrownAwayPagesCollections()).
			FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 8, 0)
		if len(a.trashItems) == 0 {
			a.emptyState(c, iconTrash, locales.TheTrashIsEmpty(), locales.ThePagesYouThrowAwayWait(), "", nil)
			return
		}
		a.scrolled(c, "trash:96", nil, ui.Scroll(c).Grow(1), func() {
			ui.Column(c).Gap(2).Padding(0, 0, 40, 0).Children(func() {
				for _, item := range a.trashItems {
					a.trashItemRow(c, item, now)
				}
			})
		})
	})
}

func (a *app) trashItemRow(c *ui.Context, item *TrashItem, now time.Time) {
	t := c.Theme()
	c.Key("trash:" + item.Dir)
	row := ui.Row(c).Padding(8, 10).Gap(10).Radius(rad(8)).AlignItems(ui.Center).Transition(rowEnter).Label(locales.TrashInTrashLabel(item.Title))
	a.contextOn(row, func() []menuItem { return a.trashMenu(item) })
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	} else {
		row.Background(t.SurfaceHover.Alpha(0))
	}
	a.kbMark(row, "x:"+item.Dir, true, func() { a.restore(item) })
	row.Children(func() {
		glyph := iconPage
		if item.Kind == "collection" {
			glyph = iconStack
		} else if item.Kind == "files" {
			glyph = iconAttach
		}
		ui.Icon(c, glyph).Width(17).Height(17).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, item.Title).FontSize(15).FontWeight(600).MaxLines(1)
			from := "collection"
			if item.Kind == "page" {
				from = item.Where
				if from == "" {
					from = "page"
				}
			} else if item.Kind == "files" && item.Where != "" { // a page's files: the page they were taken from
				from = item.Where
			}
			note := locales.TrashThrownAway(from, ago(item.Trashed, now))
			if !item.Trashed.IsZero() {
				note += "  ·  " + locales.TrashDeletedIn(trashDaysLeft(item, now))
			}
			ui.Text(c, note).FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
			if ps := a.trashPages[item.Dir]; len(ps) > 0 { // what went with the collection: its count, which unfolds the pages (#481)
				open := a.trashUnfold[item.Dir]
				glyph := iconChevron
				if open {
					glyph = iconChevronDown
				}
				fold := ui.Row(c.Key("trash-fold:" + item.Dir)).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).Role(ui.RoleButton).
					Label(locales.CommonPageCount(len(ps)))
				fold.Children(func() {
					ui.Icon(c, glyph).Width(11).Height(11).TextColor(t.TextMuted)
					ui.Text(c, locales.CommonPageCount(len(ps))).FontSize(12.5).FontWeight(560).TextColor(t.TextMuted)
				})
				if fold.Clicked() {
					if a.trashUnfold == nil {
						a.trashUnfold = map[string]bool{}
					}
					a.trashUnfold[item.Dir] = !open
				}
				if open {
					ui.Column(c.Key("trash-pages:"+item.Dir)).Gap(3).Padding(4, 0, 2, 15).Transition(rowEnter).Children(func() {
						for _, p := range ps {
							ui.Row(c.Key("trash-page:"+p.Dir)).Gap(6).AlignItems(ui.Center).Padding(0, 0, 0, float32(p.Depth())*14).Children(func() {
								ui.Icon(c, iconPage).Width(12).Height(12).TextColor(t.TextMuted)
								ui.Text(c, p.Title).FontSize(13).TextColor(t.Text).MaxLines(1).Shrink(1).MinWidth(0)
								// the page alone, out of the collection: restoring the collection brings them all
								if a.iconButton(c.Key("restore-alone"), iconRestore, locales.TrashRestoreAlone(p.Title)) {
									a.restoreOutOf(item, p)
								}
							})
						}
					})
				}
			}
		})
		if ryugo.Button(c.Key("restore:"+item.Dir), locales.CommonRestore(), ryugo.ButtonOptions{Icon: iconRestore, Size: ryugo.Sm}).Clicked() {
			a.restore(item)
		}
		if ryugo.Button(c.Key("delete:"+item.Dir), locales.TrashDeleteForGoodLabel(item.Title), ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			a.deleteForever(item)
		}
	})
}

// tasksRow is the task view's button, beside the trash's, with how many tasks are late
// or due today: thousands open say nothing, a day's say what is to be done.
// navRowState is the state of a row of the sidebar that opens a screen (Tasks, Trash): open while it shows.
func navRowState(open, hovered bool) rowState {
	switch {
	case open:
		return rowOpen
	case hovered:
		return rowHover
	}
	return rowRest
}

func (a *app) tasksRow(c *ui.Context) {
	late, today := 0, 0
	start := startOfDay(a.clock())
	for _, k := range a.links.tasks {
		switch {
		case k.done || k.due.IsZero():
		case k.due.Before(start):
			late++
		case sameDay(k.due, start):
			today++
		}
	}
	{
		r := ui.Row(c.Key("tasks-row")).Grow(1).Basis(0).MinWidth(0).Height(rowH).Padding(0, 8).Gap(7).Radius(rad(7)).AlignItems(ui.Center).
			Label(locales.CommonTasks()).Role(ui.RoleButton).Transition(rowMove)
		st := navRowState(a.tasksOpen, r.Hovered())
		textC, iconC := sideRowLook(c, r, st)
		r.Children(func() {
			ui.Icon(c, iconTask).Width(15).Height(15).TextColor(iconC)
			ui.Text(c, locales.CommonTasks()).FontSize(14).TextColor(textC).MaxLines(1).Grow(1).MinWidth(0)
			switch {
			case late > 0:
				ryugo.Badge(c.Key("due-badge"), strconv.Itoa(late+today), ryugo.ToneUp)
			case today > 0:
				ryugo.Badge(c.Key("due-badge"), strconv.Itoa(today), ryugo.ToneWarn)
			}
		})
		a.kbMark(r, "nav:tasks", false, a.openTasks)
		if r.Clicked() {
			a.openTasks()
		}
	}
}

// deleteForever asks, then deletes item from the trash for good: a page and its sub-pages, or a collection, with no
// way back.
func (a *app) deleteForever(item *TrashItem) {
	a.ask(locales.TrashDeleteForGood2(), locales.TrashDeleteForGoodConfirm(item.Title), locales.TrashDeleteForGood3(), func() {
		if err := a.lib.DeleteForever(item); err != nil {
			a.fail(err)
		}
		a.loadTrash()
	})
}
