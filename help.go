package main

import (
	"sort"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The help: every shortcut on one page (? in the normal mode, ⌘⇧/ or the command), grouped by where it is
// used; and a short tour, the first time the app is opened, that says where the rest is.

type helpGroup struct {
	title string
	rows  [][2]string
}

// helpGroups are the shortcuts. The commands that have a key are listed from the palette's own list, so
// that one that is added there is here.
func (a *app) helpGroups() []helpGroup {
	groups := []helpGroup{
		{"Her yerde", [][2]string{
			{"⌘P · ⇧⌘P", "Komutlar ve bütün ayarlar"}, {"⌘K", "Sayfaya git, metinde ara"}, {"⌘,", "Ayarlar"},
			{"⌘S", "Kaydet"}, {"⌘F", "Sayfada bul"}, {"⇧⌘F", "Kitaplıkta ara"}, {"⌘J", "Bugünün sayfası"},
			{"⌘\\", "Kenar çubuğu"}, {"⇧⌘\\", "Yan panel"}, {"⌘[ ⌘]", "Geri, ileri"}, {"⌘= ⌘− ⌘0", "Yazıyı büyüt, küçült, varsayılan"},
			{"⌘1 ⌘2 ⌘3", "Kenar çubuğuna, yan panele, metne odaklan"}, {"⌥⌘→ ⌥⌘↵ ⌥⌘W", "Bölmeler: geç, yanda aç, kapat"},
		}},
		{"Normal kip (Esc ile girilir, i ile çıkılır)", [][2]string{
			{"j k ↓ ↑", "Sıradaki, önceki satır (5j: beş satır)"}, {"gg G", "İlk, son satır"}, {"l ↵", "Seçileni aç"}, {"h", "Geri"},
			{"f", "Basılabilen her şeye harf etiketi koy; harfi yaz"}, {"Space", "Leader menüsü (which-key)"},
			{"/", "Sayfada bul"}, {":", "Komut satırı"}, {"m a · ' a", "a harfiyle yer işaretle, ona git"},
			{"u ⌃R", "Geri al, yinele"}, {"p o s t b n w", "Komutlar, sayfa aç, ayarlar, görevler, kenar çubuğu, yeni sayfa, bölme"}, {"?", "Bu sayfa"},
		}},
		{"Palet kipleri", [][2]string{
			{"@", "Sayfa, koleksiyon, metin"}, {">", "Komutlar ve ayarlar (yazı boyutu 18)"}, {"#", "Etiketler"}, {":", "Satıra git, :w :q :e ad :tema koyu"},
			{"/", "Bu sayfada ara"}, {"!", "Son komutlar"}, {"?", "Kipleri listele"},
		}},
		{"Metin", [][2]string{
			{"⌘B ⌘I ⌘U", "Kalın, italik, altı çizili"}, {"⌘K", "Bağlantı"}, {"⌘Z ⇧⌘Z", "Geri al, yinele"}, {"⌘/", "Katla"}, {"⇧⌘↑ ⇧⌘↓", "Bloğu taşı"},
			{"⌘D", "Bloğu çoğalt"}, {"/", "Satır menüsü"}, {"Tab ⇧Tab", "Girinti"}, {"⌃Boşluk", "Satır menüsü"},
		}},
	}
	var cmds [][2]string
	seen := map[string]bool{}
	for _, it := range a.commands() {
		if it.keys != "" && it.keys != "✓" && it.sub == "Komut" && !seen[it.title] {
			seen[it.title] = true
			cmds = append(cmds, [2]string{it.keys, it.title})
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i][1] < cmds[j][1] })
	if len(cmds) > 0 {
		groups = append(groups, helpGroup{"Komutların kısayolları", cmds})
	}
	return groups
}

// helpView is the page of shortcuts.
func (a *app) helpView(c *ui.Context) {
	if !a.kb.help {
		return
	}
	cl := ryugo.From(c).Colors
	ryugo.Dialog(c.Key("help"), &a.kb.help, ryugo.DialogOptions{Title: "Kısayollar", Width: 680}, func() {
		ui.Scroll(c.Key("help-scroll")).MaxHeight(520).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 6, 0, 0).Children(func() {
				for gi, g := range a.helpGroups() {
					ui.Column(c.Key("hg" + itoa(gi))).FillWidth().Gap(4).Children(func() {
						ui.Text(c, T(g.title)).FontSize(13).FontWeight(680).TextColor(cl.Muted)
						for ri, r := range g.rows {
							ui.Row(c.Key("hr"+itoa(gi)+":"+itoa(ri))).FillWidth().Gap(12).Padding(2, 0).Children(func() {
								ui.Text(c, r[0]).FontSize(12.5).FontWeight(650).Font(ryugo.FontMono).TextColor(cl.Accent).Width(150)
								ui.Text(c, T(r[1])).FontSize(13).TextColor(cl.Text).Grow(1)
							})
						}
					})
				}
				ui.Text(c, T("Fare olmadan her şey: Esc ile normal kipe geç, f ile harf etiketlerini aç, ya da ⌘P'ye ne istediğini yaz.")).FontSize(12.5).TextColor(cl.Muted).Wrap()
			})
		})
	})
}

// ---- the tour ----

var tourSteps = []struct{ title, body string }{
	{"Yaz", "Bir sayfa aç ve yaz. # başlık, - liste, [] görev, > alıntı; yazdıkça biçime girer. Bir şey kaybolmaz: sürüm geçmişi ve çöp var."},
	{"Her şey ⌘P'de", "⌘P komutları ve bütün ayarları açar: “yazı boyutu 18”, “tema koyu”, “yeni sayfa”. ⌘K sayfaya götürür, # etiketlere, : satıra."},
	{"Fare olmadan", "Esc ile normal kipe geç: j k ile gez, ↵ ile aç, f ile her şeye harf koy, Space ile menü, ? ile kısayolların hepsi. i yazmaya döndürür."},
	{"Senin kitaplığın", "Her şey tek dosyada, bu bilgisayarda; her gün yedeği alınır. Ayarlar'dan yazı tipini, temayı ve MCP'yi seç."},
}

// tourView is the first-run tour, a card with four steps, which is not shown again once it is closed.
func (a *app) tourView(c *ui.Context) {
	if a.settings.TourDone || a.tourOff || a.modal() || a.settingsFile == "" && a.settingsStore == nil { // a library with no settings of its own is a test's, or a scratch one
		return
	}
	cl := ryugo.From(c).Colors
	step := min(a.tourAt, len(tourSteps)-1)
	ui.Overlay(c, func() {
		ui.Column(c.Key("tour")).Absolute().Right(20).Bottom(46).Width(340).Padding(16, 18).Gap(8).Radius(rad(14)).Background(cl.Card).Border(1, cl.Hairline).Children(func() {
			ui.Text(c, itoa(step+1)+" / "+itoa(len(tourSteps))).FontSize(11.5).TextColor(cl.Muted)
			ui.Text(c, T(tourSteps[step].title)).FontSize(16).FontWeight(700).TextColor(cl.Text)
			ui.Text(c, tourSteps[step].body).FontSize(13).TextColor(cl.Text2).Wrap()
			ui.Row(c).Gap(8).Justify(ui.End).Padding(6, 0, 0, 0).Children(func() {
				if ryugo.Button(c.Key("tour-skip"), "Bir daha gösterme", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.TourDone = true
					a.saveSettings()
				}
				label := "İleri"
				if step == len(tourSteps)-1 {
					label = "Bitti"
				}
				if ryugo.Button(c.Key("tour-next"), label, ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
					if step == len(tourSteps)-1 {
						a.settings.TourDone = true
						a.saveSettings()
					} else {
						a.tourAt++
					}
				}
			})
		})
	})
}
