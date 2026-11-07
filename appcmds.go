package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The app's commands, in one list: the menu bar is made from it (menu.go), the window takes their keys
// from it when there is no menu bar (the tests, the web), the palette lists them, the keys a command of
// one's own cannot have are read from it, and an agent lists and runs them through MCP (list_commands,
// run_command: the palette's whole list, these and the rest). A command is written once, here.

// appCmd is a command of the app.
type appCmd struct {
	title string
	accel string // its key, as parseAccel reads it ("cmd+shift+n"); "" none
	keys  string // what its key is shown as, when it is not accel (the quick note's global key)
	menu  string // the menu it is in: "app", "file", "view", "block", "go"; "" none
	sep   bool   // a line above it in its menu
	dots  bool   // it opens something to fill in: "…" after it in the menu
	noPal bool   // not in the palette (another line there says it better)
	glyph *ui.SVG
	run   func(a *app)
	off   func(a *app) string // why it cannot run now; nil: it always can
}

// noPageOff is why a command of the page cannot run: there is no page.
func noPageOff(a *app) string {
	if a.page == nil || a.doc == nil || a.noPage() {
		return T("Açık sayfa yok")
	}
	return ""
}

// onePaneOff is why a command of the panes cannot run: there is one pane.
func onePaneOff(a *app) string {
	if a.paneCount() < 2 {
		return T("Tek bölme var")
	}
	return ""
}

// appCommands is the list. It is made anew each time: the commands are closures on nothing but the app.
func appCommands() []appCmd {
	caret := func(a *app) int { _, c := a.doc.Selection(); return c }
	return []appCmd{
		{title: "Ayarlar", accel: "cmd+,", menu: "app", dots: true, glyph: iconSettings, run: (*app).toggleSettings},

		{title: "Yeni sayfa", accel: "cmd+n", menu: "file", noPal: true, glyph: iconPlus, run: func(a *app) {
			switch {
			case a.coll != nil:
				a.newPage(a.coll, nil)
			case a.page != nil:
				a.newPage(a.page.Coll, nil)
			case len(a.lib.Collections) > 0:
				a.newPage(a.lib.Collections[0], nil)
			}
		}},
		{title: "Yeni koleksiyon", accel: "cmd+shift+n", menu: "file", glyph: iconStack, run: func(a *app) {
			if c, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
				a.newPage(c, nil)
			} else {
				a.saveErr = err.Error()
			}
		}},
		{title: "Çöp", menu: "file", sep: true, glyph: iconTrash, run: (*app).openTrash},
		{title: "Kaydet", accel: "cmd+s", menu: "file", glyph: iconExport, run: (*app).saveNow},

		{title: "Kenar çubuğunu aç/kapat", accel: "cmd+\\", menu: "view", glyph: iconSidebar, run: (*app).toggleSidebar},
		{title: "Yan paneli aç/kapat", accel: "cmd+shift+\\", menu: "view", glyph: iconPanel, run: (*app).togglePanel, off: phoneOff},
		{title: "Caret'i değiştir", accel: "cmd+shift+m", menu: "view", glyph: iconPage, run: func(a *app) { a.setCaret((a.caretKind + 1) % caretCount()) }},
		{title: "Yazıyı büyüt", accel: "cmd+=", menu: "view", glyph: iconEditing, run: func(a *app) { a.stepFont(1) }},
		{title: "Yazıyı küçült", accel: "cmd+-", menu: "view", glyph: iconEditing, run: func(a *app) { a.stepFont(-1) }},
		{title: "Yazı boyutunu sıfırla", accel: "cmd+0", menu: "view", glyph: iconEditing, run: (*app).resetFont},
		{title: "Yan bölmede aç: bu ekran", accel: "alt+cmd+enter", menu: "view", sep: true, glyph: iconPanel, run: (*app).openHereInSide, off: phoneOff},
		{title: "Yeni bölme: bu ekran", menu: "view", glyph: iconPanel, run: (*app).newPaneHere, off: phoneOff},
		{title: "Bölmeler arası geç", accel: "alt+cmd+right", menu: "view", glyph: iconPanel, run: (*app).nextPane, off: onePaneOff},
		{title: "Bölmeyi kapat", accel: "alt+cmd+w", menu: "view", glyph: iconClose, run: (*app).closeThisPane, off: onePaneOff},
		{title: "Bölmeyi ayrı pencereye al", menu: "view", glyph: iconPanel, run: (*app).tearOffPane, off: func(a *app) string {
			switch {
			case phone:
				return T("Telefonda yok")
			case a.dockHost == nil:
				return T("Burada ayrı pencere açılamıyor")
			case a.activeElsewhere():
				return T("Bu bölme zaten ayrı bir pencerede")
			}
			return ""
		}},

		{title: "Bloğu yukarı taşı", accel: "cmd+shift+up", menu: "block", glyph: iconStack, run: func(a *app) { moveBlock(a.doc, -1) }, off: noPageOff},
		{title: "Bloğu aşağı taşı", accel: "cmd+shift+down", menu: "block", glyph: iconStack, run: func(a *app) { moveBlock(a.doc, 1) }, off: noPageOff},
		{title: "Bloğu çoğalt", accel: "cmd+d", menu: "block", glyph: iconStack, run: func(a *app) { duplicateBlock(a.doc, caret(a)) }, off: noPageOff},
		{title: "Katla / aç", accel: "cmd+/", menu: "block", glyph: iconChevron, run: func(a *app) { a.toggleFold(caret(a)) }, off: noPageOff},

		{title: "Sayfa ara", accel: "cmd+k", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openPalette},
		{title: "Komutlar", accel: "cmd+p", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openCommands},
		{title: "Komutlar (⇧⌘P)", accel: "cmd+shift+p", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openCommands},
		{title: "Sayfada bul", accel: "cmd+f", menu: "go", dots: true, glyph: iconSearch, run: (*app).openFind, off: noPageOff},
		{title: "Kitaplıkta ara", accel: "cmd+shift+f", menu: "go", dots: true, glyph: iconSearch, run: func(a *app) { a.openSearch(a.searchQ) }},
		{title: "Bugünün sayfası", accel: "cmd+j", menu: "go", sep: true, glyph: iconCalendar, run: (*app).openToday},
		{title: "Hızlı not", menu: "go", dots: true, glyph: iconPlus, run: (*app).showQuick, off: func(a *app) string {
			if a.quick == nil {
				return T("Burada hızlı not paneli yok")
			}
			return ""
		}},
		{title: "Geri", accel: "cmd+[", menu: "go", glyph: iconBack, run: func(a *app) { a.step(-1) }},
		{title: "İleri", accel: "cmd+]", menu: "go", glyph: iconChevron, run: func(a *app) { a.step(1) }},
	}
}

// accelLabel is a key as the palette shows it: "cmd+shift+n" is ⇧⌘N.
func accelLabel(accel string) string {
	if accel == "" {
		return ""
	}
	parts := strings.Split(accel, "+")
	key := parts[len(parts)-1]
	mods := ""
	for _, m := range []struct{ name, sign string }{{"ctrl", "⌃"}, {"alt", "⌥"}, {"shift", "⇧"}, {"cmd", "⌘"}} {
		for _, p := range parts[:len(parts)-1] {
			if p == m.name {
				mods += m.sign
			}
		}
	}
	switch key {
	case "enter":
		key = "↵"
	case "up":
		key = "↑"
	case "down":
		key = "↓"
	case "left":
		key = "←"
	case "right":
		key = "→"
	default:
		key = strings.ToUpper(key)
	}
	return mods + key
}

// menuAccel is a key as the menu bar takes it: "cmd+shift+n" is CmdOrCtrl+Shift+N.
func menuAccel(accel string) string {
	if accel == "" {
		return ""
	}
	parts := strings.Split(accel, "+")
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		switch {
		case i == len(parts)-1:
			switch p {
			case "enter", "up", "down", "left", "right":
				p = strings.ToUpper(p[:1]) + p[1:]
			default:
				p = strings.ToUpper(p)
			}
			out = append(out, p)
		case p == "cmd":
			out = append(out, "CmdOrCtrl")
		default:
			out = append(out, strings.ToUpper(p[:1])+p[1:])
		}
	}
	// the menu bar wants the modifiers in its own order, CmdOrCtrl first
	if len(out) > 2 {
		mods := out[:len(out)-1]
		for i, m := range mods {
			if m == "CmdOrCtrl" && i > 0 {
				copy(mods[1:i+1], mods[:i])
				mods[0] = "CmdOrCtrl"
			}
		}
	}
	return strings.Join(out, "+")
}

// appAccels are the keys of the app's commands, by their parseAccel names: a command of one's own
// cannot have them.
func appAccels() map[string]string {
	out := map[string]string{}
	for _, c := range appCommands() {
		if _, _, norm, ok := parseAccel(c.accel); ok {
			out[norm] = strings.ToLower(T(c.title))
		}
	}
	return out
}

// appKeys runs the commands whose keys were pressed, in the window: when there is no menu bar to take them.
func (a *app) appKeys(c *ui.Context) {
	for _, cmd := range appCommands() {
		mods, key, _, ok := parseAccel(cmd.accel)
		if !ok || !c.Shortcut(mods, key) {
			continue
		}
		if cmd.off == nil || cmd.off(a) == "" {
			cmd.run(a)
		}
	}
}

// appPalette are the app's commands as lines of the palette.
func (a *app) appPalette() []palItem {
	var out []palItem
	for _, cmd := range appCommands() {
		if cmd.noPal {
			continue
		}
		cmd := cmd
		keys := cmd.keys
		if keys == "" {
			keys = accelLabel(cmd.accel)
		}
		if cmd.title == "Hızlı not" {
			keys = a.quickKeys()
		}
		it := palItem{title: T(cmd.title), sub: "Komut", keys: keys, glyph: cmd.glyph, run: func() { cmd.run(a) }}
		if cmd.off != nil {
			it.off = cmd.off(a)
		}
		out = append(out, it)
	}
	return out
}

// menuBarItems are the app's commands of the menu named group, as the menu bar's items.
func (a *app) menuBarItems(group string) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for _, cmd := range appCommands() {
		if cmd.menu != group {
			continue
		}
		cmd := cmd
		if cmd.sep && len(out) > 0 {
			out = append(out, mygo.Separator())
		}
		label := T(cmd.title)
		if cmd.dots {
			label += "…"
		}
		out = append(out, &mygo.MenuItem{Label: label, Accelerator: menuAccel(cmd.accel), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.do(func() error {
				if cmd.off == nil || cmd.off(a) == "" {
					cmd.run(a)
				}
				return nil
			})
		}})
	}
	return out
}
