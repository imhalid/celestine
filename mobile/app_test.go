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

// A new library has the desktop's journal; the +'s first makes today's page, as the desktop makes a day, and what is
// typed in it is saved to its file as the library's document (#633).
func TestTodayPageWritten(t *testing.T) {
	a, tt := newTestApp(t)
	j := a.journalColl()
	if j == nil || !j.Journal || j.Icon != "calendar" {
		t.Fatalf("journal %+v", j)
	}
	if err := tt.Click(locales.PlusOpen()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if err := tt.Click(locales.ActionToday()); err != nil {
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
		t.Fatalf("the pages written lately: %v", tt.Texts())
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
	a.menuOpen = true // as a pull down past the top opens it (the Tester's fingers make no pull, #638)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	tt.Press(380, 820) // a tap but on a place closes it
	tt.Release(380, 820)
	tt.Advance(400 * time.Millisecond)
	if a.menuOpen {
		t.Fatal("the menu not closed by a tap beside its places")
	}
	a.menuOpen = true
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if err := tt.Click(locales.NavTasks()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.router.Path() != pathTasks || a.menuOpen || !tt.HasText("süt al") || tt.HasText("yapıldı") {
		t.Fatalf("tasks: %s %v %v", a.router.Path(), a.menuOpen, tt.Texts())
	}
	// Ticked off, the task is written to its page and shown done until the screen is left; ticked again, it is open.
	if err := tt.Click("süt al"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if md, _ := p.Text(); !strings.Contains(md, "- [x] süt al") || len(a.openTasks()) != 1 || !tt.HasText("süt al") {
		t.Fatalf("ticked: %q %d", md, len(a.openTasks()))
	}
	if err := tt.Click("süt al"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.openTasks()) != 2 {
		t.Fatal("not ticked back")
	}
	tt.Click("süt al")
	tt.Frame()
	if err := tt.Click("Plan"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.ed.id != p.ID || len(a.ticked) != 0 {
		t.Fatal("the task's page not opened")
	}
	a.router.Back()
	tt.Advance(600 * time.Millisecond)
	if tt.HasText("süt al") {
		t.Fatal("a task done still shown, the screen left and come back to")
	}
	// A past day's tasks stay as they were.
	j := a.journalColl()
	d := startOfDay(time.Now()).AddDate(0, 0, -3)
	j.Creating = true
	day, _ := j.NewPage(nil, d.Format("2006-01-02"))
	j.Creating = false
	day.Props = map[string]string{journalDayKey: d.Format("2006-01-02")}
	day.SaveText("- [ ] eski iş")
	a.lockDays()
	tt.Frame()
	if err := tt.Click("eski iş"); err != nil {
		t.Fatal(err)
	}
	if md, _ := day.Text(); strings.Contains(md, "[x]") || a.notice != locales.TasksLocked() {
		t.Fatalf("a past day's task ticked: %q %q", md, a.notice)
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

// The field's words are light by night and on the dark look, dark by day (#636).
func TestSky(t *testing.T) {
	day := func(h int) time.Time { return time.Date(2026, 11, 27, h, 0, 0, 0, time.Local) }
	if s := skyAt(day(13), false); s.light != 0 {
		t.Fatalf("noon's words light %v", s.light)
	}
	if s := skyAt(day(23), false); s.light != 1 {
		t.Fatalf("night's words dark %v", s.light)
	}
	if s := skyAt(day(13), true); s.light != 1 {
		t.Fatalf("noon on the dark look's words dark %v", s.light)
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

// A collection held opens its sheet under its preview, which renames it, colors it, and titles the journal's days
// again by the way chosen (#637, #638, #640).
func TestCollSettings(t *testing.T) {
	a, tt := newTestApp(t)
	j := a.journalColl()
	d := startOfDay(time.Now()).AddDate(0, 0, -1)
	j.Creating = true
	p, _ := j.NewPage(nil, d.Format("2006-01-02"))
	j.Creating = false
	p.Props = map[string]string{journalDayKey: d.Format("2006-01-02")}
	p.SaveText("dün")
	a.openCollection(j)
	tt.Advance(400 * time.Millisecond)
	a.holdCollection(j, ui.Rect{}) // as its name held opens its sheet, under its preview (#638, #640)
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText(locales.CollSettings()) || !tt.HasText(titleOf(p)) {
		t.Fatalf("the sheet and the preview: %v", tt.Texts())
	}
	a.setCollLook(j, "c3", "star") // a color and an icon are kept at once
	if c2 := a.collAt(j.Dir); c2.Color != "c3" || c2.Icon != "star" {
		t.Fatalf("look %q %q", c2.Color, c2.Icon)
	}
	// The journal's days titled by a way chosen in the sheet, the past ones too; the name kept as the sheet closes.
	if err := tt.Click(formatDay("D MMMM YYYY", time.Now())); err != nil {
		t.Fatal(err)
	}
	a.cs.name = "Günlük"
	a.closeAll()
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if j.Name != "Günlük" || j.JournalFormat != "D MMMM YYYY" || p.Title != formatDay("D MMMM YYYY", d) {
		t.Fatalf("%q %q %q", j.Name, j.JournalFormat, p.Title)
	}
	if a.router.Path() != "/c/"+j.Dir {
		t.Fatalf("at %s", a.router.Path())
	}
}

// The sentence of the day's marks open what they count, the weekday at the top is at least four letters, and the +'s
// list rises in rather than standing at once where it ends (#643).
func TestSummaryAndPlusMotion(t *testing.T) {
	a, tt := newTestApp(t)
	c, _ := a.lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Plan")
	p.SaveText("- [ ] süt al")
	tt.Frame()
	if n := len(a.weekPages(time.Now())); n != 1 {
		t.Fatalf("%d pages this week", n)
	}
	if err := tt.Click("1 note"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(500 * time.Millisecond)
	if a.router.Path() != pathWeek || !tt.HasText("Plan") {
		t.Fatalf("at %s: %v", a.router.Path(), tt.Texts())
	}
	a.router.Back()
	tt.Advance(500 * time.Millisecond)
	if err := tt.Click("1 open task"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(500 * time.Millisecond)
	if a.router.Path() != pathTasks {
		t.Fatalf("at %s", a.router.Path())
	}
	if got := homeDay(time.Date(2026, 11, 27, 9, 0, 0, 0, time.Local)); got != "Friday" {
		t.Fatalf("the weekday %q", got)
	}

	a.goTo(pathHome)
	tt.Advance(500 * time.Millisecond)
	if err := tt.Click(locales.PlusOpen()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(16 * time.Millisecond)
	early, _ := tt.Find(locales.ActionNote())
	tt.Advance(600 * time.Millisecond)
	late, ok := tt.Find(locales.ActionNote())
	if !ok || early.Y-late.Y < 2 {
		t.Fatalf("the list stood where it ends at once: %v then %v", early, late)
	}
}

// While a page is written, the task button makes the caret's line a task and is kept as the page is saved (#653).
func TestTaskButton(t *testing.T) {
	a, tt := newTestApp(t)
	c, _ := a.lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Alışveriş")
	p.SaveText("süt al")
	a.openPage(p)
	tt.Advance(2 * time.Second)
	chip, ok := tt.Find("Notes") // the text is under the collection's chip and the title
	if !ok {
		t.Fatalf("no chip: %v", tt.Texts())
	}
	tt.Press(chip.X+40, chip.Y+110)
	tt.Release(chip.X+40, chip.Y+110)
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click(locales.PageTask()); err != nil {
		t.Fatalf("%v: %v", err, tt.Texts())
	}
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()
	if md, _ := p.Text(); !strings.Contains(md, "- [ ] süt al") {
		t.Fatalf("saved %q", md)
	}
}

// A tap on a task's box ticks that task, the second of three too, and does not focus the text (#654, #655).
func TestTapTaskBox(t *testing.T) {
	a, tt := newTestApp(t)
	c, _ := a.lib.NewCollection("Notes")
	p, _ := c.NewPage(nil, "Alışveriş")
	p.SaveText("- [ ] ekmek\n- [ ] süt al\n- [ ] yumurta")
	a.openPage(p)
	tt.Advance(2 * time.Second)
	boxes := a.ed.doc.VisibleParagraphs()
	if len(boxes) == 0 {
		t.Fatal("no paragraphs shown")
	}
	var body ui.Element // the focus is read back from the frame
	a.tapText(&a.ed, body, 1, boxes[1].Top+4)
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()
	if md, _ := p.Text(); !strings.Contains(md, "- [ ] ekmek\n- [x] süt al\n- [ ] yumurta") {
		t.Fatalf("saved %q", md)
	}
	if a.ed.writing {
		t.Fatal("the text took the focus at a tap on its box")
	}
}
