package main

import (
	"math"
	"math/rand"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// What the carets of Cursor-Smith's effects share: letters drawn turned and scaled, colors from
// a hue, the box cursor, springs.

// glyphAt draws the text s centered on (cx, cy), at size, turned by angle (radians) and scaled
// by sc about its center.
func glyphAt(p *ui.Painter, s string, cx, cy, size float32, col ui.Color, bold bool, angle, sc float64) {
	wt := 400
	if bold {
		wt = 750
	}
	span := ui.Span{Text: s, Size: size, Weight: wt, Color: col}
	w, h := p.MeasureText(0, span)
	draw := func() { p.RichText(cx-w/2, cy-h/2, w+4, span) }
	if angle == 0 && sc == 1 {
		draw()
		return
	}
	t := ui.Rotation(f32(angle)).Then(ui.Scaling(f32(sc), f32(sc)))
	p.Transform(t, cx, cy, draw)
}

// hsl is the color of a hue (degrees), saturation and lightness (0..1).
func hsl(h, s, l float64) ui.Color {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch int(h / 60) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return ui.RGBA(uint8((r+m)*255+0.5), uint8((g+m)*255+0.5), uint8((b+m)*255+0.5), 1)
}

// drawBoxCursor draws the plain box cursor the effects are on: the color, a halo, a quarter-round box.
func drawBoxCursor(p *ui.Painter, box ui.Rect, col ui.Color, alpha float64) {
	glowAround(p, box, col, alpha)
	p.Fill(box, withAlpha(col, 0.9*alpha), boxRadius(box.W, box.H))
}

// boxOf is the box of a caret frame: where its letter is.
func boxOf(f caretFrame) ui.Rect {
	return ui.Rect{X: f.r.X, Y: f.r.Y, W: f32(math.Max(float64(f.cw), 6)), H: f.r.H}
}

func smoothstep(x float64) float64 { x = clamp01(x); return x * x * (3 - 2*x) }

func easeSine(x float64) float64 { return -(math.Cos(math.Pi*clamp01(x)) - 1) / 2 }

func rnd() float64 { return rand.Float64() }

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// perm is how wide the letters of s are at size, one by one.
func letterWidths(p *ui.Painter, s string, size float32) (runes []string, widths []float64) {
	for _, r := range s {
		w, _ := p.MeasureText(0, ui.Span{Text: string(r), Size: size})
		runes = append(runes, string(r))
		widths = append(widths, float64(w))
	}
	return
}

// textSize is the size of the letters on a caret's line.
func textSize(f caretFrame) float32 { return f.r.H / 1.25 }

// pix draws a square cell, snapped to a grid of cell px.
func pix(p *ui.Painter, x, y, cell float64, size float32, col ui.Color) {
	gx, gy := math.Floor(x/cell)*cell, math.Floor(y/cell)*cell
	p.Fill(ui.Rect{X: f32(gx), Y: f32(gy), W: size, H: size}, col, 0)
}

// restBody is the shape of a caret's body when it is not doing anything of its own: the box, or
// the jelly's shape of it.
func restBody(f caretFrame, box ui.Rect) *ui.Path {
	if f.jelly != nil {
		return f.jelly
	}
	return shapeBody(box, f.style.bracket)
}

// glassBody draws a caret's body in col as the slime is drawn: its shape (the box, its bracket's, or the jelly's), but
// see-through, lit from above (lighter at the top, the color deepening down it), with a rim of the color and a
// gloss down its upper left side. Every caret's body is glass, as the slime's is.
func glassBody(p *ui.Painter, f caretFrame, box ui.Rect, col ui.Color, alpha, glow float64) {
	styledDraw(p, f, box, func() {
		path := restBody(f, box)
		if f.jelly == nil {
			glowAround(p, box, col, glow*0.8)
		} else {
			for i, w := range []float32{9, 6, 3.5} {
				p.StrokePath(path, w, withAlpha(col, glow*[]float64{0.035, 0.06, 0.09}[i]))
			}
		}
		glassFill(p, path, col, col, f.dark, alpha)
		glassGloss(p, box, f.dark, alpha)
	})
}

// glassFill fills path as glass: top's color above melting into bottom's below (the same for one color), lit
// from above, with a rim that melts the same way.
func glassFill(p *ui.Painter, path *ui.Path, top, bottom ui.Color, dark bool, alpha float64) {
	light := mix(top, ui.RGBA(255, 255, 255, 1), 0.45)
	p.FillPathGradient(path, ryugo.InOklch(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: withAlpha(light, 0.62*alpha), At: 0},
		{Color: withAlpha(top, 0.4*alpha), At: 0.4},
		{Color: withAlpha(bottom, 0.42*alpha), At: 0.65},
		{Color: withAlpha(bottom, 0.52*alpha), At: 1},
	}}))
	p.StrokePathGradient(path, 1.25, ryugo.InOklch(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: withAlpha(top, 0.95*alpha), At: 0.3},
		{Color: withAlpha(bottom, 0.95*alpha), At: 0.7},
	}}))
}

// glassGloss is the glint down a glass body's upper left side.
func glassGloss(p *ui.Painter, box ui.Rect, dark bool, alpha float64) {
	a := 0.55
	if !dark {
		a = 0.8
	}
	gx := box.X + max(1.6, box.W*0.22)
	gloss := &ui.Path{}
	gloss.MoveTo(gx, box.Y+box.H*0.14).LineTo(gx, box.Y+box.H*0.36)
	p.StrokePath(gloss, 1.4, ui.RGBA(255, 255, 255, float32(a*alpha)))
}

// faceInk is what a glass caret's face is drawn in: dark on a light page, where the glass is light; light on a
// dark page, where the dark shows through the glass and a dark face would not be seen.
func faceInk(dark bool, light ui.Color) ui.Color {
	if dark {
		return ui.Hex("#fafaf9")
	}
	return light
}
