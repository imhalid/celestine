package main

import (
	"time"

	"github.com/egoist/mygo/ui"
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
)

// titleBar is the bar across the top of the content: room for the window's
// controls when the sidebar is folded, the sidebar's button and where we are.
// It moves the window, as a title bar does.
func (a *app) titleBar(c *ui.Context, sidebarShown float32) {
	t := c.Theme()
	tb := c.TitleBar()
	left := max(tb.Left-sidebarShown, 0) + 10
	ui.Row(c).Height(max(tb.Height, barH)).Padding(0, 16, 0, left).Gap(6).AlignItems(ui.Center).DragWindow().Children(func() {
		if a.iconButton(c, iconSidebar, "Kenar çubuğu (⌘\\)") {
			a.toggleSidebar()
		}
		ui.Text(c, a.location()).FontSize(13).TextColor(t.TextMuted).MaxLines(1).Margin(0, 0, 0, 4)
	})
}

// location is where the content is: the collection, and the pages down to the open one.
func (a *app) location() string {
	switch {
	case a.coll != nil:
		return a.coll.Name
	case a.page != nil:
		return a.crumbs() + "  ›  " + a.page.Title
	}
	return "ryudit"
}

func (a *app) toggleSidebar() { a.sidebarClosed = !a.sidebarClosed }

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
				if a.iconButton(c, iconPlus, "Yeni koleksiyon") {
					if coll, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
						a.newPage(coll, nil)
					} else {
						a.saveErr = err.Error()
					}
				}
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(0, 8, 16, 8).Children(func() {
					for _, coll := range a.lib.Collections {
						a.collectionRow(c, coll)
						c.Key("coll-kids:" + coll.Dir)
						if coll.Open && len(coll.Pages) > 0 {
							ui.Column(c).Transition(branch).Clip().Children(func() {
								for _, p := range coll.Pages {
									a.pageRow(c, p)
								}
							})
						}
					}
				})
			})
		})
	})
	return w
}

// treeRow is a row of the tree: its chevron, icon and name, and a + that
// shows while the row is hovered. It reports a click on the row, on the
// chevron and on the +.
func (a *app) treeRow(c *ui.Context, key, label string, depth int, glyph *ui.SVG, name string, strong, selected, has, open, plus bool) (row, chev, add bool) {
	t := c.Theme()
	c.Key(key)
	r := ui.Row(c).Height(rowH).Padding(0, 6, 0, 4+float32(depth)*indentW).Gap(5).Radius(7).AlignItems(ui.Center).
		Label(label).Role(ui.RoleButton).Transition(rowMove)
	hover := r.Hovered()
	switch {
	case selected:
		r.Background(t.Selection)
	case hover:
		r.Background(t.SurfaceHover)
	default:
		r.Background(t.Surface.Alpha(0)) // a color to fade from
	}
	r.Children(func() {
		chev = a.chevron(c, has, open)
		ui.Icon(c, glyph).Width(15).Height(15).TextColor(t.TextMuted)
		tx := ui.Text(c, name).FontSize(13.5).MaxLines(1).Grow(1)
		if strong {
			tx.FontWeight(620)
		}
		if plus {
			add = a.revealButton(c, iconPlus, "Ekle: "+name, hover)
		}
	})
	return r.Clicked() && !chev && !add, chev, add
}

func (a *app) collectionRow(c *ui.Context, coll *Collection) {
	row, chev, add := a.treeRow(c, "coll:"+coll.Dir, "Koleksiyon: "+coll.Name, 0, iconStack, coll.Name, true,
		coll == a.coll, len(coll.Pages) > 0, coll.Open, true)
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
	row, chev, add := a.treeRow(c, "page:"+p.Dir, "Sayfa: "+p.Title, p.Depth()+1, iconPage, p.Title, false,
		p == a.page && a.coll == nil, len(p.Children) > 0, p.Open, p.Coll.SubPages)
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
		ui.Column(c).Transition(branch).Clip().Children(func() {
			for _, ch := range p.Children {
				a.pageRow(c, ch)
			}
		})
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
		Label(label).Role(ui.RoleButton).Tooltip(label).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
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
