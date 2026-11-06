package main

import "strconv"

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

// quickSettings are the lines of the dropdown.
func (a *app) quickSettings() []menuItem {
	look := []string{"Sistem", "Açık", "Koyu"}
	return []menuItem{
		a.into("Tema", iconTheme, func() []menuItem {
			var out []menuItem
			for i, name := range look {
				i := i
				out = append(out, menuItem{title: name, hint: tick(a.settings.Theme%3 == i), run: func() {
					a.settings.Theme = i
					a.saveSettings()
				}})
			}
			return out
		}),
		a.into("Renk teması", iconTheme, func() []menuItem {
			var out []menuItem
			for i, t := range appThemes {
				i := i
				out = append(out, menuItem{title: t.name, hint: tick(a.settings.AppTheme%len(appThemes) == i), run: func() { a.setAppTheme(i) }})
			}
			return out
		}),
		a.into("Caret", iconEditing, func() []menuItem {
			var out []menuItem
			for k := caretKind(0); k < caretCount(); k++ {
				k := k
				out = append(out, menuItem{title: caretName(k), hint: tick(a.caretKind == k), run: func() { a.setCaret(k) }})
			}
			return out
		}),
		a.into("Köşeler", iconBar, func() []menuItem {
			var out []menuItem
			for _, v := range []int{0, 50, 100, 150, 200} {
				v := v
				out = append(out, menuItem{title: strconv.Itoa(v) + "%", hint: tick(a.cornerPercent() == v), run: func() {
					a.settings.Corner = &v
					a.saveSettings()
				}})
			}
			return out
		}),
		{title: "Kenar çubuğu", hint: "⌘\\", glyph: iconSidebar, run: a.toggleSidebar},
		{title: "Tüm ayarlar…", hint: "⌘,", group: "~", glyph: iconSettings, run: func() { a.settingsOpen = true }},
	}
}
