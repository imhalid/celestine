package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestCurvedTrack: a bar against a card's rounded corners bends a little way round the top corner, runs down the
// side and bends a little way round the bottom corner, inside the area; a square corner leaves its arc out.
func TestCurvedTrack(t *testing.T) {
	r := ui.Rect{X: 100, Y: 50, W: 400, H: 300}
	tr := curvedTrack(r, 12, 12)
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }
	side := r.X + r.W - funBarLine
	x, y := tr.at(0)
	if x >= side || x < side-12 || y <= r.Y+funBarLine || y >= r.Y+funBarLine+12 {
		t.Fatalf("starts at %v,%v: not on the top corner's arc", x, y)
	}
	x, y = tr.at(tr.arcT)
	if !near(x, side) || !near(y, r.Y+funBarLine+12) {
		t.Fatalf("round the corner at %v,%v: not on the side", x, y)
	}
	x, y = tr.at(tr.length())
	if x >= side || y >= r.Y+r.H-funBarLine || y <= r.Y+r.H-funBarLine-12 {
		t.Fatalf("ends at %v,%v: not on the bottom corner's arc", x, y)
	}
	if b := tr.path().Bounds(); b.X+b.W > side+0.01 || b.Y < r.Y || b.Y+b.H > r.Y+r.H || b.X < side-12 {
		t.Fatalf("the path %v goes past the line, or along the edges", b)
	}
	sq := curvedTrack(r, 0, 12) // a square top: straight from the top down
	if sq.arcT != 0 || sq.top != r.Y+funBarInset || sq.arcB == 0 {
		t.Fatalf("square top: %+v", sq)
	}
	if x, y := sq.at(0); !near(x, sq.x) || !near(y, r.Y+funBarInset) {
		t.Fatalf("square top starts at %v,%v, not at the side's top", x, y)
	}
}
