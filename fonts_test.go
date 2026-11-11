package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// waitFonts waits for the families to be found.
func waitFonts(t *testing.T, a *app) {
	t.Helper()
	a.families()
	for i := 0; i < 400; i++ {
		a.fonts.mu.Lock()
		ok := a.fonts.ready
		a.fonts.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Skip("the system did not list its fonts")
}

// A family that is not installed leaves the default in its place; one that is, goes first.
func TestFontStack(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	waitFonts(t, a)
	if got := a.stackOf("Yok Böyle Bir Yazı Tipi", "sans"); got != "sans" {
		t.Fatalf("missing family kept: %q", got)
	}
	if got := a.stackOf("", "sans"); got != "sans" {
		t.Fatalf("empty: %q", got)
	}
	fam := a.families()
	if len(fam) == 0 {
		t.Skip("no fonts installed")
	}
	if got := a.stackOf(fam[0].Name, "sans"); got != fam[0].Name+", sans" {
		t.Fatalf("installed family: %q", got)
	}
	if got := a.stackOf("a, b", "sans"); got != "sans" {
		t.Fatalf("a list in a name: %q", got)
	}
}

// The editor's font is the one chosen, else the app's; it reaches the editor and the code theme.
func TestFontReachesEditor(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	waitFonts(t, a)
	fam := a.families()
	if len(fam) < 2 {
		t.Skip("no fonts installed")
	}
	a.setFont(fontEditor, fam[0].Name)
	a.setFont(fontMono, fam[1].Name)
	a.settings.NoLigatures, a.settings.TabularNums, a.settings.FontWeight = true, true, 600
	tt.Frame()
	tt.Frame()
	if !strings.HasPrefix(a.font.Family, fam[0].Name) {
		t.Fatalf("editor font %q", a.font.Family)
	}
	if !strings.HasPrefix(a.theme.CodeFont, fam[1].Name) {
		t.Fatalf("code font %q", a.theme.CodeFont)
	}
	if a.font.Weight != 600 || !strings.Contains(a.font.Features, "liga=0") || !strings.Contains(a.font.Features, "tnum") {
		t.Fatalf("weight %d features %q", a.font.Weight, a.font.Features)
	}
	// the app's own font is the window's, for what the editor has none
	a.setFont(fontEditor, "")
	a.setFont(fontApp, fam[1].Name)
	tt.Frame()
	tt.Frame()
	if !strings.HasPrefix(a.font.Family, fam[1].Name) {
		t.Fatalf("editor does not follow the app: %q", a.font.Family)
	}
}

// The families used last and the starred come first in the list; the code list is the monospaced.
func TestFontList(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	waitFonts(t, a)
	fam := a.families()
	if len(fam) < 3 {
		t.Skip("few fonts installed")
	}
	a.toggleFav(fam[2].Name)
	a.setFont(fontEditor, fam[1].Name)
	rows, _ := a.fontRows(fontEditor)
	if rows[0] != fam[2].Name || rows[1] != fam[1].Name {
		t.Fatalf("order %v", rows[:3])
	}
	a.toggleFav(fam[2].Name)
	if a.isFav(fam[2].Name) {
		t.Fatal("star not taken away")
	}
	rows, _ = a.fontRows(fontMono)
	mono := map[string]bool{}
	for _, f := range fam {
		mono[f.Name] = f.Monospaced
	}
	for _, r := range rows[1:] { // the first is the one used last
		if !mono[r] {
			t.Fatalf("%q in the code list is not monospaced", r)
		}
	}
	a.fonts.query[fontApp] = fam[0].Name[:min(3, len(fam[0].Name))]
	rows, _ = a.fontRows(fontApp)
	if len(rows) == 0 {
		t.Fatal("search found nothing")
	}
}

// The settings page of typefaces draws, with its lists open, and passing over a row shows it.
func TestFontSettingsPage(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	waitFonts(t, a)
	a.openSettings()
	a.settingsTab = tabFonts
	a.fonts.open = [3]bool{true, true, true}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if _, ok := tt.Find("Okuma"); !ok {
		t.Fatalf("no preset buttons: %v", tt.Texts())
	}
	fam := a.families()
	if len(fam) == 0 {
		t.Skip("no fonts installed")
	}
	if err := tt.Click(msg("Editör yazı tipi") + ": " + locales.FontsTheApps()); err != nil {
		t.Fatal(err, tt.Texts())
	}
	_ = ryugo.FontSans
}

// The typeface commands: a family typed after the command, the presets, and the settings' fields.
func TestFontCommands(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	waitFonts(t, a)
	fam := a.families()
	if len(fam) == 0 {
		t.Skip("no fonts installed")
	}
	items := a.paletteItems(">editör yazı tipi " + fam[0].Name)
	found := false
	for _, it := range items {
		if strings.HasPrefix(it.title, "Editör yazı tipi → ") {
			a.runItem(it)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no command for %q", fam[0].Name)
	}
	if a.settings.FontEditor == "" {
		t.Fatal("not set")
	}
	a.noticeUndo()
	if a.settings.FontEditor != "" {
		t.Fatal("not taken back")
	}
	if got := a.paletteItems(">yazı tipi hazırı"); len(got) < 3 {
		t.Fatalf("presets: %d", len(got))
	}
}
