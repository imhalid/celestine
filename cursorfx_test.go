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

// The follower comes to the pointer; the ripple rings a press; the rocket, clicked, locks on, strikes and bursts.
func TestCursorFXFollowAndStrike(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(800, 500)
	a.settings.CursorFX = "Follower"
	tt.Move(100, 100)
	tt.Advance(50 * time.Millisecond)
	tt.Move(400, 300)
	tt.Advance(2 * time.Second)
	f := a.cursor.fx.(*follower)
	if math.Hypot(f.s.x-410, f.s.y-312) > 1 {
		t.Fatalf("the follower is at %.0f,%.0f, not by the pointer", f.s.x, f.s.y)
	}

	a.settings.CursorFX = "Ripple"
	tt.Frame()
	tt.Press(300, 200)
	tt.Advance(16 * time.Millisecond)
	if r := a.cursor.fx.(*ripples); len(r.rings) != 1 {
		t.Fatalf("a press rang %d rings", len(r.rings))
	}
	tt.Release(300, 200)

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
	hit := false
	for i := 0; i < 60 && !hit; i++ {
		tt.Advance(16 * time.Millisecond)
		hit = r.flash > 0
	}
	if !hit || r.locked {
		t.Fatalf("locked on, the rocket did not strike: flash %.2f, locked %v", r.flash, r.locked)
	}
}
