package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// ↑ from a heading goes to the paragraph above, as from any line.
func TestUpFromHeading(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\n# Başlık\n\n## Alt\n\n### Üç\n\nSon\n")
	tt.Frame()
	for _, s := range []string{"Başlık", "Alt", "Üç", "Son"} {
		p := at(a.pane().doc, s) + 2
		a.pane().doc.SetSelection(p, p)
		tt.Frame()
		tt.Key(0, ui.KeyUp)
		tt.Frame()
		_, c := a.pane().doc.Selection()
		if c >= at(a.pane().doc, s) {
			t.Errorf("%s: ↑ from %d went to %d, not above (paragraph at %d)", s, p, c, at(a.pane().doc, s))
		}
	}
}
