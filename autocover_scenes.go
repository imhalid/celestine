package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// More landscapes for the automatic covers (autocover.go), and what moves in them.
//
// Seen from above: rice terraces holding the sky, a river under a stone bridge, a tulip field
// and its windmill, a raked Japanese garden with its koi, a frozen lake and its skaters, a
// harbour of moored boats, a camp round its fire, a railway through a forest, an autumn forest.
// From the side: a coral reef, the sea cut through under a boat, cherry trees under a snowy
// cone, a city in the rain, a storm at sea, balloons over a valley, the northern lights, a
// viaduct and its train, space, a huge moon over the sea.
//
// The season is the day's too: in autumn green turns to amber and leaves fall, in winter it
// whitens and snow falls, in spring petals drift.

// coverNow is the day covers are made for: its moon, its season.
var coverNow = time.Now

// sceneOfSeed is the landscape a page's seed picks.
func sceneOfSeed(seed uint64) int {
	if sceneOf >= 0 {
		return sceneOf
	}
	rng := splitmix(seed)
	return int(rng.next() % sceneKinds)
}

// seasonOf is the season of day t, in the north: 0 spring, 1 summer, 2 autumn, 3 winter.
func seasonOf(t time.Time) int {
	switch m := t.Month(); {
	case m >= 3 && m <= 5:
		return 0
	case m >= 6 && m <= 8:
		return 1
	case m >= 9 && m <= 11:
		return 2
	}
	return 3
}

// seasonal is a color of the land in the season: a green turns amber in autumn, pale under
// frost in winter, fresh in spring.
func (ac *autoCover) seasonal(c rgb) rgb {
	if !(c[1] > c[0]*1.08 && c[1] > c[2]*1.05) {
		return c
	}
	ac.green = true
	switch ac.season {
	case 0:
		return c.mix(hexRGB("#b9e07c"), 0.15)
	case 2:
		return c.mix(hexRGB("#c9782e"), 0.5)
	case 3:
		return c.mix(hexRGB("#e8eef4"), 0.55)
	}
	return c
}

// coverLife is what moves in these scenes, in the picture's pixels.
type coverLife struct {
	river     []float64 // the river's middle, a column's
	riverW    float64
	path      float64 // the tulip field's path, its row
	pond      [4]float64
	skaters   [][3]float64 // a loop's middle, and its size
	rail      float64      // the railway, from above
	deck      float64      // the viaduct's top
	fire      [2]float64
	kelp      [][3]float64 // a stalk's foot, and its height
	surface   float64      // under water: where the surface is
	bubbles   [][2]float64 // where they rise from
	fish      int
	fishTop   float64
	rain      bool
	street    float64
	storm     bool
	balloons  [][3]float64 // where, and its colors
	aurora    bool
	comet     bool
	dolphin   [2]float64
	moored    [][3]float64 // a boat by a pier, and its color
	tractor   bool
	trainTop  bool
	surfaceAt float64 // the boat on the sea's surface, under water
}

// kit is what a scene draws with: the canvas, the seed's numbers, the hour's light, and the
// ways of drawImage's own scenes.
type kit struct {
	cv      *canvas
	rng     *splitmix
	l       *coverLight
	ac      *autoCover
	ns      uint64
	sx, sy  float64
	shine   float64
	ground  func(base rgb, depth float64) rgb
	ridge   func(base, amp, freq float64, oct int, sharp bool, k uint64) func(float64) float64
	sea     func()
	reflect func(water rgb)
	water   *float64
}

// in is a color seen from above, in the season and the hour: the hour's light only tints
// it; under the moon it is dark and blue.
func (k *kit) in(hex string) rgb {
	c := k.ac.seasonal(hexRGB(hex)).mul(rgb{255, 255, 255}.mix(k.l.lit, 0.35))
	if k.l.night {
		c = c.mul(rgb{120, 135, 175})
	}
	return c
}

// crown is a tree from above: its shadow down and to the right, its rim dark, lit from the
// top left.
func (k *kit) crown(cx, cy, r float64, leaf, shade rgb) {
	k.cv.disc(cx+r*0.5, cy+r*0.45, r, shade, 0.5)
	k.cv.disc(cx, cy, r, leaf.mix(rgb{20, 40, 16}, 0.45), 1)
	k.cv.disc(cx-r*0.15, cy-r*0.15, r*0.78, leaf, 1)
	k.cv.disc(cx-r*0.35, cy-r*0.35, r*0.38, leaf.mul(rgb{330, 340, 300}), 1) // lighter, in whatever light
}

// canopy fills from y0 to y1 with crowns, close, of these colors.
func (k *kit) canopy(y0, y1 float64, leaves []rgb) {
	base := leaves[0].mix(rgb{10, 24, 12}, 0.55)
	k.cv.rect(0, y0, picW, y1, base, 1)
	for y := y0 - 2; y < y1+3; y += k.rng.in(3, 5) {
		for x := k.rng.in(-4, 0); x < picW; x += k.rng.in(4, 8) {
			c := leaves[int(k.rng.next()%uint64(len(leaves)))]
			k.crown(x, y+k.rng.in(-1.5, 1.5), k.rng.in(3, 5.5), c, base)
		}
	}
}

// overcast covers the sky in cloud, grey, darker for a storm.
func (k *kit) overcast(dark bool) {
	top, low := hexRGB("#8a93a3"), hexRGB("#c2c7cf")
	if dark {
		top, low = hexRGB("#4a5262"), hexRGB("#7c8594")
	}
	if k.l.night {
		top, low = top.mul(rgb{70, 78, 100}), low.mul(rgb{80, 90, 115})
	}
	for y := 0; y < picHz+4; y++ {
		c := top.mix(low, float64(y)/picHz)
		for x := 0; x < picW; x++ {
			n := valueNoise(k.ns+300, float64(x)*0.03, float64(y)*0.12)
			k.cv.set(x, y, c.mix(top.mix(rgb{0, 0, 0}, 0.25), smooth(0.55, 0.85, n)*0.6), 1)
		}
	}
	k.ac.ownSky, k.ac.stars, k.ac.shooting = true, nil, false
}

// ownSky paints the sky from top to bottom, and stars in it.
func (k *kit) ownSky(top, low rgb, stars int) {
	for y := 0; y < picH; y++ {
		c := top.mix(low, smooth(0, 1, float64(y)/picHz))
		for x := 0; x < picW; x++ {
			k.cv.set(x, y, c, 1)
		}
	}
	k.ac.ownSky, k.ac.stars = true, nil
	for i := 0; i < stars; i++ {
		s := coverStar{x: k.rng.f() * picW, y: k.rng.f() * picHz * 0.8, b: k.rng.in(0.3, 1), ph: k.rng.f() * 6.28}
		k.ac.stars = append(k.ac.stars, s)
		k.cv.set(int(s.x), int(s.y), rgb{255, 255, 255}, s.b*0.4)
	}
}

func (k *kit) draw(scene int) {
	cv, rng, ac, lf := k.cv, k.rng, k.ac, &k.ac.life
	night := k.l.night
	switch scene {
	case sceneTerraces:
		ac.noSky = true
		water, rice, bund := k.in("#a8d3d6"), k.in("#86b84c"), k.in("#587a36")
		phase := rng.f() * 6
		edge := func(i int, x float64) float64 {
			return -6 + float64(i)*9 + 5*math.Sin(x*0.008+float64(i)*0.45+phase) + 3*fbm(k.ns+200+uint64(i), x*0.015, 2)
		}
		const bands = 14
		flooded := make([]bool, bands)
		for i := range flooded {
			flooded[i] = rng.f() < 0.45
		}
		for x := 0; x < picW; x++ {
			fx := float64(x)
			for y := 0; y < picH; y++ {
				fy := float64(y)
				i := 0
				for i < bands-1 && edge(i+1, fx) <= fy {
					i++
				}
				top := edge(i, fx)
				d := fy - top
				var c rgb
				switch {
				case d < 1:
					c = bund
				case d < 2:
					c = bund.mix(rice, 0.5)
				case flooded[i]:
					c = water.mix(k.in("#e6f4f4"), 0.35*valueNoise(k.ns+201, fx*0.05, fy*0.2))
				default:
					c = rice
					if (x+i*2)%3 == 0 {
						c = rice.mix(bund, 0.3)
					}
				}
				cv.set(x, y, c, 1)
			}
		}
		for len(ac.glints) < 40 {
			x, y := rng.in(0.2, 0.8)*picW, rng.in(4, 70)
			if c := cv.get(int(x), int(y)); c[2] > c[0]+20 {
				ac.glints = append(ac.glints, [2]float64{x, y})
			}
		}
		ac.birds = !night
	case sceneRiver:
		ac.noSky = true
		grass, field := k.in("#7aa652"), k.in("#9cbb5e")
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ { // fields, some greener than others
				n := valueNoise(k.ns+210, float64(x)*0.012, float64(y)*0.03)
				c := grass
				if n > 0.6 {
					c = field
				}
				cv.set(x, y, c.mix(rgb{40, 70, 30}, 0.12*valueNoise(k.ns+211, float64(x)*0.3, float64(y)*0.3)), 1)
			}
		}
		lf.river, lf.riverW = make([]float64, picW), 7
		ph := rng.f() * 6
		for x := range lf.river {
			lf.river[x] = 46 + 18*math.Sin(float64(x)*0.005+ph) + 8*(fbm(k.ns+212, float64(x)*0.01, 2)-0.5)
		}
		bx := rng.in(0.4, 0.6) * picW
		road := k.in("#c9b48a")
		for y := 0; y < picH; y++ { // a road, down to the bridge and on
			cx := bx + 4*math.Sin(float64(y)*0.08)
			cv.rect(cx-2, float64(y), cx+2, float64(y+1), road, 1)
		}
		deep, shallow, bank := k.in("#2f7c96"), k.in("#62a9bd"), k.in("#d8c79a")
		for x := 0; x < picW; x++ {
			c := lf.river[x]
			for y := int(c - lf.riverW - 2); y <= int(c+lf.riverW+2); y++ {
				d := math.Abs(float64(y) - c)
				switch {
				case d < lf.riverW:
					cv.set(x, y, shallow.mix(deep, 1-d/lf.riverW), 1)
				case d < lf.riverW+1.5:
					cv.set(x, y, bank, 1)
				}
			}
		}
		leaf, shade := k.in("#4f8a3c"), k.in("#3d5a2c")
		for x := rng.in(0, 8); x < picW; x += rng.in(6, 16) { // trees along the banks
			if math.Abs(x-bx) < 8 {
				continue
			}
			c := lf.river[int(x)]
			side := 1.0
			if rng.f() < 0.5 {
				side = -1
			}
			k.crown(x, c+side*(lf.riverW+rng.in(4, 8)), rng.in(3, 5), leaf, shade)
		}
		c := lf.river[int(bx)] // the bridge, of stone, its parapets
		stone := k.in("#b3aa9a")
		cv.rect(bx-4, c-lf.riverW-3, bx+4, c+lf.riverW+3, stone, 1)
		cv.rect(bx-4, c-lf.riverW-3, bx-3, c+lf.riverW+3, stone.mix(rgb{60, 55, 50}, 0.4), 1)
		cv.rect(bx+3, c-lf.riverW-3, bx+4, c+lf.riverW+3, stone.mix(rgb{60, 55, 50}, 0.4), 1)
		cv.rect(bx+4, c-lf.riverW-1, bx+6, c+lf.riverW+1, shade, 0.35) // its shadow on the water
		ac.birds = !night
	case sceneTulips:
		ac.noSky = true
		colors := []string{"#e0453a", "#f2c43a", "#f08fb5", "#8a5bd0", "#f08a3a"}
		leaf, soil := k.in("#5f8f3e"), k.in("#9a7a52")
		ph := rng.f() * 6
		type stripe struct {
			y0, y1 float64
			c      rgb
		}
		var stripes []stripe
		lf.path = 0
		for y := -6.0; y < picH+6; {
			h := 7.0
			if lf.path == 0 && y > 36 {
				lf.path = y + 2 // a path along one gap, wider
				y += 5
				continue
			}
			stripes = append(stripes, stripe{y, y + h, k.in(colors[int(rng.next()%uint64(len(colors)))])})
			y += h + 2
		}
		for x := 0; x < picW; x++ {
			bend := 2 * math.Sin(float64(x)*0.004+ph)
			for y := 0; y < picH; y++ {
				fy := float64(y) - bend
				c := leaf
				if math.Abs(fy-lf.path) < 2 {
					c = soil
				}
				for _, s := range stripes {
					if fy >= s.y0 && fy < s.y1 {
						h := splitmix(k.ns ^ uint64(x)*0x9e37 ^ uint64(y)*0xc2b2)
						c = s.c
						if h.f() < 0.18 {
							c = leaf
						} else if h.f() < 0.2 {
							c = s.c.mix(rgb{255, 255, 255}, 0.4)
						}
						break
					}
				}
				cv.set(x, y, c, 1)
			}
		}
		mx, my := rng.in(0.55, 0.7)*picW, 22.0 // a windmill, from above: its round cap; its sails turn
		cv.disc(mx+3, my+3, 7, leaf.mix(rgb{0, 0, 0}, 0.4), 0.5)
		cv.disc(mx, my, 7, k.in("#8a5a3a"), 1)
		cv.disc(mx-1, my-1, 5, k.in("#a87048"), 1)
		ac.mills = append(ac.mills, [3]float64{mx, my, 14})
		ac.millInk = k.in("#efe6d6")
		lf.tractor = true
		ac.birds = !night
	case sceneZen:
		ac.noSky = true
		gravel, groove, moss := k.in("#e6e1d6"), k.in("#cbc3b2"), k.in("#7f9e58")
		type stone struct{ x, y, r float64 }
		var stones []stone
		for i := 0; i < 4; i++ {
			stones = append(stones, stone{(0.26 + 0.1*float64(i) + rng.in(-0.02, 0.02)) * picW, rng.in(26, 58), rng.in(3, 6)})
		}
		pcx, pcy, prx, pry := rng.in(0.64, 0.72)*picW, 42.0, 34.0, 15.0
		lf.pond = [4]float64{pcx, pcy, prx, pry}
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ {
				fx, fy := float64(x), float64(y)
				d := 1e9
				for _, s := range stones {
					d = math.Min(d, math.Hypot(fx-s.x, (fy-s.y)*1.3)-s.r)
				}
				pd := math.Hypot((fx-pcx)/prx, (fy-pcy)/pry)
				c := gravel
				switch { // the rake's lines: round each stone, and straight elsewhere
				case d < 15:
					if math.Mod(d, 3) < 1 {
						c = groove
					}
				case pd < 1.5:
					if math.Mod((pd-1)*prx, 3) < 1 {
						c = groove
					}
				default:
					if y%3 == 0 {
						c = groove
					}
				}
				cv.set(x, y, c, 1)
			}
		}
		water := k.in("#2e6b70")
		for y := int(pcy - pry - 2); y <= int(pcy+pry+2); y++ { // the pond, its stones round it
			for x := int(pcx - prx - 2); x <= int(pcx+prx+2); x++ {
				pd := math.Hypot((float64(x)-pcx)/prx, (float64(y)-pcy)/pry)
				switch {
				case pd < 1:
					cv.set(x, y, water.mix(k.in("#5aa0a0"), 0.4*(1-pd)), 1)
				case pd < 1.09 && (x+y)%3 != 0:
					cv.set(x, y, k.in("#8f8a80"), 1)
				}
			}
		}
		for i := 0; i < 5; i++ { // lily pads
			a := rng.f() * 6.28
			cv.disc(pcx+math.Cos(a)*prx*0.7, pcy+math.Sin(a)*pry*0.6, rng.in(1.5, 2.5), k.in("#5f9a48"), 1)
		}
		for _, s := range stones {
			cv.disc(s.x+2, s.y+2, s.r, groove.mix(rgb{0, 0, 0}, 0.2), 0.6)
			cv.disc(s.x, s.y, s.r, k.in("#7c7a75"), 1)
			cv.disc(s.x-s.r*0.25, s.y-s.r*0.25, s.r*0.6, k.in("#a19e97"), 1)
			cv.disc(s.x+s.r*0.3, s.y+s.r*0.2, s.r*0.4, moss, 1)
		}
		for x := 0; x < picW; x++ { // moss along the top, and maples
			e := 6 + 4*fbm(k.ns+220, float64(x)*0.04, 2)
			cv.rect(float64(x), 0, float64(x+1), e, moss, 1)
		}
		maple, shade := k.in("#c4482f"), moss.mix(rgb{0, 0, 0}, 0.3)
		if ac.season == 1 || ac.season == 0 {
			maple = k.in("#6f9e44")
		}
		for x := rng.in(0, 30); x < picW; x += rng.in(30, 70) {
			k.crown(x, rng.in(2, 7), rng.in(4, 7), maple, shade)
		}
	case sceneIce:
		ac.noSky = true
		ice, iceDark, crack := k.in("#d4e7f1"), k.in("#a5c6da"), k.in("#f7fbfd")
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ {
				fx, fy := float64(x), float64(y)
				c := ice.mix(iceDark, valueNoise(k.ns+230, fx*0.015, fy*0.05))
				n := fbm(k.ns+231, fx*0.02+fy*0.01, 3) + 0.5*valueNoise(k.ns+232, fx*0.05, fy*0.13)
				if math.Abs(n-0.75) < 0.012 {
					c = crack
				}
				cv.set(x, y, c, 1)
			}
		}
		snow, pine, shade := k.in("#f4f7fa"), k.in("#2f5a42"), k.in("#9db4c8")
		for x := 0; x < picW; x++ {
			e := 12 + 5*fbm(k.ns+233, float64(x)*0.03, 3)
			cv.rect(float64(x), 0, float64(x+1), e, snow, 1)
			cv.rect(float64(x), e, float64(x+1), e+1, shade, 0.6)
		}
		for x := rng.in(0, 10); x < picW; x += rng.in(8, 20) {
			y := rng.in(1, 9)
			k.crown(x, y, rng.in(3, 5), pine, shade)
			cv.disc(x-1, y-1, 1.5, snow, 0.9)
		}
		for i := 0; i < 4; i++ { // skaters, round and round, their marks on the ice
			cx, cy, r := (0.3+0.13*float64(i))*picW+rng.in(-10, 10), rng.in(32, 50), rng.in(10, 18)
			lf.skaters = append(lf.skaters, [3]float64{cx, cy, r})
			for a := 0.0; a < 6.28; a += 0.04 {
				cv.set(int(cx+math.Cos(a)*r*1.6), int(cy+math.Sin(a)*r*0.6), crack, 0.5)
			}
		}
	case sceneReef:
		ac.noSky, ac.noWeather = true, true
		top, deep := k.in("#62c4d6"), k.in("#114e78")
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ {
				c := top.mix(deep, smooth(0, 1, float64(y)/picH))
				if math.Mod(float64(x)+float64(y)*0.6+20*valueNoise(k.ns+240, float64(x)*0.01, 0), 70) < 9 { // rays from above
					c = c.mix(rgb{255, 255, 255}, 0.1*(1-float64(y)/picH))
				}
				cv.set(x, y, c, 1)
			}
		}
		cv.rect(0, 0, picW, 2, k.in("#c8f0f4"), 0.6)
		floor := func(x float64) float64 { return 70 + 5*fbm(k.ns+241, x*0.02, 3) }
		cv.land(floor, k.in("#e2d3a8"), k.in("#b8a57a"))
		corals := []string{"#ff7f6e", "#ffb347", "#c86dd7", "#f5d547", "#ff6fa5", "#6fd0c0"}
		for x := rng.in(0, 10); x < picW; x += rng.in(6, 16) {
			c := k.in(corals[int(rng.next()%uint64(len(corals)))])
			by := floor(x) + 1
			switch rng.next() % 3 {
			case 0: // branching
				h := rng.in(10, 26)
				var branch func(x, y, a, len float64, depth int)
				branch = func(x, y, a, ln float64, depth int) {
					for t := 0.0; t < ln; t += 0.5 {
						cv.disc(x+math.Cos(a)*t, y+math.Sin(a)*t, 1, c, 1)
					}
					if depth > 0 {
						ex, ey := x+math.Cos(a)*ln, y+math.Sin(a)*ln
						branch(ex, ey, a-0.5, ln*0.65, depth-1)
						branch(ex, ey, a+0.45, ln*0.6, depth-1)
					}
				}
				branch(x, by, -math.Pi/2+rng.in(-0.2, 0.2), h*0.45, 3)
			case 1: // a brain coral, round
				r := rng.in(4, 8)
				cv.disc(x, by-r*0.6, r, c, 1)
				for i := 0.0; i < r; i += 2 {
					cv.disc(x, by-r*0.6, r-i, c.mix(rgb{0, 0, 0}, 0.12*math.Mod(i, 4)/2), 0.6)
				}
			default: // a fan
				r := rng.in(8, 16)
				for a := -2.6; a < -0.5; a += 0.08 {
					for t := 0.0; t < r; t += 1 {
						cv.set(int(x+math.Cos(a)*t), int(by+math.Sin(a)*t), c, 0.55)
					}
				}
			}
			if rng.f() < 0.3 {
				lf.bubbles = append(lf.bubbles, [2]float64{x, by - 6})
			}
		}
		lf.fish, lf.fishTop = 22, 14
	case sceneHarbor:
		ac.noSky = true
		water := k.in("#3d8e9a")
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ {
				n := valueNoise(k.ns+250, float64(x)*0.08, float64(y)*0.25)
				cv.set(x, y, water.mix(k.in("#6fb7bf"), 0.25*smooth(0.6, 0.9, n)), 1)
			}
		}
		quay := k.in("#b9b2a4")
		cv.rect(0, 0, picW, 14, quay, 1)
		cv.rect(0, 14, picW, 15, quay.mix(rgb{40, 40, 40}, 0.4), 1)
		cv.rect(0, 15, picW, 17, water.mix(rgb{0, 0, 0}, 0.3), 0.5) // the quay's shadow
		roofs := []string{"#c4553c", "#d9813f", "#b0453a", "#8f6a5a", "#c9a46a"}
		for x := rng.in(-6, 0); x < picW; { // the houses' roofs along the quay
			w := rng.in(10, 18)
			c := k.in(roofs[int(rng.next()%uint64(len(roofs)))])
			cv.rect(x, 0, x+w-1, 9, c, 1)
			cv.rect(x, 4, x+w-1, 5, c.mix(rgb{0, 0, 0}, 0.3), 1)
			cv.rect(x, 0, x+w-1, 4, c.mix(rgb{255, 255, 255}, 0.12), 1)
			x += w
		}
		wood := k.in("#8a6a48")
		for i := 0; i < 5; i++ { // piers out into the water, boats tied along them
			px := (0.24 + 0.13*float64(i) + rng.in(-0.02, 0.02)) * picW
			ln := rng.in(30, 48)
			cv.rect(px+2, 15, px+6, 15+ln+2, water.mix(rgb{0, 0, 0}, 0.35), 0.5)
			cv.rect(px-2, 15, px+2, 15+ln, wood, 1)
			for y := 16.0; y < 15+ln; y += 2 {
				cv.rect(px-2, y, px+2, y+0.6, wood.mix(rgb{0, 0, 0}, 0.3), 1)
			}
			hulls := []string{"#f4f1ea", "#e8eef2", "#d9534a", "#3f6fb0", "#f2c94c"}
			for y := 20.0; y < 15+ln-8; y += rng.in(14, 18) {
				side := 6.0
				if rng.f() < 0.5 {
					side = -6
				}
				lf.moored = append(lf.moored, [3]float64{px + side, y, float64(rng.next() % uint64(len(hulls)))})
			}
		}
		for len(ac.glints) < 30 {
			x, y := rng.in(0.15, 0.85)*picW, rng.in(20, 80)
			if c := cv.get(int(x), int(y)); c[2] > c[0]+20 {
				ac.glints = append(ac.glints, [2]float64{x, y})
			}
		}
		ac.birds = !night
	case sceneCamp:
		ac.noSky = true
		k.canopy(0, picH, []rgb{k.in("#3f6e3a"), k.in("#4f7f3f"), k.in("#355f35")})
		cx, cy, rx, ry := rng.in(0.42, 0.58)*picW, 44.0, 74.0, 26.0
		grass, dirt := k.in("#7f9b52"), k.in("#9c7d55")
		for y := 0; y < picH; y++ {
			for x := 0; x < picW; x++ {
				fx, fy := float64(x), float64(y)
				d := math.Hypot((fx-cx)/rx, (fy-cy)/ry) + 0.15*fbm(k.ns+260, fx*0.05, 2)
				switch {
				case math.Hypot((fx-cx)/12, (fy-cy)/7) < 1:
					cv.set(x, y, dirt, 1)
				case d < 1:
					cv.set(x, y, grass.mix(rgb{60, 90, 40}, 0.15*valueNoise(k.ns+261, fx*0.3, fy*0.3)), 1)
				}
			}
		}
		for a := 0.0; a < 6.28; a += 0.5 { // stones round the fire
			cv.disc(cx+math.Cos(a)*4, cy+math.Sin(a)*2.6, 1, k.in("#8d8a84"), 1)
		}
		for _, a := range []float64{0.3, 1.9, 3.6} { // logs to sit on
			lx, ly := cx+math.Cos(a)*10, cy+math.Sin(a)*6
			cv.rect(lx-3, ly-0.5, lx+3, ly+1, k.in("#6b4a30"), 1)
		}
		tents := []string{"#e07a3a", "#3f7fc0", "#e8c33f"}
		for i, a := range []float64{-2.4, -0.7, 2.6} {
			tx, ty := cx+math.Cos(a)*36, cy+math.Sin(a)*15
			c := k.in(tents[i])
			cv.rect(tx-3, ty-3, tx+8, ty+6, grass.mix(rgb{0, 0, 0}, 0.4), 0.5)
			cv.rect(tx-5, ty-4, tx+5, ty+4, c, 1)
			cv.rect(tx-5, ty-4, tx, ty+4, c.mix(rgb{255, 255, 255}, 0.18), 1)
			cv.rect(tx-0.5, ty-4, tx+0.5, ty+4, c.mix(rgb{0, 0, 0}, 0.35), 1) // its ridge
		}
		lf.fire = [2]float64{cx, cy}
		ac.fireflies = night
	case sceneRail:
		ac.noSky = true
		k.canopy(0, picH, []rgb{k.in("#3d6b3c"), k.in("#4c7c40"), k.in("#5a8a46")})
		lf.rail = rng.in(34, 44)
		ry := lf.rail
		cv.rect(0, ry-6, picW, ry+6, k.in("#6b6a5e"), 0.5) // the clearing's shadow
		cv.rect(0, ry-5, picW, ry+5, k.in("#a8a294"), 1)   // the ballast
		for x := 0.0; x < picW; x += 3 {
			cv.rect(x, ry-3, x+1.5, ry+3, k.in("#6e5038"), 1) // sleepers
		}
		cv.rect(0, ry-2, picW, ry-1, k.in("#d4d6da"), 1)
		cv.rect(0, ry+1, picW, ry+2, k.in("#d4d6da"), 1)
		lf.trainTop = true
	case sceneAutumn:
		ac.noSky, ac.noWeather, ac.leaves = true, true, true
		autumn := []rgb{hexRGB("#d9822b"), hexRGB("#c0392b"), hexRGB("#e8b830"), hexRGB("#a0522d"), hexRGB("#6f8f3a"), hexRGB("#e06a2c")}
		for i := range autumn {
			autumn[i] = autumn[i].mul(rgb{255, 255, 255}.mix(k.l.lit, 0.35))
			if night {
				autumn[i] = autumn[i].mul(rgb{120, 135, 175})
			}
		}
		k.canopy(0, picH, autumn)
		path := k.in("#b7925f")
		ph := rng.f() * 6
		for x := 0; x < picW; x++ { // a path under the trees, seen in its gaps
			py := 46 + 10*math.Sin(float64(x)*0.01+ph)
			if valueNoise(k.ns+270, float64(x)*0.08, 0) > 0.42 {
				cv.rect(float64(x), py-1.5, float64(x+1), py+1.5, path, 1)
			}
		}
	case sceneUnderwater:
		ac.noWeather = true
		lf.surface = picHz - 22
		sf := lf.surface
		top, deep := k.l.low.mix(k.in("#2a9bb0"), 0.75), k.in("#0b3a5c")
		for y := int(sf); y < picH; y++ {
			c := top.mix(deep, smooth(sf, picH, float64(y)))
			for x := 0; x < picW; x++ {
				cc := c
				if math.Mod(float64(x)+float64(y-int(sf))*0.5, 80) < 10 {
					cc = c.mix(rgb{255, 255, 255}, 0.1*(1-(float64(y)-sf)/(picH-sf)))
				}
				cv.set(x, y, cc, 1)
			}
		}
		floor := func(x float64) float64 { return 72 + 5*fbm(k.ns+280, x*0.02, 3) }
		cv.land(floor, k.in("#cdbb8c"), k.in("#9c8a62"))
		for x := rng.in(0, 30); x < picW; x += rng.in(20, 50) { // rocks
			cv.disc(x, floor(x)+1, rng.in(3, 6), k.in("#5f6b70"), 1)
		}
		for x := rng.in(0, 8); x < picW; x += rng.in(7, 18) {
			lf.kelp = append(lf.kelp, [3]float64{x, floor(x) + 1, rng.in(16, 38)})
			if rng.f() < 0.25 {
				lf.bubbles = append(lf.bubbles, [2]float64{x + 2, floor(x) - 4})
			}
		}
		lf.fish, lf.fishTop = 12, sf+8
		lf.surfaceAt = rng.in(0.35, 0.65) * picW
		ac.birds = !night
	case sceneSakura:
		ac.noWeather, ac.petals = true, true
		fx := rng.in(0.4, 0.6) * picW // a snowy cone far off
		cone := func(x float64) float64 {
			d := math.Abs(x-fx) / 120
			return picHz + 2 - 40*math.Max(0, 1-d)*math.Max(0, 1-d*0.4) + 2*smooth(0.06, 0, d)
		}
		cv.land(cone, k.ground(hexRGB("#7f8fb0"), 0.4), k.ground(hexRGB("#7f8fb0"), 0.45))
		for x := 0; x < picW; x++ {
			t := cone(float64(x))
			for y := int(t); float64(y) < picHz-24+3*valueNoise(k.ns+290, float64(x)*0.3, 0); y++ {
				cv.set(x, y, k.ground(hexRGB("#f2f5fa"), 0.6), 1)
			}
		}
		hill := k.ridge(picHz+8, 8, 0.005, 3, false, 291)
		cv.land(hill, k.ground(hexRGB("#7fae5e"), 0.9), k.ground(hexRGB("#5b8a46"), 1))
		tx := fx - rng.in(80, 140) // a torii, red, on the hill
		if rng.f() < 0.5 {
			tx = fx + rng.in(80, 140)
		}
		red := k.ground(hexRGB("#c8372d"), 0.95)
		ty := hill(tx)
		cv.rect(tx-6, ty-14, tx-4, ty+1, red, 1)
		cv.rect(tx+4, ty-14, tx+6, ty+1, red, 1)
		cv.rect(tx-9, ty-15, tx+9, ty-13, red, 1)
		cv.rect(tx-7, ty-11, tx+7, ty-10, red, 1)
		pink, pale, deepP := k.ground(hexRGB("#f4b3c7"), 1), k.ground(hexRGB("#fbd9e3"), 1), k.ground(hexRGB("#d98aa6"), 1)
		bark := k.ground(hexRGB("#4a3530"), 1)
		for i := 0; i < 4; i++ { // cherry trees in flower
			x := (0.2 + 0.2*float64(i) + rng.in(-0.05, 0.05)) * picW
			by := hill(x) + 2
			h := rng.in(18, 26)
			cv.rect(x-1, by-h*0.6, x+1, by, bark, 1)
			for j := 0; j < 14; j++ {
				a := rng.in(-math.Pi, 0)
				r := rng.in(0, h*0.5)
				px, py := x+math.Cos(a)*r*1.4, by-h*0.6+math.Sin(a)*r*0.8
				rr := rng.in(3, 6)
				cv.disc(px+1, py+1, rr, deepP, 1)
				cv.disc(px, py, rr, pink, 1)
				cv.disc(px-1, py-1, rr*0.5, pale, 1)
			}
		}
	case sceneRain:
		ac.noWeather = true
		k.overcast(false)
		lf.rain, lf.street = true, picHz+8
		c := k.ground(hexRGB("#5d6678"), 0.9)
		lit := hexRGB("#ffd27a")
		for x := rng.in(-10, 0); x < picW; {
			w, h := rng.in(8, 18), rng.in(10, 30)
			top := lf.street - h
			cv.rect(x, top, x+w, lf.street, c.mix(rgb{255, 255, 255}, rng.in(0, 0.08)), 1)
			for wy := top + 2; wy < lf.street-2; wy += 3 {
				for wx := x + 1.5; wx < x+w-1; wx += 2.5 {
					if rng.f() < 0.35 || (night && rng.f() < 0.25) {
						cv.set(int(wx), int(wy), lit, 0.85)
					}
				}
			}
			x += w + rng.in(0, 2)
		}
		asphalt := k.ground(hexRGB("#3e4452"), 1)
		cv.rect(0, lf.street, picW, picH, asphalt, 1)
		cv.rect(0, lf.street, picW, lf.street+1, k.ground(hexRGB("#8d93a0"), 1), 1) // the kerb
		for x := 0.0; x < picW; x += rng.in(2, 5) {                                 // the lights in the wet street
			if rng.f() < 0.35 {
				cv.rect(x, lf.street+2, x+1, lf.street+2+rng.in(3, 8), lit, 0.3)
			}
		}
		for x := rng.in(20, 60); x < picW; x += 90 { // street lamps
			cv.rect(x, lf.street-14, x+1, lf.street, k.ground(hexRGB("#2a2e38"), 1), 1)
			cv.glow(x+0.5, lf.street-14, 9, lit, 0.5)
			cv.rect(x-1, lf.street-15, x+2, lf.street-13, lit, 1)
			cv.rect(x-2, lf.street+2, x+3, lf.street+8, lit, 0.25)
		}
	case sceneStorm:
		ac.noWeather = true
		k.overcast(true)
		lf.storm, lf.rain = true, true
		sea := k.ground(hexRGB("#2d4655"), 1)
		for y := picHz; y < picH; y++ {
			for x := 0; x < picW; x++ {
				fx, fy := float64(x), float64(y)
				c := sea.mix(rgb{10, 20, 30}, 0.3*(fy-picHz)/(picH-picHz))
				w := math.Sin(fx*0.05 + fy*0.9 + 6*valueNoise(k.ns+300, fx*0.02, fy*0.1))
				if w > 0.82 {
					c = c.mix(rgb{220, 230, 235}, 0.45)
				} else if w < -0.6 {
					c = c.mix(rgb{0, 0, 0}, 0.2)
				}
				cv.set(x, y, c, 1)
			}
		}
		bx := rng.in(0.35, 0.65) * picW // a ship on the horizon, leaning
		dark := k.ground(hexRGB("#1c2228"), 1)
		for i := 0.0; i < 14; i++ {
			cv.rect(bx-7+i, picHz-2+i*0.12, bx-6+i, picHz+1, dark, 1)
		}
		cv.rect(bx-2, picHz-9, bx-1, picHz-1, dark, 1)
	case sceneBalloons:
		cv.land(k.ridge(picHz, picHz*0.5, 0.006, 5, true, 310), k.ground(hexRGB("#7484a6"), 0.35), k.ground(hexRGB("#7484a6"), 0.4))
		cv.land(k.ridge(picHz+6, 10, 0.005, 3, false, 311), k.ground(hexRGB("#86ae64"), 0.7), k.ground(hexRGB("#6a9650"), 0.75))
		cv.land(k.ridge(picHz+14, 6, 0.008, 3, false, 312), k.ground(hexRGB("#6f9e52"), 1), k.ground(hexRGB("#4f7c3e"), 1))
		for i := 0; i < 6; i++ {
			lf.balloons = append(lf.balloons, [3]float64{(0.22 + 0.11*float64(i) + rng.in(-0.03, 0.03)) * picW, rng.in(10, 40), float64(rng.next() % 6)})
		}
	case sceneAurora:
		ac.noWeather = true
		k.ownSky(hexRGB("#060f22"), hexRGB("#1d3a5a"), 90)
		lf.aurora = true
		cv.land(k.ridge(picHz, 16, 0.006, 4, true, 320), hexRGB("#3a5272"), hexRGB("#4a6282"))
		snow := hexRGB("#7d8ea8") // under a white title, in the dark
		if !k.ac.dark {
			snow = hexRGB("#e4ecf5")
		}
		cv.land(k.ridge(picHz+8, 5, 0.005, 3, false, 321), snow, snow.mix(hexRGB("#ffffff"), 0.12))
		for x := rng.in(0, 30); x < picW; x += rng.in(14, 40) {
			cv.pine(x, picHz+7+rng.f()*4, rng.in(7, 13), hexRGB("#13202e"))
		}
		hx := rng.in(0.35, 0.65) * picW // a cabin, its window lit
		cv.rect(hx-6, picHz+2, hx+6, picHz+9, hexRGB("#3a2a24"), 1)
		cv.tri(hx, picHz+3, 15, 6, snow)
		cv.rect(hx-2, picHz+4, hx+1, picHz+7, hexRGB("#ffcf6e"), 1)
		cv.glow(hx, picHz+5, 8, hexRGB("#ffcf6e"), 0.35)
	case sceneViaduct:
		cv.land(k.ridge(picHz-6, picHz*0.55, 0.006, 5, true, 330), k.ground(hexRGB("#7484a6"), 0.35), k.ground(hexRGB("#7484a6"), 0.4))
		cv.land(k.ridge(picHz, 10, 0.006, 3, false, 331), k.ground(hexRGB("#7aa25a"), 0.6), k.ground(hexRGB("#6a9050"), 0.65))
		lf.deck = picHz - 16
		d := lf.deck
		stone := k.ground(hexRGB("#a3988a"), 0.85)
		const span = 26.0
		for x := 0; x < picW; x++ {
			u := math.Mod(float64(x), span)/span*2 - 1 // -1..1 between two piers
			open := d + 7 + 16*(1-math.Sqrt(math.Max(0, 1-u*u*1.15)))
			if math.Abs(u) > 0.93 {
				open = picH
			}
			for y := int(d); y < int(open) && y < picH; y++ {
				c := stone
				if y < int(d)+2 {
					c = stone.mix(rgb{255, 255, 255}, 0.15)
				} else if (x+y/3)%7 == 0 {
					c = stone.mix(rgb{0, 0, 0}, 0.12)
				}
				cv.set(x, y, c, 1)
			}
		}
		cv.land(k.ridge(picHz+16, 4, 0.008, 3, false, 332), k.ground(hexRGB("#5f8f46"), 1), k.ground(hexRGB("#4a7038"), 1))
	case sceneSpace:
		ac.noWeather = true
		if k.ac.dark {
			k.ownSky(hexRGB("#04050d"), hexRGB("#151a3c"), 140)
		} else {
			k.ownSky(hexRGB("#bdb6e6"), hexRGB("#f4dbe7"), 0)
		}
		for y := 0; y < picH; y++ { // a nebula
			for x := 0; x < picW; x++ {
				n := fbm(k.ns+340, float64(x)*0.006+float64(y)*0.01, 4)
				if n > 0.55 {
					c := hexRGB("#8a4fd0").mix(hexRGB("#2fb4c8"), valueNoise(k.ns+341, float64(x)*0.004, 0))
					cv.set(x, y, c, (n-0.55)*1.2)
				}
			}
		}
		px, py, r := rng.in(0.55, 0.68)*picW, 26.0, 15.0
		bands := []rgb{hexRGB("#e8c7a0"), hexRGB("#c99a6a"), hexRGB("#f0dcc0"), hexRGB("#b07a50")}
		ring := hexRGB("#d9c9a8")
		ringAt := func(front bool) {
			for a := 0.0; a < 6.28; a += 0.01 {
				if (math.Sin(a) > 0) != front {
					continue
				}
				for w := 0.0; w < 3; w++ {
					cv.set(int(px+math.Cos(a)*(r*1.8+w)), int(py+math.Sin(a)*(r*0.45+w*0.25)), ring, 0.8-w*0.2)
				}
			}
		}
		ringAt(false)
		for y := int(py - r); y <= int(py+r); y++ {
			for x := int(px - r); x <= int(px+r); x++ {
				d := math.Hypot(float64(x)+0.5-px, float64(y)+0.5-py)
				if d > r+0.5 {
					continue
				}
				b := bands[int(math.Abs(float64(y)-py+3*valueNoise(k.ns+342, float64(x)*0.2, 0))/3)%len(bands)]
				shade := clamp01((float64(x) - px + r*0.4) / (2 * r))
				cv.set(x, y, b.mix(rgb{20, 16, 30}, shade*0.6), clamp01(r-d+0.5))
			}
		}
		ringAt(true)
		mx := rng.in(0.3, 0.42) * picW
		cv.disc(mx, 18, 4, hexRGB("#cfcac2"), 1)
		cv.disc(mx+1, 18, 3.5, hexRGB("#2a2a3a"), 0.35)
		surface := k.ridge(68, 5, 0.01, 3, false, 343) // a moon's ground, cratered
		grey := hexRGB("#615f6c")
		if !k.ac.dark {
			grey = hexRGB("#d8d4dc")
		}
		cv.land(surface, grey, grey.mix(rgb{60, 60, 70}, 0.3))
		for i := 0; i < 16; i++ {
			cx, cy := rng.f()*picW, rng.in(72, 90)
			rr := rng.in(2, 6)
			for y := int(cy - rr*0.4); y <= int(cy+rr*0.4); y++ {
				for x := int(cx - rr); x <= int(cx+rr); x++ {
					if math.Hypot((float64(x)-cx)/rr, (float64(y)-cy)/(rr*0.4)) < 1 {
						cv.set(x, y, grey.mix(rgb{40, 40, 50}, 0.3), 1)
					}
				}
			}
		}
		lf.comet = true
	case sceneMoonSea:
		*k.water = picHz + 2
		mr := 26.0
		my := picHz - mr*0.7
		if k.l.night {
			cv.moon(k.sx, my, mr, 0.5, k.l.sun, k.l.mid)
		} else { // the moon by day: pale, and its seas
			cv.disc(k.sx, my, mr, k.l.mid.mix(rgb{255, 255, 255}, 0.55), 0.8)
			for i := 0; i < 6; i++ {
				cv.disc(k.sx+rng.in(-mr, mr)*0.6, my+rng.in(-mr, mr)*0.6, rng.in(3, 7), k.l.mid, 0.25)
			}
		}
		k.sea()
		lf.dolphin = [2]float64{k.sx + rng.in(30, 80), *k.water}
	}
}

// paintScene is what moves in these scenes at time t (seconds), on a tile.
func (ac *autoCover) paintScene(pt *ui.Painter, t float64, at func(px, py float64) (float32, float32), flip bool) {
	const s = autoPx
	lf := &ac.life
	dot := func(x, y float64, c rgb, a float64) {
		X, Y := at(x, y)
		if flip {
			X -= s
		}
		pt.Fill(ui.Rect{X: X, Y: Y, W: s, H: s}, c.ui(a), 0)
	}
	white := rgb{255, 255, 255}
	night := ac.dark

	if lf.aurora { // the curtain of light waves over the sky
		for i := 0; i < 70; i++ {
			u := float64(i) / 70
			x := u*picW + math.Sin(t*0.3+u*9)*12
			top := 4 + 10*math.Sin(t*0.25+u*7) + 6*math.Sin(u*23)
			bot := picHz - 6 + 4*math.Sin(t*0.4+u*5)
			a := 0.25 + 0.2*math.Sin(t*0.5+u*13)
			X, Y := at(x, top)
			_, Y2 := at(x, bot)
			if flip {
				X -= 10 * s
			}
			pt.FillGradient(ui.Rect{X: X, Y: Y, W: 10 * s, H: Y2 - Y}, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
				{Color: rgb{170, 120, 255}.ui(0), At: 0},
				{Color: rgb{170, 120, 255}.ui(a * 0.5), At: 0.35},
				{Color: rgb{60, 240, 170}.ui(a), At: 0.8},
				{Color: rgb{60, 240, 170}.ui(0), At: 1},
			}}, 0)
		}
	}
	if lf.comet { // a comet crosses, slowly; a satellite blinks along
		k := math.Mod(t, 60) / 40
		if k < 1 {
			hx, hy := k*(picW+200)-100, 8+k*14
			for j := 0.0; j < 30; j++ {
				dot(hx-j*1.5, hy-j*0.35+math.Sin(j*0.7)*0.4, rgb{220, 240, 255}, (1-j/30)*0.8)
			}
			dot(hx, hy, white, 1)
		}
		sx := math.Mod(t*6, picW)
		if math.Mod(t, 1.2) < 0.6 {
			dot(sx, 12+sx*0.01, white, 0.9)
		}
	}
	for _, b := range lf.balloons { // balloons rise and sink, and drift
		colors := [][2]rgb{{{230, 70, 60}, {250, 200, 70}}, {{70, 130, 220}, {240, 240, 240}}, {{240, 150, 50}, {250, 90, 120}}, {{120, 80, 200}, {80, 200, 180}}, {{60, 170, 90}, {250, 220, 90}}, {{240, 100, 160}, {255, 240, 200}}}
		c := colors[int(b[2])%len(colors)]
		x := b[0] + math.Sin(t*0.05+b[2])*20
		y := b[1] + math.Sin(t*0.25+b[0])*5
		const r = 7.0
		for dy := -r; dy <= r+2; dy++ {
			w := math.Sqrt(math.Max(0, r*r-dy*dy))
			if dy > 0 {
				w = math.Max(0, r-dy*0.9) // narrowing to the basket
			}
			for dx := -w; dx <= w; dx++ {
				band := int(math.Floor((dx+r)/(r*2/5))) % 2
				col := c[band]
				if dx < -w*0.3 {
					col = col.mix(white, 0.2)
				}
				dot(x+dx, y+dy, col, 1)
			}
		}
		dot(x-1, y+r+3, rgb{60, 40, 30}, 0.7)
		dot(x+1, y+r+3, rgb{60, 40, 30}, 0.7)
		dot(x, y+r+5, rgb{120, 80, 50}, 1)
		dot(x-1, y+r+5, rgb{120, 80, 50}, 1)
	}
	if lf.river != nil { // a boat goes down the river; the water glints as it flows
		x := math.Mod(t*4, picW+20) - 10
		if x >= 0 && int(x) < picW {
			y := lf.river[int(x)]
			for d := 1.0; d < 8; d++ {
				dot(x-2-d, y-d*0.4, white, 0.3*(1-d/8))
				dot(x-2-d, y+d*0.4, white, 0.3*(1-d/8))
			}
			for dx := -2.0; dx <= 2; dx++ {
				dot(x+dx, y, rgb{150, 90, 50}, 1)
			}
			dot(x, y-1, rgb{240, 236, 220}, 1)
		}
		r := splitmix(ac.seed ^ 0x7a)
		for i := 0; i < 40; i++ {
			fx := math.Mod(r.f()*picW+t*6, picW)
			off := r.in(-0.6, 0.6) * lf.riverW
			if math.Sin(t*2+float64(i)) > 0.3 {
				dot(fx, lf.river[int(fx)]+off, white, 0.5)
			}
		}
	}
	if lf.tractor { // a tractor along the path, slowly
		x := math.Mod(t*5, picW+30) - 15
		y := lf.path + 2*math.Sin(x*0.004)
		body := rgb{200, 60, 50}
		if ac.season == 3 {
			body = rgb{60, 120, 200}
		}
		for dx := -2.0; dx <= 2; dx++ {
			for dy := -1.0; dy <= 1; dy++ {
				dot(x+dx, y+dy, body, 1)
			}
		}
		dot(x+1, y, rgb{240, 240, 230}, 1)
		for _, w := range [][2]float64{{-2, -2}, {-2, 2}, {2, -2}, {2, 2}} {
			dot(x+w[0], y+w[1], rgb{30, 30, 30}, 1)
		}
		if !night {
			for d := 0.0; d < 4; d++ {
				dot(x-4-d*2, y+math.Sin(t*3+d), rgb{190, 160, 120}, 0.3*(1-d/4))
			}
		}
	}
	if lf.pond[2] > 0 { // koi round the pond; a ring now and then
		p := lf.pond
		kc := []rgb{{240, 120, 40}, {245, 245, 240}, {230, 170, 40}, {220, 70, 50}, {250, 250, 250}}
		for i := 0; i < 5; i++ {
			a := t*(0.25+0.05*float64(i)) + float64(i)*1.3
			if i%2 == 1 {
				a = -a
			}
			rr := 0.4 + 0.12*float64(i%3)
			for j := 0.0; j < 4; j++ {
				aa := a - j*0.06*math.Copysign(1, a)
				c := kc[i]
				if j == 1 && i%2 == 1 {
					c = rgb{230, 80, 40}
				}
				dot(p[0]+math.Cos(aa)*p[2]*rr, p[1]+math.Sin(aa)*p[3]*rr, c, 1-j*0.15)
			}
		}
		k := math.Mod(t, 6) / 3
		if k < 1 {
			r := splitmix(ac.seed ^ uint64(t/6))
			cx, cy := p[0]+r.in(-0.5, 0.5)*p[2], p[1]+r.in(-0.4, 0.4)*p[3]
			for a := 0.0; a < 6.28; a += 0.4 {
				dot(cx+math.Cos(a)*k*6, cy+math.Sin(a)*k*3, white, 0.5*(1-k))
			}
		}
	}
	for i, sk := range lf.skaters { // skaters go round
		a := t*0.5*(1+0.2*float64(i)) + float64(i)
		x, y := sk[0]+math.Cos(a)*sk[2]*1.6, sk[1]+math.Sin(a)*sk[2]*0.6
		scarf := []rgb{{220, 60, 60}, {60, 120, 220}, {240, 180, 40}, {80, 180, 100}}[i%4]
		dot(x, y, rgb{40, 40, 50}, 1)
		dot(x, y-1, scarf, 1)
		dot(x+0.5, y+1, rgb{140, 150, 170}, 0.5)
	}
	for i, b := range lf.moored { // boats by the piers rock on the water
		hulls := []rgb{{244, 241, 234}, {232, 238, 242}, {217, 83, 74}, {63, 111, 176}, {242, 201, 76}}
		X, Y := at(b[0], b[1])
		dy := f32(math.Sin(t*1.1+float64(i)*1.7) * 1.2)
		w, h := float32(4*s), float32(11*s)
		if flip {
			X -= w
		}
		pt.Fill(ui.Rect{X: X - w/2 + s, Y: Y + dy + s, W: w, H: h}, rgb{0, 0, 0}.ui(0.18), w/2)
		pt.Fill(ui.Rect{X: X - w/2, Y: Y + dy, W: w, H: h}, hulls[int(b[2])].ui(1), w/2)
		pt.Fill(ui.Rect{X: X - w/2 + s/2, Y: Y + dy + h*0.3, W: w - s, H: h * 0.35}, rgb{150, 110, 70}.ui(1), 1)
	}
	if lf.fire != [2]float64{} { // the fire flickers, and lights the clearing at night
		fx, fy := lf.fire[0], lf.fire[1]
		fl := 0.75 + 0.25*math.Sin(t*9)*math.Sin(t*5.3)
		X, Y := at(fx, fy)
		g := float32(14 * s)
		if night {
			g = 30 * s
		}
		pt.Fill(ui.Rect{X: X - g, Y: Y - g*0.6, W: 2 * g, H: 1.2 * g}, rgb{255, 150, 60}.ui(0.18*fl), g)
		r := splitmix(ac.seed ^ uint64(t*8))
		for i := 0; i < 9; i++ {
			c := rgb{255, 200, 80}.mix(rgb{240, 70, 30}, r.f())
			dot(fx+r.in(-2, 2), fy+r.in(-2, 1.5), c, 1)
		}
		dot(fx, fy, rgb{255, 245, 200}, 1)
		if !night {
			for i := 0; i < 5; i++ {
				k := math.Mod(t/4+float64(i)/5, 1)
				dot(fx+k*10+math.Sin(k*8)*2, fy-k*12, rgb{200, 200, 200}, 0.4*(1-k))
			}
		}
	}
	if lf.trainTop || lf.deck > 0 { // a train comes through, now and then
		const every, cross = 24.0, 10.0
		k := math.Mod(t+float64(ac.seed%17), every) / cross
		if k < 1 {
			head := k*(picW+260) - 130
			cars := []rgb{{180, 50, 45}, {60, 90, 140}, {60, 90, 140}, {60, 90, 140}, {200, 160, 70}, {60, 90, 140}}
			if lf.trainTop {
				y := lf.rail
				for ci, c := range cars {
					x0 := head - float64(ci)*13
					for dx := 0.0; dx < 12; dx++ {
						for dy := -2.0; dy <= 2; dy++ {
							cc := c
							if dy == -2 || dy == 2 {
								cc = c.mix(rgb{0, 0, 0}, 0.3)
							}
							dot(x0-dx, y+dy, cc, 1)
						}
					}
				}
				for i := 0; i < 6; i++ { // its smoke, left behind
					d := float64(i)
					dot(head-3-d*4, lf.rail-3-d*0.8+math.Sin(t*3+d), rgb{210, 210, 210}, 0.5*(1-d/6))
				}
			} else {
				y := lf.deck
				for ci, c := range cars {
					x0 := head - float64(ci)*13
					for dx := 0.0; dx < 12; dx++ {
						for dy := 1.0; dy <= 5; dy++ {
							cc := c
							if ci > 0 && dy == 2 && int(dx)%3 == 1 {
								cc = rgb{255, 220, 140} // windows
							}
							dot(x0-dx, y-dy, cc, 1)
						}
					}
				}
				dot(head-3, y-6, rgb{40, 40, 40}, 1)
				dot(head-3, y-7, rgb{40, 40, 40}, 1)
				for i := 0; i < 8; i++ { // steam, rising behind
					d := float64(i)
					X, Y := at(head-3-d*3.5, y-9-d*1.6)
					r := f32((1.5 + d*0.4) * s)
					pt.Fill(ui.Rect{X: X - r, Y: Y - r, W: 2 * r, H: 2 * r}, rgb{235, 235, 235}.ui(0.55*(1-d/8)), r)
				}
			}
		}
	}
	if lf.surface > 0 { // under the sea: the surface heaves, the kelp sways, a boat rocks above
		for x := 0.0; x < picW; x += 2 {
			y := lf.surface + math.Sin(x*0.05+t*1.4)*1.2
			dot(x, y, white, 0.6)
			dot(x+1, y, white, 0.6)
		}
		for _, kp := range lf.kelp {
			for j := 0.0; j < kp[2]; j++ {
				x := kp[0] + math.Sin(t*0.9+j*0.15+kp[0])*j*0.12
				c := rgb{60, 140, 70}.mix(rgb{120, 190, 90}, j/kp[2])
				if ac.dark {
					c = c.mul(rgb{120, 135, 175})
				}
				dot(x, kp[1]-j, c, 1)
				if int(j)%5 == 2 {
					dot(x+1, kp[1]-j, c, 1)
				}
			}
		}
		bx, by := lf.surfaceAt+math.Sin(t*0.1)*30, lf.surface+math.Sin(t*1.4+lf.surfaceAt*0.05)*1.2
		for dx := -6.0; dx <= 6; dx++ {
			dot(bx+dx, by-1, rgb{180, 70, 50}, 1)
			if math.Abs(dx) < 5 {
				dot(bx+dx, by, rgb{120, 60, 40}, 0.6) // its hull, under the water
			}
		}
		for y := 2.0; y < 9; y++ {
			dot(bx, by-1-y, rgb{80, 60, 50}, 1)
		}
		for y := 2.0; y < 8; y++ {
			for dx := 1.0; dx < 8-y; dx++ {
				dot(bx+dx, by-1-y, rgb{245, 240, 225}, 1)
			}
		}
	}
	for _, b := range lf.bubbles { // bubbles rise
		for i := 0; i < 3; i++ {
			k := math.Mod(t*0.4+float64(i)/3+b[0]*0.01, 1)
			top := lf.surface
			if top == 0 {
				top = 2
			}
			y := b[1] - k*(b[1]-top)
			dot(b[0]+math.Sin(k*12)*1.2, y, white, 0.6*(1-k*0.5))
		}
	}
	if lf.fish > 0 { // fish, bright, along the reef or under the boat
		r := splitmix(ac.seed ^ 0xf00)
		cs := []rgb{{250, 210, 60}, {60, 150, 240}, {250, 130, 60}, {240, 240, 240}, {230, 80, 140}}
		for i := 0; i < lf.fish; i++ {
			fx, fy, v := r.f()*picW, r.in(lf.fishTop, 62), r.in(2, 6)
			dir := 1.0
			if i%3 == 0 {
				dir = -1
			}
			x := math.Mod(fx+t*v*dir+picW*10, picW)
			y := fy + math.Sin(t*0.8+float64(i))*2
			c := cs[i%len(cs)]
			if ac.dark {
				c = c.mul(rgb{140, 150, 190})
			}
			dot(x, y, c, 1)
			dot(x-dir, y, c, 1)
			wag := 0.0
			if math.Sin(t*6+float64(i)) > 0 {
				wag = 1
			}
			dot(x-2*dir, y-0.5+wag*0.6, c.mix(rgb{0, 0, 0}, 0.2), 1)
		}
	}
	if lf.rain { // rain, slanting; in the street, people under umbrellas
		r := splitmix(ac.seed ^ 0x8a1)
		for i := 0; i < 140; i++ {
			fx, sp := r.f()*picW, r.in(40, 60)
			y := math.Mod(r.f()*picH+t*sp, picH+10) - 5
			x := fx - y*0.25
			for j := 0.0; j < 3; j++ {
				dot(x+j*0.25, y+j, rgb{210, 220, 235}, 0.5)
			}
		}
		if lf.street > 0 {
			ucol := []rgb{{220, 60, 60}, {40, 40, 50}, {60, 120, 220}, {240, 190, 50}, {80, 170, 110}, {230, 110, 170}, {40, 40, 50}}
			r := splitmix(ac.seed ^ 0x0b)
			for i, c := range ucol {
				v := r.in(2, 4)
				if i%2 == 1 {
					v = -v
				}
				x := math.Mod(r.f()*picW+t*v+picW*10, picW)
				y := lf.street - 1
				step := math.Sin(t*5+float64(i)) > 0
				dot(x, y, rgb{30, 30, 40}, 1) // legs
				if step {
					dot(x+1, y, rgb{30, 30, 40}, 1)
				}
				dot(x, y-1, rgb{40, 40, 60}, 1)
				dot(x, y-2, rgb{40, 40, 60}, 1)
				for dx := -2.0; dx <= 2; dx++ { // the umbrella
					dot(x+dx, y-4, c, 1)
				}
				for dx := -1.0; dx <= 1; dx++ {
					dot(x+dx, y-5, c, 1)
				}
			}
		}
	}
	if lf.storm { // the waves roll; lightning, now and then, lights it all
		for x := 0.0; x < picW; x += 1 {
			y := picHz + 4 + 2*math.Sin(x*0.04-t*1.2) + 1.5*math.Sin(x*0.11+t*0.7)
			if math.Sin(x*0.04-t*1.2) > 0.4 {
				dot(x, y, white, 0.55)
			}
			y2 := picHz + 14 + 3*math.Sin(x*0.03-t*0.9+2)
			if math.Sin(x*0.03-t*0.9+2) > 0.5 {
				dot(x, y2, white, 0.45)
			}
		}
		const every = 7.0
		k := math.Mod(t, every)
		if k < 0.35 {
			r := splitmix(ac.seed ^ uint64(t/every))
			x, y := r.in(0.25, 0.75)*picW, 0.0
			path := &ui.Path{}
			X, Y := at(x, y)
			path.MoveTo(X, Y)
			for y < picHz {
				x += r.in(-5, 5)
				y += r.in(3, 7)
				X, Y = at(x, y)
				path.LineTo(X, Y)
			}
			a := 1 - k/0.35
			pt.StrokePath(path, 2*s, rgb{240, 245, 255}.ui(a))
			x0, y0 := at(0, 0)
			x1, y1 := at(picW, picH)
			pt.Fill(ui.Rect{X: min(x0, x1), Y: y0, W: float32(math.Abs(float64(x1 - x0))), H: y1 - y0}, white.ui(0.18*a), 0)
		}
	}
	if lf.dolphin != [2]float64{} { // a dolphin leaps now and then
		const every, jump = 13.0, 1.8
		k := math.Mod(t, every) / jump
		if k < 1 {
			x0, wy := lf.dolphin[0], lf.dolphin[1]
			x := x0 + k*24
			y := wy - math.Sin(k*math.Pi)*10
			ang := math.Atan2(-math.Cos(k*math.Pi)*10*math.Pi, 24)
			body := rgb{90, 110, 140}
			if ac.dark {
				body = rgb{40, 50, 75}
			}
			for j := -3.0; j <= 3; j++ {
				dot(x+math.Cos(ang)*j, y+math.Sin(ang)*j, body, 1)
			}
			dot(x-math.Sin(ang)*1, y+math.Cos(ang)*-1, body, 1) // its fin
			for _, sx := range []float64{x0, x0 + 24} {
				if math.Abs(x-sx) < 4 {
					for i := 0.0; i < 5; i++ {
						dot(sx+i-2, wy-1-math.Abs(i-2)*0.6, white, 0.8)
					}
				}
			}
		}
	}
	if ac.leaves || ac.petals { // leaves fall, or petals drift
		r := splitmix(ac.seed ^ 0x1eaf)
		cs := []rgb{{214, 120, 40}, {190, 60, 45}, {230, 180, 50}, {160, 82, 45}}
		if ac.petals {
			cs = []rgb{{248, 190, 205}, {252, 215, 225}, {240, 170, 190}}
		}
		for i := 0; i < 50; i++ {
			fx, sp, ph := r.f()*picW, r.in(3, 7), r.f()*6.28
			y := math.Mod(r.f()*picH+t*sp, picH+6) - 3
			x := fx + math.Sin(t*0.7+ph)*6 + t*2
			x = math.Mod(x, picW)
			c := cs[i%len(cs)]
			if ac.dark {
				c = c.mul(rgb{150, 160, 200})
			}
			dot(x, y, c, 0.9)
			if math.Sin(t*2+ph) > 0 {
				dot(x+1, y, c, 0.9)
			} else {
				dot(x, y+1, c, 0.9)
			}
		}
	}
}
