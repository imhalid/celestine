package main

import (
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
	x := a.sidebarWidth() - 2
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
