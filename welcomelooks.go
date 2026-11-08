package main

import (
	"math"
	"os"

	"github.com/egoist/mygo/ui/shader"
)

// The looks of the welcome, to choose among in the settings (Görünüm ▸ Karşılama ekranı) and to try out with
// "Yeniden başlat": the ground (what is drawn over the desktop under the words) and the opening (how the app's
// window comes out after Başla). A new one is a line in welcomeGrounds or welcomeOpenings.
//
// A ground is a radial veil and a layer over it (groundShader). The layer is a function
// layer(p vec2, e float) vec4: p is the place with the middle at 0 and the height 1, e 0 in the middle and 1 at the
// edges' middles; it gives a premultiplied color, and the ground shows it as Mesh (0..1) grows. The mesh grounds
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

// easeIn starts slowly and speeds up, as a thing let go does.
func easeIn(t float64) float64 { return math.Pow(t, 2.2) }

var welcomeGrounds = []welcomeGround{
	{"", "İki ton", groundShader(meshLayer(`
func tint(i int) vec3 {
	a := vec3(0.30, 0.34, 1.00) // indigo
	b := vec3(1.00, 0.50, 0.40) // coral
	if i == 0 {
		return a
	}
	if i == 1 {
		return b
	}
	if i == 2 {
		return mix(a, vec3(0.62, 0.40, 1.00), 0.45)
	}
	if i == 3 {
		return mix(b, vec3(1.00, 0.72, 0.46), 0.40)
	}
	if i == 4 {
		return mix(a, vec3(0.16, 0.20, 0.70), 0.50)
	}
	return mix(b, vec3(0.92, 0.34, 0.52), 0.45)
}
`)), false},
	{"renkli", "Renkli", groundShader(meshLayer(`
func tint(i int) vec3 {
	if i == 0 {
		return vec3(1.00, 0.52, 0.34)
	}
	if i == 1 {
		return vec3(0.98, 0.36, 0.62)
	}
	if i == 2 {
		return vec3(0.58, 0.42, 1.00)
	}
	if i == 3 {
		return vec3(0.26, 0.58, 1.00)
	}
	if i == 4 {
		return vec3(0.22, 0.82, 0.78)
	}
	return vec3(1.00, 0.78, 0.40)
}
`)), false},
	{"gece", "Gece", groundShader(meshLayer(`
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
	{"cizgiler", "Çizgiler", groundShader(linesLayer), true},
	{"ascii", "ASCII", groundShader(asciiLayer), true},
	{"bulut", "Bulut", groundShader(cloudLayer), true},
	{"sistem", "Sistem", groundShader(orbitLayer), true},
}

var welcomeOpenings = []welcomeOpening{
	{id: "", name: "Halka", sh: shader.MustCompile(ringOpening + ringShared), maxOffset: 28, ring: true},
	{id: "perde", name: "Perde", sh: shader.MustCompile(curtainOpening + ringShared), maxOffset: 0},
	{id: "cozulme", name: "Çözülme", sh: shader.MustCompile(dissolveOpening + ringShared), maxOffset: 0},
	{id: "cizgiler", name: "Çizgiler", sh: shader.MustCompile(linesOpening + ringShared), maxOffset: 0},
	{id: "namedrop", name: "NameDrop", sh: shader.MustCompile(namedropOpening + ringShared), dur: 3.2, ease: easeIn},
	{id: "yarimton", name: "Yarım ton", sh: shader.MustCompile(halftoneOpening + ringShared), maxOffset: 0},
	{id: "siyahbeyaz", name: "Siyah beyaz", sh: shader.MustCompile(monoOpening + ringShared), maxOffset: 0},
	{id: "cam", name: "Cam dalgası", sh: shader.MustCompile(rippleOpening + ringShared), maxOffset: 24},
	{id: "renkkaymasi", name: "Renk kayması", sh: shader.MustCompile(aberrationOpening + ringShared), maxOffset: 48},
	{id: "crt", name: "CRT", sh: shader.MustCompile(crtOpening + ringShared), maxOffset: 0},
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

	lay := layer(p, e) * Mesh
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

func layer(p vec2, e float) vec4 {
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

// linesLayer is rings of light running out from the middle, each a few thin lines split in red, green and blue,
// over a mosaic of tall cells, so that they break into vertical streaks (after "Shader Lines"), on a ground darker
// than the veil. As Mesh grows they come out from the middle, and the ground darkens.
const linesLayer = `
func rnd(x float) float {
	return fract(sin(x) * 10000)
}

// lines is the light of channel j (0 red, 1 green, 2 blue) at distance l from the middle, at time t
func lines(t float, l float, j float) float {
	c := 0.0
	for i := 0; i < 5; i++ {
		fi := float(i)
		c += 0.0008 * fi * fi / max(abs(fract(t-0.01*j+fi*0.01)-l), 0.002)
	}
	return c
}

func layer(p vec2, e float) vec4 {
	uv := p * 2
	uv.x = floor(uv.x*64) / 64 // the mosaic: cells four times as tall as wide, on a 256 grid
	uv.y = floor(uv.y*128) / 128
	t := Time*0.18 + rnd(uv.x)*0.4
	l := length(uv)
	col := min(vec3(lines(t, l, 2), lines(t, l, 1), lines(t, l, 0)), vec3(1))
	col *= 1 - smoothstep(Mesh*1.7-0.5, Mesh*1.7, l) // they come out from the middle
	col *= 1 - smoothstep(0.95, 1.6, e)
	col *= mix(0.22, 1.0, smoothstep(0.30, 0.95, l)) // and the middle, where the words are, is quiet
	a := max(max(col.r, col.g), col.b)
	// the lines want a deep ground, darkest in the middle: it comes up with them, over the veil
	return vec4(col, a) + vec4(0, 0, 0, mix(0.92, 0.70, smoothstep(0.2, 1.4, l)))*(1-a)
}
`

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

// asciiLayer is a field of letters on a dark ground: a slow flow of noise, each cell of it the letter whose weight is
// its light (5x5 bitmaps in a number, after movAX13h), in green going to blue across the screen. The cells come on
// one by one, in no order, as Mesh grows; the middle, where the words are, is quiet.
const asciiLayer = fbmShader + `
func glyph(n float, p vec2) float {
	p = floor(p*vec2(4, -4) + 2.5)
	if p.x < 0 || p.x > 4 || p.y < 0 || p.y > 4 {
		return 0
	}
	return mod(floor(n/exp2(p.x+5*p.y)), 2)
}

func layer(p vec2, e float) vec4 {
	cell := 0.017
	at := (floor(p/cell) + 0.5) * cell
	f := fbm(at*2.4+vec2(Time*0.045, -Time*0.03)) * 1.5 - 0.42
	f += 0.18 * sin(length(at)*11 - Time*0.7) // a slow wave out from the middle
	l := clamp(f, 0, 1) * mix(0.30, 1.0, smoothstep(0.16, 0.55, length(at)))
	on := smoothstep(0.0, 0.08, Mesh*1.1-hash(at*97.0)) // each cell in its own time
	n := 0.0
	if l > 0.08 {
		n = 4096 // .
	}
	if l > 0.2 {
		n = 65600 // :
	}
	if l > 0.32 {
		n = 332772 // *
	}
	if l > 0.44 {
		n = 15255086 // o
	}
	if l > 0.56 {
		n = 23385164 // &
	}
	if l > 0.68 {
		n = 15252014 // 8
	}
	if l > 0.8 {
		n = 13199452 // @
	}
	if l > 0.9 {
		n = 11512810 // #
	}
	g := glyph(n, mod(p/(cell*0.5), 2)-1) * on
	tint := mix(vec3(0.36, 0.95, 0.62), vec3(0.45, 0.66, 1.0), clamp(at.x*0.7+0.5, 0, 1))
	col := tint * (0.45 + 0.75*l) * g * (1 - smoothstep(0.95, 1.5, e))
	return vec4(col, g) + vec4(0, 0, 0, mix(0.93, 0.78, smoothstep(0.2, 1.3, e)))*(1-g)
}
`

// cloudLayer is a sky of slow clouds at dusk: noise bent by noise, so that they billow, lit from below on the side
// of a low warm light, on a dark sky.
const cloudLayer = fbmShader + `
func layer(p vec2, e float) vec4 {
	q := p*1.5 + vec2(Time*0.018, Time*0.004)
	w := vec2(fbm(q+vec2(0, Time*0.012)), fbm(q+vec2(5.2, 1.3)))
	d := fbm(q + 1.7*w)
	lit := fbm(q + 1.7*w + vec2(-0.05, 0.07)) // a little toward the light
	dens := smoothstep(0.30, 0.74, d)
	shade := clamp((d-lit)*6+0.5, 0, 1)
	sky := mix(vec3(0.03, 0.035, 0.08), vec3(0.12, 0.08, 0.20), clamp(p.y+0.6, 0, 1))
	cloud := mix(vec3(0.24, 0.25, 0.40), vec3(1.0, 0.72, 0.54), shade*shade) * 1.25
	col := mix(sky, cloud, dens*0.92)
	col *= mix(0.55, 1.0, smoothstep(0.12, 0.6, length(p))) // the middle, where the words are, darker
	a := mix(0.96, 0.84, smoothstep(0.3, 1.4, e))
	return vec4(col*a, a)
}
`

// orbitLayer is a system seen from a little above: thin orbits round a faint sun behind the words, a planet of its
// own color on each, going slower the farther out, a halo round each, over a field of stars that twinkle.
const orbitLayer = `
func layer(p vec2, e float) vec4 {
	col := vec3(0)
	cell := floor(p * 220)
	st := hash(cell)
	twinkle := 0.55 + 0.45*sin(Time*1.7+st*60)
	col += vec3(0.85, 0.9, 1.0) * step(0.9965, st) * twinkle * 0.9
	col += vec3(1.0, 0.72, 0.42) * 0.012 / (length(p*vec2(1, 1.6)) + 0.03)
	for i := 0; i < 6; i++ {
		fi := float(i)
		r := 0.22 + fi*0.075
		q := vec2(p.x, p.y/0.36) // the plane of the orbits, seen from above it
		d := abs(length(q) - r)
		col += vec3(0.42, 0.52, 0.95) * exp(-d*d/0.0000045) * 0.28
		a := Time*0.55/(1+fi*0.7) + fi*1.9
		at := vec2(cos(a)*r, sin(a)*r*0.36)
		pd := distance(p, at)
		tint := mix(vec3(0.55, 0.75, 1.0), vec3(1.0, 0.62, 0.45), hash(vec2(fi, 4.2)))
		col += tint * (exp(-pd*pd/0.000016)*1.1 + 0.00035/(pd*pd+0.0006))
	}
	col = min(col, vec3(1)) * (1 - smoothstep(1.0, 1.6, e))
	g := max(max(col.r, col.g), col.b)
	return vec4(col, g) + vec4(0, 0, 0, mix(0.94, 0.80, smoothstep(0.2, 1.3, e)))*(1-g)
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

// curtainOpening: the window comes down from the top behind a soft edge that waves a little, a line of light on it.
const curtainOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	soft := 120.0
	wave := 26 * sin(pos.x/110+Prog*5) * sin(Prog*3.14159)
	edge := Prog*(Size.y+soft*2) - soft + wave
	m := 1 - smoothstep(edge-soft, edge, pos.y)
	at := (pos.y - (edge - soft*0.35)) / (soft * 0.22)
	glow := exp(-at*at) * (1 - smoothstep(0.6, 1.0, Prog)) * 0.8
	col := layerSample(pos) * m
	col.rgb += mix(vec3(0.46, 0.52, 1.0), vec3(1.0, 0.56, 0.48), pos.x/max(Size.x, 1)) * glow
	col.a = max(col.a, glow)
	return col
}
`

// dissolveOpening: the window comes up out of a cloud, the parts of it the noise picks first, with a rim of light
// where it is coming.
const dissolveOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	n := noise(pos/70)*0.62 + noise(pos/23)*0.28 + noise(pos/7)*0.10
	t := Prog*1.25 - 0.12
	m := smoothstep(n-0.10, n, t)
	rim := (smoothstep(n-0.16, n-0.06, t) - m) * (1 - smoothstep(0.7, 1.0, Prog))
	col := layerSample(pos) * m
	col.rgb += mix(vec3(0.40, 0.48, 1.0), vec3(1.0, 0.52, 0.46), n) * rim
	col.a = max(col.a, rim)
	return col
}
`

// linesOpening: the window comes out of the middle of the screen inside a circle that grows, its edge a few rings
// of light split in red, green and blue, as the lines ground has them.
const linesOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	d := distance(pos, mid)
	r := Prog * (length(mid) + 140)
	m := 1 - smoothstep(r-50, r, d)
	glow := vec3(0)
	for i := 0; i < 5; i++ {
		fi := float(i)
		w := 0.9 * (fi + 1) / 5
		glow.b += w / max(abs(d-(r-fi*16)), 1.5)
		glow.g += w / max(abs(d-(r-fi*16-3)), 1.5)
		glow.r += w / max(abs(d-(r-fi*16-6)), 1.5)
	}
	glow = min(glow*(1-smoothstep(0.65, 1.0, Prog)), vec3(1))
	col := layerSample(pos) * m
	col.rgb += glow * (1 - col.a*0.5)
	col.a = max(col.a, max(max(glow.r, glow.g), glow.b))
	return col
}
`

// namedropOpening is after iOS's NameDrop (github.com/MartinRGB/iOS17_Namedrop_R3F): the window springs out of the
// top of the screen from a third of its size, a wave runs down from there, bending what it passes and blowing its
// light out, and streaks of light burst from the same point.
const namedropOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	anchor := vec2(Size.x*0.5, 0)
	// the size: from 0.3, on a spring, settling exactly at 1
	sp := 1 - 0.7*exp(-5.0*Prog)*cos(7.5*Prog)
	sc := mix(sp, 1, smoothstep(0.8, 1.0, Prog))
	uv := anchor + (pos-anchor)/sc
	uv.y -= (1 - smoothstep(0.0, 0.55, Prog)) * Size.y * 0.06
	// the wave: from the anchor, faster as it goes
	d := distance(uv, anchor)
	far := length(Size)
	r := Prog * Prog * far * 1.15
	diff := (d - r) / 140
	band := 1 - smoothstep(0.0, 1.0, abs(diff))
	life := 1 - smoothstep(0.55, 1.0, Prog)
	dir := (uv - anchor) / max(d, 1)
	uv += dir * diff * band * 26 * life
	col := vec4(0)
	if uv.x >= 0 && uv.y >= 0 && uv.x <= Size.x && uv.y <= Size.y {
		col = layerSample(uv)
	}
	col.rgb += col.rgb * band * 1.4 * life
	// the streaks: thin lines of light running out from the anchor
	glow := 0.0
	for i := 0; i < 36; i++ {
		fi := float(i)
		a := 0.15 + 2.84*hash(vec2(fi, 3.1))
		v := vec2(cos(a), sin(a))
		t := clamp((Prog-0.08*hash(vec2(fi, 9.7)))/0.55, 0, 1)
		head := t * (220 + 520*hash(vec2(fi, 1.3)))
		along := dot(pos-anchor, v)
		across := abs(dot(pos-anchor, vec2(-v.y, v.x)))
		inLine := smoothstep(head-70, head, along) * (1 - smoothstep(head, head+4, along))
		glow += inLine * (1 - smoothstep(0.6, 1.6, across)) * (1 - t)
	}
	top := exp(-distance(pos, anchor)/180) * (1 - smoothstep(0.2, 0.9, Prog))
	light := vec3(0.80, 0.88, 1.0) * min(glow*1.2+top*0.9, 1)
	col.rgb += light
	col.a = max(col.a, max(max(light.r, light.g), light.b))
	return col
}
`

// halftoneOpening: the window comes as a print of light dots on a dark sheet, on a slanted grid, each as big as
// its light is, the grid growing finer, until the dots run together into the window.
const halftoneOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	cell := mix(26.0, 4.0, smoothstep(0.0, 0.8, Prog))
	c := 0.7071
	rot := vec2(c*pos.x-c*pos.y, c*pos.x+c*pos.y)
	at := (floor(rot/cell) + 0.5) * cell
	mid := vec2(c*at.x+c*at.y, -c*at.x+c*at.y)
	s := layerSample(mid)
	l := dot(s.rgb, vec3(0.299, 0.587, 0.114)) / max(s.a, 0.001)
	r := cell * 0.5 * sqrt(clamp(l*1.3+0.12, 0, 1.4)) * smoothstep(0.0, 0.3, Prog)
	dot := 1 - smoothstep(r-0.8, r+0.8, distance(rot, at))
	ink := min(s.rgb/max(s.a, 0.001)*1.5+0.08, vec3(1)) // light dots on a dark sheet, the window's shape
	dots := vec4(mix(vec3(0.03, 0.03, 0.04), ink, dot), 1) * s.a * 0.94 * smoothstep(0.0, 0.25, Prog)
	return mix(dots, layerSample(pos), smoothstep(0.6, 1.0, Prog))
}
`

// monoOpening: the window comes in gray and hard, and its colors flood in from the button outward.
const monoOpening = `package main

var Prog float
var Size vec2
var Origin vec2

func LayerEffect(pos vec2) vec4 {
	s := layerSample(pos)
	l := dot(s.rgb, vec3(0.299, 0.587, 0.114))
	hard := clamp((l/max(s.a, 0.001)-0.5)*1.35+0.5, 0, 1) * s.a
	gray := vec4(vec3(hard), s.a)
	r := smoothstep(0.35, 1.0, Prog) * (length(Size) + 200)
	flood := 1 - smoothstep(r-160, r, distance(pos, Origin))
	return mix(gray, s, flood) * smoothstep(0.0, 0.3, Prog)
}
`

// rippleOpening: the window comes out of the button as a disc of water, rings running over it and bending it,
// calming as it fills the screen.
const rippleOpening = `package main

var Prog float
var Size vec2
var Origin vec2
var Time float

func LayerEffect(pos vec2) vec4 {
	rel := pos - Origin
	d := length(rel)
	dir := rel / max(d, 1)
	r := Prog * (length(Size) + 160)
	m := 1 - smoothstep(r-70, r, d)
	calm := 1 - smoothstep(0.4, 1.0, Prog)
	wave := sin(d*0.055-Time*9) * exp(-max(r-d, 0)/260)
	off := dir * wave * 16 * calm
	col := layerSample(pos-off) * m
	shine := max(wave, 0) * calm * m * 0.22
	col.rgb += vec3(shine)
	return col
}
`

// aberrationOpening: the window fades in pulled apart into red, green and blue, a little too big, and they come
// together as it settles.
const aberrationOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	k := (1 - Prog) * (1 - Prog)
	zoom := 1 + 0.05*k
	p := mid + (pos-mid)/zoom
	off := (p - mid) / max(length(mid), 1) * 44 * k
	r := layerSample(p - off)
	g := layerSample(p)
	b := layerSample(p + off)
	a := max(max(r.a, g.a), b.a)
	return vec4(r.r, g.g, b.b, a) * smoothstep(0.0, 0.45, Prog)
}
`

// crtOpening: an old screen comes on: a line of light across the middle, which opens to the picture, curved, with
// its lines and its three colors, flickering a little, until it is the window.
const crtOpening = `package main

var Prog float
var Size vec2
var Time float

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	sx := smoothstep(0.0, 0.2, Prog)
	sy := max(smoothstep(0.16, 0.48, Prog), 0.004)
	q := pos - mid
	q = vec2(q.x/max(sx, 0.001), q.y/sy)
	tube := 1 - smoothstep(0.5, 1.0, Prog)
	uv := q / mid
	uv *= 1 + 0.10*tube*dot(uv, uv)
	if abs(uv.x) > 1 || abs(uv.y) > 1 {
		return vec4(0)
	}
	col := layerSample(mid + uv*mid)
	line := (1 - smoothstep(0.16, 0.42, Prog)) * sx
	col = vec4(mix(col.rgb, vec3(col.a)*vec3(0.85, 0.92, 1.0), line*0.85), max(col.a, line))
	scan := 1 - tube*0.32*(0.5+0.5*sin(pos.y*2.2))
	cx := mod(floor(pos.x), 3)
	mask := vec3(1)
	if cx == 0 {
		mask = vec3(1, 0.78, 0.78)
	}
	if cx == 1 {
		mask = vec3(0.78, 1, 0.78)
	}
	if cx == 2 {
		mask = vec3(0.78, 0.78, 1)
	}
	flick := 1 + tube*0.05*sin(Time*58)
	col.rgb *= scan * mix(vec3(1), mask, tube) * flick
	return col
}
`
