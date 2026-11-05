package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSlashMenu(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Advance(100 * time.Millisecond)
	if !a.slash.open || !tt.HasText("Bloklar") {
		t.Fatalf("not open: %+v", a.slash)
	}
	typeEach(tt, "bas")
	tt.Frame()
	if items := slashItems(a.slashQuery()); len(items) != 3 || items[0].kind != "h1" {
		t.Fatalf("narrowed: %+v", items)
	}
	tt.Key(0, ui.KeyDown) // the second: Başlık 2
	tt.Frame()
	if a.slash.sel != 1 {
		t.Fatalf("sel %d", a.slash.sel)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.slash.open {
		t.Fatal("still open")
	}
	typeEach(tt, "Başlık")
	if got := mdOf(a.doc); got != "Önce\n\n## Başlık" {
		t.Fatalf("chosen: %q", got)
	}
	// Esc closes it; the "/" stays as typed
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.slash.open {
		t.Fatal("Esc did not close it")
	}
	// a / inside a line opens nothing
	typeEach(tt, "a/b")
	if a.slash.open {
		t.Fatal("open inside a line")
	}
}
