package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// kbd drives the app with the keyboard alone: it has no pointer to move, and a test written with it is
// a proof that the thing can be done without one.
type kbd struct {
	t  *testing.T
	a  *app
	tt *ui.Tester
}

func newKbd(t *testing.T, src string) *kbd {
	a, tt := newTestApp(t, src)
	a.settings.EscNormal = true // the keyboard mode is what these test: Esc enters it
	tt.SetSize(1100, 800)
	tt.Frame()
	return &kbd{t, a, tt}
}

var kbNamed = map[string]ui.Key{
	"esc": ui.KeyEscape, "enter": ui.KeyEnter, "down": ui.KeyDown, "up": ui.KeyUp, "left": ui.KeyLeft, "right": ui.KeyRight,
	"space": ui.KeySpace, "tab": ui.KeyTab, "backspace": ui.KeyBackspace, "/": ui.KeySlash, "'": ui.KeyQuote, ",": ui.KeyComma,
	"=": ui.KeyEqual, ".": ui.KeyPeriod, "-": ui.KeyMinus, "[": ui.KeyBracketLeft,
}

// press sends keys: "esc", "j", "G" (shift), "cmd+p", "ctrl+r", ":" ...
func (k *kbd) press(keys ...string) {
	k.t.Helper()
	for _, s := range keys {
		var mods ui.Modifiers
		for {
			switch {
			case strings.HasPrefix(s, "cmd+"):
				mods |= ui.Cmd
				s = s[4:]
				continue
			case strings.HasPrefix(s, "ctrl+"):
				mods |= ui.Ctrl
				s = s[5:]
				continue
			case strings.HasPrefix(s, "shift+"):
				mods |= ui.Shift
				s = s[6:]
				continue
			}
			break
		}
		var key ui.Key
		switch {
		case s == "$":
			key, mods = ui.Key4, mods|ui.Shift
		case s == ":":
			key, mods = ui.KeySemicolon, mods|ui.Shift
		case s == "?":
			key, mods = ui.KeySlash, mods|ui.Shift
		case kbNamed[s] != 0 || s == "enter":
			key = kbNamed[s]
		case len(s) == 1 && s[0] >= 'a' && s[0] <= 'z':
			key = kbLetterKeys[s[0]-'a']
		case len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z':
			key, mods = kbLetterKeys[s[0]-'A'], mods|ui.Shift
		case len(s) == 1 && s[0] >= '0' && s[0] <= '9':
			key = kbDigitKeys[s[0]-'0']
		default:
			k.t.Fatalf("unknown key %q", s)
		}
		k.tt.Key(mods, key)
		k.tt.Advance(30 * time.Millisecond)
		k.tt.Frame()
	}
}

// write types text: into the editor or a field that has the focus.
func (k *kbd) write(text string) {
	k.t.Helper()
	k.tt.Type(text)
	k.tt.Advance(30 * time.Millisecond)
	k.tt.Frame()
}

func (k *kbd) frames() {
	k.tt.Advance(80 * time.Millisecond)
	k.tt.Frame()
	k.tt.Advance(80 * time.Millisecond)
}

// Esc leaves the text for the keys, i goes back; and a click anywhere is the text's again.
func TestKbNormalModeEntersAndLeaves(t *testing.T) {
	k := newKbd(t, "bir iki\n")
	k.frames()
	if !k.a.focused {
		t.Fatal("the editor has no focus to start with")
	}
	k.press("esc")
	k.frames()
	if !k.a.kb.normal || k.a.focused {
		t.Fatalf("not in the normal mode: normal %v focused %v", k.a.kb.normal, k.a.focused)
	}
	before := k.a.doc.Text().String()
	k.press("j", "k", "l") // letters, here, are commands: they write nothing
	if k.a.doc.Text().String() != before {
		t.Fatalf("letters were written in the normal mode: %q", k.a.doc.Text().String())
	}
	k.press("i")
	k.frames()
	if k.a.kb.normal || !k.a.focused {
		t.Fatalf("i did not go back to writing: normal %v focused %v", k.a.kb.normal, k.a.focused)
	}
	k.write("x")
	if !strings.Contains(k.a.doc.Text().String(), "x") {
		t.Fatalf("not writing: %q", k.a.doc.Text().String())
	}
}

// j and k move over the rows of a list, a count goes further, gg and G go to the ends, Enter opens.
func TestKbListRows(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	for _, n := range []string{"B", "C", "D", "E"} {
		c.NewPage(nil, n)
	}
	k.a.openCollection(c)
	k.frames()
	k.press("esc")
	k.frames()
	if !k.a.kb.normal {
		t.Fatal("Esc did not enter the normal mode on a list")
	}
	rows := k.a.kb.rows()
	if len(rows) < 4 {
		t.Fatalf("%d rows shown to the keys", len(rows))
	}
	k.press("j")
	first := k.a.kb.selKey
	k.press("j")
	if k.a.kb.selKey == first {
		t.Fatal("j did not move")
	}
	k.press("2", "j")
	if k.a.kb.selKey != rows[min(3, len(rows)-1)].key {
		t.Fatalf("2j: %q, want %q", k.a.kb.selKey, rows[min(3, len(rows)-1)].key)
	}
	k.press("G")
	if k.a.kb.selKey != rows[len(rows)-1].key {
		t.Fatal("G is not the last")
	}
	k.press("g", "g")
	if k.a.kb.selKey != rows[0].key {
		t.Fatal("gg is not the first")
	}
	k.press("j")
	want := k.a.kb.selKey
	k.press("enter")
	k.frames()
	if k.a.coll != nil || k.a.page == nil || !strings.HasSuffix(want, ":"+k.a.page.Dir) && !strings.HasSuffix(want, k.a.page.Dir) {
		t.Fatalf("enter did not open %q: page %v coll %v", want, k.a.page, k.a.coll)
	}
}

// Space is the leader: the next key runs what the menu says.
func TestKbLeader(t *testing.T) {
	k := newKbd(t, "x\n")
	k.press("esc")
	k.press("space")
	k.frames()
	if !k.a.kb.which || !k.tt.HasText("Ayarlar") {
		t.Fatalf("no menu: %v", k.tt.Texts())
	}
	k.press("s")
	k.frames()
	if !k.a.settingsOpen {
		t.Fatal("Space s did not open the settings")
	}
	// Esc goes back
	k.press("esc")
	k.frames()
	if k.a.settingsOpen {
		t.Fatal("Esc did not leave the settings")
	}
	k.press("space", "i")
	k.frames()
	if !k.a.statsScreen {
		t.Fatal("Space i: no statistics")
	}
	// a key that is nothing says so
	k.press("space", "z")
	if !k.tt.HasText("Space z: böyle bir kısayol yok") {
		t.Fatalf("no word: %v", k.tt.Texts())
	}
}

// m a marks the place, ' a goes back to it.
func TestKbMarks(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	p2, _ := c.NewPage(nil, "Öteki")
	first := k.a.page
	k.press("esc")
	k.press("m", "a")
	if _, ok := k.a.kb.marks['a']; !ok {
		t.Fatal("no mark")
	}
	k.a.open(p2)
	k.frames()
	k.press("esc")
	k.press("'", "a")
	k.frames()
	if k.a.page != first {
		t.Fatalf("' a went to %v", k.a.page.Title)
	}
	k.press("'", "q")
	if !k.tt.HasText(locales.KeysNoMark("q")) {
		t.Fatalf("a missing mark did not say: %v", k.tt.Texts())
	}
}

// f puts letters on what can be pressed; a letter presses it.
func TestKbHints(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	c.NewPage(nil, "Hedef")
	k.a.openCollection(c)
	k.frames()
	k.press("esc")
	k.frames()
	k.press("f")
	k.frames()
	if !k.a.kb.hints {
		t.Fatal("no hints")
	}
	targets := k.a.kbHints()
	if len(targets) < 3 || targets[0].label == "" {
		t.Fatalf("%d targets", len(targets))
	}
	var tree kbTarget
	for _, tg := range targets {
		if strings.HasPrefix(tg.key, "tree:") && strings.Contains(tg.key, "Hedef") || tg.key == "p:"+c.All()[len(c.All())-1].Dir {
			tree = tg
		}
	}
	if tree.key == "" {
		t.Fatalf("the page is not among the targets: %+v", targets)
	}
	for _, r := range tree.label {
		k.press(string(r))
	}
	k.frames()
	if k.a.kb.hints {
		t.Fatal("hints still shown")
	}
	if k.a.page == nil || k.a.coll != nil {
		t.Fatalf("the letters did not open the page: page %v coll %v", k.a.page, k.a.coll)
	}
	// a letter that is no label cancels
	k.press("esc")
	k.press("f")
	k.press("z")
	if k.a.kb.hints {
		t.Fatal("a wrong letter left the hints on")
	}
}

func TestKbHintLabels(t *testing.T) {
	l := hintLabels(5)
	if len(l) != 5 || len(l[0]) != 1 {
		t.Fatalf("%v", l)
	}
	l = hintLabels(30)
	seen := map[string]bool{}
	for _, s := range l {
		if len(s) != 2 || seen[s] {
			t.Fatalf("labels %v", l)
		}
		seen[s] = true
	}
	// no label is the start of another: a letter is enough or two are needed
	for _, x := range l {
		for _, y := range l {
			if x != y && strings.HasPrefix(y, x) {
				t.Fatalf("%q starts %q", y, x)
			}
		}
	}
}

// The help lists the shortcuts, with the commands' own.
func TestKbHelp(t *testing.T) {
	k := newKbd(t, "x\n")
	k.press("esc")
	k.press("?")
	k.frames()
	if !k.a.kb.help || !k.tt.HasText("Kısayollar") || !k.tt.HasText("Normal kip (Esc ile girilir, i ile çıkılır)") {
		t.Fatalf("%v", k.tt.Texts())
	}
	k.press("esc")
	k.frames()
	if k.a.kb.help {
		t.Fatal("Esc did not close the help")
	}
}

// The command line: :12 goes to a paragraph, :set fontsize=20, :tema koyu, :w, :e name.
func TestKbCommandLine(t *testing.T) {
	k := newKbd(t, "bir\n\niki\n\nüç\n\ndört\n")
	k.press("esc")
	k.press(":")
	k.frames()
	if !k.a.pal.open || palModeOf(k.a.pal.query) != ':' {
		t.Fatalf("no command line: open %v query %q", k.a.pal.open, k.a.pal.query)
	}
	k.write("3")
	k.frames()
	k.press("enter")
	k.frames()
	_, caret := k.a.doc.Selection()
	if got := k.a.doc.Text().Slice(caret, caret+2).String(); got != "üç" {
		t.Fatalf(":3 went to %q", got)
	}
	run := func(line string) {
		t.Helper()
		items := k.a.paletteItems(":" + line)
		if len(items) == 0 || items[0].off != "" {
			t.Fatalf(":%s: %+v", line, items)
		}
		k.a.runItem(items[0])
	}
	run("set fontsize=20")
	if k.a.fontSize() != 20 {
		t.Fatalf("fontsize %v", k.a.fontSize())
	}
	run("set autosave=off")
	if !k.a.settings.NoAutoSave {
		t.Fatal("autosave not off")
	}
	run("set autosave=on")
	if k.a.settings.NoAutoSave {
		t.Fatal("autosave not on")
	}
	run("tema koyu")
	if k.a.settings.Theme != 2 {
		t.Fatalf("theme %d", k.a.settings.Theme)
	}
	k.a.dirty = true
	run("w")
	if k.a.dirty {
		t.Fatal(":w did not save")
	}
	run("e " + k.a.page.Title)
	for _, bad := range []string{"zzz", "set nope=1", "set fontsize=abc", "e hiçbirsayfaadı"} {
		if it := k.a.paletteItems(":" + bad); len(it) == 0 || it[0].off == "" {
			t.Errorf(":%s should say why not: %+v", bad, it)
		}
	}
}

// ⌘1 puts the keys in the tree, ⌘3 in the text.
func TestKbRegions(t *testing.T) {
	k := newKbd(t, "x\n")
	k.press("cmd+1")
	k.frames()
	if k.a.focused {
		t.Fatal("the text kept the focus after ⌘1")
	}
	k.press("cmd+3")
	k.frames()
	if !k.a.focused {
		t.Fatal("⌘3 did not go to the text")
	}
}

// Everything the acceptance of the keyboard asks, with no pointer: a new page, writing, a sub-page, a
// view of the collection, a setting, the theme, the page to the trash, undo, search, export.
func TestKbOnlyScenario(t *testing.T) {
	k := newKbd(t, "ilk\n")
	a := k.a
	cmd := func(query, title string) {
		t.Helper()
		k.press("cmd+p")
		k.frames()
		k.write(query)
		k.frames()
		var got []string
		items := a.paletteItems(a.pal.query)
		for _, it := range items {
			got = append(got, it.title)
			if it.title == title {
				// ↓ to it, Enter
				for i := range items {
					if items[i].title == title {
						for j := 0; j < i; j++ {
							k.press("down")
						}
						break
					}
				}
				k.press("enter")
				k.frames()
				return
			}
		}
		t.Fatalf("%q does not give %q: %v", query, title, got)
	}
	pages := len(a.page.Coll.All())
	cmd("yeni sayfa", "Yeni "+pageNoun(a.page.Coll)+": "+a.page.Coll.Name)
	if len(a.page.Coll.All()) != pages+1 {
		t.Fatal("no new page")
	}
	k.press("enter") // the new page's title has the focus: Enter goes to the text
	k.frames()
	k.write("Klavyeyle yazılan satır")
	k.frames()
	if !strings.Contains(a.doc.Text().String(), "Klavyeyle yazılan satır") {
		t.Fatalf("not written: %q", a.doc.Text().String())
	}
	if a.page.Coll.SubPages {
		parent := a.page
		cmd("alt sayfa", "Alt sayfa ekle: "+parent.Title)
		if len(parent.Children) != 1 {
			t.Fatal("no sub-page")
		}
		a.open(parent)
		k.frames()
	}
	cmd("tema koyu", "Tema: Koyu")
	if a.settings.Theme != 2 {
		t.Fatal("theme not changed")
	}
	cmd("yazı boyutu 19", "Yazı boyutu → 19.0 px")
	if a.fontSize() != 19 {
		t.Fatalf("size %v", a.fontSize())
	}
	cmd("otomatik kayıt", "Otomatik kayıt: kapat")
	cmd("geri al", "Geri al")
	p := a.page
	cmd("çöpe taşı", "Çöpe taşı: "+p.Title)
	if a.page == p {
		t.Fatal("the page was not thrown away")
	}
	k.press("cmd+k")
	k.frames()
	k.write("ilk")
	k.frames()
	k.press("enter")
	k.frames()
	if a.page == nil || !strings.Contains(a.page.Title, "") {
		t.Fatal("search did not open a page")
	}
	cmd("markdown olarak dışa aktar (zip)", "Markdown olarak dışa aktar (zip)")
}

func (k *kbd) text() string { return k.a.doc.Text().String() }

func (k *kbd) caret() int { _, c := k.a.doc.Selection(); return c }

// With the setting, the page's normal mode moves and edits as Vim does.
func TestKbVimMotions(t *testing.T) {
	k := newKbd(t, "bir iki üç\n\ndört beş\n\naltı\n")
	k.a.settings.VimKeys = true
	k.a.doc.SetSelection(0, 0)
	k.frames()
	k.press("esc")
	k.frames()
	k.press("0")
	if k.caret() != 0 {
		t.Fatalf("0: %d", k.caret())
	}
	k.press("l", "l")
	if k.caret() != 2 {
		t.Fatalf("ll: %d", k.caret())
	}
	k.press("h")
	if k.caret() != 1 {
		t.Fatalf("h: %d", k.caret())
	}
	k.press("0", "w")
	if k.caret() != 4 { // iki
		t.Fatalf("w: %d in %q", k.caret(), k.text())
	}
	k.press("w")
	if k.caret() != 8 { // üç
		t.Fatalf("ww: %d", k.caret())
	}
	k.press("b")
	if k.caret() != 4 {
		t.Fatalf("b: %d", k.caret())
	}
	k.press("e")
	if k.caret() != 6 {
		t.Fatalf("e: %d", k.caret())
	}
	k.press("$")
	if k.caret() != 10 {
		t.Fatalf("$: %d", k.caret())
	}
	k.press("j")
	if got := k.text()[k.caretByte():]; !strings.HasPrefix(got, "") || k.caret() < 11 {
		t.Fatalf("j: %d", k.caret())
	}
	k.press("2", "j")
	if k.a.doc.Text().ParagraphStyleAt(0).Heading != 0 {
		t.Fatal("style")
	}
	k.press("g", "g")
	if k.caret() != 0 {
		t.Fatalf("gg: %d", k.caret())
	}
	k.press("G")
	if k.caret() != len([]rune(k.text())) {
		t.Fatalf("G: %d", k.caret())
	}
	// x takes a letter
	k.press("g", "g", "x")
	if !strings.HasPrefix(k.text(), "ir iki") {
		t.Fatalf("x: %q", k.text())
	}
	// dd takes a paragraph, p puts it back below
	k.press("d", "d")
	if strings.Contains(k.text(), "ir iki") {
		t.Fatalf("dd: %q", k.text())
	}
	k.press("p")
	if !strings.Contains(k.text(), "ir iki") {
		t.Fatalf("p: %q", k.text())
	}
	// yy then p copies
	k.press("y", "y", "p")
	if strings.Count(k.text(), "ir iki") != 2 {
		t.Fatalf("yy p: %q", k.text())
	}
	// u takes the last back
	k.press("u")
	if strings.Count(k.text(), "ir iki") != 1 {
		t.Fatalf("u: %q", k.text())
	}
	// o opens a line below and writes in it
	k.press("o")
	k.frames()
	if k.a.kb.normal || !k.a.focused {
		t.Fatalf("o did not go to writing: normal %v focused %v", k.a.kb.normal, k.a.focused)
	}
	k.write("yeni satır")
	if !strings.Contains(k.text(), "yeni satır") {
		t.Fatalf("not written: %q", k.text())
	}
}

func (k *kbd) caretByte() int { return len(string([]rune(k.text())[:k.caret()])) }

// Without the setting, the normal mode leaves the text alone.
func TestKbVimOff(t *testing.T) {
	k := newKbd(t, "bir iki\n")
	k.a.doc.SetSelection(0, 0)
	k.frames()
	k.press("esc")
	before := k.text()
	k.press("x", "d", "d", "l", "w")
	if k.text() != before || k.caret() != 0 {
		t.Fatalf("the text changed or moved with Vim off: %q %d", k.text(), k.caret())
	}
}

// A count: 3x takes three letters.
func TestKbVimCount(t *testing.T) {
	k := newKbd(t, "abcdefgh\n")
	k.a.settings.VimKeys = true
	k.a.doc.SetSelection(0, 0)
	k.frames()
	k.press("esc")
	k.press("3", "x")
	if !strings.HasPrefix(k.text(), "defgh") {
		t.Fatalf("3x: %q", k.text())
	}
	k.press("1", "0", "l") // ten: to the end
	if k.caret() != 5 {
		t.Fatalf("10l: %d", k.caret())
	}
}

// . does again the last edit; dd . takes two paragraphs; x . takes two letters.
func TestKbRepeat(t *testing.T) {
	k := newKbd(t, "abcdef\n\nsatır bir\n\nsatır iki\n\nson\n")
	k.a.settings.VimKeys = true
	k.a.doc.SetSelection(0, 0)
	k.frames()
	k.press("esc")
	k.press("x", ".")
	if !strings.HasPrefix(k.text(), "cdef") {
		t.Fatalf("x .: %q", k.text())
	}
	k.press("j", "d", "d")
	before := strings.Count(k.text(), "satır")
	k.press(".")
	if strings.Count(k.text(), "satır") >= before {
		t.Fatalf("dd .: %q", k.text())
	}
}

// ⌃O goes back, Tab forward, as ⌃O and ⌃I go through the places visited.
func TestKbJumpList(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	p2, _ := c.NewPage(nil, "İkinci")
	first := k.a.page
	k.a.open(p2)
	k.frames()
	k.press("esc")
	k.press("ctrl+o")
	k.frames()
	if k.a.page != first {
		t.Fatalf("⌃O: %v", k.a.page.Title)
	}
	k.press("esc")
	k.press("tab")
	k.frames()
	if k.a.page != p2 {
		t.Fatalf("Tab: %v", k.a.page.Title)
	}
}

// The ring: the row the keys are on is ringed without the normal mode, when the setting asks.
func TestKbAlwaysRing(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	c.NewPage(nil, "Bir")
	k.a.openCollection(c)
	k.frames()
	k.press("esc")
	k.press("j")
	k.a.settings.AlwaysRing = true
	k.a.kb.normal = false
	k.frames()
	if k.a.kb.selKey == "" {
		t.Fatal("no row chosen")
	}
	if k.a.followAtCaret(); k.a.notice == "" {
		t.Fatal("gf on a collection said nothing")
	}
}

// The board's cards, the calendar's days and the table's rows are rows for the keys.
func TestKbOtherViews(t *testing.T) {
	k := newKbd(t, "x\n")
	c := k.a.page.Coll
	c.NewPage(nil, "Kart bir")
	c.NewPage(nil, "Kart iki")
	for _, v := range c.Views {
		if err := c.SetView(v.ID); err != nil {
			t.Fatal(err)
		}
		k.a.openCollection(c)
		k.frames()
		k.frames()
		k.press("esc")
		if n := len(k.a.kb.rows()); n == 0 {
			t.Errorf("view %q: no rows for the keys", v.Label)
		}
	}
}

// TestSettingsSlashSearch: / on the settings screen puts the keys in their search, from the keys' mode or from no
// field at all, and the / itself is not typed into it.
func TestSettingsSlashSearch(t *testing.T) {
	for _, normal := range []bool{false, true} {
		k := newKbd(t, "x\n")
		if normal { // the keys' mode, then Space s
			k.press("esc", "space", "s")
		} else {
			k.a.openSettings()
		}
		k.frames()
		if !k.a.settingsOpen || k.a.kb.normal != normal {
			t.Fatalf("normal %v: settings %v, keys' mode %v", normal, k.a.settingsOpen, k.a.kb.normal)
		}
		k.tt.TypeKey(0, ui.KeySlash, "/")
		k.frames()
		k.frames()
		if !k.tt.Focused(locales.SettingsSearchSettings()) {
			t.Fatalf("normal %v: / did not focus the search", normal)
		}
		if k.a.find.open || k.a.setSearch != "" {
			t.Fatalf("normal %v: find %v, search %q", normal, k.a.find.open, k.a.setSearch)
		}
		k.write("font")
		if k.a.setSearch != "font" {
			t.Fatalf("normal %v: typed %q", normal, k.a.setSearch)
		}
	}
}

func TestKbVimEmptyLastLineDD(t *testing.T) {
	k := newKbd(t, "abc\n\ndef\n")
	k.a.settings.VimKeys = true
	// Add a trailing newline so the last line is truly empty.
	n := k.a.doc.Text().Len()
	k.a.doc.SetSelection(n, n)
	k.tt.Key(0, ui.KeyEnter)
	k.frames()

	before := k.text()
	beforeLines := strings.Count(before, "\n")
	end := len([]rune(before))
	k.a.doc.SetSelection(end, end)
	k.press("esc")
	k.frames()
	k.press("d", "d")
	got := k.text()
	if !strings.Contains(got, "def") {
		t.Fatalf("dd on empty last line deleted previous line def: %q", got)
	}
	if gotLines := strings.Count(got, "\n"); gotLines != beforeLines-1 {
		t.Fatalf("dd on empty last line did not delete the empty line: before=%q (%d newlines) after=%q (%d newlines)", before, beforeLines, got, gotLines)
	}
}

// TestEscStaysInText: by default Esc in the text does not enter the keyboard mode, so letters typed after it are
// text; with Vim's motions on it does (#492, #207).
func TestEscStaysInText(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 800)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	if a.kb.normal {
		t.Fatal("Esc entered the keyboard mode")
	}
	a.settings.VimKeys = true
	tt.Key(0, ui.KeyEscape)
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	if !a.kb.normal {
		t.Fatal("with Vim's motions Esc did not enter the keyboard mode")
	}
}

// The page of the shortcuts makes its groups once while it is open, the commands' list each, not at every frame,
// and again when it opens next (#204).
func TestHelpGroupsKept(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.kb.help = true
	tt.Frame()
	if len(a.kb.helpKept) == 0 {
		t.Fatal("no groups")
	}
	first := &a.kb.helpKept[0]
	tt.Frame()
	if &a.kb.helpKept[0] != first {
		t.Fatal("made again with the page open")
	}
	a.kb.help = false
	tt.Frame()
	if a.kb.helpKept != nil {
		t.Fatal("kept once closed")
	}
}

// The app's command keys are parsed once, not at each frame where there is no menu bar, and every command with a key
// is among them (#204).
func TestAppKeyedOnce(t *testing.T) {
	a, b := appKeyed(), appKeyed()
	if len(a) == 0 || &a[0] != &b[0] {
		t.Fatal("parsed again")
	}
	n := 0
	for _, c := range appCommands() {
		if _, _, _, ok := parseAccel(c.accel); ok {
			n++
		}
	}
	if len(a) != n {
		t.Fatalf("%d commands with keys, %d parsed", n, len(a))
	}
}
