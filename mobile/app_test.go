package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// registerOnce registers the app's font once for the tests.
var registerOnce sync.Once

// newTestApp is the app on a library of its own, its view driven by a Tester of a phone's size.
func newTestApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	registerOnce.Do(registerFonts)
	a := newApp(t.TempDir())
	if err := a.open(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 412, 860)
	tt.Frame()
	return a, tt
}

// A new library has the desktop's journal; today's card makes today's page, as the desktop makes a day, and what is
// typed in it is saved to its file as the library's document (#633).
func TestTodayPageWritten(t *testing.T) {
	a, tt := newTestApp(t)
	j := a.journalColl()
	if j == nil || !j.Journal || j.Icon != "calendar" {
		t.Fatalf("journal %+v", j)
	}
	if err := tt.Click(locales.TodayPage()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	p := a.todayPage()
	if p == nil || p.Props[journalDayKey] != startOfDay(time.Now()).Format("2006-01-02") {
		t.Fatalf("today's page %+v", p)
	}
	if a.ed.id != p.ID {
		t.Fatal("today's page not opened")
	}
	a.ed.doc.SetText(textOf(library.Doc{{R: []library.Run{{T: "Yazdım "}, {T: "kalın", W: 700}}}}))
	a.typed(tt.Now())
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()
	md, _ := p.Text()
	if !strings.Contains(md, "Yazdım **kalın**") {
		t.Fatalf("saved %q", md)
	}
	a.router.Back()
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("Yazdım kalın") {
		t.Fatalf("today's card: %v", tt.Texts())
	}
}

// The + makes a note in a collection of notes, made when there is none, and opens it, its title asked for first; a
// past day of the journal is read only, and a day is not moved to the trash.
func TestNewNoteAndLockedDay(t *testing.T) {
	a, tt := newTestApp(t)
	if err := tt.Click(locales.PlusOpen()); err != nil { // the + opens what it makes
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click(locales.ActionNote()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	p := a.lib.PageByID(a.ed.id)
	if p == nil || p.Coll.Name != "Notes" {
		t.Fatalf("new note %+v", p)
	}
	if a.ed.title != "" { // asked for empty, through the screens' slide too, not "Untitled" to type after
		t.Fatalf("a new note's title field holds %q", a.ed.title)
	}
	a.ed.title = "Alışveriş"
	a.typed(tt.Now())
	a.router.Back()
	tt.Advance(400 * time.Millisecond)
	if p.Title != "Alışveriş" {
		t.Fatalf("title %q", p.Title)
	}
	j := a.journalColl()
	j.Creating = true
	past, err := j.NewPage(nil, "2026-01-01")
	j.Creating = false
	if err != nil {
		t.Fatal(err)
	}
	past.Props = map[string]string{journalDayKey: "2026-01-01"}
	if err := past.Save(""); err != nil {
		t.Fatal(err)
	}
	a.lockDays()
	a.openPage(past)
	tt.Advance(400 * time.Millisecond)
	if a.ed.readOnly == "" || !tt.HasText(locales.PageLocked()) {
		t.Fatalf("a past day editable: %q", a.ed.readOnly)
	}
	a.trash(past)
	if a.lib.PageByID(past.ID) == nil {
		t.Fatal("a day went to the trash")
	}
}

// The search finds a page by its words, Turkish letters by their plain ones.
func TestSearch(t *testing.T) {
	a, tt := newTestApp(t)
	c, _ := a.lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Gezi")
	p.SaveText("İstanbul'da çarşı")
	a.goTo(pathSearch)
	a.query = "carsi"
	tt.Advance(600 * time.Millisecond)
	if !tt.HasText("Gezi") {
		t.Fatalf("not found: %v", tt.Texts())
	}
}

// formatDay writes a day as the desktop's journal formats do.
func TestFormatDay(t *testing.T) {
	d := time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local)
	for layout, want := range map[string]string{"": "2026-11-27", "DD.MM.YYYY": "27.11.2026", `D \de MMMM`: "27 de November"} {
		if got := formatDay(layout, d); got != want {
			t.Errorf("%q: %q, want %q", layout, got, want)
		}
	}
}

// The menu goes to the app's places, and the tasks not done of the pages show on the home screen and the Tasks
// screen, which taps through to their page.
func TestMenuAndTasks(t *testing.T) {
	a, tt := newTestApp(t)
	c, _ := a.lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Plan")
	p.SaveText("- [ ] süt al\n- [x] yapıldı\n- [ ] ara")
	tt.Frame()
	if n := len(a.openTasks()); n != 2 {
		t.Fatalf("%d open tasks", n)
	}
	if err := tt.Click(locales.MenuOpen()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click(locales.NavTasks()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.router.Path() != pathTasks || a.menuOpen || !tt.HasText("süt al") || tt.HasText("yapıldı") {
		t.Fatalf("tasks: %s %v %v", a.router.Path(), a.menuOpen, tt.Texts())
	}
	if err := tt.Click("Plan"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.ed.id != p.ID {
		t.Fatal("the task's page not opened")
	}
}

// The streak counts the days in a row the journal has words, up to today or yesterday.
func TestStreak(t *testing.T) {
	a, _ := newTestApp(t)
	j := a.journalColl()
	now := time.Now()
	for _, back := range []int{1, 2, 4} {
		d := startOfDay(now).AddDate(0, 0, -back)
		j.Creating = true
		p, _ := j.NewPage(nil, d.Format("2006-01-02"))
		j.Creating = false
		p.Props = map[string]string{journalDayKey: d.Format("2006-01-02")}
		p.SaveText("yazdım")
	}
	if n := a.streak(now); n != 2 {
		t.Fatalf("streak %d, want 2", n)
	}
}

// The + makes today's page, a note, a note in each further collection (two at most) and a search: the first
// collection of notes is New note's alone.
func TestPlusActions(t *testing.T) {
	a, _ := newTestApp(t)
	for _, name := range []string{"Notes", "İş", "Okuma", "Fikirler"} {
		a.lib.NewCollection(name)
	}
	var labels []string
	for _, act := range a.plusActions() {
		labels = append(labels, act.label)
	}
	want := []string{locales.ActionToday(), locales.ActionNote(), locales.ActionIn("İş"), locales.ActionIn("Okuma"), locales.ActionSearch()}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", labels)
	}
}
