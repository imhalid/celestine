package main

import (
	"math"
	"reflect"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's themes, as Obsidian has them: a theme colors the whole window - the page,
// the sidebar, the fields, the lines, the text - not only the accent. A theme is the
// page's color, the text's and the accent, for the light appearance and for the dark;
// the rest of the grays (a card, a field, a line, a hover) are those two colors
// mixed in the steps Ryugo's own grays are, so that every component keeps the
// contrast it was made with.

// skinColors are the colors of a theme in one appearance (hex): the page, the text and the
// accent, and, when the theme has them, the sidebar's color, the cards' (menus, dialogs,
// the palette) and the lines'. What it leaves out is mixed from the page and the text.
type skinColors struct{ bg, text, accent, side, card, line string }

type appTheme struct {
	name        string
	light, dark skinColors // empty: Ryugo's own
}

var appThemes = []appTheme{
	{name: "Varsayılan"},
	{"Gece Yarısı",
		skinColors{bg: "#eef2ff", text: "#141a33", accent: "#3b5bdb", side: "#dde4fb", card: "#fafbff", line: "#c7d0f0"},
		skinColors{bg: "#0a0f1e", text: "#e8edff", accent: "#5b8cff", side: "#060a15", card: "#131a30", line: "#222c4a"}},
	{"Obsidyen",
		skinColors{bg: "#f6f3fb", text: "#1d1530", accent: "#7c3aed", side: "#e8e2f2", card: "#fdfcff", line: "#d6cdea"},
		skinColors{bg: "#0c0a11", text: "#eee9f7", accent: "#a45cff", side: "#07060b", card: "#16131e", line: "#2b2538"}},
	{"Kor",
		skinColors{bg: "#fbf3ec", text: "#2b1a10", accent: "#d9480f", side: "#f0e1d3", card: "#fffaf5", line: "#e0cbb8"},
		skinColors{bg: "#120e0c", text: "#f3e8de", accent: "#ff6a2b", side: "#0b0807", card: "#1e1714", line: "#35271f"}},
	{"Derin Deniz",
		skinColors{bg: "#e8f6f5", text: "#0c2a2c", accent: "#0b8f86", side: "#d3ebea", card: "#f6fcfc", line: "#b6dad8"},
		skinColors{bg: "#061518", text: "#d7f3f1", accent: "#1fe0d0", side: "#030d0f", card: "#0e2a2e", line: "#1a4046"}},
	{"Aurora",
		skinColors{bg: "#eef8f1", text: "#0f2a1b", accent: "#1a9b57", side: "#dcefe2", card: "#f9fdfa", line: "#c0dccb"},
		skinColors{bg: "#08120d", text: "#def6e8", accent: "#42e08a", side: "#040a07", card: "#10201a", line: "#1d3629"}},
	{"Mürekkep",
		skinColors{bg: "#f6f8fa", text: "#1f2328", accent: "#0969da", side: "#e8ecf1", card: "#ffffff", line: "#d0d7de"},
		skinColors{bg: "#0b0e14", text: "#e6edf3", accent: "#58a6ff", side: "#06080c", card: "#141922", line: "#252d3a"}},
	{"Kiraz",
		skinColors{bg: "#fbf0f1", text: "#3a1f27", accent: "#c4284f", side: "#f1dcdf", card: "#fff8f9", line: "#e3c6cb"},
		skinColors{bg: "#150a0e", text: "#f8e3ea", accent: "#ff4f7b", side: "#0e0509", card: "#23111a", line: "#3a1c28"}},
	{"Kehribar",
		skinColors{bg: "#fcf6e6", text: "#2e2410", accent: "#b8860b", side: "#f1e6c8", card: "#fffbf0", line: "#e3d4a8"},
		skinColors{bg: "#130f08", text: "#f5e9d0", accent: "#f7b928", side: "#0c0905", card: "#201a0e", line: "#382d19"}},
	{"Grafit",
		skinColors{bg: "#f2f2f3", text: "#1c1c1f", accent: "#e8590c", side: "#e2e2e5", card: "#ffffff", line: "#cfcfd4"},
		skinColors{bg: "#0e0e10", text: "#ededf0", accent: "#ff8a3d", side: "#080809", card: "#18181b", line: "#2b2b30"}},
	{"Gruvbox",
		skinColors{bg: "#fbf1c7", text: "#3c3836", accent: "#d65d0e", side: "#ebdbb2", card: "#fdf6d3", line: "#d5c4a1"},
		skinColors{bg: "#1d2021", text: "#ebdbb2", accent: "#fabd2f", side: "#141617", card: "#282828", line: "#3c3836"}},
	{"Sepya",
		skinColors{bg: "#f4ecd8", text: "#3b2c1b", accent: "#a0431f", side: "#e8dcbe", card: "#fbf5e6", line: "#d4c5a0"},
		skinColors{bg: "#1a130d", text: "#ecdcc0", accent: "#e0954a", side: "#120d08", card: "#271d14", line: "#3e3022"}},
	{"Orman",
		skinColors{bg: "#eef3ec", text: "#1b2b20", accent: "#1f8a4c", side: "#dce8d9", card: "#f9fcf7", line: "#c4d6c0"},
		skinColors{bg: "#0a130e", text: "#d9e8dd", accent: "#4fd18b", side: "#060c09", card: "#13211a", line: "#22382c"}},
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
	// what the theme gives itself: the cards, the lines
	put := func(name string, c ui.Color) {
		if f := v.FieldByName(name); f.IsValid() {
			c.A = f.Interface().(ui.Color).A
			f.Set(reflect.ValueOf(c))
		}
	}
	if p.card != "" {
		card := ui.Hex(p.card)
		put("Card", card)
		put("Surface", card)
		put("Surface2", ryugo.Mix(card, bg, 0.5))
	}
	if p.line != "" {
		line := ui.Hex(p.line)
		put("Hairline", line)
		put("Border", line)
		put("WellBorder", line)
		put("ControlBorder", ryugo.Mix(line, tx, 0.14))
	}
	cl := t.Colors
	side := cl.Well
	if p.side != "" {
		side = ui.Hex(p.side)
	}
	th.Background = cl.Bg
	th.Surface = side
	th.SurfaceHover = ryugo.Mix(side, cl.Text, 0.07)
	th.SurfacePressed = ryugo.Mix(side, cl.Text, 0.12)
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
			ui.Text(c, locales.CommonColorTheme()).FontSize(15).FontWeight(620).TextColor(cl.Text)
			ui.Text(c, locales.ThemesTheColorWholeInterface()+T(a.appTheme().name)+locales.ThemesIsSelected()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
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
	side := fg.Alpha(0.08)
	if p.bg != "" {
		bg, fg, ac = ui.Hex(p.bg), ui.Hex(p.text), ui.Hex(p.accent)
		side = fg.Alpha(0.08)
		if p.side != "" {
			side = ui.Hex(p.side)
		}
	} else if dark != ryugo.From(c).Dark {
		bg, fg = ui.Hex("#f4f4f6"), ui.Hex("#1b1b1f")
		side = fg.Alpha(0.08)
	}
	ui.Row(c).Width(112).Height(66).Background(bg).Clip().Children(func() {
		ui.Box(c).Width(26).Height(66).Background(side).Shrink(0).Children(func() {
			ui.Box(c).Width(14).Height(5).Radius(rad(2)).Background(ac).Margin(14, 0, 0, 6)
		})
		ui.Column(c).Padding(12, 9).Gap(6).Children(func() {
			ui.Box(c).Width(44).Height(6).Radius(rad(3)).Background(fg.Alpha(0.85))
			ui.Box(c).Width(58).Height(4).Radius(rad(2)).Background(fg.Alpha(0.35))
			ui.Box(c).Width(48).Height(4).Radius(rad(2)).Background(fg.Alpha(0.35))
			ui.Box(c).Width(30).Height(8).Radius(rad(4)).Background(ac)
		})
	})
}
