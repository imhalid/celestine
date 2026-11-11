package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// journalDays makes the journal's pages of days, each by a clock set to it, and leaves the clock at now.
func journalDays(t *testing.T, a *app, now time.Time, days ...string) {
	t.Helper()
	for _, s := range days {
		d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
		a.clock = func() time.Time { return d.Add(9 * time.Hour) }
		if _, err := a.todayPage(); err != nil {
			t.Fatal(err)
		}
	}
	a.clock = func() time.Time { return now }
}

// The day before and after the one shown are the nearest days with pages, from the open day or from today.
func TestJournalStepDay(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	journalDays(t, a, now, "2026-11-01", "2026-11-03", "2026-11-06", "2026-11-09")
	tt.Frame()
	notes := a.page
	shown := func() string {
		t.Helper()
		if a.page == nil {
			return ""
		}
		return a.page.Title
	}
	// from a page not of the journal: the days around today
	a.stepDay(-1)
	if shown() != "2026-11-06" {
		t.Fatalf("before today: %q", shown())
	}
	for _, want := range []string{"2026-11-03", "2026-11-01"} {
		a.stepDay(-1)
		if shown() != want {
			t.Fatalf("back: %q, not %s", shown(), want)
		}
	}
	a.notice = ""
	a.stepDay(-1)
	if shown() != "2026-11-01" || a.notice == "" {
		t.Fatalf("before the first: %q %q", shown(), a.notice)
	}
	a.stepDay(2) // a count passes over one
	if shown() != "2026-11-06" {
		t.Fatalf("two on: %q", shown())
	}
	a.stepDay(9)
	if shown() != "2026-11-09" {
		t.Fatalf("as far as there are: %q", shown())
	}
	a.open(notes)
	a.notice = ""
	a.stepDay(1)
	if shown() != notes.Title || a.notice == "" {
		t.Fatalf("after today: %q %q", shown(), a.notice)
	}
	// the commands are in the palette, under Git
	have := map[string]string{}
	for _, it := range a.commands() {
		have[it.id] = it.cat
	}
	for _, n := range []string{"previousDay", "nextDay", "common_goDate"} {
		if have[n] != "common_go" {
			t.Fatalf("%q: %q", n, have[n])
		}
	}
	a.runCmdTitled(t, "Önceki gün")
	if shown() != "2026-11-06" {
		t.Fatalf("the command: %q", shown())
	}
}

// runCmdTitled runs the palette's command of that title.
func (a *app) runCmdTitled(t *testing.T, title string) {
	t.Helper()
	for _, it := range a.commands() {
		if it.title == title {
			a.runItem(it)
			return
		}
	}
	t.Fatalf("no command %q", title)
}

// Tarihe git takes a day said in words, in the palette; a day without a page says so.
func TestJournalGoToDate(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	journalDays(t, a, now, "2026-11-03", "2026-11-06")
	tt.Frame()
	a.runCmdTitled(t, "Tarihe git")
	if !a.pal.open || a.pal.query != ">Tarihe git " {
		t.Fatalf("palette: %+v", a.pal)
	}
	items := a.commandItems("Tarihe git 3 gün önce")
	if len(items) == 0 || items[0].title != "Tarihe git: "+shortDate(now.AddDate(0, 0, -3)) || items[0].off != "" {
		t.Fatalf("first: %+v", items[0])
	}
	a.runItem(items[0])
	if a.page == nil || a.page.Title != "2026-11-06" {
		t.Fatalf("opened %+v", a.page)
	}
	if it := a.dateItems("go to date last tuesday"); len(it) != 1 || it[0].off != "" {
		t.Fatalf("in English: %+v", it)
	}
	if it := a.dateItems("tarihe git yarın"); len(it) != 1 || it[0].off == "" {
		t.Fatalf("a day with no page: %+v", it)
	}
	if it := a.dateItems("tarihe git bugün"); len(it) != 1 || it[0].off != "" { // today's is made
		t.Fatalf("today: %+v", it)
	}
	if it := a.dateItems("tarihe git elma"); len(it) != 1 || it[0].off == "" {
		t.Fatalf("not a day: %+v", it)
	}
	if it := a.dateItems("tarih"); it != nil {
		t.Fatalf("nothing said: %+v", it)
	}
}

// In the normal mode, [ and ] go to the day before and after.
func TestJournalKeys(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1100, 800)
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	journalDays(t, a, now, "2026-11-03", "2026-11-06")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	if !a.kb.normal {
		t.Fatal("not in the normal mode")
	}
	tt.Key(0, ui.KeyBracketLeft)
	tt.Frame()
	if a.page == nil || a.page.Title != "2026-11-06" {
		t.Fatalf("[: %+v", a.page)
	}
	tt.Key(0, ui.KeyBracketLeft)
	tt.Frame()
	tt.Key(0, ui.KeyBracketRight)
	tt.Frame()
	if a.page.Title != "2026-11-06" {
		t.Fatalf("[ ]: %q", a.page.Title)
	}
}

// A page of the journal keeps its day: after a journey to another time zone a late evening page is still on its day,
// and today does not get a second page (#144).
func TestJournalDayKeptAcrossZones(t *testing.T) {
	was := time.Local
	defer func() { time.Local = was }()
	time.Local = time.FixedZone("İstanbul", 3*3600)
	a, _ := newTestApp(t, "x\n")
	late := time.Date(2026, 11, 9, 23, 30, 0, 0, time.Local)
	a.clock = func() time.Time { return late }
	p, err := a.todayPage()
	if err != nil {
		t.Fatal(err)
	}
	time.Local = time.FixedZone("New York", -5*3600) // 23:30 in İstanbul is 15:30 in New York, the same day; and back
	time.Local = time.FixedZone("Tokyo", 9*3600)     // 23:30 in İstanbul is 05:30 the next day in Tokyo
	day := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)
	c, _ := a.journal()
	if got := a.journalPage(c, day); got != p {
		t.Fatalf("the page of 9 November moved: %v", dayOf(p))
	}
	if got := a.journalPage(c, day.AddDate(0, 0, 1)); got != nil {
		t.Fatal("the next day took the page")
	}
}
