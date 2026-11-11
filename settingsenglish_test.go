package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// In English the settings' Library tab of a one-file library, with backups of each kind, has no Turkish left: its
// buttons, its notes, the backups' kinds; and the bar's tips are English too (#285).
func TestLibrarySettingsInEnglish(t *testing.T) {
	withLang(t, "en")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // the backups are written here, not among the computer's
	st, _ := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	for i, label := range []string{"", "elle", "içe aktarma"} {
		if _, err := backupTo(st, backupDir(), label, time.Date(2026, 11, 9, 10, i, 0, 0, time.Local)); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(backups(backupDir())); n != 3 || filepath.Dir(backups(backupDir())[0].Path) != backupDir() {
		t.Fatalf("%d backups", n)
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1200, 1100)
	tt.Advance(300 * time.Millisecond)
	a.settingsOpen, a.settingsTab = true, 5
	tt.Advance(900 * time.Millisecond)
	texts := tt.Texts()
	from, to := slices.Index(texts, "The files and where they are"), slices.Index(texts, "search settings")
	if from < 0 || to < from {
		t.Fatalf("not the Library tab: %q", texts)
	}
	turkish := regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`)
	for _, s := range texts[from:to] {
		if turkish.MatchString(s) {
			t.Errorf("Turkish in the Library tab: %q", s)
		}
	}
	for _, want := range []string{"Write a zip", "Back up now", "Restore", "Backups", "by hand · ", "before an import · ", "daily · "} {
		found := false
		for _, s := range texts[from:to] {
			found = found || s == want || len(s) > len(want) && s[:len(want)] == want
		}
		if !found {
			t.Errorf("no %q in the Library tab", want)
		}
	}
	for _, tip := range []string{"Switch to the dark theme", "Sidebar (⌘\\)", "Back (⌘[)", "Forward (⌘])", "Settings", "Search settings"} {
		if !slices.Contains(texts, tip) {
			t.Errorf("no tip %q", tip)
		}
	}
}
