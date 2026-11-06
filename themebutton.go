package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

var (
	iconSun  = icon(`<circle cx="12" cy="12" r="3.8"/><path d="M12 3.5v2M12 18.5v2M3.5 12h2M18.5 12h2M6 6l1.4 1.4M16.6 16.6 18 18M18 6l-1.4 1.4M7.4 16.6 6 18"/>`)
	iconMoon = icon(`<path d="M19.5 14.2A7.8 7.8 0 0 1 9.8 4.5a7.8 7.8 0 1 0 9.7 9.7z"/>`)
)

// themeButton is the status bar's button between the light and the dark
// look: it shows the one it goes to.
func (a *app) themeButton(c *ui.Context) {
	dark := ryugo.From(c).Dark
	glyph, label := iconMoon, "Koyu temaya geç"
	if dark {
		glyph, label = iconSun, "Açık temaya geç"
	}
	if a.barButton(c, glyph, label) {
		a.settings.Theme = map[bool]int{true: 1, false: 2}[dark] // 1 light, 2 dark
		a.saveSettings()
	}
}
