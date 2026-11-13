package main

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSoundSamples: every sound starts and ends in silence and stays well under full scale, so none clips; the
// chords are long and the notices short; a sound is made once.
func TestSoundSamples(t *testing.T) {
	for kind := range soundKinds {
		s := soundSamples(kind)
		if len(s) < soundRate/2 {
			t.Fatalf("sound %d lasts %d samples", kind, len(s))
		}
		var peak float64
		for _, v := range s {
			peak = max(peak, math.Abs(float64(v)))
		}
		if peak < 0.05 || peak > 0.9 {
			t.Errorf("sound %d peaks at %.3f", kind, peak)
		}
		if first, last := math.Abs(float64(s[0])), math.Abs(float64(s[len(s)-1])); first > 0.001 || last > 0.001 {
			t.Errorf("sound %d starts at %.4f and ends at %.4f", kind, first, last)
		}
		long := kind == soundWelcome || kind == soundStartup
		if d := time.Duration(len(s)) * time.Second / soundRate; long != (d > 2*time.Second) {
			t.Errorf("sound %d lasts %v", kind, d)
		}
	}
	if soundOf(soundOK) != soundOf(soundOK) {
		t.Error("a sound is made again each time it is played")
	}
}

// TestSoundForToast: each kind of notice has its own sound.
func TestSoundForToast(t *testing.T) {
	cases := []struct {
		in   toastKind
		want soundKind
	}{
		{toastInfo, soundInfo},
		{toastOK, soundOK},
		{toastWarn, soundWarn},
		{toastErr, soundErr},
	}
	for _, tc := range cases {
		if got := soundForToast(tc.in); got != tc.want {
			t.Errorf("soundForToast(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestSoundDebounceAndMute: the same sound twice within soundDebounce is heard once, another sound is heard at once;
// turned off in the settings, or in tests, nothing is heard but a Try button.
func TestSoundDebounceAndMute(t *testing.T) {
	a, _ := newTestApp(t, "")
	a.noSound = false
	if a.settings.NoSound {
		t.Fatal("default settings.NoSound should be false (sounds enabled)")
	}
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	a.clock = func() time.Time { return now }
	var played []soundKind
	a.soundPlay = func(k soundKind) { played = append(played, k) }

	a.playSound(soundInfo)
	now = now.Add(50 * time.Millisecond)
	a.playSound(soundInfo) // debounced (<150ms for same kind)
	a.playSound(soundOK)   // different kind is not debounced
	now = now.Add(soundDebounce)
	a.playSound(soundInfo) // cooldown elapsed

	want := []soundKind{soundInfo, soundOK, soundInfo}
	if !slices.Equal(played, want) {
		t.Fatalf("played = %v, want %v", played, want)
	}

	// Muted via settings disables playSound, while preview still fires.
	a.settings.NoSound = true
	now = now.Add(time.Second)
	a.playSound(soundWarn)
	if len(played) != 3 {
		t.Fatalf("muted settings allowed playSound: %v", played)
	}
	a.playSoundPreview(soundWarn)
	if len(played) != 4 || played[3] != soundWarn {
		t.Fatalf("preview while muted failed: %v", played)
	}

	// Muted via app.noSound disables playSound as well.
	a.settings.NoSound = false
	a.noSound = true
	now = now.Add(time.Second)
	a.playSound(soundErr)
	if len(played) != 4 {
		t.Fatalf("app.noSound allowed playSound: %v", played)
	}
}

// TestStartupSoundSkippedDuringWelcome: the window's chord is off unless the settings turn it on, and with the welcome
// showing it is not played even then: the welcome has its own.
func TestStartupSoundSkippedDuringWelcome(t *testing.T) {
	a, _ := newTestApp(t, "")
	a.noSound = false
	var played []soundKind
	a.soundPlay = func(k soundKind) { played = append(played, k) }

	a.playStartupSound() // off by default: a chord at every start tires
	if len(played) != 0 {
		t.Fatalf("the window's chord played with StartChord off: %v", played)
	}
	a.settings.StartChord = true

	a.wel.on = true
	a.playStartupSound()
	if len(played) != 0 {
		t.Fatalf("startup sound played during welcome screen: %v", played)
	}

	a.wel.on = false
	a.playStartupSound()
	if !slices.Equal(played, []soundKind{soundStartup}) {
		t.Fatalf("played = %v, want [soundStartup]", played)
	}
}

// TestFirstLaunchPlaysExactlyWelcomeSound: the first opening plays the welcome's chord, once, and nothing else, with
// the welcome in the window (Windows, Linux) or on its own stage (macOS).
func TestFirstLaunchPlaysExactlyWelcomeSound(t *testing.T) {
	prev := welcomeShaders
	welcomeShaders = false
	t.Cleanup(func() { welcomeShaders = prev })

	// 1. In-window welcome (Windows/Linux: wel.clear == false): startup sound is skipped, welcome plays once.
	a, tt := newTestApp(t, "")
	if !a.noSound {
		t.Fatal("a test app must be silent unless the test asks for sound")
	}
	a.noSound = false
	a.wel = welcomeState{on: true}
	var played []soundKind
	a.soundPlay = func(k soundKind) { played = append(played, k) }

	a.playStartupSound() // simulates show() in main.go on Windows/Linux
	tt.SetSize(560, 660)
	tt.Advance(1500 * time.Millisecond)
	if err := tt.Click("Başla"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(3 * time.Second)
	if !slices.Equal(played, []soundKind{soundWelcome}) {
		t.Fatalf("in-window first launch played = %v, want exactly [soundWelcome]", played)
	}

	// 2. Stage + opening window welcome (macOS: wel.clear == true): still plays soundWelcome exactly once.
	a2, _ := newTestApp(t, "")
	a2.noSound = false
	a2.wel = welcomeState{on: true, clear: true}
	var macPlayed []soundKind
	a2.soundPlay = func(k soundKind) { macPlayed = append(macPlayed, k) }

	a2.playStartupSound()
	stageTT := ui.NewTester(a2.stageView, 560, 660)
	stageTT.Advance(1500 * time.Millisecond)
	openTT := ui.NewTester(a2.openingView, 560, 660)
	openTT.Advance(200 * time.Millisecond)
	if !slices.Equal(macPlayed, []soundKind{soundWelcome}) {
		t.Fatalf("stage+opening first launch played = %v, want exactly [soundWelcome]", macPlayed)
	}
}
