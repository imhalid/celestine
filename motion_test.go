package main

import (
	"testing"

	"github.com/imhalid/celestine/locales"
)

// TestReducedMotionCaret: with reduced motion the gear's menu has no caret list, the editor has no caret card, and the
// switches of the shaders and the ripple are not shown (#402, #405).
func TestReducedMotionCaret(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	has := func() bool {
		for _, m := range a.quickSettings() {
			if m.title == "caret" {
				return true
			}
		}
		return false
	}
	tt.Frame()
	if !has() {
		t.Fatal("no caret list in the menu")
	}
	a.settings.ReduceMotion = true
	a.openSettings()
	a.pane().settingsTab = 1
	tt.Frame()
	tt.Frame()
	if has() {
		t.Fatal("a caret list with reduced motion")
	}
	if tt.HasText(msg(caretName(caretSlime))) {
		t.Fatalf("the caret card: %v", tt.Texts())
	}
	a.pane().settingsTab = 0
	tt.Frame()
	if !tt.HasText(locales.SettingsReduceMotion()) || tt.HasText(locales.SettingsShaderEffects()) || tt.HasText(locales.CommonPressWaves()) {
		t.Fatalf("the motion's switches: %v", tt.Texts())
	}
}

// TestSettingsSearchGrouped: the search's results are under their sections' names, the motion's in Appearance (#403).
func TestSettingsSearchGrouped(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.openSettings()
	tt.Frame()
	a.setSearch = "motion"
	tt.Frame()
	if !tt.HasText(msg("common_appearance")) || !tt.HasText(locales.SettingsReduceMotion()) {
		t.Fatalf("results: %v", tt.Texts())
	}
	if tabOfField("reduceMotion") != tabAppearance {
		t.Fatal("reduce motion is not in Appearance")
	}
}

// TestSettingsNavClearsSearch: a section clicked during a search or a list shows the section, the search's words
// cleared and the list closed (#404).
func TestSettingsNavClearsSearch(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.openSettings()
	tt.Frame()
	a.setSearch = "motion"
	tt.Frame()
	if err := tt.Click(msg("editor")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.setSearch != "" || a.pane().settingsTab != 1 || tt.HasText("motion") {
		t.Fatalf("search %q tab %d: %v", a.setSearch, a.pane().settingsTab, tt.Texts())
	}
	a.setMode = "changed"
	tt.Frame()
	if err := tt.Click(msg("common_appearance")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.setMode != "" || a.pane().settingsTab != 0 {
		t.Fatalf("mode %q tab %d", a.setMode, a.pane().settingsTab)
	}
}
