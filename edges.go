package main

import (
	"github.com/egoist/mygo/ui"
)

// The sidebar and the side panel are as wide as the user drags them (a strip at the edge toward the
// text), and never more than a share of the window: a small window keeps room for the text, and the
// search and the toolbar in it.

const (
	sidebarW      = 252 // the design's own widths
	panelW        = 280
	sidebarMin    = 170
	panelMin      = 210
	sideMax       = 520
	sidebarShare  = 0.30 // of the window, at most
	panelShare    = 0.32
	edgeGripWidth = 7
)

// edgeDrag says which edge is being dragged, so that its width follows the pointer at once.
type edgeDrag struct{ sidebar, panel bool }

// sideWidth is a width of the user's (0: the design's), within what the window allows.
func (a *app) sideWidth(user int, def, lo, share float32) float32 {
	w := def
	if user > 0 {
		w = float32(user)
	}
	if a.winRect.W > 0 {
		w = min(w, max(lo, a.winRect.W*share))
	}
	return max(lo, min(w, sideMax))
}

func (a *app) sidebarWidth() float32 {
	return a.sideWidth(a.settings.SidebarWidth, sidebarW, sidebarMin, sidebarShare)
}

func (a *app) panelWidth() float32 {
	return a.sideWidth(a.settings.PanelWidth, panelW, panelMin, panelShare)
}

// edgeGrip is the strip over the edge of the sidebar (right) or the side panel (left) that the pointer
// drags: the width is put in *user as it moves, and kept when it is let go; a double click goes back to
// the design's width. sign is +1 for an edge that grows the box to the right of the pointer.
func (a *app) edgeGrip(c *ui.Context, key string, right bool, now float32, user *int, on *bool) {
	g := ui.Box(c.Key(key)).Absolute().Top(0).Bottom(0).Width(edgeGripWidth).Cursor(ui.CursorResizeEW).Label(key).Role(ui.RoleButton)
	if right {
		g.Right(0)
	} else {
		g.Left(0)
	}
	dx, _, pressed := g.Dragged()
	if pressed {
		if !right {
			dx = -dx
		}
		if dx != 0 {
			*user = int(max(0, min(now+dx, sideMax)))
		}
		*on = true
		return
	}
	if *on { // let go: the width is kept
		*on = false
		a.saveSettingsSoon()
	}
	if g.DoubleClicked() {
		*user = 0
		a.saveSettingsSoon()
	}
}
