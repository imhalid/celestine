package main

import (
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The graph of the library: its pages as points, a line from a page to each
// page it links to with [[…]]. The points settle by themselves, as springs
// would - the linked draw together, all push apart - and then stay; the wheel
// zooms, a drag of the background moves the picture, a drag of a point moves
// it, a click opens its page. A page's color is its collection's; the pages
// that link to nothing and are linked to by nothing are left out unless asked
// for.

var iconGraph = icon(`<circle cx="6" cy="7" r="2.2"/><circle cx="18" cy="6" r="2.2"/><circle cx="12" cy="18" r="2.2"/><path d="M7.9 8.2 10.5 16M16.2 7.8 13.3 16M8.2 6.9 15.8 6.2"/>`)

type gnode struct {
	page   *Page
	x, y   float64 // where it is in the plane, in units
	vx, vy float64
	deg    int
	pinned bool
}

type graphState struct {
	nodes    []gnode
	edges    [][2]int
	builtAt  time.Time // the index it was made from
	showAll  bool      // the pages with no link too
	iter     int       // the steps of the layout done
	panX     float64
	panY     float64
	zoom     float64
	hover    int // the point under the pointer, or -1
	drag     int // the point being dragged, or -1
	panning  bool
	moved    bool
	downX    float32
	downY    float32
	w, h     float32 // the canvas, last frame
	r        ui.Rect // and where it is in the window
	query    string
	fit      bool // the picture is fitted to the canvas, until a hand moves it
	startedZ bool
}

// graphMax is how many points the graph takes: more are the most linked.
const graphMax = 700

// openGraph shows the graph.
func (a *app) openGraph() {
	a.flush()
	a.graphOpen = true
	a.tag, a.coll, a.trashOpen, a.tasksOpen, a.searchOpen = "", nil, false, false, false
	a.linksStale()
	a.graph.builtAt = time.Time{} // made again from the index
	a.visit()
}

// buildGraph makes the points and lines from the links index.
func (a *app) buildGraph() {
	g := &a.graph
	a.linksTo(nil, a.clock(), a.links.stale)
	if g.builtAt.Equal(a.links.built) && g.nodes != nil {
		return
	}
	g.builtAt = a.links.built
	byTitle := map[string]*Page{}
	var pages []*Page
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			pages = append(pages, p)
			if k := strings.ToLower(strings.TrimSpace(p.Title)); byTitle[k] == nil {
				byTitle[k] = p
			}
		}
	}
	type pair struct{ from, to *Page }
	seen := map[pair]bool{}
	var links []pair
	for key, from := range a.links.into {
		to := byTitle[key]
		if to == nil {
			continue
		}
		for _, src := range from {
			if src != to && !seen[pair{src, to}] {
				seen[pair{src, to}] = true
				links = append(links, pair{src, to})
			}
		}
	}
	deg := map[*Page]int{}
	for _, l := range links {
		deg[l.from]++
		deg[l.to]++
	}
	var keep []*Page
	for _, p := range pages {
		if deg[p] > 0 || g.showAll {
			keep = append(keep, p)
		}
	}
	sort.SliceStable(keep, func(i, j int) bool { return deg[keep[i]] > deg[keep[j]] })
	if len(keep) > graphMax {
		keep = keep[:graphMax]
	}
	old := map[*Page]gnode{}
	for _, n := range g.nodes {
		old[n.page] = n
	}
	idx := map[*Page]int{}
	g.nodes = g.nodes[:0:0]
	for i, p := range keep {
		idx[p] = i
		n := gnode{page: p, deg: deg[p]}
		if o, ok := old[p]; ok {
			n.x, n.y, n.pinned = o.x, o.y, o.pinned
		} else {
			h := fnv.New64a()
			h.Write([]byte(p.Dir))
			s := h.Sum64()
			ang := float64(s%3600) / 3600 * 2 * math.Pi
			rad := 120 + float64((s/3600)%1000)/1000*220
			n.x, n.y = math.Cos(ang)*rad, math.Sin(ang)*rad
		}
		g.nodes = append(g.nodes, n)
	}
	g.edges = g.edges[:0:0]
	for _, l := range links {
		i, ok1 := idx[l.from]
		j, ok2 := idx[l.to]
		if ok1 && ok2 {
			g.edges = append(g.edges, [2]int{i, j})
		}
	}
	g.iter, g.fit = 0, true
	if g.zoom == 0 {
		g.zoom = 1
	}
	g.hover, g.drag = -1, -1
}

// relax moves the points a few steps toward rest: all repel, the linked draw
// together, and the whole is held toward the middle. It reports whether it
// moved anything still.
func (g *graphState) relax(steps int) bool {
	n := len(g.nodes)
	if n == 0 || g.iter >= 320 {
		return false
	}
	k := 60.0 // how far apart they like to be
	for s := 0; s < steps && g.iter < 320; s++ {
		temp := 14 * (1 - float64(g.iter)/320)
		for i := range g.nodes {
			g.nodes[i].vx, g.nodes[i].vy = 0, 0
		}
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx, dy := g.nodes[i].x-g.nodes[j].x, g.nodes[i].y-g.nodes[j].y
				d2 := dx*dx + dy*dy
				if d2 < 0.01 {
					dx, dy, d2 = 0.1, 0.1, 0.02
				}
				d := math.Sqrt(d2)
				f := k * k / d2 // repulsion k²/d, along the unit vector
				g.nodes[i].vx += dx * f
				g.nodes[i].vy += dy * f
				g.nodes[j].vx -= dx * f
				g.nodes[j].vy -= dy * f
				_ = d
			}
		}
		for _, e := range g.edges {
			a, b := &g.nodes[e[0]], &g.nodes[e[1]]
			dx, dy := a.x-b.x, a.y-b.y
			d := math.Sqrt(dx*dx+dy*dy) + 0.01
			f := d / k // attraction d²/k, along the unit vector
			a.vx -= dx * f
			a.vy -= dy * f
			b.vx += dx * f
			b.vy += dy * f
		}
		for i := range g.nodes {
			nd := &g.nodes[i]
			nd.vx -= nd.x * 0.02 // the middle holds them
			nd.vy -= nd.y * 0.02
			if nd.pinned {
				continue
			}
			l := math.Hypot(nd.vx, nd.vy)
			if l > temp {
				nd.vx, nd.vy = nd.vx/l*temp, nd.vy/l*temp
			}
			nd.x += nd.vx
			nd.y += nd.vy
		}
		g.iter++
	}
	return g.iter < 320
}

func (g *graphState) radius(n gnode) float64 { return 4 + math.Sqrt(float64(n.deg))*2.4 }

// toScreen is where a point of the plane is in the canvas.
func (g *graphState) toScreen(x, y float64) (float32, float32) {
	return g.w/2 + float32((x+g.panX)*g.zoom), g.h/2 + float32((y+g.panY)*g.zoom)
}

// toPlane is the point of the plane at a place of the canvas.
func (g *graphState) toPlane(sx, sy float32) (float64, float64) {
	return float64(sx-g.w/2)/g.zoom - g.panX, float64(sy-g.h/2)/g.zoom - g.panY
}

// pointAt is the point under (sx, sy) of the canvas, or -1.
func (g *graphState) pointAt(sx, sy float32) int {
	best, bd := -1, math.MaxFloat64
	for i, n := range g.nodes {
		x, y := g.toScreen(n.x, n.y)
		d := math.Hypot(float64(sx-x), float64(sy-y))
		if d <= g.radius(n)*g.zoom+4 && d < bd {
			best, bd = i, d
		}
	}
	return best
}

// fitToCanvas puts the points in the middle of the canvas, as large as it
// lets them be.
func (g *graphState) fitToCanvas() {
	if len(g.nodes) == 0 || g.w < 50 || g.h < 50 {
		return
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, n := range g.nodes {
		minX, maxX = math.Min(minX, n.x), math.Max(maxX, n.x)
		minY, maxY = math.Min(minY, n.y), math.Max(maxY, n.y)
	}
	bw, bh := math.Max(maxX-minX, 1)+120, math.Max(maxY-minY, 1)+120 // room for the labels
	g.zoom = math.Min(math.Min(float64(g.w)/bw, float64(g.h)/bh), 2.2)
	g.panX, g.panY = -(minX+maxX)/2, -(minY+maxY)/2
}

func (g *graphState) zoomAt(sx, sy float32, factor float64) {
	z := math.Min(math.Max(g.zoom*factor, 0.15), 6)
	// the point under the pointer stays under it
	px, py := g.toPlane(sx, sy)
	g.zoom = z
	nx, ny := g.toPlane(sx, sy)
	g.panX += nx - px
	g.panY += ny - py
}

// input takes the canvas's pointer: the drags, the wheel, a pinch.
func (a *app) graphInput(ev ui.InputEvent) bool {
	g := &a.graph
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		g.downX, g.downY, g.moved = ev.X, ev.Y, false
		g.fit = false
		if i := g.pointAt(ev.X, ev.Y); i >= 0 {
			g.drag = i
		} else {
			g.panning = true
		}
		return true
	case ui.InputPointerMove:
		if g.drag >= 0 {
			if math.Hypot(float64(ev.X-g.downX), float64(ev.Y-g.downY)) > 3 {
				g.moved = true
			}
			if g.moved {
				x, y := g.toPlane(ev.X, ev.Y)
				n := &g.nodes[g.drag]
				n.x, n.y, n.pinned = x, y, true
				g.iter = min(g.iter, 250) // the others settle round it
			}
			return true
		}
		if g.panning {
			if math.Hypot(float64(ev.X-g.downX), float64(ev.Y-g.downY)) > 3 {
				g.moved = true
			}
			g.panX += float64(ev.X-g.downX) / g.zoom
			g.panY += float64(ev.Y-g.downY) / g.zoom
			g.downX, g.downY = ev.X, ev.Y
			return true
		}
		if h := g.pointAt(ev.X, ev.Y); h != g.hover {
			g.hover = h
			return true
		}
		return false
	case ui.InputPointerUp:
		d, was := g.drag, g.moved
		g.drag, g.panning, g.moved = -1, false, false
		if d >= 0 && !was && d < len(g.nodes) {
			p := g.nodes[d].page
			a.later(func() { a.open(p) })
		}
		return true
	case ui.InputScroll:
		g.fit = false
		switch {
		case ev.Precise && ev.Mods&ui.Ctrl == 0:
			g.panX -= float64(ev.DX) / g.zoom
			g.panY -= float64(ev.DY) / g.zoom
		default:
			g.zoomAt(ev.X, ev.Y, math.Exp(-float64(ev.DY)*0.0025))
		}
		return true
	case ui.InputZoom:
		g.fit = false
		g.zoomAt(ev.X, ev.Y, float64(ev.Scale))
		return true
	}
	return false
}

// graphView is the graph and what stands over it.
func (a *app) graphView(c *ui.Context) {
	g := &a.graph
	t := c.Theme()
	cl := ryugo.From(c).Colors
	a.buildGraph()
	moving := g.relax(6)
	if g.fit {
		g.fitToCanvas()
	}
	ui.Column(c).Fill().MaxWidth(4000).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Bağlantı grafiği").FontSize(32).FontWeight(720).Padding(2, 0, 2, 0).Grow(1).NoWrap()
			ryugo.Input(c.Key("graph-q"), &g.query, ryugo.InputOptions{Label: "Grafikte ara", Placeholder: "Sayfa ara…", PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}).Width(200)
			on := g.showAll
			ryugo.Switch(c.Key("graph-all"), &on, ryugo.SwitchOptions{Label: "Bağlantısız sayfalar"})
			if on != g.showAll {
				g.showAll, g.builtAt = on, time.Time{}
			}
			if ryugo.Button(c.Key("graph-reset"), "Yeniden düzenle", ryugo.ButtonOptions{Icon: iconRestore, Size: ryugo.Sm}).Clicked() {
				for i := range g.nodes {
					g.nodes[i].pinned = false
				}
				g.iter, g.fit = 0, true
			}
		})
		ui.Text(c, strconv.Itoa(len(g.nodes))+" sayfa, "+strconv.Itoa(len(g.edges))+" bağlantı  ·  tekerlek yakınlaştırır, boşluğu sürüklemek kaydırır, bir noktaya tıklamak sayfayı açar").
			FontSize(13).TextColor(t.TextMuted)
		canvas := ui.Box(c.Key("graph-canvas")).Grow(1).Radius(14).Background(cl.Well).Border(1, cl.WellBorder).Clip().Label("Bağlantı grafiği")
		canvas.HandleInput(a.graphInput)
		switch {
		case g.drag >= 0:
			canvas.Cursor(ui.CursorGrabbing)
		case g.hover >= 0:
			canvas.Cursor(ui.CursorPointer)
		case g.panning:
			canvas.Cursor(ui.CursorGrabbing)
		default:
			canvas.Cursor(ui.CursorGrab)
		}
		b := canvas.LayoutBounds()
		g.w, g.h, g.r = b.W, b.H, b
		canvas.Draw(func(p *ui.Painter, r ui.Rect) { a.drawGraph(p, r, cl) })
		if len(g.nodes) == 0 {
			canvas.Children(func() {
				ui.Text(c, "Henüz bağlantı yok. Bir sayfada [[ yazıp başka bir sayfaya bağlan.").TextColor(t.TextMuted).Padding(24)
			})
		}
	})
	if moving || g.drag >= 0 || g.panning {
		c.Invalidate()
	}
}

// drawGraph draws the lines and the points.
func (a *app) drawGraph(p *ui.Painter, r ui.Rect, cl ryugo.Colors) {
	g := &a.graph
	g.w, g.h = r.W, r.H
	q := fold(strings.TrimSpace(g.query))
	near := map[int]bool{}
	if g.hover >= 0 {
		near[g.hover] = true
		for _, e := range g.edges {
			if e[0] == g.hover {
				near[e[1]] = true
			}
			if e[1] == g.hover {
				near[e[0]] = true
			}
		}
	}
	at := func(i int) (float32, float32) {
		x, y := g.toScreen(g.nodes[i].x, g.nodes[i].y)
		return r.X + x, r.Y + y
	}
	for _, e := range g.edges {
		x0, y0 := at(e[0])
		x1, y1 := at(e[1])
		col, w := cl.Muted.Alpha(0.30), float32(1)
		if g.hover >= 0 {
			if e[0] == g.hover || e[1] == g.hover {
				col, w = cl.Accent.Alpha(0.85), 1.6
			} else {
				col = cl.Muted.Alpha(0.1)
			}
		}
		p.StrokePolyline([][2]float32{{x0, y0}, {x1, y1}}, w, col)
	}
	type label struct {
		x, y float32
		text string
		hot  bool
	}
	var labels []label
	for i, n := range g.nodes {
		x, y := at(i)
		if x < r.X-30 || x > r.X+r.W+30 || y < r.Y-30 || y > r.Y+r.H+30 {
			continue
		}
		rad := float32(g.radius(n) * g.zoom)
		tone := columnTone2(n.page.Coll.Name, cl)
		alpha := float32(0.9)
		if g.hover >= 0 && !near[i] {
			alpha = 0.25
		}
		match := q != "" && strings.Contains(fold(n.page.Title), q)
		if q != "" && !match {
			alpha = min(alpha, 0.2)
		}
		if a.page != nil && n.page == a.page {
			p.Fill(ui.Rect{X: x - rad - 3, Y: y - rad - 3, W: 2 * (rad + 3), H: 2 * (rad + 3)}, cl.Accent.Alpha(0.35), rad+3)
		}
		if match {
			p.Fill(ui.Rect{X: x - rad - 4, Y: y - rad - 4, W: 2 * (rad + 4), H: 2 * (rad + 4)}, cl.Accent.Alpha(0.55), rad+4)
		}
		p.Fill(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, tone.Alpha(alpha), rad)
		hot := i == g.hover
		if hot || match || (g.hover < 0 && q == "" && (g.zoom >= 1.5 || n.deg >= 6 && g.zoom >= 0.7)) || (g.hover >= 0 && near[i]) {
			labels = append(labels, label{x, y + rad + 3, n.page.Title, hot})
		}
	}
	for _, l := range labels {
		col := cl.Text2
		if l.hot {
			col = cl.Text
		}
		sp := ui.Span{Text: l.text, Size: 12, Weight: map[bool]int{true: 650, false: 450}[l.hot], Color: col}
		w, _ := p.MeasureText(0, sp)
		p.RichText(l.x-w/2, l.y, w+2, sp)
	}
}

// columnTone2 is a color made of a collection's name (as columnTone's, from
// the palette's eight), for a painter that has no context.
func columnTone2(name string, cl ryugo.Colors) ui.Color {
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h = (h ^ uint32(name[i])) * 16777619
	}
	return []ui.Color{cl.C1, cl.C2, cl.C3, cl.C4, cl.C5, cl.C6, cl.C7, cl.C8}[h%8]
}
