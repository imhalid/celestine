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
	open := func() []menuItem {
		r, ok := tt.Find("Başlık")
		if !ok {
			t.Fatalf("no title: %v", tt.Texts())
		}
		tt.RightClickAt(r.X+20, r.Y+r.H/2)
		tt.Frame()
		if !a.ctx.open {
			t.Fatalf("no menu on the title at %v; platform menu %v", r, tt.Menu() != nil)
		}
		if m := tt.Menu(); len(m) == 0 || m[0] != T(a.ctx.items[0].title) {
			t.Fatalf("the menu shown is not the app's: %v", m)
		}
		return a.ctx.items
	}
	run := func(items []menuItem, title string) {
		for _, it := range items {
			if it.title == title {
				a.ctx.open = false
				it.run()
				tt.Frame()
				return
			}
		}
		t.Fatalf("no %q in the menu", title)
	}
	r, _ := tt.Find("Başlık")
	tt.ClickAt(r.X+20, r.Y+r.H/2) // into the title: it is being edited
	tt.Frame()
	items := open()
	has := map[string]bool{}
	for _, it := range items {
		has[it.title] = true
	}
	if !has["Yapıştır"] || !has["Tümünü seç"] || !has["Çöpe taşı"] {
		t.Fatalf("items: %+v", items)
	}
	run(items, "Tümünü seç")
	run(open(), "Kopyala")
	if copied != a.title {
		t.Fatalf("copied %q, the title %q", copied, a.title)
	}
	run(open(), "Yapıştır") // over the whole title, selected
	if a.title != "Yeni ad" || a.page.Title != "Yeni ad" {
		t.Fatalf("pasted: %q %q", a.title, a.page.Title)
	}
}

// The title's menu is the page's, the same as its row's in the sidebar, and has no text lines while the title is not
// being edited (#267).
func TestTitleMenuIsThePageMenu(t *testing.T) {
	a, tt := newTestApp(t, "gövde")
	tt.SetSize(1000, 700)
	tt.Advance(300 * time.Millisecond)
	titles := func(items []menuItem) []string {
		var out []string
		for _, it := range items {
			out = append(out, it.title)
		}
		return out
	}
	rightClick := func(label string) []string {
		t.Helper()
		a.ctx.open = false
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q: %v", label, tt.Texts())
		}
		tt.RightClickAt(r.X+20, r.Y+r.H/2)
		tt.Frame()
		if !a.ctx.open {
			t.Fatalf("no menu on %q", label)
		}
		return titles(a.ctx.items)
	}
	a.refocus = true // the caret in the text, not the title
	tt.Frame()
	title := rightClick("Başlık")
	a.ctx.open = false
	tt.Frame()
	if err := tt.RightClick("Sayfa: Sayfa"); err != nil || !a.ctx.open {
		t.Fatalf("no menu on the row: %v", err)
	}
	row := titles(a.ctx.items)
	if !slices.Equal(title, row) {
		t.Fatalf("the title's menu %v, the row's %v", title, row)
	}
	if slices.Contains(title, "Yapıştır") || !slices.Contains(title, "Yeniden adlandır") {
		t.Fatalf("the title's menu: %v", title)
	}
}
