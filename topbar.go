package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The header: a region of the window of its own, as the sidebar, the content and the status bar
// are. It is a row across the top of the content, above every pane, as high as the window's own
// title bar and never less than headerH, with a thin line under it. Nothing scrolls under it and
// nothing moves it: one pane or many, a page or the settings, it is the same row in the same place.
// It moves the window; it says what the pane in use shows, and holds the panel's button and the
// window's controls, in the row, not over it. When the sidebar is folded, its buttons (which sit
// over the sidebar's top) lie at the header's left end, and the header leaves them their room.

const headerH = 38

// headerHeight is the header's height: the window's title bar, at least headerH.
func (a *app) headerHeight(c *ui.Context) float32 {
	if phone { // a finger's buttons
		return max(a.titleBar(c).Height, 50)
	}
	return max(a.titleBar(c).Height, headerH)
}

// header draws the header.
func (a *app) header(c *ui.Context) {
	cl := ryugo.From(c).Colors
	t := c.Theme()
	h := a.headerHeight(c)
	ui.Column(c.Key("header")).FillWidth().Height(h).Label(locales.SidebarTopBar()).Background(a.pageBg).Children(func() {
		ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Center).Padding(0, 8, 0, 14).Gap(6).DragWindow().Children(func() {
			if phone {
				a.phoneHeader(c)
				return
			}
			if a.sidebarShown < 1 { // the sidebar's buttons are here, over the header: their room
				ui.Box(c).Width(a.titleBar(c).Left + sidebarButtonsW)
			}
			if title := a.headerTitle(); title != "" {
				ui.Text(c, title).FontSize(12).FontWeight(560).TextColor(t.TextMuted).MaxLines(1).Grow(1).MinWidth(0)
			} else {
				ui.Box(c).Grow(1)
			}
			a.updateChip(c) // a version that waits (updatepanel.go)
			a.windowControls(c)
		})
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
	})
}

// contentTop is where the sidebar, the panes and the side panel start: under the header, or in the window's
// frame, which has no header in its card, at the card's top.
func (a *app) contentTop(c *ui.Context) float32 {
	if a.wframe.on {
		return 0
	}
	return a.headerHeight(c)
}

// headerTitle is what the pane in use shows, as its tab says it.
func (a *app) headerTitle() string { return a.paneTitle() }
