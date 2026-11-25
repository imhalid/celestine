package main

import "github.com/imhalid/celestine/internal/table"

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
	kindName          = table.KindName
	calcName          = table.CalcName
	nextOf            = table.NextOf
	splitCells        = table.SplitCells
)

type (
	tableCol   = table.Col
	tableModel = table.Model
)
