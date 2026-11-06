package main

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's controls. On macOS they are the traffic lights, the system's.
// On Linux and Windows they would be GTK's or Windows' own buttons, which do
// not match the page, so the window hides them and the app draws its own:
// minimize, maximize and close, flat and small, at the top right corner, the
// close going red under the pointer.

// ownWindowControls reports whether the app draws the window's controls.
var ownWindowControls = runtime.GOOS != "darwin"

const (
	windowControlsW = 3*26 + 2*2 + 12 // the room they take at the right end
	windowControlH  = 20
)

var (
	iconMinimize = icon(`<path d="M5.5 12h13"/>`)
	iconMaximize = icon(`<rect x="6" y="6" width="12" height="12" rx="2.5"/>`)
	iconWinClose = icon(`<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>`)
)

// windowControls draws the three buttons, over everything, at the top right.
func (a *app) windowControls(c *ui.Context) {
	if a.win == nil {
		return
	}
	t := c.Theme()
	cl := ryugo.From(c).Colors
	ui.Row(c.Key("window-controls")).Absolute().Right(6).Top((barH - windowControlH) / 2).Gap(2).Children(func() {
		btn := func(glyph *ui.SVG, label string, danger bool, do func()) {
			b := ui.Box(c.Key(label)).Width(26).Height(windowControlH).Radius(rad(6)).AlignItems(ui.Center).Justify(ui.Center).
				Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
			color := t.TextMuted
			switch {
			case b.Hovered() && danger:
				b.Background(cl.Up)
				color = ui.Hex("#ffffff")
			case b.Hovered():
				b.Background(t.SurfaceHover)
				color = t.Text
			default:
				b.Background(t.SurfaceHover.Alpha(0))
			}
			b.Children(func() { ui.Icon(c, glyph).Width(14).Height(14).TextColor(color) })
			if b.Clicked() {
				do()
			}
		}
		btn(iconMinimize, "Küçült", false, a.win.Minimize)
		btn(iconMaximize, "Büyüt", false, a.win.ToggleMaximize)
		btn(iconWinClose, "Kapat", true, a.win.Close)
	})
}
