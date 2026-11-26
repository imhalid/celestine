package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The first start after an update says so without a dialog (#392): a small note at the bottom right, the version, a
// few words of what changed, and a link to the changelog on the website. Closed, it does not come again. It comes as
// it is, with no motion: the wave of glass that swept the window first (#417) is gone (#630).

// A helloState is the note of the version just installed: when it was first shown, its words, whether it was closed.
type helloState struct {
	at     time.Time
	notes  string
	closed bool
}

// helloChangelog is the changelog on the website, where the note's link goes.
const helloChangelog = "https://celestine.halid.dev/changelog"

// startHello begins the hello of the version just installed, once: what its first start shows in place of a dialog.
func (a *app) startHello(c *ui.Context) {
	a.hello = helloState{at: c.Now(), notes: a.settings.UpdateNotes}
	if updTrialRestarted { // the trial's saves nothing (updtrial.go)
		updTrialRestarted = false
		return
	}
	a.settings.UpdateSeen = version
	a.saveSettings()
}

// updateHello is the note of the version just installed, at the window's bottom right, until it is closed.
func (a *app) updateHello(c *ui.Context) {
	if a.hello.at.IsZero() || a.hello.closed || a.wel.on {
		return
	}
	cl := ryugo.From(c).Colors
	r := rad(12)
	ui.Overlay(c, func() {
		// as far from the content's bottom edge as from its right: in the frame, the content's card ends frameW in from
		// the window's right and at the status bar (or frameW up, without it); else at the window's right and the bar
		in := float32(16)
		bottom := in
		switch {
		case a.wframe.on && a.settings.HideStatus:
			in -= frameW
			bottom = frameW + in
		case a.wframe.on:
			in -= frameW
			bottom = a.statusH + in
		case !a.settings.HideStatus:
			bottom = a.statusH + in
		}
		card := ui.Column(c.Key("upd-hello")).Absolute().Right(16).Bottom(bottom).Width(helloW).Radius(r).
			// a faint tint and a light border of the theme's accent, as the update's card has: a notice (#398, #400)
			Background(ryugo.Mix(cl.Card, cl.Accent, 0.06)).Border(1, cl.Accent.Alpha(0.28)).Shadow(0, 6, 18, -6, black.Alpha(0.18)).
			Role(ui.RoleDialog).Label(locales.UpdateWhatsNew(shownVersion(version)))
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Start).Padding(12, 10, 12, 12).Children(func() {
				updBadge(c, cl.ColorAccent400, cl.ColorAccent600, iconCheck, "upd-hello") // installed
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, locales.UpdateWhatsNew(shownVersion(version))).FontSize(13.5).FontWeight(680).TextColor(cl.Text).Wrap()
					ui.Text(c, helloSummary(a.hello.notes)).FontSize(12).TextColor(cl.Text2).Wrap()
					l := ui.Text(c.Key("upd-hello-link"), locales.UpdateChangelog()+" →").FontSize(12).FontWeight(600).
						TextColor(cl.Accent).Role(ui.RoleLink).Cursor(ui.CursorPointer).Margin(4, 0, 0, 0)
					if l.Hovered() {
						l.Underline()
					}
					if l.Clicked() {
						c.OpenURL(helloChangelog)
					}
				})
				x := ui.Box(c.Key("upd-hello-close")).Width(22).Height(22).Radius(11).AlignItems(ui.Center).Justify(ui.Center).
					Shrink(0).Role(ui.RoleButton).Label(locales.Close()).Cursor(ui.CursorPointer)
				if x.Hovered() {
					x.Background(cl.Fill)
				}
				x.Children(func() { ui.Icon(c, iconClose).Width(13).Height(13).TextColor(cl.Muted) })
				if x.Clicked() {
					a.hello.closed = true
				}
			})
		})
	})
}

// helloW is the note's width: wider than the update's card, for a few words of what changed.
const helloW = 300

// helloSummary is a few words of what changed: the notes' first lines as plain text, cut at a sentence's length; or,
// with no notes, that there are none.
func helloSummary(notes string) string {
	var lines []string
	for _, l := range strings.Split(notes, "\n") { // each line of the notes on its own: a list's items stay apart
		if l = strings.TrimSpace(plainText(l)); l != "" {
			lines = append(lines, strings.TrimLeft(l, "-*• "))
		}
		if len(lines) == 3 {
			break
		}
	}
	if len(lines) == 0 {
		return locales.UpdateNoNotes()
	}
	s := strings.Join(lines, " · ")
	if rs := []rune(s); len(rs) > 160 {
		s = strings.TrimSpace(string(rs[:157])) + "…"
	}
	return s
}
