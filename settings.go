package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The settings: a Ryugo dialog the gear at the foot of the sidebar opens, big,
// with its sections listed on the left and the one chosen on the right, in
// groups. Its title drags it about the window (Ryugo's dialogs do); the
// section chosen glows in the list and the choices that have a look - the
// theme, the caret - are tiles that lift as the pointer comes.

type settingsSection struct {
	name, blurb string
	glyph       *ui.SVG
}

var (
	iconTheme   = icon(`<circle cx="12" cy="12" r="7.5"/><path d="M12 4.5v15a7.5 7.5 0 0 0 0-15Z" fill="currentColor"/>`)
	iconEditing = icon(`<path d="M5 6.5h14M5 11h9M5 15.5h12M5 20h6"/>`)
	iconBar     = icon(`<rect x="4" y="15" width="16" height="4.5" rx="1.8"/><path d="M7.5 17.2h3M14 17.2h2.5"/><path d="M4 8.5h16M4 5h10" opacity=".55"/>`)
	iconPlug    = icon(`<path d="M9 4v5M15 4v5M6.5 9h11v2.5a5.5 5.5 0 0 1-11 0zM12 17v3.5"/>`)
	iconLibrary = icon(`<path d="M4.5 7.5a1.5 1.5 0 0 1 1.5-1.5h4l1.8 2H18a1.5 1.5 0 0 1 1.5 1.5v7.5a1.5 1.5 0 0 1-1.5 1.5H6a1.5 1.5 0 0 1-1.5-1.5z"/>`)
)

var settingsSections = []settingsSection{
	{"Görünüm", "Tema ve sayfanın genişliği", iconTheme},
	{"Editör", "Yazı, satır aralığı ve caret", iconEditing},
	{"Durum çubuğu", "Altta neyin görüneceği", iconBar},
	{"MCP", "Ajanların kitaplığa bağlanması", iconPlug},
	{"Kitaplık", "Dosyalar ve nerede durdukları", iconLibrary},
}

// selSpring is the list's choice settling, and a tile picked.
var selSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.1}}

// keepSettings writes what saveSettingsSoon left, once the settings have been
// still a moment or the dialog is shut.
func (a *app) keepSettings(c *ui.Context) {
	if !a.settingsDirty {
		return
	}
	if wait := 300*time.Millisecond - a.clock().Sub(a.settingsAt); wait > 0 && a.settingsOpen {
		c.After(wait)
		return
	}
	a.saveSettings()
}

func (a *app) settingsDialog(c *ui.Context) {
	_, wh := c.Size()
	h := max(420, min(620, wh-110))
	// no header across the top: the dialog is its two sides, edge to edge, and
	// a strip over their tops, which the window drags by, with the close button
	ryugo.Dialog(c.Key("settings"), &a.settingsOpen, ryugo.DialogOptions{Width: 960, Bare: func(grip func(ui.Element), closeIt func()) {
		cl := ryugo.From(c).Colors
		ui.Row(c).FillWidth().Height(h).AlignItems(ui.Stretch).Children(func() {
			a.settingsNav(c)
			ui.Box(c.Key("set-div")).Width(1).Background(cl.Hairline)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				sec := settingsSections[min(a.settingsTab, len(settingsSections)-1)]
				ui.Column(c.Key("set-head:"+strconv.Itoa(a.settingsTab))).FillWidth().Gap(4).Padding(22, 26, 14, 28).Shrink(0).Transition(viewSwap).Children(func() {
					ui.Text(c, sec.name).FontSize(22).FontWeight(700).TextColor(cl.Text)
					ui.Text(c, sec.blurb).FontSize(ryugo.FsMd).TextColor(cl.Muted)
				})
				ui.Scroll(c.Key("set-scroll")).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c.Key("set-body:"+strconv.Itoa(a.settingsTab))).FillWidth().Gap(18).Padding(4, 26, 24, 28).Transition(viewSwap).Children(func() {
						switch a.settingsTab {
						case 0:
							a.settingsAppearance(c)
						case 1:
							a.settingsEditor(c)
						case 2:
							a.settingsStatus(c)
						case 3:
							a.settingsMCP(c)
						default:
							a.settingsLibrary(c)
						}
					})
				})
			})
		})
		g := ui.Box(c.Key("set-grip")).Absolute().Top(0).Left(0).Right(0).Height(64)
		grip(g)
		x := ryugo.Button(c.Key("set-close"), "Kapat", ryugo.ButtonOptions{Icon: iconClose, IconOnly: true, Size: ryugo.Sm}).Absolute().Top(14).Right(14)
		if x.Clicked() {
			closeIt()
		}
	}}, func() {})
}

// settingsNav is the list of sections, on a ground of its own: the one chosen
// glows - the accent in a gentle gradient, as a button is - and the others
// come up as the pointer does.
func (a *app) settingsNav(c *ui.Context) {
	t := ryugo.From(c)
	cl := t.Colors
	ui.Column(c.Key("set-nav")).Width(200).Padding(0, 10, 12, 10).Gap(2).Shrink(0).Background(ryugo.Mix(cl.Card, cl.Bg, 0.7)).Children(func() {
		ui.Text(c, "Ayarlar").FontSize(17).FontWeight(650).TextColor(cl.Text).Padding(22, 10, 14, 10)
		for i, s := range settingsSections {
			sel := a.settingsTab == i
			e := ui.Row(c.Key("snav:"+s.name)).FillWidth().Height(34).Padding(0, 10).Gap(9).Radius(rad(9)).AlignItems(ui.Center).
				Label(s.name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			on := e.AnimateTo("on", b2f(sel), selSpring)
			hov := e.AnimateTo("hov", b2f(e.Hovered() && !sel), fadeQuick)
			if on > 0.01 {
				e.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true,
					From: ryugo.Mix(cl.Card, cl.Accent, 0.26*on), To: ryugo.Mix(cl.Card, cl.Accent, 0.14*on)}).Border(1, cl.Accent.Alpha(0.42*on))
			} else if hov > 0.01 {
				e.Background(cl.Fill.Alpha(hov))
			}
			e.Translate(2*on, 0)
			e.Children(func() {
				ui.Icon(c, s.glyph).Width(16).Height(16).TextColor(ryugo.Mix(cl.Muted, cl.Accent, on))
				ui.Text(c, s.name).FontSize(13.5).FontWeight(int(540 + 120*on)).TextColor(ryugo.Mix(cl.Text2, cl.Text, on)).Grow(1).MaxLines(1)
			})
			if e.Clicked() {
				a.settingsTab = i
			}
		}
	})
}

// settingGroup is a card of settings, a line between them.
func settingGroup(c *ui.Context, key string, rows ...func()) {
	cl := ryugo.From(c).Colors
	ui.Column(c.Key(key)).FillWidth().Radius(rad(16)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
		for i, r := range rows {
			if i > 0 {
				ui.Box(c.Key(key + "-d" + strconv.Itoa(i))).FillWidth().Height(1).Background(cl.Hairline)
			}
			ui.Column(c.Key(key+"-r"+strconv.Itoa(i))).FillWidth().Padding(14, 18).Children(r)
		}
	})
}

// settingRow is a setting: what it is, and its control.
func settingRow(c *ui.Context, title, help string, control func()) {
	cl := ryugo.From(c).Colors
	ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(15).FontWeight(620).TextColor(cl.Text)
			if help != "" {
				ui.Text(c, help).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			}
		})
		control()
	})
}

// choiceTile is a choice that has a look: a picture over its name, raised as a
// button is; it lifts as the pointer comes, and the chosen one is ringed in
// the accent. It reports a click.
func choiceTile(c *ui.Context, key, label string, on bool, preview func()) bool {
	t := ryugo.From(c)
	cl := t.Colors
	e := ui.Column(c.Key(key)).Width(128).Gap(8).Padding(7, 7, 10, 7).Radius(rad(16)).AlignItems(ui.Center).
		Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	lift := e.AnimateTo("lift", b2f(e.Hovered()), fadeQuick)
	sel := e.AnimateTo("sel", b2f(on), selSpring)
	e.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: cl.ControlTop, To: cl.ControlBottom}).
		Border(1.5, ryugo.Mix(cl.ControlBorder, cl.Accent, sel))
	if on {
		t.Shadows.Ring.Apply(e)
	} else {
		t.Shadows.ControlShadow.Apply(e)
	}
	e.Translate(0, -2*lift)
	e.Children(func() {
		ui.Box(c).Width(112).Height(66).Radius(rad(10)).Clip().Children(preview)
		ui.Text(c, label).FontSize(13.5).FontWeight(int(540 + 120*sel)).TextColor(ryugo.Mix(cl.Text2, cl.Text, sel))
	})
	return e.Clicked()
}

// mock is a little window, light or dark, for a tile.
func mock(c *ui.Context, dark bool, w float32) {
	bg, fg := ui.Hex("#f4f4f6"), ui.Hex("#1b1b1f")
	if dark {
		bg, fg = ui.Hex("#17171a"), ui.Hex("#ececf1")
	}
	ui.Row(c).Width(w).Height(66).Background(bg).Clip().Children(func() {
		ui.Box(c).Width(22).Height(66).Background(fg.Alpha(0.08)).Shrink(0)
		ui.Column(c).Padding(12, 8).Gap(6).Children(func() {
			ui.Box(c).Width(40).Height(6).Radius(rad(3)).Background(fg.Alpha(0.8))
			ui.Box(c).Width(54).Height(4).Radius(rad(2)).Background(fg.Alpha(0.3))
			ui.Box(c).Width(46).Height(4).Radius(rad(2)).Background(fg.Alpha(0.3))
			ui.Box(c).Width(28).Height(8).Radius(rad(4)).Background(ryugo.From(c).Colors.Accent)
		})
	})
}

func (a *app) settingsAppearance(c *ui.Context) {
	cl := ryugo.From(c).Colors
	theme := a.settings.Theme % 3
	settingGroup(c, "set-theme",
		func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "Tema").FontSize(15).FontWeight(620).TextColor(cl.Text)
					ui.Text(c, "Sistemin görünümünü izler ya da hep açık ya da koyu olur.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
				})
				ui.Row(c).Gap(14).Children(func() {
					if choiceTile(c, "tile-sys", "Sistem", theme == 0, func() { ui.Row(c).Children(func() { mock(c, false, 56); mock(c, true, 56) }) }) {
						theme = 0
					}
					if choiceTile(c, "tile-light", "Açık", theme == 1, func() { mock(c, false, 112) }) {
						theme = 1
					}
					if choiceTile(c, "tile-dark", "Koyu", theme == 2, func() { mock(c, true, 112) }) {
						theme = 2
					}
				})
			})
		})
	if theme != a.settings.Theme {
		a.settings.Theme = theme
		a.saveSettings()
	}
	settingGroup(c, "set-accent", func() { a.appThemePicker(c) })
	w := float64(a.pageWidth())
	settingGroup(c, "set-width", func() {
		settingRow(c, "Sayfa genişliği", "Metin sütununun genişliği.", func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-width-s"), &w, 520, 1100, ryugo.SliderOptions{Label: "Sayfa genişliği", Step: 20, Format: func(v float64) string { return strconv.Itoa(int(v)) + " px" }})
			})
		})
	})
	if int(w) != int(a.pageWidth()) {
		a.settings.PageWidth = w
		a.saveSettingsSoon()
	}
	corner := float64(a.cornerPercent())
	settingGroup(c, "set-corner", func() {
		settingRow(c, "Köşeler", "Bütün köşeler tek bir ölçüden hesaplanır: 0 köşeli, 100 tasarımın kendisi, 200 iki kat yuvarlak.", func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-corner-s"), &corner, 0, 200, ryugo.SliderOptions{Label: "Köşe yuvarlaklığı", Step: 10, Format: func(v float64) string { return strconv.Itoa(int(v)) + "%" }})
			})
		})
	})
	if int(corner) != a.cornerPercent() {
		v := int(corner)
		a.settings.Corner = &v
		a.saveSettingsSoon()
	}
}

func (a *app) settingsEditor(c *ui.Context) {
	cl := ryugo.From(c).Colors
	caret := int(a.caretKind)
	settingGroup(c, "set-caret",
		func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "Caret").FontSize(15).FontWeight(620).TextColor(cl.Text)
					ui.Text(c, "Slime yazarken esner; Mizan yazma hızını gösterir; Diğerleri hazır ayarlar: her biri olduğu gibi gelir, ayarı yoktur. ⌘⇧M ile değişir.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
				})
				ui.Row(c).Gap(14).Wrap().Children(func() {
					for k := caretKind(0); k < caretCount(); k++ {
						k := k
						if choiceTile(c, "tile-caret"+itoa(int(k)+1), caretName(k), caret == int(k), func() { caretMock(c, k) }) {
							caret = int(k)
						}
					}
				})
			})
		})
	if caret != int(a.caretKind) {
		a.setCaret(caretKind(caret))
	}
	fs, lh := float64(a.fontSize()), float64(a.lineHeight())
	settingGroup(c, "set-type",
		func() {
			settingRow(c, "Yazı boyutu", "", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-font"), &fs, 13, 24, ryugo.SliderOptions{Label: "Yazı boyutu", Step: 0.5, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + " px" }})
				})
			})
		},
		func() {
			settingRow(c, "Satır aralığı", "", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-lh"), &lh, 1.3, 2, ryugo.SliderOptions{Label: "Satır aralığı", Step: 0.05, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }})
				})
			})
		},
		func() { // how it reads, as it is set
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Önizleme").FontSize(ryugo.FsSm).FontWeight(600).TextColor(cl.Muted)
				ui.Text(c, "Bir fikir, bir sayfa. Yazdıkça çizgiler kendiliğinden biçime girer: başlık, liste, görev. Satırın aralığı ve harflerin boyu böyle okunur.").
					FontSize(float32(fs)).LineHeight(float32(lh)).TextColor(cl.Text)
			})
		})
	auto := !a.settings.NoAutoCover
	settingGroup(c, "set-cover", func() {
		settingRow(c, "Otomatik kapak", "Kapağı olmayan her sayfa, kendi tohumundan üretilen hareketsiz bir mozaik kapak alır: açık temada pastel, koyuda derin yumuşak renklerde, hepsi birbirinden farklı. Sayfada Kapağı kaldır dersen o sayfa kapaksız kalır.", func() {
			ryugo.Switch(c.Key("set-autocover"), &auto, ryugo.SwitchOptions{}).Label("Otomatik kapak")
		})
	})
	if auto != !a.settings.NoAutoCover {
		a.settings.NoAutoCover = !auto
		a.saveSettings()
	}
	a.settingsQuick(c)
	if float32(fs) != a.fontSize() {
		a.settings.FontSize = fs
		a.saveSettingsSoon()
	}
	if float32(lh) != a.lineHeight() {
		a.settings.LineHeight = lh
		a.saveSettingsSoon()
	}
}

// caretMock is a caret for a tile: Slime a green jelly, Mizan a line with the pace under it.
func caretMock(c *ui.Context, kind caretKind) {
	cl := ryugo.From(c).Colors
	ui.Row(c).Width(112).Height(66).Background(cl.Well).Padding(0, 14).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Merhab").FontSize(15).FontWeight(560).TextColor(cl.Text)
		switch i := int(kind) - int(caretFirstFX); {
		case kind == caretMizan:
			ui.Box(c).Width(2).Height(20).Radius(rad(1)).Background(cl.Accent)
		case i >= 0 && i < len(caretPresets):
			ui.Box(c).Size(36, 24).Draw(func(p *ui.Painter, r ui.Rect) { caretPresets[i].mock(p, r, ryugo.Painting(p).Dark) })
		default:
			ui.Box(c).Width(9).Height(21).Radius(rad(5)).LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ui.Hex("#7cf75e"), To: ui.Hex("#2f9a1c")}).Border(1, ui.Hex("#59f939").Alpha(0.6))
		}
	})
}

func (a *app) settingsStatus(c *ui.Context) {
	toggle := func(key, title, help string, v *bool, invert bool) func() {
		return func() {
			on := *v != invert
			settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(title) })
			if on != (*v != invert) {
				*v = on != invert
				a.saveSettings()
			}
		}
	}
	settingGroup(c, "set-status",
		toggle("set-saved", "Kaydetme durumu", "Kaydedildi, kaydediliyor…", &a.settings.HideSaved, true),
		toggle("set-words", "Kelime sayısı", "Açık sayfanın kelimeleri.", &a.settings.ShowWords, false),
		toggle("set-chars", "Harf sayısı", "Boşluklar ve satır sonları sayılmaz.", &a.settings.ShowChars, false),
		toggle("set-today", "Bugün yazılan", "Bugün yazılan kelime sayısı; tıklayınca istatistikler açılır.", &a.settings.ShowToday, false),
		toggle("set-reading", "Okuma süresi", "Dakikada 200 kelime ile: \"3 dk okuma\".", &a.settings.ShowReading, false),
		toggle("set-path", "Sayfa yolu", "Koleksiyon › üst sayfa › sayfa.", &a.settings.ShowPath, false),
		toggle("set-sel", "Seçili harf sayısı", "Metin seçiliyken: \"42 harf seçili\".", &a.settings.ShowSel, false),
		toggle("set-pos", "Satır ve sütun", "Caret'in paragrafı ve içindeki yeri.", &a.settings.ShowPos, false),
		toggle("set-edited", "Son düzenleme", "\"3 dk önce düzenlendi\".", &a.settings.ShowEdited, false),
		toggle("set-links", "Geri bağlantılar", "Bu sayfaya bağlanan sayfa sayısı; tıklayınca yan panel açılır.", &a.settings.ShowLinks, false),
	)
}

func (a *app) settingsLibrary(c *ui.Context) {
	cl := ryugo.From(c).Colors
	pages := 0
	for _, co := range a.lib.Collections {
		pages += len(co.All())
	}
	row := func(label, text, key string) func() {
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: label}, func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(rad(ryugo.From(c).R(ryugo.RadiusControl)-2)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
						ui.Text(c, text).FontSize(12.5).Font(ryugo.FontMono).TextColor(cl.Text2).Selectable().MaxLines(1)
					})
					ryugo.CopyButton(c.Key(key), text, ryugo.CopyOptions{Label: "Kopyala: " + label})
				})
			})
		}
	}
	store, inFile := a.lib.fs.(*Store)
	if !inFile {
		settingGroup(c, "set-lib",
			row("Kitaplık klasörü", a.lib.Root, "copy-root"),
			row("Ayar dosyası", a.settingsFile, "copy-settings"),
			func() {
				settingRow(c, strconv.Itoa(len(a.lib.Collections))+" koleksiyon, "+strconv.Itoa(pages)+" sayfa", "Hepsi düz dosya: başka bir editörle de açılır.", func() {})
			})
		return
	}
	settingGroup(c, "set-lib",
		row("Kitaplık dosyası", store.File, "copy-root"),
		func() {
			settingRow(c, strconv.Itoa(len(a.lib.Collections))+" koleksiyon, "+strconv.Itoa(pages)+" sayfa",
				"Hepsi bu tek dosyada; ayarlar ve MCP anahtarı da. Dosyayı uygulamanın yanında taşıman yeter.", func() {})
		},
		func() { a.settingsImport(c) },
		func() {
			note := a.exportNote
			if note == "" {
				note = "Düz .md klasörü olarak, dosyanın yanına yazılır."
			}
			settingRow(c, "Markdown olarak dışa aktar", note, func() {
				if ryugo.Button(c.Key("export-md"), "Dışa aktar", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.exportNote = a.exportLibrary(store)
				}
			})
		})
}

// exportLibrary writes the library as a folder of Markdown files next to
// its file - the folder library of plain files, which ryudit opens as it is
// given on the command line - and says where it went.
func (a *app) exportLibrary(store *Store) string {
	dir := filepath.Join(filepath.Dir(store.File), "kitaplik-md-"+time.Now().Format("20060102-150405"))
	a.flush()
	if err := copyTree(store, storeRoot, osFS{}, dir); err != nil {
		return "Olmadı: " + err.Error()
	}
	return "Yazıldı: " + dir
}

func (a *app) settingsMCP(c *ui.Context) {
	t := ryugo.From(c)
	cl := t.Colors
	on := a.settings.MCP
	settingGroup(c, "set-mcp", func() {
		settingRow(c, "MCP sunucusu", "Açıkken ajanlar (Claude Code ya da başka bir MCP istemcisi) bu kitaplığa bağlanır: koleksiyon ve sayfa oluşturur, okur, düzenler, taşır. Yaptıkları pencerede anında görünür.", func() {
			ryugo.Switch(c.Key("mcp-switch"), &on, ryugo.SwitchOptions{}).Label("MCP sunucusunu aç")
		})
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
	copyRow := func(label, text, key string) func() {
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: label}, func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(rad(ryugo.RadiusControl-2)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
						ui.Text(c, text).FontSize(12.5).Font(ryugo.FontMono).TextColor(cl.Text2).Selectable()
					})
					ryugo.CopyButton(c.Key(key), text, ryugo.CopyOptions{Label: "Kopyala: " + label})
				})
			})
		}
	}
	settingGroup(c, "set-mcp-how",
		copyRow("Adres", a.mcpURL(), "copy-url"),
		copyRow("Claude Code'a eklemek için", a.mcpCommand(), "copy-cmd"),
		func() {
			settingRow(c, "Anahtar", "Yalnız bu bilgisayardan, anahtarla bağlanılır.", func() {
				if ryugo.Button(c.Key("new-token"), "Anahtarı yenile", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.Token = newToken()
					a.saveSettings()
				}
			})
		})
}

// settingsQuick is the key of the quick note.
func (a *app) settingsQuick(c *ui.Context) {
	cl := ryugo.From(c).Colors
	cur := "—"
	if a.quick != nil && a.quick.shortcut != "" {
		cur = a.quick.shortcut
	} else if a.settings.QuickKey != "" {
		cur = a.settings.QuickKey
	}
	if a.quickKeyDraft == "" {
		a.quickKeyDraft = cur
	}
	settingGroup(c, "set-quick", func() {
		settingRow(c, "Hızlı not tuşu", "Her yerden paneli açar. Örnek: CmdOrCtrl+Alt+N, Alt+Shift+Space. Wayland'da masaüstü onay isteyebilir.", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ryugo.Input(c.Key("set-quick-in"), &a.quickKeyDraft, ryugo.InputOptions{Label: "Hızlı not tuşu", Size: ryugo.Sm}).Width(190)
				apply := ryugo.Button(c.Key("set-quick-ok"), "Uygula", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() || in.Submitted()
				if apply && a.quick != nil {
					if err := a.quick.setShortcut(strings.TrimSpace(a.quickKeyDraft)); err != nil {
						a.quickKeyErr = err.Error()
					} else {
						a.quickKeyErr = ""
						a.settings.QuickKey = a.quick.shortcut
						a.saveSettings()
					}
				}
			})
		})
		if a.quickKeyErr != "" {
			ui.Text(c, a.quickKeyErr).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
	})
}
