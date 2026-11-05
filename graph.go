package main

import (
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	group  int     // its collection's place in the groups
	x, y   float64 // where it is in the plane, in units
	vx, vy float64
	deg    int
	pinned bool
	fresh  bool   // placed by the groups, not yet by the layout
	seed   uint64 // from its folder: where on its ring it starts
}

// The kinds of line: a [[link]], a page that names another in its text, a
// page and the sub-page under it.
const (
	edgeLink = iota
	edgeMention
	edgeTree
)

type gedge struct{ a, b, kind int }

// ggroup is a collection in the graph: its points gather round its center,
// and the picture draws a soft ring round them with its name.
type ggroup struct {
	name   string
	coll   *Collection
	cx, cy float64
	n      int
}

type graphState struct {
	nodes    []gnode
	edges    []gedge
	kinds    [3]bool // which kinds of line are drawn
	groups   []ggroup
	legend   []*Collection   // the collections that have points, for the chips
	hidden   map[string]bool // the collections left out
	builtAt  time.Time       // the index it was made from
	showAll  bool            // the pages with no link too
	iter     int             // the steps of the layout done
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
	if a.graph.zoom == 0 {
		a.graph.kinds = [3]bool{true, true, true}
	}
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
	type found struct {
		pair
		kind int
	}
	seen := map[pair]bool{}
	var links []found
	add := func(from, to *Page, kind int) {
		if from == to || !g.kinds[kind] || seen[pair{from, to}] || seen[pair{to, from}] {
			return
		}
		seen[pair{from, to}] = true
		links = append(links, found{pair{from, to}, kind})
	}
	for key, from := range a.links.into {
		if to := byTitle[key]; to != nil {
			for _, src := range from {
				add(src, to, edgeLink)
			}
		}
	}
	for _, p := range pages {
		if p.Parent != nil {
			add(p.Parent, p, edgeTree)
		}
	}
	// a page that names another by its title in its text, as a note says
	// "see `Biological Neural Networks`" with no link made
	if g.kinds[edgeMention] && len(pages) <= 1500 {
		type titled struct {
			p *Page
			t string
		}
		var titles []titled
		for _, p := range pages {
			if t := fold(strings.TrimSpace(p.Title)); utf8.RuneCountInString(t) >= 8 {
				titles = append(titles, titled{p, t})
			}
		}
		for _, p := range pages {
			text := fold(a.links.texts[p])
			if text == "" {
				continue
			}
			for _, t := range titles {
				if t.p != p && strings.Contains(text, t.t) {
					add(p, t.p, edgeMention)
				}
			}
		}
	}
	deg := map[*Page]int{}
	for _, l := range links {
		deg[l.from]++
		deg[l.to]++
	}
	var keep []*Page
	g.legend = g.legend[:0]
	shown := map[*Collection]bool{}
	for _, p := range pages {
		if deg[p] > 0 || g.showAll {
			if !shown[p.Coll] {
				shown[p.Coll] = true
				g.legend = append(g.legend, p.Coll)
			}
			if g.hidden[p.Coll.Name] {
				continue
			}
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
			n.fresh = true
		}
		h := fnv.New64a()
		h.Write([]byte(p.Dir))
		n.seed = h.Sum64()
		g.nodes = append(g.nodes, n)
	}
	g.edges = g.edges[:0:0]
	for _, l := range links {
		i, ok1 := idx[l.from]
		j, ok2 := idx[l.to]
		if ok1 && ok2 {
			g.edges = append(g.edges, gedge{i, j, l.kind})
		}
	}
	a.layoutGroups()
	g.iter, g.fit = 0, true
	if g.zoom == 0 {
		g.zoom = 1
	}
	g.hover, g.drag = -1, -1
}

// groupR is how far a collection's points reach from its center.
func groupR(n int) float64 { return 24*math.Sqrt(float64(n)) + 28 }

// layoutGroups gives the collections their places - far enough apart for
// their points, near each other as the pages of one link to the pages of the
// other - and puts the new points in them.
func (a *app) layoutGroups() {
	g := &a.graph
	old := map[*Collection]ggroup{}
	for _, gr := range g.groups {
		old[gr.coll] = gr
	}
	idx := map[*Collection]int{}
	g.groups = g.groups[:0]
	for i := range g.nodes {
		c := g.nodes[i].page.Coll
		gi, ok := idx[c]
		if !ok {
			gi = len(g.groups)
			idx[c] = gi
			g.groups = append(g.groups, ggroup{name: c.Name, coll: c})
		}
		g.nodes[i].group = gi
		g.groups[gi].n++
	}
	G := len(g.groups)
	if G == 0 {
		return
	}
	cross := make([][]float64, G)
	for i := range cross {
		cross[i] = make([]float64, G)
	}
	for _, e := range g.edges {
		x, y := g.nodes[e.a].group, g.nodes[e.b].group
		if x != y {
			cross[x][y]++
			cross[y][x]++
		}
	}
	// the first places: on a ring, the biggest first
	order := make([]int, G)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return g.groups[order[i]].n > g.groups[order[j]].n })
	sum := 0.0
	for _, gr := range g.groups {
		sum += groupR(gr.n)
	}
	ring := math.Max(sum*0.8, 120)
	for k, i := range order {
		if o, ok := old[g.groups[i].coll]; ok {
			g.groups[i].cx, g.groups[i].cy = o.cx, o.cy
			continue
		}
		ang := 2 * math.Pi * float64(k) / float64(G)
		if G == 1 {
			ring = 0
		}
		g.groups[i].cx, g.groups[i].cy = math.Cos(ang)*ring, math.Sin(ang)*ring
	}
	// then they settle: apart for their size, together for their links
	for step := 0; step < 260; step++ {
		temp := 30 * (1 - float64(step)/260)
		fx, fy := make([]float64, G), make([]float64, G)
		for i := 0; i < G; i++ {
			for j := i + 1; j < G; j++ {
				dx, dy := g.groups[i].cx-g.groups[j].cx, g.groups[i].cy-g.groups[j].cy
				d := math.Max(math.Hypot(dx, dy), 0.5)
				want := groupR(g.groups[i].n) + groupR(g.groups[j].n) + 36
				f := 0.0
				if d < want*1.6 {
					f = (want*1.6 - d) * 0.6 // pushed off, not by the distance it already has
				}
				// pulled together by the links between them, never closer than they are wide
				f -= math.Min(cross[i][j], 30) * 1.4 * math.Max(d-want, 0) / 60
				fx[i] += dx / d * f
				fy[i] += dy / d * f
				fx[j] -= dx / d * f
				fy[j] -= dy / d * f
			}
			fx[i] -= g.groups[i].cx * 0.02
			fy[i] -= g.groups[i].cy * 0.02
		}
		for i := range g.groups {
			l := math.Hypot(fx[i], fy[i])
			if l > temp {
				fx[i], fy[i] = fx[i]/l*temp, fy[i]/l*temp
			}
			g.groups[i].cx += fx[i]
			g.groups[i].cy += fy[i]
		}
	}
	for i := range g.nodes {
		n := &g.nodes[i]
		if !n.fresh {
			continue
		}
		gr := g.groups[n.group]
		ang := float64(n.seed%3600) / 3600 * 2 * math.Pi
		rad := groupR(gr.n) * (0.25 + 0.6*float64((n.seed/3600)%1000)/1000)
		n.x, n.y = gr.cx+math.Cos(ang)*rad, gr.cy+math.Sin(ang)*rad
		n.fresh = false
	}
}

// relax moves the points a few steps toward rest: all repel, the linked draw
// together (those of another collection less), each is drawn to its
// collection's center, and the collections keep their places. It reports
// whether it moved anything still.
func (g *graphState) relax(steps int) bool {
	n := len(g.nodes)
	if n == 0 || g.iter >= 320 {
		return false
	}
	const k = 34.0 // how far apart the points like to be
	for s := 0; s < steps && g.iter < 320; s++ {
		temp := 12 * (1 - float64(g.iter)/320)
		for i := range g.nodes {
			g.nodes[i].vx, g.nodes[i].vy = 0, 0
		}
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx, dy := g.nodes[i].x-g.nodes[j].x, g.nodes[i].y-g.nodes[j].y
				d2 := dx*dx + dy*dy
				if d2 > 120*120 {
					continue // too far to care
				}
				if d2 < 0.01 {
					dx, dy, d2 = 0.1, 0.1, 0.02
				}
				f := k * k / d2
				g.nodes[i].vx += dx * f
				g.nodes[i].vy += dy * f
				g.nodes[j].vx -= dx * f
				g.nodes[j].vy -= dy * f
			}
		}
		for _, e := range g.edges {
			a, b := &g.nodes[e.a], &g.nodes[e.b]
			dx, dy := a.x-b.x, a.y-b.y
			d := math.Sqrt(dx*dx+dy*dy) + 0.01
			f := d / (k * 1.4)
			switch e.kind {
			case edgeMention:
				f *= 0.4
			case edgeTree:
				f *= 0.9
			}
			if a.group != b.group {
				f *= 0.15 // between collections they pull less, so that each stays a cluster
			}
			a.vx -= dx * f
			a.vy -= dy * f
			b.vx += dx * f
			b.vy += dy * f
		}
		for i := range g.nodes {
			nd := &g.nodes[i]
			if nd.group < len(g.groups) {
				gr := g.groups[nd.group]
				nd.vx += (gr.cx - nd.x) * 0.2
				nd.vy += (gr.cy - nd.y) * 0.2
			}
			if nd.pinned {
				continue
			}
			l := math.Hypot(nd.vx, nd.vy)
			if l > temp {
				nd.vx, nd.vy = nd.vx/l*temp, nd.vy/l*temp
			}
			nd.x += nd.vx
			nd.y += nd.vy
			if nd.group < len(g.groups) { // a point stays in its collection's round
				gr := g.groups[nd.group]
				if dx, dy := nd.x-gr.cx, nd.y-gr.cy; math.Hypot(dx, dy) > groupR(gr.n)*0.9 {
					l := math.Hypot(dx, dy)
					nd.x, nd.y = gr.cx+dx/l*groupR(gr.n)*0.9, gr.cy+dy/l*groupR(gr.n)*0.9
				}
			}
		}
		g.iter++
	}
	return g.iter < 320
}

func (g *graphState) radius(n gnode) float64 { return 4.2 + math.Sqrt(float64(n.deg))*1.9 }

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

// halo is the round a collection's points are in: where its middle is, and
// how far it reaches.
func (g *graphState) halo(gi int) (cx, cy, r float64, ok bool) {
	n := 0
	for _, nd := range g.nodes {
		if nd.group == gi {
			cx, cy = cx+nd.x, cy+nd.y
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0, false
	}
	cx, cy = cx/float64(n), cy/float64(n)
	for _, nd := range g.nodes {
		if nd.group == gi {
			r = math.Max(r, math.Hypot(nd.x-cx, nd.y-cy)+g.radius(nd))
		}
	}
	return cx, cy, r + 22, true
}

// fitToCanvas puts the collections in the middle of the canvas, as large as
// it lets them be.
func (g *graphState) fitToCanvas() {
	if len(g.nodes) == 0 || g.w < 50 || g.h < 50 {
		return
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for gi := range g.groups {
		if cx, cy, r, ok := g.halo(gi); ok {
			minX, maxX = math.Min(minX, cx-r), math.Max(maxX, cx+r)
			minY, maxY = math.Min(minY, cy-r-18), math.Max(maxY, cy+r) // the name above
		}
	}
	if math.IsInf(minX, 1) {
		return
	}
	bw, bh := math.Max(maxX-minX, 1)+40, math.Max(maxY-minY, 1)+40
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
		a.graphKinds(c, cl)
		a.graphLegend(c, cl)
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

// drawGraph draws the collections' rounds, the lines and the points.
func (a *app) drawGraph(p *ui.Painter, r ui.Rect, cl ryugo.Colors) {
	g := &a.graph
	g.w, g.h = r.W, r.H
	q := fold(strings.TrimSpace(g.query))
	near := map[int]bool{}
	if g.hover >= 0 {
		near[g.hover] = true
		for _, e := range g.edges {
			if e.a == g.hover {
				near[e.b] = true
			}
			if e.b == g.hover {
				near[e.a] = true
			}
		}
	}
	at := func(i int) (float32, float32) {
		x, y := g.toScreen(g.nodes[i].x, g.nodes[i].y)
		return r.X + x, r.Y + y
	}
	// each collection: a soft round, and its name over it
	type tag struct {
		x, y float32
		text string
		col  ui.Color
	}
	var names []tag
	for gi, gr := range g.groups {
		cx, cy, rad, ok := g.halo(gi)
		if !ok {
			continue
		}
		sx, sy := g.toScreen(cx, cy)
		sr := float32(rad * g.zoom)
		tone := columnTone2(gr.name, cl)
		x, y := r.X+sx, r.Y+sy
		p.Fill(ui.Rect{X: x - sr - 1, Y: y - sr - 1, W: 2 * (sr + 1), H: 2 * (sr + 1)}, tone.Alpha(0.28), sr+1)
		p.Fill(ui.Rect{X: x - sr, Y: y - sr, W: 2 * sr, H: 2 * sr}, cl.Well, sr)
		p.Fill(ui.Rect{X: x - sr, Y: y - sr, W: 2 * sr, H: 2 * sr}, tone.Alpha(0.08), sr)
		names = append(names, tag{x, y - sr - 16, gr.name + "  " + strconv.Itoa(gr.n), tone})
	}
	for _, e := range g.edges {
		x0, y0 := at(e.a)
		x1, y1 := at(e.b)
		col, w := cl.Muted.Alpha(0.34), float32(1)
		switch e.kind {
		case edgeLink:
			col, w = cl.Accent.Alpha(0.55), 1.4
		case edgeMention:
			col = cl.C3.Alpha(0.38)
		case edgeTree:
			col = cl.Muted.Alpha(0.26)
		}
		if g.hover >= 0 {
			if e.a == g.hover || e.b == g.hover {
				col, w = cl.Accent.Alpha(0.9), 1.8
			} else {
				col = cl.Muted.Alpha(0.08)
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
		alpha := float32(0.95)
		if g.hover >= 0 && !near[i] {
			alpha = 0.22
		}
		match := q != "" && strings.Contains(fold(n.page.Title), q)
		if q != "" && !match {
			alpha = min(alpha, 0.18)
		}
		if a.page != nil && n.page == a.page {
			p.Fill(ui.Rect{X: x - rad - 3, Y: y - rad - 3, W: 2 * (rad + 3), H: 2 * (rad + 3)}, cl.Accent.Alpha(0.4), rad+3)
		}
		if match {
			p.Fill(ui.Rect{X: x - rad - 4, Y: y - rad - 4, W: 2 * (rad + 4), H: 2 * (rad + 4)}, cl.Accent.Alpha(0.55), rad+4)
		}
		p.Fill(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, tone.Alpha(alpha), rad)
		hot := i == g.hover
		if hot || match || (g.hover < 0 && q == "" && g.zoom >= 2) || (g.hover >= 0 && near[i]) {
			labels = append(labels, label{x, y + rad + 3, n.page.Title, hot})
		}
	}
	for _, t := range names {
		sp := ui.Span{Text: t.text, Size: 13, Weight: 680, Color: t.col}
		w, _ := p.MeasureText(0, sp)
		p.RichText(t.x-w/2, t.y, w+2, sp)
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

// graphLegend is a chip for each collection that has points: its color and
// name, and a click leaves it out of the graph, or brings it back.
func (a *app) graphLegend(c *ui.Context, cl ryugo.Colors) {
	g := &a.graph
	if len(g.legend) < 2 {
		return
	}
	ui.Row(c.Key("graph-legend")).Wrap().Gap(6).Children(func() {
		for _, co := range g.legend {
			co := co
			off := g.hidden[co.Name]
			tone := columnTone2(co.Name, cl)
			chip := ui.Row(c.Key("gl:"+co.Dir)).Gap(6).Padding(3, 10, 3, 8).Radius(12).AlignItems(ui.Center).
				Label("Koleksiyon: " + co.Name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if chip.Hovered() {
				chip.Background(cl.FillSelected)
			} else {
				chip.Background(cl.Fill)
			}
			chip.Opacity(map[bool]float32{true: 0.5, false: 1}[off])
			chip.Children(func() {
				ui.Box(c).Size(9, 9).Radius(4.5).Background(tone)
				t := ui.Text(c, co.Name).FontSize(12.5).TextColor(cl.Text2)
				if off {
					t.Strikethrough()
				}
			})
			if chip.Clicked() {
				if g.hidden == nil {
					g.hidden = map[string]bool{}
				}
				g.hidden[co.Name] = !off
				g.builtAt = time.Time{}
			}
		}
	})
}

// graphKinds are the switches of the kinds of line, each with its color.
func (a *app) graphKinds(c *ui.Context, cl ryugo.Colors) {
	g := &a.graph
	names := [3]string{"[[Bağlantı]]", "Metinde adı geçen", "Alt sayfa"}
	tones := [3]ui.Color{cl.Accent, cl.C3, cl.Muted}
	ui.Row(c.Key("graph-kinds")).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Çizgiler:").FontSize(12.5).TextColor(cl.Muted)
		for k := range names {
			k := k
			on := g.kinds[k]
			chip := ui.Row(c.Key("gk"+strconv.Itoa(k))).Gap(6).Padding(3, 10, 3, 8).Radius(12).AlignItems(ui.Center).
				Label("Çizgi: " + names[k]).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if chip.Hovered() {
				chip.Background(cl.FillSelected)
			} else {
				chip.Background(cl.Fill)
			}
			chip.Opacity(map[bool]float32{true: 1, false: 0.5}[on])
			chip.Children(func() {
				ui.Box(c).Width(14).Height(3).Radius(1.5).Background(tones[k])
				t := ui.Text(c, names[k]).FontSize(12.5).TextColor(cl.Text2)
				if !on {
					t.Strikethrough()
				}
			})
			if chip.Clicked() {
				g.kinds[k] = !on
				g.builtAt = time.Time{}
			}
		}
	})
}
