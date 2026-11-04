// A small Markdown editor with a jelly caret, built on MyGo's application-owned
// text input (HandleTextInput + TextLayout): the app owns the text, the
// selection and the painting, so the caret is whatever we draw.
// Run with: go run .
package main

import (
	"log"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const pad = 28

type doc struct {
	text      string
	selection ui.TextInputSelection
	marked    *ui.TextInputRange
	layout    *ui.TextLayout
	width     float32
	desiredX  float32 // kept while going up and down lines
	hasX      bool
	theme     *ui.Theme
	jelly     *slime
	quake     *quake
	mizan     *mizanCaret
	caret     caretKind
	wrote     int // letters written since the last frame, for the caret
	edited    bool
}

// shape lays the text out with light Markdown styling. The markers stay in the
// text (muted), so offsets in the layout are offsets in the text.
func (d *doc) shape() {
	t := d.theme
	text, muted, accent := t.Text, t.TextMuted, t.Accent
	var spans []ui.Span
	lines := strings.SplitAfter(d.text, "\n")
	for _, line := range lines {
		body := strings.TrimSuffix(line, "\n")
		nl := line[len(body):]
		size, weight := float32(17), 400
		switch {
		case strings.HasPrefix(body, "### "):
			size, weight = 19, 650
		case strings.HasPrefix(body, "## "):
			size, weight = 23, 680
		case strings.HasPrefix(body, "# "):
			size, weight = 30, 720
		}
		if weight > 400 {
			i := strings.Index(body, " ") + 1
			spans = append(spans, ui.Span{Text: body[:i], Size: size, Weight: weight, Color: muted})
			spans = append(spans, ui.Span{Text: body[i:] + nl, Size: size, Weight: weight, Color: text})
			continue
		}
		if strings.HasPrefix(body, "- ") || strings.HasPrefix(body, "> ") {
			spans = append(spans, ui.Span{Text: body[:2], Size: size, Color: accent})
			body = body[2:]
		}
		spans = append(spans, inline(body+nl, size, text, muted, t)...)
	}
	base, width := ui.Font{Size: 17, LineHeight: 1.55}, max(d.width-2*pad, 50)
	d.layout = ui.ShapeRichText(spans, base, width)
	if shockwaveOn {
		d.quake.build(d.text, spans, d.layout, base, width)
	}
}

// inline styles **bold** and `code` within a line.
func inline(s string, size float32, text, muted ui.Color, t *ui.Theme) []ui.Span {
	var out []ui.Span
	for s != "" {
		b, c := strings.Index(s, "**"), strings.Index(s, "`")
		switch {
		case b >= 0 && (c < 0 || b < c):
			end := strings.Index(s[b+2:], "**")
			if end < 0 {
				out = append(out, ui.Span{Text: s, Size: size, Color: text})
				return out
			}
			out = append(out,
				ui.Span{Text: s[:b], Size: size, Color: text},
				ui.Span{Text: "**", Size: size, Color: muted},
				ui.Span{Text: s[b+2 : b+2+end], Size: size, Weight: 700, Color: text},
				ui.Span{Text: "**", Size: size, Color: muted})
			s = s[b+4+end:]
		case c >= 0:
			end := strings.Index(s[c+1:], "`")
			if end < 0 {
				out = append(out, ui.Span{Text: s, Size: size, Color: text})
				return out
			}
			out = append(out,
				ui.Span{Text: s[:c], Size: size, Color: text},
				ui.Span{Text: s[c : c+2+end], Size: size - 1, Font: "monospace", Color: text, Background: t.Surface, Radius: 4})
			s = s[c+2+end:]
		default:
			out = append(out, ui.Span{Text: s, Size: size, Color: text})
			return out
		}
	}
	return out
}

// ---- ui.TextInputClient: the system's text services (typing, IME, menus) ----

func (d *doc) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	a, z := ui.UTF16ByteOffset(d.text, r.Start), ui.UTF16ByteOffset(d.text, r.End)
	return d.text[a:z], ui.TextInputRange{Start: ui.UTF16Len(d.text[:a]), End: ui.UTF16Len(d.text[:z])}
}
func (d *doc) Selection() ui.TextInputSelection { return d.selection }
func (d *doc) MarkedRange() (ui.TextInputRange, bool) {
	if d.marked != nil {
		return *d.marked, true
	}
	return ui.TextInputRange{}, false
}
func (d *doc) target(r *ui.TextInputRange) ui.TextInputRange {
	if r != nil {
		return *r
	}
	if d.marked != nil {
		return *d.marked
	}
	return d.selection.Range
}
func (d *doc) ReplaceText(r *ui.TextInputRange, text string) {
	w := d.target(r)
	a, z := ui.UTF16ByteOffset(d.text, w.Start), ui.UTF16ByteOffset(d.text, w.End)
	if shockwaveOn {
		power := wavePower
		if strings.Contains(text, "\n") {
			power *= 1.7 // Enter: a bigger wave
		}
		d.quake.edited(w.Start, w.End, ui.UTF16Len(text), power)
	}
	// What the caret hears of the edit: writing (letters on the line) or any other change.
	if text != "" && !strings.Contains(text, "\n") && w.Start == w.End {
		d.wrote += utf8.RuneCountInString(text)
	} else {
		d.edited = true
	}
	d.text = d.text[:a] + text + d.text[z:]
	end := w.Start + ui.UTF16Len(text)
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	d.marked, d.hasX = nil, false
	d.shape()
}
func (d *doc) SetMarkedText(r *ui.TextInputRange, text string, selected ui.TextInputRange) {
	w := d.target(r)
	d.ReplaceText(&w, text)
	mark := ui.TextInputRange{Start: w.Start, End: w.Start + ui.UTF16Len(text)}
	d.marked = &mark
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: mark.Start + selected.Start, End: mark.Start + selected.End}}
}
func (d *doc) UnmarkText() { d.marked = nil }
func (d *doc) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	b := d.layout.Caret(r.Start)
	b.X += pad
	b.Y += pad
	return b, r, true
}
func (d *doc) IndexForPoint(p ui.Point) (int, bool) {
	return d.layout.IndexAt(ui.Point{X: p.X - pad, Y: p.Y - pad}), true
}

// ---- keys and pointer ----

func (d *doc) move(to int, extend bool) {
	if !extend {
		d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: to, End: to}}
		return
	}
	anchor := d.selection.Range.Start
	if d.selection.Reversed {
		anchor = d.selection.Range.End
	}
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: min(anchor, to), End: max(anchor, to)}, Reversed: to < anchor}
}

func (d *doc) vertical(dir int, extend bool) {
	c := d.layout.Caret(d.selection.Caret())
	if !d.hasX {
		d.desiredX, d.hasX = c.X, true
	}
	y := c.Y - 2
	if dir > 0 {
		y = c.Y + c.H + 2
	}
	d.move(d.layout.IndexAt(ui.Point{X: d.desiredX, Y: y}), extend)
}

func (d *doc) input(ev ui.InputEvent) bool {
	shift := ev.Mods&ui.Shift != 0
	switch ev.Kind {
	case ui.InputPointerDown:
		to, _ := d.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
		d.hasX = false
		d.move(to, shift)
		return true
	case ui.InputPointerMove:
		if ev.Button == 0 {
			to, _ := d.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
			d.move(to, true)
			return true
		}
	case ui.InputPointerUp:
		return true
	case ui.InputKeyDown:
		r, caret := d.selection.Range, d.selection.Caret()
		u := unitOf(ev.Mods)
		switch ev.Key {
		case ui.KeyLeft, ui.KeyRight:
			d.hasX = false
			dir := 1
			if ev.Key == ui.KeyLeft {
				dir = -1
			}
			if r.Start != r.End && !shift && u == byChar { // a selection collapses to its edge
				edge := r.End
				if dir < 0 {
					edge = r.Start
				}
				d.move(edge, false)
			} else {
				d.move(d.step(caret, dir, u), shift)
			}
			return true
		case ui.KeyHome, ui.KeyEnd:
			d.hasX = false
			dir := 1
			if ev.Key == ui.KeyHome {
				dir = -1
			}
			if ev.Mods&ui.Ctrl != 0 { // Ctrl+Home/End: the whole document
				d.move(d.step(caret, dir, byDoc), shift)
			} else {
				d.move(d.step(caret, dir, byLine), shift)
			}
			return true
		case ui.KeyUp, ui.KeyDown:
			dir := 1
			if ev.Key == ui.KeyUp {
				dir = -1
			}
			if ev.Mods&ui.Cmd != 0 { // Cmd+Up/Down: the document's start or end
				d.hasX = false
				d.move(d.step(caret, dir, byDoc), shift)
			} else {
				d.vertical(dir, shift)
			}
			return true
		case ui.KeyM:
			if ev.Mods == ui.Cmd|ui.Shift {
				d.caret = (d.caret + 1) % 2
				return true
			}
		case ui.KeyEnter:
			d.ReplaceText(nil, "\n")
			return true
		case ui.KeyBackspace, ui.KeyDelete:
			if r.Start == r.End {
				dir := 1
				if ev.Key == ui.KeyBackspace {
					dir = -1
				}
				next := d.step(r.Start, dir, u) // Alt: a word, Cmd: to the line's edge
				r = ui.TextInputRange{Start: min(next, r.Start), End: max(next, r.End)}
			}
			d.ReplaceText(&r, "")
			return true
		}
	case ui.InputCommand:
		switch ev.Text {
		case "selectAll":
			d.selection = ui.TextInputSelection{Range: ui.TextInputRange{End: ui.UTF16Len(d.text)}}
		case "copy", "cut":
			s, _ := d.TextForRange(d.selection.Range)
			mygo.Clipboard.WriteText(s)
			if ev.Text == "cut" {
				d.ReplaceText(nil, "")
			}
		case "paste":
			d.ReplaceText(nil, mygo.Clipboard.ReadText())
		case "delete":
			d.ReplaceText(nil, "")
		default:
			return false
		}
		return true
	}
	return false
}

// caretBox is the caret's text box in the layout - from the line's ascent to
// its descent, not its leading, so the jelly is as tall as the letters - and
// the width of the letter under it.
func (d *doc) caretBox() (ui.Rect, float32) {
	i := d.selection.Caret()
	c := d.layout.Caret(i)
	mid := c.Y + c.H/2
	for _, l := range d.layout.Lines() {
		if mid >= l.Rect.Y && mid <= l.Rect.Y+l.Rect.H && l.Ascent > 0 {
			c.Y, c.H = l.Baseline-l.Ascent, l.Ascent+l.Descent
			break
		}
	}
	cw := c.H * 0.5
	if n := d.layout.NextBoundary(i); n > i {
		if nc := d.layout.Caret(n); nc.X > c.X && math.Abs(float64(nc.Y-d.layout.Caret(i).Y)) < 0.5 {
			if s, _ := d.TextForRange(ui.TextInputRange{Start: i, End: n}); s != "\n" {
				cw = nc.X - c.X
			}
		}
	}
	return c, cw
}

func (d *doc) view(c *ui.Context) {
	t := c.Theme()
	if d.theme != t {
		d.theme = t
		d.shape()
	}
	slimeColor := ui.Hex("#59f939")
	if !t.Dark {
		slimeColor = ui.Hex("#2f9a1c")
	}
	ui.Box(c).Fill().Background(t.Background).Label("Markdown").Role(ui.RoleTextField).Value(d.text).
		HandleTextInput(d).AutoFocus().
		HandleInput(d.input).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if r.W != d.width {
				d.width = r.W
				d.shape()
			}
			ox, oy := r.X+pad, r.Y+pad
			for _, b := range d.layout.SelectionRects(d.selection.Range) {
				b.X += ox
				b.Y += oy
				p.Fill(b, t.Selection, 3)
			}
			cr, cw := d.caretBox()
			if shockwaveOn {
				d.quake.step(p.Now(), float64(cr.X+cw/2), float64(cr.Y+cr.H/2))
			}
			if shockwaveOn && d.quake.busy() {
				d.quake.draw(p, ox, oy, slimeColor, p.Now())
			} else {
				p.TextLayout(d.layout, ox, oy, t.Text)
			}
			cr.X += ox
			cr.Y += oy
			f := caretFrame{r: cr, cw: cw, dark: t.Dark, bounds: r, wrote: d.wrote, edited: d.edited}
			d.wrote, d.edited = 0, false
			if d.caret == caretMizan {
				d.mizan.draw(p, f)
			} else {
				d.jelly.draw(p, cr, cw, slimeColor, t.Dark)
			}
			// Which caret, and how to change it.
			name := map[caretKind]string{caretSlime: "Slime", caretMizan: "Mizan"}[d.caret]
			p.RichText(r.X+r.W-190, r.Y+r.H-28, 0, ui.Span{Text: "Caret: " + name + "  ·  ⌘⇧M to switch", Size: 11.5, Color: t.TextMuted})
		})
}

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
)

// caretFrame is what a caret is told each frame: its text box and the width
// of the letter under it (absolute), the editor's box, and what happened.
type caretFrame struct {
	r      ui.Rect
	cw     float32
	dark   bool
	bounds ui.Rect
	wrote  int  // letters that went in at the caret
	edited bool // any other change: a deletion, a new line, a paste
}

const sample = `# Slime caret

A small **Markdown** editor on MyGo, with a caret made of jelly.

## Try it
- Type a few letters: it leans the way it goes.
- Press Enter or click far away: it squashes and is pulled across.
- Stop typing: it sways and breathes.

> The text, the selection and the painting belong to the app, so the caret is ` + "`whatever we draw`" + `.
`

func main() {
	d := &doc{text: sample, jelly: newSlime(), quake: newQuake(), mizan: &mizanCaret{}, width: 820}
	end := ui.UTF16Len(d.text)
	d.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: "Slime caret", Width: 820, Height: 560, Content: ui.View(d.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
