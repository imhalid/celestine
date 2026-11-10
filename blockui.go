package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The grip of a block (⋮⋮): it shows left of the paragraph the pointer is
// on. A click opens the block's menu - turn it into another kind, duplicate,
// move, delete; a drag lifts the block: in its place it fades and glows, a
// copy of its first line follows the pointer, and where it would go the
// paragraphs slide apart (the editor's Space) to open room for a preview
// of it, its first lines. Let go, it moves there, what is under it with it.

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
	toIdx    int             // the paragraph (from 0) the room opens above, -1 at the end of the text
	rooms    map[int]float32 // the room open above paragraphs, by index, as it springs open and shut
	roomH    float32         // the room a preview of the block dragged takes
	bg       ui.Color        // the page's background, which the block lifted fades into
	pos      ui.Point        // the pointer, in window DIPs
	hover    bool            // the pointer is on the grip
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

// blockHead is the paragraph the block holding the paragraph at start begins at: a block of
// code, or a quotation, is one block of many lines, and the grip of each line is that of the
// first.
func (a *app) blockHead(start int) int { return blockHeadOf(a.doc.Text(), start) }

// gripAt puts the grip by the block of paragraph box b: where its first line is.
func (a *app) gripAt(b richtext.ParagraphBox) {
	g := &a.grip
	head := a.blockHead(b.Start)
	g.para, g.top, g.h = b.Start, b.Top, b.Height
	if head == b.Start {
		return
	}
	for _, hb := range a.boxes {
		if hb.Start == head {
			g.para, g.top, g.h = hb.Start, hb.Top, hb.Height
			return
		}
	}
	g.para = head // its first line is out of view: it follows when it is back
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
			a.gripAt(b)
			return
		}
	case x >= -40 && x < 0 && g.para >= 0 && y >= g.top-6 && y < g.top+g.h+6:
		return // on the way to the grip, left of the text
	}
	if !g.hover {
		g.para = -1
	}
}

// The grip is as tall as a line of text, and small.
const (
	gripW = 16
	gripH = 20
)

// firstLineH is the height of the first line of the block at para: a line of
// its text (a heading's is taller), or the block when it is lower - a table, a
// picture - so that the grip is at its top.
func (a *app) firstLineH(para int, h float32) float32 {
	size := a.font.Size
	if st := a.doc.Text().ParagraphStyleAt(min(para, a.doc.Text().Len())); st.Heading > 0 && a.theme != nil {
		size *= a.theme.Headings[min(max(st.Heading, 1), 6)-1].Size
	}
	return min(h, size*a.font.LineHeight)
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
	if phone || a.doc == nil || a.noPage() || a.editorR.W == 0 || g.dragging || g.menu || g.pressed || a.modal() { // a finger has no pointer to show it under
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
		top, bottom = max(top, a.scrollR.Y+a.barInset), min(bottom, a.scrollR.Y+a.scrollR.H)
	}
	if bottom <= top {
		return
	}
	ui.Overlay(c, func() {
		gut := ui.Box(c.Key("gutter")).Absolute().Left(x0).Top(top).Size(r.X-x0, bottom-top).Cursor(ui.CursorText)
		if _, y, over := gut.PointerPosition(); over {
			if b, ok := a.paraAtY(y + top - r.Y); ok {
				a.gripAt(b)
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
	if phone || a.doc == nil || a.noPage() || g.para < 0 || g.para > a.doc.Text().Len() {
		g.hover = false
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	r := a.editorR
	// where the paragraph is now: the page may have scrolled or been edited since the pointer was over it
	found := false
	for _, b := range a.boxes {
		if b.Start == g.para {
			g.top, g.h, found = b.Top, b.Height, true
			break
		}
	}
	if !found {
		g.hover = false
		return
	}
	// the first line of the block: the grip is as tall as a line of text and level with it, at the block's top
	line := a.firstLineH(g.para, g.h)
	gy := r.Y + g.top + max(0, (line-gripH)/2)
	ui.Overlay(c, func() {
		grip := ui.Box(c.Key("grip")).Absolute().Left(r.X-gripW-8).Top(gy).Size(gripW, gripH).Radius(rad(5)).
			AlignItems(ui.Center).Justify(ui.Center).Label(T("Blok tutamağı")).Role(ui.RoleButton).Cursor(ui.CursorGrab)
		g.hover = grip.Hovered()
		if g.hover || g.menu {
			grip.Background(cl.Fill)
		}
		grip.Children(func() { gripDots(c, cl.Muted) })
		// a block with lines under it, or folded: a chevron to fold it, if there is room
		if t := a.doc.Text(); r.X-gripW*2-12 >= a.pageR.X && g.para <= t.Len() && (blockHasKids(t, g.para) || foldOf(t, g.para) != nil) {
			folded := foldOf(t, g.para) != nil
			ch := ui.Box(c.Key("fold-chevron")).Absolute().Left(r.X-gripW*2-10).Top(gy).Size(gripW, gripH).Radius(rad(5)).
				AlignItems(ui.Center).Justify(ui.Center).Label(map[bool]string{true: "Aç", false: "Katla"}[folded]).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if ch.Hovered() {
				ch.Background(cl.Fill)
			}
			ch.Children(func() {
				i := ui.Icon(c, iconFoldOpen).Width(14).Height(14).TextColor(cl.Muted)
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
			g.bg = a.bg(c)
			a.blockPreview(c)
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
			g.dragging, g.from, g.to, g.toIdx, g.rooms = true, [2]int{s, e}, -1, -1, map[int]float32{}
			// the preview is as tall as the block, up to three lines of it
			top, bottom, ok := a.blockSpan(s, e)
			if !ok {
				top, bottom = 0, a.firstLineH(s, 24)
			}
			g.roomH = min(bottom-top, 3*previewLineH) + 2*previewPad + previewGap
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
		// the block takes the room's place: it shuts at once, not after it
		g.dragging, g.pressed, g.para, g.rooms = false, false, -1, nil
		c.AnimationFrame()
	case !pressed:
		g.pressed = false
	}
}

// The preview of a block dragged, in the room opened for it: lines of 14
// DIP text, an inset well around them, and a gap to the line under it.
const (
	previewLineH = 20
	previewPad   = 8
	previewGap   = 6
)

// blockRoom is the room the editor leaves above paragraph p (its Space): the
// room a block dragged opens where it would go.
func (a *app) blockRoom(p int) float32 { return a.grip.rooms[p] }

// roomAbove is how far the rooms open push paragraph p down: those above it
// and its own.
func (a *app) roomAbove(p int) float32 {
	var d float32
	for i, h := range a.grip.rooms {
		if i <= p {
			d += h
		}
	}
	return d
}

// blockSpan is where the block from s to e is in the editor, its top and
// bottom, or false when none of it is in view.
func (a *app) blockSpan(s, e int) (top, bottom float32, ok bool) {
	for _, b := range a.boxes {
		if b.Start < s || b.Start > e {
			continue
		}
		if !ok {
			top, ok = b.Top, true
		}
		bottom = b.Top + b.Height
	}
	return top, bottom, ok
}

// aimBlock works out where the block dragged would go, from y in the
// editor: before the paragraph whose middle is below, outside the block.
// The paragraphs are taken where they would be with no room open, so that
// opening it does not move where the pointer aims.
func (a *app) aimBlock(y float32) {
	g := &a.grip
	t := a.doc.Text()
	g.to, g.toIdx = -1, -1
	for _, b := range a.boxes {
		if b.Start >= g.from[0] && b.Start <= g.from[1] {
			continue
		}
		if top := b.Top - a.roomAbove(b.Index); y < top+b.Height/2 {
			if b.Start == g.from[1]+1 { // right after itself: where it is
				return
			}
			g.to, g.toIdx = b.Start, b.Index
			return
		}
	}
	if n := len(a.boxes); n > 0 && g.from[1] < t.Len() {
		last := a.boxes[n-1]
		if _, e := t.ParagraphRange(last.Start); e == t.Len() {
			g.to, g.toY = t.Len()+1, last.Top+last.Height+previewGap
		}
	}
}

// openRooms springs the rooms open and shut, on el, a frame's element of
// the drag: the one where the block would go opens to its preview's height,
// the others close and are let go once shut.
func (a *app) openRooms(el ui.Element) {
	g := &a.grip
	if g.toIdx >= 0 {
		if _, ok := g.rooms[g.toIdx]; !ok {
			el.Jump("room"+strconv.Itoa(g.toIdx), 0)
			g.rooms[g.toIdx] = 0
		}
	}
	for i := range g.rooms {
		to := float32(0)
		if i == g.toIdx {
			to = g.roomH
		}
		h := el.AnimateTo("room"+strconv.Itoa(i), to, springShift)
		if to == 0 && h < 0.5 {
			delete(g.rooms, i)
			continue
		}
		g.rooms[i] = h
	}
}

// drawBlockDrop draws the block dragged lifted from its place: faded into
// the page, with a glow of the accent around it.
func (a *app) drawBlockDrop(p *ui.Painter, r ui.Rect) {
	g := &a.grip
	if !g.dragging {
		return
	}
	top, bottom, ok := a.blockSpan(g.from[0], g.from[1])
	if !ok {
		return
	}
	acc := p.Theme().Accent
	box := ui.Rect{X: r.X - 8, Y: r.Y + top - 4, W: r.W + 16, H: bottom - top + 8}
	p.Shadow(box, 8, 0, 0, 14, 0, acc.Alpha(0.22))
	p.Fill(box, g.bg.Alpha(0.6), rad(8))
	p.Stroke(box, acc.Alpha(0.45), rad(8), 1)
}

// blockPreview is the block dragged where it would go: its first lines, in
// the room opened for them, an inset well that fades in as the room opens.
func (a *app) blockPreview(c *ui.Context) {
	g := &a.grip
	t := ryugo.From(c)
	cl := t.Colors
	var y, h float32
	switch {
	case g.toIdx >= 0:
		for _, b := range a.boxes {
			if b.Index == g.toIdx {
				h = g.rooms[g.toIdx]
				y = b.Top - h
				break
			}
		}
	case g.to >= 0: // at the end of the text: below it, nothing to move
		y, h = g.toY, g.roomH
	}
	if h < 1 {
		return
	}
	text := a.doc.Text().Slice(g.from[0], g.from[1]).String()
	if text == "" {
		text = "…"
	}
	lines := max(1, int((g.roomH-2*previewPad-previewGap)/previewLineH))
	pv := ui.Column(c.Key("block-preview")).Absolute().Left(a.editorR.X-8).Top(a.editorR.Y+y).
		Width(a.editorR.W+16).Height(h-previewGap).Padding(previewPad, 12).Clip().PassThrough().
		Radius(8).Background(ryugo.Mix(cl.Well, cl.ColorAccent500, 0.06)).Border(1, cl.ColorAccent500).BorderStyle(ui.BorderDashed).
		Opacity(min(1, h/g.roomH))
	pv.Children(func() {
		ui.Text(c, text).FontSize(14).LineHeight(previewLineH).TextColor(cl.Muted).MaxLines(lines)
	})
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
	a.openRooms(gh)
	gh.Rotate(gh.AnimateTo("tilt", -1.2, springTilt)).Scale(1.02, 1.02)
	gh.Children(func() {
		gripDots(c, cl.Muted)
		ui.Text(c, first).FontSize(14).TextColor(cl.Text).MaxLines(1)
		if g.from[1] > e {
			ui.Text(c, T("+ altındakiler")).FontSize(12).TextColor(cl.Muted)
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
	ui.Icon(c, iconGrip).Width(16).Height(16).TextColor(color)
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
		{title: "Dönüştür…", glyph: iconFormat, nav: true, run: func() { g.view, g.sel, g.seen, g.scroll = "turn", 1, -1, ui.ScrollState{} }},
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
	back := func() { g.view, g.sel, g.seen, g.scroll = "", 0, -1, ui.ScrollState{} }
	items := a.blockItems()
	if g.view != "" { // a list gone into: ← Geri is its first line, as any other
		items = append([]menuItem{{title: "Geri", glyph: iconBack, nav: true, back: true, run: back}}, items...)
	}
	g.sel = max(0, min(g.sel, len(items)-1))
	pl := placeMenu(c, at.X, at.Y, at.Y+at.H, items, 0, 0) // no header
	keepInSight(g.sel, &g.seen, &g.scroll, pl)
	choose := func(it menuItem) {
		it.run()
		if !it.nav {
			g.menu = false
		}
	}
	ww, wh := c.Size()
	if ui.Box(c.Key("block-away")).Absolute().Left(0).Top(0).Size(ww, wh).Clicked() {
		g.menu = false
		return
	}
	m := &menuPanel{key: "block", label: "Blok menüsü", pl: pl, items: items, sel: g.sel, selPtr: &g.sel, hoverSeen: &g.hseen, scroll: &g.scroll, choose: choose, empty: "Boş"}
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
	case g.view != "" && panel.Shortcut(0, ui.KeyLeft):
		back()
	case panel.Shortcut(0, ui.KeyEnter) && len(items) > 0:
		choose(items[g.sel])
	}
}

// modal reports whether a dialog is open over the window: what is under it is
// not pointed at (no grip, no bar) while it is.
func (a *app) modal() bool {
	return a.cs.open || a.prop.open || a.pal.open || a.tagRen.open || a.crashOpen || a.vhist.open || a.lightbox != nil || a.wel.on
}
