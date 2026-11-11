package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestParseAccel(t *testing.T) {
	for _, c := range []struct {
		in   string
		norm string
		ok   bool
	}{
		{"cmd+shift+y", "cmd+shift+y", true}, {"Shift + Cmd + Y", "cmd+shift+y", true}, {"⌘⇧y", "", false}, {"cmd+9", "cmd+9", true},
		{"alt+cmd+enter", "alt+cmd+enter", true}, {"y", "", false}, {"cmd+", "", false}, {"hyper+y", "", false}, {"cmd+ğ", "", false}, {"ctrl+,", "ctrl+,", true},
	} {
		_, _, norm, ok := parseAccel(c.in)
		if ok != c.ok || norm != c.norm {
			t.Errorf("%q: %q %v, want %q %v", c.in, norm, ok, c.norm, c.ok)
		}
	}
}

func TestBindKeyRefusals(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	for _, bad := range []struct{ cmd, key string }{
		{"common_tasks", "cmd+p"}, {"common_tasks", "cmd+q"}, {"common_tasks", "y"}, {"Böyle komut yok zzz", "cmd+shift+y"}, {"common_tasks", "cmd+shift+müthiş"},
	} {
		if err := a.bindKey(bad.cmd, bad.key); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if err := a.bindKey("common_tasks", "cmd+shift+y"); err != nil {
		t.Fatal(err)
	}
	if err := a.bindKey("common_trash", "shift+cmd+y"); err == nil || !strings.Contains(err.Error(), msg("common_tasks")) {
		t.Fatalf("a taken key given again: %v", err)
	}
	if err := a.bindKey("common_tasks", "cmd+shift+u"); err != nil { // the same command may change its key
		t.Fatal(err)
	}
	if a.settings.Keys["common_tasks"] != "cmd+shift+u" {
		t.Fatal("not changed")
	}
}

// A key of one's own runs its command, and the palette shows it.
func TestBoundKeyRuns(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if err := a.bindKey("common_trash", "cmd+shift+y"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyY)
	tt.Advance(100 * time.Millisecond)
	if !a.trashOpen {
		t.Fatal("the key did not run the command")
	}
	shown := false
	for _, it := range a.commands() {
		if it.id == "common_trash" && it.keys == "cmd+shift+y" {
			shown = true
		}
	}
	if !shown {
		t.Fatal("the palette does not show the key")
	}
	a.unbindKey("common_trash")
	a.trashOpen = false
	tt.Frame()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyY)
	tt.Advance(100 * time.Millisecond)
	if a.trashOpen {
		t.Fatal("a key taken off still runs")
	}
}

func TestMacros(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	if err := a.saveMacro("", "common_tasks"); err == nil {
		t.Fatal("no name")
	}
	if err := a.saveMacro("m", "yoksayılanBirKomutZzz"); err == nil {
		t.Fatal("a command that is not there")
	}
	if err := a.saveMacro("m", " + ; "); err == nil {
		t.Fatal("no commands")
	}
	if err := a.saveMacro("Gün", "Bugünün sayfası + Görevler"); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.Macros["Gün"]; len(got) != 2 || got[1] != "common_tasks" {
		t.Fatalf("%v", got)
	}
	runCmd(t, a, ">makro", "Makro: Gün")
	if !a.tasksOpen {
		t.Fatal("the macro did not run to its end")
	}
	if a.runMacro("yok") != 0 {
		t.Fatal("a macro that is not there ran")
	}
}

func TestKeysPage(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.openSettings()
	a.setMode = "keys"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Komuta kısayol ver", "Makro yaz"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
}
