package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/mobile/locales"
)

// Sign-In sheet for desktop E2E sync. Keys are derived locally (internal/account); password never leaves the device.

func (a *app) signInSheet(c *ui.Context) {
	open := a.sheet == sheetSignIn
	ui.Overlay(c, func() {
		top, _, bottom, _ := c.SafeArea()
		_, winH := c.Size()
		maxH := max(300, min(winH*0.8, winH-top-bottom-24))

		sheet := ui.Column(c.Key("signin-sheet")).Absolute().Left(0).Right(0).Bottom(bottom).
			Height(maxH).Clip().Radius(sheetR, sheetR, 0, 0).
			Background(pal.sheet).Border(1, pal.line).Padding(10, 0, 16, 0)
		k := sheet.AnimateTo("open", b2f(open), sheetAnim())
		if k < 0.01 && !open {
			sheet.Hidden(true)
			return
		}
		sheet.Translate(0, (1-k)*520)
		shadow(sheet)

		sheet.Children(func() {
			sheetHandle(c)
			ui.Row(c).FillWidth().Padding(2, sheetPad, 12, sheetPad).AlignItems(ui.Center).Gap(10).Shrink(0).Children(func() {
				back := pressable(ui.Box(c.Key("si-back")).Size(34, 34).Radius(17).Background(pal.surface).
					AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(locales.CommonBack()))
				back.Children(func() { ui.Icon(c, iconBack).Size(17, 17).TextColor(pal.text) })
				if back.Clicked() {
					a.sheet = sheetSettings
				}
				ui.Text(c, locales.SyncSignIn()).FontSize(22).FontWeight(800).TextColor(pal.text).Grow(1)
				if sheetCloseButton(c, "si-close") {
					a.sheet = sheetNone
				}
			})

			ui.Box(c).FillWidth().Height(1).Background(pal.line.Alpha(0.7)).Shrink(0)

			ui.Scroll(c.Key("signin-scroll")).Grow(1).Basis(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(16, sheetPad, 20, sheetPad).Gap(12).Children(func() {
					ui.Text(c, locales.SyncAbout()).FontSize(14).LineHeight(1.45).TextColor(pal.muted).Wrap()

					fieldOn(c, "si-email", pal.surface, func() ui.Element {
						return ui.TextInput(c.Key("email"), &a.sy.email).
							Placeholder(locales.SyncEmail()).Label(locales.SyncEmail()).
							Keyboard(ui.KeyboardEmail).Capitalize(ui.CapitalizeNone).Autocorrect(false).
							Autofill(ui.AutofillEmail).ReturnKey(ui.ReturnNext)
					})
					pw := fieldOn(c, "si-pass", pal.surface, func() ui.Element {
						return ui.TextInput(c.Key("password"), &a.sy.pass).
							Placeholder(locales.SyncPassword()).Label(locales.SyncPassword()).
							Password().Autofill(ui.AutofillPassword).ReturnKey(ui.ReturnGo)
					})
					if pw.Submitted() {
						a.signIn()
					}
					if a.sy.signErr != "" {
						ui.Text(c, a.sy.signErr).FontSize(14).FontWeight(600).TextColor(pal.danger).Wrap().Padding(2, 4)
					}

					ctaLabel := locales.SyncSignIn()
					if a.sy.signing {
						ctaLabel = locales.SyncSigningIn()
					}
					goBtn := pressable(ui.Row(c.Key("si-go")).FillWidth().Height(52).Radius(16).
						Margin(4, 0, 0, 0).Background(pal.accent).Justify(ui.Center).AlignItems(ui.Center).
						Role(ui.RoleButton).Label(ctaLabel))
					goBtn.Children(func() {
						ui.Text(c, ctaLabel).FontSize(16).FontWeight(700).TextColor(pal.accentText)
					})
					if goBtn.Clicked() {
						a.signIn()
					}

					ui.Text(c, locales.SyncSignInNote()).FontSize(13).LineHeight(1.45).TextColor(pal.faint).Wrap().Padding(4, 2)
				})
			})
		})
	})
}

func fieldOn(c *ui.Context, key string, bg ui.Color, input func() ui.Element) ui.Element {
	var in ui.Element
	ui.Row(c.Key("f:"+key)).FillWidth().Height(50).Padding(0, 16).AlignItems(ui.Center).Radius(14).Background(bg).
		Children(func() {
			in = input().FontSize(16).Grow(1).Background(bg).Border(0, bg)
		})
	return in
}
