package main

import (
	"os"
	"strings"
	"testing"
	"time"
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
	other.Save("- [ ] öteki görev\n")
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
	if body, _ := other.Body(); !strings.Contains(body, "- [x] öteki görev") {
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
	body, _ := a.page.Body()
	src, _ := os.ReadFile(a.page.file())
	if !strings.Contains(body, "- [x] ilk görev") || !strings.Contains(string(src), "[x] ilk görev") {
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
	other.Save("başka\n")
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
