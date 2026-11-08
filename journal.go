package main

import (
	"math/rand"
	"strings"
	"time"
)

// The journal: a page for each day, in a collection every library has,
// marked journal in its collection.json (Collection.Journal). Rules:
//
//   - A page is added only for today (⌘J, the journal's button, any "new
//     page" in it, a date chip's "open the day" when it is today): the
//     library refuses any other (Collection.NewPage). The past is edited,
//     not added to.
//   - A day's page is titled by the collection's format (JournalFormat,
//     set in the journal's settings; the title is not typed in the page),
//     and is the day it was made: its Created. A date chip, [[2026-11-05]],
//     finds a day by that, not by the title.
//   - A random day can be opened.

const (
	journalName   = "Günlük"
	defaultFormat = "YYYY-MM-DD"
)

var (
	monthNames = []string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"}
	dayNames   = []string{"Pazar", "Pazartesi", "Salı", "Çarşamba", "Perşembe", "Cuma", "Cumartesi"} // time.Weekday order
)

// the formats offered, with how each looks
var journalFormats = []string{"YYYY-MM-DD", "DD.MM.YYYY", "D MMMM YYYY", "D MMMM YYYY, dddd"}

// formatDay writes day d as layout says: YYYY, YY, MMMM, MMM, MM, M, DD, D,
// dddd and ddd, anything else as it is; months and days in Turkish.
func formatDay(layout string, d time.Time) string {
	if layout == "" {
		layout = defaultFormat
	}
	tokens := []struct {
		t string
		f func() string
	}{
		{"YYYY", func() string { return pad(d.Year(), 4) }},
		{"YY", func() string { return pad(d.Year()%100, 2) }},
		{"MMMM", func() string { return monthNames[d.Month()-1] }},
		{"MMM", func() string { return string([]rune(monthNames[d.Month()-1])[:3]) }},
		{"MM", func() string { return pad(int(d.Month()), 2) }},
		{"M", func() string { return pad(int(d.Month()), 0) }},
		{"DD", func() string { return pad(d.Day(), 2) }},
		{"D", func() string { return pad(d.Day(), 0) }},
		{"dddd", func() string { return dayNames[d.Weekday()] }},
		{"ddd", func() string { return string([]rune(dayNames[d.Weekday()])[:3]) }},
	}
	var b strings.Builder
	for i := 0; i < len(layout); {
		matched := false
		for _, tk := range tokens {
			if strings.HasPrefix(layout[i:], tk.t) {
				b.WriteString(tk.f())
				i += len(tk.t)
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(layout[i])
			i++
		}
	}
	return b.String()
}

func pad(n, width int) string {
	s := ""
	for v := n; ; v /= 10 {
		s = string(rune('0'+v%10)) + s
		if v < 10 {
			break
		}
	}
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// dayOf is the day a page of the journal is for: the day it was made.
func dayOf(p *Page) time.Time {
	d := p.Created.Local()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local)
}

// dayTitle is the title of a page of day d in collection c.
func dayTitle(c *Collection, d time.Time) string { return formatDay(c.JournalFormat, d) }

// journal is the journal's collection, made if the library has none: the one
// marked so, or, in a library from before, the one called Günlük.
func (a *app) journal() (*Collection, error) {
	for _, c := range a.lib.Collections {
		if c.Journal {
			return c, nil
		}
	}
	for _, c := range a.lib.Collections {
		if strings.EqualFold(c.Name, journalName) {
			c.Journal, c.SubPages = true, false
			return c, c.Save()
		}
	}
	c, err := a.lib.NewCollection(journalName)
	if err != nil {
		return nil, err
	}
	c.Icon, c.Journal, c.SubPages = "calendar", true, false
	c.Views = []View{
		{ID: "calendar", Label: "Takvim", Type: "calendar", Sort: "created", DateField: "created"},
		{ID: "list", Label: "Liste", Type: "list", Sort: "created", Desc: true},
		{ID: "table", Label: "Tablo", Type: "table", Sort: "created", Desc: true},
	}
	c.ActiveView = "calendar"
	if err := c.Save(); err != nil {
		return nil, err
	}
	return c, nil
}

// journalColl is the journal's collection if the library has one, and makes none: what is drawn each frame asks it.
func (a *app) journalColl() *Collection {
	for _, c := range a.lib.Collections {
		if c.Journal || strings.EqualFold(c.Name, journalName) {
			return c
		}
	}
	return nil
}

// journalPage is the page of day d, or nil.
func (a *app) journalPage(c *Collection, d time.Time) *Page {
	for _, p := range c.All() {
		if sameDay(dayOf(p), d) {
			return p
		}
	}
	return nil
}

// openDay opens the page of day d in the journal. A day with none gets one
// only when it is today.
func (a *app) openDay(d time.Time) {
	c, err := a.journal()
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	if p := a.journalPage(c, d); p != nil {
		a.open(p)
		return
	}
	if !sameDay(d, a.clock()) {
		a.notice = "Geçmiş güne sayfa eklenemez: günlüğe yalnız bugünün sayfası eklenir"
		return
	}
	p, err := a.makeDay(c, d)
	if err != nil {
		a.saveErr = err.Error()
	}
	if p != nil {
		a.open(p)
	}
}

// makeDay makes the page of day d, which is today: its title is the
// journal's format, and the day it is for the day it is made, by the app's clock.
func (a *app) makeDay(c *Collection, d time.Time) (*Page, error) {
	c.Creating = true
	p, err := c.NewPage(nil, dayTitle(c, d))
	c.Creating = false
	if err != nil {
		return nil, err
	}
	p.Created = a.clock().Truncate(time.Second)
	return p, p.Save("") // a page, though its file would not write
}

// todayPage is the page of today, made if there is none.
func (a *app) todayPage() (*Page, error) {
	c, err := a.journal()
	if err != nil {
		return nil, err
	}
	if p := a.journalPage(c, a.clock()); p != nil {
		return p, nil
	}
	return a.makeDay(c, a.clock())
}

// openToday opens today's page, making it first.
func (a *app) openToday() { a.openDay(a.clock()) }

// openRandomDay opens a day of the journal at random, other than the one
// shown.
func (a *app) openRandomDay() {
	c, err := a.journal()
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	var days []*Page
	for _, p := range c.All() {
		if p != a.page {
			days = append(days, p)
		}
	}
	if len(days) == 0 {
		a.notice = "Günlükte başka gün yok"
		return
	}
	a.open(days[rand.Intn(len(days))])
}

// retitleJournal gives every day's page the title the format says.
func (a *app) retitleJournal(c *Collection) {
	for _, p := range c.All() {
		if title := dayTitle(c, dayOf(p)); title != p.Title {
			if err := p.SetTitle(title); err != nil {
				a.saveErr = err.Error()
			}
			if p == a.page {
				a.title = title
			}
		}
	}
}

func formatOrDefault(f string) string {
	if strings.TrimSpace(f) == "" {
		return defaultFormat
	}
	return f
}
