package main

import (
	"fmt"
	"math"
	"runtime"

	"github.com/egoist/mygo"
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

// frameW is the frame's width around the card, and frameR the card's corner at the design's measure (radius.go):
// with the window's corners frameR+frameW, as a titled macOS window without a toolbar draws its own (#600).
const (
	frameW = 8
	frameR = 8
)

// winCorner is the corner radius the window is opened with: the card's corner at the measure of the Corners
// setting, and the frame's width around it, so that the two are concentric (#600). The window's radius cannot be
// changed once it is open: a new measure reaches it at the next launch, and the cards follow the window's until then.
func winCorner(percent int) int {
	return frameW + int(math.Round(float64(frameR)*float64(percent)/100))
}

// cardR is the corner of the frame's cards, as drawn: the window's own radius less the frame's width, so that the
// card's corners are concentric with the window's (on macOS 26 a window with a toolbar has 26, the cards 18), or
// frameR at the measure where the window's corners are square or the system does not tell them (#517, #600). It is
// not scaled again by rad: the window's radius already is.
func (a *app) cardR() float32 {
	if r := float32(a.winRadius) - frameW; r > 0 {
		return r
	}
	return rad(frameR)
}

// frameGap is the room between the cards in the frame: the sidebar, the panes, the Dock's areas.
const frameGap = 6

// frameTopH is the top edge's height, the height of the window's own title bar's buttons and macOS's window
// buttons, which are centered in it (TrafficLightPosition); frameTabH is a tab's.
const (
	frameTopH = 42
	frameTabH = 30
)

// chromeMid is the line macOS's window buttons, the sidebar's buttons at the top left and the pane bar's title are
// centered on: the middle of the strip at the top of the card, under the frame's thin top edge (#604).
const chromeMid = frameW + (frameTopH-frameW)/2

// trafficLights is where macOS's window buttons go (14 high): on chromeMid, and in from the card's corner by more than
// the frame's width, as macOS's own apps keep them clear of their rounded corner (#604).
func trafficLights() *mygo.Point { return &mygo.Point{X: frameW + 12, Y: chromeMid - 14/2} }

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

// frameGroundOf is the frame's ground in c's theme, so that it goes with the color theme in use (#619): made in
// OKLCH in the theme's own hue from top to bottom, at the lightness of a step off the sidebar's surface toward the
// text, the accent's chroma lighting it at the top and easing off downward without going to gray, the hue turning a
// little on the way down so that the foot keeps a color of its own, and the accent faintly again toward the right;
// in light and in dark alike, as the theme's colors are. A gray accent gives a gray ground.
func frameGroundOf(c *ui.Context) frameGround {
	t := c.Theme()
	base := ryugo.Mix(t.Surface, t.Text, 0.06)
	l, _, _ := ryugo.OklchOf(base)
	_, ac, ah := ryugo.OklchOf(t.Accent)
	k := min(ac, 0.2) // the accent's chroma, held to what a ground can bear
	if t.Dark {
		k *= 0.8 // a dark ground shows its color more
	}
	at := func(dl, chroma, turn float64) ui.Color {
		return ui.Oklch(float32(l+dl), float32(k*chroma), float32(math.Mod(ah+turn+360, 360)))
	}
	lift := 0.02 // the top a little lighter on a dark ground, a little deeper on a light one
	if !t.Dark {
		lift = -0.01
	}
	return frameGround{
		top:    at(lift, 0.3, 0),
		mid:    at(lift/3, 0.19, 10),
		low:    at(0, 0.13, 22),
		bottom: at(-lift/3, 0.1, 34),
		tint:   at(lift, 0.45, -18).Alpha(0.1),
	}
}

// paint paints the ground over r: one gradient blended in OKLCH, top at the top, mid at 38%, low at 70% and bottom at
// the foot, and the tint from clear at 35% of the width to itself at the right.
func (g frameGround) paint(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ryugo.InOklch(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: g.top}, {Color: g.mid, At: 0.38}, {Color: g.low, At: 0.7}, {Color: g.bottom, At: 1}}}), 0)
	clear := g.tint
	clear.A = 0
	p.FillGradient(r, ryugo.InOklch(ui.LinearGradient{From: clear, To: g.tint, Angle: 90, Start: 0.35, End: 1}), 0)
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
	base := a.wallTintBase() // the page's background, and the wallpaper's color when it is the ground (#606)
	if a.glassHasTint && a.glassBg == base && a.glassDark == dark {
		return
	}
	tint := glassTint(base, alpha)
	a.glassTintRuns++
	a.glassTint, a.glassBg, a.glassDark, a.glassHasTint = tint, base, dark, true
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
		if !ownWindowControls { // under the frame's top edge, on chromeMid with the window's buttons and the bars' titles
			btns.Padding(frameW, 0, 0, left).DragWindow()
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
