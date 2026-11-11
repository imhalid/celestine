package main

import (
	"encoding/base64"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The account's screen: the profile (a screen of the window like the settings: who one is, the server, the quota, the
// sync, the recovery key, the computers signed in, the password). The app is used without an account: one is only for
// synchronizing through the server, and is made or signed in to in the sync settings (account.go).

var iconUser = icon(`<circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-6 8-6s8 2 8 6"/>`)

// openProfile shows the profile.
func (a *app) openProfile() {
	if a.profileScreen {
		return
	}
	a.flush()
	a.profileScreen, a.statsScreen, a.settingsOpen, a.coll, a.tag, a.trashOpen, a.tasksOpen, a.searchOpen = true, false, false, nil, "", false, false, false
	a.ac.meAt = time.Time{} // the server is asked again: the quota and the computers are what they are now
	a.ac.err = ""
	a.visit()
}

// closeProfile goes back to where the profile was opened from.
func (a *app) closeProfile() {
	if !a.step(-1) {
		a.profileScreen = false
	}
}

// profileView is the screen.
func (a *app) profileView(c *ui.Context) {
	cl := ryugo.From(c).Colors
	s := &a.settings
	in := s.AccountUser != "" && s.SyncToken != ""
	if in {
		a.acctRefresh()
	}
	title, sub := T("Profil"), T("Giriş yapılmadı: kitaplığın yalnız bu bilgisayarda.")
	initial := "?"
	if in {
		title, initial = s.AccountUser, strings.ToUpper(string([]rune(s.AccountUser)[:1]))
		sub = a.acctHost()
	}
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Row(c).FillWidth().MaxWidth(viewMax).Gap(14).AlignItems(ui.Center).Padding(2, 0, 0, 0).Children(func() {
				ui.Column(c.Key("prof-avatar")).Width(56).Height(56).Shrink(0).Radius(rad(28)).Background(cl.Accent.Alpha(0.22)).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, initial).FontSize(24).FontWeight(700).TextColor(cl.Text)
				})
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, title).FontSize(30).FontWeight(720).TextColor(cl.Text).MaxLines(1)
					ui.Text(c, sub).FontSize(ryugo.FsMd).TextColor(cl.Muted).MaxLines(1)
				})
			})
		})
		a.scrolled(c, "prof-scroll", nil, ui.Scroll(c.Key("prof-scroll")).FillWidth().Grow(1).MinHeight(0), func() {
			ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
				ui.Column(c.Key("prof-body")).FillWidth().MaxWidth(viewMax).Gap(14).Padding(0, 4, 40, 0).Children(func() {
					if !in {
						settingGroup(c, "prof-in", func() {
							ui.Text(c, T("Hesap yalnız sunucu üzerinden eşitleme içindir. Giriş yapmadan da her şey çalışır; dosyaların bu bilgisayarda, düz ve taşınabilir kalır.")).FontSize(13).TextColor(cl.Muted).Wrap()
						}, func() { a.acctForm(c) })
						return
					}
					a.profileAccount(c, cl)
					a.profileSync(c, cl)
					a.profileRecovery(c, cl)
					a.acctManage(c)
					settingGroup(c, "prof-out", func() {
						settingRow(c, "Çıkış yap", "Bu bilgisayar hesaptan çıkar; kitaplık burada kalır, eşitleme durur.", func() {
							if ryugo.Button(c.Key("acct-out"), T("Çıkış yap"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
								a.acctSignOut()
							}
						})
					})
				})
			})
		})
	})
}

// acctHost is the server in a word: its host.
func (a *app) acctHost() string {
	if u, err := url.Parse(a.acctServer()); err == nil && u.Host != "" {
		return u.Host
	}
	return a.acctServer()
}

// profileAccount is who one is, and the room used on the server.
func (a *app) profileAccount(c *ui.Context, cl ryugo.Colors) {
	s := &a.settings
	settingGroup(c, "prof-acct",
		func() {
			settingRow(c, "Kullanıcı adı", "", func() { ui.Text(c, s.AccountUser).FontSize(14).TextColor(cl.Text) })
		},
		func() {
			settingRow(c, "Sunucu", "", func() { ui.Text(c, a.acctHost()).FontSize(14).TextColor(cl.Text) })
		},
		func() {
			if a.ac.me == nil {
				ui.Text(c, T("Yükleniyor…")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				return
			}
			me := a.ac.me
			frac := float32(0)
			if me.Quota > 0 {
				frac = float32(min(me.Bytes, me.Quota)) / float32(me.Quota)
			}
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Row(c).FillWidth().Children(func() {
					ui.Text(c, T("Kullanılan yer")).FontSize(15).FontWeight(620).TextColor(cl.Text).Grow(1)
					ui.Text(c, humanBytes(me.Bytes)+" / "+humanBytes(me.Quota)).FontSize(13).TextColor(cl.Muted)
				})
				col := cl.Accent
				if frac > 0.9 {
					col = cl.Warn
				}
				ui.Box(c.Key("prof-quota")).FillWidth().Height(8).Radius(rad(4)).Background(cl.Well).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * frac, H: r.H}, col, rad(4))
				})
			})
		})
}

// profileSync is the state of the synchronization, with the two buttons: sync now, and try the connection.
func (a *app) profileSync(c *ui.Context, cl ryugo.Colors) {
	status := a.sy.status
	if status == "" {
		status = T("henüz eşitlenmedi")
	}
	if a.sy.err != "" {
		status += " · " + a.sy.err
	}
	settingGroup(c, "prof-sync",
		func() {
			settingRow(c, T("Eşitleme: ")+status, a.sy.result, func() {
				ui.Row(c).Gap(8).Children(func() {
					if ryugo.Button(c.Key("prof-sync-now"), T("Şimdi eşitle"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						a.syncNow()
					}
					if ryugo.Button(c.Key("prof-probe"), T("Bağlantıyı dene"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.syncProbe()
					}
				})
			})
			if a.ac.probe != "" || a.ac.probing {
				msg := a.ac.probe
				if a.ac.probing {
					msg = T("Deneniyor…")
				}
				ui.Text(c, msg).FontSize(13).TextColor(cl.Muted).Wrap()
			}
		},
		func() {
			ui.Text(c, T("Bir sayfanın sunucudaki eski halleri: sayfanın menüsünde “Sunucudaki sürümler…”. Sunucu, bir sayfa değişirken önceki hâlini en çok 10 dakikada bir, 10 kopya ve 30 gün tutar; kopyalar kotandan düşer.")).FontSize(13).TextColor(cl.Muted).Wrap()
		})
}

// profileRecovery shows the recovery key on request: it is the data key in words, kept here (settings), so that it can be looked at again.
func (a *app) profileRecovery(c *ui.Context, cl ryugo.Colors) {
	key, err := base64.StdEncoding.DecodeString(a.settings.SyncKey)
	if err != nil || len(key) != 32 {
		return
	}
	settingGroup(c, "prof-rec", func() {
		settingRow(c, "Kurtarma anahtarı", "Şifreni unutursan verini bununla geri alırsın (giriş ekranında “Şifremi unuttum”). Güvenli bir yere yaz; bunu bilen verini açabilir.", func() {
			if ryugo.Button(c.Key("prof-rec-show"), map[bool]string{true: T("Gizle"), false: T("Göster")}[a.ac.showRec], ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				a.ac.showRec = !a.ac.showRec
			}
		})
		if a.ac.showRec {
			rec := RecoveryKey(key)
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
				ui.Text(c, rec).FontSize(13.5).FontWeight(600).TextColor(cl.Text).Wrap().Grow(1)
				ryugo.CopyButton(c.Key("copy-recovery-prof"), rec, ryugo.CopyOptions{Label: T("Kopyala: kurtarma anahtarı")})
			})
		}
	})
}

// syncProbe tries the connection to where the library is synced, away from the window: that the place answers, that the account is
// good, that the key opens what is there; and how many files, and how long it took. It changes nothing.
func (a *app) syncProbe() {
	if a.ac.probing {
		return
	}
	remote, err := a.syncRemote()
	if err != nil {
		a.ac.probe = T("Olmadı: ") + T(err.Error())
		return
	}
	a.ac.probing, a.ac.probe = true, ""
	go func() {
		defer a.guard()
		t0 := time.Now()
		list, err := remote.List()
		took := time.Since(t0)
		a.do(func() error {
			a.ac.probing = false
			if err != nil {
				a.ac.probe = T("Olmadı: ") + err.Error()
				return nil
			}
			n := 0
			for name, e := range list {
				if !e.Deleted && !strings.HasPrefix(name, "_") {
					n++
				}
			}
			a.ac.probe = Tf("Bağlantı tamam: %d dosya, %d ms.", n, took.Milliseconds())
			return nil
		})
	}()
}

// ---- the versions the server keeps ----

// serverHist is the versions the server kept of a page, brought here, in the shape the history dialog reads.
type serverHist struct {
	vers []Version
	data map[int64][]byte
}

func (h *serverHist) Versions(string) []Version { return h.vers }
func (h *serverHist) VersionData(_ string, id int64) ([]byte, bool) {
	b, ok := h.data[id]
	return b, ok
}
func (h *serverHist) SnapshotNow(string)         {}
func (h *serverHist) SaveNamed(_, _ string) bool { return false }

// serverVersionsShown is how many of the newest versions are brought: each is a request, and the dialog needs them all to show a difference.
const serverVersionsShown = 10

// openServerHistory brings the versions of p that the server holds, and shows them in the history dialog.
func (a *app) openServerHistory(p *Page) {
	remote, err := a.syncRemote()
	enc, ok := remote.(*EncRemote)
	if err != nil || !ok || a.settings.SyncMode != "server" {
		a.notice = "Sunucudaki sürümler için sunucu hesabıyla eşitlemek gerekir"
		return
	}
	a.flush()
	name := strings.TrimPrefix(filepath.ToSlash(p.File()), StoreRoot)
	a.notice = "Sunucudaki sürümler getiriliyor…"
	go func() {
		defer a.guard()
		h := &serverHist{data: map[int64][]byte{}}
		vers, err := enc.Versions(name)
		for i := 0; err == nil && i < len(vers) && i < serverVersionsShown; i++ {
			var b []byte
			if b, err = enc.VersionData(name, vers[i].At); err == nil {
				h.vers = append(h.vers, Version{ID: vers[i].At, At: time.UnixMilli(vers[i].At), Size: len(b)})
				h.data[vers[i].At] = b
			}
		}
		a.do(func() error {
			if err != nil {
				a.notice = "Olmadı: " + err.Error()
				return nil
			}
			a.notice = ""
			a.vhist = histView{open: true, page: p, vers: h.vers, opsFor: -1, src: h, server: true}
			return nil
		})
	}()
}
