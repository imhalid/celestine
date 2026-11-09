package main

// enFind is the English of the words of the search's query language, the query block, the task view's kinds,
// and the journal's commands. It is merged into enText, and a key already there keeps its English.
var enFind = map[string]string{
	"Her kelimeyi içeren sayfalar. \"tam ifade\", VEYA, -kelime, (…), başlık:, koleksiyon:, #etiket, @todo, @done, @today, created:>2026-11-01, özellik:değer yazabilirsin.":        "Pages with every word. You can write \"an exact phrase\", OR, -word, (…), title:, in:, #tag, @todo, @done, @today, created:>2026-11-01, property:value.",
	"Sayfaların başlığında ve metninde ara; \"tam ifade\", VEYA, -kelime, başlık:, koleksiyon:, #etiket, @todo, @done, created:, modified: ve özellik:değer süzgeçleri de çalışır.": "Search the pages' titles and text; \"an exact phrase\", OR, -word, title:, in:, #tag, @todo, @done, created:, modified: and property:value filters work too.",
	"Bu koleksiyon, aramanın bulduğu sayfaları gösterir: kelime, \"tam ifade\", VEYA, -kelime, başlık:, koleksiyon:, #etiket, @todo, created:, özellik:değer.":                      "This collection shows the pages the search finds: words, \"an exact phrase\", OR, -word, title:, in:, #tag, @todo, created:, property:value.",
	// the query block (queryblock.go)
	"Sorgu":                            "Query",
	"Sorgu…":                           "Query…",
	"aramanın bulduğu sayfalar, canlı": "the pages a search finds, live",
	"Bir sorgu yaz: #etiket @todo …":   "Write a query: #tag @todo …",
	"ve daha fazlası: aramada aç ↗":    "and more: open in the search ↗",
	// the task view's kinds (tasks.go)
	"Tüm türler":     "All kinds",
	"Düz görev":      "Plain task",
	"Tür":            "Kind",
	"Yıldızlı görev": "Starred task",
	"Önemli görev":   "Important task",
	"Soru":           "Question",
	"Süren görev":    "Task in progress",
	"Hatalı görev":   "Failed task",
}

func init() {
	for k, v := range enFind {
		if _, ok := enText[k]; !ok {
			enText[k] = v
		}
	}
}
