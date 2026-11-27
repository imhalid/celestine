package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/mobile/locales"
)

// Minimal discreet bottom sheet tucked behind Today's corner cloud icon: Appearance, Desktop Sync, Copy today's text,
// and Version.

func (a *app) settingsSheet(c *ui.Context) {
	open := a.sheet == sheetSettings
	ui.Overlay(c, func() {
		top, _, bottom, _ := c.SafeArea()
		_, winH := c.Size()
		maxH := max(260, min(winH*0.72, winH-top-bottom-28))

		sheet := ui.Column(c.Key("settings-sheet")).Absolute().Left(0).Right(0).Bottom(bottom).
			Height(maxH).Clip().Radius(sheetR, sheetR, 0, 0).
			Background(pal.sheet).Border(1, pal.line).Padding(10, 0, 16, 0)
		k := sheet.AnimateTo("open", b2f(open), sheetAnim())
		if k < 0.01 && !open {
			sheet.Hidden(true)
			return
		}
		sheet.Translate(0, (1-k)*480)
		shadow(sheet)

		sheet.Children(func() {
			sheetHandle(c)
			ui.Row(c).FillWidth().Padding(2, sheetPad, 12, sheetPad).AlignItems(ui.Center).Gap(10).Shrink(0).Children(func() {
				ui.Text(c, locales.SettingsTitle()).FontSize(22).FontWeight(800).TextColor(pal.text).Grow(1)
				if sheetCloseButton(c, "set-close") {
					a.sheet = sheetNone
				}
			})

			ui.Box(c).FillWidth().Height(1).Background(pal.line.Alpha(0.7)).Shrink(0)

			ui.Scroll(c.Key("settings-scroll")).Grow(1).Basis(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(16, sheetPad, 18, sheetPad).Gap(16).Children(func() {
					// Appearance segmented picker.
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						sectionLabel(c, locales.SettingsAppearance())
						ui.Row(c).FillWidth().Padding(4).Radius(16).Background(pal.surface).Gap(4).Children(func() {
							a.lookSegment(c, "", locales.SettingsSystem(), iconGear)
							a.lookSegment(c, "light", locales.SettingsLight(), iconSun)
							a.lookSegment(c, "dark", locales.SettingsDark(), iconMoon)
						})
					})

					// Desktop E2E Sync.
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						sectionLabel(c, locales.SettingsSync())
						a.syncCard(c)
					})

					// Copy today's text.
					copyRow := pressable(ui.Row(c.Key("copy-today")).FillWidth().Height(48).Padding(0, 16).Radius(16).
						Background(pal.surface).Gap(12).AlignItems(ui.Center).
						Role(ui.RoleButton).Label(locales.PageCopy()))
					copyRow.Children(func() {
						ui.Icon(c, iconCopy).Size(18, 18).TextColor(pal.text)
						ui.Text(c, locales.PageCopy()).FontSize(15).FontWeight(600).TextColor(pal.text).Grow(1)
					})
					if copyRow.Clicked() {
						text := ""
						if a.ed.doc != nil {
							text = a.ed.doc.Text().PlainText()
						}
						c.WriteClipboard(text)
						a.sheet = sheetNone
						a.say(locales.PageCopied())
					}

					ui.Row(c).FillWidth().Padding(12, 16).Radius(16).Background(pal.surface).AlignItems(ui.Center).Children(func() {
						ui.Text(c, locales.SettingsVersion()).FontSize(13).FontWeight(600).TextColor(pal.muted).Grow(1)
						ui.Text(c, version).FontSize(13).FontWeight(650).TextColor(pal.text)
					})
				})
			})
		})
	})
}

func sheetHandle(c *ui.Context) {
	ui.Box(c).Size(38, 4).Radius(2).Background(pal.line).AlignSelf(ui.Center).Margin(2, 0, 8, 0)
}

func sheetCloseButton(c *ui.Context, key string) bool {
	b := pressable(ui.Box(c.Key(key)).Size(34, 34).Radius(17).Background(pal.surface).
		AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(locales.CommonClose()))
	b.Children(func() { ui.Icon(c, iconClose).Size(16, 16).TextColor(pal.muted) })
	return b.Clicked()
}

func sectionLabel(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(12).FontWeight(700).TextColor(pal.faint).Padding(0, 4)
}

func (a *app) lookSegment(c *ui.Context, id, label string, glyph *ui.SVG) {
	on := a.settings.Theme == id
	bg, fg := ui.Color{}, pal.muted
	if on {
		bg, fg = pal.sheet, pal.text
	}
	b := pressable(ui.Row(c.Key("look:" + id)).Grow(1).Basis(0).Height(40).Radius(12).
		Background(bg).Gap(6).AlignItems(ui.Center).Justify(ui.Center).
		Role(ui.RoleRadio).Label(label).Checked(on))
	b.Children(func() {
		ui.Icon(c, glyph).Size(16, 16).TextColor(fg)
		ui.Text(c, label).FontSize(13).FontWeight(650).TextColor(fg)
	})
	if b.Clicked() && !on {
		mygo.Haptics.Selection()
		a.settings.Theme = id
		a.saveSettings()
		a.applyTheme()
	}
}

func (a *app) syncCard(c *ui.Context) {
	ui.Column(c.Key("sync-card")).FillWidth().Padding(16).Radius(18).Background(pal.surface).Gap(14).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(38, 38).Radius(12).Background(pal.accentSoft).AlignItems(ui.Center).Justify(ui.Center).
				Shrink(0).Children(func() { ui.Icon(c, iconCloud).Size(20, 20).TextColor(pal.accent) })
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				if !a.signedIn() {
					ui.Text(c, locales.SyncAbout()).FontSize(14).LineHeight(1.4).TextColor(pal.muted).Wrap()
					return
				}
				ui.Text(c, a.settings.AccountUser).FontSize(15).FontWeight(700).TextColor(pal.text).MaxLines(1)
				statusColor := pal.muted
				if a.sy.err != "" {
					statusColor = pal.danger
				}
				ui.Text(c, a.syncLine()).FontSize(13).TextColor(statusColor).Wrap()
			})
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			if !a.signedIn() {
				if pillButton(c, "sync-signin", locales.SyncSignIn(), true) {
					a.sy.signErr = ""
					a.sheet = sheetSignIn
				}
				return
			}
			if pillButton(c, "sync-now", locales.SyncSyncNow(), true) {
				a.syncNow()
			}
			if pillButton(c, "sync-out", locales.SyncSignOut(), false) {
				a.signOut()
			}
		})
	})
}

func pillButton(c *ui.Context, key, label string, primary bool) bool {
	bg, fg := pal.sheet, pal.text
	if primary {
		bg, fg = pal.accent, pal.accentText
	}
	b := pressable(ui.Row(c.Key(key)).Height(42).Padding(0, 18).Radius(21).
		AlignItems(ui.Center).Background(bg).Role(ui.RoleButton).Label(label))
	if !primary {
		b.Border(1, pal.line)
	}
	b.Children(func() { ui.Text(c, label).FontSize(14).FontWeight(700).TextColor(fg) })
	return b.Clicked()
}
