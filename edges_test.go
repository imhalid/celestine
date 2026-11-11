package main

import (
	"path/filepath"
	"testing"
	"time"
)

// The sidebar's edge is dragged: it gets wider, within what a window allows, and is kept; narrow, it
// leaves the text its room.
func TestSidebarEdgeDrag(t *testing.T) {
	a, tt := newTestApp(t, "Sayfa\n")
	tt.SetSize(1400, 800)
	tt.Advance(400 * time.Millisecond)
	if w := a.sidebarWidth(); w != sidebarW {
		t.Fatalf("starts at %v", w)
	}
	x := frameW + a.sidebarWidth() - 2 // the sidebar starts inside the window's frame (windowframe.go)
	tt.Press(x, 300)
	tt.Move(x+60, 300)
	tt.Move(x+120, 300)
	tt.Release(x+120, 300)
	tt.Advance(400 * time.Millisecond)
	if w := a.sidebarWidth(); w < sidebarW+80 {
		t.Fatalf("not dragged wider: %v (setting %d)", w, a.settings.SidebarWidth)
	}
	// a small window: the sidebar gives way
	tt.SetSize(520, 700)
	tt.Advance(400 * time.Millisecond)
	if w := a.sidebarWidth(); w > 520*sidebarShare+1 && w > sidebarMin {
		t.Fatalf("sidebar %v in a window of 520", w)
	}
}

// Dragged past the most, the sidebar stops at sideMax; let go, its width is written to the settings and read
// back as after a restart (#369).
func TestSidebarEdgeClampedAndKept(t *testing.T) {
	a, tt := newTestApp(t, "Sayfa\n")
	now := time.Now()
	a.clock = func() time.Time { return now }
	a.settingsStore = nil
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	tt.SetSize(2400, 800) // wide enough that the window's share is more than sideMax
	tt.Advance(400 * time.Millisecond)
	x := frameW + a.sidebarWidth() - 2
	tt.Press(x, 300)
	for dx := float32(100); dx <= 600; dx += 100 {
		tt.Move(x+dx, 300)
	}
	if w := a.sidebarWidth(); w != sideMax {
		t.Fatalf("dragged past the most: %v (setting %d)", w, a.settings.SidebarWidth)
	}
	tt.Release(x+600, 300)
	now = now.Add(time.Second) // the settings are written a moment after the last change (keepSettings)
	tt.Advance(400 * time.Millisecond)
	if a.settingsDirty {
		t.Fatal("not written")
	}
	a.settings.SidebarWidth = 0
	a.loadSettings() // a restart reads the settings again
	if w := a.sidebarWidth(); w != sideMax {
		t.Fatalf("not kept: %v (setting %d)", w, a.settings.SidebarWidth)
	}
}

func TestSideWidthLimits(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.winRect.W = 2000
	if w := a.sideWidth(9000, sidebarW, sidebarMin, sidebarShare); w != sideMax {
		t.Fatalf("too wide: %v", w)
	}
	if w := a.sideWidth(10, sidebarW, sidebarMin, sidebarShare); w != sidebarMin {
		t.Fatalf("too narrow: %v", w)
	}
	a.winRect.W = 400
	if w := a.sideWidth(400, panelW, panelMin, panelShare); w != panelMin {
		t.Fatalf("small window: %v", w)
	}
}
