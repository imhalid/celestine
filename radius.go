package main

import ryugo "github.com/imhalid/ryu-go"

// Corners: every corner of the app is a radius of the app's own scale times
// one measure, which the settings set (Ayarlar ▸ Görünüm ▸ Köşeler): rad(px)
// is the radius the design gives a corner of px, at the measure. 100% is the
// design as it is, 0 is square, 200% twice as round. Ryugo's own components
// (its buttons, switches, fields, badges) take the same measure: its CornerScale.

const cornerDefault = 100 // percent

// radiusScale is the measure, 1 for the design as it is. It is set at the top of each frame.
var radiusScale float32 = 1

// rad is the radius of a corner that the design gives px.
func rad(px float32) float32 { return px * radiusScale }

// cornerPercent is the measure the settings say, in percent.
func (a *app) cornerPercent() int {
	if a.settings.Corner == nil {
		return cornerDefault
	}
	return max(0, min(*a.settings.Corner, 200))
}

// ryugoScale is the measure for Ryugo's components, which also rounds its pills (buttons,
// switches) below 100%; a little over zero at the square end, as zero means unset to it.
func (a *app) ryugoScale() float32 { return max(0.01, float32(a.cornerPercent())/100) }

// ryugoCorners is the step of Ryugo's that is nearest the measure.
func (a *app) ryugoCorners() ryugo.Corners {
	switch p := a.cornerPercent(); {
	case p < 75:
		return ryugo.CornersSm
	case p < 125:
		return ryugo.CornersMd
	}
	return ryugo.CornersLg
}
