package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// TestListSpecs: a list's look is read from the ID of one of its items and holds for all of its items, those Enter
// made too; a deeper list between them is a list of its own; Roman numerals count (#493).
func TestListSpecs(t *testing.T) {
	s := listSpec{mark: "⭐", num: "ROMAN", task: "number"}
	if got, ok := parseListSpec(s.id()); !ok || got != s {
		t.Fatalf("spec back: %+v %v from %q", got, ok, s.id())
	}
	if (listSpec{}).id() != "" {
		t.Fatal("the usual look has an ID")
	}
	if _, ok := parseListSpec("celestine:task:urgent"); ok {
		t.Fatal("a task's kind read as a list's look")
	}
	for n, want := range map[int]string{1: "i", 4: "iv", 9: "ix", 14: "xiv", 40: "xl", 1994: "mcmxciv", 4000: "4000"} {
		if got := romanNumeral(n, false); got != want {
			t.Errorf("roman %d = %q, not %q", n, got, want)
		}
	}
	if romanNumeral(12, true) != "XII" {
		t.Error("capitals")
	}
	bullets := richtext.ParagraphStyle{List: richtext.Bullets}
	star := bullets
	star.ID = listSpec{mark: "⭐"}.id()
	deeper := richtext.ParagraphStyle{List: richtext.Numbers, Indent: 1}
	txt := richtext.TextOf(
		richtext.Paragraph{Style: bullets, Runs: []richtext.Run{{Text: "bir"}}},
		richtext.Paragraph{Style: star, Runs: []richtext.Run{{Text: "iki"}}}, // its look holds for the whole list
		richtext.Paragraph{Style: deeper, Runs: []richtext.Run{{Text: "a"}}},
		richtext.Paragraph{Style: bullets, Runs: []richtext.Run{{Text: "üç"}}},
		richtext.Paragraph{Runs: []richtext.Run{{Text: "ara"}}},
		richtext.Paragraph{Style: bullets, Runs: []richtext.Run{{Text: "başka"}}},
	)
	lm := listsOf(txt)
	for _, p := range []int{0, 1, 3} {
		if it := lm.items[p]; it == nil || it.spec.mark != "⭐" {
			t.Fatalf("item %d has not its list's look: %+v", p, it)
		}
	}
	if lm.items[2] != nil || lm.items[5] != nil {
		t.Fatal("a look outside its list")
	}
	if it := lm.all[3]; it.n != 3 {
		t.Fatalf("the third item counted %d", it.n)
	}
}

// TestListLooks: from the block's menu a bulleted list takes a star for its mark, a numbered one Roman numerals,
// and a list's items become tasks that keep their numbers before the boxes; every item of the list has the look,
// one Enter made too, and the page keeps it (#493).
func TestListLooks(t *testing.T) {
	a, tt := newTestApp(t, "- bir\n- iki\n\n1. elma\n2. armut\n")
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	a.setListSpec(0, listSpec{mark: iconMarkPrefix + "star"})
	if it := a.listItemAt(a.pane(), 1); it == nil || it.spec.mark != "icon:star" {
		t.Fatalf("the second item has not the star: %+v", it)
	}
	// the menu's lines for a numbered list, and Roman numerals from them
	t2 := a.pane().doc.Text()
	at := strings.Index(t2.String(), "elma")
	at = len([]rune(t2.String()[:at]))
	var numbering *menuItem
	for _, it := range a.listItems(at) {
		if it.title == "lists_numbering" {
			numbering = &it
		}
	}
	if numbering == nil {
		t.Fatalf("no numbering in the menu of %q", t2.Slice(at, at+4).String())
	}
	for _, it := range numbering.items() {
		if it.title == "I, II, III" {
			it.run()
		}
	}
	p := a.lists().byStart[at]
	if it := a.listItemAt(a.pane(), p+1); it == nil || listMarkText(it, richtext.Numbers) != "II." {
		t.Fatalf("not in Roman numerals: %+v", it)
	}
	// tasks, the numbers kept before the boxes
	a.listToTasks(p)
	st := a.pane().doc.Text().ParagraphStyleAt(at)
	if st.List != richtext.Tasks || !a.markedTask(a.pane(), p, st) || !a.markedTask(a.pane(), p+1, a.pane().doc.Text().ParagraphStyleAt(at+5)) {
		t.Fatalf("not tasks with their numbers: %+v", st)
	}
	// a new item at the end, as Enter makes it: no ID, the list's look
	end := a.pane().doc.Text().Len()
	a.pane().doc.SetSelection(end, end)
	a.pane().doc.InsertText("\nkiraz")
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if it := a.listItemAt(a.pane(), p+2); it == nil || listMarkText(it, richtext.Tasks) != "III." {
		t.Fatalf("the new item has not the list's look: %+v", it)
	}
	// the box of a numbered task is at the end of the marker's room, and a click on it checks the task
	if err := tt.Click("Görev"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if !a.pane().doc.Text().ParagraphStyleAt(at).Checked {
		t.Fatal("the numbered task's box did not check it")
	}
	body := encodeBody(a.pane().doc.Text())
	if !strings.Contains(body, listIDPrefix) || !strings.Contains(body, "num=ROMAN") {
		t.Fatalf("the page did not keep the look: %s", body)
	}
}

// TestInlineTask: a box typed after words is a task in the running text, checked by a click, kept by the page and
// written [x] in Markdown; at the start of a line it is a line's task, as before (#493).
func TestInlineTask(t *testing.T) {
	a, tt := newTestApp(t, "Ara\n")
	tt.Frame()
	end := a.pane().doc.Text().Len()
	a.pane().doc.SetSelection(end, end)
	tt.Frame()
	typeEach(tt, " süt al [ ] ve ekmek")
	tt.Advance(200 * time.Millisecond)
	if n := strings.Count(a.pane().doc.Text().String(), string(richtext.ObjectReplacement)); n != 1 {
		t.Fatalf("%d chips in %q", n, a.pane().doc.Text().String())
	}
	tt.Frame()
	if len(a.chipSeen) != 1 {
		t.Fatalf("%d chips drawn", len(a.chipSeen))
	}
	r := a.chipSeen[0].r
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(200 * time.Millisecond)
	if md := mdOf(a.pane().doc); !strings.Contains(md, `süt al \[x\] ve ekmek`) && !strings.Contains(md, "süt al [x] ve ekmek") {
		t.Fatalf("not checked: %q", mdOf(a.pane().doc))
	}
	body := encodeBody(a.pane().doc.Text())
	if !strings.Contains(body, `"k":"check"`) || !strings.Contains(body, `"target":"x"`) {
		t.Fatalf("the page did not keep the task: %s", body)
	}
	if got := decodeBody(body); !strings.Contains(strings.ReplaceAll(bodyMarkdown(got), `\`, ""), "[x]") {
		t.Fatalf("read back: %q", bodyMarkdown(got))
	}
	// at the start of a line: not a chip
	if a.checkShortcut(richtext.NewText("[ ]"), 3, true) {
		t.Fatal("a line's box taken for one in the text")
	}
}

// The paint makes no map of the lists' items for a text none of whose lists has a look: typing in a long list
// rebuilt them at each key (#543). A list with a look still has its marks.
func TestListLooksOnlyWhenThere(t *testing.T) {
	var b strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&b, "- item %d\n", i)
	}
	a, tt := newTestApp(t, b.String())
	tt.Frame()
	a.pane().doc.SetSelection(3, 3)
	tt.Type("x")
	tt.Frame()
	if a.listMarks != nil {
		t.Fatal("the lists' items were made for a text with no look")
	}
	if a.looks == nil || len(a.looks.items) != 0 {
		t.Fatalf("looks: %+v", a.looks)
	}
	a.setListSpec(0, listSpec{mark: "★"})
	tt.Frame()
	if it := a.listItemAt(a.pane(), 0); it == nil || it.spec.mark != "★" {
		t.Fatalf("a list with a look has no mark: %+v", it)
	}
}

// The settings' definitions are made once, not at each call (#543).
func TestSettingDefsOnce(t *testing.T) {
	if a, b := settingDefs(), settingDefs(); len(a) == 0 || &a[0] != &b[0] {
		t.Fatal("made again")
	}
}

// TestOwnMarker: the editor paints no marker the app draws itself, the box of every task and the mark of a list with
// a look of its own, and paints the others; the app covered the editor's before, a dark square on the glass (#664).
func TestOwnMarker(t *testing.T) {
	a, tt := newTestApp(t, "- bir\n- iki\n\n1. elma\n2. armut\n\n- [ ] görev\n")
	tt.Frame()
	pn := a.pane()
	own := func(word string) bool {
		t.Helper()
		i := 0
		for _, para := range pn.doc.Text().Paragraphs() {
			var line strings.Builder
			for _, r := range para.Runs {
				line.WriteString(r.Text)
			}
			if line.String() == word {
				return a.ownMarker(pn, i, para.Style)
			}
			i++
		}
		t.Fatalf("no paragraph %q", word)
		return false
	}
	if !own("görev") {
		t.Fatal("the editor paints a task's box under the app's")
	}
	if own("bir") || own("elma") {
		t.Fatal("the editor paints no marker of a list without a look of its own")
	}
	a.setListSpec(0, listSpec{mark: "★"})
	tt.Frame()
	if !own("bir") || !own("iki") {
		t.Fatal("the editor paints the bullets of a list the app marks with a star")
	}
	if own("elma") {
		t.Fatal("the editor paints no number of a list without a look of its own")
	}
}
