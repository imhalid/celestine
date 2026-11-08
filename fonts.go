package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The typefaces: the app's, the editor's and the one of code, chosen from the families installed in the
// computer, each listed in its own letters and shown on the page as it is passed over. A family that is
// not there (another computer, a synced setting) is left out of the stack, and the default stands in.

type fontRole int

const (
	fontApp fontRole = iota
	fontEditor
	fontMono
)

var fontRoles = []struct {
	name, help, sample string
}{
	{"Uygulama yazı tipi", "Menüler, kenar çubuğu, başlıklar ve düğmeler.", "Kenar çubuğu · Ayarlar · Çöp"},
	{"Editör yazı tipi", "Sayfanın metni. Boşsa uygulamanınki kullanılır.", "Pijamalı hasta yağız şoföre çabucak güvendi."},
	{"Kod yazı tipi", "Kod blokları ve satır içi kod.", "func main() { fmt.Println(\"ışığı çöz\") }"},
}

// Candidates for the presets: the first that is installed is taken.
var (
	serifFonts = []string{"Charter", "Iowan Old Style", "Georgia", "Source Serif 4", "Source Serif Pro", "Noto Serif", "Liberation Serif", "DejaVu Serif", "Times New Roman", "Cambria"}
	sansFonts  = []string{"Inter", "SF Pro Text", "Helvetica Neue", "Noto Sans", "Segoe UI", "Roboto", "Liberation Sans", "DejaVu Sans", "Arial"}
	monoFonts  = []string{"JetBrains Mono", "SF Mono", "Menlo", "Fira Code", "Cascadia Code", "Consolas", "Noto Sans Mono", "Liberation Mono", "DejaVu Sans Mono", "Courier New"}
)

// fontState is the families installed, found in the background, and what is being looked at.
type fontState struct {
	mu      sync.Mutex
	started bool
	fam     []ui.FontFamily
	ready   bool
	query   [3]string // the search of each list
	open    [3]bool   // the list is open
}

// families are the installed families; empty until the system has told.
func (a *app) families() []ui.FontFamily {
	f := &a.fonts
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.started {
		f.started = true
		inv := a.invalidate
		go func() {
			fam := ui.FontFamilies()
			f.mu.Lock()
			f.fam, f.ready = fam, true
			f.mu.Unlock()
			if inv != nil {
				inv()
			}
		}()
	}
	return f.fam
}

// hasFont reports whether the family is installed; until the list is known, it is taken to be.
func (a *app) hasFont(name string) bool {
	fam := a.families()
	a.fonts.mu.Lock()
	ready := a.fonts.ready
	a.fonts.mu.Unlock()
	if !ready {
		return true
	}
	for _, f := range fam {
		if strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

func (s *Settings) fontField(r fontRole) *string {
	switch r {
	case fontEditor:
		return &s.FontEditor
	case fontMono:
		return &s.FontMono
	}
	return &s.FontApp
}

// fontChosen is the family of the role: the one set. A list shows each family in its own letters;
// nothing on the page changes until one is chosen (a page that changed under the pointer moved the whole layout).
func (a *app) fontChosen(r fontRole) string {
	return *a.settings.fontField(r)
}

// stackOf is the family put first in a stack that ends in the default, so that a family that is
// gone leaves the default in its place.
func (a *app) stackOf(name, def string) string {
	if name == "" || !a.hasFont(name) || strings.ContainsAny(name, ",\"'") {
		return def
	}
	return name + ", " + def
}

// appFont, editorFont and monoFont are the stacks the window is drawn in.
func (a *app) appFont() string { return a.stackOf(a.fontChosen(fontApp), ryugo.FontSans) }
func (a *app) editorFont() string {
	if n := a.fontChosen(fontEditor); n != "" {
		return a.stackOf(n, a.appFont())
	}
	return a.appFont()
}
func (a *app) monoFont() string { return a.stackOf(a.fontChosen(fontMono), "monospace") }

// fontFeatures are the OpenType features the settings ask for, for the text.
func (a *app) fontFeatures() string {
	var f []string
	if a.settings.NoLigatures {
		f = append(f, "liga=0", "clig=0")
	}
	if a.settings.TabularNums {
		f = append(f, "tnum")
	}
	return strings.Join(f, ",")
}

// fontWeight is the weight of the text: 400 when not set.
func (a *app) fontWeight() int {
	if w := a.settings.FontWeight; w >= 300 && w <= 700 {
		return w
	}
	return 400
}

// setFont keeps the family of a role, and the lists of those used last: the family goes first.
func (a *app) setFont(r fontRole, name string) {
	*a.settings.fontField(r) = name
	if name != "" {
		l := []string{name}
		for _, t := range a.settings.FontLast {
			if t != name && len(l) < 8 {
				l = append(l, t)
			}
		}
		a.settings.FontLast = l
	}
	a.saveSettingsSoon()
}

// toggleFav stars the family, or takes the star away.
func (a *app) toggleFav(name string) {
	var out []string
	had := false
	for _, f := range a.settings.FontFavs {
		if f == name {
			had = true
			continue
		}
		out = append(out, f)
	}
	if !had {
		out = append(out, name)
	}
	a.settings.FontFavs = out
	a.saveSettingsSoon()
}

func (a *app) isFav(name string) bool {
	for _, f := range a.settings.FontFavs {
		if f == name {
			return true
		}
	}
	return false
}

// firstInstalled is the first of the candidates that is installed.
func (a *app) firstInstalled(list []string) string {
	for _, n := range list {
		if a.hasFontStrict(n) {
			return n
		}
	}
	return ""
}

// hasFontStrict is hasFont, but false while the list is not known.
func (a *app) hasFontStrict(name string) bool {
	a.families()
	a.fonts.mu.Lock()
	ready := a.fonts.ready
	a.fonts.mu.Unlock()
	return ready && a.hasFont(name)
}

// fontPreset puts the editor and code fonts as a way of writing asks: reading in a serif, writing in
// a sans, code in a monospace.
func (a *app) fontPreset(kind string) bool {
	switch kind {
	case "Okuma":
		if f := a.firstInstalled(serifFonts); f != "" {
			a.setFont(fontEditor, f)
			return true
		}
	case "Yazı":
		if f := a.firstInstalled(sansFonts); f != "" {
			a.setFont(fontEditor, f)
			return true
		}
	case "Kod":
		if f := a.firstInstalled(monoFonts); f != "" {
			a.setFont(fontEditor, f)
			a.setFont(fontMono, f)
			return true
		}
	}
	return false
}

// fontRows are the families of a list: the starred, those used last, and then all, narrowed by the
// search, at most 80 at once (the rest are a search away).
func (a *app) fontRows(r fontRole) (rows []string, more int) {
	q := a.fonts.query[r]
	seen := map[string]bool{}
	add := func(n string) {
		if seen[n] || (q != "" && match(q, n) < 0) {
			return
		}
		seen[n] = true
		rows = append(rows, n)
	}
	fam := a.families()
	has := map[string]bool{}
	for _, f := range fam {
		has[f.Name] = true
	}
	for _, n := range a.settings.FontFavs {
		if has[n] {
			add(n)
		}
	}
	for _, n := range a.settings.FontLast {
		if has[n] {
			add(n)
		}
	}
	var rest []string
	for _, f := range fam {
		if r == fontMono && !f.Monospaced && q == "" {
			continue // the code font's list is the monospaced ones, till the search asks for others
		}
		if !seen[f.Name] && (q == "" || match(q, f.Name) >= 0) {
			rest = append(rest, f.Name)
		}
	}
	if q != "" {
		sort.SliceStable(rest, func(i, j int) bool { return match(q, rest[i]) > match(q, rest[j]) })
	}
	for _, n := range rest {
		if len(rows) >= 80 {
			more++
			continue
		}
		add(n)
	}
	return rows, more
}

// settingsFonts is the settings' page of the typefaces.
func (a *app) settingsFonts(c *ui.Context) {
	cl := ryugo.From(c).Colors
	for ri := range fontRoles {
		r := fontRole(ri)
		role := fontRoles[ri]
		cur := *a.settings.fontField(r)
		settingGroup(c, "set-font-"+strconv.Itoa(ri), func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, role.name).FontSize(15).FontWeight(620).TextColor(cl.Text)
						ui.Text(c, role.help).FontSize(ryugo.FsSm).TextColor(cl.Muted)
					})
					label := cur
					switch {
					case label == "" && r == fontMono:
						label = "Sistem (tek aralıklı)"
					case label == "" && r == fontEditor:
						label = "Uygulamanınki"
					case label == "":
						label = "Sistem"
					}
					b := ui.Row(c.Key("fontbtn"+strconv.Itoa(ri))).Gap(6).Padding(6, 12).Radius(rad(8)).Background(cl.Fill).AlignItems(ui.Center).
						Label(role.name + ": " + label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					b.Children(func() {
						ui.Text(c, label).FontSize(13).Font(a.stackOf(cur, ryugo.FontSans)).TextColor(cl.Text).MaxLines(1)
						ui.Text(c, map[bool]string{true: "▴", false: "▾"}[a.fonts.open[ri]]).FontSize(11).TextColor(cl.Muted)
					})
					if b.Clicked() {
						a.fonts.open[ri] = !a.fonts.open[ri]
					}
				})
				ui.Text(c, role.sample).FontSize(15).Font(a.stackOf(cur, map[bool]string{true: "monospace", false: ryugo.FontSans}[r == fontMono])).TextColor(cl.Text2).MaxLines(1)
				if !a.fonts.open[ri] {
					return
				}
				q := a.fonts.query[ri]
				in := ui.TextInputBase(c.Key("fontq"+strconv.Itoa(ri)), &q).Label(T("Yazı tipi ara")).Placeholder(T("Yazı tipi ara…")).PlaceholderColor(cl.Muted).
					FontSize(13).Padding(7, 10).Radius(rad(8)).Background(cl.Card).Border(1, cl.Hairline).FocusRing(false)
				a.fieldMenu(in, &q, fieldOpts{oneLine: true, changed: func() { a.fonts.query[ri] = q }})
				if in.Changed() {
					a.fonts.query[ri] = q
				}
				rows, more := a.fontRows(r)
				ready := a.fonts.ready
				a.scrolled(c, "fontlist"+strconv.Itoa(ri), nil, ui.Scroll(c.Key("fontlist"+strconv.Itoa(ri))).MaxHeight(260), func() {
					ui.Column(c).FillWidth().Gap(1).Children(func() {
						if !ready {
							ui.Text(c, T("Yazı tipleri taranıyor…")).FontSize(12).TextColor(cl.Muted).Padding(8)
						}
						if cur != "" {
							a.fontRow(c, r, "", "Varsayılan", false)
						}
						for _, n := range rows {
							a.fontRow(c, r, n, n, n == cur)
						}
						if more > 0 {
							ui.Text(c, "… "+strconv.Itoa(more)+" yazı tipi daha: aramayla bul").FontSize(11.5).TextColor(cl.Muted).Padding(6, 8)
						}
						if ready && len(rows) == 0 {
							ui.Text(c, T("Eşleşen yazı tipi yok.")).FontSize(12).TextColor(cl.Muted).Padding(8)
						}
					})
				})
			})
		})
	}
	// the shape of the letters
	settingGroup(c, "set-font-presets",
		func() {
			settingRow(c, "Hazır yazı tipleri", "Okuma serif, yazı sans, kod tek aralıklı; bilgisayarda olanlardan ilki seçilir.", func() {
				ui.Row(c).Gap(8).Children(func() {
					for _, k := range []string{"Okuma", "Yazı", "Kod"} {
						k := k
						if ryugo.Button(c.Key("fpreset"+k), k, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() && !a.fontPreset(k) {
							a.notice = Tf("Bu bilgisayarda “%s” için uygun bir yazı tipi bulunamadı.", k)
						}
					}
				})
			})
		})
	lig := !a.settings.NoLigatures
	tab := a.settings.TabularNums
	weight := float64(a.fontWeight())
	settingGroup(c, "set-font-detail",
		func() {
			settingRow(c, "Ligatürler", "fi, fl gibi harf çiftleri tek harf olarak çizilir. Kod yazarken kapatmak işe yarar.", func() { ryugo.Switch(c.Key("set-liga"), &lig, ryugo.SwitchOptions{}).Label(T("Ligatürler")) })
		},
		func() {
			settingRow(c, "Aynı genişlikte rakamlar", "Rakamların hepsi aynı genişliktedir: sütunlar ve tablolar hizalı durur.", func() {
				ryugo.Switch(c.Key("set-tnum"), &tab, ryugo.SwitchOptions{}).Label(T("Aynı genişlikte rakamlar"))
			})
		},
		func() {
			settingRow(c, "Yazı kalınlığı", "Editör metninin kalınlığı; yazı tipinde o kalınlık yoksa en yakını çizilir.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-weight"), &weight, 300, 700, ryugo.SliderOptions{Label: "Yazı kalınlığı", Step: 100, Format: func(v float64) string { return strconv.Itoa(int(v)) }})
				})
			})
		})
	if lig != !a.settings.NoLigatures {
		a.settings.NoLigatures = !lig
		a.saveSettings()
	}
	if tab != a.settings.TabularNums {
		a.settings.TabularNums = tab
		a.saveSettings()
	}
	if int(weight) != a.fontWeight() {
		a.settings.FontWeight = int(weight)
		a.saveSettingsSoon()
	}
}

// fontRow is a family of a list, in its own letters; a click chooses it.
func (a *app) fontRow(c *ui.Context, r fontRole, name, label string, on bool) {
	cl := ryugo.From(c).Colors
	e := ui.Row(c.Key("frow"+strconv.Itoa(int(r))+":"+name)).FillWidth().Height(32).Padding(0, 10).Gap(8).Radius(rad(7)).AlignItems(ui.Center).
		Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	if on {
		e.Background(cl.Accent.Alpha(0.16))
	} else if e.Hovered() {
		e.Background(cl.Fill)
	}
	e.Children(func() {
		f := ryugo.FontSans
		if name != "" {
			f = a.stackOf(name, ryugo.FontSans)
		}
		ui.Text(c, label).FontSize(14).Font(f).TextColor(cl.Text).MaxLines(1).Grow(1)
		if on {
			ui.Text(c, "✓").FontSize(12).TextColor(cl.Accent)
		}
		if name != "" {
			star := ui.Text(c.Key("fstar"+strconv.Itoa(int(r))+":"+name), map[bool]string{true: "★", false: "☆"}[a.isFav(name)]).FontSize(13).TextColor(cl.Muted)
			if a.isFav(name) {
				star.TextColor(cl.Accent)
			}
		}
	})
	if name != "" {
		// a click on the star's side stars it; elsewhere it chooses
		if x, _, over := e.PointerPosition(); over && e.Clicked() && x > e.LayoutBounds().W-34 {
			a.toggleFav(name)
			return
		}
	}
	if e.Clicked() {
		a.setFont(r, name)
	}
}
