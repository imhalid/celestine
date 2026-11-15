package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// What every caret has, whichever is chosen: it dozes off after five seconds of rest, with
// z's that rise slowly. (The jelly, a setting, is smear in caretfx_look.go.)

// glassPath is a caret's body as glass: a soft glow, the color thin and lit from above, a rim.
// It is what the slime is made of, for a body that is not the slime's.
func glassPath(p *ui.Painter, path *ui.Path, col ui.Color, dark bool) {
	glow := 1.0
	if !dark {
		glow = 0.6
	}
	for i, w := range []float32{9, 6, 3.5} {
		p.StrokePath(path, w, withAlpha(col, glow*[]float64{0.035, 0.06, 0.09}[i]))
	}
	top := mix(col, ui.RGBA(255, 255, 255, 1), 0.45)
	p.FillPathGradient(path, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: withAlpha(top, 0.62), At: 0},
		{Color: withAlpha(col, 0.38), At: 0.55},
		{Color: withAlpha(col, 0.5), At: 1},
	}})
	p.StrokePath(path, 1.25, withAlpha(col, 0.95))
}

// sleepAfter is how long the caret is at rest before it dozes: the setting's (focus.go).
var sleepAfter = 5 * time.Second

const (
	sleepZLife  = 3800 * time.Millisecond // how long a z lives
	sleepZGap   = 1600 * time.Millisecond // from one z to the next
	sleepFor    = 90 * time.Second        // it sleeps this long in sight, then lies still
	sleepBreath = 4.0                     // seconds a breath takes
)

// sleeper watches the caret rest. After five seconds it sleeps: it breathes, slowly, and the z's
// come out of it.
type sleeper struct {
	at    ui.Rect   // where the caret was last seen
	since time.Time // the last time anything happened to it
	zs    []sleepZ
	next  time.Time
	seen  bool
	n     int
	amp   float64 // how deeply asleep, 0..1
}

type sleepZ struct {
	t0   time.Time
	seed float64
}

// rested is how long the caret has stood still, nothing written, as observe last saw it.
func (s *sleeper) rested(now time.Time) time.Duration {
	if !s.seen {
		return 0
	}
	return now.Sub(s.since)
}

// idleFrame asks for the next frame of a caret that moves of itself (it breathes, blinks, sways, hums), by how long
// it has rested: at the display's rate a moment after a key, at 30 a second while the sleeper is awake to it, and none
// after that, so that a window left with the caret in its text is a window at rest (#188). What it draws then stays.
func idleFrame(p *ui.Painter, f caretFrame) {
	switch {
	case f.rest < caretLively:
		p.AnimationFrame()
	case f.rest < caretRests():
		p.AnimationFrameRate(30)
	}
}

// caretRests is how long a caret goes on moving of itself once it stands still: as long as the sleeper is awake to
// it, its z's gone. After that it asks for no frame.
func caretRests() time.Duration { return sleepAfter + sleepFor + 3*time.Second }

// caretLively is how long after a key a caret that moves of itself is drawn at the display's rate.
const caretLively = 2 * time.Second

// wake puts the caret awake: a page opened, a key pressed.
func (s *sleeper) wake() { s.seen, s.zs, s.next, s.amp = false, nil, time.Time{}, 0 }

// observe is called each frame before the caret is drawn: it sees whether anything happened,
// and how asleep the caret is.
func (s *sleeper) observe(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	moved := !s.seen || math.Abs(float64(f.r.X-s.at.X)) > 0.5 || math.Abs(float64(f.r.Y-s.at.Y)) > 0.5
	if moved || k.wrote > 0 || k.edited || k.erased > 0 {
		s.seen, s.at, s.since, s.zs, s.next, s.amp = true, f.r, now, nil, time.Time{}, 0
		p.After(sleepAfter + 30*time.Millisecond)
		return
	}
	rest := now.Sub(s.since)
	if rest < sleepAfter {
		s.amp = 0
		p.After(sleepAfter - rest + 30*time.Millisecond)
		return
	}
	asleep := rest - sleepAfter
	s.amp = smoothstep(asleep.Seconds()/1.5) * (1 - smoothstep((asleep-sleepFor).Seconds()/3))
	if asleep < sleepFor && (s.next.IsZero() || !now.Before(s.next)) { // a z lets go
		s.n++
		s.zs = append(s.zs, sleepZ{t0: now, seed: float64(s.n) * 1.7})
		s.next = now.Add(sleepZGap)
	}
	if s.amp > 0 || len(s.zs) > 0 {
		p.AnimationFrameRate(30)
	}
}

// breathe is the sleeping caret: it shrinks and grows back, up and down, with its breath, never
// past its own size, standing on its feet: the transform is about the middle of its base.
func (s *sleeper) breathe(now time.Time, f caretFrame) (t ui.Transform, px, py float32, on bool) {
	if s.amp <= 0.001 {
		return ui.Transform{}, 0, 0, false
	}
	b := math.Sin(2 * math.Pi * now.Sub(s.since).Seconds() / sleepBreath) // -1 empty, 1 full
	sy := 1 - 0.16*(0.5-0.5*b)*s.amp                                      // full height when it is full of breath, never taller
	sx := 1.0
	box := boxOf(f)
	return ui.Scaling(f32(sx), f32(sy)), box.X + box.W/2, box.Y + box.H, true
}

// sleepZSizes is how many sizes a z takes over its life after the first; with its three tilts, the shapes the z's
// are drawn in are a few dozen at most.
const sleepZSizes = 7

// snapZ rounds v to the nearest 1/per, so that a z's shapes repeat.
func snapZ(v, per float64) float64 { return math.Round(v*per) / per }

// zPath is a z, as lines, centered on (x, y), half a side s, turned by ang: drawn as a shape it
// stays steady as it grows and fades, where a letter of text shimmers.
func zPath(x, y, s, ang float64) *ui.Path {
	pts := [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}}
	sin, cos := math.Sincos(ang)
	path := &ui.Path{}
	for i, q := range pts {
		px, py := q[0]*s, q[1]*s
		X, Y := f32(x+px*cos-py*sin), f32(y+px*sin+py*cos)
		if i == 0 {
			path.MoveTo(X, Y)
		} else {
			path.LineTo(X, Y)
		}
	}
	return path
}

// draw is the z's, out of the caret's top, up and to the right, bigger as they go.
func (s *sleeper) draw(p *ui.Painter, f caretFrame, ink ui.Color) {
	now := p.Now()
	box := boxOf(f)
	size := textSize(f)
	out := s.zs[:0]
	for _, z := range s.zs {
		age := float64(now.Sub(z.t0)) / float64(sleepZLife)
		if age >= 1 {
			continue
		}
		out = append(out, z)
		fade := smoothstep(age/0.18) * (1 - smoothstep((age-0.55)/0.45)) // thin at both ends
		// The z's place, size, tilt and stroke are taken in steps: MyGo keeps a mask for each shape it strokes, and
		// a shape new at each frame would fill its atlas, which then paints the whole window again (#558).
		// Its place is in whole pixels, so that a shape is the same wherever it is drawn.
		x := math.Round(float64(box.X+box.W) + 3 + age*30 + math.Sin(age*5+z.seed)*3)
		y := math.Round(float64(box.Y) - 2 - age*38)
		sc := 0.6 + 0.8*snapZ(age, sleepZSizes)
		hs := float64(size) * 0.19 * sc
		tilt := -0.18 + 0.12*snapZ(math.Sin(age*4+z.seed), 1)
		p.StrokePath(zPath(x, y, hs, tilt), f32(math.Max(2, hs*0.55)), withAlpha(ink, 0.85*fade))
	}
	s.zs = out
}

// ---- the tiles of the settings: each caret, live ----

// caretDemo is a caret of a tile, writing and erasing a word over and over, as the real one does.
type caretDemo struct {
	fx    caretFX
	slime *slime
	miz   *mizanCaret
	jelly smear
	t0    time.Time
	n     int // letters on show
}

const (
	demoWord = "Merhaba"
	demoLoop = 6.0 // seconds
)

// demoLetters is how many letters of the word show t seconds into the loop: written, kept,
// three erased, kept, all gone at once (a jump), and again.
func demoLetters(t float64) int {
	n := len([]rune(demoWord))
	switch {
	case t < 1.4:
		return min(n, int(t/0.2))
	case t < 2.6:
		return n
	case t < 3.5:
		return n - min(3, int((t-2.6)/0.25))
	case t < 5.0:
		return n - 3
	}
	return 0
}

func (a *app) demoOf(k caretKind) *caretDemo {
	if a.demos == nil {
		a.demos = map[caretKind]*caretDemo{}
	}
	d := a.demos[k]
	if d == nil {
		d = &caretDemo{}
		switch i := int(k) - int(caretFirstFX); {
		case k == caretSlime:
			d.slime = newSlime()
		case k == caretMizan:
			d.miz = &mizanCaret{}
		case i >= 0 && i < len(caretPresets):
			d.fx = caretPresets[i].make()
		}
		a.demos[k] = d
	}
	return d
}

// caretMock is a caret for a tile, alive: the word is written and erased, and the caret does as it does.
func (a *app) caretMock(c *ui.Context, kind caretKind) {
	cl := ryugo.From(c).Colors
	d := a.demoOf(kind)
	ui.Box(c).Width(112).Height(66).Background(cl.Well).Draw(func(p *ui.Painter, r ui.Rect) {
		now := p.Now()
		if d.t0.IsZero() {
			d.t0 = now
		}
		t := math.Mod(now.Sub(d.t0).Seconds(), demoLoop)
		runes := []rune(demoWord)
		n := demoLetters(t)
		var k kick
		switch grew := n - d.n; {
		case grew > 0 && grew <= 2:
			k.wrote, k.typed = grew, string(runes[d.n:n])
		case grew < 0 && grew >= -3:
			k.erased, k.dir, k.gone = -grew, -1, string(runes[n:d.n])
			k.edited = true
		case grew != 0:
			k.edited = true
		}
		d.n = n
		const size = 15
		span := func(s string) ui.Span { return ui.Span{Text: s, Size: size, Weight: 560, Color: cl.Text} }
		pw, lh := p.MeasureText(0, span(string(runes[:n])))
		if n == 0 {
			pw, lh = 0, size*1.25
		}
		x0, y0 := r.X+14, r.Y+(r.H-lh)/2
		if n > 0 {
			p.RichText(x0, y0, 100, span(string(runes[:n])))
		}
		cw := float32(9)
		if n < len(runes) {
			cw, _ = p.MeasureText(0, span(string(runes[n:n+1])))
		}
		f := caretFrame{r: ui.Rect{X: x0 + pw, Y: y0, W: 1, H: lh}, cw: cw, dark: ryugo.Painting(p).Dark, bg: cl.Well, bounds: r, wrote: k.wrote, edited: k.edited}
		p.Clip(r, 0, func() {
			body := func() {
				switch {
				case d.slime != nil:
					col := ui.Hex("#59f939")
					if !f.dark {
						col = ui.Hex("#2f9a1c")
					}
					d.slime.draw(p, f.r, f.cw, col, f.dark)
				case d.miz != nil:
					d.miz.draw(p, f)
				case d.fx != nil:
					d.fx.draw(p, f, k)
				}
			}
			if a.settings.Jelly && kind != caretSlime {
				f.jelly = d.jelly.body(p, f)
			}
			body()
		})
		p.AnimationFrame()
	})
}

// tinted is a caret whose color is not the cursor's green.
type tinted interface{ tint(dark bool) ui.Color }

func (g *speedDemon) tint(bool) ui.Color { return heatColor(0.75) }

// caretTint is the color of caret kind: what is drawn with it, the z's of its sleep, is in it.
func caretTint(p *ui.Painter, kind caretKind, fx caretFX) ui.Color {
	dark := p.Theme().Dark
	switch {
	case kind == caretMizan:
		return p.Theme().Accent
	case kind == caretSlime:
		if dark {
			return ui.Hex("#59f939")
		}
		return ui.Hex("#2f9a1c")
	}
	if t, ok := fx.(tinted); ok {
		return t.tint(dark)
	}
	return cursorColor(dark)
}

func (a *app) caretTint(p *ui.Painter) ui.Color {
	var fx caretFX
	if i := int(a.caretKind) - int(caretFirstFX); i >= 0 && i < len(caretPresets) {
		fx = a.fxOf(i)
	}
	return caretTint(p, a.caretKind, fx)
}
