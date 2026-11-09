package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
	now = now.Add(4 * time.Second)
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
