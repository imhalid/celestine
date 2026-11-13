package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
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
type edgeDrag struct {
	sidebar, panel bool
	handleY        float32 // where the handle was, along the edge, for it to fade there when the pointer leaves
}

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

// sidebarWidth is the sidebar's width. In the window's frame on macOS its buttons sit in its card, after the
// window's: it is never so narrow that they do not fit in it.
func (a *app) sidebarWidth() float32 {
	lo := float32(sidebarMin)
	if a.wframe.on && !ownWindowControls {
		lo = max(lo, a.barLeft-2*frameW+barLead+4*barBtnW+6)
	}
	return a.sideWidth(a.settings.SidebarWidth, sidebarW, lo, sidebarShare)
}

func (a *app) panelWidth() float32 {
	return a.sideWidth(a.settings.PanelWidth, panelW, panelMin, panelShare)
}

// edgeGrip is the strip over the edge of the sidebar (right) or the side panel (left) that the pointer
// drags: the width is put in *user as it moves, and kept when it is let go; a double click goes back to
// the design's width. box is the sidebar or the panel, whose edge it is.
//
// The strip shows a handle: a short bar on the edge, at the pointer's height. It comes in faint as the
// pointer nears the edge in the box, plain on the strip, and in the accent while it is dragged; elsewhere
// there is nothing, so that the edge is a plain edge until the pointer looks for it.
func (a *app) edgeGrip(c *ui.Context, key string, right bool, now float32, user *int, on *bool, box ui.Element) {
	g := ui.Box(c.Key(key)).Absolute().Top(0).Bottom(0).Width(edgeGripWidth).Cursor(ui.CursorResizeEW).Label(key).Role(ui.RoleButton)
	if right {
		g.Right(0)
	} else {
		g.Left(0)
	}
	dx, _, pressed := g.Dragged()
	a.edgeHandle(c, g, box, right, pressed)
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

// The edge's handle: as long and as wide as it is drawn, and how near the edge the pointer brings it in.
const (
	handleH    = 36
	handleW    = 4
	handleNear = 28
)

// edgeHandle draws the handle of the edge strip g of box (see edgeGrip, #459): how much it shows comes from how
// near the pointer is, on a spring, so that it fades in and out rather than blinking.
func (a *app) edgeHandle(c *ui.Context, g, box ui.Element, right, dragged bool) {
	px, py, over := box.PointerPosition()
	b := box.LayoutBounds()
	show := float32(0)
	if over && b.W > 0 {
		d := px // how far the pointer is from the edge, in the box
		if right {
			d = b.W - px
		}
		if d < handleNear {
			show = 0.45 * (1 - max(d, 0)/handleNear)
		}
	}
	if g.Hovered() {
		show = 1
	}
	if dragged {
		show = 1
	}
	k := g.AnimateTo("handle", show, springShift)
	if k < 0.02 {
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	color := cl.Muted
	if dragged {
		color = cl.Accent
	}
	if py <= 0 || !over { // the pointer left: the handle stays where it was as it fades
		py = a.edge.handleY
	} else {
		a.edge.handleY = py
	}
	y := min(max(py-handleH/2, 8), b.H-handleH-8)
	g.Children(func() {
		ui.Box(c.Key("handle")).Absolute().Left((edgeGripWidth-handleW)/2).Top(y).Width(handleW).Height(handleH).Radius(handleW/2).
			Background(color.Alpha(min(1, k))).PassThrough().Scale(1, 0.7+0.3*k)
	})
}
