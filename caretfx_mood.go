package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Carets of the writing itself, each fixed as it ships:
//
//	Duygu yüzü  a face that takes the word just written: a smile for "harika", a frown for "üzgün", wide eyes
//	            at "!", a raised brow at "?", a blush when it erases; it blinks while it waits
//	Akış        writing on without erasing, it starts to glow and to turn through colors; an erase, or a long
//	            rest, breaks the flow and it dims
//	Noktalama   each mark its own: "." a bump, "," a little tail, "!" a burst, "?" a question over it, ":" two dots

// ---- Duygu yüzü ----

// moodWords are the words a face takes, folded (Turkish letters as their ASCII, lower case).
var moodWords = map[string]int{
	// 1 glad
	"harika": 1, "guzel": 1, "super": 1, "mutlu": 1, "mutluyum": 1, "sevgi": 1, "seviyorum": 1, "tesekkur": 1, "tesekkurler": 1, "iyi": 1,
	"muhtesem": 1, "bravo": 1, "yasasin": 1, "sevindim": 1, "great": 1, "good": 1, "happy": 1, "love": 1, "thanks": 1, "nice": 1, "yay": 1,
	// 2 sad
	"uzgun": 2, "uzuldum": 2, "kotu": 2, "maalesef": 2, "agladim": 2, "yorgun": 2, "yoruldum": 2, "keske": 2, "sad": 2, "bad": 2, "tired": 2, "sorry": 2,
	// 3 cross
	"kizgin": 3, "sinir": 3, "sinirli": 3, "nefret": 3, "ofkeli": 3, "angry": 3, "hate": 3,
}

const (
	moodNone = iota
	moodGlad
	moodSad
	moodCross
	moodWow
	moodHuh
	moodShy
)

type moodFace struct {
	fxBase
	word  []rune  // the word being written
	mood  int     // the face it has now
	since float64 // since when
	blink float64 // when it blinks next
}

func newMoodFace() *moodFace { return &moodFace{blink: 1500} }

func (g *moodFace) reset() { g.word, g.mood, g.have = nil, moodNone, false }

func (g *moodFace) shift(dy float32) { g.prev.Y += dy }

func (g *moodFace) feel(m int, tn float64) { g.mood, g.since = m, tn }

func (g *moodFace) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	for _, r := range k.typed {
		switch {
		case r == '!':
			g.feel(moodWow, tn)
		case r == '?':
			g.feel(moodHuh, tn)
		case r == ' ' || r == '\n' || r == '.' || r == ',' || r == ';' || r == ':':
			if m, ok := moodWords[fold(string(g.word))]; ok {
				g.feel(m, tn)
			}
			g.word = g.word[:0]
		default:
			if len(g.word) < 24 {
				g.word = append(g.word, r)
			}
		}
	}
	if k.erased > 0 {
		g.feel(moodShy, tn)
		if n := len(g.word) - k.erased; n >= 0 {
			g.word = g.word[:n]
		} else {
			g.word = g.word[:0]
		}
	}
	if tn-g.since > 2600 && g.mood != moodNone {
		g.mood = moodNone
	}
	col := map[int]ui.Color{moodNone: ui.Hex("#a78bfa"), moodGlad: ui.Hex("#facc15"), moodSad: ui.Hex("#60a5fa"), moodCross: ui.Hex("#f87171"),
		moodWow: ui.Hex("#fb923c"), moodHuh: ui.Hex("#2dd4bf"), moodShy: ui.Hex("#f9a8d4")}[g.mood]
	glassBody(p, f, box, col, 1, 0.9)
	ink := faceInk(f.dark, ui.Hex("#1c1917"))
	b := box
	er := max(1, b.W*0.11)
	blinking := false
	if tn > g.blink {
		blinking = tn-g.blink < 120
		if !blinking {
			g.blink = tn + 2000 + rnd()*2500
		}
	}
	for i, ox := range []float32{0.3, 0.7} {
		ex, ey := b.X+b.W*ox, b.Y+b.H*0.36
		switch {
		case blinking:
			p.Fill(ui.Rect{X: ex - er, Y: ey, W: 2 * er, H: max(0.8, er*0.4)}, ink, 0)
		case g.mood == moodWow:
			p.Fill(ui.Rect{X: ex - er*1.3, Y: ey - er*1.3, W: er * 2.6, H: er * 2.6}, ink, er*1.3)
			p.Fill(ui.Rect{X: ex - er*0.4, Y: ey - er*0.8, W: er * 0.8, H: er * 0.8}, ui.RGBA(255, 255, 255, 0.9), er*0.4)
		case g.mood == moodGlad: // eyes that smile: arcs
			arc := &ui.Path{}
			arc.MoveTo(ex-er, ey+er*0.4).QuadTo(ex, ey-er*1.2, ex+er, ey+er*0.4)
			p.StrokePath(arc, max(1, er*0.6), ink)
		default:
			p.Fill(ui.Rect{X: ex - er, Y: ey - er, W: 2 * er, H: 2 * er}, ink, er)
		}
		brow := &ui.Path{}
		by := b.Y + b.H*0.2
		switch {
		case g.mood == moodCross: // brows down toward the middle
			if i == 0 {
				brow.MoveTo(ex-er*1.3, by-er*0.6).LineTo(ex+er, by+er*0.3)
			} else {
				brow.MoveTo(ex-er, by+er*0.3).LineTo(ex+er*1.3, by-er*0.6)
			}
			p.StrokePath(brow, max(0.8, er*0.45), ink)
		case g.mood == moodHuh && i == 1: // one brow up
			brow.MoveTo(ex-er, by-er*0.4).QuadTo(ex, by-er*1.6, ex+er, by-er*0.6)
			p.StrokePath(brow, max(0.8, er*0.45), ink)
		case g.mood == moodSad: // brows up toward the middle
			if i == 0 {
				brow.MoveTo(ex-er*1.2, by+er*0.2).LineTo(ex+er, by-er*0.6)
			} else {
				brow.MoveTo(ex-er, by-er*0.6).LineTo(ex+er*1.2, by+er*0.2)
			}
			p.StrokePath(brow, max(0.8, er*0.45), ink)
		}
	}
	mx, my, mw := b.X+b.W/2, b.Y+b.H*0.68, b.W*0.26
	mouth := &ui.Path{}
	switch g.mood {
	case moodGlad:
		mouth.MoveTo(mx-mw, my-b.H*0.04).QuadTo(mx, my+b.H*0.16, mx+mw, my-b.H*0.04)
	case moodSad, moodCross:
		mouth.MoveTo(mx-mw, my+b.H*0.06).QuadTo(mx, my-b.H*0.1, mx+mw, my+b.H*0.06)
	case moodWow:
		r := b.W * 0.14
		p.Fill(ui.Rect{X: mx - r, Y: my - r, W: 2 * r, H: 2.2 * r}, ink, r)
	case moodHuh:
		mouth.MoveTo(mx-mw, my+b.H*0.02).LineTo(mx+mw, my-b.H*0.04)
	default:
		mouth.MoveTo(mx-mw*0.7, my).QuadTo(mx, my+b.H*0.05, mx+mw*0.7, my)
	}
	if g.mood != moodWow {
		p.StrokePath(mouth, max(0.9, b.W*0.07), ink)
	}
	if g.mood == moodShy { // cheeks
		for _, ox := range []float32{0.18, 0.82} {
			cx := b.X + b.W*ox
			p.Fill(ui.Rect{X: cx - er*1.2, Y: b.Y + b.H*0.52, W: er * 2.4, H: er * 1.2}, ui.RGBA(244, 63, 94, 0.55), er*0.6)
		}
	}
	p.AnimationFrame() // it blinks
}

// ---- Akış ----

type flow struct {
	fxBase
	start float64 // when the flow began; < 0: none
	level float64 // how deep it is: 0..1, on its way
	broke float64 // when it was broken
	motes motes
	hue   float64
}

func newFlow() *flow { return &flow{start: -1, broke: -1e9} }

func (g *flow) reset() { g.start, g.level, g.motes, g.have = -1, 0, nil, false }

func (g *flow) shift(dy float32) { g.motes.shift(dy); g.prev.Y += dy }

func (g *flow) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.erased > 0 || g.idle(tn) > 3.5 { // broken
		if g.start >= 0 && g.level > 0.15 {
			g.broke = tn
		}
		g.start = -1
	}
	if k.wrote > 0 && g.start < 0 {
		g.start = tn
	}
	want := 0.0
	if g.start >= 0 {
		want = clamp01((tn - g.start) / 60000) // a minute without erasing: the whole of it
	}
	rate := 0.6
	if want < g.level {
		rate = 3
	}
	g.level += (want - g.level) * math.Min(1, dt*rate)
	g.hue += dt * (20 + 160*g.level)
	base := ui.Hex("#94a3b8")
	col := mix(base, hsl(g.hue, 0.85, 0.6), smoothstep(g.level*2))
	glow := 0.6 + 2.4*g.level
	if bk := (tn - g.broke) / 400; bk < 1 { // the break: a dim flash
		col = mix(col, ui.Hex("#475569"), 1-bk)
	}
	if g.level > 0.45 && rnd() < g.level*0.6 { // deep in it, motes rise
		g.motes = append(g.motes, mote{x: float64(box.X) + rnd()*float64(box.W), y: float64(box.Y) + h*0.3, vx: (rnd() - 0.5) * 10, vy: -25 - rnd()*25, t0: tn, life: 900, size: 1 + rnd()*1.6,
			col: hsl(g.hue+rnd()*60, 0.9, 0.7)})
	}
	g.motes.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s, W: 2 * s, H: 2 * s}, withAlpha(m.col, 1-age), s)
	})
	glassBody(p, f, box, col, 1, glow)
	if g.level > 0.005 || len(g.motes) > 0 || tn-g.broke < 400 || g.start >= 0 {
		p.AnimationFrame()
	}
}

// ---- Noktalama ----

type punct struct {
	fxBase
	bump  float64 // "."
	tail  float64 // ","
	ask   float64 // "?"
	colon float64 // ":"
	dash  float64 // "-"
	burst motes   // "!"
}

func newPunct() *punct { return &punct{bump: -1e9, tail: -1e9, ask: -1e9, colon: -1e9, dash: -1e9} }

func (g *punct) reset() { g.burst, g.have = nil, false }

func (g *punct) shift(dy float32) { g.burst.shift(dy); g.prev.Y += dy }

func (g *punct) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	accent := ui.Hex("#8b5cf6")
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	for _, r := range k.typed {
		switch r {
		case '.':
			g.bump = tn
		case ',':
			g.tail = tn
		case '?':
			g.ask = tn
		case ':', ';':
			g.colon = tn
		case '-':
			g.dash = tn
		case '!':
			for i := 0; i < 12; i++ {
				a := float64(i) / 12 * 2 * math.Pi
				sp := 110 + rnd()*60
				g.burst = append(g.burst, mote{x: cx, y: cy, vx: math.Cos(a) * sp, vy: math.Sin(a) * sp, t0: tn, life: 420, size: 1.6 + rnd(),
					col: hsl(rnd()*360, 0.9, 0.6)})
			}
		}
	}
	b := box
	if bk := (tn - g.bump) / 240; bk < 1 { // a full stop: it bumps the floor
		s := math.Sin(bk * math.Pi)
		b.H *= f32(1 - 0.22*s)
		b.Y += box.H - b.H
		b.X -= f32(s * 1.2)
		b.W += f32(s * 2.4)
		dust := f32(s) * box.W
		p.Fill(ui.Rect{X: box.X - dust, Y: box.Y + box.H, W: box.W + 2*dust, H: 1.2}, withAlpha(accent, 0.4*(1-bk)), 0.6)
	}
	g.burst.stepDraw(p, tn, dt, func(m *mote, age float64) {
		m.vx *= 0.9
		m.vy *= 0.9
		s := f32(m.size * (1 - age*0.5))
		p.Fill(ui.Rect{X: f32(m.x) - s, Y: f32(m.y) - s, W: 2 * s, H: 2 * s}, withAlpha(m.col, 1-age), s)
	})
	fr := f
	fr.jelly = nil
	glassBody(p, fr, b, accent, 1, 0.9)
	if tk := (tn - g.tail) / 420; tk < 1 { // a comma: a little tail curls from its foot
		l := easeOut(tk)
		tl := &ui.Path{}
		x0, y0 := b.X+b.W*0.6, b.Y+b.H
		tl.MoveTo(x0, y0).QuadTo(x0+b.W*0.3, y0+f32(h*0.25*l), x0-b.W*0.3, y0+f32(h*0.35*l))
		p.StrokePath(tl, max(1.2, b.W*0.25), withAlpha(accent, 1-tk*tk))
	}
	if ak := (tn - g.ask) / 1000; ak < 1 { // a question: "?" rises over it
		glyphAt(p, "?", b.X+b.W/2, b.Y-f32(h*(0.3+0.4*easeOut(ak))), f32(h*0.6), withAlpha(accent, 1-ak*ak), true, 0, 1)
	}
	if ck := (tn - g.colon) / 500; ck < 1 { // a colon: two dots pop beside it
		for _, oy := range []float64{0.3, 0.7} {
			r := f32(h * 0.08 * math.Sin(math.Min(1, ck*2)*math.Pi))
			x, y := b.X+b.W+f32(h*0.25), b.Y+b.H*f32(oy)
			if r > 0 {
				p.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, withAlpha(accent, 1-ck), r)
			}
		}
	}
	if dk := (tn - g.dash) / 300; dk < 1 { // a dash: a swish across
		w := box.W * 3 * f32(easeOut(dk))
		p.Fill(ui.Rect{X: b.X - w, Y: b.Y + b.H*0.5, W: w, H: max(1, b.H*0.07)}, withAlpha(accent, 0.6*(1-dk)), 1)
	}
	if len(g.burst) > 0 || tn-g.bump < 240 || tn-g.tail < 420 || tn-g.ask < 1000 || tn-g.colon < 500 || tn-g.dash < 300 {
		p.AnimationFrame()
	}
}
