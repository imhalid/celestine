package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's history, laid out as Apple's Time Machine lays out a folder's and drawn as the rest of the app is (its
// theme, Ryugo's controls): the page becomes the front of a stack, the copies kept of it behind it, going off into the
// distance, each smaller and fainter. It comes and goes at once; only the stack glides as a copy is chosen. On the right, the copies as bars, Now at the
// foot and the oldest at the top, as long as the page was, the days written where they change: a click on a bar,
// the buttons under them, ↑ ↓, or a click on a card behind brings that copy to the front, the stack gliding, and one
// passed goes by towards the one looking and fades. At the foot, a toolbar: Cancel, the day of the front copy, the
// differences, Restore. Restore makes the page what that copy is; Cancel or Esc goes back into the editor, the way
// it came. The differences, line by line, and keeping a copy by name are the version dialog's (historyui.go).

type timeMachine struct {
	open    bool
	closing bool
	pages   map[int]*Page              // each copy as a page: its own title, cover and values (tmVersionPage)
	docs    map[int]*richtext.Document // each copy's text, as the editor reads a page (tmDoc)
	page    *Page
	vers    []Version        // the copies, the latest first
	texts   map[int]string   // what each was, as Markdown, by its place: 0 the page now
	lines   map[int][]tmLine // each copy's text, read into lines once
	sel     int              // the copy at the front: 0 the page now, i the copy vers[i-1]
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
	t.texts, t.lines, t.pages, t.docs = map[int]string{}, map[int][]tmLine{}, nil, nil // the page now is the copy: the front card says so
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

// The layout of the history, in the window: the bars' room on the right, the toolbar's at the foot.
const (
	tmBarsW = 150
	tmFootH = 76
)

var iconChevronUp = icon(`<path d="M6.5 14.5 12 9l5.5 5.5"/>`)

// timeMachineView draws the history over the window while it is open.
func (a *app) timeMachineView(c *ui.Context) {
	t := &a.tm
	if !t.open || t.page == nil {
		return
	}
	if t.closing { // it comes and goes at once: the copies are whole pages, and moving them every frame cost too much
		*t = timeMachine{}
		a.refocus = true // back into the editor, the caret where it was
		return
	}
	ww, wh := c.Size()
	cl := ryugo.From(c).Colors
	ui.Overlay(c, func() {
		// it has the keys while it is open, not the editor under it
		back := ui.Box(c.Key("tm-back")).Absolute().Left(0).Top(0).Size(ww, wh).Label(T("Page history")).Role(ui.RoleDialog).
			Focusable().FocusRing(false).Focus()
		pos := back.AnimateTo("tm-pos", float32(t.sel), ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.05}})
		vp := ui.Point{X: (ww - tmBarsW) / 2, Y: wh * 0.12} // where the stack goes off to
		// the window's own ground, a little deeper; the accent's light far off, where the copies go
		back.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ryugo.Mix(cl.Surface2, cl.Accent, 0.04), To: cl.Bg}).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.FillRadialGradient(r, ui.RadialGradient{Center: ui.Point{X: vp.X / r.W, Y: vp.Y / r.H}, Radius: r.W * 0.55, Oklab: true,
					Stops: []ui.GradientStop{{Color: cl.Accent.Alpha(0.10), At: 0}, {Color: cl.Accent.Alpha(0), At: 1}}}, 0)
			})
		a.tmStack(c, ww, wh, pos, vp)
		a.tmBars(c, ww, wh)
		a.tmFoot(c, ww, wh)
	})
}

// tmVersionPage is the page as copy i had it: its title, cover and values from the copy's own front matter, the rest the page's.
func (a *app) tmVersionPage(i int) *Page {
	t := &a.tm
	if i == 0 {
		return t.page
	}
	if vp, ok := t.pages[i]; ok {
		return vp
	}
	vp := new(Page)
	*vp = *t.page
	if h, ok := historyOf(t.page); ok {
		data, _ := h.VersionData(t.page.File(), t.vers[i-1].ID)
		meta, _ := SplitPage(data)
		if s := meta["title"]; s != "" {
			vp.Title = s
		}
		vp.ReadMeta(meta)
	}
	if t.pages == nil {
		t.pages = map[int]*Page{}
	}
	t.pages[i] = vp
	return vp
}

// tmPageArea is the area the page is in: each copy is laid out in a room that size.
func (a *app) tmPageArea(ww, wh float32) ui.Rect {
	r := a.pageR
	if r.W < 320 || r.H < 240 {
		r = ui.Rect{X: 252, Y: 0, W: max(320, ww-252), H: wh}
	}
	return r
}

// tmStack draws the copies as a stack of pages going off towards vp, the one at pos at the front. Each is the whole page area as it was:
// the cover, the title, the properties and the text, laid out as the page is and scaled down.
func (a *app) tmStack(c *ui.Context, ww, wh, pos float32, vp ui.Point) {
	t := &a.tm
	th := ryugo.From(c)
	cl := th.Colors
	pr := a.tmPageArea(ww, wh)
	lw, lh := pr.W, pr.H
	cwMax := min(ww-tmBarsW-120, float32(1000))
	chMax := max(220, min(640, wh-tmFootH-vp.Y-90))
	base := min(1, min(cwMax/lw, chMax/lh)) // the page as large as the room is
	cw, ch := lw*base, lh*base
	x0 := (ww - tmBarsW - cw) / 2
	y0 := wh - tmFootH - 16 - ch // the front page stands just over the toolbar
	cx0, cy0 := x0+cw/2, y0+ch/2
	for i := len(t.vers); i >= 0; i-- {
		d := float32(i) - pos
		if d < -1.3 || d > 12 {
			continue
		}
		var scale, op, ty float32
		if d >= 0 { // further back: smaller, further off, fainter, in perspective
			scale = 1 / (1 + d*0.42)
			op = float32(math.Pow(float64(scale), 1.4))
			ty = (vp.Y - cy0) * (1 - scale) // its centre on the line from the front page's to where they go
		} else { // gone past: towards the one looking, fading
			scale = 1 - d*0.5
			op = max(0, 1+d*1.4)
			ty = -d * 60
		}
		sc := base * scale
		front := math.Abs(float64(d)) < 0.5
		card := ui.Column(c.Key("tm-card:"+itoa(i))).Absolute().Left(cx0-lw/2).Top(cy0+ty-lh/2).Size(lw, lh).
			Background(a.bg(c)).Border(1, cl.Hairline).Radius(th.R(ryugo.RadiusPop)/max(sc, 0.2)).Clip().
			TransformOrigin(0.5, 0.5).Scale(sc, sc).Opacity(op).ZIndex(200 - i).Cache().
			Label(T("Page history: ") + a.tmWhen(i)).Role(ui.RoleButton)
		th.Shadows.ShadowPop.Apply(card)
		if !front {
			card.Cursor(ui.CursorPointer)
			if card.Clicked() && !t.closing {
				t.sel = i
			}
		}
		card.Children(func() { a.tmPage(c, i, lw, lh, math.Abs(float64(d)) > 2.5) })
	}
}

// tmDoc is the text of copy i as the editor holds a page's: the same reading of its blocks, links and pictures, so that it is laid out as
// the page is. The page now is the page's own document.
func (a *app) tmDoc(i int) *richtext.Document {
	t := &a.tm
	if i == 0 && a.doc != nil && t.page == a.page {
		return a.doc
	}
	if d, ok := t.docs[i]; ok {
		return d
	}
	vp := a.tmVersionPage(i)
	var body string
	if i == 0 {
		body, _ = vp.Body()
	} else if h, ok := historyOf(t.page); ok {
		data, _ := h.VersionData(t.page.File(), t.vers[i-1].ID)
		_, body = SplitPage(data)
	}
	d := richtext.NewDocument(a.parsePage(vp, body))
	if t.docs == nil {
		t.docs = map[int]*richtext.Document{}
	}
	t.docs[i] = d
	return d
}

// tmPage is the page as copy i was, in the room of the page area and laid out as the page is - the band of its cover, the title on it,
// the properties, the text read as the editor reads it, read-only - or, for one far back, the shape of a page. The page's own
// structure is followed (pageView, pageBody, pageHead), so that nothing moves as the page steps back.
func (a *app) tmPage(c *ui.Context, i int, lw, lh float32, shape bool) {
	cl := ryugo.From(c).Colors
	vp := a.tmVersionPage(i)
	wrap := ui.Column(c.Key("tm-wrap:" + itoa(i))).Fill()
	if !shape { // far back, the cover is a flat band: the real one costs a drawing
		a.paintCover(c, vp, wrap)
	} else if a.coverKind(vp) != coverKindNone {
		wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: a.coverBandOf(vp)}, cl.Well, 0)
		})
	}
	wrap.Children(func() {
		if a.topSpace > 0 {
			ui.Box(c.Key("tm-top:" + itoa(i))).FillWidth().Height(a.topSpace)
		}
		ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, pageMargin()).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Gap(4).Children(func() {
				ui.Column(c.Key("tm-head:" + itoa(i))).FillWidth().Children(func() {
					ink := c.Theme().Text
					if a.coverKind(vp) != coverKindNone { // the title lies on the cover, as it does on the page
						ui.Box(c.Key("tm-room:" + itoa(i))).FillWidth().Height(a.coverRoomOf(vp))
						ink = ui.Hex("#ffffff")
						if a.coverKind(vp) == coverKindAuto && !ryugo.From(c).Dark {
							ink = c.Theme().Text
						}
					}
					ui.Text(c, vp.Title).FontSize(32).FontWeight(720).Padding(6, 0, 10, 0).TextColor(ink).MaxLines(2)
					if a.coverKind(vp) != coverKindNone {
						ui.Box(c.Key("tm-foot-gap:" + itoa(i))).FillWidth().Height(coverFoot)
					}
					ui.Column(c).Gap(1).Padding(0, 0, 14, 0).Children(func() {
						for k := range vp.Coll.Properties {
							pr := &vp.Coll.Properties[k]
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, pr.Name).FontSize(13).TextColor(cl.Muted).Width(propNameW).Height(propRowH).Padding(0, 6).MaxLines(1)
								ui.Text(c, vp.Value(pr)).FontSize(propFs).TextColor(cl.Text).MaxLines(1)
							})
						}
						ui.Box(c).Height(28) // the row of "add a property", where the page has its own
					})
				})
				if shape { // far back: the shape of a page, not its words
					for k := 0; k < 9; k++ {
						ui.Box(c).Height(14).Width(lw * float32(0.25+0.4*math.Mod(float64(k)*0.37+float64(i)*0.21, 1))).Radius(5).Background(cl.Fill)
					}
					return
				}
				c.Key("tm-ed:" + itoa(i))
				richtext.EditorBase(c, a.tmDoc(i), richtext.EditorOptions{
					Label: T("Page history: ") + a.tmWhen(i), Font: a.font, Theme: a.theme, ReadOnly: true,
					Spans:        func(_ int, _ richtext.ParagraphStyle, spans []ui.Span) []ui.Span { return spaced(spans) },
					SpansVersion: a.settings.LetterSpacing*100 + a.settings.AppTheme,
				}).FillWidth().Height(lh).FocusRing(false).PassThrough() // a click goes to the page: it brings it to the front
			})
		})
	})
}

// tmSize is how large the copy at place i is, for its bar: the page now by its text.
func (a *app) tmSize(i int) int {
	if i == 0 {
		return len(a.tmText(0))
	}
	return a.tm.vers[i-1].Size
}

// tmBars draws the copies on the right as bars, Now at the foot and the oldest at the top, each as long as the page
// was, the days written where they change; and the buttons under them, older and newer.
func (a *app) tmBars(c *ui.Context, ww, wh float32) {
	t := &a.tm
	cl := ryugo.From(c).Colors
	n := len(t.vers)
	top, bottom := float32(56), wh-tmFootH-96
	right := ww - 28
	yOf := func(i int) float32 {
		if n == 0 {
			return bottom
		}
		return bottom - (bottom-top)*float32(i)/float32(n)
	}
	row := float32(18) // each bar's room to be pressed: as high as there is room for
	if n > 0 {
		row = min(18, max(6, (bottom-top)/float32(n)))
	}
	most := 1
	for i := 0; i <= n; i++ {
		most = max(most, a.tmSize(i))
	}
	last := float32(1e9) // the day written last, from the foot: none closer than a line to the one under it
	for i := 0; i <= n; i++ {
		y := yOf(i)
		w := 16 + 70*float32(a.tmSize(i))/float32(most)
		e := ui.Box(c.Key("tm-bar:"+itoa(i))).Absolute().Left(right-90).Top(y-row/2).Size(90, row).Cursor(ui.CursorPointer).
			Label(a.tmWhen(i)).Role(ui.RoleButton)
		col := cl.Text.Alpha(0.2)
		switch {
		case i == t.sel:
			col = cl.Accent
		case e.Hovered():
			col = cl.Text.Alpha(0.5)
		}
		e.Draw(func(p *ui.Painter, r ui.Rect) {
			h := min(4, max(2, row-2))
			p.Fill(ui.Rect{X: r.X + r.W - w, Y: r.Y + (r.H-h)/2, W: w, H: h}, col, h/2)
		})
		if e.Hovered() {
			ryugo.Tooltip(c, e, a.tmWhen(i))
		}
		if e.Clicked() && !t.closing {
			t.sel = i
		}
		label := ""
		switch {
		case i == 0:
			label = T("Now")
		case i == 1 && sameDay(t.vers[0].At, a.clock()):
			label = T("Today")
		case i == 1 || !sameDay(t.vers[i-1].At, t.vers[i-2].At):
			label = t.vers[i-1].At.Format("2 Jan")
		}
		if label != "" && last-y >= 16 {
			tc := cl.Muted
			if i == t.sel {
				tc = cl.Accent
			}
			ui.Text(c.Key("tm-day:"+itoa(i)), label).Absolute().Left(right - 180).Top(y - 8).Width(84).TextAlign(ui.End).
				FontSize(ryugo.FsSm).TextColor(tc).PassThrough()
			last = y
		}
	}
	// older and newer
	ui.Row(c.Key("tm-steps")).Absolute().Left(right - 74).Top(bottom + 26).Gap(6).Children(func() {
		if ryugo.Button(c.Key("tm-older"), T("Older"), ryugo.ButtonOptions{Icon: iconChevronUp, IconOnly: true, Size: ryugo.Sm, Disabled: t.sel >= n}).Clicked() && !t.closing {
			t.sel = min(t.sel+1, n)
		}
		if ryugo.Button(c.Key("tm-newer"), T("Newer"), ryugo.ButtonOptions{Icon: iconChevronDown, IconOnly: true, Size: ryugo.Sm, Disabled: t.sel == 0}).Clicked() && !t.closing {
			t.sel = max(t.sel-1, 0)
		}
	})
	if n == 0 {
		ui.Text(c.Key("tm-none"), T("No copies of this page are kept yet: one is kept when it is written over, every few minutes.")).Absolute().
			Left(40).Top(24).Width(ww - tmBarsW - 80).TextAlign(ui.Center).FontSize(ryugo.FsSm).TextColor(cl.Muted).PassThrough()
	}
}

// tmFoot draws the toolbar at the foot, as the app's floating bars are: Cancel, the day of the front copy, the
// differences, Restore.
func (a *app) tmFoot(c *ui.Context, ww, wh float32) {
	t := &a.tm
	th := ryugo.From(c)
	cl := th.Colors
	ui.Row(c.Key("tm-foot")).Absolute().Left(0).Width(ww - tmBarsW).Top(wh - tmFootH + 16).Justify(ui.Center).Children(func() {
		bar := ui.Row(c.Key("tm-tools")).Padding(6, 6, 6, 14).Gap(8).AlignItems(ui.Center).
			Radius(th.R(ryugo.RadiusControl)+4).Background(cl.Card).Border(1, cl.Hairline).Label(T("Page history"))
		th.Shadows.ShadowPop.Apply(bar)
		bar.Children(func() {
			ui.Text(c.Key("tm-when"), a.tmWhen(t.sel)).FontSize(14).FontWeight(650).TextColor(cl.Text).MinWidth(170)
			if ryugo.Button(c.Key("tm-diff"), T("Show the differences"), ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: t.sel == 0}).Clicked() && t.sel > 0 {
				sel := t.sel - 1
				p, vers := t.page, t.vers
				*t = timeMachine{}
				a.vhist = histView{open: true, page: p, vers: vers, sel: sel, opsFor: -1}
				return
			}
			if ryugo.Button(c.Key("tm-cancel"), T("Cancel"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() && !t.closing {
				t.closing = true
			}
			if ryugo.Button(c.Key("tm-restore"), T("Restore"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid, Disabled: t.sel == 0}).Clicked() && !t.closing {
				a.tmPutBack()
			}
		})
	})
}
