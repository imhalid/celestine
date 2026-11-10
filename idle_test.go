package main

import (
	"testing"
	"time"
)

// A window left with the caret in its text comes to rest: the caret moves of itself a while (it breathes, sleeps,
// blinks), then is drawn no more until something happens (#188). One caret of each way of asking for frames: the
// slime's spring, the mizan's breath, the plain line, the marine's blink, and a preset's idleFrame. The pointer rests
// over the page's scroll area, which asks for none either (#189).
func TestIdleAsksNoFrames(t *testing.T) {
	for _, name := range []string{"Slime", "Mizan", "Line", "Marine", "Kawaii"} {
		k := caretKind(-1)
		for i := caretKind(0); i < caretCount(); i++ {
			if caretName(i) == name {
				k = i
			}
		}
		if k < 0 {
			t.Fatalf("no caret %q", name)
		}
		a, tt := newTestApp(t, "Bir paragraf, okunacak kadar uzun.\n\nİkincisi.\n")
		a.setCaret(k)
		a.doc.SetSelection(5, 5)
		tt.Frame()
		tt.Move(450, 300)
		tt.Advance(caretRests() + 30*time.Second) // what moved last (a blink, a wound) is done by then
		n, v := a.caretDraws, a.frameSeq
		tt.Advance(5 * time.Second)
		if d := a.caretDraws - n; d > 1 || a.frameSeq != v { // one paint a while is a timer's, not a frame at each refresh
			t.Errorf("%s: %d paints and %d views in five seconds at rest", name, d, a.frameSeq-v)
		}
	}
}
