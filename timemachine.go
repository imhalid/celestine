package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's history as Apple's Time Machine shows a folder's: the page steps back from the editor and becomes the
// front of a stack, the copies kept of it behind it, going off towards a spiral galaxy, each smaller and fainter.
// On the right, a timeline from Now at its foot up to the oldest copy, the days written beside it: a click or a drag
// along it, the tilted arrows under it, ↑ ↓, or a click on a card behind brings that copy to the front, the stack
// gliding, and one passed goes by towards the one looking and fades. Along the foot, a bar: Cancel, the day of the
// copy at the front, Restore. Restore makes the page what that copy is; Cancel or Esc goes back into the editor, the
// way it came. The differences, line by line, and keeping a copy by name are the version dialog's (historyui.go).

type timeMachine struct {
	open    bool
	closing bool
	page    *Page
	vers    []Version        // the copies, the latest first
	texts   map[int]string   // what each was, as Markdown, by its place: 0 the page now
	lines   map[int][]tmLine // each copy's text, read into lines once
	sel     int              // the copy at the front: 0 the page now, i the copy vers[i-1]
	at      time.Time        // when it opened: the galaxy turns from there
}

// tmLine is a line of a copy's text, as the card shows it.
type tmLine struct {
	text  string
	level int // a heading's, 0 for text
}

// tmShaders is false in the tests, whose CPU renderer draws the galaxy far too slowly.
var tmShaders = true

// openTimeMachine shows the history of page p.
func (a *app) openTimeMachine(p *Page) {
	h, ok := historyOf(p)
	if !ok {
		a.notice = "Sürüm geçmişi yalnız tek dosya kitaplığında tutulur"
		return
	}
	a.flush()
	a.tm = timeMachine{open: true, page: p, vers: h.Versions(p.File()), texts: map[int]string{}, lines: map[int][]tmLine{}, at: a.clock()}
}

// tmText is what the copy at place i was, as Markdown: 0 is the page now.
func (a *app) tmText(i int) string {
	t := &a.tm
	if s, ok := t.texts[i]; ok {
		return s
	}
	var s string
	if i == 0 {
		s, _ = t.page.Text()
	} else if h, ok := historyOf(t.page); ok {
		data, _ := h.VersionData(t.page.File(), t.vers[i-1].ID)
		_, body := SplitPage(data)
		s = bodyText(body)
	}
	t.texts[i] = s
	return s
}

// tmLines are the lines of the copy at place i, as a card shows them.
func (a *app) tmLines(i int) []tmLine {
	if ls, ok := a.tm.lines[i]; ok {
		return ls
	}
	var out []tmLine
	for _, l := range strings.Split(a.tmText(i), "\n") {
		l = strings.TrimRight(l, " ")
		if l == "" || strings.HasPrefix(l, blockMarkStart) {
			continue
		}
		n := 0
		for n < len(l) && n < 4 && l[n] == '#' {
			n++
		}
		if n > 0 && n < len(l) && l[n] == ' ' {
			out = append(out, tmLine{strings.TrimSpace(l[n:]), n})
			continue
		}
		out = append(out, tmLine{l, 0})
	}
	a.tm.lines[i] = out
	return out
}

// tmWhen is when the copy at place i was written: Now for the page; today's copies by the hour, as Today 14:05.
func (a *app) tmWhen(i int) string {
	if i == 0 {
		return T("Today (Now)")
	}
	at := a.tm.vers[i-1].At
	if sameDay(at, a.clock()) {
		return T("Today") + " " + at.Format("15:04")
	}
	return at.Format("2 Jan 2006 · 15:04")
}

// tmPutBack makes the page what the copy at the front is, as the version dialog does (the page now kept as a
// copy first), and goes back into the editor.
func (a *app) tmPutBack() {
	t := &a.tm
	if t.sel == 0 || t.sel > len(t.vers) {
		return
	}
	a.vhist = histView{page: t.page, vers: t.vers, sel: t.sel - 1, opsFor: -1}
	a.putBackVersion()
	t.texts, t.lines = map[int]string{}, map[int][]tmLine{} // the page now is the copy: the front card says so
	t.sel, t.closing = 0, true
}

// tmKeys takes the history's keys, early in the frame: before the editor's normal mode takes Esc, and the arrows
// move the caret; it says whether the history is open.
func (a *app) tmKeys(c *ui.Context) bool {
	t := &a.tm
	if !t.open {
		return false
	}
	if t.closing {
		return true
	}
	n := len(t.vers)
	switch {
	case c.Shortcut(0, ui.KeyEscape):
		t.closing = true
	case c.Shortcut(0, ui.KeyUp), c.Shortcut(0, ui.KeyLeft):
		t.sel = min(t.sel+1, n)
	case c.Shortcut(0, ui.KeyDown), c.Shortcut(0, ui.KeyRight):
		t.sel = max(t.sel-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		a.tmPutBack()
	}
	return true
}

// The layout of the history, in the window: the bar along the foot, the timeline's room on the right.
const (
	tmBarH      = 46
	tmLineRight = 128
)

// timeMachineView draws the history over the window while it is open.
func (a *app) timeMachineView(c *ui.Context) {
	t := &a.tm
	if !t.open || t.page == nil {
		return
	}
	ww, wh := c.Size()
	ui.Overlay(c, func() {
		// it has the keys while it is open, not the editor under it
		back := ui.Box(c.Key("tm-back")).Absolute().Left(0).Top(0).Size(ww, wh).Label(T("Page history")).Role(ui.RoleDialog).
			Focusable().FocusRing(false).Focus()
		in := back.AnimateTo("tm-in", b2f(!t.closing), ui.Animation{Spring: ui.SpringOptions{Duration: 560 * time.Millisecond, Bounce: 0.04}})
		if t.closing && in < 0.02 {
			*t = timeMachine{}
			a.refocus = true // back into the editor, the caret where it was
			return
		}
		pos := back.AnimateTo("tm-pos", float32(t.sel), ui.Animation{Spring: ui.SpringOptions{Duration: 440 * time.Millisecond, Bounce: 0.05}})
		vp := ui.Point{X: (ww - tmLineRight) / 2, Y: wh * 0.17} // where the stack goes off to: the galaxy's heart
		turn := float32(c.Now().Sub(t.at).Seconds())
		back.Opacity(min(1, in*1.5)).Draw(func(p *ui.Painter, r ui.Rect) {
			if tmShaders {
				tmGalaxy.Draw(p, r, shader.Options{Uniforms: map[string]any{"Time": turn, "Center": []float32{vp.X, vp.Y}, "Scale": min(ww, wh) * 0.42}})
				return
			}
			p.Fill(r, ui.Hex("#05060d"), 0) // the night without the galaxy, as the tests draw it
		})
		if tmShaders {
			c.After(33 * time.Millisecond) // the galaxy turns, slowly: a frame every 33 ms is enough
		}
		a.tmStack(c, ww, wh, in, pos, vp)
		a.tmTimeline(c, ww, wh, in)
		a.tmBar(c, ww, wh, in)
	})
}

// tmStack draws the copies as a stack of cards going off towards vp, the one at pos at the front.
func (a *app) tmStack(c *ui.Context, ww, wh, in, pos float32, vp ui.Point) {
	t := &a.tm
	cl := ryugo.From(c).Colors
	cw := min(820, ww-tmLineRight-120)
	ch := max(220, min(620, wh-tmBarH-vp.Y-110))
	x0 := (ww - tmLineRight - cw) / 2
	y0 := wh - tmBarH - 34 - ch // the front card's top: it stands just over the bar
	cy := y0 + ch/2
	for i := len(t.vers); i >= 0; i-- {
		d := float32(i) - pos
		if d < -1.3 || d > 12 {
			continue
		}
		var scale, op, ty float32
		if d >= 0 { // further back: smaller, nearer the galaxy, fainter, in perspective
			scale = 1 / (1 + d*0.42)
			op = float32(math.Pow(float64(scale), 1.3))
			ty = (vp.Y - cy) * (1 - scale) // its centre on the line from the front card's to the galaxy's
		} else { // gone past: towards the one looking, fading
			scale = 1 - d*0.5
			op = max(0, 1+d*1.4)
			ty = -d * 60
		}
		// the page steps back from the editor: it starts as large as the text was, the others come out of the dark
		scale *= 0.9 + 0.1*(1-in)
		if i > 0 {
			op *= in
		}
		front := math.Abs(float64(d)) < 0.5
		// placed where it goes (a translation under a scale does not move it), scaled round its centre
		card := ui.Column(c.Key("tm-card:"+itoa(i))).Absolute().Left(x0).Top(y0+ty).Size(cw, ch).
			Background(cl.Bg).Border(1, cl.Hairline).Radius(rad(10)).Clip().Padding(24, 36).Gap(6).
			Shadow(0, 20, 60, 0, ui.Hex("#000000").Alpha(0.55*op)).
			TransformOrigin(0.5, 0.5).Scale(scale, scale).Opacity(op).ZIndex(200 - i).
			Label(T("Page history: ") + a.tmWhen(i)).Role(ui.RoleButton)
		if !front {
			card.Cursor(ui.CursorPointer)
			if card.Clicked() {
				t.sel = i
			}
		}
		card.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, t.page.Title).FontSize(22).FontWeight(700).TextColor(cl.Text).Grow(1).MaxLines(1)
				ui.Text(c, a.tmWhen(i)).FontSize(12.5).TextColor(cl.Muted)
			})
			if math.Abs(float64(d)) > 2.5 { // far back: the shape of a page, not its words
				for k := 0; k < 7; k++ {
					ui.Box(c).Height(10).Width(cw * float32(0.35+0.55*math.Mod(float64(k)*0.37+float64(i)*0.21, 1))).Radius(4).Background(cl.Text.Alpha(0.08))
				}
				return
			}
			for k, l := range a.tmLines(i) {
				if k > 40 {
					break
				}
				size, weight := float32(15), 400
				if l.level > 0 {
					size, weight = []float32{0, 21, 18, 16, 15}[l.level], 650
				}
				ui.Text(c, l.text).FontSize(size).FontWeight(weight).TextColor(cl.Text2).Wrap()
			}
		})
	}
}

// tmTimeline draws the timeline on the right: Now at its foot, the oldest copy at its top, a mark for each copy and
// the day where the days change, and the tilted arrows under it.
func (a *app) tmTimeline(c *ui.Context, ww, wh, in float32) {
	t := &a.tm
	n := len(t.vers)
	white := ui.Hex("#ffffff")
	top, bottom := float32(64), wh-tmBarH-120
	lx := ww - 34 + (1-in)*80 // the line's x: it comes in from the right
	yOf := func(i int) float32 {
		if n == 0 {
			return bottom
		}
		return bottom - (bottom-top)*float32(i)/float32(n)
	}
	bar := ui.Box(c.Key("tm-line")).Absolute().Left(lx-tmLineRight+20).Top(top-14).Size(tmLineRight, bottom-top+28).
		Cursor(ui.CursorPointer).Label(T("Timeline")).Role(ui.RoleSlider).Opacity(in)
	_, py, over := bar.PointerPosition()
	at := func(y float32) int { // the copy nearest y on the line
		if n == 0 {
			return 0
		}
		k := int(math.Round(float64((bottom - (y + top - 14)) / (bottom - top) * float32(n))))
		return min(max(k, 0), n)
	}
	if bar.Pressed() && !t.closing { // a click, or a drag along it: the copy under the pointer comes to the front
		t.sel = at(py)
	}
	hover := -1
	if over {
		hover = at(py)
	}
	sel := t.sel
	// the days, where they change, and Now; none closer than a line's height to the one under it
	type mark struct {
		y     float32
		label string
	}
	var marks []mark
	last := float32(1e9)
	for i := 0; i <= n; i++ {
		label := ""
		switch {
		case i == 0:
			label = T("Now")
		case i == 1 && sameDay(t.vers[0].At, a.clock()):
			label = T("Today")
		case i == 1 || !sameDay(t.vers[i-1].At, t.vers[i-2].At):
			label = t.vers[i-1].At.Format("2 Jan")
		}
		if y := yOf(i); label != "" && last-y >= 16 {
			marks = append(marks, mark{y, label})
			last = y
		}
	}
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		x := r.X + r.W - 20
		p.Fill(ui.Rect{X: x, Y: top, W: 1.5, H: bottom - top}, white.Alpha(0.25), 0)
		for i := 0; i <= n; i++ {
			y := yOf(i)
			w, col := float32(10), white.Alpha(0.45)
			if i == hover {
				w, col = 16, white.Alpha(0.85)
			}
			if i == sel {
				w, col = 24, ui.Hex("#ff9f43")
			}
			p.Fill(ui.Rect{X: x - w + 1, Y: y - 1, W: w, H: 2}, col, 1)
		}
	})
	for k, m := range marks {
		ui.Text(c.Key("tm-mark:"+itoa(k)), m.label).Absolute().Left(lx - tmLineRight).Top(m.y - 8).Width(tmLineRight - 30).TextAlign(ui.End).
			FontSize(11.5).TextColor(white.Alpha(0.7)).Opacity(in).PassThrough()
	}
	// the arrows under the line, tilted back as Time Machine's are: older, newer
	arrow := func(key string, up bool, y float32) bool {
		e := ui.Box(c.Key(key)).Absolute().Left(lx-60).Top(y).Size(56, 34).Radius(6).Cursor(ui.CursorPointer).
			Label(map[bool]string{true: T("Older"), false: T("Newer")}[up]).Role(ui.RoleButton).
			LinearGradient(ui.LinearGradient{Angle: 180, From: ui.Hex("#f2f4f8"), To: ui.Hex("#7d8594")}).
			Border(1, ui.Hex("#ffffff").Alpha(0.5)).Shadow(0, 6, 14, 0, ui.Hex("#000000").Alpha(0.5)).
			TransformOrigin(0.5, 1).Perspective(160).RotateX(38).Opacity(in)
		if e.Hovered() {
			e.LinearGradient(ui.LinearGradient{Angle: 180, From: ui.Hex("#ffffff"), To: ui.Hex("#9aa3b3")})
		}
		e.Draw(func(p *ui.Painter, r ui.Rect) {
			path := &ui.Path{}
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			s := float32(1)
			if !up {
				s = -1
			}
			path.MoveTo(cx, cy-10*s)
			path.LineTo(cx+13, cy+5*s)
			path.LineTo(cx+5, cy+5*s)
			path.LineTo(cx+5, cy+11*s)
			path.LineTo(cx-5, cy+11*s)
			path.LineTo(cx-5, cy+5*s)
			path.LineTo(cx-13, cy+5*s)
			path.Close()
			p.FillPath(path, ui.Hex("#2b3140"))
		})
		return e.Clicked() && !t.closing
	}
	if arrow("tm-older", true, bottom+22) {
		t.sel = min(t.sel+1, n)
	}
	if arrow("tm-newer", false, bottom+62) {
		t.sel = max(t.sel-1, 0)
	}
	if n == 0 {
		ui.Text(c.Key("tm-none"), T("No copies of this page are kept yet: one is kept when it is written over, every few minutes.")).Absolute().
			Left(40).Top(30).Width(ww - tmLineRight - 80).TextAlign(ui.Center).FontSize(13).TextColor(white.Alpha(0.75)).Opacity(in).PassThrough()
	}
}

// tmBar draws the bar along the foot: Cancel, the day of the copy at the front, Restore; and the differences.
func (a *app) tmBar(c *ui.Context, ww, wh, in float32) {
	t := &a.tm
	white := ui.Hex("#ffffff")
	y := wh - tmBarH + (1-in)*tmBarH
	ui.Box(c.Key("tm-bar")).Absolute().Left(0).Top(y).Size(ww, tmBarH).
		LinearGradient(ui.LinearGradient{Angle: 180, From: ui.Hex("#454b57"), To: ui.Hex("#111318")}).Opacity(in).PassThrough().
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 1}, white.Alpha(0.35), 0)
			p.Fill(ui.Rect{X: r.X + r.W*0.2, Y: r.Y + 8, W: 1, H: r.H - 16}, white.Alpha(0.12), 0)
			p.Fill(ui.Rect{X: r.X + r.W*0.8, Y: r.Y + 8, W: 1, H: r.H - 16}, white.Alpha(0.12), 0)
		})
	word := func(key, text string, x, w float32, on bool) bool {
		e := ui.Box(c.Key(key)).Absolute().Left(x).Top(y).Size(w, tmBarH).AlignItems(ui.Center).Justify(ui.Center).
			Label(text).Role(ui.RoleButton).Opacity(in)
		col := white.Alpha(0.35)
		if on {
			e.Cursor(ui.CursorPointer)
			col = white.Alpha(0.85)
			if e.Hovered() {
				col = white
			}
		}
		e.Children(func() { ui.Text(c, text).FontSize(15).FontWeight(650).TextColor(col) })
		return on && e.Clicked() && !t.closing
	}
	if word("tm-cancel", T("Cancel"), 0, ww*0.2, true) {
		t.closing = true
	}
	ui.Text(c.Key("tm-when"), a.tmWhen(t.sel)).Absolute().Left(ww * 0.2).Top(y + 13).Width(ww * 0.6).TextAlign(ui.Center).
		FontSize(16).FontWeight(700).TextColor(white).Opacity(in).PassThrough()
	if word("tm-restore", T("Restore"), ww*0.8, ww*0.2, t.sel > 0) {
		a.tmPutBack()
	}
	if t.sel > 0 { // the dialog of the differences, at this copy
		d := ui.Box(c.Key("tm-diff")).Absolute().Left(ww*0.8-150).Top(y+12).Size(140, 22).AlignItems(ui.Center).Justify(ui.End).
			Label(T("Show the differences")).Role(ui.RoleButton).Cursor(ui.CursorPointer).Opacity(in)
		alpha := float32(0.55)
		if d.Hovered() {
			alpha = 0.95
		}
		d.Children(func() { ui.Text(c, T("Show the differences")).FontSize(12).TextColor(white.Alpha(alpha)) })
		if d.Clicked() {
			sel := t.sel - 1
			p, vers := t.page, t.vers
			*t = timeMachine{}
			a.vhist = histView{open: true, page: p, vers: vers, sel: sel, opsFor: -1}
		}
	}
}

// tmGalaxy draws the night behind the history: a spiral galaxy seen a little from above, turning slowly, its heart at
// Center, Scale pixels across; its dust in blocks of a few pixels, and stars scattered round it.
var tmGalaxy = shader.MustCompile(`package main

var Time float
var Center vec2
var Scale float

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func Fragment(dst vec4, pos vec2) vec4 {
	px := floor(pos/2) * 2
	q := (px - Center) / Scale
	q.y *= 2.3 // seen from a little above: a disc, not a circle
	r := length(q)
	ang := atan2(q.y, q.x)
	// two arms wound in a log spiral, turning
	arm := 0.5 + 0.5*cos(2*ang-6*log(r+0.03)+Time*0.06)
	arm = arm * arm * arm
	disc := exp(-r * 2.2)
	grain := 0.55 + 0.45*hash(floor(px/3))
	col := vec3(0.35, 0.47, 0.95) * arm * disc * grain * 1.1
	col += vec3(0.75, 0.6, 1.0) * arm * exp(-r*5) * 0.35
	// the heart: white hot, then gold
	col += vec3(1.0, 0.95, 0.85) * exp(-r*r*55) * 1.6
	col += vec3(1.0, 0.78, 0.5) * exp(-r*6) * 0.45
	// stars, twinkling
	s := hash(floor(pos/2) + vec2(7, 13))
	col += vec3(step(0.9965, s)) * (0.55 + 0.45*sin(Time*1.7+s*60))
	// the night, a little blue at the foot
	col += mix(vec3(0.012, 0.014, 0.03), vec3(0.03, 0.04, 0.09), pos.y/rectSize().y)
	return vec4(col, 1)
}
`)
