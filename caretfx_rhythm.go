package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Carets of rhythm and time, each fixed as it ships:
//
//	Ekolayzır       little bars beside it that jump with every key and fall back, higher the faster the writing
//	Metronom        a pendulum over it: writing at an even pace, it swings in time with you and glows; uneven, it stops
//	Davul           every key a small ring, a space a bigger one, Enter a flash and a shimmer, as a cymbal
//	Kum saati       an hourglass: writing fills its top, at rest the sand runs down
//	Kelime sayacı   a badge over it with its paragraph's words, that pops at every fifty
//	Yazım ısısı     a strip under the line behind it: the last twenty seconds of writing, cool to hot

// ---- Ekolayzır ----

type equalizer struct {
	fxBase
	bars  [6]float64
	peaks [6]float64
	vel   [6]float64
}

func newEqualizer() *equalizer { return &equalizer{} }

func (g *equalizer) reset() { g.have = false }

func (g *equalizer) shift(dy float32) { g.prev.Y += dy }

func (g *equalizer) draw(p *ui.Painter, f caretFrame, k kick) {
	_, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	busy := false
	for i := range g.bars {
		if k.wrote > 0 { // a key: each bar jumps, some more than others
			g.bars[i] = math.Min(1, g.bars[i]+(0.35+rnd()*0.55)*float64(k.wrote))
		}
		g.bars[i] = math.Max(0, g.bars[i]-dt*1.8)
		if g.bars[i] > g.peaks[i] {
			g.peaks[i], g.vel[i] = g.bars[i], 0
		} else { // the peak falls slower, as on an old stereo
			g.vel[i] += dt * 2
			g.peaks[i] = math.Max(0, g.peaks[i]-g.vel[i]*dt)
		}
		if g.bars[i] > 0 || g.peaks[i] > 0 {
			busy = true
		}
	}
	bw := math.Max(2, h*0.14)
	x0 := float64(box.X+box.W) + h*0.25
	base := float64(box.Y + box.H)
	for i, v := range g.bars {
		x := x0 + float64(i)*(bw+1.5)
		bh := v * h * 1.4
		col := hsl(140-140*v, 0.85, 0.55) // green, to yellow, to red
		if bh > 0.5 {
			p.Fill(ui.Rect{X: f32(x), Y: f32(base - bh), W: f32(bw), H: f32(bh)}, withAlpha(col, 0.85), 1)
		}
		if pk := g.peaks[i]; pk > 0.02 {
			p.Fill(ui.Rect{X: f32(x), Y: f32(base - pk*h*1.4 - 2), W: f32(bw), H: 1.5}, withAlpha(ui.Hex("#f8fafc"), 0.9), 0)
		}
	}
	glassBody(p, f, box, ui.Hex("#22c55e"), 1, 0.9)
	if busy {
		p.AnimationFrame()
	}
}

// ---- Metronom ----

type metronome struct {
	fxBase
	lastKey  float64
	interval float64 // ms between keys, smoothed
	spread   float64 // how uneven, smoothed
	steady   float64 // 0..1: how in time
	angle    float64
	vel      float64
	phase    float64
}

func newMetronome() *metronome { return &metronome{interval: 300, spread: 1} }

func (g *metronome) reset() { g.have = false }

func (g *metronome) shift(dy float32) { g.prev.Y += dy }

func (g *metronome) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.wrote > 0 {
		if g.lastKey > 0 {
			iv := tn - g.lastKey
			if iv < 1500 {
				dev := math.Abs(iv-g.interval) / math.Max(1, g.interval)
				g.interval += (iv - g.interval) * 0.25
				g.spread += (dev - g.spread) * 0.25
			}
		}
		g.lastKey = tn
	}
	want := 0.0
	if tn-g.lastKey < 1200 { // in time: the keys come at an even pace
		want = clamp01(1 - g.spread/0.35)
	}
	g.steady += (want - g.steady) * math.Min(1, dt*3)
	// the pendulum: in time, it swings with your keys; out of it, it comes to rest
	g.phase += dt * 1000 / math.Max(120, g.interval) * math.Pi
	target := math.Sin(g.phase) * 0.6 * g.steady
	g.vel += (60*(target-g.angle) - 8*g.vel) * dt
	g.angle += g.vel * dt
	amber := ui.Hex("#f59e0b")
	px, py := float64(box.X+box.W/2), float64(box.Y)-h*0.15
	l := h * 1.3
	tip := [2]float64{px + math.Sin(g.angle)*l, py - math.Cos(g.angle)*l}
	arm := &ui.Path{}
	arm.MoveTo(f32(px), f32(py)).LineTo(f32(tip[0]), f32(tip[1]))
	p.StrokePath(arm, 1.4, withAlpha(faceInk(f.dark, ui.Hex("#57534e")), 0.9))
	wr := h * 0.13
	wy := py - math.Cos(g.angle)*l*0.65
	wx := px + math.Sin(g.angle)*l*0.65
	p.Fill(ui.Rect{X: f32(wx - wr), Y: f32(wy - wr), W: f32(2 * wr), H: f32(2 * wr)}, withAlpha(amber, 0.6+0.4*g.steady), f32(wr*0.4))
	glassBody(p, f, box, mix(ui.Hex("#94a3b8"), amber, g.steady), 1, 0.6+1.4*g.steady)
	if g.steady > 0.01 || math.Abs(g.angle) > 0.002 || math.Abs(g.vel) > 0.01 {
		p.AnimationFrame()
	}
}

// ---- Davul ----

type drum struct {
	fxBase
	rings []float64 // when each ring began
	big   []float64 // how big: 1 a key, 2 a space, 3 Enter
	rx    []float64
	ry    []float64
	flash float64
}

func newDrum() *drum { return &drum{flash: -1e9} }

func (g *drum) reset() { g.rings, g.big, g.rx, g.ry, g.have = nil, nil, nil, nil, false }

func (g *drum) shift(dy float32) {
	for i := range g.ry {
		g.ry[i] += float64(dy)
	}
	g.prev.Y += dy
}

func (g *drum) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)
	add := func(b float64) {
		g.rings, g.big, g.rx, g.ry = append(g.rings, tn), append(g.big, b), append(g.rx, cx), append(g.ry, cy)
	}
	switch {
	case strings.Contains(k.typed, "\n"):
		add(3)
		g.flash = tn
	case strings.Contains(k.typed, " "):
		add(2)
	case k.wrote > 0:
		add(1)
	}
	gold := ui.Hex("#eab308")
	keep := 0
	for i, t0 := range g.rings {
		life := 300 + 200*g.big[i]
		u := (tn - t0) / life
		if u >= 1 {
			continue
		}
		g.rings[keep], g.big[keep], g.rx[keep], g.ry[keep] = t0, g.big[i], g.rx[i], g.ry[i]
		keep++
		r := h * (0.6 + g.big[i]*0.9*easeOut(u))
		p.StrokePath((&ui.Path{}).Circle(f32(g.rx[i]), f32(g.ry[i]), f32(r)), f32(1+g.big[i]*0.6*(1-u)), withAlpha(gold, (1-u)*0.8))
	}
	g.rings, g.big, g.rx, g.ry = g.rings[:keep], g.big[:keep], g.rx[:keep], g.ry[:keep]
	if fk := (tn - g.flash) / 600; fk < 1 { // the cymbal: a flash, and a shimmer of short lines round it
		for i := 0; i < 10; i++ {
			a := float64(i)/10*2*math.Pi + tn/200
			r0, r1 := h*(1+fk), h*(1.4+fk*1.5)
			ln := &ui.Path{}
			ln.MoveTo(f32(cx+math.Cos(a)*r0), f32(cy+math.Sin(a)*r0)).LineTo(f32(cx+math.Cos(a)*r1), f32(cy+math.Sin(a)*r1))
			p.StrokePath(ln, 1.2, withAlpha(ui.Hex("#fef9c3"), 1-fk))
		}
	}
	glow := 0.8
	if fk := (tn - g.flash) / 300; fk < 1 {
		glow += 2 * (1 - fk)
	}
	glassBody(p, f, box, gold, 1, glow)
	if len(g.rings) > 0 || tn-g.flash < 600 {
		p.AnimationFrame()
	}
}

// ---- Kum saati ----

type hourglass struct {
	fxBase
	top, bottom float64 // the sand in each half: 0..1
	flip        float64 // when it turned over
}

func newHourglass() *hourglass { return &hourglass{flip: -1e9} }

func (g *hourglass) reset() { g.have = false }

func (g *hourglass) shift(dy float32) { g.prev.Y += dy }

func (g *hourglass) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if k.wrote > 0 { // writing fills the top
		g.top = math.Min(1, g.top+0.015*float64(k.wrote))
	}
	running := g.idle(tn) > 1.2 && g.top > 0
	if running { // at rest the sand runs down, slowly
		d := math.Min(g.top, dt*0.05)
		g.top -= d
		g.bottom = math.Min(1, g.bottom+d)
		if g.top <= 0 && g.bottom > 0.95 { // all run through: it turns over, empty, for the next
			g.flip, g.bottom = tn, 0
		}
	}
	// the glass: two bulbs that meet at a waist, sand in each
	w, hh := math.Max(float64(box.W)*1.5, h*0.75), h*1.3
	x, y := float64(box.X+box.W/2)-w/2, float64(box.Y)-h*0.15
	ang := 0.0
	if fk := (tn - g.flip) / 500; fk < 1 {
		ang = easeOut(fk) * 180
	}
	sand := ui.Hex("#f59e0b")
	frame := faceInk(f.dark, ui.Hex("#78350f"))
	cx, cy := f32(x+w/2), f32(y+hh/2)
	p.Transform(ui.Rotation(f32(ang)), cx, cy, func() {
		glass := &ui.Path{}
		glass.MoveTo(f32(x), f32(y)).LineTo(f32(x+w), f32(y)).LineTo(f32(x+w*0.56), f32(y+hh/2)).LineTo(f32(x+w), f32(y+hh)).
			LineTo(f32(x), f32(y+hh)).LineTo(f32(x+w*0.44), f32(y+hh/2)).Close()
		glassFill(p, glass, ui.Hex("#bae6fd"), ui.Hex("#bae6fd"), f.dark, 0.7)
		if g.top > 0.01 { // the top: a triangle of sand, lower as it runs
			lv := hh / 2 * g.top
			ty := y + hh/2 - lv
			hw := (w / 2) * (lv / (hh / 2)) * 0.88
			t := &ui.Path{}
			t.MoveTo(f32(x+w/2-hw), f32(ty)).LineTo(f32(x+w/2+hw), f32(ty)).LineTo(f32(x+w/2), f32(y+hh/2)).Close()
			p.FillPath(t, withAlpha(sand, 0.95))
		}
		if g.bottom > 0.01 { // the bottom: a heap
			lv := hh / 2 * g.bottom
			b := &ui.Path{}
			b.MoveTo(f32(x+w*0.06), f32(y+hh)).LineTo(f32(x+w*0.94), f32(y+hh)).LineTo(f32(x+w/2), f32(y+hh-lv)).Close()
			p.FillPath(b, withAlpha(sand, 0.95))
		}
		if running { // the stream through the waist
			p.Fill(ui.Rect{X: cx - 0.6, Y: cy, W: 1.2, H: f32(hh/2 - hh/2*g.bottom)}, withAlpha(sand, 0.9), 0)
		}
		p.Fill(ui.Rect{X: f32(x - 1), Y: f32(y - 1.5), W: f32(w + 2), H: 1.5}, frame, 0.7)
		p.Fill(ui.Rect{X: f32(x - 1), Y: f32(y + hh), W: f32(w + 2), H: 1.5}, frame, 0.7)
	})
	if running || tn-g.flip < 500 {
		p.AnimationFrame()
	}
}

// ---- Kelime sayacı ----

type wordBadge struct {
	fxBase
	count int
	pop   float64 // when it passed a fifty
	bump  float64 // when it changed
}

func newWordBadge() *wordBadge { return &wordBadge{pop: -1e9, bump: -1e9} }

func (g *wordBadge) reset() { g.have, g.count = false, 0 }

func (g *wordBadge) shift(dy float32) { g.prev.Y += dy }

func (g *wordBadge) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	if f.words != nil && (k.wrote > 0 || k.erased > 0 || g.count == 0) {
		n := f.words.paraWords()
		if n != g.count {
			if n/50 > g.count/50 && g.count > 0 {
				g.pop = tn
			}
			g.count, g.bump = n, tn
		}
	}
	teal := ui.Hex("#14b8a6")
	glassBody(p, f, box, teal, 1, 0.8)
	if g.count == 0 {
		return
	}
	label := strconv.Itoa(g.count)
	size := f32(math.Max(9, h*0.5))
	span := ui.Span{Text: label, Size: size, Weight: 720, Color: ui.RGBA(255, 255, 255, 1)}
	tw, th := p.MeasureText(0, span)
	sc := 1.0
	if bk := (tn - g.bump) / 220; bk < 1 {
		sc += 0.12 * math.Sin(bk*math.Pi)
	}
	col := teal
	if pk := (tn - g.pop) / 900; pk < 1 { // a fifty: it pops, gold
		sc += 0.45 * math.Sin(math.Min(1, pk*2)*math.Pi) * (1 - pk)
		col = mix(ui.Hex("#eab308"), teal, pk)
		for i := 0; i < 8; i++ {
			a := float64(i) / 8 * 2 * math.Pi
			r := h * (0.9 + pk*1.4)
			bx, by := float64(box.X+box.W)+h*0.6, float64(box.Y)-h*0.6
			p.Fill(ui.Rect{X: f32(bx + math.Cos(a)*r - 1), Y: f32(by + math.Sin(a)*r - 1), W: 2, H: 2}, withAlpha(col, 1-pk), 1)
		}
	}
	bw, bh := (tw+f32(h*0.5))*f32(sc), (th+f32(h*0.15))*f32(sc)
	bx, by := box.X+box.W+f32(h*0.2), box.Y-bh-f32(h*0.15)
	p.Fill(ui.Rect{X: bx, Y: by, W: bw, H: bh}, withAlpha(col, 0.9), bh/2)
	glyphAt(p, label, bx+bw/2, by+bh/2, size*f32(sc), ui.RGBA(255, 255, 255, 1), true, 0, 1)
	if tn-g.bump < 220 || tn-g.pop < 900 {
		p.AnimationFrame()
	}
}

// ---- Yazım ısısı ----

type writingHeat struct {
	fxBase
	secs [20]float64 // keys in each of the last twenty seconds, the newest last
	at   float64     // the second the newest is
}

func newWritingHeat() *writingHeat { return &writingHeat{} }

func (g *writingHeat) reset() { g.have = false }

func (g *writingHeat) shift(dy float32) { g.prev.Y += dy }

func (g *writingHeat) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	sec := math.Floor(tn / 1000)
	for g.at < sec { // the seconds go by: the strip slides along
		copy(g.secs[:], g.secs[1:])
		g.secs[len(g.secs)-1] = 0
		g.at++
		if sec-g.at > float64(len(g.secs)) {
			g.at = sec
			g.secs = [20]float64{}
		}
	}
	g.secs[len(g.secs)-1] += float64(k.wrote)
	cw := math.Max(2, h*0.22)
	x1 := float64(box.X) - 2
	y := float64(box.Y+box.H) + 2
	any := false
	for i := len(g.secs) - 1; i >= 0; i-- {
		v := clamp01(g.secs[i] / 8) // eight keys a second is as hot as it gets
		if g.secs[i] > 0 {
			any = true
		}
		x := x1 - float64(len(g.secs)-i)*(cw+1)
		col := hsl(220-220*v, 0.8, 0.55-0.1*v) // blue, through green and yellow, to red
		a := 0.15 + 0.75*v
		if g.secs[i] == 0 { // a quiet second: a faint cell, so the strip shows as a strip
			col, a = ui.Hex("#94a3b8"), 0.18
		}
		p.Fill(ui.Rect{X: f32(x), Y: f32(y), W: f32(cw), H: f32(math.Max(3, h*0.2))}, withAlpha(col, a), 1.5)
	}
	v := clamp01(g.secs[len(g.secs)-1] / 8)
	glassBody(p, f, box, hsl(220-220*v, 0.8, 0.55), 1, 0.8+v)
	if any {
		p.AnimationFrame()
	}
}
