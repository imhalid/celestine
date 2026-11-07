package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
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
		ui.Text(c, T("Satır ")+strconv.Itoa(line)+T(", sütun ")+strconv.Itoa(col)).FontSize(11.5).TextColor(c.Theme().TextMuted).MaxLines(1)
	}
}

// hint is a key to press, and what it does.
type hint struct{ key, what string }

// hints are the keys that can be pressed now, by where the user is: the first the most useful.
func (a *app) hints() []hint {
	switch {
	case a.pal.open:
		return []hint{{"↑↓", "seç"}, {"↵", "aç"}, {"Esc", "kapat"}, {">", "komutlar"}, {"#", "etiketler"}, {":", "satıra git"}}
	case a.find.open:
		return []hint{{"↵", "sonraki"}, {"⇧↵", "önceki"}, {"Esc", "kapat"}}
	case a.slash.open:
		return []hint{{"↑↓", "seç"}, {"↵", "ekle"}, {"Esc", "kapat"}}
	case a.kb.normal:
		return []hint{{"f", "harf etiketleri"}, {"i", "yaz"}, {"j k", "gez"}, {"Space", "komutlar"}, {"?", "tüm kısayollar"}}
	case a.settingsOpen:
		return []hint{{"/", "ayar ara"}, {"Esc", "geri"}}
	case a.statsScreen, a.trashOpen, a.tasksOpen, a.searchOpen, a.tag != "":
		return []hint{{"Esc", "geri"}, {"⌘P", "komutlar"}, {"⌘K", "sayfa ara"}}
	case a.coll != nil:
		return []hint{{"⌘P", "komutlar"}, {"⌘K", "sayfa ara"}, {"⌘N", "yeni sayfa"}, {"⌘\\", "kenar çubuğu"}}
	case a.page != nil && !a.noPage():
		return []hint{{"⌘P", "komutlar"}, {"⌘K", "sayfa ara"}, {"⌘F", "sayfada bul"}, {"/", "blok ekle"}, {"[[", "bağlantı"}, {"Esc", "normal kip"}}
	}
	return []hint{{"⌘P", "komutlar"}, {"⌘K", "sayfa ara"}}
}

// statusHints writes the keys that can be pressed now, at the left of the status bar, in the room the
// bar has: the ones that do not fit are left out, not squeezed.
func (a *app) statusHints(c *ui.Context) {
	cl := ryugo.From(c).Colors
	row := ui.Row(c.Key("status-hints")).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Center).Clip()
	if a.settings.HideHints || phone { // a phone has no keys to press
		row.Height(0)
		return
	}
	row.Children(func() {
		for _, h := range a.hints() {
			ui.Row(c.Key("hint:" + h.key)).Gap(5).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Row(c).Padding(1, 5).Radius(rad(4)).Background(cl.Fill).Border(1, cl.Hairline).Children(func() {
					ui.Text(c, h.key).FontSize(10.5).FontWeight(600).TextColor(cl.Text2).NoWrap()
				})
				ui.Text(c, T(h.what)).FontSize(11.5).TextColor(cl.Muted).NoWrap()
			})
		}
	})
}
