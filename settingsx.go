package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// More of the settings screen, on the list of settings (cmdreg.go): a search over all of them, the list of
// those that are not as they came, a way back to the default for each and for all, profiles (a look and a
// way of writing that are saved under a name and put on with one command), and the settings as a file.

// defaultsApp is an app with nothing set: what a setting is by default is what it reads there.
func defaultsApp() *app { return &app{caretKind: caretLine()} } // the caret not chosen is the line (#282)

// state is the setting as a word: on/off, a number with its unit, the choice's name.
func (d settingDef) state(a *app) string {
	switch d.kind {
	case setToggle:
		return map[bool]string{true: "açık", false: "kapalı"}[d.on(a)]
	case setNumber:
		return d.fmt(d.val(a))
	}
	for _, ch := range d.choices(a) {
		if ch.current {
			return ch.name
		}
	}
	return ""
}

// changed reports whether the setting is not as it is by default.
func (d settingDef) changed(a *app) bool { return d.state(a) != d.state(defaultsApp()) }

// reset puts the setting back to its default.
func (d settingDef) reset(a *app) {
	z := defaultsApp()
	switch d.kind {
	case setToggle:
		d.flip(a, d.on(z))
	case setNumber:
		d.put(a, d.val(z))
	case setChoice:
		want := d.state(z)
		for _, ch := range d.choices(a) {
			if ch.name == want {
				ch.set(a)
				return
			}
		}
	}
	a.saveSettings()
}

// changedDefs are the settings that are not as they came.
func (a *app) changedDefs() []settingDef {
	var out []settingDef
	for _, d := range settingDefs() {
		if d.changed(a) {
			out = append(out, d)
		}
	}
	return out
}

// resetAll puts every changed setting back, and says how many.
func (a *app) resetAll() int {
	n := 0
	for _, d := range a.changedDefs() {
		d.reset(a)
		n++
	}
	return n
}

// searchDefs are the settings that the words ask for, best first.
func searchDefs(q string) []settingDef {
	type scored struct {
		d settingDef
		s int
	}
	var all []scored
	for _, d := range settingDefs() {
		s := match(q, T(d.name))
		if s2 := match(q, d.name+" "+d.alias); s < 0 && s2 >= 0 {
			s = s2 / 2
		}
		if s >= 0 {
			all = append(all, scored{d, s})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].s > all[j].s })
	out := make([]settingDef, len(all))
	for i, e := range all {
		out[i] = e.d
	}
	return out
}

// tabOf is the tab of the settings a field is on.
func tabOfField(f string) int {
	switch {
	case strings.HasPrefix(f, "font") || f == "noLigatures" || f == "tabularNums":
		return tabFonts
	case strings.HasPrefix(f, "update"):
		return tabUpdate
	case strings.HasPrefix(f, "sync"):
		return tabSync
	case f == "mcp":
		return tabMCP
	case f == "noBackup":
		return tabLibrary
	case f == "hideSaved" || f == "showChars" || f == "showPos" || f == "hideStatus":
		return tabStatus
	case f == "theme" || f == "appTheme" || f == "pageWidth" || f == "corner" || f == "lang":
		return tabAppearance
	}
	return tabEditor
}

// ---- profiles ----

// profileKeys are the fields a profile holds: how it looks and how it is written in.
var profileKeys = []string{"theme", "appTheme", "caretName", "jelly", "fontSize", "lineHeight", "pageWidth", "paraGap", "indent",
	"fontApp", "fontEditor", "fontMono", "noLigatures", "tabularNums", "fontWeight", "corner", "showSpaces", "showTabs", "showBreaks", "markCode"}

// builtinProfiles are ways of writing that come with the app.
var builtinProfiles = map[string]map[string]any{
	"Yazar": {"fontSize": 18.0, "lineHeight": 1.7, "pageWidth": 680.0, "paraGap": 0.75, "indent": 0.0, "fontWeight": 400},
	"Kod":   {"fontSize": 14.5, "lineHeight": 1.45, "pageWidth": 900.0, "paraGap": 0.3, "showSpaces": true, "showTabs": true, "noLigatures": true},
	"Sıkı":  {"fontSize": 14.0, "lineHeight": 1.35, "pageWidth": 820.0, "paraGap": 0.3},
	"Okuma": {"fontSize": 19.0, "lineHeight": 1.8, "pageWidth": 640.0, "paraGap": 0.9, "indent": 1.0},
}

// profileNames are the saved profiles and the built-in ones.
func (a *app) profileNames() []string {
	seen := map[string]bool{}
	var out []string
	for n := range a.settings.Profiles {
		seen[n] = true
		out = append(out, n)
	}
	for n := range builtinProfiles {
		if !seen[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// saveProfile keeps the look and the way of writing as they are now, under a name.
func (a *app) saveProfile(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	data, _ := json.Marshal(a.settings)
	var all map[string]any
	json.Unmarshal(data, &all)
	prof := map[string]any{}
	for _, k := range profileKeys {
		if v, ok := all[k]; ok {
			prof[k] = v
		}
	}
	if a.settings.Profiles == nil {
		a.settings.Profiles = map[string]map[string]any{}
	}
	a.settings.Profiles[name] = prof
	a.saveSettings()
}

// applyProfile puts a profile on: what it holds is set, the rest of the settings stay.
func (a *app) applyProfile(name string) bool {
	prof, ok := a.settings.Profiles[name]
	if !ok {
		prof, ok = builtinProfiles[name]
	}
	if !ok {
		return false
	}
	data, _ := json.Marshal(a.settings)
	var all map[string]any
	json.Unmarshal(data, &all)
	for _, k := range profileKeys { // a profile is what it says: the fields it does not hold are the defaults
		delete(all, k)
	}
	for k, v := range prof {
		all[k] = v
	}
	data, _ = json.Marshal(all)
	var next Settings // from nothing: what the profile does not say is the default, what is not a look is in all
	if err := json.Unmarshal(data, &next); err != nil {
		return false
	}
	a.settings = next
	a.caretKind = a.savedCaret()
	a.saveSettings()
	return true
}

func (a *app) deleteProfile(name string) {
	delete(a.settings.Profiles, name)
	a.saveSettings()
}

// ---- the settings as a file ----

// secretKeys are never exported nor imported: they belong to this computer.
var secretKeys = []string{"token", "port", "mcp", "syncToken", "syncKey", "accountUser", "accountAsked", "syncDir", "syncUrl", "syncMode", "writing", "recentCmds", "recent", "treeOpen", "treeSaved",
	"updateAt", "updateSnooze", "updateSkip", "updateNotes", "updateFor", "updateSeen", "tourDone", "welcomeDone", "quickKey", "sidebarClosed", "panelOpen"}

// exportSettingsTo writes the portable settings as JSON.
func (a *app) exportSettingsTo(path string) error {
	data, _ := json.Marshal(a.settings)
	var all map[string]any
	json.Unmarshal(data, &all)
	for _, k := range secretKeys {
		delete(all, k)
	}
	out, _ := json.MarshalIndent(all, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0o755)
	return WriteFilePrivate(path, out) // no key in it, but the user's own settings
}

// importSettingsFrom reads settings from a file written by exportSettingsTo: what it holds is set, secrets are not touched.
func (a *app) importSettingsFrom(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var in map[string]any
	if err := json.Unmarshal(b, &in); err != nil {
		return 0, err
	}
	for _, k := range secretKeys {
		delete(in, k)
	}
	data, _ := json.Marshal(a.settings)
	var all map[string]any
	json.Unmarshal(data, &all)
	for k, v := range in {
		all[k] = v
	}
	data, _ = json.Marshal(all)
	next := a.settings
	if err := json.Unmarshal(data, &next); err != nil {
		return 0, err
	}
	a.settings = next
	a.caretKind = a.savedCaret()
	a.saveSettings()
	return len(in), nil
}

func (a *app) settingsFileOut() string {
	dir := filepath.Dir(a.settingsFile)
	if a.settingsFile == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "ayarlar-"+a.clock().Format("20060102-150405")+".json")
}

func (a *app) pickSettingsFile() {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: T("İçe aktarılacak ayar dosyası")})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error {
			n, err := a.importSettingsFrom(paths[0])
			if err != nil {
				a.notice = "Olmadı: " + err.Error()
			} else {
				a.notice = Tf("%d ayar içe aktarıldı", n)
			}
			return nil
		})
	}()
}

// ---- the screen's top: search, changed, profiles ----

// settingsTop is the bar over a section: the search, the list of changed settings, the profiles.
func (a *app) settingsTop(c *ui.Context) {
	cl := ryugo.From(c).Colors
	search := func() {
		q := a.setSearch
		in := ui.TextInputBase(c.Key("set-search"), &q).Label("Ayarlarda ara").Placeholder(T("Ayarlarda ara…")).PlaceholderColor(cl.Muted).
			FontSize(13).Padding(7, 11).Radius(rad(9)).Background(cl.Card).Border(1, cl.Hairline).Grow(1).FocusRing(false)
		a.fieldMenu(in, &q, fieldOpts{oneLine: true, changed: func() { a.setSearch = q }})
		if phone {
			in.FillWidth().Padding(10, 12)
		}
		if in.Changed() {
			a.setSearch = q
		}
	}
	modes := func() {
		for _, m := range []struct{ id, name string }{{"changed", "Değiştirilenler"}, {"profiles", "Profiller"}, {"keys", "Kısayollar"}, {"file", "Dosya"}} {
			m := m
			on := a.setMode == m.id
			b := ui.Row(c.Key("set-mode:"+m.id)).Padding(7, 12).Radius(rad(8)).Background(map[bool]ui.Color{true: cl.Accent.Alpha(0.2), false: cl.Fill}[on]).
				Label(T(m.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			b.Children(func() { ui.Text(c, T(m.name)).FontSize(12.5).TextColor(cl.Text) })
			if b.Clicked() {
				a.setMode = map[bool]string{true: "", false: m.id}[on]
				a.setSearch = ""
			}
		}
	}
	box := ui.Box(c.Key("set-top-box")).FillWidth()
	narrow := phone
	if b := box.LayoutBounds(); b.W > 0 && b.W < 560 { // a small window: no room for the search beside the pages
		narrow = true
	}
	box.Children(func() {
		if narrow { // the search on a line of its own, the pages under it
			ui.Column(c.Key("set-top")).FillWidth().Gap(8).Padding(0, 0, 10, 0).Children(func() {
				search()
				ui.Row(c).Gap(6).Wrap().Children(modes)
			})
			return
		}
		ui.Row(c.Key("set-top")).FillWidth().Gap(8).AlignItems(ui.Center).Padding(0, 0, 10, 0).Children(func() {
			search()
			modes()
		})
	})
}

// settingsExtra draws the search results or the special pages in place of a section; it reports that it did.
func (a *app) settingsExtra(c *ui.Context) bool {
	cl := ryugo.From(c).Colors
	switch {
	case strings.TrimSpace(a.setSearch) != "":
		res := searchDefs(a.setSearch)
		if len(res) == 0 {
			ui.Text(c, T("Eşleşen ayar yok.")).FontSize(13).TextColor(cl.Muted)
			return true
		}
		var rows []func()
		for _, d := range res {
			d := d
			rows = append(rows, func() { a.settingLine(c, d, false) })
		}
		settingGroup(c, "set-found", rows...)
		return true
	case a.setMode == "changed":
		ch := a.changedDefs()
		if len(ch) == 0 {
			ui.Text(c, T("Hiçbir ayar değişmemiş: hepsi varsayılan.")).FontSize(13).TextColor(cl.Muted)
			return true
		}
		if ryugo.Button(c.Key("reset-all"), T("Hepsini varsayılana döndür")+" ("+strconv.Itoa(len(ch))+")", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			n := a.resetAll()
			a.notice = Tf("%d ayar varsayılana döndü", n)
		}
		var rows []func()
		for _, d := range ch {
			d := d
			rows = append(rows, func() { a.settingLine(c, d, true) })
		}
		settingGroup(c, "set-changed", rows...)
		return true
	case a.setMode == "profiles":
		a.settingsProfiles(c)
		return true
	case a.setMode == "keys":
		a.settingsKeys(c)
		return true
	case a.setMode == "file":
		settingGroup(c, "set-file",
			func() {
				settingRow(c, "Ayarları dosyaya yaz", "Görünüm ve yazı ayarları JSON olarak yazılır; parolalar, jetonlar ve bu bilgisayara ait konumlar yazılmaz. Başka bir bilgisayara taşımak için.", func() {
					if ryugo.Button(c.Key("set-export"), T("Dışa aktar"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						p := a.settingsFileOut()
						if err := a.exportSettingsTo(p); err != nil {
							a.notice = "Olmadı: " + err.Error()
						} else {
							a.notice = "Yazıldı: " + p
						}
					}
				})
			},
			func() {
				settingRow(c, "Ayar dosyası oku", "Dosyadakiler ayarlanır; yazılmayanlar olduğu gibi kalır.", func() {
					if ryugo.Button(c.Key("set-import"), T("İçe aktar…"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.pickSettingsFile()
					}
				})
			})
		return true
	}
	return false
}

// settingLine is a setting in a list, with a control that is enough, where it is, and its way back.
func (a *app) settingLine(c *ui.Context, d settingDef, withReset bool) {
	cl := ryugo.From(c).Colors
	tab := tabOfField(d.fields[0])
	where := T(settingsSections[tab].name) + " · ⌘P › " + T(d.name)
	settingRow(c, d.name, where, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			switch d.kind {
			case setToggle:
				on := d.on(a)
				ryugo.Switch(c.Key("sl:"+d.name), &on, ryugo.SwitchOptions{}).Label(T(d.name))
				if on != d.on(a) {
					d.flip(a, on)
				}
			case setNumber:
				ui.Text(c, d.fmt(d.val(a))).FontSize(13).FontWeight(620).TextColor(cl.Text2)
				for _, st := range []struct {
					l string
					d float64
				}{{"−", -d.step}, {"+", d.step}} {
					st := st
					b := ui.Row(c.Key("sln:"+d.name+st.l)).Padding(4, 10).Radius(rad(7)).Background(cl.Fill).Label(st.l + " " + T(d.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					b.Children(func() { ui.Text(c, st.l).FontSize(13).TextColor(cl.Text) })
					if b.Clicked() {
						d.put(a, d.clamp(d.val(a)+st.d))
					}
				}
			default:
				ui.Text(c, T(d.state(a))).FontSize(13).FontWeight(620).TextColor(cl.Text2)
			}
			if withReset || d.changed(a) {
				b := ui.Row(c.Key("slr:"+d.name)).Padding(4, 10).Radius(rad(7)).Background(cl.Fill).Label(T("Varsayılana dön") + ": " + T(d.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				b.Children(func() { ui.Text(c, T("Varsayılana dön")).FontSize(12).TextColor(cl.Text2) })
				if b.Clicked() {
					d.reset(a)
				}
			}
			g := ui.Row(c.Key("slg:"+d.name)).Padding(4, 10).Radius(rad(7)).Background(cl.Fill).Label(T("Git") + ": " + T(d.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			g.Children(func() { ui.Text(c, T("Git")).FontSize(12).TextColor(cl.Text2) })
			if g.Clicked() {
				a.settingsTab, a.setSearch, a.setMode = tab, "", ""
			}
		})
	})
}

func (a *app) settingsProfiles(c *ui.Context) {
	cl := ryugo.From(c).Colors
	var rows []func()
	for _, n := range a.profileNames() {
		n := n
		_, mine := a.settings.Profiles[n]
		rows = append(rows, func() {
			help := T("Hazır profil")
			if mine {
				help = T("Kayıtlı profil")
			}
			settingRow(c, n, help, func() {
				ui.Row(c).Gap(6).Children(func() {
					if ryugo.Button(c.Key("prof-use:"+n), T("Uygula"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						a.applyProfile(n)
						a.notice = "Profil: " + n
					}
					if mine && ryugo.Button(c.Key("prof-del:"+n), T("Sil"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.deleteProfile(n)
					}
				})
			})
		})
	}
	settingGroup(c, "set-profiles", rows...)
	settingGroup(c, "set-profile-new", func() {
		settingRow(c, "Şimdikini profil olarak kaydet", "Tema, renk, yazı tipleri, boyutlar, caret ve boşluk gösterimi bir ad altında saklanır; ⌘P'de “Profil: ad” ile geçilir.", func() {
			ui.Row(c).Gap(6).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("prof-name"), &a.profName, ryugo.InputOptions{Label: "Profil adı", Size: ryugo.Sm, Placeholder: T("Profil adı")}), &a.profName, fieldOpts{oneLine: true})
				if ryugo.Button(c.Key("prof-save"), T("Kaydet"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.saveProfile(a.profName)
					a.profName = ""
				}
			})
		})
	})
}

// choiceBar is Ryugo's segmented control for a choice among a few: the id chosen is put in *cur.
func choiceBar(c *ui.Context, key, label string, ids, names []string, cur *string) {
	sel := 0
	for i, id := range ids {
		if id == *cur {
			sel = i
		}
	}
	was := sel
	ryugo.Segmented(c.Key(key), &sel, names, ryugo.SegmentedOptions{Label: label})
	if sel != was {
		*cur = ids[sel]
	}
}
