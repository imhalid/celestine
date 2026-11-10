package main

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
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

// allTasks is every task, the pages changed last first, each page's in the order of its text. It is
// sorted when the index of links is made, not for each frame: 20000 tasks.
func (a *app) allTasks(force bool) []taskItem {
	a.linksTo(nil, a.clock(), force)
	if a.links.sorted == nil && len(a.links.tasks) > 0 {
		out := append([]taskItem{}, a.links.tasks...)
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].page != out[j].page {
				return out[i].page.Modified.After(out[j].page.Modified)
			}
			return out[i].line < out[j].line
		})
		a.links.sorted = out
	}
	return a.links.sorted
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
	a.linksStale()
	a.visit()
}

// toggleTask ticks a task off, or on again: in the page's file, and in its text where that is open. The body is
// changed where the task is, not written again from its Markdown, which loses what Markdown cannot say (#123).
func (a *app) toggleTask(t taskItem) {
	p := t.page
	if p == a.page {
		a.flush()
	}
	body, err := p.Body()
	if err == nil {
		err = bodyReadable(body) // what this version does not read would be dropped by writing it again
	}
	if err != nil {
		a.fail(err)
		return
	}
	tx := decodeBody(body)
	at, ok := taskParagraph(tx, p, t)
	if !ok {
		a.linksStale() // the page changed since: read it again
		return
	}
	doc := richtext.NewDocument(tx)
	doc.ToggleChecked(at)
	rollRecurringIn(doc, a.clock(), importMarkdown) // a task that comes again: this one is done, and the next is made
	if err := p.Save(encodeBody(doc.Text())); err != nil {
		a.fail(err)
		return
	}
	a.takeOutside(p) // its text, in the editor, as the file has it now
	a.linksStale()
}

// taskParagraph is where in tx, page p's text, the task t starts: the task paragraph whose Markdown is t's line,
// counted among the tasks as tasksOf counts them. False when the page has changed since t was read.
func taskParagraph(tx richtext.Text, p *Page, t taskItem) (int, bool) {
	md := bodyMarkdown(tx)
	lines := strings.Split(md, "\n")
	if t.line >= len(lines) {
		return 0, false
	}
	k := -1
	for i, ti := range tasksOf(p, md) {
		if ti.line == t.line {
			k = i
		}
	}
	if k < 0 {
		return 0, false
	}
	for s, para := range tx.Paragraphs() {
		if para.Style.List != richtext.Tasks {
			continue
		}
		s, e := tx.ParagraphRange(s)
		line := strings.TrimSpace(strings.TrimRight(bodyMarkdown(tx.Slice(s, e)), "\n"))
		if m := taskLine.FindStringSubmatch(line); m == nil || m[3] == "" {
			continue
		}
		if k--; k < 0 {
			return s, line == strings.TrimSpace(lines[t.line])
		}
	}
	return 0, false
}

// tasksView is the view: a filter, and the tasks under their pages.
func (a *app) tasksView(c *ui.Context) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	all := a.allTasks(a.links.stale) // a tick shows at once
	open, done := 0, 0
	for _, k := range all {
		if k.done {
			done++
		} else {
			open++
		}
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
		ui.Text(c, T("Görevler")).FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		ui.Text(c, strconv.Itoa(open)+" açık, "+strconv.Itoa(done)+" tamamlandı  ·  sayfalardaki - [ ] satırları").
			FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 4, 0)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			ryugo.Segmented(c.Key("task-filter"), &a.taskFilter, []string{"Açık", "Tamamlanan", "Hepsi"}, ryugo.SegmentedOptions{Label: "Görevler", Size: ryugo.Sm})
			if a.taskFilter == 0 { // the open ones by their day
				ryugo.Segmented(c.Key("task-due"), &a.taskDue, []string{"Hepsi", "Bugün", "Bu hafta", "Gecikmiş"}, ryugo.SegmentedOptions{Label: T("Gün"), Size: ryugo.Sm})
			}
			ui.Spacer(c)
			if len(kinds) > 1 || a.taskKindF != "" {
				opts := []ryugo.Option[string]{{Value: "", Label: T("Tüm türler")}, {Value: "plain", Label: T("Düz görev") + "  ·  " + strconv.Itoa(kinds["plain"])}}
				for _, tk := range taskKinds {
					if kinds[tk.id] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: tk.id, Label: T(tk.name) + "  ·  " + strconv.Itoa(kinds[tk.id])})
					}
				}
				ui.Box(c.Key("task-kind-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-kind"), &a.taskKindF, opts, ryugo.SelectOptions{Label: T("Tür"), Size: ryugo.Sm})
				})
			}
			if len(colls) > 1 {
				opts := []ryugo.Option[string]{{Value: "", Label: T("Tüm koleksiyonlar")}}
				for _, co := range a.lib.Collections {
					if colls[co] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: co.Dir, Label: co.Name + "  ·  " + strconv.Itoa(colls[co])})
					}
				}
				ui.Box(c.Key("task-coll-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-coll"), &a.taskColl, opts, ryugo.SelectOptions{Label: "Koleksiyon", Size: ryugo.Sm})
				})
			}
			a.fieldMenu(ryugo.Input(c.Key("task-q"), &a.taskQuery, ryugo.InputOptions{Label: T("Görevlerde ara"), Placeholder: T("Görevlerde ara…"), PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}), &a.taskQuery, fieldOpts{oneLine: true}).Width(176)
		})
		q := fold(strings.TrimSpace(a.taskQuery))
		now := a.clock()
		var shown []taskItem
		for _, k := range all {
			switch {
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
		a.taskList.Key = func(i int) any { return rows[i].key() }
		ui.List(c.Key("tasks-list"), a.taskList, len(rows), func(i int) {
			if r := rows[i]; r.head {
				a.taskHead(c, cl, r, i > 0)
			} else {
				a.taskRow(c, cl, r.task)
			}
		}).Grow(1)
	})
}

// noTasks says there are no tasks to show: none at all, or none with the filters.
func (a *app) noTasks(c *ui.Context, none bool) {
	o := ryugo.EmptyStateOptions{Icon: iconTask, Title: T("Bu süzgece uyan görev yok"), Description: T("Süzgeci gevşetince burada görünürler.")}
	switch {
	case none:
		o.Title, o.Description = "Henüz görev yok", "Bir sayfada [] yazınca görev olur; burada hepsi, sayfalarına göre durur."
	case a.taskFilter == 1:
		o.Title, o.Description = "Tamamlanan görev yok", "Bir görevi işaretleyince buraya gelir."
	case a.taskFilter == 0 && a.taskDue == 0 && a.taskColl == "" && a.taskKindF == "" && strings.TrimSpace(a.taskQuery) == "":
		o.Title, o.Description = "Açık görev kalmadı", "Hepsi tamam."
	}
	if !none && (a.taskColl != "" || strings.TrimSpace(a.taskQuery) != "" || a.taskDue != 0 || a.taskKindF != "") {
		o.Action = func() {
			if ryugo.Button(c.Key("task-clear-filters"), T("Süzgeçleri temizle"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				a.taskColl, a.taskQuery, a.taskDue, a.taskKindF = "", "", 0, ""
			}
		}
	}
	ui.Column(c).FillWidth().Children(func() { ryugo.EmptyState(c, o) })
}

// taskGroup is a page and its tasks.
// taskListRow is a row of the list of tasks: the head of a page's tasks, or one task.
type taskListRow struct {
	head bool
	task taskItem
	n    int // of a head: how many tasks its page has here
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
		rows = append(rows, taskListRow{head: true, task: shown[i], n: j - i})
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
		Label("Sayfa: " + p.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
			Label(map[bool]string{true: "Görevleri aç: ", false: "Görevleri kapat: "}[folded] + p.Title).Role(ui.RoleButton)
		if fold.Hovered() {
			fold.Background(cl.Fill)
		}
		angle := fold.AnimateTo("turn", map[bool]float32{true: 0, false: 90}[folded], springTurn)
		fold.Children(func() { ui.Icon(c, iconChevron).Width(13).Height(13).TextColor(cl.Muted).Rotate(angle) })
		ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted).Margin(0, 4, 0, 2)
		ui.Text(c, p.Title).FontSize(13.5).FontWeight(650).TextColor(cl.Text).MaxLines(1).Shrink(1)
		ui.Text(c, p.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1).Margin(0, 0, 0, 8)
		ui.Text(c, strconv.Itoa(r.n)).FontSize(12).TextColor(cl.Muted).FontFeatures("tnum")
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
func (a *app) taskRow(c *ui.Context, cl ryugo.Colors, k taskItem) {
	c.Key("tk:" + k.page.Dir + ":" + strconv.Itoa(k.line))
	row := ui.Row(c).FillWidth().Padding(3, 8, 3, 14).Gap(8).Radius(rad(8)).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(cl.Fill.Alpha(0.5))
	}
	row.Children(func() {
		on := k.done
		ryugo.Checkbox(c.Key("ck"), &on, "", ryugo.CheckOptions{}).Label(T("Görev: ") + k.text)
		if tk, ok := taskKindByID(k.kind); ok && !k.done { // its kind, in its color
			ui.Icon(c.Key("kind"), tk.icon).Width(15).Height(15).TextColor(ui.Hex(tk.color)).Label(T(tk.name))
		}
		col, st := cl.Text, false
		if k.done {
			col, st = cl.Muted, true
		}
		body := ui.Row(c.Key("tkt")).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Label(T("Satıra git: ") + k.text).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
			a.toggleTask(k)
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

// next is the day after d.
func (r recur) next(d time.Time) time.Time {
	switch r.unit {
	case "week":
		return d.AddDate(0, 0, 7*r.n)
	case "month":
		return d.AddDate(0, r.n, 0)
	case "year":
		return d.AddDate(r.n, 0, 0)
	}
	return d.AddDate(0, 0, r.n)
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
	m := regexp.MustCompile(`^(\s*[-*+]\s+)\[[ xX*!?/e]\](\s*)(.*)$`).FindStringSubmatch(line)
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
	next := r.next(base)
	for !next.After(today) {
		next = r.next(next)
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
