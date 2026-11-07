package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The jelly: when an agent (MCP) adds to a page, the window itself shakes from
// side to side like a jelly, hopping as it goes, and settles where it was - the
// more it was given, the harder it shakes. The window is moved by a goroutine of
// its own, a step every frame, so that a slow frame of the view does not slow it
// (and the view, which holds the app's lock, never waits on the window). Where
// a window cannot be moved (a maximized one, a full screen one, and Wayland, which
// leaves a window's place to the compositor) the shake is laid on what is drawn in
// the window instead: a slide and a lean and a squeeze of the whole view.

const (
	wobbleLen  = 1600 * time.Millisecond
	wobbleHz   = 3.4  // swings in a second
	wobbleDamp = 3.0  // how fast they die away, per second
	wobbleMin  = 3.0  // the least it slides, in px, and
	wobbleMax  = 10.0 // the most
	hopHz      = 1.7  // hops in a second: one for every two swings
	winGain    = 1.6  // the window swings this much wider than what is drawn in it
	hopGain    = 1.1  // and hops this high, of the swing
)

type wobble struct {
	at  time.Time
	amp float32
}

// windowMover is what the shake needs of a window: where it is, to move it, and whether it can be.
type windowMover interface {
	Position() (x, y int)
	SetPosition(x, y int)
	IsMaximized() bool
	IsFullScreen() bool
	IsMinimized() bool
}

// wobbleState is the shake as it is now; it is read by the view and by the window's goroutine.
func (a *app) wobbleState() wobble {
	a.wobMu.Lock()
	defer a.wobMu.Unlock()
	return a.wob
}

// jiggle shakes the window for n characters of text added. A shake already going
// on is taken over by the stronger of the two.
func (a *app) jiggle(n int) {
	if a.settings.NoWobble || n <= 0 {
		return
	}
	amp := float32(math.Min(wobbleMax, wobbleMin+math.Sqrt(float64(n))/6))
	now := time.Now()
	a.wobMu.Lock()
	if left := wobbleAmpOf(a.wob, now); left > amp {
		amp = left
	}
	a.wob = wobble{at: now, amp: amp}
	a.wobMu.Unlock()
	a.shakeWindow()
}

// wobbleAmpOf is how far the shake w slides at now, at its widest swing.
func wobbleAmpOf(w wobble, now time.Time) float32 {
	t := now.Sub(w.at)
	if w.at.IsZero() || t < 0 || t > wobbleLen {
		return 0
	}
	return w.amp * float32(math.Exp(-wobbleDamp*t.Seconds()))
}

// swingOf is the shake w at now: how far it slides, its lean in degrees, the squeeze in the
// two directions and how high it hops; on is false once it is still.
func swingOf(w wobble, now time.Time) (dx, lean, sx, sy, hop float32, on bool) {
	amp := wobbleAmpOf(w, now)
	if amp < 0.05 {
		return 0, 0, 1, 1, 0, false
	}
	t := now.Sub(w.at).Seconds()
	phase := 2 * math.Pi * wobbleHz * t
	osc, turn := float32(math.Sin(phase)), float32(math.Cos(phase))
	dx = amp * osc
	lean = -0.09 * amp * turn // leans against the way it is going: its top lags
	k := 0.0035 * amp
	hop = amp * float32(math.Abs(math.Sin(2*math.Pi*hopHz*t))) // up and down on its landing
	return dx, lean, 1 + k*absf(turn), 1 - k*0.6*absf(turn), hop, true
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// shake lays the shake on what is drawn in the window, while the window itself
// is not the one moving, and asks for the frames of it.
func (a *app) shake(c *ui.Context, root ui.Element) {
	dx, lean, sx, sy, _, on := swingOf(a.wobbleState(), time.Now())
	if !on {
		return
	}
	c.After(16 * time.Millisecond)
	if a.winShaking.Load() {
		return // the window moves: what is in it stays
	}
	root.Translate(dx, 0).Skew(lean, 0).Scale(sx, sy)
}

// shakeWindow moves the window with the shake, if it can be moved and is not
// being moved already. The goroutine stops when the shake has died away, and puts
// the window back where it was; or, when the user takes hold of the window and moves
// it, where they put it.
func (a *app) shakeWindow() {
	m := a.mover
	if m == nil || a.winBusy.Swap(true) {
		return
	}
	go func() {
		defer a.guard()
		defer a.winBusy.Store(false)
		defer a.winShaking.Store(false)
		if m.IsMaximized() || m.IsFullScreen() || m.IsMinimized() {
			return
		}
		a.winShaking.Store(true) // the window is the one that moves, until it shows it does not
		hx, hy := m.Position()
		lx, ly := hx, hy // where the last step put it
		for step := 0; ; step++ {
			time.Sleep(16 * time.Millisecond)
			dx, _, _, _, hop, on := swingOf(a.wobbleState(), time.Now())
			// not where the last step put it: the user has it, or the window does not obey; either way, leave it be
			if cx, cy := m.Position(); step > 0 && (absInt(cx-lx) > 3 || absInt(cy-ly) > 3) {
				return
			}
			if !on {
				m.SetPosition(hx, hy)
				return
			}
			nx := max(hx+int(math.Round(float64(dx*winGain))), 0)
			ny := max(hy-int(math.Round(float64(hop*winGain*hopGain))), 0)
			m.SetPosition(nx, ny)
			lx, ly = nx, ny
		}
	}()
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
