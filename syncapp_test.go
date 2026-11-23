package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
)

func syncApp(t *testing.T, remoteDir string) *app {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.settingsStore = st
	a.backupDir = filepath.Join(t.TempDir(), "b")
	a.settings.SyncMode, a.settings.SyncDir = "dir", remoteDir
	a.settings.SyncKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)) // encryption is not a choice
	return a
}

func waitSync(t *testing.T, a *app) {
	t.Helper()
	for i := 0; i < 500; i++ {
		a.mu.Lock()
		busy := a.sy.busy || a.sy.conflictsBusy // the conflicts are looked for after the pass
		a.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("sync did not finish")
}

// Two apps, a folder between them: what one writes the other reads, the badge says so, and a conflict
// can be chosen between.
func TestSyncApps(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	c, _ := a.lib.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Merhaba")
	p.SaveText("birinci metin\n")
	a.syncNow()
	waitSync(t, a)
	if !strings.HasPrefix(a.sy.status, "Son eşitleme") {
		t.Fatalf("status %q err %q", a.sy.status, a.sy.err)
	}
	b.syncNow()
	waitSync(t, b)
	var found *Page
	for _, cc := range b.lib.Collections {
		for _, pp := range cc.All() {
			found = pp
		}
	}
	if found == nil || found.Title != "Merhaba" {
		t.Fatalf("the other app does not have it: %v %q", b.lib.Collections, b.sy.err)
	}
	// a conflict: both change the page
	pa := a.lib.Collections[0].All()[0]
	pa.SaveText("A'nın metni\n")
	found.SaveText("B'nin metni\n")
	a.sy.tried = time.Time{}
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	if b.sy.conflict != 1 || b.sy.status != locales.SyncConflictCount(1) {
		t.Fatalf("no conflict on B: %d %q", b.sy.conflict, b.sy.status)
	}
	files := b.conflictFiles()
	if len(files) != 1 {
		t.Fatalf("%v", files)
	}
	// B chooses its own text back
	if err := b.resolveConflict(files[0], true); err != nil {
		t.Fatal(err)
	}
	if len(b.conflictFiles()) != 0 {
		t.Fatal("the copy stayed")
	}
	got, _ := b.lib.Collections[0].All()[0].Text()
	if !strings.Contains(got, "B'nin metni") {
		t.Fatalf("the choice was not made: %q", got)
	}
	if mainOfConflict("Ortak/Merhaba/page (çakışma 2026-11-07 10-00-00).json") != "Ortak/Merhaba/page.json" {
		t.Fatal("mainOfConflict")
	}
}

// setPassphrase turns encryption on with pass as the settings' button does, from a frame, and waits for the
// passphrase to be checked against the remote.
func setPassphrase(t *testing.T, a *app, pass string) {
	t.Helper()
	a.mu.Lock()
	a.settings.SyncKey = "" // syncApp's own key: the passphrase is for an app with none
	a.mu.Unlock()
	enterPassphrase(t, a, pass)
}

// enterPassphrase enters pass as the settings' button does, from a frame, with the key the app has, and waits for
// it to be checked against the remote.
func enterPassphrase(t *testing.T, a *app, pass string) {
	t.Helper()
	a.mu.Lock()
	a.setSyncPassphrase(pass)
	a.mu.Unlock()
	for i := 0; i < 500; i++ {
		a.mu.Lock()
		busy := a.sy.passBusy
		a.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the passphrase was not checked")
}

// With encryption on from the passphrase, two apps agree, and the folder holds nothing readable.
func TestSyncAppsEncrypted(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	setPassphrase(t, a, "parola")
	if a.settings.SyncKey == "" || a.sy.recovery == "" {
		t.Fatalf("no key: %q", a.notice)
	}
	setPassphrase(t, b, "parola")
	if a.settings.SyncKey != b.settings.SyncKey {
		t.Fatal("two devices with the same passphrase made different keys")
	}
	c, _ := a.lib.NewCollection("Gizli")
	p, _ := c.NewPage(nil, "Banka")
	p.SaveText("hesap numarası 99887766\n")
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	if len(b.lib.Collections) != 1 || len(b.lib.Collections[0].All()) != 1 {
		t.Fatalf("not through: %q", b.sy.err)
	}
	_, all := allRemoteBytes(t, dir)
	if bytes.Contains(all, []byte("99887766")) || bytes.Contains(all, []byte("Banka")) {
		t.Fatal("readable on the remote")
	}
	// a wrong passphrase is refused and not kept (a typo when joining), and the sync, with no key, gets nothing
	c2 := syncApp(t, dir)
	setPassphrase(t, c2, "yanlış")
	if c2.settings.SyncKey != "" || c2.sy.recovery != "" || c2.notice != locales.SyncErrWrongPassphrase() {
		t.Fatalf("the wrong passphrase: key %q notice %q", c2.settings.SyncKey, c2.notice)
	}
	c2.syncNow()
	waitSync(t, c2)
	if c2.sy.err == "" || len(c2.lib.Collections) != 0 {
		t.Fatalf("the wrong passphrase: err %q collections %d", c2.sy.err, len(c2.lib.Collections))
	}
	// and the right one, typed again, is
	setPassphrase(t, c2, "parola")
	if c2.settings.SyncKey != a.settings.SyncKey {
		t.Fatalf("the right passphrase after a wrong one: %q", c2.notice)
	}
}

// With the passphrase set, the settings offer to enter it again, not to change it (#153). With none, encryption
// is turned on there.
func TestSyncPassphraseField(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode, a.settings.SyncDir = "dir", t.TempDir()
	a.settings.SyncKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText(locales.SyncEndEndEncryption()) {
		t.Fatalf("no encryption row: %v", tt.Texts())
	}
	// with the key set, the passphrase is entered again, checked against the remote; it is not turned on anew
	if tt.HasText(locales.TurnOnEncryption()) || !tt.HasText(locales.SyncEnterPassphraseAgain()) {
		t.Fatalf("the passphrase's button with the key set: %v", tt.Texts())
	}
	if _, ok := tt.Find(locales.SyncPassphrase()); !ok {
		t.Fatal("no passphrase field with the key set")
	}
	a.settings.SyncKey = ""
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText(locales.TurnOnEncryption()) {
		t.Fatalf("no button to turn encryption on: %v", tt.Texts())
	}
	if _, ok := tt.Find(locales.SyncPassphrase()); !ok {
		t.Fatal("no passphrase field with no key")
	}
}

// A device whose stored key does not open the remote fails every pass; the passphrase entered again puts it right,
// and it syncs. A passphrase that does not open the remote is refused and the key kept, and so is any passphrase
// on a remote with nothing sealed to check it against (#153).
func TestSyncPassphraseEnteredAgain(t *testing.T) {
	// a remote with a salt and no index: nothing to check a passphrase against, the key kept stays
	empty := syncApp(t, t.TempDir())
	key := empty.settings.SyncKey
	enterPassphrase(t, empty, "parola")
	if empty.settings.SyncKey != key || empty.notice != locales.SyncErrNothingToCheck() {
		t.Fatalf("a remote with no index: key changed %v, notice %q", empty.settings.SyncKey != key, empty.notice)
	}

	dir := t.TempDir()
	a := syncApp(t, dir)
	setPassphrase(t, a, "parola")
	c, _ := a.lib.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Merhaba")
	p.SaveText("birinci metin\n")
	a.syncNow()
	waitSync(t, a)

	b := syncApp(t, dir) // its key is not the remote's
	b.syncNow()
	waitSync(t, b)
	if b.sy.err == "" || len(b.lib.Collections) != 0 {
		t.Fatalf("the wrong key: err %q collections %d", b.sy.err, len(b.lib.Collections))
	}
	wrong := b.settings.SyncKey
	enterPassphrase(t, b, "yanlış")
	if b.settings.SyncKey != wrong || b.notice != locales.SyncErrWrongPassphrase() {
		t.Fatalf("a passphrase that does not open the remote: key changed %v, notice %q", b.settings.SyncKey != wrong, b.notice)
	}
	enterPassphrase(t, b, "parola")
	if b.settings.SyncKey != a.settings.SyncKey || b.notice != locales.SyncPassphraseAccepted() || b.sy.err != "" {
		t.Fatalf("the right passphrase entered again: notice %q err %q", b.notice, b.sy.err)
	}
	b.syncNow()
	waitSync(t, b)
	if b.sy.err != "" || len(b.lib.Collections) != 1 || len(b.lib.Collections[0].All()) != 1 {
		t.Fatalf("not synced after the passphrase: err %q", b.sy.err)
	}
	// and once right, a passphrase of another remote is no way to change it
	enterPassphrase(t, b, "başka")
	if b.settings.SyncKey != a.settings.SyncKey {
		t.Fatal("the passphrase was changed")
	}
}

// A panic while the passphrase is checked is kept as a crash, and the button is let go: it does not stay on
// "Trying…" for good, and the passphrase can be entered again.
func TestSyncPassphrasePanicLetsGo(t *testing.T) {
	a := syncApp(t, t.TempDir())
	a.crashDir = t.TempDir()
	open := OpenEnc
	OpenEnc = func(Remote, string) (*EncRemote, error) { panic("the check fell over") }
	defer func() { OpenEnc = open }()
	key := a.settings.SyncKey
	enterPassphrase(t, a, "parola") // it fails the test if the button stays busy
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.crashText != "the check fell over" || a.settings.SyncKey != key {
		t.Fatalf("crash %q, key changed %v", a.crashText, a.settings.SyncKey != key)
	}
}

func TestSyncSettingsPage(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode = "dir"
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Nereye", "Uçtan uca şifreleme", "Durum"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
}

// Sync never goes unencrypted: with no passphrase there is no remote at all.
func TestSyncNeedsEncryption(t *testing.T) {
	a := syncApp(t, t.TempDir())
	a.settings.SyncKey = ""
	if r, err := a.syncRemote(); err == nil || r != nil {
		t.Fatalf("a remote without a key: %v %v", r, err)
	}
	a.syncNow()
	waitSync(t, a)
	if a.sy.err == "" {
		t.Fatal("no error said")
	}
}

// What has been in the trash more than trashDays is deleted for good; what is younger stays.
func TestPurgeTrash(t *testing.T) {
	root := t.TempDir()
	lib := &Library{Root: root, FS: OSFS{}}
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	for name, age := range map[string]int{"eski": 31, "yeni": 29} {
		dir := filepath.Join(root, ".cop", name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "page.json"), []byte(`{"meta":{},"body":[]}`), 0o644)
		data, _ := json.Marshal(TrashItem{Kind: "page", Title: name, Trashed: now.AddDate(0, 0, -age)})
		os.WriteFile(filepath.Join(dir, TrashNote), data, 0o644)
	}
	if n := lib.PurgeTrash(now); n != 1 {
		t.Fatalf("purged %d", n)
	}
	items := lib.Trash()
	if len(items) != 1 || items[0].Title != "yeni" {
		t.Fatalf("left: %+v", items)
	}
	if d := trashDaysLeft(items[0], now); d != 1 {
		t.Fatalf("days left %d", d)
	}
}

// A conflict chosen while the page is open in the editor: the editor shows the choice, and what is written
// next does not put the other text back over it.
func TestResolveConflictUpdatesOpenPage(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	c, _ := a.lib.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Merhaba")
	p.SaveText("birinci metin\n")
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	pa := a.lib.Collections[0].All()[0]
	var pb *Page
	for _, cc := range b.lib.Collections {
		for _, pp := range cc.All() {
			pb = pp
		}
	}
	pa.SaveText("A'nın metni\n")
	pb.SaveText("B'nin metni\n")
	a.sy.tried = time.Time{}
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	files := b.conflictFiles()
	if len(files) != 1 {
		t.Fatalf("%v", files)
	}
	for _, cc := range b.lib.Collections {
		for _, pp := range cc.All() {
			pb = pp
		}
	}
	b.open(pb) // the page is in the editor: the remote's text, which stayed
	if !strings.Contains(b.doc.Text().String(), "A'nın metni") {
		t.Fatalf("the editor has %q", b.doc.Text().String())
	}
	if err := b.resolveConflict(files[0], true); err != nil {
		t.Fatal(err)
	}
	if b.doc == nil || !strings.Contains(b.doc.Text().String(), "B'nin metni") {
		t.Fatalf("the editor does not show the choice: %v", b.doc)
	}
	b.flush()
	disk, _ := b.page.Text()
	if !strings.Contains(disk, "B'nin metni") {
		t.Fatalf("what is saved after the choice is %q", disk)
	}
}

// The library's sync tests (internal/library/sync_test.go) have these too: a test of one package does not
// see another's.

// A device is a folder that stands for a library, with what it knew at its last sync.
type device struct {
	t    *testing.T
	root string
	st   SyncState
}

func newDevice(t *testing.T) *device { return &device{t: t, root: t.TempDir()} }

func (d *device) write(name, text string) {
	d.t.Helper()
	p := filepath.Join(d.root, filepath.FromSlash(name))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		d.t.Fatal(err)
	}
}

func (d *device) read(name string) string {
	b, err := os.ReadFile(filepath.Join(d.root, filepath.FromSlash(name)))
	if err != nil {
		return "<yok>"
	}
	return string(b)
}

func (d *device) sync(r Remote, skip ...string) SyncResult {
	d.t.Helper()
	sk := map[string]bool{}
	for _, s := range skip {
		sk[s] = true
	}
	res, err := SyncOnce(OSFS{}, d.root, r, &d.st, sk, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))
	if err != nil {
		d.t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		d.t.Fatalf("errors: %v", res.Errors)
	}
	return res
}

func allRemoteBytes(t *testing.T, root string) (names []string, all []byte) {
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			names = append(names, rel)
			b, _ := os.ReadFile(p)
			all = append(all, b...)
		}
		return nil
	})
	return
}

// What is written goes up once the writing has been quiet a few seconds, not at the next round of minutes; and
// what is written last goes up as the app quits.
func TestSyncAfterWritingAndAtQuit(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	a.settings.AccountUser = "ali@ornek.com" // past the sign-in
	now := time.Now()
	a.clock = func() time.Time { return now }
	tt := ui.NewTester(a.view, 900, 700)
	tt.Frame() // the pass at the start
	waitSync(t, a)
	has := func(title string) bool {
		t.Helper()
		b.syncNow()
		waitSync(t, b)
		for _, c := range b.lib.Collections {
			for _, p := range c.All() {
				if p.Title == title {
					return true
				}
			}
		}
		return false
	}
	c, _ := a.lib.NewCollection("Shared")
	p, _ := c.NewPage(nil, "Written")
	p.SaveText("words\n")
	tt.Frame() // the writing is seen
	now = now.Add(syncQuiet / 2)
	tt.Frame()
	waitSync(t, a)
	if has("Written") {
		t.Fatal("synced before the writing was quiet")
	}
	now = now.Add(syncQuiet)
	tt.Frame()
	waitSync(t, a)
	if !has("Written") {
		t.Fatalf("not synced after the writing stopped: %q", a.sy.err)
	}
	q, _ := c.NewPage(nil, "Last")
	q.SaveText("the last words\n")
	a.syncAtQuit(5 * time.Second)
	if !has("Last") {
		t.Fatal("not synced at quit")
	}
}

// A new library waits for its account before it has a welcome page: an account that never held anything gets
// one; a second device gets the account's pages and makes none of its own, with no conflict over the journal it
// made; and a welcome page thrown away on the account does not come back on a third.
func TestSyncNewLibraryWaitsForTheAccount(t *testing.T) {
	dir := t.TempDir()
	open := func() *app {
		a := syncApp(t, dir)
		a.sy.seedWait = bareLibrary(a.lib)
		if _, err := a.journal(); err != nil {
			t.Fatal(err)
		}
		a.markPristine()
		return a
	}
	titles := func(a *app) map[string]bool {
		out := map[string]bool{}
		for _, c := range a.lib.Collections {
			for _, p := range c.All() {
				out[p.Title] = true
			}
		}
		return out
	}
	a := open()
	if titles(a)["Welcome"] {
		t.Fatal("a welcome page before the account was asked")
	}
	a.syncNow()
	waitSync(t, a)
	if !titles(a)["Welcome"] || a.page == nil || a.page.Title != "Welcome" {
		t.Fatalf("an empty account got no welcome page: %v", titles(a))
	}
	var notes *Collection
	for _, c := range a.lib.Collections {
		if !c.Journal {
			notes = c
		}
	}
	p, _ := notes.NewPage(nil, "From the first")
	p.SaveText("words\n")
	a.syncNow()
	waitSync(t, a)

	time.Sleep(10 * time.Millisecond) // the second device's journal is made later, and differs
	b := open()
	b.syncNow()
	waitSync(t, b)
	if b.sy.conflict != 0 || len(b.conflictFiles()) != 0 {
		t.Fatalf("conflicts on the second device: %v (%s)", b.conflictFiles(), b.sy.result)
	}
	if got := titles(b); !got["From the first"] || !got["Welcome"] {
		t.Fatalf("the account's pages did not come down: %v (%s)", got, b.sy.result)
	}
	n := 0
	for _, c := range b.lib.Collections {
		if !c.Journal {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the second device made a collection of its own: %d", n)
	}

	for _, pp := range a.lib.Collections {
		for _, q := range pp.All() {
			if q.Title == "Welcome" {
				a.trashPage(q)
			}
		}
	}
	a.syncNow()
	waitSync(t, a)
	c := open()
	c.syncNow()
	waitSync(t, c)
	if got := titles(c); got["Welcome"] || !got["From the first"] {
		t.Fatalf("the third device: %v (%s)", got, c.sy.result)
	}
}

// A new library not signed in to an account has its welcome page at once, opened; signing in later takes the account's
// copies over it, untouched, with no conflict. One signed in waits for its first pass.
func TestSyncNewLibraryWithoutAccount(t *testing.T) {
	dir := t.TempDir()
	open := func(user string) *app {
		a := syncApp(t, dir)
		if _, err := a.journal(); err != nil {
			t.Fatal(err)
		}
		a.settings.AccountUser = user
		a.seedNew(bareLibrary(a.lib))
		a.markPristine()
		return a
	}
	first := open("ali@ornek.com")
	if !first.sy.seedWait || len(first.lib.Collections) != 1 {
		t.Fatalf("signed in, the welcome page did not wait: %d collections", len(first.lib.Collections))
	}
	first.syncNow()
	waitSync(t, first)
	if first.page == nil || first.page.Title != "Welcome" {
		t.Fatal("an empty account got no welcome page")
	}
	first.page.SaveText("the account's own welcome\n")
	first.syncNow()
	waitSync(t, first)

	time.Sleep(10 * time.Millisecond)
	second := open("") // used without an account, then signed in
	if second.sy.seedWait || second.page == nil || second.page.Title != "Welcome" {
		t.Fatalf("without an account, no welcome page at once: wait %v", second.sy.seedWait)
	}
	second.settings.AccountUser = "ali@ornek.com"
	second.syncNow()
	waitSync(t, second)
	if second.sy.conflict != 0 || len(second.conflictFiles()) != 0 {
		t.Fatalf("conflicts on signing in: %v (%s)", second.conflictFiles(), second.sy.result)
	}
	var welcomes []string
	for _, c := range second.lib.Collections {
		for _, p := range c.All() {
			if p.Title == "Welcome" {
				text, _ := p.Text()
				welcomes = append(welcomes, text)
			}
		}
	}
	if len(welcomes) != 1 || welcomes[0] != "the account's own welcome\n" {
		t.Fatalf("the welcome pages after signing in: %q", welcomes)
	}
}

// changesServer is a sync server of the Worker's protocol over a folder, with its change counter; it counts how
// often the counter was asked for.
type changesServer struct {
	mu      sync.Mutex
	dir     DirRemote
	changes int64
	asked   int
}

func (s *changesServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.URL.Path == "/changes" {
		s.asked++
		json.NewEncoder(w).Encode(map[string]int64{"changes": s.changes})
		return
	}
	name := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/files"), "/")
	prev := strings.Trim(r.Header.Get("If-Match"), `"`)
	var err error
	switch {
	case name == "":
		var list map[string]RemoteEntry
		if list, err = s.dir.List(); err == nil {
			json.NewEncoder(w).Encode(list)
		}
	case r.Method == "GET":
		var b []byte
		if b, err = s.dir.Get(name); err == nil {
			w.Write(b)
		}
	case r.Method == "PUT":
		b, _ := io.ReadAll(r.Body)
		if err = s.dir.Put(name, b, prev); err == nil {
			s.changes++
		}
	case r.Method == "DELETE":
		if err = s.dir.Delete(name, prev); err == nil {
			s.changes++
		}
	}
	switch {
	case errors.Is(err, library.ErrConflict):
		http.Error(w, "stale", http.StatusPreconditionFailed)
	case err != nil:
		http.Error(w, err.Error(), http.StatusNotFound)
	}
}

// While the window is in front, the server is asked every syncPoll whether anything was written; a pass starts
// when the counter moved, and none when it did not. Behind other windows, it is not asked.
func TestSyncWhenTheServerChanged(t *testing.T) {
	library.AllowHTTP = true
	defer func() { library.AllowHTTP = false }()
	srv := &changesServer{dir: DirRemote{Root: t.TempDir()}}
	hs := httptest.NewServer(srv)
	defer hs.Close()
	server := func(a *app) {
		a.settings.SyncMode, a.settings.SyncURL, a.settings.SyncToken = "server", hs.URL, "jeton"
	}
	a, b := syncApp(t, ""), syncApp(t, "")
	server(a)
	server(b)
	now := time.Now()
	b.clock = func() time.Time { return now }
	tt := ui.NewTester(b.view, 900, 700)
	tt.Frame() // the pass at the start
	waitSync(t, b)
	asked := func() int { srv.mu.Lock(); defer srv.mu.Unlock(); return srv.asked }
	tick := func() {
		now = now.Add(syncPoll + time.Second)
		tt.Move(float32(now.Second()%50+100), 100) // one is here, doing something
		tt.Frame()
		for i := 0; i < 200; i++ { // the answer, then the frame it asks for
			b.mu.Lock()
			asking := b.sy.asking
			b.mu.Unlock()
			if !asking {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
		waitSync(t, b)
	}
	settle := func() {
		for range 2 {
			now = now.Add(syncQuiet + time.Second)
			tt.Frame()
			waitSync(t, b)
		}
	}
	last := b.sy.last
	tick()
	if asked() == 0 {
		t.Fatal("the server was not asked")
	}
	if b.sy.last != last {
		t.Fatal("a pass started though nothing changed")
	}
	// still: the next ask waits longer
	n := asked()
	tick()
	if asked() != n {
		t.Fatal("asked again though the counter stayed still")
	}
	c, _ := a.lib.NewCollection("Shared")
	p, _ := c.NewPage(nil, "From A")
	p.SaveText("words\n")
	a.syncNow()
	waitSync(t, a)
	tick()
	if b.sy.last == last {
		t.Fatal("no pass after the server changed")
	}
	found := false
	for _, co := range b.lib.Collections {
		for _, q := range co.All() {
			found = found || q.Title == "From A"
		}
	}
	if !found {
		t.Fatalf("the page did not come: %q", b.sy.err)
	}
	// behind other windows: not asked
	b.sy.behind = true
	settle() // the pass for what came down, which reads the counter too
	n = asked()
	tick()
	if asked() != n {
		t.Fatal("asked behind other windows")
	}
	// in front, but nobody doing anything: not asked; the first move asks
	b.sy.behind = false
	now = now.Add(syncIdle)
	settle()
	n = asked()
	now = now.Add(syncPollMost)
	tt.Frame()
	time.Sleep(20 * time.Millisecond)
	if asked() != n {
		t.Fatal("asked while nobody was there")
	}
	tt.Move(300, 300)
	tt.Frame()
	waitSync(t, b)
	for i := 0; i < 200 && asked() == n; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if asked() == n {
		t.Fatal("not asked when one came back")
	}
}

// folderSyncApp is an app over a folder library, its writes counted as main counts them, its settings in a
// folder of the test's, syncing to the folder remoteDir.
func folderSyncApp(t *testing.T, remoteDir string) *app {
	lib, err := OpenLibraryIn(&countFS{FS: OSFS{}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.settingsFile = filepath.Join(t.TempDir(), "celestine", "settings.json")
	a.settings.SyncMode, a.settings.SyncDir = "dir", remoteDir
	a.settings.SyncKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	return a
}

// A folder library syncs as a library file does: what is written goes up once the writing is quiet, its state is
// kept in the config folder, and a page changed on both sides is merged.
func TestSyncFolderLibrary(t *testing.T) {
	dir := t.TempDir()
	a, b := folderSyncApp(t, dir), folderSyncApp(t, dir)
	now := time.Now()
	a.clock = func() time.Time { return now }
	tt := ui.NewTester(a.view, 900, 700)
	tt.Frame() // the pass at the start
	waitSync(t, a)
	if a.sy.err != "" {
		t.Fatalf("the pass: %s", a.sy.err)
	}
	c, _ := a.lib.NewCollection("Shared")
	p, _ := c.NewPage(nil, "Written")
	p.SaveText("one\n\ntwo\n\nthree\n")
	if a.libWrites() == a.sy.writes {
		t.Fatal("the writes were not counted")
	}
	tt.Frame() // the writing is seen
	now = now.Add(syncQuiet + time.Second)
	tt.Frame()
	waitSync(t, a)
	if a.sy.err != "" {
		t.Fatalf("the pass: %s", a.sy.err)
	}
	if _, err := os.Stat(filepath.Join(a.syncStateDir(), "state.json")); err != nil {
		t.Fatalf("the state is not in the config folder: %v", err)
	}
	b.syncNow()
	waitSync(t, b)
	page := func(x *app) *Page {
		for _, co := range x.lib.Collections {
			for _, q := range co.All() {
				if q.Title == "Written" {
					return q
				}
			}
		}
		return nil
	}
	pb := page(b)
	if pb == nil {
		t.Fatalf("the page did not come: %q", b.sy.err)
	}
	// both change it, each a block of its own: merged, with no conflict
	edit := func(x *app, q *Page, from, to string) {
		src, err := x.lib.FS.ReadFile(filepath.Join(q.Dir, PageFileName))
		if err != nil {
			t.Fatal(err)
		}
		if err := x.lib.FS.WriteFile(filepath.Join(q.Dir, PageFileName), []byte(strings.Replace(string(src), from, to, 1))); err != nil {
			t.Fatal(err)
		}
	}
	edit(a, page(a), `"one"`, `"ONE"`)
	edit(b, pb, `"three"`, `"THREE"`)
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	src, _ := b.lib.FS.ReadFile(filepath.Join(page(b).Dir, PageFileName))
	if !strings.Contains(string(src), `"ONE"`) || !strings.Contains(string(src), `"THREE"`) || len(b.conflictFiles()) > 0 {
		t.Fatalf("not merged (%q): %s", b.sy.err, src)
	}
}

// A remote that serves an older index stops the pass with its own message, and the user's "trust the remote
// again" takes it at the next pass, after which the passes go on (#148).
func TestSyncTrustRemoteAgain(t *testing.T) {
	dir := t.TempDir()
	a := syncApp(t, dir)
	c, _ := a.lib.NewCollection("Not")
	p, _ := c.NewPage(nil, "Bir")
	p.SaveText("bir\n")
	a.syncNow()
	waitSync(t, a)
	before, err := os.ReadFile(filepath.Join(dir, IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	q, _ := c.NewPage(nil, "İki")
	q.SaveText("iki\n")
	a.syncNow()
	waitSync(t, a)
	os.WriteFile(filepath.Join(dir, IndexFile), before, 0o644) // the remote restored from a backup
	a.syncNow()
	waitSync(t, a)
	if !a.sy.rolledBack || a.sy.err != locales.SyncErrIndexRollback() {
		t.Fatalf("the older index: rolled back %v, err %q", a.sy.rolledBack, a.sy.err)
	}
	a.sy.trustIndex = true // the button in the sync settings
	a.syncNow()
	waitSync(t, a)
	if a.sy.rolledBack || a.sy.trustIndex {
		t.Fatalf("trusted again: rolled back %v, err %q", a.sy.rolledBack, a.sy.err)
	}
	// the page the older index left out is still here: its name goes in again before the remote is listed
	if pages := a.lib.Collections[0].All(); len(pages) != 2 {
		t.Fatalf("trusted again, the library holds %d pages", len(pages))
	}
	a.syncNow()
	waitSync(t, a)
	if a.sy.rolledBack || len(a.sy.refused) > 0 {
		t.Fatalf("the pass after: err %q", a.sy.err)
	}
	// a pass that does not reach the remote leaves what the last one that did refused, and its button
	a.sy.refused = []string{"Not/Bir/page.json"}
	a.settings.SyncMode, a.settings.SyncURL, a.settings.SyncToken = "server", "http://127.0.0.1:1", "x"
	a.syncNow()
	waitSync(t, a)
	if len(a.sy.refused) != 1 {
		t.Fatalf("a pass that did not reach the remote forgot the refusal: err %q", a.sy.err)
	}
}

// The sync settings offer the way out of a refusal: the message, and the button that trusts the remote again.
func TestSyncTrustRemoteAgainShown(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode = "dir"
	a.sy.refused = []string{"Not/Bir/page.json", "Not/İki/page.json"}
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{locales.SyncRefusedFiles(2), locales.SyncTrustRemoteAgain(), "Not/İki/page.json"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
}

// A sync folder that is not there (its drive not connected) is offline, with its own message, and is not made.
func TestSyncFolderNotThere(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Volume")
	a := syncApp(t, dir)
	a.syncNow()
	waitSync(t, a)
	if a.sy.err != locales.SyncErrNoFolder() {
		t.Fatalf("a folder not there: %q", a.sy.err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the pass made the folder")
	}
	// the user makes it, from Settings, and the next pass runs with it (#568)
	if !a.sy.noFolder {
		t.Fatal("no folder, and nothing offers to make it")
	}
	a.makeSyncDir()
	for i := 0; i < 200; i++ {
		a.mu.Lock()
		made := a.sy.err == ""
		a.mu.Unlock()
		if made {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("the folder was not made: %v", err)
	}
	a.syncNow()
	waitSync(t, a)
	if a.sy.err != "" || a.sy.noFolder {
		t.Fatalf("the pass after the folder was made: %q", a.sy.err)
	}
}

// Settings offer to make the sync folder that a pass did not find, in folder mode only.
func TestSyncFolderNotThereButton(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode, a.settings.SyncDir = "dir", filepath.Join(t.TempDir(), "New")
	a.sy.noFolder, a.sy.err = true, locales.SyncErrNoFolder()
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	r, ok := tt.Find(locales.SyncCreateFolder())
	if !ok {
		t.Fatalf("no button to make the folder: %v", tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(a.settings.SyncDir); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(a.settings.SyncDir); err != nil {
		t.Fatalf("the button did not make the folder: %v", err)
	}
	tt.Advance(100 * time.Millisecond)
	if tt.HasText(locales.SyncCreateFolder()) {
		t.Fatal("the button stays once the folder is made")
	}
	a.settings.SyncMode = "server"
	a.sy.noFolder = true
	tt.Advance(100 * time.Millisecond)
	if tt.HasText(locales.SyncCreateFolder()) {
		t.Fatal("a server has no folder to make")
	}
}

// A pass that waits for the remote's hashes (a folder bringing them late) does not read as synced: what is to come
// down has not (#568).
func TestSyncWaitingForTheHashes(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	c, _ := a.lib.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Bir")
	p.SaveText("bir\n")
	a.syncNow()
	waitSync(t, a)
	b.syncNow()
	waitSync(t, b)
	q, _ := c.NewPage(nil, "İki")
	q.SaveText("iki\n")
	a.syncNow()
	waitSync(t, a)
	if err := os.Remove(filepath.Join(dir, library.HashesFile)); err != nil {
		t.Fatal(err)
	}
	b.syncNow()
	waitSync(t, b)
	if b.sy.err != "" || b.sy.status != locales.SyncWaitingForRemote() {
		t.Fatalf("waiting for the hashes: status %q, err %q", b.sy.status, b.sy.err)
	}
}

// A page's name on the remote is its path from the sync's root, in a library's file and in a folder library alike:
// the folder's own path is not part of it (#163).
func TestSyncName(t *testing.T) {
	a := syncApp(t, t.TempDir())
	if got := a.syncName("/Not/Bir/page.json"); got != "Not/Bir/page.json" {
		t.Errorf("in the store: %q", got)
	}
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := newApp(lib)
	file := filepath.Join(lib.Root, "Not", "Bir", PageFileName)
	if got := b.syncName(file); got != "Not/Bir/"+PageFileName {
		t.Errorf("in a folder: %q", got)
	}
}

// A server that no longer takes this device's token says so, and is not "offline" (#155).
func TestSyncDeviceRefused(t *testing.T) {
	library.AllowHTTP = true
	defer func() { library.AllowHTTP = false }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusUnauthorized) }))
	defer srv.Close()
	a := syncApp(t, t.TempDir())
	a.settings.SyncMode, a.settings.SyncURL, a.settings.SyncToken = "server", srv.URL, "revoked"
	a.syncNow()
	waitSync(t, a)
	if a.sy.err != locales.SyncErrDeviceRefused() || a.sy.status == locales.SyncOffline() {
		t.Fatalf("a refused token: err %q, status %q", a.sy.err, a.sy.status)
	}
}

// A pass takes its backup of the library only when it is about to change something here: sending up what was written
// takes none, bringing down what another device wrote takes one (#150).
func TestSyncBackupOnlyBeforeAChange(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	count := func(x *app) int {
		ents, _ := os.ReadDir(x.backupDirOf())
		n := 0
		for _, e := range ents {
			if strings.Contains(e.Name(), "senkron") {
				n++
			}
		}
		return n
	}
	c, _ := a.lib.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Bir")
	p.SaveText("bir\n")
	a.syncNow()
	waitSync(t, a)
	if a.sy.err != "" || count(a) != 0 {
		t.Fatalf("a pass that only sent: err %q, %d backups", a.sy.err, count(a))
	}
	b.syncNow()
	waitSync(t, b)
	if b.sy.err != "" || count(b) != 1 {
		t.Fatalf("a pass that brought files down: err %q, %d backups", b.sy.err, count(b))
	}
}
