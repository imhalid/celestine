package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Effects of the mouse pointer, as the carets are effects of the text's caret (#287): chosen in the settings,
// off by default. Each goes with the system's pointer, which stays as it is, and is drawn over the window in
// an overlay the pointer passes through, so a click lands where it would without it.
//
//	Spider        walks to the pointer on its eight legs, its silk behind it (cursorfx_chase.go)
//	Rocket        homes in on the pointer; a click locks it on and it strikes (cursorfx_chase.go)

// cursorFX is an effect of the pointer: told where the pointer is each frame, it moves and draws itself.
type cursorFX interface {
	step(m pointerAt, dt float64) bool // moves it on dt seconds; true while it still moves and wants frames
	draw(p *ui.Painter, m pointerAt)
}

// pointerAt is the pointer of a frame, in the window's DIPs.
type pointerAt struct {
	x, y  float64
	in    bool // over the window
	press bool // the button went down since the last frame
	col   ui.Color
	dark  bool
}

// cursorPreset is an effect of the pointer: its name, and how to make one.
type cursorPreset struct {
	name string
	make func() cursorFX
}

// cursorPresets are the effects of the pointer, in the order the settings list them.
var cursorPresets []cursorPreset

func init() {
	cursorPresets = []cursorPreset{
		{"Spider", func() cursorFX { return &spider{} }},
		{"Rocket", func() cursorFX { return &rocket{} }},
	}
}

// cursorFXNamed makes the effect called name, or nil: "" is none.
func cursorFXNamed(name string) cursorFX {
	for _, c := range cursorPresets {
		if c.name == name {
			return c.make()
		}
	}
	return nil
}

// cursorState is the effect of the pointer the settings chose, and what the frames told it.
type cursorState struct {
	name   string
	fx     cursorFX
	at     pointerAt
	press  bool      // a press seen by a build, for the next draw
	last   time.Time // the last draw
	moving bool      // the effect still moved at the last draw, and asked for another
}

// cursorFXView puts the chosen effect of the pointer over the window. The root tells where the pointer is
// and whether it went down there, without taking it; on a phone, with no pointer, there is nothing to draw.
func (a *app) cursorFXView(c *ui.Context, root ui.Element) {
	s := &a.cursor
	if a.settings.CursorFX != s.name {
		s.name, s.fx, s.last = a.settings.CursorFX, cursorFXNamed(a.settings.CursorFX), time.Time{}
	}
	if s.fx == nil || phone {
		return
	}
	x, y, over := root.PointerPosition()
	s.at.x, s.at.y, s.at.in = float64(x), float64(y), over
	if root.PressedInside() {
		s.press = true
	}
	w, h := a.winW, a.winH
	if w <= 0 {
		w, h = c.Size()
	}
	ui.Overlay(c, func() {
		ui.Box(c.Key("cursor-fx")).Absolute().Left(0).Top(0).Width(w).Height(h).PassThrough().Role(ui.RoleNone).
			Draw(func(p *ui.Painter, r ui.Rect) {
				now := p.Now()
				dt := dtOf(now, s.last)
				s.last = now
				m := s.at
				m.press, s.press = s.press, false
				m.col, m.dark = p.Theme().Accent, p.Theme().Dark
				if s.moving = s.fx.step(m, dt); s.moving {
					p.AnimationFrame()
				}
				s.fx.draw(p, m)
			})
	})
}

// ---- what they share ----

// turnTo turns angle a toward b (radians) by the shortest way, at rate per second.
func turnTo(a, b, rate, dt float64) float64 {
	d := math.Remainder(b-a, 2*math.Pi)
	step := rate * dt
	if math.Abs(d) <= step {
		return b
	}
	return a + math.Copysign(step, d)
}

// turned is (x, y) turned by ang about the origin and moved to (cx, cy).
func turned(cx, cy, x, y, ang float64) (float32, float32) {
	s, c := math.Sincos(ang)
	return f32(cx + x*c - y*s), f32(cy + x*s + y*c)
}

// shape is a closed path through pts (x, y pairs about the origin), turned by ang and moved to (cx, cy).
func shape(cx, cy, ang float64, pts ...float64) *ui.Path {
	path := &ui.Path{}
	for i := 0; i+1 < len(pts); i += 2 {
		x, y := turned(cx, cy, pts[i], pts[i+1], ang)
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return path.Close()
}

// slant is a straight line from (x0, y0) to (x1, y1) at any slant, which Painter.Line is not: it draws only the
// level and the upright.
func slant(p *ui.Painter, x0, y0, x1, y1, width float32, col ui.Color) {
	p.StrokePolyline([][2]float32{{x0, y0}, {x1, y1}}, width, col)
}

// ink is the color drawn on the page in the dark or the light: near white, or near black.
func ink(dark bool) ui.Color {
	if dark {
		return ui.Hex("#f4f4f5")
	}
	return ui.Hex("#18181b")
}
