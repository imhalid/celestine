package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// A caret in Doom's spirit, drawn here, not taken from the game (#284):
//
//	Marine  a pixel marine's face in a gray panel, as a status bar shows it: short brown hair, a heavy brow, a
//	        square jaw. Its eyes look the way the writing goes; typing fast it grins, a long fast run turns its
//	        eyes gold; an erase makes it flinch in a red flash and the letters erased burst into pixels; at a
//	        space a muzzle flash and a little shake; left alone it slowly bleeds, which writing heals

// marineFace is the face as cells, row by row, 13 wide, a long head: r R q the hair, dark to light; K k S L W the
// skin, darkest to lightest; E the whites of the eyes; m the shut mouth. The pupils, the brows' moods, the mouth
// and the blood are drawn over it, as the marine feels.
var marineFace = [...]string{
	"..rrrrrrrrr..",
	".rRRRqqRRRRr.",
	"rRRqRRRRRqRRr",
	"rRRRRRRRRRRRr",
	"rkSLLWWWLLSkr",
	"kSLLLWWWLLLSk",
	"kKrrKSLSKrrKk",
	"kSEEEkLkEEESk",
	"kSKKkSLSkKKSk",
	"kSLLSkWkSLLSk",
	"kSLLSkLkSLLSk",
	"kkSLKkkkKLSkk",
	"kkSSSSSSSSSkk",
	"kkSSmmmmmSSkk",
	"kKSSSkkkSSSKk",
	".KkSSSSSSSkK.",
	".KKkkSSSkkKK.",
	"..KKKkkkKKK..",
}

const (
	marineCols   = 13
	marineEyeRow = 7
	marineMouth  = 13
)

// marineColors are the face's cells' colors, and the panel's.
var marineColors = map[byte]ui.Color{
	'r': ui.Hex("#2e1c0e"), 'R': ui.Hex("#4e3016"), 'q': ui.Hex("#704a26"),
	'K': ui.Hex("#542c16"), 'k': ui.Hex("#7c4422"), 'S': ui.Hex("#aa6838"), 'L': ui.Hex("#ce8c54"), 'W': ui.Hex("#e8b47c"),
	'E': ui.Hex("#ece6d6"), 'M': ui.Hex("#46160e"), 'm': ui.Hex("#5e2814"),
	'B': ui.Hex("#343438"), 'b': ui.Hex("#56565c"),
}

type marine struct {
	fxBase
	rate  float64 // letters a second, eased
	fast  float64 // ms, since when it has been typing fast: gold eyes after a while of it
	ouch  float64 // ms, when an erase last hurt it
	shot  float64 // ms, the last space: the flash and the shake
	blink float64 // ms, when it blinks next
	gibs  motes
	drawn ui.Rect // the panel, where it was last drawn
}

func newMarine() *marine { return &marine{ouch: -1e9, shot: -1e9, fast: -1} }

func (g *marine) reset() { g.gibs, g.have, g.rate, g.fast = nil, false, 0, -1 }

func (g *marine) shift(dy float32) { g.gibs.shift(dy); g.prev.Y += dy }

func (g *marine) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, dt, _, _ := g.step(p, f, k)
	box := boxOf(f)
	g.rate += (float64(k.wrote)/dt - g.rate) * math.Min(1, dt*4)
	switch {
	case g.rate > 6 && g.fast < 0:
		g.fast = tn
	case g.rate < 3:
		g.fast = -1
	}
	if k.erased > 0 {
		g.ouch = tn
		cell := float64(box.H) * 1.1 / float64(len(marineFace))
		for _, l := range goneLetters(p, f, k) { // the letters burst into pixels
			for i := 0; i < 6; i++ {
				col := ui.Hex("#b3121a")
				if i%3 == 0 {
					col = faceInk(f.dark, ui.Hex("#1f2937"))
				}
				a := -math.Pi/2 + (rnd()-0.5)*2.4
				v := 40 + rnd()*90
				g.gibs = append(g.gibs, mote{x: l.x + rnd()*l.w, y: float64(box.Y+box.H/2) + (rnd()-0.5)*float64(box.H)*0.5,
					vx: math.Cos(a) * v, vy: math.Sin(a) * v, grav: 420, t0: tn, life: 520 + rnd()*300, size: cell * (0.8 + rnd()*0.6), col: col})
			}
		}
		if len(g.gibs) > 240 {
			g.gibs = g.gibs[len(g.gibs)-240:]
		}
	}
	if k.typed == " " {
		g.shot = tn
	}
	if tn > g.blink+120 { // the blink is over: the next, a few seconds on
		g.blink = tn + 2600 + rnd()*3000
	}
	ouch := clamp01(1 - (tn-g.ouch)/450) // the flinch, fading
	shot := clamp01(1 - (tn-g.shot)/140)
	god := g.fast >= 0 && tn-g.fast > 1500
	grin := g.rate > 4 && ouch == 0
	blinking := tn > g.blink && tn-g.blink < 120 && ouch == 0
	hurt := 0 // wounds, taken while it is left alone
	switch idle := g.idle(tn); {
	case idle > 90:
		hurt = 3
	case idle > 45:
		hurt = 2
	case idle > 20:
		hurt = 1
	}

	// the panel and the face in it, a little taller than the line, after the caret: its left edge, a cell over it,
	// is where the caret is, and the letter before it shows
	rows := float32(len(marineFace) + 2)
	cell := float32(math.Max(1, float64(box.H)*1.25/float64(rows)))
	fw, fh := cell*(marineCols+2), cell*rows
	x0 := box.X - cell
	if b := f.bounds; b.W > fw && x0+fw > b.X+b.W { // at the field's right edge it stays in, over the last letters
		x0 = b.X + b.W - fw
	}
	y0 := box.Y + box.H/2 - fh/2
	if shot > 0 { // the shake of the shot
		x0 += f32(math.Sin(tn*0.9) * 1.4 * shot)
		y0 += f32(math.Cos(tn*1.3) * 0.8 * shot)
	}
	g.drawn = ui.Rect{X: x0, Y: y0, W: fw, H: fh}
	p.Fill(g.drawn, marineColors['B'], cell*0.6)
	p.Stroke(ui.Rect{X: x0, Y: y0, W: fw, H: fh}, marineColors['b'], cell*0.6, max(1, cell*0.5))
	fx0, fy0 := x0+cell, y0+cell // the face in the panel
	if ouch > 0 {                // the flinch: it jerks back, away from the way it looked
		fx0 -= g.gaze() * cell * 0.7 * f32(ouch)
	}
	at := func(c, r float32) ui.Rect { return ui.Rect{X: fx0 + c*cell, Y: fy0 + r*cell, W: cell, H: cell} }
	cellAt := func(c, r float32, w float32, col ui.Color) {
		rc := at(c, r)
		rc.W = w * cell
		p.Fill(rc, col, 0)
	}
	p.Clip(ui.Rect{X: x0 + cell*0.5, Y: y0 + cell*0.5, W: fw - cell, H: fh - cell}, cell*0.4, func() {
		for r, row := range marineFace {
			for c := 0; c < len(row); c++ {
				if col, ok := marineColors[row[c]]; ok {
					p.Fill(at(float32(c), float32(r)), col, 0)
				}
			}
		}
		dark := marineColors['K']
		// the eyes: the pupils the way it looks; shut while it blinks, squeezed in a flinch, gold in god mode
		look := g.gaze()
		for _, ex := range []float32{2, 8} {
			switch {
			case blinking:
				cellAt(ex, marineEyeRow, 3, marineColors['k'])
			case ouch > 0.3:
				cellAt(ex, marineEyeRow, 3, dark)
				cellAt(ex, marineEyeRow-0.5, 3, marineColors['r']) // the brows pulled down
			default:
				pupil := ui.Hex("#34425f")
				if god {
					pupil = ui.Hex("#ffd23f")
					glowAround(p, at(ex+1+look, marineEyeRow), pupil, 0.5)
				}
				p.Fill(at(ex+1+look*0.9, marineEyeRow), pupil, 0)
			}
		}
		if grin || god { // the brows down and in: the grin is a mean one
			cellAt(4, marineEyeRow-1, 1, marineColors['r'])
			cellAt(8, marineEyeRow-1, 1, marineColors['r'])
		}
		// the mouth: shut; a grin of two rows of teeth, its corners up; open, shouting, in a flinch
		mouth := marineColors['M']
		teeth := ui.Hex("#eee6d0")
		switch {
		case ouch > 0.3:
			cellAt(4, marineMouth-1, 5, mouth)
			cellAt(4, marineMouth, 5, mouth)
			cellAt(5, marineMouth+1, 3, mouth)
			cellAt(5, marineMouth-1, 3, teeth)
		case grin:
			cellAt(2, marineMouth, 9, mouth)
			cellAt(2, marineMouth-1, 1, mouth)
			cellAt(10, marineMouth-1, 1, mouth)
			for c := float32(3); c < 10; c++ {
				p.Fill(ui.Rect{X: fx0 + c*cell + cell*0.1, Y: fy0 + marineMouth*cell, W: cell * 0.8, H: cell * 0.42}, teeth, 0)
				p.Fill(ui.Rect{X: fx0 + c*cell + cell*0.1, Y: fy0 + marineMouth*cell + cell*0.58, W: cell * 0.8, H: cell * 0.42}, teeth, 0)
			}
		}
		// the blood of a while alone, more the longer: down the forehead, the cheek, the nose and the chin
		blood := ui.Hex("#9e1018")
		if hurt >= 1 {
			cellAt(9, 4, 1, blood)
			cellAt(9, 5, 1, blood)
			cellAt(10, 6, 0.6, blood)
		}
		if hurt >= 2 {
			cellAt(1, 9, 1, blood)
			cellAt(1, 10, 1, blood)
			cellAt(2, 11, 1, withAlpha(blood, 0.8))
			cellAt(6, 10, 1, blood)
		}
		if hurt >= 3 {
			cellAt(3, 4, 2, blood)
			cellAt(10, 12, 1, blood)
			cellAt(6, 15, 1, withAlpha(blood, 0.85))
		}
		if ouch > 0 { // the red flash of the hit
			p.Fill(ui.Rect{X: x0, Y: y0, W: fw, H: fh}, withAlpha(ui.Hex("#ff2a1f"), 0.45*ouch), 0)
		}
	})
	if shot > 0 { // the muzzle flash, out at its side
		fx, fy := x0+fw+cell*1.2, y0+fh*0.62
		r := cell * (1.6 + 1.4*f32(shot))
		glowAround(p, ui.Rect{X: fx - r, Y: fy - r, W: 2 * r, H: 2 * r}, ui.Hex("#ffb000"), 1.6*shot)
		p.Fill(ui.Rect{X: fx - r, Y: fy - cell/2, W: 2 * r, H: cell}, withAlpha(ui.Hex("#ffe066"), shot), 0)
		p.Fill(ui.Rect{X: fx - cell/2, Y: fy - r*0.7, W: cell, H: 1.4 * r}, withAlpha(ui.Hex("#ffe066"), shot), 0)
		p.Fill(ui.Rect{X: fx - cell/2, Y: fy - cell/2, W: cell, H: cell}, withAlpha(ui.Hex("#ffffff"), shot), 0)
	}
	g.gibs.stepDraw(p, tn, dt, func(m *mote, age float64) {
		s := f32(m.size)
		p.Fill(ui.Rect{X: f32(m.x) - s/2, Y: f32(m.y) - s/2, W: s, H: s}, withAlpha(m.col, 1-age*age), 0)
	})
	if ouch > 0 || shot > 0 || len(g.gibs) > 0 || g.rate > 0.5 || god {
		p.AnimationFrame()
	} else if f.rest < caretRests() { // to blink, and to take its wounds; then it stands, wounded, the window at rest (#188)
		p.After(800 * time.Millisecond)
	}
}
