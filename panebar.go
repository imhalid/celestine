package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The pane's bar (#468): a thin strip at the top of each pane's card, in place of the tabs the frame's top edge
// held. At its left the pane's title, which comes in once the page's own big title has scrolled out of sight; at its
// right the page's menu (the one its title's right click opens) and a close button; the rest of it is where the pane
// is held. With two panes or more, the bar dragged carries the pane to another place in the Dock, as a page dragged
// from the sidebar goes there; with one, it moves the window, as the frame's edge does. The X closes the pane, and
// with one pane it closes the page: its collection is shown.
//
// The panes stand side by side, never on top of each other: with no tabs, a pane stacked in an area would have
// nothing to show it by. A pane the Dock puts in an area's stack goes beside it (unstackPanes).

// paneBarH is how tall the pane's bar is: on macOS the room under the window's buttons the card keeps anyway
// (noTabHeaderHeight), where the window draws its own controls a strip of its own in the card.
func (a *app) paneBarH() float32 {
	switch {
	case !a.wframe.on || phone:
		return 0
	case !ownWindowControls:
		return frameTopH - frameW
	default:
		return 30
	}
}

// paneBar draws pane i's bar at the top of its column col.
func (a *app) paneBar(c *ui.Context, col ui.Element, pn *paneState, i int, h float32) {
	t := ryugo.From(c)
	cl := t.Colors
	a.ps.ensureIDs()
	id := a.ps.ids[i]
	many := len(a.ps.states) > 1 && a.ps.layout != nil
	// the window's buttons and the sidebar's lie over the left end of the leftmost pane's bar on macOS: its title
	// starts after them
	left := float32(14)
	if !ownWindowControls {
		if cb := col.LayoutBounds(); cb.W > 0 && a.topLeftR.W > 0 && cb.X < a.topLeftR.X+a.topLeftR.W {
			left = max(left, a.topLeftR.X+a.topLeftR.W-cb.X+10)
		}
	}
	bar := ui.Row(c.Key("pane-bar")).FillWidth().Height(h).Shrink(0).Padding(0, 8, 0, left).Gap(4).AlignItems(ui.Center).
		Label(locales.PanesBar())
	pn.headDragR = bar.LayoutBounds()
	if !many {
		bar.DragWindow() // one pane: the window, held by its top
	} else {
		a.dragPane(c, bar, pn, id)
	}
	bar.Children(func() {
		// the title, once the page's own has gone up out of sight; a screen without a big title of its own says it
		// all the time
		title := a.paneTitleOf(pn)
		show := float32(1)
		if (pn.page != nil && !a.noPageOf(pn)) || pn.coll != nil {
			if pn.headH == 0 || pn.pageScroll.Y < pn.headH-8 { // its own height not known yet: it is in sight
				show = 0
			}
		}
		if k := bar.AnimateTo("title", show, springSheet); k > 0.02 { // out of sight, it is not there at all
			ui.Text(c, title).FontSize(12.5).FontWeight(600).TextColor(cl.Text).MaxLines(1).TruncateMiddle().MinWidth(0).
				Shrink(1).Opacity(k).Translate(0, 4*(1-k))
		}
		ui.Box(c).Grow(1).Height(h) // where the bar is held
		if pn.page != nil && !a.noPageOf(pn) {
			p := pn.page
			if m := a.paneBarButton(c, "pane-menu", iconMore, locales.PanesMenu()); m.Clicked() {
				b := m.LayoutBounds()
				a.showContext(b.X, b.Y+b.H+4, a.pageMenu(p, true))
			}
		}
		closes := many || (pn.page != nil && !a.noPageOf(pn) && pn.page.Coll != nil)
		if closes {
			label := locales.CloseThePane()
			if !many {
				label = locales.PanesClosePage()
			}
			if x := a.paneBarButton(c, "pane-close", iconWinClose, label); x.Clicked() {
				a.later(func() { a.closeFromBar(pn, i, id) })
			}
		}
	})
}

// paneBarButton is a small icon button of the pane's bar: faint until the pointer is on it.
func (a *app) paneBarButton(c *ui.Context, key string, glyph *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.Box(c.Key(key)).Size(24, 24).Radius(rad(6)).Center().Shrink(0).Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	ryugo.Tooltip(c, b, label)
	if b.Hovered() {
		b.Background(t.Text.Alpha(0.08))
	}
	b.Children(func() { ui.Icon(c, glyph).Size(14, 14).TextColor(t.TextMuted) })
	return b
}

// closeFromBar is the X of pane i's bar: with other panes the pane goes; alone, its page closes, and its collection
// is shown in its place.
func (a *app) closeFromBar(pn *paneState, i int, id string) {
	if len(a.ps.states) > 1 && a.ps.layout != nil {
		a.flush()
		a.ps.layout.ClosePane(id)
		return
	}
	if pn.page != nil && pn.page.Coll != nil {
		a.openCollectionIn(pn, pn.page.Coll)
	}
}

// dragPane makes pane id's bar carry the pane: held and moved a little, it goes to the Dock as something dragged
// in from outside does (DockOptions.Drop), the compass showing where it would go, and let go over a place it moves
// there. Its title goes by the pointer.
func (a *app) dragPane(c *ui.Context, bar ui.Element, pn *paneState, id string) {
	k := &a.paneDrag
	dx, dy, pressed := bar.Dragged()
	px, py, _ := bar.PointerPosition()
	b := bar.LayoutBounds()
	at := ui.Point{X: b.X + px, Y: b.Y + py}
	switch {
	case pressed && k.id != id:
		*k = paneDragState{id: id}
	case pressed:
		k.moved += abs32(dx) + abs32(dy)
		if k.moved > 4 && a.carry.drop == nil {
			p := a.ps.layout.FindPane(id)
			if p == nil {
				return
			}
			a.carry = dockCarry{id: id, title: a.paneTitleOf(pn), glyph: a.paneGlyphOf(pn), drop: &ryugo.DockDrop{Pane: p}}
			k.active = true
		}
		if k.active && a.carry.drop != nil {
			a.carry.drop.At = at
			k.pos = at
		}
	case k.id == id:
		if k.active { // let go: the Dock reads it as it starts to be built, and this bar is built inside it: next frame
			a.later(func() { a.letCarry() })
		}
		*k = paneDragState{}
	}
}

// paneDragState is a pane being carried by its bar.
type paneDragState struct {
	id     string   // the pane, by its id
	moved  float32  // how far the pointer went, pressed
	active bool     // far enough: it is carried
	pos    ui.Point // the pointer, in the window
}

// paneDragView draws the title of a pane carried by its bar by the pointer, as a page carried from the sidebar has.
func (a *app) paneDragView(c *ui.Context) {
	if a.paneDrag.active && a.carry.drop != nil {
		a.carryChip(c, a.paneDrag.pos)
	}
}

// unstackPanes puts every pane the Dock holds in an area's stack in an area of its own beside it: with no tabs, a
// pane under another could not be shown. It runs before the Dock is built, for a pane let go on an area's middle and
// for a layout saved when the panes had tabs.
func (a *app) unstackPanes() {
	l := a.ps.layout
	if l == nil {
		return
	}
	for {
		var stacked *ryugo.Pane
		var beside string
		areas := map[string]string{} // an area's first pane, by the area
		for _, tab := range l.Tabs("") {
			if first, ok := areas[tab.Area]; ok {
				stacked, beside = tab.Pane, first
				break
			}
			areas[tab.Area] = tab.Pane.ID
		}
		if stacked == nil {
			return
		}
		l.RemovePane(stacked.ID)
		l.AddPane(beside, ryugo.DockRow, stacked)
		l.ShowPane(stacked.ID)
	}
}
