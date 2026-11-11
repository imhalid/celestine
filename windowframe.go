package main

import (
	"runtime"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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

// frameStripH is the top edge's height with the tabs in it, and frameTabH a tab's; the tabs' row is centered on
// macOS's window buttons (TrafficLightPosition), which are in line with it.
const (
	frameStripH = 46
	frameTabH   = 30
)

// frameMaterial is the material the main window shows in its frame.
const frameMaterial = mygo.VibrancyGlass

// frameState is what a frame of the window knows of the window's frame.
type frameState struct {
	on    bool    // the frame is drawn
	strip float32 // how far the top edge's strip of tabs is open, 0 to 1
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

// lightsRoom is the room the card leaves at its top left for macOS's window buttons: none while the strip, where
// they are then, is open.
func (a *app) lightsRoom(c *ui.Context) float32 {
	return a.titleBar(c).Left * (1 - a.wframe.strip)
}

// frameTop is the frame's top edge: thin, or the strip of the panes' tabs as it opens. It moves the window.
func (a *app) frameTop(c *ui.Context) {
	p := a.wframe.strip
	h := frameW + (frameStripH-frameW)*p
	top := ui.Box(c.Key("frame-top")).FillWidth().Height(h).Shrink(0).Clip().DragWindow()
	if p < 0.01 {
		return
	}
	left := max(a.titleBar(c).Left-frameW+4, 4) // after macOS's window buttons
	y := float32(frameStripH-frameTabH)/2 + 2
	top.Children(func() {
		ui.Row(c.Key("frame-tabs")).Absolute().Left(left).Right(4).Top(y).Height(frameTabH).Gap(4).AlignItems(ui.Center).
			Opacity(p).Translate(0, -10*(1-p)).Children(func() {
			for i := range a.ps.states {
				a.frameTab(c, i)
			}
		})
	})
}

// frameTab is pane i's tab: what it shows, lit while it is the one in use. A click goes to it.
func (a *app) frameTab(c *ui.Context, i int) {
	t := c.Theme()
	var title string
	a.withPane(i, func() { title = a.paneTitle() })
	used := i == a.ps.active
	tab := ui.Row(c.Key("frame-tab:"+strconv.Itoa(i))).Height(frameTabH).MaxWidth(220).MinWidth(0).Shrink(1).
		Padding(0, 12).AlignItems(ui.Center).Radius(rad(9)).Label(title).Role(ui.RoleTab)
	switch {
	case used:
		tab.Background(a.pageBg).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
	case tab.Hovered():
		tab.Background(t.Text.Alpha(0.06))
	}
	tab.Children(func() {
		col := t.TextMuted
		if used {
			col = t.Text
		}
		ui.Text(c, title).FontSize(12.5).FontWeight(560).TextColor(col).MaxLines(1).MinWidth(0)
	})
	if tab.Clicked() && !used {
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
