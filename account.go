package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The account (cloudflare/sync/auth.js, docs/22-hesap-ve-bekleme-listesi.md): an email address, a password and, once, the
// access code the waitlist sent to that address (an account of before has a username where the address is). The
// password does not leave this computer. From it, and the address, two things are
// made here: "auth" (what the server takes as proof of who one is) and a key that seals the data key. The data
// key is made at random when the account is opened; the server keeps it sealed and cannot open it. It is the key
// of end-to-end encryption (sync.go): whoever has the password has it on any computer, and whoever has the server
// has nothing. Forget the password and the data is gone, unless the recovery key (the data key in words) was kept.

// defaultServer is where accounts are opened and the waitlist is.
const defaultServer = "https://celestine.halid.dev"

const acctIterations = 200_000

// acctKeys makes, from the name and the password, the proof for the server and the key that seals the data key.
func acctKeys(username, password string) (auth string, kek []byte, err error) {
	master, err := pbkdf2.Key(sha256.New, password, []byte("celestine/v1/"+strings.ToLower(strings.TrimSpace(username))), acctIterations, 32)
	if err != nil {
		return "", nil, err
	}
	part := func(label string) []byte {
		m := hmac.New(sha256.New, master)
		m.Write([]byte(label))
		return m.Sum(nil)
	}
	return hex.EncodeToString(part("auth")), part("kek"), nil
}

// recoveryAuth is what the recovery key proves to the server (/api/recover): made from the data key, so that it is the same wherever the key is,
// and one way, so that the server, holding only its hash, cannot make the key from it.
func recoveryAuth(key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("celestine/recovery-auth"))
	return hex.EncodeToString(m.Sum(nil))
}

func wrapDataKey(kek, key []byte) (string, error) {
	blk, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, key, nil)), nil
}

func unwrapDataKey(kek []byte, s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("anahtar bozuk")
	}
	blk, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	if len(raw) < g.NonceSize() {
		return nil, errors.New("anahtar bozuk")
	}
	key, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return nil, errors.New(locales.AccountErrDecrypt()) // the server said the proof was good, so this is a damaged blob
	}
	return key, nil
}

// An acctClient speaks to the account part of the server.
type acctClient struct {
	base string
	hc   *http.Client
}

func newAcctClient(base string) *acctClient {
	if strings.TrimSpace(base) == "" {
		base = defaultServer
	}
	return &acctClient{strings.TrimRight(base, "/"), &http.Client{Timeout: 30 * time.Second}}
}

// A DeviceInfo is a computer signed in to the account.
type DeviceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
	Seen    int64  `json:"seen"`
	Current bool   `json:"current"`
}

// AcctMe is what the server says of the account.
type AcctMe struct {
	Username string       `json:"username"`
	Bytes    int64        `json:"bytes"`
	Quota    int64        `json:"quota"`
	Devices  []DeviceInfo `json:"devices"`
}

func (c *acctClient) call(method, path, token string, in, out any) error {
	if err := HTTPSOnly(c.base); err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return errors.New(locales.AccountErrServer(strconv.Itoa(resp.StatusCode)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func deviceName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "cihaz"
}

// CheckInvite asks whether code is the access code sent to email, unused, and the address has no account yet; it spends nothing.
func (c *acctClient) CheckInvite(email, code string) error {
	return c.call("POST", "/api/invite", "", map[string]string{"email": acctName(email), "code": code}, nil)
}

// acctName is an address or a username as the server and the keys take it.
func acctName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Register opens an account for email with the access code sent to it. It makes the data key; the token is this computer's.
func (c *acctClient) Register(email, password, code string) (token string, key []byte, err error) {
	auth, kek, err := acctKeys(email, password)
	if err != nil {
		return "", nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", nil, err
	}
	wrapped, err := wrapDataKey(kek, key)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token string `json:"token"`
	}
	err = c.call("POST", "/api/register", "", map[string]string{
		"email": acctName(email), "auth": auth, "wrapped": wrapped, "code": code, "device": deviceName(), "recovery": recoveryAuth(key),
	}, &out)
	if err != nil {
		return "", nil, err
	}
	return out.Token, key, nil
}

// Login signs this computer in and brings the data key back, opened here.
func (c *acctClient) Login(username, password string) (token string, key []byte, err error) {
	auth, kek, err := acctKeys(username, password)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token   string `json:"token"`
		Wrapped string `json:"wrapped"`
	}
	if err := c.call("POST", "/api/login", "", map[string]string{
		"username": strings.ToLower(strings.TrimSpace(username)), "auth": auth, "device": deviceName(),
	}, &out); err != nil {
		return "", nil, err
	}
	key, err = unwrapDataKey(kek, out.Wrapped)
	if err == nil {
		// an account made before the recovery key could be used has none on the server: it is put there now, with the proof of the password.
		// The server takes it once; a failure here is not one of the sign-in.
		c.call("POST", "/api/recovery", out.Token, map[string]string{"auth": auth, "recovery": recoveryAuth(key)}, nil)
	}
	return out.Token, key, err
}

// Recover is for a forgotten password: the recovery key (the words that were shown when the account was made) says who one is, and a new
// password is set; the data key stays, so what is on the server is still readable. Every other computer is signed out.
func (c *acctClient) Recover(username, recovery, newPassword string) (token string, key []byte, err error) {
	key, err = KeyFromRecovery(recovery)
	if err != nil {
		return "", nil, err
	}
	newAuth, newKek, err := acctKeys(username, newPassword)
	if err != nil {
		return "", nil, err
	}
	wrapped, err := wrapDataKey(newKek, key)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token string `json:"token"`
	}
	err = c.call("POST", "/api/recover", "", map[string]string{
		"username": strings.ToLower(strings.TrimSpace(username)), "recovery": recoveryAuth(key), "newAuth": newAuth, "newWrapped": wrapped, "device": deviceName(),
	}, &out)
	if err != nil {
		return "", nil, err
	}
	return out.Token, key, nil
}

func (c *acctClient) Me(token string) (AcctMe, error) {
	var me AcctMe
	return me, c.call("GET", "/api/me", token, nil, &me)
}

func (c *acctClient) Logout(token string) error {
	return c.call("POST", "/api/logout", token, struct{}{}, nil)
}

func (c *acctClient) RemoveDevice(token, id string) error {
	return c.call("DELETE", "/api/devices/"+id, token, struct{}{}, nil)
}

// ChangePassword seals the data key anew under the new password; the server signs the other computers out.
func (c *acctClient) ChangePassword(token, username, oldPassword, newPassword string, key []byte) error {
	oldAuth, _, err := acctKeys(username, oldPassword)
	if err != nil {
		return err
	}
	newAuth, newKek, err := acctKeys(username, newPassword)
	if err != nil {
		return err
	}
	wrapped, err := wrapDataKey(newKek, key)
	if err != nil {
		return err
	}
	return c.call("POST", "/api/password", token, map[string]string{"auth": oldAuth, "newAuth": newAuth, "newWrapped": wrapped}, nil)
}

// DeleteAccount removes the account and what is on the server.
func (c *acctClient) DeleteAccount(token, username, password string) error {
	auth, _, err := acctKeys(username, password)
	if err != nil {
		return err
	}
	return c.call("DELETE", "/api/account", token, map[string]string{"auth": auth}, nil)
}

// ---- the app's side ----

type acctUI struct {
	mode     string // "invite" (the address and the code), "password" (then the password), "recovery" (the key, once), "login", "reset"
	user     string // the address (or an older account's username)
	pass     string
	pass2    string // the password again, when it is set
	code     string
	newPass  string
	delPass  string
	confirm  bool // the delete asks once more
	busy     bool
	err      string
	me       *AcctMe
	meAt     time.Time
	meBusy   bool
	showPass bool   // the password change is open
	recKey   string // the recovery key being typed, to set a new password
	showRec  bool   // the profile shows the recovery key
	probe    string // what the last try of the connection found
	probing  bool
}

func (a *app) acctServer() string {
	if a.settings.SyncURL != "" {
		return a.settings.SyncURL
	}
	return defaultServer
}

// acctDone keeps what a sign-in gave: the token, the key, and that sync is on, then syncs.
func (a *app) acctDone(user, token string, key []byte, fresh bool) {
	s := &a.settings
	s.AccountUser, s.SyncMode, s.SyncURL, s.SyncToken = user, "server", a.acctServer(), token
	s.SyncKey = base64.StdEncoding.EncodeToString(key)
	a.sy.hasKey = true
	if fresh {
		a.sy.recovery = RecoveryKey(key)
	}
	a.ac = acctUI{mode: "login"}
	if fresh { // the recovery key is shown before anything else: it is the only way back to the data
		a.ac.mode = "recovery"
	}
	a.saveSettings()
	a.sy.tried = time.Time{}
	a.say(toastOK, locales.TheAccountIsReady())
	a.syncNow()
}

// acctForget drops what ties this computer to the account; the library stays as it is.
func (a *app) acctForget() {
	s := &a.settings
	s.AccountUser, s.SyncToken, s.SyncKey = "", "", ""
	a.sy.hasKey, a.sy.recovery, a.sy.status = false, "", ""
	a.ac = acctUI{mode: "login"}
	a.saveSettings()
}

// acctRun does a call to the server away from the window, and then what must be done in it.
func (a *app) acctRun(work func(cl *acctClient) error, then func()) {
	if a.ac.busy {
		return
	}
	a.ac.busy, a.ac.err = true, ""
	cl := newAcctClient(a.acctServer())
	go func() {
		defer a.guard()
		err := work(cl)
		a.do(func() error {
			a.ac.busy = false
			if err != nil {
				a.ac.err = err.Error()
			} else if then != nil {
				then()
			}
			return nil
		})
	}()
}

func (a *app) acctRefresh() {
	if a.ac.meBusy || a.settings.SyncToken == "" || time.Since(a.ac.meAt) < time.Minute {
		return
	}
	a.ac.meBusy = true
	cl, token := newAcctClient(a.acctServer()), a.settings.SyncToken
	go func() {
		defer a.guard()
		me, err := cl.Me(token)
		a.do(func() error {
			a.ac.meBusy, a.ac.meAt = false, time.Now()
			if err == nil {
				a.ac.me = &me
			} else if strings.Contains(err.Error(), "oturum yok") { // the server no longer knows this computer
				a.acctForget()
				a.ac.err = "Oturum kapanmış: yeniden giriş yap."
			}
			return nil
		})
	}()
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
}

// acctForm is the form of getting in. One with an invite opens the account in steps: the address and the access code that was sent
// to it (checked, not spent), then the password, then the recovery key, shown once. One with an account signs in with the address and
// the password; one who forgot the password sets a new one with the recovery key. The sync settings and the profile of one not signed
// in show it; a first sight of it is the invite's, as most who come to it have one in the mail.
func (a *app) acctForm(c *ui.Context) {
	cl := ryugo.From(c).Colors
	if a.ac.mode == "" {
		a.ac.mode = "invite"
	}
	btn := func(key, label string, opts ryugo.ButtonOptions) bool {
		return ryugo.Button(c.Key(key), label, opts).Clicked()
	}
	entered := false // Enter in any of the fields is the form's button
	field := func(key, label string, v *string, opts ...func(*ryugo.InputOptions)) {
		ryugo.Field(c, ryugo.FieldOptions{Label: T(label)}, func() {
			o := ryugo.InputOptions{Label: T(label), Size: ryugo.Sm}
			for _, f := range opts {
				f(&o)
			}
			if a.fieldMenu(ryugo.Input(c.Key(key), v, o), v, fieldOpts{oneLine: true, secret: o.Password}).Submitted() {
				entered = true
			}
		})
	}
	secret := func(o *ryugo.InputOptions) { o.Password = true }
	note := func(s string) { ui.Text(c, T(s)).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap() }
	mode := a.ac.mode
	if mode == "recovery" {
		a.acctRecoveryStep(c)
		return
	}
	ui.Column(c).Gap(10).FillWidth().Children(func() {
		switch mode {
		case "invite", "login":
			a.acctWays(c, mode)
		case "password":
			note(locales.AccountInviteConfirmed(acctName(a.ac.user)))
		case "reset":
			note("Şifreni unuttun mu? Hesabı açarken gösterilen kurtarma anahtarıyla yeni bir şifre koyarsın; verin aynı kalır, diğer bilgisayarlarda yeniden giriş yapman gerekir.")
		}
		switch mode {
		case "invite":
			note("Bekleme listesinden onaylandıysan e-postana bir erişim kodu geldi.")
			field("acct-user", "E-posta", &a.ac.user, func(o *ryugo.InputOptions) { o.Placeholder = locales.AccountNameExampleCom() })
			field("acct-code", "Erişim kodu", &a.ac.code, func(o *ryugo.InputOptions) { o.Placeholder = "RYU-XXXXXX" })
		case "password":
			field("acct-pass", "Şifre", &a.ac.pass, secret)
			field("acct-pass2", "Şifre (tekrar)", &a.ac.pass2, secret)
			note("Şifren bu bilgisayardan çıkmaz; unutursan verini kimse geri getiremez. Hesabı açınca bir kurtarma anahtarı göstereceğiz: sakla.")
		case "reset":
			field("acct-user", "E-posta", &a.ac.user)
			field("acct-rec", "Kurtarma anahtarı", &a.ac.recKey, func(o *ryugo.InputOptions) { o.Placeholder = "XXXX-XXXX-…" })
			field("acct-pw-new", "Yeni şifre", &a.ac.newPass, secret)
		default:
			field("acct-user", "E-posta", &a.ac.user)
			field("acct-pass", "Şifre", &a.ac.pass, secret)
		}
		if a.ac.err != "" {
			ui.Text(c, T(a.ac.err)).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			label := map[string]string{"invite": "Devam", "password": "Hesabı aç", "login": "Giriş yap", "reset": "Şifreyi yenile"}[mode]
			if a.ac.busy {
				label = "…"
			}
			if (btn("acct-go", T(label), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}) || entered) && !a.ac.busy {
				a.acctSubmit()
			}
			switch mode {
			case "login":
				if btn("acct-forgot", locales.AccountIForgotMyPassword(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					a.ac.mode, a.ac.err = "reset", ""
				}
			case "invite":
				if btn("acct-wait", locales.AccountNoCodeJoinWaitlist(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					c.OpenURL(strings.TrimSuffix(a.acctServer(), "/") + "/#waitlist")
				}
			case "password":
				if btn("acct-back", locales.CommonBack(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					a.ac.mode, a.ac.err, a.ac.pass, a.ac.pass2 = "invite", "", "", ""
				}
			case "reset":
				if btn("acct-back", locales.AccountSign(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					a.ac.mode, a.ac.err = "login", ""
				}
			}
		})
	})
}

// acctWays are the two ways into an account, as Ryugo's segmented control (#286): opening one with an invite, or
// signing in.
func (a *app) acctWays(c *ui.Context, mode string) {
	ways := []string{"invite", "login"}
	sel := slices.Index(ways, mode)
	// its track a step deeper than the well: the settings' group it is in is the well's color, and the track would
	// not show on it
	ryugo.Segmented(c.Key("acct-ways"), &sel, []string{locales.AccountOpenAccountWithInvite(), locales.AccountSign()}, ryugo.SegmentedOptions{Label: locales.AccountAccount(), Size: ryugo.Sm}).
		Background(ryugo.From(c).Colors.WellStrong)
	if ways[sel] != mode {
		a.ac.mode, a.ac.err = ways[sel], ""
	}
}

// acctRecoveryStep is the last step of opening an account: the recovery key, which is the only way back to the data without the
// password. The form goes on only when one says it is kept.
func (a *app) acctRecoveryStep(c *ui.Context) {
	cl := ryugo.From(c).Colors
	ui.Column(c).Gap(10).FillWidth().Children(func() {
		ui.Text(c, locales.AccountYourAccountOpenWrite()).FontSize(13).TextColor(cl.Text2).Wrap()
		ui.Text(c, a.sy.recovery).FontSize(15).FontWeight(600).TextColor(cl.Text).Selectable().Wrap()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ryugo.CopyButton(c.Key("copy-recovery-step"), a.sy.recovery, ryugo.CopyOptions{Label: locales.CommonCopyRecoveryKey()})
			if ryugo.Button(c.Key("acct-kept"), locales.AccountIKeptContinue(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
				a.ac.mode = "login"
			}
		})
	})
}

// acctSubmit does what the form asks, in the mode it is in.
func (a *app) acctSubmit() {
	user, pass, code, rec, nw, mode := strings.TrimSpace(a.ac.user), a.ac.pass, strings.TrimSpace(a.ac.code), a.ac.recKey, a.ac.newPass, a.ac.mode
	var token string
	var key []byte
	switch {
	case user == "":
		a.ac.err = "E-posta gerekli."
	case mode == "invite" && !strings.Contains(user, "@"):
		a.ac.err = "E-posta adresi geçersiz."
	case mode == "invite" && code == "":
		a.ac.err = "Erişim kodu gerekli."
	case mode == "invite":
		a.acctRun(func(cl *acctClient) error { return cl.CheckInvite(user, code) }, func() {
			a.ac.mode, a.ac.err, a.ac.pass, a.ac.pass2 = "password", "", "", ""
		})
	case mode == "password" && len(pass) < 8:
		a.ac.err = "Şifre en az 8 karakter olsun."
	case mode == "password" && pass != a.ac.pass2:
		a.ac.err = "Şifreler aynı değil."
	case mode == "reset" && strings.TrimSpace(rec) == "":
		a.ac.err = "Kurtarma anahtarı gerekli."
	case mode == "reset" && len(nw) < 8:
		a.ac.err = "Yeni şifre en az 8 karakter olsun."
	case mode == "login" && pass == "":
		a.ac.err = "E-posta ve şifre gerekli."
	default:
		a.acctRun(func(cl *acctClient) (err error) {
			switch mode {
			case "password":
				token, key, err = cl.Register(user, pass, code)
			case "reset":
				token, key, err = cl.Recover(user, rec, nw)
			default:
				token, key, err = cl.Login(user, pass)
			}
			return
		}, func() { a.acctDone(acctName(user), token, key, mode == "password") })
	}
}

// acctManage is what one signed in does with the account: the password, the computers signed in, deleting it.
func (a *app) acctManage(c *ui.Context) {
	cl := ryugo.From(c).Colors
	s := &a.settings
	btn := func(key, label string, opts ryugo.ButtonOptions) bool {
		return ryugo.Button(c.Key(key), label, opts).Clicked()
	}
	errLine := func() {
		if a.ac.err != "" {
			ui.Text(c, T(a.ac.err)).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
	}
	settingGroup(c, "set-acct-me",
		func() {
			settingRow(c, locales.AccountChangePassword(), locales.AccountYouSignAgainYour(), func() {
				if btn("acct-pw-open", map[bool]string{true: locales.CommonCancel(), false: locales.CommonChange()}[a.ac.showPass], ryugo.ButtonOptions{Size: ryugo.Sm}) {
					a.ac.showPass, a.ac.err = !a.ac.showPass, ""
				}
			})
			if a.ac.showPass {
				ui.Column(c).Gap(8).FillWidth().Children(func() {
					a.fieldMenu(ryugo.Input(c.Key("acct-pw-old"), &a.ac.pass, ryugo.InputOptions{Label: locales.AccountCurrentPassword(), Size: ryugo.Sm, Password: true, Placeholder: locales.AccountCurrentPassword()}), &a.ac.pass, fieldOpts{oneLine: true, secret: true})
					a.fieldMenu(ryugo.Input(c.Key("acct-pw-new"), &a.ac.newPass, ryugo.InputOptions{Label: locales.AccountNewPassword(), Size: ryugo.Sm, Password: true, Placeholder: locales.AccountNewPassword()}), &a.ac.newPass, fieldOpts{oneLine: true, secret: true})
					errLine()
					if btn("acct-pw-go", locales.AccountChangePassword(), ryugo.ButtonOptions{Size: ryugo.Sm}) && !a.ac.busy {
						old, nw := a.ac.pass, a.ac.newPass
						key, err := base64.StdEncoding.DecodeString(s.SyncKey)
						switch {
						case err != nil || len(key) != 32:
							a.ac.err = "Şifreleme anahtarı bu bilgisayarda yok: çıkış yapıp yeniden gir."
						case len(nw) < 8:
							a.ac.err = "Yeni şifre en az 8 karakter olsun."
						default:
							a.acctRun(func(cl *acctClient) error { return cl.ChangePassword(s.SyncToken, s.AccountUser, old, nw, key) },
								func() {
									a.ac.showPass, a.ac.pass, a.ac.newPass = false, "", ""
									a.ac.meAt = time.Time{}
									a.say(toastOK, locales.PasswordChanged())
								})
						}
					}
				})
			}
		},
		func() {
			if a.ac.me == nil || len(a.ac.me.Devices) == 0 {
				ui.Text(c, locales.AccountLoadingDevices()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				return
			}
			ui.Column(c).Gap(8).FillWidth().Children(func() {
				ui.Text(c, locales.AccountComputersSignedThisAccount()).FontSize(15).FontWeight(620).TextColor(cl.Text)
				for _, d := range a.ac.me.Devices {
					d := d
					ui.Row(c.Key("acct-dev:" + d.ID)).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
						name := d.Name
						if d.Current {
							name = locales.AccountDeviceThis(name)
						}
						ui.Text(c, name).FontSize(13).TextColor(cl.Text).Grow(1)
						ui.Text(c, time.Unix(d.Seen, 0).Format("2006-01-02 15:04")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
						if !d.Current && btn("acct-devrm:"+d.ID, locales.AccountRemove(), ryugo.ButtonOptions{Size: ryugo.Sm}) {
							cl, token := newAcctClient(a.acctServer()), s.SyncToken
							a.acctRun(func(*acctClient) error { return cl.RemoveDevice(token, d.ID) }, func() { a.ac.meAt = time.Time{} })
						}
					})
				}
			})
		},
		func() {
			settingRow(c, locales.AccountDeleteAccount(), locales.AccountTheAccountEveryFile(), func() {
				if btn("acct-del-open", map[bool]string{true: locales.CommonCancel(), false: locales.AccountDelete()}[a.ac.confirm], ryugo.ButtonOptions{Size: ryugo.Sm}) {
					a.ac.confirm, a.ac.err = !a.ac.confirm, ""
				}
			})
			if a.ac.confirm {
				ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
					a.fieldMenu(ryugo.Input(c.Key("acct-del-pass"), &a.ac.delPass, ryugo.InputOptions{Label: locales.AccountPassword(), Size: ryugo.Sm, Password: true, Placeholder: locales.AccountTypeYourPassword()}), &a.ac.delPass, fieldOpts{oneLine: true, secret: true})
					if btn("acct-del-go", locales.AccountDeleteAccountForGood(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Up}) && !a.ac.busy {
						pw := a.ac.delPass
						a.acctRun(func(cl *acctClient) error { return cl.DeleteAccount(s.SyncToken, s.AccountUser, pw) },
							func() { a.acctForget(); a.say(toastOK, locales.TheAccountWasDeleted()) })
					}
				})
				errLine()
			}
		})
}

// acctSignOut tells the server and forgets the account here.
func (a *app) acctSignOut() {
	cl, token := newAcctClient(a.acctServer()), a.settings.SyncToken
	go func() { defer a.guard(); cl.Logout(token) }()
	a.acctForget()
}

// accountSection is the part of the sync settings for the account: the form when there is none; when there is, who it is and the way
// to the profile (profile.go), where it is looked after.
func (a *app) accountSection(c *ui.Context) {
	s := &a.settings
	if s.AccountUser == "" || s.SyncToken == "" || a.ac.mode == "recovery" { // a new account's key, before anything else
		settingGroup(c, "set-acct-in", func() { a.acctForm(c) })
		return
	}
	a.acctRefresh()
	settingGroup(c, "set-acct-me0", func() {
		help := ""
		if a.ac.me != nil {
			help = humanBytes(a.ac.me.Bytes) + " / " + humanBytes(a.ac.me.Quota)
		}
		settingRow(c, locales.AccountAccountOf(s.AccountUser), help, func() {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				if ryugo.Button(c.Key("acct-profile"), locales.CommonProfile(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconUser}).Clicked() {
					a.openProfile()
				}
				if ryugo.Button(c.Key("acct-out"), locales.AccountSignOut(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconSignOut, Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft}).Clicked() {
					a.acctSignOut()
				}
			})
		})
	})
}
