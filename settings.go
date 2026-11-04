package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The settings: a Ryugo dialog the gear at the foot of the sidebar opens.
// For now, the MCP server: on, it shows where agents reach it and the line
// that adds it to Claude Code.

func (a *app) settingsDialog(c *ui.Context) {
	ryugo.Dialog(c.Key("settings"), &a.settingsOpen, ryugo.DialogOptions{Title: "Ayarlar", Width: 520}, func() {
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
	})
}
