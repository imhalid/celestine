package main

import (
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// The settings as one list, which the palette reads: every setting of the settings screen is a command,
// so that nothing needs the mouse. A toggle is turned over, a number is stepped or typed ("yazı boyutu 18"),
// a choice is each of its values. A test checks that every field of Settings is in the list or has a
// reason not to be (settingsApart).

type settingKind int

const (
	setToggle settingKind = iota
	setNumber
	setChoice
)

type settingChoice struct {
	name    string
	current bool
	set     func(a *app)
	preview func(a *app) // shows it without keeping it, while it is the one chosen
}

type settingDef struct {
	name, alias string
	fields      []string // the keys of Settings it covers (json names)
	kind        settingKind

	on   func(a *app) bool // toggle
	flip func(a *app, v bool)

	val           func(a *app) float64 // number
	put           func(a *app, v float64)
	min, max, def float64
	step          float64
	unit          string
	dec           int

	choices func(a *app) []settingChoice // choice
}

// settingsApart are the fields of Settings that are not settings to change by a command, and why.
var settingsApart = map[string]string{
	"sidebarWidth":   "dragged at the sidebar's edge",
	"sidebarName":    "a name typed in the settings, not turned on or off",
	"updateChannel":  "trial versions are the maintainer's, set in the settings file: not shown (#407)",
	"panelWidth":     "dragged at the side panel's edge",
	"port":           "the MCP server's port: typed in its own settings",
	"token":          "the MCP server's secret",
	"mcpAccess":      "what agents may do with each collection: chosen per collection in the MCP settings",
	"recent":         "what the / menu was used for",
	"recentCmds":     "the commands used last",
	"pendingRenames": "a rename being typed, ended at the next start (relink.go)",
	"writing":        "the words written, by day",
	"writeGoal":      "the goal: set on the statistics page",
	"quickKey":       "the quick note's key: pressed in its own settings",
	"fontFavs":       "the families starred: set in the list of typefaces",
	"fontLast":       "the families used last",
	"syncDir":        "the folder: typed in the sync settings",
	"syncUrl":        "the server's address: typed in the sync settings",
	"syncToken":      "the device's secret",
	"syncKey":        "the encryption key: made from a passphrase in the sync settings",
	"accountUser":    "the account signed in to: in the sync settings",
	"accountAsked":   "whether the sign-in dialog has been answered: it is shown once (profile.go)",
	"syncOff":        "the collections left out: switches in the sync settings",
	"profiles":       "profiles: saved on the settings' profiles page",
	"keys":           "keys: given on the shortcuts page of the settings",
	"macros":         "macros: written on the shortcuts page of the settings",
	"updateUrl":      "the manifest's address: shown in the update settings",
	"updateAt":       "when it last asked",
	"updateSnooze":   "remind me in three days",
	"updateSkip":     "the version said no to: set in the dialog",
	"updateNotes":    "the notes of the version installed",
	"updateFor":      "and its number",
	"updateSeen":     "the notes shown",
	"welcomeDone":    "the first opening was seen",
	"treeSaved":      "where the sidebar's tree was left",
	"treeOpen":       "where the sidebar's tree was left",
	"panes":          "the panes as they were left",
	"glass":          "macOS only: Liquid Glass requires macOS NSGlassEffectView (#361)",
}

func (a *app) save1() { a.saveSettings() }

// settingDefs is the list of settings. The settings screen draws its own rows; this list is what the
// palette and the argument commands go by, and the test holds the two together.
func settingDefs() []settingDef { return settingDefsFor(runtime.GOOS == "darwin") }

// settingLimits are the bounds of the number setting that covers field, as the palette's command has them: the
// settings screen's slider takes them too, so a value set by the command is never past the slider's end (#224).
func settingLimits(field string) (lo, hi float64) {
	for _, d := range settingDefs() {
		if d.kind == setNumber && len(d.fields) == 1 && d.fields[0] == field {
			return d.min, d.max
		}
	}
	panic("no number setting covers " + field)
}

// settingDefsFor returns the command-palette setting definitions for the given platform capability.
func settingDefsFor(macOS bool) []settingDef {
	tog := func(name, alias string, fields []string, on func(*app) bool, flip func(*app, bool)) settingDef {
		return settingDef{name: name, alias: alias, fields: fields, kind: setToggle, on: on, flip: flip}
	}
	// simple is a toggle of a bool field, which may mean the reverse of its name
	simple := func(name, alias, field string, get func(*Settings) *bool, invert bool) settingDef {
		return tog(name, alias, []string{field},
			func(a *app) bool { return *get(&a.settings) != invert },
			func(a *app, v bool) { *get(&a.settings) = v != invert; a.saveSettings() })
	}
	num := func(name, alias, field string, min, max, def, step float64, unit string, dec int, val func(*app) float64, put func(*app, float64)) settingDef {
		return settingDef{name: name, alias: alias, fields: []string{field}, kind: setNumber, val: val, put: put, min: min, max: max, def: def, step: step, unit: unit, dec: dec}
	}
	defs := []settingDef{
		{name: "common_theme", alias: "theme görünüm açık koyu sistem", fields: []string{"theme"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for i, n := range []string{"common_system", "common_light", "common_dark"} {
				i := i
				out = append(out, settingChoice{name: n, current: a.settings.Theme%3 == i,
					set:     func(a *app) { a.settings.Theme = i; a.saveSettings() },
					preview: func(a *app) { a.settings.Theme = i }})
			}
			return out
		}},
		{name: "common_colorTheme", alias: "accent renk", fields: []string{"appTheme"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for i, t := range appThemes {
				i := i
				out = append(out, settingChoice{name: t.name, current: a.settings.AppTheme%len(appThemes) == i,
					set:     func(a *app) { a.setAppTheme(i) },
					preview: func(a *app) { a.settings.AppTheme = i }})
			}
			return out
		}},
		{name: "caret", alias: "imleç", fields: []string{"caret", "caretName"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for k := caretKind(0); k < caretCount(); k++ {
				k := k
				out = append(out, settingChoice{name: caretName(k), current: a.caretKind == k,
					set:     func(a *app) { a.setCaret(k) },
					preview: func(a *app) { a.caretKind = k }})
			}
			return out
		}},
		{name: "common_cursorFx", alias: "mouse pointer fare imleç efekt effect trail", fields: []string{"cursorFx"}, kind: setChoice, choices: func(a *app) []settingChoice {
			out := []settingChoice{{name: locales.CommonOff(), current: a.settings.CursorFX == "",
				set:     func(a *app) { a.settings.CursorFX = ""; a.saveSettings() },
				preview: func(a *app) { a.settings.CursorFX = "" }}}
			for _, p := range cursorPresets {
				name := p.name
				out = append(out, settingChoice{name: msg(name), current: a.settings.CursorFX == name,
					set:     func(a *app) { a.settings.CursorFX = name; a.saveSettings() },
					preview: func(a *app) { a.settings.CursorFX = name }})
			}
			return out
		}},
		{name: "letterSpacing", alias: "letter spacing tracking harf aralığı", fields: []string{"letterSpacing"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for i, s := range letterSpacings {
				i := i
				out = append(out, settingChoice{name: msg(s.name), current: a.settings.LetterSpacing == i,
					set: func(a *app) { a.settings.LetterSpacing = i; a.saveSettings() }})
			}
			return out
		}},
		{name: "caretBlink", alias: "imleç yanıp sönme blink cursor", fields: []string{"caretBlink"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, o := range []struct {
				name string
				ms   int
			}{{"Off", 0}, {"Slow", 1000}, {"Normal", 530}, {"Fast", 350}} {
				o := o
				out = append(out, settingChoice{name: msg(o.name), current: a.settings.CaretBlink == o.ms,
					set: func(a *app) { a.settings.CaretBlink = o.ms; a.saveSettings() }})
			}
			return out
		}},
		{name: "common_corners", alias: "köşe yuvarlaklığı radius", fields: []string{"corner"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, p := range []int{0, 50, 100, 150, 200} {
				p := p
				out = append(out, settingChoice{name: strconv.Itoa(p) + "%", current: a.cornerPercent() == p,
					set: func(a *app) { v := p; a.settings.Corner = &v; a.saveSettings() }})
			}
			return out
		}},
		{name: "theWelcomesGround", alias: "welcome ground background onboarding karşılama", fields: []string{"welcomeGround"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, g := range welcomeGrounds {
				id := g.id
				out = append(out, settingChoice{name: msg(g.name), current: a.settings.WelcomeGround == id, set: func(a *app) { a.settings.WelcomeGround = id; a.saveSettings() }})
			}
			return out
		}},
		{name: "opening", alias: "welcome opening onboarding karşılama açılış", fields: []string{"welcomeOpen"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, o := range welcomeOpenings {
				id := o.id
				out = append(out, settingChoice{name: msg(o.name), current: a.settings.WelcomeOpen == id, set: func(a *app) { a.settings.WelcomeOpen = id; a.saveSettings() }})
			}
			return out
		}},
		{name: "common_language", alias: "language english türkçe", fields: []string{"lang"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, l := range []string{"en", "tr"} {
				l := l
				out = append(out, settingChoice{name: langName(l), current: a.settings.Lang == l || (l == "en" && a.settings.Lang != "tr"), set: func(a *app) { a.settings.Lang = l; setLang(l); a.saveSettings() }})
			}
			return out
		}},
		fontChoice("appTypeface", "app font arayüz", "fontApp", fontApp),
		fontChoice("editorTypeface", "editor font metin yazı tipi", "fontEditor", fontEditor),
		fontChoice("codeTypeface", "code monospace font", "fontMono", fontMono),
		{name: "common_focusMode", alias: "focus line paragraph satır paragraf", fields: []string{"focus"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, m := range []struct{ id, name string }{{"", "off2"}, {"line", "settings_focusLine"}, {"paragraph", "settings_focusParagraph"}} {
				m := m
				out = append(out, settingChoice{name: m.name, current: a.focusMode() == m.id, set: func(a *app) { a.settings.Focus = m.id; a.saveSettings() }})
			}
			return out
		}},
		simple("common_typewriterScrolling", "typewriter scroll", "typewriter", func(s *Settings) *bool { return &s.Typewriter }, false),
		simple("common_alwaysShowFocusRing", "focus ring keyboard halka", "alwaysRing", func(s *Settings) *bool { return &s.AlwaysRing }, false),
		simple("common_lightMode", "light mode caret efekt kapat", "noCaretFx", func(s *Settings) *bool { return &s.NoCaretFX }, false),
		simple("settings_reduceMotion", "reduce motion animation still hareket animasyon azalt", "reduceMotion", func(s *Settings) *bool { return &s.ReduceMotion }, false),
		simple("settings_shaderEffects", "shader effects gpu glass light efekt cam ışık", "noShaders", func(s *Settings) *bool { return &s.NoShaders }, true),
		num("common_lineLength", "line length column hard wrap sütun karakter", "lineChars", 0, 120, 0, 5, " karakter", 0,
			func(a *app) float64 { return float64(a.settings.LineChars) }, func(a *app, v float64) { a.settings.LineChars = int(v); a.saveSettingsSoon() }),
		num("common_tabWidth", "tab width space", "tabSpaces", 0, 8, 0, 1, "common_spaces", 0,
			func(a *app) float64 { return float64(a.settings.TabSpaces) }, func(a *app, v float64) { a.settings.TabSpaces = int(v); a.saveSettingsSoon() }),
		num("common_caretSleep", "caret sleep", "sleepSecs", 1, 60, 5, 1, "common_s", 0,
			func(a *app) float64 { return a.caretSleep().Seconds() }, func(a *app, v float64) { a.settings.SleepSecs = v; a.saveSettingsSoon() }),
		simple("common_vimMotions", "vim hjkl normal kip", "vimKeys", func(s *Settings) *bool { return &s.VimKeys }, false),
		simple("settings_escNormal", "esc escape normal kip keyboard klavye", "escNormal", func(s *Settings) *bool { return &s.EscNormal }, false),
		simple("settings_linkPreviews", "link bağlantı preview önizleme favicon og", "noLinkPreviews", func(s *Settings) *bool { return &s.NoLinkPreviews }, true),
		simple("settings_linkIcons", "link bağlantı favicon icon simge site", "noLinkIcons", func(s *Settings) *bool { return &s.NoLinkIcons }, true),
		simple("checkForUpdates", "update güncelleme", "updateOff", func(s *Settings) *bool { return &s.UpdateOff }, true),
		simple("quietUpdateMode", "update quiet", "updateQuiet", func(s *Settings) *bool { return &s.UpdateQuiet }, false),
		simple("updateAutomatically", "update auto kur", "updateAuto", func(s *Settings) *bool { return &s.UpdateAuto }, false),
		num("updateCheckInterval", "update hours saat", "updateHours", 1, 720, 24, 6, " sa", 0,
			func(a *app) float64 { return a.updateEvery().Hours() },
			func(a *app, v float64) { a.settings.UpdateHours = int(v); a.saveSettingsSoon() }),
		{name: "sync4", alias: "sync eşitle klasör sunucu", fields: []string{"syncMode"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, m := range []struct{ id, name string }{{"", "off2"}, {"dir", "folder2"}, {"server", "server2"}} {
				m := m
				out = append(out, settingChoice{name: m.name, current: a.settings.SyncMode == m.id, set: func(a *app) { a.settings.SyncMode = m.id; a.sy.tried = time.Time{}; a.saveSettings() }})
			}
			return out
		}},
		num("syncInterval", "sync minutes dakika", "syncMinutes", 1, 1440, 5, 5, " dk", 0,
			func(a *app) float64 { return a.syncEvery().Minutes() },
			func(a *app, v float64) { a.settings.SyncMinutes = int(v); a.saveSettingsSoon() }),
		simple("common_collectStatistics", "stats analiz kullanım", "noStats", func(s *Settings) *bool { return &s.NoStats }, true),
		simple("common_dailyBackup", "backup yedekle", "noBackup", func(s *Settings) *bool { return &s.NoBackup }, true),
		{name: "backup_every", alias: "backup daily weekly monthly yedek günlük haftalık aylık", fields: []string{"backupEvery"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, e := range []struct{ v, name string }{{"", "backup_everyDay"}, {"week", "backup_everyWeek"}, {"month", "backup_everyMonth"}} {
				out = append(out, settingChoice{name: e.name, current: a.settings.BackupEvery == e.v, set: func(a *app) { a.settings.BackupEvery = e.v; a.saveSettings() }})
			}
			return out
		}},
		simple("common_ligatures", "ligature fi fl", "noLigatures", func(s *Settings) *bool { return &s.NoLigatures }, true),
		simple("common_tabularNumbers", "tabular numbers tnum", "tabularNums", func(s *Settings) *bool { return &s.TabularNums }, false),
		num("view_zoom", "zoom büyüt yakınlaştır scale ölçek", "zoom", 50, 300, 100, 10, "%", 0,
			func(a *app) float64 { return float64(a.windowZoom() * 100) },
			func(a *app, v float64) { a.setZoom(float32(v) / 100) }),
		num("common_fontWeight", "weight bold kalın", "fontWeight", 300, 700, 400, 100, "", 0,
			func(a *app) float64 { return float64(a.fontWeight()) },
			func(a *app, v float64) { a.settings.FontWeight = int(v); a.saveSettingsSoon() }),
		tog("common_jellyEffect", "caret jöle jelly", []string{"jelly"},
			func(a *app) bool { return a.settings.Jelly },
			func(a *app, v bool) { a.settings.Jelly = v; a.saveSettings() }),
		simple("common_shimmer", "parıltı ışık jöle sarsıntısı wobble shake shimmer", "noWobble", func(s *Settings) *bool { return &s.NoWobble }, true),
		simple("common_sounds", "sound audio chime bell mute ses bildirim", "noSound", func(s *Settings) *bool { return &s.NoSound }, true),
		tog("notify_name", "notification notifications bildirim bildirimler alert", []string{"notify"},
			func(a *app) bool { return a.settings.Notify == notifyOn },
			func(a *app, v bool) { a.setNotify(v) }),
		simple("sounds_startup", "sound chord startup opening open akor açılış", "startChord", func(s *Settings) *bool { return &s.StartChord }, false),
		simple("common_pressWaves", "button ripple wave press düğme dalga", "noWaves", func(s *Settings) *bool { return &s.NoWaves }, true),
		simple("common_automaticCover", "kapak cover", "autoCover", func(s *Settings) *bool { return &s.AutoCover }, false),
		simple("common_colorWordsByDay", "time-lapse timelapse renk color sürüm version", "noLapse", func(s *Settings) *bool { return &s.NoLapse }, true),
		simple("common_autosave", "autosave kaydet save", "noAutoSave", func(s *Settings) *bool { return &s.NoAutoSave }, true),
		simple("showSpaces", "whitespace boşluk space", "showSpaces", func(s *Settings) *bool { return &s.ShowSpaces }, false),
		simple("showTabs", "tab whitespace", "showTabs", func(s *Settings) *bool { return &s.ShowTabs }, false),
		simple("showParagraphEnds", "pilcrow satır sonu eol", "showBreaks", func(s *Settings) *bool { return &s.ShowBreaks }, false),
		simple("whiteSpaceOnlyInTheSelection", "whitespace selection boşluk seçim", "whiteInSel", func(s *Settings) *bool { return &s.WhiteInSel }, false),
		simple("showWhitespaceInCodeBlocks", "kod code whitespace", "markCode", func(s *Settings) *bool { return &s.MarkCode }, false),
		simple("shortcutHints", "keys shortcuts hints status kısayol ipucu", "hideHints", func(s *Settings) *bool { return &s.HideHints }, true),
		simple("settings_sidebarCard", "kenar çubuğu sidebar zemin arka plan background kart card", "sidebarCard", func(s *Settings) *bool { return &s.SidebarCard }, false),
		simple("hideTheStatusBar", "odak modu focus hide statusbar", "hideStatus", func(s *Settings) *bool { return &s.HideStatus }, false),
		simple("saveStatusInTheStatusBar", "kaydedildi status", "hideSaved", func(s *Settings) *bool { return &s.HideSaved }, true),
		simple("letterCountInTheStatusBar", "karakter chars", "showChars", func(s *Settings) *bool { return &s.ShowChars }, false),
		simple("lineAndColumnInTheStatus", "line column konum", "showPos", func(s *Settings) *bool { return &s.ShowPos }, false),
		tog("sidebar", "sidebar", []string{"sidebarClosed"},
			func(a *app) bool { return !a.settings.SidebarClosed },
			func(a *app, v bool) {
				if v == a.settings.SidebarClosed {
					a.toggleSidebar()
				}
			}),
		tog("common_sidePanel", "panel anahat outline", []string{"panelOpen"},
			func(a *app) bool { return a.settings.PanelOpen },
			func(a *app, v bool) {
				if v != a.settings.PanelOpen {
					a.togglePanel()
				}
			}),
		tog("common_mcpServer", "mcp ajan agent", []string{"mcp"},
			func(a *app) bool { return a.settings.MCP || a.mcpServer != nil },
			func(a *app, v bool) { a.setMCP(v) }),
		num("common_fontSize", "font size punto", "fontSize", minFont, maxFont, 16.5, 0.5, " px", 1,
			func(a *app) float64 { return float64(a.fontSize()) }, func(a *app, v float64) { a.setFontSize(v) }),
		num("common_lineSpacing", "line height", "lineHeight", 1.2, 2.2, 1.55, 0.05, "", 2,
			func(a *app) float64 { return float64(a.lineHeight()) },
			func(a *app, v float64) { a.settings.LineHeight = v; a.saveSettingsSoon() }),
		num("common_pageWidth", "genişlik width sütun", "pageWidth", 480, 1200, float64(columnMax), 20, " px", 0,
			func(a *app) float64 { return float64(a.pageWidth()) },
			func(a *app, v float64) { a.settings.PageWidth = v; a.saveSettingsSoon() }),
		num("common_paragraphSpacing", "paragraph spacing boşluk", "paraGap", 0, 2, defaultParaGap, 0.05, " em", 2,
			func(a *app) float64 { return float64(a.paraGap()) },
			func(a *app, v float64) { a.settings.ParaGap = &v; a.saveSettingsSoon() }),
		num("common_firstLineIndent", "indent girinti", "indent", 0, 4, 0, 0.25, " em", 2,
			func(a *app) float64 { return float64(a.firstIndent()) },
			func(a *app, v float64) { a.settings.Indent = v; a.saveSettingsSoon() }),
		num("common_saveDelay", "autosave delay bekleme", "saveAfter", 0.2, 30, defaultSaveAt, 0.5, "common_s", 1,
			func(a *app) float64 { return a.saveAfter().Seconds() },
			func(a *app, v float64) { a.settings.SaveAfter = v; a.saveSettingsSoon() }),
		num("common_cornerRoundness", "corner radius", "corner", 0, 200, 100, 10, "%", 0,
			func(a *app) float64 { return float64(a.cornerPercent()) },
			func(a *app, v float64) { p := int(v); a.settings.Corner = &p; a.saveSettingsSoon() }),
	}
	if macOS {
		defs = append(defs, tog("settings_glass", "liquid glass vibrancy cam saydam macos", []string{"glass"},
			func(a *app) bool { return a.settings.Glass },
			func(a *app, v bool) { a.setGlass(v) }))
	}
	return defs
}

func (d settingDef) fmt(v float64) string {
	s := strconv.FormatFloat(v, 'f', d.dec, 64)
	switch strings.TrimSpace(d.unit) { // the units in words, in the window's language
	case "karakter":
		return locales.SettingsUnitChars(s, int(v))
	case "boşluk":
		return locales.SettingsUnitSpaces(s, int(v))
	case "sn":
		return locales.DatesSeconds(s)
	case "sa":
		return locales.SettingsUnitHours(s)
	case "dk":
		return locales.SettingsUnitMinutes(s)
	}
	return s + d.unit
}

func (d settingDef) clamp(v float64) float64 { return min(max(v, d.min), d.max) }

// undoable runs a change of a setting and offers to take it back: the notice has a Geri al.
func (a *app) undoable(what string, back func(), do func()) func() {
	return func() {
		do()
		a.say(toastInfo, what)
		a.noticeUndo = func() { back(); a.saveSettings() }
	}
}

// settingCmds are the settings as commands, for the palette.
func (a *app) settingCmds(cmd func(title, keys string, glyph *ui.SVG, run func()) palItem) []palItem {
	var out []palItem
	for _, d := range settingDefs() {
		d := d
		add := func(it palItem) {
			it.alias = d.alias + " ayar"
			out = append(out, it)
		}
		switch d.kind {
		case setToggle:
			on := d.on(a)
			state := map[bool]string{true: locales.SettingsOn(), false: locales.SettingsOff()}[on]
			it := cmd(d.name, state, iconSettings, a.undoable(
				locales.SettingsChanged(msg(d.name), map[bool]string{true: locales.SettingsOff(), false: locales.SettingsOn()}[on]), func() { d.flip(a, on) }, func() { d.flip(a, !on) }))
			it.title = msg(d.name) + ": " + map[bool]string{true: locales.SettingsTurnOff(), false: locales.SettingsTurnOn()}[on]
			it.id = d.name + ": " + map[bool]string{true: "off3", false: "on"}[on]
			it.sub, it.kind = locales.PaletteSetting(), palSetting
			add(it)
		case setNumber:
			cur := d.val(a)
			steps := []struct {
				label string
				to    float64
				ok    bool
			}{
				{"increase", d.clamp(cur + d.step), cur+d.step <= d.max+1e-9},
				{"decrease", d.clamp(cur - d.step), cur-d.step >= d.min-1e-9},
				{"default2", d.def, cur != d.def},
			}
			for _, s := range steps {
				s := s
				it := cmd(d.name+": "+s.label, d.fmt(cur), iconSettings, a.undoable(locales.SettingsChanged(msg(d.name), d.fmt(s.to)), func() { d.put(a, cur) }, func() { d.put(a, s.to) }))
				it.title = msg(d.name) + ": " + msg(s.label)
				it.sub, it.kind = locales.PaletteSetting(), palSetting
				if !s.ok {
					switch s.label {
					case "increase":
						it.off = locales.SettingsAlreadyHighest(msg(d.name), d.fmt(cur))
					case "decrease":
						it.off = locales.SettingsAlreadyLowest(msg(d.name), d.fmt(cur))
					default:
						it.off = locales.SettingsAlreadyDefault(msg(d.name), d.fmt(cur))
					}
				}
				add(it)
			}
		case setChoice:
			for _, ch := range d.choices(a) {
				ch := ch
				cur := d.choices(a)
				var was func()
				for _, c := range cur {
					if c.current {
						c := c
						was = func() { c.set(a) }
					}
				}
				it := cmd(d.name+": "+ch.name, map[bool]string{true: "✓", false: ""}[ch.current], iconSettings, a.undoable(locales.SettingsChanged(msg(d.name), msg(ch.name)), func() {
					if was != nil {
						was()
					}
				}, func() { ch.set(a) }))
				it.title = msg(d.name) + ": " + msg(ch.name)
				it.sub, it.kind = locales.PaletteSetting(), palSetting
				if ch.preview != nil {
					pv := ch.preview
					it.preview = func() { pv(a) }
				}
				add(it)
			}
		}
	}
	return out
}

var argCmd = regexp.MustCompile(`^(.*\S)\s+(-?\d+(?:[.,]\d+)?)\s*(?:px|em|sn|s|%)?$`)

// argItems are the commands typed with their value: "yazı boyutu 18" sets it, with nothing more to do.
func (a *app) argItems(q string) []palItem {
	m := argCmd.FindStringSubmatch(strings.TrimSpace(q))
	if m == nil {
		return nil
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", "."), 64)
	if err != nil {
		return nil
	}
	var out []palItem
	for _, d := range settingDefs() {
		d := d
		if d.kind != setNumber {
			continue
		}
		s := match(m[1], names(d.name)+" "+d.alias)
		if s < 0 || match(m[1], names(d.name)) < 0 && match(m[1], d.alias) < 0 {
			continue
		}
		to, cur := d.clamp(v), d.val(a)
		it := palItem{title: locales.SettingsSetTo(msg(d.name), d.fmt(to)), sub: locales.PaletteSetting(), kind: palSetting, id: d.name, glyph: iconSettings, score: 1000 + s,
			run: a.undoable(locales.SettingsChanged(msg(d.name), d.fmt(to)), func() { d.put(a, cur) }, func() { d.put(a, to) })}
		if to != v {
			it.title = locales.SettingsSetToClamped(msg(d.name), d.fmt(to), d.fmt(d.min), d.fmt(d.max))
		}
		out = append(out, it)
	}
	return out
}

// fontChoice is a typeface setting as a choice: the default, the starred and those used last; any other is
// found in the settings' list, or by its name typed after the command ("editör yazı tipi Georgia").
func fontChoice(name, alias, field string, role fontRole) settingDef {
	return settingDef{name: name, alias: alias, fields: []string{field}, kind: setChoice, choices: func(a *app) []settingChoice {
		cur := *a.settings.fontField(role)
		out := []settingChoice{{name: "default2", current: cur == "",
			set: func(a *app) { a.setFont(role, "") }}}
		seen := map[string]bool{}
		for _, list := range [][]string{a.settings.FontFavs, a.settings.FontLast, {cur}} {
			for _, n := range list {
				n := n
				if n == "" || seen[n] || !a.hasFont(n) {
					continue
				}
				seen[n] = true
				out = append(out, settingChoice{name: n, current: cur == n,
					set: func(a *app) { a.setFont(role, n) }})
			}
		}
		return out
	}}
}

// fontNameItems are "editör yazı tipi Georgia": a command with the family typed after it.
func (a *app) fontNameItems(q string) []palItem {
	var out []palItem
	for _, d := range settingDefs() {
		d := d
		if d.kind != setChoice || len(d.fields) != 1 || !strings.HasPrefix(d.fields[0], "font") || d.fields[0] == "fontEditor" && false {
			continue
		}
		var role fontRole
		switch d.fields[0] {
		case "fontApp":
			role = fontApp
		case "fontEditor":
			role = fontEditor
		case "fontMono":
			role = fontMono
		default:
			continue
		}
		low := fold(q)
		pre := fold(d.name) // the name in Turkish, or in the window's language
		if !strings.HasPrefix(low, pre+" ") {
			if pre = fold(msg(d.name)); !strings.HasPrefix(low, pre+" ") {
				continue
			}
		}
		want := strings.TrimSpace(low[len(pre)+1:])
		if want == "" {
			continue
		}
		for _, f := range a.families() {
			if match(want, f.Name) >= 0 {
				f := f
				was := *a.settings.fontField(role)
				out = append(out, palItem{title: locales.SettingsSetTo(msg(d.name), f.Name), sub: locales.PaletteSetting(), kind: palSetting, id: d.name, glyph: iconSettings, score: 900 + match(want, f.Name),
					run: a.undoable(locales.SettingsChanged(msg(d.name), f.Name), func() { a.setFont(role, was) }, func() { a.setFont(role, f.Name) })})
				if len(out) >= 8 {
					return out
				}
			}
		}
	}
	return out
}
