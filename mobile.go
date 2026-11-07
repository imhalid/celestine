package main

import (
	"runtime"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The phone: the web version in a narrow window, as the Android app is (a WebView), laid out for a hand.
// A finger scrolls, taps and long-presses (web/touch.js); here the window is made for it:
//
//	the sidebar     a drawer that slides in from the left over the page, behind a veil, from ☰ in the
//	                header; it goes as a page is opened
//	the header      taller, with ☰, back, the title, and the page search and the commands at its right
//	one pane        no split view, no window of a pane's own, no floating side panel: there is no room
//	touch           rows and buttons a finger can hit; what shows under a pointer (a block's grip, the
//	                gutter) is not drawn; the status bar does not list keys
//	the screens     narrower margins; a setting's control under its name, not beside it; the sections of
//	                the settings a row of chips over them; the tour as wide as the screen
//
// It is the web build at under phoneMax DIPs wide; forcePhone makes any window a phone (the tests).

// phoneMax is the width under which a window in a browser is a phone's.
const phoneMax = 760

// phone reports whether the window of this frame is laid out for a phone. It is set as a frame starts
// (updatePhone): the layout helpers that have no app read it.
var phone bool

// updatePhone sets phone, and the sizes that follow it, for the frame.
func (a *app) updatePhone(c *ui.Context) {
	w, _ := c.Size()
	was := phone
	phone = a.forcePhone || (runtime.GOOS == "js" && w > 0 && w < phoneMax)
	rowH = 30
	if phone {
		rowH = 42
	}
	if phone && !was { // a phone starts with the page: the drawer is opened by hand
		a.drawerOpen = false
	}
	if phone {
		if here := a.here(); here != a.drawerFrom { // something was opened: the drawer goes, as on any phone
			a.drawerOpen = false
		}
	}
	a.drawerFrom = a.here()
}

// touchSize is a button's side: what a finger hits on a phone, a pointer's size elsewhere.
func touchSize(desk float32) float32 {
	if phone {
		return max(desk, 38)
	}
	return desk
}

// pageMargin is the room either side of a page or a screen: a phone's narrow.
func pageMargin() float32 {
	if phone {
		return 16
	}
	return 32
}

// phoneOff is why a command of the panes or the panel cannot run: a phone has one pane and no panel.
func phoneOff(a *app) string {
	if phone {
		return T("Telefonda yok")
	}
	return ""
}

// drawer is the sidebar on a phone: over the page from the left, with a veil over the rest that closes it.
func (a *app) drawer(c *ui.Context) {
	open := float32(0)
	if a.drawerOpen {
		open = 1
	}
	veil := ui.Box(c.Key("drawer-veil")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Label(T("Çekmeceyi kapat"))
	k := veil.AnimateTo("open", open, springSheet)
	if k < 0.01 && !a.drawerOpen {
		veil.Hidden(true)
		a.sidebarShown = 0
		return
	}
	veil.Background(ui.RGBA(0, 0, 0, 0.42*k))
	if veil.Clicked() {
		a.drawerOpen = false
	}
	a.sidebarShown = a.sidebar(c)
}

// drawerWidth is how wide the drawer is: most of a phone's width, at most the sidebar's own.
func (a *app) drawerWidth() float32 {
	w := a.winRect.W
	if w <= 0 {
		w = 390
	}
	return min(320, w*0.86)
}

// phoneHeader is the header's row on a phone: ☰, back, the title, the page search and the commands.
func (a *app) phoneHeader(c *ui.Context) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	if a.iconButton(c, iconSidebar, T("Menü")) {
		a.drawerOpen = !a.drawerOpen
	}
	ui.Box(c.Key("phone-back")).Opacity(map[bool]float32{true: 1, false: 0.35}[a.canStep(-1)]).Children(func() {
		if a.iconButton(c, iconBack, T("Geri")) {
			a.step(-1)
		}
	})
	if title := a.headerTitle(); title != "" {
		ui.Text(c, title).FontSize(15).FontWeight(620).TextColor(t.Text).MaxLines(1).Grow(1).MinWidth(0)
	} else {
		ui.Box(c).Grow(1)
	}
	_ = cl
	if a.iconButton(c, iconSearch, T("Sayfa ara")) {
		a.openPalette()
	}
	if a.iconButton(c, iconMore, T("Komutlar")) {
		a.openCommands()
	}
}
