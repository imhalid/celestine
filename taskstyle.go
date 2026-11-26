package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The look of the task boxes (#597), BetterTask's (#611, taskfaces.go draws them): each status a shape of its own,
// in one of three presets (Neon Glass, Vivid Solid, Minimal Crisp), legible in light and dark. Besides the kinds of
// taskkinds.go, a task can have a status by a tag in its line (#kawaii, #bug…): seven slots whose names the settings
// set, each with a drawing of its own; the tag stays a plain tag in the file, found and synced as any. A task done
// keeps its status's color and shape, a white tick in it. In the settings, on unless turned off: the boxes' small
// ambient motions (never with reduced motion), the burst as a task is done (its shape and colors the status's), a
// tint along the row of a task with a status, and the counts of a task's subtasks.

// The presets, as the settings keep them.
const (
	taskLookNeon    = ""
	taskLookVivid   = "vivid"
	taskLookMinimal = "minimal"
)

// taskLooks are the presets, each with its name.
var taskLooks = []struct{ id, name string }{
	{taskLookNeon, "tasks_lookNeon"}, {taskLookVivid, "tasks_lookVivid"}, {taskLookMinimal, "tasks_lookMinimal"},
}

// taskDone is the color of a plain task done.
const taskDone = "#10B981"

// tagStatusSlots are the seven statuses a tag gives, by slot, in BetterTask's order (1 to 7 in the picker): the name
// each has unless the settings rename it, and its color. A slot's drawing stays with it whatever it is named.
var tagStatusSlots = [7]struct{ name, color string }{
	{"kawaii", "#F472B6"}, {"bug", "#84CC16"}, {"draft", "#60A5FA"}, {"deadline", "#FB923C"},
	{"flower", "#E879F9"}, {"sun", "#F59E0B"}, {"moon", "#A5B4FC"},
}

// tagNames are the names of the tag statuses now, by slot, lower case.
func (a *app) tagNames() [7]string {
	var out [7]string
	for i, s := range tagStatusSlots {
		out[i] = s.name
		if i < len(a.settings.TaskTags) {
			if n := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(a.settings.TaskTags[i], "#"))); n != "" {
				out[i] = n
			}
		}
	}
	return out
}

// hashTag is a tag in a line.
var hashTag = regexp.MustCompile(`(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_-]+)`)

// tagStatusIn is the slot of the first tag status in line, and where its tag is in the line's runes, [from, to);
// -1 for none.
func (a *app) tagStatusIn(line string) (slot, from, to int) {
	names := a.tagNames()
	for _, m := range hashTag.FindAllStringSubmatchIndex(line, -1) {
		name := strings.ToLower(line[m[4]:m[5]])
		for i, n := range names {
			if n == name {
				hash := m[4] - 1
				return i, len([]rune(line[:hash])), len([]rune(line[:m[5]]))
			}
		}
	}
	return -1, 0, 0
}

// taskFace is what a task's box shows: its status, by the kind's id, "tag" with its slot, or "" for a plain task.
type taskFace struct {
	id    string
	slot  int
	color ui.Color
}

// faceOf is the face of the task of style st whose line is line.
func (a *app) faceOf(st richtext.ParagraphStyle, line string) taskFace {
	if slot, _, _ := a.tagStatusIn(line); slot >= 0 {
		return taskFace{id: "tag", slot: slot, color: ui.Hex(tagStatusSlots[slot].color)}
	}
	if k, ok := taskKindOf(st); ok {
		return taskFace{id: k.id, slot: -1, color: ui.Hex(k.color)}
	}
	return taskFace{slot: -1, color: ui.Hex(taskDone)}
}

// urgent reports whether the face says the task cannot wait: important, an alert, a bug, a deadline.
func (f taskFace) urgent() bool {
	return f.id == "important" || f.id == "error" || f.id == "tag" && (f.slot == 1 || f.slot == 3)
}

// paraLine is the text of the paragraph starting at s and ending at e in t.
func paraLine(t richtext.Text, s, e int) string { return t.Slice(s, e).String() }

// ---- the tags' text ----

// tagStatusSpans colors the tag of a task's tag status in its line in the status's color; drawTags puts the pill of
// that color under it (#597).
func (a *app) tagStatusSpans(st richtext.ParagraphStyle, spans []ui.Span) []ui.Span {
	if st.List != richtext.Tasks {
		return spans
	}
	var line strings.Builder
	for _, s := range spans {
		line.WriteString(s.Text)
	}
	slot, from, to := a.tagStatusIn(line.String())
	if slot < 0 {
		return spans
	}
	c := a.tagColor(slot)
	return restyleRunes(spans, [][2]int{{from, to}}, func(s *ui.Span) {
		if s.ObjectWidth == 0 {
			s.Color = c
		}
	})
}

// tagStatusColor is the color of tag name as a task's status, and whether it is one.
func (a *app) tagStatusColor(name string) (ui.Color, bool) {
	name = strings.ToLower(name)
	for i, n := range a.tagNames() {
		if n == name {
			return a.tagColor(i), true
		}
	}
	return ui.Color{}, false
}

// tagColor is the color of slot's tag in the page: BetterTask's chip's, deeper on a light page.
func (a *app) tagColor(slot int) ui.Color {
	if sp := tagSpecs[slot]; !darkBg(a.pageBg) && sp.chipL.A > 0 {
		return sp.chipL
	}
	return ui.Hex(tagStatusSlots[slot].color)
}

// ---- the picker ----

// taskPick is the picker of a task's status, opened by ⌥-click on its box: the kinds and the tag statuses, 1 to 7
// the tags', 0 none.
type taskPick struct {
	open bool
	at   int // a rune of the task's paragraph
	x, y float32
	pn   *paneState
}

// setTaskStatus gives the task holding rune at in pn the status id: a kind's id, "tag" with slot, or "" for none.
// The tag statuses in its line go, and a tag's is written at its end.
func (a *app) setTaskStatus(pn *paneState, at int, id string, slot int) {
	t := pn.doc.Text()
	s, e := t.ParagraphRange(at)
	line := []rune(paraLine(t, s, e))
	names := a.tagNames()
	type cut struct{ from, to int }
	var cuts []cut
	str := string(line)
	for _, m := range hashTag.FindAllStringSubmatchIndex(str, -1) {
		name := strings.ToLower(str[m[4]:m[5]])
		for _, n := range names {
			if n == name {
				from := len([]rune(str[:m[4]-1]))
				to := len([]rune(str[:m[5]]))
				if from > 0 && line[from-1] == ' ' {
					from--
				}
				cuts = append(cuts, cut{from, to})
			}
		}
	}
	pn.doc.Edit(func(ed *richtext.Edit) {
		for i := len(cuts) - 1; i >= 0; i-- {
			ed.ReplaceString(s+cuts[i].from, s+cuts[i].to, "", richtext.Attributes{})
		}
		_, e2 := ed.Text().ParagraphRange(s)
		if id == "tag" {
			ed.ReplaceString(e2, e2, " #"+names[slot], ed.Text().AttributesAt(max(s, e2-1)))
		}
		ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) {
			p.List, p.ID = richtext.Tasks, ""
			if id != "" && id != "tag" {
				p.ID = taskIDPrefix + id
			}
		})
	})
}

// tagTitles are the tag statuses' names as the picker and the settings show them, by slot.
var tagTitles = [7]string{"tasks_tagKawaii", "tasks_tagBug", "tasks_tagDraft", "tasks_tagDeadline", "tasks_tagFlower", "tasks_tagSun", "tasks_tagMoon"}

// pickerColors are the colors of BetterTask's picker: its back, border and text, on a dark page or a light one.
func pickerColors(dark bool) (back, border, text ui.Color) {
	if dark {
		return rgba(17, 20, 30, 0.97), rgba(148, 163, 184, 0.22), ui.Hex("#e5e7eb")
	}
	return rgba(255, 255, 255, 0.98), rgba(71, 85, 105, 0.18), ui.Hex("#1e293b")
}

// taskPickerView draws the picker while it is open, over the window, as BetterTask's: the kinds in a row, then the
// custom statuses, each with its name, its tag and its key, None, and the keys at its foot.
func (a *app) taskPickerView(c *ui.Context) {
	pk := &a.tasks.pick
	if !pk.open || pk.pn == nil || pk.pn.doc == nil {
		return
	}
	ww, wh := c.Size()
	dark := darkBg(a.pageBg)
	back, border, ink := pickerColors(dark)
	choose := func(id string, slot int) {
		a.setTaskStatus(pk.pn, pk.at, id, slot)
		pk.open = false
	}
	t := pk.pn.doc.Text()
	s, e := t.ParagraphRange(min(pk.at, t.Len()))
	cur := a.faceOf(t.ParagraphStyleAt(s), paraLine(t, s, e))
	const w = 236
	ui.Overlay(c, func() {
		shade := ui.Box(c.Key("taskpick-shade")).Absolute().Left(0).Top(0).Size(ww, wh)
		if shade.Clicked() {
			pk.open = false
		}
		x, y := min(pk.x, ww-w-8), min(pk.y, wh-340)
		panel := ui.Column(c.Key("taskpick")).Absolute().Left(max(8, x)).Top(max(8, y)).Width(w).Padding(6).
			Radius(12).Background(back).Border(1, border).Label(locales.TasksStatusPicker()).Focusable()
		panel.Shadow(0, 12, 32, 0, rgba(0, 0, 0, map[bool]float32{true: 0.42, false: 0.16}[dark]))
		if !panel.Focused() {
			panel.Focus()
		}
		for k := 0; k <= 7; k++ {
			if panel.Shortcut(0, ui.Key0+ui.Key(k)) {
				if k == 0 {
					choose("", -1)
				} else {
					choose("tag", k-1)
				}
			}
		}
		if panel.Shortcut(0, ui.KeyEscape) {
			pk.open = false
		}
		head := func(text string) {
			ui.Text(c, strings.ToUpper(text)).FontSize(10.5).FontWeight(600).LetterSpacing(0.5).TextColor(ink.Alpha(0.55)).Padding(4, 8, 6, 8)
		}
		hover := rgba(148, 163, 184, 0.14)
		current := rgba(56, 189, 248, 0.14)
		face := func(f taskFace, size float32) {
			ui.Box(c).Size(22, 22).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				a.drawFace(p, ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, f, p.Now())
			})
		}
		row := func(key, name, tag, hint string, f taskFace, on bool) {
			e := ui.Row(c.Key(key)).FillWidth().Gap(8).AlignItems(ui.Center).Padding(5, 8).Radius(8).
				Label(name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			switch {
			case on:
				e.Background(current)
			case e.Hovered():
				e.Background(hover)
			}
			e.Children(func() {
				face(f, 16)
				ui.Text(c, name).FontSize(13).FontWeight(600).TextColor(ink).Grow(1).MaxLines(1)
				ui.Text(c, tag).FontSize(11).Font(monoStack).TextColor(ink.Alpha(0.55)).MaxLines(1)
				ui.Text(c, hint).FontSize(10).FontWeight(700).Font(monoStack).TextColor(ink.Alpha(0.45)).Width(14).TextAlign(ui.End)
			})
			if e.Clicked() {
				choose(f.id, f.slot)
			}
		}
		panel.Children(func() {
			head(locales.TasksPickStatus())
			ui.Row(c).Gap(1).Padding(0, 2, 4, 2).Children(func() {
				for _, k := range taskKinds {
					f := taskFace{id: k.id, slot: -1, color: ui.Hex(k.color)}
					cell := ui.Box(c.Key("tp-"+k.id)).Size(27, 27).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
						Label(msg(k.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer).Tooltip(msg(k.name))
					switch {
					case cur.id == k.id:
						cell.Background(current)
					case cell.Hovered():
						cell.Background(hover)
					}
					cell.Children(func() { face(f, 16) })
					if cell.Clicked() {
						choose(f.id, -1)
					}
				}
			})
			head(locales.TasksPickCustom())
			names := a.tagNames()
			ui.Column(c).Gap(1).Children(func() {
				for i, ts := range tagStatusSlots {
					f := taskFace{id: "tag", slot: i, color: ui.Hex(ts.color)}
					row("tp-tag"+strconv.Itoa(i), msg(tagTitles[i]), "#"+names[i], strconv.Itoa(i+1), f, cur.id == "tag" && cur.slot == i)
				}
				row("tp-none", locales.TasksPickNone(), locales.TasksPickNoneHint(), "0", taskFace{slot: -1}, cur.id == "")
			})
			ui.Box(c).Height(1).FillWidth().Background(rgba(148, 163, 184, 0.16)).Margin(4, 0, 0, 0)
			ui.Text(c, locales.TasksPickFoot()).FontSize(10.5).FontWeight(500).TextColor(ink.Alpha(0.55)).Padding(6, 8, 2, 8)
		})
	})
}

// ---- settings ----

// settingsTasks is the settings of the task boxes, in the Editor's section, as BetterTask's page: the presets as
// tiles, the switches, the statuses as cards, and the custom statuses, each with its tag's name.
func (a *app) settingsTasks(c *ui.Context) {
	cl := ryugo.From(c).Colors
	settingGroup(c, "set-task-look", func() {
		ui.Column(c).Gap(14).Children(func() {
			settingTitle(c, cl, msg("tasks_look"))
			ui.Row(c).Gap(14).Wrap().Children(func() {
				for _, l := range taskLooks {
					id := l.id
					if choiceTile(c, "tile-tasklook-"+strconv.Itoa(len(id)), msg(l.name), a.settings.TaskLook == id, func() { a.taskLookMock(c, id) }) {
						a.settings.TaskLook = id
						a.saveSettings()
					}
				}
			})
			ui.Row(c).Gap(10).Wrap().Children(func() {
				card := func(key string, f taskFace, name, kbd string) {
					ui.Column(c.Key(key)).Width(146).Gap(10).Padding(12).Radius(12).Border(1, rgba(148, 163, 184, 0.16)).
						Background(rgba(148, 163, 184, 0.04)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Box(c).Size(20, 20).Draw(func(p *ui.Painter, r ui.Rect) { a.drawFace(p, r, f, p.Now()) })
							ui.Text(c, kbd).FontSize(11).FontWeight(700).Font(monoStack).TextColor(cl.Muted).Padding(2, 6).Radius(6).
								Background(rgba(148, 163, 184, 0.14))
						})
						ui.Text(c, name).FontSize(13).FontWeight(650).TextColor(cl.Text).MaxLines(1)
					})
				}
				card("taskcard-open", taskFace{slot: -1, color: ui.Hex(taskDone)}, msg("common_task"), "[ ]")
				for _, k := range taskKinds {
					card("taskcard-"+k.id, taskFace{id: k.id, slot: -1, color: ui.Hex(k.color)}, msg(k.name), "["+k.md+"]")
				}
			})
		})
	})
	toggle := func(key, title, help string, v *bool, invert bool) func() {
		return func() {
			on := *v != invert
			settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(msg(title)) })
			if on != (*v != invert) {
				*v = on != invert
				a.saveSettings()
			}
		}
	}
	settingGroup(c, "set-task-fx",
		toggle("set-task-motion", "tasks_motion", "tasks_motionHelp", &a.settings.TaskStill, true),
		toggle("set-task-burst", "tasks_burst", "tasks_burstHelp", &a.settings.TaskNoFX, true),
		toggle("set-task-tint", "tasks_tint", "tasks_tintHelp", &a.settings.TaskNoTint, true),
		toggle("set-task-counts", "tasks_counts", "tasks_countsHelp", &a.settings.TaskNoCounts, true),
	)
	settingGroup(c, "set-task-tags", func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				settingTitle(c, cl, msg("tasks_tagStatuses"))
				ui.Text(c, msg("tasks_tagStatusesHelp")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			})
			ui.Row(c).Gap(10).Wrap().Children(func() {
				for i, s := range tagStatusSlots {
					i := i
					ui.Row(c.Key("tagslot" + strconv.Itoa(i))).Gap(8).AlignItems(ui.Center).Width(226).Children(func() {
						f := taskFace{id: "tag", slot: i, color: ui.Hex(s.color)}
						ui.Box(c).Size(18, 18).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) { a.drawFace(p, r, f, p.Now()) })
						ui.Text(c, msg(tagTitles[i])).FontSize(12).FontWeight(600).TextColor(cl.Text).Grow(1).MaxLines(1)
						v := "" // what was typed: empty, the slot's own name, shown faint
						if i < len(a.settings.TaskTags) {
							v = a.settings.TaskTags[i]
						}
						ui.Row(c).AlignItems(ui.Center).Gap(2).Padding(0, 8).Radius(7).Border(1, rgba(148, 163, 184, 0.24)).
							Background(rgba(148, 163, 184, 0.06)).Children(func() {
							ui.Text(c, "#").FontSize(12).Font(monoStack).TextColor(cl.Muted)
							in := ui.TextInputBase(c.Key("tagslot-in"+strconv.Itoa(i)), &v).Label(locales.TasksTagSlot(i+1)).FontSize(12).
								Font(monoStack).Placeholder(s.name).PlaceholderColor(cl.Muted).Padding(5, 0).Width(92).FocusRing(false)
							if in.Changed() {
								for len(a.settings.TaskTags) < len(tagStatusSlots) {
									a.settings.TaskTags = append(a.settings.TaskTags, "")
								}
								a.settings.TaskTags[i] = strings.TrimPrefix(strings.TrimSpace(v), "#")
								a.saveSettingsSoon()
							}
						})
					})
				}
			})
		})
	})
}

// taskLookMock is a preset's tile: a few tasks, each status in that preset.
func (a *app) taskLookMock(c *ui.Context, look string) {
	cl := ryugo.From(c).Colors
	ui.Box(c).Width(112).Height(66).Background(cl.Card).Draw(func(p *ui.Painter, r ui.Rect) {
		was := a.settings.TaskLook
		a.settings.TaskLook = look
		defer func() { a.settings.TaskLook = was }()
		faces := []taskFace{{id: "progress", slot: -1}, {id: "important", slot: -1}, {id: "tag", slot: 6}, {id: "star", slot: -1}}
		for i, f := range faces {
			at := ui.Rect{X: r.X + 12 + float32(i%2)*50, Y: r.Y + 12 + float32(i/2)*26, W: 14, H: 14}
			a.drawFace(p, at, f, p.Now())
			p.Fill(ui.Rect{X: at.X + 20, Y: at.Y + 5, W: 22, H: 4}, cl.Text.Alpha(0.3), 2)
		}
	})
}
