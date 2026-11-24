package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A window has one pane or two, side by side, each showing a screen of its own: a page, a
// collection, the settings, the trash, the tasks, a search, a tag. What a pane shows and keeps is
// paneState, which the app has in itself, embedded, for the pane in use: all the code that says
// a.page or a.doc is of the pane in use. A pane that is not in use has its paneState put aside
// (panes); to build it, it is swapped in, built, and swapped out again.
type paneState struct {
	page          *Page              // the page open
	doc           *richtext.Document // its text
	title         string             // the title field's text
	coll          *Collection        // the collection whose page is shown, instead of a page
	collName      string             // its name field's text
	search        string             // its search field's text
	fresh         bool               // the page was just made: its title takes the focus
	version       uint64
	length        int
	caretAt       int
	wrote         int
	edited        bool
	erased        int // letters erased since the caret was drawn, which way (-1 Backspace, 1 Delete) and what they were (caretfx.go)
	eraseDir      int
	gone          string
	typed         string        // what was written since the caret was drawn
	prevText      richtext.Text // the text before the last edit, for what was erased
	editAt        time.Time
	dirty         bool
	trashOpen     bool // the trash is shown, instead of a page
	tasksOpen     bool // the task view is shown (tasks.go)
	searchOpen    bool // the search over the whole library is shown (search.go)
	searchQ       string
	searchFocus   bool
	taskDue       int                  // 0 any day, 1 today, 2 this week, 3 late
	taskFilter    int                  // 0 open, 1 done, 2 all
	taskList      *ui.ListState        // the list of tasks, a row for each in view (tasks.go)
	taskQuery     string               // what is typed in the tasks' search
	taskColl      string               // the collection the tasks are of (its Dir), "" all of them
	taskKindF     string               // the kind of the tasks shown (taskkinds.go): its id, "plain", or "" any
	taskFolded    map[*Page]bool       // the pages whose tasks are folded away
	taskLeaving   map[string]time.Time // tasks just done in the open ones, going out (taskRow): by taskKey, since when
	tag           string               // the tag (its key) whose pages are shown, instead of a page (tags.go)
	colDrag       colDragState
	cellEd        cellEdit
	titleCaret    titleCaretSpec // where the caret in the title is, to draw it (the title is an input)
	titleSeen     bool
	titleLen      int
	titleW        float32 // the title's room, as last laid out: what it wraps at (titlesize.go)
	selPrevA      int     // the selection before a pointer went down in the editor
	selPrevC      int
	boxes         []richtext.ParagraphBox
	paras         paraTable // the text's paragraphs, read once for each version of it (shown.go)
	linksH        float32   // the backlinks' height, last frame: each pane's own, as its page's
	dayH          float32   // and what a day of the journal shows over them (dayextras.go)
	focused       bool
	lastSave      string                  // the body as last written
	lastWords     int                     // the words it had, or -1 when not known
	style         richtext.ParagraphStyle // the caret paragraph's, last frame
	refocus       bool                    // the editor takes the keyboard focus in the next frame
	pageR         ui.Rect                 // the area the page is in, last frame: the editor reaches its left edge
	caretPara     int                     // where the caret was, last frame: its paragraph, its place in it, the paragraph's top
	caretRel      float32
	caretTop      float32
	sel           selBar          // the bar over a selection (selbar.go)
	edPressed     bool            // the pointer is down in the editor
	boardFocus    string          // the board column asked for, wide
	boardScroll   *ui.ScrollState // the board scrolled sideways: where its edges fade (edgeFades)
	pageScroll    *ui.ScrollState // the page scrolled as a whole (pageView)
	scrollR       ui.Rect         // its room in the window, last frame
	headDragR     ui.Rect         // non-scrolling top drag strip when tabs are closed on macOS
	barInset      float32         // the part of the top of the page the title bar lies over (the page scrolls under it)
	topSpace      float32         // what the page leaves above its title, in its scroll
	headH         float32         // the title's and properties' height, last frame
	titleFocused  bool
	retitle       titleEdit // a rename being typed in the title field, its links written when it is left (relink.go)
	edH           float32   // the editor's height, last frame
	revealWant    bool      // the caret moved: bring it into view
	statsVer      uint64    // the version of the text the counts are of
	statWords     int
	statChars     int
	settingsTab   int       // the settings' tab
	calDay        time.Time // the day open on the calendar, and whether it is
	calOpen       bool
	calNext       time.Time // the day to open once the open one has closed: one day goes before the next comes
	calGrid       ui.Rect
	calDrag       calDrag   // a page being dragged to another day (calendarview.go)
	calMonth      time.Time // the month the calendar view shows
	editorR       ui.Rect   // the editor's box, from its last frame
	bulk          bulkState // pages being chosen in a collection, to be done to all (bulk.go)
	settingsOpen  bool
	statsScreen   bool    // the page of figures is shown, instead of a page (statsscreen.go)
	profileScreen bool    // the profile is shown, instead of a page (profile.go)
	caretWin      ui.Rect // the caret in the window, last drawn
	find          finder  // finding in the page (find.go)
	// grip is the grip of the block under the pointer and the block dragged by it (blockui.go): each pane its own,
	// or a drag in one opens room in the other's text too (#480).
	grip gripState

	// What the pane's text drew this frame, and the last frame's, where a click is tested: each pane its own,
	// or a click in one is tested against what another drew (#390).
	tagHits    []tagHit // the tags (tags.go)
	tagSeen    []tagHit
	linkHits   []linkHit // the links (links.go)
	linkSeen   []linkHit
	picHits    []picHit // the pictures (images.go)
	picSeen    []picHit
	tableHits  []tableHit // the cells of the tables (tables.go)
	tableSeen  []tableHit
	borderHits []colBorder // the borders between columns
	borderSeen []colBorder
}

// newPaneState is a pane with nothing in it: its scrolls and lists are its own.
func newPaneState() paneState {
	return paneState{pageScroll: new(ui.ScrollState), boardScroll: new(ui.ScrollState), taskList: new(ui.ListState), grip: gripState{para: -1}}
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

	// With two panes or more, Ryugo's Dock lays them out: areas that split and resize and hold tabs. ids
	// say which pane is which (they stay when the others come and go), layout is the Dock's tree.
	ids    []string
	next   int
	layout *ryugo.Layout
	rects  map[string]ui.Rect // where each pane was drawn, for the zone a page held would go into
}

// newID is a name for a pane that has none.
func (s *paneSet) newID() string {
	s.next++
	return "p" + strconv.Itoa(s.next)
}

// ensureIDs gives every pane an ID.
func (s *paneSet) ensureIDs() {
	for len(s.ids) < len(s.states) {
		s.ids = append(s.ids, s.newID())
	}
	s.ids = s.ids[:len(s.states)]
}

// index is the place of pane id in the list, -1 when there is none.
func (s *paneSet) index(id string) int {
	for i, x := range s.ids {
		if x == id {
			return i
		}
	}
	return -1
}

// key names pane i for the elements built in it.
func (s *paneSet) key(i int) string {
	if i >= 0 && i < len(s.ids) {
		return s.ids[i]
	}
	return strconv.Itoa(i)
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
	a.resyncDoc()
	a.refocus = true // its text takes the keyboard
	a.ps.settle, a.ps.focus = 4, true
	if a.frame != nil {
		a.frame()
	}
}

// addPane makes a pane beside the one in use, at the end of the list, and says where it is.
func (a *app) addPane() int {
	if len(a.ps.states) == 0 {
		a.ps.states = []paneState{{}}
	}
	a.ps.ensureIDs()
	a.ps.states = append(a.ps.states, newPaneState())
	id := a.ps.newID()
	a.ps.ids = append(a.ps.ids, id)
	if a.ps.layout == nil {
		a.ps.layout = ryugo.NewLayout(ryugo.DockArea(&ryugo.Pane{ID: a.ps.ids[0], Kind: "pane"}))
	}
	a.ps.layout.AddPane(a.ps.ids[a.ps.active], ryugo.DockRow, &ryugo.Pane{ID: id, Kind: "pane"})
	return len(a.ps.states) - 1
}

// openAtPane shows place pl in pane i, making a new pane of the window if i is the number of panes (one
// past the last), and puts that pane in use.
func (a *app) openAtPane(i int, pl place) {
	if len(a.ps.states) == 0 { // only the one, not yet in the list
		a.ps.states = []paneState{{}}
	}
	if i == len(a.ps.states) { // one past the last: a new pane
		i = a.addPane()
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

// openInSide shows place pl in the next pane: the second, made if there is not one yet.
func (a *app) openInSide(pl place) {
	if phone { // one pane: it opens here
		a.goTo(pl)
		return
	}
	n := a.paneCount()
	if n == 1 {
		a.openAtPane(1, pl)
		return
	}
	a.openAtPane((a.ps.active+1)%n, pl)
}

// closePane takes pane i away; the next one is in use, if it was.
func (a *app) closePane(i int) {
	if len(a.ps.states) < 2 || i < 0 || i >= len(a.ps.states) {
		return
	}
	a.flush()
	a.ps.ensureIDs()
	if a.ps.layout != nil {
		a.ps.layout.RemovePane(a.ps.ids[i])
	}
	a.dropPane(i)
}

// dropPane forgets pane i: the pane in use goes to a neighbor if it was that one.
func (a *app) dropPane(i int) {
	a.ps.states[a.ps.active] = a.paneState
	a.endRename(a.ps.states[i].retitle) // a title being typed in it: its links follow (#560)
	a.ps.states[i].retitle = titleEdit{}
	active := a.ps.active
	switch {
	case i < active:
		active--
	case i == active:
		active = min(i, len(a.ps.states)-2)
		a.refocus = true
	}
	a.ps.states = append(a.ps.states[:i:i], a.ps.states[i+1:]...)
	a.ps.ids = append(a.ps.ids[:i:i], a.ps.ids[i+1:]...)
	if len(a.ps.states) < 2 {
		a.ps.layout = nil
	}
	a.ps.active, a.ps.cur, a.ps.want = active, active, -1
	a.paneState = a.ps.states[active]
	if a.frame != nil {
		a.frame()
	}
}

// syncPanes brings the panes in step with the Dock's tree: a tab the user closed there is a pane gone,
// a pane an Undo brought back is an empty pane again; with one left, the Dock goes.
func (a *app) syncPanes() {
	l := a.ps.layout
	if l == nil {
		return
	}
	a.ps.ensureIDs()
	have := map[string]bool{}
	for _, id := range l.PaneIDs() {
		have[id] = true
	}
	if len(have) == 0 { // the last tab was closed: the pane in use stays
		have[a.ps.ids[a.ps.active]] = true
	}
	for i := len(a.ps.ids) - 1; i >= 0 && len(a.ps.states) > 1; i-- {
		if !have[a.ps.ids[i]] {
			a.flush()
			a.dropPane(i)
		}
	}
	if a.ps.layout == nil {
		return
	}
	for _, id := range l.PaneIDs() { // a pane the Dock has and the app has not
		if a.ps.index(id) < 0 {
			a.ps.states = append(a.ps.states, newPaneState())
			a.ps.ids = append(a.ps.ids, id)
			if pl := a.here(); pl.kind != "" { // split by the user: two views of what the pane in use shows
				a.withPane(len(a.ps.states)-1, func() { a.goTo(pl) })
			}
		}
	}
	if len(a.ps.states) < 2 && a.carry.drop == nil { // one pane: no Dock, unless something is being dragged into it
		a.ps.layout = nil
	}
}

// paneTitle is what pane in use shows, in a word: the tab of its area.
func (a *app) paneTitle() string {
	switch {
	case a.settingsOpen:
		return locales.CommonSettings()
	case a.statsScreen:
		return locales.CommonStatistics()
	case a.profileScreen:
		return locales.CommonProfile()
	case a.trashOpen:
		return locales.CommonTrash()
	case a.tag != "":
		return "#" + a.tag
	case a.tasksOpen:
		return locales.CommonTasks()
	case a.searchOpen:
		return locales.CommonSearch()
	case a.coll != nil:
		return a.coll.Name
	case a.page != nil && !a.noPage():
		if a.page.Title != "" {
			return a.page.Title
		}
		return locales.CommonUntitled()
	}
	return locales.CommonEmpty()
}

// paneGlyph is the icon of what the pane in use shows, beside its title on its tab.
func (a *app) paneGlyph() *ui.SVG {
	switch {
	case a.settingsOpen:
		return iconSettings
	case a.statsScreen:
		return iconStats
	case a.profileScreen:
		return iconUser
	case a.trashOpen:
		return iconTrash
	case a.tag != "":
		return iconTag
	case a.tasksOpen:
		return iconTask
	case a.searchOpen:
		return iconSearch
	case a.coll != nil:
		return iconStack
	}
	return iconPage
}

// panesView builds the panes, each in its own state, and gives the use to the one the user went to.
// One pane fills the window; two or more are Ryugo's Dock: areas the user splits, resizes, stacks as
// tabs and closes.
func (a *app) panesView(c *ui.Context, th *richtext.Theme) {
	if len(a.ps.states) == 0 {
		a.ps.states = []paneState{{}}
	}
	a.syncPanes()
	a.keepPanes()
	var el ui.Element
	if phone { // one pane: the one in use, the whole window (mobile.go)
		el = ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.withPane(a.ps.active, func() { a.paneColumn(c, th, a.ps.active, 1, false) })
		})
	} else if a.ps.layout == nil {
		el = ui.Row(c).Grow(1).AlignItems(ui.Stretch)
		if a.wframe.on { // in the window's frame, a card of its own, as the Dock's areas are (windowframe.go)
			cardBg := a.pageBg
			if a.glassLive(c) {
				cardBg = cardBg.Alpha(glassOpacity(c, 0.50))
			}
			frameCard(c, el, cardBg, rad(a.cardR()), 1)
			el.Clip()
		}
		el.Children(func() {
			a.withPane(0, func() { a.paneColumn(c, th, 0, 1, false) })
		})
	} else {
		a.unstackPanes() // side by side, never on top of each other: the panes have no tabs (panebar.go)
		a.ps.ensureIDs()
		for i, id := range a.ps.ids { // the tab of each area says what its pane shows
			if p := a.ps.layout.FindPane(id); p != nil {
				a.withPane(i, func() { p.Title = a.paneTitle() })
			}
		}
		el = ryugo.Dock(c.Key("panes"), a.ps.layout, ryugo.DockOptions{Kinds: a.paneKinds(th), MinSize: 240, Host: a.host(), Drop: a.carry.drop, NoHeaders: a.wframe.on}).Grow(1)
		a.carried()
	}
	if b := el.LayoutBounds(); b.W > 0 { // a Dock just made is not laid out yet
		a.panesR = b
	}
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
// trash, the tasks, a search, a tag - and the bar over it. In the Dock (docked) the window's bar is
// above the areas and each area has its own header, so the pane has neither.
func (a *app) paneColumn(c *ui.Context, th *richtext.Theme, i, n int, docked bool) {
	// It takes the keyboard when there is no text to: a region named after what it shows, so that the focus
	// coming to it says where it is (#362).
	col := ui.Column(c.Key("pane:" + a.ps.key(i))).Grow(1).MinWidth(0).Focusable().FocusRing(false).Role(ui.RoleRegion).Label(a.paneTitle())
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
		// the page scrolls over the whole width, its bar at the window's edge:
		// the margins are inside it
		padX := pageMargin()
		if !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && !a.settingsOpen && !a.statsScreen && !a.profileScreen && (a.coll != nil || a.page != nil) {
			padX = 0
		}
		topPad := float32(pageTop - barH + 8)
		if phone { // a phone's screen starts near its header
			topPad = 12
		}
		if a.coverKind(a.page) != coverKindNone && !a.noPage() { // the cover starts under the bar
			topPad = 0
		}
		// a page and a collection scroll as a whole, from under the header, with their room above the title in
		// the scroll: a cover begins right under the header
		a.barInset, a.topSpace = 0, 0
		if !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && !a.settingsOpen && !a.statsScreen && !a.profileScreen && (a.coll != nil || a.page != nil) {
			a.topSpace, topPad = topPad, 0
		}
		a.headDragR = ui.Rect{}
		if h := a.paneBarH(); h > 0 { // its bar: its title, its menu and close; held, it moves the pane or the window (panebar.go)
			a.paneBar(c, col, i, h)
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
			if a.profileScreen {
				a.focused = false
				a.profileView(c)
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
				a.emptyState(c, iconPage, locales.PickAPage(), locales.OpenAPageFromTheTree(), "", nil)
				return
			}
			a.pageView(c, th)
		})
	})
	if a.ps.rects == nil {
		a.ps.rects = map[string]ui.Rect{}
	}
	a.ps.rects[a.ps.key(i)] = col.LayoutBounds()
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

// paneAtPoint is the pane drawn under p, -1 when none is.
func (a *app) paneAtPoint(p ui.Point) int {
	for i, id := range a.ps.ids {
		if r, ok := a.ps.rects[id]; ok && p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H {
			return i
		}
	}
	return -1
}

// openHereInSide shows what the pane in use shows in the other pane too: two views of it side by side.
func (a *app) openHereInSide() {
	if pl := a.here(); pl.kind != "" {
		a.openInSide(pl)
	}
}

// newPaneHere makes one more pane beside the one in use, showing what it shows: the Dock splits it
// further, and the panes stay as many as the user makes.
func (a *app) newPaneHere() {
	if pl := a.here(); pl.kind != "" {
		a.openAtPane(a.paneCount(), pl)
	}
}

// nextPane goes to the next pane, from the last to the first.
func (a *app) nextPane() {
	if n := a.paneCount(); n > 1 {
		a.usePane((a.ps.active + 1) % n)
	}
}

// closeThisPane closes the pane in use.
func (a *app) closeThisPane() { a.closePane(a.ps.active) }

// paneCommands are the palette's commands of the panes.
func (a *app) paneCommands(cmd func(title, keys string, glyph *ui.SVG, run func()) palItem) []palItem {
	// the screen in use, the panes in turn and closing one are the app's commands (appcmds.go)
	if phone {
		return nil
	}
	return []palItem{
		cmd("panes_settingsBeside", "", iconSettings, func() { a.openInSide(place{kind: "settings"}) }),
		cmd("panes_tasksBeside", "", iconTask, func() { a.openInSide(place{kind: "tasks"}) }),
		cmd("panes_trashBeside", "", iconTrash, func() { a.openInSide(place{kind: "trash"}) }),
	}
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
					st.page, st.doc = p, a.docFor(p) // a page whose text was never read has none yet: drawn so, it would fail
				}
			}
		}
	}
}

// paneSave is the panes as they are kept in the settings: the Dock's tree, what each pane shows, and the one in use.
type paneSave struct {
	Layout json.RawMessage      `json:"layout"`
	Places map[string][2]string `json:"places"` // a pane's ID to the kind and the place of what it shows
	Active string               `json:"active"`
}

// panesSnapshot is the panes in the form they are kept in: "" with one pane.
func (a *app) panesSnapshot() string {
	if a.ps.layout == nil || len(a.ps.states) < 2 {
		return ""
	}
	a.ps.ensureIDs()
	raw, err := a.ps.layout.MarshalJSON()
	if err != nil {
		return ""
	}
	save := paneSave{Layout: raw, Places: map[string][2]string{}, Active: a.ps.ids[a.ps.active]}
	for i, id := range a.ps.ids {
		a.withPane(i, func() {
			pl := a.here()
			save.Places[id] = [2]string{pl.kind, pl.dir}
		})
	}
	b, err := json.Marshal(save)
	if err != nil {
		return ""
	}
	return string(b)
}

// keepPanes writes the panes in the settings when they changed, and clears them when there is one pane again.
func (a *app) keepPanes() {
	if snap := a.panesSnapshot(); snap != a.settings.Panes {
		a.settings.Panes = snap
		a.saveSettingsSoon()
	}
}

// restorePaneLayout opens the panes kept in the settings, each at what it showed: the window opens as it was closed.
// What is no more (a page deleted) leaves its pane empty; a layout that cannot be read is left alone.
func (a *app) restorePaneLayout() {
	if a.settings.Panes == "" {
		return
	}
	var save paneSave
	if json.Unmarshal([]byte(a.settings.Panes), &save) != nil {
		return
	}
	l, err := ryugo.ParseLayout(save.Layout)
	if err != nil {
		return
	}
	ids := l.PaneIDs()
	if len(ids) < 2 || len(ids) > 12 {
		return
	}
	a.ps.states = make([]paneState, len(ids))
	for i := range a.ps.states {
		a.ps.states[i] = newPaneState()
	}
	a.ps.ids, a.ps.layout = ids, l
	for _, id := range ids { // new IDs go on from the highest
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "p")); err == nil {
			a.ps.next = max(a.ps.next, n)
		}
	}
	active := max(a.ps.index(save.Active), 0)
	a.ps.active, a.ps.cur, a.ps.want = active, active, -1
	a.paneState = a.ps.states[active]
	for i, id := range ids {
		if pl, ok := save.Places[id]; ok && pl[0] != "" {
			a.withPane(i, func() { a.goTo(place{pl[0], pl[1]}) })
		}
	}
}

// resyncDoc takes the pane in use up with its text, if the text was written while the pane was not in
// use (the same page in another pane, an agent, a sync): track would read the difference as a paste and
// make of it what a paste is made of.
func (a *app) resyncDoc() {
	if a.doc == nil || a.page == nil || a.doc.Version() == a.version {
		return
	}
	_, caret := a.doc.Selection()
	a.version, a.length, a.caretAt = a.doc.Version(), a.doc.Text().Len(), caret
	a.prevText = a.doc.Text()
	a.lastSave, a.lastWords, a.dirty = encodeBody(a.doc.Text()), -1, false
	a.statsVer = 0
}

// dropFromPanes takes what was thrown away out of the panes that are not in use: each shows the collection
// of its page instead (the pane in use is the caller's). page and coll say what is gone.
func (a *app) dropFromPanes(page func(*Page) bool, coll func(*Collection) bool) {
	for i := range a.ps.states {
		if i == a.ps.active {
			continue
		}
		st := &a.ps.states[i]
		if st.page != nil && (page(st.page) || coll(st.page.Coll)) {
			c := st.page.Coll
			st.page, st.doc, st.title, st.dirty = nil, nil, "", false
			if !coll(c) {
				st.coll = c
			}
		}
		if st.coll != nil && coll(st.coll) {
			st.coll = nil
		}
	}
}

// flushPanes saves the pages written in the panes that are not in use: one is dirty only when it was left
// so, but what is not saved is lost when the window closes.
func (a *app) flushPanes() {
	for i := range a.ps.states {
		if i != a.ps.active && a.ps.states[i].dirty && a.ps.states[i].page != nil {
			a.withPane(i, func() {
				if a.page != nil && a.doc != nil && a.dirty {
					a.save()
				}
			})
		}
	}
}
