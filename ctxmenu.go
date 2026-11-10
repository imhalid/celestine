package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// The context menu, the one for the whole app: a right click on a row of the tree, a page in a list or on a board,
// a block's margin, a field, or a page in the trash opens it where the pointer is, and the "…" of the bar and a few
// buttons open it where they are. It is MyGo's menu drawn in the window (popmenu.go), from an anchor at that point:
// its keys (↑↓ ← → ↵ Esc, the first letters of a line), its submenus (a line made by into, with ›) and its placement
// are MyGo's. Its lines are the ones that thing offers.

type ctxMenu struct {
	open  bool
	x, y  float32
	items []menuItem // what the menu offers; nil when none is open
}

// showContext opens the menu at x, y in the window.
func (a *app) showContext(x, y float32, items []menuItem) {
	if phone { // a phone has one pane: what opens beside is not offered
		kept := items[:0:0]
		for _, it := range items {
			if it.title != "Yan bölmede aç" {
				kept = append(kept, it)
			}
		}
		items = kept
	}
	a.ctx = ctxMenu{open: true, x: x, y: y, items: items}
	if a.frame != nil { // asked for after the menu's anchor was built in this frame: it opens in the next
		a.frame()
	}
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

// into is a line that opens another list beside the menu: a submenu.
func (a *app) into(title string, glyph *ui.SVG, items func() []menuItem) menuItem {
	return menuItem{title: title, glyph: glyph, items: items}
}

// contextView draws the menu, over the window, while it is open, and in the frame after a line of it was chosen,
// when the line runs. A line that opens another menu (showContext) leaves it open with the new lines.
func (a *app) contextView(c *ui.Context) {
	m := &a.ctx
	if m.items == nil {
		return
	}
	items := m.items
	menuAt(c, "ctx-at", "Bağlam menüsü", ui.Rect{X: m.x, Y: m.y}, &m.open, items)
	if !m.open {
		m.items = nil
	}
}

// ---- what each thing offers ----

// pageMenu is what a page offers, the same wherever it is opened: its row in the tree, a list, a table, a board, its
// title and the "…" of the bar (#267). inPlace is a row that turns into a field to rename the page; elsewhere the
// page is opened and the caret put in its title.
func (a *app) pageMenu(p *Page, inPlace bool) []menuItem {
	out := []menuItem{{title: "Aç", glyph: iconPage, run: func() { a.open(p) }}, {title: "Yan bölmede aç", glyph: iconPanel, run: func() { a.openInSide(place{"page", p.Dir}) }}}
	if !p.Coll.Journal {
		rename := func() { a.open(p); a.fresh = true }
		if inPlace {
			rename = func() { a.startRename(p, p.Title) }
		}
		out = append(out, menuItem{title: "Yeniden adlandır", glyph: iconEdit, run: rename})
	}
	if p.Coll.SubPages {
		out = append(out, menuItem{title: "Alt sayfa ekle", glyph: iconPlus, run: func() { a.newPage(p.Coll, p) }})
	}
	coverTitle := "Kapak resmi ekle…"
	if p.ImageCover() {
		coverTitle = "Kapak resmini değiştir…"
	}
	out = append(out, menuItem{title: coverTitle, group: "~", glyph: iconCover, run: func() { a.pickCover(p) }})
	if a.coverKind(p) != coverKindNone {
		out = append(out, menuItem{title: "Kapağı kaldır", glyph: iconTrash, run: func() { a.removeCover(p) }})
	}
	if p.Cover == CoverNone && !a.settings.NoAutoCover {
		out = append(out, menuItem{title: "Otomatik kapağı geri getir", glyph: iconCover, run: func() { a.restoreAutoCover(p) }})
	}
	if !p.Coll.Journal {
		out = append(out, menuItem{title: "Çoğalt", glyph: iconDuplicate, run: func() { a.duplicatePage(p) }})
	}
	if a.settings.SyncMode == "server" && a.settings.SyncToken != "" && !p.Coll.Journal {
		out = append(out, menuItem{title: "Sunucudaki sürümler…", glyph: iconHistory, run: func() { a.openServerHistory(p) }})
	}
	out = append(out,
		menuItem{title: "Bağlantıyı kopyala", sub: "[[" + p.Title + "]]", group: "~", glyph: iconLink, run: func() { a.copyText("[[" + p.Title + "]]") }},
		menuItem{title: "Markdown'u kopyala", glyph: iconPage, run: func() { a.copyText(a.currentMarkdown(p)) }})
	if _, ok := historyOf(p); ok { // the history is the page's room: the page is opened, then its history (#265)
		out = append(out, menuItem{title: "Sürüm geçmişi…", glyph: iconHistory, run: func() { a.open(p); a.openTimeMachine(p) }})
	}
	out = append(out,
		menuItem{title: "Dosya olarak dışa aktar…", glyph: iconExport, run: func() { a.exportPage(p) }},
		menuItem{title: "Çöpe taşı", group: "~", glyph: iconTrash, danger: true, run: func() { a.trashPage(p) }})
	return out
}

// collectionMenu is what a collection offers.
func (a *app) collectionMenu(coll *Collection) []menuItem {
	out := []menuItem{
		{title: "Aç", glyph: iconStack, run: func() { a.openCollection(coll) }},
		{title: "Yan bölmede aç", glyph: iconPanel, run: func() { a.openInSide(place{"coll", coll.Dir}) }},
		{title: "Yeniden adlandır", glyph: iconEdit, run: func() { a.startRename(coll, coll.Name) }},
		{title: newPageLabel(coll), glyph: iconPlus, run: func() { a.newPage(coll, nil) }},
		{title: "Ayarlar…", glyph: iconSettings, run: func() { a.openCollSettings(coll, 0) }},
		{title: map[bool]string{true: T("Change banner…"), false: T("Add banner…")}[coll.Banner != ""], glyph: iconCover, run: func() { a.pickBanner(coll) }},
		{title: map[bool]string{true: "Sabitlemeyi kaldır", false: "Sabitle"}[coll.Pinned], group: "~", glyph: iconPin, run: func() { a.pinCollection(coll, !coll.Pinned) }},
		{title: "Yukarı taşı", glyph: iconMoveUp, run: func() { a.moveCollection(coll, -1) }},
		{title: "Aşağı taşı", glyph: iconMoveDown, run: func() { a.moveCollection(coll, 1) }},
	}
	if !coll.Journal && len(coll.AllTemplates()) > 0 { // pages from its templates
		out = slices.Insert(out, 4, a.into("Şablondan yeni…", iconTemplate, func() []menuItem { return a.templateMenu(coll, nil) }))
	}
	if coll.Banner != "" {
		out = append(out, menuItem{title: T("Remove banner"), glyph: iconCover, run: func() { a.removeBanner(coll) }})
	}
	if !coll.Journal { // the journal is not thrown away
		out = append(out, menuItem{title: "Çöpe taşı", group: "~", glyph: iconTrash, danger: true, run: func() { a.trashCollection(coll) }})
	}
	return out
}

// trashMenu is what a page in the trash offers.
func (a *app) trashMenu(item *TrashItem) []menuItem {
	return []menuItem{
		{title: "Geri yükle", glyph: iconRestore, run: func() { a.restore(item) }},
		{title: "Kalıcı olarak sil", group: "~", glyph: iconTrash, danger: true, run: func() {
			a.deleteForever(item)
		}},
	}
}

var iconPin = icon(`<path d="M12 16.5V21"/><path d="M9.2 10.3a2 2 0 0 1-1.1 1.8l-1.7.9a2 2 0 0 0-1.1 1.8v.2a1 1 0 0 0 1 1h11.4a1 1 0 0 0 1-1v-.2a2 2 0 0 0-1.1-1.8l-1.7-.9a2 2 0 0 1-1.1-1.8V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H9a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>`)

// pinCollection pins a collection to the top of the sidebar, or lets it go.
func (a *app) pinCollection(c *Collection, pin bool) {
	if err := a.lib.PinCollection(c, pin); err != nil {
		a.fail(err)
	}
}

// moveCollection moves a collection up or down in the sidebar.
func (a *app) moveCollection(c *Collection, delta int) {
	if err := a.lib.MoveCollection(c, delta); err != nil {
		a.fail(err)
	}
}
