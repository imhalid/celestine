package main

import (
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
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
	if a.pane().page == nil || a.pane().doc == nil || a.noPage() {
		return locales.CommandsNoPageOpen()
	}
	return ""
}

// onePaneOff is why a command of the panes cannot run: there is one pane.
func onePaneOff(a *app) string {
	if a.paneCount() < 2 {
		return locales.CommandsThereOnePane()
	}
	return ""
}

// appCommands is the list. It is made anew each time: the commands are closures on nothing but the app.
func appCommands() []appCmd {
	caret := func(a *app) int { _, c := a.pane().doc.Selection(); return c }
	return []appCmd{
		{title: "common_settings", accel: "cmd+,", menu: "app", dots: true, glyph: iconSettings, run: (*app).toggleSettings},
		{title: "showTheWelcomeScreen", glyph: iconSearch, run: (*app).welcomeAgain},
		{title: "debug_toggle", glyph: iconSettings, run: (*app).toggleDebug},

		{title: "common_newPage", accel: "cmd+n", menu: "file", noPal: true, glyph: iconPlus, run: func(a *app) {
			switch {
			case a.pane().coll != nil:
				a.newPage(a.pane().coll, nil)
			case a.pane().page != nil:
				a.newPage(a.pane().page.Coll, nil)
			case len(a.lib.Collections) > 0:
				a.newPage(a.lib.Collections[0], nil)
			}
		}},
		{title: "newCollection", accel: "cmd+shift+n", menu: "file", glyph: iconStack, run: func(a *app) {
			if c, err := a.lib.NewCollection(msg("newCollection")); err == nil {
				a.newPage(c, nil)
			} else {
				a.fail(err)
			}
		}},
		{title: "common_trash", menu: "file", sep: true, glyph: iconTrash, run: (*app).openTrash},
		{title: "common_save", accel: "cmd+s", menu: "file", glyph: iconExport, run: (*app).saveNow},

		{title: "toggleTheSidebar", accel: "cmd+\\", menu: "view", glyph: iconSidebar, run: (*app).toggleSidebar},
		{title: "toggleTheSidePanel", accel: "cmd+shift+\\", menu: "view", glyph: iconPanel, run: (*app).togglePanel, off: phoneOff},
		{title: "changeTheCaret", accel: "cmd+shift+m", menu: "view", glyph: iconPage, run: func(a *app) { a.setCaret((a.caretKind + 1) % caretCount()) }},
		{title: "view_zoomIn", accel: "cmd+=", menu: "view", sep: true, glyph: iconMaximize, run: func(a *app) { a.stepZoom(1) }},
		{title: "view_zoomOut", accel: "cmd+-", menu: "view", glyph: iconMinimize, run: func(a *app) { a.stepZoom(-1) }},
		{title: "view_actualSize", accel: "cmd+0", menu: "view", glyph: iconSearch, run: func(a *app) { a.setZoom(1) }},
		{title: "biggerText", accel: "alt+cmd+=", menu: "view", glyph: iconEditing, run: func(a *app) { a.stepFont(1) }},
		{title: "smallerText", accel: "alt+cmd+-", menu: "view", glyph: iconEditing, run: func(a *app) { a.stepFont(-1) }},
		{title: "resetTheFontSize", menu: "view", glyph: iconEditing, run: (*app).resetFont},
		{title: "panes_openHereInSide", accel: "alt+cmd+enter", menu: "view", sep: true, glyph: iconPanel, run: (*app).openHereInSide, off: phoneOff},
		{title: "newPaneThisScreen", menu: "view", glyph: iconPanel, run: (*app).newPaneHere, off: phoneOff},
		{title: "switchPane", accel: "alt+cmd+right", menu: "view", glyph: iconPanel, run: (*app).nextPane, off: onePaneOff},
		{title: "closeThePane", accel: "alt+cmd+w", menu: "view", glyph: iconClose, run: (*app).closeThisPane, off: onePaneOff},
		{title: "moveThePaneToAWindow", menu: "view", glyph: iconPanel, run: (*app).tearOffPane, off: func(a *app) string {
			switch {
			case phone:
				return locales.CommonNotPhone()
			case a.dockHost == nil:
				return locales.CommandsNoWindowOwnCan()
			case a.activeElsewhere():
				return locales.CommandsThisPaneWindowOwn()
			}
			return ""
		}},

		{title: "moveTheBlockUp", accel: "cmd+shift+up", menu: "block", glyph: iconStack, run: func(a *app) { moveBlock(a.pane().doc, -1) }, off: noPageOff},
		{title: "moveTheBlockDown", accel: "cmd+shift+down", menu: "block", glyph: iconStack, run: func(a *app) { moveBlock(a.pane().doc, 1) }, off: noPageOff},
		{title: "duplicateTheBlock", accel: "cmd+d", menu: "block", glyph: iconStack, run: func(a *app) { duplicateBlock(a.pane().doc, caret(a), a.copyChipsIn) }, off: noPageOff},
		{title: "foldUnfold", accel: "cmd+/", menu: "block", glyph: iconChevron, run: func(a *app) { a.toggleFold(caret(a)) }, off: noPageOff},
		{title: "zoomIntoTheBlock", accel: zoomInKey, menu: "block", glyph: iconChevron, run: func(a *app) { a.zoomIn(caret(a)) }, off: noPageOff},
		{title: "zoomOut", accel: zoomOutKey, menu: "block", glyph: iconChevron, run: (*app).zoomOut, off: func(a *app) string {
			if !a.zoomed() {
				return "thePageIsNotZoomedInto"
			}
			return ""
		}},
		{title: "attachAFile", menu: "block", dots: true, glyph: iconAttach, run: (*app).pickFileToAttach, off: noPageOff},
		{title: "selectBlocks", menu: "block", glyph: iconStack, run: (*app).startBlockSel, off: noPageOff},
		{title: "common_blockMenu", accel: "alt+cmd+.", menu: "block", dots: true, glyph: iconStack, run: (*app).openBlockMenu, off: noPageOff},
		{title: "openBlockBeside", menu: "block", glyph: iconPanel, run: (*app).openBlockBeside, off: noPageOff},
		{title: "copyAsMarkdown", accel: "cmd+shift+c", menu: "block", sep: true, glyph: iconCopy, run: func(a *app) { a.copyAs("markdown") }, off: noPageOff},
		{title: "copyAsPlainText", menu: "block", glyph: iconCopy, run: func(a *app) { a.copyAs("plain") }, off: noPageOff},
		{title: "copyAsHtml", menu: "block", glyph: iconCopy, run: func(a *app) { a.copyAs("html") }, off: noPageOff},
		{title: "pasteAsMarkdown", accel: "alt+cmd+v", menu: "block", glyph: iconPaste, run: func(a *app) { a.pasteAs("markdown") }, off: noPageOff},
		{title: "pasteAsPlainText", accel: "cmd+shift+v", menu: "block", glyph: iconPaste, run: func(a *app) { a.pasteAs("plain") }, off: noPageOff},

		{title: "common_findPage", accel: "cmd+k", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openPalette},
		{title: "common_commands", accel: "cmd+p", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openCommands},
		{title: "commandsP", accel: "cmd+shift+p", menu: "go", dots: true, noPal: true, glyph: iconSearch, run: (*app).openCommands},
		{title: "findInPage2", accel: "cmd+f", menu: "go", dots: true, glyph: iconSearch, run: (*app).openFind, off: noPageOff},
		{title: "searchTheLibrary", accel: "cmd+shift+f", menu: "go", dots: true, glyph: iconSearch, run: func(a *app) { a.openSearch(a.pane().searchQ) }},
		{title: "todaysPage", accel: "cmd+j", menu: "go", sep: true, glyph: iconCalendar, run: (*app).openToday},
		{title: "previousDay", menu: "go", glyph: iconCalendar, run: func(a *app) { a.stepDay(-1) }},
		{title: "nextDay", menu: "go", glyph: iconCalendar, run: func(a *app) { a.stepDay(1) }},
		{title: "common_goDate", menu: "go", dots: true, glyph: iconCalendar, run: (*app).askDate},
		{title: "common_quickNote", menu: "go", dots: true, glyph: iconPlus, run: (*app).showQuick, off: func(a *app) string {
			if a.quick == nil {
				return locales.CommandsThereNoQuickNote()
			}
			return ""
		}},
		{title: "common_back", accel: "cmd+[", menu: "go", glyph: iconBack, run: func(a *app) { a.step(-1) }},
		{title: "forward", accel: "cmd+]", menu: "go", glyph: iconChevron, run: func(a *app) { a.step(1) }},
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
			out[norm] = strings.ToLower(msg(c.title))
		}
	}
	return out
}

// appKeys runs the commands whose keys were pressed, in the window: when there is no menu bar to take them.
func (a *app) appKeys(c *ui.Context) {
	for _, k := range appKeyed() {
		if !c.Shortcut(k.mods, k.key) {
			continue
		}
		if k.cmd.off == nil || k.cmd.off(a) == "" {
			k.cmd.run(a)
		}
	}
}

// keyedCmd is a command of the app and its key, parsed.
type keyedCmd struct {
	cmd  appCmd
	mods ui.Modifiers
	key  ui.Key
}

// appKeyed are the app's commands that have keys, with them parsed: once, where every command was made and every key
// parsed at each frame where there is no menu bar (#204). The commands and their keys are the same for the app's life.
func appKeyed() []keyedCmd {
	madeKeyed.Do(func() {
		for _, cmd := range appCommands() {
			if mods, key, _, ok := parseAccel(cmd.accel); ok {
				keyedList = append(keyedList, keyedCmd{cmd, mods, key})
			}
		}
	})
	return keyedList
}

var (
	madeKeyed sync.Once
	keyedList []keyedCmd
)

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
		if cmd.title == "common_quickNote" {
			keys = a.quickKeys()
		}
		it := palItem{title: msg(cmd.title), sub: locales.PaletteCommand(), kind: palCommand, id: cmd.title, keys: keys, glyph: cmd.glyph, run: func() { cmd.run(a) }, cat: cmdCat(cmd.title), fam: cmdFam(cmd.title), name: cmd.title}
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
		label := msg(cmd.title)
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
