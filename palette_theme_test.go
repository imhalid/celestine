package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
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
	if !a.pane().settingsOpen {
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

// TestAppThemeWellAlphaAndSurface verifies that applySkin preserves Well and WellBorder alpha,
// keeps WellBorder at HEAD's line color, and keeps sidebar Surface independent from card-derived Well (#452).
func TestAppThemeWellAlphaAndSurface(t *testing.T) {
	shotDir := os.Getenv("THEME_SHOT_DIR")
	prefix := os.Getenv("THEME_SHOT_PREFIX")
	a, tt := newTestApp(t, "# Midnight Theme Check\n\nSome content in the editor.\n")
	tt.SetSize(1180, 760)
	a.setAppTheme(1) // midnight
	a.pane().settingsOpen, a.pane().settingsTab = true, 0
	tt.Advance(500 * time.Millisecond)

	save := func(name string) {
		if shotDir == "" || prefix == "" {
			return
		}
		_ = os.MkdirAll(shotDir, 0o755)
		f, err := os.Create(filepath.Join(shotDir, prefix+"-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}

	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		a.pane().settingsTab = 0 // Appearance / theme picker + well groups
		tt.Advance(400 * time.Millisecond)
		label := map[bool]string{false: "light", true: "dark"}[dark]
		save("settings-" + label)
	}

	// Verify Well and WellBorder preserve token alpha and th.Surface stays separate from Well when p.side is empty (#452).
	for _, tc := range []struct {
		name string
		dark bool
		mode ryugo.Mode
	}{
		{"light", false, ryugo.Light},
		{"dark", true, ryugo.Dark},
	} {
		var base, got, noSide ryugo.Tokens
		var thGot, thNoSide, thNoCard ui.Theme
		checker := ui.NewTester(func(c *ui.Context) {
			root := ryugo.Root(c, ryugo.Options{Mode: tc.mode, Size: ryugo.Sm})
			base = *root
			got = *root
			noSide = *root
			noCard := *root
			thGot = *c.Theme()
			thNoSide = *c.Theme()
			thNoCard = *c.Theme()

			mid := appThemes[1].colors(tc.dark)
			a.applySkin(&got, &thGot, mid)

			withoutSide := mid
			withoutSide.side = ""
			a.applySkin(&noSide, &thNoSide, withoutSide)

			withoutCard := withoutSide
			withoutCard.card = ""
			a.applySkin(&noCard, &thNoCard, withoutCard)
		}, 400, 300)
		checker.Frame()

		if got.Colors.Well.A != base.Colors.Well.A {
			t.Fatalf("%s Well alpha = %d, want %d", tc.name, got.Colors.Well.A, base.Colors.Well.A)
		}
		line := ui.Hex(appThemes[1].colors(tc.dark).line)
		if got.Colors.WellBorder.R != line.R || got.Colors.WellBorder.G != line.G || got.Colors.WellBorder.B != line.B || got.Colors.WellBorder.A != base.Colors.WellBorder.A {
			t.Fatalf("%s WellBorder = %+v, want rgb(%d,%d,%d) alpha=%d", tc.name, got.Colors.WellBorder, line.R, line.G, line.B, base.Colors.WellBorder.A)
		}
		if thNoSide.Surface != thNoCard.Surface {
			t.Fatalf("%s sidebar Surface changed when card overridden without side: %+v != %+v", tc.name, thNoSide.Surface, thNoCard.Surface)
		}
		if thNoSide.Surface == noSide.Colors.Well {
			t.Fatalf("%s sidebar Surface unexpectedly equals card-derived Well %+v", tc.name, noSide.Colors.Well)
		}
	}
}

// TestDefaultThemeTileKeepsItsColors: the Default theme's tile shows Ryugo's own colors whichever theme is chosen,
// not the window's, which the chosen theme paints (#484).
func TestDefaultThemeTileKeepsItsColors(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Frame()
	own := a.ryugoOwn
	for i := range appThemes {
		if i == 0 {
			continue
		}
		a.setAppTheme(i)
		tt.Frame()
		if a.ryugoOwn != own {
			t.Fatalf("theme %s changed the Default tile's colors: %v, was %v", appThemes[i].name, a.ryugoOwn.Bg, own.Bg)
		}
	}
	if win := ryugoColorsNow(a, tt); win.Bg == own.Bg {
		t.Fatal("the window should be in the chosen theme's colors, not Ryugo's own")
	}
}

// ryugoColorsNow are the colors the window is drawn in this frame.
func ryugoColorsNow(a *app, tt *ui.Tester) ryugo.Colors {
	var got ryugo.Colors
	view := a.view
	tt2 := ui.NewTester(func(c *ui.Context) { view(c); got = ryugo.From(c).Colors }, 400, 300)
	tt2.Frame()
	return got
}

// A preview starts from what was there when the palette opened: a color theme passed over after "Theme: Dark" is shown
// in the light theme the user has (#512).
func TestPalettePreviewsDoNotStack(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.Theme, a.settings.AppTheme = 1, 0
	items := []palItem{
		{title: "dark", preview: func() { a.settings.Theme = 2 }},
		{title: "ember", preview: func() { a.settings.AppTheme = 3 }},
	}
	a.pal.sel = 0
	a.palPreview(items)
	a.pal.sel = 1
	a.palPreview(items)
	if a.settings.Theme != 1 || a.settings.AppTheme != 3 {
		t.Fatalf("theme %d, color theme %d", a.settings.Theme, a.settings.AppTheme)
	}
	a.palRestore()
	if a.settings.Theme != 1 || a.settings.AppTheme != 0 {
		t.Fatalf("not put back: theme %d, color theme %d", a.settings.Theme, a.settings.AppTheme)
	}
}
