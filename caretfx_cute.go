package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Cute carets, each fixed as it ships:
//
//	Kawaii  a pastel box with big shining eyes and blushing cheeks; sparkles as it writes, hearts at "!",
//	        its eyes squeeze shut (^ ^) with joy at a space, it blinks while it waits
//	UwU     a face of its own letters: u w u at rest, > w < when the writing is quick, O w O at "!" or "?",
//	        T w T when it erases; little "~" float off it as it writes

// ---- Kawaii ----

type kawaii struct {
	fxBase
	joy     float64 // when a space made it glad
	blink   float64
	sparkle motes
	hearts  motes
	hue     float64
}

func newKawaii() *kawaii { return &kawaii{joy: -1e9, blink: 1800} }

func (g *kawaii) reset() { g.sparkle, g.hearts, g.have = nil, nil, false }

func (g *kawaii) shift(dy float32) { g.sparkle.shift(dy); g.hearts.shift(dy); g.prev.Y += dy }

func (g *kawaii) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	if strings.Contains(k.typed, " ") {
		g.joy = tn
	}
	for i := 0; i < k.wrote && i < 2; i++ { // sparkles, a few at each letter
		a := -math.Pi/2 + (rnd()-0.5)*2.4
		g.sparkle = append(g.sparkle, mote{x: cx, y: cy - h*0.3, vx: math.Cos(a) * 45, vy: math.Sin(a)*45 - 10, spin: 4, t0: tn, life: 650, size: h * (0.16 + rnd()*0.12),
			col: hsl(300+rnd()*80, 0.9, 0.75)})
	}
	if strings.Contains(k.typed, "!") {
		for i := 0; i < 3; i++ {
			g.hearts = append(g.hearts, mote{x: cx + (rnd()-0.5)*h*0.6, y: cy - h*0.4, vx: (rnd() - 0.5) * 30, vy: -55 - rnd()*25, t0: tn, life: 1100, size: h * 0.4})
		}
	}
	g.sparkle.stepDraw(p, tn, dt, func(m *mote, age float64) { // a four-pointed star
		s := f32(m.size * (1 - age*0.6))
		st := &ui.Path{}
		st.MoveTo(0, -s).QuadTo(0, 0, s, 0).QuadTo(0, 0, 0, s).QuadTo(0, 0, -s, 0).QuadTo(0, 0, 0, -s).Close()
		p.Transform(ui.Rotation(deg(m.ang)).Then(ui.Translation(f32(m.x), f32(m.y))), 0, 0, func() { p.FillPath(st, withAlpha(m.col, 1-age)) })
	})
	g.hearts.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "♥", f32(m.x), f32(m.y), f32(m.size), withAlpha(ui.Hex("#fb7185"), 1-age), true, 0, 1+0.25*age)
	})
	pink := ui.Hex("#f9a8d4")
	if f.dark {
		pink = ui.Hex("#f472b6")
	}
	g.hue += dt * 30
	// the face, wider than a letter so that it can be seen, over the caret's place
	w := f32(math.Max(float64(box.W)*2.3, h*1.2))
	b := ui.Rect{X: box.X + box.W/2 - w/2, Y: box.Y, W: w, H: box.H}
	if b.X < box.X-w*0.15 { // it does not cover the letter before it more than a little
		b.X = box.X - w*0.15
	}
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, mix(pink, hsl(290+20*math.Sin(g.hue/40), 0.7, 0.8), 0.25), 1, 1)
	// eyes big and shining, or ^ ^ for joy, or a line while it blinks; cheeks; a tiny cat mouth
	ink := faceInk(f.dark, ui.Hex("#3b0764"))
	glad := tn-g.joy < 700
	blinking := false
	if tn > g.blink {
		blinking = tn-g.blink < 130
		if !blinking {
			g.blink = tn + 2200 + rnd()*2600
		}
	}
	er := max(1.2, b.H*0.11)
	look := g.gaze()
	for _, ox := range []float32{0.28, 0.72} {
		ex, ey := b.X+b.W*ox+look*b.W*0.05, b.Y+b.H*0.4
		switch {
		case glad:
			arc := &ui.Path{}
			arc.MoveTo(ex-er, ey+er*0.3).LineTo(ex, ey-er*0.7).LineTo(ex+er, ey+er*0.3)
			p.StrokePath(arc, max(0.9, er*0.5), ink)
		case blinking:
			p.Fill(ui.Rect{X: ex - er, Y: ey, W: 2 * er, H: max(0.8, er*0.35)}, ink, 0)
		default:
			p.Fill(ui.Rect{X: ex - er, Y: ey - er*1.2, W: 2 * er, H: 2.4 * er}, ink, er)
			// the glints on the side it looks to, the big one high, the small one low on the other
			gx := ex + look*er*0.45
			p.Fill(ui.Rect{X: gx - er*0.35, Y: ey - er*0.95, W: er * 0.7, H: er * 0.7}, ui.RGBA(255, 255, 255, 0.95), er*0.35)
			sx := ex - look*er*0.5
			p.Fill(ui.Rect{X: sx - er*0.17, Y: ey + er*0.25, W: er * 0.35, H: er * 0.35}, ui.RGBA(255, 255, 255, 0.85), er*0.2)
		}
	}
	for _, ox := range []float32{0.14, 0.86} {
		x := b.X + b.W*ox
		p.Fill(ui.Rect{X: x - er, Y: b.Y + b.H*0.6, W: er * 2, H: er * 0.9}, ui.RGBA(251, 113, 133, 0.6), er*0.45)
	}
	mx, my, mw := b.X+b.W/2, b.Y+b.H*0.66, b.H*0.07
	mouth := &ui.Path{}
	mouth.MoveTo(mx-2*mw, my).QuadTo(mx-mw, my+mw*1.4, mx, my).QuadTo(mx+mw, my+mw*1.4, mx+2*mw, my)
	p.StrokePath(mouth, max(0.8, b.H*0.05), ink)
	p.AnimationFrame() // it blinks
}

// ---- UwU ----

type uwu struct {
	fxBase
	rate   float64
	shock  float64 // "!" or "?": O w O
	sad    float64 // erasing: T w T
	tildes motes
}

func newUwu() *uwu { return &uwu{shock: -1e9, sad: -1e9} }

func (g *uwu) reset() { g.tildes, g.have = nil, false }

func (g *uwu) shift(dy float32) { g.tildes.shift(dy); g.prev.Y += dy }

func (g *uwu) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*4)
	if strings.ContainsAny(k.typed, "!?") {
		g.shock = tn
	}
	if k.erased > 0 {
		g.sad = tn
	}
	if k.wrote > 0 && rnd() < 0.35 {
		g.tildes = append(g.tildes, mote{x: float64(box.X + box.W), y: float64(box.Y) + h*0.2, vx: 25 + rnd()*20, vy: -30 - rnd()*20, t0: tn, life: 900, size: h * 0.45})
	}
	lilac := ui.Hex("#c4b5fd")
	if f.dark {
		lilac = ui.Hex("#a78bfa")
	}
	g.tildes.stepDraw(p, tn, dt, func(m *mote, age float64) {
		glyphAt(p, "~", f32(m.x), f32(m.y), f32(m.size), withAlpha(lilac, 1-age), true, 0, 1)
	})
	// the face, wider than the box: its eyes and mouth are letters
	eyes := "u"
	switch {
	case tn-g.sad < 900:
		eyes = "T"
	case tn-g.shock < 900:
		eyes = "O"
	case g.rate > 7:
		eyes = "><"
	}
	w := f32(math.Max(float64(box.W)*2.6, h*1.5))
	b := ui.Rect{X: box.X + box.W/2 - w/2, Y: box.Y, W: w, H: box.H}
	if b.X < box.X-w*0.15 { // it does not cover the letter before it more than a little
		b.X = box.X - w*0.15
	}
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, lilac, 0.95, 0.9)
	ink := faceInk(f.dark, ui.Hex("#2e1065"))
	size := box.H * 0.62
	cy := b.Y + b.H*0.48
	left, right := eyes, eyes
	if eyes == "><" {
		left, right = ">", "<"
	}
	lx := g.gaze() * b.W * 0.05 // the face turned the way it looks
	glyphAt(p, left, b.X+b.W*0.2+lx, cy, size, ink, true, 0, 1)
	glyphAt(p, "w", b.X+b.W*0.5+lx, cy+box.H*0.06, size*0.85, ink, true, 0, 1)
	glyphAt(p, right, b.X+b.W*0.8+lx, cy, size, ink, true, 0, 1)
	for _, ox := range []float32{0.08, 0.92} { // blush
		x := b.X + b.W*ox
		p.Fill(ui.Rect{X: x - b.H*0.08, Y: b.Y + b.H*0.66, W: b.H * 0.16, H: b.H * 0.08}, ui.RGBA(244, 114, 182, 0.7), b.H*0.04)
	}
	if len(g.tildes) > 0 || tn-g.sad < 900 || tn-g.shock < 900 || g.rate > 0.5 {
		p.AnimationFrame()
	}
}
