package main

import (
	_ "embed"
	"log"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// The phone's look is its own, not the desktop's: blocks of rounded paper on a warm ground by day, of soft ink on a
// deep one by night; Geist for the words, heavy for what is big; a color for each collection; the blue of the icon
// for what is chosen. Blocks press in a little under a finger, with their corners kept.

//go:embed fonts/Geist.ttf
var geistFont []byte

// fontFamily is the app's typeface.
const fontFamily = "Geist"

// registerFonts gives the text engine the app's typeface. It is called from main, not at init: on Android the text
// engine is the host's, which is not there while the library loads.
func registerFonts() {
	if err := ui.RegisterFont(geistFont, fontFamily); err != nil {
		log.Printf("font %s: %v", fontFamily, err)
	}
}

// palette is one look's colors.
type palette struct {
	bg, block, raised, line, text, muted, faint, accent, accentText, danger ui.Color
}

var (
	lightPalette = palette{
		bg: ui.Hex("#F2EFE9"), block: ui.Hex("#FFFFFF"), raised: ui.Hex("#E9E5DD"), line: ui.Hex("#E4DFD6"),
		text: ui.Hex("#17161A"), muted: ui.Hex("#6F6B64"), faint: ui.Hex("#A39E95"), accent: ui.Hex("#3D6EE8"),
		accentText: ui.Hex("#FFFFFF"), danger: ui.Hex("#D1453B"),
	}
	darkPalette = palette{
		bg: ui.Hex("#0D0E11"), block: ui.Hex("#18191E"), raised: ui.Hex("#22242A"), line: ui.Hex("#26282E"),
		text: ui.Hex("#EEEBE6"), muted: ui.Hex("#9C988F"), faint: ui.Hex("#66635D"), accent: ui.Hex("#86A6FF"),
		accentText: ui.Hex("#0D1220"), danger: ui.Hex("#FF7B6E"),
	}
)

// pal is the palette of the frame being built, set with its theme; dark says which.
var (
	pal  = lightPalette
	dark bool
)

// tints are the colors of collections that were given none, in turn.
var tints = []ui.Color{ui.Hex("#5B8DEF"), ui.Hex("#E9825B"), ui.Hex("#4FB38A"), ui.Hex("#B07BE0"), ui.Hex("#E3B33F"),
	ui.Hex("#E26D9A"), ui.Hex("#3FB3C4")}

// themeOf is the window's theme in the light or the dark look.
func themeOf(isDark bool) *ui.Theme {
	t := ui.LightTheme()
	pal, dark = lightPalette, isDark
	if isDark {
		t = ui.DarkTheme()
		pal = darkPalette
	}
	p := pal
	t.Font, t.FontSize = fontFamily, 16
	t.Background, t.Surface, t.Border = p.bg, p.block, p.line
	t.SurfaceHover, t.SurfacePressed = p.raised, p.raised
	t.Text, t.TextMuted = p.text, p.muted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText, t.InactiveAccent = p.accent, p.accent, p.accent, p.accentText, p.accent
	t.Danger = p.danger
	t.Selection = p.accent.Alpha(0.25)
	t.Focus = p.accent.Alpha(0.5)
	return t
}

// textTheme is how a page's text looks in the editor: room between paragraphs, links and the caret in the accent.
func textTheme() *richtext.Theme {
	return &richtext.Theme{Text: pal.text, Muted: pal.muted, Link: pal.accent, Caret: pal.accent,
		Selection: pal.accent.Alpha(0.25), CodeBackground: pal.raised, CodeBlockBackground: pal.raised, ParagraphSpacing: 0.55}
}

// Block sizes.
const (
	side    = 18 // the room at a screen's sides
	gap     = 12 // between two blocks
	blockR  = 26 // a block's corners
	smallR  = 18 // a small block's
	pressIn = 0.965
)

// block is a rounded block of the screen, in color bg, that presses in under a finger.
func block(c *ui.Context, key any, bg ui.Color, r float32) ui.Element {
	e := ui.Column(c.Key(key)).Radius(r).Background(bg)
	return pressable(e)
}

// pressable has e press in a little while a finger is on it, its corners kept.
func pressable(e ui.Element) ui.Element {
	target := float32(1)
	if e.Pressed() {
		target = pressIn
	}
	s := e.Animate("press", target, 140*time.Millisecond)
	return e.Scale(s, s)
}

// shadow is the soft shadow under what floats.
func shadow(e ui.Element) ui.Element {
	a := float32(0.16)
	if dark {
		a = 0.5
	}
	return e.Shadow(0, 10, 28, -6, ui.RGB(0, 0, 0).Alpha(a))
}
