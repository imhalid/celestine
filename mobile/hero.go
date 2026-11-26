package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The home screen's top block is a field of soft colors that drift slowly, as light through a window moves over a
// morning: four glows (peach, lilac, sky, rose) wander on a warm ground, a fine grain over them; by night the same in
// deep ink. The date stands on it. It moves only while it shows, and not at all with reduced motion.

// heroShader is the drifting field. Time is in seconds, Dark is 1 for the night's colors.
var heroShader = shader.MustCompile(heroSource)

const heroSource = `package main

var Time float
var Dark float

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

	peach := vec3(1.00, 0.70, 0.52)
	lilac := vec3(0.66, 0.58, 1.00)
	sky := vec3(0.45, 0.68, 1.00)
	rose := vec3(1.00, 0.56, 0.70)
	ground := vec3(1.00, 0.93, 0.86)
	if Dark > 0.5 {
		peach = vec3(0.55, 0.26, 0.22)
		lilac = vec3(0.30, 0.22, 0.62)
		sky = vec3(0.12, 0.28, 0.62)
		rose = vec3(0.50, 0.16, 0.36)
		ground = vec3(0.07, 0.07, 0.10)
	}

	col := ground
	col = mix(col, peach, glow(uv, p1, 0.55)*0.95)
	col = mix(col, lilac, glow(uv, p2, 0.50)*0.90)
	col = mix(col, sky, glow(uv, p3, 0.60)*0.85)
	col = mix(col, rose, glow(uv, p4, 0.40)*0.70)

	grain := fract(sin(dot(dst.xy, vec2(12.9898, 78.233))) * 43758.5453)
	col += (grain - 0.5) * 0.035
	return vec4(col, 1)
}
`

// heroStart is when the field's clock started.
var heroStart = time.Now()

// heroField fills e with the drifting field, its corners rounded r.
func heroField(c *ui.Context, e ui.Element, r float32) ui.Element {
	if testing.Testing() { // the tests draw on the CPU, where a shader over a block takes long: its colors instead
		return e.Gradient(ui.Hex("#FFB385"), ui.Hex("#A894FF"), 135)
	}
	still := c.Preferences().ReduceMotion
	t := float32(0)
	if !still {
		t = float32(c.Now().Sub(heroStart).Seconds())
		c.AnimationFrame()
	}
	d := float32(0)
	if dark {
		d = 1
	}
	return e.Draw(func(p *ui.Painter, rect ui.Rect) {
		heroShader.Draw(p, rect, shader.Options{Uniforms: map[string]any{"Time": t, "Dark": d}, Radius: r})
	})
}
