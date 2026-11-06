package main

import (
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// Cursor-Smith's writing carets (MIT, see NOTICE.md), each as it ships and with nothing to
// set: letters that pop off the caret, a lightning bolt on Enter, a firework on Space, the sparks of
// an anvil, and the typewriter's strike, ink and carriage return.

// typedLetter is a letter just written, where it stands: the caret is right after it.
type typedLetter struct {
	ch   string
	x, w float64
}

func typedLetters(p *ui.Painter, f caretFrame, k kick) (letters []typedLetter, enter, space bool) {
	if k.typed == "" {
		return
	}
	runes, widths := letterWidths(p, k.typed, textSize(f))
	x := float64(f.r.X)
	for i := len(runes) - 1; i >= 0; i-- {
		switch runes[i] {
		case "\n":
			enter = true
			continue
		case " ":
			space = true
		}
		x -= widths[i]
		letters = append([]typedLetter{{runes[i], x, widths[i]}}, letters...)
	}
	return
}

// clock keeps a time of its own for a caret.
type clock struct{ base time.Time }

func (c *clock) at(p *ui.Painter) float64 {
	now := p.Now()
	if c.base.IsZero() {
		c.base = now
	}
	return ms(now.Sub(c.base))
}

// ---- letter pop ----

type popLetter struct {
	ch          string
	x0, y0      float64
	vx, vy, rot float64
	t0          float64
	size        float32
	hue         float64
}

type letterPop struct {
	clock
	list []popLetter
	rise bool
}

func newLetterPop(rise bool) *letterPop { return &letterPop{rise: rise} }

func (g *letterPop) reset() { g.list = nil }

func (g *letterPop) shift(dy float32) {
	for i := range g.list {
		g.list[i].y0 += float64(dy)
	}
}

func (g *letterPop) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	drawBoxCursor(p, boxOf(f), col, 1)
	letters, _, _ := typedLetters(p, f, k)
	for _, l := range letters {
		if strings.TrimSpace(l.ch) == "" {
			continue
		}
		g.list = append(g.list, popLetter{ch: l.ch, x0: l.x + l.w/2, y0: float64(f.r.Y), vx: (rnd() - 0.5) * 120, vy: -150 - rnd()*130, rot: (rnd() - 0.5) * 4, t0: tn, size: textSize(f)})
	}
	out := g.list[:0]
	for _, l := range g.list {
		if g.rise {
			u := (tn - l.t0) / 650
			if u >= 1 {
				continue
			}
			out = append(out, l)
			y := l.y0 - float64(f.r.H)*1.6*(1-math.Pow(1-u, 3))
			glyphAt(p, l.ch, f32(l.x0), f32(y), l.size*0.95, withAlpha(col, 0.9*math.Pow(1-u, 1.4)), false, 0, 1)
			continue
		}
		el := (tn - l.t0) / 1000
		if el > 0.45 {
			continue
		}
		out = append(out, l)
		t := el / 0.45
		x := l.x0 + l.vx*el
		y := l.y0 + l.vy*el + 0.5*320*el*el
		glyphAt(p, l.ch, f32(x), f32(y), l.size*0.9, withAlpha(col, 1-t), true, l.rot*el*5, 1)
	}
	g.list = out
	if len(g.list) > 0 {
		p.AnimationFrame()
	}
}

// ---- lightning on Enter ----

type bolt struct {
	cells   [][3]float64 // x, y, t along the bolt
	t0      float64
	flicker [8]float64
	tx, ty  float64
	cell    float64
	sparks  []boltSpark
	flash   float64
}

type boltSpark struct{ x, y, vx, vy, size float64 }

type thunder struct {
	clock
	bolts []bolt
	prev  ui.Rect
	have  bool
}

func newThunder() *thunder { return &thunder{} }

func (g *thunder) reset() { g.bolts, g.have = nil, false }

func (g *thunder) shift(dy float32) {
	for i := range g.bolts {
		g.bolts[i].ty += float64(dy)
		for j := range g.bolts[i].cells {
			g.bolts[i].cells[j][1] += float64(dy)
		}
		for j := range g.bolts[i].sparks {
			g.bolts[i].sparks[j].y += float64(dy)
		}
	}
}

// boltPath bends the line from (x0,y0) to (x1,y1) by halving it five times, each time a notch sideways.
func boltPath(x0, y0, x1, y1, jitter float64) [][2]float64 {
	pts := [][2]float64{{x0, y0}, {x1, y1}}
	amp := jitter
	for pass := 0; pass < 5; pass++ {
		next := [][2]float64{pts[0]}
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			dx, dy := b[0]-a[0], b[1]-a[1]
			l := math.Hypot(dx, dy)
			off := (rnd() - 0.5) * 2 * amp
			mx, my := (a[0]+b[0])/2, (a[1]+b[1])/2
			if l > 0 {
				mx += -dy / l * off
				my += dx / l * off
			}
			next = append(next, [2]float64{mx, my}, b)
		}
		pts = next
		amp *= 0.55
	}
	return pts
}

func (g *thunder) strike(tx, ty float64, cw float64, h float64, tn float64, cell float64) {
	angle := (rnd() - 0.5) * 2 * 0.5
	reach := 260 / math.Max(0.35, math.Cos(angle))
	ox, oy := tx+math.Sin(angle)*reach, ty-math.Cos(angle)*reach
	b := bolt{t0: tn, tx: tx, ty: ty, cell: cell, flash: math.Max(cell*2, h*0.4)}
	seen := map[[2]float64]bool{}
	emit := func(pts [][2]float64, t0, t1 float64) {
		n := float64(len(pts) - 1)
		for i := 1; i < len(pts); i++ {
			a, c := pts[i-1], pts[i]
			d := math.Hypot(c[0]-a[0], c[1]-a[1])
			steps := int(math.Max(1, math.Ceil(d/(cell*0.7))))
			for s := 0; s <= steps; s++ {
				px, py := a[0]+(c[0]-a[0])*float64(s)/float64(steps), a[1]+(c[1]-a[1])*float64(s)/float64(steps)
				g := [2]float64{math.Round(px/cell) * cell, math.Round(py/cell) * cell}
				if seen[g] {
					continue
				}
				seen[g] = true
				b.cells = append(b.cells, [3]float64{g[0], g[1], t0 + ((float64(i-1)+float64(s)/float64(steps))/n)*(t1-t0)})
			}
		}
	}
	main := boltPath(ox, oy, tx, ty, reach*0.09)
	emit(main, 0, 1)
	forks := 0
	if rnd() < 0.75 {
		forks++
	}
	if rnd() < 0.2 {
		forks++
	}
	for i := 0; i < forks; i++ {
		ft := 0.15 + rnd()*0.4
		st := main[int(float64(len(main))*ft)]
		side := 1.0
		if rnd() < 0.5 {
			side = -1
		}
		spread := math.Max(-1.1, math.Min(1.1, angle+side*(0.45+rnd()*0.55)))
		l := reach * (0.15 + rnd()*0.18)
		emit(boltPath(st[0], st[1], st[0]+math.Sin(spread)*l, st[1]+math.Cos(spread)*l, l*0.16), ft, math.Min(1, ft+0.3))
	}
	b.flicker[0] = 1
	for i := 1; i < 8; i++ {
		b.flicker[i] = 0.6 + rnd()*0.4
	}
	for i, n := 0, 3+int(rnd()*3); i < n; i++ {
		dir := (rnd() - 0.5) * math.Pi
		speed := 30 + rnd()*45
		b.sparks = append(b.sparks, boltSpark{tx + (rnd()-0.5)*cw, ty + rnd()*h*0.4, math.Sin(dir) * speed, -math.Abs(math.Cos(dir)) * speed * 0.8, math.Max(1, cell*(0.5+rnd()*0.5))})
	}
	g.bolts = append(g.bolts, b)
	if len(g.bolts) > 4 {
		g.bolts = g.bolts[1:]
	}
}

var boltRamp = []ui.Color{ui.Hex("#d8ecff"), ui.Hex("#9ec5ff"), ui.Hex("#b794ff"), ui.Hex("#6ee7ff")}

func boltColor(t float64) ui.Color {
	t = clamp01(t) * float64(len(boltRamp)-1)
	i := int(math.Min(float64(len(boltRamp)-2), math.Floor(t)))
	return mix(boltRamp[i], boltRamp[i+1], t-float64(i))
}

func (g *thunder) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	box := boxOf(f)
	drawBoxCursor(p, box, col, 1)
	_, enter, _ := typedLetters(p, f, k)
	if enter {
		g.strike(float64(box.X+box.W/2), float64(box.Y), float64(box.W), float64(box.H), tn, 3)
	}
	out := g.bolts[:0]
	for _, b := range g.bolts {
		t := (tn - b.t0) / 450
		if t >= 1 {
			continue
		}
		out = append(out, b)
		fade := 1.0
		if t >= 0.12 {
			fade = 1 - (t-0.12)/0.88
		}
		step := int(math.Min(7, math.Floor(t*8)))
		alpha := fade * b.flicker[step] * 0.9
		if alpha <= 0.02 {
			continue
		}
		for _, c := range b.cells {
			band := math.Round(c[2] * 13)
			cc := boltColor(band / 13)
			p.Fill(ui.Rect{X: f32(c[0] - b.cell), Y: f32(c[1] - b.cell), W: f32(b.cell * 3), H: f32(b.cell * 3)}, withAlpha(cc, alpha*0.16), 0)
		}
		for _, c := range b.cells {
			band := math.Round(c[2] * 13)
			cc := mix(boltColor(band/13), ui.RGBA(255, 255, 255, 1), 0.45)
			p.Fill(ui.Rect{X: f32(c[0]), Y: f32(c[1]), W: f32(b.cell), H: f32(b.cell)}, withAlpha(cc, alpha), 0)
		}
		if fl := 1 - math.Min(1, t/0.4); fl > 0 {
			sz := b.flash * (0.5 + fl)
			p.Fill(ui.Rect{X: f32(b.tx - sz/2), Y: f32(b.ty - sz/2), W: f32(sz), H: f32(sz)}, withAlpha(mix(boltColor(1), ui.RGBA(255, 255, 255, 1), 0.45), alpha*fl*0.4), 0)
		}
		el := (tn - b.t0) / 1000
		for _, s := range b.sparks {
			a := fade * (1 - t)
			p.Fill(ui.Rect{X: f32(s.x + s.vx*el), Y: f32(s.y + s.vy*el + 0.5*200*el*el), W: f32(s.size), H: f32(s.size)}, withAlpha(ui.Hex("#d8f7ff"), a), 0)
		}
	}
	g.bolts = out
	if len(g.bolts) > 0 {
		p.AnimationFrame()
	}
}

// ---- fireworks on Space ----

type fwSpark struct {
	ang, speed, size, tw, tr float64
	ci                       int
}

type firework struct {
	t0        float64
	x0, y0    float64
	bx, by    float64
	palette   []ui.Color
	sparks    []fwSpark
	riseMs    float64
	flash, lh float64
}

type fireworks struct {
	clock
	list     []firework
	lastShot float64
}

func newFireworks() *fireworks { return &fireworks{lastShot: -1e9} }

func (g *fireworks) reset() { g.list = nil }

func (g *fireworks) shift(dy float32) {
	for i := range g.list {
		g.list[i].y0 += float64(dy)
		g.list[i].by += float64(dy)
	}
}

const (
	fwCell    = 3.0
	fwFall    = 1100.0
	fwGravity = 260.0
)

func (g *fireworks) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	box := boxOf(f)
	drawBoxCursor(p, box, col, 1)
	_, _, space := typedLetters(p, f, k)
	if space && tn-g.lastShot > 90 && len(g.list) < 6 {
		g.lastShot = tn
		lh := float64(box.H)
		fw := firework{t0: tn, x0: float64(box.X + box.W/2), y0: float64(box.Y), lh: lh}
		rise := lh * (3.5 + (rnd()-0.5)*0.5)
		fw.bx = fw.x0 + (rnd()-0.5)*28
		fw.by = math.Min(fw.y0-lh*0.9, fw.y0-rise)
		base := rnd() * 360
		for i := 0; i < 4; i++ {
			fw.palette = append(fw.palette, hsl(base+float64(i)*28+rnd()*20, 0.85, 0.62))
		}
		for i := 0; i < 12; i++ {
			sz := fwCell
			if rnd() < 0.22 {
				sz = fwCell * 2
			}
			fw.sparks = append(fw.sparks, fwSpark{ang: rnd() * 2 * math.Pi, speed: lh * (3.4 + rnd()*6.2), size: sz, tw: rnd() * 2 * math.Pi, tr: 9 + rnd()*14, ci: int(rnd() * 4)})
		}
		fw.riseMs = 420 * (0.85 + rnd()*0.3)
		fw.flash = math.Max(2*fwCell, lh*0.32)
		g.list = append(g.list, fw)
	}
	out := g.list[:0]
	for _, fw := range g.list {
		t := tn - fw.t0
		if t >= fw.riseMs+fwFall {
			continue
		}
		out = append(out, fw)
		if t < fw.riseMs {
			pp := t / fw.riseMs
			e := 1 - (1-pp)*(1-pp)
			cx, cy := fw.x0+(fw.bx-fw.x0)*e, fw.y0+(fw.by-fw.y0)*e
			a := 0.9 * math.Min(1, 5*pp)
			dx, dy := fw.bx-fw.x0, fw.by-fw.y0
			if d := math.Hypot(dx, dy); d > 0 {
				dx, dy = dx/d, dy/d
			}
			for kk := 2; kk >= 1; kk-- {
				pix(p, cx-dx*fwCell*2*float64(kk), cy-dy*fwCell*2*float64(kk), fwCell, fwCell, ui.RGBA(255, 255, 255, float32(a*0.18/float64(kk))))
			}
			pix(p, cx, cy, fwCell, fwCell, ui.RGBA(255, 245, 220, float32(a)))
			continue
		}
		u := (t - fw.riseMs) / fwFall
		el := u * fwFall / 1000
		a := 0.9 * math.Max(0, 1-u*u)
		for _, s := range fw.sparks {
			if u > 0.5 && math.Sin(s.tw+u*s.tr) < -0.35 {
				continue
			}
			vx, vy := math.Cos(s.ang)*s.speed, math.Sin(s.ang)*s.speed
			pix(p, fw.bx+vx*el, fw.by+vy*el+0.5*fwGravity*el*el, fwCell, f32(s.size), withAlpha(fw.palette[s.ci], a))
		}
		if fl := 1 - math.Min(1, u/0.18); fl > 0 {
			sz := fw.flash * (0.4 + fl)
			p.Fill(ui.Rect{X: f32(fw.bx - sz/2), Y: f32(fw.by - sz/2), W: f32(sz), H: f32(sz)}, ui.RGBA(255, 252, 240, float32(0.9*fl*0.5)), 0)
		}
	}
	g.list = out
	if len(g.list) > 0 {
		p.AnimationFrame()
	}
}

// ---- anvil sparks on Space after a word ----

type anvilSpark struct {
	x0, y0, vx, vy, life, tint float64
}

type anvilBurst struct {
	t0     float64
	sparks []anvilSpark
	g      float64
	cell   float64
}

type anvil struct {
	clock
	list     []anvilBurst
	wordW    float64 // how wide the word being written is
	lastKick float64
}

func newAnvil() *anvil { return &anvil{lastKick: -1e9} }

func (g *anvil) reset() { g.list, g.wordW = nil, 0 }

func (g *anvil) shift(dy float32) {
	for i := range g.list {
		for j := range g.list[i].sparks {
			g.list[i].sparks[j].y0 += float64(dy)
		}
	}
}

var anvilPalette = []ui.Color{ui.Hex("#fff4d6"), ui.Hex("#ffc24a"), ui.Hex("#ff7a1f"), ui.Hex("#c8321a")}

func (g *anvil) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	box := boxOf(f)
	drawBoxCursor(p, box, cursorColor(f.dark), 1)
	letters, _, space := typedLetters(p, f, k)
	for _, l := range letters {
		if l.ch == " " {
			continue
		}
		g.wordW += l.w
	}
	if space {
		if g.wordW > 0 && tn-g.lastKick > 120 {
			g.lastKick = tn
			em := float64(textSize(f))
			right := float64(box.X) - em*0.3
			left := right - g.wordW
			cx, ww := (left+right)/2, math.Max(1, g.wordW)
			b := anvilBurst{t0: tn, g: 16 * em, cell: math.Max(2, math.Round(0.2*em))}
			y0 := float64(box.Y+box.H) + 0.15*em
			for i := 0; i < 14; i++ {
				ang := 0.12*math.Pi + rnd()*(0.88-0.12)*math.Pi
				speed := em * (4 + rnd()*6)
				b.sparks = append(b.sparks, anvilSpark{x0: cx + (rnd()*2-1)*0.35*ww, y0: y0, vx: math.Cos(ang) * speed, vy: math.Sin(ang) * speed, life: 0.35 + rnd()*0.4, tint: rnd()})
			}
			g.list = append(g.list, b)
		}
		g.wordW = 0
	}
	out := g.list[:0]
	for _, b := range g.list {
		t := (tn - b.t0) / 1000
		alive := false
		for _, s := range b.sparks {
			if t >= s.life {
				continue
			}
			alive = true
			rem := 1 - t/s.life
			step := int(math.Min(3, math.Floor((1-rem)*4*(0.85+0.15*s.tint))))
			alpha := 1.0
			if rem <= 0.25 {
				alpha = 0.55
			}
			at := func(tt float64) (float64, float64) {
				fr := (1 - math.Exp(-3*tt)) / 3
				return s.x0 + s.vx*fr, s.y0 + s.vy*fr + 0.5*b.g*tt*tt
			}
			px, py := at(t)
			pix(p, px, py, b.cell, f32(b.cell), withAlpha(anvilPalette[step], alpha))
			if rem >= 0.45 {
				for kk := 0; kk < 2; kk++ {
					tt := t - 0.03*float64(kk+1)
					if tt < 0 {
						break
					}
					qx, qy := at(tt)
					pix(p, qx, qy, b.cell, f32(b.cell), withAlpha(anvilPalette[int(math.Min(3, float64(step+kk+1)))], alpha*[]float64{0.55, 0.3}[kk]))
				}
			}
		}
		if alive {
			out = append(out, b)
		}
	}
	g.list = out
	if len(g.list) > 0 {
		p.AnimationFrame()
	}
}

// ---- typewriter ----

type inkStamp struct {
	ch      string
	x, y    float64
	w, h    float64
	t0      float64
	size    float32
	capital bool
}

type returnStreak struct {
	x0, xs, y, h float64
	t0           float64
}

type typewriter struct {
	clock
	strike  float64
	capital bool
	stamps  []inkStamp
	wet     []inkStamp
	returns []returnStreak
	prev    ui.Rect
	have    bool
}

func newTypewriter() *typewriter { return &typewriter{strike: -1e9} }

func (g *typewriter) reset() {
	g.stamps, g.wet, g.returns, g.have, g.strike = nil, nil, nil, false, -1e9
}

func (g *typewriter) shift(dy float32) {
	for i := range g.stamps {
		g.stamps[i].y += float64(dy)
	}
	for i := range g.wet {
		g.wet[i].y += float64(dy)
	}
	for i := range g.returns {
		g.returns[i].y += float64(dy)
	}
	g.prev.Y += dy
}

func (g *typewriter) draw(p *ui.Painter, f caretFrame, k kick) {
	tn := g.at(p)
	col := cursorColor(f.dark)
	box := boxOf(f)
	text := p.Theme().Text
	letters, enter, _ := typedLetters(p, f, k)
	size := textSize(f)
	for _, l := range letters {
		if strings.TrimSpace(l.ch) == "" {
			continue
		}
		cap := len([]rune(l.ch)) == 1 && unicode.IsUpper([]rune(l.ch)[0])
		g.strike, g.capital = tn, cap
		st := inkStamp{ch: l.ch, x: l.x, y: float64(f.r.Y), w: l.w, h: float64(f.r.H), t0: tn, size: size}
		g.stamps = append(g.stamps, st)
		g.wet = append(g.wet, st)
	}
	if enter && g.have {
		g.returns = append(g.returns, returnStreak{x0: float64(g.prev.X), xs: float64(f.bounds.X), y: float64(g.prev.Y+g.prev.H/2) + float64(size)*0.3, h: float64(g.prev.H), t0: tn})
	}
	g.prev, g.have = f.r, true
	// fresh ink: the letters just typed, heavier, drying over a second and a half
	out := g.wet[:0]
	for _, s := range g.wet {
		age := tn - s.t0
		if age >= 1500 {
			continue
		}
		out = append(out, s)
		u := age / 1500
		wet := 1.0
		if u > 0.3 {
			wet = 1 - easeSine((u-0.3)/0.7)
		}
		glyphAt(p, s.ch, f32(s.x+s.w/2), f32(s.y+s.h/2), s.size, withAlpha(text, 0.55*wet), true, 0, 1)
	}
	g.wet = out
	// the ink stamp: a bigger letter lands on its cell and shrinks onto it, the real letter covered while it does
	so := g.stamps[:0]
	for _, s := range g.stamps {
		t := (tn - s.t0) / 120
		if t >= 1 {
			continue
		}
		so = append(so, s)
		land := math.Min(1, t/0.6)
		grow := 1 + 0.15*(1-land)*(1-land)
		p.Fill(ui.Rect{X: f32(s.x - 1), Y: f32(s.y), W: f32(s.w + 2), H: f32(s.h)}, f.bg, 0)
		glyphAt(p, s.ch, f32(s.x+s.w/2), f32(s.y+s.h/2), s.size, text, false, 0, grow)
	}
	g.stamps = so
	// the carriage return: a streak from the end of the old line back to its start, and a ding
	ro := g.returns[:0]
	for _, r := range g.returns {
		u := (tn - r.t0) / 250
		if u >= 1 {
			continue
		}
		ro = append(ro, r)
		if r.x0-r.xs <= 2 {
			continue
		}
		ease := func(v float64) float64 { v = clamp01(v); return 1 - (1-v)*(1-v)*(1-v) }
		head := r.x0 - (r.x0-r.xs)*ease(u/0.6)
		tail := r.x0 - (r.x0-r.xs)*ease((u-0.25)/0.75)
		a := 0.75 * (1 - easeSine(u))
		line := &ui.Path{}
		line.MoveTo(f32(tail), f32(r.y)).LineTo(f32(head), f32(r.y))
		line.Cap(ui.RoundCap)
		p.StrokePath(line, 1.5, withAlpha(col, a))
		su := math.Min(1, u/0.8)
		inner, outer := r.h*(0.12+0.1*su), r.h*(0.22+0.2*su)
		cx, cy := r.x0, r.y-r.h*0.45
		for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			tick := &ui.Path{}
			tick.MoveTo(f32(cx+d[0]*inner), f32(cy+d[1]*inner)).LineTo(f32(cx+d[0]*outer), f32(cy+d[1]*outer))
			p.StrokePath(tick, 1.25, withAlpha(col, 0.8*(1-su)))
		}
	}
	g.returns = ro
	// the box itself: struck down, squashed, pushed forward, on a spring
	u := (tn - g.strike) / 170
	dy, sy, dx := 0.0, 1.0, 0.0
	if u >= 0 && u < 1 {
		wt := 1.0
		if g.capital {
			wt = 1.5
		}
		s := math.Exp(-4*u) * math.Cos(2*math.Pi*1.25*u)
		dy = 0.18 * float64(box.H) * wt * s
		sy = 1 - 0.10*wt*s
	}
	if u2 := (tn - g.strike) / 120; u2 >= 0 && u2 < 1 {
		dx = 0.15 * float64(box.W) * math.Sin(math.Pi*u2) * (1 - u2)
	}
	t := ui.Scaling(1, f32(sy)).Then(ui.Translation(f32(dx), f32(dy)))
	p.Transform(t, box.X+box.W/2, box.Y+box.H, func() { drawBoxCursor(p, box, col, 1) })
	if len(g.stamps) > 0 || len(g.wet) > 0 || len(g.returns) > 0 || u < 1 {
		p.AnimationFrame()
	}
}

// ---- tiles ----

func letterPopMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H - 13, W: 8, H: 12}, withAlpha(col, 0.9), 2)
	glyphAt(p, "a", r.X+r.W/2+8, r.Y+6, 11, col, true, 0.5, 1)
	glyphAt(p, "b", r.X+r.W/2-8, r.Y+3, 9, withAlpha(col, 0.6), true, -0.4, 1)
}

func riseMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H - 13, W: 8, H: 12}, withAlpha(col, 0.9), 2)
	for i := 0; i < 3; i++ {
		glyphAt(p, "a", r.X+r.W/2, r.Y+r.H-18-float32(i)*6, 11, withAlpha(col, 0.9-0.3*float64(i)), false, 0, 1)
	}
}

func thunderMock(p *ui.Painter, r ui.Rect, dark bool) {
	x, y := float64(r.X+r.W/2), float64(r.Y)
	for i := 0; i < 8; i++ {
		x += math.Sin(float64(i)*2.1) * 3
		p.Fill(ui.Rect{X: f32(x), Y: f32(y), W: 3, H: 3}, mix(boltColor(float64(i)/8), ui.RGBA(255, 255, 255, 1), 0.45), 0)
		y += 3
	}
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H - 5, W: 8, H: 4}, withAlpha(cursorColor(dark), 0.9), 1)
}

func fireworkMock(p *ui.Painter, r ui.Rect, dark bool) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2-2
	for i := 0; i < 10; i++ {
		a := float64(i) / 10 * 2 * math.Pi
		p.Fill(ui.Rect{X: cx + f32(math.Cos(a)*9), Y: cy + f32(math.Sin(a)*9), W: 3, H: 3}, hsl(float64(i)*36, 0.85, 0.62), 0)
	}
}

func anvilMock(p *ui.Painter, r ui.Rect, dark bool) {
	for i := 0; i < 7; i++ {
		a := 0.3 + float64(i)*0.4
		p.Fill(ui.Rect{X: r.X + r.W/2 + f32(math.Cos(a)*float64(5+i*2)), Y: r.Y + 4 + f32(math.Sin(a)*float64(3+i*2)), W: 3, H: 3}, anvilPalette[i%4], 0)
	}
}

func typewriterMock(p *ui.Painter, r ui.Rect, dark bool) {
	col := cursorColor(dark)
	p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + 4, W: 8, H: r.H - 6}, withAlpha(col, 0.9), 2)
	line := &ui.Path{}
	line.MoveTo(r.X+4, r.Y+r.H-2).LineTo(r.X+r.W-4, r.Y+r.H-2)
	p.StrokePath(line, 1.5, withAlpha(col, 0.5))
}
