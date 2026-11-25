package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Every caret draws through typing, erasing, a jump and a page change, and the
// cycle of ⇧⌘M goes through them all.
func TestCaretKindsDraw(t *testing.T) {
	for k := caretKind(0); k < caretCount(); k++ {
		a, tt := newTestApp(t, "")
		a.caretKind = k
		tt.SetSize(700, 420)
		tt.Frame()
		typeEach(tt, "merhaba dünya Ali")
		tt.Key(0, ui.KeyEnter)
		typeEach(tt, "bir iki ")
		tt.Advance(80 * time.Millisecond)
		tt.Key(0, ui.KeyBackspace)
		tt.Key(0, ui.KeyBackspace)
		tt.Advance(80 * time.Millisecond)
		a.pane().doc.SetSelection(0, 0)
		tt.Frame()
		tt.Advance(120 * time.Millisecond)
		a.setCaret((k + 1) % caretCount())
		if a.caretKind != (k+1)%caretCount() {
			t.Fatalf("the cycle: %d", a.caretKind)
		}
	}
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == "" {
			t.Fatalf("caret %d has no name", k)
		}
	}
}

// The settings show every caret as a tile.
func TestCaretTiles(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.pane().settingsOpen, a.pane().settingsTab = true, 1
	tt.Advance(900 * time.Millisecond)
	for k := caretKind(0); k < caretCount(); k++ {
		if !tt.HasText(msg(caretName(k))) { // shown in the window's language: Line is Çizgi
			t.Fatalf("no tile for %s: %v", caretName(k), tt.Texts())
		}
	}
}

// TestCaretSleepShapes: a z takes a few sizes and tilts over its life, whatever its seed, so the masks MyGo keeps
// for them repeat; a shape new at each frame filled MyGo's atlas, which painted the whole window again (#558).
func TestCaretSleepShapes(t *testing.T) {
	shapes := map[[2]float64]bool{}
	for n := range 20 {
		seed := float64(n) * 1.7
		for age := 0.0; age < 1; age += 1.0 / 240 {
			shapes[[2]float64{snapZ(age, sleepZSizes), snapZ(math.Sin(age*4+seed), 1)}] = true
		}
	}
	if len(shapes) > 3*(sleepZSizes+1) {
		t.Errorf("the z's take %d shapes, more than %d", len(shapes), 3*(sleepZSizes+1))
	}
}

// Five seconds of rest, and the caret dozes: z's come out; a key and they are gone.
func TestCaretSleeps(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(700, 420)
	tt.Frame()
	typeEach(tt, "merhaba")
	tt.Advance(100 * time.Millisecond)
	if len(a.sleep.zs) != 0 {
		t.Fatalf("z's while writing: %d", len(a.sleep.zs))
	}
	tt.Advance(4 * time.Second)
	if len(a.sleep.zs) != 0 {
		t.Fatalf("z's before five seconds: %d", len(a.sleep.zs))
	}
	tt.Advance(2 * time.Second)
	if len(a.sleep.zs) == 0 {
		t.Fatal("no z's after the rest")
	}
	typeEach(tt, "x")
	tt.Frame()
	if len(a.sleep.zs) != 0 {
		t.Fatalf("z's after a key: %d", len(a.sleep.zs))
	}
}

// The jelly, on, stretches a caret that is not the slime; every caret still draws.
func TestJellyOnEveryCaret(t *testing.T) {
	for k := caretKind(0); k < caretCount(); k++ {
		a, tt := newTestApp(t, "")
		a.settings.Jelly = true
		a.caretKind = k
		tt.SetSize(700, 420)
		tt.Frame()
		typeEach(tt, "merhaba dünya")
		tt.Key(0, ui.KeyEnter)
		typeEach(tt, "bir iki")
		tt.Advance(80 * time.Millisecond)
	}
}

// The caret takes the look of the text it is in.
func TestCaretStyleFollowsText(t *testing.T) {
	a, tt := newTestApp(t, "(a **kalın** _eğik_ ~~çizili~~ [b {c")
	tt.SetSize(700, 420)
	tt.Frame()
	for _, c := range []struct {
		find    string
		after   bool // the caret after it
		strike  bool
		bracket rune
	}{
		{"(a ", true, false, '('},
		{"kalın", true, false, '('},
		{"çizili", true, true, '('},
		{"[b ", true, false, '['},
		{"{c", true, false, '{'},
	} {
		s := a.pane().doc.Text().String()
		i := strings.Index(s, c.find)
		if i < 0 {
			t.Fatalf("no %q in %q", c.find, s)
		}
		at := len([]rune(s[:i])) + len([]rune(c.find))
		a.pane().doc.SetSelection(at, at)
		tt.Frame()
		st := a.styleAt(at)
		if st.strike != c.strike || st.bracket != c.bracket {
			t.Errorf("after %q: %+v", c.find, st)
		}
	}
}
