package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// The window's zoom (#350): Cmd/Ctrl +, − and 0 zoom the whole content of the window through MyGo's Context.SetZoom,
// as a browser zooms a page - laid out smaller and drawn larger at the screen's resolution, so text and icons stay
// sharp - and the zoom is kept in the settings. The editor's own text size, which only the page's text follows, stays
// a step of its own on Alt+Cmd +, − and 0 (biggerText, smallerText, resetTheFontSize).

// zoomSteps are the zooms Zoom in and Zoom out go through, as the browsers' are.
var zoomSteps = []float32{0.5, 0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3}

// windowZoom is the window's zoom: 1 when none is set.
func (a *app) windowZoom() float32 {
	if z := a.settings.Zoom; z > 0 {
		return z
	}
	return 1
}

// applyZoom zooms the content of the window of c; every window's frame calls it, through frameTheme, and the quick
// note's.
func (a *app) applyZoom(c *ui.Context) {
	if z := a.windowZoom(); c.Zoom() != z {
		c.SetZoom(z)
	}
}

// stepZoom goes d steps of zoomSteps from the zoom set, or from the step nearest it.
func (a *app) stepZoom(d int) {
	z := a.windowZoom()
	i, found := slices.BinarySearch(zoomSteps, z)
	switch {
	case found:
		i += d
	case d < 0:
		i-- // between two steps: Zoom out goes to the one below, Zoom in to the one above (i)
	}
	a.setZoom(zoomSteps[min(max(i, 0), len(zoomSteps)-1)])
}

// setZoom keeps zoom z, 1 as none.
func (a *app) setZoom(z float32) {
	if z == 1 {
		z = 0
	}
	if a.settings.Zoom != z {
		a.settings.Zoom = z
		a.saveSettingsSoon()
	}
}
