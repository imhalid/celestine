package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/netproxy"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/account"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The account: its protocol and its keys are internal/account's, which the phone's app speaks too (#634); here are
// the desktop's names for them, its errors in the window's language, and the account's part of the settings.

// defaultServer is where accounts are opened and the waitlist is.
const defaultServer = account.DefaultServer

// The account's types, by the desktop's names.
type (
	acctClient = account.Client
	DeviceInfo = account.DeviceInfo
	AcctMe     = account.Me
)

func newKDF() (string, error) { return account.NewKDF() }
func acctKeys(username, password, kdf string) (string, []byte, error) {
	return account.Keys(username, password, kdf)
}
func recoveryAuth(key []byte) string                     { return account.RecoveryAuth(key) }
func recoveryAuthOfBefore(key []byte) string             { return account.RecoveryAuthOfBefore(key) }
func wrapDataKey(kek, key []byte) (string, error)        { return account.WrapDataKey(kek, key) }
func unwrapDataKey(kek []byte, s string) ([]byte, error) { return account.UnwrapDataKey(kek, s) }

// newAcctClient is a client of the server at base, through the system's proxy, this computer named as deviceName says,
// its errors in the window's language.
func newAcctClient(base string) *acctClient {
	cl := account.New(base, netproxy.Transport(), deviceName)
	cl.Messages = func(err error) error {
		var st account.StatusError
		switch {
		case errors.Is(err, account.ErrDecrypt): // the server said the proof was good, so this is a damaged blob
			return errors.New(locales.AccountErrDecrypt())
		case errors.As(err, &st):
			return errors.New(locales.AccountErrServer(strconv.Itoa(st.Status)))
		}
		return err
	}
	return cl
}

// deviceName is what this computer is called in the account's list of computers: the name given to it in the
// settings (ownDeviceName, #161), or its host name.
func deviceName() string {
	if n := strings.TrimSpace(ownDeviceName); n != "" {
		return n
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "cihaz"
}

// acctName is an address or a username as the server and the keys take it.
func acctName(s string) string { return account.Name(s) }

// ownDeviceName is the name the settings give this computer (Settings.DeviceName), set as they are read and changed.
var ownDeviceName string

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
	devName  string // this computer's name, as typed in the list of computers
	devEdit  bool   // and being typed
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
				a.ac.err = "signedOutSignInAgain"
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
		ryugo.Field(c, ryugo.FieldOptions{Label: msg(label)}, func() {
			o := ryugo.InputOptions{Label: msg(label), Size: ryugo.Sm}
			for _, f := range opts {
				f(&o)
			}
			if a.fieldMenu(ryugo.Input(c.Key(key), v, o), v, fieldOpts{oneLine: true, secret: o.Password}).Submitted() {
				entered = true
			}
		})
	}
	secret := func(o *ryugo.InputOptions) { o.Password = true }
	note := func(s string) { ui.Text(c, msg(s)).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap() }
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
			note("forgotYourPasswordSetANew")
		}
		switch mode {
		case "invite":
			note("ifYouWereApprovedFromThe")
			field("acct-user", "email", &a.ac.user, func(o *ryugo.InputOptions) { o.Placeholder = locales.AccountNameExampleCom() })
			field("acct-code", "accessCode", &a.ac.code, func(o *ryugo.InputOptions) { o.Placeholder = "RYU-XXXXXX" })
		case "password":
			field("acct-pass", "account_password", &a.ac.pass, secret)
			field("acct-pass2", "passwordAgain", &a.ac.pass2, secret)
			note("yourPasswordNeverLeavesThisComputer")
		case "reset":
			field("acct-user", "email", &a.ac.user)
			field("acct-rec", "account_recoveryKey", &a.ac.recKey, func(o *ryugo.InputOptions) { o.Placeholder = "XXXX-XXXX-…" })
			field("acct-pw-new", "account_newPassword", &a.ac.newPass, secret)
		default:
			field("acct-user", "email", &a.ac.user)
			field("acct-pass", "account_password", &a.ac.pass, secret)
		}
		if a.ac.err != "" {
			ui.Text(c, serverError(a.ac.err)).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			label := map[string]string{"invite": "continue", "password": "createAccount", "login": "account_sign", "reset": "setTheNewPassword"}[mode]
			if a.ac.busy {
				label = "…"
			}
			if (btn("acct-go", msg(label), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}) || entered) && !a.ac.busy {
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
			a.secretCopyButton(c, "copy-recovery-step", a.sy.recovery, locales.CommonCopyRecoveryKey())
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
		a.ac.err = "anEmailAddressIsNeeded"
	case mode == "invite" && !strings.Contains(user, "@"):
		a.ac.err = "thatEmailAddressIsntValid"
	case mode == "invite" && code == "":
		a.ac.err = "anAccessCodeIsNeeded"
	case mode == "invite":
		a.acctRun(func(cl *acctClient) error { return cl.CheckInvite(user, code) }, func() {
			a.ac.mode, a.ac.err, a.ac.pass, a.ac.pass2 = "password", "", "", ""
		})
	case mode == "password" && len(pass) < 8:
		a.ac.err = "thePasswordNeedsAtLeast8"
	case mode == "password" && pass != a.ac.pass2:
		a.ac.err = "thePasswordsAreNotTheSame"
	case mode == "reset" && strings.TrimSpace(rec) == "":
		a.ac.err = "theRecoveryKeyIsNeeded"
	case mode == "reset" && len(nw) < 8:
		a.ac.err = "theNewPasswordNeedsAtLeast"
	case mode == "login" && pass == "":
		a.ac.err = "anEmailAddressAndAPassword"
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
			ui.Text(c, serverError(a.ac.err)).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
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
							a.ac.err = "theEncryptionKeyIsNotOn"
						case len(nw) < 8:
							a.ac.err = "theNewPasswordNeedsAtLeast"
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
						if d.Current { // this one's name can be changed here (#161)
							a.deviceNameField(c, d.Name)
						} else {
							ui.Text(c, d.Name).FontSize(13).TextColor(cl.Text).Grow(1)
						}
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

// serverError is what the sync Worker said went wrong, in the window's language: it answers in Turkish
// (cloudflare/sync/auth.js), or the app's own message id, or the system's words, which are said as they are.
func serverError(s string) string {
	switch s {
	case "bu e-postayla açılmış bir hesap var: giriş yap":
		return msg("thereIsAlreadyAnAccountWith")
	case "e-posta adresi geçersiz":
		return msg("thatEmailAddressIsntValid2")
	case "e-posta ya da erişim kodu yanlış, ya da kodun süresi dolmuş":
		return msg("theEmailOrTheAccessCode")
	case "e-posta ya da kurtarma anahtarı yanlış":
		return msg("wrongEmailOrRecoveryKey")
	case "e-posta ya da şifre yanlış":
		return msg("wrongEmailOrPassword")
	case "çok fazla deneme, sonra tekrar dene":
		return msg("tooManyTriesTryAgainLater")
	}
	return msg(s)
}

// deviceNameField is this computer's name in the account's list, as a field: what is typed is its name there once
// it is entered, and the name it signs in with from then on.
func (a *app) deviceNameField(c *ui.Context, current string) {
	if !a.ac.devEdit {
		a.ac.devName = current
	}
	in := a.fieldMenu(ryugo.Input(c.Key("acct-devname"), &a.ac.devName, ryugo.InputOptions{Label: locales.AccountThisComputerName(), Size: ryugo.Sm}), &a.ac.devName, fieldOpts{oneLine: true}).Grow(1).MinWidth(0)
	a.ac.devEdit = in.Focused()
	name := strings.TrimSpace(a.ac.devName)
	if !in.Submitted() || name == "" || name == current || a.ac.busy {
		return
	}
	a.settings.DeviceName, ownDeviceName = name, name
	a.saveSettings()
	cl, token := newAcctClient(a.acctServer()), a.settings.SyncToken
	a.acctRun(func(*acctClient) error { return cl.RenameDevice(token, name) }, func() { a.ac.devEdit, a.ac.meAt = false, time.Time{} })
}
