package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The app's icons are duotone (#638): a shape filled softly in the color of where they are, and strokes on a 24 grid
// over it in the same color. tone marks the soft part of an icon's body: a path or a shape whose fill is the color at a
// quarter, with no stroke.

// icon is an icon of the SVG elements body, its soft parts written with tone.
func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" ` +
		`stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

// tone is the soft part of a duotone icon: element (a path, a rect, a circle, as `path d="…"`) filled at a quarter.
func tone(element string) string {
	return "<" + strings.TrimSpace(element) + ` fill="currentColor" fill-opacity=".24" stroke="none"/>`
}

var (
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconBack     = icon(`<path d="M15 5l-7 7 7 7"/>`)
	iconMore     = icon(`<circle cx="6" cy="12" r=".9"/><circle cx="12" cy="12" r=".9"/><circle cx="18" cy="12" r=".9"/>`)
	iconSearch   = icon(tone(`circle cx="11" cy="11" r="6.5"`) + `<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4.5 4.5"/>`)
	iconPage     = icon(tone(`path d="M7 3.5h7l4 4v13H7z"`) + `<path d="M7 3.5h7l4 4v13H7z"/><path d="M14 3.5v4h4M10 12.5h5M10 16h4"/>`)
	iconCalendar = icon(tone(`rect x="4" y="5" width="16" height="5" rx="1"`) +
		`<rect x="4" y="5" width="16" height="15" rx="3"/><path d="M4 10h16M9 3v4M15 3v4"/>`)
	iconFolder = icon(tone(`path d="M3.5 7.5a2 2 0 012-2h4l2 2h7a2 2 0 012 2v8a2 2 0 01-2 2h-13a2 2 0 01-2-2z"`) +
		`<path d="M3.5 7.5a2 2 0 012-2h4l2 2h7a2 2 0 012 2v8a2 2 0 01-2 2h-13a2 2 0 01-2-2z"/>`)
	iconLock = icon(tone(`rect x="5" y="11" width="14" height="9" rx="2.5"`) +
		`<rect x="5" y="11" width="14" height="9" rx="2.5"/><path d="M8 11V8a4 4 0 018 0v3"/>`)
	iconArrow = icon(`<path d="M7 17L17 7M9 7h8v8"/>`)
	iconTrash = icon(tone(`path d="M6.5 7l1 13h9l1-13z"`) + `<path d="M4.5 7h15M10 11v6M14 11v6M6.5 7l1 13h9l1-13M9.5 7V4.5h5V7"/>`)
	iconCheck = icon(`<path d="M5 12.5l4.5 4.5L19 7.5"/>`)
	iconTasks = icon(tone(`circle cx="12" cy="12" r="8.5"`) + `<circle cx="12" cy="12" r="8.5"/><path d="M8.5 12.2l2.4 2.4 4.8-5"/>`)
	iconFlame = icon(tone(`path d="M12 21c-1.8 0-3-1.2-3-2.9 0-2 1.8-2.8 2.4-4.6 1.3 1.1 3.6 2.4 3.6 4.6 0 1.7-1.2 2.9-3 2.9z"`) +
		`<path d="M12 21c4 0 6.5-2.6 6.5-6.2 0-3.4-2.4-5.3-3.6-8.3-.6 2-1.8 3-2.9 3.4.3-2.6-.6-5-3-6.9.2 3.6-4.5 6.2-4.5 11.2C4.5 18.4 8 21 12 21z"/>`)
	iconSun = icon(tone(`circle cx="12" cy="12" r="4"`) +
		`<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.6 4.6l1.4 1.4M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4L6 18M18 6l1.4-1.4"/>`)
	iconBooks = icon(tone(`path d="M10.5 4.5H14v15h-3.5z"`) + `<path d="M5 4.5h3.5v15H5zM10.5 4.5H14v15h-3.5zM16 5.2l3.2-.8 3 14.6-3.2.7z"/>`)
	iconGear  = icon(tone(`path d="M12 2.8l1.6 2.4 2.8-.6.6 2.8 2.4 1.6-1.5 2.4 1.5 2.4-2.4 1.6-.6 2.8-2.8-.6L12 21.2l-1.6-2.4-2.8.6-.6-2.8-2.4-1.6 1.5-2.4-1.5-2.4 2.4-1.6.6-2.8 2.8.6z"`) +
		`<circle cx="12" cy="12" r="3"/><path d="M12 2.8l1.6 2.4 2.8-.6.6 2.8 2.4 1.6-1.5 2.4 1.5 2.4-2.4 1.6-.6 2.8-2.8-.6L12 21.2l-1.6-2.4-2.8.6-.6-2.8-2.4-1.6 1.5-2.4-1.5-2.4 2.4-1.6.6-2.8 2.8.6z"/>`)
	iconMoon  = icon(tone(`path d="M19.5 14.5A8 8 0 019.5 4.5a8 8 0 1010 10z"`) + `<path d="M19.5 14.5A8 8 0 019.5 4.5a8 8 0 1010 10z"/>`)
	iconCloud = icon(tone(`path d="M7 18.5h10a4 4 0 00.6-8 6 6 0 00-11.4-1.5A4.5 4.5 0 007 18.5z"`) +
		`<path d="M7 18.5h10a4 4 0 00.6-8 6 6 0 00-11.4-1.5A4.5 4.5 0 007 18.5z"/>`)
	iconCopy = icon(tone(`rect x="8" y="8" width="12" height="12" rx="2.5"`) +
		`<rect x="8" y="8" width="12" height="12" rx="2.5"/><path d="M16 8V6a2 2 0 00-2-2H6a2 2 0 00-2 2v8a2 2 0 002 2h2"/>`)
	iconPalette = icon(tone(`path d="M12 3.5a8.5 8.5 0 000 17c1.3 0 2-.8 2-1.8 0-1.4-1.3-1.7-1.3-3 0-.9.8-1.7 1.8-1.7h2.2a3.8 3.8 0 003.8-3.8C20.5 6.6 16.7 3.5 12 3.5z"`) +
		`<path d="M12 3.5a8.5 8.5 0 000 17c1.3 0 2-.8 2-1.8 0-1.4-1.3-1.7-1.3-3 0-.9.8-1.7 1.8-1.7h2.2a3.8 3.8 0 003.8-3.8C20.5 6.6 16.7 3.5 12 3.5z"/>` +
		`<circle cx="8" cy="11" r=".8"/><circle cx="10.5" cy="7.5" r=".8"/><circle cx="14.5" cy="7.5" r=".8"/>`)
	iconStack = icon(tone(`path d="M12 3.5 20.5 8 12 12.5 3.5 8z"`) +
		`<path d="M12 3.5 20.5 8 12 12.5 3.5 8 12 3.5Z"/><path d="m3.5 12 8.5 4.5 8.5-4.5"/><path d="m3.5 16 8.5 4.5 8.5-4.5"/>`)
)

// collIcons are the icons a collection can have, by the names the desktop gives them (collicons.go of the desktop),
// in the picker's order; "" is the stack.
var collIcons = []struct {
	name  string
	glyph *ui.SVG
}{
	{"", iconStack},
	{"book", icon(tone(`path d="M5 4.5h10a3 3 0 013 3v6H8a3 3 0 00-3 3z"`) + `<path d="M5 4.5h10a3 3 0 0 1 3 3v12H8a3 3 0 0 1-3-3z"/><path d="M5 16.5a3 3 0 0 1 3-3h10"/>`)},
	{"note", iconPage},
	{"folder", iconFolder},
	{"check", icon(tone(`rect x="4" y="4" width="16" height="16" rx="3"`) + `<rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`)},
	{"calendar", iconCalendar},
	{"star", icon(tone(`path d="m12 4 2.5 5.1 5.6.8-4 3.9.9 5.6-5-2.6-5 2.6.9-5.6-4-3.9 5.6-.8z"`) + `<path d="m12 4 2.5 5.1 5.6.8-4 3.9.9 5.6-5-2.6-5 2.6.9-5.6-4-3.9 5.6-.8z"/>`)},
	{"heart", icon(tone(`path d="M12 19s-7-4.4-7-9.5A3.8 3.8 0 0112 7a3.8 3.8 0 017 2.5C19 14.6 12 19 12 19z"`) + `<path d="M12 19s-7-4.4-7-9.5A3.8 3.8 0 0 1 12 7a3.8 3.8 0 0 1 7 2.5C19 14.6 12 19 12 19z"/>`)},
	{"flag", icon(tone(`path d="M6 5h11l-2 4 2 4H6z"`) + `<path d="M6 20V4.5M6 5h11l-2 4 2 4H6"/>`)},
	{"bulb", icon(tone(`path d="M12 3.5a6 6 0 00-3.5 10.9V17h7v-2.6A6 6 0 0012 3.5z"`) + `<path d="M9 17.5h6M10 20.5h4M12 3.5a6 6 0 0 0-3.5 10.9V17h7v-2.6A6 6 0 0 0 12 3.5z"/>`)},
	{"search", iconSearch},
	{"code", icon(tone(`rect x="3" y="4.5" width="18" height="15" rx="3"`) + `<path d="m9 8-4 4 4 4M15 8l4 4-4 4"/>`)},
	{"briefcase", icon(tone(`rect x="3.5" y="7.5" width="17" height="12" rx="2"`) + `<rect x="3.5" y="7.5" width="17" height="12" rx="2"/><path d="M9 7.5V5.5a1.5 1.5 0 0 1 1.5-1.5h3A1.5 1.5 0 0 1 15 5.5v2M3.5 12.5h17"/>`)},
	{"home", icon(tone(`path d="M4.5 11 12 4.5l7.5 6.5V19a1 1 0 01-1 1h-13a1 1 0 01-1-1z"`) + `<path d="M4.5 11 12 4.5l7.5 6.5V19a1 1 0 0 1-1 1h-4v-5h-5v5h-4a1 1 0 0 1-1-1z"/>`)},
	{"people", icon(tone(`circle cx="9" cy="9" r="3"`) + `<circle cx="9" cy="9" r="3"/><path d="M3.5 19a5.5 5.5 0 0 1 11 0"/><path d="M15.5 6.5a3 3 0 0 1 0 5.5M17.5 14a5.5 5.5 0 0 1 3 5"/>`)},
	{"music", icon(tone(`circle cx="6.5" cy="18" r="2.5"`) + tone(`circle cx="16.5" cy="15.5" r="2.5"`) + `<path d="M9 18V5.5l10-2v12"/><circle cx="6.5" cy="18" r="2.5"/><circle cx="16.5" cy="15.5" r="2.5"/>`)},
	{"globe", icon(tone(`circle cx="12" cy="12" r="8"`) + `<circle cx="12" cy="12" r="8"/><path d="M4 12h16M12 4c2.5 2.5 3.5 5 3.5 8s-1 5.5-3.5 8c-2.5-2.5-3.5-5-3.5-8s1-5.5 3.5-8z"/>`)},
}

// collGlyphOf is the icon named name, the journal's calendar for one with none, or the stack.
func collGlyphOf(name string, journal bool) *ui.SVG {
	for _, ic := range collIcons {
		if ic.name == name && name != "" {
			return ic.glyph
		}
	}
	if journal {
		return iconCalendar
	}
	return iconStack
}
