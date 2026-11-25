package main

import (
	"testing"
	"time"
)

// The gear at the top left opens a short menu: a list is gone into and a
// choice made in it; "Tüm ayarlar…" is the dialog.
func TestGearMenu(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if err := tt.Click("Ayarlar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if a.pane().settingsOpen {
		t.Fatal("the gear opened the dialog, not a menu")
	}
	if err := tt.Click("Köşeler"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("50%"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if a.cornerPercent() != 50 {
		t.Fatalf("corners %d%%", a.cornerPercent())
	}
	if err := tt.Click("Ayarlar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Tüm ayarlar…"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if !a.pane().settingsOpen {
		t.Fatal("the dialog did not open")
	}
}

// A slider moves a setting at every frame: it is kept once, a moment after it stops.
func TestSettingsKeptSoon(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	st, err := OpenStore(t.TempDir() + "/k.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a.settingsStore = st
	a.clock = tt.Now // the settings screen keeps what is set a moment after it stops: by the window's clock
	a.pane().settingsOpen = true
	tt.Frame()
	a.settings.PageWidth = 800
	a.saveSettingsSoon()
	tt.Frame()
	if st.Meta(SettingsKey) != nil {
		t.Fatal("written at once")
	}
	tt.Advance(600 * time.Millisecond)
	if st.Meta(SettingsKey) == nil {
		t.Fatal("never written")
	}
}
