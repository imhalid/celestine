package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func TestTasksOf(t *testing.T) {
	p := &Page{}
	got := tasksOf(p, "- [ ] bir [[Sayfa|ad]] görevi\n- [x] bitti\ntext\n  * [X] iç\n```\n- [ ] kodda\n```\n- [ ]\n1. [ ] numaralı")
	// a numbered task is a task too, as the page reads it (#536)
	if len(got) != 4 || got[0].text != "bir ad görevi" || got[0].done || !got[1].done || got[1].line != 1 || !got[2].done || got[2].line != 3 || got[3].text != "numaralı" || got[3].line != 8 {
		t.Fatalf("%+v", got)
	}
}

// TestTaskView: the tasks of every page in one view; ticking one writes the
// page, open or not, and the filter tells the done from the open.
func TestTaskView(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] ilk görev\n- [x] yapılmış\n\nmetin\n")
	tt.SetSize(1100, 800)
	other, _ := a.pane().page.Coll.NewPage(nil, "Öteki")
	other.SaveText("- [ ] öteki görev\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	if err := tt.Click("Görevler"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !a.pane().tasksOpen || !tt.HasText("ilk görev") || !tt.HasText("öteki görev") || tt.HasText("yapılmış") {
		t.Fatalf("view: open %v %v", a.pane().tasksOpen, tt.Texts())
	}
	if !tt.HasText("Açık 2/3") || !tt.HasText("Tamamlanan 1/3") { // the stats beside the title
		t.Fatalf("count: %v", tt.Texts())
	}
	// ticked in a page that is not open: its file
	if err := tt.Click("Görev: öteki görev"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if body, _ := other.Text(); !strings.Contains(body, "- [x] öteki görev") {
		t.Fatalf("file: %q", body)
	}
	tt.Advance(time.Second) // it goes out of the open ones, its letters flying off (#494)
	if tt.HasText("öteki görev") {
		t.Fatal("a done task stays in the open list")
	}
	// ticked in the page that is open: the file and the editor's text
	if err := tt.Click("Görev: ilk görev"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	body, _ := a.pane().page.Text()
	src, _ := os.ReadFile(a.pane().page.File())
	if !strings.Contains(body, "- [x] ilk görev") || len(src) == 0 {
		t.Fatalf("file: %q", body)
	}
	if md := a.pageMarkdown(a.pane().page, a.pane().doc.Text()); !strings.Contains(md, "- [x] ilk görev") {
		t.Fatalf("the editor's text: %q", md)
	}
	if !a.lib.Collections[0].Pages[0].Modified.IsZero() && a.pane().dirty {
		t.Fatal("left dirty")
	}
	// the filter: done ones
	if err := tt.Click("Tamamlanan"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("ilk görev") || !tt.HasText("yapılmış") || !tt.HasText("öteki görev") {
		t.Fatalf("done: %v", tt.Texts())
	}
	// back and the palette
	a.openTasks()
	if got := a.goTo(place{"tasks", ""}); !got {
		t.Fatal("no place")
	}
	if a.here().kind != "tasks" {
		t.Fatalf("here %v", a.here())
	}
}

// TestTickingKeepsTheOrder: ticking a task off writes its page, which changes it last, and its tasks stay where
// they were in the view; opened again, the view sorts the pages as they are then.
func TestTickingKeepsTheOrder(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] ilk görev\n")
	other, _ := a.pane().page.Coll.NewPage(nil, "Öteki")
	other.SaveText("- [ ] öteki görev\n")
	other.Modified = a.pane().page.Modified.Add(time.Hour) // changed last: first
	a.openTasks()
	tt.Frame()
	pages := func() []*Page {
		var out []*Page
		for _, k := range a.allTasks(true) {
			if len(out) == 0 || out[len(out)-1] != k.page {
				out = append(out, k.page)
			}
		}
		return out
	}
	before := pages()
	if len(before) != 2 || before[0] != other {
		t.Fatalf("not the one changed last first: %v", before)
	}
	for _, k := range a.allTasks(true) {
		if k.page == a.pane().page {
			a.toggleTask(k)
		}
	}
	a.pane().page.Modified = other.Modified.Add(time.Hour) // as the save makes it, whatever the clock
	if after := pages(); after[0] != before[0] || after[1] != before[1] {
		t.Fatalf("ticked, the order changed: %v, was %v", after, before)
	}
	a.openTasks()
	if again := pages(); again[0] != a.pane().page {
		t.Fatalf("opened again, not the one changed last first: %v", again)
	}
}

// TestTaskDue: a day named in a task is its day; the filter groups by it.
func TestTaskDue(t *testing.T) {
	p := &Page{}
	got := tasksOf(p, "- [ ] rapor [[2026-11-10]] bitir\n- [ ] günsüz\n- [ ] [[Sayfa]] bak")
	if got[0].text != "rapor  bitir" && got[0].text != "rapor bitir" {
		t.Fatalf("text %q", got[0].text)
	}
	if got[0].due.Format("2006-01-02") != "2026-11-10" || !got[1].due.IsZero() || got[2].text != "Sayfa bak" {
		t.Fatalf("%+v", got)
	}
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC) // a Thursday
	day := func(s string) taskItem { d, _ := parseDay(s); return taskItem{due: d} }
	for _, tc := range []struct {
		due   string
		group int
		want  bool
	}{
		{"2026-11-05", 1, true}, {"2026-11-06", 1, false},
		{"2026-11-08", 2, true}, {"2026-11-09", 2, false}, {"2026-11-04", 2, false},
		{"2026-11-04", 3, true}, {"2026-11-05", 3, false},
	} {
		if dueMatches(day(tc.due), tc.group, now) != tc.want {
			t.Errorf("%s in group %d: want %v", tc.due, tc.group, tc.want)
		}
	}
	if dueMatches(taskItem{}, 1, now) || !dueMatches(taskItem{}, 0, now) {
		t.Error("a task without a day")
	}
}

// TestTaskGoesToItsLine: a task's text opens its page with the caret on it.
func TestTaskGoesToItsLine(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] ilk\n\nmetin\n\n- [ ] ikinci [[2026-11-10]]\n- [ ] üçüncü\n")
	other, _ := a.pane().page.Coll.NewPage(nil, "Öteki")
	other.SaveText("başka\n")
	a.open(other)
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tt.Click("Görevler")
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Satıra git: üçüncü"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if a.pane().page.Title != "Sayfa" || a.pane().tasksOpen {
		t.Fatalf("page %q", a.pane().page.Title)
	}
	_, caret := a.pane().doc.Selection()
	ps, pe := a.pane().doc.Text().ParagraphRange(caret)
	if got := a.pane().doc.Text().Slice(ps, pe).String(); got != "üçüncü" {
		t.Fatalf("the caret is in %q", got)
	}
}

func TestParseEvery(t *testing.T) {
	for in, want := range map[string]string{
		"çöp every:week":    "her hafta",
		"vitamin her:2gün":  "her 2 günde",
		"kira EVERY:month":  "her ay",
		"rapor her:3hafta":  "her 3 haftada",
		"yılbaşı every:yıl": "her yıl",
	} {
		r, ok := parseEvery(in)
		if !ok || r.label() != want {
			t.Errorf("%q: %v %q, want %q", in, ok, r.label(), want)
		}
	}
	if _, ok := parseEvery("her gün bir şey"); ok {
		t.Error("a word is not a recurrence")
	}
	p := &Page{}
	k := tasksOf(p, "- [ ] çöpü çıkar [[2026-11-05]] every:hafta")
	if len(k) != 1 || k[0].text != "çöpü çıkar" || k[0].every != "her hafta" || k[0].due.IsZero() {
		t.Fatalf("%+v", k)
	}
}

// TestRecurringTask: ticking a task that comes again leaves it done and makes
// the next one, a period on from its day (or from today, when it has none).
func TestRecurringTask(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] çöpü çıkar [[2026-11-05]] every:hafta\n- [ ] vitamin her:2gün\n- [ ] bir kez\n")
	a.clock = func() time.Time { return time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC) }
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tasks := a.allTasks(true)
	var weekly, daily taskItem
	for _, k := range tasks {
		switch k.text {
		case "çöpü çıkar":
			weekly = k
		case "vitamin":
			daily = k
		}
	}
	a.toggleTask(weekly)
	a.links.built = time.Time{}
	a.linksStale()
	for _, k := range a.allTasks(true) { // the lines moved: they are read again, as the view reads them
		if k.text == "vitamin" {
			daily = k
		}
	}
	a.toggleTask(daily)
	body, _ := a.pane().page.Text()
	for _, want := range []string{
		"- [x] çöpü çıkar [[2026-11-05]]\n",
		"- [ ] çöpü çıkar [[2026-11-12]] every:hafta",
		"- [x] vitamin\n",
		"- [ ] vitamin [[2026-11-07]] her:2gün",
		"- [ ] bir kez",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("no %q in:\n%s", want, body)
		}
	}
	// a plain task is only ticked
	for _, k := range a.allTasks(true) {
		if k.text == "bir kez" {
			a.toggleTask(k)
		}
	}
	if body, _ = a.pane().page.Text(); !strings.Contains(body, "- [x] bir kez") || strings.Count(body, "bir kez") != 1 {
		t.Fatalf("%s", body)
	}
}

// TestRecurringInText: a task ticked in the page's own text comes again too.
func TestRecurringInText(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] çöpü çıkar [[2026-11-05]] every:hafta\n- [ ] başka\n")
	a.clock = func() time.Time { return time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC) }
	tt.Frame()
	a.pane().doc.ToggleChecked(0)
	tt.Frame()
	a.save()
	md := a.pageMarkdown(a.pane().page, a.pane().doc.Text())
	if !strings.Contains(md, "- [x] çöpü çıkar [[2026-11-05]]\n") || !strings.Contains(md, "- [ ] çöpü çıkar [[2026-11-12]] every:hafta\n") || !strings.Contains(md, "- [ ] başka") {
		t.Fatalf("%q", md)
	}
	a.save()
	if again := a.pageMarkdown(a.pane().page, a.pane().doc.Text()); again != md {
		t.Fatalf("it rolls on again:\n%q\n%q", md, again)
	}
}

// TestRecurringMonthEnd verifies month-end and leap-year recurrence clamping.
func TestRecurringMonthEnd(t *testing.T) {
	loc := time.UTC
	jan31 := time.Date(2026, 1, 31, 9, 0, 0, 0, loc)
	if got := (recur{n: 1, unit: "month"}).next(jan31); got != time.Date(2026, 2, 28, 9, 0, 0, 0, loc) {
		t.Fatalf("2026-01-31 + 1 month = %s, want 2026-02-28", got.Format("2006-01-02"))
	}
	feb29 := time.Date(2024, 2, 29, 9, 0, 0, 0, loc)
	if got := (recur{n: 1, unit: "year"}).next(feb29); got != time.Date(2025, 2, 28, 9, 0, 0, 0, loc) {
		t.Fatalf("2024-02-29 + 1 year = %s, want 2025-02-28", got.Format("2006-01-02"))
	}
	// ticked off month after month, it comes back to the 31st: the day it is meant for is kept while it is cut (#214)
	line := "- [x] kira [[2026-01-31]] every:month"
	var days []string
	for _, now := range []time.Time{time.Date(2026, 2, 1, 9, 0, 0, 0, loc), time.Date(2026, 3, 1, 9, 0, 0, 0, loc), time.Date(2026, 4, 1, 9, 0, 0, 0, loc), time.Date(2026, 5, 1, 9, 0, 0, 0, loc)} {
		next, ok := comesAgain(line, now)
		if !ok {
			t.Fatalf("no next of %q", line)
		}
		days = append(days, next)
		line = strings.Replace(next, "[ ]", "[x]", 1)
	}
	want := []string{"- [ ] kira [[2026-02-28]] every:month@31", "- [ ] kira [[2026-03-31]] every:month", "- [ ] kira [[2026-04-30]] every:month@31", "- [ ] kira [[2026-05-31]] every:month"}
	if !slices.Equal(days, want) {
		t.Fatalf("month after month:\n%q\nwant\n%q", days, want)
	}
	if next, _ := comesAgain("- [x] doğum günü [[2024-02-29]] her:yıl", time.Date(2024, 3, 1, 0, 0, 0, 0, loc)); next != "- [ ] doğum günü [[2025-02-28]] her:yıl@29" {
		t.Fatalf("a leap day: %q", next)
	}
}

// A library of thousands of tasks builds only the rows in view, and the tasks
// are still all there: the list scrolls on, and a click on a task goes to its line.
func TestTasksViewBuildsOnlyWhatShows(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("K")
	for i := 0; i < 40; i++ {
		p, _ := c.NewPage(nil, fmt.Sprintf("Sayfa %02d", i))
		var b strings.Builder
		for j := 0; j < 100; j++ {
			fmt.Fprintf(&b, "- [ ] görev %02d-%03d\n", i, j)
		}
		p.SaveText(b.String())
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1000, 700)
	a.pane().tasksOpen, a.pane().coll = true, nil
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("4000") { // the open ones, beside the title
		t.Fatalf("not 4000 tasks: %v", tt.Texts()[:min(8, len(tt.Texts()))])
	}
	built := 0
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "görev ") {
			built++
		}
	}
	if built == 0 || built > 120 {
		t.Fatalf("%d tasks built of 4000", built)
	}
	// the wheel takes the list on: others show
	first := ""
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "görev ") {
			first = s
			break
		}
	}
	for i := 0; i < 20; i++ {
		tt.Wheel(500, 400, 0, 2000)
		tt.Advance(100 * time.Millisecond)
	}
	later := ""
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "görev ") {
			later = s
			break
		}
	}
	if later == "" || later == first {
		t.Fatalf("the list did not scroll on: %q then %q", first, later)
	}
}

// The tasks are searched, chosen by collection, and a page's folded away; with
// nothing to show the view says why and offers to clear what narrows it.
func TestTasksFilterSearchFold(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := lib.NewCollection("Kitaplar")
	k2, _ := lib.NewCollection("Projeler")
	p1, _ := k1.NewPage(nil, "Dune")
	p2, _ := k2.NewPage(nil, "Ev tadilatı")
	p1.SaveText("- [ ] baharat siparişi\n- [ ] kapak tartışması\n")
	p2.SaveText("- [ ] boya al\n- [ ] kapak ölç\n")
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 760)
	a.pane().tasksOpen, a.pane().coll = true, nil
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, s := range []string{"baharat siparişi", "boya al", "kapak ölç", "kapak tartışması"} {
		if !tt.HasText(s) {
			t.Fatalf("no task %q: %v", s, tt.Texts())
		}
	}
	a.pane().taskQuery = "kapak"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("baharat siparişi") || tt.HasText("boya al") || !tt.HasText("kapak ölç") || !tt.HasText("kapak tartışması") {
		t.Fatalf("search kapak: %v", tt.Texts())
	}
	a.pane().taskQuery = "TADİLAT" // a page's title, folded once a frame (#195)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("boya al") || !tt.HasText("kapak ölç") || tt.HasText("baharat siparişi") {
		t.Fatalf("search by a page's title: %v", tt.Texts())
	}
	a.pane().taskQuery = "KAPAK"
	a.pane().taskColl = k2.Dir
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("kapak tartışması") || !tt.HasText("kapak ölç") {
		t.Fatalf("search kapak in Projeler: %v", tt.Texts())
	}
	a.pane().taskQuery = "yok böyle bir şey"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Bu filtreye uyan görev yok") || !tt.HasText("Filtreleri temizle") {
		t.Fatalf("no empty state: %v", tt.Texts())
	}
	if err := tt.Click("Filtreleri temizle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.pane().taskQuery != "" || a.pane().taskColl != "" || !tt.HasText("baharat siparişi") {
		t.Fatalf("filters not cleared: %q %q", a.pane().taskQuery, a.pane().taskColl)
	}
	// a page's tasks fold away, and come back
	if err := tt.Click("Görevleri kapat: Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("baharat siparişi") || !tt.HasText("boya al") {
		t.Fatalf("Dune's tasks did not fold: %v", tt.Texts())
	}
	if err := tt.Click("Görevleri aç: Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("baharat siparişi") {
		t.Fatalf("Dune's tasks did not unfold: %v", tt.Texts())
	}
}

// A box ticked in the text leaves the page where it is: the caret may be far away (here at the end),
// and an edit that moves nothing does not take the page to it.
func TestTickingABoxKeepsThePage(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "Satır %d\n\n", i)
	}
	b.WriteString("- [ ] bir\n- [ ] iki\n- [ ] üç\n")
	a, tt := newTestApp(t, b.String())
	tt.SetSize(1200, 700)
	tt.Advance(400 * time.Millisecond)
	a.pane().pageScroll.Y = 0
	tt.Advance(200 * time.Millisecond)
	at := strings.Index(a.pane().doc.Text().String(), "iki")
	a.pane().doc.ToggleChecked(at)
	tt.Advance(600 * time.Millisecond)
	if a.pane().pageScroll.Y != 0 {
		t.Fatalf("the page went to %v", a.pane().pageScroll.Y)
	}
	if !strings.Contains(a.pane().doc.Text().String(), "iki") {
		t.Fatal("the task is gone")
	}
}

// TestTaskKinds: a task knows its kind from its box, and the view shows the open tasks of one kind.
func TestTaskKinds(t *testing.T) {
	got := tasksOf(&Page{}, "- [!] acil\n- [/] sürüyor\n- [ ] düz\n- [x] bitti\n- [?] soru\n")
	kinds := []string{}
	for _, k := range got {
		kinds = append(kinds, taskKindKey(k))
	}
	if strings.Join(kinds, ",") != "important,progress,plain,plain,question" || got[3].kind != "" || !got[3].done {
		t.Fatalf("%v", kinds)
	}
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := lib.NewCollection("İş")
	p1, _ := k1.NewPage(nil, "Plan")
	p1.SaveText("- [!] faturayı öde\n- [/] raporu yaz\n- [ ] çiçekleri sula\n")
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1100, 760)
	a.pane().tasksOpen, a.pane().coll = true, nil
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("faturayı öde") || !tt.HasText("raporu yaz") || !tt.HasText("çiçekleri sula") {
		t.Fatalf("not all: %v", tt.Texts())
	}
	for kind, want := range map[string]string{"important": "faturayı öde", "progress": "raporu yaz", "plain": "çiçekleri sula"} {
		a.pane().taskKindF = kind
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		n := 0
		for _, s := range []string{"faturayı öde", "raporu yaz", "çiçekleri sula"} {
			if tt.HasText(s) {
				n++
			}
		}
		if n != 1 || !tt.HasText(want) {
			t.Fatalf("%s: %v", kind, tt.Texts())
		}
	}
	// a kind no open task is any more is let go
	a.pane().taskKindF = "error"
	tt.Frame()
	if a.pane().taskKindF != "" {
		t.Fatalf("kept %q", a.pane().taskKindF)
	}
}

// Ticking a task from the Tasks view, a quick note and a tag renamed change the body where they must and nothing
// else: a starred task, an alignment, empty lines and underline stay as they were (#123).
func TestBodyEditsKeepTheRest(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	p := a.pane().page
	star := richtext.ParagraphStyle{List: richtext.Tasks, ID: taskIDPrefix + "star"}
	under := richtext.Attributes{Underline: true}
	rich := func(firstDone bool) richtext.Text {
		task := richtext.ParagraphStyle{List: richtext.Tasks, Checked: firstDone}
		return richtext.TextOf(
			richtext.Paragraph{Style: task, Runs: []richtext.Run{{Text: "işaretlenecek #fikir"}}},
			richtext.Paragraph{Style: star, Runs: []richtext.Run{{Text: "yıldızlı"}}},
			richtext.Paragraph{},
			richtext.Paragraph{},
			richtext.Paragraph{Style: richtext.ParagraphStyle{Align: ui.Center}, Runs: []richtext.Run{{Text: "ortada "}, {Text: "altı çizili", Attributes: under}}},
		)
	}
	if err := p.Save(encodeBody(rich(false))); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(p)
	body, _ := p.Text()
	a.toggleTask(tasksOf(p, body)[0])
	got, _ := p.Body()
	if want := encodeBody(rich(true)); got != want {
		t.Fatalf("ticked:\n%s\nwant\n%s", got, want)
	}
	if n, err := a.renameTag("fikir", "plan"); err != nil || n != 1 {
		t.Fatalf("renamed %d: %v", n, err)
	}
	got, _ = p.Body()
	if want := strings.Replace(encodeBody(rich(true)), "#fikir", "#plan", 1); got != want {
		t.Fatalf("renamed:\n%s\nwant\n%s", got, want)
	}
	before := got
	got = appendBody(before, importMarkdown("yeni not\n"))
	if !strings.HasPrefix(got, strings.TrimSuffix(before, "]")) || !strings.Contains(got, "yeni not") {
		t.Fatalf("appended:\n%s", got)
	}
}

// The tasks in the running text are in the task view with the lines' tasks: each says the words after its box,
// a tick there writes its chip and no other, and closing a page's tasks closes them too (#536).
func TestInlineTasksInView(t *testing.T) {
	a, tt := newTestApp(t, "Buy [ ] milk and [x] bread.\n\n- [ ] call [ ] Ana\n")
	tt.Frame()
	a.save()
	var got []string
	for _, k := range a.allTasks(true) {
		got = append(got, fmt.Sprintf("%d.%d %v %s", k.line, k.inl, k.done, k.text))
	}
	want := []string{"0.1 false milk and", "0.2 true bread", "2.0 false call Ana", "2.1 false Ana"}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("tasks:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var milk taskItem
	for _, k := range a.allTasks(true) {
		if k.inl == 1 && k.line == 0 {
			milk = k
		}
	}
	a.mu.Lock()
	a.toggleTaskSoon(milk)
	a.mu.Unlock()
	a.toggling.Wait()
	tt.Frame()
	_, done := checkChips(a.pane().doc.Text())
	if fmt.Sprint(done) != "[true true false]" {
		t.Fatalf("the chips after the tick: %v", done)
	}
	if a.pane().doc.Text().ParagraphStyleAt(a.pane().doc.Text().Len() - 1).Checked {
		t.Fatal("the line's task was ticked too")
	}
	// an agent's set_task: by its line and its box, at once (toggleTask)
	a.linksStale()
	for _, k := range a.allTasks(true) {
		if k.line == 0 && k.inl == 2 {
			a.toggleTask(k)
		}
	}
	if _, done := checkChips(a.pane().doc.Text()); fmt.Sprint(done) != "[true false false]" {
		t.Fatalf("the chips after the agent's untick: %v", done)
	}
	a.linksStale()
	var open []taskItem
	for _, k := range a.allTasks(true) {
		if !k.done {
			open = append(open, k)
		}
	}
	a.mu.Lock()
	a.closeTasks(a.pane().page, open)
	a.mu.Unlock()
	a.toggling.Wait()
	tt.Frame()
	a.linksStale()
	for _, k := range a.allTasks(true) {
		if !k.done {
			t.Errorf("open after closing all: %d.%d %s", k.line, k.inl, k.text)
		}
	}
}

// TestTasksGlide: a ticked task that leaves the open ones lets the rows under it glide up to their places, no frame
// moving them far (#577); with reduced motion they are there at once.
func TestTasksGlide(t *testing.T) {
	for _, still := range []bool{false, true} {
		a, tt := newTestApp(t, "- [ ] birinci\n- [ ] ikinci\n- [ ] üçüncü\n")
		tt.SetSize(1100, 800)
		a.settings.ReduceMotion = still
		a.linksStale()
		a.links.built = time.Time{}
		tt.Frame()
		if err := tt.Click("Görevler"); err != nil {
			t.Fatal(err)
		}
		tt.Advance(300 * time.Millisecond)
		from, ok := tt.Find("Görev: üçüncü")
		if !ok {
			t.Fatalf("no row: %v", tt.Texts())
		}
		if err := tt.Click("Görev: birinci"); err != nil {
			t.Fatal(err)
		}
		ys := []float32{from.Y}
		for range 250 {
			tt.Advance(8 * time.Millisecond)
			if r, ok := tt.Find("Görev: üçüncü"); ok {
				ys = append(ys, r.Y)
			}
		}
		end := ys[len(ys)-1]
		if end >= from.Y {
			t.Fatalf("still %v: the row did not go up: %v to %v", still, from.Y, end)
		}
		jump := float32(0)
		for i := 1; i < len(ys); i++ {
			jump = max(jump, ys[i-1]-ys[i])
		}
		if still && jump != from.Y-end || !still && jump > 3 {
			t.Fatalf("reduce motion %v: a frame moved the row %v (%v to %v): %v", still, jump, from.Y, end, ys)
		}
	}
}
