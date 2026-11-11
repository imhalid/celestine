package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's commands are written once: the menu bar has each with its key, the palette lists each,
// the window takes their keys without a menu bar, and a command of one's own cannot take them.
func TestAppCommandsAreOneList(t *testing.T) {
	a, tt := newTestApp(t, "merhaba\n")
	tt.SetSize(900, 600)
	tt.Advance(200 * time.Millisecond)
	menu := map[string]string{}
	for _, top := range a.menu().Items() {
		for _, it := range top.Submenu {
			menu[it.Label] = it.Accelerator
		}
	}
	pal := map[string]string{}
	for _, it := range a.commands() {
		pal[it.title] = it.keys
	}
	for _, c := range appCommands() {
		if c.accel != "" {
			if _, _, _, ok := parseAccel(c.accel); !ok {
				t.Fatalf("%s: the key %q does not read", c.title, c.accel)
			}
		}
		if c.menu != "" {
			label := msg(c.title)
			if c.dots {
				label += "…"
			}
			acc, ok := menu[label]
			if !ok || acc != menuAccel(c.accel) {
				t.Fatalf("%s: not in the menu bar as %q with %q: %v", c.title, label, menuAccel(c.accel), menu)
			}
		}
		if !c.noPal {
			if keys, ok := pal[msg(c.title)]; !ok || c.accel != "" && keys != accelLabel(c.accel) {
				t.Fatalf("%s: not in the palette with %q (%q, %v)", c.title, accelLabel(c.accel), keys, ok)
			}
		}
		if _, _, norm, ok := parseAccel(c.accel); ok && a.accelConflict(norm, "x") == "" {
			t.Fatalf("%s: its key %s can be given to another command", c.title, norm)
		}
	}
	if got := menuAccel("alt+cmd+enter"); got != "CmdOrCtrl+Alt+Enter" {
		t.Fatalf("menuAccel: %q", got)
	}
	if got := accelLabel("cmd+shift+up"); got != "⇧⌘↑" {
		t.Fatalf("accelLabel: %q", got)
	}
	// without a menu bar the window takes the keys
	closed := a.sidebarClosed
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Advance(100 * time.Millisecond)
	if a.sidebarClosed == closed {
		t.Fatal("⌘\\ did not fold the sidebar")
	}
	n := len(a.lib.Collections)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyN)
	tt.Advance(100 * time.Millisecond)
	if len(a.lib.Collections) != n+1 {
		t.Fatalf("⇧⌘N made no collection: %d", len(a.lib.Collections))
	}
	// what cannot run now is dim in the palette and does not run
	for _, it := range a.commands() {
		if it.title == msg("Bölmeyi kapat") && it.off == "" {
			t.Fatal("closing the only pane is offered")
		}
	}
	if a.runCommand(msg("Bölmeyi kapat")) {
		t.Fatal("the only pane was closed")
	}
}

// Every command of the app's list has its English, as the window speaks it.
func TestAppCommandsInEnglish(t *testing.T) {
	setLang("en")
	defer setLang("tr")
	var missing []string
	for _, c := range appCommands() {
		if msg(c.title) == c.title {
			missing = append(missing, c.title)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no English for: %q", missing)
	}
}
