package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// The status bar's hints follow the caret: a heading's, a list item's, a selection's, an empty line's; the keys known
// to all are not among them, and a key rebound is shown as it is bound (#270).
func TestHintsFollowTheBlock(t *testing.T) {
	a, tt := newTestApp(t, "Intro\n\n# Notes\n\ntext under it\n\n- one\n  - two\n\nplain\n")
	tt.Advance(100 * time.Millisecond)
	a.refocus = true
	tt.Frame()
	whats := func(at string, sel int) []string {
		t.Helper()
		i := strings.Index(a.doc.Text().String(), at)
		if at == "" {
			i = a.doc.Text().Len()
		}
		a.doc.SetSelection(i, i+sel)
		tt.Frame()
		var out []string
		for _, h := range a.hints() {
			if h.key == "⌘P" || h.key == "⌘K" || h.key == "⌘F" || h.key == "[[" || h.what == "normal kip" {
				t.Fatalf("a key known to all at %q: %+v", at, h)
			}
			out = append(out, locales.Catalog.Format("en", h.what, nil)) // its English, as the hints were written
		}
		return out
	}
	if !a.focused {
		t.Fatal("the text has not the focus")
	}
	if h := whats("Notes", 0); !slices.Contains(h, "zoom into the section") || !slices.Contains(h, "fold") || !slices.Contains(h, "heading level") {
		t.Fatalf("a heading's: %v", h)
	}
	if h := whats("one", 0); !slices.Contains(h, "indent") || !slices.Contains(h, "zoom into the item") {
		t.Fatalf("a list item's: %v", h)
	}
	if h := whats("two", 0); !slices.Contains(h, "indent") || slices.Contains(h, "zoom into the item") {
		t.Fatalf("a list item with nothing under it: %v", h)
	}
	if h := whats("plain", 3); h[0] != "bold" {
		t.Fatalf("a selection's: %v", h)
	}
	if h := whats("plain", 0); !slices.Contains(h, "move") || !slices.Contains(h, "duplicate") {
		t.Fatalf("a paragraph's: %v", h)
	}
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	a.doc.InsertText("\n") // an empty line at the end
	if h := whats("", 0); len(h) == 0 || h[0] != "add a block" {
		t.Fatalf("an empty line's: %v", h)
	}
	// a key rebound is shown as bound
	if err := a.bindKey("Bloğu çoğalt", "alt+shift+d"); err != nil {
		t.Fatal(err)
	}
	whats("plain", 0)
	found := false
	for _, h := range a.hints() {
		found = found || h.what == "duplicate" && h.key == "⌥⇧D"
	}
	if !found {
		t.Fatalf("the rebound key is not shown: %+v", a.hints())
	}
	// zoomed: zooming out comes first
	a.zoomIn(strings.Index(a.doc.Text().String(), "one"))
	tt.Frame()
	if h := a.hints(); len(h) == 0 || h[0].what != "zoomOut2" {
		t.Fatalf("while zoomed: %+v", h)
	}
}

// A heading whose section is the whole page offers no zoom: zoomIn does nothing there (#270).
func TestHintsNoZoomIntoTheWholePage(t *testing.T) {
	a, tt := newTestApp(t, "# Only\n\ntext under it\n")
	tt.Advance(100 * time.Millisecond)
	a.refocus = true
	a.doc.SetSelection(1, 1)
	tt.Frame()
	for _, h := range a.hints() {
		if h.what == "zoom into the section" {
			t.Fatalf("a zoom into the whole page: %+v", a.hints())
		}
	}
	if h := a.hints(); len(h) == 0 || h[0].what != "fold" {
		t.Fatalf("the heading's: %+v", h)
	}
}
