package main

import (
	"encoding/json"
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
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
	if tt.HasText("Erişim kodu") || !tt.HasText("Kenar çubuğu (⌘\\)") { // the app itself, its sidebar; the corner tour it once had is gone (#488)
		t.Fatalf("the app does not open without an account: %v", tt.Texts())
	}
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	for _, want := range []string{"Davetle hesap aç", "Giriş yap", "E-posta", "Erişim kodu", locales.AccountNoCodeJoinWaitlist()} {
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
	check("login", "", "x", "", "", "", "anEmailAddressIsNeeded")
	check("login", "ali@ornek.com", "", "", "", "", "anEmailAddressAndAPassword")
	check("invite", "ali", "", "RYU-AAAAAA", "", "", "thatEmailAddressIsntValid")
	check("invite", "ali@ornek.com", "", " ", "", "", "anAccessCodeIsNeeded")
	check("password", "ali@ornek.com", "kısa", "RYU-AAAAAA", "", "", "thePasswordNeedsAtLeast8")
	check("reset", "ali@ornek.com", "", "", "", "yeni-parola", "theRecoveryKeyIsNeeded")
	check("reset", "ali@ornek.com", "", "", "AAAA-BBBB", "kısa", "theNewPasswordNeedsAtLeast")
	a.ac = acctUI{mode: "password", user: "ali@ornek.com", pass: "uzun-parola", pass2: "uzun-parolA", code: "RYU-AAAAAA"}
	a.acctSubmit()
	if a.ac.err != "thePasswordsAreNotTheSame" || a.ac.busy {
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
	var copiedSecret string
	var copiedOpts mygo.ClipboardOptions
	a.clipWrite = func(s string, opts mygo.ClipboardOptions) error {
		copiedSecret, copiedOpts = s, opts
		return nil
	}
	tt.Click(locales.CommonCopyRecoveryKey())
	tt.Frame()
	if copiedSecret != a.sy.recovery || !copiedOpts.Concealed {
		t.Fatalf("recovery key not copied as concealed secret: got %q concealed=%v want %q", copiedSecret, copiedOpts.Concealed, a.sy.recovery)
	}
	tt.Advance(20 * time.Millisecond)
	if !tt.HasText(locales.CommonCopied()) {
		// Icon-only button exposes its label to accessibility tree; verify Find works.
		if _, ok := tt.Find(locales.CommonCopied()); !ok {
			t.Fatalf("expected secret copy button label %q during copied feedback window", locales.CommonCopied())
		}
	}
	tt.Click(locales.AccountIKeptContinue())
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if a.ac.mode != "login" || tt.HasText(locales.AccountIKeptContinue()) {
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
	for _, want := range []string{"Profil", locales.AccountNotSignedYourLibrary(), "Giriş yap", "Davetle hesap aç", "E-posta"} {
		if !tt.HasText(want) {
			t.Errorf("signed out: no %q: %v", want, tt.Texts())
		}
	}

	a.settings.AccountUser, a.settings.SyncToken, a.ac.meAt = "ali", "tok", time.Now() // no call to a server
	// nor a pass toward sync.example.com: its failure comes when the name fails to resolve, at once here, late on a
	// slow runner, and its long words then move the screen under a click on its way, which is lost (the release's
	// test run). A pass tried just now waits minutes for the next.
	a.sy.tried = a.clock()
	a.settings.SyncMode, a.settings.SyncURL = "server", "https://sync.example.com"
	a.settings.SyncKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	a.ac.me = &AcctMe{Username: "ali", Bytes: 5 << 20, Quota: 200 << 20, Devices: []DeviceInfo{{ID: "aaaaaaaaaaaa", Name: "MacBook", Seen: 1, Current: true}, {ID: "bbbbbbbbbbbb", Name: "Telefon", Seen: 1}}}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"ali", "sync.example.com", "Kullanılan yer", "5.0 MB / 200.0 MB", "Şimdi eşitle", "Bağlantıyı dene", "Kurtarma anahtarı", "Şifreyi değiştir", "Bu bilgisayarın adı", "Telefon", "Hesabı sil", "Çıkış yap"} {
		if !tt.HasText(want) {
			t.Errorf("signed in: no %q: %v", want, tt.Texts())
		}
	}
	if a.ac.devName != "MacBook" { // this computer's name, in its field (#161)
		t.Errorf("this computer's field: %q", a.ac.devName)
	}
	if tt.HasText("Giriş yapılmadı") {
		t.Error("signed in, and it says it is not")
	}
	// the recovery key is not shown until it is asked for
	rec := RecoveryKey(make([]byte, 32))
	if tt.HasText(rec) {
		t.Fatal("the recovery key is on the screen unasked")
	}
	if err := tt.Click("Göster"); err != nil {
		t.Fatalf("the recovery key's Show cannot be clicked: %v", err)
	}
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
	if !strings.Contains(got, "Bağlantı çalışıyor: 0 dosya") {
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

// TestSecretCopyNoUnconcealedFallback checks that failed concealed clipboard writes toast an error without
// falling back to plain text, and that the Settings MCP secret row copies with Concealed=true (#343).
func TestSecretCopyNoUnconcealedFallback(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "x\n")
	plainCalled := false
	a.clipText = func(string) { plainCalled = true }
	a.clipWrite = func(string, mygo.ClipboardOptions) error {
		return errors.New("clipboard locked")
	}
	if a.copySecret("top-secret") {
		t.Fatal("expected copySecret to return false on error")
	}
	if plainCalled {
		t.Fatal("copySecret fell back to plain clipText on failure")
	}
	if a.noticeTone == nil || *a.noticeTone != toastErr || !strings.Contains(a.notice, "clipboard locked") {
		t.Fatalf("expected error toast on failed concealed write, got tone=%v notice=%q", a.noticeTone, a.notice)
	}

	// Concealed MCP copy row in Settings copies via clipWrite with Concealed=true.
	var gotSecret string
	var gotOpts mygo.ClipboardOptions
	a.clipWrite = func(s string, opts mygo.ClipboardOptions) error {
		gotSecret, gotOpts = s, opts
		return nil
	}
	a.settings.Port = 0
	a.setMCP(true)
	defer a.setMCP(false)
	if a.mcpServer == nil {
		t.Fatalf("mcpServer did not start: %s", a.mcpErr)
	}
	tt.SetSize(1180, 760)
	a.openSettings()
	a.settingsTab = int(tabMCP)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()

	lbl := locales.CommonCopyLabel(msg("toAddItToClaudeCode"))
	if err := tt.Click(lbl); err != nil {
		t.Fatalf("click %q: %v (mcp=%v texts=%v)", lbl, err, tt.HasText(a.mcpCommand()), tt.Texts())
	}
	tt.Advance(20 * time.Millisecond)
	if gotSecret != a.mcpCommand() || !gotOpts.Concealed {
		t.Fatalf("MCP command copy: got %q concealed=%v (want %q, concealed=true)", gotSecret, gotOpts.Concealed, a.mcpCommand())
	}
	if _, ok := tt.Find(locales.CommonCopied()); !ok {
		t.Fatalf("expected %q feedback label after copying MCP command", locales.CommonCopied())
	}
	tt.Advance(1600 * time.Millisecond)
	if _, ok := tt.Find(lbl); !ok {
		t.Fatalf("expected secret copy button label to revert to %q after 1.5s", lbl)
	}

	// Attempting to drag-select + Cmd+C the MCP command must not leak to plain clipboard.
	tt.SetClipboard("")
	cmdR, ok := tt.Find(a.mcpCommand())
	if !ok {
		t.Fatalf("MCP command text %q not found", a.mcpCommand())
	}
	tt.Press(cmdR.X+4, cmdR.Y+cmdR.H/2)
	tt.Move(cmdR.X+cmdR.W-4, cmdR.Y+cmdR.H/2)
	tt.Release(cmdR.X+cmdR.W-4, cmdR.Y+cmdR.H/2)
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != "" {
		t.Fatalf("MCP command allowed unconcealed Cmd+C copy: %q", got)
	}

	// Right-click -> Copy on the MCP command row must copy via Concealed=true.
	gotSecret = ""
	gotOpts = mygo.ClipboardOptions{}
	if err := tt.RightClick(a.mcpCommand()); err != nil {
		t.Fatalf("right-click MCP command: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.CommonCopy()); err != nil {
		t.Fatalf("click context menu Copy: %v", err)
	}
	tt.Advance(20 * time.Millisecond)
	if gotSecret != a.mcpCommand() || !gotOpts.Concealed {
		t.Fatalf("MCP context menu copy: got %q concealed=%v (want %q, concealed=true)", gotSecret, gotOpts.Concealed, a.mcpCommand())
	}
}

// TestSecretCopyButtonMatchesRyugoCopyButton verifies that secretCopyButton matches
// ryugo.CopyButton's round geometry, check-draw animation, and settled pixels (#343).
func TestSecretCopyButtonMatchesRyugoCopyButton(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.clipWrite = func(string, mygo.ClipboardOptions) error { return nil }

	for _, dark := range []bool{false, true} {
		mode := ryugo.Light
		if dark {
			mode = ryugo.Dark
		}
		mk := func(secret bool) (*ui.Tester, ui.Rect) {
			tt := ui.NewTester(func(c *ui.Context) {
				ryugo.Root(c, ryugo.Options{Mode: mode})
				ui.Column(c).Padding(24).Children(func() {
					if secret {
						a.secretCopyButton(c, "btn", "top-secret", "Copy secret")
					} else {
						ryugo.CopyButton(c.Key("btn"), "top-secret", ryugo.CopyOptions{Label: "Copy secret"})
					}
				})
			}, 120, 120)
			tt.SetDark(dark)
			tt.Advance(100 * time.Millisecond)
			r, ok := tt.Find("Copy secret")
			if !ok {
				t.Fatalf("dark=%v secret=%v button not found", dark, secret)
			}
			return tt, r
		}

		ttRef, rRef := mk(false)
		ttSec, rSec := mk(true)
		if rRef != rSec || rSec.W != rSec.H {
			t.Fatalf("dark=%v bounds mismatch: ref=%+v sec=%+v", dark, rRef, rSec)
		}
		box := image.Rect(int(rRef.X)-6, int(rRef.Y)-6, int(rRef.X+rRef.W)+6, int(rRef.Y+rRef.H)+6)
		snap := func(tt *ui.Tester) *image.RGBA {
			src := tt.Image()
			dst := image.NewRGBA(src.Rect)
			copy(dst.Pix, src.Pix)
			return dst
		}
		refIdle := snap(ttRef)
		secIdle := snap(ttSec)
		if d := pixelsDiffer(refIdle, secIdle, box, image.Rectangle{}); d != 0 {
			t.Fatalf("dark=%v idle secretCopyButton differs from ryugo.CopyButton by %d px", dark, d)
		}

		if err := ttRef.Click("Copy secret"); err != nil {
			t.Fatal(err)
		}
		if err := ttSec.Click("Copy secret"); err != nil {
			t.Fatal(err)
		}
		// Mid-animation (~90ms): both icon shrink and check-draw are visibly in progress.
		ttRef.Advance(90 * time.Millisecond)
		ttSec.Advance(90 * time.Millisecond)
		refMid := snap(ttRef)
		secMid := snap(ttSec)
		if d := pixelsDiffer(secIdle, secMid, box, image.Rectangle{}); d < 20 {
			t.Fatalf("dark=%v secretCopyButton mid-animation changed only %d px from idle", dark, d)
		}
		if d := pixelsDiffer(refMid, secMid, box, image.Rectangle{}); d != 0 {
			t.Fatalf("dark=%v mid-animation secretCopyButton differs from ryugo.CopyButton by %d bytes", dark, d)
		}
		// After animations settle (~350ms): check stroke matches ryugo.CopyButton cleanly.
		ttRef.Advance(260 * time.Millisecond)
		ttSec.Advance(260 * time.Millisecond)
		refDone := snap(ttRef)
		secDone := snap(ttSec)
		if d := pixelsDiffer(secIdle, secDone, box, image.Rectangle{}); d < 20 {
			t.Fatalf("dark=%v secretCopyButton copied state changed only %d bytes from idle", dark, d)
		}
		if d := pixelsDiffer(refDone, secDone, box, image.Rectangle{}); d != 0 {
			t.Fatalf("dark=%v copied secretCopyButton differs from ryugo.CopyButton by %d bytes", dark, d)
		}

		// After 1.5s hold + return animation + moving pointer away, reverts cleanly to idle copy icon.
		ttRef.Move(0, 0)
		ttSec.Move(0, 0)
		ttRef.Advance(1400 * time.Millisecond)
		ttSec.Advance(1400 * time.Millisecond)
		if d := pixelsDiffer(secIdle, snap(ttSec), box, image.Rectangle{}); d != 0 {
			t.Fatalf("dark=%v reverted secretCopyButton differs from idle by %d bytes", dark, d)
		}
		if d := pixelsDiffer(snap(ttRef), snap(ttSec), box, image.Rectangle{}); d != 0 {
			t.Fatalf("dark=%v reverted secretCopyButton differs from ryugo.CopyButton by %d bytes", dark, d)
		}
	}
}
