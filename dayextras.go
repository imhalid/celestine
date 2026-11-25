package main

import (
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Under a day of the journal (#552): the tasks due that day, tickable where they are, with today's overdue ones
// beside today's; and the pages of the journal for the same day in earlier years. The daily review in one place.
// Each is folded by its heading, which the settings keep; the settings turn both off.

// dayFoldTasks and dayFoldYears are the sections under a day, as the settings keep them folded.
const (
	dayFoldTasks = "tasks"
	dayFoldYears = "years"
)

// dayTasks are the tasks of other pages due on day: and when day is today, the open ones due before it.
func dayTasks(all []taskItem, p *Page, day, today time.Time) []taskItem {
	var out []taskItem
	for _, k := range all {
		if k.page == p || k.due.IsZero() {
			continue
		}
		due := time.Date(k.due.Year(), k.due.Month(), k.due.Day(), 0, 0, 0, 0, time.Local) // the day it names, here
		if due.Equal(day) || day.Equal(today) && !k.done && due.Before(day) {
			out = append(out, k)
		}
	}
	return out
}

// onThisDay are the pages of journal c for the month and day of day in earlier years, the latest first.
func onThisDay(c *Collection, day time.Time) []*Page {
	var out []*Page
	for _, q := range c.All() {
		if d := dayOf(q); d.Month() == day.Month() && d.Day() == day.Day() && d.Year() < day.Year() {
			out = append(out, q)
		}
	}
	slices.SortStableFunc(out, func(x, y *Page) int { return dayOf(y).Compare(dayOf(x)) })
	return out
}

// dayExtras is what a day of the journal shows under its text, while the settings have it.
func (a *app) dayExtras(c *ui.Context, pn *paneState, p *Page) {
	pn.dayH = 0
	if a.settings.NoDayExtras || p == nil || p.Coll == nil || !p.Coll.Journal {
		return
	}
	day, today := dayOf(p), startOfDay(a.clock().In(time.Local))
	tasks := dayTasks(a.allTasks(false), p, day, today)
	years := onThisDay(p.Coll, day)
	if len(tasks) == 0 && len(years) == 0 {
		return
	}
	cl := ryugo.From(c).Colors
	box := ui.Column(c.Key("day-extras")).FillWidth().Gap(2).Padding(14, 0, 0, 0)
	box.Children(func() {
		ui.Box(c.Key("day-line")).FillWidth().Height(1).Background(cl.Hairline).Margin(0, 0, 10, 0)
		if len(tasks) > 0 && a.daySection(c, dayFoldTasks, locales.JournalDayTasks(len(tasks))) {
			for _, k := range tasks {
				a.taskRow(c, cl, pn, k)
			}
		}
		if len(years) > 0 && a.daySection(c, dayFoldYears, locales.JournalOnThisDay()) {
			for _, q := range years {
				q := q
				row := ui.Row(c.Key("otd:"+q.Dir)).FillWidth().Padding(6, 8).Gap(10).Radius(rad(8)).AlignItems(ui.Center).
					Label(locales.CommonLinkLabel(q.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(cl.Fill)
				}
				a.contextOn(row, func() []menuItem { return a.pageMenu(q, false) })
				row.Children(func() {
					ui.Text(c, strconv.Itoa(dayOf(q).Year())).FontSize(12.5).FontWeight(600).TextColor(cl.Muted).Font(monoStack)
					ui.Text(c, q.Title).FontSize(14).TextColor(cl.Text).MaxLines(1).Shrink(1).Grow(1)
				})
				if row.Clicked() {
					a.openIn(pn, q)
				}
			}
		}
	})
	pn.dayH = box.LayoutBounds().H
}

// daySection is the heading of a section under a day, which folds it and unfolds it; it reports whether the
// section is open.
func (a *app) daySection(c *ui.Context, key, title string) bool {
	cl := ryugo.From(c).Colors
	open := !slices.Contains(a.settings.DayFold, key)
	head := ui.Row(c.Key("day-head:"+key)).Padding(6, 6, 4, 6).Gap(6).AlignItems(ui.Center).Radius(rad(6)).
		Role(ui.RoleDisclosure).Label(title).Expanded(open).Cursor(ui.CursorPointer)
	head.Children(func() {
		ui.Icon(c, iconChevron).Width(12).Height(12).TextColor(cl.Muted).Rotate(map[bool]float32{true: 90}[open])
		ui.Text(c, title).FontSize(12).FontWeight(650).TextColor(cl.Muted)
	})
	if head.Clicked() {
		if open {
			a.settings.DayFold = append(a.settings.DayFold, key)
		} else {
			a.settings.DayFold = slices.DeleteFunc(a.settings.DayFold, func(k string) bool { return k == key })
		}
		a.saveSettings()
		open = !open
	}
	return open
}
