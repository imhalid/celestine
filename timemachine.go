package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's history as Apple's Time Machine shows a folder's: the page steps back from the editor and becomes the
// front of a stack, the copies kept of it behind it, each further back, smaller and fainter; a timeline at the
// foot runs from the oldest copy to Now, and moving along it (a click, a drag, ← →) brings that day's page to the
// front, the stack gliding. Put back makes the page what the front copy is; Esc or Cancel goes back into the
// editor, the way it came. The differences, line by line, and keeping a copy by name are the version dialog's
// (historyui.go), a click away.

type timeMachine struct {
	open    bool
	closing bool
	page    *Page
	vers    []Version        // the copies, the latest first
	texts   map[int]string   // what each was, as Markdown, by its place: 0 the page now
	sel     int              // the copy at the front: 0 the page now, i the copy vers[i-1]
	lines   map[int][]tmLine // each copy's text, read into lines once
	stars   [][3]float32     // the night behind the stack: x, y as fractions, and how bright
}

// tmLine is a line of a copy's text, as the card shows it.
type tmLine struct {
	text  string
	level int // a heading's, 0 for text
}

// openTimeMachine shows the history of page p.
func (a *app) openTimeMachine(p *Page) {
	h, ok := historyOf(p)
	if !ok {
		a.notice = "Sürüm geçmişi yalnız tek dosya kitaplığında tutulur"
		return
	}
	a.flush()
	a.tm = timeMachine{open: true, page: p, vers: h.Versions(p.File()), texts: map[int]string{}, lines: map[int][]tmLine{}}
	for i := 0; i < 90; i++ { // the same night each time: a page's history is a place
		f := float64(i)
		a.tm.stars = append(a.tm.stars, [3]float32{float32(math.Mod(f*0.618034, 1)), float32(math.Mod(f*0.414214+0.13, 1)), float32(0.25 + 0.75*math.Mod(f*0.7548, 1))})
	}
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

// tmWhen is when the copy at place i was written: Now for the page.
func (a *app) tmWhen(i int) string {
	if i == 0 {
		return T("Now")
	}
	return a.tm.vers[i-1].At.Format("2 Jan 2006 · 15:04")
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
	case c.Shortcut(0, ui.KeyLeft), c.Shortcut(0, ui.KeyUp):
		t.sel = min(t.sel+1, n)
	case c.Shortcut(0, ui.KeyRight), c.Shortcut(0, ui.KeyDown):
		t.sel = max(t.sel-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		a.tmPutBack()
	}
	return true
}

// timeMachineView draws the history over the window while it is open.
func (a *app) timeMachineView(c *ui.Context) {
	t := &a.tm
	if !t.open || t.page == nil {
		return
	}
	ww, wh := c.Size()
	ui.Overlay(c, func() {
		// it has the keys while it is open, not the editor under it
		back := ui.Box(c.Key("tm-back")).Absolute().Left(0).Top(0).Size(ww, wh).Label(T("Page history")).Role(ui.RoleDialog).Focusable().FocusRing(false).Focus()
		in := back.AnimateTo("tm-in", b2f(!t.closing), ui.Animation{Spring: ui.SpringOptions{Duration: 520 * time.Millisecond, Bounce: 0.04}})
		if t.closing && in < 0.02 {
			*t = timeMachine{}
			a.refocus = true // back into the editor, the caret where it was
			return
		}
		pos := back.AnimateTo("tm-pos", float32(t.sel), ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.06}})
		stars := t.stars
		back.Opacity(min(1, in*1.4)).LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ui.Hex("#05081a"), To: ui.Hex("#18224a")}).Draw(func(p *ui.Painter, r ui.Rect) {
			for _, s := range stars {
				sz := 1 + s[2]
				p.Fill(ui.Rect{X: r.X + s[0]*r.W, Y: r.Y + s[1]*r.H*0.8, W: sz, H: sz}, ui.Hex("#ffffff").Alpha(0.15+0.5*s[2]), sz/2)
			}
		})
		a.tmStack(c, ww, wh, in, pos)
		a.tmTimeline(c, ww, wh, in)
	})
}

// tmStack draws the copies as a stack of cards going back, the one at pos at the front.
func (a *app) tmStack(c *ui.Context, ww, wh, in, pos float32) {
	t := &a.tm
	cl := ryugo.From(c).Colors
	cw := min(780, ww-160)
	ch := max(240, wh-250)
	x0 := (ww - cw) / 2
	y0 := float32(96)
	for i := len(t.vers); i >= 0; i-- {
		d := float32(i) - pos
		if d < -1.3 || d > 7 {
			continue
		}
		var scale, ty, op float32
		if d >= 0 { // at the front, or further back: smaller, higher, fainter
			scale = float32(math.Pow(0.93, float64(d)))
			ty = -d * 30
			op = max(0, 1-d*0.17)
		} else { // gone past: towards the one looking, fading
			scale = 1 - d*0.22
			ty = -d * 40
			op = max(0, 1+d*1.2)
		}
		// the page steps back from the editor: it starts as large as the text was, the others come out of the dark
		scale *= 0.86 + 0.14*(1-in)
		if i > 0 {
			op *= in
		}
		ty = ty*in + (1-in)*(-y0+24)
		front := math.Abs(float64(d)) < 0.5
		card := ui.Column(c.Key("tm-card:"+itoa(i))).Absolute().Left(x0).Top(y0).Size(cw, ch).
			Background(cl.Bg).Border(1, cl.Hairline).Radius(rad(14)).Clip().Padding(26, 40).Gap(6).
			Shadow(0, 18, 50, 0, ui.Hex("#000000").Alpha(0.45*op)).
			TransformOrigin(0.5, 0).Translate(0, ty).Scale(scale, scale).Opacity(op).ZIndex(100 - i).
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
				for k := 0; k < 6; k++ {
					ui.Box(c).Height(9).Width(cw * float32(0.4+0.5*math.Mod(float64(k)*0.37+float64(i)*0.21, 1))).Radius(4).Background(cl.Text.Alpha(0.08))
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

// tmTimeline draws the timeline at the foot, the buttons, and the day of the copy at the front over it.
func (a *app) tmTimeline(c *ui.Context, ww, wh, in float32) {
	t := &a.tm
	n := len(t.vers)
	tw := min(900, ww-200)
	left := (ww - tw) / 2
	ty := wh - 86 + (1-in)*60
	white := ui.Hex("#ffffff")
	// place k on the line, from the oldest (0) to Now (n): the copy vers[n-1-k], Now last
	xOf := func(i int) float32 {
		if n == 0 {
			return left + tw
		}
		return left + tw*float32(n-i)/float32(n)
	}
	bar := ui.Box(c.Key("tm-line")).Absolute().Left(left-12).Top(ty-14).Size(tw+24, 40).Cursor(ui.CursorPointer).
		Label(T("Timeline")).Role(ui.RoleSlider).Opacity(in)
	px, _, over := bar.PointerPosition()
	at := func(x float32) int { // the copy nearest x on the line
		if n == 0 {
			return 0
		}
		k := int(math.Round(float64((x - 12) / tw * float32(n))))
		return n - min(max(k, 0), n)
	}
	if bar.Pressed() && !t.closing { // a click, or a drag along it: the copy under the pointer comes to the front
		t.sel = at(px)
	}
	sel := t.sel
	hover := -1
	if over {
		hover = at(px)
	}
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		y := r.Y + 14
		p.Fill(ui.Rect{X: r.X + 12, Y: y, W: tw, H: 2}, white.Alpha(0.22), 1)
		for i := 0; i <= n; i++ {
			x := xOf(i) - left + r.X + 12
			h, col := float32(10), white.Alpha(0.45)
			if i == hover {
				h, col = 14, white.Alpha(0.8)
			}
			if i == sel {
				h, col = 20, ui.Hex("#ff9f43")
			}
			p.Fill(ui.Rect{X: x - 1.5, Y: y + 1 - h/2, W: 3, H: h}, col, 1.5)
		}
	})
	// the day of the front copy, over its mark; the ends' days under the line
	label := a.tmWhen(sel)
	ui.Text(c.Key("tm-when"), label).Absolute().Left(min(max(xOf(sel)-110, 8), ww-228)).Top(ty - 46).Width(220).TextAlign(ui.Center).
		FontSize(15).FontWeight(650).TextColor(white).Opacity(in).PassThrough()
	if n > 0 {
		ui.Text(c.Key("tm-oldest"), a.tmWhen(n)).Absolute().Left(left - 40).Top(ty + 16).Width(200).FontSize(11.5).TextColor(white.Alpha(0.55)).Opacity(in).PassThrough()
	}
	ui.Text(c.Key("tm-now"), T("Now")).Absolute().Left(left + tw - 60).Top(ty + 16).Width(100).TextAlign(ui.End).FontSize(11.5).TextColor(white.Alpha(0.55)).Opacity(in).PassThrough()
	if n == 0 {
		ui.Text(c.Key("tm-none"), T("No copies of this page are kept yet: one is kept when it is written over, every few minutes.")).Absolute().
			Left(left).Top(ty - 80).Width(tw).TextAlign(ui.Center).FontSize(13).TextColor(white.Alpha(0.7)).Opacity(in).PassThrough()
	}
	// the buttons
	ui.Row(c.Key("tm-buttons")).Absolute().Right(28).Bottom(24 - (1-in)*60).Gap(8).Opacity(in).Children(func() {
		if t.sel > 0 && ryugo.Button(c.Key("tm-diff"), T("Show the differences"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			sel := t.sel - 1
			p, vers := t.page, t.vers
			*t = timeMachine{}
			a.vhist = histView{open: true, page: p, vers: vers, sel: sel, opsFor: -1}
			return
		}
		if ryugo.Button(c.Key("tm-cancel"), T("Cancel"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			t.closing = true
		}
		if t.sel > 0 && ryugo.Button(c.Key("tm-restore"), T("Put this back"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid}).Clicked() {
			a.tmPutBack()
		}
	})
}
