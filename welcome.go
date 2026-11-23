package main

import (
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The first opening. There is no window to be seen: the desktop itself darkens, most at its edges, and a mesh of
// colors comes up slowly in the middle of it; the words of the app rise into it one after another, from below; when the last is there a button
// says "Başla". Pressed, the app's own window opens out of the button, a ring of light running over it (a
// shader on the whole window, reveal below), and the colors go.
//
// On macOS that is three windows (a first run, or CELESTINE_WELCOME=1): the stage, frameless and clear, over the
// whole display, which has the colors and the words; the app's window, built all along but hidden; and, for
// the opening only, a frameless clear window just over the app's, in which the app is drawn inside the ring.
// The app's window has its traffic lights from the moment it is made, and nothing takes them away, so it is
// not what the ring opens: it is shown unseen (no opacity) under the opening's window, and when the ring has
// run over all of it, it is seen, lights, shadow and all, and the opening's window goes. Started again from the
// settings it is the same. On Linux and Windows the welcome is drawn in the app's window, on a plain ground.

// welcomeShaders is false in the tests, whose CPU renderer runs a shader far too slowly to step through a
// few seconds of frames: they check where the welcome is, not how it looks (that is looked at in the app).
var welcomeShaders = true

// welcomeState is where the welcome is.
type welcomeState struct {
	on       bool      // it is up: the intro or the opening
	start    time.Time // the first frame of it
	goAt     time.Time // when Başla was pressed; zero until then
	origin   ui.Point  // where the opening runs out from: the button
	win      *mygo.Window
	stage    *mygo.Window // the clear window over the display that has the colors and the words (clear only)
	opening  *mygo.Window // the frameless window the app opens in, over its own (clear only)
	inOpen   bool         // the frame being built is the opening window's
	opened   bool         // the opening window has drawn its first frame: the opening's clock runs from then
	chosen   bool         // the welcome has started: the ground comes, and then the opening, by itself
	chosenAt time.Time    // when
	sounded  bool         // soundWelcome has already played for this welcome session
	bar      ui.TitleBar  // the app's window's title bar, which the opening window has not (titleBar)
	clear    bool         // the welcome is on the stage, and the app's window waits unseen until it has opened
}

const (
	welVeil   = 0.8  // the desktop darkens over this long
	welMeshAt = 0.25 // and the ground's colors come in from here, over welMesh
	welMesh   = 1.2
	welPause  = 0.0  // the ground starts at once
	welHead   = 0.60 // then the words rise in, and Başla
	welButton = 0.90
	welOpen   = 2.1 // the opening takes this long, in seconds, unless its look says otherwise
	welSwap   = 0.6 // and the app's window takes the opening's place over this long
	welCorner = 9   // the opening window's corners, as the app's window has them (macOS)
)

// wantWelcome: the first run of a library that keeps its settings, or when asked (CELESTINE_WELCOME=1, or 0 for never).
func (a *app) wantWelcome() bool {
	switch os.Getenv("CELESTINE_WELCOME") {
	case "1":
		return true
	case "0": // a run without it, whatever the settings say
		return false
	}
	return !a.settings.WelcomeDone && (a.settingsFile != "" || a.settingsStore != nil)
}

// welcomeAgain is the command, and the button of the settings: the welcome, once more. On macOS it is the first run's: the app's
// window goes, and the colors come onto the desktop; elsewhere it sits in the window, on a ground.
func (a *app) welcomeAgain() {
	a.closeSettings()
	a.wel = welcomeState{on: true, win: a.wel.win}
	a.settings.WelcomeDone = false
	if runtime.GOOS == "darwin" && a.wel.win != nil {
		a.wel.clear = true
		a.wel.win.SetHasShadow(false)
		a.wel.win.Hide()
		a.openStage()
	}
}

// welcomeUp: the welcome has the window.
func (a *app) welcomeUp() bool { return a.wel.on }

// ---- the shaders ----

// ---- the frames ----

// welcomeClock is how far the intro is, in seconds; the first frame starts it.
func (a *app) welcomeClock(c *ui.Context) float64 {
	if a.wel.start.IsZero() {
		a.wel.start = c.Now()
	}
	return c.Now().Sub(a.wel.start).Seconds()
}

// welcomeProgress is how far the opening is, 0 until Başla, then to 1.
func (a *app) welcomeProgress(c *ui.Context) float64 { return a.welcomeProgressAt(c.Now()) }

// welcomeProgressAt is how far the opening is at now: what a paint asks, with the time of its own frame.
func (a *app) welcomeProgressAt(now time.Time) float64 {
	if a.wel.goAt.IsZero() {
		return 0
	}
	if fx.still { // reduced motion: the window is there at once
		return 1
	}
	o := a.welcomeOpeningLook()
	return o.progress(now.Sub(a.wel.goAt).Seconds() / o.length())
}

// openStage puts up the stage: a window over the whole display the app's window is on, with no frame, no
// shadow and no ground, so that only what the welcome draws is seen on the desktop.
func (a *app) openStage() {
	d := mygo.Screen.DisplayMatching(a.wel.win.Bounds()).Bounds
	opts := mygo.WindowOptions{Title: "celestine", X: d.X, Y: d.Y, Width: d.Width, Height: d.Height,
		Frameless: true, Transparent: true, DisableShadow: true, DisableResize: true, DisableMove: true,
		DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, Content: ui.View(a.stageView)}
	st := mygo.NewWindow(opts)
	st.SetBounds(d) // a centered window would be kept off the menu bar
	st.Focus()
	a.wel.stage = st
}

// welcomeContinue starts the welcome: the ground comes, and then the opening, by itself (welcomeView). A ground that
// blurs the desktop has its dark material only now, brought in slowly with the window's opacity.
func (a *app) welcomeContinue(c *ui.Context) {
	if !a.wel.sounded {
		a.wel.sounded = true
		a.playSound(soundWelcome)
	}
	if a.wel.chosen {
		return
	}
	a.wel.chosen, a.wel.chosenAt = true, c.Now()
	a.wel.start = c.Now().Add(time.Duration(welPause * float64(time.Second))) // the ground's clock
	if st := a.wel.stage; a.wel.clear && st != nil && a.welcomeGroundLook().blur {
		time.AfterFunc(time.Duration(welPause*float64(time.Second)), func() {
			st.SetOpacity(0.01)
			st.SetVibrancy(mygo.VibrancyHUD)
			a.fadeWindow(st, 0, 1, 0, welVeil)
		})
	}
	c.AnimationFrame()
}

// welcomeSkip is Esc: no welcome, the app's window at once, faded in as on any start.
func (a *app) welcomeSkip(c *ui.Context) {
	a.wel.on = false
	a.welcomeSeen()
	if a.wel.clear && a.wel.win != nil {
		w := a.wel.win
		w.SetHasShadow(true)
		w.SetOpacity(0)
		w.Show()
		w.Focus()
		w.Invalidate()
		a.fadeWindow(w, 0, 1, 0, 0.25)
		if st := a.wel.stage; st != nil {
			st.Close()
		}
		a.wel.stage, a.wel.clear = nil, false
	}
	c.Invalidate()
}

// welcomeSeen keeps that the welcome was seen, so that it is not shown again; one asked for by the variable is a
// look, not a first run.
func (a *app) welcomeSeen() {
	if os.Getenv("CELESTINE_WELCOME") != "1" {
		a.settings.WelcomeDone = true
		a.saveSettings()
	}
}

// fadeWindow takes the opacity of w from one value to another, eased, after delay seconds, over dur, and then does
// then (if any): a window's material, and a window as a whole, cannot fade by themselves.
func (a *app) fadeWindow(w *mygo.Window, from, to, delay, dur float64, then ...func()) {
	go func() {
		defer a.guard()
		time.Sleep(time.Duration(delay * float64(time.Second)))
		start := time.Now()
		for {
			t := time.Since(start).Seconds() / dur
			if w.IsDestroyed() {
				return
			}
			w.SetOpacity(from + (to-from)*smoothstep(t))
			if t >= 1 {
				for _, f := range then {
					f()
				}
				return
			}
			time.Sleep(16 * time.Millisecond)
		}
	}()
}

// onlyOnce is fn for the first call and nothing for the later ones. Unlike sync.Once, a later call does not wait for
// the first to finish: fn may wait for the main thread (showing a window does), and a call made on the main thread
// meanwhile would hold it, each waiting on the other (#567).
func onlyOnce(fn func()) func() {
	var called atomic.Bool
	return func() {
		if called.CompareAndSwap(false, true) {
			fn()
		}
	}
}

// stageView is the stage's frame: the colors and the words, and nothing under them.
func (a *app) stageView(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.frameTheme(c)
	c.Root().Background(ui.Transparent)
	a.welcomeBackdrop(c, true)
	a.welcomeView(c, true)
}

// welcomeBackdrop draws the ground (the veil and the colors): in the app's window built before its own root, so that it is under it.
// onStage is the stage's frame; each is drawn in one window only.
func (a *app) welcomeBackdrop(c *ui.Context, onStage bool) {
	if !a.wel.on || a.wel.clear != onStage {
		return
	}
	a.welcomeClock(c) // its first frame starts the clock
	tok := ryugo.From(c)
	clear := c.Vibrancy() || onStage
	if clear {
		c.Root().Background(ui.Transparent) // the desktop is the ground
	}
	if !a.wel.chosen { // the window asks first: the ground comes with Devam
		return
	}
	base := tok.Colors.Card // no glass (Linux, Windows): the colors sit on the app's ground
	baseV := []float32{float32(base.R) / 255, float32(base.G) / 255, float32(base.B) / 255, 1}
	if onStage { // the desktop as it is: only the veil and the colors on it
		baseV = []float32{0, 0, 0, 0}
	} else if clear { // a veil only, so that the colors and the words keep their contrast over a bright desktop
		if tok.Dark {
			baseV = []float32{0, 0, 0, 0.16}
		} else {
			baseV = []float32{0.16, 0.16, 0.16, 0.16}
		}
	}
	dark, inWin := float32(0), float32(0)
	if tok.Dark {
		dark = 1
	}
	if !clear { // the app's own window, with no desktop under it
		inWin = 1
	}
	// while the app opens, the ground keeps to outside the ring that shows it; on the stage the app's window
	// is over it, and it goes as the window opens
	open, radius, prog := float32(0), float32(0), float32(0)
	ring := a.welcomeOpeningLook().ring
	if !onStage && ring && !a.wel.goAt.IsZero() {
		p := a.welcomeProgress(c)
		open, radius, prog = 1, float32(a.welcomeRadius(c, p)), float32(p)
	}
	win := []float32{0, 0, 0, 0} // the app's window, in the ground's window: on the stage, where it will open
	if onStage && a.wel.stage != nil && a.wel.win != nil {
		sb, wb := a.wel.stage.ContentBounds(), a.wel.win.ContentBounds()
		win = []float32{float32(wb.X - sb.X), float32(wb.Y - sb.Y), float32(wb.Width), float32(wb.Height)}
	}
	ui.Box(c.Key("welcome-ground")).Absolute().Fill().Draw(func(p *ui.Painter, r ui.Rect) {
		if win[2] == 0 { // the app's own window: all of it
			win = []float32{0, 0, r.W, r.H}
		}
		if !welcomeShaders || !shadersOn() {
			return
		}
		// the steps are timed by the paint's own frame: the view is built again only now and then (welcomeView)
		t := p.Now().Sub(a.wel.start).Seconds()
		veil, mesh, fade := smoothstep(t/welVeil), smoothstep((t-welMeshAt)/welMesh), 1.0
		if onStage || !ring { // the app's window is over it, or the opening has no ring to keep it outside of
			fade = 1 - smoothstep((a.welcomeProgressAt(p.Now())-0.15)/0.85)
		}
		a.welcomeGroundLook().sh.Draw(p, r, shader.Options{Uniforms: map[string]any{
			"Time": float32(t), "Fade": float32(fade), "Veil": float32(veil), "Mesh": float32(mesh), "Dark": dark, "InWin": inWin,
			"Open": open, "Origin": []float32{a.wel.origin.X, a.wel.origin.Y}, "Radius": radius, "Prog": prog,
			"Base": []float32{baseV[0] * baseV[3], baseV[1] * baseV[3], baseV[2] * baseV[3], baseV[3]},
			"Win":  win,
		}})
		p.AnimationFrame()
	})
}

// welcomeRoot hides the app until Başla, and then opens it with the shader.
func (a *app) welcomeRoot(c *ui.Context, root ui.Element) {
	if !a.wel.on {
		return
	}
	if a.wel.clear && !a.wel.inOpen { // the app's own window: hidden, then itself, unseen under the opening
		if a.wel.goAt.IsZero() {
			root.Hidden(true)
		}
		return
	}
	if a.wel.clear { // the opening window is clear outside the ring
		c.Root().Background(ui.Transparent)
	}
	if a.wel.goAt.IsZero() {
		root.Hidden(true)
		return
	}
	prog := a.welcomeProgress(c)
	if prog >= 1 {
		a.welcomeEnd(c)
		return
	}
	o := a.wel.origin
	radius := a.welcomeRadius(c, prog)
	if welcomeShaders && shadersOn() {
		w, h := c.Size()
		look := a.welcomeOpeningLook()
		root.Effect(look.sh.Effect(shader.Options{
			Uniforms: map[string]any{"Origin": []float32{o.X, o.Y}, "Radius": float32(radius), "Prog": float32(prog), "Size": []float32{w, h},
				"Time": float32(c.Now().Sub(a.wel.goAt).Seconds())},
			MaxSampleOffset: ui.Point{X: look.maxOffset, Y: look.maxOffset},
		}))
	}
	c.AnimationFrame()
}

// welcomeRadius is how far the ring has run: past the farthest corner at the end, with the width of the edge.
func (a *app) welcomeRadius(c *ui.Context, prog float64) float64 {
	w, h := c.Size()
	o := a.wel.origin
	far := math.Hypot(math.Max(float64(o.X), float64(w)-float64(o.X)), math.Max(float64(o.Y), float64(h)-float64(o.Y)))
	return prog * (far + 160)
}

// welcomeEnd: the app is all there. The welcome is done and not shown again; the glass goes.
func (a *app) welcomeEnd(c *ui.Context) {
	a.wel.on = false
	a.welcomeSeen()
	if a.wel.clear && a.wel.win != nil {
		// The app's own window takes the opening's place without anything coming all at once. It has the focus first,
		// while it is still unseen: it comes in front of the opening's window, and its traffic lights are in color
		// already (a window without the focus has them grey). Then it fades in over that window, which shows the
		// same, so that only what is its own - the traffic lights and the shadow - is seen coming; and the opening's
		// window, under it, is closed.
		w, op, st := a.wel.win, a.wel.opening, a.wel.stage
		a.wel.opening, a.wel.stage, a.wel.clear = nil, nil, false
		w.SetHasShadow(true)
		w.Focus()
		w.Invalidate()
		a.fadeWindow(w, 0, 1, 0, welSwap, func() {
			if op != nil {
				op.Close()
			}
		})
		if st != nil {
			a.fadeWindow(st, st.Opacity(), 0, 0, welSwap, st.Close)
		}
	}
	for _, w := range []**mygo.Window{&a.wel.opening, &a.wel.stage} {
		if *w != nil {
			(*w).Close()
			*w = nil
		}
	}
	c.Invalidate()
}

// welcomeStart: Başla (or Esc, or Enter), from where the button is. On the stage that is where it is in the
// app's window, which comes up now and opens.
func (a *app) welcomeStart(c *ui.Context, from ui.Rect) {
	if !a.wel.goAt.IsZero() {
		return
	}
	w, h := c.Size()
	if from.W == 0 { // the button has not been laid out: the middle, low
		from = ui.Rect{X: w/2 - 1, Y: h*0.72 - 1, W: 2, H: 2}
	}
	a.wel.origin = ui.Point{X: from.X + from.W/2, Y: from.Y + from.H/2}
	a.wel.goAt = c.Now()
	if a.wel.clear && a.wel.stage != nil && a.wel.win != nil {
		s, w := a.wel.stage.ContentBounds(), a.wel.win.ContentBounds()
		a.wel.origin.X += float32(s.X - w.X)
		a.wel.origin.Y += float32(s.Y - w.Y)
		a.wel.win.SetOpacity(0) // drawn, ready, and not seen until the ring has run
		a.wel.win.Show()
		if a.welcomeGroundLook().blur { // the blur goes as the window comes
			n := a.welcomeOpeningLook().length()
			a.fadeWindow(a.wel.stage, 1, 0, 0.15*n, 0.85*n)
		}
		op := mygo.NewWindow(mygo.WindowOptions{Title: "celestine", X: w.X, Y: w.Y, Width: w.Width, Height: w.Height,
			Frameless: true, Transparent: true, DisableShadow: true, DisableResize: true, CornerRadius: welCorner,
			Content: ui.View(a.openingView)})
		op.SetContentBounds(w)
		op.Focus()
		a.wel.opening = op
	}
	c.AnimationFrame()
}

// titleBar is the window's title bar, where the header keeps clear of the traffic lights. The opening window
// is frameless and has none: it is given the app's window's, so that nothing moves when the one takes the
// other's place.
func (a *app) titleBar(c *ui.Context) ui.TitleBar {
	if a.wel.inOpen {
		return a.wel.bar
	}
	tb := c.TitleBar()
	if a.wel.clear {
		a.wel.bar = tb
	}
	return tb
}

// openingView is the opening window's frame: the app, as in its own window, inside the ring.
func (a *app) openingView(c *ui.Context) {
	if !a.wel.opened { // the window took a moment to come: the opening starts now, so that none of it is missed
		a.wel.opened, a.wel.goAt = true, c.Now()
	}
	a.wel.inOpen = true
	defer func() { a.wel.inOpen = false }()
	a.view(c)
}

// welcomeLogo is the app's icon for the welcome, read from its bytes once.
var welcomeLogo = sync.OnceValue(func() ui.ImageSource {
	img, err := ui.DecodeImage(appIcon)
	if err != nil {
		return nil
	}
	return img
})

// welcomeView is the app's logo and its name (the same in every language) and Başla, over the ground, and nothing
// else, which keeps the welcome quiet (#507): the ground starts on the first frame, and the words and the button rise in one after another as it
// comes. Başla (or Enter) starts the opening from the button; Esc goes straight to the app. They lift off as the
// app opens.
func (a *app) welcomeView(c *ui.Context, onStage bool) {
	if !a.wel.on || a.wel.clear != onStage {
		return
	}
	a.welcomeClock(c)
	if !a.wel.chosen {
		a.welcomeContinue(c)
	}
	el := c.Now().Sub(a.wel.chosenAt).Seconds()
	prog := a.welcomeProgress(c)
	going := !a.wel.goAt.IsZero()
	if going { // lifting off: a frame each frame, the app's window has its own
		c.AnimationFrame()
	} else if el < welButton+1.0 {
		c.After(16 * time.Millisecond)
	}
	rise := ui.Animation{Spring: ui.SpringOptions{Duration: 700 * time.Millisecond, Bounce: 0.12}}
	rises := func(e ui.Element, at float64) ui.Element {
		pose := ui.Pose{Y: 36, Fade: 1}
		if el >= at {
			pose = ui.Pose{}
		}
		return e.AnimatePose(pose, rise)
	}
	w, h := c.Size()
	var button ui.Element
	start := func() {
		if !going && button.Bounds().W > 0 {
			a.welcomeStart(c, button.Bounds())
		}
	}
	ui.Overlay(c, func() {
		stage := ui.Column(c.Key("welcome")).Absolute().Left(0).Top(0).Width(w).Height(h).AlignItems(ui.Center).Justify(ui.Center)
		stage.Opacity(float32(1-min(1, prog*3))).Translate(0, float32(-24*prog))
		stage.Children(func() {
			if logo := welcomeLogo(); logo != nil {
				rises(ui.Image(c.Key("w-logo"), logo).Size(112, 112), welHead)
				ui.Box(c.Key("w-gap1")).Height(4)
			}
			rises(ui.Text(c.Key("w-head"), "Celestine").FontSize(40).FontWeight(720).TextColor(ryugo.From(c).Colors.Text).TextAlign(ui.Center), welHead+0.08)
			ui.Box(c.Key("w-gap2")).Height(10) // Başla's box has its glow's room round it
			glow, b := a.welcomeButton(c)
			rises(glow, welButton)
			button = b
		})
	})
	if going {
		return
	}
	if el >= welButton && c.Shortcut(0, ui.KeyEnter) {
		start()
	}
	if c.Shortcut(0, ui.KeyEscape) {
		a.welcomeSkip(c)
	}
	if button.Clicked() {
		start()
	}
}

// welcomeButton is Başla: a pill of the accent's color, lighter at its top, a thin light running round its edge and a
// glow under it that breathes; brighter under the pointer, pressed in when pressed. The glow comes in slowly once
// the pill has risen, spreading as it brightens. The pill and its glow are drawn by a shader (welcomeButtonShader) on
// a box larger than the pill, for the glow's room; the button - what is pressed, and has the pointer - is the pill
// alone, in the middle of it.
func (a *app) welcomeButton(c *ui.Context) (glow, button ui.Element) {
	cl := ryugo.From(c).Colors
	const pad = 56 // the glow's room round the pill: it is gone well before the box's edge
	bw, bh := float32(168), float32(50)
	var hover, down bool
	glow = ui.Box(c.Key("w-go-glow")).Width(bw + 2*pad).Height(bh + 2*pad).AlignItems(ui.Center).Justify(ui.Center)
	glow.Draw(func(p *ui.Painter, r ui.Rect) {
		if !welcomeShaders || !shadersOn() {
			return
		}
		hv, dn := float32(0), float32(0)
		if hover {
			hv = 1
		}
		if down {
			dn = 1
		}
		in := smoothstep((p.Now().Sub(a.wel.chosenAt).Seconds() - welButton - 0.2) / 1.1) // the glow, after the pill
		welcomeButtonShader.Draw(p, r, shader.Options{Uniforms: map[string]any{
			"Time": float32(p.Now().Sub(a.wel.start).Seconds()), "Pad": float32(pad), "Hover": hv, "Down": dn, "In": float32(in), "Accent": cl.Accent,
		}})
		p.AnimationFrame()
	})
	glow.Children(func() {
		button = ui.Box(c.Key("w-go")).Width(bw).Height(bh).Radius(bh / 2).AlignItems(ui.Center).Justify(ui.Center).
			Role(ui.RoleButton).Label(locales.WelcomeStart()).Cursor(ui.CursorPointer)
		hover, down = button.Hovered(), button.Pressed()
		if hover {
			c.AnimationFrame()
		}
		button.Children(func() {
			scale := float32(1)
			if down {
				scale = 0.97
			}
			ui.Row(c.Key("w-go-label")).AlignItems(ui.Center).Gap(8).Scale(scale, scale).Children(func() {
				ui.Text(c, locales.WelcomeStart()).FontSize(16).FontWeight(680).TextColor(ui.RGB(255, 255, 255))
				ui.Icon(c, iconArrowRight).Width(16).Height(16).TextColor(ui.RGB(255, 255, 255))
			})
		})
	})
	return glow, button
}

var iconArrowRight = icon(`<path d="M5 12h14M13 6l6 6-6 6"/>`)

// welcomeButtonShader draws Başla's pill on a box Pad larger than it on each side: its body the accent, lighter at
// the top and a little glassy; a thin line of light going round its edge; a glow under it that breathes, more under
// the pointer; a little darker, and smaller, pressed. In (0..1) brings the glow and the running light in.
var welcomeButtonShader = shader.MustCompile(`package main

var Time float
var Pad float
var Hover float
var Down float
var In float
var Accent vec4

func pill(p vec2, half vec2) float {
	r := half.y
	q := abs(p) - (half - vec2(r))
	return length(max(q, vec2(0))) + min(max(q.x, q.y), 0) - r
}

func Fragment(dst vec4, pos vec2) vec4 {
	sz := rectSize()
	half := sz*0.5 - vec2(Pad)
	half *= 1 - Down*0.03
	p := pos - sz*0.5
	sd := pill(p, half)
	acc := Accent.rgb
	// the glow under it, breathing
	breath := 0.5 + 0.5*sin(Time*2.2)
	reach := (9 + 5*breath + 5*Hover) * mix(0.35, 1.0, In) // it spreads as it comes
	glow := exp(-max(sd, 0)/reach) * (0.35 + 0.20*breath + 0.35*Hover) * In * step(0, sd)
	glow *= 1 - smoothstep(Pad*0.45, Pad-4, sd) // and nothing of it at the box's edge, so no square shows
	out := vec4(acc*glow, glow)
	// the body: lighter at the top, a sheen across it
	inside := 1 - smoothstep(-0.8, 0.8, sd)
	t := clamp(p.y/(half.y*2)+0.5, 0, 1)
	body := mix(mix(acc, vec3(1), 0.32), acc*0.86, t)
	body += vec3(1) * 0.10 * exp(-((p.y+half.y*0.45)*(p.y+half.y*0.45))/(half.y*half.y*0.18)) // the sheen
	body *= 1 + 0.12*Hover - 0.10*Down
	// the edge: a thin line of light running round it
	ang := atan2(p.y, p.x)
	run := pow(0.5+0.5*cos(ang-Time*1.6), 6.0)
	edge := exp(-abs(sd+1.0)/0.9) * (0.25 + 0.9*run*In)
	body += vec3(1) * edge * 0.9
	out = vec4(body, 1)*inside + out*(1-inside)
	rimOut := exp(-abs(sd)/1.2) * run * 0.5 * In * (1 - inside)
	out += vec4(vec3(rimOut), rimOut)
	return out
}
`)
