package main

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The context menus (ctxmenu.go) and a block's menu (blockui.go) are MyGo's menus drawn in the window
// (PopoverMenuButton): their keys, submenus, focus, placement inside the window and what assistive technology sees
// are MyGo's. The app keeps its lists as menuItems, as the / menu, which stays its own (menupanel.go: a field and a
// calendar inside it), has them; fillMenu writes them into a MyGo menu (#305).

// fillMenu writes items into m, in the window's language, and runs the one chosen; chosen is told of it first.
// A line with items of its own (into) is a submenu; a group other than the one before is a separator over it.
func fillMenu(m *ui.Menu, items []menuItem, chosen func()) {
	group := ""
	for i, it := range items {
		if it.back { // MyGo's submenus have their own way back
			continue
		}
		if it.group != group {
			if i > 0 {
				m.Separator()
			}
			group = it.group
		}
		if it.items != nil {
			sub := it.items
			mi := m.Submenu(msg(it.title), func(sm *ui.Menu) { fillMenu(sm, sub(), chosen) })
			mi.Icon(menuIcon(it.glyph))
			continue
		}
		mi := m.Item(msg(it.title))
		if it.sub != "" {
			mi.Subtitle(msg(it.sub))
		}
		if it.hint != "" {
			mi.Trailing(it.hint)
		}
		if it.danger {
			mi.Danger()
		}
		if it.dot != nil {
			mi.Icon(menuIcon(iconDot)).IconColor(*it.dot)
		} else {
			mi.Icon(menuIcon(it.glyph))
		}
		if mi.Chosen() && it.run != nil {
			chosen()
			it.run()
		}
	}
}

// iconDot is a group's dot in a menu, taking the group's color (IconColor).
var iconDot = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><rect x="7" y="7" width="10" height="10" rx="3.5"/></svg>`))

// menuIcons are the menu icons made of the app's glyphs, each made once.
var menuIcons sync.Map // *ui.SVG → ui.MenuIcon

// menuIcon is glyph as a menu item's icon, in the color of the item's text; nil for none.
func menuIcon(glyph *ui.SVG) ui.MenuIcon {
	if glyph == nil {
		return nil
	}
	if ic, ok := menuIcons.Load(glyph); ok {
		return ic.(ui.MenuIcon)
	}
	ic, err := mygo.NewMenuIconSVG(glyph, true)
	if err != nil {
		menuIcons.Store(glyph, ui.MenuIcon(nil))
		return nil
	}
	menuIcons.Store(glyph, ui.MenuIcon(ic))
	return ic
}

// menuPanelLook gives MyGo's menu panels, and their submenus', the app's own menu look, the / menu's
// (menupanel.go) on a lighter step: the theme's surface and hairline. MyGo's own is the theme's background, pure
// black in the dark.
func menuPanelLook(c *ui.Context) func(ui.Element) {
	cl := ryugo.From(c).Colors
	return func(panel ui.Element) { panel.Background(cl.Surface).Border(1, cl.Hairline) }
}

// menuAt draws MyGo's menu of items at the box at in the window, below it (above where the window has no room),
// while *open: an anchor of its own, so the menu opens from anywhere the app asks, not only from a click on it.
// It reports whether an item was chosen in this frame.
func menuAt(c *ui.Context, key, label string, at ui.Rect, open *bool, items []menuItem) (chosen bool) {
	ui.Overlay(c, func() {
		anchor := ui.Box(c.Key(key)).Absolute().Left(at.X).Top(at.Y).Size(max(at.W, 1), max(at.H, 1)).Label(msg(label)).PassThrough()
		anchor.PopoverMenuButton(func(m *ui.Menu) {
			m.MaxWidth(menuW + 40)
			fillMenu(m, items, func() { chosen = true })
		}, ui.PopoverMenuButtonOptions{Open: open, Panel: menuPanelLook(c)})
	})
	return chosen
}
