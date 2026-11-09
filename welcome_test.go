package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func welcomeApp(t *testing.T) *app {
	welcomeShaders = false
	t.Cleanup(func() { welcomeShaders = true })
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.wel.on = true
	return a
}

// TestWelcome: the words rise, Başla opens the app, and the welcome is done.
func TestWelcome(t *testing.T) {
	a := welcomeApp(t)
	tt := ui.NewTester(a.view, 560, 660)
	tt.Advance(1500 * time.Millisecond)
	if !a.wel.chosen || !tt.HasText("Başla") || !tt.HasText("Ryudit") {
		t.Fatalf("the welcome is not up: %v", tt.Texts())
	}
	if !a.modal() {
		t.Fatal("the app takes keys under the welcome")
	}
	tt.Advance(2 * time.Second)
	if !a.wel.goAt.IsZero() {
		t.Fatal("the opening started without Başla")
	}
	// only the pill is the button: its glow, round it, is not
	r, _ := tt.Find("Başla")
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2+48)
	tt.Frame()
	if !a.wel.goAt.IsZero() {
		t.Fatal("a click in the glow, off the button, started the opening")
	}
	if err := tt.Click("Başla"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(3 * time.Second)
	if a.wel.on {
		t.Fatalf("the welcome did not end: %v", tt.Texts())
	}
	if !a.settings.WelcomeDone {
		t.Fatal("it will be shown again")
	}
}

// The settings start the welcome again: the button is in the appearance, it closes the settings, and the welcome is up from its start.
func TestWelcomeAgainFromSettings(t *testing.T) {
	a := welcomeApp(t)
	a.wel.on, a.settings.WelcomeDone = false, true
	tt := ui.NewTester(a.view, 900, 4200) // tall: the button is low on the page
	a.openSettings()
	a.settingsTab = tabAppearance
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Yeniden başlat"); err != nil {
		t.Fatalf("no button: %v %v", err, tt.Texts())
	}
	tt.Frame()
	if !a.wel.on || a.settingsOpen || a.settings.WelcomeDone || !a.wel.goAt.IsZero() {
		t.Fatalf("the welcome did not start again: on %v, settings %v, done %v", a.wel.on, a.settingsOpen, a.settings.WelcomeDone)
	}
	tt.Advance(500 * time.Millisecond)
	if !a.wel.chosen {
		t.Fatal("the welcome did not start again")
	}
}

// The looks of the welcome are chosen in the settings, from a list each: the choice is kept, and one that is no longer there is the first.
func TestWelcomeLooks(t *testing.T) {
	a := welcomeApp(t)
	a.wel.on = false
	tt := ui.NewTester(a.view, 900, 4200)
	a.openSettings()
	a.settingsTab = tabAppearance
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	pick := func(current, want string) {
		t.Helper()
		if err := tt.Click(current); err != nil { // the list opens
			t.Fatal(err)
		}
		tt.Advance(300 * time.Millisecond)
		if err := tt.Click(want); err != nil {
			t.Fatalf("no %q in the list: %v %v", want, err, tt.Texts())
		}
		tt.Advance(300 * time.Millisecond)
	}
	pick(welcomeGrounds[0].name, "Bulut")
	if a.settings.WelcomeGround != "bulut" || a.welcomeGroundLook().name != "Bulut" || a.welcomeOpeningLook().name != "Halka" {
		t.Fatalf("chosen: %q %q", a.settings.WelcomeGround, a.settings.WelcomeOpen)
	}
	a.settings.WelcomeGround, a.settings.WelcomeOpen = "yok", "yok"
	if a.welcomeGroundLook().id != welcomeGrounds[0].id || a.welcomeOpeningLook().id != welcomeOpenings[0].id {
		t.Fatal("a look that is not there is not the first")
	}
}

// Every look draws: each ground, and each opening halfway, in a small window, a frame or two each (the CPU renderer runs the
// shaders slowly). The clock is set back rather than run, so that the ground is all there and the opening is half done.
func TestWelcomeLooksDraw(t *testing.T) {
	type pair struct{ g, o int }
	var pairs []pair // each ground under the first opening, and each opening over the first ground
	for i := range welcomeGrounds {
		pairs = append(pairs, pair{i, 0})
	}
	for i := 1; i < len(welcomeOpenings); i++ {
		pairs = append(pairs, pair{0, i})
	}
	for _, pr := range pairs {
		g, o := welcomeGrounds[pr.g], welcomeOpenings[pr.o]
		{
			t.Run(g.name+"/"+o.name, func(t *testing.T) {
				a := welcomeApp(t)
				welcomeShaders = true
				a.settings.WelcomeGround, a.settings.WelcomeOpen = g.id, o.id
				tt := ui.NewTester(a.view, 48, 36)
				tt.Frame() // the first frame starts the clock
				now := a.wel.start
				a.wel.chosen, a.wel.start = true, now.Add(-5*time.Second) // Devam was pressed: the ground is there
				tt.Frame()
				a.wel.origin, a.wel.goAt = ui.Point{X: 24, Y: 28}, now.Add(-time.Duration(o.length()/2*float64(time.Second))) // Başla, as welcomeStart has it
				tt.Frame()
				if !a.wel.on || a.welcomeProgressAt(now) <= 0 || a.welcomeProgressAt(now) >= 1 {
					t.Fatalf("not halfway: on %v, progress %v", a.wel.on, a.welcomeProgressAt(now))
				}
			})
		}
	}
}

// Esc opens the app at once, with nothing after it.
func TestWelcomeSkip(t *testing.T) {
	a := welcomeApp(t)
	tt := ui.NewTester(a.view, 560, 660)
	tt.Advance(500 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.wel.on || !a.settings.WelcomeDone {
		t.Fatalf("the welcome did not end on Esc: on %v, done %v", a.wel.on, a.settings.WelcomeDone)
	}
}

// Showing or focusing the app's window from a frame that holds the app's lock (Başla does) makes the system answer with the focus in that
// same call: the answer must not wait for the lock, or the window freezes.
func TestWindowFocusDoesNotWaitForTheLock(t *testing.T) {
	a := welcomeApp(t)
	a.mu.Lock()
	done := make(chan struct{})
	go func() { a.windowIn(); a.windowOut(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		a.mu.Unlock()
		t.Fatal("the focus handlers wait for the lock a frame holds")
	}
	a.mu.Unlock()
	for i := 0; i < 200; i++ { // and they do their work once the frame lets go
		a.mu.Lock()
		behind := a.sy.behind
		a.mu.Unlock()
		if behind {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("going behind was not noted")
}
