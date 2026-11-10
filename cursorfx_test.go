package main

import (
	"math"
	"testing"
	"time"
)

// The pointer's effects are off until one is chosen; each, chosen, goes after the pointer and comes to rest
// when it stops, asking for no more frames then (#287).
func TestCursorFX(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(800, 500)
	tt.Frame()
	if a.settings.CursorFX != "" || a.cursor.fx != nil {
		t.Fatalf("a pointer effect by default: %q", a.settings.CursorFX)
	}
	for _, p := range cursorPresets {
		a.settings.CursorFX = p.name
		tt.Move(200, 200)
		tt.Advance(100 * time.Millisecond)
		for i := 0; i < 20; i++ {
			tt.Move(200+float32(i)*15, 200+float32(i)*6)
			tt.Advance(16 * time.Millisecond)
		}
		if a.cursor.fx == nil || a.cursor.name != p.name {
			t.Fatalf("%s: not made", p.name)
		}
		tt.Advance(6 * time.Second)
		if a.cursor.moving {
			t.Errorf("%s: still asks for frames with the pointer still", p.name)
		}
	}
	a.settings.CursorFX = ""
	tt.Frame()
	if a.cursor.fx != nil {
		t.Fatal("turned off, the effect stays")
	}
}

// The rocket comes to rest beside the pointer; clicked, it locks on the place clicked, strikes there and
// bursts, however the pointer goes on.
func TestCursorFXStrike(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(800, 500)
	a.settings.CursorFX = "Rocket"
	tt.Move(400, 250)
	tt.Advance(3 * time.Second)
	r := a.cursor.fx.(*rocket)
	if math.Hypot(r.x-420, r.y-272) > 2 {
		t.Fatalf("the rocket rests at %.0f,%.0f, not beside the pointer", r.x, r.y)
	}
	tt.Press(400, 250)
	tt.Advance(16 * time.Millisecond)
	tt.Release(400, 250)
	if !r.locked {
		t.Fatal("a click did not lock the rocket on")
	}
	tt.Move(700, 60) // locked on, it keeps to the place clicked, not to the pointer
	tt.Advance(16 * time.Millisecond)
	hit := false
	for i := 0; i < 60 && !hit; i++ {
		tt.Advance(16 * time.Millisecond)
		hit = r.flash > 0
	}
	if !hit || r.locked {
		t.Fatalf("locked on, the rocket did not strike: flash %.2f, locked %v", r.flash, r.locked)
	}
	if r.bx != 400 || r.by != 250 {
		t.Fatalf("it burst at %.0f,%.0f, not where it was locked on", r.bx, r.by)
	}
}
