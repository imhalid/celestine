package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// In a task of the running text its words are shown as one while the caret is in them; Tab ends it where the caret
// is and the caret goes on outside it, the line not indented; what is typed after is not the task's, in the file, the
// task view and the strike of a task done; Tab in a task that has its end goes past it (#610).
func TestInlineTaskTabEnds(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 800)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "Al /görev")
	tt.Advance(150 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(150 * time.Millisecond)
	typeEach(tt, "süt")
	tt.Frame()
	pn := a.pane()
	if !a.inlineTaskAt(pn).ok {
		t.Fatal("the caret is not in the task")
	}
	tt.Frame()
	if act := pn.inlineAct; !act.ok {
		t.Fatal("the task is not shown as one")
	}
	banded := false
	p := strings.Count(pn.doc.Text().String(), "\n") // the last paragraph
	for _, s := range a.paintSpans(pn, p, pn.doc.Text().ParagraphStyleAt(pn.doc.Text().Len()), spansOfPara(a, p)) {
		if strings.Contains(s.Text, "süt") && s.Background.A > 0 {
			banded = true
		}
	}
	if !banded {
		t.Fatal("its words have no band")
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.inlineTaskAt(pn).ok {
		t.Fatal("still in the task after Tab")
	}
	typeEach(tt, "ve ekmek")
	tt.Frame()
	if got := mdOf(pn.doc); got != "x\n\nAl [ ] süt<!--/task--> ve ekmek" {
		t.Fatalf("written: %q", got)
	}
	if st := pn.doc.Text().ParagraphStyleAt(pn.doc.Text().Len()); st.Indent != 0 || st.List != 0 {
		t.Fatalf("Tab indented the line: %+v", st)
	}
	var texts []string
	for _, k := range pageTasks(nil, bodyMarkdown(pn.doc.Text())) {
		texts = append(texts, k.text)
	}
	if strings.Join(texts, "|") != "süt" {
		t.Fatalf("the task view: %v", texts)
	}
	// done: "süt" struck, what is after its end not
	a.toggleCheckChip(checksIn(a)[0])
	tt.Frame()
	if s := struck(a.inlineDoneSpans(pn, p, spansOfPara(a, p))); s != "süt" {
		t.Fatalf("struck %q", s)
	}
	// back in the task: Tab goes past its end, adding none
	text := pn.doc.Text()
	at := strings.Index(string([]rune(text.String())), "süt")
	pn.doc.SetSelection(len([]rune(text.String()[:at]))+1, len([]rune(text.String()[:at]))+1)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if got := mdOf(pn.doc); got != "x\n\n"+"Al [x] süt<!--/task--> ve ekmek" {
		t.Fatalf("Tab with an end: %q", got)
	}
	if a.inlineTaskAt(pn).ok {
		t.Fatal("Tab did not go past the end")
	}
}

func TestInlineTaskCopyAndCodeEnd(t *testing.T) {
	a, tt := newTestApp(t, "Al [ ] süt<!--/task--> ve ekmek\n")
	tt.Frame()
	src := a.pane().doc.Text()
	plain := withoutChips(src).PlainText()
	html := withoutChips(src).HTML()
	if strings.Contains(plain, "<!--/task-->") || strings.Contains(html, "/task") {
		t.Fatalf("checkend leaked: plain=%q html=%q", plain, html)
	}
	lineTasks := tasksOf(a.pane().page, "- [ ] Buy [ ] milk<!--/task--> today\n")
	if len(lineTasks) != 1 || strings.Contains(lineTasks[0].text, "<!--/task-->") {
		t.Fatalf("tasksOf leaked marker: %+v", lineTasks)
	}
	inl := inlineTasksOf(a.pane().page, "Note [ ] fix `<!--/task-->` bug\n")
	if len(inl) != 1 || !strings.Contains(inl[0].text, "bug") {
		t.Fatalf("inlineTasksOf split on code span: %+v", inl)
	}
}
