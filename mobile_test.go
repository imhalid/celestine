package main

import (
	"github.com/imhalid/celestine/locales"
	"testing"
	"time"
)

// A phone: the sidebar is a drawer that ☰ opens and that goes as something is opened; one pane, no side
// panel; what opens beside opens here; the commands of the panes and the panel are dim.
func TestPhoneLayout(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	defer func() { phone = false; rowH = 30 }()
	a.forcePhone = true
	a.settings.TourDone = true
	tt.SetSize(390, 844)
	tt.Advance(300 * time.Millisecond)
	if !phone || rowH != 42 {
		t.Fatalf("not a phone: %v, rows %v", phone, rowH)
	}
	if a.sidebarShown != 0 || a.drawerOpen {
		t.Fatalf("the drawer is open at the start: %v %v", a.sidebarShown, a.drawerOpen)
	}
	if err := tt.Click(locales.MobileMenu()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if !a.drawerOpen || a.sidebarShown < 100 {
		t.Fatalf("☰ did not open the drawer: %v %v", a.drawerOpen, a.sidebarShown)
	}
	if err := tt.Click("Sayfa: " + coll.Pages[0].Title); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.page != coll.Pages[0] || a.drawerOpen {
		t.Fatalf("a page opened from the drawer: page %v, drawer %v", a.page, a.drawerOpen)
	}
	a.mu.Lock()
	a.openInSide(place{kind: "tasks"})
	a.mu.Unlock()
	tt.Frame()
	if a.paneCount() != 1 || !a.tasksOpen {
		t.Fatalf("beside on a phone: %d panes, tasks %v", a.paneCount(), a.tasksOpen)
	}
	for _, it := range a.commands() {
		switch it.title {
		case msg("Yan paneli aç/kapat"), msg(locales.NewPaneThisScreen()), msg("Yan bölmede aç: bu ekran"), msg("Bölmeyi ayrı pencereye al"):
			if it.off == "" {
				t.Fatalf("%q is offered on a phone", it.title)
			}
		}
	}
	// a desktop window is not a phone
	a.forcePhone = false
	tt.SetSize(1000, 700)
	tt.Advance(300 * time.Millisecond)
	if phone || rowH != 30 {
		t.Fatalf("a desktop window is laid out as a phone: %v %v", phone, rowH)
	}
}
