package table

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/imhalid/celestine/locales"
)

// Lang is the fenced code block language used to serialize tables in Markdown.
const Lang = "celestine-table"

// Column kinds and calculations.
const (
	KindText   = ""
	KindNumber = "number"
	KindDate   = "date"
	KindCheck  = "check"
)

// Kinds lists the available column kinds in cycle order.
var Kinds = []string{KindText, KindNumber, KindDate, KindCheck}

// KindName returns the localized display name of column kind k.
func KindName(k string) string {
	switch k {
	case KindNumber:
		return locales.TablesKindNumber()
	case KindDate:
		return locales.TablesKindDate()
	case KindCheck:
		return locales.TablesKindCheck()
	}
	return locales.TablesKindText()
}

// Calcs lists the footer calculations in cycle order.
var Calcs = []string{"", "sum", "avg", "count", "min", "max"}

// CalcName returns the localized display name of footer calculation k.
func CalcName(k string) string {
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

// Col describes one column of a table.
type Col struct {
	Name  string  `json:"name"`
	Kind  string  `json:"kind,omitempty"`
	Align int     `json:"align,omitempty"` // 0 start, 1 center, 2 end
	W     float32 `json:"w,omitempty"`     // width, or 0 to auto-size to cells
	Calc  string  `json:"calc,omitempty"`
}

// Model is a typed structured table with column schemas, cell rows, and filter state.
type Model struct {
	Cols   []Col
	Rows   [][]string
	Filter string
	vis    []int
	visOf  string
}

// New creates an empty table of ncols columns and nrows rows.
func New(ncols, nrows int) *Model {
	m := &Model{}
	for c := 0; c < ncols; c++ {
		m.Cols = append(m.Cols, Col{Name: locales.TablesColumnN(c + 1)})
	}
	for r := 0; r < nrows; r++ {
		m.Rows = append(m.Rows, make([]string, ncols))
	}
	return m
}

// Clone returns a deep copy of m so edits never mutate existing undo snapshots.
func (m *Model) Clone() *Model {
	n := &Model{Cols: append([]Col(nil), m.Cols...), Filter: m.Filter}
	for _, r := range m.Rows {
		n.Rows = append(n.Rows, append([]string(nil), r...))
	}
	return n
}

// Cell returns the text of a cell; row -1 returns the column header.
func (m *Model) Cell(row, col int) string {
	if col < 0 || col >= len(m.Cols) {
		return ""
	}
	if row < 0 {
		return m.Cols[col].Name
	}
	if row >= len(m.Rows) || col >= len(m.Rows[row]) {
		return ""
	}
	return m.Rows[row][col]
}

// SetCell sets the text of a cell; row -1 updates the column header.
func (m *Model) SetCell(row, col int, s string) {
	if col < 0 || col >= len(m.Cols) {
		return
	}
	if row < 0 {
		m.Cols[col].Name = s
		return
	}
	if row < len(m.Rows) {
		m.Rows[row][col] = s
	}
}

// AddRow inserts an empty row after index after (-1 inserts at the start).
func (m *Model) AddRow(after int) {
	after = max(-1, min(after, len(m.Rows)-1))
	m.Rows = append(m.Rows[:after+1], append([][]string{make([]string, len(m.Cols))}, m.Rows[after+1:]...)...)
}

// DelRow deletes row at index row if present.
func (m *Model) DelRow(row int) {
	if row < 0 || row >= len(m.Rows) {
		return
	}
	m.Rows = append(m.Rows[:row], m.Rows[row+1:]...)
}

// AddCol inserts an empty column after index after (-1 inserts at the start).
func (m *Model) AddCol(after int) {
	after = max(-1, min(after, len(m.Cols)-1))
	m.Cols = append(m.Cols[:after+1], append([]Col{{Name: locales.TablesColumnN(len(m.Cols) + 1)}}, m.Cols[after+1:]...)...)
	for i, r := range m.Rows {
		m.Rows[i] = append(r[:after+1], append([]string{""}, r[after+1:]...)...)
	}
}

// DelCol removes column col unless it is the last remaining column.
func (m *Model) DelCol(col int) {
	if col < 0 || col >= len(m.Cols) || len(m.Cols) < 2 {
		return
	}
	m.Cols = append(m.Cols[:col], m.Cols[col+1:]...)
	for i, r := range m.Rows {
		m.Rows[i] = append(r[:col], r[col+1:]...)
	}
}

// SetKind changes column col's kind, adjusting default alignment and normalizing check values.
func (m *Model) SetKind(col int, kind string) {
	c := &m.Cols[col]
	if c.Kind == kind {
		return
	}
	c.Kind = kind
	switch kind {
	case KindNumber:
		c.Align = 2
	case KindCheck:
		c.Align = 1
	default:
		c.Align = 0
	}
	if kind == KindCheck {
		for _, r := range m.Rows {
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

func calcFits(kind, calc string) bool {
	switch {
	case calc == "":
		return true
	case kind == KindCheck:
		return calc == "count"
	case kind == KindNumber:
		return true
	}
	return calc == "count"
}

// NextOf returns the item after v in list, wrapping to the first element.
func NextOf(list []string, v string) string {
	for i, s := range list {
		if s == v {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// NextCalc returns the next footer calculation allowed for c's kind.
func (c Col) NextCalc() string {
	calc := c.Calc
	for range Calcs {
		calc = NextOf(Calcs, calc)
		if calcFits(c.Kind, calc) {
			return calc
		}
	}
	return ""
}

// HasCalc reports whether any column defines a footer calculation.
func (m *Model) HasCalc() bool {
	for _, c := range m.Cols {
		if c.Calc != "" {
			return true
		}
	}
	return false
}

// ParseNumber parses a number allowing whitespace and comma decimal/thousand separators.
func ParseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		if strings.Contains(s, ".") {
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

// FormatNumber formats v rounded to 2 decimal places without trailing zeros.
func FormatNumber(v float64) string {
	v = math.Round(v*100) / 100
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Calc computes the formatted footer aggregate for column c over visible rows.
func (m *Model) Calc(c int) string {
	col := m.Cols[c]
	if col.Calc == "" {
		return ""
	}
	shown := m.Shown()
	if col.Kind == KindCheck {
		n := 0
		for _, i := range shown {
			if m.Rows[i][c] != "" {
				n++
			}
		}
		return strconv.Itoa(n) + "/" + strconv.Itoa(len(shown))
	}
	if col.Calc == "count" {
		n := 0
		for _, i := range shown {
			if strings.TrimSpace(m.Rows[i][c]) != "" {
				n++
			}
		}
		return strconv.Itoa(n)
	}
	var vals []float64
	for _, i := range shown {
		if v, ok := ParseNumber(m.Rows[i][c]); ok {
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
	return FormatNumber(out)
}

// Text serializes the table to its line-delimited JSON representation inside fenced blocks.
func (m *Model) Text() string {
	var sb strings.Builder
	head, _ := json.Marshal(m.Cols)
	sb.Write(head)
	for _, r := range m.Rows {
		row, _ := json.Marshal(r)
		sb.WriteString("\n")
		sb.Write(row)
	}
	return sb.String()
}

// Parse reads a serialized table written by Text.
func Parse(s string) (*Model, bool) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	m := &Model{}
	if err := json.Unmarshal([]byte(lines[0]), &m.Cols); err != nil || len(m.Cols) == 0 {
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
		for len(row) < len(m.Cols) {
			row = append(row, "")
		}
		m.Rows = append(m.Rows, row[:len(m.Cols)])
	}
	return m, true
}

// SplitCells splits one line of a Markdown pipe table into trimmed cell strings.
func SplitCells(line string) []string {
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

// FromMarkdown parses a Markdown pipe table into a typed table Model.
func FromMarkdown(lines []string) *Model {
	if len(lines) < 2 {
		return nil
	}
	head := SplitCells(lines[0])
	sep := SplitCells(lines[1])
	m := &Model{}
	for c, name := range head {
		col := Col{Name: name}
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
		m.Cols = append(m.Cols, col)
	}
	for _, ln := range lines[2:] {
		row := SplitCells(ln)
		for len(row) < len(m.Cols) {
			row = append(row, "")
		}
		m.Rows = append(m.Rows, row[:len(m.Cols)])
	}
	for c := range m.Cols {
		numeric, any := true, false
		for _, r := range m.Rows {
			if strings.TrimSpace(r[c]) == "" {
				continue
			}
			any = true
			if _, ok := ParseNumber(r[c]); !ok {
				numeric = false
				break
			}
		}
		if any && numeric {
			m.Cols[c].Kind = KindNumber
			if m.Cols[c].Align == 0 {
				m.Cols[c].Align = 2
			}
		}
	}
	return m
}

const dashUnit = 8
