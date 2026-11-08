package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
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
	"sidebarWidth": "dragged at the sidebar's edge",
	"panelWidth":   "dragged at the side panel's edge",
	"port":         "the MCP server's port: typed in its own settings",
	"token":        "the MCP server's secret",
	"recent":       "what the / menu was used for",
	"recentCmds":   "the commands used last",
	"writing":      "the words written, by day",
	"writeGoal":    "the goal: set on the statistics page",
	"quickKey":     "the quick note's key: pressed in its own settings",
	"fontFavs":     "the families starred: set in the list of typefaces",
	"fontLast":     "the families used last",
	"syncDir":      "the folder: typed in the sync settings",
	"syncUrl":      "the server's address: typed in the sync settings",
	"syncToken":    "the device's secret",
	"syncKey":      "the encryption key: made from a passphrase in the sync settings",
	"accountUser":  "the account signed in to: in the sync settings",
	"accountAsked": "whether the sign-in dialog has been answered: it is shown once (profile.go)",
	"syncOff":      "the collections left out: switches in the sync settings",
	"profiles":     "profiles: saved on the settings' profiles page",
	"keys":         "keys: given on the shortcuts page of the settings",
	"macros":       "macros: written on the shortcuts page of the settings",
	"updateUrl":    "the manifest's address: shown in the update settings",
	"updateAt":     "when it last asked",
	"updateSnooze": "remind me in three days",
	"updateSkip":   "the version said no to: set in the dialog",
	"updateNotes":  "the notes of the version installed",
	"updateFor":    "and its number",
	"updateSeen":   "the notes shown",
	"tourDone":     "the first-run tour was shown",
	"welcomeDone":  "the first opening was seen",
	"treeSaved":    "where the sidebar's tree was left",
	"treeOpen":     "where the sidebar's tree was left",
	"panes":        "the panes as they were left",
}

func (a *app) save1() { a.saveSettings() }

// settingDefs is the list of settings. The settings screen draws its own rows; this list is what the
// palette and the argument commands go by, and the test holds the two together.
func settingDefs() []settingDef {
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
	return []settingDef{
		{name: "Tema", alias: "theme görünüm açık koyu sistem", fields: []string{"theme"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for i, n := range []string{"Sistem", "Açık", "Koyu"} {
				i := i
				out = append(out, settingChoice{name: n, current: a.settings.Theme%3 == i,
					set:     func(a *app) { a.settings.Theme = i; a.saveSettings() },
					preview: func(a *app) { a.settings.Theme = i }})
			}
			return out
		}},
		{name: "Renk teması", alias: "accent renk", fields: []string{"appTheme"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for i, t := range appThemes {
				i := i
				out = append(out, settingChoice{name: t.name, current: a.settings.AppTheme%len(appThemes) == i,
					set:     func(a *app) { a.setAppTheme(i) },
					preview: func(a *app) { a.settings.AppTheme = i }})
			}
			return out
		}},
		{name: "Caret", alias: "imleç", fields: []string{"caret", "caretName"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for k := caretKind(0); k < caretCount(); k++ {
				k := k
				out = append(out, settingChoice{name: caretName(k), current: a.caretKind == k,
					set:     func(a *app) { a.setCaret(k) },
					preview: func(a *app) { a.caretKind = k }})
			}
			return out
		}},
		{name: "Köşeler", alias: "köşe yuvarlaklığı radius", fields: []string{"corner"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, p := range []int{0, 50, 100, 150, 200} {
				p := p
				out = append(out, settingChoice{name: strconv.Itoa(p) + "%", current: a.cornerPercent() == p,
					set: func(a *app) { v := p; a.settings.Corner = &v; a.saveSettings() }})
			}
			return out
		}},
		{name: "Dil", alias: "language english türkçe", fields: []string{"lang"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, l := range []string{"", "tr", "en"} {
				l := l
				out = append(out, settingChoice{name: langName(l), current: a.settings.Lang == l, set: func(a *app) { a.settings.Lang = l; setLang(l); a.saveSettings() }})
			}
			return out
		}},
		fontChoice("Uygulama yazı tipi", "app font arayüz", "fontApp", fontApp),
		fontChoice("Editör yazı tipi", "editor font metin yazı tipi", "fontEditor", fontEditor),
		fontChoice("Kod yazı tipi", "code monospace font", "fontMono", fontMono),
		{name: "Odak modu", alias: "focus line paragraph satır paragraf", fields: []string{"focus"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, m := range []struct{ id, name string }{{"", "kapalı"}, {"line", "satır"}, {"paragraph", "paragraf"}} {
				m := m
				out = append(out, settingChoice{name: m.name, current: a.focusMode() == m.id, set: func(a *app) { a.settings.Focus = m.id; a.saveSettings() }})
			}
			return out
		}},
		simple("Daktilo kaydırması", "typewriter scroll", "typewriter", func(s *Settings) *bool { return &s.Typewriter }, false),
		simple("Odak halkasını her zaman göster", "focus ring keyboard halka", "alwaysRing", func(s *Settings) *bool { return &s.AlwaysRing }, false),
		simple("Hafif mod", "light mode caret efekt kapat", "noCaretFx", func(s *Settings) *bool { return &s.NoCaretFX }, false),
		num("Satır uzunluğu", "line length column hard wrap sütun karakter", "lineChars", 0, 120, 0, 5, " karakter", 0,
			func(a *app) float64 { return float64(a.settings.LineChars) }, func(a *app, v float64) { a.settings.LineChars = int(v); a.saveSettingsSoon() }),
		num("Tab uzunluğu", "tab width space", "tabSpaces", 0, 8, 0, 1, " boşluk", 0,
			func(a *app) float64 { return float64(a.settings.TabSpaces) }, func(a *app, v float64) { a.settings.TabSpaces = int(v); a.saveSettingsSoon() }),
		num("Caret uykusu", "caret sleep", "sleepSecs", 1, 60, 5, 1, " sn", 0,
			func(a *app) float64 { return a.caretSleep().Seconds() }, func(a *app, v float64) { a.settings.SleepSecs = v; a.saveSettingsSoon() }),
		simple("Vim hareketleri", "vim hjkl normal kip", "vimKeys", func(s *Settings) *bool { return &s.VimKeys }, false),
		simple("Yeni sürümü denetle", "update güncelleme", "updateOff", func(s *Settings) *bool { return &s.UpdateOff }, true),
		simple("Güncelleme sessiz mod", "update quiet", "updateQuiet", func(s *Settings) *bool { return &s.UpdateQuiet }, false),
		simple("Kendiliğinden güncelle", "update auto kur", "updateAuto", func(s *Settings) *bool { return &s.UpdateAuto }, false),
		{name: "Güncelleme kanalı", alias: "update channel beta deneme", fields: []string{"updateChannel"}, kind: setChoice, choices: func(a *app) []settingChoice {
			return []settingChoice{
				{name: "Kararlı", current: a.settings.UpdateChannel != "beta", set: func(a *app) { a.settings.UpdateChannel = ""; a.saveSettings() }},
				{name: "Deneme (beta)", current: a.settings.UpdateChannel == "beta", set: func(a *app) { a.settings.UpdateChannel = "beta"; a.saveSettings() }},
			}
		}},
		num("Güncelleme denetim sıklığı", "update hours saat", "updateHours", 1, 720, 24, 6, " sa", 0,
			func(a *app) float64 { return a.updateEvery().Hours() },
			func(a *app, v float64) { a.settings.UpdateHours = int(v); a.saveSettingsSoon() }),
		{name: "Senkron", alias: "sync eşitle klasör sunucu", fields: []string{"syncMode"}, kind: setChoice, choices: func(a *app) []settingChoice {
			var out []settingChoice
			for _, m := range []struct{ id, name string }{{"", "kapalı"}, {"dir", "klasör"}, {"server", "sunucu"}} {
				m := m
				out = append(out, settingChoice{name: m.name, current: a.settings.SyncMode == m.id, set: func(a *app) { a.settings.SyncMode = m.id; a.sy.tried = time.Time{}; a.saveSettings() }})
			}
			return out
		}},
		num("Senkron aralığı", "sync minutes dakika", "syncMinutes", 1, 1440, 5, 5, " dk", 0,
			func(a *app) float64 { return a.syncEvery().Minutes() },
			func(a *app, v float64) { a.settings.SyncMinutes = int(v); a.saveSettingsSoon() }),
		simple("İstatistik toplama", "stats analiz kullanım", "noStats", func(s *Settings) *bool { return &s.NoStats }, true),
		simple("Günlük yedek", "backup yedekle", "noBackup", func(s *Settings) *bool { return &s.NoBackup }, true),
		simple("Ligatürler", "ligature fi fl", "noLigatures", func(s *Settings) *bool { return &s.NoLigatures }, true),
		simple("Aynı genişlikte rakamlar", "tabular numbers tnum", "tabularNums", func(s *Settings) *bool { return &s.TabularNums }, false),
		num("Yazı kalınlığı", "weight bold kalın", "fontWeight", 300, 700, 400, 100, "", 0,
			func(a *app) float64 { return float64(a.fontWeight()) },
			func(a *app, v float64) { a.settings.FontWeight = int(v); a.saveSettingsSoon() }),
		tog("Jöle efekti", "caret jöle jelly", []string{"jelly"},
			func(a *app) bool { return a.settings.Jelly },
			func(a *app, v bool) { a.settings.Jelly = v; a.saveSettings() }),
		simple("Jöle sarsıntısı", "pencere sallanma wobble shake", "noWobble", func(s *Settings) *bool { return &s.NoWobble }, true),
		simple("Otomatik kapak", "kapak cover", "noAutoCover", func(s *Settings) *bool { return &s.NoAutoCover }, true),
		simple("Otomatik kayıt", "autosave kaydet save", "noAutoSave", func(s *Settings) *bool { return &s.NoAutoSave }, true),
		simple("Boşlukları göster", "whitespace boşluk space", "showSpaces", func(s *Settings) *bool { return &s.ShowSpaces }, false),
		simple("Sekmeleri göster", "tab whitespace", "showTabs", func(s *Settings) *bool { return &s.ShowTabs }, false),
		simple("Paragraf sonlarını göster", "pilcrow satır sonu eol", "showBreaks", func(s *Settings) *bool { return &s.ShowBreaks }, false),
		simple("Kod bloklarında boşlukları göster", "kod code whitespace", "markCode", func(s *Settings) *bool { return &s.MarkCode }, false),
		simple("Kısayol ipuçları", "keys shortcuts hints status kısayol ipucu", "hideHints", func(s *Settings) *bool { return &s.HideHints }, true),
		simple("Durum çubuğunu gizle", "odak modu focus hide statusbar", "hideStatus", func(s *Settings) *bool { return &s.HideStatus }, false),
		simple("Durum çubuğunda kaydetme durumu", "kaydedildi status", "hideSaved", func(s *Settings) *bool { return &s.HideSaved }, true),
		simple("Durum çubuğunda harf sayısı", "karakter chars", "showChars", func(s *Settings) *bool { return &s.ShowChars }, false),
		simple("Durum çubuğunda satır ve sütun", "line column konum", "showPos", func(s *Settings) *bool { return &s.ShowPos }, false),
		tog("Kenar çubuğu", "sidebar", []string{"sidebarClosed"},
			func(a *app) bool { return !a.settings.SidebarClosed },
			func(a *app, v bool) {
				if v == a.settings.SidebarClosed {
					a.toggleSidebar()
				}
			}),
		tog("Yan panel", "panel anahat outline", []string{"panelOpen"},
			func(a *app) bool { return a.settings.PanelOpen },
			func(a *app, v bool) {
				if v != a.settings.PanelOpen {
					a.togglePanel()
				}
			}),
		tog("MCP sunucusu", "mcp ajan agent", []string{"mcp"},
			func(a *app) bool { return a.settings.MCP },
			func(a *app, v bool) { a.setMCP(v) }),
		num("Yazı boyutu", "font size punto", "fontSize", minFont, maxFont, 16.5, 0.5, " px", 1,
			func(a *app) float64 { return float64(a.fontSize()) }, func(a *app, v float64) { a.setFontSize(v) }),
		num("Satır aralığı", "line height", "lineHeight", 1.2, 2.2, 1.55, 0.05, "", 2,
			func(a *app) float64 { return float64(a.lineHeight()) },
			func(a *app, v float64) { a.settings.LineHeight = v; a.saveSettingsSoon() }),
		num("Sayfa genişliği", "genişlik width sütun", "pageWidth", 480, 1200, float64(columnMax), 20, " px", 0,
			func(a *app) float64 { return float64(a.pageWidth()) },
			func(a *app, v float64) { a.settings.PageWidth = v; a.saveSettingsSoon() }),
		num("Paragraf aralığı", "paragraph spacing boşluk", "paraGap", 0, 2, defaultParaGap, 0.05, " em", 2,
			func(a *app) float64 { return float64(a.paraGap()) },
			func(a *app, v float64) { a.settings.ParaGap = &v; a.saveSettingsSoon() }),
		num("İlk satır girintisi", "indent girinti", "indent", 0, 4, 0, 0.25, " em", 2,
			func(a *app) float64 { return float64(a.firstIndent()) },
			func(a *app, v float64) { a.settings.Indent = v; a.saveSettingsSoon() }),
		num("Kayıt gecikmesi", "autosave delay bekleme", "saveAfter", 0.2, 30, defaultSaveAt, 0.5, " sn", 1,
			func(a *app) float64 { return a.saveAfter().Seconds() },
			func(a *app, v float64) { a.settings.SaveAfter = v; a.saveSettingsSoon() }),
		num("Köşe yuvarlaklığı", "corner radius", "corner", 0, 200, 100, 10, "%", 0,
			func(a *app) float64 { return float64(a.cornerPercent()) },
			func(a *app, v float64) { p := int(v); a.settings.Corner = &p; a.saveSettingsSoon() }),
	}
}

func (d settingDef) fmt(v float64) string {
	return strconv.FormatFloat(v, 'f', d.dec, 64) + d.unit
}

func (d settingDef) clamp(v float64) float64 { return min(max(v, d.min), d.max) }

// undoable runs a change of a setting and offers to take it back: the notice has a Geri al.
func (a *app) undoable(what string, back func(), do func()) func() {
	return func() {
		do()
		a.notice = what
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
			state := map[bool]string{true: "açık", false: "kapalı"}[on]
			it := cmd(T(d.name)+": "+T(map[bool]string{true: "kapat", false: "aç"}[on]), state, iconSettings, a.undoable(
				d.name+": "+map[bool]string{true: "kapalı", false: "açık"}[on], func() { d.flip(a, on) }, func() { d.flip(a, !on) }))
			it.sub = "Ayar"
			add(it)
		case setNumber:
			cur := d.val(a)
			steps := []struct {
				label string
				to    float64
				ok    bool
			}{
				{"artır", d.clamp(cur + d.step), cur+d.step <= d.max+1e-9},
				{"azalt", d.clamp(cur - d.step), cur-d.step >= d.min-1e-9},
				{"varsayılan", d.def, cur != d.def},
			}
			for _, s := range steps {
				s := s
				it := cmd(T(d.name)+": "+T(s.label), d.fmt(cur), iconSettings, a.undoable(T(d.name)+": "+d.fmt(s.to), func() { d.put(a, cur) }, func() { d.put(a, s.to) }))
				it.sub = "Ayar"
				if !s.ok {
					it.off = d.name + " zaten " + map[bool]string{true: "en yüksekte", false: "en düşükte"}[s.label == "artır"] + " (" + d.fmt(cur) + ")"
					if s.label == "varsayılan" {
						it.off = d.name + " zaten varsayılan (" + d.fmt(cur) + ")"
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
				it := cmd(T(d.name)+": "+T(ch.name), map[bool]string{true: "✓", false: ""}[ch.current], iconSettings, a.undoable(T(d.name)+": "+T(ch.name), func() {
					if was != nil {
						was()
					}
				}, func() { ch.set(a) }))
				it.sub = "Ayar"
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
		s := match(m[1], d.name+" "+d.alias)
		if s < 0 || match(m[1], d.name) < 0 && match(m[1], d.alias) < 0 {
			continue
		}
		to, cur := d.clamp(v), d.val(a)
		it := palItem{title: d.name + " → " + d.fmt(to), sub: "Ayar", glyph: iconSettings, score: 1000 + s,
			run: a.undoable(d.name+": "+d.fmt(to), func() { d.put(a, cur) }, func() { d.put(a, to) })}
		if to != v {
			it.title += " (sınırda: " + d.fmt(d.min) + "–" + d.fmt(d.max) + ")"
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
		out := []settingChoice{{name: "varsayılan", current: cur == "",
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
		pre := fold(d.name)
		if !strings.HasPrefix(low, pre+" ") {
			continue
		}
		want := strings.TrimSpace(low[len(pre)+1:])
		if want == "" {
			continue
		}
		for _, f := range a.families() {
			if match(want, f.Name) >= 0 {
				f := f
				was := *a.settings.fontField(role)
				out = append(out, palItem{title: d.name + " → " + f.Name, sub: "Ayar", glyph: iconSettings, score: 900 + match(want, f.Name),
					run: a.undoable(d.name+": "+f.Name, func() { a.setFont(role, was) }, func() { a.setFont(role, f.Name) })})
				if len(out) >= 8 {
					return out
				}
			}
		}
	}
	return out
}
