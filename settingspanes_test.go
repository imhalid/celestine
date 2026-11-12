package main

import (
	"testing"

	"github.com/imhalid/celestine/locales"
)

// settingsAt opens the settings at the section named name.
func settingsAt(t *testing.T, a *app, name string) {
	t.Helper()
	a.openSettings()
	for i, s := range settingsSections {
		if s.name == name {
			a.settingsTab = i
			return
		}
	}
	t.Fatalf("no section %q", name)
}

// TestSyncFolderPicker: the folder mode of the sync has a button that opens the system's folder picker (#406).
func TestSyncFolderPicker(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode = "dir"
	settingsAt(t, a, "sync4")
	tt.Frame()
	tt.Frame()
	if !tt.HasText(locales.BackupChooseFolder()) {
		t.Fatalf("no picker: %v", tt.Texts())
	}
}

// TestShortcutsListed: the Shortcuts page lists the app's shortcuts, not only one's own (#412).
func TestShortcutsListed(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.openSettings()
	tt.Frame()
	a.setMode = "keys"
	tt.Frame()
	tt.Frame()
	if !tt.HasText(locales.KeysAllShortcuts()) || !tt.HasText(locales.HelpBoldItalic()) || !tt.HasText("⌘B ⌘I ⌘U") {
		t.Fatalf("no shortcuts: %v", tt.Texts())
	}
}
