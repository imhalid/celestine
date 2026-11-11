package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The keyboard, all of it. Esc in the text leaves it - the focus goes to an anchor that is not seen -
// and the keys are then commands, as in Vim's normal mode; i, a or Enter go back to writing. A click
// anywhere takes the focus off the anchor, and the keys are the text's again: there is no mode to be
// stuck in.
//
//	j k ↓ ↑   the next row, the one before        gg G    first, last
//	l ↵       open it                            h       back
//	5j        five rows                          f       letters on everything that can be pressed
//	Space     the leader: a menu of what comes next (which-key)
//	/ :       find in the page; the command line (:w :q :e name :theme koyu :set fontsize=18)
//	m a  ' a  mark a place with a letter, go to it
//	u ⌃R      undo, redo        ?  the shortcuts
//
// A screen shows its rows to the keys by kbMark: the rows are what j and k move over, and what f puts
// letters on.

// A kbTarget is something on the screen that can be pressed.
type kbTarget struct {
	key   string
	r     ui.Rect
	act   func()
	list  bool // a row of the list: j and k move over it
	label string
}

type kbState struct {
	frame, last []kbTarget
	selKey      string   // the row chosen
	moved       bool     // the choice moved this frame: it is scrolled into view
	normal      bool     // the anchor has the focus: the keys are commands
	park        bool     // the focus goes to the anchor next frame
	count       int      // the number typed before a command
	pending     string   // the first key of a pair: g, m, '
	again       struct { // what was done last, for .
		r    rune
		n    int
		pair string
	}
	repeating bool
	hints     bool   // letters are shown
	typed     string // what of a letter was typed
	which     bool   // the leader was pressed
	marks     map[rune]place
	help      bool // the page of the shortcuts
	accent    ui.Color
}

// kbBegin starts the frame's list of targets.
func (a *app) kbBegin() {
	k := &a.kb
	k.last, k.frame = k.frame, k.last[:0]
}

// kbMark shows a row or a button to the keys: key says which it is, from frame to frame, act is what
// pressing it does. A row of a list is lit when it is the chosen one, in the normal mode.
func (a *app) kbMark(e ui.Element, key string, list bool, act func()) {
	k := &a.kb
	k.frame = append(k.frame, kbTarget{key: key, r: e.LayoutBounds(), act: act, list: list})
	if list && (k.normal || a.settings.AlwaysRing) && key == k.selKey {
		e.Border(2, k.accent)
		if k.moved {
			e.ScrollIntoView()
		}
	}
}

// kbAnchor is the place the focus goes to when the keys are commands.
func (a *app) kbAnchor(c *ui.Context) {
	e := ui.Box(c.Key("kb-anchor")).Size(0, 0).Focusable().FocusRing(false).Label(locales.KeyboardMode())
	if a.kb.park {
		a.kb.park = false
		e.Focus()
	}
	a.kb.normal = e.Focused()
	a.kb.accent = ryugo.From(c).Colors.Accent
}

// kbEnter takes the focus off the text: the keys are commands.
func (a *app) kbEnter() { a.kb.park = true }

// kbLeave goes back to writing.
func (a *app) kbLeave() {
	a.kb.normal, a.kb.hints, a.kb.which, a.kb.pending, a.kb.count = false, false, false, "", 0
	a.refocus = true
}

// rows are the list rows of the last frame.
func (k *kbState) rows() []kbTarget {
	var out []kbTarget
	for _, t := range k.last {
		if t.list {
			out = append(out, t)
		}
	}
	return out
}

// move chooses the row n further, or n before.
func (a *app) kbMove(n int) {
	k := &a.kb
	rows := k.rows()
	if len(rows) == 0 {
		return
	}
	at := -1
	for i, r := range rows {
		if r.key == k.selKey {
			at = i
		}
	}
	at = min(max(at+n, 0), len(rows)-1)
	if at == -1 {
		at = 0
	}
	k.selKey, k.moved = rows[at].key, true
}

// kbTo chooses the first or the last row.
func (a *app) kbTo(last bool) {
	rows := a.kb.rows()
	if len(rows) == 0 {
		return
	}
	i := 0
	if last {
		i = len(rows) - 1
	}
	a.kb.selKey, a.kb.moved = rows[i].key, true
}

// kbOpen does what the chosen row does.
func (a *app) kbOpen() bool {
	for _, r := range a.kb.last {
		if r.list && r.key == a.kb.selKey && r.act != nil {
			r.act()
			return true
		}
	}
	return false
}

// the leader's menu: the key after Space, what it does
var leaderKeys = []struct {
	key  rune
	name string
	run  func(a *app)
}{
	{'s', "common_settings", func(a *app) { a.openSettings() }},
	{'p', "common_commands", func(a *app) { a.openCommands() }},
	{'o', "Sayfa aç", func(a *app) { a.openPalette() }},
	{'f', "findInPage2", func(a *app) { a.openFind() }},
	{'/', "searchTheLibrary", func(a *app) { a.openSearch(a.searchQ) }},
	{'t', "common_tasks", func(a *app) { a.openTasks() }},
	{'d', "common_trash", func(a *app) { a.openTrash() }},
	{'j', "todaysPage", func(a *app) { a.openToday() }},
	{'g', "common_goDate", func(a *app) { a.askDate() }},
	{'i', "common_statistics", func(a *app) { a.openStats() }},
	{'n', "common_newPage", func(a *app) {
		if c := a.currentColl(); c != nil {
			a.newPage(c, nil)
		}
	}},
	{'b', "sidebar", func(a *app) { a.toggleSidebar() }},
	{'w', "switchPane", func(a *app) { a.nextPane() }},
	{'c', "closeThePane", func(a *app) { a.closeThisPane() }},
	{'v', "openInASidePane", func(a *app) { a.openHereInSide() }},
	{'k', "shortcuts", func(a *app) { a.kb.help = true }},
}

// kbKey reads the keys of the normal mode; it reports a key that it took.
func (a *app) kbKeys(c *ui.Context) {
	k := &a.kb
	if !k.normal || a.modal() || a.pal.open || a.cellEd.open {
		return
	}
	r, ok := kbLetter(c)
	if !ok {
		return
	}
	if a.bsel.on && a.blockSelKey(r) { // the blocks chosen take their keys first
		return
	}
	if r == 0x1b { // Esc: leaves what was begun
		switch {
		case k.help:
			k.help = false
		case k.hints || k.which || k.pending != "" || k.count > 0:
			k.hints, k.which, k.pending, k.count = false, false, "", 0
			k.typed = ""
		case a.statsScreen:
			a.closeStats()
		case a.profileScreen:
			a.closeProfile()
		case a.settingsOpen:
			a.closeSettings()
		default:
			a.step(-1)
		}
		return
	}
	if k.help {
		return
	}
	if k.hints {
		a.kbHintKey(r)
		return
	}
	if k.which {
		k.which = false
		for _, l := range leaderKeys {
			if l.key == r {
				l.run(a)
				return
			}
		}
		a.say(toastWarn, locales.KeysNoSuchShortcut(string(r)))
		return
	}
	if k.pending != "" {
		p := k.pending
		k.pending = ""
		switch {
		case (p == "d" || p == "y") && a.vimPair(rune(p[0]), r):
		case p == "g" && r == 'g':
			if a.vimOn() {
				a.doc.SetSelection(0, 0)
				a.revealWant = true
			} else {
				a.kbTo(false)
			}
		case p == "g" && r == 'f': // gf: the link, tag or chip the caret is on
			a.followAtCaret()
		case p == "m":
			if k.marks == nil {
				k.marks = map[rune]place{}
			}
			k.marks[r] = a.here()
			a.say(toastInfo, locales.KeysMarkSet(string(r)))
		case p == "'":
			if pl, ok := k.marks[r]; ok {
				a.goTo(pl)
			} else {
				a.say(toastWarn, locales.KeysNoMark(string(r)))
			}
		}
		return
	}
	n := max(k.count, 1)
	if r == 'd' || r == 'y' {
		if a.vimOn() {
			k.pending = string(r)
			return
		}
	}
	if a.vimOn() && !(r >= '0' && r <= '9' && (r != '0' || k.count > 0)) && a.vimKey(r, n) {
		k.count = 0
		return
	}
	switch {
	case r >= '1' && r <= '9' || r == '0' && k.count > 0:
		k.count = k.count*10 + int(r-'0')
		return
	case r == 'j' || r == kbDown:
		a.kbMove(n)
	case r == 'k' || r == kbUp:
		a.kbMove(-n)
	case r == 'G':
		a.kbTo(true)
	case r == 'g' || r == 'm' || r == '\'':
		k.pending = string(r)
		return
	case r == 'l' || r == kbEnter:
		if !a.kbOpen() && a.page != nil && !a.noPage() {
			a.kbLeave()
		}
	case r == 'h':
		a.step(-1)
	case r == kbBack:
		a.step(-1)
	case r == kbTab:
		a.step(1)
	case r == '.': // again, what was done last
		if l := k.again; l.r != 0 {
			a.kbRun(l.r, l.n, l.pair)
		}
		k.count = 0
		return
	case r == 'i' || r == 'a':
		if a.page != nil && !a.noPage() {
			a.kbLeave()
		}
	case r == 'V':
		if a.page != nil && !a.noPage() {
			a.startBlockSel()
		}
	case r == 'f':
		k.hints, k.typed = true, ""
	case r == ' ':
		k.which = true
		return
	case r == '[':
		a.stepDay(-n)
	case r == ']':
		a.stepDay(n)
	case r == '/':
		a.openFind()
	case r == ':':
		a.pal = palette{open: true, query: ":"}
	case r == '?':
		k.help = true
	case r == 'u':
		if a.doc != nil && a.page != nil && !a.noPage() {
			a.doc.Undo()
		}
	case r == 'p':
		a.openCommands()
	case r == 'o':
		a.openPalette()
	case r == 's':
		a.openSettings()
	case r == 't':
		a.openTasks()
	case r == 'b':
		a.toggleSidebar()
	case r == 'n':
		if c := a.currentColl(); c != nil {
			a.newPage(c, nil)
		}
	case r == 'w':
		a.nextPane()
	case r == kbRedo:
		if a.doc != nil && a.page != nil && !a.noPage() {
			a.doc.Redo()
		}
	}
	k.count = 0
}

// the keys that are not letters, as the runes they are given as
const (
	kbDown  = rune(0x2193)
	kbUp    = rune(0x2191)
	kbEnter = rune('\r')
	kbLeft  = rune(0x2190)
	kbRight = rune(0x2192)
	kbRedo  = rune(0x12) // ⌃R
	kbBack  = rune(0x0f) // ⌃O: back through the places visited
	kbTab   = rune('\t') // Tab, which is ⌃I: forward
)

var kbDigitKeys = []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}

var kbLetterKeys = []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM,
	ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}

// kbLetter reads the key pressed this frame as a rune: a letter, a digit, a sign, or one of the keys of kb*.
func kbLetter(c *ui.Context) (rune, bool) {
	if c.Shortcut(0, ui.KeyEscape) {
		return 0x1b, true
	}
	if c.Shortcut(0, ui.KeyDown) {
		return kbDown, true
	}
	if c.Shortcut(0, ui.KeyUp) {
		return kbUp, true
	}
	if c.Shortcut(0, ui.KeyLeft) {
		return kbLeft, true
	}
	if c.Shortcut(0, ui.KeyRight) {
		return kbRight, true
	}
	if c.Shortcut(0, ui.KeyEnter) {
		return kbEnter, true
	}
	if c.Shortcut(0, ui.KeySpace) {
		return ' ', true
	}
	if c.Shortcut(ui.Ctrl, ui.KeyO) {
		return kbBack, true
	}
	if c.Shortcut(0, ui.KeyTab) {
		return kbTab, true
	}
	if c.Shortcut(0, ui.KeyPeriod) {
		return '.', true
	}
	if c.Shortcut(ui.Ctrl, ui.KeyR) {
		return kbRedo, true
	}
	if c.Shortcut(0, ui.KeySlash) {
		return '/', true
	}
	if c.Shortcut(ui.Shift, ui.KeySlash) {
		return '?', true
	}
	if c.Shortcut(ui.Shift, ui.KeySemicolon) {
		return ':', true
	}
	if c.Shortcut(ui.Shift, ui.Key4) {
		return '$', true
	}
	if c.Shortcut(0, ui.KeyQuote) {
		return '\'', true
	}
	if c.Shortcut(0, ui.KeyBracketLeft) {
		return '[', true
	}
	if c.Shortcut(0, ui.KeyBracketRight) {
		return ']', true
	}
	for i, key := range kbDigitKeys {
		if c.Shortcut(0, key) {
			return rune('0' + i), true
		}
	}
	for i, key := range kbLetterKeys {
		if c.Shortcut(0, key) {
			return rune('a' + i), true
		}
		if c.Shortcut(ui.Shift, key) {
			return rune('A' + i), true
		}
	}
	return 0, false
}

// ---- letters on what can be pressed ----

const hintLetters = "asdfjklgh"

// hintLabels are the labels of n targets: one letter while there are few, two when there are more.
func hintLabels(n int) []string {
	out := make([]string, 0, n)
	if n <= len(hintLetters) {
		for i := 0; i < n; i++ {
			out = append(out, string(hintLetters[i]))
		}
		return out
	}
	for _, x := range hintLetters {
		for _, y := range hintLetters {
			out = append(out, string(x)+string(y))
			if len(out) == n {
				return out
			}
		}
	}
	return out // more than eighty targets: the rest have none
}

// kbHints are the targets that can be shown letters, in the window, in order.
func (a *app) kbHints() []kbTarget {
	var out []kbTarget
	w, h := a.winW, a.winH
	for _, t := range a.kb.last {
		if t.act == nil || t.r.W <= 0 || t.r.H <= 0 {
			continue
		}
		if w > 0 && (t.r.X > w || t.r.Y > h || t.r.X+t.r.W < 0 || t.r.Y+t.r.H < 0) {
			continue
		}
		out = append(out, t)
	}
	labels := hintLabels(len(out))
	for i := range out {
		if i < len(labels) {
			out[i].label = labels[i]
		}
	}
	return out
}

// kbHintKey takes a letter of a label.
func (a *app) kbHintKey(r rune) {
	k := &a.kb
	if r < 'a' || r > 'z' {
		return
	}
	k.typed += string(r)
	targets := a.kbHints()
	var match []kbTarget
	for _, t := range targets {
		if t.label == k.typed {
			k.hints, k.typed = false, ""
			t.act()
			return
		}
		if strings.HasPrefix(t.label, k.typed) {
			match = append(match, t)
		}
	}
	if len(match) == 0 {
		k.hints, k.typed = false, ""
	}
}

// kbView draws what the keys show: the letters, the leader's menu, the help.
func (a *app) kbView(c *ui.Context) {
	k := &a.kb
	if w, h := c.Size(); w > 0 {
		a.winW, a.winH = w, h
	}
	cl := ryugo.From(c).Colors
	if k.normal && !k.hints && !k.which && !k.help && !a.modal() && !a.pal.open {
		ui.Overlay(c, func() { // a small sign in the corner: the keys are commands
			ui.Row(c.Key("kb-sign")).Absolute().Right(14).Bottom(34).Padding(3, 9).Radius(rad(7)).Background(cl.Accent.Alpha(0.9)).Children(func() {
				ui.Text(c, "NORMAL"+map[bool]string{true: " " + strconv.Itoa(k.count), false: ""}[k.count > 0]+map[bool]string{true: " " + k.pending, false: ""}[k.pending != ""]).
					FontSize(11).FontWeight(700).TextColor(cl.OnAccent)
			})
		})
	}
	if k.hints {
		targets := a.kbHints()
		ui.Overlay(c, func() {
			for i, t := range targets {
				if t.label == "" || !strings.HasPrefix(t.label, k.typed) {
					continue
				}
				ui.Row(c.Key("hint:"+strconv.Itoa(i))).Absolute().Left(max(t.r.X, 0)).Top(max(t.r.Y, 0)).Padding(1, 5).Radius(rad(5)).
					Background(cl.Warn).Border(1, cl.Text.Alpha(0.5)).Children(func() {
					ui.Text(c, strings.ToUpper(t.label)).FontSize(12).FontWeight(750).TextColor(ui.Hex("#1b1b1f")).Font(ryugo.FontMono)
				})
			}
		})
	}
	if k.which {
		ui.Overlay(c, func() {
			ui.Column(c.Key("which")).Absolute().Right(18).Bottom(60).Padding(10, 14).Gap(4).Radius(rad(12)).Background(cl.Card).Border(1, cl.Hairline).Children(func() {
				ui.Text(c, locales.KeysSpace()).FontSize(12).FontWeight(700).TextColor(cl.Muted)
				for _, l := range leaderKeys {
					ui.Row(c.Key("wk:" + string(l.key))).Gap(10).Children(func() {
						ui.Text(c, string(l.key)).FontSize(13).FontWeight(700).Font(ryugo.FontMono).TextColor(cl.Accent).Width(14)
						ui.Text(c, l.name).FontSize(13).TextColor(cl.Text)
					})
				}
			})
		})
	}
	a.helpView(c)
}

// focusTree puts the keys in the sidebar's tree, on the row of what is shown.
func (a *app) focusTree() {
	a.kb.normal = false
	if a.settings.SidebarClosed {
		a.toggleSidebar()
	}
	switch {
	case a.page != nil && !a.noPage():
		a.treeNav, a.treeFocus = true, any(a.page)
	case a.coll != nil:
		a.treeNav, a.treeFocus = true, any(a.coll)
	case len(a.lib.Collections) > 0:
		a.treeNav, a.treeFocus = true, any(a.lib.Collections[0])
	}
}

// colonItems are the lines of the command line (:): a number goes to that paragraph; w writes, q goes
// back, wq both, e opens a page, tema/theme and set name=value change a setting, help shows the shortcuts.
func (a *app) colonItems(q string) []palItem {
	q = strings.TrimSpace(q)
	hasPage := a.page != nil && a.doc != nil && !a.noPage()
	item := func(title, sub string, run func()) palItem {
		return palItem{title: title, sub: sub, glyph: iconPage, run: run}
	}
	if q == "" {
		out := []palItem{{title: locales.PaletteLineHint(), glyph: iconPage, off: locales.PaletteLineHelp()}}
		return out
	}
	if n, err := strconv.Atoi(q); err == nil {
		if !hasPage {
			return []palItem{{title: locales.PaletteGoToLine(), sub: locales.PaletteNoPageOpen(), glyph: iconPage, off: locales.PaletteOpenPageForLine()}}
		}
		if n < 1 {
			return []palItem{{title: locales.PaletteParagraphFromOne(), glyph: iconPage, off: locales.PaletteTypeNumber()}}
		}
		total := countParagraphs(a.doc.Text())
		to := min(n, total)
		return []palItem{item(locales.PaletteGoToParagraph(to), locales.PaletteOfParagraphs(total), func() { a.goToParagraph(to) })}
	}
	cmd, arg, _ := strings.Cut(q, " ")
	arg = strings.TrimSpace(arg)
	switch strings.ToLower(cmd) {
	case "w", "write":
		return []palItem{item(locales.PaletteSave(), "⌘S", a.saveNow)}
	case "q", "quit":
		return []palItem{item(locales.PaletteCloseBack(), "⌘[", func() { a.step(-1) })}
	case "wq", "x":
		return []palItem{item(locales.PaletteSaveBack(), "", func() { a.saveNow(); a.step(-1) })}
	case "help", "h":
		return []palItem{item(locales.PaletteShortcuts(), "?", func() { a.kb.help = true })}
	case "e", "edit", "open":
		var out []palItem
		for _, it := range a.places() {
			if it.to.kind == "tag" {
				continue
			}
			if arg == "" || match(arg, it.title) >= 0 {
				it.score = match(arg, it.title)
				out = append(out, it)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
		if len(out) > 12 {
			out = out[:12]
		}
		if len(out) == 0 {
			return []palItem{{title: locales.PaletteNoSuchPage(arg), glyph: iconPage, off: locales.PaletteTryAnotherName()}}
		}
		return out
	case "tema", "theme":
		var out []palItem
		for _, it := range a.commands() {
			if strings.HasPrefix(it.id, "common_theme: ") && (arg == "" || match(arg, names(strings.TrimPrefix(it.id, "common_theme: "))) >= 0 || match(arg, it.title) >= 0) {
				out = append(out, it)
			}
		}
		if len(out) == 0 {
			return []palItem{{title: locales.PaletteNoSuchTheme(arg), glyph: iconPage, off: locales.PaletteThemeNames()}}
		}
		return out
	case "set", "se":
		name, val, ok := strings.Cut(arg, "=")
		if !ok {
			name, val, ok = strings.Cut(arg, " ")
		}
		name, val = strings.TrimSpace(name), strings.TrimSpace(val)
		if !ok || name == "" || val == "" {
			return []palItem{{title: locales.PaletteSetUsage(), sub: locales.PaletteSetExample(), glyph: iconPage, off: locales.PaletteSetHelp()}}
		}
		return a.setItems(name, val)
	}
	return []palItem{{title: locales.PaletteUnknownCommand(cmd), sub: locales.PaletteUnknownHint(), glyph: iconPage, off: locales.PaletteUnknown()}}
}

// vimNames are the names of Vim's options that mean a setting here.
var vimNames = map[string]string{
	"fontsize": "yazı boyutu", "fs": "yazı boyutu", "lineheight": "satır aralığı", "lh": "satır aralığı", "width": "sayfa genişliği",
	"textwidth": "sayfa genişliği", "autosave": "otomatik kayıt", "list": "boşlukları göster", "spaces": "boşlukları göster",
	"tabs": "sekmeleri göster", "ligatures": "ligatürler", "indent": "ilk satır girintisi", "sidebar": "sidebar3", "mcp": "mcp sunucusu",
	"stats": "istatistik toplama", "backup": "günlük yedek", "jelly": "jöle efekti", "wobble": "parıltı", "cover": "otomatik kapak",
	"weight": "yazı kalınlığı", "corner": "köşe yuvarlaklığı", "delay": "kayıt gecikmesi",
}

// setItems is :set name=value: a number sets a number, on/off (aç/kapat, 1/0, yes/no) a switch.
func (a *app) setItems(name, val string) []palItem {
	want := name
	if v, ok := vimNames[strings.ToLower(name)]; ok {
		want = v
	}
	for _, d := range settingDefs() {
		d := d
		if match(want, names(d.name)) < 200 && match(want, d.alias) < 200 { // every word of it, not letters in order: set nope=1 is no setting
			continue
		}
		switch d.kind {
		case setNumber:
			v, err := strconv.ParseFloat(strings.ReplaceAll(val, ",", "."), 64)
			if err != nil {
				continue
			}
			to, cur := d.clamp(v), d.val(a)
			return []palItem{{title: locales.SettingsSetTo(msg(d.name), d.fmt(to)), sub: locales.PaletteSetting(), kind: palSetting, id: d.name, glyph: iconSettings,
				run: a.undoable(locales.SettingsChanged(msg(d.name), d.fmt(to)), func() { d.put(a, cur) }, func() { d.put(a, to) })}}
		case setToggle:
			on, ok := map[string]bool{"on": true, "off": false, "1": true, "0": false, "yes": true, "no": false, "aç": true, "kapat": false, "açık": true, "kapalı": false, "true": true, "false": false}[strings.ToLower(val)]
			if !ok {
				continue
			}
			was := d.on(a)
			state := map[bool]string{true: locales.SettingsOn(), false: locales.SettingsOff()}[on]
			return []palItem{{title: locales.SettingsSetTo(msg(d.name), state), sub: locales.PaletteSetting(), kind: palSetting, id: d.name, glyph: iconSettings,
				run: a.undoable(locales.SettingsChanged(msg(d.name), state), func() { d.flip(a, was) }, func() { d.flip(a, on) })}}
		}
	}
	return []palItem{{title: locales.PaletteNoSuchSetting(name, val), glyph: iconPage, off: locales.PaletteSettingMismatch()}}
}

// kbRun does again what . asks for: a motion of Vim that edits, or a pair like dd.
func (a *app) kbRun(r rune, n int, pair string) {
	if pair != "" {
		a.vimPair(rune(pair[0]), rune(pair[1]))
		return
	}
	a.vimKey(r, max(n, 1))
}

// followAtCaret opens the link, tag or chip the caret is on.
func (a *app) followAtCaret() {
	if a.page == nil || a.noPage() || a.caretWin.W <= 0 && a.caretWin.H <= 0 {
		a.say(toastWarn, locales.ThereIsNoLinkUnderThe())
		return
	}
	x, y := a.caretWin.X+1, a.caretWin.Y+a.caretWin.H/2
	if key, ok := a.tagAt(x, y); ok {
		a.openTag(key)
		return
	}
	if url, ok := a.linkAt(x, y); ok {
		a.openLink(url)
		return
	}
	if a.chipAt(x, y) != nil {
		a.clickChip(x, y)
		return
	}
	a.say(toastWarn, locales.ThereIsNoLinkTagOr())
}
