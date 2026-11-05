package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The grip of a block (⋮⋮): it shows left of the paragraph the pointer is
// on. A click opens the block's menu - turn it into another kind, duplicate,
// move, delete; a drag lifts the block, a copy of its first line follows
// the pointer, a line shows where it would go, and let go it moves there,
// what is under it with it.

type gripState struct {
	para     int     // the start of the paragraph the grip is for, -1 for none
	top, h   float32 // its line in the editor
	menu     bool
	view     string // "turn" in the menu's list of kinds, "" the first
	sel      int
	seen     int
	hseen    int // the row last hovered
	scroll   ui.ScrollState
	pressed  bool
	moved    float32
	dragging bool
	from     [2]int // the block dragged
	to       int    // where it would go: the paragraph it goes before, or the text's length+1
	toY      float32
	pos      ui.Point // the pointer, in window DIPs
	hover    bool     // the pointer is on the grip
}

// blockKinds are what a block can be turned into, in the menus.
var blockKinds = []struct{ kind, name, md string }{
	{"text", "Metin", ""},
	{"h1", "Başlık 1", "#"},
	{"h2", "Başlık 2", "##"},
	{"h3", "Başlık 3", "###"},
	{"bullets", "Madde listesi", "-"},
	{"numbers", "Numaralı liste", "1."},
	{"tasks", "Görev", "[]"},
	{"quote", "Alıntı", ">"},
	{"code", "Kod bloğu", "```"},
	{"hr", "Ayırıcı", "---"},
}

// paraAtY is the paragraph box at y in the editor, or false.
func (a *app) paraAtY(y float32) (richtext.ParagraphBox, bool) {
	for i, b := range a.boxes {
		bottom := b.Top + b.Height
		if i+1 < len(a.boxes) {
			bottom = a.boxes[i+1].Top
		}
		if y >= b.Top-6 && y < bottom {
			return b, true
		}
	}
	return richtext.ParagraphBox{}, false
}

// trackGrip follows the pointer over the editor to put the grip by its paragraph.
func (a *app) trackGrip(ed ui.Element) {
	g := &a.grip
	if g.dragging || g.menu || g.pressed {
		return
	}
	x, y, over := ed.PointerPosition()
	switch {
	case over && x >= 0 && x < a.editorR.W+10:
		if b, ok := a.paraAtY(y); ok {
			g.para, g.top, g.h = b.Start, b.Top, b.Height
			return
		}
	case x >= -40 && x < 0 && g.para >= 0 && y >= g.top-6 && y < g.top+g.h+6:
		return // on the way to the grip, left of the text
	}
	if !g.hover {
		g.para = -1
	}
}

// gutterW is the strip of the page's margin left of the text, where the grip
// is: the pointer there is over the editor too.
const gutterW = 32

// gutterView is the page's margin left of the text, over the window and
// invisible: from the page area's edge to the text, it is the editor's. The
// pointer in it puts the grip by the paragraph it is level with, and a
// click puts the caret at the start of that paragraph.
func (a *app) gutterView(c *ui.Context) {
	g := &a.grip
	if a.doc == nil || a.noPage() || a.editorR.W == 0 || g.dragging || g.menu || g.pressed || a.modal() {
		return
	}
	r := a.editorR
	x0 := r.X - gutterW
	if a.pageR.W > 0 && a.pageR.X < x0 {
		x0 = a.pageR.X
	}
	// only where the page shows: the editor is as tall as its text
	top, bottom := r.Y, r.Y+r.H
	if a.scrollR.H > 0 {
		top, bottom = max(top, a.scrollR.Y), min(bottom, a.scrollR.Y+a.scrollR.H)
	}
	if bottom <= top {
		return
	}
	ui.Overlay(c, func() {
		gut := ui.Box(c.Key("gutter")).Absolute().Left(x0).Top(top).Size(r.X-x0, bottom-top).Cursor(ui.CursorText)
		if _, y, over := gut.PointerPosition(); over {
			if b, ok := a.paraAtY(y + top - r.Y); ok {
				g.para, g.top, g.h = b.Start, b.Top, b.Height
				if gut.Clicked() {
					a.doc.SetSelection(b.Start, b.Start)
					a.refocus = true
				}
				if gut.RightClicked() { // the block's menu, as from its grip
					g.menu, g.view, g.sel, g.seen, g.scroll = true, "", 0, -1, ui.ScrollState{}
				}
			}
		}
	})
}

// gripView is the grip and its menu, over the window.
func (a *app) gripView(c *ui.Context) {
	g := &a.grip
	if a.doc == nil || a.noPage() || g.para < 0 || g.para > a.doc.Text().Len() {
		g.hover = false
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	r := a.editorR
	// the first line of the paragraph: a heading's is taller
	line := min(g.h, a.font.Size*a.font.LineHeight*1.6)
	ui.Overlay(c, func() {
		grip := ui.Box(c.Key("grip")).Absolute().Left(r.X-30).Top(r.Y+g.top+(line-24)/2).Size(22, 24).Radius(6).
			AlignItems(ui.Center).Justify(ui.Center).Label("Blok tutamağı").Role(ui.RoleButton).Cursor(ui.CursorGrab)
		g.hover = grip.Hovered()
		if g.hover || g.menu {
			grip.Background(cl.Fill)
		}
		grip.Children(func() { gripDots(c, cl.Muted) })
		// a block with lines under it, or folded: a chevron to fold it, if there is room
		if t := a.doc.Text(); r.X-52 >= a.pageR.X && g.para <= t.Len() && (blockHasKids(t, g.para) || foldOf(t, g.para) != nil) {
			folded := foldOf(t, g.para) != nil
			ch := ui.Box(c.Key("fold-chevron")).Absolute().Left(r.X-52).Top(r.Y+g.top+(line-24)/2).Size(20, 24).Radius(6).
				AlignItems(ui.Center).Justify(ui.Center).Label(map[bool]string{true: "Aç", false: "Katla"}[folded]).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if ch.Hovered() {
				ch.Background(cl.Fill)
			}
			ch.Children(func() {
				i := ui.Icon(c, iconFoldOpen).Width(16).Height(16).TextColor(cl.Muted)
				if folded {
					i.Rotate(-90)
				}
			})
			g.hover = g.hover || ch.Hovered()
			if ch.Clicked() {
				a.toggleFold(g.para)
			}
		}
		a.dragGrip(c, grip)
		if (grip.Clicked() && !g.dragging && g.moved < 4) || grip.RightClicked() {
			g.menu, g.view, g.sel, g.seen, g.scroll = true, "", 0, -1, ui.ScrollState{}
		}
		if g.menu {
			a.blockMenu(c, grip.LayoutBounds())
		}
		if g.dragging {
			a.blockGhost(c)
		}
	})
}

// turnBlock turns the paragraph starting at at into a kind of block; a
// divider is an empty paragraph of its own (blocks.go).
func (a *app) turnBlock(at int, kind string) {
	if kind != "hr" {
		turnInto(a.doc, at, kind)
		return
	}
	t := a.doc.Text()
	s, e := t.ParagraphRange(at)
	a.doc.Edit(func(ed *richtext.Edit) {
		if s == e { // empty: it becomes the divider
			ed.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
			return
		}
		ed.Replace(e, e, richtext.NewText("\n"))
		ed.FormatParagraphs(e+1, e+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
	})
}

// dragGrip lifts the block by its grip and moves it where it is let go.
func (a *app) dragGrip(c *ui.Context, grip ui.Element) {
	g := &a.grip
	dx, dy, pressed := grip.Dragged()
	switch {
	case pressed && !g.dragging:
		if !g.pressed {
			g.pressed, g.moved = true, 0
		}
		if g.moved += abs32(dx) + abs32(dy); g.moved > 4 {
			s, e := blockRange(a.doc.Text(), g.para)
			g.dragging, g.from, g.to = true, [2]int{s, e}, -1
		}
	case pressed && g.dragging:
		b := grip.LayoutBounds()
		px, py, _ := grip.PointerPosition()
		g.pos = ui.Point{X: b.X + px, Y: b.Y + py}
		a.aimBlock(g.pos.Y - a.editorR.Y)
	case !pressed && g.dragging:
		if g.to >= 0 {
			moveBlockTo(a.doc, g.from[0], g.from[1], g.to)
		}
		g.dragging, g.pressed, g.para = false, false, -1
		c.AnimationFrame()
	case !pressed:
		g.pressed = false
	}
}

// aimBlock works out where the block dragged would go, from y in the
// editor: before the paragraph whose middle is below, outside the block.
func (a *app) aimBlock(y float32) {
	g := &a.grip
	t := a.doc.Text()
	g.to = -1
	for _, b := range a.boxes {
		if b.Start >= g.from[0] && b.Start <= g.from[1] {
			continue
		}
		if y < b.Top+b.Height/2 {
			if b.Start == g.from[1]+1 { // right after itself: where it is
				return
			}
			g.to, g.toY = b.Start, b.Top-3
			return
		}
	}
	if n := len(a.boxes); n > 0 && g.from[1] < t.Len() {
		last := a.boxes[n-1]
		if _, e := t.ParagraphRange(last.Start); e == t.Len() {
			g.to, g.toY = t.Len()+1, last.Top+last.Height+3
		}
	}
}

// drawBlockDrop draws where the block dragged would go: a line across the
// editor with a dot at its start, as the tree's.
func (a *app) drawBlockDrop(p *ui.Painter, r ui.Rect) {
	g := &a.grip
	if !g.dragging || g.to < 0 {
		return
	}
	th := p.Theme()
	y := r.Y + g.toY
	p.Fill(ui.Rect{X: r.X, Y: y - 1, W: r.W, H: 2}, th.Accent, 1)
	p.StrokePath((&ui.Path{}).Circle(r.X-3, y, 3), 1.5, th.Accent)
}

// blockGhost is the block's first line, lifted, following the pointer.
func (a *app) blockGhost(c *ui.Context) {
	g := &a.grip
	t := ryugo.From(c)
	cl := t.Colors
	s, e := a.doc.Text().ParagraphRange(g.from[0])
	first := a.doc.Text().Slice(s, e).String()
	if first == "" {
		first = "…"
	}
	gh := ui.Row(c.Key("block-ghost")).Absolute().Left(g.pos.X-12).Top(g.pos.Y-16).MaxWidth(420).Padding(6, 12).Gap(8).
		Radius(10).Background(cl.Card).Border(1, cl.Hairline).PassThrough().AlignItems(ui.Center)
	t.Shadows.ShadowPop.Apply(gh)
	gh.Rotate(gh.AnimateTo("tilt", -1.2, springTilt)).Scale(1.02, 1.02)
	gh.Children(func() {
		gripDots(c, cl.Muted)
		ui.Text(c, first).FontSize(14).TextColor(cl.Text).MaxLines(1)
		if g.from[1] > e {
			ui.Text(c, "+ altındakiler").FontSize(12).TextColor(cl.Muted)
		}
	})
}

// iconGrip is the grip's six dots, two columns of three, drawn as an icon.
var iconGrip = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` +
	`<circle cx="8.5" cy="5" r="2.1"/><circle cx="15.5" cy="5" r="2.1"/>` +
	`<circle cx="8.5" cy="12" r="2.1"/><circle cx="15.5" cy="12" r="2.1"/>` +
	`<circle cx="8.5" cy="19" r="2.1"/><circle cx="15.5" cy="19" r="2.1"/></svg>`))

// gripDots are the grip's dots.
func gripDots(c *ui.Context, color ui.Color) {
	ui.Icon(c, iconGrip).Width(22).Height(22).TextColor(color)
}

var (
	iconDuplicate = icon(`<rect x="8.5" y="8.5" width="11" height="11" rx="2.5"/><path d="M15.5 8.5v-2a2 2 0 0 0-2-2h-7a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h2"/>`)
	iconMoveUp    = icon(`<path d="M12 19V5M6 11l6-6 6 6"/>`)
	iconMoveDown  = icon(`<path d="M12 5v14M6 13l6 6 6-6"/>`)
)

// blockItems are the lines of the block's menu.
func (a *app) blockItems() []menuItem {
	g := &a.grip
	at := g.para
	done := func(fn func()) func() { return func() { fn() } }
	if g.view == "turn" {
		var out []menuItem
		for _, k := range blockKinds {
			k := k
			out = append(out, menuItem{title: k.name, glyph: kindIcon[k.kind], run: done(func() { a.turnBlock(at, k.kind) })})
		}
		return out
	}
	var fold []menuItem
	if ch := foldOf(a.doc.Text(), at); ch != nil {
		fold = []menuItem{{title: "Aç (katlı)", hint: "⌘/", glyph: iconFoldOpen, run: done(func() { a.unfold(ch) })}}
	} else if blockHasKids(a.doc.Text(), at) {
		fold = []menuItem{{title: "Katla", hint: "⌘/", glyph: iconFold, run: done(func() { a.fold(at) })}}
	}
	return append(fold, []menuItem{
		{title: "Dönüştür…", glyph: iconFormat, nav: true, run: func() { g.view, g.sel, g.seen, g.scroll = "turn", 0, -1, ui.ScrollState{} }},
		{title: "Çoğalt", hint: "⌘D", glyph: iconDuplicate, run: done(func() { duplicateBlock(a.doc, at) })},
		{title: "Yukarı taşı", hint: "⌘⇧↑", glyph: iconMoveUp, run: done(func() { a.doc.SetSelection(at, at); moveBlock(a.doc, -1) })},
		{title: "Aşağı taşı", hint: "⌘⇧↓", glyph: iconMoveDown, run: done(func() { a.doc.SetSelection(at, at); moveBlock(a.doc, 1) })},
		{title: "Sil", group: "~", glyph: iconTrash, danger: true, run: done(func() { deleteBlock(a.doc, at) })},
	}...)
}

// blockMenu is the menu of a block, in the / menu's design, under its grip
// (the span at): ↑↓ move, ↵ chooses, Esc goes back and then closes.
func (a *app) blockMenu(c *ui.Context, at ui.Rect) {
	g := &a.grip
	t := ryugo.From(c)
	items := a.blockItems()
	g.sel = max(0, min(g.sel, len(items)-1))
	head := float32(0) // the first menu has no header
	if g.view != "" {
		head = menuTop + 1 + menuAfter
	}
	pl := placeMenu(c, at.X, at.Y, at.Y+at.H, items, 0, head)
	keepInSight(g.sel, &g.seen, &g.scroll, pl)
	choose := func(it menuItem) {
		it.run()
		if !it.nav {
			g.menu = false
		}
	}
	back := func() { g.view, g.sel, g.seen, g.scroll = "", 0, -1, ui.ScrollState{} }
	ww, wh := c.Size()
	if ui.Box(c.Key("block-away")).Absolute().Left(0).Top(0).Size(ww, wh).Clicked() {
		g.menu = false
		return
	}
	m := &menuPanel{key: "block", label: "Blok menüsü", pl: pl, items: items, sel: g.sel, selPtr: &g.sel, hoverSeen: &g.hseen, scroll: &g.scroll, choose: choose, empty: "Boş"}
	if g.view != "" {
		m.head = func() { menuBack(c, t, "Dönüştür", back) }
	}
	panel := a.drawMenu(c, t, m).Focusable().Focus()
	switch {
	case panel.Shortcut(0, ui.KeyEscape):
		if g.view != "" {
			back()
		} else {
			g.menu = false
		}
	case panel.Shortcut(0, ui.KeyDown):
		g.sel = min(g.sel+1, len(items)-1)
	case panel.Shortcut(0, ui.KeyUp):
		g.sel = max(g.sel-1, 0)
	case panel.Shortcut(0, ui.KeyEnter) && len(items) > 0:
		choose(items[g.sel])
	}
}

// modal reports whether a dialog is open over the window: what is under it is
// not pointed at (no grip, no bar) while it is.
func (a *app) modal() bool {
	return a.settingsOpen || a.cs.open || a.prop.open || a.pal.open
}
