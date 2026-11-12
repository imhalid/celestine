package main

import (
	"fmt"
	"log"
	"os"
	"runtime/metrics"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// The frame meter: with CELESTINE_FRAMES=1 the app writes to its log, once a second while it draws, how many times the
// view was built and how many times the window was painted, and how long the views took. A window at rest should
// show nothing at all (fable-reports/04-performance.md, "What to measure first").

var frameMeterOn = os.Getenv("CELESTINE_FRAMES") == "1"

// frameMeter is what was counted since the last line, and the frames the debug mode shows.
type frameMeter struct {
	mu            sync.Mutex // the views count after the frame's lock is let go; the paints under it
	since         time.Time
	views, paints int
	took, longest time.Duration

	// hud is the debug mode (debugView): the frames are kept in ring while it is on. view reads it before it takes
	// a.mu, so it is not behind it.
	hud   atomic.Bool
	ring  [hudFrames]hudFrame
	n     int           // the frames kept in all; the latest is ring[(n-1)%hudFrames]
	built time.Duration // how long the view built since the last paint took, 0 when none was
	last  time.Time     // when the last paint was
	heap  hudHeap
}

// meterView counts a view that took d.
func (a *app) meterView(d time.Duration) {
	m := &a.meter
	if m.hud.Load() {
		m.built += d
	}
	if !frameMeterOn {
		return
	}
	m.views++
	m.took += d
	m.longest = max(m.longest, d)
	a.meterLine()
}

// meterPaint counts a paint of the window at now (drawn last, over everything: frameMeterView).
func (a *app) meterPaint(now time.Time) {
	a.meter.mu.Lock()
	defer a.meter.mu.Unlock()
	if a.meter.hud.Load() {
		a.meter.keep(now)
	}
	if frameMeterOn {
		a.meter.paints++
		a.meterLine()
	}
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

// frameMeterView is a point of the window drawn at every paint, which counts them, and the debug mode's card.
func (a *app) frameMeterView(c *ui.Context) {
	if !frameMeterOn && !a.meter.hud.Load() {
		return
	}
	ui.Box(c.Key("frame-meter")).Absolute().Top(0).Left(0).Size(1, 1).Draw(func(p *ui.Painter, r ui.Rect) { a.meterPaint(p.Now()) })
	a.debugView(c)
}

// The debug mode: a card in the window's corner says how fast the window draws, for finding what makes it slow. It
// is painted as the window paints and asks for no frame of its own: a window at rest draws nothing, so the card
// keeps the numbers of the last frames until something changes, and a drop stays in its bars after it happened.
// CELESTINE_DEBUG=1 starts the app with it on; a command of the palette turns it on and off.
//
// MyGo has no frame timing an app can read: its inspector's Performance tab (F12) and MYGO_FRAME_STATS log are for
// development builds. So the card measures what the app sees: the time between two paints of the window, and how
// long its view took to build.

const (
	hudFrames = 120                    // the frames the card keeps, as many bars
	hudSpan   = 2 * time.Second        // the frames the averages and the worst are of
	hudIdle   = 250 * time.Millisecond // a pause between paints longer than this is rest, not a slow frame
	hudSlow   = 33 * time.Millisecond  // a frame slower than this missed at least one of a 60 Hz display's
	hudW      = 300                    // the card's width
)

// hudFrame is a paint of the window: when it was, how long after the paint before (0 after a rest), and how long
// the view built for it took (0 when it only painted).
type hudFrame struct {
	at         time.Time
	gap, build time.Duration
}

// hudHeap is the Go heap and the garbage collections, read at most twice a second.
type hudHeap struct {
	at       time.Time
	live, gc uint64
	samples  [2]metrics.Sample
}

var debugOnStart = os.Getenv("CELESTINE_DEBUG") == "1"

// keep notes a paint at now, under m.mu.
func (m *frameMeter) keep(now time.Time) {
	f := hudFrame{at: now, build: m.built}
	if !m.last.IsZero() {
		if d := now.Sub(m.last); d <= hudIdle {
			f.gap = d
		}
	}
	m.ring[m.n%hudFrames] = f
	m.n++
	m.last, m.built = now, 0
}

// toggleDebug turns the debug mode on or off.
func (a *app) toggleDebug() {
	a.setDebug(!a.meter.hud.Load())
}

func (a *app) setDebug(on bool) {
	m := &a.meter
	m.mu.Lock()
	m.n, m.built, m.last = 0, 0, time.Time{}
	m.mu.Unlock()
	m.hud.Store(on)
}

// hudStats is what the card says of the frames kept.
type hudStats struct {
	fps                   int // the paints in the second up to the last
	gap, gapAvg, gapWorst time.Duration
	build, bAvg, bWorst   time.Duration
	slow                  int // the frames slower than hudSlow in the span
	builds, paints        int // in the span
}

// stats reads the frames kept, under m.mu.
func (m *frameMeter) stats() (s hudStats) {
	if m.n == 0 {
		return s
	}
	latest := m.ring[(m.n-1)%hudFrames]
	s.gap, s.build = latest.gap, latest.build
	var gaps, gapSum, bSum time.Duration
	var nGaps int
	for i := 0; i < min(m.n, hudFrames); i++ {
		f := m.ring[(m.n-1-i)%hudFrames]
		age := latest.at.Sub(f.at)
		if age >= hudSpan {
			break
		}
		if age < time.Second {
			s.fps++
		}
		s.paints++
		if f.gap > 0 {
			nGaps++
			gapSum += f.gap
			s.gapWorst = max(s.gapWorst, f.gap)
			if f.gap > hudSlow {
				s.slow++
			}
		}
		if f.build > 0 {
			s.builds++
			bSum += f.build
			s.bWorst = max(s.bWorst, f.build)
		}
		gaps += f.gap
	}
	if nGaps > 0 {
		s.gapAvg = gapSum / time.Duration(nGaps)
	}
	if s.builds > 0 {
		s.bAvg = bSum / time.Duration(s.builds)
	}
	return s
}

// read returns the heap's live bytes and the collections so far, read again when half a second has gone.
func (h *hudHeap) read(now time.Time) (live, gc uint64) {
	if h.at.IsZero() || now.Sub(h.at) >= 500*time.Millisecond || now.Before(h.at) {
		h.samples[0].Name, h.samples[1].Name = "/gc/heap/live:bytes", "/gc/cycles/total:gc-cycles"
		metrics.Read(h.samples[:])
		if h.samples[0].Value.Kind() == metrics.KindUint64 {
			h.live = h.samples[0].Value.Uint64()
		}
		if h.samples[1].Value.Kind() == metrics.KindUint64 {
			h.gc = h.samples[1].Value.Uint64()
		}
		h.at = now
	}
	return h.live, h.gc
}

// hudMs writes a duration in milliseconds, to a tenth.
func hudMs(d time.Duration) string { return fmt.Sprintf("%.1f", float64(d)/float64(time.Millisecond)) }

// debugView is the debug mode's card, at the window's bottom right. Its title is an element; the numbers and the
// bars are painted, so that a frame that only paints shows them too.
func (a *app) debugView(c *ui.Context) {
	if !a.meter.hud.Load() {
		return
	}
	rows := len(a.rows) // the sidebar's rows, under a.mu as the view is built
	const pad, line, barsH = 10, 15, 40
	h := float32(pad + 18 + 6*line + 6 + barsH + pad)
	bg, fg, dim := ui.Hex("#101014e6"), ui.Hex("#f2f2f2"), ui.Hex("#9a9aa6")
	good, bad, build := ui.Hex("#5fd38d"), ui.Hex("#ff6b6b"), ui.Hex("#f2b630")
	ui.Box(c.Key("debug-hud")).Absolute().Right(12).Bottom(40).Size(hudW, h).Radius(8).Background(bg).PassThrough().
		Padding(pad).Draw(func(p *ui.Painter, r ui.Rect) {
		m := &a.meter
		m.mu.Lock()
		s := m.stats()
		live, gc := m.heap.read(time.Now())
		bars := make([]hudFrame, 0, hudFrames)
		for i := max(0, m.n-hudFrames); i < m.n; i++ {
			bars = append(bars, m.ring[i%hudFrames])
		}
		m.mu.Unlock()
		x, y := r.X+pad, r.Y+pad+18
		text := func(s string, c ui.Color) {
			p.RichText(x, y, r.W-2*pad, ui.Span{Text: s, Font: monoStack, Size: 11, Color: c})
			y += line
		}
		fpsColor := good
		if s.slow > 0 {
			fpsColor = bad
		}
		p.RichText(r.X+r.W-pad-90, r.Y+pad-1, 90, ui.Span{Text: locales.DebugFps(s.fps), Font: monoStack, Size: 13, Weight: 600, Color: fpsColor})
		text(locales.DebugFrame(hudMs(s.gap), hudMs(s.gapAvg), hudMs(s.gapWorst)), fg)
		text(locales.DebugBuild(hudMs(s.build), hudMs(s.bAvg), hudMs(s.bWorst)), fg)
		slowColor := dim
		if s.slow > 0 {
			slowColor = bad
		}
		text(locales.DebugSlow(s.slow, fmt.Sprint(hudSlow.Milliseconds())), slowColor)
		text(locales.DebugBuildsPaints(s.builds, s.paints), dim)
		text(locales.DebugRows(rows), dim)
		text(locales.DebugHeap(fmt.Sprintf("%.1f", float64(live)/(1<<20)), int(gc)), dim)
		y += 6
		// The bars: a frame's time between paints, the latest at the right, its build in yellow at its foot; the
		// lines are a 60 Hz frame and hudSlow. A rest is a gray stub.
		top, bw := y, (r.W-2*pad)/hudFrames
		scale := float32(barsH) / float32(2*hudSlow)
		p.Fill(ui.Rect{X: x, Y: top, W: r.W - 2*pad, H: barsH}, ui.Hex("#ffffff0d"), 2)
		for _, ref := range []time.Duration{time.Second / 60, hudSlow} {
			ly := top + barsH - float32(ref)*scale
			p.Line(x, ly, x+r.W-2*pad, ly, 1, ui.Hex("#ffffff26"))
		}
		bx := x + r.W - 2*pad - float32(len(bars))*bw
		for _, f := range bars {
			if f.gap == 0 {
				p.Fill(ui.Rect{X: bx, Y: top + barsH - 2, W: max(bw-1, 1), H: 2}, dim, 0)
			} else {
				hgt := min(float32(f.gap)*scale, barsH)
				col := good
				if f.gap > hudSlow {
					col = bad
				}
				p.Fill(ui.Rect{X: bx, Y: top + barsH - hgt, W: max(bw-1, 1), H: hgt}, col, 0)
			}
			if f.build > 0 {
				hgt := min(max(float32(f.build)*scale, 1), barsH)
				p.Fill(ui.Rect{X: bx, Y: top + barsH - hgt, W: max(bw-1, 1), H: hgt}, build, 0)
			}
			bx += bw
		}
	}).Children(func() {
		ui.Text(c, locales.DebugTitle()).Font(monoStack).FontSize(11).FontWeight(600).TextColor(dim)
	})
}
