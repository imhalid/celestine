package main

import (
	"github.com/imhalid/celestine/locales"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The marine grins as it is written to fast and turns gold on a long run, flinches and bursts the letters
// erased into pixels, flashes at a space, and takes wounds left alone, which writing heals (#284).
func TestMarineMoods(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(700, 420)
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == "Marine" {
			a.setCaret(k)
		}
	}
	if caretName(a.caretKind) != "Marine" {
		t.Fatal("no Marine among the carets")
	}
	tt.Frame()
	g := a.fxNamed("Marine").(*marine)
	for i := 0; i < 50; i++ {
		tt.Type("a")
		tt.Advance(60 * time.Millisecond)
	}
	if g.rate < 4 || g.fast < 0 {
		t.Fatalf("typing fast: rate %.1f, fast since %v", g.rate, g.fast)
	}
	tn := g.last
	if !(g.fast >= 0 && tn-g.fast > 1500) {
		t.Fatalf("a long run is not gold: fast since %v, now %v", g.fast, tn)
	}
	tt.Type(" ")
	tt.Advance(20 * time.Millisecond)
	if g.last-g.shot > 140 {
		t.Fatal("a space shot nothing")
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyBackspace)
	tt.Advance(30 * time.Millisecond)
	if g.last-g.ouch > 450 || len(g.gibs) == 0 {
		t.Fatalf("an erase: hurt %v ago, %d gibs", g.last-g.ouch, len(g.gibs))
	}
	if g.gaze() > 0 {
		t.Fatalf("erasing back, it looks at %v", g.gaze())
	}
	tt.Advance(60 * time.Second)
	if g.idle(g.last) < 45 {
		t.Fatalf("left alone a minute: idle %v", g.idle(g.last))
	}
	tt.Type("x")
	tt.Advance(50 * time.Millisecond)
	if g.idle(g.last) > 1 {
		t.Fatal("writing did not heal it")
	}
}

// In the settings' field to try a caret, written to its end, the marine stays in the field (#284).
func TestMarineStaysInTheField(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == "Marine" {
			a.setCaret(k)
		}
	}
	a.settingsOpen, a.settingsTab = true, 1
	tt.Advance(900 * time.Millisecond)
	if err := tt.Click(locales.EditorTryCaret()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 90; i++ {
		tt.Type("a")
		tt.Advance(20 * time.Millisecond)
	}
	tt.Advance(200 * time.Millisecond)
	field, ok := tt.Find(locales.EditorTryCaret())
	g := a.fxNamed("Marine").(*marine)
	if !ok || g.drawn.W == 0 {
		t.Fatalf("no field %v or no marine %v", ok, g.drawn)
	}
	if g.drawn.X+g.drawn.W > field.X+field.W+0.5 {
		t.Fatalf("the marine %v goes out of the field %v", g.drawn, field)
	}
}
