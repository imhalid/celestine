package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
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
	if len(kept) != 1 || !strings.Contains(a.notice, kept[0]) {
		t.Fatalf("kept %v, notice %q", kept, a.notice)
	}
	os.WriteFile(a.settingsFile, []byte(`{"theme": -1, "appTheme": -7}`), 0o600)
	a.loadSettings()
	if a.settings.Theme < 0 || a.settings.AppTheme < 0 || a.settings.AppTheme >= len(appThemes) {
		t.Fatalf("out of range: %d %d", a.settings.Theme, a.settings.AppTheme)
	}
	a.appTheme()
}

// The press waves are on until the Appearance settings' switch turns them
// off; the setting is what Ryugo's Root is given (frameTheme).
func TestPressWavesSetting(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000+2*frameW+frameGap, 2400) // the content 1000 wide inside the window's frame and its gap (windowframe.go)
	if a.settings.NoWaves {
		t.Fatal("the waves start off")
	}
	a.settingsOpen, a.settingsTab = true, 0
	tt.Advance(900 * time.Millisecond)
	title, ok := tt.Find(msg("common_pressWaves"))
	if !ok {
		t.Fatal("no Press waves row")
	}
	// the switch is at the row's end, level with the middle of its title and help
	flip := func() bool {
		was := a.settings.NoWaves
		for dy := float32(0); dy <= 40; dy += 8 {
			for x := float32(990); x > title.X+title.W; x -= 6 {
				tt.ClickAt(x, title.Y+title.H/2+dy)
				tt.Advance(50 * time.Millisecond)
				if a.settings.NoWaves != was {
					return true
				}
			}
		}
		return false
	}
	if !flip() || !a.settings.NoWaves {
		t.Fatal("the switch did not turn the waves off")
	}
	if !flip() || a.settings.NoWaves {
		t.Fatal("the switch did not turn the waves back on")
	}
}

// TestSidebarCardSetting: in the window's frame the sidebar lies on the frame's ground, and the setting of its
// background gives it a card (#381, #489).
func TestSidebarCardSetting(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000+2*frameW+frameGap, 2400)
	tt.Advance(100 * time.Millisecond)
	if !a.wframe.on || !a.sideOnGround() || a.settings.SidebarCard {
		t.Fatalf("framed %v, on the ground %v, card %v", a.wframe.on, a.sideOnGround(), a.settings.SidebarCard)
	}
	a.settingsOpen, a.settingsTab = true, 0
	tt.Advance(900 * time.Millisecond)
	title, ok := tt.Find(locales.SettingsSidebarCard())
	if !ok {
		t.Fatal("no Sidebar background row")
	}
	for dy := float32(0); dy <= 40 && !a.settings.SidebarCard; dy += 8 {
		for x := float32(990); x > title.X+title.W && !a.settings.SidebarCard; x -= 6 {
			tt.ClickAt(x, title.Y+title.H/2+dy)
			tt.Advance(50 * time.Millisecond)
		}
	}
	if !a.settings.SidebarCard || a.sideOnGround() {
		t.Fatalf("the switch: card %v, on the ground %v", a.settings.SidebarCard, a.sideOnGround())
	}
}
