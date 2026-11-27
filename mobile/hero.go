package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Home's ground takes the colors of the hour (dawn, day, evening, night), passing from one to the next as the day
// goes, deepened almost to black for the summary (home.go). It was a field of four glows drawn by a shader; one flat
// color, it meets the date row, the sheet's fade and the system's bar with no edge (#645).

// A sky is the field's colors at an hour: its ground, its four glows, and whether words on it are light.
type sky struct {
	at    float64 // the hour, from 0 to 24
	cols  [5]ui.Color
	light float32 // 1 when words on it are light
}

// skies are the field's colors through a day, the hours between them passing from one to the next.
var skies = []sky{
	{0, [5]ui.Color{ui.Hex("#0F1124"), ui.Hex("#3B2B63"), ui.Hex("#22336F"), ui.Hex("#152452"), ui.Hex("#4B2450")}, 1},
	{5, [5]ui.Color{ui.Hex("#0F1124"), ui.Hex("#3B2B63"), ui.Hex("#22336F"), ui.Hex("#152452"), ui.Hex("#4B2450")}, 1},
	{7, [5]ui.Color{ui.Hex("#FFE8DA"), ui.Hex("#FFAE88"), ui.Hex("#F6A2C2"), ui.Hex("#B8C6FF"), ui.Hex("#FFD49A")}, 0},
	{10, [5]ui.Color{ui.Hex("#EEF4FF"), ui.Hex("#9EC7FF"), ui.Hex("#B9A9FF"), ui.Hex("#92DECF"), ui.Hex("#FFD5AC")}, 0},
	{16, [5]ui.Color{ui.Hex("#EEF4FF"), ui.Hex("#9EC7FF"), ui.Hex("#B9A9FF"), ui.Hex("#92DECF"), ui.Hex("#FFD5AC")}, 0},
	{18.5, [5]ui.Color{ui.Hex("#FFD8C2"), ui.Hex("#FF8B5E"), ui.Hex("#C96DC4"), ui.Hex("#6F6DD8"), ui.Hex("#FFB46C")}, 0},
	{21, [5]ui.Color{ui.Hex("#0F1124"), ui.Hex("#3B2B63"), ui.Hex("#22336F"), ui.Hex("#152452"), ui.Hex("#4B2450")}, 1},
	{24, [5]ui.Color{ui.Hex("#0F1124"), ui.Hex("#3B2B63"), ui.Hex("#22336F"), ui.Hex("#152452"), ui.Hex("#4B2450")}, 1},
}

// skyAt is the field's colors at now, between the skies of the hours around it; deeper on the dark look.
func skyAt(now time.Time, deep bool) sky {
	h := float64(now.Hour()) + float64(now.Minute())/60
	s := skies[0]
	for i := 1; i < len(skies); i++ {
		if h <= skies[i].at {
			a, b := skies[i-1], skies[i]
			t := float32((h - a.at) / (b.at - a.at))
			s = sky{at: h, light: a.light + (b.light-a.light)*t}
			for k := range s.cols {
				s.cols[k] = a.cols[k].MixOklab(b.cols[k], t)
			}
			break
		}
	}
	if deep && s.light < 1 { // the day's colors, dimmed for the dark look, words on them light
		for k := range s.cols {
			s.cols[k] = s.cols[k].MixOklab(ui.Hex("#0B0B12"), 0.55)
		}
		s.light = 1
	}
	return s
}

// ink is the color of words on the sky.
func (s sky) ink() ui.Color { return ui.Hex("#2A1E1A").Mix(ui.Hex("#F6F1EC"), s.light) }
