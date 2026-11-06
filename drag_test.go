package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// dragLibrary: collection "Bir" with pages A, B, C (B has a sub-page B1),
// and an empty collection "İki".
func dragLibrary(t *testing.T) (*Library, map[string]*Page, *Collection, *Collection) {
	t.Helper()
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	one, _ := lib.NewCollection("Bir")
	two, _ := lib.NewCollection("İki")
	ps := map[string]*Page{}
	for _, name := range []string{"A", "B", "C"} {
		ps[name], _ = one.NewPage(nil, name)
	}
	ps["B1"], _ = one.NewPage(ps["B"], "B1")
	return lib, ps, one, two
}

func order(ps []*Page) []string { return titles(ps) }

func TestMovePage(t *testing.T) {
	lib, ps, one, two := dragLibrary(t)
	// Reorder: C to the top.
	if err := lib.MovePage(ps["C"], one, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := order(one.Pages); !equal(got, []string{"C", "A", "B"}) {
		t.Fatalf("reorder: %v", got)
	}
	// Under another page: B (with B1) under A; its folder goes along.
	oldB1 := ps["B1"].Dir
	if err := lib.MovePage(ps["B"], one, ps["A"], 0); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(ps["B"].Dir) != ps["A"].Dir || filepath.Dir(ps["B1"].Dir) != ps["B"].Dir {
		t.Fatalf("folders: %s %s", ps["B"].Dir, ps["B1"].Dir)
	}
	if _, err := os.Stat(oldB1); !os.IsNotExist(err) {
		t.Fatal("the old folder is still there")
	}
	if body, err := ps["B1"].Text(); err != nil || body != "" {
		t.Fatalf("B1 unreadable after the move: %v", err)
	}
	// Not under itself.
	if err := lib.MovePage(ps["A"], one, ps["B1"], 0); err == nil {
		t.Fatal("a page went under its own sub-page")
	}
	// To another collection, sub-pages and all.
	if err := lib.MovePage(ps["A"], two, nil, 0); err != nil {
		t.Fatal(err)
	}
	if ps["B1"].Coll != two || len(one.Pages) != 1 || filepath.Dir(ps["A"].Dir) != two.Dir {
		t.Fatalf("across: coll %v, left %v", ps["B1"].Coll.Name, order(one.Pages))
	}
	// Read back: the same tree, in the same order.
	again, _ := OpenLibrary(lib.Root)
	var a1, a2 *Collection
	for _, c := range again.Collections {
		switch c.Name {
		case "Bir":
			a1 = c
		case "İki":
			a2 = c
		}
	}
	if !equal(order(a1.Pages), []string{"C"}) || !equal(order(a2.Pages), []string{"A"}) ||
		!equal(order(a2.Pages[0].Children), []string{"B"}) || !equal(order(a2.Pages[0].Children[0].Children), []string{"B1"}) {
		t.Fatalf("read back: %v %v", order(a1.Pages), order(a2.Pages))
	}
}

func TestOrderSurvives(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	lib.MovePage(ps["A"], one, nil, 3) // A to the end
	again, _ := OpenLibrary(lib.Root)
	for _, c := range again.Collections {
		if c.Name == "Bir" && !equal(order(c.Pages), []string{"B", "C", "A"}) {
			t.Fatalf("order after reopening: %v", order(c.Pages))
		}
	}
}

// dragTo drags the row of page from to fraction frac of the height of row
// i of the tree, as a pointer does, and lets go.
func dragTo(t *testing.T, a *app, tt *ui.Tester, from *Page, i int, frac float32) {
	t.Helper()
	row := func(p *Page) int {
		for j, e := range a.rows {
			if e.page == p {
				return j
			}
		}
		t.Fatalf("no row for %s", from.Title)
		return -1
	}
	r := a.treeR
	x := r.X + 70
	y0 := r.Y + float32(row(from))*rowH + rowH/2
	// Picked up, the page's rows close their room: the rows below come up.
	if f := row(from); i > f {
		n := 0
		for _, e := range a.rows {
			if e.page != nil && from.Contains(e.page) {
				n++
			}
		}
		i -= n
	}
	y1 := r.Y + (float32(i)+frac)*rowH
	tt.Press(x, y0)
	tt.Move(x, y0+8) // picked up: it may go back to where it was
	tt.Frame()
	for k := 1; k <= 8; k++ {
		tt.Move(x, y0+(y1-y0)*float32(k)/8)
		tt.Frame()
	}
	if a.drag.page != from {
		t.Fatalf("no drag under way")
	}
	tt.Release(x, y1)
	tt.Advance(time.Second)
}

func rowOf(a *app, item any) int {
	for j, e := range a.rows {
		if (e.page != nil && any(e.page) == item) || (e.coll != nil && any(e.coll) == item) {
			return j
		}
	}
	return -1
}

func TestDragInTree(t *testing.T) {
	lib, ps, one, two := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 900)
	tt.Advance(time.Second)
	// C onto the middle of A: a sub-page of A.
	dragTo(t, a, tt, ps["C"], rowOf(a, ps["A"]), 0.5)
	if ps["C"].Parent != ps["A"] || filepath.Dir(ps["C"].Dir) != ps["A"].Dir {
		t.Fatalf("into: parent %v", ps["C"].Parent)
	}
	if a.page != ps["A"] {
		t.Fatal("the drop opened the page as a click would")
	}
	// B onto the top edge of A: before it.
	dragTo(t, a, tt, ps["B"], rowOf(a, ps["A"]), 0.1)
	if got := order(one.Pages); !equal(got, []string{"B", "A"}) {
		t.Fatalf("before: %v", got)
	}
	// A onto the other collection's row: to its top pages, C with it.
	dragTo(t, a, tt, ps["A"], rowOf(a, two), 0.5)
	if ps["A"].Coll != two || ps["C"].Coll != two || !equal(order(one.Pages), []string{"B"}) {
		t.Fatalf("to a collection: %v / %v", order(one.Pages), order(two.Pages))
	}
	// Let go where it was, a page stays: the room it left is where it lands.
	ps["B"].Open = true
	tt.Advance(time.Second)
	dragTo(t, a, tt, ps["B"], rowOf(a, ps["B"]), 0.1)
	if ps["B"].Parent != nil || ps["B1"].Parent != ps["B"] || !equal(order(one.Pages), []string{"B"}) {
		t.Fatal("the page moved")
	}
	if a.drag.landing != nil || a.moving() {
		t.Fatal("the copy never landed")
	}
}

// TestClickAfterDrag: the click right after a drag is a click again.
func TestClickAfterDrag(t *testing.T) {
	lib, ps, _, _ := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 900)
	tt.Advance(time.Second)
	dragTo(t, a, tt, ps["C"], rowOf(a, ps["A"]), 0.1)
	if err := tt.Click("Sayfa: C"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != ps["C"] {
		t.Fatalf("the click after a drag was swallowed: open %v", a.page.Title)
	}
}
