package main

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's history, laid out as Apple's Time Machine lays out a folder's and drawn as the rest of the app is (its
// theme, Ryugo's controls): the page becomes the front of a stack, the copies kept of it behind it, going off into the
// distance, each smaller and fainter. It comes in and goes out in a moment, the copies rising into place, and the
// stack glides as a copy is chosen: only the layers of the copies move, each drawn and laid out once. The stack
// has the page's room; in the sidebar, where the tree fades out, the timeline (tmside.go): a row a day, today at the
// top, a dot for each copy. A click on a day or a dot, the buttons in the toolbar, the arrows (↑ newer, ↓ older), or
// a click on a card behind brings that copy to the front, the stack gliding, and one passed goes by towards the one
// looking and fades. On a phone, which has no sidebar beside the page, the copies are bars on the right instead. At
// the foot, a toolbar: the day of the front copy, older and newer, the differences, Cancel, Restore. Restore makes
// the page what that copy is, the whole of it; Cancel or Esc goes back into the editor, the way it came. The
// differences are shown in the stack's place, side by side with the page now (tmdiff.go); keeping a copy by name is
// the version dialog's (historyui.go).

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
	days    []tmDay          // the copies by the day, for the timeline in the sidebar (tmside.go)
	shown   int              // the copy the timeline was last scrolled to
	begun   bool             // it has been drawn: its coming in has begun
	in      float32          // how far it is in: 0 gone, 1 here (timeMachineView)

	diff       bool                // the differences from the page now are shown, not the stack (tmdiff.go)
	diffs      map[int][]tmDiffRow // each copy's, by its place
	diffScroll ui.ScrollState
	diffFor    int // the copy the differences were last scrolled for

	reveal   tmReveal // a restore being shown: the light running down the front card (tmreveal.go)
	noReveal bool     // the system asks for less motion: a restore goes straight back into the editor
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
		a.say(toastInfo, locales.VersionHistoryIsKeptOnlyIn())
		return
	}
	a.flush()
	a.tm = timeMachine{open: true, page: p, vers: tmCopies(h, p), texts: map[int]string{}, lines: map[int][]tmLine{}, shown: -1}
	a.tm.days = tmDaysOf(a.tm.vers, a.clock())
}

// tmCopies are the copies kept of page p, the latest first, but those that say what the page now says: after going
// back to a copy the page is that copy, and the stack would show it twice, as Now and as itself (#263). They stay
// kept, and the version dialog lists them.
func tmCopies(h Historian, p *Page) []Version {
	vers := h.Versions(p.File())
	now, err := p.FS().ReadFile(p.File())
	if err != nil {
		return vers
	}
	return slices.DeleteFunc(vers, func(v Version) bool {
		data, ok := h.VersionData(p.File(), v.ID)
		return ok && samePage(data, now)
	})
}

// tmBarsWidth is the room of the bars on the right: none where the sidebar has the timeline.
func tmBarsWidth() float32 {
	if phone {
		return tmBarsW
	}
	return 0
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
	if i <= 0 { // the page now, or as it was while a restore is shown
		return locales.HistoryTodayNow()
	}
	at := a.tm.vers[i-1].At
	if sameDay(at, a.clock()) {
		return locales.HistoryTodayAt(clockTime(at))
	}
	return dateTime(at, true)
}

// tmPutBack makes the page what the copy at the front is, as the version dialog does (the page now kept as a
// copy first), and goes back into the editor: after the light has run down the front card, the copy in its wake
// (tmreveal.go), unless less motion is asked for.
func (a *app) tmPutBack() {
	t := &a.tm
	if t.sel == 0 || t.sel > len(t.vers) || t.busy() {
		return
	}
	sel := t.sel
	var keep func()
	if !t.noReveal {
		keep = a.startReveal(sel)
	}
	a.vhist = histView{page: t.page, vers: t.vers, sel: sel - 1, opsFor: -1}
	a.putBackVersion()
	t.texts, t.lines, t.pages, t.docs, t.diffs = map[int]string{}, map[int][]tmLine{}, nil, nil, nil // the page now is the copy: the front card says so
	t.diff = false
	if keep != nil {
		keep()
		return
	}
	t.sel, t.closing = 0, true
}

// tmKeys takes the history's keys, early in the frame: before the editor's normal mode takes Esc, and the arrows
// move the caret; it says whether the history is open.
func (a *app) tmKeys(c *ui.Context) bool {
	t := &a.tm
	if !t.open {
		return false
	}
	if t.busy() {
		return true
	}
	n := len(t.vers)
	up, down := c.Shortcut(0, ui.KeyUp), c.Shortcut(0, ui.KeyDown)
	if !phone { // the timeline has the newest at the top; the bars, at the foot
		up, down = down, up
	}
	switch {
	case c.Shortcut(0, ui.KeyEscape):
		t.closing = true
	case up, c.Shortcut(0, ui.KeyLeft):
		t.sel = min(t.sel+1, n)
	case down, c.Shortcut(0, ui.KeyRight):
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

// tmInOut is how the history comes in and goes out.
var tmInOut = ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseOut}

// timeMachineView draws the history over the window while it is open.
func (a *app) timeMachineView(c *ui.Context) {
	t := &a.tm
	if !t.open || t.page == nil {
		return
	}
	ww, wh := c.Size()
	ox := float32(0) // its left edge: the sidebar, which has the timeline, stays
	if !phone {
		ox = min(a.sidebarShown, ww-320)
	}
	aw := ww - ox
	cl := ryugo.From(c).Colors
	t.noReveal = c.Preferences().ReduceMotion
	ui.Overlay(c, func() {
		// it has the keys while it is open, not the editor under it
		back := ui.Box(c.Key("tm-back")).Absolute().Left(ox).Top(0).Size(aw, wh).Label(locales.HistoryPageHistory()).Role(ui.RoleDialog).
			Focusable().FocusRing(false).Focus()
		// It comes in and goes out in a moment: the ground fades, the copies rise into place. Only what the GPU does
		// to a layer moves (where it is, how large, how clear): each copy is drawn once (Cache) and laid out once.
		if !t.begun {
			back.Jump("tm-in", 0)
			t.begun = true
		}
		t.in = back.AnimateTo("tm-in", map[bool]float32{false: 1}[t.closing], tmInOut)
		if t.closing && t.in < 0.02 {
			*t = timeMachine{}
			a.refocus = true // back into the editor, the caret where it was
			return
		}
		back.Opacity(t.in)
		pos := back.AnimateTo("tm-pos", float32(t.sel), ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.05}})
		vp := ui.Point{X: ox + (aw-tmBarsWidth())/2, Y: wh * 0.12} // where the stack goes off to
		// the window's own ground, a little deeper; the accent's light far off, where the copies go
		back.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ryugo.Mix(cl.Surface2, cl.Accent, 0.04), To: cl.Bg}).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.FillRadialGradient(r, ui.RadialGradient{Center: ui.Point{X: (vp.X - ox) / r.W, Y: vp.Y / r.H}, Radius: r.W * 0.55, Oklab: true,
					Stops: []ui.GradientStop{{Color: cl.Accent.Alpha(0.10), At: 0}, {Color: cl.Accent.Alpha(0), At: 1}}}, 0)
			})
		if t.diff && t.sel > 0 {
			a.tmDiffView(c, ox, aw, wh)
		} else {
			a.tmStack(c, ox, aw, wh, pos, vp)
		}
		if phone {
			a.tmBars(c, ww, wh)
		}
		if len(t.vers) == 0 {
			ui.Text(c.Key("tm-none"), locales.HistoryNoCopiesThisPage()).Absolute().
				Left(ox + 40).Top(24).Width(aw - tmBarsWidth() - 80).TextAlign(ui.Center).FontSize(ryugo.FsSm).TextColor(cl.Muted).PassThrough()
		}
		a.tmFoot(c, ox, aw, wh)
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
func (a *app) tmStack(c *ui.Context, ox, aw, wh, pos float32, vp ui.Point) {
	t := &a.tm
	th := ryugo.From(c)
	cl := th.Colors
	pr := a.tmPageArea(ox+aw, wh)
	lw, lh := pr.W, pr.H
	cwMax := min(aw-tmBarsWidth()-120, float32(1000))
	chMax := max(220, min(640, wh-tmFootH-vp.Y-90))
	base := min(1, min(cwMax/lw, chMax/lh)) // the page as large as the room is
	cw, ch := lw*base, lh*base
	x0 := ox + (aw-tmBarsWidth()-cw)/2
	y0 := wh - tmFootH - 16 - ch // the front page stands just over the toolbar
	cx0, cy0 := x0+cw/2, y0+ch/2
	rp := a.revealStep(c) // a restore being shown: the copy over the page as it was, in the front's place
	if t.reveal.sel > 0 {
		sc := base * revealScale(rp)
		old := ui.Column(c.Key("tm-card:"+itoa(tmRevealOld))).Absolute().Left(cx0-lw/2).Top(cy0-lh/2).Size(lw, lh).
			Background(a.bg(c)).Border(1, cl.Hairline).Radius(th.R(ryugo.RadiusPop)/sc).Clip().
			TransformOrigin(0.5, 0.5).Scale(sc, sc).Opacity(t.in).ZIndex(200 - t.reveal.sel).Cache().PassThrough()
		th.Shadows.ShadowPop.Apply(old)
		revealEffect(old, rp, false, lw, lh, th.R(ryugo.RadiusPop)/sc, cl)
		old.Children(func() { a.tmPage(c, tmRevealOld, lw, lh, false) })
	}
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
		sc := base * scale * (0.96 + 0.04*t.in) // coming in, it grows the last of the way
		shown := t.reveal.sel > 0 && i == t.reveal.sel
		z := 200 - i
		if shown {
			sc = base * revealScale(rp)
			z++ // over the page as it was, in the same place
		}
		front := math.Abs(float64(d)) < 0.5
		card := ui.Column(c.Key("tm-card:"+itoa(i))).Absolute().Left(cx0-lw/2).Top(cy0+ty-lh/2).Size(lw, lh).
			Background(a.bg(c)).Border(1, cl.Hairline).Radius(th.R(ryugo.RadiusPop)/max(sc, 0.2)).Clip().
			TransformOrigin(0.5, 0.5).Scale(sc, sc).Translate(0, (1-t.in)*36).Opacity(op * t.in).ZIndex(z).Cache().
			Label(locales.HistoryPageHistoryAt(a.tmWhen(i))).Role(ui.RoleButton)
		th.Shadows.ShadowPop.Apply(card)
		if shown {
			revealEffect(card, rp, true, lw, lh, th.R(ryugo.RadiusPop)/sc, cl)
		}
		if !front {
			card.Cursor(ui.CursorPointer)
			if card.Clicked() && !t.busy() {
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
					ink := a.coverInk(c, vp) // as on the page (cover.go)
					if a.coverKind(vp) != coverKindNone {
						ui.Box(c.Key("tm-room:" + itoa(i))).FillWidth().Height(a.coverRoomOf(vp))
					}
					ui.Text(c, vp.Title).FontSize(32).FontWeight(720).Padding(6, 0, 10, 0).TextColor(ink).MaxLines(2)
					if h := coverFootOf(a.coverKind(vp)); h > 0 {
						ui.Box(c.Key("tm-foot-gap:" + itoa(i))).FillWidth().Height(h)
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
					Label: locales.HistoryPageHistoryAt(a.tmWhen(i)), Font: a.font, Theme: a.theme, ReadOnly: true,
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
// was, the days written where they change: on a phone, where no sidebar has the timeline.
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
		if e.Clicked() && !t.busy() {
			t.sel = i
		}
		label := ""
		switch {
		case i == 0:
			label = locales.HistoryNow()
		case i == 1 && sameDay(t.vers[0].At, a.clock()):
			label = locales.HistoryToday()
		case i == 1 || !sameDay(t.vers[i-1].At, t.vers[i-2].At):
			label = dayMonth(t.vers[i-1].At)
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
}

// tmFoot draws the toolbar at the foot, as the app's floating bars are: the day of the front copy, older and newer,
// the differences, Cancel, Restore.
func (a *app) tmFoot(c *ui.Context, ox, aw, wh float32) {
	t := &a.tm
	th := ryugo.From(c)
	cl := th.Colors
	n := len(t.vers)
	ui.Row(c.Key("tm-foot")).Absolute().Left(ox).Width(aw-tmBarsWidth()).Top(wh-tmFootH+16).Justify(ui.Center).Translate(0, (1-t.in)*20).Opacity(t.in).Children(func() {
		bar := ui.Row(c.Key("tm-tools")).Padding(6, 6, 6, 14).Gap(8).AlignItems(ui.Center).
			Radius(th.R(ryugo.RadiusControl)+4).Background(cl.Card).Border(1, cl.Hairline).Label(locales.HistoryPageHistory())
		th.Shadows.ShadowPop.Apply(bar)
		bar.Children(func() {
			ui.Text(c.Key("tm-when"), a.tmWhen(t.sel)).FontSize(14).FontWeight(650).TextColor(cl.Text).MinWidth(170)
			if ryugo.Button(c.Key("tm-older"), locales.HistoryOlder(), ryugo.ButtonOptions{Icon: iconChevronDown, IconOnly: true, Size: ryugo.Sm, Disabled: t.sel >= n}).Clicked() && !t.busy() {
				t.sel = min(t.sel+1, n)
			}
			if ryugo.Button(c.Key("tm-newer"), locales.HistoryNewer(), ryugo.ButtonOptions{Icon: iconChevronUp, IconOnly: true, Size: ryugo.Sm, Disabled: t.sel == 0}).Clicked() && !t.busy() {
				t.sel = max(t.sel-1, 0)
			}
			label := locales.HistoryShowDifferences() // in its place, side by side with the page now (tmdiff.go)
			if t.diff && t.sel > 0 {
				label = locales.HistoryShowPage()
			}
			if ryugo.Button(c.Key("tm-diff"), label, ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: t.sel == 0}).Clicked() && t.sel > 0 && !t.busy() {
				t.diff = !t.diff
			}
			if ryugo.Button(c.Key("tm-cancel"), locales.CommonCancel2(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() && !t.busy() {
				t.closing = true
			}
			if ryugo.Button(c.Key("tm-restore"), locales.HistoryRestore(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid, Disabled: t.sel == 0}).Clicked() && !t.busy() {
				a.tmPutBack()
			}
		})
	})
}
