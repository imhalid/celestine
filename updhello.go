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

// The first start after an update says so without a dialog (#392): the window comes up frosted, and a wave of glass
// sweeps it clear from its bottom left corner (updHelloShader, #417), so the app feels new and clean, and a small note rises at the bottom right: the
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
	helloWave  = 2000 * time.Millisecond
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
// start, the window frosted, so that it is ready when the wave starts and the wave starts from what was seen.
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
	t := float32(0) // warm: the start, the window frosted as the version that was, until the wave sweeps it clear
	if h.going {
		t = float32(h.run.Seconds() / helloWave.Seconds())
	}
	e := t * t * (3 - 2*t) // in slowly, through the middle, and slowing as it leaves
	w, ht := c.Size()
	dark := float32(0)
	if c.Theme().Dark {
		dark = 1
	}
	// the arc's center is beyond the bottom left corner; its front starts there and ends a band's width past the top right
	ox, oy := -0.3*w, ht+0.3*ht
	reach := hypot(w-ox, oy) + 260
	root.Effect(updHelloShader.Effect(shader.Options{
		Uniforms: map[string]any{"Origin": []float32{ox, oy}, "Radius": e * reach, "Prog": t,
			"Time": float32(h.run.Seconds()), "Dark": dark, "Size": []float32{w, ht}},
		MaxSampleOffset: ui.Point{X: 96, Y: 96},
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

// updHelloShader is the wave of the first start after an update (#417): a transition from the version before to the one
// just installed, as a snapshot bent into the next. One smooth arc sweeps the window from beyond its bottom left corner
// to past its top right. Ahead of it the window is frosted, blurred and a little dimmed, as the version that was; behind
// it the window is clear. Between them a band bends the content as thick glass does, pushing it outward from the arc's
// center, blurring it, most along the way the wave goes, and splitting its colors; an iridescent rim runs on the front
// (cyan, blue, violet, pink, gold going round it), in a glow, and the window's own edges glow faintly in the same
// colors while it passes. Light is added in dark (Dark 1) and tints the window in light. Origin is the arc's center,
// Radius how far its front is, Prog how far the wave is, 0 to 1, over a window of Size.
var updHelloShader = shader.MustCompile(helloShader)

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

// iris is the rim's color, going round: cyan, blue, violet, pink, gold
func iris(u float) vec3 {
	u = fract(u) * 5
	c0 := vec3(90, 230, 255) / 255
	c1 := vec3(80, 130, 255) / 255
	c2 := vec3(170, 100, 255) / 255
	c3 := vec3(255, 100, 190) / 255
	c4 := vec3(255, 215, 120) / 255
	if u < 1 {
		return mix(c0, c1, u)
	} else if u < 2 {
		return mix(c1, c2, u-1)
	} else if u < 3 {
		return mix(c2, c3, u-2)
	} else if u < 4 {
		return mix(c3, c4, u-3)
	}
	return mix(c4, c0, u-4)
}

// frost is the window at p blurred over a disk of radius r, stretched along dir, with its red and blue drawn split
// apart by split: a spiral of taps, each read three times
func frost(p vec2, r float, dir vec2, split vec2) vec4 {
	lo := vec2(0.5)
	hi := Size - 0.5 // read inside the window: past its edges there is nothing, and it would show as a pale band
	if r < 0.6 {
		g := layerSample(clamp(p, lo, hi))
		return vec4(layerSample(clamp(p-split, lo, hi)).r, g.g, layerSample(clamp(p+split, lo, hi)).b, g.a)
	}
	side := vec2(-dir.y, dir.x)
	sum := vec4(0)
	for i := 0; i < 22; i++ {
		f := float(i) + 0.5
		k := sqrt(f / 22) * r
		a := f * 2.39996
		o := dir*cos(a)*k*1.5 + side*sin(a)*k*0.8
		g := layerSample(clamp(p+o, lo, hi))
		sum += vec4(layerSample(clamp(p+o-split, lo, hi)).r, g.g, layerSample(clamp(p+o+split, lo, hi)).b, g.a)
	}
	return sum / 22
}

func LayerEffect(pos vec2) vec4 {
	rel := pos - Origin
	d := length(rel)
	dir := rel / max(d, 0.001)
	live := 1 - smoothstep(0.88, 1.0, Prog) // all of it gone as the front leaves the far corner

	// s is how far behind the front pos is: ahead (s < 0) the version that was, frosted; behind it, clear
	s := Radius - d
	band := line(s, 70, 95) * smoothstep(-30, 10, s) // the glass, just behind the front
	ahead := 1 - smoothstep(-24, 18, s)
	blur := ahead*7 + band*26
	off := dir * band * 34 // the glass pushes the content outward, away from the arc's center
	split := dir * (band*8 + ahead*1.2)
	col := frost(pos-off, blur*live, dir, split*live)
	col.rgb *= 1 - ahead*0.14*live // the version that was, a little dimmer

	// the rim on the front, its colors split a little, in a glow; and the window's edges, in the same colors
	ang := atan2(dir.y, dir.x)
	tint := iris(ang*1.4 + s*0.002 + Time*0.3)
	rim := vec3(line(s, 3.5, 3.2), line(s, 1, 3.0), line(s, -1.5, 3.2))
	glow := line(s, 10, 30)*0.7 + band*0.28
	e := min(min(pos.x, Size.x-pos.x), min(pos.y, Size.y-pos.y))
	etint := iris((pos.x+pos.y)/(Size.x+Size.y)*1.5 + Time*0.25)
	frame := exp(-e/7) * sin(min(Prog/0.92, 1)*3.14159) * 0.85
	if Dark > 0.5 { // screen: it adds light, so it glows
		light := (rim*tint*1.3 + tint*glow*0.75 + etint*frame) * live
		return vec4(col.rgb+light*(col.a-col.rgb*0.5), col.a)
	}
	// light: the glow tints the window, the rim and the edges are thin lines of color
	out := mix(col.rgb, col.rgb*mix(vec3(1), tint, 0.8), min(glow, 1)*0.75*live)
	out = mix(out, tint*col.a, min(max(max(rim.r, rim.g), rim.b)*0.8*live, 1))
	out = mix(out, etint*col.a, min(frame*0.7*live, 1))
	return vec4(out, col.a)
}
`
