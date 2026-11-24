package main

import (
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The windows of an update: "a new version is out" with its notes; "this version is new" after one; and the
// settings' part, with when it last asked.

// updateRestartArgs returns the Relaunch arguments for an update trial restart.
func updateRestartArgs(args []string) []string {
	return append(withoutUpdateRestartedFlag(args), updateRestartedFlag)
}

// restartApp starts the program again and quits this one.
func (a *app) restartApp() {
	a.flush()
	if updTrial { // the trial's restart shows its "what's new", as an update's does (updtrial.go)
		os.Setenv("CELESTINE_UPDATE", "restarted")
		a.relaunch(updateRestartArgs(os.Args[1:])...)
		return
	}
	a.relaunch()
}

// updateView is the dialog of a new version, or of the notes of the one that was installed.
func (a *app) updateView(c *ui.Context) {
	cl := ryugo.From(c).Colors
	if a.updAuto && !a.updBusy.Load() {
		a.updAuto = false
		a.startUpdate()
	}
	if a.whatsNew() && !a.updOpen && !a.modal() {
		if phone {
			a.updNotes = true
			a.updOpen = true
		} else { // on the desktop, a wave over the window and a note at its bottom right instead (updhello.go)
			a.startHello(c)
		}
	}
	if !phone {
		a.updateHello(c)
	}
	if !a.updNotes && !phone {
		a.updateCard(c) // a waiting version has a card of its own at the top right (updatepanel.go)
		return
	}
	if !a.updOpen {
		return
	}
	title, notes := locales.UpdateNewVersion(), ""
	if a.updRel != nil {
		title, notes = locales.UpdateVersionAvailable(shownVersion(a.updRel.Version)), a.updRel.Notes
	}
	if a.updNotes {
		title, notes = locales.UpdateWhatsNew(shownVersion(version)), a.settings.UpdateNotes
	}
	open := a.updOpen
	ryugo.Dialog(c.Key("update"), &open, ryugo.DialogOptions{Title: title, Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			if a.updRel != nil && !a.updNotes {
				ui.Text(c, releasedLine(a.updRel.Date)).FontSize(12.5).TextColor(cl.Muted)
			}
			if strings.TrimSpace(notes) != "" {
				a.scrolled(c, "upd-notes", nil, ui.Scroll(c.Key("upd-notes")).MaxHeight(240), func() {
					s := plainText(notes)
					a.textMenu(ui.Text(c, s).FontSize(13).TextColor(cl.Text2).Wrap().Selectable(), func() string { return s }, nil)
				})
			} else {
				ui.Text(c, locales.UpdateNoNotes()).FontSize(13).TextColor(cl.Muted)
			}
			if a.updErr != "" {
				ui.Text(c, a.updErr).FontSize(12.5).TextColor(cl.Up).Wrap()
			}
			ui.Text(c, locales.UpdateSafety()).FontSize(12).TextColor(cl.Muted).Wrap()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				switch {
				case a.updNotes:
					if ryugo.Button(c.Key("upd-ok"), locales.CommonOk(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						open = false
					}
				case a.updState == updReady:
					if ryugo.Button(c.Key("upd-restart"), locales.UpdateRestart(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						a.restartApp()
					}
					if ryugo.Button(c.Key("upd-later2"), locales.UpdateLater(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						open = false
					}
				case a.updState == updDownloading:
					ui.Text(c, locales.UpdateDownloading()).FontSize(13).TextColor(cl.Muted)
				default:
					if ryugo.Button(c.Key("upd-skip"), locales.UpdateSkip(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() && a.updRel != nil {
						a.settings.UpdateSkip = a.updRel.Version
						a.saveSettings()
						open = false
					}
					if ryugo.Button(c.Key("upd-snooze"), locales.UpdateRemindLater(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.settings.UpdateSnooze = a.clock().Add(72 * time.Hour).Unix()
						a.saveSettings()
						open = false
					}
					if ryugo.Button(c.Key("upd-now"), locales.UpdateUpdateNow(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						a.startUpdate()
					}
				}
			})
		})
	})
	if !open {
		if a.updNotes {
			if updTrialRestarted { // the trial's "what's new" is shown once and saves nothing
				updTrialRestarted = false
			} else {
				a.settings.UpdateSeen = version
				a.saveSettings()
			}
			a.updNotes = false
		}
		a.updOpen = false
	}
}

// settingsUpdate is the settings' section of the updates.
func (a *app) settingsUpdate(c *ui.Context) {
	cl := ryugo.From(c).Colors
	check := !a.settings.UpdateOff
	quiet := a.settings.UpdateQuiet
	auto := a.settings.UpdateAuto
	status := locales.UpdateStatus(shownVersion(version))
	switch {
	case a.updRel != nil:
		status = locales.UpdateStatusAvailable(shownVersion(version), shownVersion(a.updRel.Version))
	case a.updErr != "":
		status = locales.UpdateStatusFailed(shownVersion(version), a.updErr)
	case a.settings.UpdateAt > 0:
		status = locales.UpdateStatusChecked(shownVersion(version), dateTime(time.Unix(a.settings.UpdateAt, 0), false))
	}
	local := a.updateURLOf() == "" // a build made by hand, with no address to ask: it says so, and where releases are (#497)
	if local {
		status = locales.UpdateLocalBuild(shownVersion(version))
	}
	settingGroup(c, "set-update",
		func() {
			settingRow(c, locales.UpdateCheck(), locales.UpdateCheckHelp(), func() {
				ryugo.Switch(c.Key("set-upd-on"), &check, ryugo.SwitchOptions{}).Label(locales.UpdateCheck())
			})
		},
		func() {
			settingRow(c, locales.UpdateStatusTitle(), status, func() {
				ui.Row(c).Gap(8).Children(func() {
					if local {
						if ryugo.Button(c.Key("upd-releases"), locales.UpdateReleases(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
							a.shellOpenSoon(releasesPage, false)
						}
						return
					}
					if ryugo.Button(c.Key("upd-check"), locales.UpdateCheckNow(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.checkUpdate(true)
					}
					if a.updRel != nil && ryugo.Button(c.Key("upd-show"), locales.UpdateShowNew(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						a.updNotes, a.updOpen = false, true
					}
				})
			})
		},
		func() {
			settingRow(c, locales.UpdateQuiet(), locales.UpdateQuietHelp(), func() {
				ryugo.Switch(c.Key("set-upd-quiet"), &quiet, ryugo.SwitchOptions{}).Label(locales.UpdateQuiet())
			})
		},
		func() {
			settingRow(c, locales.UpdateAuto(), locales.UpdateAutoHelp(), func() {
				ryugo.Switch(c.Key("set-upd-auto"), &auto, ryugo.SwitchOptions{}).Label(locales.UpdateAuto())
			})
		})
	if check != !a.settings.UpdateOff {
		a.settings.UpdateOff = !check
		a.saveSettings()
	}
	if quiet != a.settings.UpdateQuiet {
		a.settings.UpdateQuiet = quiet
		a.saveSettings()
	}
	if auto != a.settings.UpdateAuto {
		a.settings.UpdateAuto = auto
		a.saveSettings()
	}
	if a.settings.UpdateSkip != "" {
		settingGroup(c, "set-upd-skip", func() {
			settingRow(c, locales.UpdateSkipped(shownVersion(a.settings.UpdateSkip)), locales.UpdateSkippedHelp(), func() {
				if ryugo.Button(c.Key("upd-unskip"), locales.UpdateUnskip(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.UpdateSkip = ""
					a.saveSettings()
				}
			})
		})
	}
}

// releasedLine is the line under the title of the dialog of a new version: when it came out, as the window writes a
// date, and the version in use.
func releasedLine(date string) string {
	if d, err := time.Parse("2006-01-02", date); err == nil {
		return locales.UpdateReleasedYouHave(shortDate(d), shownVersion(version))
	}
	return locales.UpdateYouHave(shownVersion(version))
}

// shownVersion is a version as a sentence says it, after "Version": its number, without the tag's "v".
func shownVersion(v string) string { return strings.TrimPrefix(v, "v") }
