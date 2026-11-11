package main

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/imhalid/celestine/locales"
)

// A table is data, not text: columns that have a kind (text, number, date,
// check), an alignment, a width and a calculation for their foot, and rows of
// cells that are Markdown as it reads (bold, [[pages]], #tags). In the editor
// it is one object (tables.go); in the file it is a fenced block the app
// writes itself:
//
//	```celestine-table
//	[{"name":"Ad"},{"name":"Sayı","kind":"number","align":2,"calc":"sum"}]
//	["elma","3"]
//	["armut","12"]
//	```
//
// the columns on the first line, each row on a line of its own.

const tableLang = "celestine-table"

// Column kinds and calculations.
const (
	kindText   = ""
	kindNumber = "number"
	kindDate   = "date"
	kindCheck  = "check"
)

var tableKinds = []string{kindText, kindNumber, kindDate, kindCheck}

// kindName is the name of a column's kind, in the window's language.
func kindName(k string) string {
	switch k {
	case kindNumber:
		return locales.TablesKindNumber()
	case kindDate:
		return locales.TablesKindDate()
	case kindCheck:
		return locales.TablesKindCheck()
	}
	return locales.TablesKindText()
}

// calcs are what a column's foot can say, in the order a button goes through them.
var calcs = []string{"", "sum", "avg", "count", "min", "max"}

// calcName is the name of what a column's bottom row works out, in the window's language.
func calcName(k string) string {
	switch k {
	case "sum":
		return locales.TablesCalcSum()
	case "avg":
		return locales.TablesCalcAvg()
	case "count":
		return locales.TablesCalcCount()
	case "min":
		return locales.TablesCalcMin()
	case "max":
		return locales.TablesCalcMax()
	}
	return locales.TablesCalcNone()
}

type tableCol struct {
	Name  string  `json:"name"`
	Kind  string  `json:"kind,omitempty"`
	Align int     `json:"align,omitempty"` // 0 start, 1 center, 2 end
	W     float32 `json:"w,omitempty"`     // its width, or 0 for as wide as its cells
	Calc  string  `json:"calc,omitempty"`
}

type tableModel struct {
	cols   []tableCol
	rows   [][]string
	Filter string // the words a row must hold to show (tablefn.go); blank, all show
}

// newTable is a table of ncols columns and nrows empty rows.
func newTable(ncols, nrows int) *tableModel {
	m := &tableModel{}
	for c := 0; c < ncols; c++ {
		m.cols = append(m.cols, tableCol{Name: locales.TablesColumnN(c + 1)})
	}
	for r := 0; r < nrows; r++ {
		m.rows = append(m.rows, make([]string, ncols))
	}
	return m
}

// clone is a copy to change: a table in the text is never changed, as an
// undo brings back the one it had.
func (m *tableModel) clone() *tableModel {
	n := &tableModel{cols: append([]tableCol(nil), m.cols...), Filter: m.Filter}
	for _, r := range m.rows {
		n.rows = append(n.rows, append([]string(nil), r...))
	}
	return n
}

// cell is the text of a cell: row -1 is the header.
func (m *tableModel) cell(row, col int) string {
	if col < 0 || col >= len(m.cols) {
		return ""
	}
	if row < 0 {
		return m.cols[col].Name
	}
	if row >= len(m.rows) || col >= len(m.rows[row]) {
		return ""
	}
	return m.rows[row][col]
}

func (m *tableModel) setCell(row, col int, s string) {
	if col < 0 || col >= len(m.cols) {
		return
	}
	if row < 0 {
		m.cols[col].Name = s
		return
	}
	if row < len(m.rows) {
		m.rows[row][col] = s
	}
}

// addRow puts an empty row after row (-1: first).
func (m *tableModel) addRow(after int) {
	after = max(-1, min(after, len(m.rows)-1))
	m.rows = append(m.rows[:after+1], append([][]string{make([]string, len(m.cols))}, m.rows[after+1:]...)...)
}

func (m *tableModel) delRow(row int) {
	if row < 0 || row >= len(m.rows) {
		return
	}
	m.rows = append(m.rows[:row], m.rows[row+1:]...)
}

// addCol puts an empty column after col (-1: first).
func (m *tableModel) addCol(after int) {
	after = max(-1, min(after, len(m.cols)-1))
	m.cols = append(m.cols[:after+1], append([]tableCol{{Name: locales.TablesColumnN(len(m.cols) + 1)}}, m.cols[after+1:]...)...)
	for i, r := range m.rows {
		m.rows[i] = append(r[:after+1], append([]string{""}, r[after+1:]...)...)
	}
}

// delCol takes a column away, unless it is the only one.
func (m *tableModel) delCol(col int) {
	if col < 0 || col >= len(m.cols) || len(m.cols) < 2 {
		return
	}
	m.cols = append(m.cols[:col], m.cols[col+1:]...)
	for i, r := range m.rows {
		m.rows[i] = append(r[:col], r[col+1:]...)
	}
}

// setKind changes what a column is: a number's end is where it reads, and a
// check keeps only what says it is checked.
func (m *tableModel) setKind(col int, kind string) {
	c := &m.cols[col]
	if c.Kind == kind {
		return
	}
	c.Kind = kind
	switch kind {
	case kindNumber:
		c.Align = 2
	case kindCheck:
		c.Align = 1
	default:
		c.Align = 0
	}
	if kind == kindCheck {
		for _, r := range m.rows {
			if v := strings.ToLower(strings.TrimSpace(r[col])); v != "" && v != "0" && v != "false" && v != "hayır" {
				r[col] = "1"
			} else {
				r[col] = ""
			}
		}
	}
	if !calcFits(kind, c.Calc) {
		c.Calc = ""
	}
}

// calcFits reports whether a column of the kind can have the calculation.
func calcFits(kind, calc string) bool {
	switch {
	case calc == "":
		return true
	case kind == kindCheck:
		return calc == "count"
	case kind == kindNumber:
		return true
	}
	return calc == "count"
}

// nextOf is what comes after v in list, the first after the last.
func nextOf(list []string, v string) string {
	for i, s := range list {
		if s == v {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// nextCalc is the calculation after the column's own that its kind allows.
func (c tableCol) nextCalc() string {
	calc := c.Calc
	for range calcs {
		calc = nextOf(calcs, calc)
		if calcFits(c.Kind, calc) {
			return calc
		}
	}
	return ""
}

func (m *tableModel) hasCalc() bool {
	for _, c := range m.cols {
		if c.Calc != "" {
			return true
		}
	}
	return false
}

// parseNumber reads a number as it is written here: spaces and a decimal
// comma allowed.
func parseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		if strings.Contains(s, ".") { // 1.234,5
			s = strings.ReplaceAll(s, ".", "")
		}
		s = strings.Replace(s, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// formatNumber writes a number without a needless tail: 3, 2.5, 0.33.
func formatNumber(v float64) string {
	v = math.Round(v*100) / 100
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// calc is what the foot of column c says, or "" for nothing.
func (m *tableModel) calc(c int) string {
	col := m.cols[c]
	if col.Calc == "" {
		return ""
	}
	shown := m.visible() // the foot counts the rows that show
	if col.Kind == kindCheck {
		n := 0
		for _, i := range shown {
			if m.rows[i][c] != "" {
				n++
			}
		}
		return strconv.Itoa(n) + "/" + strconv.Itoa(len(shown))
	}
	if col.Calc == "count" {
		n := 0
		for _, i := range shown {
			if strings.TrimSpace(m.rows[i][c]) != "" {
				n++
			}
		}
		return strconv.Itoa(n)
	}
	var vals []float64
	for _, i := range shown {
		if v, ok := parseNumber(m.rows[i][c]); ok {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	out := vals[0]
	sum := 0.0
	for _, v := range vals {
		sum += v
		switch col.Calc {
		case "min":
			out = math.Min(out, v)
		case "max":
			out = math.Max(out, v)
		}
	}
	switch col.Calc {
	case "sum":
		out = sum
	case "avg":
		out = sum / float64(len(vals))
	}
	return formatNumber(out)
}

// text is the table as the file has it, inside its fence.
func (m *tableModel) text() string {
	var sb strings.Builder
	head, _ := json.Marshal(m.cols)
	sb.Write(head)
	for _, r := range m.rows {
		row, _ := json.Marshal(r)
		sb.WriteString("\n")
		sb.Write(row)
	}
	return sb.String()
}

// parseTable reads what text wrote; rows are made as long as the columns.
func parseTable(s string) (*tableModel, bool) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	m := &tableModel{}
	if err := json.Unmarshal([]byte(lines[0]), &m.cols); err != nil || len(m.cols) == 0 {
		return nil, false
	}
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var row []string
		if err := json.Unmarshal([]byte(ln), &row); err != nil {
			return nil, false
		}
		for len(row) < len(m.cols) {
			row = append(row, "")
		}
		m.rows = append(m.rows, row[:len(m.cols)])
	}
	return m, true
}

// tableFromMarkdown makes a table of the lines of a Markdown table, as
// other tools write them: the alignment of the dashes line and, where a
// column has dashes of its own, its width. A column whose cells are all numbers is one.
func tableFromMarkdown(lines []string) *tableModel {
	if len(lines) < 2 {
		return nil
	}
	head := splitCells(lines[0])
	sep := splitCells(lines[1])
	m := &tableModel{}
	for c, name := range head {
		col := tableCol{Name: name}
		if c < len(sep) {
			s := strings.TrimSpace(sep[c])
			switch {
			case strings.HasPrefix(s, ":") && strings.HasSuffix(s, ":") && len(s) > 1:
				col.Align = 1
			case strings.HasSuffix(s, ":"):
				col.Align = 2
			}
			if n := strings.Count(s, "-"); n != 3 && n != 0 {
				col.W = float32(n * dashUnit)
			}
		}
		m.cols = append(m.cols, col)
	}
	for _, ln := range lines[2:] {
		row := splitCells(ln)
		for len(row) < len(m.cols) {
			row = append(row, "")
		}
		m.rows = append(m.rows, row[:len(m.cols)])
	}
	for c := range m.cols {
		numeric, any := true, false
		for _, r := range m.rows {
			if strings.TrimSpace(r[c]) == "" {
				continue
			}
			any = true
			if _, ok := parseNumber(r[c]); !ok {
				numeric = false
				break
			}
		}
		if any && numeric {
			m.cols[c].Kind = kindNumber
			if m.cols[c].Align == 0 {
				m.cols[c].Align = 2
			}
		}
	}
	return m
}

// dashUnit is how wide a dash of a Markdown table's separator line makes a
// column, for those read from one.
const dashUnit = 8
