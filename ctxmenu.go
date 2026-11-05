package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The context menu, the one for the whole app: a right click on a row of
// the tree, a page in a list or on a board, a block's grip or the margin by
// its line, or a page in the trash opens it where the pointer is, in the
// design of the / menu (menupanel.go). (A right click on the text itself
// does not reach the app: MyGo's editor takes it for its own menu, which has
// nothing in it.) Its lines are the ones that thing offers; one that
// goes into another list (›) shows it with ← Geri over it. ↑↓ ↵ Esc, or a
// click out of it.

type ctxMenu struct {
	open   bool
	x, y   float32
	title  string
	stack  [][]menuItem // the lists gone through; the last is shown
	sel    int
	seen   int
	scroll ui.ScrollState
	fresh  bool // opened in this frame: the click that opened it is not one that closes it
}

// showContext opens the menu at x, y in the window.
func (a *app) showContext(x, y float32, items []menuItem) {
	a.ctx = ctxMenu{open: true, x: x, y: y, stack: [][]menuItem{items}, seen: -1, fresh: true}
}

// rightClick reports a click of the secondary button on el, where in the
// window it was.
func rightClick(el ui.Element) (x, y float32, ok bool) {
	if !el.RightClicked() {
		return 0, 0, false
	}
	px, py, _ := el.PointerPosition()
	b := el.LayoutBounds()
	return b.X + px, b.Y + py, true
}

// contextOn opens the menu of el when it was right-clicked, built by menu.
func (a *app) contextOn(el ui.Element, menu func() []menuItem) {
	if x, y, ok := rightClick(el); ok {
		if items := menu(); len(items) > 0 {
			a.showContext(x, y, items)
		}
	}
}

// into is a line that shows another list in the menu.
func (a *app) into(title string, glyph *ui.SVG, items func() []menuItem) menuItem {
	return menuItem{title: title, glyph: glyph, nav: true, run: func() {
		a.ctx.stack = append(a.ctx.stack, items())
		a.ctx.sel, a.ctx.seen, a.ctx.scroll = 0, -1, ui.ScrollState{}
	}}
}

// contextView draws the menu, over the window, while it is open.
func (a *app) contextView(c *ui.Context) {
	m := &a.ctx
	if !m.open {
		return
	}
	t := ryugo.From(c)
	items := m.stack[len(m.stack)-1]
	m.sel = max(0, min(m.sel, len(items)-1))
	head := float32(0)
	if len(m.stack) > 1 {
		head = menuTop + 1 + menuAfter
	}
	pl := placeMenu(c, m.x, m.y, m.y, items, 0, head)
	keepInSight(m.sel, &m.seen, &m.scroll, pl)
	choose := func(it menuItem) {
		m.open = false
		it.run()
		if it.nav && a.ctx.stack != nil {
			m.open = true
		}
	}
	back := func() {
		m.stack = m.stack[:len(m.stack)-1]
		m.sel, m.seen, m.scroll = 0, -1, ui.ScrollState{}
	}
	ww, wh := c.Size()
	mp := &menuPanel{key: "ctx", label: "Bağlam menüsü", pl: pl, items: items, sel: m.sel, scroll: &m.scroll, choose: choose, empty: "Boş"}
	if len(m.stack) > 1 {
		mp.head = func() { menuBack(c, t, "", back) }
	}
	ui.Overlay(c, func() {
		away := ui.Box(c.Key("ctx-away")).Absolute().Left(0).Top(0).Size(ww, wh)
		if m.fresh {
			m.fresh = false
		} else if away.Clicked() || away.RightClicked() {
			m.open = false
			return
		}
		panel := a.drawMenu(c, t, mp).Focusable().Focus()
		switch {
		case panel.Shortcut(0, ui.KeyEscape):
			if len(m.stack) > 1 {
				back()
			} else {
				m.open = false
			}
		case panel.Shortcut(0, ui.KeyDown):
			m.sel = min(m.sel+1, len(items)-1)
		case panel.Shortcut(0, ui.KeyUp):
			m.sel = max(m.sel-1, 0)
		case panel.Shortcut(0, ui.KeyEnter) && len(items) > 0:
			choose(items[m.sel])
		}
	})
}

// ---- what each thing offers ----

// pageMenu is what a page offers where it is listed: in the tree, a list,
// a table, a board.
func (a *app) pageMenu(p *Page, rename bool) []menuItem {
	out := []menuItem{{title: "Aç", glyph: iconPage, run: func() { a.open(p) }}}
	if rename {
		out = append(out, menuItem{title: "Yeniden adlandır", glyph: iconEdit, run: func() { a.startRename(p, p.Title) }})
	}
	if p.Coll.SubPages {
		out = append(out, menuItem{title: "Alt sayfa ekle", glyph: iconPlus, run: func() { a.newPage(p.Coll, p) }})
	}
	out = append(out, menuItem{title: "Çöpe taşı", group: "~", glyph: iconTrash, danger: true, run: func() { a.trashPage(p) }})
	return out
}

// collectionMenu is what a collection offers.
func (a *app) collectionMenu(coll *Collection) []menuItem {
	return []menuItem{
		{title: "Aç", glyph: iconStack, run: func() { a.openCollection(coll) }},
		{title: "Yeniden adlandır", glyph: iconEdit, run: func() { a.startRename(coll, coll.Name) }},
		{title: "Yeni " + pageNoun(coll), glyph: iconPlus, run: func() { a.newPage(coll, nil) }},
		{title: "Ayarlar…", glyph: iconSettings, run: func() { a.openCollSettings(coll, 0) }},
		{title: "Çöpe taşı", group: "~", glyph: iconTrash, danger: true, run: func() { a.trashCollection(coll) }},
	}
}

// trashMenu is what a page in the trash offers.
func (a *app) trashMenu(item *TrashItem) []menuItem {
	return []menuItem{
		{title: "Geri yükle", glyph: iconRestore, run: func() { a.restore(item) }},
		{title: "Kalıcı olarak sil", group: "~", glyph: iconTrash, danger: true, run: func() {
			if err := a.lib.DeleteForever(item); err != nil {
				a.saveErr = err.Error()
			}
			a.loadTrash()
		}},
	}
}
