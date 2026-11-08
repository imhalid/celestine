package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// A page or a collection dragged from the sidebar over the panes goes into Ryugo's Dock as a tab
// dragged there does: the compass shows over the pane under the pointer - its middle makes it a tab
// of that pane's stack, its arms a pane beside it, left, right, above or below - and the window's
// edge targets dock it along a whole edge. With one pane, the Dock comes for the drag, and goes again
// if nothing went in.

// dockCarry is what is carried from the sidebar over the panes.
type dockCarry struct {
	pl    place
	title string
	glyph *ui.SVG
	id    string // the pane it would be, made once a drag
	drop  *ryugo.DockDrop
}

// carryDock notes that pl is held at p in the window: over the panes it is offered to the Dock, else
// it is not. It reports whether it is over the panes.
func (a *app) carryDock(pl place, title string, glyph *ui.SVG, p ui.Point) bool {
	k := &a.carry
	r := a.panesR
	over := !phone && r.W > 0 && p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
	if !over {
		k.drop = nil
		return false
	}
	if k.pl != pl || k.id == "" {
		a.ps.ensureIDs()
		*k = dockCarry{pl: pl, title: title, glyph: glyph, id: a.ps.newID()}
	}
	a.ensureLayout()
	if k.drop == nil {
		k.drop = &ryugo.DockDrop{Pane: &ryugo.Pane{ID: k.id, Kind: "pane", Title: title}}
	}
	k.drop.At = p
	return true
}

// letCarry lets what is carried go: over the panes, the Dock places it as the panes build. It reports
// whether it was over them.
func (a *app) letCarry() bool {
	if a.carry.drop == nil {
		a.carry = dockCarry{}
		return false
	}
	a.carry.drop.Let = true
	if a.frame != nil {
		a.frame()
	}
	return true
}

// dropCarry forgets what is carried, when the drag ended elsewhere.
func (a *app) dropCarry() {
	if a.carry.drop != nil && !a.carry.drop.Let {
		a.carry = dockCarry{}
	}
}

// carried is the Dock's part after it was built: what it placed becomes a pane of the app, showing
// what was carried, and in use.
func (a *app) carried() {
	k := &a.carry
	if k.drop == nil || !k.drop.Let {
		return
	}
	if k.drop.Placed && a.ps.index(k.id) < 0 {
		a.ps.states = append(a.ps.states, newPaneState())
		a.ps.ids = append(a.ps.ids, k.id)
		i := len(a.ps.states) - 1
		pl := k.pl
		a.withPane(i, func() { a.goTo(pl) })
		a.ps.want = i
	}
	*k = dockCarry{}
}

// ensureLayout gives the panes the Dock's tree, with one pane too, for something to be dragged into it.
func (a *app) ensureLayout() {
	if a.ps.layout != nil {
		return
	}
	if len(a.ps.states) == 0 {
		a.ps.states = []paneState{a.paneState}
	}
	a.ps.ensureIDs()
	a.ps.layout = ryugo.NewLayout(ryugo.DockArea(&ryugo.Pane{ID: a.ps.ids[0], Kind: "pane", Title: a.paneTitle()}))
}

// carryChip is what a collection carried over the panes shows by the pointer: its name on a raised tile.
func (a *app) carryChip(c *ui.Context, p ui.Point) {
	t := ryugo.From(c)
	cl := t.Colors
	ui.Overlay(c, func() {
		g := ui.Row(c.Key("carry-chip")).Absolute().Left(p.X+10).Top(p.Y+8).Height(28).Padding(0, 12, 0, 10).Gap(6).
			AlignItems(ui.Center).Radius(rad(8)).PassThrough().Background(cl.Card).Border(1, cl.Hairline).
			Shadow(0, 6, 18, 0, ui.RGB(0, 0, 0).Alpha(0.18)).Cursor(ui.CursorGrabbing)
		g.Children(func() {
			ui.Icon(c, a.carry.glyph).Width(14).Height(14).TextColor(cl.Muted)
			ui.Text(c, a.carry.title).FontSize(13).FontWeight(560).TextColor(cl.Text).NoWrap()
		})
	})
}
