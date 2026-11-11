package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
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
	menuWas  bool // the menu showed last frame: it is built once more, when the line chosen in it runs
	pressed  bool
	moved    float32
	dragging bool
	from     [2]int // the block dragged
	to       int    // where it would go: the paragraph it goes before, or the text's length+1
	toY      float32
	toIdx    int              // the paragraph (from 0) the room opens above, -1 at the end of the text
	rooms    map[int]float32  // the room open above paragraphs, by index, as it springs open and shut
	roomH    float32          // the room a preview of the block dragged takes
	paras    [2]int           // the block's first and last paragraph, by index, for the editor's spans (blockSpans)
	lays     map[int]blockLay // the block's paragraphs, laid out for the drag (drawBlockDrop)
	ink      ui.Color         // the accent, the block's text drawn in it
	pos      ui.Point         // the pointer, in window DIPs
	hover    bool             // the pointer is on the grip
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
					g.menu = true
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
	if !found && !g.dragging { // a block dragged far keeps its grip where it was, or the drag would be lost
		g.hover = false
		return
	}
	// the first line of the block: the grip is as tall as a line of text and level with it, at the block's top
	line := a.firstLineH(g.para, g.h)
	gy := r.Y + g.top + max(0, (line-gripH)/2)
	ui.Overlay(c, func() {
		grip := ui.Box(c.Key("grip")).Absolute().Left(r.X-gripW-8).Top(gy).Size(gripW, gripH).Radius(rad(5)).
			AlignItems(ui.Center).Justify(ui.Center).Label(locales.BlocksBlockHandle()).Role(ui.RoleButton).Cursor(ui.CursorGrab)
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
			g.menu = true
		}
		if g.menu || g.menuWas {
			a.blockMenu(c, grip.LayoutBounds())
		}
		g.menuWas = g.menu
		if g.dragging {
			a.openRooms(grip)
			if g.to >= 0 { // where it would go, it shows there; elsewhere it follows the pointer
				a.blockPreview(c)
			} else {
				a.blockGhost(c)
			}
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
			t := a.doc.Text()
			s, e := blockRange(t, g.para)
			first := t.Slice(0, s).ParagraphCount() - 1
			g.dragging, g.from, g.to, g.toIdx, g.rooms = true, [2]int{s, e}, -1, -1, map[int]float32{}
			g.paras = [2]int{first, first + t.Slice(s, e).ParagraphCount() - 1}
			g.lays, g.ink = map[int]blockLay{}, c.Theme().Accent
			g.roomH = previewH + 2*previewGap
		}
	case pressed && g.dragging:
		b := grip.LayoutBounds()
		px, py, _ := grip.PointerPosition()
		g.pos = ui.Point{X: b.X + px, Y: b.Y + py}
		if a.edgeScroll(g.pos.Y) {
			c.AnimationFrame() // held still at the edge, it goes on scrolling
		}
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

// The preview of a block dragged, in the room opened for it: a card a line
// tall, with some air above and below it.
const (
	previewH   = 30
	previewGap = 5
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

// edgeScroll scrolls the page while a block is dragged near its top or
// bottom edge (y in window DIPs), faster the closer it is, as a browser
// does; it reports whether it scrolled.
func (a *app) edgeScroll(y float32) bool {
	const zone, fastest = 56, 18 // DIPs from the edge where it scrolls; DIPs a frame at the edge
	vp := a.scrollR
	if vp.H <= 0 {
		return false
	}
	top, bottom := vp.Y+a.barInset, vp.Y+vp.H
	var d float32
	switch {
	case y < top+zone:
		d = -fastest * min(1, (top+zone-y)/zone)
	case y > bottom-zone:
		d = fastest * min(1, (y-(bottom-zone))/zone)
	}
	was := a.pageScroll.Y
	a.pageScroll.Y = min(max(0, was+d), a.pageScroll.MaxY)
	return a.pageScroll.Y != was
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

// drawBlockDrop draws the shimmer over the block dragged, which the editor
// draws faded in the accent (blockSpans): a band of brighter ink going
// across it, so that it is plain which block is lifted. Where it would go
// is its preview (blockPreview), over the window.
func (a *app) drawBlockDrop(p *ui.Painter, r ui.Rect) {
	g := &a.grip
	if !g.dragging {
		return
	}
	t := a.doc.Text()
	type line struct {
		l    *ui.TextLayout
		x, y float32
	}
	var lines []line
	top, bottom := float32(-1), float32(0)
	for _, b := range a.shown(r) {
		if b.Start < g.from[0] || b.Start > g.from[1] {
			continue
		}
		s, e := t.ParagraphRange(b.Start)
		if e-s > maxCaretParagraph { // too long to lay out again each frame: it stays as the editor draws it
			continue
		}
		if top < 0 {
			top = b.Top
		}
		bottom = b.Top + b.Height
		l, ok := g.lays[s]
		if !ok || l.w != r.W {
			var para richtext.Paragraph
			for _, q := range t.Slice(s, e).Paragraphs() {
				para = q
				break
			}
			para.Style = t.ParagraphStyleAt(s)
			l.l, l.left = layoutParagraph(para, r.W, a.font, a.theme)
			l.w = r.W
			g.lays[s] = l
		}
		lines = append(lines, line{l.l, r.X + l.left, r.Y + b.Top})
	}
	if top < 0 {
		return
	}
	th := p.Theme()
	box := ui.Rect{X: r.X - 4, Y: r.Y + top - 2, W: r.W + 8, H: bottom - top + 4}
	text := func(c ui.Color) {
		for _, ln := range lines {
			p.TextLayout(ln.l, ln.x, ln.y, c)
		}
	}
	// the shimmer: a slanted band of brighter ink going left to right, soft at its edges
	const period, slant = 1.6, 40
	k := float32(math.Mod(float64(p.Now().UnixMilli())/1000/period, 1))
	x := box.X - 160 + (box.W+320)*k
	for _, w := range []float32{150, 90, 40} {
		band := (&ui.Path{}).MoveTo(x-w/2+slant, box.Y).LineTo(x+w/2+slant, box.Y).
			LineTo(x+w/2-slant, box.Y+box.H).LineTo(x-w/2-slant, box.Y+box.H).Close()
		p.ClipPath(band, func() { text(th.Accent.Alpha(0.4)) })
	}
	p.AnimationFrame()
}

// blockSpans draws the paragraphs of the block dragged faded, in the accent.
func (a *app) blockSpans(p int, spans []ui.Span) []ui.Span {
	g := &a.grip
	if !g.dragging || p < g.paras[0] || p > g.paras[1] {
		return spans
	}
	for i := range spans {
		spans[i].Color = g.ink.Alpha(0.5)
	}
	return spans
}

// blockLay is a paragraph of the block dragged laid out as the editor lays
// it out, kept for the drag, for drawBlockDrop to draw it in its own ink.
type blockLay struct {
	l       *ui.TextLayout
	left, w float32
}

// blockPreview is the block dragged where it would go: the start of its
// text on a card, one line cut short, in the room the paragraphs make for
// it there (openRooms), coming as the room opens.
func (a *app) blockPreview(c *ui.Context) {
	g := &a.grip
	t := ryugo.From(c)
	cl := t.Colors
	r := a.editorR
	var y, h float32
	switch {
	case g.toIdx >= 0:
		for _, b := range a.boxes {
			if b.Index == g.toIdx {
				h = g.rooms[g.toIdx]
				y = b.Top - h + previewGap
				break
			}
		}
	case g.to >= 0: // at the end of the text: below it, nothing to move
		y, h = g.toY, g.roomH
	}
	if h < 1 {
		return
	}
	s := a.doc.Text().Slice(g.from[0], min(g.from[1], a.doc.Text().Len())).String()
	s = strings.Join(strings.Fields(s), " ")
	if n := []rune(s); len(n) > 160 {
		s = string(n[:160]) + "…"
	}
	if s == "" {
		s = "…"
	}
	card := ui.Row(c.Key("block-preview")).Absolute().Left(r.X-8).Top(r.Y+y).Width(r.W+16).Height(previewH).Padding(0, 12).Gap(8).
		Radius(rad(8)).Background(cl.Card.Alpha(0.96)).Border(1, cl.Accent.Alpha(0.6)).PassThrough().AlignItems(ui.Center).
		Opacity(min(1, h/g.roomH))
	t.Shadows.ShadowPop.Apply(card)
	card.Children(func() {
		gripDots(c, cl.Muted)
		ui.Text(c, s).FontSize(13).TextColor(cl.Text).MaxLines(1).Opacity(0.85)
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
	gh.Rotate(gh.AnimateTo("tilt", -1.2, springTilt)).Scale(1.02, 1.02)
	gh.Children(func() {
		gripDots(c, cl.Muted)
		ui.Text(c, first).FontSize(14).TextColor(cl.Text).MaxLines(1)
		if g.from[1] > e {
			ui.Text(c, locales.BlocksThoseBelow()).FontSize(12).TextColor(cl.Muted)
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
	turn := func() []menuItem {
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
		{title: "Dönüştür…", glyph: iconFormat, items: turn},
		{title: "Çoğalt", hint: "⌘D", glyph: iconDuplicate, run: done(func() { duplicateBlock(a.doc, at) })},
		{title: "Yukarı taşı", hint: "⌘⇧↑", glyph: iconMoveUp, run: done(func() { a.doc.SetSelection(at, at); moveBlock(a.doc, -1) })},
		{title: "Aşağı taşı", hint: "⌘⇧↓", glyph: iconMoveDown, run: done(func() { a.doc.SetSelection(at, at); moveBlock(a.doc, 1) })},
		{title: "Sil", group: "~", glyph: iconTrash, danger: true, run: done(func() { deleteBlock(a.doc, at) })},
	}...)
}

// modal reports whether a dialog is open over the window: what is under it is
// not pointed at (no grip, no bar) while it is.
func (a *app) modal() bool {
	return a.cs.open || a.prop.open || a.pal.open || a.tagRen.open || a.crashOpen || a.vhist.open || a.lightbox != nil || a.wel.on
}

// blockMenu is the menu of a block, MyGo's drawn in the window (popmenu.go), under its grip (the span at): its kinds
// to turn it into are a submenu.
func (a *app) blockMenu(c *ui.Context, at ui.Rect) {
	menuAt(c, "block-at", locales.CommonBlockMenu(), at, &a.grip.menu, a.blockItems())
}
