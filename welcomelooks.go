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

// iceEase is even in time: the ice keeps its own pace, the cracks first and the shards after.
func iceEase(t float64) float64 { return t }

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
	{"kristal", "Kristal", groundShader(crystalLayer), true},
	{"prizma", "Prizma", groundShader(prismLayer), true},
	{"kuzey", "Kuzey", groundShader(auroraLayer), true},
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
	{id: "buz", name: "Buz", sh: shader.MustCompile(iceOpening + voroShader + ringShared), dur: 3.0, ease: iceEase},
	{id: "patlama", name: "Patlama", sh: shader.MustCompile(burstOpening + voroShader + ringShared), dur: 2.6, ease: iceEase, maxOffset: 40},
	{id: "ufuk", name: "Ufuk", sh: shader.MustCompile(horizonOpening + fbmShader + ringShared), dur: 2.8},
	{id: "cam", name: "Cam", sh: shader.MustCompile(glassOpening + ringShared), dur: 2.6, maxOffset: 14},
	{id: "spektrum", name: "Spektrum", sh: shader.MustCompile(spectrumOpening + ringShared), dur: 2.4, maxOffset: 24},
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

// asciiLayer is a grid of small lit signs, as a wall of LEDs: a few soft masses of light drift over it, violet going to
// blue and cyan, and each cell shows the sign of its light, from a faint plus to a box, a cross, a box of dots and,
// where it is brightest, a cell lit white; a glow lies under the lit ones. The cells come on one by one, in no
// order, as Mesh grows; the middle, where the words are, is quiet.
const asciiLayer = fbmShader + `
func blob(p vec2, c vec2, r float) float {
	d := distance(p, c) / r
	return exp(-d * d)
}

// cellSign is the sign of light l at q (-1..1 in the cell)
func cellSign(l float, q vec2) float {
	b := max(abs(q.x), abs(q.y))
	box := step(0.52, b) * step(b, 0.74)
	if l < 0.16 {
		return step(abs(q.x), 0.1) * step(abs(q.y), 0.1) // a point
	}
	if l < 0.34 {
		return max(step(abs(q.x), 0.1)*step(abs(q.y), 0.5), step(abs(q.y), 0.1)*step(abs(q.x), 0.5)) // +
	}
	if l < 0.52 {
		return max(box, step(abs(q.y), 0.1)*step(abs(q.x), 0.6)) // a box with a bar
	}
	if l < 0.70 {
		return step(abs(abs(q.x)-abs(q.y)), 0.17) * step(b, 0.66) // x
	}
	if l < 0.86 {
		dots := step(length(abs(q)-vec2(0.24)), 0.15)
		return max(box, dots) // a box of four dots
	}
	return step(b, 0.72) // lit
}

func layer(p vec2, e float) vec4 {
	cell := 0.016
	at := (floor(p/cell) + 0.5) * cell
	q := (p - at) / (cell * 0.5)
	t := Time
	w := at + 0.08*vec2(fbm(at*3+vec2(t*0.05, 0)), fbm(at*3+vec2(0, t*0.04)))-0.04
	m := blob(w, vec2(-0.42+0.10*sin(t*0.13), -0.05+0.08*cos(t*0.11)), 0.30)
	m += blob(w, vec2(0.38+0.12*cos(t*0.09), 0.10*sin(t*0.12)), 0.34)
	m += 0.8 * blob(w, vec2(0.05*sin(t*0.07), 0.36+0.06*cos(t*0.1)), 0.24)
	l := clamp(m*1.05, 0, 1) * mix(0.25, 1.0, smoothstep(0.16, 0.5, length(at)))
	on := smoothstep(0.0, 0.08, Mesh*1.1-hash(at*97.0))
	g := cellSign(l, q) * on
	hue := clamp(at.x*0.9+0.5+0.15*(fbm(at*2+vec2(t*0.03, 1))-0.5), 0, 1)
	tint := mix(mix(vec3(0.62, 0.45, 1.0), vec3(0.30, 0.42, 1.0), smoothstep(0.0, 0.5, hue)), vec3(0.25, 0.95, 0.95), smoothstep(0.5, 1.0, hue))
	lit := mix(tint*(0.30+1.0*l), vec3(1.0), smoothstep(0.78, 1.0, l)*0.85)
	dim := vec3(0.30, 0.36, 0.48) * 0.35 // the cells with little light: grey-blue
	col := mix(dim, lit, smoothstep(0.1, 0.3, l)) * g
	col += tint * l * l * 0.28 * on // the glow under the lit ones
	col *= 1 - smoothstep(0.95, 1.5, e)
	a := max(max(col.r, col.g), col.b)
	return vec4(col, min(a, 1)) + vec4(0, 0, 0, mix(0.93, 0.80, smoothstep(0.2, 1.3, e)))*(1-min(a, 1))
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

// voroShader is the cells of ice: for p in cells, the point of the nearest cell (in cells) and how far p is from
// the edge between it and the next.
const voroShader = `
func hash2(p vec2) vec2 {
	return vec2(hash(p), hash(p+vec2(19.19, 7.31)))
}

func voro(p vec2) vec3 {
	n := floor(p)
	f := fract(p)
	mg := vec2(0)
	mr := vec2(0)
	md := 8.0
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			g := vec2(float(i-1), float(j-1))
			r := g + hash2(n+g) - f
			d := dot(r, r)
			if d < md {
				md, mr, mg = d, r, g
			}
		}
	}
	md = 8.0
	for j := 0; j < 5; j++ {
		for i := 0; i < 5; i++ {
			g := mg + vec2(float(i-2), float(j-2))
			r := g + hash2(n+g) - f
			if dot(mr-r, mr-r) > 0.00001 {
				md = min(md, dot(0.5*(mr+r), normalize(r-mr)))
			}
		}
	}
	return vec3(n+mg+hash2(n+mg), max(md, 0))
}
`

// crystalLayer is the ground for the ice, after light through crystal: deep blue and violet, and ribbons of light
// that bend out from a bright heart, each white where it is brightest, split into its colors at its edges as light
// through a prism is, and wrapped in a softer glow of itself; a faint frost of crystals toward the edges.
const crystalLayer = voroShader + `
// ribbon is ribbon k at angle a and distance r, and its glow, for the color turned by off
func ribbon(a float, r float, k float, t float, off float) vec2 {
	target := k*0.628 + 0.62*sin(r*2.1+t*0.17+k*1.7) + 0.24*sin(r*4.7-t*0.23+k*0.8)
	x := a - target + off*(0.3+r)
	d := abs(atan2(sin(x), cos(x))) // the angle between, -π..π: mod is not the same for a negative number on every GPU
	w := 0.0035 + r*0.0055
	fade := smoothstep(0.03, 0.30, r) * (1 - smoothstep(0.9, 1.8, r)) * (0.55 + 0.45*sin(k*2.3+t*0.4))
	return vec2(w/(d+w*0.5), (w*7)/(d+w*7)) * fade
}

func layer(p vec2, e float) vec4 {
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
	v := voro(p * 7)
	frost := exp(-max(v.z, 0)*30) * smoothstep(0.6, 1.4, e) * 0.08 // on a GPU the edge can come out a hair below 0
	lobe := vec3(0.10, 0.12, 0.35)*exp(-length(p-vec2(-0.5, 0.3))*2.2) + vec3(0.22, 0.10, 0.30)*exp(-length(p-vec2(0.55, -0.35))*2.4)
	ground := vec3(0.025, 0.025, 0.07) + lobe*0.8
	col := ground + light + vec3(0.6, 0.7, 1.0)*frost
	al := mix(0.96, 0.88, smoothstep(0.3, 1.4, e))
	return vec4(col*al, al)
}
`

// prismLayer is a burst of light through a prism: beams out from a dark heart, each of its own breadth and tint,
// turning slowly, some one way and some the other, breathing, wider as they go out, and split into red, green and
// blue more the farther out they are; a blue haze about them.
const prismLayer = `
func beam(a float, r float, k float, t float) float {
	dirn := step(0.5, hash(vec2(k, 8.1)))*2 - 1
	ak := 6.28318*hash(vec2(k, 1.7)) + t*(0.025+0.035*hash(vec2(k, 3.3)))*dirn
	x := a - ak
	da := atan2(sin(x), cos(x))
	w := (0.025 + 0.09*hash(vec2(k, 5.9))) * (1 + r*0.7)
	b := (0.45 + 0.9*hash(vec2(k, 2.4))) * (0.7 + 0.3*sin(t*0.55+k*2.1))
	return b * exp(-(da*da)/(w*w))
}

func layer(p vec2, e float) vec4 {
	r := length(p)
	a := atan2(p.y, p.x)
	split := 0.035 * r
	col := vec3(0)
	for i := 0; i < 18; i++ {
		k := float(i)
		tint := mix(vec3(0.55, 0.70, 1.0), vec3(1.0, 0.82, 0.55), hash(vec2(k, 6.6))*hash(vec2(k, 7.7)))
		col += tint * vec3(beam(a+split, r, k, Time), beam(a, r, k, Time), beam(a-split, r, k, Time))
	}
	reach := smoothstep(0.04, 0.24, r) * exp(-r*1.5) * 2.0 // dark at the heart, bright about it, fading out
	haze := vec3(0.18, 0.25, 0.60) * exp(-r*2.0) * 0.5 * smoothstep(0.02, 0.2, r)
	light := vec3(1) - exp(-(col*reach + haze))
	light *= 1 - smoothstep(1.0, 1.7, e)
	ground := vec3(0.02, 0.02, 0.05)
	al := mix(0.96, 0.88, smoothstep(0.3, 1.4, e))
	return vec4((ground+light)*al, al)
}
`

// auroraLayer is a curtain of light hanging from an arch above the words: columns of light that shift slowly, each
// falling its own length and spreading as it falls, bright and touched with pink along the arch and blue below; a
// few stars in a deep night.
const auroraLayer = fbmShader + `
func layer(p vec2, e float) vec4 {
	x := p.x
	top := -0.46 + 0.36*x*x/0.36 // the arch
	below := p.y - top
	u := x / (1 + max(below, 0)*0.9) // the columns spread as they fall
	cols := pow(fbm(vec2(u*30, Time*0.06)), 2.2) * 2.6
	fine := noise(vec2(u*150, Time*0.25)) * 0.35
	drop := 0.14 + 0.22*noise(vec2(u*8, Time*0.05+3))
	fall := smoothstep(-0.004, 0.004, below) * exp(-max(below, 0)/drop)
	rim := exp(-abs(below)/0.010) * 0.8
	width := 1 - smoothstep(0.40, 0.66, abs(x))
	amount := ((cols+fine)*fall + rim*(cols+0.2)) * width
	pink := noise(vec2(u*10, 1.3+Time*0.03)) * exp(-max(below, 0)/0.05)
	tint := mix(vec3(0.30, 0.46, 1.0), vec3(1.0, 0.48, 0.72), pink*0.8)
	light := vec3(1) - exp(-tint*amount*1.4)
	light *= mix(0.55, 1.0, smoothstep(-0.05, -0.20, p.y)) // the words, below the arch, keep their ground
	st := hash(floor(p * 260))
	star := step(0.9975, st) * (0.5 + 0.5*sin(Time*1.5+st*50)) * (1 - fall)
	ground := mix(vec3(0.015, 0.02, 0.05), vec3(0.03, 0.04, 0.10), smoothstep(0.6, -0.4, p.y))
	col := ground + light + vec3(0.8, 0.85, 1.0)*star*0.7
	al := mix(0.96, 0.88, smoothstep(0.3, 1.4, e))
	return vec4(col*al, al)
}
`

// iceOpening: the window is behind ice, which forms over it, pale and frosted. It is struck in the middle: a flash,
// cracks running out straight from there, and between them the ice breaking into pieces, small where it was
// struck and larger outward, their edges lit violet and white and split into colors. Then the pieces go, from the
// middle outward, each flaring as it slips away, and the window is under them, clear.
const iceOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	far := length(mid)
	rel := pos - mid
	d := length(rel)
	// the pieces: cells that grow with the distance from the blow
	scale := 34 + d*0.30
	v := voro(rel / scale)
	at := mid + v.xy*scale
	edge := max(v.z, 0) * scale
	h := hash(v.xy)
	dc := distance(at, mid) / far
	dir := normalize(at - mid + vec2(0.001, 0))

	formed := smoothstep(0.0, 0.14, Prog)
	front := (Prog - 0.10) * 2.4 // how far the cracks have run, 1 at the corners
	cracked := smoothstep(dc-0.02, dc+0.05, front)
	tb := 0.44 + dc*0.32 + h*0.10
	gone := smoothstep(tb, tb+0.18, Prog)

	// the straight cracks out from the blow, which run first
	straight := 0.0
	for i := 0; i < 11; i++ {
		fi := float(i)
		an := 6.28318*(fi+0.5*hash(vec2(fi, 2.2)))/11
		u := vec2(cos(an), sin(an))
		along := dot(rel, u)
		across := abs(dot(rel, vec2(-u.y, u.x)) + 14*sin(along/80+fi))
		reach := smoothstep(along-60, along, (Prog-0.06)*3.0*far)
		straight += step(0, along) * exp(-across/0.8) * reach * exp(-along/(far*0.7))
	}
	straight = min(straight, 1) * (1 - smoothstep(0.5, 0.85, Prog)) * 0.8

	clean := layerSample(pos)
	// the ice: the window through it, pale and frosted, each piece bent a little and lit at its rim; a piece that
	// goes slips outward and shrinks
	sc := 1 - gone*0.22
	src := at + (pos-at)/sc - dir*gone*gone*(40+60*h) + dir*h*4
	seen := layerSample(clamp(src, vec2(0), Size))
	lum := dot(seen.rgb, vec3(0.299, 0.587, 0.114))
	frost := noise(pos/7)*0.07 + noise(pos/29)*0.10
	ice := mix(seen.rgb*0.5, vec3(0.74, 0.84, 1.0)*(0.42+lum*0.7), 0.62) + vec3(frost)
	ice += vec3(0.70, 0.80, 1.0) * exp(-edge/10) * 0.30 * cracked // the light in a piece's edge
	// the cracks: violet, white at their hearts, split into colors; they take the place of what is under them, so that
	// they show on a pale window as on a dark one
	rimR, rimG, rimB := exp(-edge/2.6), exp(-edge/1.8), exp(-edge/2.2)
	rimAmt := max(max(rimR, rimG), rimB)
	rimCol := mix(vec3(rimR*0.95, rimG*0.55, rimB*1.0)/max(rimAmt, 0.001), vec3(1), exp(-edge/0.7))
	crack := cracked * (1 - gone)
	flare := gone * (1 - gone) * 4 // a piece is bright as it goes
	blow := exp(-d/150) * smoothstep(0.04, 0.12, Prog) * (1 - smoothstep(0.12, 0.45, Prog)) // the flash of the blow
	piece := (1 - gone) * formed
	col := clean.rgb*(1-piece) + ice*piece*clean.a
	col = mix(col, rimCol*vec3(0.85, 0.75, 1.0), rimAmt*crack*0.9*formed)
	col += (vec3(0.80, 0.70, 1.0)*flare*0.30 + vec3(0.88, 0.82, 1.0)*straight) * formed
	col += vec3(0.90, 0.85, 1.0) * blow * 1.4
	a := max(clean.a*formed, min(max(max(col.r, col.g), col.b), 1)*formed)
	return vec4(col, a)
}
`

// burstOpening, after a crystal blown apart: a flash of violet and pink in the middle, cracks thick with light running
// out from it, streaks of speed out on every side, the window split into its colors, the more the nearer the blow,
// and the pieces flung outward, fast, until it is whole and still.
const burstOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	far := length(mid)
	rel := pos - mid
	d := length(rel)
	ang := atan2(rel.y, rel.x)
	dirp := rel / max(d, 1)

	scale := 26 + d*0.26
	v := voro(rel / scale)
	at := mid + v.xy*scale
	edge := max(v.z, 0) * scale
	h := hash(v.xy)
	dc := distance(at, mid) / far
	dir := normalize(at - mid + vec2(0.001, 0))

	front := Prog * 3.2
	cracked := smoothstep(dc-0.02, dc+0.06, front)
	fly := 1 - smoothstep(0.0, 0.75, Prog) // the pieces are out, and come home
	fly = fly * fly
	push := dir * fly * (90 + 160*h) * cracked
	split := (1 - smoothstep(0.1, 0.9, Prog)) * 26 * exp(-d/(far*0.8))
	src := pos - push
	r := layerSample(clamp(src+dirp*split, vec2(0), Size))
	g := layerSample(clamp(src, vec2(0), Size))
	b := layerSample(clamp(src-dirp*split, vec2(0), Size))
	col := vec4(r.r, g.g, b.b, max(max(r.a, g.a), b.a))
	col *= smoothstep(0.0, 0.18, Prog)

	life := 1 - smoothstep(0.20, 0.62, Prog)
	core := exp(-edge/1.1)
	halo := exp(-edge/(5.0-3.0*Prog))
	crack := cracked * life
	lightC := (vec3(1.0, 0.95, 1.0)*core*1.2 + vec3(0.75, 0.40, 1.0)*halo*0.9) * crack
	// streaks of speed, out from the middle
	sector := floor(ang * 48 / 6.28318)
	sh := hash(vec2(sector, 4.4))
	run := fract(sh - Prog*1.6)
	streak := step(0.72, sh) * exp(-abs(fract(ang*48/6.28318)-0.5)*14) * smoothstep(0.0, 0.2, run) * (1 - smoothstep(0.2, 0.5, run))
	streak *= smoothstep(far*0.15, far*0.5, d) * life
	sc := vec3(streak) * mix(vec3(1.0, 0.5, 0.9), vec3(0.5, 0.7, 1.0), sh)
	flash := exp(-d/(170+300*Prog)) * smoothstep(0.0, 0.06, Prog) * (1 - smoothstep(0.06, 0.5, Prog))
	light := lightC + sc*1.1 + vec3(0.95, 0.70, 1.0)*flash*1.8
	col.rgb += light
	col.a = max(col.a, min(max(max(light.r, light.g), light.b), 1))
	return col
}
`

// horizonOpening, after a seam of light: a line of white light across the middle, flames of colored light rising
// and falling from it, which opens up and down, the window in the opening, its edges white-hot and split into
// colors, until the light has gone.
const horizonOpening = `package main

var Prog float
var Size vec2
var Time float

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	y := (pos.y - mid.y) / mid.y
	x := (pos.x - mid.x) / mid.x
	open := smoothstep(0.18, 0.88, Prog) * 1.12
	dy := abs(y) - open
	inside := 1 - smoothstep(-0.015, 0.0, dy)
	clean := layerSample(pos)
	life := smoothstep(0.0, 0.10, Prog) * (1 - smoothstep(0.72, 1.0, Prog))
	band := 0.014 + 0.045*smoothstep(0.0, 0.25, Prog)*(1-open*0.6)
	// the edge of the opening: white-hot, red outside it and blue inside
	gr := exp(-abs(dy-0.010)/band)
	gg := exp(-abs(dy)/band)
	gb := exp(-abs(dy+0.010)/band)
	// the flames: thin bright ridges of noise drawn out from the edge, outward only
	q := vec2(x*2.2, max(dy, 0)*5.0 - Time*0.8)
	n := fbm(q + vec2(0, fbm(q*1.6)))
	ridge := pow(1-abs(n*2-1), 12.0) * exp(-max(dy, 0)/0.16) * smoothstep(-0.01, 0.01, dy)
	hue := fbm(vec2(x*1.3+Time*0.1, 2.0))
	flame := mix(vec3(0.30, 0.60, 1.0), vec3(1.0, 0.55, 0.30), smoothstep(0.35, 0.65, hue)) * ridge * 1.2
	hot := vec3(gr, gg, gb) * (1.5 - open*0.5) * exp(-x*x*0.8) * mix(0.35, 1.0, step(0, dy)) // softer on the window's side
	light := (hot + flame) * life
	light = vec3(1) - exp(-light*1.5)
	col := clean*inside*smoothstep(0.1, 0.3, Prog)
	col.rgb += light
	col.a = max(col.a, min(max(max(light.r, light.g), light.b), 1))
	return col
}
`

// glassOpening, after a letter in glass: the window comes first as glass in relief, each edge of what is in it raised,
// lit from above, a highlight on it, its rims split into colors, the window seen bent through it; and the glass
// clears into the window.
const glassOpening = `package main

var Prog float
var Size vec2

func lum(p vec2) float {
	c := layerSample(p)
	return dot(c.rgb, vec3(0.299, 0.587, 0.114)) + (1-c.a)*0.0
}

func LayerEffect(pos vec2) vec4 {
	s := 2.5
	gx := lum(pos+vec2(s, 0)) - lum(pos-vec2(s, 0))
	gy := lum(pos+vec2(0, s)) - lum(pos-vec2(0, s))
	n := normalize(vec3(-gx*5, -gy*5, 1))
	glassy := 1 - smoothstep(0.45, 1.0, Prog)
	clean := layerSample(pos + n.xy*10*glassy)
	ld := normalize(vec3(-0.45, -0.65, 0.6))
	diff := clamp(dot(n, ld)*0.5+0.5, 0, 1)
	spec := pow(max(dot(reflect(-ld, n), vec3(0, 0, 1)), 0), 28)
	g := vec2(gx, gy)
	gl := length(g)
	gdir := g / max(gl, 0.0001)
	e0 := clamp(gl*4, 0, 1)
	er := clamp(length(vec2(lum(pos+gdir*2+vec2(s, 0))-lum(pos+gdir*2-vec2(s, 0)), lum(pos+gdir*2+vec2(0, s))-lum(pos+gdir*2-vec2(0, s))))*4, 0, 1)
	eb := clamp(length(vec2(lum(pos-gdir*2+vec2(s, 0))-lum(pos-gdir*2-vec2(s, 0)), lum(pos-gdir*2+vec2(0, s))-lum(pos-gdir*2-vec2(0, s))))*4, 0, 1)
	relief := smoothstep(0.0, 0.35, gl*4) // the glass shows where there is an edge to raise; the flat is the window's own
	shade := (diff - 0.5) * 0.55 * relief
	rims := vec3(er*1.0, e0*0.80, eb*1.15)
	glass := clean.rgb*0.92 + vec3(0.55, 0.65, 1.0)*0.05 + vec3(shade) + rims*0.85 + vec3(spec)*1.2*relief
	out := mix(clean.rgb, glass, glassy)
	a := clean.a * smoothstep(0.0, 0.3, Prog)
	return vec4(out*a, a)
}
`

// spectrumOpening: a ring opens out of the middle, its edge spread into a spectrum, strongest on its left and right
// as light through a lens is, a fainter one behind it; the window is inside it, bent a little where the ring passes.
const spectrumOpening = `package main

var Prog float
var Size vec2

func band(x float, at float) float {
	t := (x - at) / 0.11
	return exp(-t * t)
}

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	rel := pos - mid
	d := length(rel)
	dir := rel / max(d, 1)
	far := length(mid)
	R := Prog * (far + 260)
	w := 110.0
	x := (d - (R - w*0.5)) / w
	lens := abs(rel.x) / max(d, 1)
	amp := (0.12 + 0.88*lens*lens) * (1 - smoothstep(0.65, 1.0, Prog))
	bend := (band(x, -0.05) - band(x, 0.25)) * 18 * amp
	clean := layerSample(pos - dir*bend)
	inner := 1 - smoothstep(R-w, R-w*0.35, d)
	spec := vec3(band(x, 0.13), band(x, 0.0), band(x, -0.13))*0.75 + vec3(band(x, 0.0))*0.35
	echo := vec3(band(x+0.9, 0.10), band(x+0.9, 0.0), band(x+0.9, -0.10)) * 0.22
	light := (spec + echo) * amp
	light = vec3(1) - exp(-light*1.4)
	col := clean * inner
	col.rgb += light
	col.a = max(col.a, min(max(max(light.r, light.g), light.b), 1))
	return col
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

// namedropOpening is after iOS's NameDrop (github.com/MartinRGB/iOS17_Namedrop_R3F): the window comes out of
// its own middle, soft and a little small, and settles on a spring as it sharpens; a wave runs out from the middle,
// bending what it passes and blowing its light out, a glow is there for a moment, and thin streaks of light burst
// out on every side.
const namedropOpening = `package main

var Prog float
var Size vec2

func LayerEffect(pos vec2) vec4 {
	mid := Size * 0.5
	// the size: from 0.86, on a spring, settling exactly at 1
	sp := 1 - 0.14*exp(-4.5*Prog)*cos(8*Prog)
	sc := mix(sp, 1, smoothstep(0.82, 1.0, Prog))
	uv := mid + (pos-mid)/sc
	// the wave: from the middle, faster as it goes
	d := distance(uv, mid)
	far := length(mid)
	r := Prog * Prog * far * 1.35
	diff := (d - r) / 120
	band := 1 - smoothstep(0.0, 1.0, abs(diff))
	life := 1 - smoothstep(0.55, 1.0, Prog)
	dir := (uv - mid) / max(d, 1)
	uv += dir * diff * band * 22 * life
	// soft at first: a few samples round the place, closer as it sharpens
	soft := (1 - smoothstep(0.0, 0.6, Prog)) * 7
	col := vec4(0)
	if uv.x >= 0 && uv.y >= 0 && uv.x <= Size.x && uv.y <= Size.y {
		col = layerSample(uv)*0.4 + (layerSample(uv+vec2(soft, 0))+layerSample(uv-vec2(soft, 0))+layerSample(uv+vec2(0, soft))+layerSample(uv-vec2(0, soft)))*0.15
	}
	col *= smoothstep(0.0, 0.35, Prog)
	col.rgb += col.rgb * band * 1.3 * life
	// the streaks: thin lines of light running out from the middle on every side
	glow := 0.0
	for i := 0; i < 40; i++ {
		fi := float(i)
		a := 6.28318 * hash(vec2(fi, 3.1))
		v := vec2(cos(a), sin(a))
		t := clamp((Prog-0.10-0.10*hash(vec2(fi, 9.7)))/0.55, 0, 1)
		head := t * (180 + far*0.9*hash(vec2(fi, 1.3)))
		along := dot(pos-mid, v)
		across := abs(dot(pos-mid, vec2(-v.y, v.x)))
		inLine := smoothstep(head-90, head, along) * (1 - smoothstep(head, head+4, along))
		glow += inLine * (1 - smoothstep(0.5, 1.5, across)) * (1 - t) * step(0.001, t)
	}
	flash := exp(-distance(pos, mid)/220) * smoothstep(0.0, 0.18, Prog) * (1 - smoothstep(0.18, 0.7, Prog))
	light := vec3(0.82, 0.88, 1.0) * min(glow*1.1+flash*0.8, 1)
	col.rgb += light
	col.a = max(col.a, max(max(light.r, light.g), light.b))
	return col
}
`
