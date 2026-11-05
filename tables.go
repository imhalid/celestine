package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// Tables: a table is a raw block (blocks.go), its lines paragraphs of Markdown
// as the file has them. While the caret is not in it, it is drawn as a table
// over those lines - a header, rows, columns as wide as their words - and a
// click on a cell opens the cell to be written in, a Tab on to the next, with
// a few buttons to add and take away rows and columns. With the caret in it,
// the lines show as they are, to be edited as Markdown.

var tableSep = regexp.MustCompile(`^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$`)

// tableBlock is a table read from the text.
type tableBlock struct {
	start, end int      // the runes it takes, from its first paragraph to the end of its last
	paras      [][2]int // each line's paragraph: its first rune and its end
	cells      [][]string
	aligns     []int // by column: 0 start, 1 center, 2 end
	ncols      int
}

// splitCells reads a table line into its cells: the pipes at its edges are
// not cells, "\|" is a pipe in one.
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

// joinCells writes cells as a table line.
func joinCells(cells []string) string {
	var sb strings.Builder
	sb.WriteString("|")
	for _, c := range cells {
		sb.WriteString(" " + strings.ReplaceAll(c, "|", `\|`) + " |")
	}
	return sb.String()
}

// tableBlockAt reads the table the paragraph at pos is in, if it is one.
func tableBlockAt(t richtext.Text, pos int) (tableBlock, bool) {
	var tb tableBlock
	if pos < 0 || pos > t.Len() {
		return tb, false
	}
	ps, _ := t.ParagraphRange(pos)
	for {
		st := t.ParagraphStyleAt(ps)
		if !st.CodeBlock {
			return tb, false
		}
		if st.ID == rawIDPrefix+"table" {
			break
		}
		if st.ID != "" || ps == 0 {
			return tb, false
		}
		ps, _ = t.ParagraphRange(ps - 1)
	}
	tb.start = ps
	for at := ps; at <= t.Len(); {
		s, e := t.ParagraphRange(at)
		if s != at {
			break
		}
		if len(tb.paras) > 0 && (!t.ParagraphStyleAt(s).CodeBlock || t.ParagraphStyleAt(s).ID != "") {
			break
		}
		tb.paras = append(tb.paras, [2]int{s, e})
		tb.cells = append(tb.cells, splitCells(t.Slice(s, e).String()))
		tb.end = e
		at = e + 1
	}
	if len(tb.cells) < 2 || !tableSep.MatchString(t.Slice(tb.paras[1][0], tb.paras[1][1]).String()) {
		return tb, false
	}
	for _, row := range tb.cells {
		tb.ncols = max(tb.ncols, len(row))
	}
	tb.aligns = make([]int, tb.ncols)
	for i, c := range tb.cells[1] {
		if i >= tb.ncols {
			break
		}
		switch {
		case strings.HasPrefix(c, ":") && strings.HasSuffix(c, ":"):
			tb.aligns[i] = 1
		case strings.HasSuffix(c, ":"):
			tb.aligns[i] = 2
		}
	}
	return tb, true
}

// cellAt is the text of a cell, "" past the end of a short row.
func (tb tableBlock) cellAt(line, col int) string {
	if line < len(tb.cells) && col < len(tb.cells[line]) {
		return tb.cells[line][col]
	}
	return ""
}

// ---- drawing ----

// tableHit is a cell drawn, where in the window.
type tableHit struct {
	r         ui.Rect
	start     int // the table, by where it starts
	line, col int // the line of the table it is in (0 the header; 1 is the dashes) and its column
}

// tableActive reports whether the caret or the selection is in the table, so
// that it shows as Markdown.
func (a *app) tableActive(tb tableBlock) bool {
	if a.tableKeep == tb.start+1 {
		return false
	}
	if a.cellEd.open && a.cellEd.start == tb.start {
		return false
	}
	an, ca := a.doc.Selection()
	lo, hi := min(an, ca), max(an, ca)
	return hi >= tb.start && lo <= tb.end
}

// drawTables draws the tables in view over their lines, those the caret is
// not in.
func (a *app) drawTables(p *ui.Painter, r ui.Rect) {
	a.tableHits, a.borderHits = a.tableHits[:0], a.borderHits[:0]
	defer func() { // what the last drawing drew
		a.tableSeen = append(a.tableSeen[:0], a.tableHits...)
		a.borderSeen = append(a.borderSeen[:0], a.borderHits...)
	}()
	if a.doc == nil {
		return
	}
	text := a.doc.Text()
	byStart := map[int]richtext.ParagraphBox{}
	for _, b := range a.boxes {
		byStart[b.Start] = b
	}
	th := p.Theme()
	cl := ryugo.Painting(p).Colors
	done := map[int]bool{}
	for _, b := range a.shown(r) {
		if b.Start > text.Len() {
			continue
		}
		kind, _ := rawKindAt(text, b.Start)
		if kind != "table" {
			continue
		}
		tb, ok := tableAtCached(a.doc, b.Start)
		if !ok || done[tb.start] {
			continue
		}
		done[tb.start] = true
		if a.tableActive(tb) {
			continue
		}
		boxes := make([]richtext.ParagraphBox, len(tb.paras))
		missing := false
		for i, pr := range tb.paras {
			bx, ok := byStart[pr[0]]
			if !ok {
				missing = true
				break
			}
			boxes[i] = bx
		}
		if missing {
			continue
		}
		a.drawTable(p, r, tb, boxes, th, cl)
	}
}

// dashUnit is how wide a dash of the separator line makes a column, when the
// line has other than the three dashes of "---": a resized column keeps its
// width in the file, in its dashes, as any other tool leaves them.
const dashUnit = 8

// colHint is the width the separator line gives column c, or 0 for none.
func (tb tableBlock) colHint(c int) float32 {
	if len(tb.cells) < 2 || c >= len(tb.cells[1]) {
		return 0
	}
	n := strings.Count(tb.cells[1][c], "-")
	if n == 3 || n == 0 {
		return 0
	}
	return float32(n * dashUnit)
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

// colBorder is the line between two columns, where it can be dragged.
type colBorder struct {
	r     ui.Rect
	start int // the table
	col   int // the column on its left
	w     float32
}

func (a *app) drawTable(p *ui.Painter, r ui.Rect, tb tableBlock, boxes []richtext.ParagraphBox, th *ui.Theme, cl ryugo.Colors) {
	size := a.fontSize() * 0.95
	const padX = 12
	// the columns are as wide as their widest cell, or as the dashes say
	cw := make([]float32, tb.ncols)
	for c := 0; c < tb.ncols; c++ {
		if h := tb.colHint(c); h > 0 {
			cw[c] = h
			continue
		}
		for line := range tb.cells {
			if line == 1 {
				continue
			}
			w, _ := p.MeasureText(4000, a.cellSpans(tb.cellAt(line, c), size, map[bool]int{true: 650, false: 400}[line == 0], th, cl.Accent)...)
			cw[c] = max(cw[c], w+2*padX)
		}
		cw[c] = max(cw[c], 72)
	}
	if live, ok := a.colLive[tb.start]; ok && a.colDrag.active { // being dragged
		cw[live.col] = max(live.w, 36)
	}
	total := float32(0)
	for c := range cw {
		total += cw[c]
	}
	if total > r.W {
		for c := range cw {
			cw[c] *= r.W / total
		}
		total = r.W
	}
	top := r.Y + boxes[0].Top
	last := boxes[len(boxes)-1]
	bottom := r.Y + last.Top + last.Height
	// what was there is covered, the gaps richtext leaves between blocks too
	p.Fill(ui.Rect{X: r.X - 2, Y: top - 9, W: r.W + 4, H: bottom - top + 18}, a.pageBg, 0)
	box := ui.Rect{X: r.X, Y: top, W: total, H: bottom - top}
	p.Fill(box, th.Border, 9)
	inner := ui.Rect{X: box.X + 1, Y: box.Y + 1, W: box.W - 2, H: box.H - 2}
	p.Fill(inner, a.pageBg, 8)
	// the header: its line and the dashes' line, as one band
	headBot := r.Y + boxes[1].Top + boxes[1].Height
	head := ui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: headBot - inner.Y}
	p.Clip(inner, 8, func() {
		p.Fill(head, th.Text.Alpha(0.055), 0)
		p.Fill(ui.Rect{X: inner.X, Y: headBot - 1, W: inner.W, H: 1}, th.Border, 0)
	})
	// the rows' lines
	for i := 2; i < len(boxes)-1; i++ {
		y := r.Y + boxes[i].Top + boxes[i].Height + (boxes[i+1].Top-boxes[i].Top-boxes[i].Height)/2
		p.Fill(ui.Rect{X: inner.X, Y: y, W: inner.W, H: 1}, th.Border.Alpha(0.6), 0)
	}
	// and the columns': each a border to be dragged
	x := box.X
	for c := 0; c < tb.ncols; c++ {
		x += cw[c]
		if c < tb.ncols-1 {
			hot := a.colDrag.active && a.colDrag.start == tb.start && a.colDrag.col == c || a.hoverBorder == (borderKey{tb.start + 1, c})
			p.Fill(ui.Rect{X: x, Y: inner.Y + 1, W: map[bool]float32{true: 2, false: 1}[hot], H: inner.H - 2}, map[bool]ui.Color{true: cl.Accent, false: th.Border.Alpha(0.5)}[hot], 0)
		}
		a.borderHits = append(a.borderHits, colBorder{ui.Rect{X: x - 4, Y: box.Y, W: 8, H: box.H}, tb.start, c, cw[c]})
	}
	// the cells
	for line := range tb.cells {
		if line == 1 {
			continue
		}
		rowTop, rowBot := r.Y+boxes[line].Top, r.Y+boxes[line].Top+boxes[line].Height
		if line == 0 {
			rowBot = headBot
		}
		x := box.X
		for c := 0; c < tb.ncols; c++ {
			cell := ui.Rect{X: x, Y: rowTop, W: cw[c], H: rowBot - rowTop}
			spans := a.cellSpans(tb.cellAt(line, c), size, map[bool]int{true: 650, false: 400}[line == 0], th, cl.Accent)
			tw, tH := p.MeasureText(max(cell.W-2*padX, 8), spans...)
			tx := cell.X + padX
			switch tb.aligns[c] {
			case 1:
				tx = cell.X + (cell.W-tw)/2
			case 2:
				tx = cell.X + cell.W - padX - tw
			}
			p.Clip(cell, 0, func() { p.RichText(tx, cell.Y+max(0, (cell.H-tH)/2), max(cell.W-2*padX, 8)+1, spans...) })
			a.tableHits = append(a.tableHits, tableHit{cell, tb.start, line, c})
			x += cw[c]
		}
	}
}

// borderKey names a border: a table (its start, and one more, so that the
// zero is none), and the column to its left.
type borderKey struct{ start, col int }

// colDragState is a column being resized.
type colDragState struct {
	active     bool
	start, col int
	w0, dx     float32
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

// widenColumn writes the width of column col as dashes in the separator line.
func (a *app) widenColumn(start, col int, w float32) {
	tb, ok := tableBlockAt(a.doc.Text(), start)
	if !ok {
		return
	}
	row := a.paddedRow(tb, 1)
	n := max(4, int(w/dashUnit+0.5))
	if n == 3 {
		n = 4
	}
	cell := strings.Repeat("-", n)
	switch tb.aligns[col] {
	case 1:
		cell = ":" + cell + ":"
	case 2:
		cell += ":"
	}
	row[col] = cell
	a.setLine(tb, 1, joinCells(row))
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

// ---- editing a cell ----

type cellEdit struct {
	open      bool
	start     int
	line, col int
	text      string
	was       bool // the input has had the focus
	fresh     bool
}

// openCell opens the cell h to be written in.
func (a *app) openCell(h *tableHit) {
	tb, ok := tableBlockAt(a.doc.Text(), h.start)
	if !ok {
		return
	}
	a.cellEd = cellEdit{open: true, start: h.start, line: h.line, col: h.col, text: tb.cellAt(h.line, h.col), fresh: true}
}

// commitCell writes the cell as it was typed.
func (a *app) commitCell() {
	ce := &a.cellEd
	if !ce.open {
		return
	}
	tb, ok := tableBlockAt(a.doc.Text(), ce.start)
	if !ok || ce.line >= len(tb.cells) {
		return
	}
	if strings.TrimSpace(ce.text) == tb.cellAt(ce.line, ce.col) {
		return
	}
	row := a.paddedRow(tb, ce.line)
	row[ce.col] = strings.TrimSpace(strings.ReplaceAll(ce.text, "\n", " "))
	a.setLine(tb, ce.line, joinCells(row))
}

// paddedRow is a line's cells, as many as the table has columns.
func (a *app) paddedRow(tb tableBlock, line int) []string {
	row := append([]string{}, tb.cells[line]...)
	for len(row) < tb.ncols {
		row = append(row, "")
	}
	return row
}

// setLine writes line i of the table as text.
func (a *app) setLine(tb tableBlock, i int, text string) {
	pr := tb.paras[i]
	a.doc.Edit(func(ed *richtext.Edit) { ed.ReplaceString(pr[0], pr[1], text, richtext.Attributes{}) })
}

// dashes is a separator cell of the width given, as the alignment says.
func dashes(align int) string {
	return map[int]string{0: "---", 1: ":---:", 2: "---:"}[align]
}

// addRow puts an empty row under line (the header's puts it first).
func (a *app) addRow(tb tableBlock, line int) {
	if line < 1 {
		line = 1
	}
	pr := tb.paras[line]
	blank := make([]string, tb.ncols)
	row := joinCells(blank)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(pr[1], pr[1], richtext.NewText("\n"+row))
		ed.FormatParagraphs(pr[1]+1, pr[1]+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{CodeBlock: true} })
	})
}

// deleteRow takes a body row away.
func (a *app) deleteRow(tb tableBlock, line int) {
	if line < 2 {
		return
	}
	pr := tb.paras[line]
	a.doc.Edit(func(ed *richtext.Edit) {
		if line == len(tb.paras)-1 {
			ed.Replace(pr[0]-1, pr[1], richtext.Text{})
		} else {
			ed.Replace(pr[0], pr[1]+1, richtext.Text{})
		}
	})
}

// addCol puts an empty column after col, in every line.
func (a *app) addCol(tb tableBlock, col int) {
	for i := len(tb.paras) - 1; i >= 0; i-- {
		row := a.paddedRow(tb, i)
		ins := ""
		if i == 1 {
			ins = "---"
		}
		row = append(row[:col+1], append([]string{ins}, row[col+1:]...)...)
		a.setLine(tb, i, joinCells(row))
	}
}

// deleteCol takes a column away, unless it is the only one.
func (a *app) deleteCol(tb tableBlock, col int) {
	if tb.ncols < 2 {
		return
	}
	for i := len(tb.paras) - 1; i >= 0; i-- {
		row := a.paddedRow(tb, i)
		row = append(row[:col], row[col+1:]...)
		a.setLine(tb, i, joinCells(row))
	}
}

// cellEditView is the box a cell is written in, over the cell, and under it
// the buttons for rows and columns.
func (a *app) cellEditView(c *ui.Context) {
	ce := &a.cellEd
	if !ce.open || a.doc == nil || a.noPage() {
		ce.open = false
		return
	}
	var hit *tableHit
	for i := range a.tableSeen {
		h := &a.tableSeen[i]
		if h.start == ce.start && h.line == ce.line && h.col == ce.col {
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
		ce.open = false
		return
	}
	step := func(dl, dc int) { // on to another cell, a row added when it is past the last
		a.commitCell()
		tb, ok := tableBlockAt(a.doc.Text(), ce.start)
		if !ok {
			ce.open = false
			return
		}
		line, col := ce.line, ce.col+dc
		if dc != 0 {
			if col >= tb.ncols {
				col, line = 0, line+1
				if line == 1 {
					line = 2
				}
			} else if col < 0 {
				col, line = tb.ncols-1, line-1
				if line == 1 {
					line = 0
				}
			}
		} else {
			line += dl
		}
		if line < 0 {
			line = 0
		}
		if line >= len(tb.paras) {
			a.addRow(tb, len(tb.paras)-1)
			tb, _ = tableBlockAt(a.doc.Text(), ce.start)
		}
		*ce = cellEdit{open: true, start: ce.start, line: line, col: col, text: tb.cellAt(line, col), fresh: true}
	}
	ui.Overlay(c, func() {
		in := ui.TextInputBase(c.Key("cell-in:"+strconv.Itoa(ce.line)+":"+strconv.Itoa(ce.col)), &ce.text).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
			Label("Hücre").FontSize(a.fontSize()*0.95).Padding(0, 11).Background(cl.Card).Radius(3).Border(1.5, cl.Accent).FocusRing(false)
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
		case in.Shortcut(0, ui.KeyTab):
			step(0, 1)
			return
		case in.Shortcut(ui.Shift, ui.KeyTab):
			step(0, -1)
			return
		case in.Shortcut(0, ui.KeyDown):
			step(1, 0)
			return
		case in.Shortcut(0, ui.KeyUp):
			step(-1, 0)
			return
		}
		tb, ok := tableBlockAt(a.doc.Text(), ce.start)
		if !ok {
			ce.open = false
			return
		}
		bar := ui.Row(c.Key("cell-bar")).Absolute().Left(r.X).Top(r.Y+r.H+4).Gap(2).Padding(3).Radius(10).Background(cl.Card).Border(1, cl.Hairline).
			Children(func() {
				btn := func(glyph *ui.SVG, label string, danger bool, do func()) {
					b := ui.Box(c.Key(label)).Size(26, 26).Radius(7).Center().Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					ryugo.Tooltip(c, b, label)
					col := cl.Muted
					if danger {
						col = cl.Up
					}
					if b.Hovered() {
						b.Background(cl.Fill)
					}
					b.Children(func() { ui.Icon(c, glyph).Width(16).Height(16).TextColor(col) })
					if b.Clicked() {
						a.commitCell()
						tb2, ok := tableBlockAt(a.doc.Text(), ce.start)
						if ok {
							_ = tb2
							do()
						}
						ce.open = false
					}
				}
				line, col := ce.line, ce.col
				btn(iconRowAdd, "Alta satır ekle", false, func() { a.addRow(a.freshTable(ce.start), line) })
				btn(iconColAdd, "Sağa sütun ekle", false, func() { a.addCol(a.freshTable(ce.start), col) })
				btn(iconRowDel, "Satırı sil", true, func() { a.deleteRow(a.freshTable(ce.start), line) })
				btn(iconColDel, "Sütunu sil", true, func() { a.deleteCol(a.freshTable(ce.start), col) })
				btn(iconEdit, "Markdown olarak düzenle", false, func() {
					a.doc.SetSelection(tb.start, tb.start)
					a.refocus = true
				})
			})
		_ = bar
	})
}

// freshTable is the table at start as the text has it now.
func (a *app) freshTable(start int) tableBlock {
	tb, _ := tableBlockAt(a.doc.Text(), start)
	return tb
}

var (
	iconRowAdd = icon(`<rect x="4.5" y="4.5" width="15" height="7" rx="2"/><path d="M12 15v5M9.5 17.5h5"/>`)
	iconColAdd = icon(`<rect x="4.5" y="4.5" width="7" height="15" rx="2"/><path d="M15 12h5M17.5 9.5v5"/>`)
	iconRowDel = icon(`<rect x="4.5" y="4.5" width="15" height="7" rx="2"/><path d="M9.5 17.5h5"/>`)
	iconColDel = icon(`<rect x="4.5" y="4.5" width="7" height="15" rx="2"/><path d="M15.5 12h4"/>`)
)

// dragColumns follows the pointer over the borders of the tables: the cursor
// there, and a drag of one, which resizes its column and, let go, writes the
// width into the table's dashes.
func (a *app) dragColumns(ed ui.Element) {
	a.hoverBorder = borderKey{}
	x, y, over := ed.PointerPosition()
	px, py := a.editorR.X+x, a.editorR.Y+y
	if over && !a.colDrag.active {
		if b := a.borderAt(px, py); b != nil {
			a.hoverBorder = borderKey{b.start + 1, b.col}
			ed.Cursor(ui.CursorResizeEW)
			if a.edPressed {
				a.colDrag = colDragState{active: true, start: b.start, col: b.col, w0: b.w}
				a.tableKeep = b.start + 1
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
	d := a.colDrag
	if a.colLive == nil {
		a.colLive = map[int]colBorder{}
	}
	a.colLive[d.start] = colBorder{start: d.start, col: d.col, w: d.w0 + d.dx}
	if !a.edPressed { // let go
		a.colDrag = colDragState{}
		delete(a.colLive, d.start)
		if d.dx != 0 {
			a.widenColumn(d.start, d.col, max(d.w0+d.dx, 36))
		}
		a.doc.SetSelection(a.selPrevA, a.selPrevC)
	}
}

// tableMemo keeps the tables read from one version of a document, by every
// rune a line of one starts at: drawTables asks for each line in view, every
// frame, and reading a table of a thousand lines takes seconds.
var tableMemo struct {
	doc  *richtext.Document
	ver  uint64
	hits map[int]*tableBlock // a line's first rune -> its table
	none map[int]bool        // the paragraphs that are no table
}

// tableAtCached is tableBlockAt, as it reads it, for the document doc as it is.
func tableAtCached(doc *richtext.Document, pos int) (tableBlock, bool) {
	m := &tableMemo
	if ver := doc.Version(); m.doc != doc || m.ver != ver || m.hits == nil {
		m.doc, m.ver, m.hits, m.none = doc, ver, map[int]*tableBlock{}, map[int]bool{}
	}
	if tb := m.hits[pos]; tb != nil {
		return *tb, true
	}
	if m.none[pos] {
		return tableBlock{}, false
	}
	tb, ok := tableBlockAt(doc.Text(), pos)
	if !ok {
		m.none[pos] = true
		return tb, false
	}
	for _, pr := range tb.paras {
		m.hits[pr[0]] = &tb
	}
	m.hits[pos] = &tb
	return tb, true
}
