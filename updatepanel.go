package main

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A new version comes as a small card at the top right, as a toast does, unfolding from the top down behind a lit
// edge (updPanelShader): the new version, a line, and update now or later (#379). Later closes it, and a second later
// a chip opens from the left in its place in the header's right end: glossy, in the accent's gradient as a button is, a light running
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

// The chip's green while the version comes in: its own, for white words on it in light and in dark. Once the
// version is in place the chip is soft, as a soft button is: a step off the header toward the text.
var (
	updGreen400 = ui.RGB(74, 205, 112)
	updGreen600 = ui.RGB(30, 146, 72)
)

// updateChip is the chip of a waiting version, in the header's right end, by the side panel's button. A second after
// the card is closed it opens from the left, growing to its width behind a lit edge, and a second after that a light
// first runs over it. A click updates where it is: it turns green, a ring in place of its mark fills as the version
// comes in and the chip fills with it, smoothly; once the version is in place it is soft and says restart, and a
// click restarts.
func (a *app) updateChip(c *ui.Context) {
	if !a.updChipShown() {
		a.updChipAt = time.Time{}
		return
	}
	if a.updChipAt.IsZero() {
		a.updChipAt = c.Now()
	}
	// it takes its place in the header as the card closes, unseen, and a moment later opens there from the left
	since := c.Now().Sub(a.updChipAt) - updChipDelay
	k := float32(0)
	if fx.still { // reduced motion: it is there at once, and its light does not run
		since, k = updChipUnfold, 1
	} else if since < 0 {
		c.After(-since)
	} else {
		k = float32(min(since.Seconds()/updChipUnfold.Seconds(), 1))
		k = 1 - (1-k)*(1-k)*(1-k)
	}
	if since >= 0 && k < 1 {
		c.AnimationFrame()
	}
	const h = 22
	busy, ready := a.updState == updDownloading, a.updState == updReady
	label := locales.UpdateBadge()
	switch {
	case busy:
		label = locales.UpdateDownloading()
	case ready:
		label = locales.UpdateRestart()
	}
	b := ui.Row(c.Key("upd-chip")).Height(h).Padding(0, 10, 0, 6).Gap(5).AlignItems(ui.Center).Radius(h/2).
		Role(ui.RoleButton).Label(label).Shrink(0).Shadow(0, 1, 2, 0, black.Alpha(0.16))
	if !busy {
		b.Cursor(ui.CursorPointer)
	}
	// g is how green it is and q how gray, each turning on a spring as the update goes on
	g := b.AnimateTo("green", map[bool]float32{true: 1}[busy], springSheet)
	q := b.AnimateTo("gray", map[bool]float32{true: 1}[ready], springSheet)
	cl := ryugo.From(c).Colors
	t := c.Theme()
	soft := ryugo.Mix(t.Surface, t.Text, 0.09)
	top := cl.ColorAccent400.Mix(updGreen400, g).Mix(soft, q)
	bottom := cl.ColorAccent600.Mix(updGreen600, g).Mix(soft, q)
	ink := white.Mix(cl.Text2, q) // its words: white on the color, the text's on the soft
	hover := b.Hovered() && !busy
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		top, bottom := top, bottom
		// read as it is painted (the frames of the download paint and do not build the view), and followed smoothly: the
		// bytes come in steps
		f := a.updEase.follow(a.updProg.frac(), p.Now())
		if hover {
			top, bottom = top.Mix(white, 0.12), bottom.Mix(white, 0.08)
		}
		if busy { // the track: the green, dimmed; what has come in fills it from the left
			p.FillGradient(r, ui.LinearGradient{From: top.Mix(black, 0.32), To: bottom.Mix(black, 0.32), Angle: 180, Oklab: true}, h/2)
			// a pill of its own, not a rectangle clipped to the track, so its ends are as smooth as the chip's
			part := ui.Rect{X: r.X, Y: r.Y, W: max(r.W*f, h), H: r.H}
			if f < 0 { // the size is not told: a part of it runs along, inside the track
				t := float32(p.Now().Sub(a.updChipAt).Seconds()) / 1.2
				t -= float32(int(t))
				part = ui.Rect{X: r.X + (r.W-r.W*0.4)*t, Y: r.Y, W: r.W * 0.4, H: r.H}
			}
			if f != 0 {
				p.FillGradient(part, ui.LinearGradient{From: top, To: bottom, Angle: 180, Oklab: true}, h/2)
			}
			p.AnimationFrame() // the fill follows the download whatever is drawn with shaders
		} else { // the body: lighter at the top, as a button's face
			p.FillGradient(r, ui.LinearGradient{From: top, To: bottom, Angle: 180, Oklab: true}, h/2)
		}
		// the gloss: a light on its upper half, as on glass, following the pill and fading out with no edge of its own
		lit := 1 - q // the soft chip has no gloss, and a hairline for its edge
		p.Clip(r, h/2, func() {
			p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H * 0.55}, ui.LinearGradient{From: white.Alpha(0.3 * lit), To: white.Alpha(0), Angle: 180}, 0)
		})
		p.Stroke(r, bottom.Mix(black, 0.25).Alpha(0.55).Mix(cl.Hairline, q), h/2, 1)
	})
	switch {
	case k < 1 && updShaders && shadersOn(): // it opens from the left, behind a lit edge with a glow trailing it
		b.Disabled(true).DimDisabled(false)
		b.Effect(updChipShader.Effect(shader.Options{Uniforms: map[string]any{"Open": k}}))
	case k < 1:
		b.Disabled(true).DimDisabled(false).Opacity(k)
	case updShaders && shadersOn() && !busy && !ready:
		// the light runs while it waits. Its place is a uniform of the effect, set as the view is built, so a pass asks
		// for frames that build it (a painter's frame would paint the light where it was); between two passes the chip
		// asks for none.
		t := since - updChipUnfold - updShimmerDelay
		if t < 0 { // the first pass comes a moment after the chip opened
			c.After(-t)
			break
		}
		t %= updSweep + updRest
		if t < updSweep {
			c.AnimationFrame()
		} else {
			c.After(updSweep + updRest - t)
		}
		b.Effect(updBadgeShader.Effect(shader.Options{Uniforms: map[string]any{
			"Head": float32(t.Seconds() / updSweep.Seconds()), "Radius": float32(h / 2)}}))
	}
	b.Children(func() {
		// its top edge catches the light, and its bottom a shade, as soft shadows inside it rather than a drawn line;
		// on a layer of its own over what the chip draws
		ui.Box(c.Key("upd-chip-lit")).Absolute().Left(0).Right(0).Top(0).Bottom(0).Radius(h/2).PassThrough().
			InsetShadow(0, 1.5, 2, 0, white.Alpha(0.42*(1-q))).InsetShadow(0, -1, 2, 0, black.Alpha(0.12*(1-q)))
		if busy {
			a.updRing(c)
		} else {
			ic := iconUpdate
			if ready {
				ic = iconRestart
			}
			ui.Icon(c, ic).Width(13).Height(13).TextColor(ink)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(680).TextColor(ink).NoWrap()
	})
	if !busy {
		a.updEase = updEase{} // the next download's fill starts from nothing
	}
	if b.Clicked() {
		switch a.updState {
		case updIdle:
			a.updErr = ""
			a.updProg.got.Store(0)
			a.updProg.size.Store(0)
			a.startUpdate()
		case updReady:
			a.restartApp()
		}
	}
}

// An updEase is the chip's fill as it is drawn: it follows how far the download is, closing most of the way in a
// fifth of a second, so that the bytes coming in steps fill it smoothly.
type updEase struct {
	v  float32
	at time.Time
}

// follow moves the fill toward f, as it is at now, and gives it; f < 0 (the size is not told) is given as it is.
func (e *updEase) follow(f float32, now time.Time) float32 {
	if f < 0 {
		return f
	}
	if !e.at.IsZero() {
		dt := float32(now.Sub(e.at).Seconds())
		e.v += (f - e.v) * (1 - float32(math.Exp(float64(-dt/0.08))))
	}
	e.at = now
	return e.v
}

// updRing is the chip's mark while the version comes in: a small ring that fills from the top as the download goes
// to its end, or a part of it turning while the size is not told. How far it is is read as it is painted, as the
// chip's fill is.
func (a *app) updRing(c *ui.Context) {
	ring := ui.Box(c.Key("upd-ring")).Width(13).Height(13).Shrink(0)
	ring.Draw(func(p *ui.Painter, r ui.Rect) {
		f := a.updEase.v // as the chip's fill is drawn, a moment before
		if a.updProg.frac() < 0 {
			f = -1
		}
		cx, cy, rr := r.X+r.W/2, r.Y+r.H/2, r.W/2-1.5
		p.StrokePath((&ui.Path{}).Circle(cx, cy, rr), 2, white.Alpha(0.3))
		from, to := float32(0), f
		if f < 0 {
			t := float32(p.Now().Sub(a.updChipAt).Seconds()) * 1.4
			from = t - float32(int(t))
			to = from + 0.3
		}
		arc := ringFromTop(cx, cy, rr)
		if to > 1 { // past the top: the part from the start too
			p.StrokePath(ringFromTop(cx, cy, rr).Trim(0, to-1), 2, white)
			to = 1
		}
		if to > from {
			p.StrokePath(arc.Trim(from, to), 2, white)
		}
	})
}

// ringFromTop is a circle that starts at its top and goes clockwise, so a trim of it fills as a clock's hand goes.
func ringFromTop(cx, cy, r float32) *ui.Path {
	const k = 0.5522847498
	p := &ui.Path{}
	p.MoveTo(cx, cy-r)
	p.CubeTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy)
	p.CubeTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r)
	p.CubeTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy)
	p.CubeTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r)
	return p
}

// updChipShader opens the chip from the left as Open goes from 0 to 1: left of a thin white edge it is there, with a
// glow of the edge's light trailing it; right of it, nothing. Nothing is drawn past the chip itself.
var updChipShader = shader.MustCompile(`package main

var Open float

func LayerEffect(pos vec2) vec4 {
	sz := rectSize()
	col := layerSample(pos)
	edge := -6 + (sz.x+12)*Open
	shown := smoothstep(edge+2, edge-2, pos.x)
	end := 1 - smoothstep(0.8, 1.0, Open) // the light goes as it settles
	d := (pos.x - edge) / 1.8
	line := exp(-d*d) * 0.9 * end * col.a
	glow := exp(-max(edge-pos.x, 0)/16) * 0.4 * end * shown * col.a
	out := col * shown
	return vec4(out.rgb+vec3(line+glow), max(out.a, line))
}
`)

// updChipDelay is how long after the card closes the chip comes, updChipUnfold how long it takes to open (a little
// quicker than the card, as it is smaller), and updShimmerDelay how long after it opened its light first runs.
var (
	updChipDelay    = time.Second
	updChipUnfold   = 700 * time.Millisecond
	updShimmerDelay = time.Second
)

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
	head := -40 + (sz.x+80)*Head
	d := (u - head) / 15
	band := exp(-d*d) * 0.55
	tail := exp(-max(head-u, 0)/36) * step(u, head) * 0.12 // a little light left behind it
	light := (band + tail) * col.a
	return vec4(mix(col.rgb, vec3(col.a), light), col.a)
}
`)

// updateCard is the card of a waiting version, at the window's top right, as a toast is: its title, a line under it,
// and what to do. It unfolds from the top down behind a lit edge (updPanelShader) when it opens. While the version
// comes in, a bar fills with it and says how far; once it is in place, the card is ready: green, and restart.
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
	if fx.still {             // reduced motion: it is there at once
		k = 1
	}
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
		card := ui.Column(c.Key("upd-card")).Absolute().Right(16).Top(top).Width(updCardW).
			// a faint tint and a light border of the accent: a notice, set apart from the content under it (#398)
			Radius(r).Background(ryugo.Mix(cl.Card, cl.Accent, 0.06)).Border(1, cl.Accent.Alpha(0.28)).Clip().Role(ui.RoleDialog).Label(a.updTitle()).
			Shadow(0, 2, 6, 0, black.Alpha(0.10)) // small and plain: inside what the unfolding effect draws, no colored glow
		if k < 1 {
			card.Disabled(true).DimDisabled(false)
			if updShaders && shadersOn() {
				card.Effect(updPanelShader.Effect(shader.Options{
					Uniforms:        map[string]any{"Open": k, "Radius": r, "Time": float32(c.Now().Sub(a.updPanelAt).Seconds()), "Tone": cl.Accent},
					MaxSampleOffset: ui.Point{X: 8, Y: 8},
				}))
			} else {
				card.Opacity(k)
			}
		}
		card.Children(func() { a.updCardBody(c, cl, a.updState, "upd-card") })
	})
}

// updCardW is the card's width.
const updCardW = 300

// updCardBody is what the card holds, as it is in state st: its mark, title and line, and what can be done. key names
// its elements.
func (a *app) updCardBody(c *ui.Context, cl ryugo.Colors, st updateState, key string) {
	ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Start).Padding(12, 10, 0, 12).Children(func() {
		a.updMark(c, cl, st, key)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, a.updTitle()).FontSize(13.5).FontWeight(680).TextColor(cl.Text).Wrap()
			ui.Text(c, a.updCardLine(st)).FontSize(12).TextColor(cl.Muted).Wrap()
		})
		// the close button keeps its place while it downloads, unseen, so the card does not move as its state changes
		x := ui.Box(c.Key(key + "-close")).Width(22).Height(22).Radius(11).AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
		if st == updDownloading {
			x.Opacity(0).Disabled(true)
		} else {
			x.Role(ui.RoleButton).Label(locales.Close()).Cursor(ui.CursorPointer)
			if x.Hovered() {
				x.Background(cl.Fill)
			}
			if x.Clicked() {
				a.updOpen = false
			}
		}
		x.Children(func() { ui.Icon(c, iconClose).Width(13).Height(13).TextColor(cl.Muted) })
	})
	ui.Row(c).FillWidth().Padding(10, 12, 12, 12).Children(func() { // as tall in each state, so the wave's copy fits
		ui.Row(c).Grow(1).Height(28).Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			a.updCardActions(c, cl, st, key)
		})
	})
}

// updUnfold is how long the card takes to open: long enough that its lit edge is seen running down.
var updUnfold = 900 * time.Millisecond

// updMark is the card's mark: a round badge, glossy as the chip, with the update's sign; green with the restart's once
// the version is in place.
func (a *app) updMark(c *ui.Context, cl ryugo.Colors, st updateState, key string) {
	from, to, ic := cl.ColorAccent400, cl.ColorAccent600, iconUpdate
	if st == updReady {
		from, to, ic = updGreen400, updGreen600, iconRestart
	}
	m := ui.Box(c.Key(key + "-mark")).Width(28).Height(28).Radius(14).AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
	m.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{From: from, To: to, Angle: 180, Oklab: true}, 14)
		p.Clip(r, 14, func() { // the gloss on its upper half follows the circle and fades out, with no edge of its own
			p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H * 0.55}, ui.LinearGradient{From: white.Alpha(0.3), To: white.Alpha(0), Angle: 180}, 0)
		})
	})
	m.Children(func() { ui.Icon(c, ic).Width(15).Height(15).TextColor(white) })
}

// updTitle is the card's title: the version that waits.
func (a *app) updTitle() string {
	if a.updRel == nil {
		return locales.UpdateNewVersion()
	}
	return locales.UpdateVersionAvailable(shownVersion(a.updRel.Version))
}

// updCardLine is the line under the title in state st: when the version came out, or where the update is.
func (a *app) updCardLine(st updateState) string {
	switch {
	case st == updDownloading:
		return locales.UpdateDownloading()
	case st == updReady:
		return locales.UpdateBadgeReady()
	case a.updErr != "":
		return a.updErr
	case a.updRel != nil:
		return releasedLine(a.updRel.Date)
	}
	return ""
}

// updCardActions are what can be done in state st: update now or later; while it downloads, how far; once in place,
// restart.
func (a *app) updCardActions(c *ui.Context, cl ryugo.Colors, st updateState, key string) {
	switch st {
	case updDownloading:
		a.updBar(c, cl, key)
	case updReady:
		if ryugo.Button(c.Key(key+"-restart"), locales.UpdateRestart(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
			a.restartApp()
		}
		if ryugo.Button(c.Key(key+"-later2"), locales.UpdateLater(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			a.updOpen = false
		}
	default:
		if ryugo.Button(c.Key(key+"-now"), locales.UpdateUpdateNow(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
			a.startUpdate()
		}
		if ryugo.Button(c.Key(key+"-later"), locales.UpdateLater(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			a.updOpen = false // the chip opens in its place
		}
	}
}

// updBar is the card's bar while the version comes in: a well, as Ryugo's Progress has, filling on a spring, deep in
// the accent at its start and lighter toward its end, and how far it is beside it. Its size not told yet, the well is
// full and pale. It asks for frames while it shows: the percentage and the fill are built each frame.
func (a *app) updBar(c *ui.Context, cl ryugo.Colors, key string) {
	f := a.updProg.frac()
	if a.updState == updReady { // the card's download at its end, outside the wave
		f = 1
	}
	c.AnimationFrame()
	const h = 6
	well := ui.Box(c.Key(key + "-bar")).Grow(1).Height(h).Radius(h / 2).Background(cl.Well).Clip().Role(ui.RoleProgress).
		Label(locales.UpdateDownloading())
	if f >= 0 {
		well.Range(0, 1, float64(f))
	}
	light := cl.ColorAccent400.Mix(white, 0.7) // the fill's color at its end
	well.Children(func() {
		fill := ui.Box(c.Key(key + "-fill")).Absolute().Top(0).Bottom(0).Left(0).Radius(h / 2)
		v := float32(1)
		if f < 0 {
			fill.Background(light.Alpha(0.5))
		} else {
			v = fill.AnimateTo("v", f, springSheet)
			fill.LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true,
				Stops: []ui.GradientStop{{Color: cl.ColorAccent600, At: 0}, {Color: cl.ColorAccent400, At: 0.55}, {Color: light, At: 1}}})
		}
		fill.WidthPercent(100 * min(max(v, 0), 1))
		if v > 0.001 {
			fill.MinWidth(h)
		}

	})
	pct := ""
	if f >= 0 {
		pct = strconv.Itoa(int(f*100)) + "%"
	}
	ui.Text(c, pct).FontSize(12).FontWeight(600).TextColor(cl.Text2).FontFeatures("tnum").Width(36).TextAlign(ui.End).NoWrap()
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
