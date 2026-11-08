package main

import (
	"os"

	"github.com/egoist/mygo/ui/shader"
)

// The looks of the welcome, to choose among in the settings (Görünüm ▸ Karşılama ekranı) and to try out with
// "Yeniden başlat": the ground (what is drawn over the desktop under the words) and the opening (how the app's
// window comes out after Başla). A new one is a line in welcomeGrounds or welcomeOpenings.
//
// A ground is a radial veil and a layer over it (groundShader). The layer is a function
// layer(p vec2, e float, pos vec2) vec4: p is the place with the middle at 0 and the height 1, e 0 in the middle and 1
// at the edges' middles, pos the place in pixels (Win is where the app's window is, in the same); it gives a
// premultiplied color, and the ground shows it as Mesh (0..1) grows. The mesh grounds
// give only their colors (meshLayer): a function tint(i int) vec3 for i of 0..5, whose points alternate round a
// ring, so that a ground of two colors puts them at even and odd i.
//
// An opening is a LayerEffect over the app's whole window, with these uniforms, of which it declares those it
// uses: Prog (0..1, eased), Time (seconds since Başla), Size (the window's, in DIPs), Origin (the middle of the
// button, in the window) and Radius (how far a ring from Origin has run, past the farthest corner at the end). What it leaves clear (alpha
// 0) is the desktop; at Prog 1 it must give the window as it is. MaxSampleOffset is how far it samples from pos.

// A welcomeGround is a look of the ground.
type welcomeGround struct {
	id, name string
	sh       *shader.Shader
	blur     bool // the desktop under it is blurred (macOS): the stage is a dark material, which comes and goes slowly
}

// A welcomeOpening is a look of the opening.
type welcomeOpening struct {
	id, name  string
	sh        *shader.Shader
	maxOffset float32
	ring      bool                    // its edge is the ring (ringEdge): the ground, drawn in the same window, keeps to outside it
	dur       float64                 // how long it takes, in seconds; zero is welOpen
	ease      func(t float64) float64 // what Prog is at t of 0..1; nil is slow at both ends (smoothstep)
}

// length is how long the opening takes, in seconds.
func (o welcomeOpening) length() float64 {
	if o.dur > 0 {
		return o.dur
	}
	return welOpen
}

// progress is Prog at t (0..1 of its length).
func (o welcomeOpening) progress(t float64) float64 {
	t = min(1, max(0, t))
	if o.ease != nil {
		return o.ease(t)
	}
	return smoothstep(t)
}

var welcomeGrounds = []welcomeGround{
	{"", "Gece", groundShader(meshLayer(`
func tint(i int) vec3 {
	a := vec3(0.10, 0.56, 0.62) // deep teal
	b := vec3(0.20, 0.20, 0.62) // midnight
	if i == 0 || i == 4 {
		return mix(a, vec3(0.30, 0.80, 0.74), float(i)/8)
	}
	if i == 2 {
		return mix(a, b, 0.35)
	}
	if i == 5 {
		return mix(b, vec3(0.42, 0.28, 0.80), 0.5)
	}
	return b
}
`)), false},
	{"tentakul", "Tentakül", groundShader(tentacleLayer), true},
	{"bulut", "Bulut", groundShader(cloudLayer), true},
}

var welcomeOpenings = []welcomeOpening{
	{id: "", name: "Halka", sh: shader.MustCompile(ringOpening + ringShared), maxOffset: 28, ring: true},
}

// welcomeGroundLook is the ground chosen in the settings, or the first. RYUDIT_WELCOME_GROUND chooses one for a run
// alone, to look at it without changing the settings (RYUDIT_WELCOME=1 RYUDIT_WELCOME_GROUND=cizgiler).
func (a *app) welcomeGroundLook() welcomeGround {
	id := a.settings.WelcomeGround
	if v, ok := os.LookupEnv("RYUDIT_WELCOME_GROUND"); ok {
		id = v
	}
	for _, g := range welcomeGrounds {
		if g.id == id {
			return g
		}
	}
	return welcomeGrounds[0]
}

// welcomeOpeningLook is the opening chosen in the settings, or the first; RYUDIT_WELCOME_OPEN chooses one for a run.
func (a *app) welcomeOpeningLook() welcomeOpening {
	id := a.settings.WelcomeOpen
	if v, ok := os.LookupEnv("RYUDIT_WELCOME_OPEN"); ok {
		id = v
	}
	for _, o := range welcomeOpenings {
		if o.id == id {
			return o
		}
	}
	return welcomeOpenings[0]
}

// ringShared is the edge of the ring opening, in the shaders that have a ring: the app is kept inside it, and the
// colors outside; and the noise the other openings are made of.
const ringShared = `
func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3 - 2*f)
	return mix(mix(hash(i), hash(i+vec2(1, 0)), u.x), mix(hash(i+vec2(0, 1)), hash(i+vec2(1, 1)), u.x), u.y)
}

// ringEdge is how far from the origin the edge is at pos: not a circle, it breathes, most in the middle.
func ringEdge(pos vec2, radius float, prog float) float {
	wob := (noise(pos/46+vec2(prog*2.4, 0))*0.65 + noise(pos/19)*0.12 - 0.38) * 100 * sin(prog*3.14159)
	return radius + wob
}
`

// groundShader is a ground with the layer given: first the desktop darkens, most at its edges (a radial veil);
// then the layer comes up over it, as Mesh grows. Premultiplied, so that what is not covered stays clear and the
// desktop shows through.
func groundShader(layer string) *shader.Shader {
	return shader.MustCompile(`package main

var Origin vec2
var Radius float
var Prog float
var Open float
var Time float
var Fade float
var Veil float
var Mesh float
var Dark float
var Base vec4
var Win vec4 // the app's window, in this one: x, y, width, height
` + layer + `
func hash21(p vec2) float {
	q := fract(p * vec2(123.34, 456.21))
	q += dot(q, q+45.32)
	return fract(q.x * q.y)
}

func Fragment(dst vec4, pos vec2) vec4 {
	sz := rectSize()
	p := (pos - sz*0.5) / sz.y
	e := length((pos - sz*0.5) / (sz * 0.5)) // 0 in the middle, 1 at the edges' middles

	// the veil: darker toward the edges
	va := Veil * mix(0.34, 0.80, smoothstep(0.0, 1.3, e))
	out := vec4(0, 0, 0, va) + Base*(1-va)

	lay := layer(p, e, pos) * Mesh
	out = lay + out*(1-lay.a)

	g := (hash21(dst.xy) - 0.5) * 0.05
	out.rgb += vec3(g) * out.a
	if Open > 0.5 { // the app has the inside of the ring
		d := distance(pos, Origin)
		edge := ringEdge(pos, Radius, Prog)
		out *= smoothstep(edge-90, edge, d)
	}
	return out * Fade
}
` + ringShared)
}

// meshLayer is a mesh of the six colors of tint: each color held by a point that drifts slowly, the colors running
// into one another where the points meet, strongest in the middle.
func meshLayer(tint string) string {
	return tint + `
// node is where color i of the mesh is: around a ring, each drifting on a slow path of its own
func node(i int, t float) vec2 {
	fi := float(i)
	ph := fi*2.1 + 0.7
	at := vec2(cos(fi*1.0472+0.4), sin(fi*1.0472+0.4)) * vec2(0.46, 0.32)
	return at + vec2(0.20*sin(t*0.09+ph)+0.06*sin(t*0.21+ph*1.7), 0.15*cos(t*0.07+ph*1.3)+0.05*cos(t*0.17+ph))
}

func layer(p vec2, e float, pos vec2) vec4 {
	// each color weighed by how near its point is, the plane bent a little so the seams flow
	q := p + vec2(0.07*sin(p.y*2.6+Time*0.12), 0.06*cos(p.x*2.2-Time*0.10))
	col := vec3(0)
	ws := 0.0
	for i := 0; i < 6; i++ {
		d := distance(q, node(i, Time))
		w := 1.0 / (pow(d, 2.4) + 0.004)
		col += tint(i) * w
		ws += w
	}
	col = col / ws * mix(1.0, 0.84, Dark)
	a := 0.86 * (1 - smoothstep(0.30, 1.20, e))
	return vec4(col*a, a)
}
`
}

// fbmShader is noise in five octaves, for the grounds that are made of it (noise is in ringShared, which every shader has).
const fbmShader = `
func fbm(p vec2) float {
	v := 0.0
	a := 0.5
	for i := 0; i < 5; i++ {
		v += a * noise(p)
		p = p*2.03 + vec2(1.7, 9.2)
		a *= 0.5
	}
	return v
}
`

// cloudLayer is a sky of slow clouds at dusk: noise bent by noise, so that they billow, lit from below on the side
// of a low warm light; between them the sky is thin, and the desktop shows through it, blurred.
const cloudLayer = fbmShader + `
func layer(p vec2, e float, pos vec2) vec4 {
	q := p*1.5 + vec2(Time*0.018, Time*0.004)
	w := vec2(fbm(q+vec2(0, Time*0.012)), fbm(q+vec2(5.2, 1.3)))
	d := fbm(q + 1.7*w)
	lit := fbm(q + 1.7*w + vec2(-0.05, 0.07)) // a little toward the light
	dens := smoothstep(0.30, 0.74, d)
	shade := clamp((d-lit)*6+0.5, 0, 1)
	sky := mix(vec3(0.10, 0.11, 0.20), vec3(0.24, 0.18, 0.34), clamp(p.y+0.6, 0, 1))
	cloud := mix(vec3(0.34, 0.36, 0.52), vec3(1.0, 0.74, 0.56), shade*shade) * 1.2
	col := mix(sky, cloud, dens*0.92)
	col *= mix(0.72, 1.0, smoothstep(0.12, 0.6, length(p))) // the middle, where the words are, a little darker
	// thin where there is no cloud, so that the desktop shows through, blurred, and the screen is never black
	a := mix(0.62, 0.90, dens) * mix(1.0, 0.92, smoothstep(0.3, 1.4, e))
	return vec4(col*a, a)
}
`

// tentacleLayer is ribbons of light that bend out from a bright heart like the arms of something deep, each white
// where it is brightest, split into its colors at its edges as light through a prism is, and wrapped in a softer
// glow of itself, over a deep blue and violet.
const tentacleLayer = `
// ribbon is ribbon k at angle a and distance r, and its glow, for the color turned by off
func ribbon(a float, r float, k float, t float, off float) vec2 {
	target := k*0.628 + 0.62*sin(r*2.1+t*0.17+k*1.7) + 0.24*sin(r*4.7-t*0.23+k*0.8)
	x := a - target + off*(0.3+r)
	d := abs(atan2(sin(x), cos(x))) // the angle between, -π..π: mod is not the same for a negative number on every GPU
	w := 0.0035 + r*0.0055
	fade := smoothstep(0.03, 0.30, r) * (1 - smoothstep(0.9, 1.8, r)) * (0.55 + 0.45*sin(k*2.3+t*0.4))
	return vec2(w/(d+w*0.5), (w*7)/(d+w*7)) * fade
}

func layer(p vec2, e float, pos vec2) vec4 {
	r := length(p)
	a := atan2(p.y, p.x)
	sharp := vec3(0)
	glow := vec3(0)
	for i := 0; i < 10; i++ {
		k := float(i)
		rr := ribbon(a, r, k, Time, -0.020)
		gg := ribbon(a, r, k, Time, 0.0)
		bb := ribbon(a, r, k, Time, 0.020)
		sharp += vec3(rr.x, gg.x, bb.x)
		glow += vec3(rr.y, gg.y, bb.y)
	}
	heart := exp(-r*9) * 1.4
	hdr := sharp*vec3(0.30, 0.27, 0.36) + glow*vec3(0.05, 0.05, 0.10) + vec3(0.8, 0.85, 1.0)*heart
	light := vec3(1) - exp(-hdr) // soft at its brightest
	light *= mix(0.45, 1.0, smoothstep(0.06, 0.35, r)) // the words keep the heart
	lobe := vec3(0.10, 0.12, 0.35)*exp(-length(p-vec2(-0.5, 0.3))*2.2) + vec3(0.22, 0.10, 0.30)*exp(-length(p-vec2(0.55, -0.35))*2.4)
	col := vec3(0.025, 0.025, 0.07) + lobe*0.8 + light
	al := mix(0.96, 0.88, smoothstep(0.3, 1.4, e))
	return vec4(col*al, al)
}
`

// ringOpening: the app's own pixels, kept inside a ring that grows from the button, with a rim of light on it,
// bent a little where it passes. Outside the ring the window is clear.
const ringOpening = `package main

var Origin vec2
var Radius float
var Prog float

func LayerEffect(pos vec2) vec4 {
	rel := pos - Origin
	d := length(rel)
	dir := rel / max(d, 0.001)
	// the edge is not a circle: it breathes, most in the middle of the opening
	edge := ringEdge(pos, Radius, Prog)
	width := 90.0
	m := 1 - smoothstep(edge-width, edge, d)
	// the rim: bright, narrow, and just behind the edge
	rimAt := (d - (edge - width*0.42)) / (width * 0.30)
	rim := exp(-rimAt * rimAt)
	// where the ring passes, the app is seen through it, bent and split in colors
	lens := smoothstep(edge-width*1.7, edge-width*0.5, d) * (1 - smoothstep(edge-width*0.5, edge+8, d))
	off := dir * lens * 20 * (1 - Prog*0.6)
	c0 := layerSample(pos - off*0.75)
	c1 := layerSample(pos - off)
	c2 := layerSample(pos - off*1.25)
	col := vec4(c0.r, c1.g, c2.b, c1.a) * m
	ang := atan2(dir.y, dir.x)
	hue := mix(vec3(1.0, 0.46, 0.70), vec3(0.46, 0.58, 1.0), 0.5+0.5*sin(ang*2+Prog*7))
	fade := (1 - smoothstep(0.55, 1.0, Prog)) * 0.9
	glow := rim * fade
	col.rgb += hue * glow * 0.85
	col.a = max(col.a, glow*0.85)
	return col
}
`
