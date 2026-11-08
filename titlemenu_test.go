package main

import (
	"testing"
	"time"
)

// A right click on the title opens the app's menu, not the editor's: its text to copy or paste, then
// what the page offers; Tümünü seç and Yapıştır work on the title.
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
		if tt.Menu() != nil {
			t.Fatal("the editor's own menu opened too")
		}
		return a.ctx.stack[0]
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
	items := open()
	has := map[string]bool{}
	for _, it := range items {
		has[it.title] = true
	}
	if !has["Yapıştır"] || !has["Tümünü seç"] || !has["Çöpe taşı"] || has["Yeniden adlandır"] {
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
