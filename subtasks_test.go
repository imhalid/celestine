package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

const subtaskPage = "- [ ] Launch\n    - [x] Notes\n    - [x] Demo\n    - [ ] Bug\n        - [ ] Reproduce\n        - [x] Test\n    - [ ] Tell\n- [ ] Room\n"

// TestSubtaskCounts: a task with subtasks counts its direct ones, a room opens under its line for its chip, a click
// on the chip folds them, and the fold's chip counts the same; the file stays the nested list it was (#477).
func TestSubtaskCounts(t *testing.T) {
	setLang("en")
	t.Cleanup(func() { setLang("tr") })
	a, tt := newTestApp(t, subtaskPage)
	tt.SetSize(1100, 900)
	a.open(a.pane().page)
	tt.Advance(300 * time.Millisecond)
	a.mu.Lock()
	st := a.subtasksOf(a.pane())
	launch, bug := st.count[0], st.count[3]
	room := a.paraRoom(a.pane(), 1)
	a.mu.Unlock()
	if launch != (subtaskCount{done: 2, total: 4}) || bug != (subtaskCount{done: 1, total: 2}) {
		t.Fatalf("counts: Launch %+v, Bug %+v", launch, bug)
	}
	if room != subtaskRoom {
		t.Fatalf("the room under Launch is %v", room)
	}
	label := locales.TasksSubtasksDone(2, 4)
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	a.mu.Lock()
	ch := foldOf(a.pane().doc.Text(), 0)
	_, still := a.subtasksOf(a.pane()).count[0]
	a.mu.Unlock()
	if ch == nil || still {
		t.Fatalf("the chip did not fold the subtasks: fold %v, counted %v", ch != nil, still)
	}
	if n, done := foldTasks(ch); n != 4 || done != 2 {
		t.Fatalf("the fold's chip counts %d/%d, not the direct 2/4", done, n)
	}
	a.mu.Lock()
	a.unfold(ch)
	md := a.currentMarkdown(a.pane().page)
	a.mu.Unlock()
	if !strings.Contains(md, "    - [ ] Bug\n        - [ ] Reproduce") || strings.Contains(md, "2/4") {
		t.Fatalf("the file is not the nested list:\n%s", md)
	}
}
