package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// The new statuses are Markdown boxes too, read and written back: [w] waiting, [$] billable, [-] canceled (#597).
func TestTaskStatusesMarkdown(t *testing.T) {
	md := "- [/] başladı\n- [w] bekliyor\n- [$] faturalı\n- [-] iptal\n- [e] uyarı\n- [ ] düz"
	text := importMarkdown(md)
	var ids []string
	for _, at := range paraStarts(text) {
		k, _ := taskKindOf(text.ParagraphStyleAt(at))
		ids = append(ids, k.id)
	}
	if got := strings.Join(ids, ","); got != "progress,blocked,billable,canceled,error," {
		t.Fatalf("kinds: %s", got)
	}
	if back := bodyMarkdown(text); back != md {
		t.Fatalf("written back:\n%s", back)
	}
}

// A tag status is found by its tag in a task's line, whatever its case, the settings' name for its slot in place of
// its own; a tag that only starts like one, or in a word, is not one (#597).
func TestTagStatusFound(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	for _, c := range []struct {
		line string
		slot int
		tag  string
	}{
		{"Ödev #Moon bitmedi", 0, "#Moon"}, {"#bug in the parser", 5, "#bug"}, {"a#bug, not one", -1, ""},
		{"#bugs are many", -1, ""}, {"plain", -1, ""}, {"çiçek #flower #sun", 2, "#flower"},
	} {
		slot, from, to := a.tagStatusIn(c.line)
		if slot != c.slot || slot >= 0 && string([]rune(c.line)[from:to]) != c.tag {
			t.Errorf("%q: slot %d at %d-%d, want %d %q", c.line, slot, from, to, c.slot, c.tag)
		}
	}
	a.settings.TaskTags = []string{"#gece"}
	if slot, _, _ := a.tagStatusIn("iş #gece"); slot != 0 {
		t.Fatal("a slot renamed is not found by its name")
	}
	if slot, _, _ := a.tagStatusIn("iş #moon"); slot != -1 {
		t.Fatal("a slot renamed is still found by its own name")
	}
}

// ⌥-click on a task's box opens the picker; 1 to 7 give the task a tag status, written at its line's end in place of
// another, a kind chosen takes the tag away, and 0 leaves a plain task. A plain click still ticks it (#597).
func TestTaskStatusPicker(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] ödev #sun\n- [ ] düz\n")
	tt.SetSize(1000, 700)
	tt.Advance(200 * time.Millisecond)
	pn := a.pane()
	b := pn.boxes[0]
	r, ok := a.taskBoxAt(pn, b, pn.doc.Text().ParagraphStyleAt(b.Start))
	if !ok {
		t.Fatal("no box")
	}
	x, y := pn.editorR.X+r.X+r.W/2, pn.editorR.Y+r.Y+r.H/2
	tt.ClickAtWith(ui.Alt, x, y)
	tt.Frame()
	if !a.tasks.pick.open || pn.doc.Text().ParagraphStyleAt(0).Checked {
		t.Fatalf("⌥-click: picker %v, checked %v", a.tasks.pick.open, pn.doc.Text().ParagraphStyleAt(0).Checked)
	}
	tt.Frame()
	tt.Key(0, ui.Key1)
	tt.Frame()
	line := func() string { s, e := pn.doc.Text().ParagraphRange(0); return paraLine(pn.doc.Text(), s, e) }
	if a.tasks.pick.open || line() != "ödev #moon" {
		t.Fatalf("1: %q, picker %v", line(), a.tasks.pick.open)
	}
	if f := a.faceOf(pn.doc.Text().ParagraphStyleAt(0), line()); f.id != "tag" || f.slot != 0 {
		t.Fatalf("face %+v", f)
	}
	a.setTaskStatus(pn, 0, "billable", -1)
	if k, ok := taskKindOf(pn.doc.Text().ParagraphStyleAt(0)); !ok || k.id != "billable" || line() != "ödev" {
		t.Fatalf("a kind: %q, %+v", line(), k)
	}
	a.tasks.pick = taskPick{open: true, at: 0, pn: pn}
	tt.Frame()
	tt.Frame()
	tt.Key(0, ui.Key0)
	tt.Frame()
	if _, ok := taskKindOf(pn.doc.Text().ParagraphStyleAt(0)); ok || line() != "ödev" || a.tasks.pick.open {
		t.Fatalf("0: %q", line())
	}
	if !strings.Contains(bodyMarkdown(pn.doc.Text()), "- [ ] ödev") {
		t.Fatalf("on disk: %q", bodyMarkdown(pn.doc.Text()))
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if !pn.doc.Text().ParagraphStyleAt(0).Checked {
		t.Fatal("a plain click no longer ticks")
	}
}

// A task's status tag is in its color in the line (drawTags puts a pill of it under), a canceled task's words faint, and a canceled subtask is
// not counted; the counts go when the settings hide them (#597).
func TestTaskStatusSpansAndCounts(t *testing.T) {
	a, tt := newTestApp(t, "- [ ] üst #deadline\n\t- [x] bir\n\t- [ ] iki\n\t- [-] üç\n- [-] iptal\n")
	tt.SetSize(1000, 700)
	tt.Advance(200 * time.Millisecond)
	pn := a.pane()
	spans := a.paintSpans(pn, 0, pn.doc.Text().ParagraphStyleAt(0), spansOfPara(a, 0))
	pill := false
	for _, s := range spans {
		if s.Text == "#deadline" && s.Color == ui.Hex(tagStatusSlots[3].color) {
			pill = true
		}
	}
	if !pill {
		t.Fatalf("no pill: %+v", spans)
	}
	s4, _ := pn.doc.Text().ParagraphRange(len([]rune(pn.doc.Text().String())) - 1)
	last := strings.Count(pn.doc.Text().Slice(0, s4).String(), "\n")
	for _, s := range a.paintSpans(pn, last, pn.doc.Text().ParagraphStyleAt(s4), spansOfPara(a, last)) {
		if s.Color != a.theme.Muted {
			t.Fatalf("a canceled task's %q is not faint", s.Text)
		}
	}
	if n := a.subtasksOf(pn).count[0]; n.total != 2 || n.done != 1 {
		t.Fatalf("counts %+v, want 1/2", n)
	}
	a.settings.TaskNoCounts = true
	if n := len(a.subtasksOf(pn).count); n != 0 {
		t.Fatalf("hidden, still %d counted", n)
	}
	_ = richtext.Tasks
}

// Every status is drawn in each preset, done or not, without a frame asked for at rest unless the ambient motions are
// on; TASK_SHOT_DIR keeps a picture of them (#597).
func TestTaskFacesDrawn(t *testing.T) {
	md := "- [ ] open\n- [/] started\n- [w] waiting\n- [$] billable\n- [!] important\n- [?] discuss\n- [e] alert\n- [*] starred\n- [-] canceled\n" +
		"- [ ] moon #moon\n- [ ] kawaii #kawaii\n- [ ] flower #flower\n- [ ] deadline #deadline\n- [ ] draft #draft\n- [ ] bug #bug\n- [ ] sun #sun\n" +
		"- [x] done\n- [x] done star\n"
	for _, look := range []string{taskLookNeon, taskLookVivid, taskLookMinimal} {
		a, tt := newTestApp(t, md)
		a.settings.TaskLook = look
		a.settings.TaskTint = look == taskLookNeon
		tt.SetSize(900, 1100)
		tt.Advance(400 * time.Millisecond)
		if dir := os.Getenv("TASK_SHOT_DIR"); dir != "" {
			f, _ := os.Create(filepath.Join(dir, "tasks-"+map[string]string{"": "neon"}[look]+look+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
}

// Settings ▸ Editor has the task boxes: the presets as tiles, the switches, and the tag statuses' names; a name
// typed there is the slot's (#597).
func TestTaskSettings(t *testing.T) {
	a, tt := newTestApp(t, "- [/] x\n")
	tt.SetSize(1400, 5600) // the whole Editor section, its carets' tiles first
	a.sidebarClosed = true
	a.pane().settingsOpen, a.pane().settingsTab = true, tabEditor
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click(msg("tasks_lookVivid")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.TaskLook != taskLookVivid {
		t.Fatalf("look %q", a.settings.TaskLook)
	}
	for _, id := range []string{"tasks_motion", "tasks_burst", "tasks_tint", "tasks_counts", "tasks_tagStatuses"} {
		if _, ok := tt.Find(msg(id)); !ok {
			t.Fatalf("no %q", msg(id))
		}
	}
	if err := tt.Click(locales.TasksTagSlot(1)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	typeEach(tt, "gece") // in an empty field, "moon" faint in it
	tt.Frame()
	tt.Frame()
	if n := a.tagNames()[0]; n != "gece" {
		t.Fatalf("slot 1 named %q", n)
	}
	for range 4 {
		tt.Key(0, ui.KeyBackspace)
		tt.Frame()
	}
	if n := a.tagNames()[0]; n != "moon" || a.settings.TaskTags[0] != "" {
		t.Fatalf("cleared: slot 1 named %q, kept %q", n, a.settings.TaskTags[0])
	}
}
