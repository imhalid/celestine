package main

import "github.com/egoist/mygo/ui"

// The app's icons: drawn here on a 24 grid, strokes 1.75 wide with round
// ends, in the color of the text around them (ui.Icon takes it).

func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconChevron     = icon(`<path d="M9.5 6.5 15 12l-5.5 5.5"/>`)
	iconBack        = icon(`<path d="M14.5 6.5 9 12l5.5 5.5"/>`)
	iconChevronDown = icon(`<path d="M6.5 9.5 12 15l5.5-5.5"/>`)
	iconEdit        = icon(`<path d="M14.5 5.5 18.5 9.5 9 19H5v-4z"/><path d="m12.5 7.5 4 4"/>`)
	iconFold        = icon(`<path d="m14 7-5 5 5 5M19 5v14"/>`)
	iconCalendar    = icon(`<rect x="4" y="5.5" width="16" height="14" rx="2"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>`)
	iconClose       = icon(`<path d="M7 7l10 10M17 7 7 17"/>`)
	iconPlus        = icon(`<path d="M12 6v12M6 12h12"/>`)
	iconPage        = icon(`<path d="M7 3.5h6.5L18 8v11.5a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1Z"/><path d="M13.5 3.5V8H18"/><path d="M9 13h6M9 16.5h4"/>`)
	iconStack       = icon(`<path d="M12 3.5 20.5 8 12 12.5 3.5 8 12 3.5Z"/><path d="m3.5 12 8.5 4.5 8.5-4.5"/><path d="m3.5 16 8.5 4.5 8.5-4.5"/>`)
	iconSidebar     = icon(`<rect x="3.5" y="4.5" width="17" height="15" rx="2.5"/><path d="M9.5 4.5v15"/>`)
	iconSignOut     = icon(`<path d="M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4"/><path d="M10 8l-4 4 4 4M6 12h10"/>`)
	iconTrash       = icon(`<path d="M4.5 7h15M10 4h4M6.5 7l.8 11.6a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4L17.5 7"/><path d="M10 11v5M14 11v5"/>`)
	iconRestore     = icon(`<path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9"/><path d="M4.5 4.5V9H9"/>`)
	iconSettings    = icon(`<path d="M4.5 7.5h9M17.5 7.5h2M4.5 16.5h2M10.5 16.5h9"/><circle cx="15.5" cy="7.5" r="2"/><circle cx="8.5" cy="16.5" r="2"/>`)
	iconBoard       = icon(`<rect x="4" y="4.5" width="4.6" height="15" rx="1.4"/><rect x="9.7" y="4.5" width="4.6" height="9" rx="1.4"/><rect x="15.4" y="4.5" width="4.6" height="12" rx="1.4"/>`)
	iconCheck       = icon(`<path d="m5.5 12.5 4.2 4.2 8.8-9.4"/>`)
	iconImage       = icon(`<rect x="4.5" y="5.5" width="15" height="13" rx="2.5"/><circle cx="9.5" cy="10.2" r="1.4"/><path d="m5 17 4.5-4.2 3 2.8 2.5-2.3L19.5 17"/>`)
	iconTag         = icon(`<path d="M9.5 4.5 7.8 19.5M16.2 4.5 14.5 19.5M5 9h14.5M4.5 15h14.5"/>`)
	iconTask        = icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3.5"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`)
	iconSearch      = icon(`<circle cx="11" cy="11" r="6"/><path d="m20 20-4.5-4.5"/>`)
)
