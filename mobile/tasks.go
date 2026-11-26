package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Tasks screen is every task not done in the library, by its page, the page written last first: a task taps
// through to its page.

// openTask is a task not done, and its page.
type openTask struct {
	p    *library.Page
	text string
}

// openTasks are the tasks not done of every page, as their Markdown has them ("- [ ] …").
func (a *app) openTasks() []openTask {
	var out []openTask
	for _, p := range a.allPages() {
		for _, l := range strings.Split(mdOf(p), "\n") {
			l = strings.TrimLeft(l, " \t>")
			for _, mark := range []string{"- [ ] ", "* [ ] ", "+ [ ] "} {
				if rest, ok := strings.CutPrefix(l, mark); ok {
					if t := strings.TrimSpace(textnorm.Plain(rest)); t != "" {
						out = append(out, openTask{p, t})
					}
					break
				}
			}
		}
	}
	return out
}

func (a *app) tasksScreen(c *ui.Context) {
	tasks := a.openTasks()
	var pages []*library.Page
	byPage := map[*library.Page][]string{}
	for _, t := range tasks {
		if byPage[t.p] == nil {
			pages = append(pages, t.p)
		}
		byPage[t.p] = append(byPage[t.p], t.text)
	}
	slices.SortStableFunc(pages, func(x, y *library.Page) int { return y.Modified.Compare(x.Modified) })
	tint := tints[2]
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				ui.Column(c).Padding(6, 0, 10, 0).Gap(4).Children(func() {
					ui.Text(c, locales.TasksTitle()).FontSize(38).FontWeight(800).Role(ui.RoleHeading).Level(1)
					if len(tasks) > 0 {
						ui.Text(c, strconv.Itoa(len(tasks))+" "+locales.HomeTasks(len(tasks))).FontSize(15).TextColor(pal.muted)
					}
				})
				if len(tasks) == 0 {
					ui.Text(c, locales.TasksNone()).FontSize(16).TextColor(pal.muted).Wrap()
				}
				for _, p := range pages {
					b := block(c, p, pal.block, smallR+4).FillWidth().Padding(16, 18).Gap(10).Role(ui.RoleButton).
						Label(titleOf(p))
					b.Children(func() {
						ui.Text(c, titleOf(p)).FontSize(13).FontWeight(700).TextColor(pal.faint).MaxLines(1)
						for i, t := range byPage[p] {
							ui.Row(c.Key(i)).Gap(12).AlignItems(ui.Start).Children(func() {
								ui.Box(c).Size(22, 22).Radius(8).Border(2, tint).Margin(1, 0, 0, 0).Shrink(0)
								ui.Text(c, t).FontSize(16).LineHeight(1.35).Wrap().Grow(1).MinWidth(0)
							})
						}
					})
					if b.Clicked() {
						a.openPage(p)
					}
				}
			})
		})
	})
}
