package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// TestLightLevels: in Ryugo's own light theme the page is the white card, a group lies a step under it and a
// control's well a step under the group (#491); in the dark theme a control still has a ground of its own in a group.
func TestLightLevels(t *testing.T) {
	for _, mode := range []ryugo.Mode{ryugo.Light, ryugo.Dark} {
		var cl ryugo.Colors
		tt := ui.NewTester(func(c *ui.Context) {
			tok := ryugo.Root(c, ryugo.Options{Mode: mode})
			lightLevels(tok)
			cl = tok.Colors
		}, 200, 200)
		tt.Advance(16 * time.Millisecond)
		page, group, well := luma(cl.Card), luma(groupBg(cl)), luma(cl.Well)
		if mode == ryugo.Light {
			if page < 0.99 || page-group < 0.01 || group-well < 0.01 {
				t.Fatalf("light: page %.3f, group %.3f, well %.3f: not a step apart", page, group, well)
			}
		} else if well-group < 0.01 {
			t.Fatalf("dark: group %.3f, well %.3f: a field has no ground of its own", group, well)
		}
	}
	// the app's page in Ryugo's own light is that card
	a, tt := newTestApp(t, "x\n")
	a.settings.Theme = 1
	tt.Advance(300 * time.Millisecond)
	a.mu.Lock()
	defer a.mu.Unlock()
	if luma(a.pageBg) < 0.99 {
		t.Fatalf("the light page is %.3f, not the white card", luma(a.pageBg))
	}
}
