package library

import (
	"bytes"
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

func TestSyncTwoDevices(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
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
	remote := DirRemote{Root: t.TempDir()}
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
	remote := DirRemote{Root: t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Gunluk/g.json", "herkes")
	a.write("Ozel/o.json", "yalnız bende")
	a.sync(remote, "Ozel")
	b.sync(remote)
	if b.read("Gunluk/g.json") != "herkes" || b.read("Ozel/o.json") != "<yok>" {
		t.Fatal("the choice was not kept")
	}
	if _, err := os.Stat(filepath.Join(remote.Root, "Ozel")); err == nil {
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
	inner := DirRemote{Root: t.TempDir()}
	r := &flaky{Remote: inner, down: true}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Bir/page.json", "çevrimdışı yazıldı")
	if _, err := SyncOnce(OSFS{}, a.root, r, &a.st, nil, time.Now()); err == nil {
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
	inner := DirRemote{Root: dir}
	open := func(pass string) *EncRemote {
		e, err := OpenEnc(inner, pass)
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
	if e, err := OpenEnc(inner, "yanlış"); err == nil {
		if _, err := SyncOnce(OSFS{}, bad.root, e, &bad.st, nil, time.Now()); err == nil {
			t.Fatal("the wrong passphrase read the library")
		}
	}
	if bad.read("Gizli koleksiyon/Banka şifrem/page.json") != "<yok>" {
		t.Fatal("plaintext came with a wrong passphrase")
	}
	// the recovery key opens it, too
	salt, _ := inner.Get(SaltFile)
	key, _ := DeriveKey("parola", salt)
	words := RecoveryKey(key)
	back, err := KeyFromRecovery(words)
	if err != nil || !bytes.Equal(back, key) {
		t.Fatalf("recovery: %q %v", words, err)
	}
	enc, _ := NewEncRemote(inner, back)
	rec := newDevice(t)
	rec.sync(enc)
	if rec.read("Gizli koleksiyon/Banka şifrem/page.json") != "çok gizli metin 12345" {
		t.Fatal("the recovery key did not open it")
	}
	if _, err := KeyFromRecovery("kısa"); err == nil {
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
			out[n] = RemoteEntry{Hash: HashOf(b)}
		}
		for n := range s.dead {
			out[n] = RemoteEntry{Deleted: true}
		}
		json.NewEncoder(w).Encode(out)
		return
	}
	cur := ""
	if b, ok := s.files[name]; ok {
		cur = HashOf(b)
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
	AllowHTTP = true
	defer func() { AllowHTTP = false }()
	fs := &fakeServer{files: map[string][]byte{}, dead: map[string]bool{}, token: "gizli-jeton"}
	srv := httptest.NewServer(fs)
	defer srv.Close()
	a, b := newDevice(t), newDevice(t)
	ra, rb := NewHTTPRemote(srv.URL, "gizli-jeton"), NewHTTPRemote(srv.URL, "gizli-jeton")
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
	wrong := NewHTTPRemote(srv.URL, "başka")
	if _, err := wrong.List(); err == nil {
		t.Fatal("a wrong token listed the files")
	}
	AllowHTTP = false
	if _, err := ra.List(); err == nil {
		t.Fatal("plain HTTP used")
	}
}

// The app's side: two libraries in files, a page written in one is seen in the other.
func TestSyncLibraries(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
	mk := func() (*Store, *Library, *SyncState) {
		st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		lib, err := OpenLibraryIn(st, StoreRoot)
		if err != nil {
			t.Fatal(err)
		}
		return st, lib, &SyncState{}
	}
	sa, la, ta := mk()
	sb, _, tb := mk()
	c, _ := la.NewCollection("Ortak")
	p, _ := c.NewPage(nil, "Birinci")
	p.SaveText("iki cihazda görünecek\n")
	if _, err := SyncOnce(sa, StoreRoot, remote, ta, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	res, err := SyncOnce(sb, StoreRoot, remote, tb, nil, time.Now())
	if err != nil || res.Pulled == 0 {
		t.Fatalf("%+v %v", res, err)
	}
	lb, err := OpenLibraryIn(sb, StoreRoot)
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

// Two devices that open the sealed remote before either has put anything, then each put a file: the
// index must hold both, or the file of the one is no longer seen (and its owner would take it for deleted).
func TestSyncEncryptedIndexRace(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	open := func() *EncRemote {
		e, err := OpenEnc(inner, "parola")
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
	inner := &listCounter{Remote: DirRemote{Root: t.TempDir()}}
	e, err := OpenEnc(inner, "parola")
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
	e2, err := OpenEnc(inner.Remote, "parola")
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
	if r.fail && name == IndexFile {
		return errors.New("ağ yok")
	}
	return r.Remote.Put(name, data, prev)
}

// The index of an encrypted remote not written at the end of a pass: what went up is not taken as
// sent, so the next pass sends it again, and never deletes it here as if it were deleted there.
func TestSyncIndexFailureKeepsLocalFiles(t *testing.T) {
	a := newDevice(t)
	a.write("notlar/bir/page.json", "bir")
	inner := &indexFails{Remote: DirRemote{Root: t.TempDir()}}
	e, err := OpenEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	inner.fail = true
	if _, err := SyncOnce(OSFS{}, a.root, e, &a.st, map[string]bool{}, time.Now()); err == nil {
		t.Fatal("no error for an index not written")
	}
	if _, ok := a.st.Files["notlar/bir/page.json"]; ok {
		t.Fatal("the file is taken as sent, its name not on the remote")
	}
	// the app opens the remote anew for the next pass, its index read from it
	inner.fail = false
	e2, err := OpenEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	res := a.sync(e2)
	if a.read("notlar/bir/page.json") != "bir" || res.Removed != 0 || res.Pushed != 1 {
		t.Fatalf("after: %q, %+v", a.read("notlar/bir/page.json"), res)
	}
	b := newDevice(t)
	e3, _ := OpenEnc(inner, "parola")
	if b.sync(e3); b.read("notlar/bir/page.json") != "bir" {
		t.Fatal("the other device did not get it")
	}
}

// The same, and the file is written again before the next pass: what is left on the remote by the pass
// that failed is older than it, and is written over, not met as a conflict for ever.
func TestSyncIndexFailureThenEdited(t *testing.T) {
	a := newDevice(t)
	a.write("notlar/bir/page.json", "bir")
	inner := &indexFails{Remote: DirRemote{Root: t.TempDir()}}
	e, err := OpenEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	inner.fail = true
	if _, err := SyncOnce(OSFS{}, a.root, e, &a.st, map[string]bool{}, time.Now()); err == nil {
		t.Fatal("no error for an index not written")
	}
	a.write("notlar/bir/page.json", "bir, ve daha")
	inner.fail = false
	e2, err := OpenEnc(inner, "parola")
	if err != nil {
		t.Fatal(err)
	}
	res := a.sync(e2)
	if len(res.Errors) != 0 || res.Pushed != 1 || res.Removed != 0 || a.read("notlar/bir/page.json") != "bir, ve daha" {
		t.Fatalf("after: %q, %+v", a.read("notlar/bir/page.json"), res)
	}
	b := newDevice(t)
	e3, _ := OpenEnc(inner, "parola")
	if b.sync(e3); b.read("notlar/bir/page.json") != "bir, ve daha" {
		t.Fatalf("the other device got %q", b.read("notlar/bir/page.json"))
	}
}

// A remote that lost many files with no tombstone (its list gone, a folder not mounted) is not a deletion
// there: the pass holds them back and counts them, and deletes them here only when told to.
func TestSyncHoldsMassRemoval(t *testing.T) {
	remoteDir := t.TempDir()
	a := newDevice(t)
	const n = 40
	for i := 0; i < n; i++ {
		a.write(fmt.Sprintf("notlar/s%02d/page.json", i), "x")
	}
	a.sync(DirRemote{Root: remoteDir})
	for i := 0; i < 30; i++ { // gone from the remote, nothing left in its place
		os.RemoveAll(filepath.Join(remoteDir, "notlar", fmt.Sprintf("s%02d", i)))
	}
	res := a.sync(DirRemote{Root: remoteDir})
	if res.Held != 30 || res.Removed != 0 {
		t.Fatalf("held %d removed %d", res.Held, res.Removed)
	}
	for i := 0; i < n; i++ {
		if a.read(fmt.Sprintf("notlar/s%02d/page.json", i)) != "x" {
			t.Fatalf("s%02d was deleted", i)
		}
	}
	if res = a.sync(DirRemote{Root: remoteDir}); res.Held != 30 { // held again: nothing was decided
		t.Fatalf("second pass held %d", res.Held)
	}
	a.st.AllowRemovals = true
	if res = a.sync(DirRemote{Root: remoteDir}); res.Removed != 30 || res.Held != 0 {
		t.Fatalf("allowed: removed %d held %d", res.Removed, res.Held)
	}
	if a.read("notlar/s05/page.json") != "<yok>" || a.read("notlar/s35/page.json") != "x" {
		t.Fatal("the wrong files went")
	}
}

// Few files gone, or many with their tombstones (another device deleted them), are deletions as ever.
func TestSyncMassRemovalNotHeldWhenDeletedThere(t *testing.T) {
	remoteDir := t.TempDir()
	a, b := newDevice(t), newDevice(t)
	const n = 60
	for i := 0; i < n; i++ {
		a.write(fmt.Sprintf("notlar/s%02d/page.json", i), "x")
	}
	a.sync(DirRemote{Root: remoteDir})
	b.sync(DirRemote{Root: remoteDir})
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("notlar/s%02d/page.json", i)
		os.Remove(filepath.Join(b.root, filepath.FromSlash(name)))
	}
	b.sync(DirRemote{Root: remoteDir}) // deleted there, with tombstones
	if res := a.sync(DirRemote{Root: remoteDir}); res.Removed != 40 || res.Held != 0 {
		t.Fatalf("removed %d held %d", res.Removed, res.Held)
	}
	for i := 40; i < 45; i++ { // a few gone with no tombstone: below the line
		os.RemoveAll(filepath.Join(remoteDir, "notlar", fmt.Sprintf("s%02d", i)))
	}
	if res := a.sync(DirRemote{Root: remoteDir}); res.Removed != 5 || res.Held != 0 {
		t.Fatalf("few: removed %d held %d", res.Removed, res.Held)
	}
}

// What a sync takes away from here goes to the trash, as a page thrown away would, and is there to be put back.
func TestSyncRemovalGoesToTrash(t *testing.T) {
	remoteDir := t.TempDir()
	a, b := newDevice(t), newDevice(t)
	a.write("notlar/selam/page.json", `{"meta":{"title":"Selam"},"body":[]}`)
	a.write("notlar/selam/resim.png", "png")
	a.write("notlar/kalan/page.json", `{"meta":{"title":"Kalan"},"body":[]}`)
	a.sync(DirRemote{Root: remoteDir})
	b.sync(DirRemote{Root: remoteDir})
	os.RemoveAll(filepath.Join(b.root, "notlar", "selam")) // deleted on the other device, by hand
	b.sync(DirRemote{Root: remoteDir})
	if res := a.sync(DirRemote{Root: remoteDir}); res.Removed != 2 {
		t.Fatalf("removed %d", res.Removed)
	}
	if a.read("notlar/selam/page.json") != "<yok>" || a.read("notlar/kalan/page.json") == "<yok>" {
		t.Fatal("the wrong files went")
	}
	if _, err := os.Stat(filepath.Join(a.root, "notlar", "selam")); !os.IsNotExist(err) {
		t.Fatalf("the empty folder is left: %v", err)
	}
	lib := &Library{Root: a.root, FS: OSFS{}}
	items := lib.Trash()
	if len(items) != 1 || items[0].Title != "Selam" || items[0].Kind != "page" || items[0].From != filepath.FromSlash("notlar/selam") {
		t.Fatalf("trash: %+v", items)
	}
	if b, err := os.ReadFile(filepath.Join(items[0].Dir, "resim.png")); err != nil || string(b) != "png" {
		t.Fatalf("the picture is not kept: %v", err)
	}
	if a.read(".cop/"+filepath.Base(items[0].Dir)+"/page.json") == "<yok>" {
		t.Fatal("the page is not in the trash")
	}
}

// A page thrown away on the other device comes down into the trash as it was moved: what is taken away from
// its old place is the same bytes, and is not put in the trash a second time.
func TestSyncRemovalNotKeptTwice(t *testing.T) {
	remoteDir := t.TempDir()
	a, b := newDevice(t), newDevice(t)
	a.write("notlar/selam/page.json", `{"meta":{"title":"Selam"},"body":[]}`)
	a.sync(DirRemote{Root: remoteDir})
	b.sync(DirRemote{Root: remoteDir})
	lib := &Library{Root: b.root, FS: OSFS{}}
	if err := os.MkdirAll(filepath.Join(b.root, ".cop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.MoveToTrash(filepath.Join(b.root, "notlar", "selam"), TrashItem{Kind: "page", Title: "Selam"}); err != nil {
		t.Fatal(err)
	}
	b.sync(DirRemote{Root: remoteDir})
	a.sync(DirRemote{Root: remoteDir})
	if a.read("notlar/selam/page.json") != "<yok>" {
		t.Fatal("it was not taken away")
	}
	if n := len((&Library{Root: a.root, FS: OSFS{}}).Trash()); n != 1 {
		t.Fatalf("the trash has %d items, want the one that came down", n)
	}
}

// A remote that names a file out of the library (../) gets nothing written or deleted there: the name is
// left out and told.
func TestSyncUnsafeNameFromRemote(t *testing.T) {
	remoteDir := t.TempDir()
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "disari-"+filepath.Base(root))
	os.WriteFile(outside, []byte("dokunma"), 0o644)
	defer os.Remove(outside)
	a := &device{t: t, root: root}
	a.write("notlar/a/page.json", "a")
	rem := &fakeNames{Remote: DirRemote{Root: remoteDir}, extra: map[string]RemoteEntry{"../disari-" + filepath.Base(root): {Hash: "x"}}}
	res, err := SyncOnce(OSFS{}, root, rem, &a.st, map[string]bool{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(outside); string(b) != "dokunma" {
		t.Fatalf("the file outside was written: %q", b)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "güvenli olmayan") {
		t.Fatalf("errors: %v", res.Errors)
	}
	// and it was never asked for
	if rem.got != 0 {
		t.Fatalf("Get called %d times for it", rem.got)
	}
}

// fakeNames is a remote that lists a name more than it has.
type fakeNames struct {
	Remote
	extra map[string]RemoteEntry
	got   int
}

func (f *fakeNames) List() (map[string]RemoteEntry, error) {
	l, err := f.Remote.List()
	for n, e := range f.extra {
		l[n] = e
	}
	return l, err
}

func (f *fakeNames) Get(name string) ([]byte, error) {
	if _, ok := f.extra[name]; ok {
		f.got++
	}
	return f.Remote.Get(name)
}

// countFS counts the files read.
type countFS struct {
	FS
	reads int
}

func (c *countFS) ReadFile(n string) ([]byte, error) { c.reads++; return c.FS.ReadFile(n) }

// A file seen as it was at the last sync is not read and hashed again: a pass over a library that did not change
// reads nothing; one that changed, only what changed, even when it is the same size.
func TestSyncDoesNotReadWhatDidNotChange(t *testing.T) {
	remoteDir := t.TempDir()
	a := newDevice(t)
	for i := 0; i < 20; i++ {
		a.write(fmt.Sprintf("notlar/s%02d/page.json", i), "aaa")
	}
	a.sync(DirRemote{Root: remoteDir})
	cf := &countFS{FS: OSFS{}}
	res, err := SyncOnce(cf, a.root, DirRemote{Root: remoteDir}, &a.st, map[string]bool{}, time.Now())
	if err != nil || len(res.Errors) != 0 {
		t.Fatal(err, res.Errors)
	}
	if cf.reads != 0 {
		t.Fatalf("a pass with nothing changed read %d files", cf.reads)
	}
	// one is written again, the same size, and its time is not the same
	a.write("notlar/s07/page.json", "bbb")
	later := time.Now().Add(3 * time.Second)
	os.Chtimes(filepath.Join(a.root, "notlar", "s07", "page.json"), later, later)
	cf.reads = 0
	res, err = SyncOnce(cf, a.root, DirRemote{Root: remoteDir}, &a.st, map[string]bool{}, time.Now())
	if err != nil || len(res.Errors) != 0 || res.Pushed != 1 {
		t.Fatalf("%v %+v", err, res)
	}
	if cf.reads > 2 { // the one that changed: read to be hashed, and to be sent
		t.Fatalf("read %d files for one that changed", cf.reads)
	}
	b := newDevice(t)
	if b.sync(DirRemote{Root: remoteDir}); b.read("notlar/s07/page.json") != "bbb" {
		t.Fatal("the change did not arrive")
	}
}

// onGet runs a function when the remote is asked for a file: the window going on while a pass runs.
type onGet struct {
	Remote
	fn func(name string)
}

func (g *onGet) Get(name string) ([]byte, error) {
	if g.fn != nil {
		g.fn(name)
	}
	return g.Remote.Get(name)
}

// A page saved while a pass is bringing the remote's version down is not written over: the next pass finds
// both changed, and keeps both.
func TestSyncDoesNotOverwriteWhatIsSavedMidPass(t *testing.T) {
	remoteDir := t.TempDir()
	a, b := newDevice(t), newDevice(t)
	a.write("notlar/s/page.json", "ilk")
	a.sync(DirRemote{Root: remoteDir})
	b.sync(DirRemote{Root: remoteDir})
	b.write("notlar/s/page.json", "b'nin")
	b.sync(DirRemote{Root: remoteDir})
	hook := &onGet{Remote: DirRemote{Root: remoteDir}}
	hook.fn = func(string) { a.write("notlar/s/page.json", "yazılırken") } // the user saves as it comes down
	res, err := SyncOnce(OSFS{}, a.root, hook, &a.st, map[string]bool{}, time.Now())
	if err != nil || len(res.Errors) != 0 {
		t.Fatal(err, res.Errors)
	}
	if a.read("notlar/s/page.json") != "yazılırken" || res.Pulled != 0 {
		t.Fatalf("written over: %q %+v", a.read("notlar/s/page.json"), res)
	}
	hook.fn = nil
	res = a.sync(DirRemote{Root: remoteDir})
	if len(res.Conflicts) != 1 {
		t.Fatalf("the next pass: %+v", res)
	}
	if a.read("notlar/s/page.json") != "b'nin" {
		t.Fatalf("the remote's did not stay: %q", a.read("notlar/s/page.json"))
	}
	found := false
	filepath.WalkDir(a.root, func(p string, d os.DirEntry, err error) error {
		if b, _ := os.ReadFile(p); err == nil && !d.IsDir() && string(b) == "yazılırken" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("what the user wrote is lost")
	}
}

// A new device: its library made its own first page and collection, untouched, and the account has the same
// files from the first device. The account's copies come down with no conflict, and what the first device wrote
// besides comes down too; a seeded file the user did change is still a conflict.
func TestSyncNewDeviceTakesTheAccountsSeed(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("notes/collection.json", `{"name":"Notes","id":"a"}`)
	a.write("notes/welcome/page.json", "welcome, from a")
	a.write("notes/meeting/page.json", "the meeting")
	a.write("journal/collection.json", `{"name":"Journal","id":"a"}`)
	a.sync(remote)

	b.write("notes/collection.json", `{"name":"Notes","id":"b"}`)
	b.write("notes/welcome/page.json", "welcome, from b")
	b.write("journal/collection.json", `{"name":"Journal","id":"b","edited":true}`)
	b.st.Pristine = map[string]string{
		"notes/collection.json":   HashOf([]byte(`{"name":"Notes","id":"b"}`)),
		"notes/welcome/page.json": HashOf([]byte("welcome, from b")),
		"journal/collection.json": HashOf([]byte(`{"name":"Journal","id":"b"}`)), // changed since it was made
	}
	r := b.sync(remote)
	if len(r.Conflicts) != 1 || r.Conflicts[0] != "journal/collection.json" {
		t.Fatalf("conflicts: %v", r.Conflicts)
	}
	for name, want := range map[string]string{
		"notes/collection.json":   `{"name":"Notes","id":"a"}`,
		"notes/welcome/page.json": "welcome, from a",
		"notes/meeting/page.json": "the meeting",
	} {
		if got := b.read(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
	if len(b.st.Pristine) != 0 {
		t.Fatalf("still pristine after the pass: %v", b.st.Pristine)
	}
}

// countingRemote counts the listings of the remote under it.
type countingRemote struct {
	Remote
	lists int
}

func (c *countingRemote) List() (map[string]RemoteEntry, error) {
	c.lists++
	return c.Remote.List()
}

// A pass that sends an edit lists the remote once: the hash of what was sent is known from the sealed bytes, which
// is the remote's own hash of them, and the next pass, finding the same, does nothing.
func TestSyncPushListsOnce(t *testing.T) {
	inner := &countingRemote{Remote: DirRemote{Root: t.TempDir()}}
	r, err := NewEncRemote(inner, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := newDevice(t)
	d.write("Not/Bir/page.json", "bir")
	d.sync(r)
	d.write("Not/Bir/page.json", "bir, düzeltildi")
	inner.lists = 0
	if res := d.sync(r); res.Pushed != 1 {
		t.Fatalf("%+v", res)
	}
	if inner.lists != 1 {
		t.Fatalf("an edit's pass listed the remote %d times", inner.lists)
	}
	if res := d.sync(r); res.Pushed+res.Pulled+len(res.Conflicts) != 0 {
		t.Fatalf("the next pass did something: %+v", res)
	}
}
