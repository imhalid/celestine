package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The home screen is blocks: the day and its week on the field of the hour's colors, today's page, the tasks open in
// one row, the collections in a row that scrolls sideways, and the pages written lately.

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
				a.tasksBlock(c)
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

// heroBlock is the day, large, and its week on the field of the hour's colors, the days in a row the journal has
// words beside the greeting.
func (a *app) heroBlock(c *ui.Context, now time.Time) {
	s := skyAt(now, dark)
	ink := s.ink()
	hero := ui.Column(c.Key("hero")).FillWidth().Radius(blockR).Padding(20, 18, 16, 18).Gap(16).Clip()
	heroField(c, hero, s, blockR)
	hero.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, greeting(now)).FontSize(16).FontWeight(550).TextColor(ink.Alpha(0.8)).Grow(1)
			if n := a.streak(now); n > 0 {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(6, 11).Radius(14).Background(ink.Alpha(0.1)).
					Label(strconv.Itoa(n) + " " + locales.HomeStreak(n)).Children(func() {
					ui.Icon(c, iconFlame).Size(15, 15).TextColor(ink)
					ui.Text(c, strconv.Itoa(n)).FontSize(14).FontWeight(700).TextColor(ink)
				})
			}
		})
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			ui.Text(c, strconv.Itoa(now.Day())).FontSize(76).FontWeight(800).LineHeight(0.95).TextColor(ink)
			ui.Column(c).Padding(0, 0, 8, 0).Children(func() {
				ui.Text(c, formatDay("dddd", now)).FontSize(22).FontWeight(700).TextColor(ink)
				ui.Text(c, formatDay("MMMM YYYY", now)).FontSize(16).FontWeight(500).TextColor(ink.Alpha(0.7))
			})
		})
		a.weekStrip(c, now, s)
	})
}

// weekStrip is the seven days up to today: those the journal has words in filled, today ringed; a day that has a
// page opens it.
func (a *app) weekStrip(c *ui.Context, now time.Time, s sky) {
	ink := s.ink()
	today := startOfDay(now)
	ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Children(func() {
		for i := 6; i >= 0; i-- {
			d := today.AddDate(0, 0, -i)
			p := a.dayPage(d)
			written := p != nil && plainOf(p) != ""
			cell := pressable(ui.Column(c.Key(d)).AlignItems(ui.Center).Gap(6).Padding(4, 2).Radius(14).
				Role(ui.RoleButton).Label(formatDay("dddd D MMMM", d)))
			cell.Children(func() {
				ui.Text(c, formatDay("ddd", d)).FontSize(12).FontWeight(600).TextColor(ink.Alpha(0.65))
				dot := ui.Box(c).Size(34, 34).Radius(17).AlignItems(ui.Center).Justify(ui.Center)
				num := ink
				switch {
				case written:
					dot.Background(ink)
					num = s.cols[0] // filled: the number in the field's ground
				case i == 0:
					dot.Border(2, ink.Alpha(0.85))
				default:
					dot.Background(ink.Alpha(0.08))
				}
				if written && i == 0 {
					dot.Border(2, ink)
				}
				dot.Children(func() {
					ui.Text(c, strconv.Itoa(d.Day())).FontSize(14).FontWeight(700).TextColor(num)
				})
			})
			if cell.Clicked() {
				switch {
				case p != nil:
					a.openPage(p)
				case i == 0:
					a.newNote(a.journalColl())
				}
			}
		}
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

// tasksBlock is one row: how many tasks are open in the library, which opens them.
func (a *app) tasksBlock(c *ui.Context) {
	n := len(a.openTasks())
	tint := tints[2]
	b := block(c, "tasks", pal.block, blockR).FillWidth().Padding(14, 16).Role(ui.RoleButton).Label(locales.NavTasks())
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Box(c).Size(36, 36).Radius(12).Background(tint.Alpha(0.16)).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Icon(c, iconTasks).Size(19, 19).TextColor(tint) })
			ui.Text(c, strconv.Itoa(n)+" "+locales.HomeTasks(n)).FontSize(16).FontWeight(650).Grow(1)
			ui.Icon(c, iconArrow).Size(18, 18).TextColor(pal.faint)
		})
	})
	if b.Clicked() {
		a.goTo(pathTasks)
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
