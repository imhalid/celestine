package main

import (
	"errors"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's toasts: a line at the bottom right of the window, with a button when what it says can be taken back or
// opened. A toast does not move as it comes or goes: a light runs across it from left to right and leaves it there,
// as if a cover were pulled off it, then goes once round its edge; another light runs across it as it goes and takes
// it away (toastShader). A toast has the color of what it says: done, a warning, an error, or only news. Toasts that
// come while one is up stand over it.

const (
	toastIn    = 0.95 // seconds the light takes to bring a toast
	toastBeam  = 2.2  // and then to go round its edge
	toastOut   = 0.62 // and to take it away
	toastStay  = 4.0  // seconds a toast stays, the pointer not on it
	toastStayA = 6.5  // one with a button, which takes longer to reach
	toastMax   = 3    // at most this many at once: the oldest goes first
	toastPad   = 40   // how far round a toast its light and shadow spill
)

// toastShaders is false in the tests, whose CPU renderer runs the light far too slowly for every toast of every test
// (helpers_test.go); without it a toast comes and goes at once.
var toastShaders = true

// toastKind is what a toast says, which gives it its color and its mark.
type toastKind int

const (
	toastInfo toastKind = iota // news: the accent
	toastOK                    // done
	toastWarn                  // something is missing or off, nothing lost
	toastErr                   // it did not work
)

// say puts up text, a message already in the window's language, as a notice of kind: the kind is told, never read
// from the words.
func (a *app) say(kind toastKind, text string) {
	a.notice, a.noticeTone = text, &kind
	a.playSound(soundForToast(kind))
}

// sayDone puts up text as done, or as an error when ok is false: for what reports both in one message.
func (a *app) sayDone(text string, ok bool) {
	a.say(map[bool]toastKind{true: toastOK, false: toastErr}[ok], text)
}

// A toastItem is a toast up.
type toastItem struct {
	id      int
	kind    toastKind
	text    string
	action  string // the button's label, or none
	fn      func()
	stay    float64   // seconds it stays
	closing bool      // it is to go now
	at      time.Time // when it came, on the window's clock: zero until it is first drawn
	until   time.Time // when it starts to go
	goneAt  time.Time // when it started to go; zero while it stays
}

// toastState is the toasts up, oldest first.
type toastState struct {
	items []*toastItem
	next  int
}

// toast puts up a toast that says text: news.
func (a *app) toast(text string) { a.toastOf(toastInfo, text, "", nil) }

// fail says that something did not work, in an error toast. The open page's own save keeps its error in saveErr,
// shown in the status bar; nothing else does, so another failure neither shows as "could not save" nor holds the
// page's autosave back (#216).
func (a *app) fail(err error) {
	if errors.Is(err, ErrLocked) { // a past day of the journal (#622): said as what it is, not as a fault
		a.sayLocked()
		return
	}
	a.toastOf(toastErr, err.Error(), "", nil)
}

// toastAction puts up a toast that says text, with a button labelled action that runs fn and closes it.
func (a *app) toastAction(text, action string, fn func()) { a.toastOf(toastInfo, text, action, fn) }

// toastOf puts up a toast of a kind; action and fn are its button, or none.
func (a *app) toastOf(kind toastKind, text, action string, fn func()) {
	stay := toastStay
	if fn != nil {
		stay = toastStayA
	}
	a.toasts.next++
	a.toasts.items = append(a.toasts.items, &toastItem{id: a.toasts.next, kind: kind, text: text, action: action, fn: fn, stay: stay})
	for n, i := len(a.toasts.items), 0; n-i > toastMax; i++ { // too many: the oldest go
		a.toasts.items[i].closing = true
	}
}

// toastTone is the color of a kind of toast, and its mark.
func toastTone(cl ryugo.Colors, k toastKind) (ui.Color, *ui.SVG) {
	switch k {
	case toastOK:
		return cl.Down, iconToastOK
	case toastWarn:
		return cl.Warn, iconToastWarn
	case toastErr:
		return cl.Up, iconToastErr
	}
	return cl.Accent, iconToastInfo
}

var (
	iconToastOK   = icon(`<path d="M5 12.5l4.2 4.2L19 7"/>`)
	iconToastWarn = icon(`<path d="M12 7.5v6"/><path d="M12 17h.01"/>`)
	iconToastErr  = icon(`<path d="M7 7l10 10M17 7L7 17"/>`)
	iconToastInfo = icon(`<path d="M12 11v6"/><path d="M12 7.5h.01"/>`)
)

// toastView draws the toasts and lets go of those gone.
func (a *app) toastView(c *ui.Context) {
	if len(a.toasts.items) == 0 {
		return
	}
	now := c.Now()
	live := a.toasts.items[:0]
	for _, t := range a.toasts.items {
		out := toastOut
		if !toastShaders || !shadersOn() {
			out = 0
		}
		if t.goneAt.IsZero() || now.Sub(t.goneAt).Seconds() < out {
			live = append(live, t)
		}
	}
	a.toasts.items = live
	if len(live) == 0 {
		return
	}
	cl := ryugo.From(c).Colors
	w, _ := c.Size()
	// Where each toast is in its coming and going is worked out here, as the frame starts: what Overlay is given is
	// built after the view, too late to ask for the frames.
	type step struct{ in, beam, out float64 }
	steps := make([]step, len(live))
	moving, going := false, false
	next := time.Duration(-1) // when the first toast's time runs out
	for i, t := range live {
		if t.at.IsZero() {
			t.at, t.until = now, now.Add(time.Duration(t.stay*float64(time.Second)))
		}
		if t.goneAt.IsZero() && (t.closing || !now.Before(t.until)) {
			t.goneAt = now
		}
		age := now.Sub(t.at).Seconds()
		st := step{in: toastEase(age / toastIn), beam: min(1, max(0, (age-toastIn)/toastBeam))}
		if !t.goneAt.IsZero() {
			st.out = toastEase(now.Sub(t.goneAt).Seconds() / toastOut)
			going = true
		} else if d := t.until.Sub(now); next < 0 || d < next {
			next = d
		}
		if st.in < 1 || st.beam < 1 {
			moving = true
		}
		steps[i] = st
	}
	switch {
	case (moving || going) && toastShaders && shadersOn(): // the light is set as the view is built, so its frames build it
		c.AnimationFrame()
	case going: // no light: a toast going is gone at the next frame
		c.After(16 * time.Millisecond)
	case next >= 0:
		c.After(next)
	}
	ui.Overlay(c, func() {
		bottom := float32(16)
		if !a.settings.HideStatus { // over the status bar, not on it
			bottom += 30
		}
		stack := ui.Column(c.Key("toasts")).Absolute().Right(16).Bottom(bottom).Gap(8).AlignItems(ui.End).MaxWidth(min(380, w-32))
		stack.Children(func() {
			for i, t := range live {
				a.toastOne(c, t, steps[i].in, steps[i].beam, steps[i].out, cl)
			}
		})
	})
}

// toastOne is one toast: the card, tinted with its color from the top left, its mark, its words and its button; the
// shader brings it (in, 0..1), runs the light round its edge (beam, 0..1) and takes it (out, 0..1), drawing its light
// and the card's shadow in the room round it.
func (a *app) toastOne(c *ui.Context, t *toastItem, in, beam, out float64, cl ryugo.Colors) {
	id := strconv.Itoa(t.id)
	tone, mark := toastTone(cl, t.kind)
	r := rad(10)
	tint := tone.Alpha(0.16)
	card := ui.Row(c.Key("toast:"+id)).AlignItems(ui.Center).Gap(11).Padding(10, 12, 10, 12).MinHeight(46).
		Radius(r).Background(ryugo.Mix(cl.Surface, tone, 0.05)).Border(1, tone.Alpha(0.30)).Role(ui.RoleStatus).Label(t.text).
		Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.14))
	card.Draw(func(p *ui.Painter, b ui.Rect) {
		p.FillRadialGradient(b, ryugo.RadialInOklch(ui.RadialGradient{Center: ui.Point{X: 0.001, Y: 0.001}, Radius: b.W * 0.8,
			Stops: []ui.GradientStop{{Color: tint, At: 0}, {Color: tint.Alpha(0), At: 1}}}), r)
	})
	if toastShaders && shadersOn() && (in < 1 || beam < 1 || out > 0) {
		card.Effect(toastShader.Effect(shader.Options{
			Uniforms: map[string]any{"In": float32(in), "Beam": float32(beam), "Out": float32(out), "Radius": r,
				"Time": float32(c.Now().Sub(t.at).Seconds()), "Tone": tone},
			MaxSampleOffset: ui.Point{X: toastPad, Y: toastPad},
		}))
	}
	if card.Hovered() && t.goneAt.IsZero() { // the time stops under the pointer
		if keep := c.Now().Add(1500 * time.Millisecond); keep.After(t.until) {
			t.until = keep
		}
	}
	card.Children(func() {
		ui.Box(c.Key("toast-mark:" + id)).Width(24).Height(24).Radius(12).Background(tone.Alpha(0.16)).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, mark).Width(14).Height(14).TextColor(tone) })
		ui.Text(c, t.text).FontSize(13.5).TextColor(cl.Text).Grow(1).MinWidth(0).MaxLines(3)
		if t.fn != nil {
			b := ui.Row(c.Key("toast-act:"+id)).Padding(5, 11).Radius(rad(7)).Background(tone.Alpha(0.14)).
				Role(ui.RoleButton).Label(t.action).Cursor(ui.CursorPointer)
			if b.Hovered() {
				b.Background(tone.Alpha(0.26))
			}
			b.Children(func() { ui.Text(c, t.action).FontSize(12.5).FontWeight(620).TextColor(tone) })
			if b.Clicked() && t.goneAt.IsZero() {
				t.closing = true
				t.fn()
			}
		}
	})
}

// toastEase is how far the light has run at t of 0..1: quick off, settling in.
func toastEase(t float64) float64 {
	if t >= 1 {
		return 1
	}
	return easeOut(t)
}

// toastShader brings a toast in with a light of its color running across it from left to right: behind the light the
// toast is there, lit for a moment as if just uncovered; ahead of it, nothing. The light's edge is not straight but
// wavers, with a few sparks thrown off it. Then a light goes once round the card's edge, with a tail behind it and a
// glow outside it (Beam). Going, a second light runs across the same way and takes the toast with it. Its positions
// start at the card's corner, and it draws a little round it too; at In 1, Beam 1 and Out 0 it gives the toast as it is.
var toastShader = shader.MustCompile(`package main

var In float
var Beam float
var Out float
var Radius float
var Time float
var Tone vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(x float) float {
	i := floor(x)
	f := fract(x)
	u := f * f * (3 - 2*f)
	return mix(hash(vec2(i, 0)), hash(vec2(i+1, 0)), u)
}

// card is how far pos is outside the card (negative inside)
func card(p vec2, half vec2, r float) float {
	q := abs(p) - (half - vec2(r))
	return length(max(q, vec2(0))) + min(max(q.x, q.y), 0) - r
}

func band(d float, w float) float {
	t := d / w
	return exp(-t * t)
}

func LayerEffect(pos vec2) vec4 {
	sz := rectSize()
	cw := sz.x
	half := sz * 0.5
	rel := pos - half
	sd := card(rel, half, Radius)
	x := pos.x // across the card, 0 at its left
	tone := Tone.rgb

	// where each light is: from before the card's left to past its right, its edge wavering down the card
	waver := (noise(pos.y/9+Time*3.0)-0.5)*14 + (noise(pos.y/3.5-Time*5.0)-0.5)*5
	span := cw + 120
	inEdge := -60 + span*In + waver
	outEdge := -60 + span*Out + waver

	shown := smoothstep(inEdge+2, inEdge-14, x) // behind the coming light
	if Out > 0 {
		shown *= smoothstep(outEdge-2, outEdge+14, x) // and ahead of the going one
	}
	col := layerSample(pos) * shown

	// just uncovered, the toast is lit, fading as the light goes on
	behind := max(inEdge-x, 0)
	lit := exp(-behind/46) * shown * (1 - smoothstep(0.85, 1.0, In))
	col.rgb += (col.rgb*0.5 + tone*0.22) * lit * col.a

	// the running light: a thin white line in a glow of the toast's color, inside the card and a little past its edge
	in := smoothstep(4, -1, sd)
	near := exp(-max(sd, 0)/4) * 0.3
	live := (1 - smoothstep(0.82, 1.0, In)) + step(0.001, Out)*(1-smoothstep(0.8, 1.0, Out))
	edge := inEdge
	if Out > 0 {
		edge = outEdge
	}
	d := x - edge
	core := band(d, 2.2)
	glow := band(d, 16) * 0.55
	wide := band(d+18, 48) * 0.18
	light := (vec3(1)*core + mix(tone, vec3(1), 0.35)*glow + tone*wide) * max(in, near) * live

	// sparks: a few points in cells along the edge, thrown back as the light runs
	cell := floor(vec2(x-edge, pos.y) / vec2(9, 7))
	h := hash(cell + floor(Time*14))
	sp := step(0.93, h) * band(d+10, 16) * in * live
	light += mix(tone, vec3(1), 0.6) * sp * 0.9

	// the beam round the edge: a head going once round, from the left, clockwise, with a tail; it comes up and dies away
	if Beam > 0 && Beam < 1 && Out == 0 {
		ang := atan2(rel.y/half.y, rel.x/half.x)
		head := -3.14159 + Beam*2.0*3.14159*1.15
		lag := ang - head
		lag = atan2(sin(lag), cos(lag)) // -pi..pi, behind the head negative
		tail := exp(min(lag, 0)*1.6) * step(lag, 0.06) + band(lag, 0.06)
		env := sin(3.14159 * Beam) // up and away
		ring := exp(-abs(sd+0.6)/1.0) * 1.35
		halo := exp(-max(sd, 0)/9) * step(-0.5, sd) * 0.75
		inner := exp(min(sd, 0)/12) * step(sd, 0) * 0.16
		b := (ring + halo + inner) * tail * env
		light += mix(tone, vec3(1), 0.55*band(lag, 0.25)) * b
	}

	out := col
	out.rgb += light
	a := max(out.a, min(max(max(light.r, light.g), light.b), 1))
	return vec4(out.rgb, a)
}
`)
