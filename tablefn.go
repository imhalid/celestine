package main

import (
	"encoding/csv"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/imhalid/celestine/locales"
)

// What a table can do with its rows: sort them by a column, show only those
// that hold a word (the filter, which the page keeps), and be read from and
// written as CSV, as a spreadsheet's cells are.

// visible are the rows that show: all of them, or those that hold the filter's
// words in a cell. A row with nothing in it always shows, or a row just added
// would vanish under the filter it does not match.
func (m *tableModel) visible() []int {
	q := strings.Fields(foldText(m.Filter))
	out := make([]int, 0, len(m.rows))
	for i, r := range m.rows {
		if len(q) == 0 || rowBlank(r) || rowHolds(r, q) {
			out = append(out, i)
		}
	}
	return out
}

// shown is visible, kept on the model for its filter: the drawing of a table asks for it each frame, and its foot
// once a column, each a fold of every row (#204). It is for a table in the text, which is never changed (clone): the
// changes are made to a copy, which has nothing kept, and they ask visible.
func (m *tableModel) shown() []int {
	if m.vis == nil || m.visOf != m.Filter {
		m.vis, m.visOf = m.visible(), m.Filter
	}
	return m.vis
}

func rowBlank(r []string) bool {
	for _, s := range r {
		if strings.TrimSpace(s) != "" {
			return false
		}
	}
	return true
}

// rowHolds reports whether the row's cells together hold every word of q (folded).
func rowHolds(r []string, q []string) bool {
	all := foldText(strings.Join(r, "\x00"))
	for _, w := range q {
		if !strings.Contains(all, w) {
			return false
		}
	}
	return true
}

// filtering reports whether the filter hides anything's way: it has words.
func (m *tableModel) filtering() bool { return strings.TrimSpace(m.Filter) != "" }

// sortKey is what a cell sorts by in a column of its kind: a number, a day's
// text, whether it is checked, or the text as the alphabet reads it. A cell with
// nothing in it has none.
func sortKey(col tableCol, s string) (num float64, text string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", false
	}
	switch col.Kind {
	case kindNumber:
		v, ok := parseNumber(s)
		return v, "", ok
	case kindCheck:
		return 1, "", true
	case kindDate:
		return 0, s, true
	}
	return 0, foldText(plainText(s)), true
}

// lessRows orders two cells of a column, those with nothing last.
func lessRows(col tableCol, a, b string) bool {
	an, at, aok := sortKey(col, a)
	bn, bt, bok := sortKey(col, b)
	switch {
	case !aok || !bok:
		return aok && !bok
	case col.Kind == kindNumber || col.Kind == kindCheck:
		return an < bn
	}
	return at < bt
}

// sortBy puts the rows in order by column col, those equal staying as they were.
func (m *tableModel) sortBy(col int, desc bool) {
	if col < 0 || col >= len(m.cols) {
		return
	}
	c := m.cols[col]
	slices.SortStableFunc(m.rows, func(a, b []string) int {
		ae, be := strings.TrimSpace(a[col]) == "", strings.TrimSpace(b[col]) == ""
		switch {
		case ae && be:
			return 0
		case ae:
			return 1
		case be:
			return -1
		}
		lt, gt := lessRows(c, a[col], b[col]), lessRows(c, b[col], a[col])
		switch {
		case lt == gt:
			return 0
		case lt != desc:
			return -1
		}
		return 1
	})
}

// toggleSort sorts by column col rising, and falling when it already is.
func (m *tableModel) toggleSort(col int) {
	m.sortBy(col, m.sortedAsc(col))
}

// sortedAsc reports whether the rows are already rising by column col, with
// at least two different cells to tell it by.
func (m *tableModel) sortedAsc(col int) bool {
	c := m.cols[col]
	differ := false
	for i := 1; i < len(m.rows); i++ {
		a, b := m.rows[i-1][col], m.rows[i][col]
		if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
			if strings.TrimSpace(a) == "" && strings.TrimSpace(b) != "" {
				return false
			}
			continue
		}
		if lessRows(c, b, a) {
			return false
		}
		if lessRows(c, a, b) {
			differ = true
		}
	}
	return differ
}

// csvSafeCell prefixes spreadsheet formula triggers (=, @, tab, CR, or +/- when not a finite number) with ' (#185).
func csvSafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '@', '\t', '\r':
		return "'" + s
	case '+', '-':
		if strings.ContainsAny(s, "_xXpP") {
			return "'" + s
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return "'" + s
		}
	}
	return s
}

// csvText is the table as CSV, the header first, in the cells' own Markdown.
func (m *tableModel) csvText(sep rune) string {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	w.Comma = sep
	head := make([]string, len(m.cols))
	for i, c := range m.cols {
		head[i] = csvSafeCell(c.Name)
	}
	w.Write(head)
	for _, r := range m.rows {
		safe := make([]string, len(r))
		for i, cell := range r {
			safe[i] = csvSafeCell(cell)
		}
		w.Write(safe)
	}
	w.Flush()
	return sb.String()
}

// tableFromCSV reads CSV (or text with tabs or semicolons between the cells,
// as a spreadsheet copies it): the first line names the columns. A column of
// numbers is one. It is nil for text that is not a table.
func tableFromCSV(s string) *tableModel {
	s = strings.TrimPrefix(strings.ReplaceAll(s, "\r\n", "\n"), "\uFEFF")
	s = strings.TrimRight(s, "\n")
	first, _, _ := strings.Cut(s, "\n")
	sep := ','
	for _, c := range []rune{'\t', ';', ','} {
		if strings.ContainsRune(first, c) {
			sep = c
			break
		}
	}
	r := csv.NewReader(strings.NewReader(s))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = sep, -1, true
	recs, err := r.ReadAll()
	if err != nil || len(recs) == 0 {
		return nil
	}
	width := 0
	for _, rec := range recs {
		width = max(width, len(rec))
	}
	if width < 2 {
		return nil
	}
	m := &tableModel{}
	for c := 0; c < width; c++ {
		name := ""
		if c < len(recs[0]) {
			name = strings.TrimSpace(recs[0][c])
		}
		if name == "" {
			name = locales.TablesColumnN(c + 1)
		}
		m.cols = append(m.cols, tableCol{Name: name})
	}
	for _, rec := range recs[1:] {
		row := make([]string, width)
		for c := range row {
			if c < len(rec) {
				row[c] = strings.TrimSpace(strings.ReplaceAll(rec[c], "\n", " "))
			}
		}
		m.rows = append(m.rows, row)
	}
	for c := range m.cols {
		numeric, any := true, false
		for _, row := range m.rows {
			if row[c] == "" {
				continue
			}
			any = true
			if _, ok := parseNumber(row[c]); !ok {
				numeric = false
				break
			}
		}
		if any && numeric {
			m.cols[c].Kind, m.cols[c].Align = kindNumber, 2
		}
	}
	return m
}

// tabular reports whether pasted text is a table a spreadsheet copied: two lines
// or more, each with the same number of tabs, at least one.
func tabular(s string) bool {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	lines := strings.Split(s, "\n")
	if len(lines) < 2 || utf8.RuneCountInString(s) > 200000 {
		return false
	}
	n := strings.Count(lines[0], "\t")
	if n == 0 {
		return false
	}
	for _, l := range lines[1:] {
		if strings.Count(l, "\t") != n {
			return false
		}
	}
	return true
}
