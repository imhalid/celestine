package main

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// More of the status bar, as the settings choose (Ayarlar ▸ Durum çubuğu):
// where the page is, when it was last changed, who links to it, what is
// selected, where the caret is, how long it takes to read.

// readingMinutes is how long the words take to read, at 200 a minute.
func readingMinutes(words int) int {
	return (words + 199) / 200
}

// readingText says it: "3 dk okuma", "1 dk'dan az" for a few lines.
func readingText(words int) string {
	if words < 100 {
		return "1 dk'dan az okuma"
	}
	return strconv.Itoa(readingMinutes(words)) + " dk okuma"
}

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
	s := &a.settings
	t := c.Theme()
	small := func(text string) ui.Element {
		return ui.Text(c, text).FontSize(11.5).TextColor(t.TextMuted).MaxLines(1)
	}
	p := a.page
	if s.ShowToday {
		b := ui.Row(c.Key("status-today")).Padding(1, 5).Radius(rad(5)).AlignItems(ui.Center).Label("Yazma istatistikleri").Role(ui.RoleButton).Cursor(ui.CursorPointer)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() { small("bugün " + strconv.Itoa(a.writtenOn(a.clock())) + " kelime") })
		if b.Clicked() {
			a.statsOpen = true
		}
	}
	if s.ShowPath {
		path := p.Coll.Name
		if crumb := parents(p); crumb != "" {
			path += " › " + crumb
		}
		small(path + " › " + p.Title).Shrink(1)
	}
	if s.ShowEdited && !p.Modified.IsZero() {
		small(ago(p.Modified, a.clock()) + " düzenlendi")
	}
	if s.ShowLinks {
		n := len(a.linksTo(p, a.clock(), false))
		b := ui.Row(c.Key("status-links")).Padding(1, 5).Radius(rad(5)).AlignItems(ui.Center).Label("Geri bağlantılar: yan paneli aç").Role(ui.RoleButton).Cursor(ui.CursorPointer)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() { small(strconv.Itoa(n) + " geri bağlantı") })
		if b.Clicked() && !s.PanelOpen {
			a.togglePanel()
		}
	}
	anchor, caret := a.doc.Selection()
	if s.ShowSel && anchor != caret {
		sel := a.doc.Text().Slice(min(anchor, caret), max(anchor, caret)).String()
		sel = strings.ReplaceAll(strings.ReplaceAll(sel, "\n", ""), string(richtext.ObjectReplacement), "")
		small(strconv.Itoa(utf8.RuneCountInString(sel)) + " harf seçili")
	}
	if s.ShowPos {
		line, col := caretPos(a.doc.Text(), caret)
		small("Satır " + strconv.Itoa(line) + ", sütun " + strconv.Itoa(col))
	}
	if s.ShowReading {
		words, _ := a.docStats()
		small(readingText(words))
	}
}
