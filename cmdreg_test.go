package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Every field of Settings is a command, or has a reason not to be one: a setting added without a
// command fails here.
func TestEverySettingIsACommand(t *testing.T) {
	covered := map[string]bool{}
	for _, d := range settingDefs() {
		for _, f := range d.fields {
			covered[f] = true
		}
	}
	rt := reflect.TypeOf(Settings{})
	for i := 0; i < rt.NumField(); i++ {
		key := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		if !covered[key] && settingsApart[key] == "" {
			t.Errorf("setting %q has no command, and no reason not to", key)
		}
	}
	for f := range covered { // and a command for nothing is wrong, too
		found := false
		for i := 0; i < rt.NumField(); i++ {
			if strings.Split(rt.Field(i).Tag.Get("json"), ",")[0] == f {
				found = true
			}
		}
		if !found {
			t.Errorf("command for %q, which is no setting", f)
		}
	}
}

// Each setting gives at least one line of the palette, each with an undo.
func TestSettingCommandsListed(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	items := a.commands()
	for _, d := range settingDefs() {
		n := 0
		for _, it := range items {
			if strings.HasPrefix(it.title, msg(d.name)) && it.sub == "Ayar" { // a name written in English is shown in the window's language
				n++
			}
		}
		if n == 0 {
			t.Errorf("%s: no command", d.name)
		}
	}
}

func runCmd(t *testing.T, a *app, query, title string) {
	t.Helper()
	for _, it := range a.paletteItems(query) {
		if it.title == title {
			a.runItem(it)
			return
		}
	}
	var got []string
	for _, it := range a.paletteItems(query) {
		got = append(got, it.title)
	}
	t.Fatalf("no %q for %q: %v", title, query, got)
}

// "yazı boyutu 18" sets it at once, and the notice takes it back.
func TestArgumentCommand(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	base := a.fontSize()
	items := a.paletteItems(">yazı boyutu 18")
	if len(items) == 0 || !strings.HasPrefix(items[0].title, "Yazı boyutu → 18") {
		t.Fatalf("first: %+v", items[0])
	}
	a.runItem(items[0])
	if a.fontSize() != 18 {
		t.Fatalf("size %v", a.fontSize())
	}
	if a.noticeUndo == nil || !strings.Contains(a.notice, "18") {
		t.Fatalf("no undo: %q", a.notice)
	}
	a.noticeUndo()
	if a.fontSize() != base {
		t.Fatalf("not taken back: %v", a.fontSize())
	}
	// out of range: the nearest, and it says so
	items = a.paletteItems(">satır aralığı 9")
	if !strings.Contains(items[0].title, "sınır") {
		t.Fatalf("no limit said: %q", items[0].title)
	}
	a.runItem(items[0])
	if a.lineHeight() != 2.2 {
		t.Fatalf("line height %v", a.lineHeight())
	}
	// a comma is a point
	a.runItem(a.paletteItems(">satır aralığı 1,6")[0])
	if got := a.lineHeight(); got < 1.59 || got > 1.61 {
		t.Fatalf("1,6 gave %v", got)
	}
}

// A toggle is turned over by its command, and back by the notice.
func TestToggleCommand(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	runCmd(t, a, ">otomatik kayıt", "Otomatik kayıt: kapat")
	if !a.settings.NoAutoSave {
		t.Fatal("not off")
	}
	a.noticeUndo()
	if a.settings.NoAutoSave {
		t.Fatal("not back on")
	}
	runCmd(t, a, ">boşlukları göster", "Boşlukları göster: aç")
	if !a.settings.ShowSpaces {
		t.Fatal("spaces not shown")
	}
	runCmd(t, a, ">boşlukları göster", "Boşlukları göster: kapat")
	if a.settings.ShowSpaces {
		t.Fatal("spaces still shown")
	}
}

// Other words find a command: in the other language, too.
func TestCommandAliases(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	for q, want := range map[string]string{"theme": "Tema: Koyu", "autosave": "Otomatik kayıt", "font size": "Yazı boyutu", "whitespace": "Boşlukları göster"} {
		found := false
		for _, it := range a.paletteItems(">" + q) {
			if strings.HasPrefix(it.title, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q does not find %q", q, want)
		}
	}
}

// The commands used last come first, and "!" lists them.
func TestRecentCommands(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	runCmd(t, a, ">görevler", "Görevler") // with nothing typed, they are in their groups
	runCmd(t, a, ">çöp", "Çöp")
	items := a.paletteItems(">")
	if items[0].title != "Çöp" || items[1].title != "Görevler" || items[0].group != "Son kullanılanlar" {
		t.Fatalf("order: %q %q (%q)", items[0].title, items[1].title, items[0].group)
	}
	last := a.paletteItems("!")
	if len(last) < 2 || last[0].title != "Çöp" {
		t.Fatalf("!: %+v", last)
	}
	runCmd(t, a, ">", "Görevler") // now among the last used
	if a.paletteItems(">")[0].title != "Görevler" {
		t.Fatal("used again, not first")
	}
}

// The modes: # tags, : a paragraph, / a word in the page, ? help.
func TestPaletteModes(t *testing.T) {
	a, tt := newTestApp(t, "bir\n\niki #meyve\n\nüç\n\ndört\n")
	tt.Advance(50 * time.Millisecond)
	if got := a.paletteItems("#mey"); len(got) == 0 || got[0].title != "#meyve" {
		t.Fatalf("#: %+v", got)
	}
	items := a.paletteItems(":3")
	if len(items) != 1 || items[0].run == nil {
		t.Fatalf(":3 %+v", items)
	}
	a.runItem(items[0])
	_, caret := a.doc.Selection()
	if got := a.doc.Text().Slice(caret, caret+2).String(); got != "üç" {
		t.Fatalf("went to %q", got)
	}
	if len(a.paletteItems(":")) != 1 || a.paletteItems(":")[0].off == "" {
		t.Fatal("no hint for a missing number")
	}
	if got := a.paletteItems(":99"); !strings.Contains(got[0].title, "paragraf") {
		t.Fatalf("past the end: %+v", got)
	}
	a.runItem(a.paletteItems("/dört")[0])
	if !a.find.open || a.find.query != "dört" {
		t.Fatalf("find: %+v", a.find)
	}
	help := a.paletteItems("?")
	if len(help) != len(palModes) {
		t.Fatalf("help %d", len(help))
	}
	// a line that is off says why and stays
	a.pal.open = true
	for _, it := range a.paletteItems("!") {
		_ = it
	}
	a.notice = ""
	a.runItem(palItem{title: "x", off: "neden"})
	if a.notice != "neden" || !a.pal.open {
		t.Fatalf("off: %q open %v", a.notice, a.pal.open)
	}
}

// A sign typed first makes the mode: ⌘⇧P opens the commands, too.
func TestPaletteSignsAndShiftP(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	tt.Advance(100 * time.Millisecond)
	if !a.pal.open || !a.palCmds() {
		t.Fatalf("⌘⇧P: open %v query %q", a.pal.open, a.pal.query)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	tt.SetSize(1000, 600)
	tt.Frame()
	a.openPalette()
	tt.Frame()
	typeEach(tt, "#")
	tt.Frame()
	tt.Frame()
	if palModeOf(a.pal.query) != '#' || a.pal.text != "" {
		t.Fatalf("# not taken as a mode: query %q text %q", a.pal.query, a.pal.text)
	}
}
