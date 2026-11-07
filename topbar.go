package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The top bar: a bar across the top of the content, as Thymer's header is,
// with a thin line under it. It moves the window; the sidebar's buttons are
// at its left end and the window's controls at its right, over it. Over a
// page it lies on the page, which scrolls under it, as a bar of the page's
// own color, matte: a cover starts under it, and the wheel passes through it;
// in the other views it is a row of the layout.

// topBar draws the bar over a page or a collection, which scroll under it.
func (a *app) topBar(c *ui.Context) {
	cl := ryugo.From(c).Colors
	bar := ui.Column(c.Key("top-bar")).Absolute().Left(0).Right(0).Top(0).Height(barH).Label(T("Üst çubuk")).IgnoreInput(true).Background(a.pageBg)
	bar.Children(func() {
		ui.Box(c).Grow(1)
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
	})
	// only as high as the window's own title bar does it take the pointer, to move the
	// window, and with it the wheel, which the page below would scroll by
	h := c.TitleBar().Height
	if h <= 0 {
		h = 28
	}
	ui.Row(c.Key("top-drag")).Absolute().Left(0).Right(0).Top(0).Height(h).DragWindow()
}

// titleBar is the bar in the views that do not scroll under it: a row of the
// layout, as high as the bar, with its line.
func (a *app) titleBar(c *ui.Context) {
	cl := ryugo.From(c).Colors
	ui.Column(c.Key("top-bar")).Height(max(c.TitleBar().Height, barH)).Label(T("Üst çubuk")).DragWindow().Children(func() {
		ui.Box(c).Grow(1)
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
	})
}
