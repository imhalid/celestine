package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
)

// A kind of task is read from its Markdown box and written back to it; a done one is "[x]".
func TestTaskKindsMarkdown(t *testing.T) {
	md := "- [*] yıldız\n- [ ] düz\n- [!] önemli\n- [?] soru\n- [/] sürüyor\n- [e] hata\n- [x] bitti"
	text := importMarkdown(md)
	var ids []string
	for _, at := range paraStarts(text) {
		st := text.ParagraphStyleAt(at)
		if st.List != richtext.Tasks {
			t.Fatalf("not a task at %d: %+v", at, st)
		}
		k, _ := taskKindOf(st)
		ids = append(ids, k.id)
	}
	if got := strings.Join(ids, ","); got != "star,,important,question,progress,error," {
		t.Fatalf("kinds: %s", got)
	}
	if back := bodyMarkdown(text); back != md {
		t.Fatalf("written back:\n%s\nnot:\n%s", back, md)
	}
	// done, it is "[x]"; the kind stays in the page
	s, _ := text.ParagraphRange(0)
	text = text.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { p.Checked = true })
	if back := bodyMarkdown(text); !strings.HasPrefix(back, "- [x] yıldız") {
		t.Fatalf("a done star: %q", back)
	}
	if k, ok := taskKindOf(text.ParagraphStyleAt(0)); !ok || k.id != "star" {
		t.Fatal("the kind went when it was done")
	}
	// a box that is no kind is no task, as before
	if st := importMarkdown("- [q] what").ParagraphStyleAt(0); st.List == richtext.Tasks {
		t.Fatal("[q] became a task")
	}
}

// "[!] " typed at a line's start makes it an important task; the / menu's kinds turn a line into them.
func TestTaskKindTypedAndTurned(t *testing.T) {
	a, tt := newTestApp(t, "\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	a.pane().doc.SetSelection(0, 0)
	typeEach(tt, "[!] ")
	tt.Frame()
	st := a.pane().doc.Text().ParagraphStyleAt(0)
	if k, ok := taskKindOf(st); !ok || k.id != "important" {
		t.Fatalf("typed [!]: %+v", st)
	}
	if a.pane().doc.Text().Slice(0, a.pane().doc.Text().Len()).String() != "" {
		t.Fatalf("the box's letters stayed: %q", a.pane().doc.Text().String())
	}
	turnInto(a.pane().doc, 0, "task:question")
	if k, ok := taskKindOf(a.pane().doc.Text().ParagraphStyleAt(0)); !ok || k.id != "question" {
		t.Fatal("turned into a question, it is not one")
	}
	found := false
	for _, k := range slashItems("soru") {
		found = found || k.kind == "task:question"
	}
	if !found {
		t.Fatal("the / menu has no Soru")
	}
}

// Checked, a task's box turns over and confetti flies, then falls away; unchecked, it turns back
// without any.
func TestTaskCheckTurnsAndBursts(t *testing.T) {
	a, tt := newTestApp(t, "- [*] yıldız\n")
	tt.SetSize(1000, 700)
	tt.Advance(100 * time.Millisecond)
	a.do(func() error { a.pane().doc.ToggleChecked(0); return nil })
	tt.Frame() // a click draws the editor again; the tester draws only when asked
	if len(a.tasks.flips) != 1 {
		t.Fatalf("no turn: %d", len(a.tasks.flips))
	}
	tt.Advance(400 * time.Millisecond)
	if len(a.tasks.bits) == 0 {
		t.Fatal("no confetti")
	}
	tt.Advance(2 * time.Second)
	if len(a.tasks.flips) != 0 || len(a.tasks.bits) != 0 {
		t.Fatalf("still turning or flying: %d %d", len(a.tasks.flips), len(a.tasks.bits))
	}
	a.do(func() error { a.pane().doc.ToggleChecked(0); return nil })
	tt.Frame() // a click draws the editor again; the tester draws only when asked
	tt.Advance(600 * time.Millisecond)
	if len(a.tasks.bits) != 0 {
		t.Fatal("confetti for an uncheck")
	}
	// typing changes the text: no task turns for it
	a.do(func() error { a.pane().doc.SetSelection(3, 3); a.pane().doc.InsertText("x"); return nil })
	tt.Frame()
	if len(a.tasks.flips) != 0 {
		t.Fatal("a turn for typing")
	}
}

// The tasks' screen finds the kinds of task, not done.
func TestTasksOfKinds(t *testing.T) {
	got := tasksOf(&Page{Title: "P"}, "- [*] yıldız\n- [!] önemli\n- [x] bitti")
	if len(got) != 3 || got[0].done || got[1].done || !got[2].done {
		t.Fatalf("%+v", got)
	}
}

// A right click on a task's box opens the menu of its kinds (the editor would take it for its own),
// and a kind chosen from it is the task's; a click on the box still ticks it.
func TestTaskBoxRightClickMenu(t *testing.T) {
	a, tt := newTestApp(t, "- [*] yıldız\n- [ ] düz\n")
	tt.SetSize(1000, 700)
	tt.Advance(200 * time.Millisecond)
	text := a.pane().doc.Text()
	b := a.pane().boxes[0]
	r, ok := a.taskBoxAt(a.pane(), b, text.ParagraphStyleAt(b.Start))
	if !ok {
		t.Fatal("no box")
	}
	x, y := a.pane().editorR.X+r.X+r.W/2, a.pane().editorR.Y+r.Y+r.H/2
	tt.RightClickAt(x, y)
	tt.Frame()
	if !a.ctx.open || len(a.ctx.items) == 0 || len(a.ctx.items) != len(taskKinds)+1 {
		t.Fatalf("no menu of kinds: open %v", a.ctx.open)
	}
	if m := tt.Menu(); len(m) == 0 || m[0] != msg(a.ctx.items[0].title) {
		t.Fatalf("the menu shown is not the app's: %v", m)
	}
	for _, it := range a.ctx.items {
		if it.title == "question" {
			it.run()
		}
	}
	if k, ok := taskKindOf(a.pane().doc.Text().ParagraphStyleAt(0)); !ok || k.id != "question" {
		t.Fatal("Soru chosen, the task is not one")
	}
	a.ctx.open = false
	tt.Frame()
	tt.ClickAt(x, y)
	tt.Frame()
	if !a.pane().doc.Text().ParagraphStyleAt(0).Checked {
		t.Fatal("a click on the box did not tick it")
	}
}

// The kinds of task the app draws are the kinds the library reads and writes in Markdown, each by the same character
// in its box (#595).
func TestTaskKindMarksAgree(t *testing.T) {
	if len(taskKinds) != len(library.TaskKindMarks) {
		t.Fatalf("%d kinds, the library has %d", len(taskKinds), len(library.TaskKindMarks))
	}
	for i, k := range taskKinds {
		if m := library.TaskKindMarks[i]; m.ID != k.id || m.MD != k.md {
			t.Errorf("kind %d: the app's %s [%s], the library's %s [%s]", i, k.id, k.md, m.ID, m.MD)
		}
	}
}
