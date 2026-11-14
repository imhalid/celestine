package main

import (
	"github.com/imhalid/celestine/internal/library"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The task view: every "- [ ] …" line of the library in one place, by the
// page it is in, to be ticked off without going there. The tasks are read
// with the backlinks' index (backlinks.go), in the same pass; ticking one
// writes the page's file (and the page's text, if it is open) at once.

var taskLine = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX*!?/e])\]\s*(.*?)\s*$`)

// taskBox is the box of a task's line, and what is before it: a kind of task's too (taskkinds.go).
var taskBox = regexp.MustCompile(`^(\s*[-*+]\s+)\[[ xX*!?/e]\]`)

// taskItem is a task: its page, which line of the page's Markdown, whether
// it is done, and what it says.
type taskItem struct {
	page  *Page
	line  int
	done  bool
	text  string
	due   time.Time // a day named in it, as [[2026-11-10]]; zero when none
	every string    // how it comes again, as "her hafta"; "" when it does not
	kind  string    // its kind's id (taskkinds.go), as its box says: "" a plain one, or a done one
}

// tasksOf are the tasks in a page's Markdown, not those in code.
func tasksOf(p *Page, body string) []taskItem {
	if !strings.Contains(body, "[") {
		return nil
	}
	var out []taskItem
	inFence := false
	for i, ln := range strings.Split(body, "\n") {
		if fenceLine.MatchString(ln) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := taskLine.FindStringSubmatch(ln); m != nil && m[3] != "" {
			var due time.Time
			text := wikiInText.ReplaceAllStringFunc(m[3], func(s string) string {
				inside := s[2 : len(s)-2]
				target, alias, _ := strings.Cut(inside, "|")
				if d, err := parseDay(strings.TrimSpace(target)); err == nil && alias == "" && due.IsZero() {
					due = d
					return ""
				}
				if alias != "" {
					return alias
				}
				return target
			})
			every := ""
			if r, ok := parseEvery(text); ok {
				every = r.label()
				text = everyTok.ReplaceAllString(text, "")
			}
			if text = strings.TrimSpace(text); text == "" {
				text = strings.TrimSpace(m[3])
			}
			kind := ""
			if k, ok := taskKindByMD(m[2]); ok {
				kind = k.id
			}
			out = append(out, taskItem{p, i, m[2] == "x" || m[2] == "X", text, due, every, kind})
		}
	}
	return out
}

// allTasks is every task, each page's in the order of its text, the pages in the order they had when the
// task view was opened: those changed last first, and a page that changes while the view is open (a task
// ticked off in it) stays where it was; one new to the view comes first. It is sorted when the index of links
// is made, not for each frame: 20000 tasks.
func (a *app) allTasks(force bool) []taskItem {
	a.linksTo(nil, a.clock(), force)
	l := &a.links
	if l.sorted == nil && len(l.tasks) > 0 {
		out := append([]taskItem{}, l.tasks...)
		rank := func(p *Page) int {
			if r, ok := l.taskRank[p]; ok {
				return r
			}
			return -1
		}
		sort.SliceStable(out, func(i, j int) bool {
			pi, pj := out[i].page, out[j].page
			if pi != pj {
				if ri, rj := rank(pi), rank(pj); ri != rj {
					return ri < rj
				}
				return pi.Modified.After(pj.Modified)
			}
			return out[i].line < out[j].line
		})
		l.taskRank = map[*Page]int{}
		for _, k := range out {
			if _, ok := l.taskRank[k.page]; !ok {
				l.taskRank[k.page] = len(l.taskRank)
			}
		}
		l.sorted = out
	}
	return l.sorted
}

// openTask opens the page of task k with the caret in the task's line.
func (a *app) openTask(k taskItem) {
	a.open(k.page)
	body, _ := k.page.Text()
	idx := 0
	for _, t := range tasksOf(k.page, body) {
		if t.line == k.line {
			break
		}
		idx++
	}
	text := a.doc.Text()
	n := 0
	for at, para := range text.Paragraphs() {
		if para.Style.List != richtext.Tasks || strings.TrimSpace(strings.ReplaceAll(paraText(para), string(richtext.ObjectReplacement), "x")) == "" {
			continue
		}
		if n == idx {
			a.gotoPos(at)
			return
		}
		n++
	}
}

// paraText is the text of a paragraph.
func paraText(p richtext.Paragraph) string {
	var sb strings.Builder
	for _, r := range p.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// openTasks shows the task view.
func (a *app) openTasks() {
	a.flush()
	a.tasksOpen, a.tag, a.coll, a.trashOpen, a.searchOpen, a.settingsOpen = true, "", nil, false, false, false
	a.statsScreen, a.profileScreen = false, false
	a.links.taskRank, a.links.sorted = nil, nil // the pages as they are now, the changed last first
	a.linksStale()
	a.visit()
}

// taskToggle is a task ticked off, or on again, as its page's new body: made away from the window (prepareToggle),
// written under its lock (applyToggle).
type taskToggle struct {
	t     taskItem
	err   error
	stale bool   // the page changed since the task was read: nothing to write
	saved string // the page's new body
	// what the indexes keep of the page, when its Markdown is the old one with the task's box changed: the index
	// is not read again from the whole page, which took most of a second on one of thirty thousand lines (#504)
	text   string
	idx    *pageIdx
	plain  string
	folded string
}

// prepareToggle reads task t's page and makes its body with the task ticked off, or on again. It reads nothing of
// the app's but the page's file, and may run away from the window. The body is changed where the task is, not
// written again from its Markdown, which loses what Markdown cannot say (#123).
func prepareToggle(t taskItem, now time.Time) taskToggle {
	j := taskToggle{t: t}
	p := t.page
	body, err := p.Body()
	if err == nil {
		err = bodyReadable(body) // what this version does not read would be dropped by writing it again
	}
	if err != nil {
		j.err = err
		return j
	}
	tx := decodeBody(body)
	md := library.TextOfBody(body) // the page's Markdown, kept since its index read it
	at, ok := taskParagraph(tx, md, p, t)
	if !ok {
		j.stale = true
		return j
	}
	doc := richtext.NewDocument(tx)
	doc.ToggleChecked(at)
	rollRecurringIn(doc, now, importMarkdown) // a task that comes again: this one is done, and the next is made
	j.saved = encodeBody(doc.Text())
	if doc.Text().ParagraphCount() == tx.ParagraphCount() { // nothing came again: the page's Markdown is the task's box changed
		st := doc.Text().ParagraphStyleAt(at)
		box := " "
		if st.Checked {
			box = "x"
		} else if k, ok := taskKindOf(st); ok {
			box = k.md
		}
		if text, ok := withTaskBox(md, t.line, box); ok {
			j.text = text
			j.idx, j.plain = readPageIdxAt(p, time.Time{}, 0, text)
			j.folded = fold(j.plain)
		}
	}
	return j
}

// applyToggle writes a task's toggle made by prepareToggle, under the window's lock: the page's file, its text in
// the editor where it is open, and the indexes.
func (a *app) applyToggle(j taskToggle) {
	p := j.t.page
	switch {
	case j.err != nil:
		a.fail(j.err)
		a.linksStale()
		return
	case j.stale:
		a.linksStale() // the page changed since: read it again
		return
	}
	if err := p.Save(j.saved); err != nil {
		a.fail(err)
		a.linksStale()
		return
	}
	if j.text != "" {
		library.RememberText(j.saved, j.text)
		l := &a.links
		j.idx.mod, j.idx.writes = p.Modified, p.Writes
		if l.pages == nil {
			l.pages = map[*Page]*pageIdx{}
		}
		l.pages[p] = j.idx
		if l.plain == nil {
			l.plain = map[*Page]*plainIdx{}
		}
		l.plain[p] = &plainIdx{mod: p.Modified, writes: p.Writes, plain: j.plain, folded: j.folded}
	}
	a.takeOutside(p) // its text, in the editor, as the file has it now
	a.linksStale()
}

// prepareClose reads page p and makes its body with the open tasks of ts ticked off: one write for them all.
func prepareClose(p *Page, ts []taskItem, now time.Time) taskToggle {
	j := taskToggle{t: taskItem{page: p}}
	body, err := p.Body()
	if err == nil {
		err = bodyReadable(body)
	}
	if err != nil {
		j.err = err
		return j
	}
	tx := decodeBody(body)
	md := library.TextOfBody(body)
	items, starts := taskParagraphs(tx, md, p)
	byLine := map[int]int{}
	for i, ti := range items {
		byLine[ti.line] = i
	}
	var at []int
	for _, t := range ts {
		if t.done || t.page != p {
			continue
		}
		i, ok := byLine[t.line]
		if !ok || i >= len(starts) || items[i].text != t.text || items[i].done {
			j.stale = true
			return j
		}
		at = append(at, starts[i])
	}
	doc := richtext.NewDocument(tx)
	doc.Edit(func(ed *richtext.Edit) {
		for _, s := range at {
			ed.FormatParagraphs(s, s, func(st *richtext.ParagraphStyle) { st.Checked = true })
		}
	})
	rollRecurringIn(doc, now, importMarkdown)
	j.saved = encodeBody(doc.Text())
	j.text = library.TextOfBody(j.saved) // written as Markdown here, away from the window, and kept
	j.idx, j.plain = readPageIdxAt(p, time.Time{}, 0, j.text)
	j.folded = fold(j.plain)
	return j
}

// closeTasks ticks off the open tasks of ts, page p's: the list shows them done at once, the page is written away
// from the window, after any toggle before (#505).
func (a *app) closeTasks(p *Page, ts []taskItem) {
	if p == a.page {
		a.flush()
	}
	ts = append([]taskItem(nil), ts...)
	open := map[int]bool{}
	for _, t := range ts {
		if !t.done {
			open[t.line] = true
		}
	}
	for _, list := range [][]taskItem{a.links.tasks, a.links.sorted} {
		for i := range list {
			if list[i].page == p && open[list[i].line] {
				list[i].done = true
			}
		}
	}
	if a.taskFilter == 0 && !stillNow() { // in the open ones they go out, as one ticked off does
		if a.taskLeaving == nil {
			a.taskLeaving = map[string]time.Time{}
		}
		for _, t := range ts {
			if open[t.line] {
				a.taskLeaving[taskKey(t)] = a.frameNow
			}
		}
	}
	now := a.clock()
	a.toggling.Add(1)
	go func() {
		defer a.toggling.Done()
		defer a.guard()
		a.toggleMu.Lock()
		defer a.toggleMu.Unlock()
		j := prepareClose(p, ts, now)
		a.do(func() error {
			a.applyToggle(j)
			return nil
		})
	}()
}

// toggleTask ticks a task off, or on again, at once: what an agent asks for, which reads the page after.
func (a *app) toggleTask(t taskItem) {
	if t.page == a.page {
		a.flush()
	}
	a.applyToggle(prepareToggle(t, a.clock()))
}

// toggleTaskSoon is toggleTask from the task view: the list shows the task ticked at once, and the page is read
// and written away from the window, one toggle after another, so that a page of thousands of tasks does not hold
// the window (#504).
func (a *app) toggleTaskSoon(t taskItem) {
	if t.page == a.page {
		a.flush()
	}
	for _, list := range [][]taskItem{a.links.tasks, a.links.sorted} { // as it will be
		for i := range list {
			if list[i].page == t.page && list[i].line == t.line {
				list[i].done = !t.done
			}
		}
	}
	now := a.clock()
	a.toggling.Add(1)
	go func() {
		defer a.toggling.Done()
		defer a.guard()
		a.toggleMu.Lock() // one after another: each reads the page the one before wrote
		defer a.toggleMu.Unlock()
		j := prepareToggle(t, now)
		a.do(func() error {
			a.applyToggle(j)
			return nil
		})
	}()
}

// taskParagraph is where in tx, page p's text, the task t starts: the task paragraph that is t's among the tasks,
// counted as tasksOf counts them in the page's Markdown. False when the page has changed since t was read: its
// line is not t's task any more. The text is written as Markdown once, not a paragraph at a time, which took
// seconds on a page of thousands of tasks (#504).
func taskParagraph(tx richtext.Text, md string, p *Page, t taskItem) (int, bool) {
	k := -1
	for i, ti := range tasksOf(p, md) {
		if ti.line == t.line {
			if ti.text != t.text || ti.done != t.done {
				return 0, false
			}
			k = i
		}
	}
	if k < 0 {
		return 0, false
	}
	for s, para := range tx.Paragraphs() { // the tasks with words, as a task's line in Markdown has them (openTask)
		if para.Style.List != richtext.Tasks || strings.TrimSpace(strings.ReplaceAll(paraText(para), string(richtext.ObjectReplacement), "x")) == "" {
			continue
		}
		if k--; k < 0 {
			return s, true
		}
	}
	return 0, false
}

// taskParagraphs are the tasks of page p's Markdown md, as tasksOf reads them, and where each starts in tx, its
// text: both in one pass, for many tasks at once.
func taskParagraphs(tx richtext.Text, md string, p *Page) ([]taskItem, []int) {
	items := tasksOf(p, md)
	starts := make([]int, 0, len(items))
	for s, para := range tx.Paragraphs() {
		if para.Style.List != richtext.Tasks || strings.TrimSpace(strings.ReplaceAll(paraText(para), string(richtext.ObjectReplacement), "x")) == "" {
			continue
		}
		starts = append(starts, s)
	}
	return items, starts
}

// withTaskBox is the Markdown md with the box of the task on line i holding box; false when that line is no task.
func withTaskBox(md string, i int, box string) (string, bool) {
	start := 0
	for n := 0; n < i; n++ {
		j := strings.IndexByte(md[start:], '\n')
		if j < 0 {
			return "", false
		}
		start += j + 1
	}
	end := len(md)
	if j := strings.IndexByte(md[start:], '\n'); j >= 0 {
		end = start + j
	}
	m := taskLine.FindStringSubmatchIndex(md[start:end])
	if m == nil {
		return "", false
	}
	return md[:start+m[4]] + box + md[start+m[5]:], true
}

// tasksView is the view: a filter, and the tasks under their pages.
func (a *app) tasksView(c *ui.Context) {
	cl := ryugo.From(c).Colors
	all := a.allTasks(a.links.stale) // a tick shows at once
	now := a.clock()
	var st taskStats
	for _, k := range all {
		st.count(k, now)
	}
	// the collections that have tasks, and the kinds the tasks are, to choose among
	colls := map[*Collection]int{}
	kinds := map[string]int{}
	for _, k := range all {
		colls[k.page.Coll]++
		if !k.done {
			kinds[taskKindKey(k)]++
		}
	}
	if a.taskKindF != "" && kinds[a.taskKindF] == 0 {
		a.taskKindF = "" // no open task of that kind now
	}
	if a.taskColl != "" && !slices.ContainsFunc(a.lib.Collections, func(co *Collection) bool { return co.Dir == a.taskColl && colls[co] > 0 }) {
		a.taskColl = "" // that collection has no tasks now
	}
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Center).Wrap().Padding(2, 0, 4, 0).Children(func() {
			ui.Text(c, locales.CommonTasks()).FontSize(32).FontWeight(720).Grow(1)
			a.taskStatBars(c, cl, st)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			ryugo.Segmented(c.Key("task-filter"), &a.taskFilter, []string{locales.TasksOpen(), locales.TasksDone(), locales.TasksAll()}, ryugo.SegmentedOptions{Label: locales.CommonTasks(), Size: ryugo.Sm})
			if a.taskFilter == 0 { // the open ones by their day
				ryugo.Segmented(c.Key("task-due"), &a.taskDue, []string{locales.TasksDueAll(), locales.TasksDueToday(), locales.TasksDueWeek(), locales.TasksDueLate()}, ryugo.SegmentedOptions{Label: locales.TasksDay(), Size: ryugo.Sm})
			}
			ui.Spacer(c)
			if len(kinds) > 1 || a.taskKindF != "" {
				opts := []ryugo.Option[string]{{Value: "", Label: locales.TasksAllKinds()}, {Value: "plain", Label: locales.TasksPlainTask() + "  ·  " + strconv.Itoa(kinds["plain"])}}
				for _, tk := range taskKinds {
					if kinds[tk.id] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: tk.id, Label: msg(tk.name) + "  ·  " + strconv.Itoa(kinds[tk.id])})
					}
				}
				ui.Box(c.Key("task-kind-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-kind"), &a.taskKindF, opts, ryugo.SelectOptions{Label: locales.CommonKind(), Size: ryugo.Sm})
				})
			}
			if len(colls) > 1 {
				opts := []ryugo.Option[string]{{Value: "", Label: locales.TasksAllCollections()}}
				for _, co := range a.lib.Collections {
					if colls[co] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: co.Dir, Label: co.Name + "  ·  " + strconv.Itoa(colls[co])})
					}
				}
				ui.Box(c.Key("task-coll-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-coll"), &a.taskColl, opts, ryugo.SelectOptions{Label: locales.Collection(), Size: ryugo.Sm})
				})
			}
			a.fieldMenu(ryugo.Input(c.Key("task-q"), &a.taskQuery, ryugo.InputOptions{Label: locales.TasksSearchTasks(), Placeholder: locales.TasksSearchTasks2(), PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}), &a.taskQuery, fieldOpts{oneLine: true}).Width(176)
		})
		q := fold(strings.TrimSpace(a.taskQuery))
		var shown []taskItem
		for _, k := range all {
			_, leaving := a.taskLeaving[taskKey(k)]
			switch {
			case leaving && a.taskFilter == 0 && k.done: // just done: it stays while it goes out (#494)
				shown = append(shown, k)
			case a.taskFilter != 2 && (a.taskFilter == 0) == k.done:
			case a.taskFilter == 0 && !dueMatches(k, a.taskDue, now):
			case a.taskColl != "" && k.page.Coll.Dir != a.taskColl:
			case a.taskKindF != "" && (k.done || taskKindKey(k) != a.taskKindF):
			case q != "" && !strings.Contains(fold(k.text), q) && !strings.Contains(fold(k.page.Title), q):
			default:
				shown = append(shown, k)
			}
		}
		if len(shown) == 0 {
			a.noTasks(c, len(all) == 0)
			return
		}
		// 20000 tasks are 20000 rows: only those in view are built (ui.List), the pages' heads and
		// their tasks in one list
		rows := taskRows(shown, a.taskFolded)
		a.taskList.Key = func(i int) any {
			if i == len(rows) {
				return "end"
			}
			return rows[i].key()
		}
		a.scrolledList(c, "tasks-list", nil, ui.Box(c.Key("tasks-wrap")).FillWidth().Grow(1).MinHeight(0), func() ui.Element {
			return ui.List(c.Key("tasks-list"), a.taskList, len(rows)+1, func(i int) {
				switch {
				case i == len(rows): // the room under the last task
					ui.Box(c.Key("tasks-end")).FillWidth().Height(taskEndRoom)
				case rows[i].head:
					a.taskHead(c, cl, rows[i], i > 0)
				default:
					a.taskRow(c, cl, rows[i].task)
				}
			}).Fill()
		})
	})
}

// taskEndRoom is the room under the last task, so that it does not end at the window's edge.
const taskEndRoom = 56

// taskStats are how many tasks there are, open and done, and of the open ones how many are of today and late.
type taskStats struct{ open, done, today, late int }

// count counts task k in, on the day of now.
func (s *taskStats) count(k taskItem, now time.Time) {
	if k.done {
		s.done++
		return
	}
	s.open++
	if !k.due.IsZero() {
		today := startOfDay(now)
		switch {
		case sameDay(k.due, today):
			s.today++
		case k.due.Before(today):
			s.late++
		}
	}
}

// taskStatBars are the small stats beside the title: the open and the done tasks, of all of them, and the
// open ones of today and late, of the open ones, each a card of ticks in a bright color of its own, the same in
// both themes. A click on one shows those.
func (a *app) taskStatBars(c *ui.Context, cl ryugo.Colors, s taskStats) {
	all, open := max(1, s.open+s.done), max(1, s.open)
	stats := []struct {
		key, label string
		n, of      int
		col        ui.Color
		filter     int
		due        int
	}{
		{"open", locales.TasksOpen(), s.open, all, ui.Hex("#3b82f6"), 0, 0},
		{"done", locales.TasksDone(), s.done, all, ui.Hex("#84cc16"), 1, 0},
		{"today", locales.TasksDueToday(), s.today, open, ui.Hex("#f59e0b"), 0, 1},
		{"late", locales.TasksDueLate(), s.late, open, ui.Hex("#f43f5e"), 0, 3},
	}
	ui.Row(c.Key("task-stats")).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		for _, st := range stats {
			on := a.taskFilter == st.filter && (st.filter != 0 || a.taskDue == st.due)
			a.taskStatCard(c, cl, st.key, st.label, st.n, st.of, st.col, on, func() { a.taskFilter, a.taskDue = st.filter, st.due })
		}
	})
}

// taskStatTicks is how many ticks a stat's bar has.
const taskStatTicks = 18

// taskStatCard is one stat: a small card, its name and n of of on it, over an inset well of ticks, as many in its
// color as n is of of. The card is tinted in the color, and a border in it says the list shows those; a click
// shows them.
func (a *app) taskStatCard(c *ui.Context, cl ryugo.Colors, key, label string, n, of int, col ui.Color, on bool, show func()) {
	card := ui.Column(c.Key("task-stat:"+key)).Width(138).Padding(5, 5, 5, 5).Gap(4).Radius(rad(11)).
		Background(mix(cl.Card, col, 0.07)).Border(1, col.Alpha(0.16)).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.05)).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label + " " + strconv.Itoa(n) + "/" + strconv.Itoa(of))
	switch {
	case on: // the border keeps its width, so that the ticks inside stay where they are
		card.Border(1, col.Alpha(0.85))
	case card.Hovered():
		card.Border(1, col.Alpha(0.4))
	}
	lit := 0
	if n > 0 {
		lit = max(1, int(float64(n)/float64(max(1, of))*taskStatTicks+0.5))
	}
	card.Children(func() {
		ui.Row(c).FillWidth().Padding(1, 5, 0, 5).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(12).FontWeight(560).TextColor(cl.Text2).MaxLines(1).Grow(1).MinWidth(0)
			ui.Row(c).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, strconv.Itoa(n)).FontSize(13).FontWeight(700).TextColor(cl.Text).FontFeatures("tnum")
				ui.Text(c, "/"+strconv.Itoa(of)).FontSize(11).TextColor(cl.Muted).FontFeatures("tnum")
			})
		})
		well := ui.Box(c.Key("ticks")).FillWidth().Height(20).Radius(rad(6)).Background(mix(cl.Well, col, 0.06))
		well.DrawOver(func(p *ui.Painter, r ui.Rect) {
			// on whole pixels, the run of ticks centered: a tick between two pixels looks wider than the others
			const padX, padY = 7, 5
			pitch := float32(int((r.W - 2*padX) / taskStatTicks))
			w := max(2, float32(int(pitch*0.46+0.5)))
			x0 := float32(int(r.X + (r.W-pitch*taskStatTicks)/2 + float32(int((pitch-w)/2)) + 0.5))
			y, h := float32(int(r.Y+padY+0.5)), float32(int(r.H-2*padY+0.5))
			for i := range taskStatTicks {
				tc := cl.Muted.Alpha(0.2)
				if i < lit {
					tc = mix(col, ui.RGBA(255, 255, 255, 1), 0.12*float64(i)/taskStatTicks)
				}
				p.Fill(ui.Rect{X: x0 + float32(i)*pitch, Y: y, W: w, H: h}, tc, w/2)
			}
		})
	})
	if card.Clicked() {
		show()
	}
}

// noTasks says there are no tasks to show: none at all, or none with the filters.
func (a *app) noTasks(c *ui.Context, none bool) {
	o := ryugo.EmptyStateOptions{Icon: iconTask, Title: locales.TasksNoTaskFitsThis(), Description: locales.TasksLoosenFilterTheyShow()}
	switch {
	case none:
		o.Title, o.Description = locales.TasksEmptyTitle(), locales.TasksEmptyHelp()
	case a.taskFilter == 1:
		o.Title, o.Description = locales.TasksNoneDone(), locales.TasksNoneDoneHelp()
	case a.taskFilter == 0 && a.taskDue == 0 && a.taskColl == "" && a.taskKindF == "" && strings.TrimSpace(a.taskQuery) == "":
		o.Title, o.Description = locales.TasksNoneOpen(), locales.TasksAllDone()
	}
	if !none && (a.taskColl != "" || strings.TrimSpace(a.taskQuery) != "" || a.taskDue != 0 || a.taskKindF != "") {
		o.Action = func() {
			if ryugo.Button(c.Key("task-clear-filters"), locales.TasksClearFilters(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				a.taskColl, a.taskQuery, a.taskDue, a.taskKindF = "", "", 0, ""
			}
		}
	}
	ui.Column(c).FillWidth().Children(func() { ryugo.EmptyState(c, o) })
}

// taskGroup is a page and its tasks.
// taskListRow is a row of the list of tasks: the head of a page's tasks, or one task.
type taskListRow struct {
	head  bool
	task  taskItem
	n     int        // of a head: how many tasks its page has here
	tasks []taskItem // and they
}

// key is what the row is known by, as the list keeps its place by it.
func (r taskListRow) key() string {
	if r.head {
		return "h:" + r.task.page.Dir
	}
	return "t:" + r.task.page.Dir + ":" + strconv.Itoa(r.task.line)
}

// taskRows are the tasks as the rows of a list: the head of each page, and its tasks under it.
func taskRows(shown []taskItem, folded map[*Page]bool) []taskListRow {
	rows := make([]taskListRow, 0, len(shown)+len(shown)/4)
	for i := 0; i < len(shown); {
		j := i
		for j < len(shown) && shown[j].page == shown[i].page {
			j++
		}
		rows = append(rows, taskListRow{head: true, task: shown[i], n: j - i, tasks: shown[i:j]})
		if !folded[shown[i].page] {
			for _, k := range shown[i:j] {
				rows = append(rows, taskListRow{task: k})
			}
		}
		i = j
	}
	return rows
}

// taskHead is the head of a page's tasks: a chevron that folds them away, its title
// and collection, which open it, and how many tasks it has.
func (a *app) taskHead(c *ui.Context, cl ryugo.Colors, r taskListRow, gap bool) {
	p := r.task.page
	folded := a.taskFolded[p]
	c.Key("tgh:" + p.Dir)
	head := ui.Row(c).FillWidth().Padding(2, 8, 2, 2).Gap(4).Radius(rad(8)).AlignItems(ui.Center).
		Label(locales.CommonPageLabel(p.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	if gap {
		head.Margin(14, 0, 0, 0) // the room between two pages' tasks
	}
	if head.Hovered() {
		head.Background(cl.Fill)
	}
	a.contextOn(head, func() []menuItem { return a.pageMenu(p, false) })
	var fold ui.Element
	head.Children(func() {
		fold = ui.Box(c.Key("tgh-fold")).Size(26, 26).Shrink(0).Radius(rad(7)).Center().Cursor(ui.CursorPointer).
			Label(map[bool]string{true: locales.TasksUnfoldLabel(p.Title), false: locales.TasksFoldLabel(p.Title)}[folded]).Role(ui.RoleButton)
		if fold.Hovered() {
			fold.Background(cl.Fill)
		}
		angle := fold.AnimateTo("turn", map[bool]float32{true: 0, false: 90}[folded], springTurn)
		fold.Children(func() { ui.Icon(c, iconChevron).Width(13).Height(13).TextColor(cl.Muted).Rotate(angle) })
		ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted).Margin(0, 4, 0, 2)
		ui.Text(c, p.Title).FontSize(13.5).FontWeight(650).TextColor(cl.Text).MaxLines(1).Shrink(1)
		ui.Text(c, p.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1).Margin(0, 0, 0, 8)
		ui.Text(c, strconv.Itoa(r.n)).FontSize(12).TextColor(cl.Muted).FontFeatures("tnum")
		open := 0
		for _, k := range r.tasks {
			if !k.done {
				open++
			}
		}
		if open > 1 { // every open task shown under it, ticked off at once (#505)
			all := ui.Row(c.Key("tgh-all")).Height(24).Padding(0, 8).Gap(5).Radius(rad(6)).AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label(locales.TasksCloseAllLabel(p.Title))
			if all.Hovered() {
				all.Background(cl.Fill)
			}
			all.Children(func() {
				ui.Icon(c, kindIcon["tasks"]).Width(13).Height(13).TextColor(cl.Muted)
				ui.Text(c, locales.TasksCloseAll()).FontSize(12).TextColor(cl.Text2).NoWrap()
			})
			if all.Clicked() {
				tasks := r.tasks
				a.ask(locales.TasksCloseAllTitle(), locales.TasksCloseAllBody(open, p.Title), locales.TasksCloseAll(), func() { a.closeTasks(p, tasks) })
			}
		}
	})
	switch {
	case fold.Clicked():
		if a.taskFolded == nil {
			a.taskFolded = map[*Page]bool{}
		}
		a.taskFolded[p] = !folded
	case head.Clicked():
		a.open(p)
	}
}

// taskRow is one task: its box, its words (a click goes to its line), its day.
// taskKey is what a task is known by between frames: its page and its line.
func taskKey(k taskItem) string { return k.page.Dir + ":" + strconv.Itoa(k.line) }

// taskLeave is how long a task just done takes to go out of the open ones: it shows done, struck through and faint,
// for a moment, then its row slides a little and closes (#494, #508).
const taskLeave = 700 * time.Millisecond

func (a *app) taskRow(c *ui.Context, cl ryugo.Colors, k taskItem) {
	c.Key("tk:" + taskKey(k))
	row := ui.Row(c).FillWidth().Padding(3, 8, 3, 14).Gap(8).Radius(rad(8)).AlignItems(ui.Center)
	leave := float32(-1) // how far it has gone out, 0 to 1; -1 when it is not going
	if t0, ok := a.taskLeaving[taskKey(k)]; ok {
		if leave = float32(c.Now().Sub(t0)) / float32(taskLeave); leave >= 1 || stillNow() {
			delete(a.taskLeaving, taskKey(k))
			leave = 1
		}
		c.Invalidate()
		if close := smooth(0.45, 1, float64(leave)); close > 0 { // done for a moment, then it goes
			row.MaxHeight(f32(34*(1-close))).Clip().Opacity(f32(1-close)).Translate(f32(-10*close), 0)
		}
	}
	if row.Hovered() {
		row.Background(cl.Fill.Alpha(0.5))
	}
	row.Children(func() {
		on := k.done || leave >= 0
		ryugo.Checkbox(c.Key("ck"), &on, "", ryugo.CheckOptions{}).Label(locales.TasksTaskLabel(k.text))
		if tk, ok := taskKindByID(k.kind); ok && !k.done { // its kind, in its color
			ui.Icon(c.Key("kind"), tk.icon).Width(15).Height(15).TextColor(ui.Hex(tk.color)).Label(msg(tk.name))
		}
		col, st := cl.Text, false
		if k.done || leave >= 0 {
			col, st = cl.Muted, true
		}
		body := ui.Row(c.Key("tkt")).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Label(locales.TasksGoToLineLabel(k.text)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		body.Children(func() {
			tx := ui.Text(c, k.text).FontSize(14).TextColor(col).Grow(1).MinWidth(0).Wrap()
			if st {
				tx.Strikethrough()
			}
			if k.every != "" {
				ui.Text(c, "↻ "+k.every).FontSize(11.5).TextColor(cl.Muted).NoWrap()
			}
			if !k.due.IsZero() {
				dc := cl.Muted
				if !k.done && k.due.Before(startOfDay(a.clock())) {
					dc = cl.Up
				}
				ui.Row(c.Key("due")).Padding(1, 7).Radius(rad(6)).Gap(4).AlignItems(ui.Center).Background(dc.Alpha(0.12)).Children(func() {
					ui.Icon(c, iconCalendar).Width(12).Height(12).TextColor(dc)
					ui.Text(c, shortDate(k.due)).FontSize(11.5).TextColor(dc).NoWrap()
				})
			}
		})
		a.kbMark(body, "k:"+k.page.Dir+":"+itoa(k.line), true, func() { a.openTask(k) })
		if body.Clicked() {
			a.openTask(k)
		}
		if on != k.done {
			if on && a.taskFilter == 0 && !stillNow() { // done in the open ones: it goes out, not at once
				if a.taskLeaving == nil {
					a.taskLeaving = map[string]time.Time{}
				}
				a.taskLeaving[taskKey(k)] = c.Now()
			}
			a.toggleTaskSoon(k)
		}
	})
}

// taskKindKey is what the kind filter knows task k by: its kind's id, or "plain".
func taskKindKey(k taskItem) string {
	if k.kind == "" {
		return "plain"
	}
	return k.kind
}

// startOfDay is d at its day's start.
func startOfDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// dueMatches reports whether open task k is in the group of days chosen:
// 0 any, 1 today's, 2 this week's (to the Sunday, today's and after), 3 late.
func dueMatches(k taskItem, group int, now time.Time) bool {
	if group == 0 {
		return true
	}
	if k.due.IsZero() {
		return false
	}
	today := startOfDay(now)
	switch group {
	case 1:
		return sameDay(k.due, today)
	case 2:
		end := weekStart(today).AddDate(0, 0, 7)
		return !k.due.Before(today) && k.due.Before(end)
	}
	return k.due.Before(today)
}

// ---- tasks that come again ----

// everyTok is how a task says it comes again: every:week, every:2week,
// her:hafta (day gün, week hafta, month ay, year yıl).
var everyTok = regexp.MustCompile(`(?i)\s*\b(?:every|her):(\d*)(day|week|month|year|gün|gun|hafta|ay|yıl|yil)\b`)

type recur struct {
	n    int
	unit string // day, week, month, year
}

// parseEvery reads the recurrence a task's text says.
func parseEvery(s string) (recur, bool) {
	m := everyTok.FindStringSubmatch(s)
	if m == nil {
		return recur{}, false
	}
	n := 1
	if m[1] != "" {
		n, _ = strconv.Atoi(m[1])
	}
	if n < 1 {
		n = 1
	}
	unit := map[string]string{"day": "day", "gün": "day", "gun": "day", "week": "week", "hafta": "week", "month": "month", "ay": "month", "year": "year", "yıl": "year", "yil": "year"}[strings.ToLower(m[2])]
	return recur{n, unit}, true
}

// taskLineRe matches a markdown task list item with its checkbox marker.
var taskLineRe = regexp.MustCompile(`^(\s*[-*+]\s+)\[[ xX*!?/e]\](\s*)(.*)$`)

// next returns the first recurrence date after d.
func (r recur) next(d time.Time) time.Time { return r.nextN(d, 1) }

// nextN returns the recurrence date steps intervals after d, clamping month ends.
func (r recur) nextN(d time.Time, steps int) time.Time {
	switch r.unit {
	case "week":
		return d.AddDate(0, 0, 7*r.n*steps)
	case "month", "year":
		months := r.n * steps
		if r.unit == "year" {
			months *= 12
		}
		y, m, day := d.Date()
		first := time.Date(y, m+time.Month(months), 1, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
		lastDay := first.AddDate(0, 1, -1).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(first.Year(), first.Month(), day, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
	}
	return d.AddDate(0, 0, r.n*steps)
}

// label says it as the task view shows it: "her hafta", "her 2 ayda".
func (r recur) label() string {
	if r.n == 1 {
		return map[string]string{"day": "her gün", "week": "her hafta", "month": "her ay", "year": "her yıl"}[r.unit]
	}
	return "her " + strconv.Itoa(r.n) + " " + map[string]string{"day": "günde", "week": "haftada", "month": "ayda", "year": "yılda"}[r.unit]
}

// comesAgain makes the line of the task that follows when the task of line is
// ticked off, if it comes again: the same, open, its day the next one after
// today (counting from its own day, or from today when it has none).
func comesAgain(line string, now time.Time) (string, bool) {
	r, ok := parseEvery(line)
	if !ok {
		return "", false
	}
	m := taskLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	text := m[3]
	today := startOfDay(now)
	base, dated := today, false
	text = wikiInText.ReplaceAllStringFunc(text, func(s string) string {
		if dated {
			return s
		}
		inside := strings.TrimSpace(s[2 : len(s)-2])
		if d, err := parseDay(inside); err == nil {
			base, dated = d, true
			return "\x00"
		}
		return s
	})
	step := 1
	next := r.nextN(base, step)
	for !next.After(today) {
		step++
		next = r.nextN(base, step)
	}
	day := "[[" + next.Format("2006-01-02") + "]]"
	if dated {
		text = strings.Replace(text, "\x00", day, 1)
	} else {
		text = strings.TrimSpace(everyTok.ReplaceAllString(text, "")) + " " + day + everyTok.FindString(m[3])
	}
	return m[1] + "[ ]" + m[2] + text, true
}

// rollRecurring looks in the open page for a task ticked off in the text
// that comes again (every:week): the done one loses its mark, and the next is
// made under it. A task ticked from the task view is done by toggleTask.
func (a *app) rollRecurring() {
	if a.doc == nil || a.page == nil {
		return
	}
	rollRecurringIn(a.doc, a.clock(), a.parse)
}

// rollRecurringIn is rollRecurring in doc, the text of what comes again made with parse.
func rollRecurringIn(doc *richtext.Document, now time.Time, parse func(string) richtext.Text) {
	text := doc.Text()
	var at []int
	for s, para := range text.Paragraphs() {
		if para.Style.List == richtext.Tasks && para.Style.Checked && everyTok.MatchString(paraText(para)) {
			at = append(at, s)
		}
	}
	for i := len(at) - 1; i >= 0; i-- {
		s, e := text.ParagraphRange(at[i])
		line := strings.TrimRight(bodyMarkdown(text.Slice(s, e)), "\n")
		next, ok := comesAgain(line, now)
		if !ok {
			continue
		}
		done := everyTok.ReplaceAllString(line, "")
		doc.Edit(func(ed *richtext.Edit) { ed.Replace(s, e, parse(done+"\n"+next)) })
		text = doc.Text()
	}
}
