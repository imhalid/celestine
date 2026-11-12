package main

import (
	"slices"
	"strconv"
)

// The settings' dropdown: the gear at the top left opens a short menu of the
// settings one reaches for most - the look, the color, the caret, the corners -
// each a list of its own, and "Tüm ayarlar…" for the dialog with all of them.

// gearMenu opens the dropdown under the gear at x (the gear's left edge).
func (a *app) gearMenu(x, y float32) {
	a.showContext(x, y, a.quickSettings())
}

// tick marks the choice in effect in a list.
func tick(on bool) string {
	if on {
		return "✓"
	}
	return ""
}

// quickSettings are the lines of the dropdown. With reduced motion the caret is plain whatever is chosen, so its list is
// not one of them (#402).
func (a *app) quickSettings() []menuItem {
	look := []string{"common_system", "common_light", "common_dark"}
	items := []menuItem{
		a.into("common_theme", iconTheme, func() []menuItem {
			var out []menuItem
			for i, name := range look {
				i := i
				out = append(out, menuItem{title: name, hint: tick(a.settings.Theme%3 == i), run: func() {
					a.settings.Theme = i
					a.saveSettingsSoon()
				}})
			}
			return out
		}),
		a.into("common_colorTheme", iconTheme, func() []menuItem {
			var out []menuItem
			for i, t := range appThemes {
				i := i
				out = append(out, menuItem{title: t.name, hint: tick(a.settings.AppTheme%len(appThemes) == i), run: func() { a.setAppTheme(i) }})
			}
			return out
		}),
		a.into("caret", iconEditing, func() []menuItem {
			var out []menuItem
			for k := caretKind(0); k < caretCount(); k++ {
				k := k
				out = append(out, menuItem{title: caretName(k), hint: tick(a.caretKind == k), run: func() { a.setCaret(k) }})
			}
			return out
		}),
		a.into("common_cursorFx", iconEditing, func() []menuItem {
			out := []menuItem{{title: "common_off", hint: tick(a.settings.CursorFX == ""), run: func() { a.settings.CursorFX = ""; a.saveSettingsSoon() }}}
			for _, p := range cursorPresets {
				name := p.name
				out = append(out, menuItem{title: name, hint: tick(a.settings.CursorFX == name), run: func() { a.settings.CursorFX = name; a.saveSettingsSoon() }})
			}
			return out
		}),
		a.into("common_corners", iconBar, func() []menuItem {
			var out []menuItem
			for _, v := range []int{0, 50, 100, 150, 200} {
				v := v
				out = append(out, menuItem{title: strconv.Itoa(v) + "%", hint: tick(a.cornerPercent() == v), run: func() {
					a.settings.Corner = &v
					a.saveSettingsSoon()
				}})
			}
			return out
		}),
		{title: "sidebar", hint: "⌘\\", glyph: iconSidebar, run: a.toggleSidebar},
		{title: "allSettings", hint: "⌘,", group: "~", glyph: iconSettings, run: a.openSettings},
	}
	if fx.still {
		items = slices.DeleteFunc(items, func(m menuItem) bool { return m.title == "caret" })
	}
	return items
}
