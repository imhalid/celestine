package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The library's requests go through the proxy the user set: sync's, and the links' pages (#559).
func init() { library.HTTPTransport, library.LinkProxy = netproxy.Transport(), netproxy.Proxy }

// The app's side of synchronization: the settings, the remote they make, the state kept in the library's
// file, a sync now and then in the background, the badge of its state, the conflicts to choose between.

const syncStateKey = "sync-state"

type syncUI struct {
	busy     bool
	offline  bool      // the computer is offline (netsync.go): a sync to a server waits
	status   string    // "eşitleniyor…", "Eşit 14:32", "çevrimdışı: …", "çakışma: 2"
	last     time.Time // when the last pass began
	at       time.Time // when the last one went well
	result   string
	err      string
	conflict int
	// the conflict files as last looked for, and when; zero: look again (conflictFiles)
	conflicts      []string
	conflictsAt    time.Time
	conflictsBusy  bool
	conflictsAgain bool
	showColls      bool // the list of collections to sync is open
	tried          time.Time
	pass           string // the passphrase being typed
	passBusy       bool   // the passphrase is being checked against the remote (setSyncPassphrase)
	recovery       string // the recovery key just made, to be written down
	hasKey         bool
	held           int       // files the last pass did not delete here, the remote having none of them (massRemovals)
	noFolder       bool      // the last pass found no sync folder: Settings offers to make it (#568)
	allow          bool      // the user said to delete them: the next pass does
	backedUp       time.Time // when a pass last backed the library up
	resend         bool      // the user said to send them up again: the next pass does
	// refused is the files the last pass that reached the remote did not take, their copy there not being theirs,
	// and rolledBack that it served an index older than one seen (#148): a pass that could not reach it leaves them as
	// they were. trustIndex and trustFiles say the user trusts the remote's index, or its files, again: the next pass
	// does, and keeps the word for the one after if it could not use it up.
	refused                []string
	rolledBack             bool
	trustIndex, trustFiles bool
	// the store's Writes when it was last synced or last seen moving, and when it moved: a pass starts once the
	// writing has been quiet for syncQuiet, so that what is typed goes up in seconds, not at the next round
	writes   uint64
	movedAt  time.Time
	doneOnce sync.Once // the pass at quit, made once
	// a new library's welcome page waits for the first pass: made only if the account turns out to hold nothing
	// at all, so that a second device shows the account's pages, and a welcome deleted there does not come back
	seedWait bool
	// the server's change counter as last read (ChangeCounter), and whether it was: it is asked every syncPoll while the
	// window is in front, and a pass starts when it moved. behind: the window is not in front.
	changes      int64
	changesKnown bool
	askedAt      time.Time
	asking       bool
	behind       bool
	// gap: how long to the next ask (syncPoll, longer while the counter stays still); activeAt: when one last did
	// something in the window, and seen what that was (activity)
	gap      time.Duration
	activeAt time.Time
	seen     string
}

// syncPoll is how often the server is asked whether anything was written to the account, while the window is in
// front: one small request, and what another device wrote is here in seconds, not at the next round of minutes.
const syncPoll = 20 * time.Second

// syncPollMost is how far apart the asks grow while the counter does not move: each still answer adds syncPoll, and
// a change brings them back to syncPoll. What this device writes goes up at once all the same (syncQuiet).
const syncPollMost = 60 * time.Second

// syncIdle is how long the window may be in front with nobody doing anything in it before the server is not asked
// any more: a window left open overnight costs nothing. The first thing done asks again.
const syncIdle = 5 * time.Minute

// syncQuiet is how long the library is left unwritten before what was written is synced: a pause in the typing,
// so that a page goes up as soon as one stops, and is there on another device by the time one gets to it.
const syncQuiet = 2 * time.Second

// syncOnFocus is how old the last pass must be for the window coming forward to start one: one comes back to
// this device from another, and what was written there comes at once.
const syncOnFocus = 20 * time.Second

// windowFocused starts a pass when the window comes forward and the last one is not recent.
func (a *app) windowFocused() {
	a.sy.behind = false
	if a.settings.SyncMode == "" || a.sy.busy || a.sy.tried.IsZero() || a.clock().Sub(a.sy.tried) < syncOnFocus {
		return
	}
	a.sy.tried = time.Time{} // the next frame's syncTick starts it
}

// markPristine notes the files a new library made for itself, so that its first pass takes the account's
// copies of them, if it has them, with no conflict (SyncState.Pristine): signing in on a second device is not
// a conflict over its welcome page.
func (a *app) markPristine() {
	fsys, root := a.syncFS()
	files, err := PristineOf(fsys, root)
	if err != nil {
		return
	}
	state := a.loadSyncState()
	state.Pristine = files
	a.saveSyncState(state)
}

// syncRemote makes the remote the settings describe.
func (a *app) syncRemote() (Remote, error) {
	var r Remote
	switch a.settings.SyncMode {
	case "dir":
		if strings.TrimSpace(a.settings.SyncDir) == "" {
			return nil, errors.New(locales.SyncNoFolderChosen())
		}
		r = DirRemote{Root: a.settings.SyncDir}
	case "server":
		if a.settings.SyncURL == "" || a.settings.SyncToken == "" {
			return nil, errors.New(locales.SyncErrNoServer())
		}
		r = NewHTTPRemote(a.settings.SyncURL, a.settings.SyncToken)
	default:
		return nil, errors.New(locales.SyncErrOff())
	}
	// end-to-end encryption is not a choice: nothing leaves this computer before a passphrase has made the key
	if a.settings.SyncKey == "" {
		return nil, errors.New(locales.SyncErrNoPassphrase())
	}
	key, err := base64.StdEncoding.DecodeString(a.settings.SyncKey)
	if err != nil {
		return nil, errors.New(locales.SyncErrKeyBroken())
	}
	return NewEncRemote(r, key)
}

// syncRemoteID names the remote the settings point at, for the sync state (SyncState.ForRemote): the folder, or the
// server with the account (or the token, without one), and the key, which names every file on the remote. A
// hash, so that the token and the key are not written beside the state.
func (a *app) syncRemoteID() string {
	s := &a.settings
	id := s.SyncMode + "\x00" + s.SyncKey + "\x00"
	switch s.SyncMode {
	case "dir":
		dir := s.SyncDir
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		id += filepath.Clean(dir)
	case "server":
		id += strings.TrimRight(s.SyncURL, "/") + "\x00"
		if s.AccountUser != "" {
			id += "account:" + s.AccountUser
		} else {
			id += "token:" + s.SyncToken
		}
	}
	return HashOf([]byte(id))
}

// syncBackupEvery is how often a sync pass backs the library up first, at most.
const syncBackupEvery = 30 * time.Minute

func (a *app) syncEvery() time.Duration {
	if m := a.settings.SyncMinutes; m >= 1 && m <= 1440 {
		return time.Duration(m) * time.Minute
	}
	// between passes made for writing here and for the window coming forward, a round every few minutes is enough:
	// each costs the server a listing
	return 5 * time.Minute
}

// loadSyncState is what the last pass knew: kept in a library file's meta, or for a folder library in a folder of
// the config folder (syncStateDir). The pages as last synced, to merge (MergePage), are kept beside it.
func (a *app) loadSyncState() *SyncState {
	st := &SyncState{}
	if a.settingsStore != nil {
		if b := a.settingsStore.Meta(syncStateKey); b != nil {
			json.Unmarshal(b, st)
		}
		st.Bases = MetaBases{Store: a.settingsStore}
	} else if dir := a.syncStateDir(); dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
			json.Unmarshal(b, st)
		}
		st.Bases = DirBases{Root: filepath.Join(dir, "bases")}
	}
	return st
}

func (a *app) saveSyncState(st *SyncState) {
	b, _ := json.Marshal(st)
	if a.settingsStore != nil {
		a.settingsStore.SetMeta(syncStateKey, b)
	} else if dir := a.syncStateDir(); dir != "" {
		if os.MkdirAll(dir, 0o700) == nil {
			WriteFilePrivate(filepath.Join(dir, "state.json"), b)
		}
	}
}

// syncStateDir is where a folder library's sync keeps what it knows: beside the settings, in a folder named for
// the library's place, so that two folders synced to two places do not mix. "" when the settings are not kept (a test's).
func (a *app) syncStateDir() string {
	if a.settingsFile == "" {
		return ""
	}
	root, err := filepath.Abs(a.lib.Root)
	if err != nil {
		root = a.lib.Root
	}
	return filepath.Join(filepath.Dir(a.settingsFile), "sync", HashOf([]byte(root))[:16])
}

// syncFS is the library's files as the sync sees them: a library file's tree, or the folder of a folder library.
func (a *app) syncFS() (FS, string) {
	if st := a.store(); st != nil {
		return st, StoreRoot
	}
	return a.lib.FS, a.lib.Root
}

// libWrites counts the writes to the library: the store's own count, or the app's for a folder library
// (countFS); a pass starts once it has been still a moment.
func (a *app) libWrites() uint64 {
	n, _ := fsWrites(a.lib.FS)
	return n
}

// fsWrites is the count of writes to fsys, and whether it keeps one: the store's own, or countFS's.
func fsWrites(fsys FS) (uint64, bool) {
	switch fs := fsys.(type) {
	case *Store:
		return fs.Writes.Load(), true
	case *countFS:
		return fs.n.Load(), true
	}
	return 0, false
}

// countFS is a folder library's disk, counting the writes made through it: the folder has no count of its own, as
// a library file has (Store.Writes). What other programs write is not counted; the round of minutes takes it.
type countFS struct {
	FS
	n atomic.Uint64
}

func (c *countFS) WriteFile(name string, data []byte) error {
	c.n.Add(1)
	return c.FS.WriteFile(name, data)
}
func (c *countFS) Rename(oldName, newName string) error {
	c.n.Add(1)
	return c.FS.Rename(oldName, newName)
}
func (c *countFS) WriteFileKeepTime(name string, data []byte) error {
	c.n.Add(1)
	return library.WriteFileKeepTime(c.FS, name, data)
}
func (c *countFS) Remove(name string) error    { c.n.Add(1); return c.FS.Remove(name) }
func (c *countFS) RemoveAll(name string) error { c.n.Add(1); return c.FS.RemoveAll(name) }

// syncSkip is the top folders not to sync: the collections chosen out, and a folder library's .git.
func (a *app) syncSkip() map[string]bool {
	skip := map[string]bool{".git": true}
	for _, d := range a.settings.SyncOff {
		skip[d] = true
	}
	return skip
}

// syncNow makes a pass in the background: what is typed is saved first, the library is read again if
// something came, and the state is told in the badge.
func (a *app) syncNow() {
	if a.sy.busy || a.settings.SyncMode == "" {
		return
	}
	st := a.store() // nil for a folder library: no backup of a file before the pass, the files are there as they are
	fsys, root := a.syncFS()
	remote, err := a.syncRemote()
	if err != nil {
		a.sy.err, a.sy.status = err.Error(), locales.SyncSyncNotSetUp()
		return
	}
	a.flush()
	backupDir := a.backupDirOf()          // the backup before it is made in the background too: compressing
	if a.settings.NoBackup || st == nil { // a big library's file took the window a second, at every pass
		backupDir = ""
	}
	// once in a while, not before every pass: passes come two seconds after each pause in typing, and each backup is
	// the whole file read and compressed (#205)
	if now := a.clock(); now.Sub(a.sy.backedUp) < syncBackupEvery {
		backupDir = ""
	} else if backupDir != "" {
		a.sy.backedUp = now
	}
	a.sy.busy, a.sy.status, a.sy.last = true, locales.SyncSyncing(), a.clock()
	skip := a.syncSkip()
	state := a.loadSyncState()
	state.ForRemote(a.syncRemoteID())
	state.AllowRemovals, a.sy.allow = a.sy.allow, false
	state.Resend, a.sy.resend = a.sy.resend, false
	state.TrustIndex, a.sy.trustIndex = a.sy.trustIndex, false
	state.TrustFiles, a.sy.trustFiles = a.sy.trustFiles, false
	keep := a.keepRunning
	go func() {
		defer a.guard()
		if keep != nil { // at full speed while no window shows, until the pass ends (netsync.go)
			defer keep("Syncing the library")()
		}
		if backupDir != "" {
			if _, err := backupTo(st, backupDir, "senkron", time.Now()); err == nil {
				pruneBackups(backupDir, time.Now())
			}
		}
		res, err := SyncOnce(fsys, root, remote, state, skip, time.Now())
		changes, cerr := remoteChanges(remote) // what this pass wrote moved it: that is not news for the next ask
		a.do(func() error {
			a.sy.busy = false
			a.sy.changes, a.sy.changesKnown = changes, cerr == nil
			if cerr == nil {
				a.sy.askedAt = a.clock() // the pass asked: the next ask is a gap from now
			}
			a.sy.writes, a.sy.movedAt = a.libWrites(), time.Time{} // what came down is not something to send
			a.saveSyncState(state)
			hadErr := a.sy.err != ""
			a.sy.noFolder = errors.Is(err, library.ErrNoFolder)
			a.sy.trustIndex = a.sy.trustIndex || state.TrustIndex // not used up: the next pass tries again
			a.sy.trustFiles = a.sy.trustFiles || state.TrustFiles
			switch {
			case errors.Is(err, library.ErrIndexRollback):
				a.sy.rolledBack = true
			case err == nil:
				a.sy.rolledBack, a.sy.refused = false, res.Refused
			}
			switch {
			case errors.Is(err, library.ErrIndexRollback): // not offline: the remote answered, with an index older than one seen
				a.sy.err, a.sy.status = locales.SyncErrIndexRollback(), locales.NotifySyncStopped()
				if !hadErr {
					a.notify(notifySync, locales.NotifySyncStopped(), a.sy.err)
				}
			case errors.Is(err, library.ErrRemoteNotYet): // a folder bringing the index late: waited for, not told
				a.sy.err, a.sy.status = "", locales.SyncWaitingForRemote()
			case errors.Is(err, library.ErrNoFolder): // a drive not connected: offline
				a.sy.err, a.sy.status = locales.SyncErrNoFolder(), locales.SyncOffline()
			case err != nil:
				a.sy.err, a.sy.status = err.Error(), locales.SyncOffline()
			default:
				a.sy.err, a.sy.at, a.sy.result = "", time.Now(), syncSummary(res)
				a.sy.status = locales.SyncLastSynced(clockTime(a.sy.at))
				if res.Waiting { // what came up went, but what is to come down waits for the remote's hashes (#568)
					a.sy.status = locales.SyncWaitingForRemote()
				}
				if len(res.Errors) > 0 {
					a.sy.err = res.Errors[0]
				}
				if a.sy.held = res.Held; res.Held > 0 && a.sy.err == "" {
					a.sy.err = locales.SyncHeldGone(res.Held)
				}
				switch { // what waits for the user is told behind other apps too (notify.go); what was only offline is not
				case len(res.Conflicts) > 0:
					a.notify(notifySync, locales.NotifySyncConflicts(), locales.SyncSummary(syncSummary(res)))
				case res.Held > 0 || (a.sy.err != "" && !hadErr):
					a.notify(notifySync, locales.NotifySyncStopped(), a.sy.err)
				}
				a.seedAfterSync(res)
				a.purgeTrash()                             // what came down may be a trash that has run out
				a.lookForConflicts()                       // the badge says "çakışma: n" when it is done
				if res.Pulled+res.Removed+res.Merged > 0 { // something came: the window takes it, as it takes what an agent writes
					a.checkDisk()
					a.reloadLibrary()
				}
				// told only when something came, or clashed: what was sent is not news, and passes run a moment
				// after every pause in the typing
				if res.Pulled+res.Removed+res.Merged+len(res.Conflicts) > 0 {
					a.notice = locales.SyncSummary(syncSummary(res))
					tone := toastOK
					if len(res.Conflicts) > 0 {
						tone = toastWarn
					}
					a.noticeTone = &tone
				}
			}
			return nil
		})
	}()
}

// syncTick starts a pass when it is time: once at the start, then every few minutes.
func (a *app) syncTick(c *ui.Context) {
	if a.settings.SyncMode == "" || a.sy.busy || a.syncWaits() { // offline, it waits for the network to come back (netsync.go)
		return
	}
	now := a.clock()
	if !a.sy.tried.IsZero() {
		if w := a.libWrites(); w != a.sy.writes { // written since: the pass waits for the writing to stop
			a.sy.writes, a.sy.movedAt = w, now
		}
	}
	quiet := !a.sy.movedAt.IsZero() && now.Sub(a.sy.movedAt) >= syncQuiet
	// a round every few minutes, but not to a server while nobody is here: what is written still goes up (quiet)
	away := a.settings.SyncMode == "server" && !a.sy.activeAt.IsZero() && now.Sub(a.sy.activeAt) >= syncIdle
	if a.sy.tried.IsZero() || quiet || !away && now.Sub(a.sy.tried) > a.syncEvery() {
		a.sy.tried, a.sy.movedAt = now, time.Time{}
		a.syncNow()
	}
	if !a.sy.movedAt.IsZero() {
		c.After(syncQuiet - now.Sub(a.sy.movedAt) + 50*time.Millisecond)
		return
	}
	if a.settings.SyncMode == "server" && !a.sy.behind {
		if a.sy.activeAt.IsZero() { // the window just opened: one is here, and the pass at the start read the counter
			a.sy.activeAt = now
		}
		if now.Sub(a.sy.activeAt) >= syncIdle {
			return // nobody here: the next thing done in the window is a frame, and asks
		}
		gap := max(a.sy.gap, syncPoll)
		if now.Sub(a.sy.askedAt) >= gap {
			a.askChanges()
		}
		wait := gap - now.Sub(a.sy.askedAt)
		if wait <= 0 { // not asked: a pass is going on, and its end asks for a frame
			wait = gap
		}
		c.After(min(wait, syncIdle-now.Sub(a.sy.activeAt)) + 50*time.Millisecond)
		return
	}
	c.After(30 * time.Second)
}

// remoteChanges is the remote's change counter, when it keeps one.
func remoteChanges(r Remote) (int64, error) {
	if cc, ok := r.(ChangeCounter); ok {
		return cc.Changes()
	}
	return 0, errors.New("no change counter")
}

// askChanges asks the server, in the background, whether anything was written to the account since the last pass;
// a pass starts if it was.
func (a *app) askChanges() {
	if a.sy.asking || a.sy.busy {
		return
	}
	remote, err := a.syncRemote()
	if err != nil {
		return
	}
	a.sy.asking, a.sy.askedAt = true, a.clock()
	go func() {
		defer a.guard()
		n, err := remoteChanges(remote)
		a.do(func() error {
			a.sy.asking = false
			if err != nil || a.sy.busy { // a pass going on reads the counter at its end
				return nil
			}
			if a.sy.changesKnown && n != a.sy.changes {
				a.sy.tried = time.Time{} // the next frame's syncTick starts a pass
				a.sy.gap = syncPoll
			} else {
				a.sy.gap = min(max(a.sy.gap, syncPoll)+syncPoll, syncPollMost)
			}
			a.sy.changes, a.sy.changesKnown = n, true
			return nil
		})
	}()
}

// syncSummary is what a pass did, in the window's language.
func syncSummary(r SyncResult) string {
	if r.Pushed+r.Pulled+r.Deleted+r.Removed+r.Held+r.Merged+len(r.Conflicts) == 0 && len(r.Errors) == 0 {
		return locales.SyncInSyncNoChanges()
	}
	parts := []string{locales.SyncSent(r.Pushed), locales.SyncReceived(r.Pulled)}
	if r.Merged > 0 {
		parts = append(parts, locales.SyncMerged(r.Merged))
	}
	if n := r.Deleted + r.Removed; n > 0 {
		parts = append(parts, locales.SyncDeleted(n))
	}
	if len(r.Conflicts) > 0 {
		parts = append(parts, locales.SyncConflictsKept(len(r.Conflicts)))
	}
	if len(r.Errors) > 0 {
		parts = append(parts, locales.SyncErrors(len(r.Errors)))
	}
	if r.Held > 0 {
		parts = append(parts, locales.SyncDeletionsHeld(r.Held))
	}
	s := strings.Join(parts, locales.CommonListSeparator())
	return s
}

// seedNew gives a new library in a file its welcome page, opened, when it is not signed in to an account: the app
// is used without one, and the page is pristine (markPristine), so that signing in later takes the account's
// copies over it. One that is signed in waits for its first pass (seedAfterSync).
func (a *app) seedNew(empty bool) {
	if !empty {
		return
	}
	if a.settings.AccountUser != "" {
		a.sy.seedWait = true
		return
	}
	if p, err := seedWelcome(a.lib); err == nil {
		a.open(p)
	}
}

// seedAfterSync is the end of a new library's first pass: an account that never held anything gets the welcome
// page, opened; one that did has its pages here now, and gets none.
func (a *app) seedAfterSync(res SyncResult) {
	if !a.sy.seedWait {
		return
	}
	a.sy.seedWait = false
	if res.Remote > 0 || !bareLibrary(a.lib) {
		return
	}
	if p, err := seedWelcome(a.lib); err == nil {
		a.open(p)
	}
}

// syncAtQuit makes a last pass as the app goes, when something was written since the last one: a pass in the
// background is waited for, and the whole is given up after wait, the app going all the same (what was not
// sent goes up at the next start).
func (a *app) syncAtQuit(wait time.Duration) {
	a.sy.doneOnce.Do(func() {
		deadline := time.Now().Add(wait)
		for {
			a.mu.Lock()
			busy := a.sy.busy
			a.mu.Unlock()
			if !busy || time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		a.mu.Lock()
		fsys, root := a.syncFS()
		if a.settings.SyncMode == "" || a.sy.busy || a.libWrites() == a.sy.writes && a.sy.movedAt.IsZero() {
			a.mu.Unlock()
			return
		}
		remote, err := a.syncRemote()
		if err != nil {
			a.mu.Unlock()
			return
		}
		skip := a.syncSkip()
		state := a.loadSyncState()
		state.ForRemote(a.syncRemoteID())
		a.sy.busy = true
		a.mu.Unlock()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer a.guard()
			if _, err := SyncOnce(fsys, root, remote, state, skip, time.Now()); err == nil {
				a.mu.Lock()
				a.saveSyncState(state)
				a.mu.Unlock()
			}
		}()
		select {
		case <-done:
		case <-time.After(time.Until(deadline)):
		}
	})
}

// conflictFiles are the files that a conflict left: "… (çakışma …)…", as last looked for. The sync settings ask
// at every frame, and going through a big library takes a good part of a second (it took 650 ms a frame, reading
// every file): so they are looked for away from the window, by name only, the first time and after a pass or a resolve.
func (a *app) conflictFiles() []string {
	if a.sy.conflictsAt.IsZero() {
		a.lookForConflicts()
	}
	return a.sy.conflicts
}

// lookForConflicts goes through the library's file names in the background; the window gets the list when it is done.
func (a *app) lookForConflicts() {
	st, root := a.syncFS()
	if a.sy.conflictsBusy { // what it finds may be from before: it looks again when done
		a.sy.conflictsAgain = true
		return
	}
	a.sy.conflictsBusy = true
	go func() {
		defer a.guard()
		var found []string
		var walk func(dir, rel string)
		walk = func(dir, rel string) {
			ents, err := st.ReadDir(dir)
			if err != nil {
				return
			}
			for _, e := range ents {
				name := e.Name()
				if e.IsDir() {
					walk(filepath.Join(dir, name), path.Join(rel, name))
				} else if strings.Contains(name, " (çakışma ") && !strings.HasSuffix(name, ".tmp~") && !IsTempFile(name) {
					found = append(found, path.Join(rel, name))
				}
			}
		}
		walk(root, "")
		sortStrings(found)
		a.do(func() error {
			a.sy.conflicts, a.sy.conflictsAt, a.sy.conflictsBusy = found, time.Now(), false
			a.sy.conflict = len(found)
			if a.sy.conflict > 0 && !a.sy.busy && a.sy.err == "" {
				a.sy.status = locales.SyncConflictCount(a.sy.conflict)
			}
			if a.sy.conflictsAgain {
				a.sy.conflictsAgain = false
				a.lookForConflicts()
			}
			return nil
		})
	}()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// mainOfConflict is the file a conflict copy is of.
func mainOfConflict(name string) string {
	i := strings.Index(name, " (çakışma ")
	j := strings.Index(name[i:], ")")
	if i < 0 || j < 0 {
		return ""
	}
	return name[:i] + name[i+j+1:]
}

// resolveConflict: use the copy (it takes the place of the main file) or throw it away; either way the copy goes.
func (a *app) resolveConflict(name string, use bool) error {
	st, root := a.syncFS()
	abs := filepath.Join(root, filepath.FromSlash(name))
	if use {
		b, err := st.ReadFile(abs)
		if err != nil {
			return err
		}
		main := mainOfConflict(name)
		if main == "" {
			return errors.New(locales.SyncErrOriginalMissing())
		}
		if err := st.WriteFile(filepath.Join(root, filepath.FromSlash(main)), b); err != nil {
			return err
		}
	}
	if err := st.Remove(abs); err != nil {
		return err
	}
	a.checkDisk()
	a.reloadLibrary()
	// the copy is gone from the list at once; the library is looked through again in the background
	var left []string
	for _, f := range a.sy.conflicts {
		if f != name {
			left = append(left, f)
		}
	}
	a.sy.conflicts, a.sy.conflict = left, len(left)
	a.lookForConflicts()
	return nil
}

// syncBadge is the state, in the status bar.
func (a *app) syncBadge(c *ui.Context) {
	if a.settings.SyncMode == "" || a.sy.status == "" {
		return
	}
	cl := ryugo.From(c).Colors
	col := c.Theme().TextMuted
	switch {
	case a.sy.conflict > 0:
		col = cl.Warn
	case a.sy.err != "":
		col = cl.Up
	}
	b := ui.Row(c.Key("sync-badge")).Padding(2, 6).Radius(rad(6)).Label(locales.SyncSummary(a.sy.status)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Text(c, "⇅ "+a.sy.status).FontSize(11.5).TextColor(col) })
	if b.Clicked() {
		a.openSettings()
		a.settingsTab = tabSync
	}
}

// settingsSync is the settings' section of synchronization.
// The marks of the sync's modes (#406): off, a cloud struck through; a folder, a folder with an arrow into it; a
// server, a server's two units.
var (
	iconSyncOff    = icon(`<path d="M7.5 18.5h10a3.5 3.5 0 0 0 1-6.85A6 6 0 0 0 7.2 9.2 4.7 4.7 0 0 0 7.5 18.5Z"/><path d="M4 4l16 16"/>`)
	iconSyncFolder = icon(`<path d="M3.5 7.5A1.5 1.5 0 0 1 5 6h4l1.8 2H19a1.5 1.5 0 0 1 1.5 1.5V17a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 17Z"/><path d="M9 13.25h6m-2-2 2 2-2 2"/>`)
	iconServer     = icon(`<rect x="4" y="4.5" width="16" height="6.5" rx="2"/><rect x="4" y="13" width="16" height="6.5" rx="2"/><path d="M7.5 7.75h.01M7.5 16.25h.01M11 7.75h5M11 16.25h5"/>`)
)

// makeSyncDir makes the sync folder, which a pass found not there, and starts a pass with it. A pass never makes it
// itself: a folder not there may be a drive not connected, and one made in its place would be taken for it (#568).
func (a *app) makeSyncDir() {
	dir := a.settings.SyncDir
	a.sy.noFolder = false
	go func() {
		defer a.guard()
		err := DirRemote{Root: dir}.Make() // a share may be slow to answer: not under the lock
		a.do(func() error {
			if err != nil {
				a.sy.err = err.Error()
				return nil
			}
			a.sy.err, a.sy.tried = "", time.Time{} // the next frame's syncTick starts a pass with it
			return nil
		})
	}()
}

// pickSyncDir opens the system's folder picker, at the folder typed (from), and makes the one chosen the sync's.
func (a *app) pickSyncDir(from string) {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: locales.BackupChooseFolder(), Directory: true, DefaultPath: from})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error {
			a.settings.SyncDir = paths[0]
			a.sy.tried = time.Time{}
			a.saveSettings()
			return nil
		})
	}()
}

func (a *app) settingsSync(c *ui.Context) {
	cl := ryugo.From(c).Colors
	s := &a.settings
	mode := s.SyncMode
	modeRow := func() {
		settingRow(c, locales.CommonWhere(), locales.SyncAFolderIcloudDropbox(), func() {
			// each a button of its own color and mark, the one in use filled
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, m := range []struct {
					id, name string
					glyph    *ui.SVG
					tone     ui.Color
				}{{"", "common_off", iconSyncOff, cl.Muted}, {"dir", "folder", iconSyncFolder, cl.Warn}, {"server", "common_server", iconServer, cl.Accent}} {
					v := ryugo.Soft
					if mode == m.id {
						v = ryugo.Solid
					}
					if ryugo.Button(c.Key("set-sync-mode:"+m.id), msg(m.name), ryugo.ButtonOptions{Size: ryugo.Sm, Color: m.tone, Variant: v, Icon: m.glyph}).Clicked() {
						mode = m.id
					}
				}
			})
		})
	}
	settingGroup(c, "set-sync", modeRow)
	if mode != s.SyncMode {
		s.SyncMode = mode
		a.sy.tried = time.Time{}
		a.saveSettings()
	}
	if s.SyncMode == "" {
		ui.Text(c, locales.SyncSyncOffLibraryOnly()).FontSize(13).TextColor(cl.Muted).Wrap()
		if s.AccountUser == "" { // the way to sync: an account, made with an invite or signed in to
			ui.Text(c, locales.SyncWithCelestineAccountYour()).FontSize(13).TextColor(cl.Text2).Wrap()
			a.accountSection(c)
		}
		return
	}
	if s.SyncMode == "server" {
		a.accountSection(c)
	}
	dir, url, token := s.SyncDir, s.SyncURL, s.SyncToken
	field := func(key, label string, v *string, secret bool) func() {
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: msg(label)}, func() {
				in := a.fieldMenu(ryugo.Input(c.Key(key), v, ryugo.InputOptions{Label: msg(label), Size: ryugo.Sm, Password: secret}), v, fieldOpts{oneLine: true, secret: secret})
				_ = in
			})
		}
	}
	rows := []func(){}
	if s.SyncMode == "dir" { // typed, or chosen in the system's picker (#406)
		rows = append(rows, func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: msg("folder")}, func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() {
						a.fieldMenu(ryugo.Input(c.Key("sync-dir"), &dir, ryugo.InputOptions{Label: msg("folder"), Size: ryugo.Sm}), &dir, fieldOpts{oneLine: true})
					})
					if ryugo.Button(c.Key("sync-dir-pick"), locales.BackupChooseFolder(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconLibrary}).Clicked() {
						a.pickSyncDir(dir)
					}
				})
			})
		})
	} else {
		if s.AccountUser == "" {
			rows = append(rows, field("sync-url", "serverAddressHttpsWithoutAnAccount", &url, false), field("sync-token", "thisDevicesToken", &token, true))
		}
	}
	if len(rows) > 0 {
		settingGroup(c, "set-sync-where", rows...)
	}
	if dir != s.SyncDir || url != s.SyncURL || token != s.SyncToken {
		s.SyncDir, s.SyncURL, s.SyncToken = dir, url, token
		a.sy.noFolder = false // another folder: the next pass says whether it is there
		a.saveSettingsSoon()
	}
	// encryption: always on; it needs a passphrase before anything is sent
	enc := s.SyncKey != ""
	if s.AccountUser == "" { // with an account the key is the account's: nothing to type
		settingGroup(c, "set-sync-enc",
			func() {
				settingRow(c, locales.SyncEndEndEncryption(), locales.SyncRequiredFilesAreEncrypted(), func() {
					// with a key set, the passphrase is entered again, not changed: it is kept only if it opens what
					// the remote holds, so that a key that fails every pass can be put right (#153)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						a.fieldMenu(ryugo.Input(c.Key("sync-pass"), &a.sy.pass, ryugo.InputOptions{Label: locales.SyncPassphrase(), Size: ryugo.Sm, Placeholder: locales.SyncPassphrase()}), &a.sy.pass, fieldOpts{oneLine: true})
						label := locales.TurnOnEncryption()
						if enc {
							label = locales.SyncEnterPassphraseAgain()
						}
						if a.sy.passBusy {
							label = locales.CommonTrying()
						}
						if ryugo.Button(c.Key("sync-setpass"), label, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
							a.setSyncPassphrase(a.sy.pass)
						}
					})
				})
			},
			func() {
				if !enc {
					ui.Text(c, locales.SyncNoPassphraseYetSync()).FontSize(12.5).TextColor(cl.Warn)
					return
				}
				ui.Text(c, locales.SyncEncryption()).FontSize(12.5).TextColor(cl.Muted)
			})
	}
	if a.sy.recovery != "" && a.ac.mode != "recovery" { // the account's step shows it, then this
		settingGroup(c, "set-sync-recovery", func() {
			settingRow(c, locales.SyncRecoveryKeyWriteDown(), a.sy.recovery, func() {
				a.secretCopyButton(c, "copy-recovery", a.sy.recovery, locales.CommonCopyRecoveryKey())
			})
		})
	}
	// which collections
	var rows2 []func()
	for _, co := range a.lib.Collections {
		co := co
		on := true
		for _, d := range s.SyncOff {
			if d == co.Dir {
				on = false
			}
		}
		rows2 = append(rows2, func() {
			now := on
			settingRow(c, co.Name, "", func() {
				ryugo.Switch(c.Key("sync-coll:"+co.Dir), &now, ryugo.SwitchOptions{}).Label(locales.SyncCollectionLabel(co.Name))
			})
			if now != on {
				var out []string
				for _, d := range s.SyncOff {
					if d != co.Dir {
						out = append(out, d)
					}
				}
				if !now {
					out = append(out, co.Dir)
				}
				s.SyncOff = out
				a.saveSettings()
			}
		})
	}
	// a library of many collections makes a long list of switches, which drawn at every frame made the page
	// slow: it shows when asked, a line telling how many are synced in its place
	if len(rows2) > 0 {
		off := 0
		for _, co := range a.lib.Collections {
			if slices.Contains(s.SyncOff, co.Dir) {
				off++
			}
		}
		settingGroup(c, "set-sync-colls-head", func() {
			settingRow(c, locales.SyncWhichCollectionsSync(), locales.SyncCollectionsSynced(len(rows2), len(rows2)-off), func() {
				if ryugo.Button(c.Key("sync-colls-show"), map[bool]string{true: locales.CommonHide(), false: locales.SyncChoose()}[a.sy.showColls], ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconStack}).Clicked() {
					a.sy.showColls = !a.sy.showColls
				}
			})
		})
		if a.sy.showColls {
			settingGroup(c, "set-sync-colls", rows2...)
		}
	}
	// state
	status := a.sy.status
	if status == "" {
		status = "common_notSyncedYet"
	}
	if a.sy.err != "" {
		status += " · " + a.sy.err
	}
	settingGroup(c, "set-sync-state", func() {
		settingRow(c, locales.SyncStateIs(msg(status)), a.sy.result, func() {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				if ryugo.Button(c.Key("sync-now"), locales.CommonSyncNow(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid, Icon: iconSync}).Clicked() {
					a.syncNow()
				}
				if a.sy.noFolder && s.SyncMode == "dir" && ryugo.Button(c.Key("sync-make-dir"), locales.SyncCreateFolder(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft, Icon: iconSyncFolder}).Clicked() {
					a.makeSyncDir()
				}
				if ryugo.Button(c.Key("sync-probe"), locales.CommonTryConnection(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Down, Variant: ryugo.Soft, Icon: iconPlug}).Clicked() {
					a.syncProbe()
				}
			})
		})
		if a.ac.probe != "" || a.ac.probing {
			msg := a.ac.probe
			if a.ac.probing {
				msg = locales.CommonTrying()
			}
			ui.Text(c, msg).FontSize(13).TextColor(cl.Muted).Wrap()
		}
	})
	if a.sy.held > 0 {
		settingGroup(c, "set-sync-held", func() {
			settingRow(c, locales.SyncNotOnRemote(a.sy.held),
				locales.SyncTheRemoteMayHave(), func() {
					ui.Row(c).Gap(8).Children(func() {
						// the way the guard is for: the remote lost them, and these are the copies to keep (#147)
						if ryugo.Button(c.Key("sync-resend"), locales.SyncSendMineUpAgain(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid, Icon: iconSync}).Clicked() {
							a.sy.resend = true
							a.syncNow()
						}
						if ryugo.Button(c.Key("sync-allow"), locales.SyncApplyDeletions(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Warn, Variant: ryugo.Soft, Icon: iconTrash}).Clicked() {
							a.sy.allow = true
							a.syncNow()
						}
					})
				})
		})
	}
	if a.sy.rolledBack || len(a.sy.refused) > 0 {
		settingGroup(c, "set-sync-refused", func() {
			title := locales.SyncErrIndexRollback()
			if !a.sy.rolledBack {
				title = locales.SyncRefusedFiles(len(a.sy.refused))
			}
			settingRow(c, title, locales.SyncTrustRemoteHint(), func() {
				// the way out of a refusal that was no attack (#148): the index, when it is what stopped the pass, and
				// the files then, each at its own word, so that trusting a file does not take an older index too
				if ryugo.Button(c.Key("sync-trust"), locales.SyncTrustRemoteAgain(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Warn, Variant: ryugo.Soft, Icon: iconSync}).Clicked() {
					if a.sy.rolledBack {
						a.sy.trustIndex = true
					} else {
						a.sy.trustFiles = true
					}
					a.syncNow()
				}
			})
			if !a.sy.rolledBack {
				for i, f := range a.sy.refused {
					if i == 8 {
						ui.Text(c, "…").FontSize(12).TextColor(cl.Muted)
						break
					}
					ui.Text(c, f).FontSize(12).TextColor(cl.Muted).Wrap()
				}
			}
		})
	}
	if files := a.conflictFiles(); len(files) > 0 {
		ui.Text(c, locales.SyncConflictsFilesThatChanged()).FontSize(13).FontWeight(620).TextColor(cl.Warn)
		var rows3 []func()
		for _, f := range files {
			f := f
			rows3 = append(rows3, func() {
				settingRow(c, mainOfConflict(f), f, func() {
					ui.Row(c).Gap(6).Children(func() {
						if ryugo.Button(c.Key("cf-use:"+f), locales.SyncUseThis(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft, Icon: iconCheck}).Clicked() {
							if err := a.resolveConflict(f, true); err != nil {
								a.say(toastErr, locales.CommonDidntWork(err.Error()))
							}
						}
						if ryugo.Button(c.Key("cf-drop:"+f), locales.CommonDelete(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Up, Variant: ryugo.Soft, Icon: iconTrash}).Clicked() {
							if err := a.resolveConflict(f, false); err != nil {
								a.say(toastErr, locales.CommonDidntWork(err.Error()))
							}
						}
					})
				})
			})
		}
		settingGroup(c, "set-sync-conf", rows3...)
	}
}

// setSyncPassphrase turns encryption on with the passphrase: the key is made with the remote's salt, checked
// against what the remote holds, kept, and its recovery words shown once. With a key set, it is the passphrase
// entered again, never changed (#153): the new key replaces the one kept only if it opens the remote's sealed
// index or its check, and a remote with neither has nothing to check it against, so the key kept stays. The remote and
// the key derivation are slow, so they run in a goroutine, out of the app's lock; what they found is kept only if
// the settings still point where they did, with the same key.
func (a *app) setSyncPassphrase(pass string) {
	if a.sy.passBusy {
		return
	}
	again := a.settings.SyncKey != ""
	if strings.TrimSpace(pass) == "" {
		a.say(toastWarn, locales.TypeAPassword())
		return
	}
	var inner Remote
	switch a.settings.SyncMode {
	case "dir":
		inner = DirRemote{Root: a.settings.SyncDir}
	case "server":
		inner = NewHTTPRemote(a.settings.SyncURL, a.settings.SyncToken)
	default:
		a.say(toastWarn, locales.FirstChooseWhereToSyncTo())
		return
	}
	id := a.syncRemoteID() // it names the key too: a key set another way while this is checked is not written over
	a.sy.passBusy = true
	go func() {
		defer a.guard()
		// the button is let go however this ends, a panic too: deferred after guard, it runs before guard recovers
		defer a.do(func() error { a.sy.passBusy = false; return nil })
		e, err := OpenEnc(inner, pass)
		a.do(func() error {
			if a.syncRemoteID() != id {
				return nil // set another way, or pointed elsewhere, while it was checked
			}
			switch {
			case errors.Is(err, library.ErrWrongPassphrase):
				a.say(toastErr, locales.SyncErrWrongPassphrase())
				return nil
			case err != nil:
				a.say(toastErr, locales.CommonDidntWork(err.Error()))
				return nil
			case again && !e.Checked():
				a.say(toastErr, locales.SyncErrNothingToCheck())
				return nil
			}
			a.settings.SyncKey = base64.StdEncoding.EncodeToString(e.Key)
			a.sy.recovery, a.sy.pass, a.sy.hasKey = RecoveryKey(e.Key), "", true
			a.saveSettings()
			if again {
				a.sy.err, a.sy.tried = "", time.Time{} // the next frame's syncTick starts a pass with it
				a.say(toastOK, locales.SyncPassphraseAccepted())
				return nil
			}
			a.say(toastWarn, locales.EncryptionIsOnWriteTheRecovery())
			return nil
		})
	}()
}

// activity notes whether one did something in the window since the last frame: the pointer moved over it or pressed,
// or the text or the caret moved. What the sync itself writes is not one doing something. Back after a still while, the server is asked at once, and as often
// as at first.
func (a *app) activity(root ui.Element) {
	x, y, over := root.PointerPosition()
	seen := fmt.Sprint(x, y, over, root.Pressed(), a.caretAt, a.version)
	if seen == a.sy.seen {
		return
	}
	idle := a.clock().Sub(a.sy.activeAt) >= syncIdle
	a.sy.seen, a.sy.activeAt = seen, a.clock()
	if idle && a.frame != nil {
		a.sy.askedAt, a.sy.gap = time.Time{}, syncPoll // back after a while: asked now, and as often as at first
		a.frame()
	}
}
