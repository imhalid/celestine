package main

import (
	"regexp"
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

var taskLine = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s*(.*?)\s*$`)

// taskItem is a task: its page, which line of the page's Markdown, whether
// it is done, and what it says.
type taskItem struct {
	page  *Page
	line  int
	done  bool
	text  string
	due   time.Time // a day named in it, as [[2026-11-10]]; zero when none
	every string    // how it comes again, as "her hafta"; "" when it does not
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
			out = append(out, taskItem{p, i, m[2] != " ", text, due, every})
		}
	}
	return out
}

// allTasks are the library's tasks: the pages last changed first, each
// page's in the order of its text.
func (a *app) allTasks(force bool) []taskItem {
	a.linksTo(nil, a.clock(), force)
	out := append([]taskItem{}, a.links.tasks...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].page != out[j].page {
			return out[i].page.Modified.After(out[j].page.Modified)
		}
		return out[i].line < out[j].line
	})
	return out
}

// openTask opens the page of task k with the caret in the task's line.
func (a *app) openTask(k taskItem) {
	a.open(k.page)
	body, _ := k.page.Body()
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
	a.tasksOpen, a.tag, a.coll, a.trashOpen, a.searchOpen = true, "", nil, false, false
	a.linksStale()
	a.visit()
}

// toggleTask ticks a task off, or on again: in the page's file, and in its
// text where that is open.
func (a *app) toggleTask(t taskItem) {
	p := t.page
	if p == a.page {
		a.flush()
	}
	body, err := p.Body()
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	lines := strings.Split(body, "\n")
	if t.line >= len(lines) || taskLine.FindStringSubmatch(lines[t.line]) == nil {
		a.linksStale() // the page changed since: read it again
		return
	}
	mark := "[x]"
	if t.done {
		mark = "[ ]"
	}
	if next, ok := comesAgain(lines[t.line], a.clock()); ok && !t.done {
		// a task that comes again: this one is done, and the next is made
		lines[t.line] = regexp.MustCompile(`\[[ xX]\]`).ReplaceAllString(everyTok.ReplaceAllString(lines[t.line], ""), "[x]")
		lines = append(lines[:t.line+1], append([]string{next}, lines[t.line+1:]...)...)
	} else {
		lines[t.line] = regexp.MustCompile(`\[[ xX]\]`).ReplaceAllString(lines[t.line], mark)
	}
	if err := p.Save(strings.Join(lines, "\n")); err != nil {
		a.saveErr = err.Error()
		return
	}
	a.takeOutside(p) // its text, in the editor, as the file has it now
	a.linksStale()
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
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, "Görevler").FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		ui.Text(c, strconv.Itoa(open)+" açık, "+strconv.Itoa(done)+" tamamlandı  ·  sayfalardaki - [ ] satırları").
			FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 4, 0)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ryugo.Segmented(c.Key("task-filter"), &a.taskFilter, []string{"Açık", "Tamamlanan", "Hepsi"}, ryugo.SegmentedOptions{Label: "Görevler", Size: ryugo.Sm})
			if a.taskFilter == 0 { // the open ones by their day
				ryugo.Segmented(c.Key("task-due"), &a.taskDue, []string{"Hangi gün olsa", "Bugün", "Bu hafta", "Gecikmiş"}, ryugo.SegmentedOptions{Label: "Gün", Size: ryugo.Sm})
			}
		})
		var shown []taskItem
		for _, k := range all {
			if (a.taskFilter == 2 || (a.taskFilter == 0) == !k.done) && (a.taskFilter != 0 || dueMatches(k, a.taskDue, a.clock())) {
				shown = append(shown, k)
			}
		}
		if len(shown) == 0 {
			msg := map[int]string{0: "Açık görev yok. Bir sayfada [] yazınca görev olur.", 1: "Tamamlanan görev yok.", 2: "Hiç görev yok. Bir sayfada [] yazınca görev olur."}[a.taskFilter]
			ui.Text(c, msg).TextColor(t.TextMuted).Padding(16, 4)
			return
		}
		ui.Scroll(c.Key("tasks-scroll")).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Padding(6, 0, 40, 0).Children(func() {
				for i := 0; i < len(shown); {
					j := i
					for j < len(shown) && shown[j].page == shown[i].page {
						j++
					}
					a.taskGroup(c, cl, shown[i:j])
					i = j
				}
			})
		})
	})
}

// taskGroup is a page and its tasks.
func (a *app) taskGroup(c *ui.Context, cl ryugo.Colors, tasks []taskItem) {
	p := tasks[0].page
	ui.Column(c.Key("tg:" + p.Dir)).FillWidth().Gap(1).Children(func() {
		head := ui.Row(c.Key("tgh")).FillWidth().Padding(4, 8).Gap(8).Radius(8).AlignItems(ui.Center).
			Label("Sayfa: " + p.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		if head.Hovered() {
			head.Background(cl.Fill)
		}
		a.contextOn(head, func() []menuItem { return a.pageMenu(p, false) })
		head.Children(func() {
			ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted)
			ui.Text(c, p.Title).FontSize(13.5).FontWeight(650).TextColor(cl.Text).MaxLines(1).Shrink(1)
			ui.Text(c, p.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1)
		})
		if head.Clicked() {
			a.open(p)
		}
		for _, k := range tasks {
			k := k
			row := ui.Row(c.Key("tk:"+strconv.Itoa(k.line))).FillWidth().Padding(3, 8, 3, 14).Gap(8).Radius(8).AlignItems(ui.Center)
			if row.Hovered() {
				row.Background(cl.Fill.Alpha(0.5))
			}
			row.Children(func() {
				on := k.done
				ryugo.Checkbox(c.Key("ck"), &on, "", ryugo.CheckOptions{}).Label("Görev: " + k.text)
				col, st := cl.Text, false
				if k.done {
					col, st = cl.Muted, true
				}
				body := ui.Row(c.Key("tkt")).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Label("Satıra git: " + k.text).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
						ui.Row(c.Key("due")).Padding(1, 7).Radius(6).Gap(4).AlignItems(ui.Center).Background(dc.Alpha(0.12)).Children(func() {
							ui.Icon(c, iconCalendar).Width(12).Height(12).TextColor(dc)
							ui.Text(c, shortDate(k.due)).FontSize(11.5).TextColor(dc).NoWrap()
						})
					}
				})
				if body.Clicked() {
					a.openTask(k)
				}
				if on != k.done {
					a.toggleTask(k)
				}
			})
		}
	})
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
		end := today.AddDate(0, 0, (7-int(today.Weekday()))%7+1)
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
	m := regexp.MustCompile(`^(\s*[-*+]\s+)\[[ xX]\](\s*)(.*)$`).FindStringSubmatch(line)
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
	text := a.doc.Text()
	var at []int
	for s, para := range text.Paragraphs() {
		if para.Style.List == richtext.Tasks && para.Style.Checked && everyTok.MatchString(paraText(para)) {
			at = append(at, s)
		}
	}
	for i := len(at) - 1; i >= 0; i-- {
		s, e := text.ParagraphRange(at[i])
		line := strings.TrimRight(bodyMarkdown(text.Slice(s, e)), "\n")
		next, ok := comesAgain(line, a.clock())
		if !ok {
			continue
		}
		done := everyTok.ReplaceAllString(line, "")
		a.doc.Edit(func(ed *richtext.Edit) { ed.Replace(s, e, a.parse(done+"\n"+next)) })
		text = a.doc.Text()
	}
}
