package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// Tables: a table (tablemodel.go) is one object in the text, as an embed's
// card is: richtext's attachment in a paragraph of its own, drawn here. It
// is always a table, never Markdown to be written: a click on a cell opens
// the cell to be written in, Tab goes on to the next, a few buttons add and
// take away rows and columns and say what a column is. A table in the text
// is never changed, a new one takes its place (editTable), so that undo
// brings back the one it had.

const (
	tableRowH    = 34  // a row
	tableFootH   = 30  // the foot, where a column says its sum
	tablePadX    = 12  // room on each side of a cell's text
	tableMinW    = 72  // a column is never narrower than this by itself
	tableMaxW    = 360 // and never wider
	tableGap     = 3   // room over and under the table
	tableFilterW = 180 // a table with a filter is at least this wide, for its strip
)

// splitCells reads a line of a Markdown table into its cells (for the tables
// of other tools, read once): the pipes at its edges are not cells, "\|" is
// a pipe in one.
func splitCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	var cells []string
	var cur strings.Builder
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs) && rs[i+1] == '|':
			cur.WriteRune('|')
			i++
		case rs[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(rs[i])
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" || len(cells) == 0 {
		cells = append(cells, s)
	}
	return cells
}

// tableChip is the object of a table: its model, and its columns' widths.
func (a *app) tableChip(m *tableModel) *chip {
	return &chip{kind: "table", tbl: m, tw: a.tableWidths(m)}
}

// tableAttachment is the table as richtext's attachment.
func (a *app) tableAttachment(ch *chip) *richtext.Attachment {
	w := float32(0)
	for _, x := range ch.tw {
		w += x
	}
	return &richtext.Attachment{Width: w + 2, Height: tableHeight(ch.tbl) + 2*tableGap, Name: "Tablo", Data: ch,
		Draw: func(p *ui.Painter, r ui.Rect) { a.drawTable(p, r, ch) }}
}

// tableHeight is how tall the table is: its header, its rows and its foot.
func tableHeight(m *tableModel) float32 {
	h := float32(len(m.visible())+1)*tableRowH + 2
	if m.hasCalc() {
		h += tableFootH
	}
	if m.Filter != "" { // the strip that says what the filter is
		h += tableFootH
	}
	return h
}

// cellWidth is how wide a cell's text is drawn, as the text itself measures
// (kept by the text: the font does not change under it).
func (a *app) cellWidth(md string, head bool) float32 {
	key := md
	if head {
		key = "\x00" + md
	}
	if w, ok := a.cellW[key]; ok {
		return w
	}
	spans := a.cellSpans(md, a.fontSize()*0.95, tableWeight(head), &ui.Theme{}, ui.Color{})
	l := ui.ShapeRichTextWith(spans, a.font, ui.TextShape{})
	w, _ := l.Size()
	if a.cellW == nil || len(a.cellW) > 20000 {
		a.cellW = map[string]float32{}
	}
	a.cellW[key] = w
	return w
}

func tableWeight(head bool) int {
	if head {
		return 650
	}
	return 400
}

// tableWidths are the widths of the columns: the width a column was given,
// or as wide as its widest cell. Together they are no wider than the page.
func (a *app) tableWidths(m *tableModel) []float32 {
	cw := make([]float32, len(m.cols))
	for c, col := range m.cols {
		switch {
		case col.W > 0:
			cw[c] = max(col.W, 36)
			continue
		case col.Kind == kindCheck:
			cw[c] = max(a.cellWidth(col.Name, true)+2*tablePadX, 56)
			continue
		}
		w := a.cellWidth(col.Name, true)
		for _, r := range m.rows {
			if r[c] != "" {
				w = max(w, a.cellWidth(a.shownText(col, r[c]), false))
			}
		}
		if v := m.calc(c); v != "" { // the foot says its calculation, and its number
			w = max(w, a.cellWidth(footText(col, v), false)*0.9)
		}
		cw[c] = min(max(w+2*tablePadX, tableMinW), tableMaxW)
	}
	total := float32(0)
	for _, w := range cw {
		total += w
	}
	if m.Filter != "" && total < tableFilterW && len(cw) > 0 { // room for the strip under it
		cw[len(cw)-1] += tableFilterW - total
		total = tableFilterW
	}
	if limit := a.pageWidth(); total > limit && total > 0 {
		for c := range cw {
			cw[c] *= limit / total
		}
	}
	return cw
}

// footText is what the foot of a column says: its calculation, and the number.
func footText(col tableCol, v string) string {
	if col.Kind == kindCheck {
		return v
	}
	return strings.ToLower(calcNames[col.Calc]) + " " + v
}

// shownText is what a cell says: a date as the day reads, the rest as written.
func (a *app) shownText(col tableCol, s string) string {
	if col.Kind == kindDate {
		if d, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
			return shortDate(d)
		}
	}
	return s
}

// tableHit is a cell drawn this frame, where it is in the window: row -1 is
// the header.
type tableHit struct {
	r        ui.Rect
	ch       *chip
	row, col int
}

// colBorder is the line between two columns, where it can be dragged.
type colBorder struct {
	r   ui.Rect
	ch  *chip
	col int // the column on its left
	w   float32
}

// borderKey names a border: a table and the column to its left. The zero is none.
type borderKey struct {
	ch  *chip
	col int
}

// colDragState is a column being resized.
type colDragState struct {
	active bool
	ch     *chip
	col    int
	w0, dx float32
}

// cellSpans are the spans of a cell: its Markdown as it reads - bold,
// italic, code, strikethrough, links, [[pages]] and #tags.
func (a *app) cellSpans(md string, size float32, weight int, th *ui.Theme, accent ui.Color) []ui.Span {
	var spans []ui.Span
	add := func(text string, at richtext.Attributes, accented bool) {
		sp := ui.Span{Text: text, Size: size, Weight: weight, Color: th.Text}
		if at.Weight >= 600 {
			sp.Weight = 700
		}
		sp.Italic, sp.Strikethrough = at.Italic, at.Strikethrough
		if at.Code {
			sp.Font, sp.Background, sp.Radius = "monospace", th.Text.Alpha(0.09), 4
		}
		if at.Link != "" || accented {
			sp.Color = accent
			sp.Underline = at.Link != ""
		}
		spans = append(spans, sp)
	}
	pos := 0
	segment := func(seg string) {
		if seg == "" {
			return
		}
		// the parser trims a text; the spaces at its ends are kept here
		lead, trail := len(seg)-len(strings.TrimLeft(seg, " ")), len(seg)-len(strings.TrimRight(seg, " "))
		if lead == len(seg) {
			add(seg, richtext.Attributes{}, false)
			return
		}
		if lead > 0 {
			add(seg[:lead], richtext.Attributes{}, false)
		}
		if trail > 0 {
			defer add(seg[len(seg)-trail:], richtext.Attributes{}, false)
		}
		seg = strings.TrimSpace(seg)
		for _, r := range richtext.ParseMarkdown(seg).Runs(0, richtext.ParseMarkdown(seg).Len()) {
			text := r.Text
			if r.Attributes.Code {
				add(text, r.Attributes, false)
				continue
			}
			at := 0
			for _, m := range tagWord.FindAllStringSubmatchIndex(text, -1) {
				if m[4]-1 > at {
					add(text[at:m[4]-1], r.Attributes, false)
				}
				add(text[m[4]-1:m[5]], r.Attributes, true)
				at = m[5]
			}
			if at < len(text) {
				add(text[at:], r.Attributes, false)
			}
		}
	}
	for _, m := range wikiInText.FindAllStringSubmatchIndex(md, -1) {
		segment(md[pos:m[0]])
		inside := md[m[2]:m[3]]
		target, alias, _ := strings.Cut(inside, "|")
		label := strings.TrimSpace(alias)
		if label == "" {
			label = strings.TrimSpace(target)
		}
		add(label, richtext.Attributes{}, true)
		pos = m[1]
	}
	segment(md[pos:])
	if len(spans) == 0 {
		spans = []ui.Span{{Text: " ", Size: size, Weight: weight, Color: th.Text}}
	}
	return spans
}

// drawTable draws the table in r, the box of its attachment, and keeps where
// its cells and the borders between its columns are for the pointer.
func (a *app) drawTable(p *ui.Painter, r ui.Rect, ch *chip) {
	m := ch.tbl
	th := p.Theme()
	cl := ryugo.Painting(p).Colors
	size := a.fontSize() * 0.95
	cw := ch.tw
	if a.colDrag.active && a.colDrag.ch == ch {
		cw = append([]float32(nil), cw...)
		cw[a.colDrag.col] = max(a.colDrag.w0+a.colDrag.dx, 36)
	}
	total := float32(0)
	for _, w := range cw {
		total += w
	}
	box := ui.Rect{X: r.X, Y: r.Y + tableGap, W: total + 2, H: r.H - 2*tableGap}
	p.Fill(box, th.Border, rad(9))
	inner := ui.Rect{X: box.X + 1, Y: box.Y + 1, W: box.W - 2, H: box.H - 2}
	p.Fill(inner, a.pageBg, rad(8))
	vis := m.visible()
	rowsH := float32(len(vis)+1) * tableRowH
	footH := float32(0)
	if m.hasCalc() {
		footH = tableFootH
	}
	p.Clip(inner, rad(8), func() {
		p.Fill(ui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: tableRowH}, th.Text.Alpha(0.055), 0)
		if m.hasCalc() {
			p.Fill(ui.Rect{X: inner.X, Y: inner.Y + rowsH, W: inner.W, H: tableFootH}, th.Text.Alpha(0.03), 0)
		}
		if m.Filter != "" {
			p.Fill(ui.Rect{X: inner.X, Y: inner.Y + rowsH + footH, W: inner.W, H: tableFootH}, cl.Accent.Alpha(0.07), 0)
		}
	})
	// the part of the table in view: a table of thousands of rows draws its rows in view
	viewTop, viewBot := float32(-1e9), float32(1e9)
	if a.editorR.H > 0 {
		viewTop, viewBot = a.editorR.Y-tableRowH, a.editorR.Y+a.editorR.H
	}
	line := func(y float32, alpha float32) {
		p.Fill(ui.Rect{X: inner.X, Y: y, W: inner.W, H: 1}, th.Border.Alpha(alpha), 0)
	}
	line(inner.Y+tableRowH-1, 1)
	// the columns' lines: each a border to be dragged
	x := inner.X
	for c := range m.cols {
		x += cw[c]
		if c < len(m.cols)-1 {
			hot := a.colDrag.active && a.colDrag.ch == ch && a.colDrag.col == c || a.hoverBorder == (borderKey{ch, c})
			p.Fill(ui.Rect{X: x, Y: inner.Y + 1, W: map[bool]float32{true: 2, false: 1}[hot], H: inner.H - 2}, map[bool]ui.Color{true: cl.Accent, false: th.Border.Alpha(0.5)}[hot], 0)
		}
		a.borderHits = append(a.borderHits, colBorder{ui.Rect{X: x - 4, Y: box.Y, W: 8, H: box.H}, ch, c, cw[c]})
	}
	for d := -1; d < len(vis); d++ {
		row := -1
		if d >= 0 {
			row = vis[d]
		}
		top := inner.Y + float32(d+1)*tableRowH
		if top+tableRowH < viewTop || top > viewBot {
			continue
		}
		if d > 0 {
			line(top-1, 0.6)
		}
		x := inner.X
		for c, col := range m.cols {
			cell := ui.Rect{X: x, Y: top, W: cw[c], H: tableRowH}
			x += cw[c]
			a.tableHits = append(a.tableHits, tableHit{cell, ch, row, c})
			a.drawCell(p, cell, m.cell(row, c), col, row < 0, size, th, cl)
		}
	}
	if m.hasCalc() && inner.Y+rowsH <= viewBot {
		line(inner.Y+rowsH-1, 1)
		x := inner.X
		for c, col := range m.cols {
			cell := ui.Rect{X: x, Y: inner.Y + rowsH, W: cw[c], H: tableFootH}
			x += cw[c]
			v := m.calc(c)
			if v == "" {
				continue
			}
			spans := []ui.Span{{Text: v, Size: size * 0.92, Weight: 620, Color: th.Text}}
			if col.Kind != kindCheck {
				spans = append([]ui.Span{{Text: strings.ToLower(calcNames[col.Calc]) + " ", Size: size * 0.8, Color: th.TextMuted}}, spans...)
			}
			a.placeText(p, cell, spans, col.Align, tableFootH)
		}
	}
	if m.Filter != "" && inner.Y+rowsH+footH <= viewBot {
		a.drawFilterStrip(p, ui.Rect{X: inner.X, Y: inner.Y + rowsH + footH, W: inner.W, H: tableFootH}, ch, len(vis), size, th, cl)
	}
}

// drawFilterStrip is the line under a table that has a filter: the words, how
// many rows show, and a cross that takes the filter away. It is a hit of its
// own (row -2 opens the words to be written, -3 is the cross).
func (a *app) drawFilterStrip(p *ui.Painter, strip ui.Rect, ch *chip, shown int, size float32, th *ui.Theme, cl ryugo.Colors) {
	m := ch.tbl
	p.Fill(ui.Rect{X: strip.X, Y: strip.Y - 1, W: strip.W, H: 1}, th.Border, 0)
	words := strings.TrimSpace(m.Filter)
	label := []ui.Span{{Text: "⌕  ", Size: size * 0.92, Color: cl.Accent}}
	if words == "" {
		label = append(label, ui.Span{Text: "Filtre…", Size: size * 0.86, Color: th.TextMuted})
	} else {
		label = append(label, ui.Span{Text: words, Size: size * 0.86, Weight: 560, Color: th.Text})
	}
	x := ui.Rect{X: strip.X + strip.W - 26, Y: strip.Y + (strip.H-22)/2, W: 22, H: 22}
	if words != "" {
		a.tableHits = append(a.tableHits, tableHit{x, ch, -3, 0})
		p.RichText(x.X+6, x.Y+2, 22, ui.Span{Text: "✕", Size: 12, Color: th.TextMuted})
	}
	a.tableHits = append(a.tableHits, tableHit{strip, ch, -2, 0})
	a.placeText(p, ui.Rect{X: strip.X, Y: strip.Y, W: strip.W - 26, H: strip.H}, label, 0, strip.H)
	if words != "" {
		count := []ui.Span{{Text: itoa(shown) + "/" + itoa(len(m.rows)) + " satır", Size: size * 0.8, Color: th.TextMuted}}
		lw, _ := p.MeasureText(4000, label...)
		cwid, _ := p.MeasureText(4000, count...)
		if lw+cwid+2*tablePadX+30 <= strip.W { // the count only where the words leave room
			a.placeText(p, ui.Rect{X: strip.X, Y: strip.Y, W: strip.W - 30, H: strip.H}, count, 2, strip.H)
		}
	}
}

// drawCell draws a cell: its text as it reads, a check as a box.
func (a *app) drawCell(p *ui.Painter, cell ui.Rect, text string, col tableCol, head bool, size float32, th *ui.Theme, cl ryugo.Colors) {
	if col.Kind == kindCheck && !head {
		const s = 17
		bx := ui.Rect{X: cell.X + (cell.W-s)/2, Y: cell.Y + (cell.H-s)/2, W: s, H: s}
		if col.Align == 0 {
			bx.X = cell.X + tablePadX
		} else if col.Align == 2 {
			bx.X = cell.X + cell.W - tablePadX - s
		}
		if text != "" {
			p.Fill(bx, cl.Accent, rad(5))
			p.RichText(bx.X+3, bx.Y+1, s, ui.Span{Text: "✓", Size: 12, Weight: 800, Color: ui.Hex("#ffffff")})
		} else {
			p.Fill(bx, th.Border, rad(5))
			p.Fill(ui.Rect{X: bx.X + 1.5, Y: bx.Y + 1.5, W: s - 3, H: s - 3}, a.pageBg, rad(4))
		}
		return
	}
	if !head {
		text = a.shownText(col, text)
	}
	a.placeText(p, cell, a.cellSpans(text, size, tableWeight(head), th, cl.Accent), col.Align, tableRowH)
}

// placeText draws spans in a cell, on one line, aligned as the column says.
func (a *app) placeText(p *ui.Painter, cell ui.Rect, spans []ui.Span, align int, h float32) {
	tw, tH := p.MeasureText(4000, spans...)
	tx := cell.X + tablePadX
	switch align {
	case 1:
		tx = cell.X + (cell.W-tw)/2
	case 2:
		tx = cell.X + cell.W - tablePadX - tw
	}
	tx = max(tx, cell.X+tablePadX)
	p.Clip(cell, 0, func() { p.RichText(tx, cell.Y+max(0, (h-tH)/2), 4000, spans...) })
}

// sealTables keeps what the editor's last painting drew of the tables, once
// it is drawn (the tables are drawn as it paints): a frame that paints nothing
// leaves the last painting's cells where they were for the pointer.
func (a *app) sealTables() {
	a.tableSeen = append(a.tableSeen[:0], a.tableHits...)
	a.borderSeen = append(a.borderSeen[:0], a.borderHits...)
	a.tableHits, a.borderHits = a.tableHits[:0], a.borderHits[:0]
}

// tableCellAt is the cell at (x, y) of the window, as the last frame drew it.
func (a *app) tableCellAt(x, y float32) *tableHit {
	for i := range a.tableSeen {
		h := &a.tableSeen[i]
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h
		}
	}
	return nil
}

// borderAt is the border at (x, y) of the window, as the last frame drew it.
func (a *app) borderAt(x, y float32) *colBorder {
	for i := range a.borderSeen {
		b := &a.borderSeen[i]
		if x >= b.r.X && x < b.r.X+b.r.W && y >= b.r.Y && y < b.r.Y+b.r.H {
			return b
		}
	}
	return nil
}

// ---- changing a table ----

// tablePos is where the table's object is in the text, or -1.
func (a *app) tablePos(ch *chip) int {
	for i, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if at := r.Attributes.Attachment; at != nil {
			if other, ok := at.Data.(*chip); ok && sameChip(other, ch) {
				return i
			}
		}
	}
	return -1
}

// editTable changes a table: a copy is changed, and takes the table's place
// in the text. It returns the table that is there now.
func (a *app) editTable(ch *chip, change func(m *tableModel)) *chip {
	pos := a.tablePos(ch)
	if pos < 0 {
		return ch
	}
	m := ch.tbl.clone()
	change(m)
	nc := a.tableChip(m)
	an, ca := a.doc.Selection()
	attrs := a.doc.Text().AttributesAt(pos)
	attrs.Attachment = a.tableAttachment(nc)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(pos, pos+1, string(richtext.ObjectReplacement), attrs)
		ed.Select(an, ca)
	})
	return nc
}

// insertTable puts a new table at start (the empty line the caret is on, or
// under the line it is in), with an empty paragraph after it to write on.
func (a *app) insertTable(start int) *chip { return a.insertTableOf(start, newTable(3, 2)) }

// insertTableOf puts the table m where insertTable would put a new one.
func (a *app) insertTableOf(start int, m *tableModel) *chip {
	t := a.doc.Text()
	ps, pe := t.ParagraphRange(min(start, t.Len()))
	at, prefix := ps, ""
	if pe > ps { // a line with text: the table goes under it
		at, prefix = pe, "\n"
	}
	ch := a.tableChip(m)
	first := at + utf8.RuneCountInString(prefix)
	attrs := richtext.Attributes{Attachment: a.tableAttachment(ch)}
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(at, at, richtext.NewText(prefix+string(richtext.ObjectReplacement)+"\n"))
		ed.ReplaceString(first, first+1, string(richtext.ObjectReplacement), attrs)
		ed.FormatParagraphs(first, first+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		ed.Select(first+2, first+2)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
	return ch
}

// ---- editing a cell ----

type cellEdit struct {
	open     bool
	ch       *chip
	row, col int
	text     string
	was      bool // the input has had the focus
	fresh    bool
	seen     bool // the caret has been drawn: lastLen is the letters it had
	lastLen  int
}

// openCell opens the cell h to be written in.
func (a *app) openCell(h *tableHit) {
	a.cellEd = cellEdit{open: true, ch: h.ch, row: h.row, col: h.col, text: cellText(h.ch.tbl, h.row, h.col), fresh: true}
}

// cellText is what a cell is written as: row -2 is the filter's words.
func cellText(m *tableModel, row, col int) string {
	if row == -2 {
		return strings.TrimSpace(m.Filter)
	}
	return m.cell(row, col)
}

// clickCell is a click on a cell: a check is ticked, the rest is opened.
func (a *app) clickCell(h *tableHit) {
	if h.row == -3 { // the cross of the filter
		a.cellEd.open = false
		a.editTable(h.ch, func(m *tableModel) { m.Filter = "" })
		return
	}
	if h.row >= 0 && h.ch.tbl.cols[h.col].Kind == kindCheck {
		a.cellEd.open = false
		a.editTable(h.ch, func(m *tableModel) {
			if m.rows[h.row][h.col] == "" {
				m.rows[h.row][h.col] = "1"
			} else {
				m.rows[h.row][h.col] = ""
			}
		})
		return
	}
	a.openCell(h)
}

// commitCell writes the cell as it was typed.
func (a *app) commitCell() {
	ce := &a.cellEd
	if !ce.open {
		return
	}
	text := strings.TrimSpace(strings.ReplaceAll(ce.text, "\n", " "))
	if text == cellText(ce.ch.tbl, ce.row, ce.col) {
		return
	}
	if ce.row == -2 {
		ce.ch = a.editTable(ce.ch, func(m *tableModel) { m.Filter = text })
		return
	}
	ce.ch = a.editTable(ce.ch, func(m *tableModel) { m.setCell(ce.row, ce.col, text) })
}

// stepCell goes on to another cell, writing this one: a row is added when it
// goes past the last. A check has no text to write, it is passed.
func (a *app) stepCell(dr, dc int) {
	ce := &a.cellEd
	a.commitCell()
	m := ce.ch.tbl
	vis := m.visible() // a step goes through the rows that show
	d := -1
	for i, r := range vis {
		if r == ce.row {
			d = i
		}
	}
	d, col := d+dr, ce.col+dc
	for dc != 0 {
		if col >= len(m.cols) {
			col, d = 0, d+1
		} else if col < 0 {
			col, d = len(m.cols)-1, d-1
		}
		if d < 0 || m.cols[col].Kind != kindCheck {
			break
		}
		col += dc
	}
	d = max(d, -1)
	if d >= len(vis) {
		ce.ch = a.editTable(ce.ch, func(m *tableModel) { m.addRow(len(m.rows) - 1) })
		m = ce.ch.tbl
		vis = m.visible()
		d = len(vis) - 1
	}
	row := -1
	if d >= 0 {
		row = vis[d]
	}
	*ce = cellEdit{open: true, ch: ce.ch, row: row, col: col, text: m.cell(row, col), fresh: true}
}

// cellEditView is the box a cell is written in, over the cell, and under it
// the buttons for rows and columns, and for what the column is.
func (a *app) cellEditView(c *ui.Context) {
	ce := &a.cellEd
	if !ce.open || a.doc == nil || a.noPage() {
		ce.open = false
		return
	}
	var hit *tableHit
	for i := range a.tableSeen {
		h := &a.tableSeen[i]
		if h.ch == ce.ch && h.row == ce.row && h.col == ce.col {
			hit = h
		}
	}
	if hit == nil {
		if !ce.fresh {
			ce.open = false
		}
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	r := hit.r
	if c.Shortcut(0, ui.KeyEscape) {
		if ce.row == -2 && !ce.ch.tbl.filtering() && ce.ch.tbl.Filter != "" { // a filter that was never written
			a.editTable(ce.ch, func(m *tableModel) { m.Filter = "" })
		}
		ce.open = false
		return
	}
	// the box grows with its text, as a spreadsheet's does: the text never scrolls in it
	size := a.fontSize() * 0.95
	tw, _ := ui.ShapeRichTextWith([]ui.Span{{Text: ce.text, Size: size}}, a.font, ui.TextShape{}).Size()
	box := ui.Rect{X: r.X, Y: r.Y, W: max(r.W, tw+2*11+12), H: r.H}
	ui.Overlay(c, func() {
		in := ui.TextInputBase(c.Key("cell-in:"+strconv.Itoa(ce.row)+":"+strconv.Itoa(ce.col)), &ce.text).Absolute().Left(box.X).Top(box.Y).Size(box.W, box.H).
			Label("Hücre").FontSize(a.fontSize()*0.95).Padding(0, 11).Background(cl.Card).Radius(rad(3)).Border(1.5, cl.Accent).FocusRing(false).CaretWidth(0)
		if ce.fresh {
			in.AutoFocus()
			in.Focus()
			ce.fresh = false
		}
		if in.Focused() {
			ce.was = true
		}
		switch {
		case in.Submitted():
			a.commitCell()
			ce.open = false
			return
		case ce.row == -2: // the filter's words: only Enter and Esc
		case in.Shortcut(0, ui.KeyTab):
			a.stepCell(0, 1)
			return
		case in.Shortcut(ui.Shift, ui.KeyTab):
			a.stepCell(0, -1)
			return
		case in.Shortcut(0, ui.KeyDown):
			a.stepCell(1, 0)
			return
		case in.Shortcut(0, ui.KeyUp):
			a.stepCell(-1, 0)
			return
		}
		// the caret is ours, the slime or the mizan, as the page's
		if in.Focused() {
			anchor, caret := in.TextAnchorCaret()
			cr, cw := a.inputCaret(ce.text, caret, a.fontSize()*0.95, 0, r.X+11, r.Y, r.H)
			n := utf8.RuneCountInString(ce.text)
			wrote, edited := 0, false
			if ce.seen && n != ce.lastLen {
				edited, wrote = true, max(n-ce.lastLen, 0)
			}
			ce.seen, ce.lastLen = true, n
			ww, wh := c.Size()
			ui.Box(c.Key("cell-caret")).Absolute().Left(0).Top(0).Size(ww, wh).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
				a.paintCaret(p, cr, cw, box, kick{wrote: wrote, edited: edited}, anchor != caret)
			})
		}
		if ce.row == -2 {
			return
		}
		col := ce.ch.tbl.cols[ce.col]
		ui.Row(c.Key("cell-bar")).Absolute().Left(r.X).Top(r.Y+r.H+4).Gap(2).Padding(3).Radius(rad(10)).Background(cl.Card).Border(1, cl.Hairline).
			Children(func() {
				btn := func(glyph *ui.SVG, label string, danger bool, do func(m *tableModel)) {
					b := ui.Box(c.Key(label)).Size(26, 26).Radius(rad(7)).Center().Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					ryugo.Tooltip(c, b, label)
					color := cl.Muted
					if danger {
						color = cl.Up
					}
					if b.Hovered() {
						b.Background(cl.Fill)
					}
					b.Children(func() { ui.Icon(c, glyph).Width(16).Height(16).TextColor(color) })
					if b.Clicked() {
						a.commitCell()
						if do == nil { // not a change: the table is copied as it is
							a.copyText(ce.ch.tbl.csvText(','))
							a.notice = "Tablo CSV olarak kopyalandı"
						} else {
							a.editTable(ce.ch, do)
						}
						ce.open = false
					}
				}
				row, ci := ce.row, ce.col
				btn(iconRowAdd, "Alta satır ekle", false, func(m *tableModel) { m.addRow(row) })
				btn(iconColAdd, "Sağa sütun ekle", false, func(m *tableModel) { m.addCol(ci) })
				btn(iconRowDel, "Satırı sil", true, func(m *tableModel) { m.delRow(row) })
				btn(iconColDel, "Sütunu sil", true, func(m *tableModel) { m.delCol(ci) })
				btn(iconKind, "Sütun türü: "+kindNames[col.Kind], false, func(m *tableModel) { m.setKind(ci, nextOf(tableKinds, m.cols[ci].Kind)) })
				btn(iconAlign, "Hizalama", false, func(m *tableModel) { m.cols[ci].Align = (m.cols[ci].Align + 1) % 3 })
				btn(iconCalc, "Alt satır: "+calcNames[col.Calc], false, func(m *tableModel) { m.cols[ci].Calc = m.cols[ci].nextCalc() })
				sortLabel := "Bu sütuna göre sırala"
				if ce.ch.tbl.sortedAsc(ci) {
					sortLabel = "Bu sütuna göre sırala (azalan)"
				}
				btn(iconSort, sortLabel, false, func(m *tableModel) { m.toggleSort(ci) })
				filterLabel := "Filtrele"
				if ce.ch.tbl.Filter != "" {
					filterLabel = "Filtreyi düzenle"
				}
				fb := ui.Box(c.Key("cell-filter")).Size(26, 26).Radius(rad(7)).Center().Label(filterLabel).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				ryugo.Tooltip(c, fb, filterLabel)
				if fb.Hovered() {
					fb.Background(cl.Fill)
				}
				fb.Children(func() { ui.Icon(c, iconFilter).Width(16).Height(16).TextColor(cl.Muted) })
				if fb.Clicked() {
					a.commitCell()
					nc := a.editTable(ce.ch, func(m *tableModel) {
						if m.Filter == "" {
							m.Filter = " " // the strip shows, to be written in
						}
					})
					a.cellEd = cellEdit{open: true, ch: nc, row: -2, text: strings.TrimSpace(nc.tbl.Filter), fresh: true}
				}
				btn(iconCopy, "CSV olarak kopyala", false, nil)
			})
	})
}

var iconCopy = icon(`<rect x="8.5" y="8.5" width="11" height="11" rx="2.2"/><path d="M15.5 8.5v-2a2 2 0 0 0-2-2h-7a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h2"/>`)

// tableInPasted turns text a spreadsheet copied (rows by lines, cells by tabs),
// which has just come into the text from..to, into a table, the first row its
// header. It reports whether it did.
func (a *app) tableInPasted(from, to int) bool {
	t := a.doc.Text()
	if from < 0 || to > t.Len() || t.ParagraphStyleAt(from).CodeBlock {
		return false
	}
	s := t.Slice(from, to).String()
	if !tabular(s) || strings.Contains(s, string(richtext.ObjectReplacement)) {
		return false
	}
	m := tableFromCSV(s)
	if m == nil {
		return false
	}
	ps, _ := t.ParagraphRange(from)
	prefix := ""
	if ps < from { // text before it in its line: the table starts a line of its own
		prefix = "\n"
	}
	first := from + utf8.RuneCountInString(prefix)
	ch := a.tableChip(m)
	attrs := richtext.Attributes{Attachment: a.tableAttachment(ch)}
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(from, to, richtext.NewText(prefix+string(richtext.ObjectReplacement)+"\n"))
		ed.ReplaceString(first, first+1, string(richtext.ObjectReplacement), attrs)
		ed.FormatParagraphs(first, first+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		ed.Select(first+2, first+2)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
	return true
}

// pasteTable makes a table of the CSV (or tab-separated text) on the clipboard,
// where the caret is.
func (a *app) pasteTable() {
	read := a.clipRead
	if read == nil {
		read = mygo.Clipboard.ReadText
	}
	m := tableFromCSV(read())
	if m == nil {
		a.notice = "Panodaki metin bir tablo değil (hücreler virgül, noktalı virgül ya da tab ile ayrılmalı)"
		return
	}
	_, caret := a.doc.Selection()
	a.insertTableOf(caret, m)
}

var (
	iconRowAdd = icon(`<rect x="4.5" y="4.5" width="15" height="7" rx="2"/><path d="M12 15v5M9.5 17.5h5"/>`)
	iconColAdd = icon(`<rect x="4.5" y="4.5" width="7" height="15" rx="2"/><path d="M15 12h5M17.5 9.5v5"/>`)
	iconRowDel = icon(`<rect x="4.5" y="4.5" width="15" height="7" rx="2"/><path d="M9.5 17.5h5"/>`)
	iconColDel = icon(`<rect x="4.5" y="4.5" width="7" height="15" rx="2"/><path d="M15.5 12h4"/>`)
	iconKind   = icon(`<path d="M5 18 10 6l5 12M6.8 14h6.4M18 9v9"/>`)
	iconAlign  = icon(`<path d="M5 7h14M5 12h8M5 17h11"/>`)
	iconCalc   = icon(`<path d="M17 6H7l5 6-5 6h10"/>`)
	iconSort   = icon(`<path d="M8 5v14M5 16l3 3 3-3M16 19V5M13 8l3-3 3 3"/>`)
	iconFilter = icon(`<path d="M4.5 6h15l-5.8 7v5l-3.4 1.5V13z"/>`)
)

// dragColumns follows the pointer over the borders of the tables: the cursor
// there, and a drag of one, which resizes its column and, let go, keeps the
// width in the table.
func (a *app) dragColumns(ed ui.Element) {
	a.hoverBorder = borderKey{}
	x, y, over := ed.PointerPosition()
	px, py := a.editorR.X+x, a.editorR.Y+y
	if over && !a.colDrag.active {
		if b := a.borderAt(px, py); b != nil {
			a.hoverBorder = borderKey{b.ch, b.col}
			ed.Cursor(ui.CursorResizeEW)
			if a.edPressed {
				a.colDrag = colDragState{active: true, ch: b.ch, col: b.col, w0: b.w}
			}
		}
	}
	if !a.colDrag.active {
		return
	}
	ed.Cursor(ui.CursorResizeEW)
	if dx, _, ok := ed.Dragged(); ok {
		a.colDrag.dx += dx
	}
	if !a.edPressed { // let go
		d := a.colDrag
		a.colDrag = colDragState{}
		if d.dx != 0 {
			w := max(d.w0+d.dx, 36)
			a.editTable(d.ch, func(m *tableModel) { m.cols[d.col].W = w })
		}
		a.doc.SetSelection(a.selPrevA, a.selPrevC)
	}
}

// ---- the text and the file ----

// tablesIn turns the tables a page has into objects: the fenced blocks
// the app writes, and the Markdown tables other tools write (made tables of
// their own, written as such from now on).
func tablesIn(t richtext.Text) richtext.Text {
	ps := starts(t)
	for i, j := 0, len(ps)-1; i < j; i, j = i+1, j-1 {
		ps[i], ps[j] = ps[j], ps[i]
	}
	type found struct {
		s, e int
		m    *tableModel
	}
	var all []found
	for i := 0; i < len(ps); i++ {
		s, e := t.ParagraphRange(ps[i])
		st := t.ParagraphStyleAt(s)
		legacy := st.ID == rawIDPrefix+"table"
		if !st.CodeBlock || !(legacy || st.ID == codeIDPrefix+tableLang) {
			continue
		}
		lines := []string{t.Slice(s, e).String()}
		for i+1 < len(ps) {
			s2, e2 := t.ParagraphRange(ps[i+1])
			if st2 := t.ParagraphStyleAt(s2); !st2.CodeBlock || st2.ID != "" {
				break
			}
			lines = append(lines, t.Slice(s2, e2).String())
			e = e2
			i++
		}
		var m *tableModel
		if legacy {
			m = tableFromMarkdown(lines)
		} else if tm, ok := parseTable(strings.Join(lines, "\n")); ok {
			m = tm
		}
		if m != nil {
			all = append(all, found{s, e, m})
		}
	}
	for k := len(all) - 1; k >= 0; k-- {
		f := all[k]
		attrs := richtext.Attributes{Attachment: &richtext.Attachment{Name: "Tablo", Data: &chip{kind: "table", tbl: f.m}}}
		t = t.ReplaceString(f.s, f.e, string(richtext.ObjectReplacement), attrs)
		t = t.FormatParagraphs(f.s, f.s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
	}
	return t
}

// tableAsText puts a table's text where its object is at i in t, as a block
// of code the file has as its fenced block; a table with other text in its
// paragraph is parted from it.
func tableAsText(t richtext.Text, i int, ch *chip, plain richtext.Attributes) richtext.Text {
	ps, pe := t.ParagraphRange(i)
	txt := ch.tbl.text()
	if pe > i+1 { // text after it
		t = t.ReplaceString(i+1, i+1, "\n", plain)
	}
	if ps < i { // and before
		t = t.ReplaceString(i, i, "\n", plain)
		i++
	}
	t = t.ReplaceString(i, i+1, txt, plain)
	end := i + utf8.RuneCountInString(txt)
	t = t.FormatParagraphs(i, end, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{CodeBlock: true} })
	return t.FormatParagraphs(i, i, func(p *richtext.ParagraphStyle) { p.ID = codeIDPrefix + tableLang })
}
