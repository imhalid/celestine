package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's chrome: our own title bar (the system's is hidden, its window
// controls kept) and the sidebar of collections and pages, which folds away
// and comes back on a spring. Motion follows mizan-go's rules: arrivals on a
// spring, sizes and places without a bounce past where they are going.

const (
	sidebarW = 252
	barH     = 46
	rowH     = 30
	indentW  = 14
)

var (
	// springSheet is the sidebar folding and unfolding: no bounce, a width.
	springSheet = ui.Animation{Spring: ui.SpringOptions{Duration: 340 * time.Millisecond}}
	// springTurn turns a chevron: a little life.
	springTurn = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.12}}
	// rowMove slides the rows below a branch that opens or closes, and fades hovers.
	rowMove = ui.ElementTransition{Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond}}, Position: true, Colors: true}
	// branch is a page's children coming and going: unfolding from nothing, fading.
	branch = ui.ElementTransition{
		Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond}},
		Position:  true, Size: true,
		Enter: &ui.Motion{Collapse: true, Opacity: 0},
		Exit:  &ui.Motion{Collapse: true, Opacity: 0},
	}
	fadeQuick = ui.Animation{Duration: 140 * time.Millisecond}
	// rowStill and branchStill are the rows and branches in the frame a drop
	// applies: they are already shown where the drop puts them.
	rowStill    = ui.ElementTransition{Animation: rowMove.Animation, Colors: true}
	branchStill = ui.ElementTransition{Animation: branch.Animation, Enter: branch.Enter, Exit: branch.Exit}
)

// branch is the column of a collection's or a page's children: clipped as
// it unfolds and folds, but not while rows are dragged out of it and in.
func (a *app) branch(c *ui.Context) ui.Element {
	e := ui.Column(c)
	if a.drag.settle {
		e.Transition(branchStill)
	} else {
		e.Transition(branch)
	}
	if !a.moving() {
		e.Clip()
	}
	return e
}

// titleBar is the bar across the top of the content: empty, as Thymer's,
// it moves the window, as a title bar does. The sidebar's button is not in
// it: it stays by the window's controls (sidebarButton).
func (a *app) titleBar(c *ui.Context) {
	tb := c.TitleBar()
	ui.Row(c).Height(max(tb.Height, barH)).DragWindow()
}

// sidebarButton is the sidebar's button, in the same place whether the
// sidebar is shown or not: right of the window's controls.
func (a *app) sidebarButton(c *ui.Context) {
	tb := c.TitleBar()
	h := max(tb.Height, barH)
	ui.Row(c.Key("sidebar-button")).Absolute().Left(tb.Left + 6).Top((h - 26) / 2).Gap(2).Children(func() {
		if a.iconButton(c, iconSidebar, "Kenar çubuğu (⌘\\)") {
			a.toggleSidebar()
		}
		// back and forward, faint when there is nowhere to go
		for _, d := range []struct {
			dir   int
			glyph *ui.SVG
			label string
		}{{-1, iconBack, "Geri (⌘[)"}, {1, iconChevron, "İleri (⌘])"}} {
			ui.Box(c.Key(d.label)).Opacity(map[bool]float32{true: 1, false: 0.35}[a.canStep(d.dir)]).Children(func() {
				if a.iconButton(c, d.glyph, d.label) {
					a.step(d.dir)
				}
			})
		}
	})
}

func (a *app) toggleSidebar() {
	a.sidebarClosed = !a.sidebarClosed
	a.settings.SidebarClosed = a.sidebarClosed
	a.saveSettings()
}

// sidebar lists the collections and their pages as a tree: a collection's +
// makes a page in it, a page's + a page under it. Folded, it is as wide as
// nothing; its content keeps its width and slides out under the clip.
func (a *app) sidebar(c *ui.Context) float32 {
	t := c.Theme()
	target := float32(sidebarW)
	if a.sidebarClosed {
		target = 0
	}
	sb := ui.Column(c).Background(t.Surface).Clip()
	w := sb.AnimateTo("width", target, springSheet)
	sb.Width(w)
	if w < 0.5 && a.sidebarClosed {
		sb.Hidden(true)
	}
	tb := c.TitleBar()
	sb.Children(func() {
		ui.Column(c).Width(sidebarW).Grow(1).Children(func() {
			// The room of the window's controls: it moves the window too.
			ui.Box(c).Height(max(tb.Height, barH)).DragWindow()
			ui.Row(c).Padding(4, 10, 6, 18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "KOLEKSİYONLAR").FontSize(11).FontWeight(650).TextColor(t.TextMuted).Grow(1)
				if a.iconButton(c, iconCollapseAll, "Hepsini kapat") {
					a.collapseAll()
				}
				if a.iconButton(c, iconPlus, "Yeni koleksiyon") {
					if coll, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
						a.newPage(coll, nil)
					} else {
						a.saveErr = err.Error()
					}
				}
			})
			ui.Scroll(c).Grow(1).Children(func() {
				if a.drag.dropped > 0 {
					a.drag.dropped--
				}
				a.settleDrop()
				tree := ui.Column(c).Padding(0, 8, 16, 8)
				a.drag.x, a.drag.y, a.drag.over = tree.PointerPosition()
				a.drag.x -= 8 // the rows start inside the padding
				a.well(c, tree)
				a.rowsLast = append(a.rowsLast[:0], a.rows...)
				a.rows = a.rows[:0]
				tree.Children(func() {
					for _, coll := range a.lib.Collections {
						a.collectionRow(c, coll)
						c.Key("coll-kids:" + coll.Dir)
						if coll.Open && len(coll.Pages) > 0 {
							a.branch(c).Children(func() {
								for _, p := range coll.Pages {
									a.pageRow(c, p)
								}
							})
						}
					}
				})
				a.aim()
				a.ghost(c)
			})
			a.trashRow(c)
		})
	})
	return w
}

// rowSpec is what a row of the tree shows and does.
type rowSpec struct {
	key, label string
	depth      int
	glyph      *ui.SVG
	tint       *ui.Color // the icon's color, when not muted
	name       string
	strong     bool
	selected   bool
	has, open  bool   // children, and whether they show
	plus       bool   // a + for a new page in it or under it
	pin        func() // a collection's pin, shown as the row is hovered
	pinned     bool
	item       any               // the page or collection, to rename it in place
	menu       func() []menuItem // what a right click offers
}

// treeRow is a row of the tree: its chevron, icon and name - a field when
// it is being renamed - and a + that shows while the row is hovered. A right
// click opens its menu. It reports a click on the row, the chevron and the +.
func (a *app) treeRow(c *ui.Context, s rowSpec) (row, chev, add bool) {
	t := c.Theme()
	c.Key(s.key)
	r := ui.Row(c).Height(rowH).Padding(0, 6, 0, 4+float32(s.depth)*indentW).Gap(5).Radius(7).AlignItems(ui.Center).
		Label(s.label).Role(ui.RoleButton).Focusable().FocusRing(false)
	a.contextOn(r, s.menu)
	if a.treeFocus != nil && a.treeFocus == s.item { // the arrow keys came here
		r.Focus()
		a.treeFocus = nil
	}
	if r.Focused() && s.item != nil && !(a.renaming == s.item) {
		a.treeHeld = true
		a.treeKeys(r, s.item)
	}
	if a.drag.settle { // the drop: the rows are where they were shown
		r.Transition(rowStill)
	} else {
		r.Transition(rowMove)
	}
	pinHit := false
	hover := r.Hovered() && !a.moving()
	renaming := s.item != nil && a.renaming == s.item
	a.placeRow(c, r, len(a.rows), s.item)
	switch x := s.item.(type) {
	case *Page:
		a.rows = append(a.rows, treeEntry{page: x})
		if !renaming {
			a.dragRow(c, r, x)
		}
	case *Collection:
		a.rows = append(a.rows, treeEntry{coll: x})
		if !renaming {
			a.collDrag(r, x)
		}
	}
	switch {
	case renaming:
		r.Background(a.pageBg).Border(1, t.Accent)
	case s.selected:
		r.Background(ryugo.Mix(t.Surface, ryugo.From(c).Colors.Accent, 0.14)) // a tint, not the loud selection color
	case hover:
		r.Background(t.SurfaceHover)
	default:
		r.Background(t.Surface.Alpha(0)) // a color to fade from
	}
	r.Children(func() {
		chev = a.chevron(c, s.has, s.open)
		tint := t.TextMuted
		if s.tint != nil {
			tint = *s.tint
		}
		ui.Icon(c, s.glyph).Width(15).Height(15).TextColor(tint)
		if renaming {
			a.renameField(c)
			return
		}
		tx := ui.Text(c, s.name).FontSize(13.5).MaxLines(1).Grow(1)
		if s.strong {
			tx.FontWeight(620)
		}
		if s.pin != nil { // pinned: its pin shows and takes it off; not: it shows as the row is hovered
			c.Key("pin:" + s.key)
			if s.pinned {
				b := ui.Box(c).Width(22).Height(22).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Label("Sabitlemeyi kaldır: " + s.name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				if b.Hovered() {
					b.Background(t.SurfacePressed)
				}
				b.Children(func() { ui.Icon(c, iconPinned).Width(15).Height(15).TextColor(ryugo.From(c).Colors.Accent) })
				if b.Clicked() {
					pinHit = true
					a.later(s.pin)
				}
			} else if a.revealButton(c, iconPin, "Sabitle: "+s.name, hover) {
				pinHit = true
				a.later(s.pin)
			}
		}
		if s.plus {
			add = a.revealButton(c, iconPlus, "Ekle: "+s.name, hover)
		}
	})
	clicked := r.Clicked()
	if clicked {
		a.treeNav = false // a click is not the keys: the page it opens takes the focus
	}
	if clicked && a.drag.dropped > 0 { // the press was a drag, not a click
		clicked = false
	}
	if clicked && s.item != nil && a.cdrag.dropped == s.item {
		clicked = false
		a.cdrag.dropped = nil
	}
	if r.Hovered() == false && a.cdrag.dropped == s.item {
		a.cdrag.dropped = nil
	}
	return clicked && !chev && !add && !renaming && !pinHit, chev, add
}

// startRename turns item's row into a field holding its name.
func (a *app) startRename(item any, name string) {
	if p, ok := item.(*Page); ok && p.Coll.Journal { // a day is titled by the journal's format
		return
	}
	a.renaming, a.renameText, a.renameFocused = item, name, false
}

// renameField is the field of a row being renamed: Enter or leaving it
// keeps the new name, Escape the old one.
func (a *app) renameField(c *ui.Context) {
	in := ui.TextInputBase(c, &a.renameText).Label("Yeni ad").FontSize(13.5).Grow(1).AutoFocus()
	switch {
	case in.Shortcut(0, ui.KeyEscape):
		a.renaming = nil
	case in.Submitted():
		a.commitRename()
	case in.Focused():
		a.renameFocused = true
	case a.renameFocused: // the focus went elsewhere
		a.commitRename()
	}
}

func (a *app) commitRename() {
	name := strings.TrimSpace(a.renameText)
	item := a.renaming
	a.renaming = nil
	if name == "" {
		return
	}
	var err error
	switch x := item.(type) {
	case *Page:
		err = x.SetTitle(name)
		if x == a.page {
			a.title = name
		}
	case *Collection:
		err = x.Rename(name)
		if x == a.coll {
			a.collName = name
		}
	}
	if err != nil {
		a.saveErr = err.Error()
	}
}

func (a *app) collectionRow(c *ui.Context, coll *Collection) {
	tint := collTint(c, coll)
	row, chev, add := a.treeRow(c, rowSpec{key: "coll:" + coll.Dir, label: "Koleksiyon: " + coll.Name, glyph: collGlyph(coll), tint: &tint, name: coll.Name,
		strong: true, selected: coll == a.coll, has: len(coll.Pages) > 0, open: coll.Open, plus: !coll.Dynamic(), item: coll,
		pin: func() { a.pinCollection(coll, !coll.Pinned) }, pinned: coll.Pinned,
		menu: func() []menuItem { return a.collectionMenu(coll) }})
	switch {
	case add:
		a.newPage(coll, nil)
	case chev:
		coll.Open = !coll.Open
	case row:
		a.openCollection(coll)
	}
}

func (a *app) pageRow(c *ui.Context, p *Page) {
	row, chev, add := a.treeRow(c, rowSpec{key: "page:" + p.Dir, label: "Sayfa: " + p.Title, depth: p.Depth() + 1, glyph: iconPage, name: p.Title,
		selected: p == a.page && !a.noPage(), has: len(p.Children) > 0, open: p.Open, plus: p.Coll.SubPages, item: p,
		menu: func() []menuItem { return a.pageMenu(p, true) }})
	switch {
	case add:
		a.newPage(p.Coll, p)
	case chev:
		p.Open = !p.Open
	case row:
		a.open(p)
	}
	c.Key("kids:" + p.Dir)
	if p.Open && len(p.Children) > 0 {
		a.branch(c).Children(func() {
			for _, ch := range p.Children {
				a.pageRow(c, ch)
			}
		})
	}
}

// trashPage moves p to the library's trash; when it, or a page under it, is
// open, the page above it opens, or the collection.
func (a *app) trashPage(p *Page) {
	open := false
	for q := a.page; q != nil; q = q.Parent {
		open = open || q == p
	}
	if open {
		a.flush()
	}
	item, err := a.lib.TrashPage(p)
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	a.trashed = item
	if open {
		a.page, a.doc = nil, nil
		if p.Parent != nil {
			a.open(p.Parent)
		} else {
			a.openCollection(p.Coll)
		}
	}
}

// trashCollection moves c to the library's trash, and shows another one's first page.
func (a *app) trashCollection(c *Collection) {
	if c.Journal {
		a.notice = "Günlük çöpe taşınamaz"
		return
	}
	if a.page != nil && a.page.Coll == c {
		a.flush()
		a.page, a.doc = nil, nil
	}
	item, err := a.lib.TrashCollection(c)
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	a.trashed = item
	if a.coll == c {
		a.coll = nil
	}
	if a.page == nil && a.coll == nil {
		for _, o := range a.lib.Collections {
			a.openCollection(o)
			break
		}
	}
}

// chevron opens and closes a row, turning a quarter on a spring; it keeps
// its room when there is nothing below the row.
func (a *app) chevron(c *ui.Context, has, open bool) bool {
	t := c.Theme()
	b := ui.Box(c).Width(16).Height(16).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
	turn := float32(0)
	if open {
		turn = 90
	}
	angle := b.AnimateTo("turn", turn, springTurn)
	if has && b.Hovered() {
		b.Background(t.SurfacePressed)
	}
	b.Children(func() {
		if has {
			ui.Icon(c, iconChevron).Width(13).Height(13).TextColor(t.TextMuted).Rotate(angle)
		}
	})
	return has && b.Clicked()
}

// iconButton is a small square button of an icon, named for assistive technology.
func (a *app) iconButton(c *ui.Context, glyph *ui.SVG, label string) bool {
	t := c.Theme()
	b := ui.Box(c).Width(26).Height(26).Radius(7).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Role(ui.RoleButton).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	ryugo.Tooltip(c, b, label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	} else {
		b.Background(t.SurfaceHover.Alpha(0))
	}
	b.Children(func() { ui.Icon(c, glyph).Width(17).Height(17).TextColor(t.TextMuted) })
	return b.Clicked()
}

// revealButton is an icon button that fades in while its row is hovered.
func (a *app) revealButton(c *ui.Context, glyph *ui.SVG, label string, shown bool) bool {
	t := c.Theme()
	b := ui.Box(c).Width(22).Height(22).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Label(label).Role(ui.RoleButton)
	if b.Hovered() {
		b.Background(t.SurfacePressed)
	}
	b.Opacity(b.AnimateTo("shown", map[bool]float32{true: 1, false: 0}[shown], fadeQuick))
	b.Children(func() { ui.Icon(c, glyph).Width(15).Height(15).TextColor(t.TextMuted) })
	return b.Clicked()
}

var iconCollapseAll = icon(`<path d="m7.5 11.5 4.5-4.5 4.5 4.5M7.5 17.5l4.5-4.5 4.5 4.5"/>`)

// collapseAll folds every collection and page of the tree (not closing
// anything: what is open stays open).
func (a *app) collapseAll() {
	var fold func(p *Page)
	fold = func(p *Page) {
		p.Open = false
		for _, ch := range p.Children {
			fold(ch)
		}
	}
	for _, c := range a.lib.Collections {
		c.Open = false
		for _, p := range c.Pages {
			fold(p)
		}
	}
}

var iconPinned = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` +
	`<path d="M12 16.5V21"/><path d="M9.2 10.3a2 2 0 0 1-1.1 1.8l-1.7.9a2 2 0 0 0-1.1 1.8v.2a1 1 0 0 0 1 1h11.4a1 1 0 0 0 1-1v-.2a2 2 0 0 0-1.1-1.8l-1.7-.9a2 2 0 0 1-1.1-1.8V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H9a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>`))

// ---- sorting collections by hand

// collDragState is a collection row being dragged to another place.
type collDragState struct {
	coll    *Collection
	moved   float32
	active  bool
	before  *Collection // it goes before this one; nil: last of its kind
	dropped any         // the row just let go: the click that ends the drag is not one
}

// collDrag follows the press on a collection's row: dragged, the line shows
// where it would go among those pinned as it is, or among the others; let go
// it goes there.
func (a *app) collDrag(r ui.Element, coll *Collection) {
	cd := &a.cdrag
	if a.collRects == nil {
		a.collRects = map[*Collection]ui.Rect{}
	}
	a.collRects[coll] = r.LayoutBounds()
	dx, dy, pressed := r.Dragged()
	switch {
	case pressed && a.drag.page == nil && (cd.coll == nil || cd.coll == coll):
		if cd.coll != coll {
			*cd = collDragState{coll: coll}
		}
		if cd.moved += abs32(dx) + abs32(dy); cd.moved > 6 {
			cd.active = true
		}
		if cd.active {
			_, py, _ := r.PointerPosition()
			y := r.LayoutBounds().Y + py
			cd.before = nil
			for _, g := range a.lib.Collections {
				if g == coll || g.Pinned != coll.Pinned {
					continue
				}
				if rc, ok := a.collRects[g]; ok && y < rc.Y+rc.H/2 {
					cd.before = g
					break
				}
			}
		}
	case !pressed && cd.coll == coll:
		if cd.active {
			before := cd.before
			a.later(func() { // not while the tree is being built from the list it changes
				if err := a.lib.MoveCollectionBefore(coll, before); err != nil {
					a.saveErr = err.Error()
				}
			})
			cd.dropped = coll
		}
		cd.coll, cd.active, cd.moved, cd.before = nil, false, 0, nil
	}
}

// collDragView draws the line where the collection dragged would go.
func (a *app) collDragView(c *ui.Context) {
	cd := &a.cdrag
	if !cd.active || cd.coll == nil {
		return
	}
	var y float32
	var group []*Collection
	for _, g := range a.lib.Collections {
		if g.Pinned == cd.coll.Pinned && g != cd.coll {
			group = append(group, g)
		}
	}
	switch {
	case cd.before != nil:
		y = a.collRects[cd.before].Y - 2
	case len(group) > 0:
		last := a.collRects[group[len(group)-1]]
		y = last.Y + last.H + 2
		if group[len(group)-1].Open {
			return // under its pages: no line
		}
	default:
		return
	}
	cl := ryugo.From(c).Colors
	ui.Overlay(c, func() {
		ui.Box(c.Key("coll-drop")).Absolute().Left(14).Top(y).Size(sidebarW-28, 2).Radius(1).Background(cl.Accent)
	})
}

// ---- the keys in the tree

// treeKeys moves through the tree with the arrows, from the row focused: ↑ ↓
// to the page or collection before or after, opening it; → opens a branch,
// ← folds it, or goes up to what holds it.
func (a *app) treeKeys(r ui.Element, item any) {
	idx := -1
	for i, e := range a.rowsLast {
		if (e.page != nil && any(e.page) == item) || (e.coll != nil && any(e.coll) == item) {
			idx = i
		}
	}
	go_ := func(i int) {
		if i < 0 || i >= len(a.rowsLast) {
			return
		}
		e := a.rowsLast[i]
		a.treeNav = true
		if e.page != nil {
			a.open(e.page)
			a.treeFocus = any(e.page)
		} else {
			a.openCollection(e.coll)
			a.treeFocus = any(e.coll)
		}
	}
	switch {
	case r.Shortcut(0, ui.KeyDown):
		go_(idx + 1)
	case r.Shortcut(0, ui.KeyUp):
		go_(idx - 1)
	case r.Shortcut(0, ui.KeyRight):
		switch x := item.(type) {
		case *Collection:
			x.Open = true
		case *Page:
			x.Open = len(x.Children) > 0 || x.Open
		}
	case r.Shortcut(0, ui.KeyLeft):
		switch x := item.(type) {
		case *Collection:
			x.Open = false
		case *Page:
			switch {
			case x.Open && len(x.Children) > 0:
				x.Open = false
			case x.Parent != nil:
				a.treeNav, a.treeFocus = true, any(x.Parent)
				a.open(x.Parent)
			default:
				a.treeNav, a.treeFocus = true, any(x.Coll)
				a.openCollection(x.Coll)
			}
		}
	}
}

// ---- what the tree is like, kept

// treeSig is a number for what is open in the tree, to see it changed.
func (a *app) treeSig() uint64 {
	var h uint64 = 14695981039346656037
	mix := func(s string) {
		for i := 0; i < len(s); i++ {
			h = (h ^ uint64(s[i])) * 1099511628211
		}
		h = (h ^ 0xff) * 1099511628211
	}
	var walk func(ps []*Page)
	walk = func(ps []*Page) {
		for _, p := range ps {
			if p.Open {
				mix(p.Dir)
			}
			walk(p.Children)
		}
	}
	for _, c := range a.lib.Collections {
		if c.Open {
			mix(c.Dir)
		}
		walk(c.Pages)
	}
	return h
}

// keepTree keeps what is open in the tree in the settings, when it changed.
func (a *app) keepTree() {
	if sig := a.treeSig(); sig != a.treeSigSeen {
		first := a.treeSigSeen == 0
		a.treeSigSeen = sig
		if first && !a.settings.TreeSaved {
			return // what the library starts with is not a choice
		}
		var open []string
		var walk func(ps []*Page)
		walk = func(ps []*Page) {
			for _, p := range ps {
				if p.Open {
					open = append(open, p.Dir)
				}
				walk(p.Children)
			}
		}
		for _, c := range a.lib.Collections {
			if c.Open {
				open = append(open, c.Dir)
			}
			walk(c.Pages)
		}
		a.settings.TreeOpen, a.settings.TreeSaved = open, true
		a.saveSettings()
	}
}

// applyTree opens what the settings say was open, and the sidebar as it was.
func (a *app) applyTree() {
	a.sidebarClosed = a.settings.SidebarClosed
	if !a.settings.TreeSaved {
		a.collapseBigLibrary()
		return
	}
	open := map[string]bool{}
	for _, d := range a.settings.TreeOpen {
		open[d] = true
	}
	var walk func(ps []*Page)
	walk = func(ps []*Page) {
		for _, p := range ps {
			p.Open = open[p.Dir]
			walk(p.Children)
		}
	}
	for _, c := range a.lib.Collections {
		c.Open = open[c.Dir]
		walk(c.Pages)
	}
}

// later has fn done once the view is built: what changes the list the tree is
// made from is not done while it is being made from it.
func (a *app) later(fn func()) { a.afterView = append(a.afterView, fn) }

// runLater does what was left for after the view.
func (a *app) runLater() {
	fns := a.afterView
	a.afterView = nil
	for _, fn := range fns {
		fn()
	}
}

// bigLibraryPages is how many pages a library has, over which it starts with
// its collections closed: the tree is built of a row for every page of an open
// collection, each frame, and a library started in a mass of 7000 rows took
// most of a second to draw one frame.
const bigLibraryPages = 300

// collapseBigLibrary closes the collections of a big library that has no tree
// of its own yet (settings.TreeSaved), but the one of the page open.
func (a *app) collapseBigLibrary() {
	n := 0
	for _, c := range a.lib.Collections {
		n += len(c.All())
	}
	if n <= bigLibraryPages {
		return
	}
	for _, c := range a.lib.Collections {
		c.Open = a.page != nil && a.page.Coll == c
	}
}
