package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Settings screen: the look (as the system is, light or dark) as three blocks to tap, sync, the library's size
// and the version.

func (a *app) settingsScreen(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		a.screenScroll(c).Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				ui.Text(c, locales.TabSettings()).FontSize(38).FontWeight(800).Padding(6, 0, 6, 0).Role(ui.RoleHeading).Level(1)
				label(c, locales.SettingsAppearance())
				ui.Row(c).FillWidth().Gap(10).Children(func() {
					a.lookTile(c, "", locales.SettingsSystem(), iconGear)
					a.lookTile(c, "light", locales.SettingsLight(), iconSun)
					a.lookTile(c, "dark", locales.SettingsDark(), iconMoon)
				})
				label(c, locales.SettingsSync())
				a.syncBlock(c)
				label(c, locales.SettingsLibrary())
				ui.Row(c).FillWidth().Gap(10).Children(func() {
					stat(c, strconv.Itoa(len(a.allPages())), locales.SettingsPages())
					stat(c, strconv.Itoa(len(a.lib.Collections)), locales.LibraryCollections())
				})
				label(c, locales.SettingsAbout())
				ui.Row(c).FillWidth().Padding(18).Radius(smallR + 4).Background(pal.block).Children(func() {
					ui.Text(c, locales.SettingsVersion()).FontSize(16).Grow(1)
					ui.Text(c, version).FontSize(16).TextColor(pal.muted)
				})
			})
		})
	})
}

// lookTile is one of the looks to choose: a block with its glyph and its name, lit when it is the one chosen.
func (a *app) lookTile(c *ui.Context, id, name string, glyph *ui.SVG) {
	on := a.settings.Theme == id
	bg, fg := pal.block, pal.text
	if on {
		bg, fg = pal.accent, pal.accentText
	}
	b := block(c, "look:"+id, bg, smallR+4).Grow(1).Basis(0).Height(96).Padding(16).Justify(ui.SpaceBetween).
		Role(ui.RoleRadio).Label(name).Checked(on)
	b.Children(func() {
		ui.Icon(c, glyph).Size(24, 24).TextColor(fg)
		ui.Text(c, name).FontSize(15).FontWeight(650).TextColor(fg)
	})
	if b.Clicked() && !on {
		a.settings.Theme = id
		a.saveSettings()
		a.applyTheme()
	}
}

// stat is a number of the library and what it counts, in a block.
func stat(c *ui.Context, n, what string) {
	ui.Column(c.Key(what)).Grow(1).Basis(0).Padding(18).Gap(2).Radius(smallR + 4).Background(pal.block).Children(func() {
		ui.Text(c, n).FontSize(32).FontWeight(800)
		ui.Text(c, what).FontSize(14).FontWeight(600).TextColor(pal.muted)
	})
}

// syncBlock is the account and the sync: signed out, what sync is and a way to sign in; signed in, the account, how
// the sync stands, and buttons to sync now and to sign out.
func (a *app) syncBlock(c *ui.Context) {
	ui.Column(c.Key("sync")).FillWidth().Padding(18).Gap(14).Radius(smallR + 4).Background(pal.block).Children(func() {
		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(40, 40).Radius(14).Background(pal.accent.Alpha(0.15)).AlignItems(ui.Center).Justify(ui.Center).
				Shrink(0).Children(func() { ui.Icon(c, iconCloud).Size(22, 22).TextColor(pal.accent) })
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				if !a.signedIn() {
					ui.Text(c, locales.SyncAbout()).FontSize(15).LineHeight(1.4).TextColor(pal.muted).Wrap()
					return
				}
				ui.Text(c, a.settings.AccountUser).FontSize(16).FontWeight(650).MaxLines(1)
				color := pal.muted
				if a.sy.err != "" {
					color = pal.danger
				}
				ui.Text(c, a.syncLine()).FontSize(14).TextColor(color).Wrap()
			})
		})
		ui.Row(c).FillWidth().Gap(10).Children(func() {
			if !a.signedIn() {
				if pill(c, "signin", locales.SyncSignIn(), true) {
					a.sy.signErr = ""
					a.router.Push(pathSignIn)
				}
				return
			}
			if pill(c, "now", locales.SyncSyncNow(), true) {
				a.syncNow()
			}
			if pill(c, "out", locales.SyncSignOut(), false) {
				a.signOut()
			}
		})
	})
}

// pill is a button of a block, filled with the accent when it is the one to press.
func pill(c *ui.Context, key, label string, main bool) bool {
	bg, fg := pal.raised, pal.text
	if main {
		bg, fg = pal.accent, pal.accentText
	}
	b := ui.Row(c.Key(key)).Height(46).Padding(0, 20).Radius(23).AlignItems(ui.Center).Background(bg).Role(ui.RoleButton).Label(label)
	pressable(b)
	b.Children(func() { ui.Text(c, label).FontSize(15).FontWeight(700).TextColor(fg) })
	return b.Clicked()
}
