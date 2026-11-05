package main

import (
	"math"
	"testing"
	"time"
)

// TestGraph: the pages that link, and are linked, are the points; they come
// to rest, a click on one opens its page, and the wheel zooms.
func TestGraph(t *testing.T) {
	a, tt := newTestApp(t, "bu [[Beta]] ve [[Gama]] sayfalarına bağlanır\n")
	tt.SetSize(1200, 800)
	b, _ := a.page.Coll.NewPage(nil, "Beta")
	b.Save("beta, [[Sayfa]] sayfasına döner\n")
	c, _ := a.page.Coll.NewPage(nil, "Gama")
	c.Save("gama\n")
	lone, _ := a.page.Coll.NewPage(nil, "Yalnız")
	lone.Save("kimseyle bağı yok\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	if err := tt.Click("Bağlantı grafiği"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		tt.Advance(32 * time.Millisecond)
	}
	g := &a.graph
	if !a.graphOpen || len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Fatalf("graph: open %v, %d points, %d lines", a.graphOpen, len(g.nodes), len(g.edges))
	}
	if g.iter < 320 {
		t.Fatalf("still moving after the steps: %d", g.iter)
	}
	// the points do not lie on one another
	for i := range g.nodes {
		for j := i + 1; j < len(g.nodes); j++ {
			if dx, dy := g.nodes[i].x-g.nodes[j].x, g.nodes[i].y-g.nodes[j].y; dx*dx+dy*dy < 100 {
				t.Fatalf("points %d and %d on one another", i, j)
			}
		}
	}
	// the wheel zooms
	if !g.fit {
		t.Fatal("not fitted before a hand")
	}
	z := g.zoom
	b0 := g.w / 2
	tt.Wheel(b0, 300, 0, -120)
	tt.Advance(100 * time.Millisecond)
	if g.zoom <= z {
		t.Fatalf("zoom %v -> %v", z, g.zoom)
	}
	// a click on a point opens its page
	g.zoom, g.panX, g.panY = 1, 0, 0
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	var target *gnode
	for i := range g.nodes {
		if g.nodes[i].page == c {
			target = &g.nodes[i]
		}
	}
	sx, sy := g.toScreen(target.x, target.y)
	tt.Move(g.r.X+sx, g.r.Y+sy)
	tt.Frame()
	if g.hover < 0 {
		t.Fatal("no point under the pointer")
	}
	tt.ClickAt(g.r.X+sx, g.r.Y+sy)
	tt.Advance(200 * time.Millisecond)
	if a.page != c || a.graphOpen {
		t.Fatalf("page %q, graph %v", a.page.Title, a.graphOpen)
	}
	// the pages without links on request
	a.openGraph()
	tt.Advance(100 * time.Millisecond)
	if len(g.nodes) != 3 {
		t.Fatalf("the lone page is in: %d", len(g.nodes))
	}
}

// TestGraphKinds: a page's sub-page and a page that names another by its
// title are lines too, each kind to be put out; the points keep to their
// collections' rounds.
func TestGraphKinds(t *testing.T) {
	a, tt := newTestApp(t, "Burada Uzun Başlıklı Sayfa diye bir şeyden söz edilir\n")
	tt.SetSize(1200, 800)
	other, _ := a.lib.NewCollection("Öteki")
	big, _ := other.NewPage(nil, "Uzun Başlıklı Sayfa")
	big.Save("içerik\n")
	other.SubPages = true
	sub, _ := other.NewPage(big, "Çocuk")
	sub.Save("alt\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	a.openGraph()
	for i := 0; i < 40; i++ {
		tt.Advance(32 * time.Millisecond)
	}
	g := &a.graph
	kinds := map[int]int{}
	for _, e := range g.edges {
		kinds[e.kind]++
	}
	if kinds[edgeMention] != 1 || kinds[edgeTree] != 1 || kinds[edgeLink] != 0 {
		t.Fatalf("lines: %v", kinds)
	}
	if len(g.groups) != 2 {
		t.Fatalf("groups: %d", len(g.groups))
	}
	for _, n := range g.nodes {
		gr := g.groups[n.group]
		if d := math.Hypot(n.x-gr.cx, n.y-gr.cy); d > groupR(gr.n)+1 {
			t.Fatalf("%s is %.0f from its center, past %.0f", n.page.Title, d, groupR(gr.n))
		}
	}
	// the kind put out
	g.kinds[edgeMention] = false
	g.builtAt = time.Time{}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	kinds = map[int]int{}
	for _, e := range g.edges {
		kinds[e.kind]++
	}
	if kinds[edgeMention] != 0 || kinds[edgeTree] != 1 {
		t.Fatalf("after: %v", kinds)
	}
}
