package library

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sealKey is the key of the tests of #148: a remote made with it reads what another made with it put.
var sealKey = bytes.Repeat([]byte{9}, 32)

// sealRemote is this version's remote over inner, with labels on or off.
func sealRemote(t *testing.T, inner Remote, labels bool) *EncRemote {
	t.Helper()
	e, err := NewEncRemote(inner, sealKey)
	if err != nil {
		t.Fatal(err)
	}
	e.Labels = labels
	return e
}

// legacySeal seals b as the versions before #148 did: with no label.
func legacySeal(e *EncRemote, b []byte) []byte {
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, nil)
}

// passOf makes one pass of d with this version, and gives back what it did and its error, errors and all.
func passOf(d *device, r Remote) (SyncResult, error) { return passAt(d, r, sealT0) }

// passAt is a pass at the time given: what is waited for is counted in it.
func passAt(d *device, r Remote, at time.Time) (SyncResult, error) {
	return SyncOnce(OSFS{}, d.root, r, &d.st, nil, at)
}

var sealT0 = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

// reload keeps d's state and reads it back, as the app does between passes.
func reload(d *device) {
	b, _ := json.Marshal(d.st)
	d.st = SyncState{}
	json.Unmarshal(b, &d.st)
}

func mustGet(t *testing.T, r Remote, name string) []byte {
	t.Helper()
	b, err := r.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// opened opens the remote's object as a map of strings, under the label given (nil for none): the index as a
// version before #148 reads it, or the hashes.
func opened(t *testing.T, inner Remote, name string, aad []byte) map[string]string {
	t.Helper()
	e := sealRemote(t, inner, false)
	raw := mustGet(t, inner, name)
	plain, err := e.gcm.Open(nil, raw[:12], raw[12:], aad)
	if err != nil {
		t.Fatalf("%s does not open with the label %q: %v", name, aad, err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(plain, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// opensWithNoLabel says whether the remote's object opens as a version before #148 opens it.
func opensWithNoLabel(t *testing.T, inner Remote, name string) bool {
	t.Helper()
	raw := mustGet(t, inner, name)
	_, err := sealRemote(t, inner, false).gcm.Open(nil, raw[:12], raw[12:], nil)
	return err == nil
}

// object is the file of a page's sealed object in a folder remote.
func object(t *testing.T, inner DirRemote, page string) string {
	return inner.file(sealRemote(t, inner, false).opaque(page))
}

// An oldBuild is a released version from before #148, syncing as it did.
type oldBuild struct {
	name string
	pass func(d *device, inner Remote) (SyncResult, error)
}

var oldBuilds = []oldBuild{
	{"v0.1.0", func(d *device, inner Remote) (SyncResult, error) {
		return v010Pass(d, newV010EncRemote(inner, sealKey))
	}},
	{"cd21b2a", func(d *device, inner Remote) (SyncResult, error) { return passOf(d, newOldEncRemote(inner, sealKey)) }},
}

// clean fails the test when a pass erred or refused something.
func clean(t *testing.T, what string, res SyncResult, err error) {
	t.Helper()
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("%s: %v %v", what, err, res.Errors)
	}
}

// verRemote is a folder that keeps each earlier state of a file, as the Worker does (cloudflare/sync/worker.js).
type verRemote struct {
	DirRemote
	old map[string][][]byte
}

func (v *verRemote) Put(name string, data []byte, prev string) error {
	was, _ := v.DirRemote.Get(name)
	if err := v.DirRemote.Put(name, data, prev); err != nil {
		return err
	}
	if was != nil && !strings.HasPrefix(name, "_") {
		v.old[name] = append(v.old[name], was)
	}
	return nil
}

func (v *verRemote) Versions(name string) ([]RemoteVersion, error) {
	var out []RemoteVersion
	for i, b := range v.old[name] {
		out = append(out, RemoteVersion{At: int64(i), Size: len(b)})
	}
	return out, nil
}

func (v *verRemote) VersionData(name string, at int64) ([]byte, error) { return v.old[name][at], nil }

// A library sealed before #148, with no labels, no generation and no hashes, is read: files, index and versions of
// the past. Its index gets a generation at the first pass, and still opens as before, and the hashes are put beside
// it; with labels on, what is written is sealed with them, and the versions of the past open either way.
func TestSyncSealLegacyObjectsOpen(t *testing.T) {
	inner := &verRemote{DirRemote: DirRemote{Root: t.TempDir()}, old: map[string][][]byte{}}
	old := sealRemote(t, inner, false)
	const page = "Not/Eski/page.json"
	idx, _ := json.Marshal(map[string]string{old.opaque(page): page})
	inner.Put(IndexFile, legacySeal(old, idx), "")
	inner.Put(old.opaque(page), legacySeal(old, []byte("eski bir")), "")
	inner.Put(old.opaque(page), legacySeal(old, []byte("eski iki")), HashOf(mustGet(t, inner, old.opaque(page))))

	d := newDevice(t)
	if d.sync(sealRemote(t, inner, false)); d.read(page) != "eski iki" {
		t.Fatalf("a legacy library was not read: %q", d.read(page))
	}
	if m := opened(t, inner, IndexFile, nil); m[genKey] == "" || m[old.opaque(page)] != page {
		t.Fatalf("the index was not numbered, or does not open as before: %v", m)
	}
	if h := opened(t, inner, HashesFile, nil)[old.hashKey(page)]; !strings.HasPrefix(h, short(HashOf([]byte("eski iki")))+":") {
		t.Fatalf("the hashes have %q for the page", h)
	}
	d.write(page, "yeni")
	if d.sync(sealRemote(t, inner, false)); !opensWithNoLabel(t, inner, old.opaque(page)) {
		t.Fatal("a file written with labels off does not open as before")
	}
	d.write(page, "yeni, etiketli")
	if d.sync(sealRemote(t, inner, true)); opensWithNoLabel(t, inner, old.opaque(page)) || opensWithNoLabel(t, inner, HashesFile) {
		t.Fatal("with labels on, what was written opens with no label")
	}
	d.write(page, "son")
	d.sync(sealRemote(t, inner, true))
	for _, labels := range []bool{false, true} {
		r := sealRemote(t, inner, labels)
		vs, _ := r.Versions(page)
		var got []string
		for _, v := range vs {
			b, err := r.VersionData(page, v.At)
			if err != nil {
				t.Fatalf("version %d: %v", v.At, err)
			}
			got = append(got, string(b))
		}
		if strings.Join(got, "|") != "eski bir|eski iki|yeni|yeni, etiketli" {
			t.Fatalf("labels %v: the versions read %q", labels, got)
		}
	}
	fresh := newDevice(t)
	if fresh.sync(sealRemote(t, inner, false)); fresh.read(page) != "son" {
		t.Fatalf("with labels off, what labels on wrote was not read: %q", fresh.read(page))
	}
}

// The sealed bytes of two pages exchanged on the remote are refused, by the device that put them and by a new one,
// with labels off too: each is the other's, by what the device held and by the hashes (with labels on neither even
// opens under the other's label). No page here is written over (#148).
func TestSyncSealSwapRefused(t *testing.T) {
	for _, labels := range []bool{false, true} {
		inner := DirRemote{Root: t.TempDir()}
		a := newDevice(t)
		a.write("A/Bir/page.json", "bir")
		a.write("A/İki/page.json", "iki")
		a.sync(sealRemote(t, inner, labels))
		one, two := object(t, inner, "A/Bir/page.json"), object(t, inner, "A/İki/page.json")
		b1, _ := os.ReadFile(one)
		b2, _ := os.ReadFile(two)
		os.WriteFile(one, b2, 0o644)
		os.WriteFile(two, b1, 0o644)
		for i, d := range []*device{a, newDevice(t)} {
			res, err := passOf(d, sealRemote(t, inner, labels))
			if err != nil || len(res.Errors) != 2 || len(res.Refused) != 2 || res.Pulled != 0 { // a labelled swap, too: sealed for another name
				t.Fatalf("labels %v, device %d took the swap: %v %+v", labels, i, err, res)
			}
		}
		if a.read("A/Bir/page.json") != "bir" || a.read("A/İki/page.json") != "iki" {
			t.Fatalf("labels %v: a page here was written over by the swap", labels)
		}
	}
}

// A page moved away and back by a version before #148 comes back to this one, and to a new device, as it is: the
// hashes that version left as they were are of objects deleted since, and refuse nothing.
func TestSyncSealOldBuildMovesBack(t *testing.T) {
	for _, ob := range oldBuilds {
		t.Run(ob.name, func(t *testing.T) {
			inner := DirRemote{Root: t.TempDir()}
			n, o := newDevice(t), newDevice(t)
			const a, b = "Not/A/page.json", "Not/B/page.json"
			n.write(a, "metin")
			n.sync(sealRemote(t, inner, false))
			res, err := ob.pass(o, inner)
			clean(t, "the old version's first pass", res, err)
			os.RemoveAll(filepath.Join(o.root, "Not/A"))
			o.write(b, "metin")
			res, err = ob.pass(o, inner)
			clean(t, "the old version's move", res, err)
			res, err = passOf(n, sealRemote(t, inner, false))
			clean(t, "the move away", res, err)
			if n.read(b) != "metin" || n.read(a) != "<yok>" {
				t.Fatalf("the move away: %q %q", n.read(a), n.read(b))
			}
			os.RemoveAll(filepath.Join(o.root, "Not/B"))
			o.write(a, "metin")
			res, err = ob.pass(o, inner)
			clean(t, "the old version's move back", res, err)
			for i, d := range []*device{n, newDevice(t)} {
				res, err := passOf(d, sealRemote(t, inner, false))
				clean(t, fmt.Sprintf("device %d after the move back", i), res, err)
				if d.read(a) != "metin" || d.read(b) != "<yok>" {
					t.Fatalf("device %d: the page is at A %q, at B %q", i, d.read(a), d.read(b))
				}
			}
		})
	}
}

// A folder copied between computers by another program keeps one of two indexes put at once: the other device's
// index, or a version before #148's, is taken as one that did not see this one's, not as an older one, and the
// names lost are put again. Two hashes put at once are neither older than the other either.
func TestSyncSealIndexFork(t *testing.T) {
	type writer struct {
		name string
		pass func(d *device, inner Remote) (SyncResult, error)
	}
	writers := []writer{{"this version", func(d *device, inner Remote) (SyncResult, error) { return passOf(d, sealRemote(t, inner, false)) }}}
	for _, ob := range oldBuilds {
		writers = append(writers, writer(ob))
	}
	for _, w := range writers {
		t.Run(w.name, func(t *testing.T) {
			inner := DirRemote{Root: t.TempDir()}
			a, b := newDevice(t), newDevice(t)
			a.write("Not/Bir/page.json", "bir")
			a.sync(sealRemote(t, inner, false))
			res, err := w.pass(b, inner)
			clean(t, "b's first pass", res, err)
			index, hashes := mustGet(t, inner, IndexFile), mustGet(t, inner, HashesFile)
			a.write("Not/A/page.json", "a'nın")
			a.write("Not/Bir/page.json", "bir, a")
			a.sync(sealRemote(t, inner, false))
			os.WriteFile(inner.file(IndexFile), index, 0o644) // b put its own, from the copy before a's
			if w.name == "this version" {                     // and its hashes: an older version puts none
				os.WriteFile(inner.file(HashesFile), hashes, 0o644)
			}
			b.write("Not/B/page.json", "b'nin")
			res, err = w.pass(b, inner)
			clean(t, "b's pass", res, err)
			for pass := 0; pass < 2; pass++ {
				res, err = passOf(a, sealRemote(t, inner, false))
				clean(t, "a's pass after the fork", res, err)
			}
			if a.read("Not/B/page.json") != "b'nin" || a.read("Not/A/page.json") != "a'nın" {
				t.Fatal("a lost a page in the fork")
			}
			res, err = w.pass(b, inner)
			clean(t, "b's pass after the fork", res, err)
			if b.read("Not/A/page.json") != "a'nın" || b.read("Not/Bir/page.json") != "bir, a" {
				t.Fatalf("b did not get a's pages: %q %q", b.read("Not/A/page.json"), b.read("Not/Bir/page.json"))
			}
		})
	}
}

// An index withheld from a device that has seen one is waited for a few minutes, then refused: the remote holds
// files, and with no index each would read as gone. A remote emptied, with nothing in it, is the one honest way an index goes: it starts again.
func TestSyncSealIndexWithheld(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a := newDevice(t)
	a.write("A/Bir/page.json", "bir")
	a.write("A/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	index := mustGet(t, inner, IndexFile)
	one, two := object(t, inner, "A/Bir/page.json"), object(t, inner, "A/İki/page.json")
	b1, _ := os.ReadFile(one)
	b2, _ := os.ReadFile(two)
	os.WriteFile(one, b2, 0o644) // and the pages exchanged
	os.WriteFile(two, b1, 0o644)
	os.Remove(inner.file(IndexFile))
	// waited for a while, as a folder may bring it late, then refused
	for _, after := range []time.Duration{0, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 7 * time.Minute} {
		reload(a)
		_, err := passAt(a, sealRemote(t, inner, false), sealT0.Add(time.Hour+after))
		if waits := after < missingWait; waits != errors.Is(err, ErrRemoteNotYet) || !waits != errors.Is(err, ErrIndexRollback) {
			t.Fatalf("%v after it went: %v", after, err)
		}
		if a.read("A/Bir/page.json") != "bir" || a.read("A/İki/page.json") != "iki" {
			t.Fatal("a page was written over or deleted")
		}
	}
	os.WriteFile(inner.file(IndexFile), index, 0o644)
	os.WriteFile(one, b1, 0o644)
	os.WriteFile(two, b2, 0o644)
	// emptied: everything goes up again
	ents, _ := os.ReadDir(inner.Root)
	for _, e := range ents {
		if e.Name() != SaltFile {
			os.RemoveAll(inner.file(e.Name()))
		}
	}
	res, err := passOf(a, sealRemote(t, inner, false))
	if clean(t, "the pass on an emptied remote", res, err); res.Pushed != 2 {
		t.Fatalf("an emptied remote: %+v", res)
	}
	fresh := newDevice(t)
	if fresh.sync(sealRemote(t, inner, false)); fresh.read("A/İki/page.json") != "iki" {
		t.Fatal("the library sent again did not come")
	}
}

// A page's older sealed bytes served again under its name are refused by a device that held them, across passes
// with the state kept, and the page here stays; a device that never held them, with labels off, takes them and puts
// their hash in the hashes, and the devices that held them refuse them still.
func TestSyncSealOldBlobRefused(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "birinci")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	first, _ := os.ReadFile(object(t, inner, page))
	a.write(page, "ikinci")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	os.WriteFile(object(t, inner, page), first, 0o644)

	refused := func(when string) {
		t.Helper()
		for i, d := range []*device{a, b} {
			reload(d)
			res, err := passOf(d, sealRemote(t, inner, false))
			if err != nil || len(res.Refused) != 1 || d.read(page) != "ikinci" {
				t.Fatalf("%s, device %d took the old copy: %v %+v", when, i, err, res)
			}
		}
	}
	refused("first")
	refused("again")
	fresh := newDevice(t)
	if fresh.sync(sealRemote(t, inner, false)); fresh.read(page) != "birinci" {
		t.Fatalf("with labels off, a new device read %q", fresh.read(page))
	}
	refused("after a new device took it")
}

// listLies is a folder whose listing gives each object a hash of its own making: what a device keeps of the copies
// it held is what it hashed, not the remote's word.
type listLies struct{ DirRemote }

func (l listLies) List() (map[string]RemoteEntry, error) {
	out, err := l.DirRemote.List()
	for n, e := range out {
		if !e.Deleted && !strings.HasPrefix(n, "_") {
			e.Hash = HashOf([]byte("lie " + e.Hash))
			out[n] = e
		}
	}
	return out, err
}

// The copies a device keeps of a file are hashed from what it fetched, so a remote whose listing lies does not
// make an older copy pass; and they go back far: an older copy fifty edits back is refused.
func TestSyncSealOldBlobKeptLongAndHashedHere(t *testing.T) {
	dir := DirRemote{Root: t.TempDir()}
	inner := listLies{dir}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "0")
	a.sync(sealRemote(t, dir, false))
	b.sync(sealRemote(t, inner, false))
	first, _ := os.ReadFile(object(t, dir, page))
	for i := 1; i <= 50; i++ {
		a.write(page, fmt.Sprint(i))
		a.sync(sealRemote(t, dir, false))
		b.sync(sealRemote(t, inner, false))
	}
	os.WriteFile(object(t, dir, page), first, 0o644)
	for i, d := range []*device{a, b} {
		reload(d)
		var r Remote = sealRemote(t, dir, false)
		if d == b {
			r = sealRemote(t, inner, false)
		}
		if res, err := passOf(d, r); err != nil || len(res.Refused) != 1 || d.read(page) != "50" {
			t.Fatalf("device %d took the copy fifty edits back: %v %+v", i, err, res)
		}
	}
}

// A page deleted is not brought back by its last object served again, its tombstone gone: the device that deleted
// it, another that saw it go, and a new one refuse it.
func TestSyncSealDeletedNotBack(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "bir")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	last, _ := os.ReadFile(object(t, inner, page))
	os.RemoveAll(filepath.Join(a.root, "Not/Bir"))
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	if b.read(page) != "<yok>" {
		t.Fatal("the deletion did not come")
	}
	os.Remove(object(t, inner, page) + tombSuffix)
	os.WriteFile(object(t, inner, page), last, 0o644)
	for i, d := range []*device{a, b, newDevice(t)} {
		reload(d)
		if res, err := passOf(d, sealRemote(t, inner, false)); err != nil || len(res.Refused) != 1 || d.read(page) != "<yok>" {
			t.Fatalf("device %d brought the page back: %v %+v", i, err, res)
		}
	}
}

// An older index served again is refused across passes, by the generation the state keeps, and it is not put back
// merged; a remote whose index was read before the pass (OpenEnc reads it) reads it again under the state.
func TestSyncSealIndexRollbackRefused(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Bir/page.json", "bir")
	a.sync(sealRemote(t, inner, false))
	before := mustGet(t, inner, IndexFile)
	a.write("Not/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	os.WriteFile(inner.file(IndexFile), before, 0o644)
	for pass := 0; pass < 2; pass++ {
		for i, d := range []*device{a, b} {
			reload(d)
			if _, err := passOf(d, sealRemote(t, inner, false)); !errors.Is(err, ErrIndexRollback) {
				t.Fatalf("pass %d, device %d took the old index: %v", pass, i, err)
			}
			if d.read("Not/İki/page.json") != "iki" {
				t.Fatalf("pass %d, device %d: a page went", pass, i)
			}
		}
	}
	if !bytes.Equal(mustGet(t, inner, IndexFile), before) {
		t.Fatal("the old index was merged and put again")
	}
	pre := sealRemote(t, inner, false)
	if err := pre.loadIndex(); err != nil {
		t.Fatal(err)
	}
	if _, err := passOf(a, pre); !errors.Is(err, ErrIndexRollback) {
		t.Fatalf("an index read before the pass was taken: %v", err)
	}
}

// failIndex is a folder that refuses to have its index put.
type failIndex struct{ DirRemote }

func (f failIndex) Put(name string, data []byte, prev string) error {
	if name == IndexFile {
		return errors.New("no room")
	}
	return f.DirRemote.Put(name, data, prev)
}

// The user trusts the remote again: an index trusted is taken and put above what was seen, and the passes after
// it, here and on another device, are as before; a trust that could not put the index stays for the next pass.
// Trusting the files takes a file refused, and not an older index.
func TestSyncSealTrustAgain(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "birinci")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	before := mustGet(t, inner, IndexFile)
	first, _ := os.ReadFile(object(t, inner, page))
	a.write(page, "ikinci")
	a.write("Not/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	os.WriteFile(inner.file(IndexFile), before, 0o644) // the remote restored from a backup
	os.WriteFile(object(t, inner, page), first, 0o644)

	a.st.TrustFiles = true
	if _, err := passOf(a, sealRemote(t, inner, false)); !errors.Is(err, ErrIndexRollback) {
		t.Fatalf("trusting the files took the older index: %v", err)
	}
	a.st.TrustFiles = false
	a.st.TrustIndex = true
	if _, err := passOf(a, sealRemote(t, failIndex{inner}, false)); err == nil || !a.st.TrustIndex {
		t.Fatalf("a trust whose index could not be put: %v, used up %v", err, !a.st.TrustIndex)
	}
	if a.read("Not/İki/page.json") != "iki" {
		t.Fatal("the page the older index leaves out went, its name not put back")
	}
	res, err := passOf(a, sealRemote(t, inner, false))
	if err != nil || len(res.Refused) != 1 || a.st.TrustIndex {
		t.Fatalf("trusting the index: %v %+v, trust left %v", err, res, a.st.TrustIndex)
	}
	a.st.TrustFiles = true
	res, err = passOf(a, sealRemote(t, inner, false))
	if clean(t, "trusting the files", res, err); a.read(page) != "birinci" || a.st.TrustFiles {
		t.Fatalf("trusting the files: %q", a.read(page))
	}
	reload(a)
	res, err = passOf(a, sealRemote(t, inner, false))
	clean(t, "the pass after", res, err)
	if a.read("Not/İki/page.json") != "iki" {
		t.Fatal("the page the older index left out went")
	}
	// the other device held the restored copy before, and a newer one since: it refuses it until it trusts too
	if res, _ := passOf(b, sealRemote(t, inner, false)); len(res.Refused) != 1 || b.read(page) != "ikinci" {
		t.Fatalf("the other device took the restored copy: %+v", res)
	}
	b.st.TrustFiles = true
	res, err = passOf(b, sealRemote(t, inner, false))
	if clean(t, "the other device, trusting", res, err); b.read(page) != "birinci" {
		t.Fatalf("the other device, trusting: %q", b.read(page))
	}
}

// A version before #148 and this one sync the same remote, both ways: the older one reads the index and the files
// this one writes, its own entries leave no file behind there, and a union it puts back carries them; this one takes
// the older one's new page, its edit of a page the hashes have, and the same bytes written again.
func TestSyncSealOldClientBothWays(t *testing.T) {
	for _, ob := range oldBuilds {
		t.Run(ob.name, func(t *testing.T) {
			inner := DirRemote{Root: t.TempDir()}
			n, o := newDevice(t), newDevice(t)
			n.write("Not/Bir/page.json", "bir")
			n.sync(sealRemote(t, inner, false))
			res, err := ob.pass(o, inner)
			clean(t, "the older version's pass", res, err)
			if o.read("Not/Bir/page.json") != "bir" {
				t.Fatal("the older version did not read what this one wrote")
			}
			if ents, _ := os.ReadDir(o.root); len(ents) != 1 {
				t.Fatalf("the older version made %d entries", len(ents))
			}
			o.write("Not/Bir/page.json", "bir, eskiden düzeltildi")
			o.write("Not/Eski/page.json", "eski cihazdan")
			res, err = ob.pass(o, inner)
			clean(t, "the older version's writes", res, err)
			if opened(t, inner, IndexFile, nil)[genKey] == "" {
				t.Fatal("the older version's union lost the generation")
			}
			res, err = passOf(n, sealRemote(t, inner, false))
			if clean(t, "this version after the older one", res, err); res.Pulled != 2 {
				t.Fatalf("this version took %d files", res.Pulled)
			}
			if n.read("Not/Bir/page.json") != "bir, eskiden düzeltildi" || n.read("Not/Eski/page.json") != "eski cihazdan" {
				t.Fatal("the older version's writes did not come")
			}
			n.write("Not/Eski/page.json", "yeni cihazdan")
			n.sync(sealRemote(t, inner, false))
			res, err = ob.pass(o, inner)
			if clean(t, "the older version again", res, err); o.read("Not/Eski/page.json") != "yeni cihazdan" {
				t.Fatal("the older version did not read the edit back")
			}
			o.write("Not/Bir/page.json", "bir") // as it was: sealed again, it is no older copy
			ob.pass(o, inner)
			res, err = passOf(n, sealRemote(t, inner, false))
			if clean(t, "the page put back", res, err); n.read("Not/Bir/page.json") != "bir" {
				t.Fatal("the older version's page put back was not taken")
			}
		})
	}
}

// With labels on, a new device refuses a labelled file the hashes do not vouch for: an older copy, or one they have
// nothing of. It waits for them first, missingWait, taking nothing: another device puts a file before the hashes
// that vouch for it (#148, phase 2). An unlabelled one, as a version before #148 puts it, is taken.
func TestSyncSealStrict(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a := newDevice(t)
	a.write(page, "birinci")
	a.sync(sealRemote(t, inner, true))
	first, _ := os.ReadFile(object(t, inner, page))
	a.write(page, "ikinci")
	a.sync(sealRemote(t, inner, true))
	second, _ := os.ReadFile(object(t, inner, page))
	os.WriteFile(object(t, inner, page), first, 0o644)
	b := newDevice(t)
	if res, _ := passOf(b, sealRemote(t, inner, true)); !res.Waiting || res.Pulled != 0 || len(res.Refused) != 0 {
		t.Fatalf("an older labelled copy, before the wait: %+v", res)
	}
	if res, _ := passAt(b, sealRemote(t, inner, true), sealT0.Add(missingWait+time.Minute)); len(res.Refused) != 1 || b.read(page) != "<yok>" {
		t.Fatalf("an older labelled copy was taken: %+v", res)
	}
	os.WriteFile(object(t, inner, page), second, 0o644)
	a.write("Not/Eski/page.json", "eski")
	a.sync(sealRemote(t, inner, false)) // unlabelled
	os.Remove(inner.file(HashesFile))
	fresh := newDevice(t)
	passOf(fresh, sealRemote(t, inner, true)) // the hashes waited for
	res, _ := passAt(fresh, sealRemote(t, inner, true), sealT0.Add(missingWait+time.Minute))
	if fresh.read("Not/Eski/page.json") != "eski" || len(res.Refused) != 1 || res.Refused[0] != page {
		t.Fatalf("with labels on and no hashes: %+v", res)
	}
}

// What a pass learnt goes in the hashes only over what they had when it learnt it: a newer hash another device put
// meanwhile stays.
func TestSyncSealHashesCompareAndSet(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "bir")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	slow := sealRemote(t, inner, false)
	slow.beginPass(&SyncState{}, time.Now())
	if _, err := slow.List(); err != nil {
		t.Fatal(err)
	}
	if _, err := slow.Get(page); err != nil { // learnt, with the hashes as they are now
		t.Fatal(err)
	}
	slow.know(page, fileHashes{plain: "eski", sealed: "eski"})
	b.write(page, "iki")
	b.sync(sealRemote(t, inner, false))
	if err := slow.saveHashes(); err != nil {
		t.Fatal(err)
	}
	e := sealRemote(t, inner, false)
	if h := opened(t, inner, HashesFile, nil)[e.hashKey(page)]; !strings.HasPrefix(h, short(HashOf([]byte("iki")))) {
		t.Fatalf("the newer hash was written over: %q", h)
	}
}

// A library synced before #148 has the hashes of its files after one pass of this version, with nothing sent or
// brought down again and the index put once, for its generation.
func TestSyncSealHashesWithNothingSent(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	d := newDevice(t)
	d.write("Not/Bir/page.json", "bir")
	d.write("Not/İki/page.json", "iki")
	oldBuilds[0].pass(d, inner)
	if res := d.sync(sealRemote(t, inner, false)); res.Pushed+res.Pulled != 0 {
		t.Fatalf("the pass sent or brought: %+v", res)
	}
	h := opened(t, inner, HashesFile, nil)
	e := sealRemote(t, inner, false)
	if !strings.HasPrefix(h[e.hashKey("Not/Bir/page.json")], short(HashOf([]byte("bir")))) || len(h) != 3 || h[genKey] == "" {
		t.Fatalf("the hashes: %v", h)
	}
}

// A page changed here while the remote serves an older copy of it is not met as a conflict: the copy is refused
// before ours is kept beside, so no conflict file is left at each pass, and the page here stays as it was typed.
func TestSyncSealOldBlobNoConflictCopies(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a := newDevice(t)
	a.write(page, "birinci")
	a.sync(sealRemote(t, inner, false))
	first, _ := os.ReadFile(object(t, inner, page))
	a.write(page, "ikinci")
	a.sync(sealRemote(t, inner, false))
	os.WriteFile(object(t, inner, page), first, 0o644)
	a.write(page, "yerel")
	for pass := 0; pass < 2; pass++ {
		res, err := passOf(a, sealRemote(t, inner, false))
		if err != nil || len(res.Refused) != 1 || len(res.Conflicts) != 0 {
			t.Fatalf("pass %d: %v %+v", pass, err, res)
		}
	}
	if ents, _ := os.ReadDir(filepath.Join(a.root, "Not/Bir")); len(ents) != 1 || a.read(page) != "yerel" {
		t.Fatalf("the folder holds %d files, the page %q", len(ents), a.read(page))
	}
}

// A state that changes remote forgets what it saw of the other one, but not the device's name.
func TestSyncSealForRemoteForgets(t *testing.T) {
	st := SyncState{Remote: "first", Device: "d", IndexSeen: map[string]int64{"d": 7}, IndexNames: "x", Earlier: map[string][]string{"a": {"x"}}}
	if st.ForRemote("first"); len(st.IndexSeen) != 1 || len(st.Earlier) != 1 {
		t.Fatalf("the same remote forgot: %+v", st)
	}
	if !st.ForRemote("second") || st.IndexSeen != nil || st.IndexNames != "" || st.Earlier != nil || st.Device != "d" {
		t.Fatalf("another remote: %+v", st)
	}
}

// putBytes is a remote that counts the puts and the bytes put, by what they are.
type putBytes struct {
	Remote
	n, puts map[string]int
}

func (p *putBytes) Put(name string, data []byte, prev string) error {
	kind := "files"
	if strings.HasPrefix(name, "_") {
		kind = name
	}
	p.n[kind] += len(data)
	p.puts[kind]++
	return p.Remote.Put(name, data, prev)
}

// What ten edits put, each followed by a pass of three devices, on a library of 500 pages, by each version; and
// what the sync state keeps of 2000 pages synced (go test -v -run SyncSealCost). Only the puts of the index and the
// hashes are held to a bound.
func TestSyncSealCost(t *testing.T) {
	pages := 100 // the bound is held on a small library; the measure is taken on the full one with -v
	if testing.Verbose() {
		pages = 500
	}
	for _, v := range []string{"v0.1.0", "cd21b2a", "this"} {
		inner := &putBytes{Remote: DirRemote{Root: t.TempDir()}, n: map[string]int{}, puts: map[string]int{}}
		devs := []*device{newDevice(t), newDevice(t), newDevice(t)}
		for i := range pages {
			devs[0].write(fmt.Sprintf("Koleksiyon %d/Bir sayfa adı %d/page.json", i%10, i), fmt.Sprintf(`{"title":"sayfa %d","body":"..."}`, i))
		}
		pass := func(d *device) {
			switch v {
			case "v0.1.0":
				oldBuilds[0].pass(d, inner)
			case "cd21b2a":
				oldBuilds[1].pass(d, inner)
			default:
				passOf(d, sealRemote(t, inner, false))
			}
		}
		for _, d := range devs {
			pass(d)
		}
		inner.n, inner.puts = map[string]int{}, map[string]int{}
		for i := range 10 {
			devs[0].write("Koleksiyon 1/Bir sayfa adı 1/page.json", fmt.Sprintf(`{"title":"sayfa 1","body":"%d"}`, i))
			for _, d := range devs {
				pass(d)
			}
		}
		t.Logf("%-8s 10 edits at %d pages, 3 devices: files %d puts %d B, _index %d puts %d B, _hashes %d puts %d B", v, pages,
			inner.puts["files"], inner.n["files"], inner.puts[IndexFile], inner.n[IndexFile], inner.puts[HashesFile], inner.n[HashesFile])
		if inner.puts[IndexFile] > 0 || inner.puts[HashesFile] > 10 { // the editor's one a pass; who only took, none
			t.Errorf("%s: the index was put %d times, the hashes %d", v, inner.puts[IndexFile], inner.puts[HashesFile])
		}
	}
	if !testing.Verbose() {
		return
	}
	inner := DirRemote{Root: t.TempDir()}
	d := newDevice(t)
	for i := range 2000 {
		d.write(fmt.Sprintf("Koleksiyon %d/Bir sayfa adı %d/page.json", i%20, i), fmt.Sprint(i))
	}
	d.sync(sealRemote(t, inner, false))
	for range 5 {
		for i := range 2000 {
			d.write(fmt.Sprintf("Koleksiyon %d/Bir sayfa adı %d/page.json", i%20, i), fmt.Sprint(i, "+"))
		}
		d.sync(sealRemote(t, inner, false))
	}
	st, _ := json.Marshal(d.st)
	earlier, _ := json.Marshal(d.st.Earlier)
	hashes := mustGet(t, inner, HashesFile)
	t.Logf("2000 pages, each synced 6 times: state %d B (copies held %d B), _hashes %d B", len(st), len(earlier), len(hashes))
}

// The copies a device held refuse a swap, a page deleted brought back, by themselves: with the remote's hashes
// gone, a device that synced the files refuses them still.
func TestSyncSealHeldWithoutHashes(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a := newDevice(t)
	a.write("A/Bir/page.json", "bir")
	a.write("A/İki/page.json", "iki")
	a.write("A/Üç/page.json", "üç")
	a.sync(sealRemote(t, inner, false))
	last, _ := os.ReadFile(object(t, inner, "A/Üç/page.json"))
	os.RemoveAll(filepath.Join(a.root, "A/Üç"))
	a.sync(sealRemote(t, inner, false))
	one, two := object(t, inner, "A/Bir/page.json"), object(t, inner, "A/İki/page.json")
	b1, _ := os.ReadFile(one)
	b2, _ := os.ReadFile(two)
	os.WriteFile(one, b2, 0o644)
	os.WriteFile(two, b1, 0o644)
	os.Remove(object(t, inner, "A/Üç/page.json") + tombSuffix)
	os.WriteFile(object(t, inner, "A/Üç/page.json"), last, 0o644)
	os.Remove(inner.file(HashesFile))
	reload(a)
	if res, err := passOf(a, sealRemote(t, inner, false)); err != nil || len(res.Refused) != 3 || a.read("A/Üç/page.json") != "<yok>" {
		t.Fatalf("with no hashes, the device took them: %v %+v", err, res)
	}
}

// Two devices make the same page at once, in a folder copied by another program: each index names nothing the
// other does not know, and neither is below the other, so neither is taken for an older one.
func TestSyncSealIndexForkSameName(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Bir/page.json", "bir")
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	index := mustGet(t, inner, IndexFile)
	a.write("Not/X/page.json", "a'nın")
	a.sync(sealRemote(t, inner, false))
	os.WriteFile(inner.file(IndexFile), index, 0o644)
	b.write("Not/X/page.json", "a'nın")
	b.sync(sealRemote(t, inner, false))
	res, err := passOf(a, sealRemote(t, inner, false))
	clean(t, "a's pass after the fork", res, err)
}

// A page whose object goes missing for a pass, with no tombstone (a folder copied by another program has not brought
// it yet, or put it away), is taken for gone; when it comes back byte for byte, it is taken back, not refused as
// an older copy: only a deletion, by this device or with a tombstone, ends what a device held of a file.
func TestSyncSealVanishedComesBack(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a, b := newDevice(t), newDevice(t)
	for _, n := range []string{"Bir", "İki", "Üç"} {
		a.write("Not/"+n+"/page.json", n)
	}
	a.sync(sealRemote(t, inner, false))
	b.sync(sealRemote(t, inner, false))
	file := object(t, inner, "Not/Bir/page.json")
	raw, _ := os.ReadFile(file)
	os.Remove(file)
	res, err := passOf(b, sealRemote(t, inner, false))
	if clean(t, "the pass with it missing", res, err); b.read("Not/Bir/page.json") != "<yok>" {
		t.Fatal("the page missing on the remote stayed")
	}
	os.WriteFile(file, raw, 0o644)
	reload(b)
	res, err = passOf(b, sealRemote(t, inner, false))
	if clean(t, "the pass with it back", res, err); b.read("Not/Bir/page.json") != "Bir" {
		t.Fatalf("the page back was not taken: %q", b.read("Not/Bir/page.json"))
	}
}

// Files synced before #148, by v0.1.0 or the commit before this work, are guarded from the first pass of this
// version: their copies, as the remote lists them as this device last synced them, are held and go in the hashes, so
// two of them exchanged on the remote are refused here and on a new device.
func TestSyncSealUpgradeThenSwap(t *testing.T) {
	for _, ob := range oldBuilds {
		t.Run(ob.name, func(t *testing.T) {
			inner := DirRemote{Root: t.TempDir()}
			d := newDevice(t)
			d.write("A/Bir/page.json", "bir")
			d.write("A/İki/page.json", "iki")
			res, err := ob.pass(d, inner)
			clean(t, "the older version's pass", res, err)
			for range 2 {
				res, err = passOf(d, sealRemote(t, inner, false))
				clean(t, "this version's pass", res, err)
			}
			if len(d.st.Earlier["A/Bir/page.json"]) != 1 {
				t.Fatalf("the copy synced before is not held: %v", d.st.Earlier)
			}
			one, two := object(t, inner, "A/Bir/page.json"), object(t, inner, "A/İki/page.json")
			b1, _ := os.ReadFile(one)
			b2, _ := os.ReadFile(two)
			os.WriteFile(one, b2, 0o644)
			os.WriteFile(two, b1, 0o644)
			for i, x := range []*device{d, newDevice(t)} {
				reload(x)
				if res, err := passOf(x, sealRemote(t, inner, false)); err != nil || len(res.Refused) != 2 || res.Pulled != 0 {
					t.Fatalf("device %d took the swap: %v %+v", i, err, res)
				}
			}
			if d.read("A/Bir/page.json") != "bir" {
				t.Fatal("a page was written over")
			}
		})
	}
}

// A remote that claims for a pass to be empty, everything then sent up again, does not make a device forget the
// copies it held: the old objects served again after, exchanged and with the hashes gone, are refused.
func TestSyncSealClaimedEmptyKeepsHeld(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a := newDevice(t)
	a.write("A/Bir/page.json", "bir")
	a.write("A/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	one, two := object(t, inner, "A/Bir/page.json"), object(t, inner, "A/İki/page.json")
	b1, _ := os.ReadFile(one)
	b2, _ := os.ReadFile(two)
	aside := t.TempDir()
	ents, _ := os.ReadDir(inner.Root)
	for _, e := range ents {
		if e.Name() != SaltFile {
			os.Rename(inner.file(e.Name()), filepath.Join(aside, e.Name()))
		}
	}
	res, err := passOf(a, sealRemote(t, inner, false))
	if clean(t, "the pass on the remote claimed empty", res, err); res.Pushed != 2 {
		t.Fatalf("the library did not go up again: %+v", res)
	}
	os.WriteFile(one, b2, 0o644)
	os.WriteFile(two, b1, 0o644)
	os.Remove(inner.file(HashesFile))
	reload(a)
	if res, _ := passOf(a, sealRemote(t, inner, false)); res.Pulled != 0 || len(res.Refused) != 2 || a.read("A/Bir/page.json") != "bir" {
		t.Fatalf("the old objects, exchanged, were taken: %+v", res)
	}
}

// The hashes are numbered too, and vouch for files a device never held: withheld from a device that has seen them,
// they are waited for a few minutes, fetching nothing on no word, and then what a pass would fetch is refused,
// while what is typed here still goes up; older ones are refused at once. Trusting the files again takes the remote
// as it is, and puts the hashes anew, above what was seen, so that the passes after it are as before.
func TestSyncSealHashesWithheldOrOlder(t *testing.T) {
	for _, older := range []bool{false, true} {
		inner := DirRemote{Root: t.TempDir()}
		a, b := newDevice(t), newDevice(t)
		a.write("A/Bir/page.json", "bir")
		a.sync(sealRemote(t, inner, false))
		b.sync(sealRemote(t, inner, false))
		before := mustGet(t, inner, HashesFile)
		a.write("A/X/page.json", "x")
		a.write("A/Y/page.json", "y")
		a.write("B/Z/page.json", "z")
		a.sync(sealRemote(t, inner, false))
		b.sync(sealRemote(t, inner, false), "A") // it sees the newer hashes, and holds no copy of X or Y
		x, y := object(t, inner, "A/X/page.json"), object(t, inner, "A/Y/page.json")
		bx, _ := os.ReadFile(x)
		by, _ := os.ReadFile(y)
		os.WriteFile(x, by, 0o644)
		os.WriteFile(y, bx, 0o644)
		if older {
			os.WriteFile(inner.file(HashesFile), before, 0o644)
		} else {
			os.Remove(inner.file(HashesFile))
			for _, after := range []time.Duration{0, 4 * time.Minute} {
				res, _ := passAt(b, sealRemote(t, inner, false), sealT0.Add(time.Hour+after))
				if res.Pulled != 0 || len(res.Errors) != 0 || !res.Waiting {
					t.Fatalf("%v after the hashes were withheld: %+v", after, res)
				}
			}
		}
		b.write("A/Bir/page.json", "bir, b")
		res, err := passAt(b, sealRemote(t, inner, false), sealT0.Add(time.Hour+6*time.Minute))
		if err != nil || res.Pulled != 0 || len(res.Refused) != 2 || res.Pushed != 1 {
			t.Fatalf("older %v: the hashes' rollback: %v %+v", older, err, res)
		}
		b.st.TrustFiles = true
		res, err = passAt(b, sealRemote(t, inner, false), sealT0.Add(2*time.Hour))
		if clean(t, "trusting the files", res, err); b.read("A/X/page.json") != "y" {
			t.Fatalf("older %v: trusting the files did not take them", older)
		}
		if _, err := inner.Get(HashesFile); err != nil {
			t.Fatalf("older %v: trusting the files did not put the hashes again: %v", older, err)
		}
		for _, after := range []time.Duration{0, 10 * time.Minute} { // and it stays cured
			res, err = passAt(b, sealRemote(t, inner, false), sealT0.Add(2*time.Hour+after))
			clean(t, "the pass after", res, err)
		}
	}
}

// failHashes is a folder that refuses to have its hashes put.
type failHashes struct{ DirRemote }

func (f failHashes) Put(name string, data []byte, prev string) error {
	if name == HashesFile {
		return errors.New("no room")
	}
	return f.DirRemote.Put(name, data, prev)
}

// A pass whose hashes cannot be put says so by their name.
func TestSyncSealHashesErrorNamed(t *testing.T) {
	d := newDevice(t)
	d.write("A/Bir/page.json", "bir")
	res, _ := passOf(d, sealRemote(t, failHashes{DirRemote{Root: t.TempDir()}}, false))
	if len(res.Errors) != 1 || !strings.HasPrefix(res.Errors[0], HashesFile+": ") {
		t.Fatalf("the hashes' error: %v", res.Errors)
	}
}

// A folder that is not there for a pass (its drive not connected) is the remote offline, not emptied: the pass
// fails and nothing is forgotten, sent up into the mount point, or taken back as older when the folder returns.
func TestSyncSealFolderAwayAndBack(t *testing.T) {
	inner := DirRemote{Root: filepath.Join(t.TempDir(), "Volume")}
	if _, err := OpenEnc(inner, "parola"); err != nil { // setting sync up makes the folder
		t.Fatalf("setting up a folder not there yet: %v", err)
	}
	a := newDevice(t)
	a.write("A/Bir/page.json", "bir")
	a.write("A/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	away := inner.Root + " (away)"
	os.Rename(inner.Root, away)
	if _, err := passOf(a, sealRemote(t, inner, false)); !errors.Is(err, ErrNoFolder) {
		t.Fatalf("a pass with the folder away: %v", err)
	}
	if _, err := os.Stat(inner.Root); err == nil {
		t.Fatal("the pass made the folder in its place")
	}
	os.Rename(away, inner.Root)
	reload(a)
	res, err := passOf(a, sealRemote(t, inner, false))
	if clean(t, "the pass with the folder back", res, err); res.Pushed+res.Pulled+len(res.Refused) != 0 {
		t.Fatalf("the folder back: %+v", res)
	}
}

// A folder whose mount point stood empty for a pass had the library sent up into it again; when the folder comes
// back with its own copies, the same bytes as here, they are taken as they are, not refused as older ones.
func TestSyncSealMountPointThenBack(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	a := newDevice(t)
	a.write("A/Bir/page.json", "bir")
	a.write("A/İki/page.json", "iki")
	a.sync(sealRemote(t, inner, false))
	away := t.TempDir()
	ents, _ := os.ReadDir(inner.Root)
	for _, e := range ents {
		os.Rename(inner.file(e.Name()), filepath.Join(away, e.Name()))
	}
	if res, err := passOf(a, sealRemote(t, inner, false)); err != nil || res.Pushed != 2 {
		t.Fatalf("the pass into the empty mount point: %v %+v", err, res)
	}
	ents, _ = os.ReadDir(away)
	for _, e := range ents {
		os.Rename(filepath.Join(away, e.Name()), inner.file(e.Name()))
	}
	for pass := 0; pass < 2; pass++ {
		reload(a)
		res, err := passOf(a, sealRemote(t, inner, false))
		if clean(t, "the folder's own copies back", res, err); a.read("A/Bir/page.json") != "bir" {
			t.Fatal("a page changed")
		}
	}
}

// A labelled file put before the hashes that vouch for it, as another device puts a file and then its hashes, is
// waited for, not refused: once the hashes come, it is taken, and nothing is left waiting (#148, phase 2).
func TestSyncSealFileBeforeItsHashes(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	const page = "Not/Bir/page.json"
	a, b := newDevice(t), newDevice(t)
	a.write(page, "bir")
	a.sync(sealRemote(t, inner, true))
	b.sync(sealRemote(t, inner, true))
	before := mustGet(t, inner, HashesFile)
	a.write(page, "bir, sonra")
	a.sync(sealRemote(t, inner, true))
	after := mustGet(t, inner, HashesFile)
	inner.Put(HashesFile, before, HashOf(after)) // the file is there, its hashes not yet
	res, err := passAt(b, sealRemote(t, inner, true), sealT0.Add(time.Hour))
	if err != nil || !res.Waiting || len(res.Refused) != 0 || b.read(page) != "bir" || len(b.st.Unvouched) != 1 {
		t.Fatalf("before its hashes: %v %+v, here %q", err, res, b.read(page))
	}
	inner.Put(HashesFile, after, HashOf(before)) // they come
	res, err = passAt(b, sealRemote(t, inner, true), sealT0.Add(time.Hour+time.Minute))
	if err != nil || res.Waiting || len(res.Refused) != 0 || b.read(page) != "bir, sonra" || len(b.st.Unvouched) != 0 {
		t.Fatalf("after its hashes came: %v %+v, here %q", err, res, b.read(page))
	}
}
