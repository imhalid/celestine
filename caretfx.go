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
//	Mario         the box red above and blue below, with an eye; at a space it hops, a coin springs over it
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

// caretPreset is a ready caret: its name, and how to make one.
type caretPreset struct {
	name string
	make func() caretFX
}

// caretPresets are the carets after the slime and the mizan.
var caretPresets []caretPreset

func init() {
	caretPresets = []caretPreset{
		{"CRT", func() caretFX { return newCRT() }},
		{"Back-man", func() caretFX { return newBackman() }},
		{"Portal", func() caretFX { return newPortal() }},
		{"Hız iblisi", func() caretFX { return newSpeedDemon() }},
		{"Meşale", func() caretFX { return newTorch() }},
		{"Mario", func() caretFX { return newMario() }},
		// games and creatures (caretfx_game.go)
		{"Tetris", func() caretFX { return newTetris() }},
		{"Yılan", func() caretFX { return newSnake() }},
		{"Kedi", func() caretFX { return newCat() }},
		{"Tamagotchi", func() caretFX { return newPet() }},
		// nature (caretfx_nature.go)
		{"Bitki", func() caretFX { return newPlant() }},
		{"Ateş böceği", func() caretFX { return newFireflies() }},
		{"Yağmur damlası", func() caretFX { return newRaindrop() }},
		{"Mevsim", func() caretFX { return newSeason() }},
		// ink and paper (caretfx_ink.go)
		{"Dolma kalem", func() caretFX { return newPen() }},
		{"Daktilo", func() caretFX { return newTypewriter() }},
		{"Mühür", func() caretFX { return newStamp() }},
		// motion (caretfx_motion.go)
		{"Yay", func() caretFX { return newSpring() }},
		{"Kuyruklu yıldız", func() caretFX { return newComet() }},
		{"Mıknatıs", func() caretFX { return newMagnet() }},
		// the writing itself (caretfx_mood.go)
		{"Duygu yüzü", func() caretFX { return newMoodFace() }},
		{"Akış", func() caretFX { return newFlow() }},
		{"Noktalama", func() caretFX { return newPunct() }},
		// cute (caretfx_cute.go)
		{"Kawaii", func() caretFX { return newKawaii() }},
		{"UwU", func() caretFX { return newUwu() }},
		// the words (caretfx_repeat.go)
		{"Tekrar", func() caretFX { return newRepeats() }},
		// moving the letters (caretfx_letters.go)
		{"Dalga", func() caretFX { return newWave() }},
		{"Rüzgâr", func() caretFX { return newWind() }},
		{"Yerçekimi", func() caretFX { return newGravity() }},
		{"Domino", func() caretFX { return newDomino() }},
		{"Jöle satır", func() caretFX { return newJellyLine() }},
		{"Kar küresi", func() caretFX { return newSnowGlobe() }},
		// light (caretfx_light.go)
		{"Spot ışığı", func() caretFX { return newSpotlight() }},
		{"Neon tabela", func() caretFX { return newNeon() }},
		{"Kuzey ışıkları", func() caretFX { return newAurora() }},
		{"Gökkuşağı izi", func() caretFX { return newRainbowTrail() }},
		// more games (caretfx_arcade.go)
		{"Hayalet", func() caretFX { return newGhost() }},
		{"Space Invaders", func() caretFX { return newInvaders() }},
		{"Balık", func() caretFX { return newFish() }},
		{"Arı", func() caretFX { return newBee() }},
		{"Robot", func() caretFX { return newRobot() }},
		// rhythm and time (caretfx_rhythm.go)
		{"Ekolayzır", func() caretFX { return newEqualizer() }},
		{"Metronom", func() caretFX { return newMetronome() }},
		{"Davul", func() caretFX { return newDrum() }},
		{"Kum saati", func() caretFX { return newHourglass() }},
		{"Kelime sayacı", func() caretFX { return newWordBadge() }},
		{"Yazım ısısı", func() caretFX { return newWritingHeat() }},
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
	wrote  int        // letters written
	edited bool       // the text changed in another way, too
	erased int        // letters taken away
	dir    int        // -1: Backspace, took what was behind the caret; 1: Delete, what was ahead
	gone   string     // what was erased, as far as it is known
	typed  string     // what was written: the letters, a space, a "\n" for Enter
	size   float32    // the size of the letters on the caret's line, when it is known
	style  caretStyle // the text under the caret: struck through, in a bracket
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

// legacyCarets are the carets as they were numbered before some were taken away, the
// number saved in the settings then: 0 and 1 the slime and the mizan, the rest from 2.
var legacyCarets = []string{"Slime", "Mizan", "Pacman", "Jöle (smear)", "Enerji", "Aurora", "CRT", "Neon", "Back-man",
	"Shredder", "Portal", "Buharlaşma", "Harf pop", "Yükselen harfler", "Şimşek", "Havai fişek", "Örs kıvılcımı",
	"Daktilo", "Piksel izi", "Yıldız tozu", "Hız iblisi", "Ateş başı", "Meşale"}

// savedCaret is the caret the settings keep: by its name, or, in settings of before the
// name was kept, by its old number. A caret that is no more is the slime.
func (a *app) savedCaret() caretKind {
	name := a.settings.CaretName
	if name == "" && a.settings.Caret > 0 && a.settings.Caret < len(legacyCarets) {
		name = legacyCarets[a.settings.Caret]
	}
	for k := caretKind(0); k < caretCount(); k++ {
		if name == "" {
			return caretKind(a.settings.Caret % int(caretCount()))
		}
		if caretName(k) == name {
			return k
		}
	}
	return caretSlime
}
