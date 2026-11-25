package main

import (
	"strings"
	"testing"
	"time"
)

// An agent's writing sends the shimmer across, brighter for more text, and it is gone after.
func TestShimmer(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if a.wobbleState().on {
		t.Fatal("a shimmer before anything was written")
	}
	a.jiggle(0)
	if a.wobbleState().on {
		t.Fatal("a shimmer for nothing")
	}
	a.jiggle(20)
	small := a.wobbleState().strength
	a.jiggle(100000)
	w := a.wobbleState()
	if !w.on || w.strength <= small || w.strength > shimmerMax {
		t.Fatalf("strengths: %v then %v", small, w.strength)
	}
	tt.Frame() // a write from outside: the view is drawn again
	if a.wobbleState().at.IsZero() {
		t.Fatal("not timed from the frame that drew it")
	}
	tt.Advance(shimmerLen)
	tt.Advance(16 * time.Millisecond)
	if a.wobbleState().on {
		t.Fatal("still shimmering")
	}
	a.settings.NoWobble = true
	a.jiggle(500)
	if a.wobbleState().on {
		t.Fatal("a shimmer with it off")
	}
	a.shimmer(500) // the settings' button tries it all the same
	if !a.wobbleState().on {
		t.Fatal("the button sent none")
	}
}

// A page written by an agent is what sends it.
func TestShimmerOnWrite(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	if err := a.writeBody(a.pane().page, strings.Repeat("kelime ", 200)); err != nil {
		t.Fatal(err)
	}
	if !a.wobbleState().on {
		t.Fatal("writing sent no shimmer")
	}
}

// The band comes in from before the top left corner and leaves past the bottom right.
func TestShimmerHead(t *testing.T) {
	const w, h = 1000, 700
	if from, to := shimmerHead(0, w, h), shimmerHead(1, w, h); from > -shimmerW || to < w+shimmerTip*h+shimmerW {
		t.Fatalf("from %v to %v", from, to)
	}
	if shimmerHead(0.3, w, h) >= shimmerHead(0.6, w, h) {
		t.Fatal("it goes back")
	}
}
