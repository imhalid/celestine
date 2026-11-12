package main

import (
	"runtime"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's frame. The content is a card inside the window, its corners rounded, and around it the frame's own
// ground: a soft gradient from top to bottom with a tint toward the right, in the colors of the theme in use, the
// same on every platform (frameGround). A material of the window's would show what is behind it, near black over dark
// windows.
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

// frameState is what a frame of the window knows of the window's frame.
type frameState struct {
	on    bool    // the frame is drawn
	strip float32 // how far the top edge's tabs are in, 0 to 1
}

// framed reports whether this frame of the window draws the frame: not in a browser's tab nor on a phone, where
// the window is the screen. The first opening reveals the app framed already, in the opening's window too, so
// that nothing moves when it hands over to the app's window (welcome.go).
func (a *app) framed() bool {
	return runtime.GOOS != "js" && !phone
}

// frameGround is the frame's ground: a gradient from top through mid and low to bottom, and a tint toward the
// right over it.
type frameGround struct {
	top, mid, low, bottom, tint ui.Color
}

// frameGroundOf is the frame's ground in c's theme, so that it goes with the color theme in use: a step off the
// sidebar's surface toward the text, lit at the top with the theme's accent and fading out of it downward, and the
// accent faintly again toward the right; in light and in dark alike, as the theme's colors are.
func frameGroundOf(c *ui.Context) frameGround {
	t := c.Theme()
	base := ryugo.Mix(t.Surface, t.Text, 0.06)
	return frameGround{
		top:    ryugo.Mix(base, t.Accent, 0.16),
		mid:    ryugo.Mix(base, t.Accent, 0.07),
		low:    base,
		bottom: ryugo.Mix(base, t.Text, 0.04),
		tint:   t.Accent.Alpha(0.10),
	}
}

// paint paints the ground over r: the gradient in three steps, top to mid over the top 38%, mid to low over the
// next 32%, low to bottom over the rest, and the tint from clear at 35% of the width to itself at the right.
func (g frameGround) paint(p *ui.Painter, r ui.Rect) {
	h1, h2 := r.H*0.38, r.H*0.32
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: h1 + 1}, ui.LinearGradient{From: g.top, To: g.mid, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + h1, W: r.W, H: h2 + 1}, ui.LinearGradient{From: g.mid, To: g.low, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + h1 + h2, W: r.W, H: r.H - h1 - h2}, ui.LinearGradient{From: g.low, To: g.bottom, Angle: 180}, 0)
	clear := g.tint
	clear.A = 0
	p.FillGradient(r, ui.LinearGradient{From: clear, To: g.tint, Angle: 90, Start: 0.35, End: 1}, 0)
}

// frameCard gives el the look of a card on the frame's ground: its fill, lit faintly from the top and fading to
// itself a little way down (a sheen), a rim a step toward the text, and a soft shadow, two layers, a tight one under
// the edge and a wide one beneath; lift is how much it stands off, 1 for a card in the frame, more for one floating
// over another. fill may be translucent, as the sidebar's, to let the ground's tint through.
func frameCard(c *ui.Context, el ui.Element, fill ui.Color, radius, lift float32) {
	t := c.Theme()
	sheen, rim, shade := float32(0.06), t.Text.Alpha(0.07), float32(0.08)
	if t.Dark {
		sheen, rim, shade = 0.05, t.Text.Alpha(0.09), 0.32
	}
	a := float32(fill.A) / 255
	top := ryugo.Mix(fill.Alpha(1), t.Text, sheen).Alpha(a)
	el.Radius(radius).Border(1, rim).
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, min(1, shade*0.6*lift))).
		Shadow(0, 6*lift, 22*lift, -4, ui.RGBA(0, 0, 0, min(1, shade*lift))).
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: top, To: fill, Angle: 180, End: 0.45}, radius)
		})
}

// sidebarGlass is how much of the sidebar's surface covers the frame's ground under it.
const sidebarGlass = 0.82

// frameTabs reports whether the top edge lists the panes: two or more of them.
func (a *app) frameTabs() bool { return a.wframe.on && len(a.ps.states) > 1 }

// frameTop is the frame's top edge, which moves the window: after macOS's window buttons the sidebar's buttons
// (fold, back, forward, the settings); the panes' tabs, coming in as the window is split; at the right end, on Linux
// and Windows, the window's own controls. The side panel is opened from the page's menu ("Page info").
func (a *app) frameTop(c *ui.Context) {
	p := a.wframe.strip
	left := max(a.titleBar(c).Left-frameW, 6) // after macOS's window buttons
	ui.Row(c.Key("frame-top")).FillWidth().Height(frameTopH).Shrink(0).Padding(0, 4, 0, left).Gap(10).AlignItems(ui.Center).Label(locales.SidebarTopBar()).
		DragWindow().Children(func() {
		ui.Row(c.Key("frame-sidebar-buttons")).Gap(2).Shrink(0).AlignItems(ui.Center).Children(func() {
			a.sidebarButtons(c, frameW+left+3*(barBtnW+2)-2, frameTopH-4)
		})
		tabs := ui.Row(c.Key("frame-tabs")).Grow(1).MinWidth(0).Height(frameTabH).Gap(4).AlignItems(ui.Center)
		if p > 0.01 && a.ps.layout != nil {
			tabs.Opacity(p).Translate(0, -8*(1-p)).Children(func() {
				// The tabs stand together, a strip of their own sunk in the frame's ground, so they read as tabs
				// and not as words on the edge; the one in use is lifted out of it.
				t := c.Theme()
				ui.Row(c.Key("frame-tab-group")).MinWidth(0).Shrink(1).Height(frameTabH+6).Padding(3).Gap(2).AlignItems(ui.Center).
					Radius(rad(11)).Background(t.Text.Alpha(0.05)).Border(1, t.Border.Alpha(0.55)).Children(func() {
					for _, tab := range a.ps.layout.Tabs("") { // in the Dock's order: its areas, and each area's stack
						if i := a.ps.index(tab.Pane.ID); i >= 0 {
							a.frameTab(c, i, tab.Pane.ID)
						}
					}
				})
			})
		}
		a.updateChip(c) // a version that waits (updatepanel.go)
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
		Padding(0, 6, 0, 10).Gap(6).AlignItems(ui.Center).Radius(rad(8)).Label(title).Role(ui.RoleTab)
	switch {
	case used:
		tab.Background(a.pageBg).Border(1, t.Border.Alpha(0.7)).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.14))
	case tab.Hovered():
		tab.Background(t.Text.Alpha(0.07)).Border(1, t.Text.Alpha(0))
	default:
		tab.Border(1, t.Text.Alpha(0))
	}
	closed := false
	tab.Children(func() {
		col := t.TextMuted
		if used {
			col = t.Text
		}
		var glyph *ui.SVG // what the pane shows, at a glance
		a.withPane(i, func() { glyph = a.paneGlyph() })
		ui.Icon(c, glyph).Size(13, 13).TextColor(col).Shrink(0)
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
