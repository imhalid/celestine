package main

import (
	"testing"
	"time"
)

// A bar is across the top of a page, and of the views that do not scroll
// under it; no path is in it.
func TestTopBar(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find("Üst çubuk"); !ok {
		t.Fatalf("no bar over the page: %v", tt.Texts())
	}
	if tt.HasText("Test ›") || tt.HasText("›") {
		t.Fatalf("a path is in the bar: %v", tt.Texts())
	}
	a.tasksOpen = true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find("Üst çubuk"); !ok {
		t.Fatal("no bar in the tasks")
	}
}

// The caret of a text input (a table's cell, the title) is found by laying
// its text out: further along as the caret is, a letter's height high.
func TestInputCaret(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	r0, _ := a.inputCaret("elma armut", 0, 16, 400, 10, 0, 34)
	r1, cw := a.inputCaret("elma armut", 4, 16, 400, 10, 0, 34)
	r2, _ := a.inputCaret("elma armut", 10, 16, 400, 10, 0, 34)
	if r0.X != 10 || !(r1.X > r0.X && r2.X > r1.X) || r0.H < 10 || r0.H > 34 || cw <= 0 {
		t.Fatalf("%+v %+v %+v cw %v", r0, r1, r2, cw)
	}
	if r0.Y <= 0 || r0.Y+r0.H > 34 {
		t.Fatalf("not in the middle of the box: %+v", r0)
	}
}

// Light to dark is one change, not a fade of each part at its own pace: the
// frame after it shows the colors it ends in.
func TestThemeSwitchIsNotAnimated(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(800, 500)
	a.settings.Theme = 1
	tt.Advance(500 * time.Millisecond)
	a.settings.Theme = 2
	tt.Frame()
	first := tt.Image()
	tt.Advance(800 * time.Millisecond)
	last := tt.Image()
	for _, p := range [][2]int{{20, 300}, {120, 105}, {500, 400}, {20, 470}, {600, 250}} {
		if first.At(p[0], p[1]) != last.At(p[0], p[1]) {
			t.Fatalf("at %v: %v on the first frame, %v after: it fades", p, first.At(p[0], p[1]), last.At(p[0], p[1]))
		}
	}
}
