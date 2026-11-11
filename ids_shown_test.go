package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// No screen shows a message's id where its words should be: the main window, every tab of the settings, the tasks,
// the collection's settings and view, the statistics, the trash, the shortcuts and the palette's modes.
func TestNoIDShown(t *testing.T) {
	var en map[string]any
	b, _ := os.ReadFile("locales/en.json")
	json.Unmarshal(b, &en)
	isID := func(s string) bool {
		for _, w := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ':' || r == '(' || r == ')' || r == '·' }) {
			if _, ok := en[w]; ok && len(w) > 3 && (strings.Contains(w, "_") || strings.ToLower(w) != w) {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	var out []string
	look := func(where string, texts []string) {
		for _, s := range texts {
			if isID(s) && !seen[s] {
				seen[s] = true
				out = append(out, where+"\t"+s)
			}
		}
	}
	a, tt := newTestApp(t, "# Başlık\nmetin #etiket\n- [ ] görev\n")
	tt.SetSize(1300, 2400)
	frame := func() { tt.Frame(); tt.Advance(400 * time.Millisecond); tt.Frame() }
	frame()
	look("main", tt.Texts())
	a.openSettings()
	for tab := range settingsSections {
		a.settingsTab = tab
		frame()
		look("settings", tt.Texts())
	}
	a.settingsOpen = false
	a.tasksOpen = true
	frame()
	look("tasks", tt.Texts())
	a.tasksOpen = false
	a.openCollSettings(a.page.Coll, 0)
	for i := 0; i < 5; i++ {
		a.cs.tab = i
		frame()
		look("collsettings", tt.Texts())
	}
	a.cs.open = false
	a.coll = a.page.Coll
	frame()
	look("collection", tt.Texts())
	a.statsScreen = true
	frame()
	look("stats", tt.Texts())
	a.statsScreen = false
	a.trashOpen = true
	frame()
	look("trash", tt.Texts())
	a.trashOpen = false
	a.kb.help = true
	frame()
	look("help", tt.Texts())
	a.kb.help = false
	for _, q := range []string{">", ":", "!", "?", "#"} {
		for _, it := range a.paletteItems(q) {
			look("palette", []string{it.title, it.sub, it.off})
		}
	}
	sort.Strings(out)
	if len(out) > 0 {
		t.Fatalf("ids shown:\n%s", strings.Join(out, "\n"))
	}
}
