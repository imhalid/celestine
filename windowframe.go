package main

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's frame. The content is a card inside the window, its corners rounded, and around it the window shows
// MyGo's material: Liquid Glass on macOS 26 (the popover material before it), Acrylic on Windows 11. Linux has no
// material: there the frame is a plain color a step off the sidebar's. The material is the window's
// (WindowOptions.Vibrancy); the app draws only what is on it.
//
// Its sides are thin. Its bottom edge holds the status bar. Its top edge is thin while one pane shows; with two or
// more it grows down on a spring and lists the panes as tabs, the one in use lit. macOS's window buttons cannot
// move once the window is made, so they stay where they are: in the strip while it is open, and in the card's
// corner, beside the sidebar's buttons, while it is not.

// frameW is the frame's width around the card, and frameR the card's corner, in DIPs.
const (
	frameW = 8
	frameR = 10
)

// frameGap is the room between the cards in the frame: the sidebar, the panes, the Dock's areas.
const frameGap = 6

// frameTopH is the top edge's height, the height of the window's own title bar's buttons and macOS's window
// buttons, which are centered in it (TrafficLightPosition); frameTabH is a tab's.
const (
	frameTopH = 42
	frameTabH = 30
)

// frameMaterial is the material the main window shows in its frame.
const frameMaterial = mygo.VibrancyGlass

// frameState is what a frame of the window knows of the window's frame.
type frameState struct {
	on    bool    // the frame is drawn
	strip float32 // how far the top edge's tabs are in, 0 to 1
}

// framed reports whether this frame of the window draws the frame: not in a browser's tab nor on a phone, where
// the window is the screen, and not while the first opening's stage reveals the app.
func (a *app) framed() bool {
	return runtime.GOOS != "js" && !phone && !a.wel.on
}

// frameColor is the frame's ground: clear where the window's material shows through it, else a step off the
// sidebar's surface toward the text, so that the frame shows beside the sidebar too, in light and in dark.
func (a *app) frameColor(c *ui.Context) ui.Color {
	if c.Vibrancy() {
		return ui.Transparent
	}
	t := c.Theme()
	return ryugo.Mix(t.Surface, t.Text, 0.08)
}

// frameTabs reports whether the top edge lists the panes: two or more of them.
func (a *app) frameTabs() bool { return a.wframe.on && len(a.ps.states) > 1 }

// frameTop is the frame's top edge, which moves the window: after macOS's window buttons the sidebar's buttons
// (fold, back, forward, the settings); the panes' tabs, coming in as the window is split; at the right end the side
// panel's button, and on Linux and Windows the window's own controls.
func (a *app) frameTop(c *ui.Context) {
	p := a.wframe.strip
	left := max(a.titleBar(c).Left-frameW, 6) // after macOS's window buttons
	ui.Row(c.Key("frame-top")).FillWidth().Height(frameTopH).Shrink(0).Padding(0, 4, 0, left).Gap(10).AlignItems(ui.Center).Label(locales.SidebarTopBar()).
		DragWindow().Children(func() {
		ui.Row(c.Key("frame-sidebar-buttons")).Gap(2).Shrink(0).AlignItems(ui.Center).Children(func() {
			a.sidebarButtons(c, frameW+left+3*(24+2)-2, frameTopH-4)
		})
		tabs := ui.Row(c.Key("frame-tabs")).Grow(1).MinWidth(0).Height(frameTabH).Gap(4).AlignItems(ui.Center)
		if p > 0.01 && a.ps.layout != nil {
			tabs.Opacity(p).Translate(0, -8*(1-p)).Children(func() {
				for _, tab := range a.ps.layout.Tabs("") { // in the Dock's order: its areas, and each area's stack
					if i := a.ps.index(tab.Pane.ID); i >= 0 {
						a.frameTab(c, i, tab.Pane.ID)
					}
				}
			})
		}
		if a.page != nil && !a.noPage() {
			if a.iconButton(c, iconPanel, locales.CommonSidePanel()+" (⇧⌘\\)") {
				a.togglePanel()
			}
		}
		a.windowControls(c)
	})
}

// frameTab is pane i's tab, the Dock's pane id: what it shows, lit while it is the one in use, with its close
// button while it is or the pointer is over it. A click shows the pane in its area and puts it in use; the close
// button closes it as the Dock's own tab would (Layout.ClosePane), and the panes follow (syncPanes).
func (a *app) frameTab(c *ui.Context, i int, id string) {
	t := c.Theme()
	var title string
	a.withPane(i, func() { title = a.paneTitle() })
	used := i == a.ps.active
	tab := ui.Row(c.Key("frame-tab:"+id)).Height(frameTabH).MaxWidth(240).MinWidth(0).Shrink(1).
		Padding(0, 6, 0, 12).Gap(4).AlignItems(ui.Center).Radius(rad(9)).Label(title).Role(ui.RoleTab)
	switch {
	case used:
		tab.Background(a.pageBg).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
	case tab.Hovered():
		tab.Background(t.Text.Alpha(0.06))
	}
	closed := false
	tab.Children(func() {
		col := t.TextMuted
		if used {
			col = t.Text
		}
		ui.Text(c, title).FontSize(12.5).FontWeight(560).TextColor(col).MaxLines(1).MinWidth(0).Grow(1)
		x := ui.Box(c.Key("close")).Size(18, 18).Radius(9).Center().Shrink(0).Label(locales.CloseThePane() + ": " + title).Role(ui.RoleButton)
		x.Opacity(x.AnimateTo("shown", map[bool]float32{true: 1}[used || tab.Hovered()], springSheet))
		if x.Hovered() {
			x.Background(t.Text.Alpha(0.08))
		}
		x.Children(func() { ui.Icon(c, iconWinClose).Size(11, 11).TextColor(t.TextMuted) })
		closed = x.Clicked()
	})
	switch {
	case closed:
		a.flush()
		a.ps.layout.ClosePane(id)
		c.Invalidate()
	case tab.Clicked():
		a.ps.layout.ShowPane(id)
		a.usePane(i)
	}
}

// frameBottom is the frame's bottom edge: the status bar, on the frame's ground, or a thin edge without it.
func (a *app) frameBottom(c *ui.Context) {
	if a.settings.HideStatus {
		ui.Box(c.Key("frame-bottom")).FillWidth().Height(frameW).Shrink(0)
		return
	}
	a.status(c)
}
