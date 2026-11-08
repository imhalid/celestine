package main

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The windows of an update: "a new version is out" with its notes; "this version is new" after one; and the
// settings' part, with when it last asked.

// restartApp starts the program again and quits this one.
func (a *app) restartApp() {
	exe, err := os.Executable()
	if err != nil {
		a.notice = "Yeniden başlatılamadı: " + err.Error()
		return
	}
	a.flush()
	cmd := exec.Command(exe, os.Args[1:]...)
	if err := cmd.Start(); err != nil {
		a.notice = "Yeniden başlatılamadı: " + err.Error()
		return
	}
	go func() {
		defer a.guard()
		time.Sleep(200 * time.Millisecond)
		mygo.App.Quit()
	}()
}

// updateView is the dialog of a new version, or of the notes of the one that was installed.
func (a *app) updateView(c *ui.Context) {
	cl := ryugo.From(c).Colors
	if a.updAuto && !a.updBusy.Load() {
		a.updAuto = false
		a.startUpdate()
	}
	if a.whatsNew() && !a.updOpen && !a.modal() {
		a.updNotes = true
		a.updOpen = true
	}
	if !a.updOpen {
		return
	}
	title, notes := "Yeni sürüm hazır", ""
	if a.updRel != nil {
		title, notes = "Sürüm "+a.updRel.Version+" hazır", a.updRel.Notes
	}
	if a.updNotes {
		title, notes = "Bu sürümde neler yeni: "+version, a.settings.UpdateNotes
	}
	open := a.updOpen
	ryugo.Dialog(c.Key("update"), &open, ryugo.DialogOptions{Title: title, Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			if a.updRel != nil && !a.updNotes {
				ui.Text(c, T("Şimdiki sürüm: ")+version+" · "+map[bool]string{true: a.updRel.Date, false: ""}[a.updRel.Date != ""]).FontSize(12.5).TextColor(cl.Muted)
			}
			if strings.TrimSpace(notes) != "" {
				a.scrolled(c, "upd-notes", nil, ui.Scroll(c.Key("upd-notes")).MaxHeight(240), func() {
					s := plainText(notes)
					a.textMenu(ui.Text(c, s).FontSize(13).TextColor(cl.Text2).Wrap().Selectable(), func() string { return s }, nil)
				})
			} else {
				ui.Text(c, T("Sürüm notu yok.")).FontSize(13).TextColor(cl.Muted)
			}
			if a.updErr != "" {
				ui.Text(c, a.updErr).FontSize(12.5).TextColor(cl.Up).Wrap()
			}
			ui.Text(c, T("Kitaplığına dokunulmaz, güncellemeden önce yedeği alınır; açılışta düşerse önceki sürüme dönülür.")).FontSize(12).TextColor(cl.Muted).Wrap()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				switch {
				case a.updNotes:
					if ryugo.Button(c.Key("upd-ok"), "Tamam", ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						open = false
					}
				case a.updState == "hazır":
					if ryugo.Button(c.Key("upd-restart"), "Yeniden başlat", ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						a.restartApp()
					}
					if ryugo.Button(c.Key("upd-later2"), "Sonra", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						open = false
					}
				case a.updState != "":
					ui.Text(c, a.updState).FontSize(13).TextColor(cl.Muted)
				default:
					if ryugo.Button(c.Key("upd-skip"), "Bu sürümü atla", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() && a.updRel != nil {
						a.settings.UpdateSkip = a.updRel.Version
						a.saveSettings()
						open = false
					}
					if ryugo.Button(c.Key("upd-snooze"), "3 gün sonra hatırlat", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.settings.UpdateSnooze = time.Now().Add(72 * time.Hour).Unix()
						a.saveSettings()
						open = false
					}
					if ryugo.Button(c.Key("upd-now"), "Şimdi güncelle", ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
						a.startUpdate()
					}
				}
			})
		})
	})
	if !open {
		if a.updNotes {
			a.settings.UpdateSeen = version
			a.saveSettings()
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
	beta := a.settings.UpdateChannel == "beta"
	auto := a.settings.UpdateAuto
	url := a.updateURLOf()
	status := "Sürüm " + version
	switch {
	case a.updRel != nil:
		status += " · yeni sürüm: " + a.updRel.Version
	case a.updErr != "":
		status += " · son denetim olmadı: " + a.updErr
	case a.settings.UpdateAt > 0:
		status += " · son denetim: " + time.Unix(a.settings.UpdateAt, 0).Format("2 Jan 15:04")
	}
	settingGroup(c, "set-update",
		func() {
			settingRow(c, "Yeni sürümü denetle", "Yalnız bu sürümün numarası sorulur: kitaplığından hiçbir şey gönderilmez. Kapalıyken uygulama ağa hiç çıkmaz (çevrimdışı mod).", func() {
				ryugo.Switch(c.Key("set-upd-on"), &check, ryugo.SwitchOptions{}).Label(T("Yeni sürümü denetle"))
			})
		},
		func() {
			settingRow(c, "Durum", status, func() {
				ui.Row(c).Gap(8).Children(func() {
					if ryugo.Button(c.Key("upd-check"), "Şimdi denetle", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.checkUpdate(true)
					}
					if a.updRel != nil && ryugo.Button(c.Key("upd-show"), T("Yeni sürümü göster"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						a.updNotes, a.updOpen = false, true
					}
				})
			})
		},
		func() {
			settingRow(c, "Sessiz mod", "Yeni sürüm iletişim kutusuyla çıkmaz, yalnız burada görünür.", func() { ryugo.Switch(c.Key("set-upd-quiet"), &quiet, ryugo.SwitchOptions{}).Label("Sessiz mod") })
		},
		func() {
			settingRow(c, "Deneme sürümleri", "Kararlı sürümlere ek olarak beta sürümleri de bildirilir.", func() {
				ryugo.Switch(c.Key("set-upd-beta"), &beta, ryugo.SwitchOptions{}).Label(T("Deneme sürümleri"))
			})
		},
		func() {
			settingRow(c, "Kendiliğinden indir ve kur", "Bulunan sürüm sorulmadan indirilir, imzası doğrulanır ve yerine konur; açmak için yeniden başlatman istenir.", func() {
				ryugo.Switch(c.Key("set-upd-auto"), &auto, ryugo.SwitchOptions{}).Label(T("Kendiliğinden kur"))
			})
		},
		func() {
			settingRow(c, "Güncelleme adresi", map[bool]string{true: url, false: "Ayarlanmamış: bu sürüm bir adres olmadan derlenmiş."}[url != ""], func() {})
		})
	if check != !a.settings.UpdateOff {
		a.settings.UpdateOff = !check
		a.saveSettings()
	}
	if quiet != a.settings.UpdateQuiet {
		a.settings.UpdateQuiet = quiet
		a.saveSettings()
	}
	if beta != (a.settings.UpdateChannel == "beta") {
		a.settings.UpdateChannel = map[bool]string{true: "beta", false: ""}[beta]
		a.saveSettings()
	}
	if auto != a.settings.UpdateAuto {
		a.settings.UpdateAuto = auto
		a.saveSettings()
	}
	if a.settings.UpdateSkip != "" {
		settingGroup(c, "set-upd-skip", func() {
			settingRow(c, "Atlanan sürüm: "+a.settings.UpdateSkip, "Bu sürüm bir daha bildirilmez; daha yenisi çıkınca haber verilir.", func() {
				if ryugo.Button(c.Key("upd-unskip"), "Yeniden bildir", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.UpdateSkip = ""
					a.saveSettings()
				}
			})
		})
	}
}
