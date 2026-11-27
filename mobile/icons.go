package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Minimal duotone icon set on a 24x24 grid for the One Day writing experience.

func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" ` +
		`stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

func tone(element string) string {
	return "<" + strings.TrimSpace(element) + ` fill="currentColor" fill-opacity=".22" stroke="none"/>`
}

var (
	iconClose   = icon(`<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>`)
	iconBack    = icon(`<path d="M15 5l-7 7 7 7"/>`)
	iconTasks   = icon(tone(`circle cx="12" cy="12" r="8.5"`) + `<circle cx="12" cy="12" r="8.5"/><path d="M8.5 12.2l2.4 2.4 4.8-5"/>`)
	iconHeading = icon(`<path d="M6 5v14M14 5v14M6 12h8M18 11v8"/>`)
	iconBold    = icon(`<path d="M7 5h6a3.5 3.5 0 010 7H7V5zM7 12h7a3.5 3.5 0 010 7H7v-7z"/>`)
	iconSun     = icon(tone(`circle cx="12" cy="12" r="4"`) +
		`<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.6 4.6l1.4 1.4M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4L6 18M18 6l1.4-1.4"/>`)
	iconMoon = icon(tone(`path d="M19.5 14.5A8 8 0 019.5 4.5a8 8 0 1010 10z"`) + `<path d="M19.5 14.5A8 8 0 019.5 4.5a8 8 0 1010 10z"/>`)
	iconGear = icon(tone(`path d="M12 2.8l1.6 2.4 2.8-.6.6 2.8 2.4 1.6-1.5 2.4 1.5 2.4-2.4 1.6-.6 2.8-2.8-.6L12 21.2l-1.6-2.4-2.8.6-.6-2.8-2.4-1.6 1.5-2.4-1.5-2.4 2.4-1.6.6-2.8 2.8.6z"`) +
		`<circle cx="12" cy="12" r="3"/><path d="M12 2.8l1.6 2.4 2.8-.6.6 2.8 2.4 1.6-1.5 2.4 1.5 2.4-2.4 1.6-.6 2.8-2.8-.6L12 21.2l-1.6-2.4-2.8.6-.6-2.8-2.4-1.6 1.5-2.4-1.5-2.4 2.4-1.6.6-2.8 2.8.6z"/>`)
	iconCloud = icon(tone(`path d="M7 18.5h10a4 4 0 00.6-8 6 6 0 00-11.4-1.5A4.5 4.5 0 007 18.5z"`) +
		`<path d="M7 18.5h10a4 4 0 00.6-8 6 6 0 00-11.4-1.5A4.5 4.5 0 007 18.5z"/>`)
	iconCopy = icon(tone(`rect x="8" y="8" width="12" height="12" rx="2.5"`) +
		`<rect x="8" y="8" width="12" height="12" rx="2.5"/><path d="M16 8V6a2 2 0 00-2-2H6a2 2 0 00-2 2v8a2 2 0 002 2h2"/>`)
	iconLock = icon(tone(`rect x="5" y="11" width="14" height="9" rx="2.5"`) +
		`<rect x="5" y="11" width="14" height="9" rx="2.5"/><path d="M8 11V8a4 4 0 018 0v3"/>`)
)
