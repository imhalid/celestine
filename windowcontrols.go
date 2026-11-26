package main

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's controls. On macOS they are the traffic lights, the system's.
// On Linux and Windows they would be GTK's or Windows' own buttons, which do
// not match the page, so the window hides them and the app draws its own:
// minimize, maximize and close, flat and small, at the top right corner, the
// close going red under the pointer.

// ownWindowControls reports whether the app draws the window's controls.
var ownWindowControls = runtime.GOOS != "darwin" && runtime.GOOS != "js" // in a browser the tab is the window: it has none

const windowControlH = 20

var (
	iconMinimize = icon(`<path d="M5.5 12h13"/>`)
	iconMaximize = icon(`<rect x="6" y="6" width="12" height="12" rx="2.5"/>`)
	iconWinClose = icon(`<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>`)
)

// windowControls draws the three buttons, at the header's right end, in its row, where the app draws them: macOS has
// its own, though the main window is kept there too (#629).
func (a *app) windowControls(c *ui.Context) {
	if ownWindowControls {
		a.windowButtons(c, a.win)
	}
}

// windowButtons draws the three buttons of window w, in a row: the main window's, or the settings' (#615).
func (a *app) windowButtons(c *ui.Context, w *mygo.Window) {
	if w == nil {
		return
	}
	t := c.Theme()
	cl := ryugo.From(c).Colors
	ui.Row(c.Key("window-controls")).Gap(2).Children(func() {
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
		btn(iconMinimize, locales.WindowMinimize(), false, w.Minimize)
		btn(iconMaximize, locales.WindowMaximize(), false, w.ToggleMaximize)
		btn(iconWinClose, "close", true, w.Close)
	})
}

// keepInDock has the window w, the app's main one, closed as a Mac app's is (#632): its close button and ⌘W hide it,
// and the app stays in the Dock, whose icon brings it back, as opening the app again does. A quit (⌘Q, the Dock's
// Quit, the system's logout) closes it all the same, and the app with it.
func keepInDock(w *mygo.Window) {
	quitting, hidden := false, false
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
	mygo.App.OnQuitCanceled(func() { quitting = false })
	w.OnClose(func(e *mygo.CloseEvent) {
		if quitting {
			return
		}
		e.PreventDefault()
		hidden = true
		w.Hide()
	})
	mygo.App.OnActivate(func(bool) {
		if hidden {
			hidden = false
			w.Show()
			w.Focus()
		}
	})
}
