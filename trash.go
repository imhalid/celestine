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

// openTrash shows the trash instead of a page.
func (a *app) openTrash() {
	a.flush()
	a.trashOpen, a.coll = true, nil
	a.loadTrash()
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
		a.notice = "“" + item.Title + "” geri yüklendi"
		return
	}
	if p != nil {
		a.open(p)
	} else {
		a.openCollection(coll)
	}
}

// trashRow is the trash's row at the foot of the sidebar, with how much is in it.
func (a *app) trashRow(c *ui.Context) {
	t := c.Theme()
	if a.trashItems == nil {
		a.loadTrash()
		if a.trashItems == nil {
			a.trashItems = []*TrashItem{}
		}
	}
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6)).Margin(0, 12)
	ui.Column(c).Padding(6, 8, 10, 8).Children(func() {
		c.Key("trash-row")
		r := ui.Row(c).Height(rowH).Padding(0, 8, 0, 8).Gap(7).Radius(7).AlignItems(ui.Center).
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
			ui.Text(c, "Çöp").FontSize(13.5).Grow(1)
			if n := len(a.trashItems); n > 0 {
				ryugo.Badge(c, strconv.Itoa(n), ryugo.ToneNeutral)
			}
		})
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
		ui.Text(c, "Çöp").FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		ui.Text(c, "Atılan sayfalar ve koleksiyonlar burada durur; geri yüklenince eski yerlerine dönerler.").
			FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 8, 0)
		if len(a.trashItems) == 0 {
			ui.Text(c, "Çöp boş.").TextColor(t.TextMuted).Padding(16, 4)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
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
	row := ui.Row(c).Padding(8, 10).Gap(10).Radius(8).AlignItems(ui.Center).Transition(rowEnter).Label("Çöpteki: " + item.Title)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	} else {
		row.Background(t.SurfaceHover.Alpha(0))
	}
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
			ui.Text(c, from+"  ·  "+ago(item.Trashed, now)+" atıldı").FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
		})
		if ryugo.Button(c.Key("restore:"+item.Dir), "Geri yükle", ryugo.ButtonOptions{Icon: iconRestore, Size: ryugo.Sm}).Clicked() {
			a.restore(item)
		}
		if ryugo.Button(c.Key("delete:"+item.Dir), "Kalıcı olarak sil: "+item.Title, ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Danger: true, Size: ryugo.Sm}).Clicked() {
			if err := a.lib.DeleteForever(item); err != nil {
				a.saveErr = err.Error()
			}
			a.loadTrash()
		}
	})
}
