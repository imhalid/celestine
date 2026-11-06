package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// More of the status bar, as the settings choose (Ayarlar ▸ Durum çubuğu): where the caret is.

// caretPos is the paragraph the caret is in (from 1) and its place in it.
func caretPos(t richtext.Text, caret int) (line, col int) {
	for at := range t.Paragraphs() {
		if at > caret {
			break
		}
		line++
	}
	ps, _ := t.ParagraphRange(min(caret, t.Len()))
	return max(line, 1), caret - ps + 1
}

// statusExtras draws the pieces of the status bar the settings ask for,
// while a page is open.
func (a *app) statusExtras(c *ui.Context) {
	if a.page == nil || a.doc == nil || a.noPage() {
		return
	}
	if a.settings.ShowPos {
		_, caret := a.doc.Selection()
		line, col := caretPos(a.doc.Text(), caret)
		ui.Text(c, "Satır "+strconv.Itoa(line)+", sütun "+strconv.Itoa(col)).FontSize(11.5).TextColor(c.Theme().TextMuted).MaxLines(1)
	}
}
