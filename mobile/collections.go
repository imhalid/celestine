package main

import (
	"net/url"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Library screen is the collections as blocks, each in the tones of its color, two to a row, the journal first; a
// collection's screen is its pages as blocks under its name in its color (held, its sheet), the journal's newest day
// first.

func (a *app) libraryScreen(c *ui.Context) {
	colls := a.orderedCollections()
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		a.screenScroll(c).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 0, 140, 0).Children(func() {
				bigTitle(c, locales.TabLibrary(), locales.LibraryCount(len(colls)))
				ui.Column(c).FillWidth().Padding(0, side).Children(func() { a.collGrid(c, colls) })
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
	t := tonesOf(k)
	ps := a.collPages(k)
	if a.router.Path() == "/c/"+url.PathEscape(k.Dir) { // back home, its head flies into its card again
		a.router.Transition = ui.TransitionMatch
	}
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, true, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				// Its head is its card, larger: the card on home and in the Library grows into it (#646). Held, its sheet.
				head := ui.Column(c.Key("head")).FillWidth().Height(168).Padding(20).Justify(ui.SpaceBetween).
					Radius(smallR + 8).Background(t.card).Role(ui.RoleButton).Label(k.Name).Transition(cardFlight(k))
				pressable(head)
				head.Children(func() { collCard(c, k, t, 1.35) })
				if head.LongPressed() {
					a.holdCollection(k, head.Bounds())
				}
				if len(ps) == 0 {
					ui.Text(c, locales.LibraryEmpty()).FontSize(16).TextColor(pal.muted).Wrap().Padding(8, 2)
				}
				a.pageList(c, ps, false)
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
