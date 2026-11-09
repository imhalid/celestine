package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/ryudit/internal/library"
)

// A real library (it has settings of its own) with no account is not behind a sign-in: the app opens, and the account is
// made or signed in to in the sync settings, where the form starts with the invite.
func TestNoLoginGate(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "x\n")
	a.settingsFile = t.TempDir() + "/s.json"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if tt.HasText("Erişim kodu") || !tt.HasText("Bir daha gösterme") {
		t.Fatalf("the app does not open without an account: %v", tt.Texts())
	}
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	for _, want := range []string{"Davetle hesap aç", "Giriş yap", "E-posta", "Erişim kodu", "Kodum yok: bekleme listesine yazıl"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in the sync settings: %v", want, tt.Texts())
		}
	}
	// Enter in a field is the form's button
	r, ok := tt.Find("Erişim kodu") // the field's label: the field is under it
	if !ok {
		t.Fatal("no code field")
	}
	tt.ClickAt(r.X+20, r.Y+r.H+16)
	tt.Type("RYU-AAAAAA")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.ac.err == "" {
		t.Fatalf("Enter did not send the form: focused %v, code %q", tt.Focused("Erişim kodu"), a.ac.code)
	}
}

// What the form says before it asks the server anything.
func TestAcctFormChecks(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	check := func(mode, user, pass, code, rec, nw, want string) {
		t.Helper()
		a.ac = acctUI{mode: mode, user: user, pass: pass, code: code, recKey: rec, newPass: nw}
		a.acctSubmit()
		if a.ac.err != want || a.ac.busy {
			t.Errorf("%s %q: error %q, busy %v; want %q", mode, user, a.ac.err, a.ac.busy, want)
		}
	}
	check("login", "", "x", "", "", "", "E-posta gerekli.")
	check("login", "ali@ornek.com", "", "", "", "", "E-posta ve şifre gerekli.")
	check("invite", "ali", "", "RYU-AAAAAA", "", "", "E-posta adresi geçersiz.")
	check("invite", "ali@ornek.com", "", " ", "", "", "Erişim kodu gerekli.")
	check("password", "ali@ornek.com", "kısa", "RYU-AAAAAA", "", "", "Şifre en az 8 karakter olsun.")
	check("reset", "ali@ornek.com", "", "", "", "yeni-parola", "Kurtarma anahtarı gerekli.")
	check("reset", "ali@ornek.com", "", "", "AAAA-BBBB", "kısa", "Yeni şifre en az 8 karakter olsun.")
	a.ac = acctUI{mode: "password", user: "ali@ornek.com", pass: "uzun-parola", pass2: "uzun-parolA", code: "RYU-AAAAAA"}
	a.acctSubmit()
	if a.ac.err != "Şifreler aynı değil." || a.ac.busy {
		t.Errorf("two passwords: %q", a.ac.err)
	}
}

// An invite opens the account in steps: the address and the code, checked by the server and not spent, then the password, then the
// recovery key, which the form shows before anything else and leaves only when one says it is kept.
func TestInviteSteps(t *testing.T) {
	withLang(t, "tr")
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		got = append(got, r.URL.Path+" "+in["email"])
		switch r.URL.Path {
		case "/api/invite":
			if in["code"] != "RYU-AAAAAA" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"e-posta ya da erişim kodu yanlış, ya da kodun süresi dolmuş"}`))
				return
			}
			w.Write([]byte(`{"ok":true}`))
		case "/api/register":
			if in["code"] != "RYU-AAAAAA" || in["username"] != "" || len(in["auth"]) != 64 {
				w.WriteHeader(400)
				return
			}
			w.Write([]byte(`{"token":"t1"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	library.AllowHTTP = true
	defer func() { library.AllowHTTP = false }()
	a, tt := newTestApp(t, "x\n")
	a.settingsFile = t.TempDir() + "/s.json"
	a.settings.SyncURL = srv.URL
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if a.ac.mode != "invite" || !tt.HasText("Erişim kodu") {
		t.Fatalf("the form does not start with the invite: %q %v", a.ac.mode, tt.Texts())
	}
	wait := func() {
		t.Helper()
		for i := 0; i < 200 && a.ac.busy; i++ {
			time.Sleep(10 * time.Millisecond)
			tt.Frame()
		}
		tt.Frame()
	}
	a.ac.user, a.ac.code = "Ali@Ornek.com ", "RYU-BBBBBB"
	a.acctSubmit()
	wait()
	if a.ac.mode != "invite" || !strings.Contains(a.ac.err, "erişim kodu yanlış") {
		t.Fatalf("a wrong code went on: %q %q", a.ac.mode, a.ac.err)
	}
	a.ac.code = "ryu-aaaaaa"
	a.ac.code = "RYU-AAAAAA"
	a.acctSubmit()
	wait()
	if a.ac.mode != "password" || !tt.HasText("Şifre (tekrar)") {
		t.Fatalf("no password step: %q %q %v", a.ac.mode, a.ac.err, tt.Texts())
	}
	a.ac.pass, a.ac.pass2 = "uzun-parola", "uzun-parola"
	a.acctSubmit()
	wait()
	if a.ac.mode != "recovery" || a.sy.recovery == "" || !tt.HasText(a.sy.recovery) {
		t.Fatalf("the recovery key is not shown: %q %q %v", a.ac.mode, a.ac.err, tt.Texts())
	}
	if a.settings.AccountUser != "ali@ornek.com" || a.settings.SyncToken != "t1" {
		t.Fatalf("account %q token %q", a.settings.AccountUser, a.settings.SyncToken)
	}
	tt.Click("Sakladım, devam")
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if a.ac.mode != "login" || tt.HasText("Sakladım, devam") {
		t.Fatalf("the recovery step stays after the key is kept: %q", a.ac.mode)
	}
	want := []string{"/api/invite ali@ornek.com", "/api/invite ali@ornek.com", "/api/register ali@ornek.com"}
	if len(got) < 3 || strings.Join(got[:3], ",") != strings.Join(want, ",") {
		t.Fatalf("calls %v", got)
	}
}

// The profile: a screen like the settings, in the history, left with Esc; signed out it is the form, signed in it is the account.
func TestProfileScreen(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "x\n")
	a.openProfile()
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !a.profileScreen || a.paneTitle() != "Profil" || a.here().kind != "profile" {
		t.Fatalf("the profile is not shown: screen %v, title %q, place %v", a.profileScreen, a.paneTitle(), a.here())
	}
	for _, want := range []string{"Profil", "Giriş yapılmadı: kitaplığın yalnız bu bilgisayarda.", "Giriş yap", "Davetle hesap aç", "E-posta"} {
		if !tt.HasText(want) {
			t.Errorf("signed out: no %q: %v", want, tt.Texts())
		}
	}

	a.settings.AccountUser, a.settings.SyncToken, a.ac.meAt = "ali", "tok", time.Now() // no call to a server
	a.settings.SyncMode, a.settings.SyncURL = "server", "https://sync.example.com"
	a.settings.SyncKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	a.ac.me = &AcctMe{Username: "ali", Bytes: 5 << 20, Quota: 200 << 20, Devices: []DeviceInfo{{ID: "aaaaaaaaaaaa", Name: "MacBook", Seen: 1, Current: true}, {ID: "bbbbbbbbbbbb", Name: "Telefon", Seen: 1}}}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"ali", "sync.example.com", "Kullanılan yer", "5.0 MB / 200.0 MB", "Şimdi eşitle", "Bağlantıyı dene", "Kurtarma anahtarı", "Şifreyi değiştir", "MacBook", "Telefon", "Hesabı sil", "Çıkış yap"} {
		if !tt.HasText(want) {
			t.Errorf("signed in: no %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("Giriş yapılmadı") {
		t.Error("signed in, and it says it is not")
	}
	// the recovery key is not shown until it is asked for
	rec := RecoveryKey(make([]byte, 32))
	if tt.HasText(rec) {
		t.Fatal("the recovery key is on the screen unasked")
	}
	tt.Click("Göster")
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(rec) {
		t.Fatalf("the recovery key does not show when asked: %v", tt.Texts())
	}

	// it is a place in the history: from the figures, back goes to it; Esc leaves it
	a.openStats()
	if a.profileScreen || !a.statsScreen {
		t.Fatal("the figures did not take the screen")
	}
	if !a.step(-1) || !a.profileScreen {
		t.Fatal("back from the figures did not come to the profile")
	}
	a.closeProfile()
	if a.profileScreen {
		t.Fatal("the profile did not close")
	}
}

// Signing out forgets the account here and tells the server; the library stays.
func TestProfileSignOut(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.settings.AccountUser, a.settings.SyncToken, a.settings.SyncKey, a.settings.SyncMode = "ali", "tok", "k", "server"
	a.settings.SyncURL = "http://127.0.0.1:1" // nothing there: the call that tells the server fails, and that is no matter
	a.acctSignOut()
	if a.settings.AccountUser != "" || a.settings.SyncToken != "" || a.settings.SyncKey != "" {
		t.Fatalf("still signed in: %+v", a.settings)
	}
}

// "Bağlantıyı dene": the state of the place, without changing it.
func TestSyncProbe(t *testing.T) {
	remote := t.TempDir()
	a := syncApp(t, remote)
	a.syncProbe()
	var got string
	for i := 0; i < 200; i++ {
		a.mu.Lock()
		busy := a.ac.probing
		got = a.ac.probe
		a.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(got, "Bağlantı tamam: 0 dosya") {
		t.Fatalf("probe of an empty place: %q", got)
	}
	a.settings.SyncMode = ""
	a.ac.probe = ""
	a.syncProbe()
	if !strings.Contains(a.ac.probe, "Olmadı") {
		t.Fatalf("probe with sync off: %q", a.ac.probe)
	}
}

// The versions the server kept, in the history dialog: choosing one and going back to it makes the page that, and keeps
// the page's own text as a copy first where the library keeps any.
func TestServerHistoryPutsBack(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "yeni metin\n")
	p := a.page
	if p == nil {
		t.Fatal("no page")
	}
	old := []byte(`{"meta":{"title":"` + p.Title + `"},"body":[]}`)
	h := &serverHist{vers: []Version{{ID: 1700000000000, At: time.UnixMilli(1700000000000), Size: len(old)}}, data: map[int64][]byte{1700000000000: old}}
	a.vhist = histView{open: true, page: p, vers: h.vers, opsFor: -1, src: h, server: true}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Sunucudaki sürümler: "+p.Title) || tt.HasText("Sakla") {
		t.Fatalf("the dialog of the server's versions: %v", tt.Texts())
	}
	if !tt.HasText("Bu sürüme dön") {
		t.Fatalf("no way back: %v", tt.Texts())
	}
	a.putBackVersion()
	if a.vhist.open {
		t.Fatal("the dialog stayed after going back")
	}
	if txt, _ := p.Text(); strings.Contains(txt, "yeni metin") {
		t.Fatalf("the page kept the new text: %q", txt)
	}
}
