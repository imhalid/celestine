package main

import (
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestRowTitleFullWidth: a row's title runs to the row's end, keeping no room for the + that is not shown, and the
// + comes over the title's end as the pointer comes over the row, and makes a page under it (#387).
func TestRowTitleFullWidth(t *testing.T) {
	a, tt := newTestApp(t, "x")
	title := "Read Amplification in Log-Structured Merge Trees"
	p, err := a.pane().page.Coll.NewPage(nil, title)
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	row, ok := tt.Find(locales.CommonPageLabel(title))
	if !ok {
		t.Fatal("the page's row is not shown")
	}
	tx, ok := tt.Find(title)
	if !ok {
		t.Fatal("the page's title is not shown")
	}
	if end := row.X + row.W - 7; tx.X+tx.W < end-0.5 {
		t.Fatalf("the title ends at %v, short of the row's end %v: room kept for the +", tx.X+tx.W, end)
	}
	plus, ok := tt.Find(locales.SidebarAddLabel(title))
	if !ok {
		t.Fatal("the row has no +")
	}
	if plus.X < tx.X+tx.W-plus.W-6 {
		t.Fatalf("the + at %v is not over the title's end %v", plus.X, tx.X+tx.W)
	}
	// shot is the +'s box as the window shows it
	shot := func() []uint8 {
		img := tt.Image()
		s := float32(img.Bounds().Dx()) / 900
		var px []uint8
		for y := plus.Y; y < plus.Y+plus.H; y++ {
			for x := plus.X; x < plus.X+plus.W; x++ {
				r, g, b, _ := img.At(int(x*s), int(y*s)).RGBA()
				px = append(px, uint8(r>>8), uint8(g>>8), uint8(b>>8))
			}
		}
		return px
	}
	tt.Move(10, 5) // away from the tree
	tt.Advance(500 * time.Millisecond)
	rest := shot()
	tt.Move(row.X+row.W/2, row.Y+row.H/2)
	tt.Advance(500 * time.Millisecond)
	changed := 0
	for i, v := range shot() {
		if d := int(v) - int(rest[i]); d > 40 || d < -40 {
			changed++
		}
	}
	if changed < 30 {
		t.Fatalf("the + does not show on hover: %d channels changed", changed)
	}
	tt.ClickAt(plus.X+plus.W/2, plus.Y+plus.H/2)
	tt.Advance(500 * time.Millisecond)
	if len(p.Children) != 1 {
		t.Fatalf("the + made no page under %q: %d children", p.Title, len(p.Children))
	}
}
