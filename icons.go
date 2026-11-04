package main

import "github.com/egoist/mygo/ui"

// The app's icons: drawn here on a 24 grid, strokes 1.75 wide with round
// ends, in the color of the text around them (ui.Icon takes it).

func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconChevron  = icon(`<path d="M9.5 6.5 15 12l-5.5 5.5"/>`)
	iconPlus     = icon(`<path d="M12 6v12M6 12h12"/>`)
	iconPage     = icon(`<path d="M7 3.5h6.5L18 8v11.5a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1Z"/><path d="M13.5 3.5V8H18"/><path d="M9 13h6M9 16.5h4"/>`)
	iconStack    = icon(`<path d="M12 3.5 20.5 8 12 12.5 3.5 8 12 3.5Z"/><path d="m3.5 12 8.5 4.5 8.5-4.5"/><path d="m3.5 16 8.5 4.5 8.5-4.5"/>`)
	iconSidebar  = icon(`<rect x="3.5" y="4.5" width="17" height="15" rx="2.5"/><path d="M9.5 4.5v15"/>`)
	iconTrash    = icon(`<path d="M4.5 7h15M10 4h4M6.5 7l.8 11.6a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4L17.5 7"/><path d="M10 11v5M14 11v5"/>`)
	iconRestore  = icon(`<path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9"/><path d="M4.5 4.5V9H9"/>`)
	iconSettings = icon(`<path d="M4.5 7.5h9M17.5 7.5h2M4.5 16.5h2M10.5 16.5h9"/><circle cx="15.5" cy="7.5" r="2"/><circle cx="8.5" cy="16.5" r="2"/>`)
	iconSearch   = icon(`<circle cx="11" cy="11" r="6"/><path d="m20 20-4.5-4.5"/>`)
)
