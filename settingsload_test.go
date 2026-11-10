package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Settings that cannot be read are kept aside and not written over with the defaults, and a list index out of range
// from a file is brought into it (#133).
func TestDamagedSettingsKept(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.settingsStore = nil
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	bad := []byte(`{"theme": 2, "syncMode": "server", "macros": [`)
	os.WriteFile(a.settingsFile, bad, 0o600)
	a.loadSettings()
	if b, _ := os.ReadFile(a.settingsFile); string(b) != string(bad) {
		t.Fatalf("written over: %s", b)
	}
	kept, _ := filepath.Glob(a.settingsFile + ".damaged-*")
	if len(kept) != 1 || !strings.Contains(a.notice, "could not be read") {
		t.Fatalf("kept %v, notice %q", kept, a.notice)
	}
	os.WriteFile(a.settingsFile, []byte(`{"theme": -1, "appTheme": -7}`), 0o600)
	a.loadSettings()
	if a.settings.Theme < 0 || a.settings.AppTheme < 0 || a.settings.AppTheme >= len(appThemes) {
		t.Fatalf("out of range: %d %d", a.settings.Theme, a.settings.AppTheme)
	}
	a.appTheme()
}
