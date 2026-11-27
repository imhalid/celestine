package main

import (
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// A collection held (its tile, or its name on its screen) opens its sheet (#638): its color and its icon, chosen with a
// tap and kept at once, as the desktop keeps them (collicons.go of the desktop), and a way to its settings, a screen
// of their own: its name, and the journal's way of titling its days (#637). Saved, the journal's days are titled again
// by the new way.

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

// collSheetView is the sheet of the collection held, from the bottom: its name in its color, its colors, its icons,
// and its settings.
func (a *app) collSheetView(c *ui.Context) {
	k := a.collAt(a.collSheet)
	ui.Overlay(c, func() {
		_, _, bottom, _ := c.SafeArea()
		sheet := ui.Column(c.Key("coll-sheet")).Absolute().Left(10).Right(10).Bottom(10+bottom).Radius(30).
			Background(pal.block).Padding(10, 16, 14, 16).Gap(14)
		open := a.collSheet != "" && k != nil
		kk := sheet.AnimateTo("open", b2f(open), motionIn(0))
		if kk < 0.01 && !open {
			sheet.Hidden(true)
			return
		}
		sheet.Translate(0, (1-kk)*320)
		shadow(sheet)
		if k == nil {
			return
		}
		t := tonesOf(k)
		sheet.Children(func() {
			ui.Box(c).Size(40, 5).Radius(3).Background(pal.line).AlignSelf(ui.Center)
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(42, 42).Radius(14).Background(t.mark).AlignItems(ui.Center).Justify(ui.Center).
					Children(func() { ui.Icon(c, collGlyph(k)).Size(23, 23).TextColor(t.color) })
				ui.Text(c, k.Name).FontSize(20).FontWeight(750).TextColor(t.ink).MaxLines(1).Grow(1).MinWidth(0)
			})
			// Its colors: none (one by its name), then c1 to c8.
			ui.Row(c.Key("colors")).FillWidth().Justify(ui.SpaceBetween).Children(func() {
				for _, name := range append([]string{""}, collColors...) {
					was := k.Color
					k.Color = name
					col := collColor(k)
					k.Color = was
					on := k.Color == name
					sw := pressable(ui.Box(c.Key("c:"+name)).Size(32, 32).Radius(16).Background(col).Role(ui.RoleRadio).
						Label(locales.CollColor()).Checked(on).AlignItems(ui.Center).Justify(ui.Center))
					sw.Children(func() {
						if on {
							ui.Icon(c, iconCheck).Size(16, 16).TextColor(ui.Hex("#FFFFFF"))
						} else if name == "" {
							ui.Box(c).Size(10, 10).Radius(5).Background(ui.Hex("#FFFFFF").Alpha(0.85))
						}
					})
					if sw.Clicked() && !on {
						a.setCollLook(k, name, k.Icon)
					}
				}
			})
			// Its icons, six to a row.
			ui.Column(c.Key("icons")).FillWidth().Gap(6).Children(func() {
				for i := 0; i < len(collIcons); i += 6 {
					ui.Row(c.Key(i)).FillWidth().Justify(ui.SpaceBetween).Children(func() {
						for j := i; j < i+6; j++ {
							if j >= len(collIcons) {
								ui.Box(c).Size(46, 46)
								continue
							}
							ic := collIcons[j]
							on := k.Icon == ic.name
							b := pressable(ui.Box(c.Key("i:"+ic.name)).Size(46, 46).Radius(15).AlignItems(ui.Center).
								Justify(ui.Center).Role(ui.RoleRadio).Label(locales.CollIcon()).Checked(on))
							if on {
								b.Background(t.mark)
							}
							b.Children(func() {
								color := pal.muted
								if on {
									color = t.color
								}
								ui.Icon(c, collGlyphOf(ic.name, k.Journal)).Size(22, 22).TextColor(color)
							})
							if b.Clicked() && !on {
								a.setCollLook(k, k.Color, ic.name)
							}
						}
					})
				}
			})
			if sheetRow(c, "settings", iconGear, locales.CollSettings(), pal.text) {
				a.openCollSettings(k)
			}
		})
	})
}

// setCollLook gives collection k its color and its icon, and keeps them.
func (a *app) setCollLook(k *library.Collection, color, icon string) {
	k.Color, k.Icon = color, icon
	if err := k.Save(); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	a.syncSoon()
}
