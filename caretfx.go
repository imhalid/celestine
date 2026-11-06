package main

import (
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// More carets than the slime and the mizan, in their own file: each is told what
// happened since the last frame (a kick: letters written, letters erased and
// which way, whether the text changed) and draws itself where the caret is.
//
//	Pacman        the slime's own body, in yellow, with an eye; when you erase, its side opens
//	              a little, a triangle, to the way it erases: left for Backspace, right for Delete

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
	caretFirstFX // the presets of caretPresets follow: each a caret that is always as it is, with nothing to set
)

// caretFX is a caret of the presets: told what happened each frame, it draws itself.
type caretFX interface {
	draw(p *ui.Painter, f caretFrame, k kick)
	shift(dy float32) // the page scrolled: what it holds moves with it
	reset()           // a page was opened: start clean
}

// caretPreset is a ready caret: its name, how to make one, and how it looks in the settings' tile.
type caretPreset struct {
	name string
	make func() caretFX
	mock func(p *ui.Painter, r ui.Rect, dark bool)
}

// caretPresets are the carets after the slime and the mizan.
var caretPresets []caretPreset

func init() {
	caretPresets = []caretPreset{
		{"Pacman", func() caretFX { return newPacman() }, pacmanMock},
		{"Jöle (smear)", func() caretFX { return newSmear() }, smearMock},
		{"Enerji", func() caretFX { return newEnergy(false) }, energyMock},
		{"Aurora", func() caretFX { return newEnergy(true) }, auroraMock},
		{"CRT", func() caretFX { return newCRT(false) }, crtMock},
		{"Neon", func() caretFX { return newCRT(true) }, neonMock},
		{"Back-man", func() caretFX { return newBackman() }, backmanMock},
		{"Shredder", func() caretFX { return newShredder() }, shredderMock},
		{"Portal", func() caretFX { return newPortal() }, portalMock},
		{"Buharlaşma", func() caretFX { return newEvaporate() }, evaporateMock},
		{"Harf pop", func() caretFX { return newLetterPop(false) }, letterPopMock},
		{"Yükselen harfler", func() caretFX { return newLetterPop(true) }, riseMock},
		{"Şimşek", func() caretFX { return newThunder() }, thunderMock},
		{"Havai fişek", func() caretFX { return newFireworks() }, fireworkMock},
		{"Örs kıvılcımı", func() caretFX { return newAnvil() }, anvilMock},
		{"Daktilo", func() caretFX { return newTypewriter() }, typewriterMock},
		{"Piksel izi", func() caretFX { return newPixelTrail() }, pixelTrailMock},
		{"Yıldız tozu", func() caretFX { return newStardust() }, stardustMock},
		{"Hız iblisi", func() caretFX { return newSpeedDemon() }, speedMock},
		{"Ateş başı", func() caretFX { return newHotHead() }, hotHeadMock},
		{"Meşale", func() caretFX { return newTorch() }, torchMock},
	}
}

// caretCount is how many carets there are, the slime and the mizan with the presets.
func caretCount() caretKind { return caretKind(int(caretFirstFX) + len(caretPresets)) }

// caretName is what caret k is called.
func caretName(k caretKind) string {
	switch {
	case k == caretSlime:
		return "Slime"
	case k == caretMizan:
		return "Mizan"
	case int(k)-int(caretFirstFX) < len(caretPresets):
		return caretPresets[int(k)-int(caretFirstFX)].name
	}
	return ""
}

// fxOf is the caret of preset i, made the first time it is asked for.
func (a *app) fxOf(i int) caretFX {
	for len(a.fx) < len(caretPresets) {
		a.fx = append(a.fx, caretPresets[len(a.fx)].make())
	}
	return a.fx[i]
}

// fxNamed is the caret of the preset called name, or nil.
func (a *app) fxNamed(name string) caretFX {
	for i, c := range caretPresets {
		if c.name == name {
			return a.fxOf(i)
		}
	}
	return nil
}

// eachFX is fn for every caret of the presets made so far.
func (a *app) eachFX(fn func(caretFX)) {
	for _, c := range a.fx {
		fn(c)
	}
}

// kick is what happened to the text since the caret was last drawn.
type kick struct {
	wrote  int    // letters written
	edited bool   // the text changed in another way, too
	erased int    // letters taken away
	dir    int    // -1: Backspace, took what was behind the caret; 1: Delete, what was ahead
	gone   string // what was erased, as far as it is known
	typed  string // what was written: the letters, a space, a "\n" for Enter
}

// eraseOf notes letters erased from prev, between from and to, going dir.
func (a *app) eraseOf(prev richtext.Text, from, to, dir int) {
	a.erased += to - from
	a.eraseDir = dir
	if from >= 0 && to <= prev.Len() && to > from {
		s := prev.Slice(from, to).String()
		s = strings.Map(func(r rune) rune {
			if r == '\n' || r == richtext.ObjectReplacement {
				return -1
			}
			return r
		}, s)
		a.gone = s + a.gone
		if r := []rune(a.gone); len(r) > 40 {
			a.gone = string(r[len(r)-40:])
		}
	}
}

func f32(v float64) float32 { return float32(v) }

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// dtOf is the time since last, in seconds, at most 50 ms: a frame after a pause is not a leap.
func dtOf(now, last time.Time) float64 {
	if last.IsZero() {
		return 0.016
	}
	return math.Min(0.05, now.Sub(last).Seconds())
}

// ---- particles ----

type spark struct {
	x, y, vx, vy float64
	life, max    float64 // seconds left, and in all
	size         float64
	col          ui.Color
}

type sparks struct{ list []spark }

// burst throws n sparks from (x, y).
func (s *sparks) burst(x, y float64, n int, col ui.Color, power float64) {
	for i := 0; i < n; i++ {
		ang := -math.Pi/2 + (rand.Float64()-0.5)*math.Pi*1.4
		sp := power * (0.4 + rand.Float64())
		life := 0.35 + rand.Float64()*0.45
		s.list = append(s.list, spark{x, y, math.Cos(ang) * sp, math.Sin(ang) * sp, life, life, 1.6 + rand.Float64()*1.8, col})
	}
	if len(s.list) > 160 {
		s.list = s.list[len(s.list)-160:]
	}
}

// step moves them, and drops the spent.
func (s *sparks) step(dt float64) {
	out := s.list[:0]
	for _, sp := range s.list {
		sp.life -= dt
		if sp.life <= 0 {
			continue
		}
		sp.vy += 520 * dt // they fall
		sp.x += sp.vx * dt
		sp.y += sp.vy * dt
		out = append(out, sp)
	}
	s.list = out
}

func (s *sparks) draw(p *ui.Painter) {
	for _, sp := range s.list {
		a := clamp01(sp.life / sp.max)
		p.Fill(ui.Rect{X: float32(sp.x - sp.size/2), Y: float32(sp.y - sp.size/2), W: float32(sp.size), H: float32(sp.size)}, withAlpha(sp.col, a), rad(2))
	}
}

// ---- Pacman ----

// pacman is the slime, yellow, with an eye. Erasing opens a triangle in its side - the
// side it erases toward - which closes again.
type pacman struct {
	body *slime
	open float64 // how open the mouth is, 0..1
	hold float64 // seconds it stays open
	dir  int     // -1 looks left (Backspace), 1 right
	last time.Time
}

func newPacman() *pacman { return &pacman{body: newSlime(), dir: 1} }

func (g *pacman) reset() { g.body.placed, g.open, g.hold, g.dir = false, 0, 0, 1 }

func (g *pacman) shift(dy float32) { g.body.shift(0, dy) }

func (g *pacman) draw(p *ui.Painter, f caretFrame, k kick) {
	now := p.Now()
	dt := dtOf(now, g.last)
	g.last = now
	if k.erased > 0 {
		g.dir = k.dir
		if g.dir == 0 {
			g.dir = -1
		}
		g.hold = 0.16 + 0.03*math.Min(float64(k.erased), 8)
		g.open = math.Max(g.open, 0.35) // the mouth is there at once
	}
	if g.hold > 0 {
		g.hold -= dt
		g.open += (1 - g.open) * (1 - math.Exp(-dt*34))
	} else {
		g.open = math.Max(0, g.open-dt*5)
		if g.open == 0 {
			g.dir = 1
		}
	}
	body := ui.Hex("#ffd60a")
	if !f.dark {
		body = ui.Hex("#eab308")
	}
	g.body.drawWith(p, f.r, f.cw, body, f.dark, func(b ui.Rect) { g.face(p, b) })
}

// face draws the eye, and the mouth when it is open, on the jelly's box b.
func (g *pacman) face(p *ui.Painter, b ui.Rect) {
	ink := ui.Hex("#2a1d00")
	d := float32(g.dir)
	cx := b.X + b.W/2
	// the mouth: a triangle from the middle of the body to its side, opening with the erase
	if g.open > 0.02 {
		half := b.H * 0.2 * f32(g.open)
		my := b.Y + b.H*0.62
		edge := b.X + b.W
		if d < 0 {
			edge = b.X
		}
		tri := &ui.Path{}
		tri.MoveTo(cx, my).LineTo(edge+d*1.5, my-half).LineTo(edge+d*1.5, my+half).Close()
		p.FillPath(tri, withAlpha(ink, 0.8))
	}
	// the eye
	r := float32(math.Max(1.5, math.Min(float64(b.W)*0.15, 2.6)))
	ex := cx + d*b.W*0.17
	ey := b.Y + b.H*0.3
	p.Fill(ui.Rect{X: ex - r, Y: ey - r, W: 2 * r, H: 2 * r}, ink, r)
	p.Fill(ui.Rect{X: ex - r*0.2, Y: ey - r*0.6, W: r * 0.7, H: r * 0.7}, ui.RGBA(255, 255, 255, 0.9), r*0.35)
}

// ---- tiles ----

func pacmanMock(p *ui.Painter, r ui.Rect, dark bool) {
	body, ink := ui.Hex("#ffd60a"), ui.Hex("#2a1d00")
	x, y, w, h := r.X+r.W/2-4.5, r.Y+1.5, float32(9), float32(21)
	p.Fill(ui.Rect{X: x, Y: y, W: w, H: h}, withAlpha(body, 0.9), rad(5))
	p.Fill(ui.Rect{X: x + w*0.62 - 1.6, Y: y + h*0.3 - 1.6, W: 3.2, H: 3.2}, ink, 1.6)
	tri := &ui.Path{}
	tri.MoveTo(x+w/2, y+h*0.62).LineTo(x+w+1, y+h*0.62-3.5).LineTo(x+w+1, y+h*0.62+3.5).Close()
	p.FillPath(tri, withAlpha(ink, 0.8))
}
