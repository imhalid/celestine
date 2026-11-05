package main

import (
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
	if !a.graphOpen || len(g.nodes) != 3 || len(g.edges) != 3 {
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
