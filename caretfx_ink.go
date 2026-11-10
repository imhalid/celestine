package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// Two carets after the typing effects of pastel.zig (stackedlol/typing-animations, #309). That repository has no
// license: its code was read for how the effects are made, and what is here is written anew. The corner brackets
// follow the look of Mustafa Saraç's ff-tracking (MIT).
//
//	Ink      an ASCII field over the page: what happens leaves ink in a grid of cells, the ink fades, and each
//	         cell shows a letter of a ramp from a stray dot to a solid @, dithered so the steps are smooth. A
//	         letter sends a ripple and a few fish that swim off leaving a wake, more when typing fast; each word
//	         takes the next pastel. Erased letters dissolve into dense rose ink, fish sink out of them and a box
//	         collapses on the gap. A jump of the caret draws a streak along the way and a splash where it lands.
//	         The brackets of Lock-on come with it.
//	Lock-on  corner brackets: a finished word and the place a caret jumps to are locked on, the brackets
//	         snapping from wide to tight in red over a hatch, then settling in the ink and letting go. The newest
//	         wears a tag, X and Y in a pixel font; an erase collapses a hatched box on the gap.

// inkRamp is the field's letters, from the least ink to the most.
const inkRamp = ".,:;t%SX8#@"

// inkBayer is a 4×4 ordered dither: where a cell's ink falls between two letters of the ramp, its place in the
// matrix says which it shows, so the field steps evenly instead of in bands.
var inkBayer = [16]float64{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

// inkHues are the four pastels each word takes in turn (mint, sky, rose, lavender), strong enough for a letter
// of the field: deeper on a light page, softer on a dark one. Rose is the erase's.
var inkHues = [2][4]ui.Color{
	{ui.Hex("#4fb57a"), ui.Hex("#5a93d8"), ui.Hex("#e46b68"), ui.Hex("#8f73d9")},
	{ui.Hex("#a3dbb2"), ui.Hex("#a1c5ea"), ui.Hex("#eea0a0"), ui.Hex("#c2afe6")},
}

const inkRose = 2

func inkHue(dark bool, h int) ui.Color {
	d := 0
	if dark {
		d = 1
	}
	return inkHues[d][h&3]
}

// inkCell is a cell of the field: how much ink, 0..1, and in which hue.
type inkCell struct {
	v   float64
	hue int
}

// inkRipple is a ring that spreads from a letter written, flattened as a ripple seen across a page.
type inkRipple struct {
	x, y, t, power float64
	hue            int
}

// inkFish swims off on a curve, slowing, and leaves a wake of ink.
type inkFish struct {
	x, y, vx, vy, turn, t, life float64
	hue                         int
}

// inkStreak is ink drawn along a jump of the caret, from where it was to where it went.
type inkStreak struct {
	ax, ay, bx, by, t float64
	hue               int
}

// Lock-on's kinds of lock.
const (
	lockWord = iota // a word just finished
	lockJump        // where the caret landed
	lockGone        // what was erased: the box collapses on it
)

// inkLock is a pair of corner brackets on a box of the page.
type inkLock struct {
	r    ui.Rect
	t    float64 // s, since it locked on
	kind int
	tag  string // X and Y, on the newest only
}

// inkCaret is the Ink caret (field on) and Lock-on (field off).
type inkCaret struct {
	fxBase
	field bool

	cw, ch, gs float64 // a cell of the field, and the size of its letters, px; 0 until the first frame
	oy         float64 // how far the page has scrolled since the grid was laid: the ink stays with the text
	cells      map[[2]int]inkCell
	ripples    []inkRipple
	fish       []inkFish
	streaks    []inkStreak
	locks      []inkLock
	ramp       [len(inkRamp)]*ui.TextLayout // the ramp's letters, shaped once at gs
	rampAt     float64

	hue      int     // the pastel of the word being written
	combo    int     // letters written in a quick run
	inWord   bool    // the last letter written was of a word
	lastType float64 // ms, the last letter
}

func newInkCaret(field bool) *inkCaret { return &inkCaret{field: field} }

func (g *inkCaret) reset() {
	g.have = false
	g.cells, g.ripples, g.fish, g.streaks, g.locks = nil, nil, nil, nil, nil
	g.cw, g.oy = 0, 0
}

func (g *inkCaret) shift(dy float32) {
	d := float64(dy)
	g.oy += d
	for i := range g.ripples {
		g.ripples[i].y += d
	}
	for i := range g.fish {
		g.fish[i].y += d
	}
	for i := range g.streaks {
		g.streaks[i].ay += d
		g.streaks[i].by += d
	}
	for i := range g.locks {
		g.locks[i].r.Y += dy
	}
	g.prev.Y += dy
}

// alive reports whether anything is still moving or fading.
func (g *inkCaret) alive() bool {
	return len(g.cells) > 0 || len(g.ripples) > 0 || len(g.fish) > 0 || len(g.streaks) > 0 || len(g.locks) > 0
}

func (g *inkCaret) draw(p *ui.Painter, f caretFrame, k kick) {
	was, had := g.prev, g.have
	tn, dt, _, _ := g.step(p, f, k)
	dt = math.Min(dt, 0.05)
	h := float64(f.r.H)
	u := h / 34 // the effects are measured for a caret 34 px tall, and scaled to this one
	if g.cw == 0 || len(g.cells) == 0 {
		g.layGrid(float64(textSize(f)))
	}
	mid := float64(f.r.Y) + h*0.55
	x := float64(f.r.X)

	// what happened
	switch {
	case k.erased > 0 && k.gone != "":
		g.dissolve(p, f, k, u)
	case k.wrote > 0:
		g.wrote(f, k, tn, x, mid, u)
	case had && !k.edited:
		dx, dy := float64(f.r.X-was.X), float64(f.r.Y-was.Y)
		if math.Abs(dy) > h*1.5 || math.Abs(dx) > 130*u {
			g.jump(f, float64(was.X), float64(was.Y)+float64(was.H)/2, x, float64(f.r.Y)+h/2, u)
		}
	}

	g.stepLocks(dt)
	if g.field {
		g.stepField(dt, u)
		g.drawField(p, f)
	}
	for _, l := range g.locks {
		g.drawLock(p, f, l)
	}
	col := p.Theme().Accent
	if g.field {
		col = inkHue(f.dark, g.hue)
	}
	styledDraw(p, f, boxOf(f), func() { p.Fill(ui.Rect{X: f.r.X - 1, Y: f.r.Y, W: 2, H: f.r.H}, col, 1) })
	if g.alive() {
		p.AnimationFrame()
	}
}

// layGrid sizes the field's cells to the letters of the caret's line, and shapes the ramp's letters for them.
func (g *inkCaret) layGrid(size float64) {
	g.gs = math.Max(7, size*0.58)
	g.cw, g.ch = math.Round(g.gs*0.64), math.Round(g.gs*1.1)
	g.cells = map[[2]int]inkCell{}
	if g.rampAt != g.gs {
		for i, r := range inkRamp {
			g.ramp[i] = ui.ShapeText(string(r), ui.Font{Size: f32(g.gs), Weight: 700}, 200)
		}
		g.rampAt = g.gs
	}
}

// wrote is a letter, a space or a line written: a ripple and fish where it is, a lock on the word it finished.
func (g *inkCaret) wrote(f caretFrame, k kick, tn, x, mid, u float64) {
	runes := []rune(k.typed)
	last := ' '
	if len(runes) > 0 {
		last = runes[len(runes)-1]
	}
	if k.wrote > 3 { // pasted: one wide splash
		g.splash(x, mid, 1.3*u, 6)
		g.inWord = false
		return
	}
	if tn-g.lastType < 260 {
		g.combo++
	} else {
		g.combo = 1
	}
	g.lastType = tn
	inWord := isWordRune(last)
	if !inWord && g.inWord && f.words != nil { // the word before it is finished: only once, at the first mark after it
		g.lockWord(f, f.words.last(), u)
	}
	switch {
	case last == '\n':
		g.hue = (g.hue + 1) & 3
	case unicode.IsSpace(last):
		g.hue = (g.hue + 1) & 3
		g.splash(x, mid, 0.4*u, 0)
	default:
		c := float64(min(g.combo, 16))
		g.splash(x-float64(textSize(f))*0.25, mid, (0.65+c*0.035)*u, 1+min(g.combo, 16)/6)
	}
	g.inWord = inWord
}

// splash is ink where something landed: a blot, a ripple, and n fish. power is in px, scaled to the caret.
func (g *inkCaret) splash(x, y, power float64, n int) {
	if !g.field {
		return
	}
	g.blob(x, y, 13*power, 0.95, g.hue)
	if len(g.ripples) < 32 {
		g.ripples = append(g.ripples, inkRipple{x: x, y: y, power: power, hue: g.hue})
	}
	for i := 0; i < n && len(g.fish) < 160; i++ {
		a := rand.Float64() * 2 * math.Pi
		s := (110 + rand.Float64()*150) * power
		hue := g.hue
		if rand.Float64() > 0.6 {
			hue = rand.Intn(4)
		}
		g.fish = append(g.fish, inkFish{x: x, y: y, vx: math.Cos(a) * s, vy: math.Sin(a) * s * 0.6, turn: (rand.Float64() - 0.5) * 10,
			life: 0.35 + rand.Float64()*0.35, hue: hue})
	}
}

// jump is the caret gone far: a streak along the way, a splash where it landed, and a lock on it.
func (g *inkCaret) jump(f caretFrame, ax, ay, bx, by, u float64) {
	if g.field && len(g.streaks) < 8 {
		g.streaks = append(g.streaks, inkStreak{ax: ax, ay: ay, bx: bx, by: by, hue: g.hue})
	}
	g.splash(bx, by, u, 4)
	pad := f32(11 * u)
	r := ui.Rect{X: f.r.X - pad, Y: f.r.Y - pad/2, W: 2 * pad, H: f.r.H + pad}
	g.lockOn(r, lockJump, inkTag(f, f.r.X, f.r.Y))
}

// lockWord locks on word, the one just finished before the caret: of its places in view, the one on the caret's
// line that ends nearest before it.
func (g *inkCaret) lockWord(f caretFrame, word string, u float64) {
	if word == "" {
		return
	}
	rects, _ := f.words.find(word)
	best, found := ui.Rect{}, false
	for _, rc := range rects {
		if rc.Y > f.r.Y+f.r.H || rc.Y+rc.H < f.r.Y || rc.X+rc.W > f.r.X+2 {
			continue
		}
		if !found || rc.X > best.X {
			best, found = rc, true
		}
	}
	if !found {
		return
	}
	pad := f32(4 * u)
	g.lockOn(ui.Rect{X: best.X - pad, Y: best.Y - pad/2, W: best.W + 2*pad, H: best.H + pad}, lockWord, inkTag(f, best.X, best.Y))
}

// inkTag is a lock's tag: where it is in the editor's box, X and Y in px.
func inkTag(f caretFrame, x, y float32) string {
	return fmt.Sprintf("X:%d Y:%d", int(max(0, x-f.bounds.X)), int(max(0, y-f.bounds.Y)))
}

// lockOn adds a lock. Only the newest wears a tag: the ones before it drop theirs and let go sooner.
func (g *inkCaret) lockOn(r ui.Rect, kind int, tag string) {
	if kind != lockGone {
		for i := range g.locks {
			if g.locks[i].kind != lockGone {
				g.locks[i].tag = ""
				if g.locks[i].t >= 0.7 {
					g.locks[i].t = math.Max(g.locks[i].t, 1.5)
				}
			}
		}
	}
	if len(g.locks) >= 24 {
		g.locks = append(g.locks[:0], g.locks[1:]...)
	}
	if kind == lockGone {
		tag = ""
	}
	g.locks = append(g.locks, inkLock{r: r, kind: kind, tag: tag})
}

// dissolve is letters erased: their places fill with dense rose ink that fades down the ramp, a fish sinks out
// of each, and a box collapses on the gap.
func (g *inkCaret) dissolve(p *ui.Painter, f caretFrame, k kick, u float64) {
	gone := goneLetters(p, f, k)
	if len(gone) == 0 {
		return
	}
	right := float64(f.bounds.X + f.bounds.W)
	mid := float64(f.r.Y) + float64(f.r.H)*0.55
	x0, x1 := math.Inf(1), math.Inf(-1)
	for i, l := range gone {
		if l.x > right {
			continue // wrapped past the line: not where it stood
		}
		x0, x1 = math.Min(x0, l.x), math.Max(x1, l.x+l.w)
		if !g.field {
			continue
		}
		cx := l.x + l.w/2
		g.blob(cx, mid, 9*u, 1, inkRose)
		if i < 40 && len(g.fish) < 160 {
			hue := inkRose
			if rand.Float64() > 0.7 {
				hue = 3
			}
			g.fish = append(g.fish, inkFish{x: cx, y: mid, vx: (rand.Float64() - 0.5) * 160 * u, vy: (30 + rand.Float64()*100) * u,
				turn: (rand.Float64() - 0.5) * 8, life: 0.3 + rand.Float64()*0.3, hue: hue})
		}
	}
	if x1 <= x0 {
		return
	}
	pad := 4 * u
	g.lockOn(ui.Rect{X: f32(x0 - pad), Y: f.r.Y, W: f32(x1 - x0 + 2*pad), H: f.r.H}, lockGone, "")
}

// ---- the field ----

// cellOf is the cell of the field at (x, y).
func (g *inkCaret) cellOf(x, y float64) (int, int) {
	return int(math.Floor(x / g.cw)), int(math.Floor((y - g.oy) / g.ch))
}

// centre is the middle of cell (cx, cy).
func (g *inkCaret) centre(cx, cy int) (float64, float64) {
	return (float64(cx) + 0.5) * g.cw, (float64(cy)+0.5)*g.ch + g.oy
}

// deposit puts ink v in a cell, unless it already has more.
func (g *inkCaret) deposit(cx, cy int, v float64, hue int) {
	if v <= 0.02 {
		return
	}
	key := [2]int{cx, cy}
	c, ok := g.cells[key]
	if !ok && len(g.cells) >= 1600 {
		return
	}
	if v > c.v {
		g.cells[key] = inkCell{v: math.Min(v, 1), hue: hue}
	}
}

// blob is a soft round blot of radius r. Its own cell is inked full, so a blot smaller than a cell, between the
// cells' middles, still leaves one.
func (g *inkCaret) blob(x, y, r, v float64, hue int) {
	cx, cy := g.cellOf(x, y)
	g.deposit(cx, cy, v, hue)
	ax, ay := g.cellOf(x-r, y-r)
	bx, by := g.cellOf(x+r, y+r)
	for cy := ay; cy <= by; cy++ {
		for cx := ax; cx <= bx; cx++ {
			px, py := g.centre(cx, cy)
			if k := 1 - math.Hypot(px-x, py-y)/r; k > 0 {
				g.deposit(cx, cy, v*math.Sqrt(k), hue)
			}
		}
	}
}

// ring is a ring of radius r and width w, flattened to a little over half its height.
func (g *inkCaret) ring(x, y, r, w, v float64, hue int) {
	const flat = 0.55
	ax, ay := g.cellOf(x-r-w, y-r*flat-w)
	bx, by := g.cellOf(x+r+w, y+r*flat+w)
	for cy := ay; cy <= by; cy++ {
		for cx := ax; cx <= bx; cx++ {
			px, py := g.centre(cx, cy)
			if k := 1 - math.Abs(math.Hypot(px-x, (py-y)/flat)-r)/w; k > 0 {
				g.deposit(cx, cy, v*k, hue)
			}
		}
	}
}

// line is ink along a segment, a cell every few px.
func (g *inkCaret) line(ax, ay, bx, by, v float64, hue int) {
	n := int(math.Max(1, math.Hypot(bx-ax, by-ay)/(g.cw*0.6)))
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		cx, cy := g.cellOf(ax+(bx-ax)*t, ay+(by-ay)*t)
		g.deposit(cx, cy, v, hue)
	}
}

// stepField fades the ink and lets the ripples, the fish and the streaks write into it.
func (g *inkCaret) stepField(dt, u float64) {
	fade := math.Exp(-dt * 3.4)
	for key, c := range g.cells {
		c.v *= fade
		if c.v < 0.025 {
			delete(g.cells, key)
			continue
		}
		g.cells[key] = c
	}
	const rippleLife = 0.42
	out := g.ripples[:0]
	for _, r := range g.ripples {
		r.t += dt
		if r.t > rippleLife {
			continue
		}
		k := r.t / rippleLife
		g.ring(r.x, r.y, 5*u+(1-(1-k)*(1-k))*60*r.power, 8*u, 0.85*(1-k), r.hue)
		out = append(out, r)
	}
	g.ripples = out
	fish := g.fish[:0]
	for _, s := range g.fish {
		s.t += dt
		if s.t > s.life {
			continue
		}
		ox, oy := s.x, s.y
		sin, cos := math.Sincos(s.turn * dt)
		s.vx, s.vy = s.vx*cos-s.vy*sin, s.vx*sin+s.vy*cos
		drag := math.Exp(-2.2 * dt)
		s.vx *= drag
		s.vy *= drag
		s.x += s.vx * dt
		s.y += s.vy * dt
		g.line(ox, oy, s.x, s.y, 0.9*(1-s.t/s.life), s.hue)
		fish = append(fish, s)
	}
	g.fish = fish
	const streakLife = 0.22
	streaks := g.streaks[:0]
	for _, s := range g.streaks {
		k0 := clamp01(s.t / streakLife)
		s.t += dt
		k1 := clamp01(s.t / streakLife)
		g.line(s.ax+(s.bx-s.ax)*k0, s.ay+(s.by-s.ay)*k0, s.ax+(s.bx-s.ax)*k1, s.ay+(s.by-s.ay)*k1, 1, s.hue)
		if s.t <= streakLife+0.02 {
			streaks = append(streaks, s)
		}
	}
	g.streaks = streaks
}

// drawField draws each cell with ink as a letter of the ramp, dithered, in its hue: faint where the ink is thin.
func (g *inkCaret) drawField(p *ui.Painter, f caretFrame) {
	n := float64(len(inkRamp))
	for key, c := range g.cells {
		q := c.v * n
		i := int(q)
		if q-float64(i) > (inkBayer[(key[1]&3)*4+(key[0]&3)]+0.5)/16 {
			i++
		}
		if i == 0 {
			continue
		}
		l := g.ramp[min(i, len(inkRamp))-1]
		x, y := float64(key[0])*g.cw, float64(key[1])*g.ch+g.oy
		w, lh := l.Size()
		p.TextLayout(l, f32(x+(g.cw-float64(w))/2), f32(y+(g.ch-float64(lh))/2), withAlpha(inkHue(f.dark, c.hue), 0.3+0.45*c.v))
	}
}

// ---- the brackets ----

// stepLocks ages the locks and lets go of the spent.
func (g *inkCaret) stepLocks(dt float64) {
	out := g.locks[:0]
	for _, l := range g.locks {
		l.t += dt
		if (l.kind == lockGone && l.t < 0.7) || (l.kind != lockGone && l.t < 2) {
			out = append(out, l)
		}
	}
	g.locks = out
}

// lockColors are the brackets' red while they lock on, and their ink once settled.
func lockColors(dark bool) (locking, settled ui.Color) {
	if dark {
		return ui.Hex("#ff5c50"), ui.Hex("#ecebe6")
	}
	return ui.Hex("#ff4034"), ui.Hex("#1c1c1e")
}

// drawLock draws a lock: snapping in from wide to tight, red and hatched, then settled in the ink and fading; or,
// on what was erased, a hatched box collapsing.
func (g *inkCaret) drawLock(p *ui.Painter, f caretFrame, l inkLock) {
	k := clamp01(l.t / 0.7)
	locking := k < 1
	red, ink := lockColors(f.dark)
	col, alpha, hatch := red, 1.0, locking
	s := 1.0
	if l.kind == lockGone {
		s, alpha, hatch = 1-0.85*k*k, 1-k, true
	} else if locking {
		s = 1 + 0.8*(1-k)*(1-k)
	} else {
		col, alpha = ink, 0.6*(1-clamp01((l.t-1.5)/0.5))
	}
	if alpha <= 0.01 {
		return
	}
	cx, cy := l.r.X+l.r.W/2, l.r.Y+l.r.H/2
	hw, hh := l.r.W/2*f32(s), l.r.H/2*f32(s)
	r := ui.Rect{X: cx - hw, Y: cy - hh, W: 2 * hw, H: 2 * hh}
	brackets(p, r, col, alpha, hatch)
	if l.tag != "" {
		pixelTag(p, r.X, r.Y-2, l.tag, col, f.bg, alpha)
	}
}

// brackets draws the four corners of r, their arms a little over a quarter of its shorter side, and a diagonal
// hatch inside it.
func brackets(p *ui.Painter, r ui.Rect, col ui.Color, alpha float64, hatch bool) {
	r = ui.Rect{X: f32(math.Round(float64(r.X))), Y: f32(math.Round(float64(r.Y))), W: f32(math.Round(float64(r.W))), H: f32(math.Round(float64(r.H)))}
	arm := max(4, min(r.W, r.H, 40)*0.28)
	const k = 1
	l, t, rt, b := r.X, r.Y, r.X+r.W, r.Y+r.H
	for _, bar := range []ui.Rect{
		{X: l, Y: t, W: arm, H: k}, {X: l, Y: t, W: k, H: arm},
		{X: rt - arm, Y: t, W: arm, H: k}, {X: rt - k, Y: t, W: k, H: arm},
		{X: l, Y: b - k, W: arm, H: k}, {X: l, Y: b - arm, W: k, H: arm},
		{X: rt - arm, Y: b - k, W: arm, H: k}, {X: rt - k, Y: b - arm, W: k, H: arm},
	} {
		p.Fill(bar, withAlpha(col, alpha), 0)
	}
	if !hatch || r.W < 2 || r.H < 2 {
		return
	}
	p.Clip(r, 0, func() {
		path := &ui.Path{}
		for x := l - r.H; x < rt; x += 4 {
			path.MoveTo(x, b).LineTo(x+r.H, t)
		}
		p.StrokePath(path, 1, withAlpha(col, 0.12*alpha))
	})
}

// pixelFont is a 3×5 pixel font for the tags: each letter five rows of three, # a pixel.
var pixelFont = map[rune]string{
	'0': "### #.# #.# #.# ###", '1': ".#. ##. .#. .#. ###", '2': "### ..# ### #.. ###", '3': "### ..# .## ..# ###",
	'4': "#.# #.# ### ..# ..#", '5': "### #.. ### ..# ###", '6': "### #.. ### #.# ###", '7': "### ..# .#. .#. .#.",
	'8': "### #.# ### #.# ###", '9': "### #.# ### ..# ###", 'X': "#.# #.# .#. #.# #.#", 'Y': "#.# #.# .#. .#. .#.",
	':': "... .#. ... .#. ...",
}

// pixelTag draws s in the pixel font knocked out of a filled pill of col, its lower left corner at (x, y).
func pixelTag(p *ui.Painter, x, y float32, s string, col, bg ui.Color, alpha float64) {
	const dot, pad = 1.5, 3
	n := 0
	for _, r := range s {
		if _, ok := pixelFont[r]; ok || r == ' ' {
			n++
		}
	}
	w := float32(n)*4*dot - dot + 2*pad
	h := float32(5*dot + 2*pad)
	left, top := f32(math.Round(float64(x))), f32(math.Round(float64(y-h)))
	p.Fill(ui.Rect{X: left, Y: top, W: w, H: h}, withAlpha(col, alpha), 1)
	knock := withAlpha(bg, alpha)
	i := 0
	for _, r := range s {
		rows, ok := pixelFont[r]
		if !ok && r != ' ' {
			continue
		}
		for row, bits := range strings.Fields(rows) {
			for c, b := range bits {
				if b == '#' {
					p.Fill(ui.Rect{X: left + pad + (float32(i*4)+float32(c))*dot, Y: top + pad + float32(row)*dot, W: dot, H: dot}, knock, 0)
				}
			}
		}
		i++
	}
}
