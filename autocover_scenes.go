package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// More landscapes for the automatic covers (autocover.go), and what moves in them.
//
// Seen from above: a river under a stone bridge. From the side: space, a huge moon over the sea.
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
	river   []float64 // the river's middle, a column's
	riverW  float64
	comet   bool
	dolphin [2]float64
}

// kit is what a scene draws with: the canvas, the seed's numbers, the hour's light, and the
// ways of drawImage's own scenes.
type kit struct {
	cv    *canvas
	rng   *splitmix
	l     *coverLight
	ac    *autoCover
	ns    uint64
	sx    float64
	ridge func(base, amp, freq float64, oct int, sharp bool, k uint64) func(float64) float64
	sea   func()
	water *float64
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

// paintScene is what moves in these scenes at time t (seconds), in the cover's canvas.
func (ac *autoCover) paintScene(pt *ui.Painter, t float64, at func(px, py float64) (float32, float32)) {
	const s = autoPx
	lf := &ac.life
	dot := func(x, y float64, c rgb, a float64) {
		X, Y := at(x, y)
		pt.Fill(ui.Rect{X: X, Y: Y, W: s, H: s}, c.ui(a), 0)
	}
	white := rgb{255, 255, 255}

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
