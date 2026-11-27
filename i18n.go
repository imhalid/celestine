package main

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/i18n"
	"github.com/imhalid/celestine/locales"
)

// The language of the window (docs/24-i18n.md). Its messages are in the catalogs of locales/, a JSON file per
// language through MyGo's i18n package, English the source: locales/en.json has every message, and a language
// that lacks one says it in English. The code calls a message by its function (locales.UpdateTitle()); a table
// that keeps a message by its id (a command's title, a setting's name) says it with msg(id).
//
// A message is added in locales/en.json and its translation in the other files; `mygo generate` (the mygo
// command of the MyGo version go.mod requires) writes locales/catalog_gen.go again, and TestCatalogs checks that
// every language has every message.

// A language is one the window can be shown in.
type language struct {
	code string // a BCP 47 tag, as its catalog is named
	name string // in itself
	rtl  bool   // written right to left
}

// languages are the window's languages, in the order the settings offer them.
var languages = []language{{"en", "English", false}, {"tr", "Türkçe", false}}

// lang is the code of the window's language. It is set by setLang (and withLang in tests), which also keep
// langNow, the copy other goroutines read.
var lang = "tr"

// langNow is lang for any goroutine: the language MyGo's i18n package says the typed messages of locales/ in.
var langNow atomic.Value

func init() {
	langNow.Store(lang)
	// the messages of locales/ (locales.Name()) and MyGo's own words follow the window's language, not the
	// system's: this replaces the function package mygo set at its init, which runs before this one
	i18n.SetLocaleFunc(func() string { return langNow.Load().(string) })
}

// showLocale tells MyGo the window's language, so that its own words, menus and text engine follow it: set by
// main to mygo.App.SetLocale, which also keeps it; tests leave it alone.
var showLocale = func(string) {}

// setLang sets the language from the setting: English, unless another of the languages is chosen.
func setLang(setting string) {
	lang = "en"
	if i := slices.IndexFunc(languages, func(l language) bool { return l.code == setting }); i >= 0 {
		lang = setting
	}
	langNow.Store(lang)
	showLocale(lang)
}

// msg is the message of id in the window's language: for the tables that keep a message by its id (a command's
// title, a setting's name). What is no id, a page's title or what a user typed, is said as it is.
func msg(id string) string {
	if _, _, ok := locales.Catalog.Message(lang, id); ok {
		return locales.Catalog.Format(lang, id, nil)
	}
	// an id with a value after it: "collectionSettings: Books" says the id and keeps the value
	if head, rest, ok := strings.Cut(id, ": "); ok {
		if _, _, ok := locales.Catalog.Message(lang, head); ok {
			return strings.TrimSuffix(locales.Catalog.Format(lang, head, nil), ": ") + ": " + rest
		}
	}
	return id
}

// namesList are the messages of id in each language of the window, kept separate so searches and prefix rankings match
// within one language at a time.
func namesList(id string) []string {
	var out []string
	for _, l := range languages {
		if _, _, ok := locales.Catalog.Message(l.code, id); ok {
			out = append(out, locales.Catalog.Format(l.code, id, nil))
		}
	}
	if len(out) == 0 {
		return []string{id}
	}
	return out
}

// names are the words a message of id is found by, typed: its own in each language of the window, so that a
// setting or a command is found by its Turkish name in the English window too.
func names(id string) string {
	return strings.Join(namesList(id), " ") + " "
}

// langName is the language's name, in itself.
func langName(code string) string {
	for _, l := range languages {
		if l.code == code {
			return l.name
		}
	}
	return languages[0].name
}
