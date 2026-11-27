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
// sentence of the day with its numbers marked, a row of figures; on the sheet the collections and the pages
// written lately. The date's row stays where it is: scrolled up, the sheet comes over the
// rest of the summary and goes under the row (#637, #638). Pulled down past its top, the screen opens the menu.

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
	summary float32       // the height of the summary under the date row, as the last frame laid it out
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
	y := max(h.scroll.Y, 0)
	home := ui.Column(c.Key("home")).Fill()
	heroField(c, home, summarySky(now), 0)
	home.Children(func() {
		a.dateRow(c, now)
		ui.Box(c.Key("home-body")).Grow(1).Basis(0).FillWidth().Children(func() {
			// Its corners round: the sheet scrolled under the date row keeps the corners it has unscrolled (#639).
			scroll := ui.Scroll(c.Key("home-scroll")).Fill().TrackScroll(&h.scroll).Radius(30, 30, 0, 0).Clip()
			a.pullMenu(scroll)
			scroll.Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					// The rest of the summary stands still, held against the scrolling, and the sheet comes over it (#641).
					sum := ui.Column(c.Key("summary")).FillWidth().Padding(6, side+4, 58, side+4).Gap(18)
					sum.Translate(0, y)
					sum.Children(func() { a.summaryView(c, now) })
					if b := sum.LayoutBounds(); b.H > 0 {
						h.summary = b.H
					}
					a.homeSheet(c, now)
				})
			})
			// Once the sheet's top reaches the date row, it stays there with its handle, and what is on the sheet
			// scrolls under it.
			if h.summary > 0 && y >= h.summary-sheetOver {
				ui.Box(c.Key("sheet-top")).Absolute().Top(0).Left(0).Right(0).Height(sheetTop).Background(pal.bg).
					Radius(30, 30, 0, 0).Children(func() { sheetHandle(c) })
				// Under it what scrolls fades in, rather than meeting it at a sharp edge (#640); the fade comes as the
				// sheet goes under it, not at once (#643).
				fade := min((y-(h.summary-sheetOver))/40, 1)
				ui.Box(c.Key("sheet-fade")).Absolute().Top(sheetTop).Left(0).Right(0).Height(30).
					Gradient(pal.bg.Alpha(fade), pal.bg.Alpha(0), 180)
			}
		})
	})
}

// homeDay is now's weekday as the top of home says it, at least four letters: the whole name when it is four or less,
// else without its vowels after its first letter (Pzrts, Cuma), as its language's catalog has them (#643).
func homeDay(now time.Time) string {
	if names := strings.Split(locales.DateDaysHome(), ","); len(names) == 7 {
		return names[now.Weekday()]
	}
	return formatDay("dddd", now)
}

// dateRow is the top of the summary, which stays: the weekday, large, with a dot, and the date at the right.
func (a *app) dateRow(c *ui.Context, now time.Time) {
	ui.Row(c.Key("date")).FillWidth().Padding(18, side+4, 4, side+4).AlignItems(ui.Start).Shrink(0).Children(func() {
		ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, homeDay(now)).FontSize(46).FontWeight(800).LineHeight(1).TextColor(nightText)
			ui.Box(c).Size(11, 11).Radius(6).Background(coral).Margin(6, 0, 0, 0)
		})
		ui.Column(c).AlignItems(ui.End).Padding(4, 0, 0, 0).Children(func() {
			ui.Text(c, formatDay("D MMMM", now)).FontSize(15).FontWeight(600).TextColor(nightMuted)
			ui.Text(c, formatDay("YYYY", now)).FontSize(15).FontWeight(600).TextColor(nightMuted)
		})
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

// summaryView is the rest of the summary: the sentence of the day, and the figures under it. Its marks open what
// they count: the tasks open the Tasks screen, the notes written this week a screen of them (#643).
func (a *app) summaryView(c *ui.Context, now time.Time) {
	msg := locales.HomeSummary(greeting(now), len(a.openTasks()), len(a.weekPages(now)))
	ui.RichText(c.Key("sentence")).FontSize(25).FontWeight(550).LineHeight(1.32).Wrap().TextColor(nightText).Children(func() {
		for i, part := range summaryParts(msg) {
			t := ui.Text(c.Key(i), part.text)
			if part.mark == markNone {
				continue
			}
			t.TextColor(coral).DottedUnderline().DecorationColor(coral.Alpha(0.7)).UnderlineOffset(4).Role(ui.RoleLink)
			if t.Clicked() {
				switch part.mark {
				case markTasks:
					a.goTo(pathTasks)
				case markNotes:
					a.router.Transition = ui.TransitionAuto
					a.router.Push(pathWeek)
				}
			}
		}
	})
	ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
		figure(c, iconPage, locales.LibraryPages(len(a.allPages())))
		if n := a.streak(now); n > 0 {
			figure(c, iconFlame, strconv.Itoa(n)+" "+locales.HomeStreak(n))
		}
	})
}

// The marks of the sentence of the day: what its message has between ⟨ and ⟩ is the tasks open, between ⟦ and ⟧ the
// notes written this week.
const (
	markNone = iota
	markTasks
	markNotes
)

// summaryPart is a piece of the sentence of the day, and what it marks.
type summaryPart struct {
	text string
	mark int
}

// summaryParts are the pieces of message msg, split at its marks.
func summaryParts(msg string) []summaryPart {
	var out []summaryPart
	var b strings.Builder
	mark := markNone
	flush := func() {
		if b.Len() > 0 {
			out = append(out, summaryPart{b.String(), mark})
			b.Reset()
		}
	}
	for _, r := range msg {
		switch r {
		case '⟨':
			flush()
			mark = markTasks
		case '⟦':
			flush()
			mark = markNotes
		case '⟩', '⟧':
			flush()
			mark = markNone
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return out
}

// weekPages are the pages with words written in the seven days up to now, the last written first.
func (a *app) weekPages(now time.Time) []*library.Page {
	since := startOfDay(now).AddDate(0, 0, -6)
	var out []*library.Page
	for _, p := range a.recentPages(len(a.allPages())) {
		if p.Modified.Before(since) {
			break
		}
		if plainOf(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// weekScreen is the notes written this week, as a list.
func (a *app) weekScreen(c *ui.Context) {
	ps := a.weekPages(time.Now())
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, true, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				ui.Column(c).Padding(6, 0, 10, 0).Gap(4).Children(func() {
					ui.Text(c, locales.HomeWeek()).FontSize(38).FontWeight(800).Role(ui.RoleHeading).Level(1)
					ui.Text(c, locales.LibraryPages(len(ps))).FontSize(15).TextColor(pal.muted)
				})
				if len(ps) == 0 {
					ui.Text(c, locales.WeekNone()).FontSize(16).TextColor(pal.muted).Wrap()
				}
				for _, p := range ps {
					a.pageBlock(c, p, true)
				}
			})
		})
	})
}

// figure is a number of the summary, with its glyph.
func figure(c *ui.Context, glyph *ui.SVG, s string) {
	ui.Row(c.Key(s)).Gap(7).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, glyph).Size(16, 16).TextColor(nightMuted)
		ui.Text(c, s).FontSize(14).FontWeight(550).TextColor(nightMuted)
	})
}

// The sheet's top: how far it comes over the summary unscrolled, and the height of its top, where its handle is.
const (
	sheetOver = 30
	sheetTop  = 28
)

// sheetHandle is the handle on the sheet's top.
func sheetHandle(c *ui.Context) {
	ui.Row(c).FillWidth().Justify(ui.Center).Padding(11, 0, 0, 0).Children(func() {
		ui.Box(c).Size(40, 5).Radius(3).Background(pal.line)
	})
}

// homeSheet is the sheet over the summary: the collections, the pages written lately.
func (a *app) homeSheet(c *ui.Context, now time.Time) {
	_, _, bottom, _ := c.SafeArea()
	_, winH := c.Size()
	sheet := ui.Column(c.Key("sheet")).FillWidth().MinHeight(winH-100).Background(pal.bg).Margin(-sheetOver, 0, 0, 0).
		Padding(0, side, 140+bottom, side).Gap(gap)
	sheet.Radius(30, 30, 0, 0)
	sheet.Children(func() {
		ui.Box(c).FillWidth().Height(sheetTop - gap).Children(func() { sheetHandle(c) })
		a.collGrid(c, a.orderedCollections())
		for _, p := range a.recentPages(8) {
			a.pageBlock(c, p, true)
		}
	})
}

// label names a part of a screen, small, over its blocks.
func label(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(14).FontWeight(650).TextColor(pal.muted).Padding(14, 4, 0, 4)
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

// collGrid lays the collections out two to a row, going down, each in the tones of its color.
func (a *app) collGrid(c *ui.Context, colls []*library.Collection) {
	ui.Column(c.Key("grid")).FillWidth().Gap(gap).Children(func() {
		for i := 0; i < len(colls); i += 2 {
			ui.Row(c.Key(i)).FillWidth().Gap(gap).Children(func() {
				for _, k := range colls[i:min(i+2, len(colls))] {
					a.collTile(c, k).Grow(1).Basis(0)
				}
				if i+1 >= len(colls) {
					ui.Box(c).Grow(1).Basis(0)
				}
			})
		}
	})
}

// collTile is a collection as a card of its color: its icon and how many pages, its name large under them, in its
// deep tone. Held, it rises out of its place into its sheet's preview, and its place is left empty.
func (a *app) collTile(c *ui.Context, k *library.Collection) ui.Element {
	t := tonesOf(k)
	b := block(c, k, t.card, smallR+4).Height(132).Padding(16).Justify(ui.SpaceBetween).MinWidth(0).
		Role(ui.RoleButton).Label(k.Name)
	if a.collSheet == k.Dir || (a.cs.dir == k.Dir && a.csLifted) {
		b.Opacity(0)
	}
	b.Children(func() { collCard(c, k, t, 1) })
	if b.LongPressed() {
		a.holdCollection(k, b.Bounds())
	}
	if b.Clicked() {
		a.openCollection(k)
	}
	return b
}

// collCard is what a collection's card shows, at scale s: its icon and how many pages, and its name.
func collCard(c *ui.Context, k *library.Collection, t tones, s float32) {
	sub := locales.LibraryPages(len(k.All()))
	if k.Dynamic() {
		sub = locales.LibrarySaved()
	}
	ui.Row(c.Key("card-top")).Gap(7 * s).AlignItems(ui.Center).Children(func() {
		collIcon(c, k, 24*s)
		ui.Text(c, sub).FontSize(14 * s).FontWeight(550).TextColor(t.ink.Alpha(0.75)).MaxLines(1)
	})
	ui.Text(c, k.Name).FontSize(22 * s).FontWeight(650).LineHeight(1.15).MaxLines(2).Wrap().TextColor(t.ink)
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
