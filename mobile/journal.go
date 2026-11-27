package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The journal is the desktop's: the collection marked Journal, a page a day, the day in the page's celestine:day
// property, its title in the journal's format, and the days past locked. These are the desktop's rules
// (journal.go), kept the same here so that a day written on the phone is the desktop's day.

// journalDayKey is the property that holds the day a page of the journal is for.
const journalDayKey = "celestine:day"

// journalName is the name of a journal made here, as the desktop names one.
const journalName = "Journal"

// journal is the journal's collection, made if the library has none.
func (a *app) journal() (*library.Collection, error) {
	if c := a.journalColl(); c != nil {
		if !c.Journal {
			c.Journal, c.SubPages = true, false
			return c, c.Save()
		}
		return c, nil
	}
	c, err := a.lib.NewCollection(journalName)
	if err != nil {
		return nil, err
	}
	c.Icon, c.Journal, c.SubPages = "calendar", true, false
	c.Views = []library.View{
		{ID: "calendar", Label: "Calendar", Type: "calendar", Sort: "created", DateField: "created"},
		{ID: "list", Label: "List", Type: "list", Sort: "created", Desc: true},
		{ID: "table", Label: "Table", Type: "table", Sort: "created", Desc: true},
	}
	c.ActiveView = "calendar"
	return c, c.Save()
}

// journalColl is the journal if the library has one: the one marked so, or one of before called Journal or Günlük.
func (a *app) journalColl() *library.Collection {
	for _, c := range a.lib.Collections {
		if c.Journal {
			return c
		}
	}
	for _, c := range a.lib.Collections {
		if strings.EqualFold(c.Name, journalName) || strings.EqualFold(c.Name, "Günlük") {
			return c
		}
	}
	return nil
}

// dayOf is the day page p of the journal is for.
func dayOf(p *library.Page) time.Time {
	if d, err := time.ParseInLocation("2006-01-02", p.Props[journalDayKey], time.Local); err == nil {
		return d
	}
	d := p.Created.Local()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local)
}

// startOfDay is the midnight that starts t's day.
func startOfDay(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// todayPage is today's page of the journal, or nil when it is not written yet.
func (a *app) todayPage() *library.Page { return a.dayPage(startOfDay(a.now())) }

// dayPage is the journal's page of day d (a midnight), or nil when it has none.
func (a *app) dayPage(d time.Time) *library.Page {
	c := a.journalColl()
	if c == nil {
		return nil
	}
	for _, p := range c.All() {
		if dayOf(p).Equal(d) {
			return p
		}
	}
	return nil
}

// makeToday makes today's page, as the desktop makes a day.
func (a *app) makeToday() (*library.Page, error) { return a.makeDay(a.now()) }

// makeDay creates the journal page for day d, matching desktop's journal rules.
func (a *app) makeDay(d time.Time) (*library.Page, error) {
	c, err := a.journal()
	if err != nil {
		return nil, err
	}
	c.Creating = true
	p, err := c.NewPage(nil, formatDay(c.JournalFormat, d))
	c.Creating = false
	if err != nil {
		return nil, err
	}
	p.Created = d.Truncate(time.Second)
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[journalDayKey] = startOfDay(d).Format("2006-01-02")
	return p, p.Save("")
}

// lockDays locks the past days of the journal, as the desktop does: the library then refuses their edits.
func (a *app) lockDays() {
	c := a.journalColl()
	if c == nil {
		return
	}
	today := startOfDay(a.now())
	why := locales.PageLocked()
	for _, p := range c.All() {
		p.Locked = ""
		if dayOf(p).Before(today) {
			p.Locked = why
		}
	}
}

// journalDay reports whether p is a day of the journal, which is not moved to the trash.
func (a *app) journalDay(p *library.Page) bool {
	c := a.journalColl()
	return c != nil && p.Coll == c && p.Parent == nil
}

// formatDay writes day d as the journal's layout says (the desktop's tokens: YYYY YY MMMM MMM MM M DD D dddd ddd,
// a character after \ as it is), the names of months and days in the app's language.
func formatDay(layout string, d time.Time) string {
	if layout == "" {
		layout = "YYYY-MM-DD"
	}
	months, monthsShort := strings.Split(locales.DateMonths(), ","), strings.Split(locales.DateMonthsShort(), ",")
	days, daysShort := strings.Split(locales.DateDays(), ","), strings.Split(locales.DateDaysShort(), ",")
	pad := func(n, w int) string {
		s := strconv.Itoa(n)
		for len(s) < w {
			s = "0" + s
		}
		return s
	}
	pick := func(names []string, i int) string {
		if i >= 0 && i < len(names) {
			return names[i]
		}
		return ""
	}
	tokens := []struct {
		t string
		f func() string
	}{
		{"YYYY", func() string { return pad(d.Year(), 4) }},
		{"YY", func() string { return pad(d.Year()%100, 2) }},
		{"MMMM", func() string { return pick(months, int(d.Month())-1) }},
		{"MMM", func() string { return pick(monthsShort, int(d.Month())-1) }},
		{"MM", func() string { return pad(int(d.Month()), 2) }},
		{"M", func() string { return pad(int(d.Month()), 0) }},
		{"DD", func() string { return pad(d.Day(), 2) }},
		{"D", func() string { return pad(d.Day(), 0) }},
		{"dddd", func() string { return pick(days, int(d.Weekday())) }},
		{"ddd", func() string { return pick(daysShort, int(d.Weekday())) }},
	}
	var b strings.Builder
	rs := []rune(layout)
next:
	for i := 0; i < len(rs); {
		if rs[i] == '\\' && i+1 < len(rs) {
			b.WriteRune(rs[i+1])
			i += 2
			continue
		}
		for _, tk := range tokens {
			if strings.HasPrefix(string(rs[i:]), tk.t) {
				b.WriteString(tk.f())
				i += len([]rune(tk.t))
				continue next
			}
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

// sortByDay orders pages of the journal by their day, the newest first.
func sortByDay(ps []*library.Page) {
	slices.SortStableFunc(ps, func(x, y *library.Page) int { return dayOf(y).Compare(dayOf(x)) })
}
