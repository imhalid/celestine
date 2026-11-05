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
