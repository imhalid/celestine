package main

import (
	"math"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	ryugo "github.com/imhalid/ryu-go"
)

// The first opening. There is no window to be seen: soft colored circles come slowly onto the desktop itself,
// and the words of the app rise among them one after another, from below; when the last is there a button
// says "Başla". Pressed, the app's own window opens out of the button, a ring of light running over it (a
// shader on the whole window, reveal below), and the circles go.
//
// On macOS that is three windows (a first run, or RYUDIT_WELCOME=1): the stage, frameless and clear, over the
// whole display, which has the circles and the words; the app's window, built all along but hidden; and, for
// the opening only, a frameless clear window just over the app's, in which the app is drawn inside the ring.
// The app's window has its traffic lights from the moment it is made, and nothing takes them away, so it is
// not what the ring opens: it is shown unseen (no opacity) under the opening's window, and when the ring has
// run over all of it, it is seen, lights, shadow and all, and the opening's window goes. Any other time (a
// replay, Linux, Windows) the welcome is drawn in the app's window, on a plain ground.

// welcomeShaders is false in the tests, whose CPU renderer runs a shader far too slowly to step through a
// few seconds of frames: they check where the welcome is, not how it looks (that is looked at in the app).
var welcomeShaders = true

// welcomeState is where the welcome is.
type welcomeState struct {
	on      bool      // it is up: the intro or the opening
	start   time.Time // the first frame of it
	goAt    time.Time // when Başla was pressed; zero until then
	origin  ui.Point  // where the opening runs out from: the button
	win     *mygo.Window
	stage   *mygo.Window // the clear window over the display that has the circles and the words (clear only)
	opening *mygo.Window // the frameless window the app opens in, over its own (clear only)
	inOpen  bool         // the frame being built is the opening window's
	bar     ui.TitleBar  // the app's window's title bar, which the opening window has not (titleBar)
	clear   bool         // the welcome is on the stage, and the app's window waits unseen until it has opened
}

const (
	welGround  = 2.2 // the circles come in over this long, before the words
	welMark    = 1.6
	welHead    = 2.0
	welSub     = 2.35
	welCard0   = 2.75
	welCardGap = 0.22
	welButton  = 3.6
	welOpen    = 1.35 // the opening takes this long, in seconds
	welCorner  = 9    // the opening window's corners, as the app's window has them (macOS)
)

// wantWelcome: the first run of a library that keeps its settings, or when asked.
func (a *app) wantWelcome() bool {
	if os.Getenv("RYUDIT_WELCOME") == "1" {
		return true
	}
	return !a.settings.WelcomeDone && (a.settingsFile != "" || a.settingsStore != nil)
}

// welcomeAgain is the command: the welcome, once more. The window is not clear any more, so it sits on a ground.
func (a *app) welcomeAgain() {
	a.closeSettings()
	a.wel = welcomeState{on: true, win: a.wel.win}
	a.settings.WelcomeDone = false
}

// welcomeUp: the welcome has the window.
func (a *app) welcomeUp() bool { return a.wel.on }

// ---- the shaders ----

// ringShared is the edge of the opening, in both shaders: the app is kept inside it, and the circles outside.
const ringShared = `
func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3 - 2*f)
	return mix(mix(hash(i), hash(i+vec2(1, 0)), u.x), mix(hash(i+vec2(0, 1)), hash(i+vec2(1, 1)), u.x), u.y)
}

// ringEdge is how far from the origin the edge is at pos: not a circle, it breathes, most in the middle.
func ringEdge(pos vec2, radius float, prog float) float {
	wob := (noise(pos/46+vec2(prog*2.4, 0))*0.65 + noise(pos/19)*0.12 - 0.38) * 100 * sin(prog*3.14159)
	return radius + wob
}
`

// welcomeGround draws the circles: soft blobs of color that drift, over a base, premultiplied so that what
// is not covered stays clear and the desktop shows through.
var welcomeGround = shader.MustCompile(`package main

var Origin vec2
var Radius float
var Prog float
var Open float
var Time float
var Fade float
var Dark float
var Base vec4

func tint(i int) vec3 {
	if i == 0 {
		return vec3(1.00, 0.55, 0.30)
	}
	if i == 1 {
		return vec3(1.00, 0.38, 0.64)
	}
	if i == 2 {
		return vec3(0.60, 0.44, 1.00)
	}
	if i == 3 {
		return vec3(0.28, 0.62, 1.00)
	}
	if i == 4 {
		return vec3(0.24, 0.86, 0.80)
	}
	return vec3(1.00, 0.80, 0.42)
}

func hash21(p vec2) float {
	q := fract(p * vec2(123.34, 456.21))
	q += dot(q, q+45.32)
	return fract(q.x * q.y)
}

func Fragment(dst vec4, pos vec2) vec4 {
	sz := rectSize()
	p := (pos - sz*0.5) / sz.y
	out := Base
	for i := 0; i < 6; i++ {
		fi := float(i)
		ph := fi*1.7 + 0.3
		c := vec2(
			0.46*sin(Time*0.13+ph*1.3)+0.14*sin(Time*0.27+ph),
			0.34*cos(Time*0.10+ph*1.9)+0.10*cos(Time*0.21+ph*0.7),
		)
		r := 0.30 + 0.09*sin(ph*2.3+Time*0.08)
		d := distance(p, c) / r
		w := exp(-d * d * 1.7)
		col := tint(i) * mix(1.0, 0.82, Dark)
		a := w * 0.72
		out = vec4(col*a, a) + out*(1-a)
	}
	g := (hash21(dst.xy) - 0.5) * 0.06
	out.rgb += vec3(g) * out.a
	if Open > 0.5 { // the app has the inside of the ring
		d := distance(pos, Origin)
		edge := ringEdge(pos, Radius, Prog)
		out *= smoothstep(edge-90, edge, d)
	}
	return out * Fade
}
` + ringShared)

// welcomeOpen is the opening: the app's own pixels, kept inside a ring that grows from the button, with a
// rim of light on it, bent a little where it passes. Outside the ring the window is the welcome still.
var welcomeOpen = shader.MustCompile(`package main

var Origin vec2
var Radius float
var Prog float

func LayerEffect(pos vec2) vec4 {
	rel := pos - Origin
	d := length(rel)
	dir := rel / max(d, 0.001)
	// the edge is not a circle: it breathes, most in the middle of the opening
	edge := ringEdge(pos, Radius, Prog)
	width := 90.0
	m := 1 - smoothstep(edge-width, edge, d)
	// the rim: bright, narrow, and just behind the edge
	rimAt := (d - (edge - width*0.42)) / (width * 0.30)
	rim := exp(-rimAt * rimAt)
	// where the ring passes, the app is seen through it, bent and split in colors
	lens := smoothstep(edge-width*1.7, edge-width*0.5, d) * (1 - smoothstep(edge-width*0.5, edge+8, d))
	off := dir * lens * 20 * (1 - Prog*0.6)
	c0 := layerSample(pos - off*0.75)
	c1 := layerSample(pos - off)
	c2 := layerSample(pos - off*1.25)
	col := vec4(c0.r, c1.g, c2.b, c1.a) * m
	ang := atan2(dir.y, dir.x)
	hue := mix(vec3(1.0, 0.46, 0.70), vec3(0.46, 0.58, 1.0), 0.5+0.5*sin(ang*2+Prog*7))
	fade := (1 - smoothstep(0.55, 1.0, Prog)) * 0.9
	glow := rim * fade
	col.rgb += hue * glow * 0.85
	col.a = max(col.a, glow*0.85)
	return col
}
` + ringShared)

// ---- the frames ----

// welcomeClock is how far the intro is, in seconds; the first frame starts it.
func (a *app) welcomeClock(c *ui.Context) float64 {
	if a.wel.start.IsZero() {
		a.wel.start = c.Now()
	}
	return c.Now().Sub(a.wel.start).Seconds()
}

// welcomeProgress is how far the opening is, 0 until Başla, then to 1.
func (a *app) welcomeProgress(c *ui.Context) float64 {
	if a.wel.goAt.IsZero() {
		return 0
	}
	t := min(1, max(0, c.Now().Sub(a.wel.goAt).Seconds()/welOpen))
	return t * t * (3 - 2*t) // slow at both ends
}

// openStage puts up the stage: a window over the whole display the app's window is on, with no frame, no
// shadow and no ground, so that only what the welcome draws is seen on the desktop.
func (a *app) openStage() {
	d := mygo.Screen.DisplayMatching(a.wel.win.Bounds()).Bounds
	st := mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", X: d.X, Y: d.Y, Width: d.Width, Height: d.Height,
		Frameless: true, Transparent: true, DisableShadow: true, DisableResize: true, DisableMove: true,
		DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, Content: ui.View(a.stageView)})
	st.SetBounds(d) // a centered window would be kept off the menu bar
	st.Focus()
	a.wel.stage = st
}

// stageView is the stage's frame: the circles and the words, and nothing under them.
func (a *app) stageView(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.frameTheme(c)
	c.Root().Background(ui.Transparent)
	a.welcomeBackdrop(c, true)
	a.welcomeView(c, true)
}

// welcomeBackdrop draws the circles: in the app's window built before its own root, so that it is under it.
// onStage is the stage's frame; each is drawn in one window only.
func (a *app) welcomeBackdrop(c *ui.Context, onStage bool) {
	if !a.wel.on || a.wel.clear != onStage {
		return
	}
	el := a.welcomeClock(c)
	tok := ryugo.From(c)
	clear := c.Vibrancy() || onStage
	if clear {
		c.Root().Background(ui.Transparent) // the desktop is the ground
	}
	base := tok.Colors.Card // no glass (a replay, Linux): the circles sit on the app's ground
	baseV := []float32{float32(base.R) / 255, float32(base.G) / 255, float32(base.B) / 255, 1}
	if onStage { // the desktop as it is: only the circles on it
		baseV = []float32{0, 0, 0, 0}
	} else if clear { // a veil only, so that the circles and the words keep their contrast over a bright desktop
		if tok.Dark {
			baseV = []float32{0, 0, 0, 0.16}
		} else {
			baseV = []float32{0.16, 0.16, 0.16, 0.16}
		}
	}
	dark := float32(0)
	if tok.Dark {
		dark = 1
	}
	fade := float32(min(1, el/welGround))
	fade = fade * fade * (3 - 2*fade)
	// while the app opens, the circles keep to outside the ring that shows it; on the stage the app's window
	// is over them, and they go as it opens
	open, radius, prog := float32(0), float32(0), float32(0)
	if onStage {
		out := float32(min(1, max(0, (a.welcomeProgress(c)-0.15)/0.85)))
		fade *= 1 - out*out*(3-2*out)
	} else if !a.wel.goAt.IsZero() {
		p := a.welcomeProgress(c)
		open, radius, prog = 1, float32(a.welcomeRadius(c, p)), float32(p)
	}
	ui.Box(c.Key("welcome-ground")).Absolute().Fill().Draw(func(p *ui.Painter, r ui.Rect) {
		if !welcomeShaders {
			return
		}
		welcomeGround.Draw(p, r, shader.Options{Uniforms: map[string]any{
			"Time": float32(p.Now().Sub(a.wel.start).Seconds()), "Fade": fade, "Dark": dark,
			"Open": open, "Origin": []float32{a.wel.origin.X, a.wel.origin.Y}, "Radius": radius, "Prog": prog,
			"Base": []float32{baseV[0] * baseV[3], baseV[1] * baseV[3], baseV[2] * baseV[3], baseV[3]},
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
	if welcomeShaders {
		root.Effect(welcomeOpen.Effect(shader.Options{
			Uniforms:        map[string]any{"Origin": []float32{o.X, o.Y}, "Radius": float32(radius), "Prog": float32(prog)},
			MaxSampleOffset: ui.Point{X: 28, Y: 28},
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
	if os.Getenv("RYUDIT_WELCOME") != "1" { // asked for by the variable: a look, not a first run
		a.settings.WelcomeDone = true
		a.saveSettings()
	}
	if a.wel.clear && a.wel.win != nil { // the app's own window is seen, with its shadow, where the opening was
		a.wel.win.SetOpacity(1)
		a.wel.win.SetHasShadow(true)
		a.wel.win.Focus()
		a.wel.win.Invalidate()
		a.wel.clear = false
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
		op := mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", X: w.X, Y: w.Y, Width: w.Width, Height: w.Height,
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
	a.wel.inOpen = true
	defer func() { a.wel.inOpen = false }()
	a.view(c)
}

// welcomeView is the words and the button, above everything: they rise in turn, and lift off as the app opens.
func (a *app) welcomeView(c *ui.Context, onStage bool) {
	if !a.wel.on || a.wel.clear != onStage {
		return
	}
	el := a.welcomeClock(c)
	prog := a.welcomeProgress(c)
	if !a.wel.goAt.IsZero() { // lifting off: a frame each frame, the app's window has its own
		c.AnimationFrame()
	}
	cl := ryugo.From(c).Colors
	dark := ryugo.From(c).Dark
	// the next piece to rise: another frame then (the circles move on their own, in paint)
	next := math.Inf(1)
	for _, t := range []float64{welMark, welHead, welSub, welCard0, welCard0 + welCardGap, welCard0 + 2*welCardGap, welButton} {
		if t > el && t < next {
			next = t
		}
	}
	if !math.IsInf(next, 1) {
		c.After(time.Duration((next-el)*float64(time.Second)) + 8*time.Millisecond)
	} else if el < welButton+1.2 { // the last has risen; a few frames more while it settles
		c.After(50 * time.Millisecond)
	}
	rise := ui.Animation{Spring: ui.SpringOptions{Duration: 820 * time.Millisecond, Bounce: 0.1}}
	// rises is an element that comes up from below, and fades in, once its time has come
	rises := func(e ui.Element, at float64) ui.Element {
		pose := ui.Pose{Y: 64, Fade: 1}
		if el >= at {
			pose = ui.Pose{}
		}
		return e.AnimatePose(pose, rise)
	}
	glass := ui.RGBA(255, 255, 255, 0.46)
	rim := ui.RGBA(255, 255, 255, 0.55)
	if dark {
		glass, rim = ui.RGBA(255, 255, 255, 0.10), ui.RGBA(255, 255, 255, 0.18)
	}
	w, h := c.Size()
	ui.Overlay(c, func() {
		stage := ui.Column(c.Key("welcome")).Absolute().Left(0).Top(0).Width(w).Height(h).AlignItems(ui.Center).Justify(ui.Center).Gap(0)
		// lifting off as the app opens
		stage.Opacity(float32(1-min(1, prog*3))).Translate(0, float32(-30*prog))
		stage.Children(func() {
			ui.Column(c.Key("welcome-col")).Width(min(460, w-32)).AlignItems(ui.Center).Gap(0).Children(func() {
				rises(ui.Text(c.Key("w-mark"), "ryudit").FontSize(15).FontWeight(680).TextColor(cl.Text2), welMark)
				ui.Box(c.Key("w-gap0")).Height(18)
				rises(ui.Text(c.Key("w-head"), T("Yaz. Gerisi kendiliğinden.")).FontSize(40).FontWeight(720).TextColor(cl.Text).TextAlign(ui.Center), welHead)
				ui.Box(c.Key("w-gap1")).Height(12)
				rises(ui.Text(c.Key("w-sub"), T("İşaret görmeden yazdığın, tamamen sende duran bir not defteri.")).FontSize(15.5).TextColor(cl.Text2).TextAlign(ui.Center).Wrap(), welSub)
				ui.Box(c.Key("w-gap2")).Height(34)
				for i, it := range welcomeCards {
					row := ui.Row(c.Key("w-card"+itoa(i))).FillWidth().AlignItems(ui.Center).Gap(14).Padding(14, 16).
						Radius(rad(18)).Background(glass).Border(1, rim)
					rises(row, welCard0+float64(i)*welCardGap).Children(func() {
						ui.Box(c.Key("w-ic"+itoa(i))).Size(38, 38).Radius(rad(11)).Background(it.tint).Children(func() {
							ui.Icon(c, it.icon).Width(20).Height(20).TextColor(ui.RGB(255, 255, 255)).Absolute().Left(9).Top(9)
						})
						ui.Column(c.Key("w-tx" + itoa(i))).Grow(1).Gap(2).Children(func() {
							ui.Text(c, T(it.title)).FontSize(14.5).FontWeight(650).TextColor(cl.Text)
							ui.Text(c, T(it.body)).FontSize(13).TextColor(cl.Text2).Wrap()
						})
					})
					if i < len(welcomeCards)-1 {
						ui.Box(c.Key("w-cg" + itoa(i))).Height(10)
					}
				}
				ui.Box(c.Key("w-gap3")).Height(30)
				wrap := ui.Column(c.Key("w-go-box")).AlignItems(ui.Center)
				rises(wrap, welButton).Children(func() {
					if ryugo.Button(c.Key("w-go"), T("Başla"), ryugo.ButtonOptions{Size: ryugo.Lg, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						a.welcomeStart(c, wrap.Bounds())
					}
				})
				if el >= welButton && c.Shortcut(0, ui.KeyEnter) {
					a.welcomeStart(c, wrap.Bounds())
				}
			})
		})
	})
	if c.Shortcut(0, ui.KeyEscape) { // skip it
		a.welcomeStart(c, ui.Rect{})
	}
}

// welcomeCards are the three things said before the app, in the app's own words.
var welcomeCards = []struct {
	title, body string
	icon        *ui.SVG
	tint        ui.Color
}{
	{"Biçimi kendisi girer", "# başlık, - liste, [] görev: yazdıkça yapıya dönüşür, işaret görünmez.", iconEdit, ui.RGB(236, 98, 80)},
	{"Her şey ⌘P'de", "Komutlar, ayarlar ve sayfalar tek bir kutuda; fare şart değil.", iconSearch, ui.RGB(122, 96, 240)},
	{"Senin kitaplığın", "Tek dosya, bu bilgisayarda; yedeği her gün alınır, eşitleme şifreli.", iconStack, ui.RGB(36, 150, 232)},
}

func init() {
	for tr, en := range map[string]string{
		"Yaz. Gerisi kendiliğinden.":                                             "Write. The rest takes care of itself.",
		"İşaret görmeden yazdığın, tamamen sende duran bir not defteri.":         "A notebook you write in without seeing a mark, and that stays entirely with you.",
		"Biçimi kendisi girer":                                                   "It formats itself",
		"# başlık, - liste, [] görev: yazdıkça yapıya dönüşür, işaret görünmez.": "# heading, - list, [] task: as you type it becomes structure, and the mark is not shown.",
		"Her şey ⌘P'de": "Everything is in ⌘P",
		"Komutlar, ayarlar ve sayfalar tek bir kutuda; fare şart değil.": "Commands, settings and pages in one box; no mouse needed.",
		"Senin kitaplığın": "Your library",
		"Tek dosya, bu bilgisayarda; yedeği her gün alınır, eşitleme şifreli.": "One file, on this computer; backed up every day, and sync is encrypted.",
		"Başla":                     "Start",
		"Karşılama ekranını göster": "Show the welcome screen",
	} {
		if _, ok := enText[tr]; !ok {
			enText[tr] = en
		}
	}
}
