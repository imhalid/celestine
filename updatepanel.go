package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A new version comes as a small card at the top right, as a toast does, unfolding from the top down behind a lit
// edge (updPanelShader): the new version, a line, and update now or later (#379). Later closes it, and a chip
// unfolds in its place in the header's right end: glossy, in the accent's gradient as a button is, a light running
// over it now and then. A click on the chip updates where it is: it turns green, and a light runs along it while the
// version comes in. The dialog of "what's new", after an update, stays a dialog (updateui.go).

// white and black are the lights and shades the chip and the card are drawn with.
var (
	white = ui.RGB(255, 255, 255)
	black = ui.RGB(0, 0, 0)
)

// updShaders is false in tests: the tester draws on the CPU, where a shader over a panel takes seconds a frame.
var updShaders = !testing.Testing()

// The chip's light: one pass of updSweep, then updRest before the next.
const (
	updSweep = 1400 * time.Millisecond
	updRest  = 2600 * time.Millisecond
)

// updPanelOpen reports whether the panel is asked for: a version waits, or is on its way in, and it was not closed.
func (a *app) updPanelOpen() bool {
	return a.updOpen && !a.updNotes && (a.updRel != nil || a.updState != updIdle)
}

// updChipShown reports whether the chip is shown: a version waits, or is on its way in, and its card is closed.
func (a *app) updChipShown() bool {
	return !phone && (a.updRel != nil || a.updState != updIdle) && !a.updPanelOpen() && !a.updNotes
}

// The chip's green, while the version comes in and once it is in place: its own, for white words on it in light
// and in dark.
var (
	updGreen400 = ui.RGB(74, 205, 112)
	updGreen600 = ui.RGB(30, 146, 72)
)

// updateChip is the chip of a waiting version, in the header's right end, by the side panel's button. It unfolds
// as the card does once the card is closed. A click updates: the chip turns green where it is and a light runs
// along it while the version comes in; once it is in place, a click restarts.
func (a *app) updateChip(c *ui.Context) {
	if !a.updChipShown() {
		a.updChipAt = time.Time{}
		return
	}
	if a.updChipAt.IsZero() {
		a.updChipAt = c.Now()
	}
	since := c.Now().Sub(a.updChipAt)
	k := float32(min(since.Seconds()/updChipUnfold.Seconds(), 1))
	k = 1 - (1-k)*(1-k)*(1-k)
	if k < 1 {
		c.AnimationFrame()
	}
	cl := ryugo.From(c).Colors
	busy := a.updState == updDownloading
	label := locales.UpdateBadge()
	switch a.updState {
	case updDownloading:
		label = locales.UpdateDownloading()
	case updReady:
		label = locales.UpdateBadgeReady()
	}
	const h = 22
	b := ui.Row(c.Key("upd-chip")).Height(h).Padding(0, 10, 0, 7).Gap(5).AlignItems(ui.Center).Radius(h / 2).
		Role(ui.RoleButton).Label(label).Shrink(0)
	if !busy {
		b.Cursor(ui.CursorPointer)
	}
	// g is how green it is: 0 while it waits, 1 once the update is asked for, turning on a spring
	g := b.AnimateTo("green", map[bool]float32{true: 1}[a.updState != updIdle], springSheet)
	top, bottom := cl.ColorAccent400.Mix(updGreen400, g), cl.ColorAccent600.Mix(updGreen600, g)
	b.Shadow(0, 2, 8, -1, bottom.Alpha(0.45))
	hover := b.Hovered() && !busy
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		top, bottom := top, bottom
		if hover {
			top, bottom = top.Mix(white, 0.12), bottom.Mix(white, 0.08)
		}
		// the body: lighter at the top, as a button's face
		p.FillGradient(r, ui.LinearGradient{From: top, To: bottom, Angle: 180, Oklab: true}, h/2)
		// the gloss: a light on its upper half, as on glass
		gloss := ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H*0.5 - 1}
		p.FillGradient(gloss, ui.LinearGradient{From: white.Alpha(0.38), To: white.Alpha(0.04), Angle: 180}, h/2-1)
		p.Stroke(r, bottom.Mix(black, 0.25).Alpha(0.55), h/2, 1)
		p.Line(r.X+h/2, r.Y+1.5, r.X+r.W-h/2, r.Y+1.5, 1, white.Alpha(0.45)) // its top edge catches the light
		// the progress: a capsule of light going along it, as the size is not told yet (#323)
		if busy {
			t := float32(p.Now().Sub(a.updChipAt).Seconds()) / 1.1
			t -= float32(int(t))
			w := r.W * 0.4
			x0, x1 := max(r.X+(r.W+w)*t-w, r.X+1), min(r.X+(r.W+w)*t, r.X+r.W-1)
			if x1 > x0 {
				p.FillGradient(ui.Rect{X: x0, Y: r.Y + 2, W: x1 - x0, H: r.H - 4},
					ui.LinearGradient{From: white.Alpha(0.05), To: white.Alpha(0.42), Angle: 90}, h/2-2)
			}
			if updShaders {
				p.AnimationFrame()
			}
			return
		}
		// the light runs while it runs; between two passes the chip asks for no frame
		if updShaders && k >= 1 {
			if t := p.Now().Sub(a.updChipAt) % (updSweep + updRest); t < updSweep {
				p.AnimationFrame()
			} else {
				p.After(updSweep + updRest - t)
			}
		}
	})
	switch {
	case k < 1 && updShaders: // it unfolds from the top down, as the card did
		b.Disabled(true).DimDisabled(false)
		b.Effect(updPanelShader.Effect(shader.Options{
			Uniforms:        map[string]any{"Open": k, "Radius": float32(h / 2), "Time": float32(since.Seconds()), "Tone": bottom},
			MaxSampleOffset: ui.Point{X: 8, Y: 8},
		}))
	case k < 1:
		b.Disabled(true).DimDisabled(false).Opacity(k)
	case updShaders && !busy:
		t := since % (updSweep + updRest)
		b.Effect(updBadgeShader.Effect(shader.Options{Uniforms: map[string]any{
			"Head": float32(t.Seconds() / updSweep.Seconds()), "Radius": float32(h / 2)}}))
	}
	b.Children(func() {
		ui.Icon(c, iconUpdate).Width(13).Height(13).TextColor(cl.OnAccent)
		ui.Text(c, label).FontSize(11.5).FontWeight(680).TextColor(white).NoWrap()
	})
	if b.Clicked() {
		switch a.updState {
		case updIdle:
			a.updErr = ""
			a.startUpdate()
		case updReady:
			a.restartApp()
		}
	}
}

// updChipUnfold is how long the chip takes to open: a little quicker than the card, as it is smaller.
var updChipUnfold = 700 * time.Millisecond

// updBadgeShader runs a light over the chip: a slanted band of white from its left to past its right as Head goes
// from 0 to 1, brightest at its middle, and nothing past 1.
var updBadgeShader = shader.MustCompile(`package main

var Head float
var Radius float

func LayerEffect(pos vec2) vec4 {
	sz := rectSize()
	col := layerSample(pos)
	if Head >= 1 {
		return col
	}
	u := pos.x - (pos.y-sz.y*0.5)*0.9 // slanted
	head := -24 + (sz.x+48)*Head
	d := (u - head) / 9
	band := exp(-d*d) * 0.75
	tail := exp(-max(head-u, 0)/36) * step(u, head) * 0.12 // a little light left behind it
	light := (band + tail) * col.a
	return vec4(mix(col.rgb, vec3(col.a), light), col.a)
}
`)

// updateCard is the card of a waiting version, at the window's top right, as a toast is: its title, a line under it,
// and what to do. It unfolds from the top down behind a lit edge (updPanelShader) when it opens.
func (a *app) updateCard(c *ui.Context) {
	if !a.updPanelOpen() {
		a.updPanelAt = time.Time{}
		return
	}
	if a.updPanelAt.IsZero() {
		a.updPanelAt = c.Now()
	}
	k := float32(min(c.Now().Sub(a.updPanelAt).Seconds()/updUnfold.Seconds(), 1))
	k = 1 - (1-k)*(1-k)*(1-k) // out of the edge quickly, settling slowly
	if k < 1 {
		c.AnimationFrame()
	}
	cl := ryugo.From(c).Colors
	r := rad(12)
	ui.Overlay(c, func() {
		top := a.contentTop(c) + 12
		if a.wframe.on { // under the frame's top edge, over the card of the panes
			top = frameTopH + 10
		}
		card := ui.Column(c.Key("upd-card")).Absolute().Right(16).Top(top).Width(300).
			Radius(r).Background(cl.Card).Border(1, cl.Accent.Alpha(0.35)).Role(ui.RoleDialog).Label(a.updTitle()).
			Shadow(0, 8, 24, -4, cl.Accent.Alpha(0.30))
		if k < 1 {
			card.Disabled(true).DimDisabled(false)
			if updShaders {
				card.Effect(updPanelShader.Effect(shader.Options{
					Uniforms:        map[string]any{"Open": k, "Radius": r, "Time": float32(c.Now().Sub(a.updPanelAt).Seconds()), "Tone": cl.Accent},
					MaxSampleOffset: ui.Point{X: 8, Y: 8},
				}))
			} else {
				card.Opacity(k)
			}
		}
		card.Draw(func(p *ui.Painter, b ui.Rect) { // a light of the accent in its corner, as a toast has its tone's
			p.FillRadialGradient(b, ui.RadialGradient{Center: ui.Point{X: 0.001, Y: 0.001}, Radius: b.W * 0.8, Oklab: true,
				Stops: []ui.GradientStop{{Color: cl.Accent.Alpha(0.14), At: 0}, {Color: cl.Accent.Alpha(0), At: 1}}}, r)
		})
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Start).Padding(12, 10, 0, 12).Children(func() {
				a.updMark(c, cl)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, a.updTitle()).FontSize(13.5).FontWeight(680).TextColor(cl.Text).Wrap()
					ui.Text(c, a.updCardLine()).FontSize(12).TextColor(cl.Muted).Wrap()
				})
				if a.updState != updDownloading {
					x := ui.Box(c.Key("upd-card-close")).Width(22).Height(22).Radius(11).AlignItems(ui.Center).Justify(ui.Center).
						Role(ui.RoleButton).Label(locales.Close()).Cursor(ui.CursorPointer).Shrink(0)
					if x.Hovered() {
						x.Background(cl.Fill)
					}
					x.Children(func() { ui.Icon(c, iconClose).Width(13).Height(13).TextColor(cl.Muted) })
					if x.Clicked() {
						a.updOpen = false
					}
				}
			})
			ui.Row(c).FillWidth().Gap(8).Padding(10, 12, 12, 51).Children(func() { a.updCardActions(c, cl) })
		})
	})
}

// updUnfold is how long the card takes to open: long enough that its lit edge is seen running down.
var updUnfold = 900 * time.Millisecond

// updMark is the card's mark: a round badge of the accent's gradient, glossy as the chip, with the update's sign.
func (a *app) updMark(c *ui.Context, cl ryugo.Colors) {
	m := ui.Box(c.Key("upd-card-mark")).Width(28).Height(28).Radius(14).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Shadow(0, 2, 6, -1, cl.Accent.Alpha(0.45))
	m.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{From: cl.ColorAccent400, To: cl.ColorAccent600, Angle: 180, Oklab: true}, 14)
		p.FillGradient(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H*0.5 - 1}, ui.LinearGradient{From: white.Alpha(0.4), To: white.Alpha(0.04), Angle: 180}, 13)
	})
	m.Children(func() { ui.Icon(c, iconUpdate).Width(15).Height(15).TextColor(cl.OnAccent) })
}

// updTitle is the card's title: the version that waits.
func (a *app) updTitle() string {
	if a.updRel == nil {
		return locales.UpdateNewVersion()
	}
	return locales.UpdateVersionAvailable(shownVersion(a.updRel.Version))
}

// updCardLine is the line under the title: when the version came out, or where the update is.
func (a *app) updCardLine() string {
	switch {
	case a.updState == updDownloading:
		return locales.UpdateDownloading()
	case a.updState == updReady:
		return locales.UpdateBadgeReady()
	case a.updErr != "":
		return a.updErr
	case a.updRel != nil:
		return releasedLine(a.updRel.Date)
	}
	return ""
}

// updCardActions are what can be done: update now or later; while it downloads, how far; once in place, restart.
func (a *app) updCardActions(c *ui.Context, cl ryugo.Colors) {
	switch a.updState {
	case updDownloading:
		bar := ui.Box(c.Key("upd-card-bar")).Grow(1).Height(4).Radius(2).Background(cl.Well).Margin(6, 0, 6, 0)
		bar.Draw(func(p *ui.Painter, r ui.Rect) { // a light going along it: the size is not told yet (#323)
			t := float32(p.Now().Sub(a.updPanelAt).Seconds())
			x := (t*0.8 - float32(int(t*0.8))) * (r.W + 60)
			p.FillGradient(ui.Rect{X: r.X + x - 60, Y: r.Y, W: 60, H: r.H}, ui.LinearGradient{From: updGreen600.Alpha(0), To: updGreen400, Angle: 90}, 2) // green while it comes in
			if updShaders {
				p.AnimationFrame()
			}
		})
	case updReady:
		if ryugo.Button(c.Key("upd-card-restart"), locales.UpdateRestart(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
			a.restartApp()
		}
		if ryugo.Button(c.Key("upd-card-later2"), locales.UpdateLater(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			a.updOpen = false
		}
	default:
		if ryugo.Button(c.Key("upd-card-now"), locales.UpdateUpdateNow(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
			a.startUpdate()
		}
		if ryugo.Button(c.Key("upd-card-later"), locales.UpdateLater(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			a.updOpen = false // the chip unfolds in its place
		}
	}
}

// updPanelShader unfolds the card from the top down as Open goes from 0 to 1: above a wavering edge it is there,
// lit a moment as if just uncovered; below, nothing. The edge is a thin white line in a glow of the accent, with a few
// sparks thrown off it; at Open 1 the panel is as it is.
var updPanelShader = shader.MustCompile(`package main

var Open float
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

func band(d float, w float) float {
	t := d / w
	return exp(-t * t)
}

func LayerEffect(pos vec2) vec4 {
	sz := rectSize()
	tone := Tone.rgb
	waver := (noise(pos.x/14+Time*2.5)-0.5)*16 + (noise(pos.x/5-Time*4.0)-0.5)*5
	edge := -30 + (sz.y+60)*Open + waver
	y := pos.y

	shown := smoothstep(edge+2, edge-16, y) // above the edge
	col := layerSample(pos) * shown

	// just uncovered, the panel is lit, fading up from the edge
	above := max(edge-y, 0)
	lit := exp(-above/70) * shown * (1 - smoothstep(0.85, 1.0, Open))
	col.rgb += (col.rgb*0.45 + tone*0.25) * lit * col.a

	// the edge: a thin white line in the accent's glow, across the panel
	d := y - edge
	live := 1 - smoothstep(0.88, 1.0, Open)
	light := (vec3(1)*band(d, 2.0) + mix(tone, vec3(1), 0.35)*band(d, 14)*0.6 + tone*band(d+20, 50)*0.2) * live

	// sparks along the edge, thrown up behind it
	cell := floor(vec2(pos.x, y-edge) / vec2(7, 9))
	sp := step(0.94, hash(cell+floor(Time*14))) * band(d+12, 16) * live
	light += mix(tone, vec3(1), 0.6) * sp

	return col + vec4(light, max(max(light.r, light.g), light.b))*0.9
}
`)
