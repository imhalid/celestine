package main

import (
	"fmt"
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
// Its sides are thin. Its bottom edge holds the status bar. Its top edge is thin however many panes show: each pane's
// card has a bar of its own (panebar.go, #468). macOS's window buttons stay in the card's corner, beside the
// sidebar's buttons.

// frameW is the frame's width around the card, and frameR the card's corner where the window has no rounded corners
// to follow (cardR), in DIPs.
const (
	frameW = 8
	frameR = 10
)

// cardR is the corner of the frame's cards: the window's own radius less the frame's width, so that the card's
// corners are concentric with the window's (on macOS 26 a window with a toolbar has 26, the cards 18), or frameR
// where the window's corners are square or the system does not tell them (#517).
func (a *app) cardR() float32 {
	if r := float32(a.winRadius) - frameW; r > 0 {
		return r
	}
	return frameR
}

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
	on bool // the frame is drawn
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
	sheen, rim, shade := float32(0.022), t.Text.Alpha(0.08), float32(0.08) // light: a white card would take a gray band at its top (#491)
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

// glassTint formats a CSS rgba() tint from bg for macOS Liquid Glass (NSGlassEffectView.tintColor),
// keeping the glass translucent while carrying the active theme's hue.
func glassTint(bg ui.Color, alpha float32) string {
	return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", bg.R, bg.G, bg.B, alpha)
}

// glassLive reports whether the window shows the glass: it is on, the system draws it, and the window is the one in
// front. Behind another window it is matte, as the system's own windows are: the ground and the cards opaque, the
// desktop not showing through (#491).
func (a *app) glassLive(c *ui.Context) bool {
	return a.settings.Glass && c.Vibrancy() && !a.winBehind
}

// glassOpacity is how opaque a surface over the glass is: dark in the dark theme, and nearly opaque in the light one,
// where a white half let through turns the desktop behind into a muddy gray (#491).
func glassOpacity(c *ui.Context, dark float32) float32 {
	if ryugo.From(c).Dark {
		return dark
	}
	return 0.95
}

// setGlass persists the macOS Liquid Glass preference (applied on next launch so the window keeps its
// launch vibrancy/transparency state) and clears any cached theme tint when turned off.
func (a *app) setGlass(on bool) {
	a.settings.Glass = on
	a.saveSettings()
	if runtime.GOOS != "darwin" {
		return
	}
	if !on && (a.glassTint != "" || a.glassHasTint) {
		a.glassTint = ""
		a.glassHasTint = false
		if a.wel.win != nil {
			a.wel.win.SetVibrancyTint("")
		}
		if a.quick != nil && a.quick.panel != nil {
			a.quick.panel.SetVibrancyTint("")
		}
	}
}

// syncGlassTint tints the main window's and the quick note's glass with the page's background. The tint is made
// again only when that background changes (another theme, light or dark, a custom colour), not every frame.
func (a *app) syncGlassTint(c *ui.Context) {
	if !a.settings.Glass || runtime.GOOS != "darwin" {
		return
	}
	dark := ryugo.From(c).Dark
	alpha := glassOpacity(c, 0.56)
	if a.glassHasTint && a.glassBg == a.pageBg && a.glassDark == dark {
		return
	}
	tint := glassTint(a.pageBg, alpha)
	a.glassTintRuns++
	a.glassTint, a.glassBg, a.glassDark, a.glassHasTint = tint, a.pageBg, dark, true
	if a.wel.win != nil {
		a.wel.win.SetVibrancyTint(tint)
	}
	if a.quick != nil && a.quick.panel != nil {
		a.quick.panel.SetVibrancyTint(tint)
	}
}

// noTabHeaderHeight is the room kept at the top of the sidebar's and the page's cards on macOS: the cards rise to
// the window's top, round the window's buttons and the sidebar's, and what is in them starts, and scrolls, below
// those.
func (a *app) noTabHeaderHeight() float32 {
	if a.wframe.on && !ownWindowControls {
		return frameTopH - frameW
	}
	return 0
}

// frameTop is the frame's top edge, which moves the window: after macOS's window buttons the sidebar's buttons
// (fold, back, forward, the settings); at the right end, on Linux and Windows, the window's own controls. The side
// panel is opened from the page's menu ("Page info").
func (a *app) frameTop(c *ui.Context) {
	a.barLeft = a.titleBar(c).Left
	left := max(a.barLeft-frameW+barLead, 6) // after macOS's window buttons, with room between
	row := ui.Row(c.Key("frame-top")).FillWidth().Height(frameTopH).Shrink(0).Padding(0, 4, 0, 0).Gap(10).AlignItems(ui.Center).Label(locales.SidebarTopBar())
	if ownWindowControls {
		row.Padding(0, 4, 0, left).DragWindow()
	} else {
		topGap := frameTopH - a.noTabHeaderHeight() // where the cards start: see noTabHeaderHeight
		row.Margin(0, 0, -(frameTopH - topGap), 0).ZIndex(1).PassThrough()
	}
	row.Children(func() {
		if !ownWindowControls {
			ui.Box(c.Key("frame-drag-top")).Absolute().Left(0).Right(0).Top(0).Height(frameW).DragWindow()
		}
		btns := ui.Row(c.Key("frame-top-left")).Height(frameTopH).AlignItems(ui.Center).Shrink(0)
		a.topLeftR = btns.LayoutBounds()
		if !ownWindowControls {
			btns.Padding(0, 0, 0, left).DragWindow()
		}
		btns.Children(func() {
			ui.Row(c.Key("frame-sidebar-buttons")).Shrink(0).AlignItems(ui.Center).Children(func() {
				a.sidebarButtons(c, frameW+left+3*barBtnW, frameTopH-4)
			})
		})
		// the room between, where the panes' tabs were: each pane has a bar of its own now (panebar.go)
		space := ui.Box(c.Key("frame-space")).Grow(1).MinWidth(0).Height(frameTopH)
		if !ownWindowControls {
			space.PassThrough()
		}
		a.updateChip(c) // a version that waits (updatepanel.go)
		a.windowControls(c)
	})
}

// frameBottom is the frame's bottom edge: the status bar, on the frame's ground, or a thin edge without it.
func (a *app) frameBottom(c *ui.Context) {
	if a.settings.HideStatus {
		ui.Box(c.Key("frame-bottom")).FillWidth().Height(frameW).Shrink(0)
		return
	}
	a.status(c)
}
