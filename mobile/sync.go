package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/internal/account"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// Sync is the desktop's (#634): the same account (internal/account), the same end-to-end encrypted remote and the
// same pass of the library (library.SyncCounted), over the phone's folder of pages. The token and the data key are in
// the system's secret store (MyGo's Secrets: Android's keystore), the account's address in the settings. A pass runs
// as the app opens and comes back, a moment after the typing stops, every few minutes while it shows, and when asked;
// the screens read the library again after one brought pages.

// The secrets' names in the store.
const (
	secretService  = "celestine"
	secretToken    = "syncToken"
	secretKey      = "syncKey"
	syncQuiet      = 2 * time.Second  // a pass this long after the typing stopped
	syncEvery      = 5 * time.Minute  // and one this often while the app shows
	syncOnActivate = 20 * time.Second // and one as the app comes back, when the last is older
)

// A secretStore keeps the account's secrets by name: the system's, or a test's.
type secretStore interface {
	Get(name string) ([]byte, error)
	Set(name string, value []byte) error
	Delete(name string) error
}

// secrets is the app's secret store.
var secrets secretStore = mygo.Secrets.Service(secretService)

// syncState is the sync as the screens tell it.
type syncState struct {
	busy    bool
	err     string    // why the last pass failed, in the app's words; "" when it did not
	at      time.Time // when the last pass ended well
	tried   time.Time // when the last pass started
	due     time.Time // when a pass is due, the typing having stopped; zero for none
	token   string    // the device's token, read once from the store
	key     []byte    // the data key, likewise
	loaded  bool      // the secrets were read
	signing bool      // a sign-in is on its way
	signErr string    // why it failed
	email   string    // the sign-in's fields
	pass    string
}

// signedIn reports whether the phone has an account to sync with.
func (a *app) signedIn() bool {
	a.loadSecrets()
	return a.settings.AccountUser != "" && a.sy.token != "" && len(a.sy.key) == 32
}

// loadSecrets reads the token and the data key from the store, once.
func (a *app) loadSecrets() {
	if a.sy.loaded {
		return
	}
	a.sy.loaded = true
	if b, err := secrets.Get(secretToken); err == nil {
		a.sy.token = string(b)
	}
	if b, err := secrets.Get(secretKey); err == nil {
		a.sy.key, _ = base64.StdEncoding.DecodeString(string(b))
	}
}

// syncServer is the server of the account and of the sync.
func (a *app) syncServer() string {
	if s := strings.TrimSpace(a.settings.SyncURL); s != "" {
		return s
	}
	return account.DefaultServer
}

// newClient is a client of the account's server, its errors in the app's words.
func (a *app) newClient() *account.Client {
	cl := account.New(a.syncServer(), nil, func() string { return "Android" })
	cl.Messages = func(err error) error {
		var st account.StatusError
		switch {
		case errors.Is(err, account.ErrDecrypt):
			return errors.New(locales.SyncErrDecrypt())
		case errors.As(err, &st):
			return errors.New(locales.SyncErrServer(strconv.Itoa(st.Status)))
		}
		return err
	}
	return cl
}

// serverWords is what the server said, which it says in Turkish (cloudflare/sync/auth.js), in the app's words.
func serverWords(s string) string {
	switch s {
	case "e-posta ya da şifre yanlış":
		return locales.SyncErrWrong()
	case "çok fazla deneme, sonra tekrar dene":
		return locales.SyncErrTooMany()
	}
	return s
}

// signIn signs the phone in with the sign-in's fields, away from the window, and syncs once it is.
func (a *app) signIn() {
	email, pass := strings.TrimSpace(a.sy.email), a.sy.pass
	if email == "" || pass == "" {
		a.sy.signErr = locales.SyncFill()
		return
	}
	if a.sy.signing {
		return
	}
	a.sy.signing, a.sy.signErr = true, ""
	cl := a.newClient()
	go func() {
		token, key, err := cl.Login(email, pass)
		a.update(func() {
			a.sy.signing = false
			if err != nil {
				a.sy.signErr = serverWords(err.Error())
				return
			}
			if err := secrets.Set(secretToken, []byte(token)); err == nil {
				err = secrets.Set(secretKey, []byte(base64.StdEncoding.EncodeToString(key)))
				if err != nil {
					a.sy.signErr = err.Error()
					return
				}
			}
			a.sy.token, a.sy.key, a.sy.loaded = token, key, true
			a.sy.pass = ""
			a.settings.AccountUser = account.Name(email)
			a.saveSettings()
			if a.router.Path() == pathSignIn {
				a.router.Back()
			}
			a.syncNow()
		})
	}()
}

// signOut signs the phone out: the server forgets its token, the store its secrets; the pages stay.
func (a *app) signOut() {
	cl, token := a.newClient(), a.sy.token
	go cl.Logout(token) // best done; a token the server keeps signs nothing in without the data key
	secrets.Delete(secretToken)
	secrets.Delete(secretKey)
	a.sy = syncState{loaded: true}
	a.settings.AccountUser = ""
	a.saveSettings()
}

// syncDir is where the sync keeps what it knows: the state of the last pass and the pages as last synced.
func (a *app) syncDir() string { return filepath.Join(a.dir, "sync") }

func (a *app) loadSyncState() *library.SyncState {
	st := &library.SyncState{}
	if b, err := os.ReadFile(filepath.Join(a.syncDir(), "state.json")); err == nil {
		json.Unmarshal(b, st)
	}
	st.Bases = library.DirBases{Root: filepath.Join(a.syncDir(), "bases")}
	return st
}

func (a *app) saveSyncState(st *library.SyncState) {
	if b, err := json.Marshal(st); err == nil && os.MkdirAll(a.syncDir(), 0o700) == nil {
		library.WriteFilePrivate(filepath.Join(a.syncDir(), "state.json"), b)
	}
}

// markPristine notes the files a new library made for itself, so that its first pass takes the account's copies of
// them with no conflict, as the desktop's does: a phone signing in to an account is not a conflict over its journal.
func (a *app) markPristine() {
	files, err := library.PristineOf(a.lib.FS, a.lib.Root)
	if err != nil {
		return
	}
	st := a.loadSyncState()
	st.Pristine = files
	a.saveSyncState(st)
}

// remoteID names the remote for the sync state, as the desktop names it (syncRemoteID): the server and the account,
// and the key; hashed, so that the key is not written beside the state.
func (a *app) remoteID() string {
	id := "server\x00" + base64.StdEncoding.EncodeToString(a.sy.key) + "\x00" + strings.TrimRight(a.syncServer(), "/") +
		"\x00account:" + a.settings.AccountUser
	return library.HashOf([]byte(id))
}

// syncNow makes a pass in the background: what is typed is saved first, and the library is read again if pages came.
func (a *app) syncNow() {
	if a.sy.busy || !a.signedIn() {
		return
	}
	remote, err := library.NewEncRemote(library.NewHTTPRemote(a.syncServer(), a.sy.token), a.sy.key)
	if err != nil {
		a.sy.err = err.Error()
		return
	}
	a.flush()
	a.sy.busy, a.sy.tried, a.sy.due = true, time.Now(), time.Time{}
	st := a.loadSyncState()
	st.ForRemote(a.remoteID())
	fsys, root := a.lib.FS, a.lib.Root
	go func() {
		res, err := library.SyncCounted(fsys, root, remote, st, map[string]bool{".git": true}, time.Now())
		a.update(func() {
			a.sy.busy = false
			a.saveSyncState(st)
			switch {
			case errors.Is(err, library.ErrDeviceRefused): // the server no longer takes this phone
				a.signOut()
				a.sy.err = locales.SyncRefused()
			case err != nil:
				a.sy.err = locales.SyncOffline()
			default:
				a.sy.err, a.sy.at = "", time.Now()
				if len(res.Errors) > 0 {
					a.sy.err = res.Errors[0]
				}
				if res.Pulled+res.Removed+res.Merged > 0 {
					a.reload()
					a.say(locales.SyncCame(res.Pulled + res.Merged))
				}
				if len(res.Conflicts) > 0 {
					a.say(locales.SyncConflicts(len(res.Conflicts)))
				}
			}
		})
	}()
}

// syncSoon has a pass made a moment after the typing stops.
func (a *app) syncSoon() {
	if a.signedIn() {
		a.sy.due = time.Now().Add(syncQuiet)
		time.AfterFunc(syncQuiet+50*time.Millisecond, func() { a.update(func() {}) })
	}
}

// syncTick, once a frame, starts the pass that is due: the one after the typing, or the one every few minutes.
func (a *app) syncTick(now time.Time) {
	if a.sy.busy || !a.signedIn() {
		return
	}
	if (!a.sy.due.IsZero() && !now.Before(a.sy.due)) || now.Sub(a.sy.tried) >= syncEvery {
		a.syncNow()
	}
}

// reload reads the library again after a pass brought pages, the page being written kept by its ID.
func (a *app) reload() {
	lib, err := library.OpenLibraryIn(a.lib.FS, a.lib.Root)
	if err != nil {
		return
	}
	a.lib = lib
	clear(plains)
	clear(mds)
	if a.ed.id != "" && !a.ed.dirty { // what came is shown in the page being read
		if p := a.lib.PageByID(a.ed.id); p != nil {
			a.ed = a.stateOf(p)
		}
	}
	a.lockDays()
}

// update runs fn on the window's thread, as the next frame is built; at once where there is no window (a test).
func (a *app) update(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	fn()
}

// syncLine is what the sync says of itself, under the account.
func (a *app) syncLine() string {
	switch {
	case a.sy.busy:
		return locales.SyncSyncing()
	case a.sy.err != "":
		return a.sy.err
	case !a.sy.at.IsZero():
		return locales.SyncLast(ago(a.sy.at))
	}
	return locales.SyncNever()
}
