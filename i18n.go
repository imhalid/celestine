package main

import (
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/i18n"
	"github.com/imhalid/celestine/locales"
)

// The language of the window (docs/24-i18n.md). Its messages are in the catalogs of locales/, a JSON file per
// language through MyGo's i18n package, English the source: locales/en.json has every message, and a language
// that lacks one says it in English. The code still gives T the text as it writes it, the Turkish of the older
// strings and the English of the newer ones; codeKeys (i18n_keys.go) names the message of each, until phase 2
// (#331) calls the messages by name. A text with no message shows as it was written, not as a hole.
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

// T is s in the language of the window: s is a text as the code writes it (codeKeys).
func T(s string) string {
	if id, ok := codeKeys[s]; ok {
		return locales.Catalog.Format(lang, id, nil)
	}
	// a title with a name after it: "Çöpe taşı: Sayfa" → the part before is translated
	if i := strings.Index(s, ": "); i > 0 {
		if id, ok := codeKeys[s[:i]]; ok {
			return locales.Catalog.Format(lang, id, nil) + s[i:]
		}
	}
	return s
}

// Tf is T for a sentence with values in it: the format is translated, then filled. The catalogs keep the
// formats of Go's fmt for now; phase 2 (#331) makes them messages with named arguments and plurals.
func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }

// langName is the language's name, in itself.
func langName(code string) string {
	for _, l := range languages {
		if l.code == code {
			return l.name
		}
	}
	return languages[0].name
}
