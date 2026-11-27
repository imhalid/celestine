package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Tasks screen is every task not done in the library, by its page, the page written last first. A task is ticked
// off where it is, written to its page at once and shown done until the screen is left, so that a tick by mistake
// is taken back with another; a page's title opens it. A past day's tasks stay as they were: the day is locked (#637).

// openTask is a task of a page: its paragraph in the page's document, its words, and whether it is done.
type openTask struct {
	p    *library.Page
	para int
	text string
	done bool
}

// key names the task among the library's.
func (t openTask) key() string { return t.p.ID + "#" + strconv.Itoa(t.para) }

// taskCache is a page's tasks, as of its file's modification.
type taskCache struct {
	mod   time.Time
	tasks []openTask
}

// pageTasks are the pages' tasks, by page, read again only when a page's file changed.
var pageTasks = map[*library.Page]taskCache{}

// tasksOf are the tasks of page p's document, done or not.
func tasksOf(p *library.Page) []openTask {
	if tc, ok := pageTasks[p]; ok && tc.mod.Equal(p.Modified) {
		return tc.tasks
	}
	var out []openTask
	if body, err := p.Body(); err == nil {
		if d, err := library.DecodeBody(body); err == nil {
			for i, para := range d {
				if para.S == nil || para.S.List != library.ListTasks {
					continue
				}
				var b strings.Builder
				for _, r := range para.R {
					b.WriteString(r.T)
				}
				if t := strings.TrimSpace(b.String()); t != "" {
					out = append(out, openTask{p: p, para: i, text: t, done: para.S.Checked})
				}
			}
		}
	}
	pageTasks[p] = taskCache{mod: p.Modified, tasks: out}
	return out
}

// openTasks are the tasks not done of every page.
func (a *app) openTasks() []openTask {
	var out []openTask
	for _, p := range a.allPages() {
		for _, t := range tasksOf(p) {
			if !t.done {
				out = append(out, t)
			}
		}
	}
	return out
}

// tick marks task t done, or not, in its page's file.
func (a *app) tick(t openTask, done bool) {
	p := t.p
	if p.Locked != "" {
		a.say(locales.TasksLocked())
		return
	}
	if a.ed.id == p.ID {
		a.flush()
	}
	body, err := p.Body()
	if err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	d, err := library.DecodeBody(body)
	if err != nil || t.para >= len(d) || d[t.para].S == nil || d[t.para].S.List != library.ListTasks {
		return // the page changed under the list: it is read again
	}
	d[t.para].S.Checked = done
	if err := p.Save(library.EncodeBody(d)); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	if a.ed.id == p.ID && !a.ed.dirty {
		a.ed = a.stateOf(p)
	}
	if done {
		a.ticked[t.key()] = true
	} else {
		delete(a.ticked, t.key())
	}
	a.syncSoon()
}

func (a *app) tasksScreen(c *ui.Context) {
	open := a.openTasks()
	// The tasks of the pages with one open, or one ticked off here, in their order on the page.
	byPage := map[*library.Page][]openTask{}
	var pages []*library.Page
	for _, p := range a.allPages() {
		var ts []openTask
		for _, t := range tasksOf(p) {
			if !t.done || a.ticked[t.key()] {
				ts = append(ts, t)
			}
		}
		if len(ts) > 0 {
			byPage[p] = ts
			pages = append(pages, p)
		}
	}
	slices.SortStableFunc(pages, func(x, y *library.Page) int { return y.Modified.Compare(x.Modified) })
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				ui.Column(c).Padding(6, 0, 10, 0).Gap(4).Children(func() {
					ui.Text(c, locales.TasksTitle()).FontSize(38).FontWeight(800).Role(ui.RoleHeading).Level(1)
					if len(open) > 0 {
						ui.Text(c, strconv.Itoa(len(open))+" "+locales.HomeTasks(len(open))).FontSize(15).TextColor(pal.muted)
					}
				})
				if len(pages) == 0 {
					ui.Text(c, locales.TasksNone()).FontSize(16).TextColor(pal.muted).Wrap()
				}
				for _, p := range pages {
					ui.Column(c.Key(p)).FillWidth().Padding(6, 8, 8, 8).Radius(smallR + 4).Background(pal.block).Children(func() {
						head := pressable(ui.Row(c.Key("head")).FillWidth().Gap(8).AlignItems(ui.Center).Padding(10, 10, 6, 10).
							Radius(14).Role(ui.RoleButton).Label(titleOf(p)))
						head.Children(func() {
							ui.Text(c, titleOf(p)).FontSize(13).FontWeight(700).TextColor(pal.faint).MaxLines(1).Grow(1).MinWidth(0)
							if p.Locked != "" {
								ui.Icon(c, iconLock).Size(14, 14).TextColor(pal.faint)
							}
							ui.Icon(c, iconArrow).Size(15, 15).TextColor(pal.faint)
						})
						if head.Clicked() {
							a.openPage(p)
						}
						for _, t := range byPage[p] {
							a.taskRow(c, t)
						}
					})
				}
			})
		})
	})
}

// taskRow is a task to tick off: its box, which fills and shows a check as it is ticked, and its words, struck
// through once done.
func (a *app) taskRow(c *ui.Context, t openTask) {
	row := ui.Row(c.Key(t.key())).FillWidth().Gap(12).AlignItems(ui.Start).Padding(10).Radius(14).
		Role(ui.RoleCheckBox).Label(t.text).Checked(t.done)
	if row.Pressed() {
		row.Background(pal.raised)
	}
	k := row.AnimateTo("done", b2f(t.done), ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.3}})
	locked := t.p.Locked != ""
	row.Children(func() {
		box := ui.Box(c).Size(24, 24).Radius(8).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
		edge := pal.accent
		if locked {
			edge = pal.faint
		}
		box.Border(2, edge.Mix(pal.accent, k))
		if k > 0.01 {
			box.Background(pal.accent.Alpha(min(k, 1)))
		}
		box.Children(func() {
			ui.Icon(c, iconCheck).Size(16, 16).TextColor(pal.accentText).Scale(max(k, 0.001), max(k, 0.001))
		})
		color := pal.text
		if t.done {
			color = pal.faint
		}
		words := ui.Text(c, t.text).FontSize(16).LineHeight(1.35).Wrap().Grow(1).MinWidth(0).TextColor(color)
		if t.done {
			words.Strikethrough()
		}
	})
	if row.Clicked() {
		a.tick(t, !t.done)
	}
}
