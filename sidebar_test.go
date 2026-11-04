package main

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSidebarFolds: ⌘\ folds the sidebar on a spring - on its way in
// between, never past - and brings it back.
func TestSidebarFolds(t *testing.T) {
	a, tt := newTestApp(t, "x")
	tt.Advance(100 * time.Millisecond)
	if math.Abs(float64(a.sidebarShown-sidebarW)) > 0.5 {
		t.Fatalf("open: %v", a.sidebarShown)
	}
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Advance(80 * time.Millisecond)
	if mid := a.sidebarShown; mid <= 0 || mid >= sidebarW {
		t.Fatalf("folding should be under way: %v", mid)
	}
	low := a.sidebarShown
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		if a.sidebarShown < -0.5 || a.sidebarShown > low+0.5 {
			t.Fatalf("went past or back: %v after %v", a.sidebarShown, low)
		}
		low = a.sidebarShown
	}
	if a.sidebarShown > 0.5 {
		t.Fatalf("folded: %v", a.sidebarShown)
	}
	if err := tt.Click("Kenar çubuğu (⌘\\)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(800 * time.Millisecond)
	if math.Abs(float64(a.sidebarShown-sidebarW)) > 0.5 {
		t.Fatalf("back: %v", a.sidebarShown)
	}
}

// TestBranchFolds: a page's chevron hides its children, and they come back.
func TestBranchFolds(t *testing.T) {
	a, tt := newTestApp(t, "x")
	parent := a.page
	if _, err := parent.Coll.NewPage(parent, "Çocuk"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second) // the branch has unfolded
	if err := tt.Click("Sayfa: Çocuk"); err != nil {
		t.Fatal("the child row is not shown: ", err)
	}
	parent.Open = false
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click("Sayfa: Çocuk"); err == nil {
		t.Fatal("the child row is still there once its branch folded")
	}
}
