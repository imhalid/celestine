package main

import (
	"math"
	"reflect"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's themes, as Obsidian has them: a theme colors the whole window - the page,
// the sidebar, the fields, the lines, the text - not only the accent. A theme is the
// page's color, the text's and the accent, for the light appearance and for the dark;
// the rest of the grays (a card, a field, a line, a hover) are those two colors
// mixed in the steps Ryugo's own grays are, so that every component keeps the
// contrast it was made with.

type skinColors struct{ bg, text, accent string } // hex

type appTheme struct {
	name        string
	light, dark skinColors // empty: Ryugo's own
}

var appThemes = []appTheme{
	{name: "Varsayılan"},
	{"Dracula", skinColors{"#f8f8f2", "#282a36", "#7c4dcc"}, skinColors{"#282a36", "#f8f8f2", "#bd93f9"}},
	{"Nord", skinColors{"#eceff4", "#2e3440", "#5e81ac"}, skinColors{"#2e3440", "#e5e9f0", "#88c0d0"}},
	{"Solarized", skinColors{"#fdf6e3", "#586e75", "#268bd2"}, skinColors{"#002b36", "#93a1a1", "#268bd2"}},
	{"Gruvbox", skinColors{"#fbf1c7", "#3c3836", "#b57614"}, skinColors{"#282828", "#ebdbb2", "#d79921"}},
	{"Rosé Pine", skinColors{"#faf4ed", "#575279", "#907aa9"}, skinColors{"#191724", "#e0def4", "#c4a7e7"}},
	{"Tokyo Night", skinColors{"#e6e7ed", "#343b58", "#34548a"}, skinColors{"#1a1b26", "#c0caf5", "#7aa2f7"}},
	{"Catppuccin", skinColors{"#eff1f5", "#4c4f69", "#8839ef"}, skinColors{"#1e1e2e", "#cdd6f4", "#cba6f7"}},
	{"Sepya", skinColors{"#f4ecd8", "#433422", "#a0522d"}, skinColors{"#2b2118", "#e6d5b8", "#d08c4b"}},
	{"Orman", skinColors{"#eef3ec", "#1f2d22", "#2f7d4f"}, skinColors{"#14201a", "#d7e6da", "#5fbf85"}},
}

func (t appTheme) colors(dark bool) skinColors {
	if dark {
		return t.dark
	}
	return t.light
}

// appTheme is the theme in use.
func (a *app) appTheme() appTheme {
	return appThemes[a.settings.AppTheme%len(appThemes)]
}

// setAppTheme chooses theme i and keeps it.
func (a *app) setAppTheme(i int) {
	if i < 0 || i >= len(appThemes) || i == a.settings.AppTheme {
		return
	}
	a.settings.AppTheme = i
	a.saveSettings()
}

// coverTheme is the hues the made covers have in this theme: those of its accent, the far end a
// little turned. Nil for the default, where each page's seed picks its own.
func (t appTheme) coverTheme(dark bool) *coverTheme {
	p := t.colors(dark)
	if p.accent == "" {
		return nil
	}
	c := ui.Hex(p.accent)
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := max(r, g, b), min(r, g, b)
	var h float64
	switch d := mx - mn; {
	case d == 0:
		return &coverTheme{t.name, 220, 232, 0.2}
	case mx == r:
		h = math.Mod((g-b)/d, 6)
	case mx == g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h = math.Mod(h*60+360, 360)
	return &coverTheme{t.name, h, math.Mod(h+16, 360), 0.9}
}

// accentOf is the accent Ryugo is given for the appearance: it tells the scopes apart, too.
func (a *app) accentOf(dark bool) ui.Color {
	if p := a.appTheme().colors(dark); p.accent != "" {
		return ui.Hex(p.accent)
	}
	return ui.Color{}
}

// The colors of Ryugo's that are not grays: they keep their own, whatever the theme.
var keepColors = map[string]bool{
	"ColorAccent400": true, "ColorAccent500": true, "ColorAccent600": true, "ColorAccent700": true,
	"Accent": true, "FillSelected": true, "OnAccent": true, "OnAccentMuted": true, "Thumb": true,
	"Scrim": true, "ScrimSoft": true, "Dim": true, "Up": true, "Down": true, "Warn": true,
	"C1": true, "C2": true, "C3": true, "C4": true, "C5": true, "C6": true, "C7": true, "C8": true,
}

// skinned are the scopes (of Ryugo's cache) already in a theme's colors.
var skinned = map[*ryugo.Tokens]bool{}

// skin puts the theme's colors in the scope t and in its MyGo theme th, once: the scope is
// one of Ryugo's cached, one for each accent, so the next frame finds it done.
func (a *app) skin(t *ryugo.Tokens, th *ui.Theme) {
	p := a.appTheme().colors(t.Dark)
	if p.bg == "" || skinned[t] {
		return
	}
	skinned[t] = true
	bg, tx := ui.Hex(p.bg), ui.Hex(p.text)
	bg0, tx0 := t.Colors.Bg, t.Colors.Text
	l0, l1 := luma(bg0), luma(tx0)
	v := reflect.ValueOf(&t.Colors).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Type != reflect.TypeOf(ui.Color{}) || keepColors[v.Type().Field(i).Name] {
			continue
		}
		c := v.Field(i).Interface().(ui.Color)
		// where c lies between the page's color and the text's: the same place between the theme's
		f := (luma(c) - l0) / (l1 - l0)
		n := mixClamp(bg, tx, f)
		n.A = c.A
		v.Field(i).Set(reflect.ValueOf(n))
	}
	cl := t.Colors
	th.Background = cl.Bg
	th.Surface = cl.Well
	th.SurfaceHover = ryugo.Mix(cl.Well, cl.Text, 0.06)
	th.SurfacePressed = ryugo.Mix(cl.Well, cl.Text, 0.1)
	th.Border = cl.ControlBorder
	th.Text = cl.Text
	th.TextMuted = cl.Muted
}

func luma(c ui.Color) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

// mixClamp is a to b by f, which may lie outside 0 to 1; each channel stays in range.
func mixClamp(a, b ui.Color, f float64) ui.Color {
	ch := func(x, y uint8) uint8 {
		v := float64(x) + (float64(y)-float64(x))*f
		return uint8(min(max(v, 0), 255) + 0.5)
	}
	return ui.Color{R: ch(a.R, b.R), G: ch(a.G, b.G), B: ch(a.B, b.B), A: 255}
}

// appThemePicker is the settings' choice of the theme: a tile for each, with the window in its colors.
func (a *app) appThemePicker(c *ui.Context) {
	cl := ryugo.From(c).Colors
	dark := ryugo.From(c).Dark
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "Renk teması").FontSize(15).FontWeight(620).TextColor(cl.Text)
			ui.Text(c, "Bütün arayüzün rengi: sayfa, kenar çubuğu, alanlar, çizgiler ve vurgu. "+a.appTheme().name+" seçili.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
		})
		ui.Row(c).Gap(14).Wrap().Children(func() {
			for i, th := range appThemes {
				p := th.colors(dark)
				if choiceTile(c, "theme-tile:"+th.name, th.name, a.settings.AppTheme%len(appThemes) == i, func() { themeMock(c, p, dark, cl) }) {
					a.setAppTheme(i)
				}
			}
		})
	})
}

// themeMock is a little window in the colors of p (Ryugo's own for an empty one).
func themeMock(c *ui.Context, p skinColors, dark bool, own ryugo.Colors) {
	bg, fg, ac := own.Bg, own.Text, own.Accent
	if p.bg != "" {
		bg, fg, ac = ui.Hex(p.bg), ui.Hex(p.text), ui.Hex(p.accent)
	} else if dark != ryugo.From(c).Dark {
		bg, fg = ui.Hex("#f4f4f6"), ui.Hex("#1b1b1f")
	}
	ui.Row(c).Width(112).Height(66).Background(bg).Clip().Children(func() {
		ui.Box(c).Width(26).Height(66).Background(fg.Alpha(0.08)).Shrink(0)
		ui.Column(c).Padding(12, 9).Gap(6).Children(func() {
			ui.Box(c).Width(44).Height(6).Radius(3).Background(fg.Alpha(0.85))
			ui.Box(c).Width(58).Height(4).Radius(2).Background(fg.Alpha(0.35))
			ui.Box(c).Width(48).Height(4).Radius(2).Background(fg.Alpha(0.35))
			ui.Box(c).Width(30).Height(8).Radius(4).Background(ac)
		})
	})
}
