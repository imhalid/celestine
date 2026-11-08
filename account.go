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
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The account (cloudflare/sync/auth.js, docs/22-hesap-ve-bekleme-listesi.md): a name, a password and, once, an
// invite code from the waitlist. The password does not leave this computer. From it, and the name, two things are
// made here: "auth" (what the server takes as proof of who one is) and a key that seals the data key. The data
// key is made at random when the account is opened; the server keeps it sealed and cannot open it. It is the key
// of end-to-end encryption (sync.go): whoever has the password has it on any computer, and whoever has the server
// has nothing. Forget the password and the data is gone, unless the recovery key (the data key in words) was kept.

// defaultServer is where accounts are opened and the waitlist is.
const defaultServer = "https://ryudit.halid.dev"

const acctIterations = 200_000

// acctKeys makes, from the name and the password, the proof for the server and the key that seals the data key.
func acctKeys(username, password string) (auth string, kek []byte, err error) {
	master, err := pbkdf2.Key(sha256.New, password, []byte("ryudit/v1/"+strings.ToLower(strings.TrimSpace(username))), acctIterations, 32)
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
	m.Write([]byte("ryudit/recovery-auth"))
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
		return nil, errors.New("şifre çözülemedi") // the server said the proof was good, so this is a damaged blob
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
		return fmt.Errorf("sunucu %d döndü", resp.StatusCode)
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

// Register opens an account with an invite code. It makes the data key; the token is this computer's.
func (c *acctClient) Register(username, password, code string) (token string, key []byte, err error) {
	auth, kek, err := acctKeys(username, password)
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
		"username": strings.ToLower(strings.TrimSpace(username)), "auth": auth, "wrapped": wrapped, "code": code, "device": deviceName(), "recovery": recoveryAuth(key),
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
	mode     string // "login" or "register"
	user     string
	pass     string
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
	s.AccountAsked, a.login.open = true, false
	s.SyncKey = base64.StdEncoding.EncodeToString(key)
	a.sy.hasKey = true
	if fresh {
		a.sy.recovery = RecoveryKey(key)
	}
	a.ac = acctUI{mode: "login"}
	a.saveSettings()
	a.sy.tried = time.Time{}
	a.notice = "Hesap hazır."
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

// acctForm is the form of getting in: signing in, opening an account, and, for one who forgot the password, setting a new one with the
// recovery key. The screen of signing in (profile.go), the profile of one not signed in and the sync settings all show it.
func (a *app) acctForm(c *ui.Context) {
	cl := ryugo.From(c).Colors
	if a.ac.mode == "" {
		a.ac.mode = "login"
	}
	btn := func(key, label string, opts ryugo.ButtonOptions) bool {
		return ryugo.Button(c.Key(key), label, opts).Clicked()
	}
	field := func(key, label string, v *string, opts ...func(*ryugo.InputOptions)) {
		ryugo.Field(c, ryugo.FieldOptions{Label: T(label)}, func() {
			o := ryugo.InputOptions{Label: T(label), Size: ryugo.Sm}
			for _, f := range opts {
				f(&o)
			}
			a.fieldMenu(ryugo.Input(c.Key(key), v, o), v, fieldOpts{oneLine: true, secret: o.Password})
		})
	}
	secret := func(o *ryugo.InputOptions) { o.Password = true }
	mode := a.ac.mode
	ui.Column(c).Gap(10).FillWidth().Children(func() {
		if mode != "reset" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				for _, m := range []struct{ id, name string }{{"login", "Giriş yap"}, {"register", "Hesap aç"}} {
					m := m
					b := ui.Row(c.Key("acct-mode:"+m.id)).Padding(6, 14).Radius(rad(8)).Background(map[bool]ui.Color{true: cl.Accent.Alpha(0.2), false: cl.Fill}[mode == m.id]).
						Label(T(m.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					b.Children(func() { ui.Text(c, T(m.name)).FontSize(13).TextColor(cl.Text) })
					if b.Clicked() {
						a.ac.mode, a.ac.err = m.id, ""
					}
				}
			})
		} else {
			ui.Text(c, T("Şifreni unuttun mu? Hesabı açarken gösterilen kurtarma anahtarıyla yeni bir şifre koyarsın; verin aynı kalır, diğer bilgisayarlarda yeniden giriş yapman gerekir.")).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap()
		}
		field("acct-user", "Kullanıcı adı", &a.ac.user)
		switch mode {
		case "reset":
			field("acct-rec", "Kurtarma anahtarı", &a.ac.recKey, func(o *ryugo.InputOptions) { o.Placeholder = "XXXX-XXXX-…" })
			field("acct-pw-new", "Yeni şifre", &a.ac.newPass, secret)
		default:
			field("acct-pass", "Şifre", &a.ac.pass, secret)
		}
		if mode == "register" {
			field("acct-code", "Davet kodu", &a.ac.code, func(o *ryugo.InputOptions) { o.Placeholder = "RYU-XXXXXX" })
			ui.Text(c, T("Şifren bu bilgisayardan çıkmaz; unutursan verini kimse geri getiremez. Hesabı açınca bir kurtarma anahtarı göstereceğiz: sakla.")).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap()
		}
		if a.ac.err != "" {
			ui.Text(c, T(a.ac.err)).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			label := map[string]string{"login": "Giriş yap", "register": "Hesabı aç", "reset": "Şifreyi yenile"}[mode]
			if a.ac.busy {
				label = "…"
			}
			if btn("acct-go", T(label), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}) && !a.ac.busy {
				a.acctSubmit()
			}
			switch mode {
			case "login":
				if btn("acct-forgot", T("Şifremi unuttum"), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					a.ac.mode, a.ac.err = "reset", ""
				}
			case "register":
				if btn("acct-wait", T("Davet kodum yok: bekleme listesine yazıl"), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					c.OpenURL(a.acctServer())
				}
			case "reset":
				if btn("acct-back", T("Giriş yap"), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}) {
					a.ac.mode, a.ac.err = "login", ""
				}
			}
		})
	})
}

// acctSubmit does what the form asks, in the mode it is in.
func (a *app) acctSubmit() {
	user, pass, code, rec, nw, mode := strings.TrimSpace(a.ac.user), a.ac.pass, a.ac.code, a.ac.recKey, a.ac.newPass, a.ac.mode
	var token string
	var key []byte
	switch {
	case user == "":
		a.ac.err = "Kullanıcı adı gerekli."
	case mode == "reset" && strings.TrimSpace(rec) == "":
		a.ac.err = "Kurtarma anahtarı gerekli."
	case mode == "reset" && len(nw) < 8:
		a.ac.err = "Yeni şifre en az 8 karakter olsun."
	case mode != "reset" && pass == "":
		a.ac.err = "Kullanıcı adı ve şifre gerekli."
	case mode == "register" && strings.TrimSpace(code) == "":
		a.ac.err = "Davet kodu gerekli."
	case mode == "register" && len(pass) < 8:
		a.ac.err = "Şifre en az 8 karakter olsun."
	default:
		a.acctRun(func(cl *acctClient) (err error) {
			switch mode {
			case "register":
				token, key, err = cl.Register(user, pass, code)
			case "reset":
				token, key, err = cl.Recover(user, rec, nw)
			default:
				token, key, err = cl.Login(user, pass)
			}
			return
		}, func() { a.acctDone(strings.ToLower(user), token, key, mode == "register") })
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
			settingRow(c, "Şifreyi değiştir", "Diğer bilgisayarlarda yeniden giriş yapman gerekir. Verin aynı anahtarla kalır.", func() {
				if btn("acct-pw-open", map[bool]string{true: T("Vazgeç"), false: T("Değiştir")}[a.ac.showPass], ryugo.ButtonOptions{Size: ryugo.Sm}) {
					a.ac.showPass, a.ac.err = !a.ac.showPass, ""
				}
			})
			if a.ac.showPass {
				ui.Column(c).Gap(8).FillWidth().Children(func() {
					a.fieldMenu(ryugo.Input(c.Key("acct-pw-old"), &a.ac.pass, ryugo.InputOptions{Label: T("Şimdiki şifre"), Size: ryugo.Sm, Password: true, Placeholder: T("Şimdiki şifre")}), &a.ac.pass, fieldOpts{oneLine: true, secret: true})
					a.fieldMenu(ryugo.Input(c.Key("acct-pw-new"), &a.ac.newPass, ryugo.InputOptions{Label: T("Yeni şifre"), Size: ryugo.Sm, Password: true, Placeholder: T("Yeni şifre")}), &a.ac.newPass, fieldOpts{oneLine: true, secret: true})
					errLine()
					if btn("acct-pw-go", T("Şifreyi değiştir"), ryugo.ButtonOptions{Size: ryugo.Sm}) && !a.ac.busy {
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
									a.notice = "Şifre değişti."
								})
						}
					}
				})
			}
		},
		func() {
			if a.ac.me == nil || len(a.ac.me.Devices) == 0 {
				ui.Text(c, T("Cihazlar yükleniyor…")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				return
			}
			ui.Column(c).Gap(8).FillWidth().Children(func() {
				ui.Text(c, T("Bu hesaba girmiş cihazlar")).FontSize(15).FontWeight(620).TextColor(cl.Text)
				for _, d := range a.ac.me.Devices {
					d := d
					ui.Row(c.Key("acct-dev:" + d.ID)).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
						name := d.Name
						if d.Current {
							name += " · " + T("bu cihaz")
						}
						ui.Text(c, name).FontSize(13).TextColor(cl.Text).Grow(1)
						ui.Text(c, time.Unix(d.Seen, 0).Format("2006-01-02 15:04")).FontSize(ryugo.FsSm).TextColor(cl.Muted)
						if !d.Current && btn("acct-devrm:"+d.ID, T("Çıkar"), ryugo.ButtonOptions{Size: ryugo.Sm}) {
							cl, token := newAcctClient(a.acctServer()), s.SyncToken
							a.acctRun(func(*acctClient) error { return cl.RemoveDevice(token, d.ID) }, func() { a.ac.meAt = time.Time{} })
						}
					})
				}
			})
		},
		func() {
			settingRow(c, "Hesabı sil", "Hesap ve sunucudaki tüm dosyalar silinir; bu bilgisayardaki kitaplık kalır. Geri alınamaz.", func() {
				if btn("acct-del-open", map[bool]string{true: T("Vazgeç"), false: T("Sil…")}[a.ac.confirm], ryugo.ButtonOptions{Size: ryugo.Sm}) {
					a.ac.confirm, a.ac.err = !a.ac.confirm, ""
				}
			})
			if a.ac.confirm {
				ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
					a.fieldMenu(ryugo.Input(c.Key("acct-del-pass"), &a.ac.delPass, ryugo.InputOptions{Label: T("Şifre"), Size: ryugo.Sm, Password: true, Placeholder: T("Şifreni yaz")}), &a.ac.delPass, fieldOpts{oneLine: true, secret: true})
					if btn("acct-del-go", T("Hesabı kalıcı olarak sil"), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Up}) && !a.ac.busy {
						pw := a.ac.delPass
						a.acctRun(func(cl *acctClient) error { return cl.DeleteAccount(s.SyncToken, s.AccountUser, pw) },
							func() { a.acctForget(); a.notice = "Hesap silindi." })
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
	if s.AccountUser == "" || s.SyncToken == "" {
		settingGroup(c, "set-acct-in", func() { a.acctForm(c) })
		return
	}
	a.acctRefresh()
	settingGroup(c, "set-acct-me0", func() {
		help := ""
		if a.ac.me != nil {
			help = humanBytes(a.ac.me.Bytes) + " / " + humanBytes(a.ac.me.Quota)
		}
		settingRow(c, T("Hesap: ")+s.AccountUser, help, func() {
			ui.Row(c).Gap(8).Children(func() {
				if ryugo.Button(c.Key("acct-profile"), T("Profil"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.openProfile()
				}
				if ryugo.Button(c.Key("acct-out"), T("Çıkış yap"), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.acctSignOut()
				}
			})
		})
	})
}

func init() {
	for tr, en := range map[string]string{
		"Giriş yap": "Sign in", "Hesap aç": "Create account", "Hesabı aç": "Create account", "Kullanıcı adı": "Username", "Şifre": "Password",
		"Davet kodu": "Invite code", "Kullanıcı adı ve şifre gerekli.": "A username and a password are needed.", "Davet kodu gerekli.": "An invite code is needed.",
		"Davet kodum yok: bekleme listesine yazıl": "No invite code: join the waitlist",
		"Şifren bu bilgisayardan çıkmaz; unutursan verini kimse geri getiremez. Hesabı açınca bir kurtarma anahtarı göstereceğiz: sakla.": "Your password never leaves this computer; if you forget it nobody can bring your data back. After the account is made we show a recovery key: keep it.",
		"Hesap hazır.": "The account is ready.", "Hesap: ": "Account: ", "Çıkış yap": "Sign out", "Şifreyi değiştir": "Change password", "Değiştir": "Change", "Vazgeç": "Cancel",
		"Diğer bilgisayarlarda yeniden giriş yapman gerekir. Verin aynı anahtarla kalır.": "You sign in again on your other computers. Your data keeps the same key.",
		"Şimdiki şifre": "Current password", "Yeni şifre": "New password", "Yeni şifre en az 8 karakter olsun.": "The new password needs at least 8 characters.",
		"Şifre değişti.": "Password changed.", "Cihazlar yükleniyor…": "Loading devices…", "Bu hesaba girmiş cihazlar": "Computers signed in to this account",
		"bu cihaz": "this computer", "Çıkar": "Remove", "Hesabı sil": "Delete account", "Sil…": "Delete…", "Şifreni yaz": "Type your password",
		"Hesabı kalıcı olarak sil": "Delete the account for good", "Hesap silindi.": "The account was deleted.",
		"Hesap ve sunucudaki tüm dosyalar silinir; bu bilgisayardaki kitaplık kalır. Geri alınamaz.": "The account and every file on the server are deleted; the library on this computer stays. This cannot be undone.",
		"Oturum kapanmış: yeniden giriş yap.":                                                        "Signed out: sign in again.",
		"Şifreleme anahtarı bu bilgisayarda yok: çıkış yapıp yeniden gir.":                           "The encryption key is not on this computer: sign out and in again.",
	} {
		if _, ok := enText[tr]; !ok {
			enText[tr] = en
		}
	}
}
