package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// A window has one pane or two, side by side, each showing a screen of its own: a page, a
// collection, the settings, the trash, the tasks, a search, a tag. What a pane shows and keeps is
// paneState, which the app has in itself, embedded, for the pane in use: all the code that says
// a.page or a.doc is of the pane in use. A pane that is not in use has its paneState put aside
// (panes); to build it, it is swapped in, built, and swapped out again.
type paneState struct {
	page         *Page              // the page open
	doc          *richtext.Document // its text
	title        string             // the title field's text
	coll         *Collection        // the collection whose page is shown, instead of a page
	collName     string             // its name field's text
	search       string             // its search field's text
	fresh        bool               // the page was just made: its title takes the focus
	version      uint64
	length       int
	caretAt      int
	wrote        int
	edited       bool
	erased       int // letters erased since the caret was drawn, which way (-1 Backspace, 1 Delete) and what they were (caretfx.go)
	eraseDir     int
	gone         string
	typed        string        // what was written since the caret was drawn
	prevText     richtext.Text // the text before the last edit, for what was erased
	editAt       time.Time
	dirty        bool
	trashOpen    bool // the trash is shown, instead of a page
	tasksOpen    bool // the task view is shown (tasks.go)
	searchOpen   bool // the search over the whole library is shown (search.go)
	searchQ      string
	searchFocus  bool
	taskDue      int            // 0 any day, 1 today, 2 this week, 3 late
	taskFilter   int            // 0 open, 1 done, 2 all
	taskList     *ui.ListState  // the list of tasks, a row for each in view (tasks.go)
	taskQuery    string         // what is typed in the tasks' search
	taskColl     string         // the collection the tasks are of (its Dir), "" all of them
	taskFolded   map[*Page]bool // the pages whose tasks are folded away
	tag          string         // the tag (its key) whose pages are shown, instead of a page (tags.go)
	colDrag      colDragState
	cellEd       cellEdit
	titleCaret   titleCaretSpec // where the caret in the title is, to draw it (the title is an input)
	titleSeen    bool
	titleLen     int
	selPrevA     int // the selection before a pointer went down in the editor
	selPrevC     int
	boxes        []richtext.ParagraphBox
	focused      bool
	lastSave     string                  // the body as last written
	lastWords    int                     // the words it had, or -1 when not known
	style        richtext.ParagraphStyle // the caret paragraph's, last frame
	refocus      bool                    // the editor takes the keyboard focus in the next frame
	pageR        ui.Rect                 // the area the page is in, last frame: the editor reaches its left edge
	caretPara    int                     // where the caret was, last frame: its paragraph, its place in it, the paragraph's top
	caretRel     float32
	caretTop     float32
	sel          selBar          // the bar over a selection (selbar.go)
	edPressed    bool            // the pointer is down in the editor
	boardFocus   string          // the board column asked for, wide
	boardScroll  *ui.ScrollState // the board scrolled sideways: where its edges fade (edgeFades)
	pageScroll   *ui.ScrollState // the page scrolled as a whole (pageView)
	scrollR      ui.Rect         // its room in the window, last frame
	barInset     float32         // the part of the top of the page the title bar lies over (the page scrolls under it)
	topSpace     float32         // what the page leaves above its title, in its scroll
	winScrolled  bool            // a huge page is scrolled in its window: its title and properties give way (trackWindow)
	winTop       float32         // where its first paragraph is, scrolled to the top
	winTrim      float32         // the room around the window of a huge page that the page does not have (it starts with a guess)
	winFlip      time.Time       // when its title last went or came back
	headH        float32         // the title's and properties' height, last frame
	titleFocused bool
	edH          float32 // the editor's height, last frame
	revealWant   bool    // the caret moved: bring it into view
	statsVer     uint64  // the version of the text the counts are of
	statWords    int
	statChars    int
	settingsTab  int       // the settings' tab
	calDay       time.Time // the day open on the calendar, and whether it is
	calOpen      bool
	calGrid      ui.Rect
	calMonth     time.Time // the month the calendar view shows
	editorR      ui.Rect   // the editor's box, from its last frame
	bulk         bulkState // pages being chosen in a collection, to be done to all (bulk.go)
	settingsOpen bool
	statsScreen  bool    // the page of figures is shown, instead of a page (statsscreen.go)
	caretWin     ui.Rect // the caret in the window, last drawn
	find         finder  // finding in the page (find.go)
}

// newPaneState is a pane with nothing in it: its scrolls and lists are its own.
func newPaneState() paneState {
	return paneState{pageScroll: new(ui.ScrollState), boardScroll: new(ui.ScrollState), taskList: new(ui.ListState)}
}

// paneSet is the panes of the window: one, or two side by side. The pane in use has its state in
// the app (embedded paneState); states holds the others, and a stale copy for the one in use.
type paneSet struct {
	states []paneState
	active int  // the pane in use
	cur    int  // the pane whose state is in the app now: the one in use, or one being built
	want   int  // a pane to take the use at the end of the frame; -1 none
	settle int  // frames after a change of use in which the focus still being in the old pane is not a request for it back
	focus  bool // the pane just put in use has no text of its own: it takes the keyboard
}

// paneCount is how many panes there are.
func (a *app) paneCount() int { return max(1, len(a.ps.states)) }

// inactivePane reports whether the pane whose state is in the app is not the one in use.
func (a *app) inactivePane() bool { return a.ps.cur != a.ps.active }

// withPane runs fn with pane i's state in the app, and puts it back.
func (a *app) withPane(i int, fn func()) {
	if i == a.ps.active || i < 0 || i >= len(a.ps.states) {
		fn()
		return
	}
	a.ps.states[a.ps.active] = a.paneState
	a.paneState = a.ps.states[i]
	a.ps.cur = i
	fn()
	a.ps.states[i] = a.paneState
	a.paneState = a.ps.states[a.ps.active]
	a.ps.cur = a.ps.active
}

// usePane makes pane i the one in use: the page being written is saved first.
func (a *app) usePane(i int) {
	if i == a.ps.active || i < 0 || i >= len(a.ps.states) {
		return
	}
	a.flush()
	a.ps.states[a.ps.active] = a.paneState
	a.ps.active, a.ps.cur = i, i
	a.paneState = a.ps.states[i]
	a.refocus = true // its text takes the keyboard
	a.ps.settle, a.ps.focus = 4, true
	if a.frame != nil {
		a.frame()
	}
}

// openAtPane shows place pl in pane i, making a second pane of the window if i is 1 and there is only
// one, and puts that pane in use.
func (a *app) openAtPane(i int, pl place) {
	if len(a.ps.states) == 0 { // only the one, not yet in the list
		a.ps.states = []paneState{{}}
	}
	if i == 1 && len(a.ps.states) == 1 {
		a.ps.states = append(a.ps.states, newPaneState())
	}
	if i < 0 || i >= len(a.ps.states) {
		return
	}
	a.withPane(i, func() { a.goTo(pl) })
	a.ps.want = i
	if a.frame != nil {
		a.frame()
	}
}

// openInSide shows place pl in the other pane: the second, made if there is not one yet.
func (a *app) openInSide(pl place) {
	a.openAtPane(1-a.ps.active, pl)
}

// closePane takes pane i away; the other is left, and in use.
func (a *app) closePane(i int) {
	if len(a.ps.states) < 2 || i < 0 || i >= len(a.ps.states) {
		return
	}
	a.flush()
	a.ps.states[a.ps.active] = a.paneState
	keep := a.ps.active
	if keep == i {
		keep = 1 - i
	}
	a.ps.states = []paneState{a.ps.states[keep]}
	a.ps.active, a.ps.cur, a.ps.want = 0, 0, -1
	a.paneState = a.ps.states[0]
	a.refocus = true
	if a.frame != nil {
		a.frame()
	}
}

// panesView builds the panes side by side, each in its own state, and gives the use to the one
// the user went to.
func (a *app) panesView(c *ui.Context, th *richtext.Theme) {
	n := a.paneCount()
	if len(a.ps.states) == 0 {
		a.ps.states = []paneState{{}}
	}
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		for i := 0; i < n; i++ {
			if i > 0 {
				ui.Box(c.Key("pane-divider")).Width(1).Background(ryugo.From(c).Colors.Hairline)
			}
			i := i
			a.withPane(i, func() { a.paneColumn(c, th, i, n) })
		}
	})
	if a.ps.settle > 0 {
		a.ps.settle--
		c.Invalidate() // another frame, for the focus to arrive
	}
	if a.ps.want >= 0 && a.ps.want != a.ps.active {
		a.usePane(a.ps.want)
	}
	a.ps.want = -1
}

// noteVisit asks for the use of the pane that is being built, when something is opened in it.
func (a *app) noteVisit() {
	if a.ps.cur != a.ps.active {
		a.ps.want = a.ps.cur
	}
}

// paneColumn is pane i of n: its title bar, its screen - a page, a collection, the settings, the
// trash, the tasks, a search, a tag - and the bar over it.
func (a *app) paneColumn(c *ui.Context, th *richtext.Theme, i, n int) {
	col := ui.Column(c.Key("pane:" + strconv.Itoa(i))).Grow(1).MinWidth(0).Focusable().FocusRing(false)
	// what is drawn later, after the view is built, is drawn with this pane's state in the app: it is
	// swapped in as the pane starts to be drawn, and out as it is done
	col.Draw(func(*ui.Painter, ui.Rect) { a.paintIn(i) })
	col.DrawOver(func(*ui.Painter, ui.Rect) { a.paintOut(i) })
	if i == a.ps.active && a.ps.focus && (a.page == nil || a.noPage()) { // no text to take the keyboard: the pane takes it
		col.Focus()
		a.ps.focus = false
	}
	if i == a.ps.active && a.ps.focus && a.page != nil && !a.noPage() {
		a.ps.focus = false
	}
	col.Children(func() {
		// a page and a collection scroll as a whole, from the top of the window, under the title bar
		overBar := !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && !a.settingsOpen && !a.statsScreen && (a.coll != nil || a.page != nil)
		if !overBar {
			a.titleBar(c)
		}
		// the page scrolls over the whole width, its bar at the window's edge:
		// the margins are inside it
		padX := float32(32)
		if !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && !a.settingsOpen && !a.statsScreen && (a.coll != nil || a.page != nil) {
			padX = 0
		}
		topPad := float32(pageTop - barH + 8)
		if a.coverKind(a.page) != coverKindNone && !a.noPage() { // the cover starts under the bar
			topPad = 0
		}
		a.barInset, a.topSpace = 0, 0
		if overBar { // it scrolls from the top of the window, under the bar, and starts below it
			a.barInset, a.topSpace, topPad = barH, barH+topPad, 0
			if a.coverKind(a.page) != coverKindNone && !a.noPage() { // a cover begins at the very top, behind the bar
				a.topSpace = 0
			}
		}
		pageRow := ui.Row(c).Grow(1).Justify(ui.Center).Padding(topPad, padX, 0, padX)
		defer func() { a.pageR = pageRow.LayoutBounds() }()
		pageRow.Children(func() {
			if a.settingsOpen {
				a.focused = false
				a.settingsScreen(c)
				return
			}
			if a.statsScreen {
				a.focused = false
				a.statsView(c)
				return
			}
			if a.trashOpen {
				a.focused = false
				a.trashView(c)
				return
			}
			if a.tag != "" {
				a.focused = false
				a.tagView(c)
				return
			}
			if a.tasksOpen {
				a.focused = false
				a.tasksView(c)
				return
			}
			if a.searchOpen {
				a.focused = false
				a.searchView(c)
				return
			}
			if a.coll != nil {
				a.focused = false
				a.collectionView(c)
				return
			}
			if a.page == nil {
				a.emptyState(c, iconPage, "Bir sayfa seç", "Soldaki ağaçtan bir sayfa aç, koleksiyon satırındaki + ile de yenisini yaz.", "", nil)
				return
			}
			a.pageView(c, th)
		})
		if overBar {
			a.topBar(c)
		}
		if n > 1 { // a pane can be closed when there are two
			if ryugo.Button(c.Key("pane-close"), "Bölmeyi kapat", ryugo.ButtonOptions{Icon: iconClose, IconOnly: true, Size: ryugo.Sm}).Absolute().Top(4).Right(10).Clicked() {
				a.afterView = append(a.afterView, func() { a.closePane(i) })
			}
		}
	})
	if i != a.ps.active && a.ps.settle == 0 && col.FocusWithin() { // the user went into it
		a.ps.want = i
	}
}

// paintIn puts pane i's state in the app for what is drawn of it; paintOut puts the pane in use back.
func (a *app) paintIn(i int) {
	if i == a.ps.active || i >= len(a.ps.states) || a.ps.cur != a.ps.active {
		return
	}
	a.ps.states[a.ps.active] = a.paneState
	a.paneState = a.ps.states[i]
	a.ps.cur = i
}

func (a *app) paintOut(i int) {
	if a.ps.cur != i || i == a.ps.active {
		return
	}
	a.ps.states[i] = a.paneState
	a.paneState = a.ps.states[a.ps.active]
	a.ps.cur = a.ps.active
}

// sideZone is the pane a page held over the window's content would go into, or -1: with one pane,
// the right part of the content is a second pane; with two, each half is its pane.
func (a *app) sideZone() int {
	left, right := a.sidebarShown, a.winRect.W
	if right <= left+80 {
		return -1
	}
	frac := (a.drag.pos.X - left) / (right - left)
	if frac < 0 {
		return -1
	}
	if a.paneCount() == 1 {
		if frac > 0.62 {
			return 1
		}
		return -1
	}
	if frac < 0.5 {
		return 0
	}
	return 1
}

// aimSide notes where the page held is over a pane's side: let go there, the page opens in that pane.
func (a *app) aimSide() {
	d := &a.drag
	d.sideOn = false
	if d.page == nil {
		return
	}
	if z := a.sideZone(); z >= 0 {
		d.sideOn, d.side, d.target = true, z, nil
	}
}

// sideZoneRect is the part of the window the pane z covers, for the glow shown while a page is held over it.
func (a *app) sideZoneRect(z int) ui.Rect {
	left, right := a.sidebarShown, a.winRect.W
	h := a.winRect.H
	if a.paneCount() == 1 {
		x := left + 0.62*(right-left)
		return ui.Rect{X: x, Y: 0, W: right - x, H: h}
	}
	mid := left + (right-left)/2
	if z == 0 {
		return ui.Rect{X: left, Y: 0, W: mid - left, H: h}
	}
	return ui.Rect{X: mid, Y: 0, W: right - mid, H: h}
}

// sideGlow is the glow over the pane a page held would go into.
func (a *app) sideGlow(c *ui.Context) {
	d := &a.drag
	if d.page == nil || !d.sideOn {
		return
	}
	cl := ryugo.From(c).Colors
	r := a.sideZoneRect(d.side)
	label := "Yan bölmede aç"
	if a.paneCount() > 1 {
		label = "Bu bölmede aç"
	}
	ui.Overlay(c, func() {
		c.Key("side-glow")
		z := ui.Column(c).Absolute().Left(r.X+6).Top(r.Y+30).Width(r.W-12).Height(r.H-36).Radius(14).PassThrough().
			AlignItems(ui.Center).Justify(ui.Center).Background(cl.ColorAccent500.Alpha(0.10)).Border(2, cl.ColorAccent500.Alpha(0.55))
		z.Children(func() {
			ui.Text(c, label).FontSize(15).FontWeight(650).TextColor(cl.ColorAccent500)
		})
	})
}

// openHereInSide shows what the pane in use shows in the other pane too: two views of it side by side.
func (a *app) openHereInSide() {
	if pl := a.here(); pl.kind != "" {
		a.openInSide(pl)
	}
}

// nextPane goes to the other pane.
func (a *app) nextPane() {
	if a.paneCount() > 1 {
		a.usePane(1 - a.ps.active)
	}
}

// closeThisPane closes the pane in use.
func (a *app) closeThisPane() { a.closePane(a.ps.active) }

// paneCommands are the palette's commands of the panes.
func (a *app) paneCommands(cmd func(title, keys string, glyph *ui.SVG, run func()) palItem) []palItem {
	out := []palItem{
		cmd("Yan bölmede aç: bu ekran", "⌥⌘↵", iconPanel, a.openHereInSide),
		cmd("Yan bölmede aç: Ayarlar", "", iconSettings, func() { a.openInSide(place{kind: "settings"}) }),
		cmd("Yan bölmede aç: Görevler", "", iconTask, func() { a.openInSide(place{kind: "tasks"}) }),
		cmd("Yan bölmede aç: Çöp", "", iconTrash, func() { a.openInSide(place{kind: "trash"}) }),
	}
	if a.paneCount() > 1 {
		out = append(out,
			cmd("Bölmeler arası geç", "⌥⌘→", iconPanel, a.nextPane),
			cmd("Bölmeyi kapat", "⌥⌘W", iconClose, a.closeThisPane),
		)
	}
	return out
}

// paneRef is what a pane that is not in use shows, by where it is, to be found again when the
// library is read again from disk.
type paneRef struct{ page, coll string }

// paneRefs notes what the panes not in use show.
func (a *app) paneRefs() map[int]paneRef {
	refs := map[int]paneRef{}
	for i, st := range a.ps.states {
		if i == a.ps.active {
			continue
		}
		var r paneRef
		if st.page != nil {
			r.page = st.page.Dir
		}
		if st.coll != nil {
			r.coll = st.coll.Dir
		}
		refs[i] = r
	}
	return refs
}

// restorePanes finds again, in the library read anew, what the panes not in use showed: a page or
// a collection that is no more leaves the pane empty.
func (a *app) restorePanes(refs map[int]paneRef) {
	for i, r := range refs {
		if i >= len(a.ps.states) {
			continue
		}
		st := &a.ps.states[i]
		st.page, st.coll, st.doc = nil, nil, nil
		for _, c := range a.lib.Collections {
			if r.coll != "" && c.Dir == r.coll {
				st.coll = c
			}
			for _, p := range c.All() {
				if r.page != "" && p.Dir == r.page {
					st.page, st.doc = p, a.docs[p]
				}
			}
		}
	}
}
