package main

import (
	"testing"

	"github.com/imhalid/celestine/locales"
)

// TestReducedMotionCaret: with reduced motion the gear's menu has no caret list, and the editor's caret card folds to a
// line that says why (#402).
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
	a.settingsTab = 1
	tt.Frame()
	tt.Frame()
	if has() {
		t.Fatal("a caret list with reduced motion")
	}
	if !tt.HasText(locales.SettingsCaretStill()) || tt.HasText(msg(caretName(caretSlime))) {
		t.Fatalf("the caret card: %v", tt.Texts())
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
