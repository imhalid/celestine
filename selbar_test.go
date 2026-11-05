package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A selection brings up the bar: a mark is applied from it, and a link is
// asked for in the bar itself.
func TestSelectionBar(t *testing.T) {
	a, tt := newTestApp(t, "Bir iki üç\n")
	tt.Frame()
	if tt.HasText("Kalın (⌘B)") {
		t.Fatal("a bar with nothing selected")
	}
	a.doc.SetSelection(4, 7) // "iki"
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Kalın (⌘B)") {
		an, ca := a.doc.Selection()
		t.Fatalf("no bar: sel %d,%d focused %v pressed %v boxes %d slash %v", an, ca, a.focused, a.edPressed, len(a.boxes), a.slash.open)
	}
	if err := tt.Click("Kalın (⌘B)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if got := mdOf(a.doc); got != "Bir **iki** üç" {
		t.Fatalf("not bold: %q", got)
	}
	// the selection stayed, the bar too: a link
	if err := tt.Click("Bağlantı (⌘K)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	typeEach(tt, "https://example.com")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if got := mdOf(a.doc); !strings.Contains(got, "[**iki**](https://example.com)") && !strings.Contains(got, "](https://example.com)") {
		t.Fatalf("not linked: %q", got)
	}
}
