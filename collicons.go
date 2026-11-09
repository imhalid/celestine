package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// A collection's icon and color: chosen in its settings, shown in the
// sidebar, on its page and in the palette.

// collIcons are the icons a collection can have, in the picker's order.
var collIcons = []struct {
	Name  string
	Glyph *ui.SVG
}{
	{"", iconStack},
	{"book", icon(`<path d="M5 4.5h10a3 3 0 0 1 3 3v12H8a3 3 0 0 1-3-3z"/><path d="M5 16.5a3 3 0 0 1 3-3h10"/>`)},
	{"note", iconPage},
	{"folder", icon(`<path d="M3.5 7.5a2 2 0 0 1 2-2h4l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z"/>`)},
	{"check", icon(`<rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`)},
	{"calendar", icon(`<rect x="4" y="5.5" width="16" height="14" rx="2"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>`)},
	{"star", icon(`<path d="m12 4 2.5 5.1 5.6.8-4 3.9.9 5.6-5-2.6-5 2.6.9-5.6-4-3.9 5.6-.8z"/>`)},
	{"heart", icon(`<path d="M12 19s-7-4.4-7-9.5A3.8 3.8 0 0 1 12 7a3.8 3.8 0 0 1 7 2.5C19 14.6 12 19 12 19z"/>`)},
	{"flag", icon(`<path d="M6 20V4.5M6 5h11l-2 4 2 4H6"/>`)},
	{"bulb", icon(`<path d="M9 17.5h6M10 20.5h4M12 3.5a6 6 0 0 0-3.5 10.9V17h7v-2.6A6 6 0 0 0 12 3.5z"/>`)},
	{"search", iconSearch},
	{"code", icon(`<path d="m9 8-4 4 4 4M15 8l4 4-4 4"/>`)},
	{"briefcase", icon(`<rect x="3.5" y="7.5" width="17" height="12" rx="2"/><path d="M9 7.5V5.5a1.5 1.5 0 0 1 1.5-1.5h3A1.5 1.5 0 0 1 15 5.5v2M3.5 12.5h17"/>`)},
	{"home", icon(`<path d="M4.5 11 12 4.5l7.5 6.5V19a1 1 0 0 1-1 1h-4v-5h-5v5h-4a1 1 0 0 1-1-1z"/>`)},
	{"people", icon(`<circle cx="9" cy="9" r="3"/><path d="M3.5 19a5.5 5.5 0 0 1 11 0"/><path d="M15.5 6.5a3 3 0 0 1 0 5.5M17.5 14a5.5 5.5 0 0 1 3 5"/>`)},
	{"music", icon(`<path d="M9 18V5.5l10-2v12"/><circle cx="6.5" cy="18" r="2.5"/><circle cx="16.5" cy="15.5" r="2.5"/>`)},
	{"globe", icon(`<circle cx="12" cy="12" r="8"/><path d="M4 12h16M12 4c2.5 2.5 3.5 5 3.5 8s-1 5.5-3.5 8c-2.5-2.5-3.5-5-3.5-8s1-5.5 3.5-8z"/>`)},
}

// collGlyph is the icon of collection c.
func collGlyph(c *Collection) *ui.SVG {
	for _, ic := range collIcons {
		if ic.Name == c.Icon {
			return ic.Glyph
		}
	}
	return iconStack
}

// collColors are the colors a collection can have: Ryugo's chart colors.
var collColors = []string{"", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"}

// colorOf is the color named name in Ryugo's tokens, and whether it is one.
func colorOf(t *ryugo.Tokens, name string) (ui.Color, bool) {
	cl := t.Colors
	switch name {
	case "c1":
		return cl.C1, true
	case "c2":
		return cl.C2, true
	case "c3":
		return cl.C3, true
	case "c4":
		return cl.C4, true
	case "c5":
		return cl.C5, true
	case "c6":
		return cl.C6, true
	case "c7":
		return cl.C7, true
	case "c8":
		return cl.C8, true
	}
	return ui.Color{}, false
}

// collTint is the color of collection c's icon: its own, else muted.
func collTint(c *ui.Context, coll *Collection) ui.Color {
	if col, ok := colorOf(ryugo.From(c), coll.Color); ok {
		return col
	}
	return c.Theme().TextMuted
}

// newPageLabel is "New page", or "New Recipe" when the collection's pages are called so, in the window's language.
func newPageLabel(c *Collection) string {
	if c == nil || c.PageNoun == "" {
		return T("Yeni sayfa")
	}
	if lang == "en" {
		return "New " + c.PageNoun
	}
	return "Yeni " + c.PageNoun
}

// addToDayLabel is the calendar's button that makes a page on a day, by what the collection's pages are called.
func addToDayLabel(c *Collection) string {
	if c == nil || c.PageNoun == "" {
		return T("Bu güne sayfa ekle")
	}
	if lang == "en" {
		return "Add a " + c.PageNoun + " to this day"
	}
	return "Bu güne " + c.PageNoun + " ekle"
}

func init() { enText["Bu güne sayfa ekle"] = "Add a page to this day" }

// pageNoun is what a page of c is called: "sayfa" unless it says.
func pageNoun(c *Collection) string {
	if c != nil && c.PageNoun != "" {
		return c.PageNoun
	}
	return "sayfa"
}
