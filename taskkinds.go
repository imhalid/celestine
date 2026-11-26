package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Kinds of task: a task can be a star, important, a question, in progress or an error, each with its own
// box - its color and its mark - besides the plain one. richtext has one kind of task, checked or not,
// so the kind is the paragraph's ID (taskIDPrefix and its id), which stays with it as it is edited and
// is kept in the page's body; in Markdown it is the character in the box, as Obsidian has them:
// "- [*] ", "- [!] ", "- [?] ", "- [/] ", "- [e] ". A task done is "[x]" in Markdown, whatever its kind;
// in the page it keeps its kind, and unchecked it is that kind again.
//
// The boxes are drawn over richtext's own (drawTasks): every task's, so that each turns the same way.
// Checked, a box turns over on its X axis - its face goes, its back comes, done - and confetti bursts
// from it; unchecked it turns back, without the confetti.

const taskIDPrefix = "celestine:task:"

type taskKind struct {
	id, name string
	md       string // the character in its Markdown box
	color    string
	icon     *ui.SVG // in the menus
}

var taskKinds = []taskKind{
	{"star", "starredTask", "*", "#F5B301", icon(`<path d="m12 4.5 2.3 4.7 5.2.8-3.8 3.7.9 5.2-4.6-2.4-4.6 2.4.9-5.2-3.8-3.7 5.2-.8z"/>`)},
	{"important", "importantTask", "!", "#F0506E", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="M12 8v5"/><circle cx="12" cy="16" r=".6"/>`)},
	{"question", "question", "?", "#8B5CF6", icon(`<path d="M4.5 19.5V8a3.5 3.5 0 0 1 3.5-3.5h8A3.5 3.5 0 0 1 19.5 8v8a3.5 3.5 0 0 1-3.5 3.5z"/><path d="M10 10a2 2 0 1 1 2.9 1.8c-.6.3-.9.8-.9 1.4v.3"/><circle cx="12" cy="16.2" r=".6"/>`)},
	{"progress", "taskInProgress", "/", "#06B6D4", icon(`<circle cx="12" cy="12" r="7.5"/><path d="m10.5 9 4 3-4 3z"/>`)},
	{"blocked", "tasks_blocked", "w", "#F59E0B", icon(`<rect x="4" y="7" width="16" height="10" rx="5" stroke-dasharray="2.5 2"/><path d="M10.5 10v4M13.5 10v4"/>`)},
	{"billable", "tasks_billable", "$", "#34D399", icon(`<circle cx="12" cy="12" r="7.5"/><path d="M14.3 9.6c-.4-.8-1.3-1.2-2.3-1.2-1.3 0-2.3.7-2.3 1.7 0 2.4 4.8 1.2 4.8 3.7 0 1-1.1 1.8-2.5 1.8-1.1 0-2-.5-2.4-1.3M12 7v10"/>`)},
	{"error", "failedTask", "e", "#F97316", icon(`<path d="M12 4.5 20 18.5H4z"/><path d="M12 10v4"/><circle cx="12" cy="16.3" r=".6"/>`)},
	{"canceled", "tasks_canceled", "-", "#94A3B8", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="m8 16 8-8"/>`)},
}

func init() {
	// the kinds are kinds of line too: in the / menu and the block's "turn into", after the plain task
	for i, k := range blockKinds {
		if k.kind != "tasks" {
			continue
		}
		var more []struct{ kind, name, md string }
		for _, tk := range taskKinds {
			more = append(more, struct{ kind, name, md string }{"task:" + tk.id, tk.name, "[" + tk.md + "]"})
			kindIcon["task:"+tk.id] = tk.icon
			formatKinds["task:"+tk.id] = true
		}
		blockKinds = append(blockKinds[:i+1], append(more, blockKinds[i+1:]...)...)
		break
	}
}

// taskKindOf is the kind of the task of style st, or false for a plain task, or for a paragraph not a task.
func taskKindOf(st richtext.ParagraphStyle) (taskKind, bool) {
	if st.List != richtext.Tasks {
		return taskKind{}, false
	}
	id, ok := strings.CutPrefix(st.ID, taskIDPrefix)
	if !ok {
		return taskKind{}, false
	}
	for _, k := range taskKinds {
		if k.id == id {
			return k, true
		}
	}
	return taskKind{}, false
}

// taskKindByID is the kind of id, or false.
func taskKindByID(id string) (taskKind, bool) {
	for _, k := range taskKinds {
		if k.id == id {
			return k, true
		}
	}
	return taskKind{}, false
}

// taskKindByMD is the kind whose Markdown box has character c, or false.
func taskKindByMD(c string) (taskKind, bool) {
	for _, k := range taskKinds {
		if k.md == c {
			return k, true
		}
	}
	return taskKind{}, false
}

// setTaskKind gives the task holding rune at the kind id; "" makes it a plain task.
func (a *app) setTaskKind(at int, id string) {
	t := a.pane().doc.Text()
	s, _ := t.ParagraphRange(at)
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) {
			p.List, p.ID = richtext.Tasks, ""
			if id != "" {
				p.ID = taskIDPrefix + id
			}
		})
	})
}

// ---- Markdown ----

// taskBoxLine is an item of a list in Markdown with a box: what comes before the box, and what is in it.
// listItemLine is a line that starts an item of a list.
var listItemLine = regexp.MustCompile(`^\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s`)

var taskBoxLine = regexp.MustCompile(`^(\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s+)\[(.)\](\s|$)`)

// taskKindsIn takes the kinds out of the boxes of md's lines, which richtext reads as plain ones,
// and gives them back in the order of the tasks: "" for a plain one or a done one.
func taskKindsIn(lines []string) []string {
	var kinds []string
	inCode, prevBlank, prevList := false, true, false
	for i, l := range lines {
		// an indented block of code (four spaces after a blank line, not under a list item) is code: a box in it is
		// text, not to be rewritten, or the count of tasks differs and every task loses its kind (#131)
		if strings.TrimSpace(l) == "" {
			prevBlank = true
			continue
		}
		indent := len(strings.ReplaceAll(l, "\t", "    ")) - len(strings.TrimLeft(strings.ReplaceAll(l, "\t", "    "), " "))
		switch {
		case inCode && indent >= 4:
			prevBlank = false
			continue
		case !inCode && indent >= 4 && prevBlank && !prevList:
			inCode, prevBlank = true, false
			continue
		}
		inCode = false
		prevList = listItemLine.MatchString(l) || prevList && indent >= 2
		prevBlank = false
		m := taskBoxLine.FindStringSubmatchIndex(l)
		if m == nil {
			continue
		}
		c := l[m[4]:m[5]]
		switch c {
		case " ", "x", "X":
			kinds = append(kinds, "")
		default:
			k, ok := taskKindByMD(c)
			if !ok {
				continue // not a box richtext takes: no task
			}
			kinds = append(kinds, k.id)
			lines[i] = l[:m[4]] + " " + l[m[5]:]
		}
	}
	return kinds
}

// withTaskKinds gives t's tasks, in order, the kinds read from their Markdown: only when there are as
// many, or one of them could land on another task.
func withTaskKinds(t richtext.Text, kinds []string) richtext.Text {
	has := false
	for _, k := range kinds {
		has = has || k != ""
	}
	if !has {
		return t
	}
	var tasks []int
	for _, at := range paraStarts(t) {
		if t.ParagraphStyleAt(at).List == richtext.Tasks {
			tasks = append(tasks, at)
		}
	}
	if len(tasks) != len(kinds) {
		return t
	}
	for i, at := range tasks {
		if kinds[i] == "" {
			continue
		}
		id := taskIDPrefix + kinds[i]
		t = t.FormatParagraphs(at, at, func(p *richtext.ParagraphStyle) { p.ID = id })
	}
	return t
}

// paraStarts is where t's paragraphs start, first first (starts, in markdown.go, has them last first).
func paraStarts(t richtext.Text) []int {
	var out []int
	for at := range t.Paragraphs() {
		out = append(out, at)
	}
	return out
}

// markTaskKinds writes the kinds of t's tasks in their boxes in md's lines, as richtext wrote them plain.
func markTaskKinds(t richtext.Text, lines []string) {
	var kinds []string
	has := false
	for _, para := range t.Paragraphs() { // their styles as they come, not asked one by one (#504)
		st := para.Style
		if st.List != richtext.Tasks {
			continue
		}
		k, ok := taskKindOf(st)
		has = has || ok
		if ok && !st.Checked {
			kinds = append(kinds, k.md)
		} else {
			kinds = append(kinds, "")
		}
	}
	if !has { // no task of a kind: nothing to write
		return
	}
	var at []int
	for i, l := range lines {
		if m := taskBoxLine.FindStringSubmatchIndex(l); m != nil && (l[m[4]:m[5]] == " " || l[m[4]:m[5]] == "x") {
			at = append(at, i)
		}
	}
	if len(at) != len(kinds) {
		return
	}
	for i, li := range at {
		if kinds[i] != "" {
			m := taskBoxLine.FindStringSubmatchIndex(lines[li])
			lines[li] = lines[li][:m[4]] + kinds[i] + lines[li][m[5]:]
		}
	}
}

// ---- drawn ----

const (
	taskFlip     = 520 * time.Millisecond
	taskConfetti = 800 * time.Millisecond
	taskStrike   = 420 * time.Millisecond // the line drawn through a task done, left to right (#494)
)

// taskFx is what the boxes remember between frames: the state each task was last seen in, to know
// when one is checked, and what turns and flies.
type taskFx struct {
	doc   *richtext.Document
	n     int          // the text's length when they were seen: a check changes no text
	seen  map[int]bool // checked, by the paragraph's start
	flips map[int]taskTurn
	bits  []confettiBit
	// striking are the tasks just checked whose line is being drawn through them, by paragraph (from 0): richtext's
	// strike is held back and the text fades to muted as the line passes (editorSpans, drawStrike); ver changes
	// while they draw, so that the editor asks for their spans again
	striking map[int]time.Time
	strikeK  map[int]float32 // how far each one's line is, as last drawn
	ver      int
	pick     taskPick // the picker of a task's status (taskstyle.go)
}

type taskTurn struct {
	t0      time.Time
	checked bool // what it turns to
	color   ui.Color
	burst   bool // its confetti has gone
}

type confettiBit struct {
	x, y, vx, vy, rot, vr, w, h float64 // in the window, DIPs
	color                       ui.Color
	t0                          time.Time
	shape                       int // a strip of paper, a star, or a round glint (burstOf)
}

// taskBoxAt is where the box of the task at paragraph box b is, in the editor's element, as richtext
// draws it (paintMarker); false for a heading's task, whose font is another, or out of view.
func (a *app) taskBoxAt(pn *paneState, b richtext.ParagraphBox, st richtext.ParagraphStyle) (ui.Rect, bool) {
	if st.Heading > 0 || a.theme == nil {
		return ui.Rect{}, false
	}
	s, e, para := a.paraOfBox(pn, b) // from the text's table, not read from its start (#314)
	cb, ok := locateIn(pn.doc, b.Start, b, s, e, para, pn.editorR.W, a.font, a.theme)
	if !ok {
		return ui.Rect{}, false
	}
	size := a.font.Size
	if a.markedTask(pn, b.Index, st) { // a bullet or a number before it: the box at the end of the room (listmarks.go)
		return markedTaskBox(cb.full.X, cb.full, size), true
	}
	indent := size * a.theme.Indent
	box := size * 0.95
	x := cb.full.X - indent + (indent-box)/2 - indent*0.1
	y := cb.full.Y + (cb.full.H-box)/2
	return ui.Rect{X: x, Y: y, W: box, H: box}, true
}

// drawTasks draws every task's box in view over richtext's, turning those just checked or unchecked.
func (a *app) drawTasks(pn *paneState, p *ui.Painter, r ui.Rect) {
	fx := &a.tasks
	text := pn.doc.Text()
	now := p.Now()
	same := fx.doc == pn.doc && fx.n == text.Len()
	if !same || fx.seen == nil {
		fx.doc, fx.n, fx.seen, fx.flips = pn.doc, text.Len(), map[int]bool{}, map[int]taskTurn{}
	}
	look := a.theme
	busy, moving := false, false
	for _, b := range a.shown(pn, r) {
		if b.Start > text.Len() {
			continue
		}
		_, _, para := a.paraOfBox(pn, b) // from the text's table, not read from its start (#312)
		st := para.Style
		if st.List != richtext.Tasks {
			continue
		}
		box, ok := a.taskBoxAt(pn, b, st)
		if !ok {
			continue
		}
		box.X, box.Y = box.X+r.X, box.Y+r.Y
		s, e, _ := a.paraOfBox(pn, b)
		face := a.faceOf(st, paraLine(text, s, e))
		color := face.color
		if was, ok := fx.seen[b.Start]; ok && same && was != st.Checked {
			fx.flips[b.Start] = taskTurn{t0: now, checked: st.Checked, color: color}
			if st.Checked {
				// the confetti with the click, not half a turn after it, which felt late (#494)
				fx.flips[b.Start] = taskTurn{t0: now, checked: true, color: color, burst: true}
				if !a.settings.TaskNoFX {
					a.burstConfetti(float64(box.X+box.W/2), float64(box.Y+box.H/2), face, now)
				}
				if fx.striking == nil {
					fx.striking = map[int]time.Time{}
				}
				fx.striking[b.Index] = now
			} else {
				delete(fx.striking, b.Index)
			}
		}
		if t0, ok := fx.striking[b.Index]; ok {
			if k := float32(now.Sub(t0)) / float32(taskStrike); k < 1 && st.Checked {
				busy = true
				fx.ver++
				if fx.strikeK == nil {
					fx.strikeK = map[int]float32{}
				}
				fx.strikeK[b.Index] = max(k, 0)
				a.drawStrike(pn, p, r, b, k)
			} else {
				delete(fx.striking, b.Index)
				delete(fx.strikeK, b.Index)
				fx.ver++ // richtext's strike, back for good
			}
		}
		fx.seen[b.Start] = st.Checked
		if a.settings.TaskTint && !st.Checked && face.id != "" { // the row in its status's color, fading (#597)
			rowTint(p, box, r.W*0.55, color)
		}
		// richtext's box, covered
		p.Fill(ui.Rect{X: box.X - 2, Y: box.Y - 2, W: box.W + 4, H: box.H + 4}, a.pageBg, 0)
		back := st.Checked // what shows: its back (checked) or its face
		sy, grow := float32(1), float32(0)
		if turn, ok := fx.flips[b.Start]; ok {
			k := float64(now.Sub(turn.t0)) / float64(taskFlip)
			if k >= 1 {
				delete(fx.flips, b.Start)
			} else {
				busy = true
				e := easeInOut(k)
				ang := e * math.Pi
				sy = f32(math.Abs(math.Cos(ang)))
				grow = f32(math.Sin(ang)) * 0.22
				first := ang < math.Pi/2 // the side it turns from
				back = first != turn.checked
			}
		}
		w := box.W * (1 + grow)
		h := box.H * (1 + grow) * max(sy, 0.04)
		at := ui.Rect{X: box.X + (box.W-w)/2, Y: box.Y + (box.H-h)/2, W: w, H: h}
		if back {
			drawFaceBack(p, at, face)
		} else {
			a.drawFace(p, at, face, look.Muted, now)
			if face.id == "" || face.id == "blocked" {
				drawTaskPart(p, at, a.subtasksOf(pn).count[b.Index], color) // as far as its subtasks are done (subtasks.go)
			}
			if face.id != "" && a.settings.TaskMotion && !stillNow() {
				moving = true
			}
		}
	}
	if moving { // the ambient motions: a slow pace is enough for them
		p.AnimationFrameRate(30)
	}
	if busy || len(fx.bits) > 0 {
		p.AnimationFrame()
	}
}

// drawStrike draws the line through the task of paragraph box b as far as k (0 to 1) of its lines' length, from the
// left, in the muted color of a task done; r is the editor's box.
func (a *app) drawStrike(pn *paneState, p *ui.Painter, r ui.Rect, b richtext.ParagraphBox, k float32) {
	s, e, para := a.paraOfBox(pn, b)
	if e-s > maxCaretParagraph {
		return
	}
	l, left := longMemo.layout(pn.doc, s, e-s, para, pn.editorR.W, a.font, a.theme)
	lines := l.Lines()
	total := float32(0)
	for _, ln := range lines {
		total += ln.Rect.W
	}
	reach := total * f32(easeInOut(float64(k)))
	thick := max(1, a.font.Size/14)
	for _, ln := range lines {
		if reach <= 0 {
			break
		}
		w := min(ln.Rect.W, reach)
		reach -= ln.Rect.W
		y := r.Y + b.Top + ln.Baseline - ln.Ascent*0.32 // through the middle of the small letters
		p.Fill(ui.Rect{X: r.X + left + ln.Rect.X, Y: y - thick/2, W: w, H: thick}, a.theme.Muted, thick/2)
	}
}

// strikeSpans is the spans of a task done whose line is still being drawn (drawStrike): no strike of richtext's yet,
// and the text fading from its color to the muted one as the line passes; false when it is not being drawn.
func (a *app) strikeSpans(p int, st richtext.ParagraphStyle, spans []ui.Span) ([]ui.Span, bool) {
	if _, ok := a.tasks.striking[p]; !ok || !st.Checked || a.theme == nil {
		return spans, false
	}
	ink := ryugo.Mix(a.theme.Text, a.theme.Muted, min(1, a.tasks.strikeK[p]))
	for i := range spans {
		spans[i].Strikethrough = false
		spans[i].Color = ink
	}
	return spans, true
}

func easeInOut(k float64) float64 {
	k = clamp01(k)
	return k * k * (3 - 2*k)
}

// burstConfetti throws confetti up from x, y (the window's), in the task's color and others.
func (a *app) burstConfetti(x, y float64, face taskFace, now time.Time) {
	colors, shape := burstOf(face) // each status its own bits and colors (#597)
	r := splitmix(uint64(now.UnixNano()))
	// a small burst, close to the box: a few bits that rise a little and fall, not a shower over the window (#494)
	for i := 0; i < 26; i++ {
		ang := -math.Pi/2 + r.in(-1.0, 1.0)
		sp := r.in(150, 330)
		a.tasks.bits = append(a.tasks.bits, confettiBit{
			x: x, y: y, vx: math.Cos(ang) * sp, vy: math.Sin(ang) * sp,
			rot: r.in(0, math.Pi), vr: r.in(-8, 8), w: r.in(4, 7), h: r.in(2.5, 4),
			color: colors[i%len(colors)], t0: now, shape: shape,
		})
	}
}

// drawConfetti draws the confetti in the air, over the window, and lets it fall.
func (a *app) drawConfetti(p *ui.Painter) {
	fx := &a.tasks
	if len(fx.bits) == 0 {
		return
	}
	if stillNow() { // reduced motion: no confetti
		fx.bits = fx.bits[:0]
		return
	}
	now := p.Now()
	keep := fx.bits[:0]
	for _, b := range fx.bits {
		t := now.Sub(b.t0).Seconds()
		life := taskConfetti.Seconds()
		if t >= life {
			continue
		}
		keep = append(keep, b)
		// thrown, slowed by the air, falling
		drag := 2.2
		k := (1 - math.Exp(-drag*t)) / drag
		x := b.x + b.vx*k
		y := b.y + b.vy*k + 0.5*820*t*t*0.55
		alpha := 1 - smooth(0.65, 1, t/life)
		spin := math.Abs(math.Cos(b.rot + b.vr*t)) // it turns as it falls: seen edge on, it narrows
		w, h := f32(b.w*(0.45+0.55*spin)), f32(b.h)
		col := b.color.Alpha(f32(alpha))
		switch b.shape {
		case 1: // a star
			r := f32(b.w) * 0.8
			sparkle(p, box{at: ui.Rect{X: f32(x), Y: f32(y), W: 1, H: 1}, s: 1, sy: 1}, 0, 0, r, col)
		case 2: // a glint
			d := f32(b.h) * 1.3
			p.Fill(ui.Rect{X: f32(x) - d/2, Y: f32(y) - d/2, W: d, H: d}, col, d/2)
		default:
			p.Fill(ui.Rect{X: f32(x) - w/2, Y: f32(y) - h/2, W: w, H: h}, col, 1)
		}
	}
	fx.bits = keep
	if len(keep) > 0 { // until the last bit has fallen
		p.AnimationFrame()
	}
}

// taskBoxHits puts a target over each task's box in view, over the window: richtext's editor takes the
// secondary button for its own menu (cut, copy, paste), so a right click on a box would never reach us.
// A click on one ticks the task, as the editor would; a right click opens the menu of its kinds.
func (a *app) taskBoxHits(c *ui.Context) {
	pn := a.pane()
	if pn.doc == nil || a.noPage() || pn.editorR.W == 0 || a.modal() || pn.grip.dragging {
		return
	}
	r := pn.editorR
	top, bottom := r.Y, r.Y+r.H // only where the page shows
	if pn.scrollR.H > 0 {
		top, bottom = max(top, pn.scrollR.Y+pn.barInset), min(bottom, pn.scrollR.Y+pn.scrollR.H)
	}
	text := pn.doc.Text()
	ui.Overlay(c, func() {
		for _, b := range a.shown(pn, r) {
			if b.Start > text.Len() {
				continue
			}
			_, _, para := a.paraOfBox(pn, b)
			st := para.Style
			if st.List != richtext.Tasks {
				continue
			}
			box, ok := a.taskBoxAt(pn, b, st)
			if !ok {
				continue
			}
			box.X, box.Y = box.X+r.X-3, box.Y+r.Y-3
			box.W, box.H = box.W+6, box.H+6
			if box.Y < top || box.Y+box.H > bottom {
				continue
			}
			at := b.Start
			hit := ui.Box(c.Key("taskbox:"+strconv.Itoa(at))).Absolute().Left(box.X).Top(box.Y).Size(box.W, box.H).
				Cursor(ui.CursorPointer).Role(ui.RoleCheckBox).Label(locales.CommonTask())
			if hit.Clicked() {
				if hit.ClickModifiers()&ui.Alt != 0 { // ⌥-click: the picker of its status (#597)
					b := hit.LayoutBounds()
					a.tasks.pick = taskPick{open: true, at: at, x: b.X, y: b.Y + b.H + 4, pn: pn}
				} else {
					pn.doc.ToggleChecked(at)
				}
			}
			if x, y, ok := rightClick(hit); ok {
				a.taskKindMenu(at, x, y)
			}
		}
	})
}

// taskKindMenu is the menu of the kinds a task can be, at x, y in the window: the one it is, ticked.
func (a *app) taskKindMenu(at int, x, y float32) {
	cur, _ := taskKindOf(a.pane().doc.Text().ParagraphStyleAt(at))
	items := []menuItem{{title: "common_task", glyph: kindIcon["tasks"], hint: tick(cur.id == ""), run: func() { a.setTaskKind(at, "") }}}
	for _, k := range taskKinds {
		id := k.id
		items = append(items, menuItem{title: k.name, glyph: k.icon, hint: tick(cur.id == id), run: func() { a.setTaskKind(at, id) }})
	}
	a.showContext(x, y, items)
}
