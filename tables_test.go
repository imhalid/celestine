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
	m.cols[0].Name = "Ad"
	m.cols[1] = tableCol{Name: "Sayı", Kind: kindNumber, Align: 2, Calc: "sum"}
	m.rows[0] = []string{"elma \"x\"", "3"}
	m.rows[1] = []string{"armut", "12,5"}
	back, ok := parseTable(m.text())
	if !ok || back.text() != m.text() || back.rows[0][0] != "elma \"x\"" {
		t.Fatalf("%q -> %+v %v", m.text(), back, ok)
	}
	if got := m.calc(1); got != "15.5" {
		t.Fatalf("sum: %q", got)
	}
	m.cols[1].Calc = "avg"
	if got := m.calc(1); got != "7.75" {
		t.Fatalf("avg: %q", got)
	}
	if _, ok := parseTable("bu bir tablo değil"); ok {
		t.Fatal("read a table out of nothing")
	}
}

func TestTableModelEdits(t *testing.T) {
	m := newTable(2, 2)
	m.rows[0][0], m.rows[1][0] = "a", "b"
	c := m.clone()
	c.addRow(0)
	c.addCol(0)
	if len(m.rows) != 2 || len(m.cols) != 2 || len(c.rows) != 3 || len(c.cols) != 3 || c.rows[2][0] != "b" || c.rows[1][0] != "" {
		t.Fatalf("clone shares: %+v / %+v", m, c)
	}
	c.delCol(0)
	c.delCol(0)
	c.delCol(0)
	if len(c.cols) != 1 {
		t.Fatalf("the last column went: %d", len(c.cols))
	}
	m.setKind(0, kindCheck)
	if m.rows[0][0] != "1" || m.cols[0].Align != 1 {
		t.Fatalf("check: %+v", m)
	}
	if got := m.cols[0].nextCalc(); got != "count" {
		t.Fatalf("a check's calculations: %q", got)
	}
}

func TestTableFromMarkdown(t *testing.T) {
	m := tableFromMarkdown([]string{"| Ad | Sayı |", "| --- | ---: |", "| elma | 3 |", "| armut | 12 |"})
	if m == nil || len(m.cols) != 2 || len(m.rows) != 2 || m.cols[1].Kind != kindNumber || m.cols[1].Align != 2 || m.cols[0].Kind != kindText {
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
	md := a.pageMarkdown(a.page, a.doc.Text())
	if strings.Contains(md, "| elma |") || !strings.Contains(md, "```"+tableLang+"\n") || !strings.Contains(md, `["elma","3"]`) {
		t.Fatalf("written: %q", md)
	}
	if !strings.Contains(a.doc.Text().String(), "önce") || strings.Contains(a.doc.Text().String(), "elma") {
		t.Fatalf("the table is in the text: %q", a.doc.Text().String())
	}
	if len(a.tableSeen) != 6 { // the header and two rows, two cells each
		t.Fatalf("cells: %d", len(a.tableSeen))
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
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	cell := func(row, col int) tableHit {
		for _, h := range a.tableSeen {
			if h.row == row && h.col == col {
				return h
			}
		}
		t.Fatalf("no cell %d,%d", row, col)
		return tableHit{}
	}
	model := func() *tableModel {
		for _, h := range a.tableSeen {
			return h.ch.tbl
		}
		return nil
	}
	h := cell(0, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if !a.cellEd.open || a.cellEd.text != "elma" {
		t.Fatalf("not open: %+v", a.cellEd)
	}
	if an, ca := a.doc.Selection(); an != 0 || ca != 0 {
		t.Fatalf("the caret moved: %d %d", an, ca)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("kiraz")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if m := model(); m.cell(0, 0) != "kiraz" || a.cellEd.open {
		t.Fatalf("after Enter: %q open %v", m.cell(0, 0), a.cellEd.open)
	}
	// Tab goes to the next cell, and writes this one
	h = cell(0, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("7")
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	if !a.cellEd.open || a.cellEd.row != 1 || a.cellEd.col != 0 || a.cellEd.text != "armut" {
		t.Fatalf("after Tab: %+v", a.cellEd)
	}
	if m := model(); m.cell(0, 1) != "7" {
		t.Fatalf("Tab did not write: %+v", m.rows)
	}
	// Tab past the last cell: a new row
	tt.Key(0, ui.KeyTab)
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.rows) != 3 || a.cellEd.row != 2 {
		t.Fatalf("no new row: %d rows, editing %+v", len(m.rows), a.cellEd)
	}
	// the buttons: a column, and then it is taken away; a row away
	if err := tt.Click("Sağa sütun ekle"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.cols) != 3 {
		t.Fatalf("columns %d", len(m.cols))
	}
	h = cell(-1, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Sütunu sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.cols) != 2 {
		t.Fatalf("columns %d after delete", len(m.cols))
	}
	h = cell(2, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Satırı sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); len(m.rows) != 2 {
		t.Fatalf("rows %d after delete", len(m.rows))
	}
	// the foot: a column that is a number sums
	h = cell(0, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if model().cols[1].Kind != kindNumber {
		t.Fatalf("the column was read as %q", model().cols[1].Kind)
	}
	if err := tt.Click("Alt satır: Hesap yok"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if m := model(); m.cols[1].Calc != "sum" || m.calc(1) != "19" {
		t.Fatalf("foot: %q %q", m.cols[1].Calc, m.calc(1))
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, `"calc":"sum"`) || !strings.Contains(md, "sonra") {
		t.Fatalf("what is written: %q", md)
	}
	// undo brings back the table as it was
	a.doc.Undo()
	tt.Frame()
	if m := model(); m.cols[1].Calc != "" {
		t.Fatalf("undo left the foot: %+v", m.cols)
	}
}

// TestTableCellLetGo: a click into the text, with a cell open, writes the cell and lets it go,
// and what is typed goes into the text.
func TestTableCellLetGo(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var h, last tableHit
	for _, s := range a.tableSeen {
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
	if !a.cellEd.open || a.focused {
		t.Fatalf("the cell is not the one written in: open %v, text focused %v", a.cellEd.open, a.focused)
	}
	// "sonra", under the table
	tt.ClickAt(last.r.X+last.r.W/2, last.r.Y+last.r.H+3*a.fontSize())
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if a.cellEd.open || !a.focused {
		t.Fatalf("the table kept the focus: open %v, text focused %v", a.cellEd.open, a.focused)
	}
	if m := a.tableSeen[0].ch.tbl; m.cell(0, 0) != "kiraz" {
		t.Fatalf("the cell was not written: %q", m.cell(0, 0))
	}
	tt.Type("z")
	tt.Advance(200 * time.Millisecond)
	if s := a.doc.Text().String(); !strings.Contains(s, "z") || strings.Count(s, "kiraz") > 0 {
		t.Fatalf("the letter did not go into the text: %q", s)
	}
}

// TestTableCellWheel: the wheel over an open cell scrolls the page, and the cell goes with it.
func TestTableCellWheel(t *testing.T) {
	a, tt := newTestApp(t, tableMD+strings.Repeat("satır\n\n", 80))
	tt.SetSize(1000, 700)
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var h tableHit
	for _, s := range a.tableSeen {
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
	if a.pageScroll.Y < 100 || !a.cellEd.open || a.cellEd.text != "kiraz" {
		t.Fatalf("the page did not scroll under the cell: scrolled %v, %+v", a.pageScroll.Y, a.cellEd)
	}
	for _, s := range a.tableSeen {
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
	if !a.cellEd.open || a.cellEd.text != "kiraz" {
		t.Fatalf("the cell was lost out of sight: %+v", a.cellEd)
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
	for _, c := range a.tableSeen {
		if c.row == 0 && c.col == 0 {
			h = c
		}
	}
	if h.ch == nil {
		t.Fatal("no cell")
	}
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if a.cellEd.open || !strings.Contains(a.pageMarkdown(a.page, a.doc.Text()), `["1","a"]`) {
		t.Fatalf("not ticked: open %v, %q", a.cellEd.open, a.pageMarkdown(a.page, a.doc.Text()))
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
	if !a.cellEd.open || a.cellEd.row != -1 || a.cellEd.text != "Sütun 1" {
		t.Fatalf("cell: %+v", a.cellEd)
	}
	if len(a.tableSeen) != 9 { // three columns, a header and two rows
		t.Fatalf("cells drawn: %d", len(a.tableSeen))
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Ad")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, `[{"name":"Ad"},{"name":"Sütun 2"}`) {
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
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if len(a.borderSeen) != 2 {
		t.Fatalf("borders: %d", len(a.borderSeen))
	}
	b := a.borderSeen[0]
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
	for _, h := range a.tableSeen {
		w = h.ch.tbl.cols[0].W
	}
	if w < b.w+60 || a.colDrag.active {
		t.Fatalf("the column is %v wide, was %v", w, b.w)
	}
	if an, ca := a.doc.Selection(); an != 0 || ca != 0 {
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
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var ch *chip
	for _, h := range a.tableSeen {
		ch = h.ch
	}
	pos := a.tablePos(ch)
	if pos < 0 {
		t.Fatal("no table in the text")
	}
	a.doc.SetSelection(pos, pos)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Advance(200 * time.Millisecond)
	at := func(row, col int, text, when string) {
		t.Helper()
		if !a.cellEd.open || a.cellEd.row != row || a.cellEd.col != col || a.cellEd.text != text {
			t.Fatalf("%s: %+v, want %d,%d %q", when, a.cellEd, row, col, text)
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
	if a.cellEd.open {
		t.Fatalf("↓ past the last row stayed in the table: %+v", a.cellEd)
	}
	if an, ca := a.doc.Selection(); an != pos+1 || ca != pos+1 {
		t.Fatalf("out of the table the caret is at %d,%d, not after it (%d)", an, ca, pos+1)
	}
	if n := len(ch.tbl.rows); n != 2 {
		t.Fatalf("↓ added a row: %d", n)
	}
	tt.Key(0, ui.KeyUp)
	tt.Advance(200 * time.Millisecond)
	at(1, 0, "armut", "↑ from under the table")
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Advance(200 * time.Millisecond)
	if a.cellEd.open {
		t.Fatalf("↑ over the header stayed in the table: %+v", a.cellEd)
	}
	if an, ca := a.doc.Selection(); an != pos || ca != pos {
		t.Fatalf("out of the top the caret is at %d,%d, not before the table (%d)", an, ca, pos)
	}
}
