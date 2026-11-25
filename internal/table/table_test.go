package table

import (
	"slices"
	"strings"
	"testing"
)

func TestModelRoundTripAndAggregate(t *testing.T) {
	m := New(2, 2)
	m.Cols[0].Name = "Ad"
	m.Cols[1] = Col{Name: "Sayı", Kind: KindNumber, Align: 2, Calc: "sum"}
	m.Rows[0] = []string{`elma "x"`, "3"}
	m.Rows[1] = []string{"armut", "12,5"}

	back, ok := Parse(m.Text())
	if !ok || back.Text() != m.Text() || back.Rows[0][0] != `elma "x"` {
		t.Fatalf("Parse round-trip failed: %+v ok=%v", back, ok)
	}
	if got := m.Calc(1); got != "15.5" {
		t.Fatalf("sum Calc = %q, want 15.5", got)
	}
	m.Cols[1].Calc = "avg"
	if got := m.Calc(1); got != "7.75" {
		t.Fatalf("avg Calc = %q, want 7.75", got)
	}
}

func TestSortFilterAndCSV(t *testing.T) {
	m := New(2, 0)
	m.Cols[0].Name = "Ad"
	m.Cols[1] = Col{Name: "Sayı", Kind: KindNumber, Align: 2, Calc: "sum"}
	m.Rows = [][]string{{"muz", "12"}, {"elma", "3"}, {"Çilek", "7"}, {"armut", ""}, {"incir", "100"}}

	m.ToggleSort(1)
	if !m.SortedAsc(1) {
		t.Fatal("expected ascending sort on numeric column")
	}
	if m.Rows[len(m.Rows)-1][0] != "armut" {
		t.Fatalf("expected blank cell last, got %v", m.Rows)
	}

	m.Filter = "cilek"
	if vis := m.Visible(); !slices.Equal(vis, []int{1}) {
		t.Fatalf("Filter visible = %v, want [1]", vis)
	}
	if got := m.Calc(1); got != "7" {
		t.Fatalf("filtered Calc = %q, want 7", got)
	}

	m.Filter = ""
	m.Rows[0][0] = `=CMD("x")`
	csvOut := m.CSVText(',')
	if !strings.Contains(csvOut, `'=CMD`) {
		t.Fatalf("expected formula-trigger quote in CSV, got %q", csvOut)
	}
	back := FromCSV("Ad\tSayı\nelma\t3\narmut\t12")
	if back == nil || back.Cols[1].Kind != KindNumber || len(back.Rows) != 2 {
		t.Fatalf("FromCSV got %+v", back)
	}
	if !Tabular("a\tb\n1\t2") || Tabular("a\tb\n1") {
		t.Fatal("Tabular mismatch")
	}
}

func TestCSVSafeCell(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"-5", "-5"},
		{"+3.2", "+3.2"},
		{"-0.25", "-0.25"},
		{"-", "'-"},
		{"+", "'+"},
		{"- madde", "'- madde"},
		{"-1_000", "'-1_000"},
		{"-Inf", "'-Inf"},
		{"+NaN", "'+NaN"},
		{"=SUM(A1)", "'=SUM(A1)"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		{"plain", "plain"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := csvSafeCell(tc.in); got != tc.want {
			t.Errorf("csvSafeCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
