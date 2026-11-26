package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The home screen is blocks: the day on its drifting field, today's page, the tasks open and the days written in a
// row, the collections in a row that scrolls sideways, and the pages written lately.

// greeting is what the screen says at the hour of now.
func greeting(now time.Time) string {
	switch h := now.Hour(); {
	case h >= 5 && h < 12:
		return locales.GreetMorning()
	case h >= 12 && h < 18:
		return locales.GreetAfternoon()
	case h >= 18 && h < 23:
		return locales.GreetEvening()
	}
	return locales.GreetNight()
}

func (a *app) homeScreen(c *ui.Context) {
	now := time.Now()
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				a.heroBlock(c, now)
				a.todayBlock(c)
				ui.Row(c).FillWidth().Gap(gap).Children(func() {
					a.tasksBlock(c)
					a.streakBlock(c, now)
				})
				a.collectionsStrip(c)
				recent := a.recentPages(8)
				if len(recent) > 0 {
					label(c, locales.HomeRecent())
					for _, p := range recent {
						a.pageBlock(c, p, true)
					}
				}
			})
		})
	})
}

// label names a part of a screen, small, over its blocks.
func label(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(14).FontWeight(650).TextColor(pal.muted).Padding(14, 4, 0, 4)
}

// heroBlock is the day, large, on the drifting field.
func (a *app) heroBlock(c *ui.Context, now time.Time) {
	hero := ui.Column(c.Key("hero")).FillWidth().Height(210).Radius(blockR).Padding(22).Justify(ui.End).Clip()
	heroField(c, hero, blockR)
	ink := ui.Hex("#2A1E1A")
	if dark {
		ink = ui.Hex("#F4EEE8")
	}
	hero.Children(func() {
		ui.Text(c, greeting(now)).FontSize(16).FontWeight(550).TextColor(ink.Alpha(0.75))
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			ui.Text(c, strconv.Itoa(now.Day())).FontSize(84).FontWeight(800).LineHeight(0.95).TextColor(ink)
			ui.Column(c).Padding(0, 0, 10, 0).Children(func() {
				ui.Text(c, formatDay("dddd", now)).FontSize(22).FontWeight(700).TextColor(ink)
				ui.Text(c, formatDay("MMMM YYYY", now)).FontSize(16).FontWeight(500).TextColor(ink.Alpha(0.7))
			})
		})
	})
}

// todayBlock is today's page: its first lines and how many words, or a word that asks for some.
func (a *app) todayBlock(c *ui.Context) {
	p := a.todayPage()
	text := ""
	if p != nil {
		text = plainOf(p)
	}
	b := block(c, "today", pal.block, blockR).FillWidth().Padding(20).Gap(12).Role(ui.RoleButton).Label(locales.TodayPage())
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Icon(c, iconCalendar).Size(18, 18).TextColor(pal.accent)
			ui.Text(c, locales.TodayPage()).FontSize(14).FontWeight(700).TextColor(pal.accent).Grow(1)
			ui.Box(c).Size(36, 36).Radius(18).Background(pal.accent).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Icon(c, iconArrow).Size(18, 18).TextColor(pal.accentText) })
		})
		if text == "" {
			ui.Text(c, locales.TodayPrompt()).FontSize(20).FontWeight(600).LineHeight(1.3).TextColor(pal.muted).Wrap()
			return
		}
		ui.Text(c, lines(text, 4)).FontSize(18).LineHeight(1.45).MaxLines(4).Wrap()
		ui.Text(c, locales.TodayWords(wordCount(text))).FontSize(13).FontWeight(600).TextColor(pal.faint)
	})
	if b.Clicked() {
		a.newNote(a.journalColl())
	}
}

// tasksBlock is how many tasks are open in the library, which opens them.
func (a *app) tasksBlock(c *ui.Context) {
	n := len(a.openTasks())
	tint := tints[2]
	b := block(c, "tasks", tint.Alpha(0.16), blockR).Grow(1).Basis(0).Height(150).Padding(18).Justify(ui.SpaceBetween).
		Role(ui.RoleButton).Label(locales.NavTasks())
	b.Children(func() {
		ui.Icon(c, iconTasks).Size(26, 26).TextColor(tint)
		ui.Column(c).Children(func() {
			ui.Text(c, strconv.Itoa(n)).FontSize(40).FontWeight(800).LineHeight(1)
			ui.Text(c, locales.HomeTasks(n)).FontSize(14).FontWeight(600).TextColor(pal.muted).Wrap()
		})
	})
	if b.Clicked() {
		a.goTo(pathTasks)
	}
}

// streakBlock is how many days in a row the journal has words, today or yesterday the last.
func (a *app) streakBlock(c *ui.Context, now time.Time) {
	n := a.streak(now)
	tint := tints[4]
	b := block(c, "streak", tint.Alpha(0.18), blockR).Grow(1).Basis(0).Height(150).Padding(18).Justify(ui.SpaceBetween).
		Role(ui.RoleButton).Label(locales.HomeStreak(n))
	b.Children(func() {
		ui.Icon(c, iconFlame).Size(26, 26).TextColor(tint)
		ui.Column(c).Children(func() {
			ui.Text(c, strconv.Itoa(n)).FontSize(40).FontWeight(800).LineHeight(1)
			ui.Text(c, locales.HomeStreak(n)).FontSize(14).FontWeight(600).TextColor(pal.muted).Wrap()
		})
	})
	if b.Clicked() {
		if j := a.journalColl(); j != nil {
			a.openCollection(j)
		}
	}
}

// streak is how many days in a row up to today, or yesterday when today has none yet, the journal has words.
func (a *app) streak(now time.Time) int {
	j := a.journalColl()
	if j == nil {
		return 0
	}
	written := map[time.Time]bool{}
	for _, p := range j.All() {
		if plainOf(p) != "" {
			written[dayOf(p)] = true
		}
	}
	d := startOfDay(now)
	if !written[d] {
		d = d.AddDate(0, 0, -1)
	}
	n := 0
	for written[d] {
		n++
		d = d.AddDate(0, 0, -1)
	}
	return n
}

// collectionsStrip is the collections, each a colored block, in a row that scrolls sideways.
func (a *app) collectionsStrip(c *ui.Context) {
	label(c, locales.HomeCollections())
	ui.ScrollHorizontal(c.Key("colls")).Margin(0, -side).FillWidth().Children(func() {
		ui.Row(c).Padding(0, side).Gap(gap).Children(func() {
			for _, k := range a.orderedCollections() {
				a.collTile(c, k, 150, 132)
			}
		})
	})
}

// collTile is a collection as a block w×h: its color, its mark, its name and how many pages.
func (a *app) collTile(c *ui.Context, k *library.Collection, w, h float32) ui.Element {
	tint := collTint(k)
	glyph := iconFolder
	if k.Journal {
		glyph = iconCalendar
	}
	sub := locales.LibraryPages(len(k.All()))
	if k.Dynamic() {
		sub = locales.LibrarySaved()
	}
	b := block(c, k, tint.Alpha(0.17), smallR+4).Width(w).Height(h).Padding(16).Justify(ui.SpaceBetween).Shrink(0).
		Role(ui.RoleButton).Label(k.Name)
	b.Children(func() {
		ui.Box(c).Size(38, 38).Radius(13).Background(tint).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, glyph).Size(20, 20).TextColor(ui.Hex("#FFFFFF")) })
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, k.Name).FontSize(17).FontWeight(700).MaxLines(1)
			ui.Text(c, sub).FontSize(13).FontWeight(550).TextColor(pal.muted)
		})
	})
	if b.Clicked() {
		a.openCollection(k)
	}
	return b
}

// newNote makes a page in collection c, or the first collection of notes (made if there is none), and opens it; in
// the journal, today's page.
func (a *app) newNote(c *library.Collection) {
	if c == nil {
		for _, k := range a.lib.Collections {
			if !k.Journal && !k.Dynamic() {
				c = k
				break
			}
		}
	}
	if c == nil {
		var err error
		if c, err = a.lib.NewCollection("Notes"); err != nil {
			a.say(locales.PageSaveFailed(err.Error()))
			return
		}
	}
	if c.Journal {
		p := a.todayPage()
		if p == nil {
			var err error
			if p, err = a.makeToday(); err != nil {
				a.say(locales.PageSaveFailed(err.Error()))
				return
			}
		}
		a.openPage(p)
		return
	}
	p, err := c.NewPage(nil, "Untitled")
	if err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	a.fresh = p.ID // its title is asked for first
	a.openPage(p)
}
