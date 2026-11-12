package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The first start after an update says so without a dialog (#392): a wave of light runs up the window from its bottom,
// as through glass (updHelloShader), so the app feels new and clean, and a small note rises at the bottom right: the
// version, a few words of what changed, and a link to the changelog on the website. Closed, it does not come again.

// A helloState is the note of the version just installed: when its first frame was, its words, whether it was closed;
// and the wave's own clock. The window of a restarted app takes a moment to come up and to draw smoothly (the shader is
// made on the GPU the first time it draws), so the wave waits for that, drawing itself unseen meanwhile (warm frames),
// and its clock then moves at most a frame's time a frame: a slow frame holds it back rather than skip a part of it,
// so it is seen from its start.
type helloState struct {
	at     time.Time
	notes  string
	closed bool
	warm   int           // the frames drawn before the wave starts
	going  bool          // the wave has started
	last   time.Time     // the frame its clock last moved at
	run    time.Duration // how far its clock is
}

// The hello's timing: the wave starts once the window has drawn helloWarm frames and helloDelay has gone, and runs for
// helloWave, its clock moving at most helloStep a frame; the note rises with it, in helloRise.
var (
	helloDelay = 600 * time.Millisecond
	helloWave  = 2300 * time.Millisecond
	helloRise  = 450 * time.Millisecond
	helloStep  = 34 * time.Millisecond
)

const helloWarm = 12

// helloChangelog is the changelog on the website, where the note's link goes.
const helloChangelog = "https://celestine.halid.dev/changelog"

// startHello begins the hello of the version just installed, once: what its first start shows in place of a dialog.
func (a *app) startHello(c *ui.Context) {
	a.hello = helloState{at: c.Now(), notes: a.settings.UpdateNotes}
	if updTrialRestarted { // the trial's saves nothing (updtrial.go)
		updTrialRestarted = false
		return
	}
	a.settings.UpdateSeen = version
	a.saveSettings()
}

// helloTick moves the wave's clock, once a frame: first the warm frames, then a frame's time at most.
func (a *app) helloTick(c *ui.Context) {
	h := &a.hello
	if h.at.IsZero() || h.run >= helloWave {
		return
	}
	now := c.Now()
	if fx.still { // reduced motion: no wave, and the note is there at once
		h.going, h.run = true, helloWave
		return
	}
	if !h.going {
		h.warm++
		if h.warm >= helloWarm && now.Sub(h.at) >= helloDelay {
			h.going, h.last = true, now
		}
		return
	}
	h.run += min(now.Sub(h.last), helloStep)
	h.last = now
}

// helloRoot runs the wave over the window's content, root, while it goes; before it, it draws the wave's shader at its
// end, where it changes nothing, so that it is ready when the wave starts.
func (a *app) helloRoot(c *ui.Context, root ui.Element) {
	a.helloTick(c)
	h := &a.hello
	if h.at.IsZero() || h.run >= helloWave || a.wel.on {
		return
	}
	c.AnimationFrame()
	if !updShaders || !shadersOn() { // with the shader effects off the note still rises on the wave's clock
		return
	}
	t := float32(1) // warm: the end, where the window is as it is
	if h.going {
		t = float32(h.run.Seconds() / helloWave.Seconds())
	}
	e := 1 - (1-t)*(1-t) // out fast, slowing as it reaches the far corners
	w, ht := c.Size()
	dark := float32(0)
	if c.Theme().Dark {
		dark = 1
	}
	reach := float32(1.1)*hypot(w/2, ht+40) + 200 // past the far corners, with room for the curls
	root.Effect(updHelloShader.Effect(shader.Options{
		Uniforms: map[string]any{"Origin": []float32{w / 2, ht + 40}, "Radius": e * reach, "Prog": t,
			"Time": float32(h.run.Seconds()), "Dark": dark, "Size": []float32{w, ht}},
		MaxSampleOffset: ui.Point{X: 40, Y: 40},
	}))
}

// hypot is the length of the vector (x, y).
func hypot(x, y float32) float32 {
	return float32(math.Hypot(float64(x), float64(y)))
}

// updateHello is the note of the version just installed, at the window's bottom right: it rises from below as the wave
// starts, and goes when closed.
func (a *app) updateHello(c *ui.Context) {
	if a.hello.at.IsZero() || a.hello.closed || !a.hello.going {
		return
	}
	k := float32(min(a.hello.run.Seconds()/helloRise.Seconds(), 1)) // on the wave's clock, so it rises with it
	k = 1 - (1-k)*(1-k)*(1-k)
	if k < 1 {
		c.AnimationFrame()
	}
	cl := ryugo.From(c).Colors
	r := rad(12)
	ui.Overlay(c, func() {
		// as far from the content's bottom edge as from its right: in the frame, the content's card ends frameW in from
		// the window's right and at the status bar (or frameW up, without it); else at the window's right and the bar
		in := float32(16)
		bottom := in
		switch {
		case a.wframe.on && a.settings.HideStatus:
			in -= frameW
			bottom = frameW + in
		case a.wframe.on:
			in -= frameW
			bottom = a.statusH + in
		case !a.settings.HideStatus:
			bottom = a.statusH + in
		}
		card := ui.Column(c.Key("upd-hello")).Absolute().Right(16).Bottom(bottom).Width(helloW).Radius(r).
			// a faint tint and a light border of the theme's accent, as the update's card has: a notice (#398, #400)
			Background(ryugo.Mix(cl.Card, cl.Accent, 0.06)).Border(1, cl.Accent.Alpha(0.28)).Shadow(0, 6, 18, -6, black.Alpha(0.18)).
			Role(ui.RoleDialog).Label(locales.UpdateWhatsNew(shownVersion(version))).
			Translate(0, 24*(1-k)).Opacity(k)
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Start).Padding(12, 10, 12, 12).Children(func() {
				updBadge(c, cl.ColorAccent400, cl.ColorAccent600, iconCheck, "upd-hello") // installed
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, locales.UpdateWhatsNew(shownVersion(version))).FontSize(13.5).FontWeight(680).TextColor(cl.Text).Wrap()
					ui.Text(c, helloSummary(a.hello.notes)).FontSize(12).TextColor(cl.Text2).Wrap()
					l := ui.Text(c.Key("upd-hello-link"), locales.UpdateChangelog()+" →").FontSize(12).FontWeight(600).
						TextColor(cl.Accent).Role(ui.RoleLink).Cursor(ui.CursorPointer).Margin(4, 0, 0, 0)
					if l.Hovered() {
						l.Underline()
					}
					if l.Clicked() {
						c.OpenURL(helloChangelog)
					}
				})
				x := ui.Box(c.Key("upd-hello-close")).Width(22).Height(22).Radius(11).AlignItems(ui.Center).Justify(ui.Center).
					Shrink(0).Role(ui.RoleButton).Label(locales.Close()).Cursor(ui.CursorPointer)
				if x.Hovered() {
					x.Background(cl.Fill)
				}
				x.Children(func() { ui.Icon(c, iconClose).Width(13).Height(13).TextColor(cl.Muted) })
				if x.Clicked() {
					a.hello.closed = true
				}
			})
		})
	})
}

// helloW is the note's width: wider than the update's card, for a few words of what changed.
const helloW = 300

// helloSummary is a few words of what changed: the notes' first lines as plain text, cut at a sentence's length; or,
// with no notes, that there are none.
func helloSummary(notes string) string {
	var lines []string
	for _, l := range strings.Split(notes, "\n") { // each line of the notes on its own: a list's items stay apart
		if l = strings.TrimSpace(plainText(l)); l != "" {
			lines = append(lines, strings.TrimLeft(l, "-*• "))
		}
		if len(lines) == 3 {
			break
		}
	}
	if len(lines) == 0 {
		return locales.UpdateNoNotes()
	}
	s := strings.Join(lines, " · ")
	if rs := []rune(s); len(rs) > 160 {
		s = strings.TrimSpace(string(rs[:157])) + "…"
	}
	return s
}

// updHelloShader is the wave of the first start after an update, made as the welcome's ring opening is
// (welcomelooks.go): its front is not a circle but noise bent by noise, reaching out in tongues and curling back, what
// is near it turned a little round the origin so the curls wind, flowing all the while. Just behind the front the
// window bulges as under one lens and splits into its colors, the red and blue apart from the green; a light runs along
// the front in four colors (blue, violet, pink, peach), in a softer glow, with a trail of wisps behind it, tinting the
// window in light (Dark 0) and adding light in dark. Where it meets the window's edges, the content there stretches a
// little toward them, as under the bevel of a glass. All of it dies away as it passes the far corners. Origin is below
// the window's bottom middle, Radius how far the front is, Prog how far the wave is, 0 to 1, over a window of Size.
var updHelloShader = shader.MustCompile(helloShader + fbmShader + ringShared)

const helloShader = `package main

var Origin vec2
var Radius float
var Prog float
var Time float
var Dark float
var Size vec2

func line(d float, at float, w float) float {
	t := (d - at) / w
	return exp(-t * t)
}

// curled is the distance from the origin the front is measured on: the place turned round the origin near the
// front, then pushed by noise that is itself pushed by noise (as the welcome's ring)
func curled(p vec2, r float, life float, t float) float {
	l := length(p)
	near := exp(-((l-r)/260)*((l-r)/260))
	tw := 0.9 * near * life * (0.6 + 0.4*sin(atan2(p.y, p.x)*3+t*0.7))
	c := cos(tw)
	s := sin(tw)
	p = vec2(c*p.x-s*p.y, s*p.x+c*p.y)
	q := p / 170
	w1 := vec2(fbm(q+vec2(0, t*0.35)), fbm(q+vec2(5.2, 1.3)-t*0.25)) - 0.5
	w2 := vec2(fbm(q*2.1+w1*2.6+vec2(1.7, 9.2)), fbm(q*2.1+w1*2.6+vec2(8.3, 2.8))) - 0.5
	w3 := vec2(noise(q*6+w2*4+vec2(t*0.5, 0)), noise(q*6+w2*4+vec2(3.3, -t*0.5))) - 0.5
	warp := (w1 + w2*0.8 + w3*0.35) * 190 * life * (0.35 + 0.65*near)
	return length(p + warp)
}

// aurora is the light's color along the front: blue, violet, pink and peach going round it, flowing
func aurora(u float) vec3 {
	u = fract(u) * 4
	c0 := vec3(80, 140, 255) / 255
	c1 := vec3(160, 110, 255) / 255
	c2 := vec3(255, 110, 180) / 255
	c3 := vec3(255, 190, 140) / 255
	if u < 1 {
		return mix(c0, c1, u)
	} else if u < 2 {
		return mix(c1, c2, u-1)
	} else if u < 3 {
		return mix(c2, c3, u-2)
	}
	return mix(c3, c0, u-3)
}

func LayerEffect(pos vec2) vec4 {
	rel := pos - Origin
	d0 := length(rel)
	dir := rel / max(d0, 0.001)
	live := 1 - smoothstep(0.55, 1.0, Prog)
	curl := sin(min(Prog*1.3, 1)*3.14159*0.5+0.6) * live // strongest early, gone at the end
	d := curled(rel, Radius, curl, Time)
	edge := Radius

	// the lens: just behind the front the window bulges outward and splits into its colors; at the front itself and
	// ahead of it, it is as it is, so there is no seam
	s := edge - d
	bulge := smoothstep(0, 26, s) * exp(-max(s, 0)/120) * live
	stretch := exp(-max(-s, 0)/30) * step(s, 0) * live // ahead, a little pulled toward the front
	off := dir * (bulge*24 - stretch*5)
	split := dir * bulge * 7
	// the window's edges: the content there leans toward them as the wave passes, as under a glass's bevel
	e := min(min(pos.x, Size.x-pos.x), min(pos.y, Size.y-pos.y))
	toEdge := vec2(0, -1)
	if e == pos.x {
		toEdge = vec2(1, 0)
	} else if e == Size.x-pos.x {
		toEdge = vec2(-1, 0)
	} else if e == pos.y {
		toEdge = vec2(0, 1)
	}
	off += toEdge * exp(-e/14) * line(d, edge-40, 120) * 6 * live
	lo := vec2(0.5)
	hi := Size - 0.5 // read inside the window: past its edges there is nothing, and it would show as a pale band
	r := layerSample(clamp(pos-off-split, lo, hi))
	g := layerSample(clamp(pos-off, lo, hi))
	b := layerSample(clamp(pos-off+split, lo, hi))
	col := vec4(r.r, g.g, b.b, g.a)

	// the light along the curled front, its colors spread, in a glow, and a trail of wisps behind it
	at := edge - 10
	ang := atan2(dir.y, dir.x)
	tint := aurora(ang*0.6 + Time*0.25)
	rim := vec3(line(d, at+2.6, 3.2), line(d, at, 3.0), line(d, at-2.6, 3.2))
	glow := line(d, at, 28)
	smoke := fbm(rel/90 + vec2(Time*0.4, 0))
	trail := line(d, at-70, 60) * smoothstep(0.45, 0.75, smoke)
	inner := line(d, at-30, 45) // the rings of color just behind the front
	band := (glow*0.55 + trail*0.18 + inner*0.5) * live
	if Dark > 0.5 { // screen: it adds light, so it glows
		light := (rim*tint*1.2 + tint*band*0.6) * live
		return vec4(col.rgb+light*(col.a-col.rgb*0.5), col.a)
	}
	// light: the rings tint the window, and the rim is a thin bright line of color
	k := min(band, 1) * 0.7
	out := mix(col.rgb, col.rgb*mix(vec3(1), tint, 0.85), k)
	out = mix(out, tint*col.a, min(max(max(rim.r, rim.g), rim.b)*0.7*live, 1))
	return vec4(out, col.a)
}
`
