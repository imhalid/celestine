package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The home screen is a summary of the day on a dark ground, and a sheet over it: the weekday and the date, a
// sentence of the day with its numbers marked, a row of figures; on the sheet the week, the tasks open, the
// collections and the pages written lately. Scrolled up, the sheet covers the summary, which stays behind, and a bar at
// the top shows the date alone (#637).

// The summary's own colors: it is dark on the light look too.
var (
	nightBg    = ui.Hex("#131217")
	nightText  = ui.Hex("#F3EFE9")
	nightMuted = ui.Hex("#8D8982")
	coral      = ui.Hex("#FF7A50")
)

// homeState is what the home screen keeps between frames.
type homeState struct {
	scroll  ui.ScrollState
	summary float32       // the summary's height, as the last frame laid it out; 0 before
	bars    mygo.BarStyle // the system bars' icons as last set
}

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
	h := &a.home // the screen is laid out under the system's bar, which the root's ground fills (view)
	// How far the sheet has covered the summary, from 0 to 1.
	covered := min(max(h.scroll.Y/max(h.summary-120, 1), 0), 1)
	ui.Box(c.Key("home")).Fill().Background(nightBg).Children(func() {
		ui.Scroll(c.Key("home-scroll")).Fill().TrackScroll(&h.scroll).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				// The summary scrolls a third as fast as the sheet, which comes over it.
				sum := ui.Column(c.Key("summary")).FillWidth().Padding(66, side+4, 58, side+4).Gap(18)
				heroField(c, sum, summarySky(now), 0)
				sum.Opacity(1-covered*0.85).Translate(0, max(h.scroll.Y, 0)*0.65)
				sum.Children(func() { a.summaryView(c, now) })
				if b := sum.LayoutBounds(); b.H > 0 {
					h.summary = b.H
				}
				a.homeSheet(c, now)
			})
		})
		a.homeBar(c, now, covered)
	})
}

// summarySky is the hour's colors deepened for the summary, which stays dark: a hint of the hour on the night.
func summarySky(now time.Time) sky {
	s := skyAt(now, false)
	for k := range s.cols {
		s.cols[k] = s.cols[k].MixOklab(nightBg, 0.9-0.3*s.light)
	}
	s.light = 1
	return s
}

// setBars has the system bars' icons light over the home's dark summary and bar, and as the look has them elsewhere.
func (a *app) setBars(onDark bool) {
	style := mygo.BarStyleAuto
	if onDark {
		style = mygo.BarStyleLight
	}
	if a.home.bars != style {
		a.home.bars = style
		mygo.SystemBars.SetStyle(style)
	}
}

// homeBar is the bar at the top of the home screen: the menu's button, and the date once the sheet covers the
// summary, on the summary's ground.
func (a *app) homeBar(c *ui.Context, now time.Time, covered float32) {
	solid := max(0, (covered-0.7)/0.3)
	bar := ui.Row(c.Key("home-bar")).Absolute().Top(0).Left(0).Right(0).Height(66).Padding(0, side).Gap(14).
		AlignItems(ui.Center)
	if solid > 0 {
		bar.Background(nightBg.Alpha(solid))
	}
	bar.Children(func() {
		b := ui.Box(c.Key("menu")).Size(46, 46).Radius(23).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
			Background(nightText.Alpha(0.09)).Role(ui.RoleButton).Label(locales.MenuOpen())
		pressable(b)
		b.Children(func() { ui.Icon(c, iconMenu).Size(22, 22).TextColor(nightText) })
		if b.Clicked() {
			a.menuOpen = true
		}
		ui.Text(c, formatDay("dddd, D MMMM", now)).FontSize(17).FontWeight(700).TextColor(nightText).Opacity(solid).
			Translate(0, (1-solid)*8)
	})
}

// summaryView is the summary: the weekday and the date, the sentence of the day, and the figures under it.
func (a *app) summaryView(c *ui.Context, now time.Time) {
	ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
		ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, formatDay("ddd", now)).FontSize(46).FontWeight(800).LineHeight(1).TextColor(nightText)
			ui.Box(c).Size(11, 11).Radius(6).Background(coral).Margin(6, 0, 0, 0)
		})
		ui.Column(c).AlignItems(ui.End).Padding(4, 0, 0, 0).Children(func() {
			ui.Text(c, formatDay("D MMMM", now)).FontSize(15).FontWeight(600).TextColor(nightMuted)
			ui.Text(c, formatDay("YYYY", now)).FontSize(15).FontWeight(600).TextColor(nightMuted)
		})
	})
	msg := locales.HomeSummary(greeting(now), len(a.openTasks()), a.daysWritten(now))
	ui.RichText(c, summarySpans(msg)...).FontSize(25).FontWeight(550).LineHeight(1.32).Wrap()
	ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
		figure(c, iconPage, locales.LibraryPages(len(a.allPages())))
		if n := a.streak(now); n > 0 {
			figure(c, iconFlame, strconv.Itoa(n)+" "+locales.HomeStreak(n))
		}
	})
}

// summarySpans is the sentence of the day as spans: what its message has between ⟨ and ⟩ marked in coral, dotted
// under.
func summarySpans(msg string) []ui.Span {
	var spans []ui.Span
	var b strings.Builder
	marked := false
	flush := func() {
		if b.Len() > 0 {
			s := ui.Span{Text: b.String(), Color: nightText}
			if marked {
				s.Color, s.DottedUnderline, s.DecorationColor, s.UnderlineOffset = coral, true, coral.Alpha(0.7), 4
			}
			spans = append(spans, s)
			b.Reset()
		}
	}
	for _, r := range msg {
		switch r {
		case '⟨', '⟩':
			flush()
			marked = r == '⟨'
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return spans
}

// figure is a number of the summary, with its glyph.
func figure(c *ui.Context, glyph *ui.SVG, s string) {
	ui.Row(c.Key(s)).Gap(7).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, glyph).Size(16, 16).TextColor(nightMuted)
		ui.Text(c, s).FontSize(14).FontWeight(550).TextColor(nightMuted)
	})
}

// homeSheet is the sheet over the summary: the week, the tasks, the collections, the pages written lately.
func (a *app) homeSheet(c *ui.Context, now time.Time) {
	_, _, bottom, _ := c.SafeArea()
	_, winH := c.Size()
	sheet := ui.Column(c.Key("sheet")).FillWidth().MinHeight(winH-100).Background(pal.bg).Margin(-30, 0, 0, 0).
		Padding(10, side, 140+bottom, side).Gap(gap)
	sheet.Radius(30, 30, 0, 0)
	sheet.Children(func() {
		ui.Box(c).Size(40, 5).Radius(3).Background(pal.line).AlignSelf(ui.Center).Margin(0, 0, 6, 0)
		a.weekStrip(c, now)
		a.tasksBlock(c)
		label(c, locales.HomeCollections())
		a.collGrid(c, a.orderedCollections())
		if recent := a.recentPages(8); len(recent) > 0 {
			label(c, locales.HomeRecent())
			for _, p := range recent {
				a.pageBlock(c, p, true)
			}
		}
	})
}

// label names a part of a screen, small, over its blocks.
func label(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(14).FontWeight(650).TextColor(pal.muted).Padding(14, 4, 0, 4)
}

// daysWritten is how many days of the week up to now the journal has words in.
func (a *app) daysWritten(now time.Time) int {
	n, today := 0, startOfDay(now)
	for i := 0; i < 7; i++ {
		if p := a.dayPage(today.AddDate(0, 0, -i)); p != nil && plainOf(p) != "" {
			n++
		}
	}
	return n
}

// weekStrip is the seven days up to today in a block: the day over its weekday, a dot under those the journal has
// words in, today filled; a day that has a page opens it, and today is made when it has none.
func (a *app) weekStrip(c *ui.Context, now time.Time) {
	today := startOfDay(now)
	ui.Row(c.Key("week")).FillWidth().Justify(ui.SpaceBetween).Padding(10, 8).Radius(blockR).Background(pal.block).
		Children(func() {
			for i := 6; i >= 0; i-- {
				d := today.AddDate(0, 0, -i)
				p := a.dayPage(d)
				written := p != nil && plainOf(p) != ""
				cell := pressable(ui.Column(c.Key(d)).Width(42).AlignItems(ui.Center).Gap(4).Padding(9, 0, 7, 0).Radius(16).
					Role(ui.RoleButton).Label(formatDay("dddd D MMMM", d)))
				num, name := pal.text, pal.faint
				if i == 0 {
					cell.Background(pal.text)
					num, name = pal.bg, pal.bg.Alpha(0.65)
				}
				cell.Children(func() {
					ui.Text(c, strconv.Itoa(d.Day())).FontSize(19).FontWeight(700).TextColor(num)
					ui.Text(c, strings.ToUpper(formatDay("ddd", d))).FontSize(10).FontWeight(700).TextColor(name)
					dot := ui.Box(c).Size(5, 5).Radius(3)
					if written {
						dot.Background(coral)
					}
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

// tasksBlock is one row: how many tasks are open in the library, which opens them.
func (a *app) tasksBlock(c *ui.Context) {
	n := len(a.openTasks())
	b := block(c, "tasks", pal.block, blockR).FillWidth().Padding(14, 16).Role(ui.RoleButton).Label(locales.NavTasks())
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Box(c).Size(36, 36).Radius(12).Background(pal.accent.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Icon(c, iconTasks).Size(19, 19).TextColor(pal.accent) })
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

// collGrid lays the collections out two to a row, going down, each in a tone of the one hue.
func (a *app) collGrid(c *ui.Context, colls []*library.Collection) {
	ui.Column(c.Key("grid")).FillWidth().Gap(gap).Children(func() {
		for i := 0; i < len(colls); i += 2 {
			ui.Row(c.Key(i)).FillWidth().Gap(gap).Children(func() {
				for j, k := range colls[i:min(i+2, len(colls))] {
					a.collTile(c, k, i+j).Grow(1).Basis(0)
				}
				if i+1 >= len(colls) {
					ui.Box(c).Grow(1).Basis(0)
				}
			})
		}
	})
}

// tone is the i-th tone of the collections' one hue, the accent's: a block's ground, deeper by turns, and its mark.
func tone(i int) (ground, mark ui.Color) {
	steps := []float32{0.10, 0.17, 0.24, 0.31}
	s := steps[i%len(steps)]
	if dark {
		s *= 1.3
	}
	return pal.block.MixOklab(pal.accent, s), pal.accent.MixOklab(pal.text, 0.12*float32(i%len(steps)))
}

// collTile is a collection as a block in its tone: its mark, its name and how many pages.
func (a *app) collTile(c *ui.Context, k *library.Collection, i int) ui.Element {
	ground, mark := tone(i)
	glyph := iconFolder
	if k.Journal {
		glyph = iconCalendar
	}
	sub := locales.LibraryPages(len(k.All()))
	if k.Dynamic() {
		sub = locales.LibrarySaved()
	}
	b := block(c, k, ground, smallR+4).Height(132).Padding(16).Justify(ui.SpaceBetween).MinWidth(0).
		Role(ui.RoleButton).Label(k.Name)
	b.Children(func() {
		ui.Box(c).Size(38, 38).Radius(13).Background(mark).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, glyph).Size(20, 20).TextColor(pal.accentText) })
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
