package main

import (
	"sort"
	"strings"
	"testing"
)

// In English, every line the palette shows in each of its modes, the slash menu and the menus of a page and a
// collection is English: none is a Turkish word the catalogs do not translate, but the library's own names.
func TestPaletteInEnglish(t *testing.T) {
	a, tt := newTestApp(t, "# Başlık\nmetin #etiket\n- [ ] görev\n")
	withLang(t, "en")
	tt.Frame()
	seen := map[string]bool{}
	var out []string
	add := func(where, s string) {
		if s != "" && turkish(msg(s)) && !seen[s] && !strings.Contains(s, a.pane().page.Title) && !strings.Contains(s, a.pane().page.Coll.Name) {
			seen[s] = true
			out = append(out, where+"\t"+msg(s))
		}
	}
	for _, q := range []string{">", "@", "#", ":", "!", "?", "/", ">tema", ">yazı", ">set", ":set fontsize=18", ":e x", ":tema mor", ":zz", "#zz", ">dil", ">caret", ">profil"} {
		for _, it := range a.paletteItems(q) {
			add("palette "+q, it.title)
			add("palette sub "+q, it.sub)
			add("palette off "+q, it.off)
			add("palette group "+q, it.group)
		}
	}
	var walk func(where string, items []menuItem)
	walk = func(where string, items []menuItem) {
		for _, it := range items {
			add(where, it.title)
			add(where+" hint", it.hint)
			add(where+" sub", it.sub)
			add(where+" group", it.group)
			if it.items != nil {
				walk(where+">"+msg(it.title), it.items())
			}
		}
	}
	walk("slash", a.menuItems(""))
	if a.pane().page != nil {
		walk("page", a.pageMenu(a.pane().page, true))
		walk("coll", a.collectionMenu(a.pane().page.Coll))
	}
	sort.Strings(out)
	if len(out) > 0 {
		t.Fatalf("Turkish in the English palette and menus:\n%s", strings.Join(out, "\n"))
	}
}
