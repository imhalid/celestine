package main

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The window's frame. The content is a card a few DIPs inside the window, its corners rounded, and around it the
// window shows MyGo's material: Liquid Glass on macOS 26 (the popover material before it), Acrylic on Windows 11.
// Linux has no material: there the frame is a plain color a step off the page's. The material is the window's
// (WindowOptions.Vibrancy); the app draws only the room around the card.

// frameW is the frame's width around the card, and frameR the card's corner, in DIPs.
const (
	frameW = 8
	frameR = 10
)

// frameMaterial is the material the main window shows in its frame.
const frameMaterial = mygo.VibrancyGlass

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
