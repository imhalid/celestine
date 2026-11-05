package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
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
	page *Page
	line int
	done bool
	text string
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
			out = append(out, taskItem{p, i, m[2] != " ", wikiInText.ReplaceAllStringFunc(m[3], func(s string) string {
				inside := s[2 : len(s)-2]
				target, alias, _ := strings.Cut(inside, "|")
				if alias != "" {
					return alias
				}
				return target
			})})
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

// openTasks shows the task view.
func (a *app) openTasks() {
	a.flush()
	a.tasksOpen, a.tag, a.coll, a.trashOpen = true, "", nil, false
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
	lines[t.line] = regexp.MustCompile(`\[[ xX]\]`).ReplaceAllString(lines[t.line], mark)
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
		ui.Row(c).Children(func() {
			ryugo.Segmented(c.Key("task-filter"), &a.taskFilter, []string{"Açık", "Tamamlanan", "Hepsi"}, ryugo.SegmentedOptions{Label: "Görevler", Size: ryugo.Sm})
		})
		var shown []taskItem
		for _, k := range all {
			if a.taskFilter == 2 || (a.taskFilter == 0) == !k.done {
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
				tx := ui.Text(c, k.text).FontSize(14).TextColor(col).Grow(1).MinWidth(0).Wrap()
				if st {
					tx.Strikethrough()
				}
				if on != k.done {
					a.toggleTask(k)
				}
			})
		}
	})
}
