package main

import "github.com/egoist/mygo/ui"

// The app's icons: strokes on a 24 grid, drawn in the color of where they are.

// icon is an icon of the SVG paths body.
func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" ` +
		`stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	iconMenu     = icon(`<path d="M4 8.5h16M4 15.5h10"/>`)
	iconClose    = icon(`<path d="M6 6l12 12M18 6L6 18"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconBack     = icon(`<path d="M15 5l-7 7 7 7"/>`)
	iconMore     = icon(`<circle cx="6" cy="12" r=".9"/><circle cx="12" cy="12" r=".9"/><circle cx="18" cy="12" r=".9"/>`)
	iconSearch   = icon(`<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4.5 4.5"/>`)
	iconPage     = icon(`<path d="M7 3.5h7l4 4v13H7z"/><path d="M14 3.5v4h4M10 12.5h5M10 16h4"/>`)
	iconCalendar = icon(`<rect x="4" y="5" width="16" height="15" rx="3"/><path d="M4 10h16M9 3v4M15 3v4"/>`)
	iconFolder   = icon(`<path d="M3.5 7.5a2 2 0 012-2h4l2 2h7a2 2 0 012 2v8a2 2 0 01-2 2h-13a2 2 0 01-2-2z"/>`)
	iconLock     = icon(`<rect x="5" y="11" width="14" height="9" rx="2.5"/><path d="M8 11V8a4 4 0 018 0v3"/>`)
	iconArrow    = icon(`<path d="M7 17L17 7M9 7h8v8"/>`)
	iconTrash    = icon(`<path d="M4.5 7h15M10 11v6M14 11v6M6.5 7l1 13h9l1-13M9.5 7V4.5h5V7"/>`)
	iconCheck    = icon(`<path d="M5 12.5l4.5 4.5L19 7.5"/>`)
	iconCircle   = icon(`<circle cx="12" cy="12" r="8"/>`)
	iconTasks    = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M8.5 12.2l2.4 2.4 4.8-5"/>`)
	iconFlame    = icon(`<path d="M12 21c4 0 6.5-2.6 6.5-6.2 0-3.4-2.4-5.3-3.6-8.3-.6 2-1.8 3-2.9 3.4.3-2.6-.6-5-3-6.9.2 3.6-4.5 6.2-4.5 11.2C4.5 18.4 8 21 12 21z"/><path d="M12 21c-1.8 0-3-1.2-3-2.9 0-2 1.8-2.8 2.4-4.6 1.3 1.1 3.6 2.4 3.6 4.6 0 1.7-1.2 2.9-3 2.9z"/>`)
	iconSun      = icon(`<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.6 4.6l1.4 1.4M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4L6 18M18 6l1.4-1.4"/>`)
	iconBooks    = icon(`<path d="M5 4.5h3.5v15H5zM10.5 4.5H14v15h-3.5zM16 5.2l3.2-.8 3 14.6-3.2.7z"/>`)
	iconGear     = icon(`<circle cx="12" cy="12" r="3"/><path d="M12 2.8l1.6 2.4 2.8-.6.6 2.8 2.4 1.6-1.5 2.4 1.5 2.4-2.4 1.6-.6 2.8-2.8-.6L12 21.2l-1.6-2.4-2.8.6-.6-2.8-2.4-1.6 1.5-2.4-1.5-2.4 2.4-1.6.6-2.8 2.8.6z"/>`)
	iconMoon     = icon(`<path d="M19.5 14.5A8 8 0 019.5 4.5a8 8 0 1010 10z"/>`)
	iconCloud    = icon(`<path d="M7 18.5h10a4 4 0 00.6-8 6 6 0 00-11.4-1.5A4.5 4.5 0 007 18.5z"/>`)
	iconCopy     = icon(`<rect x="8" y="8" width="12" height="12" rx="2.5"/><path d="M16 8V6a2 2 0 00-2-2H6a2 2 0 00-2 2v8a2 2 0 002 2h2"/>`)
)
