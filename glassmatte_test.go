package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// TestGlassMatteBehind: the window behind another one is matte, its glass off until it is in front again; and over
// the glass the light theme's surfaces are nearly opaque, the dark one's as they were (#491).
func TestGlassMatteBehind(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	waitBehind := func(want bool) bool { // the window's events are taken on a goroutine of their own
		for i := 0; i < 200; i++ {
			a.mu.Lock()
			b := a.winBehind
			a.mu.Unlock()
			if b == want {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return false
	}
	a.windowOut()
	if !waitBehind(true) {
		t.Fatal("not behind after the window went behind")
	}
	a.windowIn()
	if !waitBehind(false) {
		t.Fatal("still behind in front")
	}
	for _, dark := range []bool{false, true} {
		var got float32
		mode := map[bool]ryugo.Mode{false: ryugo.Light, true: ryugo.Dark}[dark]
		tv := ui.NewTester(func(c *ui.Context) {
			ryugo.Root(c, ryugo.Options{Mode: mode})
			got = glassOpacity(c, 0.5)
		}, 100, 100)
		tv.Frame()
		if want := map[bool]float32{false: 0.95, true: 0.5}[dark]; got != want {
			t.Fatalf("dark %v: %v, want %v", dark, got, want)
		}
	}
}
