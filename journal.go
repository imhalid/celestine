package main

import (
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
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
//   - A random day can be opened, the day before or after the one shown
//     (Önceki gün, Sonraki gün, [ and ] in the normal mode), and a day by
//     name (Tarihe git: yarın, geçen cuma, 2026-11-05).

const (
	journalName   = "Journal"
	defaultFormat = "YYYY-MM-DD"
)

// the formats offered, with how each looks
var journalFormats = []string{"YYYY-MM-DD", "DD.MM.YYYY", "D MMMM YYYY", "D MMMM YYYY, dddd"}

// formatDay writes day d as layout says: YYYY, YY, MMMM, MMM, MM, M, DD, D,
// dddd and ddd, leaving any character preceded by \ as literal text;
// months and days are formatted in the window's language.
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
		{"MMMM", func() string { return monthName(d.Month(), false) }},
		{"MMM", func() string { return monthName(d.Month(), true) }},
		{"MM", func() string { return pad(int(d.Month()), 2) }},
		{"M", func() string { return pad(int(d.Month()), 0) }},
		{"DD", func() string { return pad(d.Day(), 2) }},
		{"D", func() string { return pad(d.Day(), 0) }},
		{"dddd", func() string { return dayName(d.Weekday(), false) }},
		{"ddd", func() string { return dayName(d.Weekday(), true) }},
	}
	var b strings.Builder
	for i := 0; i < len(layout); {
		if layout[i] == '\\' {
			if i+1 < len(layout) {
				r, size := utf8.DecodeRuneInString(layout[i+1:])
				b.WriteRune(r)
				i += 1 + size
			} else {
				b.WriteByte('\\')
				i++
			}
			continue
		}
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
			r, size := utf8.DecodeRuneInString(layout[i:])
			b.WriteRune(r)
			i += size
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

// journalDayKey is the meta key of the day a page of the journal is for, as a date: the day it was made, in the
// zone it was made in. Read from Created in the zone of the moment, a late evening page moved to another day after a
// journey, and a second page was made for today (#144). Not a property's id: those have no colon.
const journalDayKey = "celestine:day"

// dayOf is the day a page of the journal is for: its day as kept, or, for a page that has none, the day it was made.
func dayOf(p *Page) time.Time {
	if d, err := time.ParseInLocation("2006-01-02", p.Props[journalDayKey], time.Local); err == nil {
		return d
	}
	d := p.Created.Local()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local)
}

// stampJob is a page of the journal to keep its day in: the page, its file and its day, as they were when the days
// were asked for.
type stampJob struct {
	p    *Page
	file string
	day  string
}

// stampDays keeps in each page of the journal c that has none the day it is for now: from then on a change of time
// zone does not move it. It is called with the lock held, and only looks: the files are written by a goroutine, the
// key in their meta alone, and the time they were last written kept (library.StampMeta), so an old page is not one
// changed now, and no frame waits for a journal of years to be written (#538). A sync still sends each page once,
// as its file is not what it was. Each file is read again as it is written, so a page saved meanwhile keeps what was
// saved; and the key goes into a page's meta in memory once its file has it, so a save of it after keeps it too.
func (a *app) stampDays(c *Collection) {
	var jobs []stampJob
	for _, p := range c.All() {
		if p.Props[journalDayKey] == "" && p.Unreadable == "" {
			jobs = append(jobs, stampJob{p, p.File(), dayOf(p).Format("2006-01-02")})
		}
	}
	if len(jobs) == 0 {
		return
	}
	fsys, hold := c.FS, a.stampHold
	a.stamping.Add(1)
	go func() {
		defer a.stamping.Done()
		defer a.guard()
		if hold != nil {
			hold()
		}
		keep := func(j stampJob) {
			if j.p.Props == nil {
				j.p.Props = map[string]string{}
			}
			if j.p.Props[journalDayKey] == "" {
				j.p.Props[journalDayKey] = j.day
			}
		}
		if library.BatchSeals(fsys) {
			// one change of the store, outside the lock: a save the window makes meanwhile waits for it, and is
			// not lost, as each file is read inside it
			var done []stampJob
			err := library.InBatch(fsys, func() error {
				for _, j := range jobs {
					if ok, err := library.StampMeta(fsys, j.file, journalDayKey, j.day); ok && err == nil {
						done = append(done, j)
					}
				}
				return nil
			})
			if err != nil {
				return // nothing was kept: the next start tries again
			}
			a.do(func() error {
				for _, j := range done {
					keep(j)
				}
				return nil
			})
			return
		}
		// a folder (or the browser's store) has nothing that keeps the window's saves out: a page at a time, with
		// the lock held, so that no save comes between the reading of a file and its writing
		for _, j := range jobs {
			a.do(func() error {
				if j.p.Props[journalDayKey] != "" {
					return nil // kept meanwhile
				}
				if ok, err := library.StampMeta(fsys, j.p.File(), journalDayKey, j.day); ok && err == nil {
					keep(j)
				}
				return nil
			})
		}
	}()
}

// dayTitle is the title of a page of day d in collection c.
func dayTitle(c *Collection, d time.Time) string { return formatDay(c.JournalFormat, d) }

// journal is the journal's collection, made if the library has none: the one
// marked so, or, in a library from before, the one called Günlük.
func (a *app) journal() (*Collection, error) {
	for _, c := range a.lib.Collections {
		if c.Journal {
			if !a.daysStamped {
				a.daysStamped = true
				a.stampDays(c)
			}
			return c, nil
		}
	}
	for _, c := range a.lib.Collections {
		if strings.EqualFold(c.Name, journalName) || strings.EqualFold(c.Name, "Günlük") { // or a library of before
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
		{ID: "calendar", Label: locales.JournalViewCalendar(), Type: "calendar", Sort: "created", DateField: "created"},
		{ID: "list", Label: locales.JournalViewList(), Type: "list", Sort: "created", Desc: true},
		{ID: "table", Label: locales.JournalViewTable(), Type: "table", Sort: "created", Desc: true},
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
		if c.Journal || strings.EqualFold(c.Name, journalName) || strings.EqualFold(c.Name, "Günlük") {
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
		a.fail(err)
		return
	}
	if p := a.journalPage(c, d); p != nil {
		a.open(p)
		return
	}
	if !sameDay(d, a.clock()) {
		a.say(toastWarn, locales.APageCannotBeAddedTo())
		return
	}
	p, err := a.makeDay(c, d)
	if err != nil {
		a.fail(err)
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
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[journalDayKey] = d.Format("2006-01-02")
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
		a.fail(err)
		return
	}
	var days []*Page
	for _, p := range c.All() {
		if p != a.page {
			days = append(days, p)
		}
	}
	if len(days) == 0 {
		a.say(toastWarn, locales.ThereIsNoOtherDayIn())
		return
	}
	a.open(days[rand.Intn(len(days))])
}

// retitleJournal gives every day's page the title the format says.
func (a *app) retitleJournal(c *Collection) {
	for _, p := range c.All() {
		if title := dayTitle(c, dayOf(p)); title != p.Title {
			if err := p.SetTitle(title); err != nil {
				a.fail(err)
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

// stepDay opens the day of the journal n days with pages before (n < 0) or after the day shown: the open
// page's day when it is a day of the journal, today otherwise. A day without a page is passed over.
func (a *app) stepDay(n int) {
	c := a.journalColl()
	if c == nil || n == 0 {
		a.say(toastWarn, locales.TheJournalHasNoDays())
		return
	}
	base := startOfDay(a.clock())
	if a.page != nil && !a.noPage() && a.page.Coll == c {
		base = dayOf(a.page)
	}
	var days []*Page // those on the side asked, the nearest first
	for _, p := range c.All() {
		if d := dayOf(p); n < 0 && d.Before(base) || n > 0 && d.After(base) {
			days = append(days, p)
		}
	}
	if len(days) == 0 {
		a.say(toastWarn, map[bool]string{true: locales.TheJournalHasNoEarlierDay(), false: locales.TheJournalHasNoLaterDay()}[n < 0])
		return
	}
	slices.SortStableFunc(days, func(p, q *Page) int {
		if n < 0 {
			return dayOf(q).Compare(dayOf(p))
		}
		return dayOf(p).Compare(dayOf(q))
	})
	a.open(days[min(len(days), max(n, -n))-1])
}

// askDate opens the palette's commands with "Tarihe git " in it, for the day to be written after it.
func (a *app) askDate() { a.pal = palette{open: true, query: ">" + locales.CommonGoDate() + " "} }

// goDate is how a day to go to is asked in the palette: the command's name in either language, then the day.
var goDate = regexp.MustCompile(`^(?:tarihe git|tarih|go to date|go to|date)\s+(.+)$`)

// dateItems are the palette's lines for "Tarihe git" and a day said after it (parseNaturalDay): its page, or
// today's made, or what the journal does not have.
func (a *app) dateItems(q string) []palItem {
	m := goDate.FindStringSubmatch(strings.Join(strings.Fields(fold(q)), " "))
	if m == nil {
		return nil
	}
	d, ok := parseNaturalDay(m[1], a.clock())
	if !ok {
		return []palItem{{title: locales.DatesGoToDateLabel(m[1]), glyph: iconCalendar, score: 1000, off: locales.DatesNotDayWriteTomorrow()}}
	}
	it := palItem{title: locales.DatesGoToDateLabel(shortDate(d)), sub: formatDay("dddd", d), glyph: iconCalendar, score: 1000, run: func() { a.openDay(d) }}
	if c := a.journalColl(); (c == nil || a.journalPage(c, d) == nil) && !sameDay(d, a.clock()) {
		it.off = locales.DatesTheJournalHasNo()
	}
	return []palItem{it}
}
