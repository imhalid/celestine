package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Library screen is the collections as colored blocks, two to a row, the journal first; a collection's screen is
// its pages as blocks under its name, in its color, the journal's newest day first.

func (a *app) libraryScreen(c *ui.Context) {
	colls := a.orderedCollections()
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, 0, 140, 0).Children(func() {
				bigTitle(c, locales.TabLibrary(), locales.LibraryCount(len(colls)))
				ui.Column(c).FillWidth().Padding(0, side).Gap(gap).Children(func() {
					for i := 0; i < len(colls); i += 2 {
						ui.Row(c.Key(i)).FillWidth().Gap(gap).Children(func() {
							for _, k := range colls[i:min(i+2, len(colls))] {
								a.collTile(c, k, 0, 150).Grow(1).Basis(0)
							}
							if i+1 >= len(colls) {
								ui.Box(c).Grow(1).Basis(0)
							}
						})
					}
				})
			})
		})
	})
}

// collPages are the pages of collection k as its screen lists them: the journal's newest day first.
func (a *app) collPages(k *library.Collection) []*library.Page {
	ps := k.All()
	if k.Journal {
		ps = append([]*library.Page(nil), ps...)
		sortByDay(ps)
	}
	return ps
}

func (a *app) collectionScreen(c *ui.Context, k *library.Collection) {
	tint := collTint(k)
	ps := a.collPages(k)
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, true, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(4, 2, 0, 2).Children(func() {
					glyph := iconFolder
					if k.Journal {
						glyph = iconCalendar
					}
					ui.Box(c).Size(44, 44).Radius(15).Background(tint).AlignItems(ui.Center).Justify(ui.Center).
						Children(func() { ui.Icon(c, glyph).Size(22, 22).TextColor(ui.Hex("#FFFFFF")) })
					ui.Text(c, locales.LibraryPages(len(ps))).FontSize(15).FontWeight(600).TextColor(pal.muted)
				})
				ui.Text(c, k.Name).FontSize(38).FontWeight(800).LineHeight(1.05).Wrap().Padding(0, 2, 8, 2).
					Role(ui.RoleHeading).Level(1)
				if len(ps) == 0 {
					ui.Text(c, locales.LibraryEmpty()).FontSize(16).TextColor(pal.muted).Wrap().Padding(8, 2)
				}
				for _, p := range ps {
					a.pageBlock(c, p, false)
				}
			})
		})
	})
}

// trash moves page p to the trash, unless it is a day of the journal.
func (a *app) trash(p *library.Page) {
	if a.journalDay(p) {
		a.say(locales.PageDayStays())
		return
	}
	if a.ed.id == p.ID {
		a.ed = editorState{}
	}
	if _, err := a.lib.TrashPage(p); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	delete(plains, p)
	delete(mds, p)
	a.say(locales.PageTrashed())
	a.syncSoon()
}
