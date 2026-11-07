package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTasksOf(t *testing.T) {
	p := &Page{}
	got := tasksOf(p, "- [ ] bir [[Sayfa|ad]] görevi\n- [x] bitti\ntext\n  * [X] iç\n```\n- [ ] kodda\n```\n- [ ]\n1. [ ] numaralı")
	if len(got) != 3 || got[0].text != "bir ad görevi" || got[0].done || !got[1].done || got[1].line != 1 || !got[2].done || got[2].line != 3 {
		t.Fatalf("%+v", got)
	}
}

// TestTaskView: the tasks of every page in one view; ticking one writes the
// page, open or not, and the filter tells the done from the open.
func TestTaskView(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] ilk görev\n- [x] yapılmış\n\nmetin\n")
	tt.SetSize(1100, 800)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.SaveText("- [ ] öteki görev\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	if err := tt.Click("Görevler"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !a.tasksOpen || !tt.HasText("ilk görev") || !tt.HasText("öteki görev") || tt.HasText("yapılmış") {
		t.Fatalf("view: open %v %v", a.tasksOpen, tt.Texts())
	}
	if !tt.HasText("2 açık, 1 tamamlandı") {
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
	if tt.HasText("öteki görev") {
		t.Fatal("a done task stays in the open list")
	}
	// ticked in the page that is open: the file and the editor's text
	if err := tt.Click("Görev: ilk görev"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	body, _ := a.page.Text()
	src, _ := os.ReadFile(a.page.file())
	if !strings.Contains(body, "- [x] ilk görev") || len(src) == 0 {
		t.Fatalf("file: %q", body)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "- [x] ilk görev") {
		t.Fatalf("the editor's text: %q", md)
	}
	if !a.lib.Collections[0].Pages[0].Modified.IsZero() && a.dirty {
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
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
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
	if a.page.Title != "Sayfa" || a.tasksOpen {
		t.Fatalf("page %q", a.page.Title)
	}
	_, caret := a.doc.Selection()
	ps, pe := a.doc.Text().ParagraphRange(caret)
	if got := a.doc.Text().Slice(ps, pe).String(); got != "üçüncü" {
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
	body, _ := a.page.Text()
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
	if body, _ = a.page.Text(); !strings.Contains(body, "- [x] bir kez") || strings.Count(body, "bir kez") != 1 {
		t.Fatalf("%s", body)
	}
}

// TestRecurringInText: a task ticked in the page's own text comes again too.
func TestRecurringInText(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] çöpü çıkar [[2026-11-05]] every:hafta\n- [ ] başka\n")
	a.clock = func() time.Time { return time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC) }
	tt.Frame()
	a.doc.ToggleChecked(0)
	tt.Frame()
	a.save()
	md := a.pageMarkdown(a.page, a.doc.Text())
	if !strings.Contains(md, "- [x] çöpü çıkar [[2026-11-05]]\n") || !strings.Contains(md, "- [ ] çöpü çıkar [[2026-11-12]] every:hafta\n") || !strings.Contains(md, "- [ ] başka") {
		t.Fatalf("%q", md)
	}
	a.save()
	if again := a.pageMarkdown(a.page, a.doc.Text()); again != md {
		t.Fatalf("it rolls on again:\n%q\n%q", md, again)
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
	a.tasksOpen, a.coll = true, nil
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("4000 açık, 0 tamamlandı") && !tt.HasText("4000 açık, 0 tamamlandı  ·  sayfalardaki - [ ] satırları") {
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
	a.tasksOpen, a.coll = true, nil
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, s := range []string{"baharat siparişi", "boya al", "kapak ölç", "kapak tartışması"} {
		if !tt.HasText(s) {
			t.Fatalf("no task %q: %v", s, tt.Texts())
		}
	}
	a.taskQuery = "kapak"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("baharat siparişi") || tt.HasText("boya al") || !tt.HasText("kapak ölç") || !tt.HasText("kapak tartışması") {
		t.Fatalf("search kapak: %v", tt.Texts())
	}
	a.taskColl = k2.Dir
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("kapak tartışması") || !tt.HasText("kapak ölç") {
		t.Fatalf("search kapak in Projeler: %v", tt.Texts())
	}
	a.taskQuery = "yok böyle bir şey"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Bu süzgece uyan görev yok") || !tt.HasText("Süzgeçleri temizle") {
		t.Fatalf("no empty state: %v", tt.Texts())
	}
	if err := tt.Click("Süzgeçleri temizle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.taskQuery != "" || a.taskColl != "" || !tt.HasText("baharat siparişi") {
		t.Fatalf("filters not cleared: %q %q", a.taskQuery, a.taskColl)
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
	a.pageScroll.Y = 0
	tt.Advance(200 * time.Millisecond)
	at := strings.Index(a.doc.Text().String(), "iki")
	a.doc.ToggleChecked(at)
	tt.Advance(600 * time.Millisecond)
	if a.pageScroll.Y != 0 {
		t.Fatalf("the page went to %v", a.pageScroll.Y)
	}
	if !strings.Contains(a.doc.Text().String(), "iki") {
		t.Fatal("the task is gone")
	}
}
