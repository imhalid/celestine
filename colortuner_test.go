package main

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// TestColorTuner: a color changed in the tuner is the app's on the next frame, what was there before comes back with
// Reset, and Copy all's text has every value, the changed one saying what it was.
func TestColorTuner(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.mu.Lock()
	a.settings.Theme = 1 // light
	a.startTuner()
	a.mu.Unlock()
	tt.Frame()
	tv := ui.NewTester(a.tunerView, 760, 760)
	tv.Frame()
	if err := tv.Click("ryugo.Card"); err != nil {
		t.Fatal(err, tv.Texts())
	}
	tv.Frame()
	a.mu.Lock()
	look := a.tunerLook(a.tuner.dark)
	if a.tuner.sel != "ryugo.Card" {
		t.Fatalf("chosen %q", a.tuner.sel)
	}
	red := ui.RGBA(255, 0, 0, 1)
	a.tuner.colors[look] = map[string]ui.Color{"ryugo.Card": red}
	a.tuner.alphas[look] = map[string]float32{"glass.card": 0.3}
	a.mu.Unlock()
	tt.Frame()
	a.mu.Lock()
	var card ui.Color
	for tok := range a.tuner.snaps {
		if !tok.Dark {
			card = tok.Colors.Card
		}
	}
	items := a.tunerItems(look)
	text := tunerText(look, items)
	a.mu.Unlock()
	if card != red {
		t.Fatalf("the app's card is %v", card)
	}
	for _, it := range items {
		if !strings.Contains(text, it.key) {
			t.Fatalf("%s not copied:\n%s", it.key, text)
		}
	}
	if len(items) < 40 || !strings.Contains(text, "ryugo.Card") || !strings.Contains(text, "#ff0000  (changed; was #") {
		t.Fatalf("copied:\n%s", text)
	}
	tv.Frame()
	if file := os.Getenv("SHOT"); file != "" {
		f, _ := os.Create(file)
		png.Encode(f, tv.Image())
		f.Close()
	}
	if err := tv.Click(locales.TunerReset()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.mu.Lock()
	defer a.mu.Unlock()
	for tok, s := range a.tuner.snaps {
		if !tok.Dark && tok.Colors.Card != s.ryugo["Card"] {
			t.Fatalf("not reset: %v", tok.Colors.Card)
		}
	}
}
