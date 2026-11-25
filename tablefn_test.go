package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func sample3() *tableModel {
	m := newTable(2, 0)
	m.Cols[0].Name = "Ad"
	m.Cols[1] = tableCol{Name: "Sayı", Kind: kindNumber, Align: 2, Calc: "sum"}
	m.Rows = [][]string{{"muz", "12"}, {"elma", "3"}, {"Çilek", "7"}, {"armut", ""}, {"incir", "100"}}
	return m
}

func col(m *tableModel, c int) string {
	var out []string
	for _, r := range m.Rows {
		out = append(out, r[c])
	}
	return strings.Join(out, ",")
}

func TestTableSort(t *testing.T) {
	m := sample3()
	m.ToggleSort(1) // rising; the empty last
	if got := col(m, 1); got != "3,7,12,100," {
		t.Fatalf("rising: %s", got)
	}
	if !m.SortedAsc(1) {
		t.Fatal("not told it is rising")
	}
	m.ToggleSort(1) // and again: falling, the empty still last
	if got := col(m, 1); got != "100,12,7,3," {
		t.Fatalf("falling: %s", got)
	}
	m.ToggleSort(0) // the alphabet's, Turkish: Ç after C, before D
	if got := col(m, 0); got != "armut,Çilek,elma,incir,muz" {
		t.Fatalf("text: %s", got)
	}
	// rows keep their cells together
	if m.Rows[1][1] != "7" {
		t.Fatalf("a row came apart: %v", m.Rows)
	}
}

func TestTableFilter(t *testing.T) {
	m := sample3()
	m.Filter = "  "
	if len(m.Visible()) != 5 || m.Filtering() {
		t.Fatal("a blank filter hides rows")
	}
	m.Filter = "elma"
	if got := m.Visible(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("elma: %v", got)
	}
	m.Filter = "cilek" // the Turkish letters are matched as plain ones
	if got := m.Visible(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("cilek: %v", got)
	}
	m.Filter = "12 muz" // every word, in any cell
	if got := m.Visible(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("two words: %v", got)
	}
	// a row with nothing in it stays, and the foot counts what shows
	m.Rows = append(m.Rows, []string{"", ""})
	m.Filter = "elma"
	if got := m.Visible(); len(got) != 2 {
		t.Fatalf("blank row: %v", got)
	}
	if got := m.Calc(1); got != "3" {
		t.Fatalf("the foot counts rows that are hidden: %s", got)
	}
	// it is kept with the page
	back := decodeBody(encodeBody(decodeBody(`[{"r":[{"t":"￼","o":{"k":"table","cols":[{"name":"A"}],"rows":[["x"],["y"]],"q":"x"}}]}]`)))
	var kept *tableModel
	for _, r := range back.Runs(0, back.Len()) {
		if at := r.Attributes.Attachment; at != nil {
			if ch, ok := at.Data.(*chip); ok && ch.tbl != nil {
				kept = ch.tbl
			}
		}
	}
	if kept == nil || kept.Filter != "x" {
		t.Fatalf("the filter was not kept: %+v", kept)
	}
}

func TestTableCSV(t *testing.T) {
	m := sample3()
	m.Rows[0][0] = `muz, "sarı"`
	out := m.CSVText(',')
	if !strings.HasPrefix(out, "Ad,Sayı\n\"muz, \"\"sarı\"\"\",12\n") {
		t.Fatalf("csv: %q", out)
	}
	back := tableFromCSV(out)
	if back == nil || back.Rows[0][0] != `muz, "sarı"` || back.Cols[1].Kind != kindNumber || len(back.Rows) != 5 {
		t.Fatalf("read back: %+v", back)
	}
	for _, in := range []string{"a\tb\n1\t2\n", "a;b\n1;2\n", "a,b\r\n1,2\r\n"} {
		if got := tableFromCSV(in); got == nil || len(got.Cols) != 2 || got.Rows[0][1] != "2" {
			t.Fatalf("%q: %+v", in, got)
		}
	}
	if tableFromCSV("bir satır\nikinci satır") != nil {
		t.Fatal("a table out of prose")
	}
	if !tabular("a\tb\n1\t2") || tabular("a\tb\n1") || tabular("tek satır\tiki") || tabular("düz\nmetin") {
		t.Fatal("tabular")
	}
}

// The buttons of an open cell: sort, filter (written in the strip under the
// table, its cross taking it away), and the table copied as CSV.
func TestTableSortFilterUI(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	var copied string
	a.clipText = func(s string) { copied = s }
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
	h := cell(0, 1)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Bu sütuna göre sırala (azalan)"); err != nil { // 3, 12: it is rising, so this one is falling
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if got := col(model(), 1); got != "12,3" {
		t.Fatalf("sorted: %s", got)
	}
	// the filter
	h = cell(0, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Filtrele"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if !a.pane().cellEd.open || a.pane().cellEd.row != -2 {
		t.Fatalf("the filter is not open to be written: %+v", a.pane().cellEd)
	}
	tt.Type("elma")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if m := model(); m.Filter != "elma" || len(m.Visible()) != 1 {
		t.Fatalf("filter %q shows %v", m.Filter, m.Visible())
	}
	shown := 0
	for _, hh := range a.pane().tableSeen {
		if hh.row >= 0 && hh.col == 0 {
			shown++
		}
	}
	if shown != 1 {
		t.Fatalf("%d rows are drawn", shown)
	}
	// the cross takes it away
	x := cell(-3, 0)
	tt.ClickAt(x.r.X+x.r.W/2, x.r.Y+x.r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if m := model(); m.Filter != "" || len(m.Visible()) != 2 {
		t.Fatalf("not cleared: %q", m.Filter)
	}
	// CSV
	h = cell(0, 0)
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("CSV olarak kopyala"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	if !strings.HasPrefix(copied, "Ad,Sayı\n") {
		t.Fatalf("copied %q", copied)
	}
}

// Text a spreadsheet copied (cells by tabs) becomes a table when it is pasted, and
// the palette makes one of the CSV on the clipboard.
func TestTablePaste(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	_, caret := a.pane().doc.Selection()
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(caret, caret, richtext.NewText("Ad\tSayı\nelma\t3\narmut\t12"))
	})
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	n := 0
	for _, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
		if at := r.Attributes.Attachment; at != nil {
			if ch, ok := at.Data.(*chip); ok && ch.tbl != nil && len(ch.tbl.Rows) == 2 && ch.tbl.Cols[1].Kind == kindNumber {
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("no table out of the paste: %q", a.pane().doc.Text().String())
	}
	a.clipRead = func() string { return "a;b\n1;2\n" }
	a.pasteTable()
	tt.Advance(200 * time.Millisecond)
	count := 0
	for _, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
		if at := r.Attributes.Attachment; at != nil {
			if ch, ok := at.Data.(*chip); ok && ch.tbl != nil {
				count++
			}
		}
	}
	if count != 2 {
		t.Fatalf("%d tables", count)
	}
	a.clipRead = func() string { return "just words" }
	a.pasteTable()
	if a.notice == "" {
		t.Fatal("not told")
	}
}

// A change to a table is undone with ⌘Z and done again with ⇧⌘Z, as text is:
// the keyboard goes back to the text when the cell is shut; the cell's bar has the
// two as buttons too.
func TestTableUndo(t *testing.T) {
	a, tt := newTestApp(t, tableMD)
	tt.SetSize(1000, 700)
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	first := func() string {
		for _, r := range a.pane().doc.Text().Runs(0, a.pane().doc.Text().Len()) {
			if at := r.Attributes.Attachment; at != nil {
				if ch, ok := at.Data.(*chip); ok && ch.tbl != nil {
					return ch.tbl.Cell(0, 0)
				}
			}
		}
		return "?"
	}
	open := func(row, col int) {
		for _, h := range a.pane().tableSeen {
			if h.row == row && h.col == col {
				tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
				tt.Advance(200 * time.Millisecond)
				return
			}
		}
		t.Fatalf("no cell %d,%d", row, col)
	}
	open(0, 0)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("kiraz")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if first() != "kiraz" {
		t.Fatalf("not written: %q", first())
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Advance(300 * time.Millisecond)
	if first() != "elma" {
		t.Fatalf("⌘Z: %q", first())
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Advance(300 * time.Millisecond)
	if first() != "kiraz" {
		t.Fatalf("⇧⌘Z: %q", first())
	}
	// the buttons, from an open cell
	open(1, 0)
	if err := tt.Click("Geri al (⌘Z)"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if first() != "elma" || a.pane().cellEd.open {
		t.Fatalf("the button: %q open %v", first(), a.pane().cellEd.open)
	}
	open(1, 0)
	if err := tt.Click("Yinele (⇧⌘Z)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if first() != "kiraz" {
		t.Fatalf("redo button: %q", first())
	}
}
