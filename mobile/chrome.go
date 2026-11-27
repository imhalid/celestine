package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/mobile/locales"
)

// The app's chrome is its own: no bar of tabs and no button for the menu. A screen pulled down past its top opens the
// menu, which fills the screen over the screen dimmed, its places coming in one after another, and closes at a tap
// anywhere else; a round + at the bottom right opens what can be made, rising over it in a column at the right, the +
// turning to a ×. A page's ··· and a collection's long press open a sheet from the bottom.

// motionIn is how what opens comes in, and goes: quickly, easing out, with no overshoot.
func motionIn(delay time.Duration) ui.Animation {
	return ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseOut, Delay: delay}
}

// roundButton is a round button of the chrome, glyph in it, its label for assistive technology.
func roundButton(c *ui.Context, key string, glyph *ui.SVG, label string) bool {
	b := ui.Box(c.Key(key)).Size(46, 46).Radius(23).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Background(pal.block).Border(1, pal.line).Role(ui.RoleButton).Label(label)
	pressable(b)
	b.Children(func() { ui.Icon(c, glyph).Size(22, 22).TextColor(pal.text) })
	return b.Clicked()
}

// topBar is the row at the top of a screen under another: back, and what goes at its right end. A screen of the
// menu's has none, only room above its title.
func (a *app) topBar(c *ui.Context, back bool, trailing func()) {
	if !back && trailing == nil {
		ui.Box(c.Key("top")).Height(22).Shrink(0)
		return
	}
	ui.Row(c.Key("top")).FillWidth().Height(66).Padding(0, side).Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		if back && roundButton(c, "back", iconBack, locales.CommonBack()) {
			a.router.Back()
		}
		ui.Box(c).Grow(1)
		if trailing != nil {
			trailing()
		}
	})
}

// pullMenu has scroll, a screen's, open the menu when a finger pulls it down past its top.
func (a *app) pullMenu(scroll ui.Element) ui.Element {
	if scroll.PullToRefresh(&a.pull) {
		a.pull.Refreshing = false
		a.menuOpen, a.plusOpen = true, false
	}
	return scroll
}

// screenScroll is the scroll of a screen of the menu's, which a pull down opens the menu from.
func (a *app) screenScroll(c *ui.Context) ui.Element {
	return a.pullMenu(ui.Scroll(c.Key("scroll")).Grow(1).Basis(0).FillWidth())
}

// bigTitle is a screen's title, large, under its top bar.
func bigTitle(c *ui.Context, title, sub, path string) {
	ui.Column(c).Padding(6, side, 18, side).Gap(4).Shrink(0).Children(func() {
		ui.Text(c.Key("title"), title).FontSize(38).FontWeight(800).LineHeight(1.05).Wrap().Role(ui.RoleHeading).Level(1).
			AlignSelf(ui.Start).Transition(titleFlight(path))
		if sub != "" {
			ui.Text(c, sub).FontSize(15).TextColor(pal.muted)
		}
	})
}

// plusButton is the round + at the bottom of the screen, and what it opens.
func (a *app) plusButton(c *ui.Context) {
	ui.Overlay(c, func() {
		_, _, bottom, _ := c.SafeArea() // an overlay is laid out over the system's bars: kept above them
		anchor := ui.Column(c.Key("plus-col")).Absolute().Right(side).Bottom(18 + bottom).AlignItems(ui.End).Gap(10)
		k := anchor.AnimateTo("open", b2f(a.plusOpen), motionIn(0))
		anchor.Children(func() {
			// Built while closed too, hidden, so that each keeps its motion and rises in as the list opens: made
			// anew as it opened, it stood at once where it ends (#643).
			closed := k < 0.01 && !a.plusOpen
			acts := a.plusActions()
			for i, act := range acts {
				a.plusAction(c, i, len(acts), act).Hidden(closed)
			}
			ui.Box(c).Height(6).Hidden(closed)
			b := ui.Box(c.Key("plus")).Size(66, 66).Radius(33).AlignItems(ui.Center).Justify(ui.Center).
				Gradient(pal.accent, pal.accent.Mix(ui.Hex("#8B5CF6"), 0.45), 135).Role(ui.RoleButton)
			if a.plusOpen {
				b.Label(locales.PlusClose())
			} else {
				b.Label(locales.PlusOpen())
			}
			shadow(pressable(b))
			b.Children(func() { ui.Icon(c, iconPlus).Size(28, 28).TextColor(pal.accentText).Rotate(45 * k) })
			if b.Clicked() {
				mygo.Haptics.Light()
				a.plusOpen = !a.plusOpen
				a.menuOpen = false
			}
		})
	})
}

// plusAct is a thing the + makes.
type plusAct struct {
	glyph *ui.SVG
	tint  ui.Color
	label string
	do    func()
}

// plusActions are what the + makes: today's page, a note, a page in each collection of notes, and a search.
func (a *app) plusActions() []plusAct {
	acts := []plusAct{
		{iconCalendar, tints[1], locales.ActionToday(), func() { a.newNote(a.journalColl()) }},
		{iconPage, tints[0], locales.ActionNote(), func() { a.newNote(nil) }},
	}
	first, n := true, 0 // the first collection of notes is New note's; the next two have a line of their own
	for _, k := range a.lib.Collections {
		if k.Journal || k.Dynamic() || n == 2 {
			continue
		}
		if first {
			first = false
			continue
		}
		k := k
		acts = append(acts, plusAct{collGlyph(k), collColor(k), locales.ActionIn(k.Name), func() { a.newNote(k) }})
		n++
	}
	acts = append(acts, plusAct{iconSearch, tints[3], locales.ActionSearch(), func() { a.goTo(pathSearch) }})
	return acts
}

// plusAction is one of the +'s list: a pill that rises and grows in a little after the one under it.
func (a *app) plusAction(c *ui.Context, i, n int, act plusAct) ui.Element {
	row := ui.Row(c.Key(act.label)).Height(56).Padding(0, 22, 0, 8).Gap(12).AlignItems(ui.Center).Radius(28).
		Background(pal.block).Role(ui.RoleButton).Label(act.label)
	k := row.AnimateTo("in", b2f(a.plusOpen), motionIn(time.Duration(n-1-i)*22*time.Millisecond))
	s := 0.92 + 0.08*k
	row.Translate(0, (1-k)*14).Scale(s, s).Opacity(max(0, min(k, 1)))
	shadow(pressable(row))
	row.Children(func() {
		ui.Box(c).Size(40, 40).Radius(20).Background(act.tint.Alpha(0.16)).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, act.glyph).Size(20, 20).TextColor(act.tint) })
		ui.Text(c, act.label).FontSize(16).FontWeight(600)
	})
	if row.Clicked() && a.plusOpen {
		a.plusOpen = false
		act.do()
	}
	return row
}

// menuPlace is a place the menu goes to.
type menuPlace struct {
	path  string
	glyph *ui.SVG
	name  string
}

// menuView is the menu, over the whole screen while it is open: the app's places, large, one after another.
func (a *app) menuView(c *ui.Context) {
	ui.Overlay(c, func() {
		top, _, bottom, _ := c.SafeArea() // an overlay is laid out over the system's bars: kept off them
		col := ui.Column(c.Key("menu")).Absolute().Top(top).Left(0).Right(0).Bottom(bottom)
		k := col.AnimateTo("open", b2f(a.menuOpen), motionIn(0))
		col.Hidden(k < 0.01 && !a.menuOpen) // built while closed too, so that its places come in as it opens (#643)
		if col.Clicked() {                  // a tap anywhere but on a place
			a.menuOpen = false
		}
		col.Children(func() {
			ui.Box(c).Height(66).Shrink(0)
			places := []menuPlace{
				{pathHome, iconSun, locales.NavHome()},
				{pathLibrary, iconBooks, locales.TabLibrary()},
				{pathTasks, iconTasks, locales.NavTasks()},
				{pathSearch, iconSearch, locales.TabSearch()},
				{pathSettings, iconGear, locales.TabSettings()},
			}
			ui.Column(c).Padding(28, side+6).Gap(6).Children(func() {
				for i, pl := range places {
					a.menuItem(c, i, pl)
				}
			})
		})
	})
}

// menuItem is a place of the menu, large, sliding in from the left after the one above it.
func (a *app) menuItem(c *ui.Context, i int, pl menuPlace) {
	here := a.router.Path() == pl.path
	color := pal.text
	if here {
		color = pal.accent
	}
	row := ui.Row(c.Key(pl.path)).FillWidth().Height(64).Gap(16).AlignItems(ui.Center).Radius(20).Padding(0, 6).
		Role(ui.RoleButton).Label(pl.name)
	k := row.AnimateTo("in", b2f(a.menuOpen), motionIn(time.Duration(i)*35*time.Millisecond))
	row.Translate(-(1-k)*18, 0).Opacity(max(0, min(k, 1)))
	pressable(row)
	row.Children(func() {
		ui.Icon(c, pl.glyph).Size(26, 26).TextColor(color)
		name := ui.Text(c.Key("name"), pl.name).FontSize(34).FontWeight(750).TextColor(color)
		if !here { // flown into the title of the screen it opens
			name.Transition(titleFlight(pl.path))
		}
	})
	if row.Clicked() && a.menuOpen {
		a.goTo(pl.path)
	}
}

// moreSheet is a page's sheet of what to do with it, from the bottom.
func (a *app) moreSheet(c *ui.Context) {
	p := a.lib.PageByID(a.ed.id)
	ui.Overlay(c, func() {
		_, _, bottom, _ := c.SafeArea()
		sheet := ui.Column(c.Key("more")).Absolute().Left(10).Right(10).Bottom(10 + bottom).Radius(30).Background(pal.block).
			Padding(10).Gap(4)
		k := sheet.AnimateTo("open", b2f(a.moreOpen && p != nil), motionIn(0))
		if k < 0.01 && !a.moreOpen {
			sheet.Hidden(true)
			return
		}
		sheet.Translate(0, (1-k)*260)
		shadow(sheet)
		sheet.Children(func() {
			ui.Box(c).Size(40, 5).Radius(3).Background(pal.line).AlignSelf(ui.Center).Margin(4, 0, 10, 0)
			if p == nil {
				return
			}
			if sheetRow(c, "copy", iconCopy, locales.PageCopy(), pal.text) {
				a.moreOpen = false
				c.WriteClipboard(a.ed.doc.Text().PlainText())
				a.say(locales.PageCopied())
			}
			if !a.journalDay(p) && p.Locked == "" {
				if sheetRow(c, "trash", iconTrash, locales.PageTrash(), pal.danger) {
					a.moreOpen = false
					a.trash(p)
					a.router.Back()
				}
			}
		})
	})
}

// sheetRow is a row of a sheet: its glyph and its words, in color.
func sheetRow(c *ui.Context, key string, glyph *ui.SVG, label string, color ui.Color) bool {
	row := ui.Row(c.Key(key)).FillWidth().Height(58).Padding(0, 16).Gap(14).AlignItems(ui.Center).Radius(20).
		Role(ui.RoleButton).Label(label)
	if row.Pressed() {
		row.Background(pal.raised)
	}
	row.Children(func() {
		ui.Icon(c, glyph).Size(22, 22).TextColor(color)
		ui.Text(c, label).FontSize(17).FontWeight(550).TextColor(color)
	})
	return row.Clicked()
}
