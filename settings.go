package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The settings: a Ryugo dialog the gear at the foot of the sidebar opens.
// For now, the MCP server: on, it shows where agents reach it and the line
// that adds it to Claude Code.

func (a *app) settingsDialog(c *ui.Context) {
	ryugo.Dialog(c.Key("settings"), &a.settingsOpen, ryugo.DialogOptions{Title: "Ayarlar", Width: 520, Fixed: true}, func() {
		ryugo.Tabs(c.Key("settings-tabs"), &a.settingsTab, []ryugo.Tab{{Label: "Görünüm"}, {Label: "Editör"}, {Label: "Durum çubuğu"}, {Label: "MCP"}}, ryugo.TabsOptions{Label: "Ayarlar"})
		ui.Column(c).Gap(14).Padding(14, 0, 0, 0).Children(func() {
			switch a.settingsTab {
			case 0:
				a.settingsAppearance(c)
			case 1:
				a.settingsEditor(c)
			case 2:
				a.settingsStatus(c)
			default:
				a.settingsMCP(c)
			}
		})
	})
}

// settingRow is a setting: what it is, and its control.
func settingRow(c *ui.Context, title, help string, control func()) {
	cl := ryugo.From(c).Colors
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text)
			if help != "" {
				ui.Text(c, help).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			}
		})
		control()
	})
}

func (a *app) settingsAppearance(c *ui.Context) {
	theme := a.settings.Theme % 3
	settingRow(c, "Tema", "Sistemin görünümünü izler ya da hep açık ya da koyu olur.", func() {
		ryugo.Segmented(c.Key("set-theme"), &theme, []string{"Sistem", "Açık", "Koyu"}, ryugo.SegmentedOptions{Label: "Tema"})
	})
	if theme != a.settings.Theme {
		a.settings.Theme = theme
		a.saveSettings()
	}
	w := float64(a.pageWidth())
	settingRow(c, "Sayfa genişliği", "Metin sütununun genişliği.", func() {
		ui.Box(c).Width(220).Children(func() {
			ryugo.Slider(c.Key("set-width"), &w, 520, 1100, ryugo.SliderOptions{Label: "Sayfa genişliği", Step: 20, Format: func(v float64) string { return strconv.Itoa(int(v)) + " px" }})
		})
	})
	if int(w) != int(a.pageWidth()) {
		a.settings.PageWidth = w
		a.saveSettings()
	}
}

func (a *app) settingsEditor(c *ui.Context) {
	caret := int(a.caretKind)
	settingRow(c, "Caret", "Slime yazarken esner; Mizan yazma hızını gösterir. ⌘⇧M ile değişir.", func() {
		ryugo.Segmented(c.Key("set-caret"), &caret, []string{"Slime", "Mizan"}, ryugo.SegmentedOptions{Label: "Caret"})
	})
	if caret != int(a.caretKind) {
		a.setCaret(caretKind(caret))
	}
	fs := float64(a.fontSize())
	settingRow(c, "Yazı boyutu", "", func() {
		ui.Box(c).Width(220).Children(func() {
			ryugo.Slider(c.Key("set-font"), &fs, 13, 24, ryugo.SliderOptions{Label: "Yazı boyutu", Step: 0.5, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + " px" }})
		})
	})
	if float32(fs) != a.fontSize() {
		a.settings.FontSize = fs
		a.saveSettings()
	}
	lh := float64(a.lineHeight())
	settingRow(c, "Satır aralığı", "", func() {
		ui.Box(c).Width(220).Children(func() {
			ryugo.Slider(c.Key("set-lh"), &lh, 1.3, 2, ryugo.SliderOptions{Label: "Satır aralığı", Step: 0.05, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }})
		})
	})
	if float32(lh) != a.lineHeight() {
		a.settings.LineHeight = lh
		a.saveSettings()
	}
}

func (a *app) settingsStatus(c *ui.Context) {
	toggle := func(key, title, help string, v *bool, invert bool) {
		on := *v != invert
		settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(title) })
		if on != (*v != invert) {
			*v = on != invert
			a.saveSettings()
		}
	}
	toggle("set-saved", "Kaydetme durumu", "Kaydedildi, kaydediliyor…", &a.settings.HideSaved, true)
	toggle("set-caretname", "Caret adı", "", &a.settings.HideCaret, true)
	toggle("set-words", "Kelime sayısı", "", &a.settings.ShowWords, false)
	toggle("set-chars", "Harf sayısı", "", &a.settings.ShowChars, false)
}

func (a *app) settingsMCP(c *ui.Context) {
	{
		t := ryugo.From(c)
		cl := t.Colors
		ui.Column(c).Gap(14).Children(func() {
			on := a.settings.MCP
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, "MCP sunucusu").FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text)
					ui.Text(c, "Açıkken ajanlar (Claude Code ya da başka bir MCP istemcisi) bu kitaplığa bağlanır: koleksiyon ve sayfa oluşturur, okur, düzenler, taşır. Yaptıkları pencerede anında görünür.").
						FontSize(ryugo.FsSm).TextColor(cl.Muted)
				})
				ryugo.Switch(c.Key("mcp-switch"), &on, ryugo.SwitchOptions{}).Label("MCP sunucusunu aç")
			})
			if on != a.settings.MCP {
				a.setMCP(on)
			}
			if a.mcpErr != "" {
				ui.Text(c, "Açılamadı: "+a.mcpErr).FontSize(ryugo.FsSm).TextColor(cl.Up)
			}
			if !a.settings.MCP {
				return
			}
			copyRow := func(label, text, key string) {
				ryugo.Field(c, ryugo.FieldOptions{Label: label}, func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(t.R(ryugo.RadiusControl)-2).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
							ui.Text(c, text).FontSize(12.5).Font(ryugo.FontMono).TextColor(cl.Text2).Selectable()
						})
						ryugo.CopyButton(c.Key(key), text, ryugo.CopyOptions{Label: "Kopyala: " + label})
					})
				})
			}
			copyRow("Adres", a.mcpURL(), "copy-url")
			copyRow("Claude Code'a eklemek için", a.mcpCommand(), "copy-cmd")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Yalnız bu bilgisayardan, anahtarla bağlanılır.").FontSize(ryugo.FsSm).TextColor(cl.Muted).Grow(1)
				if ryugo.Button(c.Key("new-token"), "Anahtarı yenile", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.Token = newToken()
					a.saveSettings()
				}
			})
		})
	}
}
