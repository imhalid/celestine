package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Subtasks (#477). A task nests under the one above it with Tab, as any list line does, and the page stays plain
// Markdown: a nested `- [ ]` list. What is new is drawn, never written: a task with subtasks shows, in a room the
// editor leaves under its line (richtext's Space), a small chip - a chevron that folds them away, done/total of its
// direct subtasks, and the dashed ring of #494, a dash a subtask, green as each is done - and its own box fills as far
// as they are done while it is not. Ticking the last subtask does not tick the task, nor does ticking the task tick
// them: what is done is said by hand, as in the file. Folded, the fold's chip at the end of the line counts them
// (chips.go), the same direct subtasks.

const (
	subtaskRoom  = 24 // the room under a task with subtasks, for its chip
	subtaskChipH = 18
)

// subtaskCount is a task's direct subtasks, and how many of them are done.
type subtaskCount struct{ done, total int }

// subtaskState is the counts of the text last read, by paragraph index, and the paragraphs that have the room of
// their parent's chip above them.
type subtaskState struct {
	text  richtext.Text
	off   bool                 // the settings hide the counts (TaskNoCounts): none were read
	count map[int]subtaskCount // by the task's paragraph
	room  map[int]int          // by the first paragraph under a task: that task's paragraph
}

// subtasksOf is the counts of pn's text, read again only when the text is another (paraTableOf's table).
func (a *app) subtasksOf(pn *paneState) *subtaskState {
	st := &a.subtasks
	if pn.doc == nil {
		return st
	}
	t := pn.doc.Text()
	if st.count != nil && st.text.Same(t) && st.off == a.settings.TaskNoCounts {
		return st
	}
	st.text, st.count, st.room, st.off = t, map[int]subtaskCount{}, map[int]int{}, a.settings.TaskNoCounts
	if st.off {
		return st
	}
	paras := a.paraTableOf(pn, t).paras
	for i, p := range paras {
		if p.Style.List != richtext.Tasks || paraFolded(p) {
			continue
		}
		var n subtaskCount
		for j := i + 1; j < len(paras); j++ { // the lines under it: list lines indented further
			q := paras[j].Style
			if q.List == 0 || q.Indent <= p.Style.Indent {
				break
			}
			if k, ok := taskKindOf(q); ok && k.id == "canceled" && !q.Checked { // a subtask canceled is not one to do (#597)
				continue
			}
			if q.Indent == p.Style.Indent+1 && q.List == richtext.Tasks {
				n.total++
				if q.Checked {
					n.done++
				}
			}
		}
		if n.total > 0 {
			st.count[i] = n
			st.room[i+1] = i
		}
	}
	return st
}

// paraFolded reports whether paragraph p holds a fold's chip: what was under it is in the chip, not after it.
func paraFolded(p richtext.Paragraph) bool {
	for _, r := range p.Runs {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "fold" {
				return true
			}
		}
	}
	return false
}

// paraRoom is the room the editor leaves above paragraph p: a dragged block's (blockui.go) and a parent task's chip.
func (a *app) paraRoom(pn *paneState, p int) float32 {
	room := a.blockRoom(pn, p)
	if _, ok := a.subtasksOf(pn).room[p]; ok {
		room += subtaskRoom
	}
	return room
}

// drawTaskPart fills a task's box as far as its subtasks are done, while it is not: from the bottom, in the box's
// color, faint.
func drawTaskPart(p *ui.Painter, at ui.Rect, n subtaskCount, color ui.Color) {
	if n.total == 0 || n.done == 0 {
		return
	}
	k := float32(n.done) / float32(n.total)
	h := at.H * k
	p.Clip(ui.Rect{X: at.X, Y: at.Y + at.H - h, W: at.W, H: h}, 0, func() {
		p.Fill(at, color.Alpha(0.38), at.W*0.25)
	})
}

// subtaskChips puts each task's chip in view in the room under its line, over the window: a click on it folds the
// subtasks away (fold.go), where the fold's own chip says them.
func (a *app) subtaskChips(c *ui.Context) {
	pn := a.pane()
	if pn.doc == nil || a.noPage() || pn.editorR.W == 0 || a.modal() || pn.grip.dragging {
		return
	}
	st := a.subtasksOf(pn)
	if len(st.count) == 0 {
		return
	}
	r := pn.editorR
	top, bottom := r.Y, r.Y+r.H
	if pn.scrollR.H > 0 {
		top, bottom = max(top, pn.scrollR.Y+pn.barInset), min(bottom, pn.scrollR.Y+pn.scrollR.H)
	}
	cl := ryugo.From(c).Colors
	ui.Overlay(c, func() {
		for _, b := range a.shown(pn, r) {
			n, ok := st.count[b.Index]
			if !ok {
				continue
			}
			x, ok := a.taskTextX(pn, b)
			if !ok {
				continue
			}
			y := r.Y + b.Top + b.Height + (subtaskRoom-subtaskChipH)/2 - 1
			if y < top || y+subtaskChipH > bottom {
				continue
			}
			at := b.Start
			label := locales.TasksSubtasksDone(n.done, n.total)
			chip := ui.Row(c.Key("subtasks:"+strconv.Itoa(at))).Absolute().Left(r.X+x-4).Top(y).Height(subtaskChipH).
				Padding(0, 7, 0, 4).Gap(5).AlignItems(ui.Center).Radius(subtaskChipH / 2).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).Tooltip(locales.TasksFoldSubtasks())
			done := n.done == n.total
			par := pn.doc.Text().ParagraphStyleAt(at)
			ps, pe, _ := a.paraOfBox(pn, b)
			face := a.faceOf(par, paraLine(pn.doc.Text(), ps, pe))
			tint := cl.Fill // the pill in the parent's state's color (#597)
			switch {
			case done || par.Checked:
				tint = cl.Down.Alpha(0.16)
			case face.id == "canceled":
				tint = cl.Muted.Alpha(0.14)
			case face.urgent():
				tint = ui.Hex("#EF4444").Alpha(0.14)
			case face.id != "":
				tint = face.color.Alpha(0.14)
			}
			chip.Background(tint)
			if chip.Hovered() {
				chip.Background(ryugo.Mix(tint, cl.Fill, 0.6))
			}
			ink := map[bool]ui.Color{true: cl.Down, false: cl.Muted}[done]
			chip.Children(func() {
				ui.Icon(c, iconChevronDown).Width(11).Height(11).TextColor(cl.Muted)
				ui.Text(c, strconv.Itoa(n.done)+"/"+strconv.Itoa(n.total)).FontSize(11.5).FontWeight(560).TextColor(ink)
				ui.Box(c).Size(12, 12).Draw(func(p *ui.Painter, rr ui.Rect) {
					drawTaskRing(p, rr.X+rr.W/2, rr.Y+rr.H/2, 5, n.total, n.done, cl.Muted.Alpha(0.55), cl.Down)
				})
			})
			if chip.Clicked() {
				a.toggleFold(at)
			}
		}
	})
}

// taskTextX is where the text of the task at paragraph box b starts, in the editor's element.
func (a *app) taskTextX(pn *paneState, b richtext.ParagraphBox) (float32, bool) {
	if a.theme == nil {
		return 0, false
	}
	s, e, para := a.paraOfBox(pn, b)
	cb, ok := locateIn(pn.doc, b.Start, b, s, e, para, pn.editorR.W, a.font, a.theme)
	if !ok {
		return 0, false
	}
	return cb.full.X, true
}
