package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// Copying and pasting in a form of one's choosing: the selection (or the whole page, with none) as Markdown, plain
// text or HTML; and what the clipboard has as Markdown, made into blocks, or as plain text, word for word. Chips
// (dates, page links) are their words in plain text and HTML, not pictures.

// copySource is what is copied: the selection, or the whole page when nothing is selected.
func (a *app) copySource() richtext.Text {
	t := a.doc.Text()
	sa, sc := a.doc.Selection()
	if s, e := min(sa, sc), max(sa, sc); s < e {
		return t.Slice(s, e)
	}
	return t
}

// copyAs puts the selection, or the page, on the clipboard as "markdown", "plain" or "html".
func (a *app) copyAs(kind string) {
	if a.doc == nil || a.noPage() {
		return
	}
	src := a.copySource()
	switch kind {
	case "markdown":
		a.copyText(strings.TrimSuffix(bodyMarkdown(src), "\n"))
	case "plain":
		a.copyText(withoutChips(src).PlainText())
	case "html":
		a.copyText(withoutChips(src).HTML())
	}
	a.notice = map[string]string{"markdown": T("Copied as Markdown."), "plain": T("Copied as plain text."), "html": T("Copied as HTML.")}[kind]
	a.refocus = true
}

// pasteAs puts what the clipboard has at the caret, over the selection: as "markdown", its lines made into blocks,
// or as "plain" text, as it is.
func (a *app) pasteAs(kind string) {
	if a.doc == nil || a.noPage() {
		return
	}
	clip := a.readClip()
	if clip == "" {
		return
	}
	if kind == "markdown" {
		a.doc.Insert(importMarkdown(clip))
	} else {
		a.doc.InsertText(clip)
	}
	a.refocus = true
}
