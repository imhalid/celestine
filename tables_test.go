package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSplitCells(t *testing.T) {
	for in, want := range map[string][]string{
		"| a | b |":        {"a", "b"},
		"a | b":            {"a", "b"},
		"| a \\| b | c |":  {"a | b", "c"},
		"|  |  |  |":       {"", "", ""},
		"| :---: | ---: |": {":---:", "---:"},
		"| son boş | |":    {"son boş", ""},
	} {
		if got := splitCells(in); !equal(got, want) {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestTableModelFile(t *testing.T) {
	m := newTable(2, 2)
	m.Cols[0].Name = "Ad"
	m.Cols[1] = tableCol{Name: "Sayı", Kind: kindNumber, Align: 2, Calc: "sum"}
	m.Rows[0] = []string{"elma \"x\"", "3"}
	m.Rows[1] = []string{"armut", "12,5"}
	back, ok := parseTable(m.Text())
	if !ok || back.Text() != m.Text() || back.Rows[0][0] != "elma \"x\"" {
		t.Fatalf("%q -> %+v %v", m.Text(), back, ok)
	}
	if got := m.Calc(1); got != "15.5" {
		t.Fatalf("sum: %q", got)
	}
	m.Cols[1].Calc = "avg"
	if got := m.Calc(1); got != "7.75" {
		t.Fatalf("avg: %q", got)
	}
	if _, ok := parseTable("bu bir tablo değil"); ok {
		t.Fatal("read a table out of nothing")
	}
}

func TestTableModelEdits(t *testing.T) {
	m := newTable(2, 2)
	m.Rows[0][0], m.Rows[1][0] = "a", "b"
	c := m.Clone()
	c.AddRow(0)
	c.AddCol(0)
	if len(m.Rows) != 2 || len(m.Cols) != 2 || len(c.Rows) != 3 || len(c.Cols) != 3 || c.Rows[2][0] != "b" || c.Rows[1][0] != "" {
		t.Fatalf("clone shares: %+v / %+v", m, c)
	}
	c.DelCol(0)
	c.DelCol(0)
	c.DelCol(0)
	if len(c.Cols) != 1 {
		t.Fatalf("the last column went: %d", len(c.Cols))
	}
	m.SetKind(0, kindCheck)
	if m.Rows[0][0] != "1" || m.Cols[0].Align != 1 {
		t.Fatalf("check: %+v", m)
	}
	if got := m.Cols[0].NextCalc(); got != "count" {
		t.Fatalf("a check's calculations: %q", got)
	}
}

func TestTableFromMarkdown(t *testing.T) {
	m := tableFromMarkdown([]string{"| Ad | Sayı |", "| --- | ---: |", "| elma | 3 |", "| armut | 12 |"})
	if m == nil || len(m.Cols) != 2 || len(m.Rows) != 2 || m.Cols[1].Kind != kindNumber || m.Cols[1].Align != 2 || m.Cols[0].Kind != kindText {
		t.Fatalf("%+v", m)
	}
}

const tableMD = "önce\n\n| Ad | Sayı |\n| --- | ---: |\n| elma | 3 |\n| armut | 12 |\n\nsonra\n"

// A Markdown table other tools wrote is read as a table of ours, and is
// written as one: the fenced block, which reads back as it was.
func TestTableRead(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	tt.Frame()
	tt.Frame()
	md := a.pageMarkdown(a.pane().page, a.pane().doc.Text())
	if strings.Contains(md, "| elma |") || !strings.Contains(md, "```"+tableLang+"\n") || !strings.Contains(md, `["elma","3"]`) {
		t.Fatalf("written: %q", md)
	}
	if !strings.Contains(a.pane().doc.Text().String(), "önce") || strings.Contains(a.pane().doc.Text().String(), "elma") {
		t.Fatalf("the table is in the text: %q", a.pane().doc.Text().String())
	}
	if len(a.pane().tableSeen) != 6 { // the header and two rows, two cells each
		t.Fatalf("cells: %d", len(a.pane().tableSeen))
	}
	a.save()
	back := bodyMarkdown(a.parse(md))
	if strings.TrimSpace(back) != strings.TrimSpace(md) {
		t.Fatalf("not read back as it was:\n%q\n%q", md, back)
	}
}

// TestTableCellEdit: a click on a cell opens it; Enter writes it; Tab goes on
// and past the last cell adds a row; the buttons add, take away and say what a column is.
func TestTableCellEdit(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	cell := func(row, col int) tableHit {
		for _, h := range a.pane().tableSeen {
			if h.row == row && h.col == col {
				return h
			}
		}
		t.Fatalf("no cell %d,%d", row, col)
		return tableHit{}
	}
	model := func() *tableModel {
		for _, h := range a.pane().tableSeen {
			return h.ch.tbl
		}
		return nil
	}
	h := cell(0, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if !a.pane().cellEd.open || a.pane().cellEd.text != "elma" {
		t.Fatalf("not open: %+v", a.pane().cellEd)
	}
	if an, ca := a.pane().doc.Selection(); an != 0 || ca != 0 {
		t.Fatalf("the caret moved: %d %d", an, ca)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("kiraz")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if m := model(); m.Cell(0, 0) != "kiraz" || a.pane().cellEd.open {
		t.Fatalf("after Enter: %q open %v", m.Cell(0, 0), a.pane().cellEd.open)
	}
	// Tab goes to the next cell, and writes this one
	h = cell(0, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("7")
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	if !a.pane().cellEd.open || a.pane().cellEd.row != 1 || a.pane().cellEd.col != 0 || a.pane().cellEd.text != "armut" {
		t.Fatalf("after Tab: %+v", a.pane().cellEd)
	}
	if m := model(); m.Cell(0, 1) != "7" {
		t.Fatalf("Tab did not write: %+v", m.Rows)
	}
	// Tab past the last cell: a new row
	tt.Key(0, ui.KeyTab)
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.Rows) != 3 || a.pane().cellEd.row != 2 {
		t.Fatalf("no new row: %d rows, editing %+v", len(m.Rows), a.pane().cellEd)
	}
	// the buttons: a column, and then it is taken away; a row away
	if err := tt.Click("Sağa sütun ekle"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.Cols) != 3 {
		t.Fatalf("columns %d", len(m.Cols))
	}
	h = cell(-1, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Sütunu sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.Cols) != 2 {
		t.Fatalf("columns %d after delete", len(m.Cols))
	}
	h = cell(2, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Satırı sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.Rows) != 2 {
		t.Fatalf("rows %d after delete", len(m.Rows))
	}
	// the foot: a column that is a number sums
	h = cell(0, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if model().Cols[1].Kind != kindNumber {
		t.Fatalf("the column was read as %q", model().Cols[1].Kind)
	}
	if err := tt.Click("Alt satır: Hesap yok"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); m.Cols[1].Calc != "sum" || m.Calc(1) != "19" {
		t.Fatalf("foot: %q %q", m.Cols[1].Calc, m.Calc(1))
	}
	if md := a.pageMarkdown(a.pane().page, a.pane().doc.Text()); !strings.Contains(md, `"calc":"sum"`) || !strings.Contains(md, "sonra") {
		t.Fatalf("what is written: %q", md)
	}
	// undo brings back the table as it was
	a.pane().doc.Undo()
	tt.Frame()
	if m := model(); m.Cols[1].Calc != "" {
		t.Fatalf("undo left the foot: %+v", m.Cols)
	}
}

// TestTableCellLetGo: a click into the text, with a cell open, writes the cell and lets it go,
// and what is typed goes into the text.
func TestTableCellLetGo(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var h, last tableHit
	for _, s := range a.pane().tableSeen {
		if s.row == 0 && s.col == 0 {
			h = s
		}
		if s.r.Y >= last.r.Y {
			last = s
		}
	}
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("kiraz")
	tt.Advance(200 * time.Millisecond)
	if !a.pane().cellEd.open || a.pane().focused {
		t.Fatalf("the cell is not the one written in: open %v, text focused %v", a.pane().cellEd.open, a.pane().focused)
	}
	// "sonra", under the table
	tt.ClickAt(last.r.X+last.r.W/2, last.r.Y+last.r.H+3*a.fontSize())
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if a.pane().cellEd.open || !a.pane().focused {
		t.Fatalf("the table kept the focus: open %v, text focused %v", a.pane().cellEd.open, a.pane().focused)
	}
	if m := a.pane().tableSeen[0].ch.tbl; m.Cell(0, 0) != "kiraz" {
		t.Fatalf("the cell was not written: %q", m.Cell(0, 0))
	}
	tt.Type("z")
	tt.Advance(200 * time.Millisecond)
	if s := a.pane().doc.Text().String(); !strings.Contains(s, "z") || strings.Count(s, "kiraz") > 0 {
		t.Fatalf("the letter did not go into the text: %q", s)
	}
}

// TestTableCellWheel: the wheel over an open cell scrolls the page, and the cell goes with it.
func TestTableCellWheel(t *testing.T) {
	a, tt := newTestApp(t, tableMD+strings.Repeat("satır\n\n", 80))
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var h tableHit
	for _, s := range a.pane().tableSeen {
		if s.row == 0 && s.col == 0 {
			h = s
		}
	}
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("kiraz")
	tt.Advance(200 * time.Millisecond)
	tt.Wheel(h.r.X+h.r.W/2, h.r.Y+h.r.H/2, 0, 120)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if a.pane().pageScroll.Y < 100 || !a.pane().cellEd.open || a.pane().cellEd.text != "kiraz" {
		t.Fatalf("the page did not scroll under the cell: scrolled %v, %+v", a.pane().pageScroll.Y, a.pane().cellEd)
	}
	for _, s := range a.pane().tableSeen {
		if s.row == 0 && s.col == 0 && s.r.Y > h.r.Y-100 {
			t.Fatalf("the cell did not go with the page: %v, was %v", s.r.Y, h.r.Y)
		}
	}
	// far down, the cell out of sight, and back: it is open as it was
	for range 3 {
		tt.Wheel(500, 400, 0, 400)
		tt.Advance(500 * time.Millisecond)
	}
	tt.Frame()
	for range 3 {
		tt.Wheel(500, 400, 0, -400)
		tt.Advance(500 * time.Millisecond)
	}
	tt.Frame()
	if !a.pane().cellEd.open || a.pane().cellEd.text != "kiraz" {
		t.Fatalf("the cell was lost out of sight: %+v", a.pane().cellEd)
	}
}

// A check is ticked by a click, with no cell to write in.
func TestTableCheck(t *testing.T) {
	a, tt := newTestApp(t, "```"+tableLang+"\n"+`[{"name":"Bitti","kind":"check","align":1},{"name":"İş"}]`+"\n"+`["","a"]`+"\n```\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var h tableHit
	for _, c := range a.pane().tableSeen {
		if c.row == 0 && c.col == 0 {
			h = c
		}
	}
	if h.ch == nil {
		t.Fatal("no cell")
	}
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if a.pane().cellEd.open || !strings.Contains(a.pageMarkdown(a.pane().page, a.pane().doc.Text()), `["1","a"]`) {
		t.Fatalf("not ticked: open %v, %q", a.pane().cellEd.open, a.pageMarkdown(a.pane().page, a.pane().doc.Text()))
	}
}

// TestTableInserted: Ekle… ▸ Tablo makes a table, its first cell open.
func TestTableInserted(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(1000, 700)
	typeEach(tt, "/")
	tt.Advance(100 * time.Millisecond)
	typeEach(tt, "tablo")
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if !a.pane().cellEd.open || a.pane().cellEd.row != -1 || a.pane().cellEd.text != "Sütun 1" {
		t.Fatalf("cell: %+v", a.pane().cellEd)
	}
	if len(a.pane().tableSeen) != 9 { // three columns, a header and two rows
		t.Fatalf("cells drawn: %d", len(a.pane().tableSeen))
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Ad")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if md := a.pageMarkdown(a.pane().page, a.pane().doc.Text()); !strings.Contains(md, `[{"name":"Ad"},{"name":"Sütun 2"}`) {
		t.Fatalf("%q", md)
	}
}

func TestCellSpans(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	spans := a.cellSpans("**kalın** ve `kod` [[Sayfa|ad]] #etiket [bağ](http://x)", 15, 400, &ui.Theme{Text: ui.Hex("#000000")}, ui.Hex("#0000ff"))
	var text strings.Builder
	bold, code, accent, link := false, false, 0, false
	for _, s := range spans {
		text.WriteString(s.Text)
		bold = bold || (s.Text == "kalın" && s.Weight == 700)
		code = code || (s.Text == "kod" && s.Background != ui.Color{})
		if s.Color == ui.Hex("#0000ff") {
			accent++
		}
		link = link || (s.Text == "bağ" && s.Underline)
	}
	if text.String() != "kalın ve kod ad #etiket bağ" || !bold || !code || accent != 3 || !link {
		t.Fatalf("%q bold %v code %v accent %d link %v", text.String(), bold, code, accent, link)
	}
}

// TestTableColumnDrag: a column border is dragged; let go, its width is the table's.
func TestTableColumnDrag(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if len(a.pane().borderSeen) != 2 {
		t.Fatalf("borders: %d", len(a.pane().borderSeen))
	}
	b := a.pane().borderSeen[0]
	x0, y0 := b.r.X+b.r.W/2, b.r.Y+b.r.H/2
	tt.Move(x0, y0)
	tt.Frame()
	tt.Press(x0, y0)
	for k := 1; k <= 8; k++ {
		tt.Move(x0+float32(k)*10, y0)
		tt.Frame()
	}
	tt.Release(x0+80, y0)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	var w float32
	for _, h := range a.pane().tableSeen {
		w = h.ch.tbl.Cols[0].W
	}
	if w < b.w+60 || a.pane().colDrag.active {
		t.Fatalf("the column is %v wide, was %v", w, b.w)
	}
	if an, ca := a.pane().doc.Selection(); an != 0 || ca != 0 {
		t.Fatalf("the caret moved: %d %d", an, ca)
	}
}

// The keyboard goes through a table and out of it: ↓ from the line above opens the header's first
// cell, ← and → at the edges of a cell's text go to the cell beside it, ↓ past the last row and ↑
// over the header go back to the text, under and above the table; ↑ from the line under it opens
// its last row.
func TestTableArrowKeys(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var ch *chip
	for _, h := range a.pane().tableSeen {
		ch = h.ch
	}
	pos := a.tablePos(ch)
	if pos < 0 {
		t.Fatal("no table in the text")
	}
	a.pane().doc.SetSelection(pos, pos)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Advance(200 * time.Millisecond)
	at := func(row, col int, text, when string) {
		t.Helper()
		if !a.pane().cellEd.open || a.pane().cellEd.row != row || a.pane().cellEd.col != col || a.pane().cellEd.text != text {
			t.Fatalf("%s: %+v, want %d,%d %q", when, a.pane().cellEd, row, col, text)
		}
	}
	at(-1, 0, "Ad", "↓ into the table")
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyRight)
	tt.Advance(100 * time.Millisecond)
	at(-1, 1, "Sayı", "→ at the end of the text")
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyLeft)
	tt.Advance(100 * time.Millisecond)
	at(-1, 0, "Ad", "← at the start of the text")
	tt.Key(0, ui.KeyRight) // in the middle of the text: the caret moves, not the cell
	tt.Advance(100 * time.Millisecond)
	at(-1, 0, "Ad", "→ inside the text")
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	tt.Advance(100 * time.Millisecond)
	at(1, 0, "armut", "↓ ↓")
	tt.Key(0, ui.KeyDown)
	tt.Advance(200 * time.Millisecond)
	if a.pane().cellEd.open {
		t.Fatalf("↓ past the last row stayed in the table: %+v", a.pane().cellEd)
	}
	if an, ca := a.pane().doc.Selection(); an != pos+1 || ca != pos+1 {
		t.Fatalf("out of the table the caret is at %d,%d, not after it (%d)", an, ca, pos+1)
	}
	if n := len(ch.tbl.Rows); n != 2 {
		t.Fatalf("↓ added a row: %d", n)
	}
	tt.Key(0, ui.KeyUp)
	tt.Advance(200 * time.Millisecond)
	at(1, 0, "armut", "↑ from under the table")
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Advance(200 * time.Millisecond)
	if a.pane().cellEd.open {
		t.Fatalf("↑ over the header stayed in the table: %+v", a.pane().cellEd)
	}
	if an, ca := a.pane().doc.Selection(); an != pos || ca != pos {
		t.Fatalf("out of the top the caret is at %d,%d, not before the table (%d)", an, ca, pos)
	}
}

// The bar under a cell being written takes the pointer from the text it is
// drawn over: going along its buttons, the cell, the column's border or a
// chip under it does not light up or change the cursor.
func TestTableCellBarTakesPointer(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	for _, h := range a.pane().tableSeen {
		if h.row == 0 && h.col == 0 {
			tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
		}
	}
	tt.Advance(300 * time.Millisecond)
	b, ok := tt.Find("Sağa sütun ekle")
	if !ok || !a.pane().cellEd.open {
		t.Fatalf("no bar: %v %v", ok, a.pane().cellEd.open)
	}
	if a.tableCellAt(a.pane(), b.X+b.W/2, b.Y+b.H/2) == nil {
		t.Fatal("no cell under the bar: the test shows nothing")
	}
	a.edPtr = ui.Point{X: -1, Y: -1}
	for x := b.X + 2; x < b.X+b.W-2; x += 4 {
		tt.Move(x, b.Y+b.H/2)
		tt.Advance(16 * time.Millisecond)
	}
	if a.edPtr != (ui.Point{X: -1, Y: -1}) || a.hoverBorder != (borderKey{}) {
		t.Fatalf("the text took the pointer under the bar: at %v, border %v", a.edPtr, a.hoverBorder)
	}
}
