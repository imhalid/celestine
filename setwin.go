package main

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// The settings in a window of their own (#605): ⌘, opens it, or brings it forward when it is open, and leaves the
// panes as they were. It draws the same settings as a pane does (settingsScreen), in a pane of its own (sw.pn) that no
// other screen takes, and what is changed in it is the app's: the main window is asked for a frame at once, and the
// settings window when the main one changed a setting. Where a window of its own cannot be (a phone, a browser's
// tab), and in the tests that have no window, the settings are a screen of the pane in use, as before.

// setWinState is the settings' window.
type setWinState struct {
	w        *mygo.Window
	open     bool              // it is open (w is nil in a test, which draws its view itself)
	pn       *paneState        // what it shows: the section, its history, the scroll
	building bool              // its frame is being built: the menus and the questions it opens are drawn in it
	opening  bool              // asked for, not yet made
	view     func(*ui.Context) // settingsWindowView, set by newApp: named here, it made a cycle of the keys' tables' initialization
	mainSaw  uint64            // settingsGen as the main window last drew it
	winSaw   uint64            // and as this window did
}

// settingsInWindow reports whether the settings open in a window of their own.
func (a *app) settingsInWindow() bool {
	return (a.win != nil || a.newSetWin != nil) && !phone && runtime.GOOS != "js"
}

// settingsPane is the pane the settings are drawn in: the window's, while it is open, or the pane in use.
func (a *app) settingsPane() *paneState {
	if a.sw.pn != nil && (a.sw.open || a.sw.opening) {
		return a.sw.pn
	}
	return a.pane()
}

// openSettingsAt opens the settings at section tab.
func (a *app) openSettingsAt(tab int) {
	a.openSettings()
	a.settingsGoIn(a.settingsPane(), tab)
}

// showSettingsWindow opens the settings' window, or brings it forward. The window is made after the frame that asked
// for it, out of the app's lock: making it draws its first frame, which takes the lock (windowIn says why that froze).
func (a *app) showSettingsWindow() {
	if a.sw.pn == nil {
		a.sw.pn = newPaneState()
		a.sw.pn.settingsOpen = true
	}
	if a.sw.open {
		if w := a.sw.w; w != nil {
			go mygo.RunOnMain(w.Focus)
		}
		return
	}
	if a.sw.opening {
		return
	}
	if a.newSetWin != nil { // a test's: made at once, its view drawn by the test
		a.sw.w, a.sw.open = a.newSetWin(a.sw.view), true
		return
	}
	a.sw.opening = true
	make := a.newSetWin
	if make == nil {
		make = func(view func(*ui.Context)) *mygo.Window {
			opts := mygo.WindowOptions{Title: locales.CommonSettings(), Width: 980, Height: 700, MinWidth: 720, MinHeight: 480,
				Content: ui.View(view)}
			if a.settings.Glass && runtime.GOOS == "darwin" {
				opts.Transparent = true
				opts.Vibrancy = mygo.VibrancyGlass
			}
			if ownWindowControls { // framed as the main window: its title bar its own (setWinBar), the menu bar out of sight
				opts.TitleBarStyle, opts.HideWindowButtons, opts.AutoHideMenuBar = mygo.TitleBarHidden, true, true
			}
			return mygo.NewWindow(opts)
		}
	}
	go func() {
		defer a.guard()
		mygo.RunOnMain(func() {
			w := make(a.sw.view)
			a.do(func() error {
				a.sw.w, a.sw.open, a.sw.opening = w, true, false
				return nil
			})
			if w != nil {
				w.OnClosed(func() {
					go a.do(func() error { // the window's own close: what the settings window had is let go
						a.sw.w, a.sw.open = nil, false
						return nil
					})
				})
			}
		})
	}()
}

// settingsWindowView is a frame of the settings' window: the app's theme, the settings, and what they opened over
// them (a field's menu, a question before a reset).
func (a *app) settingsWindowView(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sw.building = true
	defer func() { a.sw.building = false }()
	if a.sw.pn == nil {
		a.sw.pn = newPaneState()
		a.sw.pn.settingsOpen = true
	}
	pn := a.sw.pn
	tok, _ := a.frameTheme(c)
	cl := tok.Colors
	if a.settings.Glass && runtime.GOOS == "darwin" && c.Vibrancy() {
		c.Root().Background(cl.Bg.Alpha(0.72))
	} else {
		c.Root().Background(cl.Bg)
	}
	if !c.FocusTakesText() { // ⌘[ and ⌘] through the sections, as the app's own history goes
		switch {
		case c.Shortcut(ui.Cmd, ui.KeyBracketLeft):
			a.settingsStep(pn, -1)
		case c.Shortcut(ui.Cmd, ui.KeyBracketRight):
			a.settingsStep(pn, 1)
		}
	}
	top := float32(18)
	if ownWindowControls {
		a.setWinBar(c)
		top = 4
	}
	ui.Column(c.Key("setwin")).FillWidth().Grow(1).MinHeight(0).Padding(top, 20, 0, 20).Children(func() {
		a.settingsScreen(c, pn)
	})
	a.contextView(c)
	a.askView(c)
	a.keepSettings(c)
	if a.settingsGen != a.sw.winSaw { // a setting changed here: the main window shows it at once
		a.sw.winSaw = a.settingsGen
		a.sw.mainSaw = a.settingsGen
		if a.invalidate != nil {
			a.invalidate()
		}
	}
}

// setWinBarH is how tall the settings' window's own title bar is.
const setWinBarH = 38

// setWinBar is the settings' window's own title bar, where the window has the app's controls, not the system's (#615):
// across its top, dragged to move the window, its title at the middle and the app's minimize, maximize and close at
// its right end, as the main window's header has them.
func (a *app) setWinBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c.Key("setwin-bar")).FillWidth().Height(setWinBarH).Shrink(0).AlignItems(ui.Center).Padding(0, 8, 0, 14).
		DragWindow().Children(func() {
		ui.Box(c).Width(84) // as wide as the buttons: the title at the middle
		ui.Text(c, locales.CommonSettings()).FontSize(12).FontWeight(560).TextColor(t.TextMuted).MaxLines(1).Grow(1).
			TextAlign(ui.Center).MinWidth(0)
		ui.Row(c).Width(84).Justify(ui.End).Children(func() { a.windowButtons(c, a.sw.w) })
	})
}

// followSettingsWindow is the main window's side: a setting it changed is shown in the settings' window.
func (a *app) followSettingsWindow() {
	if !a.sw.open || a.settingsGen == a.sw.mainSaw {
		return
	}
	a.sw.mainSaw, a.sw.winSaw = a.settingsGen, a.settingsGen
	if a.sw.w != nil {
		a.sw.w.Invalidate()
	}
}

// overlayHere reports whether a menu or a question opened in the settings' window (inSet) or not is drawn by the frame
// being built: each window draws its own.
func (a *app) overlayHere(inSet bool) bool { return inSet == a.sw.building }
