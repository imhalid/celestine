package main

import (
	"github.com/imhalid/celestine/internal/table"
	"github.com/imhalid/celestine/locales"
)

// Root aliases into internal/table mirroring core_alias.go so existing table UI and tests stay concise.
const (
	tableLang  = table.Lang
	kindText   = table.KindText
	kindNumber = table.KindNumber
	kindDate   = table.KindDate
	kindCheck  = table.KindCheck
)

var (
	tableKinds        = table.Kinds
	calcs             = table.Calcs
	newTable          = table.New
	parseTable        = table.Parse
	tableFromMarkdown = table.FromMarkdown
	tableFromCSV      = table.FromCSV
	tabular           = table.Tabular
	parseNumber       = table.ParseNumber
	formatNumber      = table.FormatNumber
	nextOf            = table.NextOf
	splitCells        = table.SplitCells
)

type (
	tableCol   = table.Col
	tableModel = table.Model
)

// The words of the tables, which internal/table leaves to the app (#595): a new column's name, a kind's and a footer
// calculation's.
func init() { table.ColumnName = locales.TablesColumnN }

// kindName is the name of column kind k, in the window's language.
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

// calcName is the name of footer calculation k, in the window's language.
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
