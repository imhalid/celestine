package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A right click on the page's title opens the app's context menu: the title's text to cut, copy,
// paste or select, then what the page's "…" menu offers, renaming left out, as the title is where
// that happens.

var (
	iconCut       = icon(`<circle cx="7" cy="17" r="2.5"/><circle cx="17" cy="17" r="2.5"/><path d="M8.8 15.2 17 5M15.2 15.2 7 5"/>`)
	iconPaste     = icon(`<rect x="6" y="5" width="12" height="15" rx="2.2"/><path d="M9.5 5V4a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1v1"/>`)
	iconSelectAll = icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="2.5" stroke-dasharray="3 2.4"/><path d="M8.5 12h7"/>`)
)

// readClip is the clipboard's text: the tests' when they set one.
func (a *app) readClip() string {
	if a.clipRead != nil {
		return a.clipRead()
	}
	return mygo.Clipboard.ReadText()
}

// titleMenu gives the page's title (a text input, or the journal's text) the app's menu: its text,
// then what the page's "…" menu offers, renaming left out, as the title is where that happens.
func (a *app) titleMenu(title ui.Element, p *Page, editable bool) {
	more := func() []menuItem {
		var out []menuItem
		for _, it := range a.pageMoreMenu(p) {
			if it.title != "Yeniden adlandır" {
				out = append(out, it)
			}
		}
		return out
	}
	if !editable {
		a.textMenu(title, func() string { return p.Title }, more)
		return
	}
	a.fieldMenu(title, &a.title, fieldOpts{oneLine: true, more: more, changed: func() {
		name := strings.TrimSpace(a.title)
		if name == "" {
			name = "Adsız"
		}
		if err := p.SetTitle(name); err != nil {
			a.saveErr = err.Error()
		}
	}})
}
