package table

import (
	"encoding/csv"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/textnorm"
)

// Visible returns indices of rows matching Filter, preserving blank rows so new empty rows stay visible.
func (m *Model) Visible() []int {
	q := strings.Fields(textnorm.Fold(m.Filter))
	out := make([]int, 0, len(m.Rows))
	for i, r := range m.Rows {
		if len(q) == 0 || rowBlank(r) || rowHolds(r, q) {
			out = append(out, i)
		}
	}
	return out
}

// Shown returns cached Visible row indices for Filter when m is immutable.
func (m *Model) Shown() []int {
	if m.vis == nil || m.visOf != m.Filter {
		m.vis, m.visOf = m.Visible(), m.Filter
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

func rowHolds(r []string, q []string) bool {
	all := textnorm.Fold(strings.Join(r, "\x00"))
	for _, w := range q {
		if !strings.Contains(all, w) {
			return false
		}
	}
	return true
}

// Filtering reports whether Filter hides non-matching rows.
func (m *Model) Filtering() bool { return strings.TrimSpace(m.Filter) != "" }

func sortKey(col Col, s string) (num float64, text string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", false
	}
	switch col.Kind {
	case KindNumber:
		v, ok := ParseNumber(s)
		return v, "", ok
	case KindCheck:
		return 1, "", true
	case KindDate:
		return 0, s, true
	}
	return 0, textnorm.Fold(textnorm.Plain(s)), true
}

func lessRows(col Col, a, b string) bool {
	an, at, aok := sortKey(col, a)
	bn, bt, bok := sortKey(col, b)
	switch {
	case !aok || !bok:
		return aok && !bok
	case col.Kind == KindNumber || col.Kind == KindCheck:
		return an < bn
	}
	return at < bt
}

// SortBy stably sorts rows by column col, keeping empty cells last.
func (m *Model) SortBy(col int, desc bool) {
	if col < 0 || col >= len(m.Cols) {
		return
	}
	c := m.Cols[col]
	slices.SortStableFunc(m.Rows, func(a, b []string) int {
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

// ToggleSort sorts column col ascending, or descending if already sorted ascending.
func (m *Model) ToggleSort(col int) {
	m.SortBy(col, m.SortedAsc(col))
}

// SortedAsc reports whether non-empty values in col are already ascending with at least two distinct values.
func (m *Model) SortedAsc(col int) bool {
	c := m.Cols[col]
	differ := false
	for i := 1; i < len(m.Rows); i++ {
		a, b := m.Rows[i-1][col], m.Rows[i][col]
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

// CSVText exports m using separator sep, guarding formula prefixes against spreadsheet injection.
func (m *Model) CSVText(sep rune) string {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	w.Comma = sep
	head := make([]string, len(m.Cols))
	for i, c := range m.Cols {
		head[i] = csvSafeCell(c.Name)
	}
	w.Write(head)
	for _, r := range m.Rows {
		safe := make([]string, len(r))
		for i, cell := range r {
			safe[i] = csvSafeCell(cell)
		}
		w.Write(safe)
	}
	w.Flush()
	return sb.String()
}

// FromCSV parses CSV/TSV/semicolon-separated text into a typed table Model, or returns nil if not tabular.
func FromCSV(s string) *Model {
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
	m := &Model{}
	for c := 0; c < width; c++ {
		name := ""
		if c < len(recs[0]) {
			name = strings.TrimSpace(recs[0][c])
		}
		if name == "" {
			name = ColumnName(c + 1)
		}
		m.Cols = append(m.Cols, Col{Name: name})
	}
	for _, rec := range recs[1:] {
		row := make([]string, width)
		for c := range row {
			if c < len(rec) {
				row[c] = strings.TrimSpace(strings.ReplaceAll(rec[c], "\n", " "))
			}
		}
		m.Rows = append(m.Rows, row)
	}
	for c := range m.Cols {
		numeric, any := true, false
		for _, row := range m.Rows {
			if row[c] == "" {
				continue
			}
			any = true
			if _, ok := ParseNumber(row[c]); !ok {
				numeric = false
				break
			}
		}
		if any && numeric {
			m.Cols[c].Kind, m.Cols[c].Align = KindNumber, 2
		}
	}
	return m
}

// Tabular reports whether s has at least two lines with an equal non-zero number of tab separators.
func Tabular(s string) bool {
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
