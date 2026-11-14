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
