package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The editor view: it lays the blocks out a column down the page, draws them
// with their markers in the gutter (a bullet, a number, a checkbox - never a
// Markdown mark), draws the selection and the caret, and turns keys, clicks
// and the system's text input into edits of the document.

const (
	pageTop    = 64
	pageBottom = 160
	pageSide   = 48
	columnMax  = 720
	indentStep = 26
	gutterW    = 28
	codePad    = 14
)

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
)

// caretFrame is what a caret is told each frame (see mizan.go).
type caretFrame struct {
	r      ui.Rect
	cw     float32
	dark   bool
	bounds ui.Rect
	wrote  int
	edited bool
}

type editor struct {
	doc   *Doc
	theme *ui.Theme

	width, height float32 // the element's size, from the last frame
	scrollY       float32
	follow        bool // bring the caret into view at the next frame
	boxes         []box
	cache         map[int]*shaped

	desiredX float32 // kept while going up and down
	hasX     bool
	marked   *ui.TextInputRange // an input method's composition, in the focus block
	dragging bool

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret
	wrote     int
	edited    bool

	path    string // the Markdown file the document is kept in
	editAt  time.Time
	savedOK bool
	saveErr string
	now     func() time.Time
}

func newEditor(doc *Doc, path string) *editor {
	return &editor{doc: doc, cache: map[int]*shaped{}, jelly: newSlime(), mizan: &mizanCaret{}, path: path, savedOK: true, now: time.Now}
}

// shaped is a block's laid-out text, kept while the block and the width stay.
type shaped struct {
	ver    int
	width  float32
	dark   bool
	layout *ui.TextLayout
}

// box is where a block sits, in document coordinates (y from the page top).
type box struct {
	y, h   float32 // the block's band, margins included
	tx, ty float32 // its text's top left
	tw     float32
	layout *ui.TextLayout
}

type blockStyle struct {
	size, line   float32
	weight       int
	top, bottom  float32
	mono, italic bool
}

func styleOf(k Kind) blockStyle {
	switch k {
	case Heading1:
		return blockStyle{size: 30, line: 1.25, weight: 720, top: 26, bottom: 6}
	case Heading2:
		return blockStyle{size: 23, line: 1.3, weight: 690, top: 20, bottom: 4}
	case Heading3:
		return blockStyle{size: 19, line: 1.35, weight: 660, top: 14, bottom: 2}
	case Code:
		return blockStyle{size: 14, line: 1.55, weight: 400, top: 10, bottom: 10, mono: true}
	case Quote:
		return blockStyle{size: 16.5, line: 1.6, weight: 400, top: 6, bottom: 6}
	case Divider:
		return blockStyle{size: 16.5, line: 1.6, top: 10, bottom: 10}
	}
	return blockStyle{size: 16.5, line: 1.6, weight: 400, top: 3, bottom: 3}
}

func (e *editor) column() (x, w float32) {
	w = min(columnMax, max(200, e.width-2*pageSide))
	return (e.width - w) / 2, w
}

// spans turns a block's text and marks into runs of one style each.
func (e *editor) spans(b *Block) []ui.Span {
	t := e.theme
	st := styleOf(b.Kind)
	color := t.Text
	if b.Kind == Todo && b.Checked {
		color = t.TextMuted
	}
	if b.Kind == Quote {
		color = mix(t.Text, t.TextMuted, 0.35)
	}
	base := ui.Span{Size: st.size, Weight: st.weight, Color: color, Italic: st.italic, Strikethrough: b.Kind == Todo && b.Checked}
	if st.mono {
		base.Font = "monospace"
	}
	if b.Text == "" {
		s := base
		s.Text = " " // an empty block still has a line, for the caret
		return []ui.Span{s}
	}
	cuts := map[int]bool{0: true, b.len(): true}
	for _, m := range b.Marks {
		cuts[m.S], cuts[m.E] = true, true
	}
	var at []int
	for c := range cuts {
		at = append(at, c)
	}
	sortInts(at)
	var out []ui.Span
	for i := 0; i+1 < len(at); i++ {
		s := base
		s.Text = b.slice(at[i], at[i+1])
		for _, m := range b.Marks {
			if m.S <= at[i] && m.E >= at[i+1] {
				switch m.K {
				case Bold:
					s.Weight = max(700, s.Weight+120)
				case Italic:
					s.Italic = true
				case Strike:
					s.Strikethrough = true
				case InlineCode:
					s.Font, s.Size, s.Background, s.Radius = "monospace", st.size*0.88, t.Surface, 4
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// fresh brings the layout up to date after an edit: what is asked of it
// (where the caret is, what is under the pointer) may come before the next
// frame. Blocks that did not change come from the cache.
func (e *editor) fresh() {
	if e.theme != nil && e.width > 0 {
		e.layout()
	}
}

// layout places every block; only blocks whose text or the width changed are shaped again.
func (e *editor) layout() {
	x0, w := e.column()
	y := float32(pageTop)
	e.boxes = e.boxes[:0]
	live := map[int]bool{}
	for _, b := range e.doc.Blocks {
		live[b.ID] = true
		st := styleOf(b.Kind)
		tx := x0 + float32(b.Depth)*indentStep
		switch {
		case b.Kind.isList():
			tx += gutterW
		case b.Kind == Quote:
			tx += 18
		case b.Kind == Code:
			tx += codePad
		}
		tw := x0 + w - tx
		if b.Kind == Code {
			tw -= codePad
		}
		var l *ui.TextLayout
		var th float32
		if b.Kind == Divider {
			th = 4
		} else {
			c := e.cache[b.ID]
			if c == nil || c.ver != b.Ver || c.width != tw || c.dark != e.theme.Dark {
				c = &shaped{ver: b.Ver, width: tw, dark: e.theme.Dark, layout: ui.ShapeRichText(e.spans(b), ui.Font{Size: st.size, LineHeight: st.line}, max(tw, 40))}
				e.cache[b.ID] = c
			}
			l = c.layout
			_, th = l.Size()
		}
		pad := float32(0)
		if b.Kind == Code {
			pad = codePad
		}
		e.boxes = append(e.boxes, box{y: y, h: st.top + pad + th + pad + st.bottom, tx: tx, ty: y + st.top + pad, tw: tw, layout: l})
		y += st.top + pad + th + pad + st.bottom
	}
	for id := range e.cache {
		if !live[id] {
			delete(e.cache, id)
		}
	}
}

func (e *editor) docHeight() float32 {
	if len(e.boxes) == 0 {
		return 0
	}
	last := e.boxes[len(e.boxes)-1]
	return last.y + last.h + pageBottom
}

// caretRect is the caret's box at p in document coordinates, the line's
// letters' height (not its leading), and the width of the letter after it.
func (e *editor) caretRect(p Pos) (ui.Rect, float32) {
	e.fresh()
	bx := e.boxes[p.B]
	if bx.layout == nil { // a divider: a caret across its line
		_, w := e.column()
		return ui.Rect{X: bx.tx, Y: bx.y + bx.h/2 - 9, W: 2, H: 18}, w
	}
	c := bx.layout.Caret(p.O)
	mid := c.Y + c.H/2
	for _, l := range bx.layout.Lines() {
		if mid >= l.Rect.Y && mid <= l.Rect.Y+l.Rect.H && l.Ascent > 0 {
			c.Y, c.H = l.Baseline-l.Ascent, l.Ascent+l.Descent
			break
		}
	}
	cw := c.H * 0.5
	b := e.doc.Blocks[p.B]
	if p.O < b.len() && b.slice(p.O, p.O+1) != "\n" {
		if n := bx.layout.NextBoundary(p.O); n > p.O {
			if nc := bx.layout.Caret(n); nc.X > c.X && math.Abs(float64(nc.Y-bx.layout.Caret(p.O).Y)) < 0.5 {
				cw = nc.X - c.X
			}
		}
	}
	c.X += bx.tx
	c.Y += bx.ty
	return c, cw
}

// hit is the place in the document under (x, y) in document coordinates.
func (e *editor) hit(x, y float32) Pos {
	e.fresh()
	if len(e.boxes) == 0 {
		return Pos{}
	}
	i := len(e.boxes) - 1
	for j, bx := range e.boxes {
		if y < bx.y+bx.h {
			i = j
			break
		}
	}
	bx := e.boxes[i]
	if bx.layout == nil {
		return Pos{i, 0}
	}
	o := bx.layout.IndexAt(ui.Point{X: x - bx.tx, Y: min(max(y-bx.ty, 0), bx.h)})
	return e.doc.clampPos(Pos{i, o})
}

// ---- drawing ----

func (e *editor) draw(p *ui.Painter, r ui.Rect) {
	t := e.theme
	if r.W != e.width || r.H != e.height {
		e.width, e.height = r.W, r.H
	}
	e.layout()
	maxScroll := max(0, e.docHeight()-e.height)
	if e.follow {
		c, _ := e.caretRect(e.doc.caret())
		if c.Y-40 < e.scrollY {
			e.scrollY = c.Y - 40
		}
		if c.Y+c.H+80 > e.scrollY+e.height {
			e.scrollY = c.Y + c.H + 80 - e.height
		}
		e.follow = false
	}
	e.scrollY = max(0, min(maxScroll, e.scrollY))
	ox, oy := r.X, r.Y-e.scrollY
	a, z := e.doc.Sel.ordered()
	focus := e.doc.caret()
	p.Clip(r, 0, func() {
		for i, bx := range e.boxes {
			if bx.y+bx.h < e.scrollY || bx.y > e.scrollY+e.height {
				continue // only what is in view is drawn
			}
			b := e.doc.Blocks[i]
			e.drawBlockBack(p, b, bx, ox, oy, i)
			if i >= a.B && i <= z.B && !e.doc.Sel.collapsed() {
				e.drawSelection(p, b, bx, i, a, z, ox, oy)
			}
			if bx.layout != nil {
				p.TextLayout(bx.layout, ox+bx.tx, oy+bx.ty, t.Text)
			}
			e.drawGutter(p, b, bx, i, ox, oy)
			if i == focus.B && b.Text == "" && e.doc.Sel.collapsed() {
				e.drawPlaceholder(p, b, bx, ox, oy)
			}
		}
		e.drawCaret(p, r, ox, oy)
		e.drawStatus(p, r)
	})
	e.autosave(p)
}

func (e *editor) drawBlockBack(p *ui.Painter, b *Block, bx box, ox, oy float32, i int) {
	t := e.theme
	x0, w := e.column()
	switch b.Kind {
	case Code:
		st := styleOf(Code)
		rect := ui.Rect{X: ox + x0 + float32(b.Depth)*indentStep, Y: oy + bx.y + st.top, W: w - float32(b.Depth)*indentStep, H: bx.h - st.top - st.bottom}
		p.Fill(rect, t.Surface, 8)
		p.Stroke(rect, withAlpha(t.Border, 0.6), 8, 1)
	case Quote:
		st := styleOf(Quote)
		p.Fill(ui.Rect{X: ox + x0 + float32(b.Depth)*indentStep + 2, Y: oy + bx.y + st.top, W: 3, H: bx.h - st.top - st.bottom}, withAlpha(t.Accent, 0.55), 1.5)
	case Divider:
		p.Fill(ui.Rect{X: ox + x0, Y: oy + bx.y + bx.h/2 - 0.5, W: w, H: 1}, t.Border, 0)
	}
}

func (e *editor) drawSelection(p *ui.Painter, b *Block, bx box, i int, a, z Pos, ox, oy float32) {
	t := e.theme
	s, en := 0, b.len()
	if i == a.B {
		s = a.O
	}
	if i == z.B {
		en = z.O
	}
	if bx.layout == nil || (s == en && i != z.B) || b.Text == "" {
		if i != z.B || a.B != z.B { // a whole empty block or a divider in the selection
			p.Fill(ui.Rect{X: ox + bx.tx - 2, Y: oy + bx.ty, W: 8, H: max(bx.h-styleOf(b.Kind).top-styleOf(b.Kind).bottom, 16)}, t.Selection, 3)
		}
		return
	}
	for _, rc := range bx.layout.SelectionRects(ui.TextInputRange{Start: s, End: en}) {
		rc.X += ox + bx.tx
		rc.Y += oy + bx.ty
		if i != z.B && en == b.len() {
			rc.W += 6 // the line break is selected too
		}
		p.Fill(rc, t.Selection, 3)
	}
}

// drawGutter draws a list item's marker: a bullet (a disc, a ring, a square by
// depth), its number, or a checkbox that draws its check.
func (e *editor) drawGutter(p *ui.Painter, b *Block, bx box, i int, ox, oy float32) {
	if !b.Kind.isList() || bx.layout == nil {
		return
	}
	t := e.theme
	lines := bx.layout.Lines()
	if len(lines) == 0 {
		return
	}
	l := lines[0]
	gx := ox + bx.tx - gutterW
	mid := oy + bx.ty + l.Baseline - l.Ascent*0.36
	switch b.Kind {
	case Bullet:
		cx := gx + gutterW/2 - 2
		switch b.Depth % 3 {
		case 0:
			p.FillPath((&ui.Path{}).Circle(cx, mid, 2.8), t.Text)
		case 1:
			p.StrokePath((&ui.Path{}).Circle(cx, mid, 2.6), 1.3, t.Text)
		default:
			p.Fill(ui.Rect{X: cx - 2.4, Y: mid - 2.4, W: 4.8, H: 4.8}, t.Text, 0.8)
		}
	case Numbered:
		st := styleOf(Paragraph)
		label := ui.ShapeRichText([]ui.Span{{Text: itoa(e.doc.numberOf(i)) + ".", Size: st.size, Color: t.TextMuted, Features: "tnum"}}, ui.Font{Size: st.size, LineHeight: st.line}, 0)
		w, _ := label.Size()
		p.TextLayout(label, gx+gutterW-6-w, oy+bx.ty, t.TextMuted)
	case Todo:
		s := float32(16)
		box := ui.Rect{X: gx + 2, Y: mid - s/2, W: s, H: s}
		if b.Checked {
			p.Fill(box, t.Accent, 4.5)
			path := (&ui.Path{}).Cap(ui.RoundCap).Join(ui.RoundJoin)
			path.MoveTo(box.X+4, box.Y+8.4).LineTo(box.X+6.9, box.Y+11.2).LineTo(box.X+12.2, box.Y+5)
			p.StrokePath(path, 1.9, ui.RGBA(255, 255, 255, 1))
		} else {
			p.Stroke(box, mix(t.Border, t.TextMuted, 0.5), 4.5, 1.4)
		}
	}
}

var placeholders = map[Kind]string{
	Paragraph: "Yaz…  # başlık · - liste · [] görev · > alıntı · ``` kod · --- çizgi",
	Heading1:  "Başlık 1", Heading2: "Başlık 2", Heading3: "Başlık 3",
	Bullet: "Liste", Numbered: "Liste", Todo: "Yapılacak", Quote: "Alıntı", Code: "Kod",
}

func (e *editor) drawPlaceholder(p *ui.Painter, b *Block, bx box, ox, oy float32) {
	text, ok := placeholders[b.Kind]
	if !ok || bx.layout == nil {
		return
	}
	st := styleOf(b.Kind)
	span := ui.Span{Text: text, Size: st.size, Weight: st.weight, Color: withAlpha(e.theme.TextMuted, 0.6)}
	if st.mono {
		span.Font = "monospace"
	}
	// Shaped as the block is, so it sits on the same baseline.
	l := ui.ShapeRichText([]ui.Span{span}, ui.Font{Size: st.size, LineHeight: st.line}, bx.tw)
	p.TextLayout(l, ox+bx.tx, oy+bx.ty, span.Color)
}

func (e *editor) drawCaret(p *ui.Painter, r ui.Rect, ox, oy float32) {
	c, cw := e.caretRect(e.doc.caret())
	c.X += ox
	c.Y += oy
	f := caretFrame{r: c, cw: cw, dark: e.theme.Dark, bounds: r, wrote: e.wrote, edited: e.edited}
	e.wrote, e.edited = 0, false
	if !e.doc.Sel.collapsed() {
		p.Fill(ui.Rect{X: c.X - 1, Y: c.Y, W: 2, H: c.H}, e.theme.Accent, 1)
		return
	}
	if e.caretKind == caretMizan {
		e.mizan.draw(p, f)
		return
	}
	col := ui.Hex("#59f939")
	if !e.theme.Dark {
		col = ui.Hex("#2f9a1c")
	}
	e.jelly.draw(p, c, cw, col, e.theme.Dark)
}

// drawStatus is the line at the bottom: the file, whether it is saved, the caret.
func (e *editor) drawStatus(p *ui.Painter, r ui.Rect) {
	t := e.theme
	name := e.path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	state := "kaydedildi"
	switch {
	case e.saveErr != "":
		state = "kaydedilemedi: " + e.saveErr
	case e.doc.Dirty:
		state = "kaydediliyor…"
	}
	caret := map[caretKind]string{caretSlime: "Slime", caretMizan: "Mizan"}[e.caretKind]
	text := name + " · " + state + "   ·   caret: " + caret + " (⌘⇧M)"
	span := ui.Span{Text: text, Size: 11.5, Color: withAlpha(t.TextMuted, 0.85)}
	w, h := p.MeasureText(0, span)
	p.RichText(r.X+r.W-w-18, r.Y+r.H-h-12, 0, span)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
