package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/appicon"
	"github.com/imhalid/celestine/locales"
)

// Each look's icon is drawn from the one drawing: in the Dock's shape its corners are clear and its middle is its
// ground's, the looks differ from one another, and Paper at the size the app ships is the file itself (#601).
// ICON_SHOT_DIR keeps them, to be looked at.
func TestAppIconLooksDrawn(t *testing.T) {
	if got := iconPNG(appicon.Looks[0], 512, true); !bytes.Equal(got, appIcon) {
		t.Fatal("Paper in the Dock is not assets/icon-512.png")
	}
	dir := os.Getenv("ICON_SHOT_DIR")
	mids := map[[3]uint32]string{}
	for _, l := range appicon.Looks {
		b := iconPNG(l, 160, true)
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", l.ID, err)
		}
		if img.Bounds() != image.Rect(0, 0, 160, 160) {
			t.Fatalf("%s: %v", l.ID, img.Bounds())
		}
		if _, _, _, a := img.At(1, 1).RGBA(); a != 0 {
			t.Fatalf("%s: the Dock shape's corner is not clear", l.ID)
		}
		// the ground, inside the rounded square and away from the stickers
		r, g, bl, a := img.At(34, 34).RGBA()
		if a < 0xf000 {
			t.Fatalf("%s: the ground is not opaque: %d", l.ID, a)
		}
		key := [3]uint32{r >> 12, g >> 12, bl >> 12}
		if other, ok := mids[key]; ok {
			t.Fatalf("%s's ground is %s's", l.ID, other)
		}
		mids[key] = l.ID
		sq, err := png.Decode(bytes.NewReader(iconPNG(l, 64, false)))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, a := sq.At(0, 0).RGBA(); a < 0xf000 {
			t.Fatalf("%s: the square icon does not fill its corner", l.ID)
		}
		if dir != "" {
			os.WriteFile(filepath.Join(dir, "icon-"+l.ID+".png"), iconPNG(l, 256, true), 0o644)
		}
	}
}

// The icon follows the setting: a look chosen is put on the Dock and the window, Auto puts Paper on them in a light
// window and the dark look in a dark one, and the choice is kept in the settings file (#601).
func TestAppIconFollowsTheSetting(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	var mu sync.Mutex
	var got [][]byte
	a.iconSet = func(dock, _ []byte) {
		mu.Lock()
		got = append(got, dock)
		mu.Unlock()
	}
	last := func() []byte {
		for range 200 {
			mu.Lock()
			n := len(got)
			mu.Unlock()
			if n > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(got) == 0 {
			return nil
		}
		b := got[len(got)-1]
		got = nil
		return b
	}
	look := func(id string) []byte { l, _ := appicon.ByID(id); return iconPNG(l, 512, true) }

	a.settings.Theme = 1 // light
	tt.Frame()
	if b := last(); !bytes.Equal(b, appIcon) {
		t.Fatal("the first frame did not put Paper on the Dock")
	}
	a.setAppIcon("blue")
	tt.Frame()
	if b := last(); !bytes.Equal(b, look("blue")) {
		t.Fatal("Blue chosen, not shown")
	}
	a.setAppIcon("auto")
	tt.Frame()
	if b := last(); !bytes.Equal(b, appIcon) {
		t.Fatal("Auto in a light window: not Paper")
	}
	a.settings.Theme = 2 // dark
	tt.Frame()
	if b := last(); !bytes.Equal(b, look("dark")) {
		t.Fatal("Auto in a dark window: not the dark look")
	}
	tt.Frame()
	mu.Lock()
	if len(got) != 0 {
		t.Fatal("put on again with nothing changed")
	}
	mu.Unlock()

	// kept: the settings file read again has it
	a.settingsStore = nil
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	a.setAppIcon("violet")
	a.settings.AppIcon = ""
	a.loadSettings()
	if a.settings.AppIcon != "violet" || a.appIconMode() != "violet" {
		t.Fatalf("not kept: %q", a.settings.AppIcon)
	}
	a.setAppIcon("paper")
	if a.settings.AppIcon != "" || a.appIconMode() != "paper" {
		t.Fatalf("Paper kept as %q", a.settings.AppIcon)
	}
	a.settings.AppIcon = "nonesuch"
	if a.appIconMode() != "paper" {
		t.Fatal("a look there is none of is not Paper")
	}
}

// Settings ▸ Appearance has a tile for each look, Auto first, and the palette a command for each (#601).
func TestAppIconPicker(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1200, 1800)
	a.openSettings()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find(locales.IconAppIcon()); !ok {
		t.Fatal("no App icon setting")
	}
	if err := tt.Click(msg("icon_emerald")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.AppIcon != "emerald" {
		t.Fatalf("the tile chose %q", a.settings.AppIcon)
	}
	found := 0
	for _, it := range a.commandItems(msg("icon_appIcon")) {
		if it.title == msg("icon_appIcon")+": "+msg("icon_graphite") {
			it.run()
			found++
		}
	}
	if found != 1 || a.settings.AppIcon != "graphite" {
		t.Fatalf("palette: %d commands, look %q", found, a.settings.AppIcon)
	}
}
