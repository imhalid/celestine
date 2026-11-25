package main

import (
	"slices"
	"testing"
	"time"
)

// A right click on the title opens the app's menu, not the editor's: while the title is being edited, its text to
// copy or paste, then what the page offers; Tümünü seç and Yapıştır work on the title.
func TestTitleRightClickMenu(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	tt.SetSize(1000, 700)
	tt.Advance(300 * time.Millisecond)
	var copied string
	a.clipText = func(s string) { copied = s }
	a.clipRead = func() string { return "Yeni\nad" }
	open := func() []string {
		t.Helper()
		tt.CloseMenu()
		r, ok := tt.Find("Başlık")
		if !ok {
			t.Fatalf("no title: %v", tt.Texts())
		}
		tt.RightClickAt(r.X+20, r.Y+r.H/2)
		tt.Frame()
		if tt.Menu() == nil {
			t.Fatalf("no menu on the title at %v", r)
		}
		return tt.Menu()
	}
	run := func(label string) {
		t.Helper()
		if err := tt.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	r, _ := tt.Find(msg("common_title"))
	tt.ClickAt(r.X+20, r.Y+r.H/2) // into the title: it is being edited
	tt.Frame()
	items := open()
	for _, id := range []string{"common_paste", "common_selectAllText", "moveToTrash"} {
		if !slices.Contains(items, msg(id)) {
			t.Fatalf("no %q in %v", msg(id), items)
		}
	}
	run("Tümünü seç")
	open()
	run("Kopyala")
	if copied != a.pane().title {
		t.Fatalf("copied %q, the title %q", copied, a.pane().title)
	}
	open()
	run("Yapıştır") // over the whole title, selected
	if a.pane().title != "Yeni ad" || a.pane().page.Title != "Yeni ad" {
		t.Fatalf("pasted: %q %q", a.pane().title, a.pane().page.Title)
	}
}

// The title's menu is the page's, the same as its row's in the sidebar, and has no text lines while the title is not
// being edited (#267).
func TestTitleMenuIsThePageMenu(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	tt.SetSize(1000, 700)
	tt.Advance(300 * time.Millisecond)
	rightClick := func(label string) []string {
		t.Helper()
		tt.CloseMenu()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q: %v", label, tt.Texts())
		}
		tt.RightClickAt(r.X+20, r.Y+r.H/2)
		tt.Frame()
		if tt.Menu() == nil {
			t.Fatalf("no menu on %q", label)
		}
		return tt.Menu()
	}
	a.pane().refocus = true // the caret in the text, not the title
	tt.Frame()
	title := rightClick(msg("common_title"))
	tt.CloseMenu()
	tt.Frame()
	if err := tt.RightClick("Sayfa: Sayfa"); err != nil || tt.Menu() == nil {
		t.Fatalf("no menu on the row: %v", err)
	}
	row := tt.Menu()
	if !slices.Equal(title, row) {
		t.Fatalf("the title's menu %v, the row's %v", title, row)
	}
	if slices.Contains(title, msg("common_paste")) || !slices.Contains(title, msg("common_rename")) {
		t.Fatalf("the title's menu: %v", title)
	}
}
