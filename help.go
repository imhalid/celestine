package main

import (
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The help: every shortcut on one page (? in the normal mode, ⌘⇧/ or the command), grouped by where it is
// used.

type helpGroup struct {
	title string
	rows  [][2]string
}

// helpGroups are the shortcuts. The commands that have a key are listed from the palette's own list, so
// that one that is added there is here.
func (a *app) helpGroups() []helpGroup {
	L := locales.In(lang)
	groups := []helpGroup{
		{L.HelpEverywhere(), [][2]string{
			{"⌘P · ⇧⌘P", L.HelpCommandsAndSettings()}, {"⌘K", L.HelpGoToPage()}, {"⌘,", L.HelpSettings()},
			{"⌘S", L.HelpSave()}, {"⌘F", L.HelpFindInPage()}, {"⇧⌘F", L.HelpSearchLibrary()}, {"⌘J", L.HelpToday()},
			{"⌘\\", L.HelpSidebar()}, {"⇧⌘\\", L.HelpSidePanel()}, {"⌘[ ⌘]", L.HelpBackForward()}, {"⌘= ⌘− ⌘0", L.HelpTextSize()},
			{"⌘1 ⌘2 ⌘3", L.HelpFocus()}, {"⌥⌘→ ⌥⌘↵ ⌥⌘W", L.HelpPanes()},
		}},
		{L.HelpNormalMode(), [][2]string{
			{"j k ↓ ↑", L.HelpNextLine()}, {"gg G", L.HelpFirstLast()}, {"l ↵", L.HelpOpenSelected()}, {"h", L.HelpBack()},
			{"f", L.HelpHints()}, {"Space", L.HelpLeader()},
			{"/", L.HelpFindInPage()}, {":", L.HelpCommandLine()}, {"m a · ' a", L.HelpMarks()},
			{"[ ]", L.HelpJournalDays()}, {"Space g", L.HelpGoToDate()},
			{"u ⌃R", L.HelpUndoRedo()}, {"p o s t b n w", L.HelpLeaderKeys()}, {"?", L.HelpThisPage()},
		}},
		{L.HelpPaletteModes(), [][2]string{
			{"@", L.HelpPagesText()}, {">", L.HelpCommandsExample()}, {"#", L.HelpTags()}, {":", L.HelpLineCommands()},
			{"/", L.HelpSearchThisPage()}, {"!", L.HelpRecentCommands()}, {"?", L.HelpListModes()},
		}},
		{L.HelpText(), [][2]string{
			{"⌘B ⌘I ⌘U", L.HelpBoldItalic()}, {"⌘K", L.HelpLink()}, {"⌘Z ⇧⌘Z", L.HelpUndoRedo()}, {"⌘/", L.HelpFold()}, {"⇧⌘↑ ⇧⌘↓", L.HelpMoveBlock()},
			{"⌘D", L.HelpDuplicateBlock()}, {"/", L.HelpLineMenu()}, {"Tab ⇧Tab", L.HelpIndent()}, {L.HelpCtrlSpace(), L.HelpLineMenu()},
		}},
	}
	var cmds [][2]string
	seen := map[string]bool{}
	for _, it := range a.commands() {
		if it.keys != "" && it.keys != "✓" && it.kind == palCommand && !seen[it.title] {
			seen[it.title] = true
			cmds = append(cmds, [2]string{it.keys, it.title})
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i][1] < cmds[j][1] })
	if len(cmds) > 0 {
		groups = append(groups, helpGroup{L.HelpCommandKeys(), cmds})
	}
	return groups
}

// helpView is the page of shortcuts.
func (a *app) helpView(c *ui.Context) {
	if !a.kb.help {
		a.kb.helpKept = nil
		return
	}
	if a.kb.helpKept == nil {
		a.kb.helpKept = a.helpGroups()
	}
	cl := ryugo.From(c).Colors
	ryugo.Dialog(c.Key("help"), &a.kb.help, ryugo.DialogOptions{Title: locales.HelpTitle(), Width: 680}, func() {
		a.scrolled(c, "help-scroll", nil, ui.Scroll(c.Key("help-scroll")).MaxHeight(520), func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 6, 0, 0).Children(func() {
				for gi, g := range a.kb.helpKept {
					ui.Column(c.Key("hg" + itoa(gi))).FillWidth().Gap(4).Children(func() {
						ui.Text(c, g.title).FontSize(13).FontWeight(680).TextColor(cl.Muted)
						for ri, r := range g.rows {
							ui.Row(c.Key("hr"+itoa(gi)+":"+itoa(ri))).FillWidth().Gap(12).Padding(2, 0).Children(func() {
								ui.Text(c, r[0]).FontSize(12.5).FontWeight(650).Font(monoStack).TextColor(cl.Accent).Width(150)
								ui.Text(c, r[1]).FontSize(13).TextColor(cl.Text).Grow(1)
							})
						}
					})
				}
				ui.Text(c, locales.HelpEverythingWithoutMousePress()).FontSize(12.5).TextColor(cl.Muted).Wrap()
			})
		})
	})
}
