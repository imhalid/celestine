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
