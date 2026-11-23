package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
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
	a.mu.Lock() // as a frame does: the toggle's write waits for it
	a.toggleTaskSoon(k)
	shown := false
	for _, x := range a.allTasks(false) {
		if x.line == k.line {
			shown = x.done
		}
	}
	a.mu.Unlock()
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

// TestToggleKeepsNewerEdits: a page written between the read of a task's toggle and its write, as an autosave of
// what is typed in another pane, or typed and not yet saved, is not written over: the toggle is made again from
// the page as it is, and both the typing and the tick are kept, in the file and in the editor (#530).
func TestToggleKeepsNewerEdits(t *testing.T) {
	for _, saved := range []bool{true, false} {
		a, tt := newTestApp(t, "- [ ] one\n- [ ] two\n")
		k := taskIn(t, a, "one")
		now := a.clock()
		prepare := func(r pageRead) taskToggle { return prepareToggle(k, r, now) }
		j := prepare(readOf(a.page)) // read, as toggleAway reads it away from the window
		j.again = prepare
		a.doc.SetText(importMarkdown("- [ ] one\n- [ ] two\n\ntyped meanwhile\n"))
		a.dirty = true
		if saved {
			a.save() // the autosave lands before the toggle is written
		}
		a.applyToggle(j)
		a.toggling.Wait()
		tt.Frame()
		md, _ := a.page.Text()
		if !strings.Contains(md, "typed meanwhile") || !strings.Contains(md, "- [x] one") || !strings.Contains(md, "- [ ] two") {
			t.Fatalf("saved %v: the file is\n%s", saved, md)
		}
		if got := a.doc.Text().String(); !strings.Contains(got, "typed meanwhile") {
			t.Fatalf("saved %v: the editor lost what was typed: %q", saved, got)
		}
		if !taskIn(t, a, "one").done {
			t.Fatalf("saved %v: the index does not have it ticked", saved)
		}
	}
}

// TestToggleKeepsAPull: a page's file written between the read of a toggle and its write, not through the page, as
// a sync's pull writes it, is not written over: the toggle is made again from it. A page the library no longer holds
// (a pull reads the library again) is not written, and a tick that cannot be made is told (#561).
func TestToggleKeepsAPull(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] one\n- [ ] two\n")
	p := a.page
	pull := func(from, to string) { // what a pass writes: the file, through the store, the page not told
		src, err := p.FS().ReadFile(p.File())
		if err != nil {
			t.Fatal(err)
		}
		if err := p.FS().WriteFile(p.File(), []byte(strings.Replace(string(src), from, to, 1))); err != nil {
			t.Fatal(err)
		}
	}
	k := taskIn(t, a, "one")
	now := a.clock()
	prepare := func(r pageRead) taskToggle { return prepareToggle(k, r, now) }
	j := prepare(readOf(p))
	j.again = prepare
	pull("two", "two, pulled")
	a.applyToggle(j)
	a.toggling.Wait()
	tt.Frame()
	src, _ := p.FS().ReadFile(p.File())
	if !strings.Contains(string(src), "two, pulled") {
		t.Fatalf("the pulled copy was written over: %s", src)
	}
	if !taskIn(t, a, "one").done {
		t.Fatal("the tick was not made again from the pulled copy")
	}

	// written meanwhile each time it is made: told, and nothing written
	k = taskIn(t, a, "two, pulled")
	j = prepareToggle(k, readOf(p), now)
	j.again, j.tries = prepare, taskToggleTries
	pull("one", "one, pulled again")
	a.notice = ""
	a.applyToggle(j)
	a.toggling.Wait()
	if a.notice != locales.TasksToggleNotMade() {
		t.Fatalf("a tick not made, not told: %q", a.notice)
	}
	if src, _ := p.FS().ReadFile(p.File()); !strings.Contains(string(src), "one, pulled again") {
		t.Fatalf("the newer copy was written over: %s", src)
	}

	// the library read again between the read and the write: the old page is not written
	j = prepareToggle(taskIn(t, a, "two, pulled"), readOf(p), now)
	a.reloadLibrary()
	a.notice = ""
	writes := a.libWrites()
	a.applyToggle(j)
	a.toggling.Wait()
	if a.notice != locales.TasksToggleNotMade() || a.libWrites() != writes {
		t.Fatalf("a page the library no longer holds: notice %q, written %v", a.notice, a.libWrites() != writes)
	}
}
