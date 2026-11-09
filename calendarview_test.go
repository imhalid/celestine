package main

import (
	"testing"
	"time"
)

// On a calendar by a date property, a page in an open day is dragged onto another day, which becomes its date; the
// day under the pointer is lit while it is held, and the drop opens nothing.
func TestCalendarDragToAnotherDay(t *testing.T) {
	a, tt, coll := viewsLibrary(t) // now: 4 November 2026
	pr, err := coll.AddProperty("Okuma", PropDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	var dune *Page
	for _, p := range coll.All() {
		if p.Title == "Dune" {
			dune = p
		}
	}
	if err := dune.SetValue(pr.ID, "2026-11-10"); err != nil {
		t.Fatal(err)
	}
	for i := range coll.Views {
		if coll.Views[i].Type == "calendar" {
			coll.Views[i].DateField = "prop:" + pr.ID
			coll.ActiveView = coll.Views[i].ID
		}
	}
	tt.Advance(400 * time.Millisecond)
	if err := tt.Click("Gün: 2026-11-10"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	from, ok := tt.Find("Aç: Dune")
	if !ok {
		t.Fatalf("the day's card has no Dune: %v", tt.Texts())
	}
	to, ok := tt.Find("Gün: 2026-11-19")
	if !ok {
		t.Fatal("no 19th")
	}
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1, y1 := to.X+to.W/2, to.Y+to.H/2
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+(x1-x0)*float32(k)/10, y0+(y1-y0)*float32(k)/10)
		tt.Frame()
	}
	if a.calDrag.page != dune || !sameDay(a.calDrag.over, time.Date(2026, 11, 19, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("not held over the 19th: %+v", a.calDrag)
	}
	tt.Release(x1, y1)
	tt.Advance(time.Second)
	if got := dune.Props[pr.ID]; got != "2026-11-19" {
		t.Fatalf("Dune's date: %q", got)
	}
	if a.calDrag.page != nil || a.coll != coll || a.calOpen {
		t.Fatalf("the drop left a page held %v, opened a page (collection %v), or the day open %v", a.calDrag.page, a.coll, a.calOpen)
	}
}
