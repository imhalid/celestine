package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSkyInkReads sweeps the day every five minutes under every weather and appearance and checks the page's ink
// against both ends of the sky: the text, the secondary text and the whisper labels keep their contrast on all of
// them, the accent stands out, and the polarity is the one the system bars are given.
func TestSkyInkReads(t *testing.T) {
	kinds := []weatherKind{"", weatherClear, weatherPartly, weatherCloudy, weatherFog, weatherRain, weatherStorm, weatherSnow}
	need := map[string]float64{"text": 7, "muted": 4.5, "faint": 3, "accent": 3}
	worst := map[string]float64{}
	for _, pref := range []string{"", "light", "dark"} {
		for _, w := range [][2]float32{{7.2, 16.8}, {5.4, 20.6}} {
			for _, k := range kinds {
				wx := weatherInfo{HasWeather: k != "", Kind: k, SunriseHr: w[0], SunsetHr: w[1], SolarDay: "2026-11-28"}
				for m := 0; m < 24*60; m += 5 {
					now := time.Date(2026, 11, 28, 0, m, 0, 0, time.Local)
					p, isDark := paletteAt(now, pref, wx)
					at := now.Format("15:04") + " " + string(k) + " " + pref
					if isDark != isDarkFor(now, pref, wx) {
						t.Fatalf("%s: palette and system bars disagree on polarity", at)
					}
					if pref == "light" && isDark || pref == "dark" && !isDark {
						t.Fatalf("%s: appearance not kept", at)
					}
					inks := map[string]ui.Color{"text": p.text, "muted": p.muted, "faint": p.faint, "accent": p.accent}
					for name, ink := range inks {
						for _, bg := range []ui.Color{p.bgTop, p.bgBottom} {
							got := contrast(ink, bg)
							if w, ok := worst[name]; !ok || got < w {
								worst[name] = got
							}
							if got < need[name] {
								t.Errorf("%s: %s contrast %.2f < %.1f", at, name, got, need[name])
							}
						}
					}
				}
			}
		}
	}
	t.Logf("worst contrast: %v", worst)
}

// TestSkyWeatherHasItsOwnMood checks that each weather gives the noon sky colors of its own, that a storm darkens
// even the day, and that the sky never turns green or pink on its way from a warm hour to a weather's cool one.
func TestSkyWeatherHasItsOwnMood(t *testing.T) {
	noon := time.Date(2026, 11, 28, 12, 0, 0, 0, time.Local)
	golden := time.Date(2026, 11, 28, 16, 0, 0, 0, time.Local)
	seen := map[ui.Color]weatherKind{}
	for _, k := range []weatherKind{weatherClear, weatherPartly, weatherCloudy, weatherFog, weatherRain, weatherStorm, weatherSnow} {
		wx := weatherInfo{HasWeather: true, Kind: k, SunriseHr: 7.2, SunsetHr: 16.8, SolarDay: "2026-11-28"}
		s := skyAt(noon, "", wx)
		if o, ok := seen[s.top]; ok {
			t.Errorf("%s and %s share the noon sky %v", k, o, s.top)
		}
		seen[s.top] = k
		if (k == weatherStorm) != s.dark {
			t.Errorf("%s at noon: dark = %v", k, s.dark)
		}
		g := skyAt(golden, "", wx)
		for _, c := range []ui.Color{g.top, g.bottom} {
			if l := toLCH(c); l.C > 0.03 && l.H > 120 && l.H < 200 {
				t.Errorf("%s at golden hour turned green: %v (hue %.0f)", k, c, l.H)
			}
		}
	}
}

// TestMoonPhase checks the moon's age against known full and new moons.
func TestMoonPhase(t *testing.T) {
	for _, c := range []struct {
		at   time.Time
		want float32
	}{
		{time.Date(2026, 11, 24, 14, 0, 0, 0, time.UTC), 0.5}, // full moon
		{time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC), 0},     // new moon
	} {
		got := moonAge(c.at)
		d := got - c.want
		if d > 0.5 {
			d -= 1
		}
		if d < -0.04 || d > 0.04 {
			t.Errorf("moonAge(%s) = %.3f, want about %.2f", c.at.Format("2006-01-02"), got, c.want)
		}
	}
	if moonLit(0, 0, 9, 0.001) != nil {
		t.Error("a new moon should show no lit part")
	}
	if moonLit(0, 0, 9, 0.5) == nil {
		t.Error("a full moon should be lit")
	}
}

// TestSkyAnchorsAscend checks the day's anchors stay in order for any sunrise and sunset the weather may give, so
// the sky never runs backward or jumps at midnight on a short winter day or a long summer one.
func TestSkyAnchorsAscend(t *testing.T) {
	for _, w := range [][2]float32{{7.2, 16.8}, {5.4, 20.6}, {3.1, 22.9}, {10.9, 14.1}} {
		as := skyAnchors(w[0], w[1])
		for i := 1; i < len(as); i++ {
			if as[i].hour <= as[i-1].hour || as[i].hour >= 24 {
				t.Errorf("sunrise %.1f, sunset %.1f: anchor %d at %.2f after %.2f", w[0], w[1], i, as[i].hour, as[i-1].hour)
			}
		}
	}
}

// TestSkyMovesOnlyWhenItShould checks the weather asks for frames while it moves, and asks for none while writing,
// under a sheet, or when the system asks for less motion.
func TestSkyMovesOnlyWhenItShould(t *testing.T) {
	registerOnce.Do(registerFonts)
	for _, c := range []struct {
		name   string
		reduce bool
		sheet  bool
		want   bool
	}{
		{"rain", false, false, true},
		{"reduced motion", true, false, false},
		{"under a sheet", false, true, false},
	} {
		a := newApp(t.TempDir())
		if err := a.open(); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.Local)
		a.clock = func() time.Time { return now }
		a.wx.info = weatherInfo{HasWeather: true, Kind: weatherRain, SunriseHr: 7.2, SunsetHr: 16.8, SolarDay: "2026-11-28"}
		a.skyLive = true
		if c.sheet {
			a.sheet = sheetSettings
		}
		tt := ui.NewTesterWith(a.view, 412, 860, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: c.reduce}})
		tt.Frame()
		tt.Advance(150 * time.Millisecond)
		if got := tt.FramePending(); got != c.want {
			t.Errorf("%s: frame pending = %v, want %v", c.name, got, c.want)
		}
	}
}
