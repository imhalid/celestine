package main

import (
	"testing"
	"time"
)

// TestWindowZoom: Zoom in and out go through the steps, Actual size goes back to none, the zoom is kept in the
// settings, and the window is drawn at it; the editor's text size is a step of its own (#350).
func TestWindowZoom(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.mu.Lock()
	a.stepZoom(1)
	a.stepZoom(1)
	z, font := a.settings.Zoom, a.fontSize()
	a.mu.Unlock()
	if z != 1.25 {
		t.Fatalf("two steps in: %v", z)
	}
	tt.Advance(100 * time.Millisecond)
	a.mu.Lock()
	a.settings.Zoom = 1.3 // between two steps: out goes to the one below
	a.stepZoom(-1)
	z = a.settings.Zoom
	a.setZoom(1)
	none, after := a.settings.Zoom, a.fontSize()
	a.mu.Unlock()
	if z != 1.25 || none != 0 {
		t.Fatalf("out from 1.3: %v; actual size: %v", z, none)
	}
	if after != font {
		t.Fatal("the zoom changed the editor's text size")
	}
	for _, c := range appCommands() {
		if c.title == "biggerText" && c.accel != "alt+cmd+=" || c.title == "view_zoomIn" && c.accel != "cmd+=" {
			t.Fatalf("%s is on %q", c.title, c.accel)
		}
	}
}
