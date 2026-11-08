package main

import (
	"math"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	ryugo "github.com/imhalid/ryu-go"
)

// The first opening. There is no window to be seen: the desktop itself darkens, most at its edges, and a mesh of
// colors comes up slowly in the middle of it; the words of the app rise into it one after another, from below; when the last is there a button
// says "Başla". Pressed, the app's own window opens out of the button, a ring of light running over it (a
// shader on the whole window, reveal below), and the colors go.
//
// On macOS that is three windows (a first run, or RYUDIT_WELCOME=1): the stage, frameless and clear, over the
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
	on      bool      // it is up: the intro or the opening
	start   time.Time // the first frame of it
	goAt    time.Time // when Başla was pressed; zero until then
	origin  ui.Point  // where the opening runs out from: the button
	win     *mygo.Window
	stage   *mygo.Window // the clear window over the display that has the colors and the words (clear only)
	opening *mygo.Window // the frameless window the app opens in, over its own (clear only)
	inOpen  bool         // the frame being built is the opening window's
	bar     ui.TitleBar  // the app's window's title bar, which the opening window has not (titleBar)
	clear   bool         // the welcome is on the stage, and the app's window waits unseen until it has opened
}

const (
	welVeil    = 1.4 // the desktop darkens over this long
	welMeshAt  = 0.9 // and the mesh of colors comes in from here, over welMesh
	welMesh    = 2.2
	welMark    = 2.3
	welHead    = 2.7
	welSub     = 3.05
	welCard0   = 3.45
	welCardGap = 0.22
	welButton  = 4.3
	welOpen    = 2.1 // the opening takes this long, in seconds, unless its look says otherwise
	welCorner  = 9   // the opening window's corners, as the app's window has them (macOS)
)

// wantWelcome: the first run of a library that keeps its settings, or when asked (RYUDIT_WELCOME=1, or 0 for never).
func (a *app) wantWelcome() bool {
	switch os.Getenv("RYUDIT_WELCOME") {
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

// welcomeAutoGo is when Başla presses itself, in seconds from the start: RYUDIT_WELCOME_GO=6, to watch a look
// through without touching the keys (RYUDIT_WELCOME=1 RYUDIT_WELCOME_GO=6 RYUDIT_WELCOME_OPEN=crt). Zero is never.
func welcomeAutoGo() float64 {
	v, _ := strconv.ParseFloat(os.Getenv("RYUDIT_WELCOME_GO"), 64)
	return v
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
	o := a.welcomeOpeningLook()
	return o.progress(now.Sub(a.wel.goAt).Seconds() / o.length())
}

// openStage puts up the stage: a window over the whole display the app's window is on, with no frame, no
// shadow and no ground, so that only what the welcome draws is seen on the desktop.
func (a *app) openStage() {
	d := mygo.Screen.DisplayMatching(a.wel.win.Bounds()).Bounds
	opts := mygo.WindowOptions{Title: "ryudit", X: d.X, Y: d.Y, Width: d.Width, Height: d.Height,
		Frameless: true, Transparent: true, DisableShadow: true, DisableResize: true, DisableMove: true,
		DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, Content: ui.View(a.stageView)}
	blur := a.welcomeGroundLook().blur
	if blur { // the desktop blurred under a dark material, which the window's opacity brings in slowly
		opts.Vibrancy, opts.Opacity = mygo.VibrancyHUD, 0.01
	}
	st := mygo.NewWindow(opts)
	st.SetBounds(d) // a centered window would be kept off the menu bar
	st.Focus()
	a.wel.stage = st
	if blur {
		a.fadeWindow(st, 0, 1, 0, welVeil)
	}
}

// fadeWindow takes the opacity of w from one value to another, eased, after delay seconds, over dur: the stage's
// material cannot fade by itself.
func (a *app) fadeWindow(w *mygo.Window, from, to, delay, dur float64) {
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
				return
			}
			time.Sleep(16 * time.Millisecond)
		}
	}()
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
	dark := float32(0)
	if tok.Dark {
		dark = 1
	}
	// while the app opens, the ground keeps to outside the ring that shows it; on the stage the app's window
	// is over it, and it goes as the window opens
	open, radius, prog := float32(0), float32(0), float32(0)
	ring := a.welcomeOpeningLook().ring
	if !onStage && ring && !a.wel.goAt.IsZero() {
		p := a.welcomeProgress(c)
		open, radius, prog = 1, float32(a.welcomeRadius(c, p)), float32(p)
	}
	ui.Box(c.Key("welcome-ground")).Absolute().Fill().Draw(func(p *ui.Painter, r ui.Rect) {
		if !welcomeShaders {
			return
		}
		// the steps are timed by the paint's own frame: the view is built again only now and then (welcomeView)
		t := p.Now().Sub(a.wel.start).Seconds()
		veil, mesh, fade := smoothstep(t/welVeil), smoothstep((t-welMeshAt)/welMesh), 1.0
		if onStage || !ring { // the app's window is over it, or the opening has no ring to keep it outside of
			fade = 1 - smoothstep((a.welcomeProgressAt(p.Now())-0.15)/0.85)
		}
		a.welcomeGroundLook().sh.Draw(p, r, shader.Options{Uniforms: map[string]any{
			"Time": float32(t), "Fade": float32(fade), "Veil": float32(veil), "Mesh": float32(mesh), "Dark": dark,
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
		if a.welcomeGroundLook().blur { // the blur goes as the window comes
			n := a.welcomeOpeningLook().length()
			a.fadeWindow(a.wel.stage, 1, 0, 0.15*n, 0.85*n)
		}
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
	// the next piece to rise: another frame then (the ground moves on its own, in paint)
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
				if at := welcomeAutoGo(); at > 0 && a.wel.goAt.IsZero() { // a look watched to its end, no hand on the keys
					if el >= at {
						a.welcomeStart(c, wrap.Bounds())
					} else {
						c.After(time.Duration((at-el)*float64(time.Second)) + 8*time.Millisecond)
					}
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
		"Karşılama ekranı":          "Welcome screen", "Yeniden başlat": "Start again",
		"İlk açılıştaki karşılamayı seçilen görünümle baştan gösterir.": "Shows the first run's welcome again, from the start, in the look chosen.",
		"Karşılamanın arka planı": "The welcome's ground", "Açılış": "Opening", "İki ton": "Two tones", "Renkli": "Colorful", "Gece": "Night",
		"Halka": "Ring", "Perde": "Curtain", "Çözülme": "Dissolve", "Çizgiler": "Lines",
		"Bulut": "Clouds", "Sistem": "System",
		"Buz": "Ice", "Kristal": "Crystal", "Prizma": "Prism", "Kuzey": "Aurora", "Patlama": "Burst", "Ufuk": "Horizon", "Cam": "Glass",
		"Spektrum": "Spectrum",
		"Yazıların altında, masaüstünün üstünde ne olacağı.": "What is under the words, over the desktop.",
		"Başla'dan sonra uygulamanın penceresi nasıl gelir.": "How the app's window comes after Start.",
	} {
		if _, ok := enText[tr]; !ok {
			enText[tr] = en
		}
	}
}
