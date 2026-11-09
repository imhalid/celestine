package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
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

// Copy and paste inside the app keep what a block is beyond its text: a code block's language, a task's kind, a
// divider. The editor keeps those in its paragraphs' IDs, and its own copy clears every ID, so ⌘C and ⌘X also keep
// the copy as the app's Markdown, and ⌘V, while the clipboard still holds that copy's text, pastes the blocks made
// from it.

// ownClip is the last copy in the editor that had such blocks: its plain text, as the clipboard has it, and its
// Markdown.
type ownClip struct{ plain, md string }

// editorKeys are the keys the app takes before the editor: the / menu's, then copy and paste.
func (a *app) editorKeys(key ui.Key, mods ui.Modifiers) bool {
	return a.editorKey(key, mods) || a.clipKey(key, mods)
}

// clipKey notes a copy (⌘C, ⌘X) and lets the editor make it; it takes a paste (⌘V) of that copy.
func (a *app) clipKey(key ui.Key, mods ui.Modifiers) bool {
	if mods != ui.Cmd || a.doc == nil {
		return false
	}
	switch key {
	case ui.KeyC, ui.KeyX:
		a.noteCopy()
	case ui.KeyV:
		return a.pasteOwn()
	}
	return false
}

// noteCopy keeps the selection as Markdown when a block in it has an ID the editor's copy would lose.
func (a *app) noteCopy() {
	a.ownClip = ownClip{}
	t := a.doc.Text()
	sa, sc := a.doc.Selection()
	s, e := min(sa, sc), max(sa, sc)
	if s == e {
		return
	}
	src := t.Slice(s, e)
	for _, p := range src.Paragraphs() {
		if p.Style.ID != "" {
			a.ownClip = ownClip{plain: src.PlainText(), md: bodyMarkdown(src)}
			return
		}
	}
}

// pasteOwn pastes the blocks of the last copy, when the clipboard still holds its text; otherwise the editor pastes.
func (a *app) pasteOwn() bool {
	c := a.ownClip
	if c.md == "" || a.readClip() != c.plain {
		return false
	}
	t := importMarkdown(c.md)
	sa, sc := a.doc.Selection()
	at := min(sa, sc)
	ps, _ := a.doc.Text().ParagraphRange(at)
	a.doc.Insert(t)
	// the editor's insert gives the new paragraphs no IDs: they are given back, but to the first only when it
	// starts a paragraph, not when it joins the one the caret was in
	a.doc.Edit(func(ed *richtext.Edit) {
		for start, p := range t.Paragraphs() {
			if p.Style.ID == "" || start == 0 && at != ps {
				continue
			}
			id := p.Style.ID
			ed.FormatParagraphs(at+start, at+start, func(st *richtext.ParagraphStyle) { st.ID = id })
		}
	})
	return true
}
