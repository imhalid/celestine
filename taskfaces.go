package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The faces of the task boxes are BetterTask's (#611), the Thymer plugin's design taken whole: each status's box
// with its shape, border, gradient back and glow, its glyph (BetterTask's own vectors, in its 24-unit box) filled
// with its gradient ink, in a dark and a light version, its look when done, its look in Vivid Solid and in Minimal
// Crisp, and its small motions. What CSS draws there with borders, masks and box shadows is drawn here with paths,
// clips and shadows; the measures are BetterTask's, for a box 16 points wide, and scale with the box.

// glyphPart is a part of a glyph: its path, filled, or stroked stroke units wide with round ends.
type glyphPart struct {
	path   *ui.Path
	stroke float32
}

// glyph is one of BetterTask's glyphs, in its 24-unit box, moved down dy units.
type glyph struct {
	parts []glyphPart
	dy    float32
}

// glyphOf makes a glyph of its parts, each path data after its kind: "f " filled, "e " filled even-odd, "s2.6 "
// stroked 2.6 wide.
func glyphOf(dy float32, parts ...string) glyph {
	g := glyph{dy: dy}
	for _, s := range parts {
		kind, d, _ := strings.Cut(s, " ")
		path := ui.MustParsePath(d)
		gp := glyphPart{path: path}
		switch {
		case kind == "e":
			path.FillRule(ui.EvenOdd)
		case strings.HasPrefix(kind, "s"):
			w, err := strconv.ParseFloat(kind[1:], 32)
			if err != nil {
				panic("taskfaces: " + s)
			}
			gp.stroke = float32(w)
			path.Cap(ui.RoundCap).Join(ui.RoundJoin)
		}
		g.parts = append(g.parts, gp)
	}
	return g
}

// rrectD is the path data of a rectangle with round corners of radius r.
func rrectD(x, y, w, h, r float64) string {
	return fmt.Sprintf("M%g %gH%gA%g %g 0 0 1 %g %gV%gA%g %g 0 0 1 %g %gH%gA%g %g 0 0 1 %g %gV%gA%g %g 0 0 1 %g %gZ",
		x+r, y, x+w-r, r, r, x+w, y+r, y+h-r, r, r, x+w-r, y+h, x+r, r, r, x, y+h-r, y+r, r, r, x+r, y)
}

// circ is the path data of a circle.
func circ(cx, cy, r float64) string {
	return fmt.Sprintf("M%g %gA%g %g 0 1 0 %g %gA%g %g 0 1 0 %g %gZ", cx-r, cy, r, r, cx+r, cy, r, r, cx-r, cy)
}

// BetterTask's glyphs (plugin.css, --bt-ico-* and --bt-tag-ico-*).
var (
	glyphCheck   = glyphOf(0, "s3.3 M5 12.7L9.8 17.5L19 7.2")
	glyphStarted = glyphOf(0, "f M7.3 6.6C7.3 5.7 8.3 5.15 9.05 5.65L16.95 10.95C17.6 11.38 17.6 12.62 16.95 13.05L9.05 18.35C8.3 18.85 7.3 18.3 7.3 17.4V6.6Z")
	glyphBlocked = glyphOf(0, "f "+rrectD(6.5, 5.5, 4.1, 13, 2), "f "+rrectD(13.4, 5.5, 4.1, 13, 2))
	glyphDollar  = glyphOf(0, "s2.6 M12 3.5V20.5M16.2 7.8C15.6 6.4 14 5.6 12 5.6C9.5 5.6 7.8 6.9 7.8 8.7C7.8 10.6 9.5 11.4 12 12C14.5 12.6 16.4 13.5 16.4 15.4C16.4 17.3 14.6 18.5 12 18.5C9.8 18.5 8.1 17.6 7.5 16.1")
	glyphExclaim = glyphOf(0, "f "+rrectD(10.3, 4.1, 3.4, 10.3, 1.7), "f "+circ(12, 18.3, 1.95))
	glyphQuery   = glyphOf(0, "s2.8 M8.5 8.6C8.5 6.4 10.1 4.9 12.2 4.9C14.3 4.9 15.8 6.3 15.8 8.2C15.8 10 14.3 10.9 13 11.8C12.3 12.3 12 12.9 12 13.8V14.3", "f "+circ(12, 18.4, 1.8))
	glyphStar    = glyphOf(0, "f "+starPath, "s1.3 "+starPath)
	glyphSlash   = glyphOf(0, "s2.9 M6.4 17.6L17.6 6.4")
	glyphSpark   = glyphOf(0, "f M12 2L14.4 9.6L22 12L14.4 14.4L12 22L9.6 14.4L2 12L9.6 9.6Z")
	glyphHeart   = glyphOf(0, "f M12 21C5 15.6 2.5 12.3 2.5 8.6C2.5 5.6 4.9 3.4 7.6 3.4C9.5 3.4 11 4.4 12 6C13 4.4 14.5 3.4 16.4 3.4C19.1 3.4 21.5 5.6 21.5 8.6C21.5 12.3 19 15.6 12 21Z")
	glyphBug     = glyphOf(-0.4,
		"s1.8 M10.4 5.9L8.6 3.2M13.6 5.9L15.4 3.2",
		"s1.8 M7 11.9L3.6 10.4M6.7 15.2L3.2 15.2M7.3 18.4L4.3 20.2M17 11.9L20.4 10.4M17.3 15.2L20.8 15.2M16.7 18.4L19.7 20.2",
		"f "+circ(12, 7.5, 2.5),
		"e M11.3 9.3C8.3 9.5 6.6 11.9 6.6 14.9C6.6 18.3 8.6 20.7 11.3 20.9ZM8.15 14.9a1.3 1.3 0 1 0 2.6 0a1.3 1.3 0 1 0 -2.6 0Z",
		"e M12.7 9.3C15.7 9.5 17.4 11.9 17.4 14.9C17.4 18.3 15.4 20.7 12.7 20.9ZM13.25 14.9a1.3 1.3 0 1 0 2.6 0a1.3 1.3 0 1 0 -2.6 0Z")
	glyphDraft = glyphOf(0,
		"e M4.3 19.7L5.4 15.1L15.3 5.2C16.1 4.4 17.4 4.4 18.2 5.2L18.8 5.8C19.6 6.6 19.6 7.9 18.8 8.7L8.9 18.6ZM14.1 7.6L16.4 9.9L17.3 9L15 6.7Z",
		"s1.6 M12.6 20.2C14.2 19.2 15.6 21 17.2 20C18.1 19.4 18.9 19.6 19.8 20")
	glyphDeadline = glyphOf(0,
		"s2.2 M6 3.4H18M6 20.6H18",
		"s1.7 M7.6 3.8C7.6 8.6 10.9 10.2 10.9 12C10.9 13.8 7.6 15.4 7.6 20.2M16.4 3.8C16.4 8.6 13.1 10.2 13.1 12C13.1 13.8 16.4 15.4 16.4 20.2",
		"s0.9 M12 10.8V16",
		"f M9.3 7.2H14.7C14.3 8.8 12.8 9.9 12 10.6C11.2 9.9 9.7 8.8 9.3 7.2Z",
		"f M8.7 19.3C9.3 17.1 10.9 16.2 12 15.8C13.1 16.2 14.7 17.1 15.3 19.3Z")
	glyphSun = glyphOf(0, "f "+circ(12, 12, 4.6),
		"s2.1 M18.9 12L21.7 12M16.88 16.88L18.86 18.86M12 18.9L12 21.7M7.12 16.88L5.14 18.86M5.1 12L2.3 12M7.12 7.12L5.14 5.14M12 5.1L12 2.3M16.88 7.12L18.86 5.14")
	glyphMoon = glyphOf(0, "f M10.54 4.61A8.2 8.2 0 1 0 18.95 14.8A6.7 6.7 0 0 1 10.54 4.61Z",
		"f M18.6 2.6L19.25 4.35L21 5L19.25 5.65L18.6 7.4L17.95 5.65L16.2 5L17.95 4.35Z")
)

const starPath = "M12 3.2L14.65 8.55L20.55 9.4L16.28 13.55L17.28 19.42L12 16.65L6.72 19.42L7.72 13.55L3.45 9.4L9.35 8.55L12 3.2Z"

// BetterTask's drawings in their own colors: the alert's triangle, the kawaii face and the flower.
var (
	artAlert  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><defs><linearGradient id="g" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#fb923c"/><stop offset="100%" stop-color="#ef4444"/></linearGradient></defs><path d="M10.45 4.2C11.15 2.95 12.85 2.95 13.55 4.2L21.3 18.2C21.98 19.42 21.1 21 19.7 21H4.3C2.9 21 2.02 19.42 2.7 18.2L10.45 4.2Z" fill="url(#g)"/><rect x="10.7" y="9" width="2.6" height="6.1" rx="1.3" fill="#ffffff"/><circle cx="12" cy="17.8" r="1.4" fill="#ffffff"/></svg>`))
	artKawaii = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><defs><radialGradient id="f" cx="0.38" cy="0.32" r="0.8"><stop offset="0" stop-color="#fff1f8"/><stop offset="1" stop-color="#f9a8d4"/></radialGradient></defs><path d="M5.2 7.4 L4.6 3.2 L8.6 5.2 Z M18.8 7.4 L19.4 3.2 L15.4 5.2 Z" fill="#f472b6"/><circle cx="12" cy="13" r="9" fill="url(#f)" stroke="#ec4899" stroke-width="1.1"/><path d="M7.4 12.1 q1.35 -1.9 2.7 0 M13.9 12.1 q1.35 -1.9 2.7 0" fill="none" stroke="#4a1d3d" stroke-width="1.45" stroke-linecap="round"/><ellipse cx="6.9" cy="15" rx="1.7" ry="1.05" fill="#fb7185" opacity="0.75"/><ellipse cx="17.1" cy="15" rx="1.7" ry="1.05" fill="#fb7185" opacity="0.75"/><path d="M10.5 15.2 q0.75 0.9 1.5 0 q0.75 0.9 1.5 0" fill="none" stroke="#4a1d3d" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	artFlower = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><defs><linearGradient id="p" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fdf4ff"/><stop offset="1" stop-color="#f0abfc"/></linearGradient></defs>` +
		`<ellipse cx="12" cy="6.7" rx="3.2" ry="4.3" transform="rotate(0 12 12)" fill="url(#p)" stroke="#d946ef" stroke-width="0.8"/>` +
		`<ellipse cx="12" cy="6.7" rx="3.2" ry="4.3" transform="rotate(72 12 12)" fill="url(#p)" stroke="#d946ef" stroke-width="0.8"/>` +
		`<ellipse cx="12" cy="6.7" rx="3.2" ry="4.3" transform="rotate(144 12 12)" fill="url(#p)" stroke="#d946ef" stroke-width="0.8"/>` +
		`<ellipse cx="12" cy="6.7" rx="3.2" ry="4.3" transform="rotate(216 12 12)" fill="url(#p)" stroke="#d946ef" stroke-width="0.8"/>` +
		`<ellipse cx="12" cy="6.7" rx="3.2" ry="4.3" transform="rotate(288 12 12)" fill="url(#p)" stroke="#d946ef" stroke-width="0.8"/>` +
		`<circle cx="12" cy="12" r="3.1" fill="#fde047" stroke="#f59e0b" stroke-width="0.9"/><circle cx="11.1" cy="11.1" r="0.8" fill="#fffbeb"/></svg>`))
)

// paint is what a box or a glyph is filled with: a color (from), or a gradient from from to to, linear at a CSS
// angle, or radial from cx, cy (in fractions of the box) to its farthest corner.
type paint struct {
	from, to ui.Color
	angle    float32
	radial   bool
	cx, cy   float32
}

func (pt paint) zero() bool { return pt.from.A == 0 && pt.to.A == 0 }

func (pt paint) linear() ui.LinearGradient {
	return ui.LinearGradient{From: pt.from, To: pt.to, Angle: pt.angle}
}

// lin is a linear gradient at a CSS angle.
func lin(angle float32, from, to string) paint {
	return paint{from: ui.Hex(from), to: ui.Hex(to), angle: angle}
}

// radial is a radial gradient from cx, cy.
func radial(cx, cy float32, from, to ui.Color) paint {
	return paint{from: from, to: to, radial: true, cx: cx, cy: cy}
}

// rgba is a CSS rgba().
func rgba(r, g, b uint8, a float32) ui.Color { return ui.Color{R: r, G: g, B: b, A: 255}.Alpha(a) }

// glow is a box shadow: its color and its blur.
type glow struct {
	c    ui.Color
	blur float32
}

// faceLook is a box in one state: its shape (round, or with its corners' radii, clockwise from the top left), its
// border, back and glow, a light along its top edge, and a coin's inner ring.
type faceLook struct {
	round  bool
	radii  [4]float32
	dashed bool
	border ui.Color
	back   paint
	glow   glow
	gloss  float32 // the light's alpha
	glossy bool    // the light the theme gives: brighter on a light page
	coin   ui.Color
}

// faceSpec is a status's look: its box, glyph and ink (dark and light), or its drawing in its own colors; its look
// done, in Vivid Solid and in Minimal Crisp; its tint along the row, and its colors.
type faceSpec struct {
	key       string
	box       faceLook
	borderL   ui.Color // in a light theme, when another
	backL     paint
	glyph     *glyph
	inset     float32 // the glyph's margin, a fraction of the box
	ink, inkL paint
	art       *ui.SVG
	artInset  float32
	drop      glow
	done      faceLook
	vivid     faceLook // its border, back and glow; zero keeps the box's
	minInset  float32  // the glyph's margin in Minimal Crisp
	tint      ui.Color
	chip      ui.Color // its tag's color, dark and light
	chipL     ui.Color
	colors    [3]ui.Color // its burst's
}

func sq(r float32) [4]float32 { return [4]float32{r, r, r, r} }

func hex3(a, b, c string) [3]ui.Color { return [3]ui.Color{ui.Hex(a), ui.Hex(b), ui.Hex(c)} }

// faceOpen is a plain task's box.
var faceOpen = faceSpec{key: "open",
	box:     faceLook{radii: sq(5.5), border: rgba(148, 163, 184, 0.55), back: paint{from: rgba(255, 255, 255, 0.03)}, glossy: true},
	borderL: rgba(71, 85, 105, 0.42), backL: paint{from: rgba(15, 23, 42, 0.03)},
	done:   faceLook{radii: sq(5.5), border: ui.Hex("#10b981"), back: lin(145, "#34d399", "#059669"), glow: glow{rgba(16, 185, 129, 0.36), 9}, gloss: 0.35},
	colors: hex3("#34d399", "#10b981", "#a7f3d0"),
}

// faceSpecs are the kinds' looks, by the kind's id.
var faceSpecs = map[string]*faceSpec{
	"progress": {key: "started",
		box:   faceLook{round: true, border: ui.Hex("#22d3ee"), back: radial(0.3, 0.25, rgba(56, 189, 248, 0.28), rgba(6, 182, 212, 0.14)), glow: glow{rgba(34, 211, 238, 0.28), 10}, glossy: true},
		glyph: &glyphStarted, inset: 0.15, ink: lin(145, "#bae6fd", "#22d3ee"), inkL: lin(145, "#0284c7", "#06b6d4"),
		done:  faceLook{round: true, border: ui.Hex("#22d3ee"), back: lin(145, "#22d3ee", "#0284c7"), glow: glow{rgba(34, 211, 238, 0.34), 8}, gloss: 0.35},
		vivid: faceLook{border: ui.Hex("#0ea5e9"), back: lin(145, "#38bdf8", "#0284c7"), glow: glow{rgba(14, 165, 233, 0.42), 10}},
		tint:  rgba(34, 211, 238, 0.085), colors: hex3("#38bdf8", "#22d3ee", "#bae6fd"),
	},
	"blocked": {key: "blocked",
		box:   faceLook{radii: sq(5), dashed: true, border: ui.Hex("#f59e0b"), back: radial(0.3, 0.25, rgba(251, 191, 36, 0.24), rgba(245, 158, 11, 0.10)), glow: glow{rgba(245, 158, 11, 0.22), 8}},
		glyph: &glyphBlocked, inset: 0.15, ink: lin(180, "#fde68a", "#f59e0b"), inkL: lin(180, "#d97706", "#b45309"),
		done:  faceLook{radii: sq(5), border: ui.Hex("#f59e0b"), back: lin(145, "#fbbf24", "#d97706"), glow: glow{rgba(245, 158, 11, 0.32), 8}, gloss: 0.35},
		vivid: faceLook{border: ui.Hex("#f59e0b"), back: lin(145, "#fbbf24", "#d97706"), glow: glow{rgba(245, 158, 11, 0.4), 10}},
		tint:  rgba(245, 158, 11, 0.075), colors: hex3("#fbbf24", "#f59e0b", "#fde68a"),
	},
	"billable": {key: "dollar",
		box:   faceLook{round: true, border: ui.Hex("#4ade80"), back: radial(0.3, 0.25, rgba(163, 230, 53, 0.28), rgba(22, 163, 74, 0.16)), glow: glow{rgba(74, 222, 128, 0.28), 9}, coin: rgba(250, 204, 21, 0.28)},
		glyph: &glyphDollar, inset: 0.12, ink: lin(155, "#d9f99d", "#22c55e"), inkL: lin(155, "#16a34a", "#65a30d"),
		done:  faceLook{round: true, border: ui.Hex("#4ade80"), back: lin(145, "#4ade80", "#16a34a"), glow: glow{rgba(74, 222, 128, 0.34), 8}, coin: rgba(253, 224, 71, 0.52)},
		vivid: faceLook{border: ui.Hex("#22c55e"), back: lin(145, "#4ade80", "#15803d"), glow: glow{rgba(34, 197, 94, 0.4), 10}},
		tint:  rgba(74, 222, 128, 0.07), colors: hex3("#4ade80", "#a3e635", "#fde047"),
	},
	"important": {key: "exclaim",
		box:   faceLook{radii: sq(5), border: ui.Hex("#fb7185"), back: paint{from: rgba(244, 63, 94, 0.34), to: rgba(225, 29, 72, 0.16), angle: 145}, glow: glow{rgba(244, 63, 94, 0.34), 10}, glossy: true},
		glyph: &glyphExclaim, inset: 0.11, ink: lin(180, "#ffe4e6", "#fb7185"), inkL: lin(180, "#e11d48", "#be123c"),
		done:  faceLook{radii: sq(5), border: ui.Hex("#fb7185"), back: lin(145, "#fb7185", "#e11d48"), glow: glow{rgba(244, 63, 94, 0.36), 9}, gloss: 0.32},
		vivid: faceLook{border: ui.Hex("#f43f5e"), back: lin(145, "#fb7185", "#e11d48"), glow: glow{rgba(244, 63, 94, 0.45), 10}},
		tint:  rgba(244, 63, 94, 0.09), colors: hex3("#fb7185", "#f43f5e", "#fecdd3"),
	},
	"question": {key: "question",
		box:   faceLook{radii: [4]float32{7, 7, 7, 2}, border: ui.Hex("#a78bfa"), back: paint{from: rgba(167, 139, 250, 0.26), to: rgba(124, 58, 237, 0.14), angle: 145}, glow: glow{rgba(139, 92, 246, 0.28), 9}, glossy: true},
		glyph: &glyphQuery, inset: 0.12, ink: lin(155, "#ede9fe", "#c084fc"), inkL: lin(155, "#7c3aed", "#6d28d9"),
		done:  faceLook{radii: [4]float32{7, 7, 7, 2}, border: ui.Hex("#a78bfa"), back: lin(145, "#a78bfa", "#6d28d9"), glow: glow{rgba(139, 92, 246, 0.34), 8}, gloss: 0.32},
		vivid: faceLook{border: ui.Hex("#8b5cf6"), back: lin(145, "#a78bfa", "#6d28d9"), glow: glow{rgba(139, 92, 246, 0.42), 10}},
		tint:  rgba(167, 139, 250, 0.08), colors: hex3("#c084fc", "#a78bfa", "#ede9fe"),
	},
	"error": {key: "alert",
		art: artAlert, artInset: -0.05, drop: glow{rgba(239, 68, 68, 0.58), 5},
		done: faceLook{radii: sq(5), border: ui.Hex("#f97316"), back: lin(145, "#fb923c", "#dc2626"), glow: glow{rgba(239, 68, 68, 0.38), 9}, gloss: 0.34},
		tint: rgba(239, 68, 68, 0.11), colors: hex3("#ef4444", "#f97316", "#fde047"),
	},
	"star": {key: "starred",
		box:   faceLook{round: true, border: rgba(250, 204, 21, 0.65), back: radial(0.35, 0.3, rgba(254, 240, 138, 0.28), rgba(234, 179, 8, 0.12)), glow: glow{rgba(250, 204, 21, 0.34), 10}, gloss: 0.25},
		glyph: &glyphStar, inset: 0.12, ink: lin(150, "#fef08a", "#f59e0b"), inkL: lin(150, "#f59e0b", "#d97706"),
		done:  faceLook{round: true, border: ui.Hex("#facc15"), back: lin(145, "#fde047", "#d97706"), glow: glow{rgba(250, 204, 21, 0.38), 10}, gloss: 0.45},
		vivid: faceLook{border: ui.Hex("#f59e0b"), back: lin(145, "#fde047", "#d97706"), glow: glow{rgba(245, 158, 11, 0.45), 12}},
		tint:  rgba(250, 204, 21, 0.09), colors: hex3("#fde047", "#facc15", "#f59e0b"),
	},
	"canceled": {key: "canceled",
		box:   faceLook{radii: sq(5), border: rgba(148, 163, 184, 0.42), back: paint{from: rgba(148, 163, 184, 0.10)}},
		glyph: &glyphSlash, inset: 0.15, ink: lin(135, "#cbd5e1", "#64748b"), inkL: lin(135, "#64748b", "#475569"),
		done:   faceLook{radii: sq(5), border: ui.Hex("#64748b"), back: lin(145, "#94a3b8", "#475569"), gloss: 0.25},
		vivid:  faceLook{border: ui.Hex("#64748b"), back: lin(145, "#94a3b8", "#475569")},
		colors: hex3("#94a3b8", "#cbd5e1", "#64748b"),
	},
}

// tagSpecs are the tag statuses' looks, by slot (tagStatusSlots).
var tagSpecs = [7]*faceSpec{
	{key: "kawaii",
		art: artKawaii, artInset: -0.10, drop: glow{rgba(244, 114, 182, 0.45), 3},
		done: faceLook{round: true, border: ui.Hex("#f472b6"), back: lin(145, "#f9a8d4", "#db2777"), glow: glow{rgba(244, 114, 182, 0.42), 9}, gloss: 0.4},
		tint: rgba(244, 114, 182, 0.09), chip: ui.Hex("#f472b6"), chipL: ui.Hex("#db2777"), colors: hex3("#f9a8d4", "#f472b6", "#c4b5fd"),
	},
	{key: "bug",
		box:     faceLook{radii: sq(6), border: ui.Hex("#84cc16"), back: paint{from: rgba(163, 230, 53, 0.24), to: rgba(101, 163, 13, 0.10), angle: 145}, glow: glow{rgba(132, 204, 22, 0.3), 8}, glossy: true},
		borderL: ui.Hex("#65a30d"), glyph: &glyphBug, inset: 0.09, ink: lin(180, "#ecfccb", "#bef264"), inkL: lin(160, "#65a30d", "#3f6212"),
		done:  faceLook{radii: sq(6), border: ui.Hex("#84cc16"), back: lin(145, "#a3e635", "#4d7c0f"), glow: glow{rgba(132, 204, 22, 0.36), 8}, gloss: 0.35},
		vivid: faceLook{back: lin(145, "#a3e635", "#4d7c0f")}, minInset: 0.02,
		tint: rgba(132, 204, 22, 0.08), chip: ui.Hex("#84cc16"), chipL: ui.Hex("#4d7c0f"), colors: hex3("#a3e635", "#65a30d", "#d9f99d"),
	},
	{key: "draft",
		box:     faceLook{radii: [4]float32{3, 8, 3, 3}, border: ui.Hex("#60a5fa"), back: paint{from: rgba(147, 197, 253, 0.24), to: rgba(59, 130, 246, 0.08), angle: 145}, glow: glow{rgba(96, 165, 250, 0.28), 8}, glossy: true},
		borderL: ui.Hex("#2563eb"), glyph: &glyphDraft, inset: 0.10, ink: lin(150, "#dbeafe", "#60a5fa"), inkL: lin(150, "#3b82f6", "#1d4ed8"),
		done:  faceLook{radii: [4]float32{3, 8, 3, 3}, border: ui.Hex("#3b82f6"), back: lin(145, "#60a5fa", "#1d4ed8"), glow: glow{rgba(59, 130, 246, 0.36), 8}, gloss: 0.35},
		vivid: faceLook{back: lin(145, "#60a5fa", "#1d4ed8")}, minInset: 0.02,
		tint: rgba(96, 165, 250, 0.08), chip: ui.Hex("#60a5fa"), chipL: ui.Hex("#1d4ed8"), colors: hex3("#93c5fd", "#3b82f6", "#dbeafe"),
	},
	{key: "deadline",
		box:     faceLook{radii: sq(5), border: ui.Hex("#f97316"), back: paint{from: rgba(251, 146, 60, 0.26), to: rgba(234, 88, 12, 0.10), angle: 145}, glow: glow{rgba(249, 115, 22, 0.32), 9}, glossy: true},
		borderL: ui.Hex("#ea580c"), glyph: &glyphDeadline, inset: 0.09, ink: lin(180, "#fed7aa", "#f97316"), inkL: lin(180, "#f97316", "#c2410c"),
		done:  faceLook{radii: sq(5), border: ui.Hex("#f97316"), back: lin(145, "#fb923c", "#c2410c"), glow: glow{rgba(249, 115, 22, 0.38), 9}, gloss: 0.35},
		vivid: faceLook{back: lin(145, "#fb923c", "#c2410c")}, minInset: 0.02,
		tint: rgba(249, 115, 22, 0.10), chip: ui.Hex("#fb923c"), chipL: ui.Hex("#c2410c"), colors: hex3("#fb923c", "#f97316", "#fed7aa"),
	},
	{key: "flower",
		art: artFlower, artInset: -0.12, drop: glow{rgba(232, 121, 249, 0.45), 3},
		done: faceLook{round: true, border: ui.Hex("#e879f9"), back: lin(145, "#f0abfc", "#a21caf"), glow: glow{rgba(232, 121, 249, 0.42), 9}, gloss: 0.4},
		tint: rgba(232, 121, 249, 0.08), chip: ui.Hex("#e879f9"), chipL: ui.Hex("#a21caf"), colors: hex3("#f0abfc", "#e879f9", "#fde68a"),
	},
	{key: "sun",
		box:     faceLook{round: true, border: ui.Hex("#fbbf24"), back: radial(0.35, 0.3, rgba(253, 224, 71, 0.32), rgba(251, 146, 60, 0.12)), glow: glow{rgba(251, 191, 36, 0.42), 10}},
		borderL: ui.Hex("#f59e0b"), glyph: &glyphSun, inset: 0.05, ink: lin(150, "#fef08a", "#fb923c"), inkL: lin(150, "#f59e0b", "#ea580c"),
		done:  faceLook{round: true, border: ui.Hex("#f59e0b"), back: lin(145, "#fde047", "#ea580c"), glow: glow{rgba(251, 191, 36, 0.45), 10}, gloss: 0.45},
		vivid: faceLook{back: lin(145, "#fde047", "#ea580c")}, minInset: 0.02,
		tint: rgba(251, 191, 36, 0.10), chip: ui.Hex("#f59e0b"), chipL: ui.Hex("#b45309"), colors: hex3("#fde047", "#fb923c", "#fef3c7"),
	},
	{key: "moon",
		box:     faceLook{round: true, border: ui.Hex("#a5b4fc"), back: radial(0.3, 0.25, rgba(165, 180, 252, 0.26), rgba(49, 46, 129, 0.24)), glow: glow{rgba(129, 140, 248, 0.34), 9}, glossy: true},
		borderL: ui.Hex("#6366f1"), backL: radial(0.3, 0.25, rgba(199, 210, 254, 0.5), rgba(99, 102, 241, 0.14)),
		glyph: &glyphMoon, inset: 0.08, ink: lin(150, "#eef2ff", "#a5b4fc"), inkL: lin(150, "#818cf8", "#4338ca"),
		done:  faceLook{round: true, border: ui.Hex("#818cf8"), back: lin(145, "#a5b4fc", "#4338ca"), glow: glow{rgba(129, 140, 248, 0.4), 9}, gloss: 0.35},
		vivid: faceLook{back: lin(145, "#818cf8", "#312e81")}, minInset: 0.02,
		tint: rgba(129, 140, 248, 0.09), chip: ui.Hex("#a5b4fc"), chipL: ui.Hex("#4338ca"), colors: hex3("#c7d2fe", "#a5b4fc", "#e0e7ff"),
	},
}

// spec is the look of face f.
func (f taskFace) spec() *faceSpec {
	if f.id == "tag" && f.slot >= 0 && f.slot < len(tagSpecs) {
		return tagSpecs[f.slot]
	}
	if s, ok := faceSpecs[f.id]; ok {
		return s
	}
	return &faceOpen
}

// darkBg reports whether c is a dark background.
func darkBg(c ui.Color) bool {
	return 0.2126*float32(c.R)+0.7152*float32(c.G)+0.0722*float32(c.B) < 128
}

// ---- drawn ----

// kf is the value at phase k (0 to 1) of keyframes, each {at, value}, eased in and out between them, as CSS's
// ease-in-out does.
func kf(k float64, keys ...[2]float64) float64 {
	if k <= keys[0][0] {
		return keys[0][1]
	}
	for i := 1; i < len(keys); i++ {
		if k <= keys[i][0] {
			a, b := keys[i-1], keys[i]
			u := (k - a[0]) / (b[0] - a[0])
			u = u * u * (3 - 2*u)
			return a[1] + (b[1]-a[1])*u
		}
	}
	return keys[len(keys)-1][1]
}

// boxPath adds the outline of a box at r with corners of radii (round: a circle) to path.
func boxPath(path *ui.Path, r ui.Rect, radii [4]float32, round bool) *ui.Path {
	if round {
		return path.Circle(r.X+r.W/2, r.Y+r.H/2, r.W/2)
	}
	const k = 0.5522847498
	lim := min(r.W, r.H) / 2
	tl, tr, br, bl := min(radii[0], lim), min(radii[1], lim), min(radii[2], lim), min(radii[3], lim)
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	path.MoveTo(x0+tl, y0).LineTo(x1-tr, y0)
	path.CubeTo(x1-tr+tr*k, y0, x1, y0+tr-tr*k, x1, y0+tr).LineTo(x1, y1-br)
	path.CubeTo(x1, y1-br+br*k, x1-br+br*k, y1, x1-br, y1).LineTo(x0+bl, y1)
	path.CubeTo(x0+bl-bl*k, y1, x0, y1-bl+bl*k, x0, y1-bl).LineTo(x0, y0+tl)
	path.CubeTo(x0, y0+tl-tl*k, x0+tl-tl*k, y0, x0+tl, y0)
	return path.Close()
}

// grow is r grown by d on each side.
func grow(r ui.Rect, d float32) ui.Rect {
	return ui.Rect{X: r.X - d, Y: r.Y - d, W: r.W + 2*d, H: r.H + 2*d}
}

// scaled is radii times k, plus d.
func scaled(radii [4]float32, k, d float32) [4]float32 {
	for i := range radii {
		radii[i] = max(0, radii[i]*k+d)
	}
	return radii
}

// fillPaint fills path, the outline of a box at r, with pt.
func fillPaint(p *ui.Painter, path *ui.Path, r ui.Rect, pt paint) {
	switch {
	case pt.zero():
	case pt.radial:
		p.ClipPath(path, func() {
			p.FillRadialGradient(r, ui.RadialGradient{Center: ui.Point{X: pt.cx, Y: pt.cy}, Stops: []ui.GradientStop{{Color: pt.from}, {Color: pt.to, At: 1}}}, 0)
		})
	case pt.to.A == 0 && pt.angle == 0:
		p.FillPath(path, pt.from)
	default:
		p.FillPathGradient(path, pt.linear())
	}
}

// drawGlyph draws glyph g in r with ink.
func drawGlyph(p *ui.Painter, g *glyph, r ui.Rect, ink paint) {
	k := r.W / 24
	p.Transform(ui.Translation(r.X, r.Y+g.dy*k), 0, 0, func() {
		p.Transform(ui.Scaling(k, k), 0, 0, func() {
			for _, gp := range g.parts {
				solid := ink.to.A == 0 && ink.angle == 0
				switch {
				case gp.stroke > 0 && solid:
					p.StrokePath(gp.path, gp.stroke*k, ink.from)
				case gp.stroke > 0:
					p.StrokePathGradient(gp.path, gp.stroke*k, ink.linear())
				case solid:
					p.FillPath(gp.path, ink.from)
				default:
					p.FillPathGradient(gp.path, ink.linear())
				}
			}
		})
	})
}

// faceClock is the seconds the ambient motions are at now, or -1 when they are off.
func (a *app) faceClock(now time.Time) float64 {
	if a.settings.TaskStill || stillNow() {
		return -1
	}
	return float64(now.UnixNano()%int64(time.Hour)) / 1e9
}

// drawFace draws the face of a task not done, at the box at: squashed (at.H below at.W) as it turns over.
func (a *app) drawFace(p *ui.Painter, at ui.Rect, f taskFace, now time.Time) {
	a.drawBox(p, at, f, false, now)
}

// drawFaceBack draws a task done: its status's done look, a white tick in it.
func (a *app) drawFaceBack(p *ui.Painter, at ui.Rect, f taskFace, now time.Time) {
	a.drawBox(p, at, f, true, now)
}

// drawBox draws a task's box as BetterTask does, done or not.
func (a *app) drawBox(p *ui.Painter, at ui.Rect, f taskFace, done bool, now time.Time) {
	s := at.W
	if s <= 0 || at.H <= 0 {
		return
	}
	r := ui.Rect{X: at.X, Y: at.Y + (at.H-s)/2, W: s, H: s}
	cx, cy := r.X+s/2, r.Y+s/2
	draw := func() { a.drawBoxIn(p, r, f, done, now) }
	if sy := at.H / s; sy < 0.999 || sy > 1.001 {
		p.Transform(ui.Scaling(1, sy), cx, cy, draw)
		return
	}
	draw()
}

// drawBoxIn draws a task's box in the square r.
func (a *app) drawBoxIn(p *ui.Painter, r ui.Rect, f taskFace, done bool, now time.Time) {
	sp := f.spec()
	dark := darkBg(a.pageBg)
	look := a.settings.TaskLook
	t := a.faceClock(now)
	k := r.W / 16 // BetterTask's measures are for a box 16 points wide
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	phase := func(period float64) float64 { return math.Mod(t, period) / period }

	if done {
		if look == taskLookMinimal { // a tick alone, in the status's colors
			ink := sp.done.back
			if ink.zero() {
				ink = faceOpen.done.back
			}
			drawGlyph(p, &glyphCheck, grow(r, -r.W*0.06), ink)
			if sp.key == "starred" {
				drawSpark(p, r, k, 6, 2, ui.Hex("#fef08a").Alpha(0.9), 1, 0)
			}
			return
		}
		lk := sp.done
		a.drawShell(p, r, k, lk, lk.border, lk.back, lk.glow, false, dark)
		in := grow(r, -r.W*0.14)
		p.Transform(ui.Translation(0, k), 0, 0, func() { drawGlyph(p, &glyphCheck, in, paint{from: rgba(0, 0, 0, 0.2)}) })
		drawGlyph(p, &glyphCheck, in, paint{from: ui.Hex("#ffffff")})
		if sp.key == "starred" {
			drawSpark(p, r, k, 6, 2, ui.Hex("#fef08a").Alpha(0.9), 1, 0)
		}
		return
	}

	if sp.key == "open" {
		border, back := sp.box.border, sp.box.back
		if !dark {
			border, back = sp.borderL, sp.backL
		}
		a.drawShell(p, r, k, sp.box, border, back, glow{}, false, dark)
		return
	}

	if sp.art != nil { // a drawing in its own colors, glowing round it
		sx, sc, rot, dy := float32(1), float32(1), float32(0), float32(0)
		dropC, dropB := sp.drop.c, sp.drop.blur
		if t >= 0 {
			switch sp.key {
			case "alert":
				q := phase(1.9)
				sc = f32(kf(q, [2]float64{0, 1}, [2]float64{0.5, 1.09}, [2]float64{1, 1}))
				sx = sc
				w := f32(kf(q, [2]float64{0, 0}, [2]float64{0.5, 1}, [2]float64{1, 0}))
				dropC, dropB = mix(rgba(239, 68, 68, 0.55), rgba(249, 115, 22, 0.85), float64(w)), 4+4*w
			case "kawaii": // a bounce: squashed, then up and stretched
				q := phase(2.4)
				dy = f32(kf(q, [2]float64{0, 0}, [2]float64{0.18, 0.6}, [2]float64{0.38, -1.6}, [2]float64{0.56, 0}, [2]float64{1, 0}))
				sx = f32(kf(q, [2]float64{0, 1}, [2]float64{0.18, 1.06}, [2]float64{0.38, 0.96}, [2]float64{0.56, 1.03}, [2]float64{0.7, 1}, [2]float64{1, 1}))
				sc = f32(kf(q, [2]float64{0, 1}, [2]float64{0.18, 0.92}, [2]float64{0.38, 1.05}, [2]float64{0.56, 0.97}, [2]float64{0.7, 1}, [2]float64{1, 1}))
			case "flower":
				q := phase(3.4)
				rot = f32(kf(q, [2]float64{0, -9}, [2]float64{0.5, 9}, [2]float64{1, -9}))
				sc = f32(kf(q, [2]float64{0, 1}, [2]float64{0.5, 1.07}, [2]float64{1, 1}))
				sx = sc
			}
		}
		ar := grow(r, -r.W*sp.artInset)
		if dropC.A > 0 {
			p.Shadow(grow(r, -r.W*0.18), r.W*0.32, 0, 0, dropB*k*1.6, 0, dropC)
		}
		if sp.key == "alert" && t >= 0 { // its radar, going out
			q := phase(2)
			e := 1 - math.Pow(1-min(q/0.8, 1), 3)
			ring := grow(r, 2*k-0.75*k)
			p.Transform(ui.Scaling(f32(0.75+0.77*e), f32(0.75+0.77*e)), cx, cy, func() {
				p.StrokePath(new(ui.Path).Circle(cx, cy, ring.W/2), 1.5*k, rgba(249, 115, 22, 0.72).Alpha(f32(0.72*(1-e))/0.72))
			})
		}
		tr := ui.Scaling(sx, sc).Then(ui.Rotation(rot)).Then(ui.Translation(0, dy*k))
		p.Transform(tr, cx, cy, func() { p.Image(sp.art, ar, ui.Contain) })
		if sp.key == "kawaii" && t >= 0 { // a heart, floating up from its corner
			q := phase(2.8)
			op := kf(q, [2]float64{0, 0}, [2]float64{0.25, 1}, [2]float64{0.7, 0.85}, [2]float64{1, 0})
			hy := f32(kf(q, [2]float64{0, 2}, [2]float64{0.25, 0}, [2]float64{1, -6}))
			hs := f32(kf(q, [2]float64{0, 0.3}, [2]float64{0.25, 1}, [2]float64{1, 0.75}))
			hr := ui.Rect{X: r.X + r.W + 5*k - 7*k, Y: r.Y - 5*k, W: 7 * k, H: 7 * k}
			p.Transform(ui.Scaling(hs, hs).Then(ui.Translation(0, hy*k)), hr.X+hr.W/2, hr.Y+hr.H/2, func() {
				drawGlyph(p, &glyphHeart, hr, paint{from: ui.Hex("#f472b6").Alpha(f32(op))})
			})
		}
		return
	}

	// a box: its shell, unless Minimal Crisp leaves the glyph bare
	lk := sp.box
	border, back, gl := lk.border, lk.back, lk.glow
	if !dark {
		if sp.borderL.A > 0 {
			border = sp.borderL
		}
		if !sp.backL.zero() {
			back = sp.backL
		}
	}
	ink := sp.ink
	if !dark && !sp.inkL.zero() {
		ink = sp.inkL
	}
	inset := sp.inset
	switch look {
	case taskLookVivid:
		if sp.vivid.border.A > 0 {
			border = sp.vivid.border
		}
		if !sp.vivid.back.zero() {
			back = sp.vivid.back
		}
		if sp.vivid.glow.c.A > 0 || sp.key == "canceled" {
			gl = sp.vivid.glow
		}
		lk.dashed = false
		ink = paint{from: ui.Hex("#ffffff")}
	case taskLookMinimal:
		inset = 0.04
		if sp.minInset > 0 {
			inset = sp.minInset
		}
	}
	if look != taskLookMinimal {
		if sp.key == "sun" && t >= 0 { // its warm halo, breathing
			q := phase(2.8)
			sc := f32(kf(q, [2]float64{0, 0.9}, [2]float64{0.5, 1.18}, [2]float64{1, 0.9}))
			op := f32(kf(q, [2]float64{0, 0.45}, [2]float64{0.5, 0.9}, [2]float64{1, 0.45}))
			hr := grow(r, 5*k)
			p.Transform(ui.Scaling(sc, sc), cx, cy, func() {
				p.FillRadialGradient(hr, ui.RadialGradient{Radius: hr.W / 2 * 1.414, Stops: []ui.GradientStop{
					{Color: rgba(253, 224, 71, 0.38*op)}, {Color: rgba(251, 146, 60, 0.12*op), At: 0.55}, {Color: rgba(251, 146, 60, 0), At: 0.72}}}, hr.W/2)
			})
		}
		a.drawShell(p, r, k, lk, border, back, gl, look != taskLookVivid && lk.dashed, dark)
	}

	// what goes round it
	if t >= 0 {
		switch sp.key {
		case "started": // an arc orbiting it
			ang := phase(2.2) * 2 * math.Pi
			rr := r.W/2 + 3*k - 0.75*k
			arc := func(from, to float64, c ui.Color) {
				var path ui.Path
				for i := 0; i <= 12; i++ {
					a := ang + from + (to-from)*float64(i)/12
					x, y := cx+rr*f32(math.Cos(a)), cy+rr*f32(math.Sin(a))
					if i == 0 {
						path.MoveTo(x, y)
					} else {
						path.LineTo(x, y)
					}
				}
				p.StrokePath(path.Cap(ui.RoundCap), 1.5*k, c)
			}
			arc(-0.75*math.Pi, -0.25*math.Pi, ui.Hex("#67e8f9").Alpha(0.9))
			arc(-0.25*math.Pi, 0.25*math.Pi, rgba(34, 211, 238, 0.35*0.9))
		case "blocked", "exclaim": // a ring pulsing out of it
			period, c := 3.0, rgba(251, 191, 36, 0.55)
			if sp.key == "exclaim" {
				period, c = 2.4, rgba(251, 113, 133, 0.6)
			}
			q := phase(period)
			e := min(q/0.7, 1)
			e = 1 - (1-e)*(1-e)
			sc := f32(0.92 + 0.46*e)
			op := f32(0.65 * (1 - e))
			ring := grow(r, 2*k-0.75*k)
			p.Transform(ui.Scaling(sc, sc), cx, cy, func() {
				p.StrokePath(boxPath(new(ui.Path), ring, scaled(lk.radii, k, 0), lk.round), 1.5*k, c.Alpha(op))
			})
		case "deadline": // a dashed ring, still: the hourglass inside is the motion
			ring := grow(r, 4*k-0.75*k)
			path := boxPath(new(ui.Path), ring, sq(9*k), false)
			p.StrokePath(path.Dash(0, 3*k, 2.2*k), 1.5*k, rgba(251, 146, 60, 0.7*0.9))
		}
	}

	// the glyph, and its motion
	if sp.glyph != nil {
		sc, rot, dx, dy := float32(1), float32(0), float32(0), float32(0)
		if t >= 0 {
			switch sp.key {
			case "started":
				q := kf(phase(1.8), [2]float64{0, 0}, [2]float64{0.5, 1}, [2]float64{1, 0})
				sc, dx = 1+0.06*f32(q), 0.5*f32(q)
			case "blocked":
				sc = f32(kf(phase(2.6), [2]float64{0, 1}, [2]float64{0.5, 0.92}, [2]float64{1, 1}))
			case "exclaim":
				sc = f32(kf(phase(2.4), [2]float64{0, 1}, [2]float64{0.12, 1.16}, [2]float64{0.24, 0.96}, [2]float64{0.36, 1.10}, [2]float64{0.6, 1}, [2]float64{1, 1}))
			case "question":
				rot = f32(kf(phase(3.1), [2]float64{0, -6}, [2]float64{0.5, 7}, [2]float64{1, -6}))
			case "starred":
				q := kf(phase(2.8), [2]float64{0, 0}, [2]float64{0.5, 1}, [2]float64{1, 0})
				sc, rot = 1+0.11*f32(q), 5*f32(q)
			case "bug":
				rot = f32(kf(phase(2.8), [2]float64{0, 0}, [2]float64{0.58, 0}, [2]float64{0.63, -13}, [2]float64{0.70, 11}, [2]float64{0.77, -8}, [2]float64{0.84, 5}, [2]float64{0.90, 0}, [2]float64{1, 0}))
			case "draft":
				q := phase(1.7)
				dx = f32(kf(q, [2]float64{0, 0}, [2]float64{0.25, -0.7}, [2]float64{0.5, 0.7}, [2]float64{0.75, -0.3}, [2]float64{1, 0}))
				dy = f32(kf(q, [2]float64{0, 0}, [2]float64{0.25, 0.5}, [2]float64{0.5, -0.3}, [2]float64{0.75, 0.6}, [2]float64{1, 0}))
				rot = f32(kf(q, [2]float64{0, 0}, [2]float64{0.25, -5}, [2]float64{0.5, 4}, [2]float64{0.75, -3}, [2]float64{1, 0}))
			case "deadline":
				rot = f32(kf(phase(4.8), [2]float64{0, 0}, [2]float64{0.38, 0}, [2]float64{0.5, 180}, [2]float64{0.88, 180}, [2]float64{1, 360}))
			case "sun":
				rot = f32(phase(12) * 360)
			case "moon":
				rot = f32(kf(phase(4), [2]float64{0, -10}, [2]float64{0.5, 7}, [2]float64{1, -10}))
			}
		}
		in := grow(r, -r.W*inset)
		tr := ui.Scaling(sc, sc).Then(ui.Rotation(rot)).Then(ui.Translation(dx*k, dy*k))
		p.Transform(tr, cx, cy, func() { drawGlyph(p, sp.glyph, in, ink) })
	}

	// what is over it
	switch {
	case sp.key == "dollar" && t >= 0 && look != taskLookMinimal: // a light going over the coin now and then
		q := phase(3.6)
		if q > 0.58 && q < 0.86 {
			u := f32((q - 0.58) / 0.28)
			op := f32(kf(q, [2]float64{0.58, 0}, [2]float64{0.68, 0.9}, [2]float64{0.86, 0}))
			p.ClipPath(new(ui.Path).Circle(cx, cy, r.W/2), func() {
				x := r.X - 1.4*r.W + u*2.8*r.W
				var band ui.Path
				w := r.W * 0.44
				band.MoveTo(x, r.Y-r.W*0.2).LineTo(x+w, r.Y-r.W*0.2).LineTo(x+w-r.W*0.56, r.Y+r.H*1.2).LineTo(x-r.W*0.56, r.Y+r.H*1.2).Close()
				p.FillPathGradient(&band, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{Color: rgba(255, 255, 255, 0)}, {Color: rgba(255, 255, 255, 0.52*op), At: 0.5}, {Color: rgba(255, 255, 255, 0), At: 1}}})
			})
		}
	case sp.key == "starred" && t >= 0:
		q := kf(phase(2.4), [2]float64{0, 0}, [2]float64{0.5, 1}, [2]float64{1, 0})
		drawSpark(p, r, k, 6, 2, ui.Hex("#fef08a").Alpha(f32(0.25+0.75*q)), f32(0.35+0.8*q), f32(45*q))
	case sp.key == "moon" && t >= 0:
		q := kf(phase(3), [2]float64{0, 0}, [2]float64{0.5, 1}, [2]float64{1, 0})
		c := ui.Hex("#e0e7ff")
		if !dark {
			c = ui.Hex("#6366f1")
		}
		drawSpark(p, r, k, 5, 3, c.Alpha(f32(0.25+0.75*q)), f32(0.35+0.8*q), f32(45*q))
	}
}

// drawShell draws a box's shell in the square r, k points to BetterTask's one: its glow, back, a coin's inner ring,
// the light along its top edge and its border, dashed when dashed is set.
func (a *app) drawShell(p *ui.Painter, r ui.Rect, k float32, lk faceLook, border ui.Color, back paint, gl glow, dashed, dark bool) {
	radii := scaled(lk.radii, k, 0)
	if gl.c.A > 0 {
		rr := radii[0]
		if lk.round {
			rr = r.W / 2
		}
		p.Shadow(r, rr, 0, 0, gl.blur*k, 0, gl.c)
	}
	outer := boxPath(new(ui.Path), r, radii, lk.round)
	fillPaint(p, outer, r, back)
	bw := 1.6 * k
	if lk.coin.A > 0 {
		in := grow(r, -bw-0.5*k)
		p.StrokePath(boxPath(new(ui.Path), in, radii, lk.round), 1.2*k, lk.coin)
	}
	gloss := lk.gloss
	if lk.glossy {
		gloss = map[bool]float32{true: 0.22, false: 0.65}[dark]
	}
	if gloss > 0 { // inset 0 1px 0: a crescent of light along the inner top edge
		in := grow(r, -bw)
		ir := scaled(lk.radii, k, -bw)
		inner := boxPath(new(ui.Path), in, ir, lk.round)
		both := boxPath(new(ui.Path), in, ir, lk.round)
		boxPath(both, ui.Rect{X: in.X, Y: in.Y + k, W: in.W, H: in.H}, ir, lk.round)
		p.ClipPath(inner, func() { p.FillPath(both.FillRule(ui.EvenOdd), rgba(255, 255, 255, gloss)) })
	}
	if border.A > 0 {
		edge := boxPath(new(ui.Path), grow(r, -bw/2), scaled(lk.radii, k, -bw/2), lk.round)
		if dashed {
			edge.Dash(0, 3*k, 2*k)
		}
		p.StrokePath(edge, bw, border)
	}
}

// drawSpark draws a sparkle size points wide, out from the top right corner of the box r by out points, scaled by
// sc and turned by rot degrees.
func drawSpark(p *ui.Painter, r ui.Rect, k, size, out float32, c ui.Color, sc, rot float32) {
	sr := ui.Rect{X: r.X + r.W + out*k - size*k, Y: r.Y - out*k, W: size * k, H: size * k}
	p.Transform(ui.Scaling(sc, sc).Then(ui.Rotation(rot)), sr.X+sr.W/2, sr.Y+sr.H/2, func() {
		drawGlyph(p, &glyphSpark, sr, paint{from: c})
	})
}

// rowTint draws the soft tint of a task's status along its line, as BetterTask's row mood tint: from where its text
// starts, after its box at, fading out over a little more than half the way to right.
func rowTint(p *ui.Painter, at ui.Rect, right float32, c ui.Color) {
	x := at.X + at.W + 3
	r := ui.Rect{X: x, Y: at.Y - at.H*0.3, W: max(0, right-x), H: at.H * 1.6}
	p.FillGradient(r, ui.LinearGradient{Angle: 90, From: c, To: c.Alpha(0), End: 0.55}, 6)
}

// burstOf is the colors of a status's burst as its task is done, and the shape of its bits: 0 a strip of paper, 1 a
// star, 2 a round glint.
func burstOf(f taskFace) ([]ui.Color, int) {
	sp := f.spec()
	cs := []ui.Color{sp.colors[0], sp.colors[1], sp.colors[2], ui.Hex("#ffffff")}
	switch sp.key {
	case "starred", "sun", "moon", "kawaii":
		return cs, 1
	case "dollar", "open":
		return cs, 2
	}
	return cs, 0
}
