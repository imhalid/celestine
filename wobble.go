package main

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The shimmer: when an agent (MCP) adds to a page, a band of light sweeps once
// across the whole view, from its top left corner to its bottom right, and is
// gone; the window stays where it is. The band lightens what it passes over, the
// page's ground and its letters alike, so that on a light ground the letters
// shine and on a dark one the ground does too. The more was added, the brighter
// it is. A write while one goes on brightens it, and one past its middle sends
// it again from the start.

const (
	shimmerLen = 1100 * time.Millisecond
	shimmerMin = 0.10 // how far the band lightens at its middle, for a little, and
	shimmerMax = 0.24 // for a lot
	shimmerW   = 140  // the band's half width, in DIPs
	shimmerTip = 0.45 // the slope of the band: its line goes this much down for each across
)

// shimmerDraws is false in tests: the tester draws on the CPU, where a shader over the whole
// window takes seconds a frame. The sweep is kept all the same.
var shimmerDraws = !testing.Testing()

var shimmerShader = shader.MustCompile(`package main

var Head float // where the band's middle is, along the sweep, in DIPs
var Width float
var Tip float
var Strength float

func ColorEffect(pos vec2, color vec4) vec4 {
	u := pos.x + Tip*pos.y
	d := (u - Head) / Width
	band := exp(-d * d) * Strength
	// toward white, as much of it as the pixel has: what is see-through stays so
	return vec4(mix(color.rgb, vec3(color.a), band), color.a)
}
`)

// wobble is the shimmer going on: when it started (zero until the frame that first draws it:
// it is timed on the view's clock, which a write from an agent's goroutine does not have), and
// how bright it is.
type wobble struct {
	at       time.Time
	strength float32
	on       bool
}

// wobbleState is the shimmer as it is now.
func (a *app) wobbleState() wobble {
	a.wobMu.Lock()
	defer a.wobMu.Unlock()
	return a.wob
}

// shimmerOf is how bright a shimmer is for n characters of text added.
func shimmerOf(n int) float32 {
	return float32(math.Min(shimmerMax, shimmerMin+math.Sqrt(float64(n))/250))
}

// jiggle sends the shimmer across the window for n characters of text added.
func (a *app) jiggle(n int) {
	if a.settings.NoWobble {
		return
	}
	a.shimmer(n)
}

// shimmer sends it whatever the settings say: the settings' button tries it.
func (a *app) shimmer(n int) {
	if n <= 0 {
		return
	}
	s := shimmerOf(n)
	a.wobMu.Lock()
	w := a.wob
	switch {
	case !w.on, !w.at.IsZero() && time.Since(w.at) > shimmerLen/2: // none, or one past its middle: from the start
		a.wob = wobble{strength: s, on: true}
	default:
		a.wob.strength = max(w.strength, s)
	}
	a.wobMu.Unlock()
	if a.invalidate != nil {
		a.invalidate()
	}
}

// shimmerHead is where the band's middle is at t of the sweep, across a view w by h: it comes in
// from before the top left corner and leaves past the bottom right, easing in and out.
func shimmerHead(t, w, h float32) float32 {
	t = min(max(t, 0), 1)
	e := t * t * (3 - 2*t)
	from, to := float32(-2*shimmerW), w+shimmerTip*h+2*shimmerW
	return from + (to-from)*e
}

// shake lays the shimmer on what is drawn in the window, and asks for its frames.
func (a *app) shake(c *ui.Context, root ui.Element) {
	now := c.Now()
	a.wobMu.Lock()
	if a.wob.on && a.wob.at.IsZero() {
		a.wob.at = now
	}
	w := a.wob
	if w.on && now.Sub(w.at) >= shimmerLen {
		a.wob = wobble{}
		w.on = false
	}
	a.wobMu.Unlock()
	if !w.on || fx.still || !shadersOn() {
		return
	}
	c.AnimationFrame()
	if !shimmerDraws {
		return
	}
	b := root.LayoutBounds()
	if b.W == 0 { // the first frame: its size comes after; the window's is as good
		b.W, b.H = c.Size()
	}
	t := float32(now.Sub(w.at)) / float32(shimmerLen)
	root.Effect(shimmerShader.Effect(shader.Options{
		Uniforms: map[string]any{
			"Head":     shimmerHead(t, b.W, b.H),
			"Width":    float32(shimmerW),
			"Tip":      float32(shimmerTip),
			"Strength": w.strength,
		},
	}))
}
