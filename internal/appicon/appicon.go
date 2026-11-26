// Package appicon is the app's icon as one drawing in several colorways (#601): two crystal carets as stickers on a
// ground, which SVG writes in a Look's colors. The app draws the look chosen for its Dock and window (appicon.go),
// and scripts/iconsvg writes assets/icon.svg and the brand's versions from the same drawing. It knows nothing of the
// UI: what it makes is SVG text.
package appicon

import (
	"fmt"
	"strings"
)

// A Look is a colorway of the icon.
type Look struct {
	ID       string    // as the settings keep it
	Ground   [2]string // the ground, top to bottom
	Rim      [2]string // the stickers' rims, top to bottom
	Fill     [3]string // the carets, from their top left to their bottom right
	Shadow   string    // the rims' shadow
	ShadowOp float64   // and how strong it is
	Glow     float64   // the glow of the carets' color behind them, 0 for none
	Dark     bool      // a dark icon: what the Auto mode shows in a dark window
}

// Looks are the icon's colorways, Paper first: the icon as it always was.
var Looks = []Look{
	{ID: "paper", Ground: [2]string{"#F7F6F2", "#F7F6F2"}, Rim: [2]string{"#FFFFFF", "#FFFFFF"},
		Fill: [3]string{"#DCF0FF", "#8CC4F5", "#3E86E0"}, Shadow: "#000000", ShadowOp: 0.10},
	{ID: "dark", Ground: [2]string{"#1A2033", "#0E1220"}, Rim: [2]string{"#323C55", "#252D42"},
		Fill: [3]string{"#E4F3FF", "#93C8F7", "#4A90EA"}, Shadow: "#000000", ShadowOp: 0.45, Glow: 0.35, Dark: true},
	{ID: "blue", Ground: [2]string{"#3478E6", "#2353BE"}, Rim: [2]string{"#FFFFFF", "#FFFFFF"},
		Fill: [3]string{"#DCF0FF", "#8CC4F5", "#3E86E0"}, Shadow: "#0A1F55", ShadowOp: 0.35},
	{ID: "violet", Ground: [2]string{"#2B2050", "#160F2E"}, Rim: [2]string{"#3F3369", "#2D2451"},
		Fill: [3]string{"#F2E8FF", "#C6A6FA", "#8B5DF0"}, Shadow: "#000000", ShadowOp: 0.45, Glow: 0.35, Dark: true},
	{ID: "emerald", Ground: [2]string{"#1FAF7A", "#0E7F57"}, Rim: [2]string{"#FFFFFF", "#F1FBF6"},
		Fill: [3]string{"#E4FFF4", "#93E6C4", "#24B07C"}, Shadow: "#04402A", ShadowOp: 0.35},
	{ID: "amber", Ground: [2]string{"#FFC85A", "#F29A1A"}, Rim: [2]string{"#FFFFFF", "#FFF8EC"},
		Fill: [3]string{"#FFF3D6", "#FFCF73", "#EE9512"}, Shadow: "#6B3D00", ShadowOp: 0.30},
	{ID: "graphite", Ground: [2]string{"#3A3B41", "#222327"}, Rim: [2]string{"#4B4D55", "#36383F"},
		Fill: [3]string{"#FFFFFF", "#D3D6DC", "#979CA7"}, Shadow: "#000000", ShadowOp: 0.45, Glow: 0.22, Dark: true},
}

// ByID is the look of id, and whether there is one.
func ByID(id string) (Look, bool) {
	for _, l := range Looks {
		if l.ID == id {
			return l, true
		}
	}
	return Look{}, false
}

// Parts are the layers of the drawing, for a renderer without SVG's filters to draw and blur one by one.
type Parts uint8

const (
	PartGround Parts = 1 << iota // the ground
	PartGlow                     // the glow behind the carets: their shapes, in the carets' last color
	PartRims                     // the stickers' rims
	PartCarets                   // the carets
	PartAll    = PartGround | PartGlow | PartRims | PartCarets
)

// The drawing's geometry, in its 1024 units: the stickers (rims) and the carets in them, two of each.
var (
	rims   = [2][4]int{{237, 322, 250, 460}, {537, 262, 250, 520}}
	carets = [2][4]int{{277, 362, 170, 380}, {577, 302, 170, 440}}
)

const (
	rimR   = 105
	caretR = 65
	inset  = 100 // the drawing is moved this far in from the ground's corner
)

// Shadow and glow, as SVG's filters draw them, in the drawing's units: what a renderer without filters reproduces.
const (
	ShadowDY    = 14
	ShadowBlur  = 20
	GlowBlur    = 28
	GroundUnits = 1024
	Inset       = inset
)

// SVG is the icon in look l: the parts asked for, and with filters the shadow and glow as SVG filters (for a
// renderer that has them, as librsvg); without, those layers are plain shapes for the caller to blur (PartGlow,
// PartRims).
func SVG(l Look, parts Parts, filters bool) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">` + "\n")
	b.WriteString("  <defs>\n")
	fmt.Fprintf(&b, `    <linearGradient id="ground" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`+"\n", l.Ground[0], l.Ground[1])
	fmt.Fprintf(&b, `    <linearGradient id="fill" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%s"/><stop offset="0.45" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`+"\n", l.Fill[0], l.Fill[1], l.Fill[2])
	fmt.Fprintf(&b, `    <linearGradient id="rim" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`+"\n", l.Rim[0], l.Rim[1])
	if filters {
		fmt.Fprintf(&b, `    <filter id="shadow" x="-30%%" y="-30%%" width="160%%" height="160%%"><feDropShadow dx="0" dy="%d" stdDeviation="%d" flood-color="%s" flood-opacity="%.2f"/></filter>`+"\n", ShadowDY, ShadowBlur, l.Shadow, l.ShadowOp)
		if l.Glow > 0 {
			fmt.Fprintf(&b, `    <filter id="glow" x="-50%%" y="-50%%" width="200%%" height="200%%"><feGaussianBlur stdDeviation="%d"/></filter>`+"\n", GlowBlur)
		}
	}
	b.WriteString("  </defs>\n")
	if parts&PartGround != 0 {
		b.WriteString(`  <rect width="1024" height="1024" fill="url(#ground)"/>` + "\n")
	}
	fmt.Fprintf(&b, `  <g transform="translate(%d %d)">`+"\n", inset, inset)
	if parts&PartGlow != 0 && l.Glow > 0 {
		if filters {
			fmt.Fprintf(&b, `  <g opacity="%.2f" filter="url(#glow)">`+"\n", l.Glow)
		} else {
			b.WriteString("  <g>\n")
		}
		for _, r := range carets {
			fmt.Fprintf(&b, `    <rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s"/>`+"\n", r[0], r[1], r[2], r[3], caretR, l.Fill[2])
		}
		b.WriteString("  </g>\n")
	}
	if parts&PartRims != 0 {
		if filters {
			b.WriteString(`  <g filter="url(#shadow)">` + "\n")
		} else {
			b.WriteString("  <g>\n")
		}
		for _, r := range rims {
			fmt.Fprintf(&b, `    <rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="url(#rim)"/>`+"\n", r[0], r[1], r[2], r[3], rimR)
		}
		b.WriteString("  </g>\n")
	}
	if parts&PartCarets != 0 {
		for _, r := range carets {
			fmt.Fprintf(&b, `  <rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="url(#fill)"/>`+"\n", r[0], r[1], r[2], r[3], caretR)
		}
	}
	b.WriteString("  </g>\n</svg>\n")
	return b.String()
}

// Pick is the look the icon is drawn in for mode, the settings' choice: a look's ID, "auto" for the dark look in a
// dark window and Paper in a light one, and Paper for "" or a look there is none of.
func Pick(mode string, dark bool) Look {
	if mode == "auto" {
		if dark {
			l, _ := ByID("dark")
			return l
		}
		return Looks[0]
	}
	if l, ok := ByID(mode); ok {
		return l
	}
	return Looks[0]
}
