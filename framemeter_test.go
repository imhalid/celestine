package main

import (
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestDebugMode: the palette's command turns the debug mode on, and its card shows over the window with the frames
// it counted; at rest the card asks for no frame of its own, and the command turns it off again (#396).
func TestDebugMode(t *testing.T) {
	a, tt := newTestApp(t, "Bir paragraf.\n")
	tt.Frame()
	if tt.HasText(locales.DebugTitle()) {
		t.Fatal("the debug card shows before the debug mode is on")
	}
	if !a.runCommand(locales.DebugToggle()) {
		t.Fatal("no command to turn the debug mode on")
	}
	tt.Frame()
	for range 5 { // a few frames, each built
		a.mu.Lock()
		a.frame()
		a.mu.Unlock()
		tt.Advance(16 * time.Millisecond)
	}
	if !tt.HasText(locales.DebugTitle()) {
		t.Fatalf("no debug card: %v", tt.Texts())
	}
	a.meter.mu.Lock()
	s, n := a.meter.stats(), a.meter.n
	a.meter.mu.Unlock()
	if n < 5 || s.builds == 0 || s.paints == 0 || s.fps == 0 {
		t.Errorf("the card counted %d frames: %+v", n, s)
	}

	tt.Advance(30 * time.Second) // whatever moved is done by then
	v := a.frameSeq
	a.meter.mu.Lock()
	n = a.meter.n
	a.meter.mu.Unlock()
	tt.Advance(5 * time.Second)
	a.meter.mu.Lock()
	painted := a.meter.n - n
	a.meter.mu.Unlock()
	if a.frameSeq-v > 1 || painted > 1 { // one a while is a timer's, as with the debug mode off; not one at each refresh
		t.Errorf("at rest with the debug mode on: %d views and %d paints in five seconds", a.frameSeq-v, painted)
	}

	a.runCommand(locales.DebugToggle())
	tt.Frame()
	if tt.HasText(locales.DebugTitle()) {
		t.Fatal("the debug card stays after the debug mode is off")
	}
}
