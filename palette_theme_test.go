package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// ⌘P → a theme shows as it is passed over, goes back on Esc, stays on Enter.
func TestPaletteThemePreview(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	a.settings.Theme = 0
	a.openCommands()
	tt.Frame()
	typeEach(tt, "tema: koyu")
	tt.Frame()
	tt.Frame()
	if a.settings.Theme != 2 {
		t.Fatalf("the dark theme was not shown: %d", a.settings.Theme)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(200 * time.Millisecond)
	if a.settings.Theme != 0 {
		t.Fatalf("Esc did not put the theme back: %d", a.settings.Theme)
	}
	a.openCommands()
	tt.Frame()
	typeEach(tt, "tema: koyu")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if a.settings.Theme != 2 {
		t.Fatalf("Enter did not keep the theme: %d", a.settings.Theme)
	}
}

// ⌘P → Ayarlar opens the settings, with the pointer resting wherever it is.
func TestPaletteOpensSettings(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	tt.Move(500, 150) // over the second row of the commands, which the filter will put there
	a.openCommands()
	tt.Frame()
	tt.Move(500, 150)
	typeEach(tt, "ayarlar")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Advance(400 * time.Millisecond)
	if !a.settingsOpen {
		t.Fatal("the settings did not open")
	}
}

// The mode of the palette is the sign at the left, not a letter in the field: a ">" typed first
// makes it the commands, a "@" the pages, and neither is left in the field.
func TestPaletteSignIsNotInTheField(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 600)
	tt.Frame()
	a.openPalette()
	tt.Frame()
	typeEach(tt, ">")
	tt.Frame()
	tt.Frame()
	if !a.palCmds() || a.pal.text != "" {
		t.Fatalf("a \">\" did not make the commands: query %q text %q", a.pal.query, a.pal.text)
	}
	typeEach(tt, "tema")
	tt.Frame()
	if a.pal.query != ">tema" || a.pal.text != "tema" {
		t.Fatalf("query %q text %q", a.pal.query, a.pal.text)
	}
	for i := 0; i < 4; i++ {
		tt.Key(0, ui.KeyBackspace)
		tt.Frame()
	}
	typeEach(tt, "@")
	tt.Frame()
	tt.Frame()
	if a.palCmds() || a.pal.text != "" {
		t.Fatalf("a \"@\" did not make the pages: query %q text %q", a.pal.query, a.pal.text)
	}
}
