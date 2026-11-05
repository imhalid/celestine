package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSidePanel: the panel opens with its button and its key, lists the
// page's headings - a click goes to one - and stays open as it was left.
func TestSidePanel(t *testing.T) {
	a, tt := newTestApp(t, "# Bir\n\nmetin\n\n## İki\n\nmetin daha\n\n# Üç\n")
	tt.SetSize(1100, 800)
	tt.Frame()
	if a.settings.PanelOpen || tt.HasText("İÇİNDEKİLER") {
		t.Fatal("open at the start")
	}
	if err := tt.Click("Yan panel (⇧⌘\\)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if !a.settings.PanelOpen || !tt.HasText("İÇİNDEKİLER") || !tt.HasText("BİLGİ") {
		t.Fatalf("not shown: %v", tt.Texts())
	}
	hs := a.outline()
	if len(hs) != 3 || hs[1].text != "İki" || hs[1].level != 2 || hs[2].text != "Üç" {
		t.Fatalf("outline: %+v", hs)
	}
	if err := tt.Click("Başlık: Üç"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if _, caret := a.doc.Selection(); caret != hs[2].at {
		t.Fatalf("caret %d, want %d", caret, hs[2].at)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyBackslash)
	tt.Advance(600 * time.Millisecond)
	if a.settings.PanelOpen || tt.HasText("İÇİNDEKİLER") {
		t.Fatal("not closed by the key")
	}
}
