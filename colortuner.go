package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The color tuner (#511), a temporary tool: View ▸ Color tuner opens a window of its own listing every color of the
// look in use - Ryugo's tokens, MyGo's theme, the page's background - and the opacities of the glass and the cards,
// each changed live in the app. Copy all puts every value on the clipboard as text, the look named, to be made the
// theme's own; then the tool goes. What is changed is kept for the run only, by look (theme, light or dark, glass).

// tunerState is what the tuner keeps.
type tunerState struct {
	win      *mygo.Window
	on       bool                           // it was opened this run: from then on each frame is tuned
	look     string                         // the look the window last drew
	sel      string                         // the value chosen, by key
	edit     ui.Color                       // the chosen color, as its picker edits it
	editA    float64                        // the chosen opacity, as its slider edits it
	filter   string                         // what the list is narrowed to
	copied   time.Time                      // when Copy all was last pressed
	colors   map[string]map[string]ui.Color // what was changed, by look and key
	alphas   map[string]map[string]float32  // the same for the opacities
	snaps    map[*ryugo.Tokens]*tunerSnap   // each root's colors as the app made them, before any change
	cur      *tunerSnap                     // the snapshot of this frame's root
	page     ui.Color                       // the page's background as the app made it
	alphaDef map[string]float32             // each opacity as the app has it
	dark     bool                           // the main window is dark
	editFor  string                         // the look and key the picker was given
}

// tunerSnap is a root's colors as the app made them: Ryugo's tokens and MyGo's theme, by field.
type tunerSnap struct {
	ryugo map[string]ui.Color
	mygo  map[string]ui.Color
}

var colorType = reflect.TypeOf(ui.Color{})

// colorFields is the fields of struct v (a pointer to it) that are colors, by name, in their order.
func colorFields(v any) []reflect.StructField {
	t := reflect.TypeOf(v).Elem()
	var out []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() && f.Type == colorType {
			out = append(out, f)
		}
	}
	return out
}

// tunerLook names the look in use: the theme, light or dark, and the glass.
func (a *app) tunerLook(dark bool) string {
	scheme := map[bool]string{false: "light", true: "dark"}[dark]
	glassOn := map[bool]string{false: "off", true: "on"}[a.settings.Glass]
	return fmt.Sprintf("theme %s · %s · glass %s", a.appTheme().name, scheme, glassOn)
}

// tuneTheme puts the tuner's changes into this frame's root, last in frameTheme: each color as the app made it,
// then what was changed for this look over it. Nothing is done before the tuner was first opened.
func (a *app) tuneTheme(c *ui.Context, tok *ryugo.Tokens) {
	tn := &a.tuner
	if !tn.on {
		return
	}
	th := c.Theme()
	s := tn.snaps[tok]
	if s == nil { // its first frame: the colors as Ryugo, the app's theme and lightLevels left them
		s = &tunerSnap{ryugo: map[string]ui.Color{}, mygo: map[string]ui.Color{}}
		rv := reflect.ValueOf(&tok.Colors).Elem()
		for _, f := range colorFields(&tok.Colors) {
			s.ryugo[f.Name] = rv.FieldByIndex(f.Index).Interface().(ui.Color)
		}
		tv := reflect.ValueOf(th).Elem()
		for _, f := range colorFields(th) {
			if f.Name != "Scrollbar" { // set again each frame, from Muted
				s.mygo[f.Name] = tv.FieldByIndex(f.Index).Interface().(ui.Color)
			}
		}
		tn.snaps[tok] = s
	}
	tn.cur, tn.dark = s, tok.Dark
	look := a.tunerLook(tok.Dark)
	over := tn.colors[look]
	rv := reflect.ValueOf(&tok.Colors).Elem()
	for _, f := range colorFields(&tok.Colors) {
		v := s.ryugo[f.Name]
		if o, ok := over["ryugo."+f.Name]; ok {
			v = o
		}
		rv.FieldByIndex(f.Index).Set(reflect.ValueOf(v))
	}
	tv := reflect.ValueOf(th).Elem()
	for _, f := range colorFields(th) {
		v, ok := s.mygo[f.Name]
		if !ok {
			continue
		}
		if o, ok := over["mygo."+f.Name]; ok {
			v = o
		}
		tv.FieldByIndex(f.Index).Set(reflect.ValueOf(v))
	}
	if over != nil {
		if o, ok := over["mygo.TextMuted"]; ok {
			th.Scrollbar = o.Alpha(0.45)
		}
	}
	if look != tn.look && tn.win != nil {
		tn.win.Invalidate()
	}
}

// tunePage is the page's background, bg as the app made it, or the tuner's change of it.
func (a *app) tunePage(c *ui.Context, bg ui.Color) ui.Color {
	tn := &a.tuner
	if !tn.on {
		return bg
	}
	tn.page = bg
	if o, ok := tn.colors[a.tunerLook(ryugo.From(c).Dark)]["app.pageBg"]; ok {
		return o
	}
	return bg
}

// tuneAlpha is the opacity key, def as the app has it, or the tuner's change of it.
func (a *app) tuneAlpha(c *ui.Context, key string, def float32) float32 {
	tn := &a.tuner
	if !tn.on {
		return def
	}
	tn.alphaDef[key] = def
	if o, ok := tn.alphas[a.tunerLook(ryugo.From(c).Dark)][key]; ok {
		return o
	}
	return def
}

// openTuner opens the tuner's window, or brings it forward.
func (a *app) openTuner() {
	tn := &a.tuner
	if tn.win != nil {
		tn.win.Focus()
		return
	}
	a.startTuner()
	opts := mygo.WindowOptions{Title: locales.TunerTitle(), Width: 760, Height: 760, MinWidth: 560, MinHeight: 420,
		Content: ui.View(a.tunerView)}
	go func() { // a window is made off the frame that asked, as the backup's is
		defer a.guard()
		w := mygo.NewWindow(opts)
		w.OnClosed(func() { a.do(func() error { a.tuner.win = nil; return nil }) })
		a.do(func() error { a.tuner.win = w; return nil })
	}()
	if a.invalidate != nil {
		a.invalidate()
	}
}

// startTuner starts tuning each frame, from the first opening on.
func (a *app) startTuner() {
	tn := &a.tuner
	if tn.on {
		return
	}
	tn.on = true
	tn.colors, tn.alphas = map[string]map[string]ui.Color{}, map[string]map[string]float32{}
	tn.snaps, tn.alphaDef = map[*ryugo.Tokens]*tunerSnap{}, map[string]float32{}
}

// tunerItem is one value the tuner lists.
type tunerItem struct {
	key   string
	color ui.Color // as it is now
	base  ui.Color // as the app made it
	alpha bool
	a, aB float32
}

// tunerItems is every value of the look in use, as it is now and as the app made it.
func (a *app) tunerItems(look string) []tunerItem {
	tn := &a.tuner
	var out []tunerItem
	if s := tn.cur; s != nil {
		add := func(prefix string, m map[string]ui.Color, names []string) {
			for _, n := range names {
				base, ok := m[n]
				if !ok {
					continue
				}
				key := prefix + n
				now := base
				if o, ok := tn.colors[look][key]; ok {
					now = o
				}
				out = append(out, tunerItem{key: key, color: now, base: base})
			}
		}
		var rn, mn []string
		for _, f := range colorFields(&ryugo.Colors{}) {
			rn = append(rn, f.Name)
		}
		for _, f := range colorFields(&ui.Theme{}) {
			mn = append(mn, f.Name)
		}
		add("ryugo.", s.ryugo, rn)
		add("mygo.", s.mygo, mn)
	}
	page := tn.page
	if o, ok := tn.colors[look]["app.pageBg"]; ok {
		page = o
	}
	out = append(out, tunerItem{key: "app.pageBg", color: page, base: tn.page})
	keys := make([]string, 0, len(tn.alphaDef))
	for k := range tn.alphaDef {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		def := tn.alphaDef[k]
		now := def
		if o, ok := tn.alphas[look][k]; ok {
			now = o
		}
		out = append(out, tunerItem{key: k, alpha: true, a: now, aB: def})
	}
	return out
}

// changed reports whether the item is not as the app made it.
func (it tunerItem) changed() bool {
	if it.alpha {
		return it.a != it.aB
	}
	return it.color != it.base
}

// value is the item's value as text: a color in hex, an opacity as a number.
func (it tunerItem) value() string {
	if it.alpha {
		return fmt.Sprintf("%.3f", it.a)
	}
	return hexColor(it.color)
}

// hexColor is c as #rrggbb, or #rrggbbaa when it is not opaque.
func hexColor(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// tunerText is every value of the look as text, for the clipboard: the look first, then one value a line, the
// changed ones saying what they were.
func tunerText(look string, items []tunerItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Celestine colors - %s - %s/%s\n", look, runtime.GOOS, runtime.GOARCH)
	w := 0
	for _, it := range items {
		w = max(w, len(it.key))
	}
	for _, it := range items {
		fmt.Fprintf(&b, "%-*s  %s", w, it.key, it.value())
		if it.changed() {
			was := tunerItem{alpha: it.alpha, a: it.aB, color: it.base}
			fmt.Fprintf(&b, "  (changed; was %s)", was.value())
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// tunerView is the tuner's window: the look's values on the left, the one chosen on the right in a picker.
func (a *app) tunerView(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	tn := &a.tuner
	// its own look, dark and of another size than the app's, so that none of its tokens are the app's and tuned
	tok := ryugo.Root(c, ryugo.Options{Mode: ryugo.Dark, Size: ryugo.Md})
	cl := tok.Colors
	look := a.tunerLook(tn.dark)
	tn.look = look
	items := a.tunerItems(look)
	poke := func() {
		if a.invalidate != nil {
			a.invalidate()
		}
	}
	ui.Column(c).Fill().Background(cl.Bg).Children(func() {
		ui.Row(c).FillWidth().Height(48).Padding(0, 14, 0, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, look).FontSize(12.5).FontWeight(600).TextColor(cl.Text).Grow(1)
			ryugo.Input(c.Key("tuner-filter"), &tn.filter, ryugo.InputOptions{Placeholder: locales.TunerFilter()}).Width(170)
			if ryugo.Button(c.Key("tuner-reset"), locales.TunerReset(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				delete(tn.colors, look)
				delete(tn.alphas, look)
				tn.sel = ""
				poke()
			}
			label := locales.TunerCopyAll()
			if time.Since(tn.copied) < 2*time.Second {
				label = locales.TunerCopied()
			}
			if ryugo.Button(c.Key("tuner-copy"), label, ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Solid, Color: cl.Accent}).Clicked() {
				mygo.Clipboard.WriteText(tunerText(look, items))
				tn.copied = time.Now()
				if w := tn.win; w != nil {
					go func() { time.Sleep(2100 * time.Millisecond); w.Invalidate() }() // the label back to Copy all
				}
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
		ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c.Key("tuner-list")).Grow(1).FillHeight().Children(func() {
				ui.Column(c).FillWidth().Padding(6, 8, 10, 8).Children(func() {
					q := strings.ToLower(tn.filter)
					for _, it := range items {
						if q != "" && !strings.Contains(strings.ToLower(it.key), q) {
							continue
						}
						a.tunerRow(c, cl, it)
					}
				})
			})
			ui.Box(c).Width(1).FillHeight().Background(cl.Hairline)
			ui.Column(c).Width(290).FillHeight().Padding(14, 14, 14, 14).Gap(10).Children(func() {
				a.tunerEditor(c, cl, look, items, poke)
			})
		})
	})
}

// tunerRow is one value in the list: its swatch, its key and its value; a click chooses it.
func (a *app) tunerRow(c *ui.Context, cl ryugo.Colors, it tunerItem) {
	tn := &a.tuner
	row := ui.Row(c.Key("tuner:"+it.key)).FillWidth().Height(26).Padding(0, 8, 0, 6).Gap(9).AlignItems(ui.Center).
		Radius(6).Cursor(ui.CursorPointer)
	switch {
	case tn.sel == it.key:
		row.Background(cl.FillSelected)
	case row.Hovered():
		row.Background(cl.Fill)
	}
	row.Children(func() {
		ui.Box(c).Size(16, 16).Draw(func(p *ui.Painter, r ui.Rect) {
			drawChecks(p, r)
			if it.alpha {
				p.Fill(r, ui.RGBA(255, 255, 255, it.a), 4)
			} else {
				p.Fill(r, it.color, 4)
			}
			p.Stroke(r, cl.Hairline, 4, 1)
		})
		ui.Text(c, it.key).FontSize(12).TextColor(cl.Text).Grow(1)
		ink := cl.Muted
		if it.changed() {
			ink = cl.Accent
		}
		ui.Text(c, it.value()).FontSize(11.5).Font(a.monoFont()).TextColor(ink)
	})
	if row.Clicked() {
		tn.sel = it.key
		tn.edit, tn.editA = it.color, float64(it.a)
	}
}

// drawChecks draws a checkerboard in r, under a color that is not opaque.
func drawChecks(p *ui.Painter, r ui.Rect) {
	p.Clip(r, 4, func() {
		p.Fill(r, ui.RGBA(255, 255, 255, 1), 0)
		const s = 4
		for y := float32(0); y < r.H; y += s {
			for x := float32(0); x < r.W; x += s {
				if int(x/s+y/s)%2 == 0 {
					p.Fill(ui.Rect{X: r.X + x, Y: r.Y + y, W: s, H: s}, ui.RGBA(200, 200, 200, 1), 0)
				}
			}
		}
	})
}

// tunerEditor is the right side: the chosen value in a picker (a color) or a slider (an opacity), and what it was.
func (a *app) tunerEditor(c *ui.Context, cl ryugo.Colors, look string, items []tunerItem, poke func()) {
	tn := &a.tuner
	var it *tunerItem
	for i := range items {
		if items[i].key == tn.sel {
			it = &items[i]
		}
	}
	if it == nil {
		ui.Text(c, locales.TunerPick()).FontSize(12).TextColor(cl.Muted).Wrap()
		return
	}
	ui.Text(c, it.key).FontSize(13).FontWeight(650).TextColor(cl.Text)
	if it.alpha {
		if ui.Slider(c.Key("tuner-a:"+it.key), &tn.editA, 0, 1).FillWidth().Changed() {
			if tn.alphas[look] == nil {
				tn.alphas[look] = map[string]float32{}
			}
			tn.alphas[look][it.key] = float32(tn.editA)
			poke()
		}
		ui.Text(c, fmt.Sprintf("%.3f", tn.editA)).FontSize(12).Font(a.monoFont()).TextColor(cl.Text2)
	} else {
		if f := look + "/" + it.key; tn.editFor != f { // another value or another look: the picker starts from it
			tn.editFor, tn.edit = f, it.color
		}
		if ui.ColorPicker(c.Key("tuner-c:"+it.key), &tn.edit).Changed() {
			if tn.colors[look] == nil {
				tn.colors[look] = map[string]ui.Color{}
			}
			tn.colors[look][it.key] = tn.edit
			poke()
		}
	}
	was := tunerItem{alpha: it.alpha, a: it.aB, color: it.base}
	ui.Text(c, locales.TunerWas(was.value())).FontSize(11.5).TextColor(cl.Muted)
	if it.changed() && ryugo.Button(c.Key("tuner-undo"), locales.TunerUndo(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
		delete(tn.colors[look], it.key)
		delete(tn.alphas[look], it.key)
		tn.edit, tn.editA = it.base, float64(it.aB)
		poke()
	}
}
