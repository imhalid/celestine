package main

import (
	"cmp"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Shortcuts of one's own and commands of one's own. A command of the palette can be given a key
// ("cmd+shift+y"); a macro is a name for a few commands that run one after another ("Günlüğü aç + yeni görev").
// Both are in the settings (a page of the settings screen) and in the settings file, and neither can take a key
// that something else has: the app's own and the system's are told apart, and refused.

// Settings.Keys: a command (its title, as the palette lists it) to its accelerator.
// Settings.Macros: a name to the titles of the commands it runs.

// accelNames are the keys that are not a letter or a digit.
var accelNames = map[string]ui.Key{
	"enter": ui.KeyEnter, "esc": ui.KeyEscape, "space": ui.KeySpace, "tab": ui.KeyTab, "backspace": ui.KeyBackspace,
	"up": ui.KeyUp, "down": ui.KeyDown, "left": ui.KeyLeft, "right": ui.KeyRight, "/": ui.KeySlash, ",": ui.KeyComma, ".": ui.KeyPeriod,
	";": ui.KeySemicolon, "'": ui.KeyQuote, "[": ui.KeyBracketLeft, "]": ui.KeyBracketRight, "\\": ui.KeyBackslash, "-": ui.KeyMinus, "=": ui.KeyEqual,
}

// parseAccel reads "cmd+shift+y" into modifiers and a key; the key is last, and cmd, ctrl, shift and alt come before it.
func parseAccel(s string) (ui.Modifiers, ui.Key, string, bool) {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "")), "+")
	if len(parts) < 2 || parts[len(parts)-1] == "" {
		return 0, 0, "", false
	}
	var mods ui.Modifiers
	norm := []string{}
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "cmd", "⌘", "meta":
			mods |= ui.Cmd
			norm = append(norm, "cmd")
		case "ctrl", "control", "⌃":
			mods |= ui.Ctrl
			norm = append(norm, "ctrl")
		case "shift", "⇧":
			mods |= ui.Shift
			norm = append(norm, "shift")
		case "alt", "opt", "option", "⌥":
			mods |= ui.Alt
			norm = append(norm, "alt")
		default:
			return 0, 0, "", false
		}
	}
	k := parts[len(parts)-1]
	var key ui.Key
	switch {
	case len(k) == 1 && k[0] >= 'a' && k[0] <= 'z':
		key = kbLetterKeys[k[0]-'a']
	case len(k) == 1 && k[0] >= '0' && k[0] <= '9':
		key = kbDigitKeys[k[0]-'0']
	default:
		var ok bool
		if key, ok = accelNames[k]; !ok {
			return 0, 0, "", false
		}
	}
	if mods == 0 { // a key with no modifier would take the typing
		return 0, 0, "", false
	}
	sort.Strings(norm)
	return mods, key, strings.Join(norm, "+") + "+" + k, true
}

// reservedAccels are keys the editor or the system has; the keys of the app's own commands are in their
// list (appAccels). A command of one's own is refused both.
var reservedAccels = map[string]func() string{
	"cmd+1": locales.KeysFocusSidebar, "cmd+2": locales.KeysSidePanel, "cmd+3": locales.KeysFocusText, "ctrl+n": locales.KeysNewPage,
	"cmd+b": locales.KeysBold, "cmd+i": locales.KeysItalic, "cmd+u": locales.KeysUnderline, "cmd+z": locales.KeysUndo, "cmd+shift+z": locales.KeysRedo,
	"cmd+x": locales.KeysCut, "cmd+c": locales.KeysCopy, "cmd+v": locales.KeysPaste, "cmd+a": locales.KeysSelectAll,
	"cmd+q": locales.KeysSysQuit, "cmd+w": locales.KeysSysCloseWindow, "cmd+h": locales.KeysSysHide, "cmd+space": locales.KeysSysSearch,
	"cmd+tab": locales.KeysSysSwitch, "cmd+m": locales.KeysSysMinimize, "cmd+t": locales.KeysBrowserTab,
	"cmd+l": locales.KeysBrowserAddress, "cmd+r": locales.KeysBrowserReload,
}

// accelConflict says what a key is already for, or "" when it is free (also from the other commands).
func (a *app) accelConflict(norm, forCmd string) string {
	if r, ok := appAccels()[norm]; ok { // the app's commands
		return r
	}
	if r, ok := reservedAccels[norm]; ok {
		return r()
	}
	for cmd, k := range a.settings.Keys {
		if k == norm && cmd != forCmd {
			return locales.KeysTheCommand(T(cmd))
		}
	}
	return ""
}

// bindKey gives a command a key; it says why when it cannot.
func (a *app) bindKey(cmd, accel string) error {
	_, _, norm, ok := parseAccel(accel)
	if !ok {
		return errString(locales.KeysErrKey(accel))
	}
	it := a.commandByTitle(cmd)
	if it == nil {
		return errString(locales.KeysErrNoCommand(cmd))
	}
	cmd = cmp.Or(it.id, it.title) // kept by what the command is, whatever the language
	if c := a.accelConflict(norm, cmd); c != "" {
		return errString(locales.KeysErrTaken(norm, c))
	}
	if a.settings.Keys == nil {
		a.settings.Keys = map[string]string{}
	}
	a.settings.Keys[cmd] = norm
	a.saveSettings()
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }

func (a *app) unbindKey(cmd string) {
	delete(a.settings.Keys, cmd)
	a.saveSettings()
}

// commandByTitle is the command of the palette with that title (or the first that matches it best).
func (a *app) commandByTitle(title string) *palItem {
	all := a.commands()
	for i := range all {
		if (all[i].title == title || all[i].id == title) && all[i].off == "" {
			return &all[i]
		}
	}
	best, score := -1, -1
	for i := range all {
		if all[i].off != "" {
			continue
		}
		if s := match(title, all[i].title); s > score && fold(all[i].title) != "" {
			best, score = i, s
		}
	}
	if best >= 0 && score >= 300 { // every word of it is in the title
		return &all[best]
	}
	return nil
}

// runCommand runs a command of the palette by its title; a macro by its name.
func (a *app) runCommand(title string) bool {
	if it := a.commandByTitle(title); it != nil && it.run != nil {
		it.run()
		return true
	}
	return false
}

// runMacro runs the commands of a macro, one after another.
func (a *app) runMacro(name string) int {
	n := 0
	for _, t := range a.settings.Macros[name] {
		if !a.runCommand(strings.TrimSpace(t)) {
			a.say(toastErr, locales.KeysMacroMissingCommand(name, t))
			break
		}
		n++
	}
	return n
}

// saveMacro keeps a macro: a name and the commands, separated by "+" or ";"; they must all be there.
func (a *app) saveMacro(name, list string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errString(locales.KeysErrMacroName())
	}
	var cmds []string
	for _, t := range strings.FieldsFunc(list, func(r rune) bool { return r == '+' || r == ';' }) {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		it := a.commandByTitle(t)
		if it == nil {
			return errString(locales.KeysErrNoCommand(t))
		}
		cmds = append(cmds, cmp.Or(it.id, it.title))
	}
	if len(cmds) == 0 {
		return errString(locales.KeysErrMacroEmpty())
	}
	if a.settings.Macros == nil {
		a.settings.Macros = map[string][]string{}
	}
	a.settings.Macros[name] = cmds
	a.saveSettings()
	return nil
}

// boundKeys runs the command whose key was pressed.
func (a *app) boundKeys(c *ui.Context) {
	if len(a.settings.Keys) == 0 || a.modal() || a.pal.open {
		return
	}
	for cmd, norm := range a.settings.Keys {
		mods, key, _, ok := parseAccel(norm)
		if ok && c.Shortcut(mods, key) {
			if !a.runCommand(cmd) {
				a.say(toastErr, locales.KeysCommandNotFound(cmd, norm))
			}
			return
		}
	}
}

// settingsKeys is the page of the settings with the keys and the macros.
func (a *app) settingsKeys(c *ui.Context) {
	cl := ryugo.From(c).Colors
	var rows []func()
	var cmds []string
	for k := range a.settings.Keys {
		cmds = append(cmds, k)
	}
	sort.Strings(cmds)
	for _, k := range cmds {
		k := k
		rows = append(rows, func() {
			settingRow(c, T(k), a.settings.Keys[k], func() {
				if ryugo.Button(c.Key("key-del:"+k), locales.KeysRemove(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.unbindKey(k)
				}
			})
		})
	}
	if len(rows) > 0 {
		ui.Text(c, locales.KeysYourShortcuts()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		settingGroup(c, "set-keys", rows...)
	}
	settingGroup(c, "set-key-new", func() {
		settingRow(c, locales.KeysGiveCommandShortcut(), locales.KeysTypeCommandsNameKey(), func() {
			ui.Column(c).Gap(6).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("key-cmd"), &a.keyCmd, ryugo.InputOptions{Label: "Komut", Size: ryugo.Sm, Placeholder: locales.KeysCommandName()}), &a.keyCmd, fieldOpts{oneLine: true})
				a.fieldMenu(ryugo.Input(c.Key("key-accel"), &a.keyAccel, ryugo.InputOptions{Label: locales.KeysKey(), Size: ryugo.Sm, Placeholder: "cmd+shift+y"}), &a.keyAccel, fieldOpts{oneLine: true})
				if ryugo.Button(c.Key("key-set"), locales.KeysAssign(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					if err := a.bindKey(a.keyCmd, a.keyAccel); err != nil {
						a.notice = err.Error()
					} else {
						a.keyCmd, a.keyAccel = "", ""
					}
				}
			})
		})
	})
	var mrows []func()
	var names []string
	for n := range a.settings.Macros {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		n := n
		mrows = append(mrows, func() {
			var shown []string
			for _, t := range a.settings.Macros[n] { // kept by what each command is, shown in the window's language
				shown = append(shown, T(t))
			}
			settingRow(c, n, strings.Join(shown, " + "), func() {
				ui.Row(c).Gap(6).Children(func() {
					if ryugo.Button(c.Key("macro-run:"+n), locales.KeysRun(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.runMacro(n)
					}
					if ryugo.Button(c.Key("macro-del:"+n), locales.CommonDelete(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						delete(a.settings.Macros, n)
						a.saveSettings()
					}
				})
			})
		})
	}
	if len(mrows) > 0 {
		ui.Text(c, locales.KeysMacros()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		settingGroup(c, "set-macros", mrows...)
	}
	settingGroup(c, "set-macro-new", func() {
		settingRow(c, locales.KeysWriteMacro(), locales.KeysANameCommandsSeparated(), func() {
			ui.Column(c).Gap(6).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("macro-name"), &a.macroName, ryugo.InputOptions{Label: "Makro adı", Size: ryugo.Sm, Placeholder: locales.KeysMacroName()}), &a.macroName, fieldOpts{oneLine: true})
				a.fieldMenu(ryugo.Input(c.Key("macro-list"), &a.macroList, ryugo.InputOptions{Label: "Komutlar", Size: ryugo.Sm, Placeholder: locales.KeysTodaysPageTasks()}), &a.macroList, fieldOpts{oneLine: true})
				if ryugo.Button(c.Key("macro-save"), locales.CommonSave(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					if err := a.saveMacro(a.macroName, a.macroList); err != nil {
						a.notice = err.Error()
					} else {
						a.macroName, a.macroList = "", ""
					}
				}
			})
		})
	})
}
