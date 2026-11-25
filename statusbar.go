package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
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
	if a.pane().page == nil || a.pane().doc == nil || a.noPage() {
		return
	}
	if a.settings.ShowPos {
		_, caret := a.pane().doc.Selection()
		line, col := caretPos(a.pane().doc.Text(), caret)
		ui.Text(c, locales.SidebarLineColumn(line, col)).FontSize(11.5).TextColor(c.Theme().TextMuted).MaxLines(1)
	}
}

// hint is a key to press, and what it does.
type hint struct{ key, what string }

// hints are the keys that can be pressed now, by where the user is: the first the most useful. The keys known to
// all (⌘P, ⌘K, ⌘F) are not among them: the palette lists every key.
func (a *app) hints() []hint {
	switch {
	case a.lapse.on && (a.lapse.solo != 0 || a.lapse.lit != 0) && !a.pal.open:
		return []hint{{"Esc", "back to the text"}}
	case a.pal.open:
		return []hint{{"↑↓", "choose2"}, {"↵", "on"}, {"Esc", "off3"}, {">", "commands3"}, {"#", "tags4"}, {":", "goToALine"}}
	case a.pane().find.open:
		return []hint{{"↵", "next"}, {"⇧↵", "previous"}, {"Esc", "off3"}}
	case a.slash.open:
		return []hint{{"↑↓", "choose2"}, {"↵", "insert"}, {"Esc", "off3"}}
	case a.kb.normal:
		return []hint{{"f", "letterLabels"}, {"i", "write2"}, {"j k", "move2"}, {"Space", "commands3"}, {"?", "allShortcuts"}}
	case a.pane().settingsOpen:
		return []hint{{"/", "searchSettings2"}, {"Esc", "back3"}}
	case a.pane().statsScreen, a.pane().profileScreen, a.pane().trashOpen, a.pane().tasksOpen, a.pane().searchOpen, a.pane().tag != "":
		return []hint{{"Esc", "back3"}}
	case a.pane().coll != nil:
		return []hint{{"⌘N", "newPage3"}, {"⌘\\", "sidebar3"}}
	case a.pane().page != nil && !a.noPage() && a.pane().cellEd.open:
		return []hint{{"Tab", "nextCell"}, {"⇧Tab", "previousCell"}, {"↑↓", "row"}, {"↵", "keep"}}
	case a.pane().page != nil && !a.noPage() && a.pane().titleFocused:
		return []hint{{"↵", "intoTheText"}}
	case a.pane().page != nil && !a.noPage() && a.pane().doc != nil && a.pane().focused:
		return a.editorHints() // the block the caret is in (blockhints.go)
	}
	return nil
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
				ui.Text(c, msg(h.what)).FontSize(11.5).TextColor(cl.Muted).NoWrap()
			})
		}
	})
}
