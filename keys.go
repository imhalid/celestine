package main

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
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
var reservedAccels = map[string]string{
	"cmd+1": "kenar çubuğuna odaklan", "cmd+2": "yan panel", "cmd+3": "metne odaklan", "ctrl+n": "yeni sayfa",
	"cmd+b": "kalın", "cmd+i": "italik", "cmd+u": "altı çizili", "cmd+z": "geri al", "cmd+shift+z": "yinele",
	"cmd+x": "kes", "cmd+c": "kopyala", "cmd+v": "yapıştır", "cmd+a": "tümünü seç",
	"cmd+q": "sistem: çık", "cmd+w": "sistem: pencereyi kapat", "cmd+h": "sistem: gizle", "cmd+space": "sistem: arama",
	"cmd+tab": "sistem: uygulama değiştir", "cmd+m": "sistem: küçült", "cmd+t": "tarayıcı: yeni sekme",
	"cmd+l": "tarayıcı: adres çubuğu", "cmd+r": "tarayıcı: yenile",
}

// accelConflict says what a key is already for, or "" when it is free (also from the other commands).
func (a *app) accelConflict(norm, forCmd string) string {
	if r, ok := appAccels()[norm]; ok { // the app's commands
		return r
	}
	if r, ok := reservedAccels[norm]; ok {
		return r
	}
	for cmd, k := range a.settings.Keys {
		if k == norm && cmd != forCmd {
			return "“" + cmd + "” komutu"
		}
	}
	return ""
}

// bindKey gives a command a key; it says why when it cannot.
func (a *app) bindKey(cmd, accel string) error {
	_, _, norm, ok := parseAccel(accel)
	if !ok {
		return errString("Anahtar anlaşılmadı: “" + accel + "”. Örnek: cmd+shift+y (cmd, ctrl, shift, alt ve bir tuş)")
	}
	if a.commandByTitle(cmd) == nil {
		return errString("Böyle bir komut yok: “" + cmd + "”")
	}
	if c := a.accelConflict(norm, cmd); c != "" {
		return errString(norm + " zaten " + c + " için kullanılıyor")
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
		if all[i].title == title && all[i].off == "" {
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
			a.notice = Tf("Makro “%s”: “%s” komutu bulunamadı", name, t)
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
		return errString("Makroya bir ad ver")
	}
	var cmds []string
	for _, t := range strings.FieldsFunc(list, func(r rune) bool { return r == '+' || r == ';' }) {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		it := a.commandByTitle(t)
		if it == nil {
			return errString("Böyle bir komut yok: “" + t + "”")
		}
		cmds = append(cmds, it.title)
	}
	if len(cmds) == 0 {
		return errString("Makro en az bir komut ister")
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
				a.notice = Tf("“%s” komutu bulunamadı: %s", cmd, norm)
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
			settingRow(c, k, a.settings.Keys[k], func() {
				if ryugo.Button(c.Key("key-del:"+k), T("Kaldır"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.unbindKey(k)
				}
			})
		})
	}
	if len(rows) > 0 {
		ui.Text(c, T("Kendi kısayolların")).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		settingGroup(c, "set-keys", rows...)
	}
	settingGroup(c, "set-key-new", func() {
		settingRow(c, "Komuta kısayol ver", "Komutun adını ve tuşu yaz: örn. “Şimdi yedekle” ve cmd+shift+y. Uygulamanın ve sistemin tuşları verilmez.", func() {
			ui.Column(c).Gap(6).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("key-cmd"), &a.keyCmd, ryugo.InputOptions{Label: "Komut", Size: ryugo.Sm, Placeholder: T("Komut adı")}), &a.keyCmd, fieldOpts{oneLine: true})
				a.fieldMenu(ryugo.Input(c.Key("key-accel"), &a.keyAccel, ryugo.InputOptions{Label: T("Tuş"), Size: ryugo.Sm, Placeholder: "cmd+shift+y"}), &a.keyAccel, fieldOpts{oneLine: true})
				if ryugo.Button(c.Key("key-set"), T("Ata"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
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
			settingRow(c, n, strings.Join(a.settings.Macros[n], " + "), func() {
				ui.Row(c).Gap(6).Children(func() {
					if ryugo.Button(c.Key("macro-run:"+n), T("Çalıştır"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.runMacro(n)
					}
					if ryugo.Button(c.Key("macro-del:"+n), T("Sil"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						delete(a.settings.Macros, n)
						a.saveSettings()
					}
				})
			})
		})
	}
	if len(mrows) > 0 {
		ui.Text(c, T("Makrolar")).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		settingGroup(c, "set-macros", mrows...)
	}
	settingGroup(c, "set-macro-new", func() {
		settingRow(c, "Makro yaz", "Bir ad ve + ile ayrılmış komutlar: örn. “Günlük görev” için “Bugünün sayfası + Görevler”. ⌘P'de “Makro: ad” olarak çıkar.", func() {
			ui.Column(c).Gap(6).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("macro-name"), &a.macroName, ryugo.InputOptions{Label: "Makro adı", Size: ryugo.Sm, Placeholder: T("Makro adı")}), &a.macroName, fieldOpts{oneLine: true})
				a.fieldMenu(ryugo.Input(c.Key("macro-list"), &a.macroList, ryugo.InputOptions{Label: "Komutlar", Size: ryugo.Sm, Placeholder: T("Bugünün sayfası + Görevler")}), &a.macroList, fieldOpts{oneLine: true})
				if ryugo.Button(c.Key("macro-save"), T("Kaydet"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
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
