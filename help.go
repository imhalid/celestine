package main

import (
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The help: every shortcut on one page (? in the normal mode, ⌘⇧/ or the command), grouped by where it is
// used; and a short tour, the first time the app is opened, that says where the rest is.

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
		return
	}
	cl := ryugo.From(c).Colors
	ryugo.Dialog(c.Key("help"), &a.kb.help, ryugo.DialogOptions{Title: locales.HelpTitle(), Width: 680}, func() {
		a.scrolled(c, "help-scroll", nil, ui.Scroll(c.Key("help-scroll")).MaxHeight(520), func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 6, 0, 0).Children(func() {
				for gi, g := range a.helpGroups() {
					ui.Column(c.Key("hg" + itoa(gi))).FillWidth().Gap(4).Children(func() {
						ui.Text(c, g.title).FontSize(13).FontWeight(680).TextColor(cl.Muted)
						for ri, r := range g.rows {
							ui.Row(c.Key("hr"+itoa(gi)+":"+itoa(ri))).FillWidth().Gap(12).Padding(2, 0).Children(func() {
								ui.Text(c, r[0]).FontSize(12.5).FontWeight(650).Font(ryugo.FontMono).TextColor(cl.Accent).Width(150)
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

// ---- the tour ----

// tourSteps are the steps of the tour, each a title and a body, said in the window's language when shown.
var tourSteps = []struct{ title, body func() string }{
	{locales.HelpTourWrite, locales.HelpTourWriteBody},
	{locales.HelpTourPalette, locales.HelpTourPaletteBody},
	{locales.HelpTourKeyboard, locales.HelpTourKeyboardBody},
	{locales.HelpTourLibrary, locales.HelpTourLibraryBody},
}

// tourView is the first-run tour, a card with four steps, which is not shown again once it is closed.
func (a *app) tourView(c *ui.Context) {
	if a.settings.TourDone || a.tourOff || a.modal() || a.settingsFile == "" && a.settingsStore == nil { // a library with no settings of its own is a test's, or a scratch one
		return
	}
	cl := ryugo.From(c).Colors
	step := min(a.tourAt, len(tourSteps)-1)
	ui.Overlay(c, func() {
		card := ui.Column(c.Key("tour")).Absolute()
		if phone { // as wide as the screen, over the bottom
			card.Left(12).Right(12).Bottom(20)
		} else {
			card.Right(20).Bottom(46).Width(340)
		}
		card.Padding(16, 18).Gap(8).Radius(rad(14)).Background(cl.Card).Border(1, cl.Hairline).Children(func() {
			ui.Text(c, locales.HelpTourStep(step+1, len(tourSteps))).FontSize(11.5).TextColor(cl.Muted)
			ui.Text(c, tourSteps[step].title()).FontSize(16).FontWeight(700).TextColor(cl.Text)
			ui.Text(c, tourSteps[step].body()).FontSize(13).TextColor(cl.Text2).Wrap()
			ui.Row(c).Gap(8).Justify(ui.End).Padding(6, 0, 0, 0).Children(func() {
				if ryugo.Button(c.Key("tour-skip"), locales.HelpTourDontShow(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.TourDone = true
					a.saveSettings()
				}
				label := locales.HelpTourNext()
				if step == len(tourSteps)-1 {
					label = locales.HelpTourDone()
				}
				if ryugo.Button(c.Key("tour-next"), label, ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
					if step == len(tourSteps)-1 {
						a.settings.TourDone = true
						a.saveSettings()
					} else {
						a.tourAt++
					}
				}
			})
		})
	})
}
