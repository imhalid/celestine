package main

import (
	"testing"
	"time"
)

// TestBoardOnDisk: a page moved on the board keeps its status and place
// across a reopening; a page without a status is in the first column.
func TestBoardOnDisk(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	if got := titles(one.Column("Yapılacak")); !equal(got, []string{"A", "B", "B1", "C"}) {
		t.Fatalf("first column: %v", got)
	}
	if err := one.MoveOnBoard(ps["C"], "Sürüyor", 0); err != nil {
		t.Fatal(err)
	}
	if err := one.MoveOnBoard(ps["A"], "Sürüyor", 0); err != nil {
		t.Fatal(err)
	}
	if err := one.MoveOnBoard(ps["B1"], "Yapılacak", 0); err != nil { // to the top of its own column
		t.Fatal(err)
	}
	again, _ := OpenLibrary(lib.Root)
	var c *Collection
	for _, x := range again.Collections {
		if x.Name == "Bir" {
			c = x
		}
	}
	if got := titles(c.Column("Sürüyor")); !equal(got, []string{"A", "C"}) {
		t.Fatalf("under way: %v", got)
	}
	if got := titles(c.Column("Yapılacak")); !equal(got, []string{"B1", "B"}) {
		t.Fatalf("to do: %v", got)
	}
	if body, err := c.All()[0].Body(); err != nil || body != "" {
		t.Fatalf("the body after a move: %q %v", body, err)
	}
	// A collection made before boards gets one.
	c.Views = []View{{ID: "list", Label: "Liste", Type: "list"}}
	if c.View(); c.Views[len(c.Views)-1].Type != "board" {
		t.Fatal("no board")
	}
}

// TestBoardDrag drags a card to the next column, as a pointer does: it
// takes that column's status. A click on a card opens its page.
func TestBoardDrag(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Pano"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	from, ok := tt.Find("Kart: Dune")
	if !ok {
		t.Fatalf("no card: %v", tt.Texts())
	}
	to, ok := tt.Find("Sürüyor")
	if !ok {
		t.Fatal("no column")
	}
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1, y1 := to.X+40, to.Y+60
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+(x1-x0)*float32(k)/10, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Advance(time.Second)
	var dune *Page
	for _, p := range coll.All() {
		if p.Title == "Dune" {
			dune = p
		}
	}
	if dune.Status != "Sürüyor" || !equal(titles(coll.Column("Sürüyor")), []string{"Dune"}) {
		t.Fatalf("status %q, column %v", dune.Status, titles(coll.Column("Sürüyor")))
	}
	if a.coll != coll {
		t.Fatal("the drop opened the page")
	}
	if err := tt.Click("Kart: Anna Karenina"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page == nil || a.page.Title != "Anna Karenina" || a.coll != nil {
		t.Fatal("a click on a card does not open its page")
	}
}

// TestStatusOnPage sets the open page's status from its property row.
func TestStatusOnPage(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	if err := tt.Click("Yapılacak"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Bitti"); err != nil {
		t.Fatalf("%v: %v", err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if a.page.Status != "Bitti" {
		t.Fatalf("status %q", a.page.Status)
	}
	if body, _ := a.page.Body(); body != "gövde\n" {
		t.Fatalf("the text after the status: %q", body)
	}
}
