package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// bigTaskPage is a page of n tasks, large enough that its Markdown is kept (library.TextOfBody).
func bigTaskPage(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "- [ ] a task among many, written out at some length so the page is large %d\n", i)
	}
	return b.String()
}

// TestToggleTaskSoon: a task ticked off in the task view shows done at once, and its page is written away from
// the window; the Markdown the toggle hands the indexes is the page's own (#504).
func TestToggleTaskSoon(t *testing.T) {
	a, tt := newTestApp(t, bigTaskPage(2500))
	a.openTasks()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	var k taskItem
	for _, x := range a.allTasks(true) {
		if strings.HasSuffix(x.text, " 7") {
			k = x
		}
	}
	if k.page == nil {
		t.Fatal("no task 7")
	}
	a.toggleTaskSoon(k)
	shown := false
	for _, x := range a.allTasks(false) {
		if x.line == k.line {
			shown = x.done
		}
	}
	if !shown {
		t.Fatal("the list did not show the task done at once")
	}
	a.toggling.Wait()
	tt.Frame()
	body, _ := a.page.Body()
	md := library.TextOfBody(body) // as the toggle handed it over
	if fresh := bodyText(body); strings.TrimRight(md, "\n") != strings.TrimRight(fresh, "\n") {
		t.Fatal("the Markdown handed over is not the page's")
	}
	if !strings.Contains(md, "- [x] a task among many, written out at some length so the page is large 7\n") {
		t.Fatal("not ticked in the file")
	}
	if strings.Count(md, "- [x]") != 1 {
		t.Fatalf("%d ticked", strings.Count(md, "- [x]"))
	}
}

// TestCompleteAllTasks: a page's head in the task view ticks off every open task shown under it, after asking, in
// one write (#505).
func TestCompleteAllTasks(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] bir\n- [x] iki\n- [ ] üç\n- [ ] dört\n")
	a.openTasks()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := tt.Click("Açık görevlerin hepsini tamamla: " + a.page.Title); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText("Bu görevler tamamlansın mı?") {
		t.Fatalf("not asked: %v", tt.Texts())
	}
	if err := tt.Click("Hepsini tamamla"); err != nil {
		t.Fatal(err)
	}
	a.toggling.Wait()
	tt.Advance(time.Second)
	body, _ := a.page.Text()
	if strings.Count(body, "- [x]") != 4 || strings.Contains(body, "- [ ]") {
		t.Fatalf("not all done:\n%s", body)
	}
}

// taskIn is page p's task that says text, as the task view has it.
func taskIn(t *testing.T, a *app, text string) taskItem {
	t.Helper()
	for _, k := range a.allTasks(true) {
		if k.page == a.page && k.text == text {
			return k
		}
	}
	t.Fatalf("no task %q", text)
	return taskItem{}
}

// TestTaskFindsItsParagraph: a task is ticked in its own line where the Markdown counts a task the text has in no
// task paragraph (a line in a block of HTML), or the text one the Markdown does not count (a task in a quote), and
// the index and the file then agree (#529).
func TestTaskFindsItsParagraph(t *testing.T) {
	for _, c := range []struct{ src, other, want string }{
		{"<div>\n- [ ] fake\n</div>\n\n- [ ] one\n- [ ] two", "<div>\n- [ ] fake\n</div>", "- [x] one\n- [ ] two"},
		{"> - [ ] quoted\n\n- [ ] one\n- [ ] two", "> - [ ] quoted", "- [x] one\n- [ ] two"},
	} {
		a, _ := newTestApp(t, c.src)
		body, _ := a.page.Body()
		tx, md := decodeBody(body), library.TextOfBody(body)
		for _, k := range tasksOf(a.page, md) {
			at, ok := taskParagraph(tx, md, a.page, k)
			if k.text == "fake" {
				if ok {
					t.Fatalf("%q: the line in the HTML block found a paragraph", c.src)
				}
				continue
			}
			if !ok {
				t.Fatalf("%q: %q is called stale", c.src, k.text)
			}
			s, e := tx.ParagraphRange(at)
			if got := tx.Slice(s, e).String(); got != k.text {
				t.Fatalf("%q: %q found the paragraph %q", c.src, k.text, got)
			}
		}
		a.toggleTask(taskIn(t, a, "one"))
		md, _ = a.page.Text()
		if !strings.Contains(md, c.want) || !strings.Contains(md, c.other) {
			t.Fatalf("%q: ticked as\n%s", c.src, md)
		}
		if !taskIn(t, a, "one").done || taskIn(t, a, "two").done {
			t.Fatalf("%q: the index and the file disagree", c.src)
		}
	}
}

// TestCompleteAllFindsTheParagraphs: "Complete all" ticks the page's own tasks, and leaves a task's line in a block
// of HTML as it is (#529).
func TestCompleteAllFindsTheParagraphs(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"<div>\n- [ ] fake\n</div>\n\n- [ ] one\n- [ ] two", "<div>\n- [ ] fake\n</div>\n\n- [x] one\n- [x] two"},
		{"> - [ ] quoted\n\n- [ ] one\n- [ ] two", "> - [ ] quoted\n- [x] one\n- [x] two"},
	} {
		a, _ := newTestApp(t, c.src)
		var ts []taskItem
		for _, k := range a.allTasks(true) {
			if k.page == a.page {
				ts = append(ts, k)
			}
		}
		a.closeTasks(a.page, ts)
		a.toggling.Wait()
		if md, _ := a.page.Text(); strings.TrimRight(md, "\n") != c.want {
			t.Fatalf("%q: closed as\n%s", c.src, md)
		}
	}
}
