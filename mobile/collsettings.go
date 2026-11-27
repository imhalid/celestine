package main

import (
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// A collection's settings are a screen of their own under the collection's (#637): its name, and the journal's way of
// titling its days, as the desktop's settings of a collection have them (collsettings.go of the desktop). Saved, the
// journal's days are titled again by the new way.

// journalFormats are the ways of titling the days to choose, the desktop's.
var journalFormats = []string{"YYYY-MM-DD", "DD.MM.YYYY", "D MMMM YYYY", "D MMMM YYYY, dddd"}

// collSettings are the settings being changed, before they are saved.
type collSettings struct {
	dir    string // the collection's folder; "" for none
	name   string
	format string
}

// openCollSettings shows collection k's settings.
func (a *app) openCollSettings(k *library.Collection) {
	a.cs = settingsOf(k)
	a.closeAll()
	a.router.Transition = ui.TransitionAuto
	a.router.Push("/c/" + url.PathEscape(k.Dir) + "/settings")
}

// settingsOf are collection k's settings as they are.
func settingsOf(k *library.Collection) collSettings {
	format := k.JournalFormat
	if strings.TrimSpace(format) == "" {
		format = journalFormats[0]
	}
	return collSettings{dir: k.Dir, name: k.Name, format: format}
}

// saveCollSettings writes the settings to collection k, and titles the journal's days again by a new way.
func (a *app) saveCollSettings(k *library.Collection) {
	name := strings.TrimSpace(a.cs.name)
	if name != "" && name != k.Name {
		if err := k.Rename(name); err != nil {
			a.say(locales.PageSaveFailed(err.Error()))
			return
		}
	}
	retitle := k.Journal && a.cs.format != settingsOf(k).format
	if retitle {
		k.JournalFormat = a.cs.format
	}
	if err := k.Save(); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	if retitle {
		for _, p := range k.All() {
			if t := formatDay(k.JournalFormat, dayOf(p)); t != p.Title {
				locked := p.Locked // a past day keeps its words; its title follows the journal's way
				p.Locked = ""
				p.SetTitle(t)
				p.Locked = locked
			}
		}
	}
	a.syncSoon()
	a.router.Back()
}

func (a *app) collSettingsScreen(c *ui.Context, k *library.Collection) {
	if a.cs.dir != k.Dir { // come to by the router's history: as the collection is
		a.cs = settingsOf(k)
	}
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, true, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6, side, 140, side).Gap(gap).Children(func() {
				ui.Text(c, locales.CollSettings()).FontSize(34).FontWeight(800).LineHeight(1.1).Wrap().Padding(0, 0, 8, 0).
					Role(ui.RoleHeading).Level(1)
				label(c, locales.CollName())
				field(c, "name", func() ui.Element {
					return ui.TextInput(c.Key("name"), &a.cs.name).Label(locales.CollName()).ReturnKey(ui.ReturnDone)
				})
				if k.Journal {
					label(c, locales.CollDayTitles())
					now := time.Now()
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						for _, f := range journalFormats {
							a.formatRow(c, f, formatDay(f, now))
						}
					})
				}
				b := ui.Row(c.Key("save")).FillWidth().Height(58).Radius(29).Justify(ui.Center).AlignItems(ui.Center).
					Margin(14, 0, 0, 0).Background(pal.accent).Role(ui.RoleButton).Label(locales.CollSave())
				pressable(b)
				b.Children(func() { ui.Text(c, locales.CollSave()).FontSize(17).FontWeight(700).TextColor(pal.accentText) })
				if b.Clicked() {
					a.saveCollSettings(k)
				}
			})
		})
	})
}

// formatRow is a way of titling the days, shown as today would be titled, chosen with a tap.
func (a *app) formatRow(c *ui.Context, f, today string) {
	on := a.cs.format == f
	row := pressable(ui.Row(c.Key("fmt:"+f)).FillWidth().Height(58).Padding(0, 18).Gap(12).AlignItems(ui.Center).
		Radius(20).Background(pal.block).Role(ui.RoleRadio).Label(today).Checked(on))
	if on {
		row.Border(2, pal.accent)
	}
	row.Children(func() {
		ui.Text(c, today).FontSize(16).FontWeight(600).Grow(1)
		if on {
			ui.Icon(c, iconCheck).Size(20, 20).TextColor(pal.accent)
		}
	})
	if row.Clicked() {
		a.cs.format = f
	}
}
