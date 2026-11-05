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
	if !a.slash.open || !tt.HasText("Öneriler") {
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

// One menu, whichever sign opens it: a / in a line, at a word's start,
// opens it as @ does, the lines carry no Markdown marks, and the menu stays
// inside the window.
func TestOneMenu(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "Oku /")
	tt.Frame()
	if !a.slash.open || a.slash.sign != "/" {
		t.Fatalf("a / at a word's start did not open it: %+v", a.slash)
	}
	slashItems := a.menuItems("")
	groups := map[string]bool{}
	for _, it := range slashItems {
		groups[it.group] = true
		if it.hint != "" && it.group == "Satırı biçimlendir" {
			t.Fatalf("a mark on %q: %q", it.title, it.hint)
		}
	}
	for _, g := range []string{"Öneriler", "Satırı biçimlendir", "Tarih"} {
		if !groups[g] {
			t.Fatalf("no %q in the menu: %v", g, groups)
		}
	}
	// the same lines for the other sign
	a.slash.sign = "@"
	if got := a.menuItems(""); len(got) != len(slashItems) {
		t.Fatalf("@ and / differ: %d, %d", len(got), len(slashItems))
	}
	// a kind chosen in a line turns the line, and keeps what is before the sign
	a.slash.sign = "/"
	a.menuItems("bas")[0].run()
	typeEach(tt, "x")
	if got := mdOf(a.doc); got != "Önce\n\n# Oku x" {
		t.Fatalf("chosen mid-line: %q", got)
	}
	// no room below: the menu is above the caret, inside the window
	tt.SetSize(900, 260)
	tt.Frame()
	typeEach(tt, " /")
	tt.Frame()
	tt.Frame()
	r, ok := tt.Find("Öneriler")
	if !ok {
		t.Fatal("no menu")
	}
	if r.Y < 0 || r.Y+r.H > 260 {
		t.Fatalf("outside the window: %+v", r)
	}
}
