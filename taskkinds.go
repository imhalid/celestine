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
	{"star", "starredTask", "*", "#f59f00", icon(`<path d="m12 4.5 2.3 4.7 5.2.8-3.8 3.7.9 5.2-4.6-2.4-4.6 2.4.9-5.2-3.8-3.7 5.2-.8z"/>`)},
	{"important", "importantTask", "!", "#f76707", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="M12 8v5"/><circle cx="12" cy="16" r=".6"/>`)},
	{"question", "question", "?", "#7048e8", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="M10 10a2 2 0 1 1 2.9 1.8c-.6.3-.9.8-.9 1.4v.3"/><circle cx="12" cy="16.2" r=".6"/>`)},
	{"progress", "taskInProgress", "/", "#1c7ed6", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="M8 16 16 8"/>`)},
	{"error", "failedTask", "e", "#e03131", icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="m9 9 6 6M15 9l-6 6"/>`)},
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
	t := a.doc.Text()
	s, _ := t.ParagraphRange(at)
	a.doc.Edit(func(ed *richtext.Edit) {
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
}

// taskBoxAt is where the box of the task at paragraph box b is, in the editor's element, as richtext
// draws it (paintMarker); false for a heading's task, whose font is another, or out of view.
func (a *app) taskBoxAt(b richtext.ParagraphBox, st richtext.ParagraphStyle) (ui.Rect, bool) {
	if st.Heading > 0 || a.theme == nil {
		return ui.Rect{}, false
	}
	s, e, para := a.paraOfBox(b) // from the text's table, not read from its start (#314)
	cb, ok := locateIn(a.doc, b.Start, b, s, e, para, a.editorR.W, a.font, a.theme)
	if !ok {
		return ui.Rect{}, false
	}
	size := a.font.Size
	if a.markedTask(b.Index, st) { // a bullet or a number before it: the box at the end of the room (listmarks.go)
		return markedTaskBox(cb.full.X, cb.full, size), true
	}
	indent := size * a.theme.Indent
	box := size * 0.95
	x := cb.full.X - indent + (indent-box)/2 - indent*0.1
	y := cb.full.Y + (cb.full.H-box)/2
	return ui.Rect{X: x, Y: y, W: box, H: box}, true
}

// drawTasks draws every task's box in view over richtext's, turning those just checked or unchecked.
func (a *app) drawTasks(p *ui.Painter, r ui.Rect) {
	fx := &a.tasks
	text := a.doc.Text()
	now := p.Now()
	same := fx.doc == a.doc && fx.n == text.Len()
	if !same || fx.seen == nil {
		fx.doc, fx.n, fx.seen, fx.flips = a.doc, text.Len(), map[int]bool{}, map[int]taskTurn{}
	}
	th := p.Theme()
	look := a.theme
	busy := false
	for _, b := range a.shown(r) {
		if b.Start > text.Len() {
			continue
		}
		_, _, para := a.paraOfBox(b) // from the text's table, not read from its start (#312)
		st := para.Style
		if st.List != richtext.Tasks {
			continue
		}
		box, ok := a.taskBoxAt(b, st)
		if !ok {
			continue
		}
		box.X, box.Y = box.X+r.X, box.Y+r.Y
		kind, hasKind := taskKindOf(st)
		color := look.Checkbox
		if hasKind {
			color = ui.Hex(kind.color)
		}
		if was, ok := fx.seen[b.Start]; ok && same && was != st.Checked {
			fx.flips[b.Start] = taskTurn{t0: now, checked: st.Checked, color: color}
			if st.Checked {
				// the confetti with the click, not half a turn after it, which felt late (#494)
				fx.flips[b.Start] = taskTurn{t0: now, checked: true, color: color, burst: true}
				a.burstConfetti(float64(box.X+box.W/2), float64(box.Y+box.H/2), color, now)
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
				a.drawStrike(p, r, b, k)
			} else {
				delete(fx.striking, b.Index)
				delete(fx.strikeK, b.Index)
				fx.ver++ // richtext's strike, back for good
			}
		}
		fx.seen[b.Start] = st.Checked
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
			a.drawTaskBack(p, at, color, look.Checkmark)
		} else {
			a.drawTaskFace(p, at, kind, hasKind, look.Muted, th)
			drawTaskPart(p, at, a.subtasksOf().count[b.Index], color) // as far as its subtasks are done (subtasks.go)
		}
	}
	if busy || len(fx.bits) > 0 {
		p.AnimationFrame()
	}
}

// drawStrike draws the line through the task of paragraph box b as far as k (0 to 1) of its lines' length, from the
// left, in the muted color of a task done; r is the editor's box.
func (a *app) drawStrike(p *ui.Painter, r ui.Rect, b richtext.ParagraphBox, k float32) {
	s, e, para := a.paraOfBox(b)
	if e-s > maxCaretParagraph {
		return
	}
	l, left := longMemo.layout(a.doc, s, e-s, para, a.editorR.W, a.font, a.theme)
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

// drawTaskFace is a box not done: richtext's plain one, or the kind's, its mark in it. at may be squashed
// as it turns: the mark is squashed with it.
func (a *app) drawTaskFace(p *ui.Painter, at ui.Rect, kind taskKind, hasKind bool, muted ui.Color, th *ui.Theme) {
	s := at.W
	if !hasKind {
		p.Stroke(at, muted, s*0.25, 1.25)
		return
	}
	c := ui.Hex(kind.color)
	sy := at.H / at.W
	pt := func(x, y float32) (float32, float32) { return at.X + x*s, at.Y + y*s*sy }
	stroke := max(1.5, s*0.12)
	if kind.id == "star" { // a star, not a box
		var path ui.Path
		for i := 0; i < 10; i++ {
			r := float32(0.52)
			if i%2 == 1 {
				r = 0.22
			}
			a := -math.Pi/2 + float64(i)*math.Pi/5
			x, y := pt(0.5+r*f32(math.Cos(a)), 0.53+r*f32(math.Sin(a)))
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		path.Close()
		p.FillPath(&path, c.Alpha(0.2))
		p.StrokePath(&path, max(1.25, s*0.09), c)
		return
	}
	p.Fill(at, c.Alpha(0.14), s*0.25)
	p.Stroke(at, c, s*0.25, 1.4)
	line := func(x0, y0, x1, y1 float32) {
		var path ui.Path
		X0, Y0 := pt(x0, y0)
		X1, Y1 := pt(x1, y1)
		path.MoveTo(X0, Y0).LineTo(X1, Y1)
		p.StrokePath(&path, stroke, c)
	}
	dot := func(x, y float32) {
		X, Y := pt(x, y)
		r := stroke * 0.65
		p.Fill(ui.Rect{X: X - r, Y: Y - r*sy, W: 2 * r, H: 2 * r * sy}, c, r)
	}
	switch kind.id {
	case "important":
		line(0.5, 0.24, 0.5, 0.56)
		dot(0.5, 0.76)
	case "question":
		var path ui.Path
		x0, y0 := pt(0.33, 0.38)
		path.MoveTo(x0, y0)
		for i := 1; i <= 8; i++ { // the hook: a half circle and down
			a := math.Pi + float64(i)*math.Pi/8*1.25
			x, y := pt(0.5+0.17*f32(math.Cos(a)), 0.38+0.15*f32(math.Sin(a)))
			path.LineTo(x, y)
		}
		x1, y1 := pt(0.5, 0.6)
		path.LineTo(x1, y1)
		p.StrokePath(&path, stroke, c)
		dot(0.5, 0.78)
	case "progress": // half of it done
		p.Clip(ui.Rect{X: at.X, Y: at.Y, W: at.W / 2, H: at.H}, 0, func() { p.Fill(at, c, s*0.25) })
	case "error":
		line(0.3, 0.3, 0.7, 0.7)
		line(0.7, 0.3, 0.3, 0.7)
	}
}

// drawTaskBack is a box done: filled in its color, ticked.
func (a *app) drawTaskBack(p *ui.Painter, at ui.Rect, color, mark ui.Color) {
	s := at.W
	sy := at.H / at.W
	p.Fill(at, color, s*0.25)
	var path ui.Path
	path.MoveTo(at.X+s*0.25, at.Y+s*0.52*sy).LineTo(at.X+s*0.43, at.Y+s*0.7*sy).LineTo(at.X+s*0.76, at.Y+s*0.32*sy)
	p.StrokePath(&path, max(1.5, s*0.12), mark)
}

// burstConfetti throws confetti up from x, y (the window's), in the task's color and others.
func (a *app) burstConfetti(x, y float64, color ui.Color, now time.Time) {
	colors := []ui.Color{color, ui.Hex("#ff6b6b"), ui.Hex("#ffd43b"), ui.Hex("#51cf66"), ui.Hex("#339af0"), ui.Hex("#cc5de8"), color}
	r := splitmix(uint64(now.UnixNano()))
	// a small burst, close to the box: a few bits that rise a little and fall, not a shower over the window (#494)
	for i := 0; i < 26; i++ {
		ang := -math.Pi/2 + r.in(-1.0, 1.0)
		sp := r.in(150, 330)
		a.tasks.bits = append(a.tasks.bits, confettiBit{
			x: x, y: y, vx: math.Cos(ang) * sp, vy: math.Sin(ang) * sp,
			rot: r.in(0, math.Pi), vr: r.in(-8, 8), w: r.in(4, 7), h: r.in(2.5, 4),
			color: colors[i%len(colors)], t0: now,
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
		p.Fill(ui.Rect{X: f32(x) - w/2, Y: f32(y) - h/2, W: w, H: h}, b.color.Alpha(f32(alpha)), 1)
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
	if a.doc == nil || a.noPage() || a.editorR.W == 0 || a.modal() || a.grip.dragging {
		return
	}
	r := a.editorR
	top, bottom := r.Y, r.Y+r.H // only where the page shows
	if a.scrollR.H > 0 {
		top, bottom = max(top, a.scrollR.Y+a.barInset), min(bottom, a.scrollR.Y+a.scrollR.H)
	}
	text := a.doc.Text()
	ui.Overlay(c, func() {
		for _, b := range a.shown(r) {
			if b.Start > text.Len() {
				continue
			}
			_, _, para := a.paraOfBox(b)
			st := para.Style
			if st.List != richtext.Tasks {
				continue
			}
			box, ok := a.taskBoxAt(b, st)
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
				a.doc.ToggleChecked(at)
			}
			if x, y, ok := rightClick(hit); ok {
				a.taskKindMenu(at, x, y)
			}
		}
	})
}

// taskKindMenu is the menu of the kinds a task can be, at x, y in the window: the one it is, ticked.
func (a *app) taskKindMenu(at int, x, y float32) {
	cur, _ := taskKindOf(a.doc.Text().ParagraphStyleAt(at))
	items := []menuItem{{title: "common_task", glyph: kindIcon["tasks"], hint: tick(cur.id == ""), run: func() { a.setTaskKind(at, "") }}}
	for _, k := range taskKinds {
		id := k.id
		items = append(items, menuItem{title: k.name, glyph: k.icon, hint: tick(cur.id == id), run: func() { a.setTaskKind(at, id) }})
	}
	a.showContext(x, y, items)
}
