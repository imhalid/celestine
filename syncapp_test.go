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
	if !strings.HasPrefix(a.sy.status, "Eşit") {
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
	if b.sy.conflict != 1 || !strings.HasPrefix(b.sy.status, "çakışma") {
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

// With encryption on from the passphrase, two apps agree, and the folder holds nothing readable.
func TestSyncAppsEncrypted(t *testing.T) {
	dir := t.TempDir()
	a, b := syncApp(t, dir), syncApp(t, dir)
	a.setSyncPassphrase("parola")
	if a.settings.SyncKey == "" || a.sy.recovery == "" {
		t.Fatalf("no key: %q", a.notice)
	}
	b.setSyncPassphrase("parola")
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
	// a wrong key is an error, not a library
	c2 := syncApp(t, dir)
	c2.setSyncPassphrase("yanlış")
	c2.syncNow()
	waitSync(t, c2)
	if c2.sy.err == "" || len(c2.lib.Collections) != 0 {
		t.Fatalf("the wrong passphrase: err %q collections %d", c2.sy.err, len(c2.lib.Collections))
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
