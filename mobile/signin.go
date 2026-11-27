package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/mobile/locales"
)

// The sign-in screen: the email and the password of the account the computer uses, and a button that signs in. The
// password goes nowhere: the keys are made from it here (internal/account), and only their proof leaves the phone.

func (a *app) signInScreen(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, true, nil)
		ui.Scroll(c).Grow(1).Basis(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6, side, 40, side).Gap(gap).Children(func() {
				ui.Box(c).Size(56, 56).Radius(18).Background(pal.accent.Alpha(0.15)).AlignItems(ui.Center).Justify(ui.Center).
					Children(func() { ui.Icon(c, iconCloud).Size(28, 28).TextColor(pal.accent) })
				ui.Text(c, locales.SyncSignIn()).FontSize(38).FontWeight(800).Padding(6, 0, 0, 0).Role(ui.RoleHeading).Level(1)
				ui.Text(c, locales.SyncAbout()).FontSize(16).LineHeight(1.45).TextColor(pal.muted).Wrap().Padding(0, 0, 10, 0)
				field(c, "email", func() ui.Element {
					return ui.TextInput(c.Key("email"), &a.sy.email).Placeholder(locales.SyncEmail()).Label(locales.SyncEmail()).
						Keyboard(ui.KeyboardEmail).Capitalize(ui.CapitalizeNone).Autocorrect(false).Autofill(ui.AutofillEmail).
						ReturnKey(ui.ReturnNext)
				})
				pw := field(c, "password", func() ui.Element {
					return ui.TextInput(c.Key("password"), &a.sy.pass).Placeholder(locales.SyncPassword()).Label(locales.SyncPassword()).
						Password().Autofill(ui.AutofillPassword).ReturnKey(ui.ReturnGo)
				})
				if pw.Submitted() {
					a.signIn()
				}
				if a.sy.signErr != "" {
					ui.Text(c, a.sy.signErr).FontSize(15).FontWeight(600).TextColor(pal.danger).Wrap().Padding(2, 4)
				}
				label := locales.SyncSignIn()
				if a.sy.signing {
					label = locales.SyncSigningIn()
				}
				b := ui.Row(c.Key("go")).FillWidth().Height(60).Radius(30).Justify(ui.Center).AlignItems(ui.Center).Margin(8, 0, 0, 0).
					Gradient(pal.accent, pal.accent.Mix(ui.Hex("#8B5CF6"), 0.45), 135).Role(ui.RoleButton).Label(label)
				shadow(pressable(b))
				b.Children(func() { ui.Text(c, label).FontSize(17).FontWeight(700).TextColor(pal.accentText) })
				if b.Clicked() {
					a.signIn()
				}
				ui.Text(c, locales.SyncSignInNote()).FontSize(14).LineHeight(1.45).TextColor(pal.faint).Wrap().Padding(6, 4)
			})
		})
	})
}

// field is a text field of a form, in a block of its own, as input makes it.
func field(c *ui.Context, key string, input func() ui.Element) ui.Element {
	return fieldOn(c, key, pal.block, input)
}

// fieldOn is a text field in a block of color bg, for a form on another block.
func fieldOn(c *ui.Context, key string, bg ui.Color, input func() ui.Element) ui.Element {
	var in ui.Element
	ui.Row(c.Key("f:"+key)).FillWidth().Height(56).Padding(0, 18).AlignItems(ui.Center).Radius(18).Background(bg).
		Children(func() {
			in = input().FontSize(17).Grow(1).Background(bg).Border(0, bg)
		})
	return in
}
