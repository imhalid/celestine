package main

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// The frame meter: with RYUDIT_FRAMES=1 the app writes to its log, once a second while it draws, how many times the
// view was built and how many times the window was painted, and how long the views took. A window at rest should
// show nothing at all (fable-reports/04-performance.md, "What to measure first").

var frameMeterOn = os.Getenv("RYUDIT_FRAMES") == "1"

// frameMeter is what was counted since the last line.
type frameMeter struct {
	mu            sync.Mutex // the views count after the frame's lock is let go; the paints under it
	since         time.Time
	views, paints int
	took, longest time.Duration
}

// meterView counts a view that took d.
func (a *app) meterView(d time.Duration) {
	m := &a.meter
	m.views++
	m.took += d
	m.longest = max(m.longest, d)
	a.meterLine()
}

// meterPaint counts a paint of the window (drawn last, over everything: frameMeterView).
func (a *app) meterPaint() {
	a.meter.mu.Lock()
	defer a.meter.mu.Unlock()
	a.meter.paints++
	a.meterLine()
}

// meterLine writes the line when a second has gone.
func (a *app) meterLine() {
	m := &a.meter
	now := time.Now()
	if m.since.IsZero() {
		m.since = now
		return
	}
	if now.Sub(m.since) < time.Second {
		return
	}
	avg := time.Duration(0)
	if m.views > 0 {
		avg = m.took / time.Duration(m.views)
	}
	log.Printf("frames: %d views (avg %v, longest %v), %d paints in %v", m.views, avg.Round(10*time.Microsecond), m.longest.Round(10*time.Microsecond), m.paints, now.Sub(m.since).Round(time.Millisecond))
	m.since, m.views, m.paints, m.took, m.longest = now, 0, 0, 0, 0 // not the lock, which is held
}

// frameMeterView is a point of the window drawn at every paint, which counts them.
func (a *app) frameMeterView(c *ui.Context) {
	if !frameMeterOn {
		return
	}
	ui.Box(c.Key("frame-meter")).Absolute().Top(0).Left(0).Size(1, 1).Draw(func(p *ui.Painter, r ui.Rect) { a.meterPaint() })
}
