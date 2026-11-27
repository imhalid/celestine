package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// A collection held (its card, or its name on its screen) rises out of its place into a preview over the screen, the
// card as the screen shows it, a little larger, its place left empty, while its sheet comes up under it (#638, #640,
// #641): its colors and its icons, chosen with a tap and kept at once, as the
// desktop keeps them (collicons.go of the desktop), the preview changing with them; and its settings, its name and,
// for the journal, its way of titling its days, the days titled again by a new way (#637). The name is kept as the
// sheet closes, or as it is submitted.

// journalFormats are the ways of titling the days to choose, the desktop's.
var journalFormats = []string{"YYYY-MM-DD", "DD.MM.YYYY", "D MMMM YYYY", "D MMMM YYYY, dddd"}

// collSettings is the collection whose sheet is open, and what is being changed in it.
type collSettings struct {
	dir  string  // the collection's folder; "" for none
	from ui.Rect // where it was held, in the window, that its preview rises from
	name string  // its name as typed
}

// holdCollection opens the sheet of collection k, held where from is.
func (a *app) holdCollection(k *library.Collection, from ui.Rect) {
	if k.Dynamic() {
		return
	}
	a.cs = collSettings{dir: k.Dir, from: from, name: k.Name}
	a.collSheet = k.Dir
}

// keepName gives the collection whose sheet was open the name typed in it.
func (a *app) keepName() {
	k := a.collAt(a.cs.dir)
	name := strings.TrimSpace(a.cs.name)
	if k == nil || name == "" || name == k.Name {
		return
	}
	if err := k.Rename(name); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	a.syncSoon()
}

// setDayFormat titles the journal's days by format f, the past days too: their words stay locked, their title
// follows the journal's way.
func (a *app) setDayFormat(k *library.Collection, f string) {
	k.JournalFormat = f
	if err := k.Save(); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	for _, p := range k.All() {
		if t := formatDay(f, dayOf(p)); t != p.Title {
			locked := p.Locked
			p.Locked = ""
			p.SetTitle(t)
			p.Locked = locked
		}
	}
	a.syncSoon()
}

// dayFormatOf is the journal's way of titling its days, the desktop's first when it has none.
func dayFormatOf(k *library.Collection) string {
	if strings.TrimSpace(k.JournalFormat) == "" {
		return journalFormats[0]
	}
	return k.JournalFormat
}

// collSheetView is the preview of the collection held and its sheet, while they show.
func (a *app) collSheetView(c *ui.Context) {
	if a.collSheet == "" && a.cs.name != "" { // closed: its name is kept
		a.keepName()
		a.cs.name = ""
	}
	k := a.collAt(a.cs.dir)
	open := a.collSheet != "" && k != nil
	ui.Overlay(c, func() {
		top, _, bottom, _ := c.SafeArea()
		winW, _ := c.Size()
		anchor := ui.Box(c.Key("coll-held")).Absolute().Top(0).Left(0).Size(0, 0)
		kk := anchor.AnimateTo("open", b2f(open), ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond, Bounce: 0.12}})
		a.csLifted = kk >= 0.01 // its place on the screen stays empty until it is back in it
		if !a.csLifted && !open || k == nil {
			return
		}

		// The preview is the card as the screen shows it, risen from its place to the top, a little larger.
		from := a.cs.from
		if from.W == 0 {
			from = ui.Rect{X: side, Y: top + 120, W: (winW - 2*side - gap) / 2, H: 132}
		}
		const grow = 1.18
		to := ui.Rect{X: (winW - from.W*grow) / 2, Y: top + 40, W: from.W * grow, H: from.H * grow}
		at := func(f, t float32) float32 { return f + (t-f)*kk }
		s := at(1, grow)
		t := tonesOf(k)
		pv := ui.Column(c.Key("coll-preview")).Absolute().Left(at(from.X, to.X)).Top(at(from.Y, to.Y)).
			Width(at(from.W, to.W)).Height(at(from.H, to.H)).Radius((smallR + 4) * s).Padding(16 * s).Justify(ui.SpaceBetween).
			Background(t.card).Role(ui.RoleButton).Label(k.Name)
		shadow(pv)
		pv.Children(func() { collCard(c, k, t, s) })
		if pv.Clicked() && open {
			a.openCollection(k)
		}

		// The sheet, from the bottom.
		sheet := ui.Column(c.Key("coll-sheet")).Absolute().Left(10).Right(10).Bottom(10+bottom).Radius(30).
			Background(pal.block).Padding(10, 16, 16, 16).Gap(16)
		sheet.Translate(0, (1-kk)*460)
		shadow(sheet)
		sheet.Children(func() {
			ui.Box(c).Size(40, 5).Radius(3).Background(pal.line).AlignSelf(ui.Center)
			a.colorRow(c, k)
			a.iconGrid(c, k)
			ui.Text(c, locales.CollSettings()).FontSize(14).FontWeight(650).TextColor(pal.muted).Padding(2, 4, 0, 4)
			name := fieldOn(c, "coll-name", pal.raised, func() ui.Element {
				return ui.TextInput(c.Key("coll-name"), &a.cs.name).Label(locales.CollName()).Placeholder(locales.CollName()).
					ReturnKey(ui.ReturnDone)
			})
			if name.Submitted() {
				a.keepName()
			}
			if k.Journal {
				a.formatChips(c, k, t)
			}
		})
	})
}

// formatChips are the ways of titling the journal's days, each shown as today would be titled, the one it has on its
// color.
func (a *app) formatChips(c *ui.Context, k *library.Collection, t tones) {
	now := time.Now()
	ui.Row(c.Key("formats")).FillWidth().Wrap().Gap(8).Children(func() {
		for _, f := range journalFormats {
			on := dayFormatOf(k) == f
			b := pressable(ui.Row(c.Key("fmt:"+f)).Height(40).Padding(0, 14).Radius(20).AlignItems(ui.Center).
				Background(pal.raised).Role(ui.RoleRadio).Label(formatDay(f, now)).Checked(on))
			color := pal.text
			if on {
				b.Background(t.card)
				color = t.ink
			}
			b.Children(func() { ui.Text(c, formatDay(f, now)).FontSize(14).FontWeight(600).TextColor(color) })
			if b.Clicked() && !on {
				a.setDayFormat(k, f)
			}
		}
	})
}

// colorRow is the colors collection k can have: none (one by its name), then c1 to c8, the one it has ringed.
func (a *app) colorRow(c *ui.Context, k *library.Collection) {
	ui.Row(c.Key("colors")).FillWidth().Justify(ui.SpaceBetween).Children(func() {
		for _, name := range append([]string{""}, collColors...) {
			was := k.Color
			k.Color = name
			t := tonesOf(k)
			k.Color = was
			on := k.Color == name
			sw := pressable(ui.Box(c.Key("c:"+name)).Size(34, 34).Radius(17).AlignItems(ui.Center).Justify(ui.Center).
				Role(ui.RoleRadio).Label(locales.CollColor()).Checked(on))
			if on {
				sw.Border(2.5, t.card)
			}
			sw.Children(func() { ui.Box(c).Size(24, 24).Radius(12).Background(t.card) })
			if sw.Clicked() && !on {
				a.setCollLook(k, name, k.Icon)
			}
		}
	})
}

// iconGrid is the icons collection k can have, six to a row, the one it has on its color.
func (a *app) iconGrid(c *ui.Context, k *library.Collection) {
	t := tonesOf(k)
	ui.Column(c.Key("icons")).FillWidth().Gap(4).Children(func() {
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
					color := pal.muted
					if on {
						b.Background(t.card)
						color = t.ink
					}
					b.Children(func() { ui.Icon(c, collGlyphOf(ic.name, k.Journal)).Size(22, 22).TextColor(color) })
					if b.Clicked() && !on {
						a.setCollLook(k, k.Color, ic.name)
					}
				}
			})
		}
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
