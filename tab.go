package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// tabKey makes Tab, in the text, stay in the text: a tab where the caret is, or, in a list or
// with several paragraphs selected, a level of indentation; ⇧Tab takes it back. richtext takes
// Tab only in a list and a block of code, and elsewhere the key goes on to move the focus (with
// its ring) away from the text. So while the text has the focus the key is ours, in every kind
// of paragraph: a Tab not claimed is a Tab lost.
//
// It is asked for in the frame the editor is made in, once its focus is known.
func (a *app) tabKey(c *ui.Context, pn *paneState) {
	if !pn.focused || pn.doc == nil || pn.page == nil {
		return
	}
	tab, back := c.Shortcut(0, ui.KeyTab), c.Shortcut(ui.Shift, ui.KeyTab)
	if !tab && !back || a.slash.open {
		return
	}
	if tab && a.tabOutOfInlineTask(pn) { // in a task of the running text: out of it, not a level in (#610)
		pn.revealWant = true
		c.Invalidate()
		return
	}
	anchor, caret := pn.doc.Selection()
	text := pn.doc.Text()
	st := text.ParagraphStyleAt(caret)
	lo, hi := min(anchor, caret), max(anchor, caret)
	_, end := text.ParagraphRange(lo)
	several := hi > end // the selection reaches another paragraph
	list := st.List != richtext.NoList
	switch {
	case tab && (several || list):
		pn.doc.Indent(1)
	case tab:
		pn.doc.InsertText(a.tabText())
	case several || list:
		pn.doc.Indent(-1)
	case anchor == caret && caret >= len(a.tabText()) && a.tabText() != "\t" && text.Slice(caret-len(a.tabText()), caret).String() == a.tabText():
		pn.doc.SetSelection(caret-len(a.tabText()), caret)
		pn.doc.InsertText("")
	case anchor == caret && caret > 0 && text.Slice(caret-1, caret).String() == "\t":
		pn.doc.SetSelection(caret-1, caret)
		pn.doc.InsertText("")
	case st.Indent > 0:
		pn.doc.Indent(-1)
	}
	pn.revealWant = true
	c.Invalidate()
}
