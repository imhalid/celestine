package main

import (
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/locales"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Copying and pasting in a form of one's choosing: the selection (or the whole page, with none) as Markdown, plain
// text or HTML; and what the clipboard has as Markdown, made into blocks, or as plain text, word for word. Chips
// (dates, page links) are their words in plain text and HTML, not pictures.

// copySource is what is copied: the selection, or the whole page when nothing is selected.
func (a *app) copySource() richtext.Text {
	doc := a.pane().doc
	t := doc.Text()
	sa, sc := doc.Selection()
	if s, e := min(sa, sc), max(sa, sc); s < e {
		return t.Slice(s, e)
	}
	return t
}

// copyAs puts the selection, or the page, on the clipboard as "markdown", "plain" or "html".
func (a *app) copyAs(kind string) {
	pn := a.pane()
	if pn.doc == nil || a.noPage() {
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
	a.say(toastOK, map[string]string{"markdown": locales.BlocksCopiedAsMarkdown(), "plain": locales.BlocksCopiedAsPlainText(), "html": locales.BlocksCopiedAsHtml()}[kind])
	pn.refocus = true
}

// pasteAs puts what the clipboard has at the caret, over the selection: as "markdown", its lines made into blocks,
// or as "plain" text, as it is.
func (a *app) pasteAs(kind string) {
	pn := a.pane()
	if pn.doc == nil || a.noPage() {
		return
	}
	clip := a.readClip()
	if clip == "" {
		return
	}
	if kind == "markdown" {
		a.insertBlocks(a.parse(clip))
	} else {
		pn.doc.InsertText(clip)
	}
	pn.refocus = true
}

// Copy and paste inside the app keep what a block is beyond its text: a code block's language, a task's kind, a
// divider. The editor keeps those in its paragraphs' IDs, and its own copy clears every ID, so ⌘C and ⌘X also keep
// the copy as the app's Markdown, and ⌘V, while the clipboard still holds that copy's text, pastes the blocks made
// from it. A copy with chips in it and no such blocks is kept as the text itself, and pasted with chips of its own:
// the editor's paste brings back the very chips it copied, so a task copied and pasted shared one box with the first,
// and ticking one ticked both (#539). The app pastes it in one edit, which one undo takes back whole.

// ownClip is the last copy in the editor that had such blocks or chips: its plain text, as the clipboard has it,
// and its Markdown, or the text itself (rich).
type ownClip struct {
	plain, md string
	rich      richtext.Text
}

// editorKeys are the keys the app takes before the editor: the / menu's, then copy and paste.
func (a *app) editorKeys(pn *paneState, key ui.Key, mods ui.Modifiers) bool {
	if a.pageLocked(pn.page) { // a past day (#622): the editor reads only, and none of the app's edits is made
		switch {
		case mods == ui.Cmd && key == ui.KeyC:
			a.noteCopy()
		case mods == ui.Cmd && (key == ui.KeyV || key == ui.KeyX), mods == 0 && (key == ui.KeyTab || key == ui.KeyEnter || key == ui.KeyBackspace || key == ui.KeyDelete):
			a.sayLocked()
		}
		return false
	}
	return a.editorKey(pn, key, mods) || a.clipKey(pn, key, mods)
}

// editorCommand takes an edit command of the menus before the editor does it: a Mac's menu bar sends ⌘V as
// "paste", ⌘C as "copy", which the editor's keys never see (#509). They are done as their keys are.
func (a *app) editorCommand(pn *paneState, name string) bool {
	if pn.doc == nil {
		return false
	}
	if a.pageLocked(pn.page) && name != "copy" { // a past day (#622): nothing pasted nor cut
		a.sayLocked()
		return true
	}
	switch name {
	case "copy", "cut":
		a.noteCopy()
	case "paste":
		return a.pasteLink() || a.pasteOwn() || a.pasteMarkdown()
	}
	return false
}

// clipKey notes a copy (⌘C, ⌘X) and lets the editor make it; it takes a paste (⌘V) of that copy.
func (a *app) clipKey(pn *paneState, key ui.Key, mods ui.Modifiers) bool {
	if mods != ui.Cmd || pn.doc == nil {
		return false
	}
	switch key {
	case ui.KeyC, ui.KeyX:
		a.noteCopy()
	case ui.KeyV:
		return a.pasteLink() || a.pasteOwn() || a.pasteMarkdown()
	}
	return false
}

// pasteLink makes the text selected a link when what is pasted over it is an address alone, as the bar's link
// field would, with its flourish (#509): the text stays, the address is its link's.
func (a *app) pasteLink() bool {
	pn := a.pane()
	if a.noPage() {
		return false
	}
	anchor, caret := pn.doc.Selection()
	s, e := min(anchor, caret), max(anchor, caret)
	if s == e {
		return false
	}
	t := pn.doc.Text()
	if t.ParagraphStyleAt(s).CodeBlock || t.AttributesAt(s).Code || strings.TrimSpace(t.Slice(s, e).String()) == "" {
		return false
	}
	address := strings.TrimSpace(a.readClip())
	if !pastedAddress(address) {
		return false
	}
	pn.doc.SetLink(address)
	pn.sel.wonSel, pn.sel.wonAt = [2]int{s, e}, a.frameNow
	return true
}

// pastedAddress says whether text is an address alone, which pasted over a selection links it: the web's, an
// e-mail's, a page's of the app.
func pastedAddress(text string) bool {
	if text == "" || strings.ContainsAny(text, " \t\n") {
		return false
	}
	for _, pre := range []string{"https://", "http://", "mailto:", "page://"} {
		if strings.HasPrefix(text, pre) && len(text) > len(pre) {
			return true
		}
	}
	return false
}

// pasteMarkdown pastes text with Markdown blocks in it as those blocks: a table, a fenced code block, headings,
// lists, a quotation, a divider, footnotes, as a file of it dropped on the page makes them (#307). Text the clipboard
// also has as HTML (a web page's) is the editor's to paste, which reads its HTML; ⇧⌘V pastes the text as it is.
func (a *app) pasteMarkdown() bool {
	if a.noPage() {
		return false
	}
	clip := a.readClip()
	if !looksMarkdown(clip) || a.clipHasHTML() {
		return false
	}
	a.insertBlocks(a.parse(clip))
	return true
}

// clipHasHTML says whether the clipboard holds HTML too (none in a test).
func (a *app) clipHasHTML() bool {
	if a.clipRead != nil {
		return false
	}
	return strings.TrimSpace(mygo.Clipboard.ReadHTML()) != ""
}

// mdBlockLine is a line that starts a block of Markdown: a fence, a heading, an item of a list (a task too), a
// quotation, a divider, a footnote's note, a picture.
var mdBlockLine = regexp.MustCompile(`^(?:\s{0,3}(?:` + "```" + `|~~~)|\s{0,3}#{1,6}\s|\s*(?:[-*+]|\d+[.)])\s|\s{0,3}>|\s{0,3}(?:-{3,}|\*{3,}|_{3,})\s*$|\[\^[^\]]+\]:|\s*!\[)`)

// looksMarkdown says whether text has blocks of Markdown in it, or a table: then it is pasted as them.
func looksMarkdown(text string) bool {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, l := range lines {
		if mdBlockLine.MatchString(l) {
			return true
		}
		if strings.Contains(l, "|") && i+1 < len(lines) && delimRow.MatchString(lines[i+1]) {
			return true
		}
	}
	return false
}

// insertBlocks puts t at the caret, over the selection, with what its paragraphs are (their IDs: a code block's
// language, a divider, a task's kind), which the editor's Insert gives none of: to the first only when it starts
// a paragraph, not when it joins the one the caret was in.
func (a *app) insertBlocks(t richtext.Text) {
	pn := a.pane()
	sa, sc := pn.doc.Selection()
	at := min(sa, sc)
	ps, _ := pn.doc.Text().ParagraphRange(at)
	pn.doc.Insert(t)
	pn.doc.Edit(func(ed *richtext.Edit) {
		for start, p := range t.Paragraphs() {
			if p.Style.ID == "" || start == 0 && at != ps {
				continue
			}
			id := p.Style.ID
			ed.FormatParagraphs(at+start, at+start, func(st *richtext.ParagraphStyle) { st.ID = id })
		}
	})
	pn.refocus = true
}

// noteCopy keeps the selection as Markdown when a block in it has an ID the editor's copy would lose, and as it is
// when it holds a chip.
func (a *app) noteCopy() {
	pn := a.pane()
	a.ownClip = ownClip{}
	t := pn.doc.Text()
	sa, sc := pn.doc.Selection()
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
	for _, r := range src.Runs(0, src.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if _, ok := att.Data.(*chip); ok {
				a.ownClip = ownClip{plain: src.PlainText(), rich: src}
				return
			}
		}
	}
}

// pasteOwn pastes the blocks of the last copy, or its text with chips of its own (copyChipsIn), when the clipboard
// still holds its text; otherwise the editor pastes.
func (a *app) pasteOwn() bool {
	pn := a.pane()
	c := a.ownClip
	if c.md == "" && c.rich.Len() == 0 || a.readClip() != c.plain {
		return false
	}
	if c.md != "" {
		a.insertBlocks(a.parse(c.md))
		return true
	}
	t := a.copyChipsIn(c.rich)
	sa, sc := pn.doc.Selection()
	s, e := min(sa, sc), max(sa, sc)
	pn.doc.Edit(func(ed *richtext.Edit) { // as the editor pastes, in one step of undo
		ed.Replace(s, e, t)
		ed.Select(s+t.Len(), s+t.Len())
	})
	pn.refocus = true
	return true
}
