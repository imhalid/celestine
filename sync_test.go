package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A device is a folder that stands for a library, with what it knew at its last sync.
type device struct {
	t    *testing.T
	root string
	st   syncState
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
	res, err := SyncOnce(osFS{}, d.root, r, &d.st, sk, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))
	if err != nil {
		d.t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		d.t.Fatalf("errors: %v", res.Errors)
	}
	return res
}

func TestSyncTwoDevices(t *testing.T) {
	remote := dirRemote{t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/collection.json", `{"name":"Not"}`)
	a.write("Not/Bir/page.json", "bir")
	if r := a.sync(remote); r.Pushed != 2 {
		t.Fatalf("push: %+v", r)
	}
	if r := b.sync(remote); r.Pulled != 2 || b.read("Not/Bir/page.json") != "bir" {
		t.Fatalf("pull: %+v %q", r, b.read("Not/Bir/page.json"))
	}
	if r := a.sync(remote); r.Pushed+r.Pulled != 0 {
		t.Fatalf("a second pass did something: %+v", r)
	}
	// B edits, A gets it
	b.write("Not/Bir/page.json", "bir, düzeltildi")
	b.sync(remote)
	if r := a.sync(remote); r.Pulled != 1 || a.read("Not/Bir/page.json") != "bir, düzeltildi" {
		t.Fatalf("%+v %q", r, a.read("Not/Bir/page.json"))
	}
	// a new file on A, and a deletion on B
	a.write("Not/Iki/page.json", "iki")
	a.sync(remote)
	os.Remove(filepath.Join(b.root, "Not", "Bir", "page.json"))
	b.sync(remote)
	b.sync(remote)
	if b.read("Not/Iki/page.json") != "iki" {
		t.Fatal("the new file did not come")
	}
	if r := a.sync(remote); r.Removed != 1 || a.read("Not/Bir/page.json") != "<yok>" {
		t.Fatalf("the deletion did not go: %+v %q", r, a.read("Not/Bir/page.json"))
	}
	// and it is not brought back by anyone
	b.sync(remote)
	a.sync(remote)
	if b.read("Not/Bir/page.json") != "<yok>" || a.read("Not/Bir/page.json") != "<yok>" {
		t.Fatal("a deleted file came back")
	}
}

// Both sides changed one file: the remote's stays, ours is kept beside it, and goes up too: nothing is lost.
func TestSyncConflictLosesNothing(t *testing.T) {
	remote := dirRemote{t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Bir/page.json", "ilk")
	a.sync(remote)
	b.sync(remote)
	a.write("Not/Bir/page.json", "A'nın yazdığı")
	b.write("Not/Bir/page.json", "B'nin yazdığı")
	a.sync(remote) // A is first
	r := b.sync(remote)
	if len(r.Conflicts) != 1 {
		t.Fatalf("no conflict found: %+v", r)
	}
	if b.read("Not/Bir/page.json") != "A'nın yazdığı" {
		t.Fatalf("the remote's did not stay: %q", b.read("Not/Bir/page.json"))
	}
	copyName := "Not/Bir/page (çakışma 2026-11-07 10-00-00).json"
	if b.read(copyName) != "B'nin yazdığı" {
		t.Fatalf("ours was not kept: %q", b.read(copyName))
	}
	b.sync(remote)
	a.sync(remote)
	if a.read(copyName) != "B'nin yazdığı" {
		t.Fatal("the conflict copy did not reach the other side")
	}
	// they deleted, we changed: ours stays
	a.write("Not/Iki/page.json", "x")
	a.sync(remote)
	b.sync(remote)
	os.Remove(filepath.Join(a.root, "Not", "Iki", "page.json"))
	a.sync(remote)
	b.write("Not/Iki/page.json", "x ve devamı")
	b.sync(remote)
	b.sync(remote)
	if b.read("Not/Iki/page.json") != "x ve devamı" {
		t.Fatalf("a change was lost to a deletion: %q", b.read("Not/Iki/page.json"))
	}
}

// A collection chosen out stays here: nothing of it goes up, nothing of it comes down.
func TestSyncSelective(t *testing.T) {
	remote := dirRemote{t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Gunluk/g.json", "herkes")
	a.write("Ozel/o.json", "yalnız bende")
	a.sync(remote, "Ozel")
	b.sync(remote)
	if b.read("Gunluk/g.json") != "herkes" || b.read("Ozel/o.json") != "<yok>" {
		t.Fatal("the choice was not kept")
	}
	if _, err := os.Stat(filepath.Join(remote.root, "Ozel")); err == nil {
		t.Fatal("what was chosen out was sent")
	}
	// choosing it out later does not delete it from the other side
	a.sync(remote, "Ozel")
	if b.sync(remote); b.read("Gunluk/g.json") != "herkes" {
		t.Fatal("lost")
	}
}

// A remote that cannot be reached is an error; what was done here waits, and is sent when it can be.
type flaky struct {
	Remote
	down bool
}

func (f *flaky) List() (map[string]RemoteEntry, error) {
	if f.down {
		return nil, io.ErrUnexpectedEOF
	}
	return f.Remote.List()
}

func TestSyncOffline(t *testing.T) {
	inner := dirRemote{t.TempDir()}
	r := &flaky{Remote: inner, down: true}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Bir/page.json", "çevrimdışı yazıldı")
	if _, err := SyncOnce(osFS{}, a.root, r, &a.st, nil, time.Now()); err == nil {
		t.Fatal("no error offline")
	}
	if a.read("Not/Bir/page.json") != "çevrimdışı yazıldı" {
		t.Fatal("the file was touched")
	}
	r.down = false
	a.sync(r)
	b.sync(r)
	if b.read("Not/Bir/page.json") != "çevrimdışı yazıldı" {
		t.Fatal("not sent when back")
	}
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

// Encrypted, the remote holds nothing readable: not the text, not the names; and it still syncs.
func TestSyncEncrypted(t *testing.T) {
	dir := t.TempDir()
	inner := dirRemote{dir}
	open := func(pass string) *encRemote {
		e, err := openEnc(inner, pass)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	a, b := newDevice(t), newDevice(t)
	a.write("Gizli koleksiyon/Banka şifrem/page.json", "çok gizli metin 12345")
	a.sync(open("parola"))
	names, all := allRemoteBytes(t, dir)
	for _, n := range names {
		if strings.Contains(n, "Gizli") || strings.Contains(n, "Banka") || strings.Contains(n, "page") {
			t.Fatalf("a readable name on the remote: %q", n)
		}
	}
	if bytes.Contains(all, []byte("gizli")) || bytes.Contains(all, []byte("12345")) || bytes.Contains(all, []byte("Banka")) {
		t.Fatal("readable text on the remote")
	}
	b.sync(open("parola"))
	if b.read("Gizli koleksiyon/Banka şifrem/page.json") != "çok gizli metin 12345" {
		t.Fatal("did not come through the encryption")
	}
	// a second pass changes nothing, though the sealed bytes of each pass differ
	if r := a.sync(open("parola")); r.Pushed+r.Pulled+len(r.Conflicts) != 0 {
		t.Fatalf("a second pass did something: %+v", r)
	}
	// the wrong passphrase gets nothing, and says so
	bad := newDevice(t)
	if e, err := openEnc(inner, "yanlış"); err == nil {
		if _, err := SyncOnce(osFS{}, bad.root, e, &bad.st, nil, time.Now()); err == nil {
			t.Fatal("the wrong passphrase read the library")
		}
	}
	if bad.read("Gizli koleksiyon/Banka şifrem/page.json") != "<yok>" {
		t.Fatal("plaintext came with a wrong passphrase")
	}
	// the recovery key opens it, too
	salt, _ := inner.Get(saltFile)
	key, _ := deriveKey("parola", salt)
	words := recoveryKey(key)
	back, err := keyFromRecovery(words)
	if err != nil || !bytes.Equal(back, key) {
		t.Fatalf("recovery: %q %v", words, err)
	}
	enc, _ := newEncRemote(inner, back)
	rec := newDevice(t)
	rec.sync(enc)
	if rec.read("Gizli koleksiyon/Banka şifrem/page.json") != "çok gizli metin 12345" {
		t.Fatal("the recovery key did not open it")
	}
	if _, err := keyFromRecovery("kısa"); err == nil {
		t.Fatal("a bad recovery key accepted")
	}
}

// The server of the protocol, in memory: what worker.js does.
type fakeServer struct {
	mu    sync.Mutex
	files map[string][]byte
	dead  map[string]bool
	token string
}

func (s *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+s.token {
		http.Error(w, "no", 401)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := strings.TrimPrefix(r.URL.Path, "/files")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		out := map[string]RemoteEntry{}
		for n, b := range s.files {
			out[n] = RemoteEntry{Hash: hashOf(b)}
		}
		for n := range s.dead {
			out[n] = RemoteEntry{Deleted: true}
		}
		json.NewEncoder(w).Encode(out)
		return
	}
	cur := ""
	if b, ok := s.files[name]; ok {
		cur = hashOf(b)
	}
	if m := r.Header.Get("If-Match"); r.Method != "GET" && strings.Trim(m, `"`) != cur {
		http.Error(w, "stale", 412)
		return
	}
	switch r.Method {
	case "GET":
		b, ok := s.files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	case "PUT":
		b, _ := io.ReadAll(r.Body)
		s.files[name] = b
		delete(s.dead, name)
	case "DELETE":
		delete(s.files, name)
		s.dead[name] = true
	}
}

func TestSyncOverHTTP(t *testing.T) {
	allowHTTP = true
	defer func() { allowHTTP = false }()
	fs := &fakeServer{files: map[string][]byte{}, dead: map[string]bool{}, token: "gizli-jeton"}
	srv := httptest.NewServer(fs)
	defer srv.Close()
	a, b := newDevice(t), newDevice(t)
	ra, rb := newHTTPRemote(srv.URL, "gizli-jeton"), newHTTPRemote(srv.URL, "gizli-jeton")
	a.write("Not/Türkçe ad ve boşluk/page.json", "sunucuda")
	if r := a.sync(ra); r.Pushed != 1 {
		t.Fatalf("%+v", r)
	}
	if r := b.sync(rb); r.Pulled != 1 || b.read("Not/Türkçe ad ve boşluk/page.json") != "sunucuda" {
		t.Fatalf("%+v", r)
	}
	os.Remove(filepath.Join(a.root, "Not", "Türkçe ad ve boşluk", "page.json"))
	a.sync(ra)
	if r := b.sync(rb); r.Removed != 1 {
		t.Fatalf("deletion over HTTP: %+v", r)
	}
	wrong := newHTTPRemote(srv.URL, "başka")
	if _, err := wrong.List(); err == nil {
		t.Fatal("a wrong token listed the files")
	}
	allowHTTP = false
	if _, err := ra.List(); err == nil {
		t.Fatal("plain HTTP used")
	}
}

// The app's side: two libraries in files, a page written in one is seen in the other.
func TestSyncLibraries(t *testing.T) {
	remote := dirRemote{t.TempDir()}
	mk := func() (*Store, *Library, *syncState) {
		st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		lib, err := OpenLibraryIn(st, storeRoot)
		if err != nil {
			t.Fatal(err)
		}
		return st, lib, &syncState{}
	}
	sa, la, ta := mk()
	sb, _, tb := mk()
	c, _ := la.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Birinci")
	p.SaveText("iki cihazda görünecek\n")
	if _, err := SyncOnce(sa, storeRoot, remote, ta, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	res, err := SyncOnce(sb, storeRoot, remote, tb, nil, time.Now())
	if err != nil || res.Pulled == 0 {
		t.Fatalf("%+v %v", res, err)
	}
	lb, err := OpenLibraryIn(sb, storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	var found *Page
	for _, cc := range lb.Collections {
		for _, pp := range cc.All() {
			found = pp
		}
	}
	if found == nil || found.Title != "Birinci" {
		t.Fatalf("the page is not in the other library: %+v", lb.Collections)
	}
	if txt, _ := found.Text(); !strings.Contains(txt, "iki cihazda") {
		t.Fatalf("text %q", txt)
	}
}

func syncApp(t *testing.T, remoteDir string) *app {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, storeRoot)
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

// Two devices that open the sealed remote before either has put anything, then each put a file: the
// index must hold both, or the file of the one is no longer seen (and its owner would take it for deleted).
func TestSyncEncryptedIndexRace(t *testing.T) {
	inner := dirRemote{t.TempDir()}
	open := func() *encRemote {
		e, err := openEnc(inner, "parola")
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	ea, eb := open(), open()
	if _, err := ea.List(); err != nil { // both read the index (empty) first
		t.Fatal(err)
	}
	if _, err := eb.List(); err != nil {
		t.Fatal(err)
	}
	if err := ea.Put("a/x.json", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	if err := eb.Put("b/y.json", []byte("y"), ""); err != nil {
		t.Fatal(err)
	}
	if err := ea.Flush(); err != nil { // the index is put at the end of a pass, as SyncOnce does
		t.Fatal(err)
	}
	if err := eb.Flush(); err != nil {
		t.Fatal(err)
	}
	list, err := open().List()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := list["a/x.json"]; !ok {
		t.Fatalf("the first device's file fell out of the index: %v", list)
	}
	if _, ok := list["b/y.json"]; !ok {
		t.Fatalf("the second device's file fell out of the index: %v", list)
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

// listCounter counts the remote's listings.
type listCounter struct {
	Remote
	lists int
}

func (l *listCounter) List() (map[string]RemoteEntry, error) { l.lists++; return l.Remote.List() }

// A first pass of many files puts the sealed index a few times, not once a file: each put lists the remote
// and merges, and once a file a library of thousands took minutes. Another device sees them all after it.
func TestSyncEncryptedIndexInSteps(t *testing.T) {
	a := newDevice(t)
	const n = 450
	for i := 0; i < n; i++ {
		a.write(fmt.Sprintf("notlar/s%03d/page.json", i), "x")
	}
	inner := &listCounter{Remote: dirRemote{t.TempDir()}}
	e, err := openEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	if res := a.sync(e); res.Pushed != n {
		t.Fatalf("pushed %d", res.Pushed)
	}
	if inner.lists > 12 {
		t.Fatalf("the remote was listed %d times for %d files", inner.lists, n)
	}
	b := newDevice(t)
	e2, err := openEnc(inner.Remote, "parola")
	if err != nil {
		t.Fatal(err)
	}
	if res := b.sync(e2); res.Pulled != n {
		t.Fatalf("the other device got %d of %d", res.Pulled, n)
	}
}

// indexFails is a remote whose sealed index cannot be written while fail is set.
type indexFails struct {
	Remote
	fail bool
}

func (r *indexFails) Put(name string, data []byte, prev string) error {
	if r.fail && name == indexFile {
		return errors.New("ağ yok")
	}
	return r.Remote.Put(name, data, prev)
}

// The index of an encrypted remote not written at the end of a pass: what went up is not taken as
// sent, so the next pass sends it again, and never deletes it here as if it were deleted there.
func TestSyncIndexFailureKeepsLocalFiles(t *testing.T) {
	a := newDevice(t)
	a.write("notlar/bir/page.json", "bir")
	inner := &indexFails{Remote: dirRemote{t.TempDir()}}
	e, err := openEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	inner.fail = true
	if _, err := SyncOnce(osFS{}, a.root, e, &a.st, map[string]bool{}, time.Now()); err == nil {
		t.Fatal("no error for an index not written")
	}
	if _, ok := a.st.Files["notlar/bir/page.json"]; ok {
		t.Fatal("the file is taken as sent, its name not on the remote")
	}
	// the app opens the remote anew for the next pass, its index read from it
	inner.fail = false
	e2, err := openEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	res := a.sync(e2)
	if a.read("notlar/bir/page.json") != "bir" || res.Removed != 0 || res.Pushed != 1 {
		t.Fatalf("after: %q, %+v", a.read("notlar/bir/page.json"), res)
	}
	b := newDevice(t)
	e3, _ := openEnc(inner, "parola")
	if b.sync(e3); b.read("notlar/bir/page.json") != "bir" {
		t.Fatal("the other device did not get it")
	}
}
