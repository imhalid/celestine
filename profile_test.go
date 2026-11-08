package main

import (
	"strings"
	"testing"
	"time"
)

// The first time a real library is opened (it has settings of its own) the dialog of getting in shows; closing it is "no account for now",
// and it does not come again; the tour, which waits behind it, comes after.
func TestLoginDialogFirstRun(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "x\n")
	a.settingsFile = t.TempDir() + "/s.json"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Ryudit'e hoş geldin", "Hesapsız devam et", "Giriş yap", "Hesap aç", "Şifremi unuttum"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q on the dialog: %v", want, tt.Texts())
		}
	}
	if tt.HasText("Bir daha gösterme") {
		t.Fatal("the tour shows over the dialog")
	}
	tt.Click("Hesapsız devam et")
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if tt.HasText("Ryudit'e hoş geldin") || !a.settings.AccountAsked {
		t.Fatalf("the dialog did not close, or is not answered: %v", tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Bir daha gösterme") {
		t.Fatalf("the tour does not come after: %v", tt.Texts())
	}
	// not again, in this run or the next
	if a.loginDue() {
		t.Fatal("still due")
	}
	b, _ := newTestApp(t, "x\n")
	b.settingsFile = t.TempDir() + "/s.json"
	b.settings.AccountAsked = true
	if b.loginDue() {
		t.Fatal("due after it was answered")
	}
	c, _ := newTestApp(t, "x\n")
	c.settingsFile = t.TempDir() + "/s.json"
	c.settings.AccountUser = "ali"
	if c.loginDue() {
		t.Fatal("due for one who is signed in")
	}
}

// A library with no settings of its own (a test's, a scratch one) is not asked.
func TestLoginDialogNotInScratchLibrary(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Frame()
	if a.login.open || tt.HasText("Hesapsız devam et") {
		t.Fatal("the dialog shows in a library with no settings")
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
	check("login", "", "x", "", "", "", "Kullanıcı adı gerekli.")
	check("login", "ali", "", "", "", "", "Kullanıcı adı ve şifre gerekli.")
	check("register", "ali", "kısa", "RYU-AAAAAA", "", "", "Şifre en az 8 karakter olsun.")
	check("register", "ali", "uzun-parola", " ", "", "", "Davet kodu gerekli.")
	check("reset", "ali", "", "", "", "yeni-parola", "Kurtarma anahtarı gerekli.")
	check("reset", "ali", "", "", "AAAA-BBBB", "kısa", "Yeni şifre en az 8 karakter olsun.")
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
	for _, want := range []string{"Profil", "Giriş yapılmadı: kitaplığın yalnız bu bilgisayarda.", "Giriş yap", "Hesap aç", "Kullanıcı adı"} {
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
	rec := recoveryKey(make([]byte, 32))
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
