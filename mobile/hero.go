package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The home screen's top block is a field of soft colors that drift slowly, as light through a window moves over a
// day: four glows wander on a ground, a fine grain over them, their colors the hour's (dawn, day, evening, night),
// passing from one to the next as the day goes; on the dark look the same, deeper. The date and the week stand on it.
// It moves only while it shows, and not at all with reduced motion.

// heroShader is the drifting field. Time is in seconds; Ground and Glow1 to Glow4 are the hour's colors.
var heroShader = shader.MustCompile(heroSource)

const heroSource = `package main

var Time float
var Ground vec3
var Glow1 vec3
var Glow2 vec3
var Glow3 vec3
var Glow4 vec3

func glow(uv vec2, at vec2, size float) float {
	d := uv - at
	return exp(-dot(d, d) / (size * size))
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	uv.x *= size.x / size.y
	asp := size.x / size.y
	t := Time * 0.11

	p1 := vec2(asp*(0.18+0.12*sin(t*1.3)), 0.30+0.22*cos(t*0.9))
	p2 := vec2(asp*(0.78+0.10*cos(t*1.1)), 0.25+0.20*sin(t*1.4+1.0))
	p3 := vec2(asp*(0.55+0.18*sin(t*0.7+2.0)), 0.85+0.12*cos(t*1.2))
	p4 := vec2(asp*(0.95+0.08*sin(t*1.6+3.0)), 0.80+0.15*sin(t*0.8))

	col := Ground
	col = mix(col, Glow1, glow(uv, p1, 0.55)*0.95)
	col = mix(col, Glow2, glow(uv, p2, 0.50)*0.90)
	col = mix(col, Glow3, glow(uv, p3, 0.60)*0.85)
	col = mix(col, Glow4, glow(uv, p4, 0.40)*0.70)

	grain := fract(sin(dot(dst.xy, vec2(12.9898, 78.233))) * 43758.5453)
	col += (grain - 0.5) * 0.035
	return vec4(col, 1)
}
`

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

// heroStart is when the field's clock started.
var heroStart = time.Now()

// heroField fills e with the drifting field in sky s's colors, its corners rounded r.
func heroField(c *ui.Context, e ui.Element, s sky, r float32) ui.Element {
	if testing.Testing() { // the tests draw on the CPU, where a shader over a block takes long: its colors instead
		return e.Gradient(s.cols[1], s.cols[2], 135)
	}
	still := c.Preferences().ReduceMotion
	t := float32(0)
	if !still {
		t = float32(c.Now().Sub(heroStart).Seconds())
		c.AnimationFrame()
	}
	return e.Draw(func(p *ui.Painter, rect ui.Rect) {
		heroShader.Draw(p, rect, shader.Options{Uniforms: map[string]any{"Time": t, "Ground": s.cols[0], "Glow1": s.cols[1],
			"Glow2": s.cols[2], "Glow3": s.cols[3], "Glow4": s.cols[4]}, Radius: r})
	})
}
