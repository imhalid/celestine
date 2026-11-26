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

// The look of the task boxes (#597): each status a shape of its own, drawn here in vectors, in one of three presets
// (Neon Glass, Vivid Solid, Minimal Crisp), legible in light and dark. Besides the kinds of taskkinds.go, a task can
// have a status by a tag in its line (#moon, #bug…): seven slots whose names the settings set, each with a drawing of
// its own; the tag stays a plain tag in the file, found and synced as any. A task done keeps its status's color and
// shape, a white tick in it. Optional, in the settings: the boxes' small ambient motions (never with reduced motion),
// the burst as a task is done (its shape and colors the status's), a tint along the row of a task with a status, and
// the counts of a task's subtasks.

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

// tagStatusSlots are the seven statuses a tag gives, by slot: the name each has unless the settings rename it, and
// its color. A slot's drawing stays with it whatever it is named.
var tagStatusSlots = [7]struct{ name, color string }{
	{"moon", "#6366F1"}, {"kawaii", "#EC4899"}, {"flower", "#FB7185"}, {"deadline", "#EF4444"},
	{"draft", "#64748B"}, {"bug", "#22C55E"}, {"sun", "#F59E0B"},
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
	return f.id == "important" || f.id == "error" || f.id == "tag" && (f.slot == 3 || f.slot == 5)
}

// paraLine is the text of the paragraph starting at s and ending at e in t.
func paraLine(t richtext.Text, s, e int) string { return t.Slice(s, e).String() }

// ---- drawn ----

// box is a task's box as it is drawn, squashed (sy) as it turns: what its parts are placed in.
type box struct {
	at     ui.Rect
	s, sy  float32
	glyph  ui.Color // what is drawn in it: the status's color, white on a solid one
	look   string
	motion float64 // seconds, for the ambient motions; -1 for none
}

// pt is the point at x, y of the box, in fractions of it.
func (b box) pt(x, y float32) (float32, float32) { return b.at.X + x*b.s, b.at.Y + y*b.s*b.sy }

// stroke is the width of a glyph's lines.
func (b box) stroke() float32 { return max(1.4, b.s*0.11) }

// line draws a glyph's line from x0, y0 to x1, y1, in fractions of the box.
func (b box) line(p *ui.Painter, x0, y0, x1, y1 float32, c ui.Color) {
	var path ui.Path
	X0, Y0 := b.pt(x0, y0)
	X1, Y1 := b.pt(x1, y1)
	path.MoveTo(X0, Y0).LineTo(X1, Y1)
	p.StrokePath(path.Cap(ui.RoundCap), b.stroke(), c)
}

// dot draws a round dot at x, y of radius r, in fractions of the box.
func (b box) dot(p *ui.Painter, x, y, r float32, c ui.Color) {
	X, Y := b.pt(x, y)
	R := r * b.s
	p.Fill(ui.Rect{X: X - R, Y: Y - R*b.sy, W: 2 * R, H: 2 * R * b.sy}, c, R)
}

// circle is a path of a circle at x, y of radius r, in fractions of the box, squashed with it.
func (b box) circle(x, y, r float32) *ui.Path {
	var path ui.Path
	for i := 0; i <= 32; i++ {
		ang := float64(i) / 32 * 2 * math.Pi
		X, Y := b.pt(x+r*f32(math.Cos(ang)), y+r*f32(math.Sin(ang)))
		if i == 0 {
			path.MoveTo(X, Y)
		} else {
			path.LineTo(X, Y)
		}
	}
	return path.Close()
}

// poly is a closed path through points in fractions of the box.
func (b box) poly(pts ...[2]float32) *ui.Path {
	var path ui.Path
	for i, q := range pts {
		X, Y := b.pt(q[0], q[1])
		if i == 0 {
			path.MoveTo(X, Y)
		} else {
			path.LineTo(X, Y)
		}
	}
	return path.Close()
}

// arc is a path along a circle at x, y of radius r, from angle from to to (radians, clockwise from the right).
func (b box) arc(x, y, r float32, from, to float64) *ui.Path {
	var path ui.Path
	steps := max(4, int(math.Abs(to-from)/0.15))
	for i := 0; i <= steps; i++ {
		ang := from + (to-from)*float64(i)/float64(steps)
		X, Y := b.pt(x+r*f32(math.Cos(ang)), y+r*f32(math.Sin(ang)))
		if i == 0 {
			path.MoveTo(X, Y)
		} else {
			path.LineTo(X, Y)
		}
	}
	return &path
}

// shell draws a status's shape in the preset: translucent with a lit rim (Neon Glass), filled (Vivid Solid), or its
// outline alone (Minimal Crisp). shape is "square", "circle", "pill", "triangle", "bubble" or "".
func (b box) shell(p *ui.Painter, shape string, c ui.Color) {
	fill, rim, glow := c.Alpha(0.16), c, c.Alpha(0.22)
	switch b.look {
	case taskLookVivid:
		fill, rim, glow = c, c, ui.Color{}
	case taskLookMinimal:
		fill, glow = ui.Color{}, ui.Color{}
	}
	r := b.s * 0.25
	switch shape {
	case "square":
		if glow.A > 0 {
			g := ui.Rect{X: b.at.X - 1.5, Y: b.at.Y - 1.5, W: b.at.W + 3, H: b.at.H + 3}
			p.Stroke(g, glow, r+1.5, 2)
		}
		if fill.A > 0 {
			p.Fill(b.at, fill, r)
		}
		p.Stroke(b.at, rim, r, 1.4)
	case "pill":
		if fill.A > 0 {
			p.Fill(b.at, fill, b.at.H/2)
		}
		p.StrokeDashed(b.at, rim, b.at.H/2, 1.4)
	case "circle", "triangle", "bubble":
		var path *ui.Path
		switch shape {
		case "circle":
			path = b.circle(0.5, 0.5, 0.52)
		case "triangle":
			path = b.poly([2]float32{0.5, -0.02}, [2]float32{1.06, 0.96}, [2]float32{-0.06, 0.96})
		default: // a speech bubble: three round corners and a sharp one at the bottom left
			p.FillRadii(b.at, [4]float32{r, r, r, 0}, fill)
			p.Stroke(b.at, rim, r*0.7, 1.4)
			return
		}
		if glow.A > 0 {
			p.StrokePath(path, 3.2, glow)
		}
		if fill.A > 0 {
			p.FillPath(path, fill)
		}
		p.StrokePath(path, 1.4, rim)
	}
}

// drawFace draws the face of a task not done.
func (a *app) drawFace(p *ui.Painter, at ui.Rect, f taskFace, muted ui.Color, now time.Time) {
	look := a.settings.TaskLook
	b := box{at: at, s: at.W, sy: at.H / at.W, glyph: f.color, look: look, motion: -1}
	if look == taskLookVivid {
		b.glyph = ui.Hex("#ffffff")
	}
	if a.settings.TaskMotion && !stillNow() {
		b.motion = float64(now.UnixNano()%int64(time.Hour)) / 1e9
	}
	wave := func(period float64) float64 { // 0 to 1 and back, over period seconds
		if b.motion < 0 {
			return 0
		}
		return 0.5 - 0.5*math.Cos(2*math.Pi*b.motion/period)
	}
	switch f.id {
	case "":
		if look == taskLookNeon {
			p.Fill(at, muted.Alpha(0.06), b.s*0.25)
			p.Fill(ui.Rect{X: at.X + b.s*0.25, Y: at.Y + 1, W: b.s * 0.5, H: 1}, ui.Hex("#ffffff").Alpha(0.25), 0.5) // its gloss
		}
		p.Stroke(at, muted, b.s*0.25, 1.25)
	case "progress": // Started: a play mark in a ring, a bright arc going round it
		b.shell(p, "circle", f.color)
		turn := 0.0
		if b.motion >= 0 {
			turn = b.motion * 2 * math.Pi / 1.6
		}
		p.StrokePath(b.arc(0.5, 0.5, 0.66, turn, turn+math.Pi/2).Cap(ui.RoundCap), 1.4, f.color.Alpha(0.85))
		p.FillPath(b.poly([2]float32{0.4, 0.3}, [2]float32{0.72, 0.5}, [2]float32{0.4, 0.7}), b.glyph)
	case "blocked": // Waiting: a dashed pill and two pause bars, breathing
		b.shell(p, "pill", f.color)
		h := 0.36 * (0.8 + 0.2*f32(wave(2.4)))
		for _, x := range []float32{0.39, 0.61} {
			b.line(p, x, 0.5-h/2, x, 0.5+h/2, b.glyph)
		}
	case "billable": // a coin with $, a light going over it
		c2 := ui.Hex("#A3E635")
		if look == taskLookMinimal {
			b.shell(p, "circle", f.color)
		} else {
			path := b.circle(0.5, 0.5, 0.52)
			p.FillPathGradient(path, ui.LinearGradient{Angle: 135, From: f.color.Alpha(map[bool]float32{true: 1, false: 0.35}[look == taskLookVivid]), To: c2.Alpha(map[bool]float32{true: 1, false: 0.25}[look == taskLookVivid])})
			p.StrokePath(path, 1.4, f.color)
		}
		b.line(p, 0.5, 0.2, 0.5, 0.8, b.glyph)
		var s ui.Path
		x0, y0 := b.pt(0.66, 0.34)
		s.MoveTo(x0, y0)
		c1x, c1y := b.pt(0.6, 0.24)
		m1x, m1y := b.pt(0.36, 0.28)
		mx, my := b.pt(0.38, 0.42)
		s.CubeTo(c1x, c1y, m1x, m1y, mx, my)
		c3x, c3y := b.pt(0.42, 0.52)
		c4x, c4y := b.pt(0.66, 0.5)
		ex, ey := b.pt(0.64, 0.64)
		s.CubeTo(c3x, c3y, c4x, c4y, ex, ey)
		c5x, c5y := b.pt(0.62, 0.78)
		c6x, c6y := b.pt(0.38, 0.76)
		fx, fy := b.pt(0.34, 0.66)
		s.CubeTo(c5x, c5y, c6x, c6y, fx, fy)
		p.StrokePath(s.Cap(ui.RoundCap), b.stroke()*0.9, b.glyph)
		if b.motion >= 0 && look != taskLookMinimal { // the sheen, now and then
			if k := math.Mod(b.motion, 3.2) / 0.8; k < 1 {
				p.ClipPath(b.circle(0.5, 0.5, 0.52), func() {
					x := at.X - b.s*0.5 + f32(k)*b.s*2
					var band ui.Path
					band.MoveTo(x, at.Y-b.s).LineTo(x+b.s*0.18, at.Y-b.s).LineTo(x-b.s*0.4, at.Y+at.H+b.s).LineTo(x-b.s*0.58, at.Y+at.H+b.s).Close()
					p.FillPath(&band, ui.Hex("#ffffff").Alpha(0.45))
				})
			}
		}
	case "important": // a square with !, beating
		k := 1 + 0.07*f32(math.Pow(wave(1.1), 6))
		bb := b
		bb.at = ui.Rect{X: at.X - at.W*(k-1)/2, Y: at.Y - at.H*(k-1)/2, W: at.W * k, H: at.H * k}
		bb.s = bb.at.W
		bb.shell(p, "square", f.color)
		bb.line(p, 0.5, 0.24, 0.5, 0.56, b.glyph)
		bb.dot(p, 0.5, 0.76, 0.075, b.glyph)
	case "question": // a speech bubble with ?, tilting a little
		b.shell(p, "bubble", f.color)
		tilt := 0.14 * (wave(3) - 0.5)
		rot := func(x, y float32) (float32, float32) {
			c, s := f32(math.Cos(tilt)), f32(math.Sin(tilt))
			x, y = x-0.5, y-0.5
			return 0.5 + x*c - y*s, 0.5 + x*s + y*c
		}
		var path ui.Path
		for i := 0; i <= 8; i++ { // the hook: a half circle and down
			ang := math.Pi + float64(i)*math.Pi/8*1.25
			x, y := rot(0.5+0.17*f32(math.Cos(ang)), 0.38+0.15*f32(math.Sin(ang)))
			X, Y := b.pt(x, y)
			if i == 0 {
				path.MoveTo(X, Y)
			} else {
				path.LineTo(X, Y)
			}
		}
		X, Y := b.pt(rot(0.5, 0.6))
		path.LineTo(X, Y)
		p.StrokePath(path.Cap(ui.RoundCap), b.stroke(), b.glyph)
		x, y := rot(0.5, 0.79)
		b.dot(p, x, y, 0.07, b.glyph)
	case "error": // Alert: a hazard triangle with !, a wave going out of it
		red := ui.Hex("#EF4444")
		tri := b.poly([2]float32{0.5, -0.02}, [2]float32{1.06, 0.96}, [2]float32{-0.06, 0.96})
		switch look {
		case taskLookMinimal:
			p.StrokePath(tri, 1.4, f.color)
		default:
			op := map[bool]float32{true: 1, false: 0.3}[look == taskLookVivid]
			p.FillPathGradient(tri, ui.LinearGradient{Angle: 180, From: f.color.Alpha(op), To: red.Alpha(op)})
			p.StrokePath(tri, 1.4, f.color)
		}
		if b.motion >= 0 {
			k := f32(math.Mod(b.motion, 2) / 2)
			g := 1 + k*0.6
			out := b.poly([2]float32{0.5, 0.47 - 0.49*g}, [2]float32{0.5 + 0.56*g, 0.47 + 0.49*g}, [2]float32{0.5 - 0.56*g, 0.47 + 0.49*g})
			p.StrokePath(out, 1, f.color.Alpha(0.5*(1-k)))
		}
		b.line(p, 0.5, 0.34, 0.5, 0.62, b.glyph)
		b.dot(p, 0.5, 0.8, 0.07, b.glyph)
	case "star": // a gold orb with a star, a sparkle at its corner
		b.shell(p, "circle", f.color)
		var path ui.Path
		for i := 0; i < 10; i++ {
			r := float32(0.3)
			if i%2 == 1 {
				r = 0.13
			}
			ang := -math.Pi/2 + float64(i)*math.Pi/5
			X, Y := b.pt(0.5+r*f32(math.Cos(ang)), 0.53+r*f32(math.Sin(ang)))
			if i == 0 {
				path.MoveTo(X, Y)
			} else {
				path.LineTo(X, Y)
			}
		}
		p.FillPath(path.Close(), b.glyph)
		tw := 0.6 + 0.4*f32(wave(1.8))
		if b.motion < 0 {
			tw = 1
		}
		sparkle(p, b, 0.98, 0.02, 0.16, f.color.Alpha(tw))
	case "canceled": // a frosted square, slashed
		c := f.color
		if look == taskLookNeon {
			p.Fill(at, c.Alpha(0.2), b.s*0.25)
			p.Stroke(at, c.Alpha(0.8), b.s*0.25, 1.25)
		} else {
			b.shell(p, "square", c)
		}
		b.line(p, 0.26, 0.74, 0.74, 0.26, b.glyph)
	case "tag":
		a.drawTagFace(p, b, f, wave)
	}
}

// sparkle is a four-pointed sparkle at x, y of size r, in fractions of the box.
func sparkle(p *ui.Painter, b box, x, y, r float32, c ui.Color) {
	p.FillPath(b.poly([2]float32{x, y - r}, [2]float32{x + r*0.25, y - r*0.25}, [2]float32{x + r, y}, [2]float32{x + r*0.25, y + r*0.25},
		[2]float32{x, y + r}, [2]float32{x - r*0.25, y + r*0.25}, [2]float32{x - r, y}, [2]float32{x - r*0.25, y - r*0.25}), c)
}

// drawTagFace draws the drawing of a tag status's slot.
func (a *app) drawTagFace(p *ui.Painter, b box, f taskFace, wave func(float64) float64) {
	c, g := f.color, b.glyph
	switch f.slot {
	case 0: // moon: a crescent and two stars
		b.shell(p, "circle", c)
		p.ClipPath(b.circle(0.5, 0.5, 0.52), func() {
			p.FillPath(b.circle(0.46, 0.52, 0.28), g)
			p.FillPath(b.circle(0.58, 0.44, 0.24), map[bool]ui.Color{true: c, false: a.pageBg}[b.look == taskLookVivid])
		})
		sparkle(p, b, 0.76, 0.28, 0.09, g)
		sparkle(p, b, 0.7, 0.7, 0.06, g)
	case 1: // kawaii: a round face, its eyes and smile, rosy cheeks
		b.shell(p, "circle", c)
		blink := float32(1)
		if b.motion >= 0 && math.Mod(b.motion, 4) < 0.15 {
			blink = 0.25
		}
		for _, x := range []float32{0.36, 0.64} {
			X, Y := b.pt(x, 0.44)
			R := b.s * 0.06
			p.Fill(ui.Rect{X: X - R, Y: Y - R*blink*b.sy, W: 2 * R, H: 2 * R * blink * b.sy}, g, R)
		}
		p.StrokePath(b.arc(0.5, 0.52, 0.13, 0.2*math.Pi, 0.8*math.Pi).Cap(ui.RoundCap), b.stroke()*0.8, g)
		for _, x := range []float32{0.27, 0.73} {
			b.dot(p, x, 0.6, 0.06, ui.Hex("#FDA4AF").Alpha(0.9))
		}
	case 2: // flower: five petals round a heart
		turn := 0.0
		if b.motion >= 0 {
			turn = 0.2 * (wave(5) - 0.5)
		}
		fill := c.Alpha(map[bool]float32{true: 1, false: 0.35}[b.look == taskLookVivid])
		for i := 0; i < 5; i++ {
			ang := -math.Pi/2 + turn + float64(i)*2*math.Pi/5
			pc := b.circle(0.5+0.24*f32(math.Cos(ang)), 0.5+0.24*f32(math.Sin(ang)), 0.2)
			if b.look != taskLookMinimal {
				p.FillPath(pc, fill)
			}
			p.StrokePath(pc, 1.2, c)
		}
		b.dot(p, 0.5, 0.5, 0.11, ui.Hex("#FDE68A"))
	case 3: // deadline: an hourglass in a dashed ring
		ring := b.circle(0.5, 0.5, 0.54)
		ring.Dash(0, 2.2, 1.6)
		if b.look == taskLookVivid {
			p.FillPath(b.circle(0.5, 0.5, 0.54), c)
		} else if b.look == taskLookNeon {
			p.FillPath(b.circle(0.5, 0.5, 0.54), c.Alpha(0.14))
		}
		p.StrokePath(ring, 1.3, c)
		p.StrokePath(b.poly([2]float32{0.32, 0.24}, [2]float32{0.68, 0.24}, [2]float32{0.5, 0.5}), b.stroke()*0.8, g)
		p.StrokePath(b.poly([2]float32{0.5, 0.5}, [2]float32{0.68, 0.76}, [2]float32{0.32, 0.76}), b.stroke()*0.8, g)
		sand := 0.5 + 0.5*f32(wave(3))
		p.FillPath(b.poly([2]float32{0.5 - 0.12*sand, 0.76 - 0.18*sand}, [2]float32{0.5 + 0.12*sand, 0.76 - 0.18*sand}, [2]float32{0.66, 0.76}, [2]float32{0.34, 0.76}), g)
	case 4: // draft: a card with a pencil over it, asleep
		b.shell(p, "square", c)
		b.line(p, 0.26, 0.74, 0.62, 0.38, g)
		b.line(p, 0.62, 0.38, 0.72, 0.28, g.Alpha(0.6))
		zs := float32(0.11)
		up := f32(wave(2.6)) * 0.05
		p.StrokePath(b.poly([2]float32{0.66, 0.62 - up}, [2]float32{0.66 + zs, 0.62 - up}, [2]float32{0.66, 0.62 + zs - up}, [2]float32{0.66 + zs, 0.62 + zs - up}), 1, g)
	case 5: // bug: a beetle in a glowing ring
		ring := b.circle(0.5, 0.5, 0.54)
		if b.look != taskLookMinimal {
			p.StrokePath(ring, 3.5, c.Alpha(0.2+0.2*f32(wave(1.6))))
		}
		if b.look == taskLookVivid {
			p.FillPath(ring, c)
		}
		p.StrokePath(ring, 1.3, c)
		p.FillPath(b.circle(0.5, 0.56, 0.17), g)
		b.dot(p, 0.5, 0.32, 0.08, g)
		for _, y := range []float32{0.48, 0.6, 0.72} {
			b.line(p, 0.3, y-0.04, 0.38, y, g)
			b.line(p, 0.7, y-0.04, 0.62, y, g)
		}
	case 6: // sun: a disc and its rays, turning slowly
		turn := 0.0
		if b.motion >= 0 {
			turn = b.motion * 0.4
		}
		if b.look == taskLookVivid {
			p.FillPath(b.circle(0.5, 0.5, 0.54), c)
		}
		p.FillPath(b.circle(0.5, 0.5, 0.2), map[bool]ui.Color{true: g, false: c}[b.look == taskLookVivid])
		for i := 0; i < 8; i++ {
			ang := turn + float64(i)*math.Pi/4
			cs, sn := f32(math.Cos(ang)), f32(math.Sin(ang))
			b.line(p, 0.5+0.3*cs, 0.5+0.3*sn, 0.5+0.44*cs, 0.5+0.44*sn, map[bool]ui.Color{true: g, false: c}[b.look == taskLookVivid])
		}
	}
}

// shapeOf is the shape of a face's box, which its back keeps when the task is done.
func shapeOf(f taskFace) string {
	switch f.id {
	case "progress", "billable", "star":
		return "circle"
	case "blocked":
		return "pill"
	case "error":
		return "triangle"
	case "question":
		return "bubble"
	case "tag":
		if f.slot == 4 {
			return "square"
		}
		return "circle"
	}
	return "square"
}

// drawFaceBack is a task done: its status's shape filled with its color, a white tick in it.
func drawFaceBack(p *ui.Painter, at ui.Rect, f taskFace) {
	b := box{at: at, s: at.W, sy: at.H / at.W, look: taskLookVivid}
	b.shell(p, shapeOf(f), f.color)
	var path ui.Path
	x0, y0 := b.pt(0.25, 0.52)
	x1, y1 := b.pt(0.43, 0.7)
	x2, y2 := b.pt(0.76, 0.32)
	if f.id == "error" { // lower in a triangle
		y0, y1, y2 = y0+b.s*0.1*b.sy, y1+b.s*0.1*b.sy, y2+b.s*0.1*b.sy
	}
	path.MoveTo(x0, y0).LineTo(x1, y1).LineTo(x2, y2)
	p.StrokePath(path.Cap(ui.RoundCap).Join(ui.RoundJoin), max(1.5, b.s*0.12), ui.Hex("#ffffff"))
}

// rowTint draws the soft tint of a task's status along its row, from its box to the right, fading.
func rowTint(p *ui.Painter, at ui.Rect, width float32, c ui.Color) {
	r := ui.Rect{X: at.X - 4, Y: at.Y - at.H*0.22, W: width, H: at.H * 1.44}
	p.FillGradient(r, ui.LinearGradient{Angle: 90, From: c.Alpha(0.11), To: c.Alpha(0)}, at.H*0.4)
}

// burstColors are the colors of a status's burst as its task is done, and the shape of its bits: 0 a strip of
// paper, 1 a star, 2 a round glint.
func burstOf(f taskFace) ([]ui.Color, int) {
	switch {
	case f.id == "star":
		return []ui.Color{f.color, ui.Hex("#FDE68A"), ui.Hex("#ffffff")}, 1
	case f.id == "billable":
		return []ui.Color{ui.Hex("#FACC15"), ui.Hex("#A3E635"), f.color}, 2
	case f.id == "":
		return []ui.Color{f.color, ui.Hex("#6EE7B7"), ui.Hex("#34D399"), ui.Hex("#ffffff")}, 2
	}
	return []ui.Color{f.color, ui.Hex("#ff6b6b"), ui.Hex("#ffd43b"), ui.Hex("#51cf66"), ui.Hex("#339af0"), ui.Hex("#cc5de8")}, 0
}

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
	c := ui.Hex(tagStatusSlots[slot].color)
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
			return ui.Hex(tagStatusSlots[i].color), true
		}
	}
	return ui.Color{}, false
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

// taskPickerView draws the picker while it is open, over the window.
func (a *app) taskPickerView(c *ui.Context) {
	pk := &a.tasks.pick
	if !pk.open || pk.pn == nil || pk.pn.doc == nil {
		return
	}
	cl := ryugo.From(c).Colors
	ww, wh := c.Size()
	choose := func(id string, slot int) {
		a.setTaskStatus(pk.pn, pk.at, id, slot)
		pk.open = false
	}
	ui.Overlay(c, func() {
		shade := ui.Box(c.Key("taskpick-shade")).Absolute().Left(0).Top(0).Size(ww, wh)
		if shade.Clicked() {
			pk.open = false
		}
		x, y := min(pk.x, ww-300), min(pk.y, wh-170)
		panel := ui.Column(c.Key("taskpick")).Absolute().Left(max(8, x)).Top(max(8, y)).Width(292).Padding(10).Gap(8).
			Radius(rad(ryugo.RadiusPop)).Background(cl.Card).Border(1, cl.Hairline).Label(locales.TasksStatusPicker()).Focusable()
		ryugo.From(c).Shadows.ShadowPop.Apply(panel)
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
		cell := func(key, label, hint string, f taskFace) {
			e := ui.Column(c.Key(key)).Width(30).Gap(2).AlignItems(ui.Center).Padding(4, 0).Radius(rad(7)).
				Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer).Tooltip(label)
			if e.Hovered() {
				e.Background(cl.Fill)
			}
			e.Children(func() {
				ui.Box(c).Size(16, 16).Draw(func(p *ui.Painter, r ui.Rect) { a.drawFace(p, r, f, cl.Muted, p.Now()) })
				if hint != "" {
					ui.Text(c, hint).FontSize(9.5).TextColor(cl.Muted)
				}
			})
			if e.Clicked() {
				choose(f.id, f.slot)
			}
		}
		panel.Children(func() {
			ui.Row(c).Gap(4).Wrap().Children(func() {
				cell("tp-none", msg("common_task"), "0", taskFace{slot: -1})
				for _, k := range taskKinds {
					cell("tp-"+k.id, msg(k.name), "", taskFace{id: k.id, slot: -1, color: ui.Hex(k.color)})
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(cl.Hairline)
			names := a.tagNames()
			ui.Row(c).Gap(4).Wrap().Children(func() {
				for i, s := range tagStatusSlots {
					cell("tp-tag"+strconv.Itoa(i), "#"+names[i], strconv.Itoa(i+1), taskFace{id: "tag", slot: i, color: ui.Hex(s.color)})
				}
			})
		})
	})
}

// ---- settings ----

// settingsTasks is the settings of the task boxes, in the Editor's section: the presets as tiles, the switches, and
// the names of the tag statuses.
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
		toggle("set-task-motion", "tasks_motion", "tasks_motionHelp", &a.settings.TaskMotion, false),
		toggle("set-task-burst", "tasks_burst", "tasks_burstHelp", &a.settings.TaskNoFX, true),
		toggle("set-task-tint", "tasks_tint", "tasks_tintHelp", &a.settings.TaskTint, false),
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
					ui.Row(c.Key("tagslot" + strconv.Itoa(i))).Gap(6).AlignItems(ui.Center).Width(168).Children(func() {
						f := taskFace{id: "tag", slot: i, color: ui.Hex(s.color)}
						ui.Box(c).Size(18, 18).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) { a.drawFace(p, r, f, cl.Muted, p.Now()) })
						v := "" // what was typed: empty, the slot's own name, shown faint
						if i < len(a.settings.TaskTags) {
							v = a.settings.TaskTags[i]
						}
						in := ui.TextInputBase(c.Key("tagslot-in"+strconv.Itoa(i)), &v).Label(locales.TasksTagSlot(i+1)).FontSize(13).
							Placeholder(s.name).PlaceholderColor(cl.Muted).Padding(5, 8).Radius(rad(7)).Grow(1).FocusRing(false)
						fieldLook(in, cl, in.Focused())
						if in.Changed() {
							for len(a.settings.TaskTags) < len(tagStatusSlots) {
								a.settings.TaskTags = append(a.settings.TaskTags, "")
							}
							a.settings.TaskTags[i] = strings.TrimPrefix(strings.TrimSpace(v), "#")
							a.saveSettingsSoon()
						}
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
		faces := []taskFace{{id: "progress", slot: -1, color: ui.Hex(taskKinds[3].color)}, {id: "important", slot: -1, color: ui.Hex(taskKinds[1].color)},
			{id: "tag", slot: 0, color: ui.Hex(tagStatusSlots[0].color)}, {id: "star", slot: -1, color: ui.Hex(taskKinds[0].color)}}
		for i, f := range faces {
			at := ui.Rect{X: r.X + 12 + float32(i%2)*50, Y: r.Y + 12 + float32(i/2)*26, W: 14, H: 14}
			a.drawFace(p, at, f, cl.Muted, p.Now())
			p.Fill(ui.Rect{X: at.X + 20, Y: at.Y + 5, W: 22, H: 4}, cl.Text.Alpha(0.3), 2)
		}
	})
}
