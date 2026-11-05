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
	if got := joinCells([]string{"a|b", "", "c"}); got != "| a\\|b |  | c |" {
		t.Errorf("join: %q", got)
	}
}

const tableMD = "önce\n\n| Ad | Sayı |\n| --- | ---: |\n| elma | 3 |\n| armut | 12 |\n\nsonra\n"

// TestTableDrawn: a table the caret is not in is drawn as a table, each cell
// a place to click; with the caret in it, it is the Markdown.
func TestTableDrawn(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	tb, ok := tableBlockAt(a.doc.Text(), strings.Index(a.doc.Text().String(), "| Ad"))
	if !ok || tb.ncols != 2 || len(tb.paras) != 4 || tb.aligns[1] != 2 {
		t.Fatalf("table: %+v %v", tb, ok)
	}
	if len(a.tableSeen) != 6 { // the header and two rows, two cells each
		t.Fatalf("cells: %d", len(a.tableSeen))
	}
	// the caret into it: Markdown, no cells
	a.doc.SetSelection(tb.start+2, tb.start+2)
	tt.Frame()
	tt.Frame()
	if len(a.tableSeen) != 0 {
		t.Fatalf("drawn with the caret in it: %d", len(a.tableSeen))
	}
}

// TestTableCellEdit: a click on a cell opens it; Enter writes it; Tab goes on
// and past the last cell adds a row; the buttons add and take away.
func TestTableCellEdit(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	cell := func(line, col int) tableHit {
		for _, h := range a.tableSeen {
			if h.line == line && h.col == col {
				return h
			}
		}
		t.Fatalf("no cell %d,%d", line, col)
		return tableHit{}
	}
	h := cell(2, 0)
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
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "| kiraz | 3 |") || a.cellEd.open {
		t.Fatalf("after Enter: %q open %v", md, a.cellEd.open)
	}
	// Tab goes to the next cell, and writes this one
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	h = cell(2, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("7")
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	if !a.cellEd.open || a.cellEd.line != 3 || a.cellEd.col != 0 || a.cellEd.text != "armut" {
		t.Fatalf("after Tab: %+v", a.cellEd)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "| kiraz | 7 |") {
		t.Fatalf("Tab did not write: %q", md)
	}
	// Tab past the last cell: a new row
	tt.Key(0, ui.KeyTab)
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyTab)
	tt.Advance(200 * time.Millisecond)
	tb := a.freshTable(strings.Index(a.doc.Text().String(), "| Ad"))
	if len(tb.paras) != 5 || a.cellEd.line != 4 {
		t.Fatalf("no new row: %d lines, editing %+v", len(tb.paras), a.cellEd)
	}
	// the buttons: a column, and then it is taken away; a row away
	if err := tt.Click("Sağa sütun ekle"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	tb = a.freshTable(strings.Index(a.doc.Text().String(), "| Ad"))
	if tb.ncols != 3 {
		t.Fatalf("columns %d", tb.ncols)
	}
	h = cell(0, 1) // the new column came after the first
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Sütunu sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	tb = a.freshTable(strings.Index(a.doc.Text().String(), "| Ad"))
	if tb.ncols != 2 {
		t.Fatalf("columns %d after delete", tb.ncols)
	}
	h = cell(4, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Satırı sil"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	tb = a.freshTable(strings.Index(a.doc.Text().String(), "| Ad"))
	if len(tb.paras) != 4 {
		t.Fatalf("lines %d after delete", len(tb.paras))
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "| armut | 12 |") || !strings.Contains(md, "sonra") {
		t.Fatalf("what is left: %q", md)
	}
}

// TestTableInserted: Ekle… ▸ Tablo makes a table shown as one, its first cell open.
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
	if !a.cellEd.open || a.cellEd.line != 0 || a.cellEd.text != "Sütun 1" {
		t.Fatalf("cell: %+v", a.cellEd)
	}
	if len(a.tableSeen) == 0 {
		t.Fatal("not drawn as a table")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Ad")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "| Ad | Sütun 2 | Sütun 3 |") {
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

// TestTableColumnDrag: a column border is dragged; let go, its width is the
// dashes of the separator line, as any tool leaves them.
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
	md := a.pageMarkdown(a.page, a.doc.Text())
	line := ""
	for _, l := range strings.Split(md, "\n") {
		if strings.Contains(l, "---") {
			line = l
		}
	}
	tb := a.freshTable(strings.Index(a.doc.Text().String(), "| Ad"))
	if tb.colHint(0) < b.w+60 || a.colDrag.active {
		t.Fatalf("the column is %v wide in the dashes, was %v: %q", tb.colHint(0), b.w, line)
	}
	if an, ca := a.doc.Selection(); an != 0 || ca != 0 {
		t.Fatalf("the caret moved: %d %d", an, ca)
	}
	if !strings.Contains(line, "---:") {
		t.Fatalf("the alignment of the other column is lost: %q", line)
	}
}
