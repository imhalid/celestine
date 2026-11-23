package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// para is a paragraph of style st and text, a check chip for each "☐" (not done) or "☑" (done) in it.
func para(st richtext.ParagraphStyle, text string) richtext.Paragraph {
	p := richtext.Paragraph{Style: st}
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			p.Runs = append(p.Runs, richtext.Run{Text: sb.String()})
			sb.Reset()
		}
	}
	for _, r := range text {
		if r != '☐' && r != '☑' {
			sb.WriteRune(r)
			continue
		}
		flush()
		ch := &chip{kind: "check"}
		if r == '☑' {
			ch.target = "x"
		}
		p.Runs = append(p.Runs, richtext.Run{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: &richtext.Attachment{Name: ch.label(), Data: ch}}})
	}
	flush()
	return p
}

// item is the style of an item of a list of kind, indent deep, with id.
func item(kind richtext.ListKind, indent int, id string) richtext.ParagraphStyle {
	return richtext.ParagraphStyle{List: kind, Indent: indent, ID: id}
}

// roundTrip writes t as Markdown and reads it back, as an agent's rewrite does (writeBody), and checks that the
// body comes back as it was, the Markdown holds want, and a second pass changes nothing.
func roundTrip(t *testing.T, name string, tx richtext.Text, want ...string) string {
	t.Helper()
	body := encodeBody(tx)
	md := bodyMarkdown(decodeBody(body))
	for _, w := range want {
		if !strings.Contains(md, w) {
			t.Errorf("%s: the Markdown has no %q:\n%s", name, w, md)
		}
	}
	back := encodeBody(importMarkdown(md))
	if back != body {
		t.Errorf("%s: body → Markdown → body changed it:\n%s\n%s\nthrough\n%s", name, body, back, md)
	}
	if again := bodyMarkdown(decodeBody(back)); again != md {
		t.Errorf("%s: a second pass changed the Markdown:\n%s\n%s", name, md, again)
	}
	return md
}

func TestListMarkdownRoundTrip(t *testing.T) {
	look := func(s listSpec) string { return s.id() }
	cases := []struct {
		name string
		tx   richtext.Text
		want []string
	}{
		{"inline tasks (the issue's)", richtext.TextOf(para(richtext.ParagraphStyle{}, "Buy ☐ milk ☑")), []string{"Buy [ ] milk [x]"}},
		{"inline tasks in a task, a heading, a quote", richtext.TextOf(
			para(item(richtext.Tasks, 0, ""), "call ☐ Ann (☑) ok."),
			para(richtext.ParagraphStyle{Heading: 2}, "Plan ☐"),
			para(richtext.ParagraphStyle{Quote: true}, "☐ at the start"),
		), []string{"- [ ] call [ ] Ann ([x]) ok.", "## Plan [ ]", "> [ ] at the start"}},
		{"numbered tasks (the issue's)", richtext.TextOf(
			para(item(richtext.Tasks, 0, look(listSpec{task: "number"})), "a"),
			para(richtext.ParagraphStyle{List: richtext.Tasks, Checked: true}, "b"),
			para(item(richtext.Tasks, 0, ""), "c ☐ d"),
		), []string{"1. [ ] a\n2. [x] b\n3. [ ] c [ ] d"}},
		{"emoji bullets", richtext.TextOf(
			para(richtext.ParagraphStyle{}, "Over it"),
			para(item(richtext.Bullets, 0, look(listSpec{mark: "⭐"})), "one"),
			para(item(richtext.Bullets, 0, ""), "two"),
		), []string{"<!--celestine:list mark=%E2%AD%90-->\n- one"}},
		{"icon bullets", richtext.TextOf(para(item(richtext.Bullets, 0, look(listSpec{mark: iconMarkPrefix + "arrow"})), "one")), []string{"<!--celestine:list mark=icon%3Aarrow-->"}},
		{"roman numbers", richtext.TextOf(
			para(item(richtext.Numbers, 0, look(listSpec{num: "roman"})), "one"),
			para(item(richtext.Numbers, 0, ""), "two"),
			para(richtext.ParagraphStyle{}, "between"),
			para(item(richtext.Numbers, 0, look(listSpec{num: "ROMAN"})), "three"),
		), []string{"<!--celestine:list num=roman-->\n1. one\n2. two", "<!--celestine:list num=ROMAN-->\n1. three"}},
		{"tasks with a bullet", richtext.TextOf(
			para(item(richtext.Tasks, 0, look(listSpec{mark: "🔥", task: "bullet"})), "hot"),
			para(item(richtext.Tasks, 0, ""), "cold"),
		), []string{"<!--celestine:list mark=%F0%9F%94%A5&task=bullet-->\n- [ ] hot"}},
		{"numbered tasks in Roman numerals, the first a star", richtext.TextOf(
			para(item(richtext.Tasks, 0, taskIDPrefix+"star"), "first"),
			para(item(richtext.Tasks, 0, look(listSpec{num: "roman", task: "number"})), "second"),
		), []string{"<!--celestine:list num=roman&task=number-->\n1. [*] first\n2. [ ] second"}},
		{"a nested list's look", richtext.TextOf(
			para(item(richtext.Bullets, 0, ""), "outer"),
			para(item(richtext.Bullets, 1, look(listSpec{mark: "👉"})), "inner"),
			para(item(richtext.Bullets, 1, ""), "inner 2"),
			para(item(richtext.Bullets, 0, ""), "outer 2"),
		), []string{"- outer\n    <!--celestine:list mark=%F0%9F%91%89-->\n    - inner"}},
		{"a quoted list's look", richtext.TextOf(
			para(richtext.ParagraphStyle{List: richtext.Tasks, Quote: true, ID: look(listSpec{task: "number"})}, "q"),
			para(richtext.ParagraphStyle{List: richtext.Numbers, Quote: true, Indent: 1, ID: look(listSpec{num: "roman"})}, "r"),
		), []string{"> 1. [ ] q\n>     <!--celestine:list num=roman-->\n>     1. r"}},
		{"a look this version does not know", richtext.TextOf(para(item(richtext.Bullets, 0, listIDPrefix+"mark=x&later=1"), "a")), []string{"<!--celestine:list mark=x&later=1-->"}},
	}
	for _, c := range cases {
		roundTrip(t, c.name, c.tx, c.want...)
	}
}

// A task in the running text where its box would be read as something else is written escaped, as before, and
// comes back as text: at the start of a list's item, glued to a word, in a link.
func TestInlineTaskAmbiguous(t *testing.T) {
	for _, c := range []struct {
		tx   richtext.Text
		md   string
		text string
	}{
		{richtext.TextOf(para(item(richtext.Bullets, 0, ""), "☐ milk")), `- \[ \] milk`, "[ ] milk"},
		{richtext.TextOf(para(item(richtext.Numbers, 0, ""), "☑ milk")), `1. \[x\] milk`, "[x] milk"},
		{richtext.TextOf(para(richtext.ParagraphStyle{}, "milk☐")), `milk\[ \]`, "milk[ ]"},
		{richtext.TextOf(para(richtext.ParagraphStyle{}, "a ☐☑ b")), `a \[ \]\[x\] b`, "a [ ][x] b"},
	} {
		md := bodyMarkdown(c.tx)
		if md != c.md {
			t.Errorf("%q, want %q", md, c.md)
		}
		back := importMarkdown(md)
		if s := back.String(); !strings.Contains(s, c.text) || strings.Contains(encodeBody(back), `"check"`) {
			t.Errorf("%q read back as %s", md, encodeBody(back))
		}
	}
}

// Markdown exported before #536 reads as it did: no box escaped, or not where a task in the running text can be,
// becomes one, and a comment is a list's look only where it is right over a list.
func TestListMarkdownOldFiles(t *testing.T) {
	for _, c := range []struct{ md, body string }{
		{"Buy \\[ \\] milk \\[x\\]", `[{"r":[{"t":"Buy [ ] milk [x]"}]}]`},
		{"- [ ] a\n- [x] b\n    - [ ] c", `[{"s":{"list":3},"r":[{"t":"a"}]},{"s":{"list":3,"checked":true},"r":[{"t":"b"}]},{"s":{"list":3,"indent":1},"r":[{"t":"c"}]}]`},
		{"1. a\n2. b", `[{"s":{"list":2},"r":[{"t":"a"}]},{"s":{"list":2},"r":[{"t":"b"}]}]`},
		{"- ⭐ a\n- b", `[{"s":{"list":1},"r":[{"t":"⭐ a"}]},{"s":{"list":1},"r":[{"t":"b"}]}]`},
		{"<!-- note -->\n- a", `[{"s":{"codeBlock":true,"id":"celestine:raw:html"},"r":[{"t":"\u003c!-- note --\u003e"}]},{"s":{"codeBlock":true},"r":[{"t":"- a"}]}]`},
		{"para\n\n<!-- note -->\n\n- a", `[{"r":[{"t":"para"}]},{"s":{"codeBlock":true,"id":"celestine:raw:html"},"r":[{"t":"\u003c!-- note --\u003e"}]},{"s":{"list":1},"r":[{"t":"a"}]}]`},
		{"> - [ ] q\n> - [x] r", `[{"s":{"list":3,"quote":true},"r":[{"t":"q"}]},{"s":{"list":3,"checked":true,"quote":true},"r":[{"t":"r"}]}]`},
		{"x `[x]` [x](u) [[p]] arr[x] [see [x] here](u)", `[{"r":[{"t":"x "},{"t":"[x]","c":true},{"t":" "},{"t":"x","l":"u"},{"t":" "},{"t":"￼","o":{"k":"page","target":"p"}},{"t":" arr[x] "},{"t":"see [x] here","l":"u"}]}]`},
		{"    [ ] code\n", `[{"s":{"codeBlock":true},"r":[{"t":"[ ] code"}]}]`},
		{"- [*] star\n- [ ] plain", `[{"s":{"list":3,"id":"celestine:task:star"},"r":[{"t":"star"}]},{"s":{"list":3},"r":[{"t":"plain"}]}]`},
		{"```\na [ ] b\n```", `[{"s":{"codeBlock":true,"id":"celestine:code:"},"r":[{"t":"a [ ] b"}]}]`},
		{"[x]: https://example.com\n\nsee [x] here", `[{"r":[{"t":"see "},{"t":"x","l":"https://example.com"},{"t":" here"}]}]`},
		{"[[a [x] b]] and [link [ ] text](u)", `[{"r":[{"t":"[[a [x] b]] and "},{"t":"link [ ] text","l":"u"}]}]`},
		{"<!--celestine:list mark=x-->\n\nnot a list", `[{"s":{"codeBlock":true,"id":"celestine:raw:html"},"r":[{"t":"\u003c!--celestine:list mark=x--\u003e"}]},{"r":[{"t":"not a list"}]}]`},
	} {
		if got := encodeBody(importMarkdown(c.md)); got != c.body {
			t.Errorf("%q reads as\n%s\nnot\n%s", c.md, got, c.body)
		}
	}
	// a text with the characters of the tokens in it already (an icon font's letters) is read and written as before
	for _, md := range []string{"icon  and [ ] box", "- -1 a\n1. [ ] b"} {
		if got := encodeBody(importMarkdown(md)); strings.Contains(got, `"check"`) || strings.Contains(got, listIDPrefix) {
			t.Errorf("%q reads as %s", md, got)
		}
	}
	if md := bodyMarkdown(richtext.TextOf(para(richtext.ParagraphStyle{}, "icon  ☐ x"))); md != "icon  \\[ \\] x" {
		t.Errorf("written as %q", md)
	}
	// what was read as two tasks, before, still is: now as numbered ones
	if got := encodeBody(importMarkdown("1. [ ] a\n2. [x] b")); got != `[{"s":{"list":3,"id":"celestine:list:task=number"},"r":[{"t":"a"}]},{"s":{"list":3,"checked":true},"r":[{"t":"b"}]}]` {
		t.Errorf("numbered tasks: %s", got)
	}
	// and a box in the running text, not escaped, as another app writes it, is a task
	if got := encodeBody(importMarkdown("text [x] and [ ] here")); got != `[{"r":[{"t":"text "},{"t":"￼","o":{"k":"check","target":"x"}},{"t":" and "},{"t":"￼","o":{"k":"check"}},{"t":" here"}]}]` {
		t.Errorf("inline tasks: %s", got)
	}
}

// The lines' tasks are numbered tasks too, and not the tasks in the running text (those are inlineTasksOf's, #536),
// and a task the view ticks is the one it shows (taskParagraph counts the page's tasks as tasksOf does). A line's
// words leave out the boxes of the tasks in it.
func TestListMarkdownTasks(t *testing.T) {
	tx := richtext.TextOf(
		para(item(richtext.Tasks, 0, look0(listSpec{num: "roman", task: "number"})), "first ☐"),
		para(item(richtext.Tasks, 0, ""), "second"),
		para(richtext.ParagraphStyle{}, "Buy ☐ milk"),
		para(item(richtext.Tasks, 0, ""), "third"),
	)
	md := bodyMarkdown(tx)
	items := tasksOf(nil, md)
	if len(items) != 3 {
		t.Fatalf("%d tasks in\n%s", len(items), md)
	}
	for k, it := range items {
		s, ok := taskParagraph(tx, md, nil, it)
		if !ok {
			t.Fatalf("task %d (%q) not found", k, it.text)
		}
		_, e := tx.ParagraphRange(s)
		if got := strings.TrimSpace(strings.ReplaceAll(tx.Slice(s, e).String(), string(richtext.ObjectReplacement), "")); got != it.text {
			t.Errorf("task %d: the view shows %q, a tick lands on %q", k, it.text, got)
		}
	}
	if text, ok := withTaskBox(md, items[1].line, "x"); !ok || !strings.Contains(text, "2. [x] second") {
		t.Errorf("the view's tick: %v\n%s", ok, text)
	}
}

func look0(s listSpec) string { return s.id() }

// A list's look is no words of the page: the search and the count of words leave its comment out.
func TestListMarkdownPlain(t *testing.T) {
	md := "<!--celestine:list mark=%E2%AD%90-->\n- one\n> <!--celestine:list num=roman-->\n> 1. two\n"
	if got := plainText(md); strings.Contains(got, "celestine") || !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Errorf("plain: %q", got)
	}
}

// A task that comes again, the first of a list with a look, comes again and the list keeps its look.
func TestListMarkdownRecurring(t *testing.T) {
	id := listSpec{num: "roman", task: "number"}.id()
	tx := richtext.TextOf(
		para(richtext.ParagraphStyle{List: richtext.Tasks, Checked: true, ID: id}, "water every:week"),
		para(item(richtext.Tasks, 0, ""), "other"),
	)
	doc := richtext.NewDocument(tx)
	rollRecurringIn(doc, time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC), importMarkdown)
	out := doc.Text()
	if out.ParagraphCount() != 3 {
		t.Fatalf("it did not come again: %s", encodeBody(out))
	}
	if it := listsOf(out).all[0]; it == nil || it.spec != (listSpec{num: "roman", task: "number"}) {
		t.Errorf("the list lost its look: %s", encodeBody(out))
	}
	if md := bodyMarkdown(out); !strings.Contains(md, "2. [ ] water [[2026-11-22]] every:week") {
		t.Errorf("the next one:\n%s", md)
	}
}

// An agent that reads a page and writes it back whole keeps its lists' looks and its tasks in the running text
// (mcp.go's writeBody).
func TestMCPListLooksKept(t *testing.T) {
	a, tt := newTestApp(t, "x")
	cs := agent(t, a)
	tx := richtext.TextOf(
		para(richtext.ParagraphStyle{}, "Buy ☐ milk ☑"),
		para(item(richtext.Bullets, 0, listSpec{mark: "⭐"}.id()), "star"),
		para(item(richtext.Tasks, 0, listSpec{task: "number"}.id()), "one"),
		para(item(richtext.Tasks, 0, ""), "two"),
	)
	if err := a.writeBody(a.page, bodyMarkdown(tx)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	before, _ := a.page.Body()
	if before != encodeBody(tx) {
		t.Fatalf("written as\n%s\nnot\n%s", before, encodeBody(tx))
	}
	id := a.idOf(a.page.Dir)
	var read struct{ Markdown string }
	call(t, cs, "read_page", map[string]any{"page": id}, &read)
	call(t, cs, "update_page", map[string]any{"page": id, "markdown": read.Markdown}, nil)
	tt.Frame()
	if after, _ := a.page.Body(); after != before {
		t.Fatalf("the agent's rewrite changed it:\n%s\n%s\nthrough\n%s", before, after, read.Markdown)
	}
}

// A numbered task, one in a list with a look's comment over it too, is ticked from the task view and by "Complete
// all": the task view's tasks and the page's task paragraphs line up (taskSlots), the comment no task of either.
func TestNumberedTasksTicked(t *testing.T) {
	src := "<!--celestine:list num=roman&task=number-->\n1. [ ] one\n2. [ ] two\n\nbetween\n\n1. [ ] three\n2. [ ] four\n"
	a, _ := newTestApp(t, src)
	body, _ := a.page.Body()
	if !strings.Contains(body, "num=roman") {
		t.Fatalf("the look was not read: %s", body)
	}
	a.toggleTask(taskIn(t, a, "two"))
	a.toggling.Wait()
	a.toggleTask(taskIn(t, a, "three"))
	a.toggling.Wait()
	md, _ := a.page.Text()
	if !strings.Contains(md, "<!--celestine:list num=roman&task=number-->\n1. [ ] one\n2. [x] two") || !strings.Contains(md, "1. [x] three\n2. [ ] four") {
		t.Fatalf("ticked as\n%s", md)
	}
	if !taskIn(t, a, "two").done || !taskIn(t, a, "three").done || taskIn(t, a, "one").done || taskIn(t, a, "four").done {
		t.Fatal("the index and the file disagree")
	}
	var ts []taskItem
	for _, k := range a.allTasks(true) {
		if k.page == a.page && !k.done {
			ts = append(ts, k)
		}
	}
	if len(ts) != 2 {
		t.Fatalf("%d open tasks", len(ts))
	}
	a.closeTasks(a.page, ts)
	a.toggling.Wait()
	md, _ = a.page.Text()
	if strings.Contains(md, "[ ]") || strings.Count(md, "[x]") != 4 || !strings.Contains(md, "<!--celestine:list num=roman&task=number-->\n1. [x] one") {
		t.Fatalf("closed as\n%s", md)
	}
}

// The tasks in the running text the task view reads from a page's Markdown are the chips the page's text has when
// the Markdown is read, in the same order and the same state: the view's k-th is the text's k-th (#536).
func TestInlineTaskSpots(t *testing.T) {
	pages := []string{
		"Buy [ ] milk and [x] bread.\n",
		"- [ ] a line's task with [ ] one in it\n- [x] done\n",
		"```\ncode [ ] not a task\n```\n\ntext [ ] after\n",
		"| a | b |\n| --- | --- |\n| [ ] cell | x |\n\nafter the table [x]\n",
		"> [!note] a callout [ ] held\n> still [ ] held\n\nout [ ] of it\n",
		"<!--celestine:list mark=%E2%AD%90-->\n- item [ ] inside\n",
		"[x]: https://example.com\n\nsee [x] a link, and [ ] a task\n",
		"escaped \\[ \\] and `code [ ]` and [a [ ] b](https://x.y) and [[Page]] [ ]\n",
		"# Heading [ ]\n\n---\n\nunder the rule [x]\n",
		"a footnote[^1] [ ] after\n\n[^1]: its note [ ]\n",
		"word[ ]glued and (x [ ]) and *[x]*\n",
	}
	for _, md := range pages {
		spots := inlineTaskSpots(md)
		_, done := checkChips(parseBody(md))
		if len(spots) != len(done) {
			t.Errorf("%q: %d spots, %d chips", md, len(spots), len(done))
			continue
		}
		lines := strings.Split(md, "\n")
		for k, s := range spots {
			if s.done != done[k] {
				t.Errorf("%q: task %d done %v, its chip %v", md, k, s.done, done[k])
			}
			if s.col >= 0 && lines[s.line][s.col] != '[' {
				t.Errorf("%q: task %d at %d:%d is no box", md, k, s.line, s.col)
			}
		}
	}
}
