package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// openSetWin opens a's settings in a window of their own, whose view a tester of its own draws.
func openSetWin(t *testing.T, a *app) *ui.Tester {
	t.Helper()
	var view func(*ui.Context)
	a.newSetWin = func(v func(*ui.Context)) *mygo.Window { view = v; return nil }
	a.openSettings()
	if !a.sw.open || view == nil {
		t.Fatal("the settings' window did not open")
	}
	wt := ui.NewTester(view, 980, 1400)
	wt.Frame()
	wt.Advance(300 * time.Millisecond)
	return wt
}

// ⌘, opens the settings in a window of their own and leaves the pane as it was; the window's sections are listed
// under the search with a badge each, ‹ and › (and ⌘[ ⌘]) go through those shown, and a section asked for from
// elsewhere opens in the window (#605).
func TestSettingsWindow(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.Frame()
	page := a.pane().page
	wt := openSetWin(t, a)
	if a.pane().settingsOpen || a.pane().page != page {
		t.Fatal("the settings took the pane")
	}
	if _, ok := wt.Find(locales.SettingsSearchSettings()); !ok {
		t.Fatal("no search in the window")
	}
	if _, ok := wt.Find(locales.CommonSettings()); ownWindowControls && !ok { // its own title bar (#615)
		t.Fatal("no title bar of its own")
	}
	for _, s := range settingsSections {
		if _, ok := wt.Find(msg(s.name)); !ok {
			t.Fatalf("no section %q", msg(s.name))
		}
	}
	if _, ok := wt.Find(locales.SettingsVersionLine(version)); !ok {
		t.Fatal("no version in the sidebar")
	}
	pn := a.sw.pn
	if err := wt.Click(msg("editor")); err != nil {
		t.Fatal(err)
	}
	wt.Frame()
	if err := wt.Click(msg("sync4")); err != nil {
		t.Fatal(err)
	}
	wt.Frame()
	if pn.settingsTab != tabSync {
		t.Fatalf("section %d, want Sync", pn.settingsTab)
	}
	if _, ok := wt.Find("Previous section"); ok { // no ‹ › in the head (#629)
		t.Fatal("a back button in the settings' head")
	}
	wt.Key(ui.Cmd, ui.KeyBracketLeft) // ⌘[ still goes back
	wt.Frame()
	if pn.settingsTab != tabEditor {
		t.Fatalf("⌘[: section %d, want Editor", pn.settingsTab)
	}
	wt.Key(ui.Cmd, ui.KeyBracketRight)
	wt.Frame()
	if pn.settingsTab != tabSync || len(pn.settingsFwd) != 0 {
		t.Fatalf("⌘]: section %d, forward %v", pn.settingsTab, pn.settingsFwd)
	}
	a.openSettingsAt(tabMCP) // the status bar's MCP, a notification's Sync
	if pn.settingsTab != tabMCP || a.pane().settingsOpen {
		t.Fatalf("asked for from elsewhere: section %d, pane took it %v", pn.settingsTab, a.pane().settingsOpen)
	}
}

// A setting changed in the settings' window is the app's at once: the main window is asked for a frame and draws it,
// and one changed from the main window is shown in the settings' window (#605).
func TestSettingsWindowIsLive(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.Frame()
	asked := 0
	a.invalidate = func() { asked++ }
	wt := openSetWin(t, a)
	asked = 0
	if err := wt.Click(locales.CommonDark()); err != nil {
		t.Fatal(err)
	}
	wt.Frame()
	if a.settings.Theme != 2 || asked == 0 {
		t.Fatalf("theme %d, main window asked %d times", a.settings.Theme, asked)
	}
	tt.Frame()
	if !ryugoDark(a) {
		t.Fatal("the main window is not dark")
	}
	a.setAppIcon("amber") // from the main window, as the palette does
	tt.Frame()
	if a.sw.mainSaw != a.settingsGen {
		t.Fatal("the settings' window was not told")
	}
}

// ryugoDark reports whether the app's main window was last drawn dark.
func ryugoDark(a *app) bool { return a.look != "" && a.look[:4] == "true" }

// Where there is no window of its own (a phone), the settings are a screen of the pane, as before; Typeface has its
// presets as tiles and Status bar a preview that follows its switches (#605).
func TestSettingsInlineAndPreviews(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1400, 1600)
	a.sidebarClosed = true
	a.openSettings()
	tt.Frame()
	if !a.pane().settingsOpen || a.sw.open {
		t.Fatal("with no window, the settings are not the pane's")
	}
	a.settingsGo(tabStatus)
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find(locales.EditorSaved()); !ok {
		t.Fatal("the preview does not show the save state")
	}
	a.settings.HideSaved, a.settings.ShowChars = true, true
	tt.Frame()
	if _, ok := tt.Find(locales.EditorSaved()); ok {
		t.Fatal("the save state hidden still shows")
	}
	if _, ok := tt.Find(locales.StatusChars(1284)); !ok {
		t.Fatal("the count turned on does not show")
	}
	if len(a.pane().settingsBack) == 0 || !a.settingsStep(a.pane(), -1) || a.pane().settingsTab != tabAppearance {
		t.Fatal("the pane's settings have no history")
	}
}
