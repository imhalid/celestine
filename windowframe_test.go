package main

import (
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// Split, the window's frame lists the panes as tabs in its top edge, and the Dock's areas have no headers of
// their own; a tab puts its pane in use, its close button closes it, and with one pane left the strip folds.
func TestFrameTabs(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1200, 700)
	tt.Advance(300 * time.Millisecond)
	if a.wframe.strip != 0 {
		t.Fatalf("one pane, and the strip is open: %v", a.wframe.strip)
	}
	second, err := a.page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	a.openInSide(place{"page", second.Dir})
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 2 || a.wframe.strip < 0.99 {
		t.Fatalf("panes %d, strip %v", a.paneCount(), a.wframe.strip)
	}
	if _, ok := tt.Find("Maximize"); ok {
		t.Fatal("an area has its header under the strip")
	}
	if err := tt.Click(a.ps.states[0].page.Title); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if a.ps.active != 0 {
		t.Fatalf("the first tab did not put its pane in use: %d", a.ps.active)
	}
	if err := tt.Click(locales.CloseThePane() + ": İkinci"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 1 || a.wframe.strip > 0.01 {
		t.Fatalf("after closing: panes %d, strip %v", a.paneCount(), a.wframe.strip)
	}
}
