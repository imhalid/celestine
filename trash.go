package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The trash: what was thrown away, the last first, each to put back where
// it was or to delete for good. Its row is at the bottom of the sidebar; a
// page or collection just thrown away is also offered back by a toast.

// loadTrash reads what is in the trash.
func (a *app) loadTrash() { a.trashItems = a.lib.Trash() }

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
		a.saveErr = err.Error()
		return
	}
	a.loadTrash()
	if a.trashOpen {
		a.notice = Tf("“%s” geri yüklendi", item.Title)
		return
	}
	if p != nil {
		a.open(p)
	} else {
		a.openCollection(coll)
	}
}

// trashRow is the trash's row at the foot of the sidebar, with how much is in it.
func (a *app) trashRow(c *ui.Context) { // after the tasks, in the list: they scroll with it
	t := c.Theme()
	if a.trashItems == nil {
		a.loadTrash()
		if a.trashItems == nil {
			a.trashItems = []*TrashItem{}
		}
	}
	ui.Box(c.Key("nav-divider")).Height(1).Background(t.Border.Alpha(0.6)).Margin(12, 12, 0, 12)
	a.tasksRow(c)
	ui.Row(c).Padding(6, 8, 10, 8).Gap(4).AlignItems(ui.Center).Children(func() {
		c.Key("trash-row")
		r := ui.Row(c).Height(rowH).Padding(0, 8, 0, 8).Gap(7).Radius(rad(7)).AlignItems(ui.Center).
			Label("Çöp").Role(ui.RoleButton).Transition(rowMove)
		switch {
		case a.trashOpen:
			r.Background(t.Selection)
		case r.Hovered():
			r.Background(t.SurfaceHover)
		default:
			r.Background(t.Surface.Alpha(0))
		}
		r.Children(func() {
			ui.Icon(c, iconTrash).Width(15).Height(15).TextColor(t.TextMuted)
			ui.Text(c, T("Çöp")).FontSize(13.5).Grow(1)
			if n := len(a.trashItems); n > 0 {
				ryugo.Badge(c, strconv.Itoa(n), ryugo.ToneNeutral)
			}
		})
		a.kbMark(r, "nav:trash", false, a.openTrash)
		if r.Clicked() {
			a.openTrash()
		}
		r.Grow(1)
	})
}

// trashView lists what is in the trash.
func (a *app) trashView(c *ui.Context) {
	t := c.Theme()
	now := a.clock()
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, T("Çöp")).FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		ui.Text(c, T("Atılan sayfalar ve koleksiyonlar 30 gün burada durur; geri yüklenince eski yerlerine dönerler, sonra kalıcı olarak silinirler.")).
			FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 8, 0)
		if len(a.trashItems) == 0 {
			a.emptyState(c, iconTrash, "Çöp boş", "Çöpe attığın sayfalar burada bekler; geri yüklemezsen orada kalır.", "", nil)
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
	row := ui.Row(c).Padding(8, 10).Gap(10).Radius(rad(8)).AlignItems(ui.Center).Transition(rowEnter).Label(T("Çöpteki: ") + item.Title)
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
		}
		ui.Icon(c, glyph).Width(17).Height(17).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, item.Title).FontSize(15).FontWeight(600).MaxLines(1)
			from := "Koleksiyon"
			if item.Kind == "page" {
				from = item.Where
				if from == "" {
					from = "Sayfa"
				}
			}
			note := from + "  ·  " + ago(item.Trashed, now) + " atıldı"
			if !item.Trashed.IsZero() {
				note += "  ·  " + Tf("%d gün sonra silinir", trashDaysLeft(item, now))
			}
			ui.Text(c, note).FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
		})
		if ryugo.Button(c.Key("restore:"+item.Dir), "Geri yükle", ryugo.ButtonOptions{Icon: iconRestore, Size: ryugo.Sm}).Clicked() {
			a.restore(item)
		}
		if ryugo.Button(c.Key("delete:"+item.Dir), T("Kalıcı olarak sil: ")+item.Title, ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			if err := a.lib.DeleteForever(item); err != nil {
				a.saveErr = err.Error()
			}
			a.loadTrash()
		}
	})
}

// tasksRow is the task view's row, above the trash's, with how many tasks are late
// or due today: thousands open say nothing, a day's say what is to be done.
func (a *app) tasksRow(c *ui.Context) {
	t := c.Theme()
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
	ui.Row(c.Key("tasks-row-wrap")).Padding(6, 8, 0, 8).Children(func() {
		r := ui.Row(c.Key("tasks-row")).FillWidth().Height(rowH).Padding(0, 8).Gap(7).Radius(rad(7)).AlignItems(ui.Center).
			Label("Görevler").Role(ui.RoleButton).Transition(rowMove)
		switch {
		case a.tasksOpen:
			r.Background(t.Selection)
		case r.Hovered():
			r.Background(t.SurfaceHover)
		default:
			r.Background(t.Surface.Alpha(0))
		}
		r.Children(func() {
			ui.Icon(c, iconTask).Width(15).Height(15).TextColor(t.TextMuted)
			ui.Text(c, T("Görevler")).FontSize(13.5).Grow(1)
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
	})
}
