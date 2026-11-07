package main

import (
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// Tekrar: to see the words written again and again. At the end of a word (a space, a mark, Enter), when the
// text has the word elsewhere too, every one of it in view lights up at once and fades slowly; written quickly,
// the next repeated word lights up too, in a color of its own, while the first still glows. And the caret holds
// up a sign, one-handed, on a spring: the word, and how many times the text has it.

var repeatColors = []string{"#f59e0b", "#ec4899", "#22c55e", "#3b82f6", "#a855f7", "#14b8a6"}

type repeatGlow struct {
	rects []ui.Rect
	t0    float64
	col   ui.Color
	word  string
}

type repeats struct {
	fxBase
	glows []repeatGlow
	sign  struct {
		word string
		n    int
		t0   float64
		col  ui.Color
	}
}

const (
	repeatHold = 450.0  // ms a word glows full
	repeatFade = 3200.0 // and fades over
	repeatSign = 2600.0 // ms the sign is held up
)

func newRepeats() *repeats {
	r := &repeats{}
	r.sign.t0 = -1e9
	return r
}

func (g *repeats) reset() { g.glows, g.have = nil, false; g.sign.t0 = -1e9 }

func (g *repeats) shift(dy float32) {
	for i := range g.glows {
		for j := range g.glows[i].rects {
			g.glows[i].rects[j].Y += dy
		}
	}
	g.prev.Y += dy
}

func (g *repeats) draw(p *ui.Painter, f caretFrame, k kick) {
	tn, _, _, _ := g.step(p, f, k)
	box := boxOf(f)
	h := float64(box.H)
	// a word ended: is it in the text again?
	ended := false
	for _, r := range k.typed {
		if !isWordRune(r) {
			ended = true
		}
	}
	if ended && f.words != nil {
		if w := f.words.last(); utf8.RuneCountInString(w) >= 2 {
			if rects, total := f.words.find(w); total >= 2 {
				live := 0 // the colors of the words still glowing: a new one takes the next
				same := -1
				for i, gl := range g.glows {
					if tn-gl.t0 < repeatHold+repeatFade {
						live++
					}
					if foldRunesEq(gl.word, w) {
						same = i
					}
				}
				col := ui.Hex(repeatColors[0])
				if same >= 0 { // the same word again: its own color, lit anew
					col = g.glows[same].col
					g.glows = append(g.glows[:same], g.glows[same+1:]...)
				} else if live > 0 {
					col = ui.Hex(repeatColors[live%len(repeatColors)])
				}
				g.glows = append(g.glows, repeatGlow{rects: rects, t0: tn, col: col, word: w})
				g.sign.word, g.sign.n, g.sign.t0, g.sign.col = w, total, tn, col
			}
		}
	}
	// the words: lit, then fading slowly
	out := g.glows[:0]
	for _, gl := range g.glows {
		age := tn - gl.t0
		if age > repeatHold+repeatFade {
			continue
		}
		a := 1.0
		if age > repeatHold {
			a = 1 - easeSine((age-repeatHold)/repeatFade)
		}
		pop := 1 + 0.12*math.Max(0, 1-age/220) // they light up with a little swell
		for _, r := range gl.rects {
			cxr, cyr := r.X+r.W/2, r.Y+r.H/2
			w, hh := r.W*f32(pop)+4, r.H*f32(pop)
			rr := ui.Rect{X: cxr - w/2, Y: cyr - hh/2, W: w, H: hh}
			p.Fill(ui.Rect{X: rr.X - 3, Y: rr.Y - 2, W: rr.W + 6, H: rr.H + 4}, withAlpha(gl.col, 0.12*a), rad(7))
			p.Fill(rr, withAlpha(gl.col, 0.3*a), rad(4))
			p.Fill(ui.Rect{X: rr.X + 1, Y: rr.Y + rr.H - 2, W: rr.W - 2, H: 2}, withAlpha(gl.col, 0.85*a), 1)
		}
		out = append(out, gl)
	}
	g.glows = out
	col := ui.Hex("#94a3b8")
	if tn-g.sign.t0 < repeatSign {
		col = mix(col, g.sign.col, 1-clamp01((tn-g.sign.t0-repeatSign*0.6)/(repeatSign*0.4)))
	}
	glassBody(p, f, box, col, 1, 0.9)
	g.drawSign(p, f, box, h, tn)
	if len(g.glows) > 0 || tn-g.sign.t0 < repeatSign+400 {
		p.AnimationFrame()
	}
}

// foldRunesEq reports whether two words are the same, in any case.
func foldRunesEq(a, b string) bool { return string(foldRunes(a)) == string(foldRunes(b)) }

// drawSign is the caret holding up its sign: an arm from its side that comes up on a spring, the board at its
// top with the word and its count, held a while and then let down.
func (g *repeats) drawSign(p *ui.Painter, f caretFrame, box ui.Rect, h, tn float64) {
	age := tn - g.sign.t0
	if age > repeatSign+400 {
		return
	}
	// up on a spring, past where it goes and back; down at the end
	up := 1 - math.Exp(-age/90)*math.Cos(age/1000*16)
	if age > repeatSign {
		up *= 1 - easeSine((age-repeatSign)/400)
	}
	if up <= 0.01 {
		return
	}
	ink := faceInk(f.dark, ui.Hex("#1f2937"))
	label := "×" + strconv.Itoa(g.sign.n)
	word := g.sign.word
	if utf8.RuneCountInString(word) > 14 {
		word = string([]rune(word)[:13]) + "…"
	}
	size := f32(math.Max(10, h*0.55))
	wspan := ui.Span{Text: word, Size: size * 0.85, Weight: 560, Color: ink}
	nspan := ui.Span{Text: label, Size: size, Weight: 760, Color: g.sign.col}
	ww, wh := p.MeasureText(0, wspan)
	nw, _ := p.MeasureText(0, nspan)
	bw, bh := ww+nw+f32(h*0.9), wh+f32(h*0.45)
	// the hand at the caret's side, the board up and to the right, swaying a little as it is held
	sway := f32(math.Sin(age/1000*3.2)) * f32(h*0.06) * f32(up)
	hx, hy := box.X+box.W, box.Y+box.H*0.45
	bx := box.X + box.W + f32(h*0.5) + sway
	by := box.Y - bh - f32(h*0.55)*f32(up)
	by += (1 - f32(up)) * f32(h*1.4) // it rises from the caret
	// the arm: a shoulder at the caret, an elbow, the hand at the board's foot
	gx, gy := bx+f32(h*0.35), by+bh
	ex, ey := hx+(gx-hx)*0.4+f32(h*0.35), hy+(gy-hy)*0.5
	arm := &ui.Path{}
	arm.MoveTo(hx, hy).QuadTo(ex, ey, gx, gy)
	p.StrokePath(arm, f32(math.Max(1.5, h*0.12)), withAlpha(g.sign.col, f2f(up)))
	p.Fill(ui.Rect{X: gx - f32(h*0.12), Y: gy - f32(h*0.12), W: f32(h * 0.24), H: f32(h * 0.24)}, withAlpha(g.sign.col, f2f(up)), f32(h*0.12))
	// the board: glass of the word's color, a pole down to the hand
	board := ui.Rect{X: bx, Y: by, W: bw, H: bh}
	p.Fill(ui.Rect{X: gx - 0.75, Y: by + bh - 2, W: 1.5, H: 4}, withAlpha(ink, 0.6*f2f(up)), 0)
	glowAround(p, board, g.sign.col, 0.8*up)
	glassFill(p, rrect(board.X, board.Y, board.W, board.H, rad(6)), g.sign.col, g.sign.col, f.dark, up)
	p.RichText(bx+f32(h*0.3), by+f32(h*0.22), ww+4, ui.Span{Text: word, Size: size * 0.85, Weight: 560, Color: withAlpha(ink, up)})
	p.RichText(bx+f32(h*0.3)+ww+f32(h*0.3), by+f32(h*0.22)-1, nw+4, ui.Span{Text: label, Size: size, Weight: 760, Color: withAlpha(mix(g.sign.col, ink, 0.25), up)})
}

func f2f(v float64) float64 { return clamp01(v) }
