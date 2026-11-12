package main

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/egoist/mygo/ui/shader"
	ryugo "github.com/imhalid/ryu-go"
)

// The reveal of a restore (#316): when a copy is restored in the page's history, a soft band of light runs once down
// the front card, from its top to its foot, before the history goes back into the editor. Above the band is the copy
// restored, below it the page as it was, the seam lost in the light; under the band the page bulges as through a lens,
// its lines pushed away from the light, with a little fringe of color. The card grows a little while it runs. The
// copy is written when Restore is pressed; the sweep only shows it. It is two cards in one place: the page as it was
// under, bent alone, and the copy over it, bent the same way, kept where the band has passed and carrying the light.

const (
	revealLen  = 1300 * time.Millisecond
	revealLens = 0.11  // how much the band magnifies at its middle: the part of the way to its middle a sample is drawn in
	revealGrow = 0.045 // how much larger the card is in the middle of the sweep
	revealOff  = 0.16  // how far before the top and past the foot the band starts and ends, as parts of the card's height
)

// revealDraws is false in tests: the tester draws on the CPU, where a shader over a whole page is slow. The sweep is
// timed all the same.
var revealDraws = !testing.Testing()

var revealShader = shader.MustCompile(`package main

var Front float  // where the band's middle is: 0 the card's top, 1 its foot
var Lens float   // how much it magnifies at its middle
var Glow float   // how bright its light is: 0 on the card under
var Mask float   // 1: only what the band has passed is kept (the copy); 0: all of it (the page as it was)
var Radius float // the card's corners
var Tone vec4

func card(p vec2, half vec2, r float) float {
	q := abs(p) - (half - vec2(r))
	return length(max(q, vec2(0))) + min(max(q.x, q.y), 0) - r
}

func LayerEffect(pos vec2) vec4 {
	sz := rectSize()
	front := Front * sz.y
	sig := sz.y * 0.11
	d := (pos.y - front) / sig
	g := exp(-d * d)

	// the lens: in the band, each pixel is drawn from nearer its middle, so what is there grows and what is beside it
	// is pushed away; less across than down, and each color a little apart from the others
	mid := vec2(sz.x*0.5, front)
	rel := pos - mid
	s := vec2(0.6, 1) * Lens * g
	pg := mid + rel*(vec2(1)-s)
	col := layerSample(pg)
	col.r = layerSample(mid + rel*(vec2(1)-s*0.94)).r
	col.b = layerSample(mid + rel*(vec2(1)-s*1.06)).b

	// the copy is kept where the band has passed, its edge soft in the light
	m := smoothstep(front+sig*0.9, front-sig*0.9, pos.y)
	col *= mix(1, m, Mask)

	// the light: white at its core, the tone round it, strongest across the middle of the card and inside its corners
	ax := (pos.x/sz.x - 0.5) * 2
	across := 1 - 0.6*ax*ax
	in := smoothstep(1, -1, card(pos-sz*0.5, sz*0.5, Radius))
	core := exp(-d * d * 2.6)
	light := (vec3(1)*core*0.8 + Tone.rgb*g*0.5) * across * in * Glow
	col.rgb += light // added: over the card under it too, where the copy is not yet
	return col
}
`)

// tmReveal is the sweep going on: the copy restored, at its place in the stack, and the page as it was.
type tmReveal struct {
	at   time.Time // when it began: zero until the frame that first draws it
	sel  int       // the copy's place: 0 when none goes on
	page *Page     // the page as it was, its title, cover and values
	doc  *richtext.Document
}

// busy says the history takes no clicks or keys: it is going, or a restore is being shown.
func (t *timeMachine) busy() bool { return t.closing || t.reveal.sel > 0 }

// tmRevealOld is the place in the stack's caches of the page as it was, while the sweep shows it.
const tmRevealOld = -1

// startReveal keeps, before copy sel is put back, what the sweep shows: the copy as the stack has it, and the page
// as it is, read anew, for the page itself becomes the copy.
func (a *app) startReveal(sel int) (keep func()) {
	t := &a.tm
	newPage, newDoc := a.tmVersionPage(sel), a.tmDoc(sel)
	old := new(Page)
	*old = *t.page
	body, _ := t.page.Body()
	oldDoc := richtext.NewDocument(a.parsePage(old, body))
	return func() {
		t.pages = map[int]*Page{sel: newPage, tmRevealOld: old}
		t.docs = map[int]*richtext.Document{sel: newDoc, tmRevealOld: oldDoc}
		t.reveal = tmReveal{sel: sel, page: old, doc: oldDoc}
		t.sel = sel
	}
}

// revealStep times the sweep, on the view's clock, and says how far it is (0..1); at its end the history goes.
func (a *app) revealStep(c *ui.Context) float32 {
	r := &a.tm.reveal
	if r.sel == 0 {
		return 0
	}
	now := c.Now()
	if r.at.IsZero() {
		r.at = now
	}
	p := float32(now.Sub(r.at)) / float32(revealLen)
	if p >= 1 {
		a.tm.reveal = tmReveal{}
		a.tm.closing = true
		c.AnimationFrame() // the going begins at the next frame
		return 0
	}
	c.AnimationFrame()
	return p
}

// revealFront is where the band's middle is at p of the sweep, as a part of the card's height: slow at both ends.
func revealFront(p float32) float32 {
	e := p * p * (3 - 2*p)
	return -revealOff + (1+2*revealOff)*e
}

// revealScale is how much larger the card is at p of the sweep.
func revealScale(p float32) float32 {
	return 1 + revealGrow*float32(math.Sin(math.Pi*float64(p)))
}

// revealEffect lays the sweep at p on a card: over (the copy) or under (the page as it was); w by h is its room and radius its
// corners, in that room.
func revealEffect(card ui.Element, p float32, over bool, w, h, radius float32, cl ryugo.Colors) {
	if !revealDraws || !shadersOn() {
		return
	}
	env := smoothstepf(0, 0.12, p) * (1 - smoothstepf(0.88, 1, p)) // the lens and light come up and die away
	glow, mask := float32(0), float32(0)
	if over {
		glow, mask = env, 1
	}
	off := max(w, h) * revealLens
	card.Effect(revealShader.Effect(shader.Options{
		Uniforms: map[string]any{
			"Front": revealFront(p), "Lens": revealLens * env, "Glow": glow, "Mask": mask, "Radius": radius,
			"Tone": ryugo.Mix(cl.Accent, ui.Hex("#ffffff"), 0.35),
		},
		MaxSampleOffset: ui.Point{X: off, Y: off},
	}))
}

// smoothstepf is GLSL's smoothstep, for the side of the app.
func smoothstepf(e0, e1, x float32) float32 {
	t := min(max((x-e0)/(e1-e0), 0), 1)
	return t * t * (3 - 2*t)
}
