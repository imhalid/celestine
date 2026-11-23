package library

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Synchronization. The library is a tree of small files (a page is a folder with its page.json); the
// engine compares each file with what it was at the last sync, three ways: only here changed - it goes
// up; only there - it comes down; both - the remote's stays and ours is kept beside it as "name
// (çakışma …)", so that nothing typed is lost. A deletion is a change like another: it leaves a tombstone
// that the others take. Where the files go is a Remote: a folder (iCloud, Dropbox, Syncthing do the rest) or
// a server of one's own (cloudflare/sync/worker.js); either can be wrapped in end-to-end encryption, so
// that the server sees nothing but opaque names and blobs. Nothing is sent that is not a file of the
// library, and what is not synced - a collection chosen out - never leaves.

// A RemoteEntry is what a remote says of a file.
type RemoteEntry struct {
	Hash    string `json:"hash"`
	Deleted bool   `json:"deleted,omitempty"`
	// Held says the file was gone by the time the listing read it (a folder remote, as another program renames
	// or removes it): the pass leaves its name alone, as it leaves a file gone here mid-walk, and the next one reads
	// it again (#566). Taken as not there, a file the remote still has would be deleted here.
	Held bool `json:"-"`
}

// ErrConflict says the remote's file is not the one the writer saw.
var ErrConflict = errors.New("senkron: uzaktaki dosya değişmiş")

// A Remote is the other place.
type Remote interface {
	List() (map[string]RemoteEntry, error)
	Get(name string) ([]byte, error)
	// Put writes a file if the remote's hash is prev ("" when it must not be there, or be a tombstone).
	Put(name string, data []byte, prev string) error
	// Delete leaves a tombstone, if the remote's hash is prev.
	Delete(name, prev string) error
}

func HashOf(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// ---- a folder as a remote ----

type DirRemote struct{ Root string }

func (d DirRemote) file(name string) string { return filepath.Join(d.Root, filepath.FromSlash(name)) }

// errUnsafeName is a name that would leave the folder it is joined to.
var errUnsafeName = errors.New("senkron: güvenli olmayan dosya adı")

// path is the file of name in the folder, or an error when the name leaves it.
func (d DirRemote) path(name string) (string, error) {
	p, ok := SafeSubpath(d.Root, name)
	if !ok {
		return "", errUnsafeName
	}
	return p, nil
}

// ErrNoFolder says the folder synced with is not there: a drive not connected, a share not mounted. It is the
// remote offline, not emptied: a listing of nothing would have the library sent up again into the mount point, and
// the files there taken, when it is back, for older copies. The folder is made when sync is set up (Make).
var ErrNoFolder = errors.New("the sync folder is not there")

// there says whether the folder is there, and ErrNoFolder when it is not.
func (d DirRemote) there() error {
	if fi, err := os.Stat(d.Root); err != nil || !fi.IsDir() {
		if err == nil || os.IsNotExist(err) {
			return ErrNoFolder
		}
		return err
	}
	return nil
}

// Make makes the folder, when sync is set up with it (OpenEnc).
func (d DirRemote) Make() error { return os.MkdirAll(d.Root, 0o755) }

func (d DirRemote) List() (map[string]RemoteEntry, error) {
	if err := d.there(); err != nil {
		return nil, err
	}
	out := map[string]RemoteEntry{}
	err := filepath.WalkDir(d.Root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(d.Root, p)
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, ".tmp~") { // a Put half done, here or on another device: not a file of the remote's
			return nil
		}
		fi, _ := e.Info() // before the read: a write after it has a later time, and is read again
		if !strings.HasSuffix(rel, tombSuffix) {
			if h, ok := dirHashOf(p, fi); ok { // as it was when last read: not read again (#151)
				if prev, dead := out[rel]; !dead || prev.Held {
					out[rel] = RemoteEntry{Hash: h}
				}
				return nil
			}
		}
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) { // gone while the folder was walked (#566)
			if !strings.HasSuffix(rel, tombSuffix) {
				if _, dead := out[rel]; !dead {
					out[rel] = RemoteEntry{Held: true}
				}
			}
			return nil
		}
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, tombSuffix) {
			out[strings.TrimSuffix(rel, tombSuffix)] = RemoteEntry{Deleted: true}
			return nil
		}
		h := HashOf(b)
		keepDirHash(p, fi, h)
		if prev, dead := out[rel]; !dead || prev.Held {
			out[rel] = RemoteEntry{Hash: h}
		}
		return nil
	})
	// a tombstone beside a file that is there: the later is the tombstone only if the file is gone
	for name, e := range out {
		if e.Deleted {
			if b, err := os.ReadFile(d.file(name)); err == nil {
				out[name] = RemoteEntry{Hash: HashOf(b)}
			}
		}
	}
	if os.IsNotExist(err) && d.there() != nil {
		return nil, ErrNoFolder // gone while it was read
	}
	return out, err
}

const tombSuffix = ".deleted~"

// dirHashes are the hashes of a folder remote's files as they were last read, by their path, with the size and the
// modified time they had then: a pass read and hashed every file of the folder, the whole library's bytes every few
// seconds of typing on a folder a cloud drive syncs (#151).
var dirHashes sync.Map

type dirHashed struct {
	size int64
	mod  time.Time
	hash string
}

// dirSettle is how long ago a file must have been written for its hash to be kept: a file system that keeps times in
// whole seconds, or coarser, gives a write in the same second the same time, and a write of the same size would be
// taken for the file kept. A file written since it was kept has a later time.
const dirSettle = 2 * time.Second

// dirHashOf is the hash kept of the file at p, when its size and modified time, fi, are those it had when it was read.
func dirHashOf(p string, fi fs.FileInfo) (string, bool) {
	v, ok := dirHashes.Load(p)
	if !ok || fi == nil {
		return "", false
	}
	h := v.(dirHashed)
	return h.hash, fi.Mode().IsRegular() && fi.Size() == h.size && fi.ModTime().Equal(h.mod)
}

// keepDirHash keeps hash as the file at p's, with its size and modified time as fi had them before it was read, when
// it was written a while ago.
func keepDirHash(p string, fi fs.FileInfo, hash string) {
	if fi == nil || !fi.Mode().IsRegular() || time.Since(fi.ModTime()) < dirSettle {
		dirHashes.Delete(p)
		return
	}
	dirHashes.Store(p, dirHashed{fi.Size(), fi.ModTime(), hash})
}

func (d DirRemote) current(name string) (string, error) {
	p, err := d.path(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return HashOf(b), nil
}

func (d DirRemote) Get(name string) ([]byte, error) {
	p, err := d.path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func (d DirRemote) Put(name string, data []byte, prev string) error {
	if err := d.there(); err != nil { // not into a mount point
		return err
	}
	if cur, err := d.current(name); err != nil {
		return err
	} else if cur != prev {
		return ErrConflict
	}
	p, _ := d.path(name) // current has said it is safe
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	os.Remove(p + tombSuffix)
	tmp := p + ".tmp~"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (d DirRemote) Delete(name, prev string) error {
	if err := d.there(); err != nil {
		return err
	}
	if cur, err := d.current(name); err != nil {
		return err
	} else if cur != prev {
		return ErrConflict
	}
	p, _ := d.path(name)
	os.Remove(p)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p+tombSuffix, nil, 0o644)
}

// ---- a server as a remote ----

// httpRemote speaks to a server of the protocol of cloudflare/sync/worker.js:
//
//	GET    /files            {"name": {"hash": "…", "deleted": false}, …}
//	GET    /files/<name>     the bytes
//	PUT    /files/<name>     the bytes; If-Match: <hash of what was seen>, "" for none; 412 on a conflict
//	DELETE /files/<name>     If-Match as above; leaves a tombstone
//	GET    /changes          {"changes": n}: a counter every PUT and DELETE moves (ChangeCounter)
//
// with "Authorization: Bearer <the device's token>".
type httpRemote struct {
	base, token string
	client      *http.Client
	written     atomic.Int64 // the writes the server answered "ok", each of which moved its counter once (Written)
}

// HTTPTransport is used by NewHTTPRemote when non-nil (for example netproxy.Transport()).
var HTTPTransport http.RoundTripper

func NewHTTPRemote(base, token string) *httpRemote {
	tr := HTTPTransport
	if tr == nil {
		tr = http.DefaultTransport
	}
	// no deadline for a whole transfer, which a large file on a slow line never met: a request stops when nothing
	// moves for transferIdle (send), instead (#155)
	return &httpRemote{base: strings.TrimRight(base, "/"), token: token, client: &http.Client{Transport: tr}}
}

// transferIdle is how long a request to the server may go without a byte moving, either way, or without an answer
// once it is sent, before it is given up.
var transferIdle = 30 * time.Second

// MaxFileBytes is the largest file the server takes (cloudflare/sync/worker.js, MAX_BYTES): a larger one is refused
// here, before it is sent (ErrTooLarge).
const MaxFileBytes = 50 << 20

var (
	// ErrTooLarge is a file over the server's size, refused before it is sent.
	ErrTooLarge = errors.New("the file is larger than the server takes (50 MB)")
	// ErrQuotaFull is the server's refusal of a file for the account's quota.
	ErrQuotaFull = errors.New("the account's room on the server is full")
	// ErrDeviceRefused is the server's 401: it no longer takes this device's token (signed out elsewhere, removed).
	ErrDeviceRefused = errors.New("the server no longer takes this device: sign in again")
)

// send makes req with ctx's deadline moving while bytes do: the request's body and the answer's are read through
// a watchdog that cancels the request after transferIdle with nothing read, and the wait for the answer is one too.
// The answer's body is the watchdog's until it is closed.
func (h *httpRemote) send(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	dog := time.AfterFunc(transferIdle, cancel)
	if req.Body != nil {
		req.Body = &idleReader{r: req.Body, dog: dog}
	}
	resp, err := h.client.Do(req.WithContext(ctx))
	if err != nil {
		dog.Stop()
		cancel()
		return nil, err
	}
	dog.Reset(transferIdle)
	resp.Body = &idleReader{r: resp.Body, dog: dog, done: cancel}
	return resp, nil
}

// idleReader reads r, and puts the watchdog back at each read that moved; done, if any, ends with it.
type idleReader struct {
	r    io.ReadCloser
	dog  *time.Timer
	done func()
}

func (i *idleReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	if n > 0 {
		i.dog.Reset(transferIdle)
	}
	return n, err
}

func (i *idleReader) Close() error {
	err := i.r.Close()
	if i.done != nil {
		i.dog.Stop()
		i.done()
	}
	return err
}

// statusErr is the error of an answer that is no success: the server's refusals named, the rest by their status.
func statusErr(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrDeviceRefused
	case http.StatusRequestEntityTooLarge:
		if b, _ := io.ReadAll(io.LimitReader(resp.Body, 64)); strings.Contains(string(b), "quota") {
			return ErrQuotaFull
		}
		return ErrTooLarge
	}
	return fmt.Errorf("the server answered %d", resp.StatusCode)
}

func (h *httpRemote) do(method, name string, body []byte, prev *string) (*http.Response, error) {
	if err := HTTPSOnly(h.base); err != nil {
		return nil, err
	}
	if method == "PUT" && len(body) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	u := h.base + "/files"
	if name != "" {
		u += "/" + (&url.URL{Path: name}).EscapedPath()
	}
	req, err := http.NewRequest(method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	if prev != nil {
		req.Header.Set("If-Match", `"`+*prev+`"`)
	}
	resp, err := h.send(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusPreconditionFailed {
		resp.Body.Close()
		return nil, ErrConflict
	}
	if resp.StatusCode/100 != 2 {
		err := statusErr(resp)
		resp.Body.Close()
		return nil, err
	}
	if method == "PUT" || method == "DELETE" {
		h.written.Add(1)
	}
	return resp, nil
}

func (h *httpRemote) List() (map[string]RemoteEntry, error) {
	resp, err := h.do("GET", "", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out := map[string]RemoteEntry{}
	return out, json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&out)
}

func (h *httpRemote) Get(name string) ([]byte, error) {
	resp, err := h.do("GET", name, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

func (h *httpRemote) Put(name string, data []byte, prev string) error {
	resp, err := h.do("PUT", name, data, &prev)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (h *httpRemote) Delete(name, prev string) error {
	resp, err := h.do("DELETE", name, nil, &prev)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

// A RemoteVersion is an earlier state of a file that the server kept (cloudflare/sync/worker.js, VERSIONS): when what it holds was
// written, in milliseconds, and its size.
type RemoteVersion struct {
	At   int64 `json:"at"`
	Size int   `json:"size"`
}

// versioned is a remote that keeps earlier states of its files.
type versioned interface {
	Versions(name string) ([]RemoteVersion, error)
	VersionData(name string, at int64) ([]byte, error)
}

var errNoVersions = errors.New("bu eşitleme yeri sürüm geçmişi tutmuyor")

func (h *httpRemote) versionURL(name string, at int64) string {
	u := h.base + "/versions/" + (&url.URL{Path: name}).EscapedPath()
	if at > 0 {
		u += "?at=" + fmt.Sprintf("%013d", at)
	}
	return u
}

func (h *httpRemote) get(u string) ([]byte, error) {
	if err := HTTPSOnly(h.base); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	resp, err := h.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, statusErr(resp)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (h *httpRemote) Versions(name string) ([]RemoteVersion, error) {
	b, err := h.get(h.versionURL(name, 0))
	if err != nil {
		return nil, err
	}
	var out []RemoteVersion
	return out, json.Unmarshal(b, &out)
}

func (h *httpRemote) VersionData(name string, at int64) ([]byte, error) {
	return h.get(h.versionURL(name, at))
}

// A ChangeCounter is a remote that tells, at the cost of a small request, whether anything was written to it: a
// counter that each write moves. A device asks it often, and lists the files only when it moved.
type ChangeCounter interface {
	Changes() (int64, error)
}

func (h *httpRemote) Changes() (int64, error) {
	n, _, err := h.CountedChanges()
	return n, err
}

// CountedChanges is the server's change counter, and whether it moves once for each write it answers "ok" and for
// nothing else, so that a device that counts its own (Written) can tell whether another wrote (#151).
func (h *httpRemote) CountedChanges() (int64, bool, error) {
	b, err := h.get(h.base + "/changes")
	if err != nil {
		return 0, false, err
	}
	var out struct {
		Changes *int64 `json:"changes"`
		Exact   bool   `json:"exact"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Changes == nil {
		return 0, false, errNoChanges
	}
	return *out.Changes, out.Exact, nil
}

// Written is how many writes the server answered "ok" to this remote.
func (h *httpRemote) Written() int64 { return h.written.Load() }

// ExactCounter is a remote whose server counts its writes exactly (CountedChanges), and that counts its own.
type ExactCounter interface {
	CountedChanges() (int64, bool, error)
	Written() int64
}

var errNoChanges = errors.New("bu eşitleme yeri değişiklik sayacı tutmuyor")

// Changes is the inner remote's counter: the encryption hides names and bytes, not that something was written.
func (e *EncRemote) Changes() (int64, error) {
	if c, ok := e.inner.(ChangeCounter); ok {
		return c.Changes()
	}
	return 0, errNoChanges
}

// CountedChanges and Written are the inner remote's (ExactCounter): the index's and the hashes' writes are its too.
func (e *EncRemote) CountedChanges() (int64, bool, error) {
	if c, ok := e.inner.(ExactCounter); ok {
		return c.CountedChanges()
	}
	return 0, false, errNoChanges
}

func (e *EncRemote) Written() int64 {
	if c, ok := e.inner.(ExactCounter); ok {
		return c.Written()
	}
	return 0
}

// listedFrom takes listed as the pass's listing, where the remote was not listed (SyncState.Unchanged): what the
// sealed pass learns of a listing, the sealed copy each file has there, is what the state says this device left.
func (e *EncRemote) listedFrom(listed map[string]RemoteEntry) {
	e.listed, e.inner0 = map[string]string{}, map[string]RemoteEntry{}
	for p, r := range listed {
		e.listed[p] = r.Hash
		e.inner0[e.opaque(p)] = r
	}
}

// ---- end-to-end encryption ----

// EncRemote wraps a remote: each file is sealed with AES-256-GCM under a key from the passphrase, and
// is stored under a name that is an HMAC of its path: the other side sees the count and the sizes of
// the files, and nothing else. A small sealed index, "_index", maps the names back to paths. Lose the
// passphrase and the recovery key, and the data is gone: that is what it is for.
//
// A remote that is not to be trusted with the data is not trusted with what it serves either (#148). The index
// carries a generation, a counter of each device that put it: an index older than one this device has seen is
// refused (ErrIndexRollback). Beside it a sealed object of its own, "_hashes", keeps for each file the hash of its
// bytes and of its sealed bytes. Each put seals with a nonce of its own, so that an honest write is never
// byte for byte a copy that was there before: the sealed bytes of another file, of an older copy this device
// held, or of a file deleted since, are refused (ErrFileRollback). The sync state keeps what each pass saw, since
// the remote is made again at each pass (beginPass, endPass).
type EncRemote struct {
	inner  Remote
	Key    []byte
	gcm    cipher.AEAD
	idx    map[string]string // opaque name → path
	loaded bool
	// indexed says the remote held a sealed index when it was read, and so that the key opened it (HasIndex)
	indexed bool
	// checked says the key opened the remote's check (CheckFile), put there before (Checked)
	checked bool
	// sent is the remote's hash of each file put in this pass, by its name: the hash of the sealed bytes, as a
	// folder and the Worker both have it, so that the pass need not list the remote again to learn it (PutHash)
	sent map[string]string
	// Labels seals what is put with its label (indexAAD, hashesAAD, fileAAD), and refuses a labelled file whose
	// bytes are not the ones the hashes give (sealWithLabels).
	Labels bool

	// raw is the hash of the index's sealed bytes as last read or put ("" for none): it is put against it
	raw string
	// seen is the generation of the index this device has seen, by device; device is this one's id
	seen   genVector
	device string
	// numbered says the index read had a generation; known is the short names of the index the sync state had seen
	numbered bool
	known    map[string]bool
	// trustIndex takes, for one pass, an index older than one seen, or none (SyncState.TrustIndex); trusted says
	// it was taken so, and reput that it was put again since, above what was seen
	trustIndex, trusted, reput bool
	// now is the pass's time; indexSince and hashesSince when the index, or the hashes, were first found missing
	// where they were (0 for not): a folder copied by another program may bring them late, so they are waited for a
	// while (missingWait) before what is missing is taken for withheld
	now                     time.Time
	indexSince, hashesSince int64
	// unvouched is when each labelled file the hashes did not vouch for was first found so (SyncState.Unvouched):
	// another device puts a file before the hashes that vouch for it, so it is waited for a while (missingWait)
	// before it is taken for a copy that is not the file's (#148)
	unvouched map[string]int64

	// hashes is the remote's "_hashes" as read, by short name (hashKey); hraw the hash of its sealed bytes, and
	// hloaded whether it was read; mine is what this pass knows better
	hashes  map[string]fileHashes
	hraw    string
	hloaded bool
	mine    map[string]knownHash
	// hseen is the generation of the hashes this device has seen; hrolled says the remote served older ones, or none
	// where they were for longer than missingWait, so that what is fetched is refused; hwait that they are missing
	// and waited for, so that nothing is fetched and they are not put
	hseen          genVector
	hrolled, hwait bool
	deletedHere    map[string]bool   // the files this pass deleted on the remote
	have           map[string]string // the short hash of each file as this device has it synced (SyncState.Files)
	// bySealed is the short name of the file each sealed hash the hashes have is of
	bySealed map[string]string
	// listed is the remote's hash of each file as the pass listed it, by path; inner the remote's own listing
	listed map[string]string
	inner0 map[string]RemoteEntry
	// got is the hash of the sealed bytes of each file this pass fetched and took, as hashed here
	got map[string]string
	// earlier is the sealed copies of each file this device held (SyncState.Earlier), and heldBy the path of each
	earlier map[string][]string
	heldBy  map[string]string
	// trustFiles takes, for one pass, the files the checks would refuse (SyncState.TrustFiles)
	trustFiles bool
}

// sealWithLabels is whether a remote seals what it puts with labels, and refuses a labelled file that the hashes do
// not vouch for once it has waited for them (EncRemote.Labels). It is on since every device reads both forms (#148,
// phase 2). A file sealed before, with no label, still opens, and is sealed with its label when it is next written.
const sealWithLabels = true

// PutHash is the remote's hash of what was put under name, when it is known without asking the remote.
func (e *EncRemote) PutHash(name string) (string, bool) {
	h, ok := e.sent[name]
	return h, ok
}

const SaltFile = "_salt"

// DeriveKey makes the key from a passphrase and the salt that is kept on the remote (made when there is none).
func DeriveKey(pass string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, pass, salt, 200_000, 32)
}

// RecoveryKey is the key as words to write down: base32, in groups.
func RecoveryKey(key []byte) string {
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	var parts []string
	for i := 0; i < len(s); i += 4 {
		parts = append(parts, s[i:min(i+4, len(s))])
	}
	return strings.Join(parts, "-")
}

// KeyFromRecovery reads what recoveryKey wrote.
func KeyFromRecovery(s string) ([]byte, error) {
	s = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), " ", ""))
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, errors.New("kurtarma anahtarı geçersiz")
	}
	return b, nil
}

func NewEncRemote(inner Remote, key []byte) (*EncRemote, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &EncRemote{inner: inner, Key: key, gcm: g, idx: map[string]string{}, Labels: sealWithLabels}, nil
}

// OpenEnc opens the remote with a passphrase: the salt is read from it, or made and put there. A remote that
// holds a sealed index must open with the key, or the passphrase is refused (ErrWrongPassphrase).
func OpenEnc(inner Remote, pass string) (*EncRemote, error) {
	salt, err := inner.Get(SaltFile)
	made := false // the salt is made here: this passphrase is the remote's first, and its check is put with it
	if err != nil {
		list, lerr := inner.List()
		if m, ok := inner.(interface{ Make() error }); ok && errors.Is(lerr, ErrNoFolder) {
			// setting sync up is when its folder is made: a pass later finds it, or is offline
			if lerr = m.Make(); lerr == nil {
				list = map[string]RemoteEntry{}
			}
		}
		if lerr != nil {
			return nil, lerr
		}
		if e, ok := list[SaltFile]; ok && !e.Deleted {
			return nil, err
		}
		salt = make([]byte, 16)
		rand.Read(salt)
		if err := inner.Put(SaltFile, salt, ""); err != nil {
			return nil, err
		}
		made = true
	}
	key, err := DeriveKey(pass, salt)
	if err != nil {
		return nil, err
	}
	e, err := NewEncRemote(inner, key)
	if err != nil {
		return nil, err
	}
	// the index, when the remote has one, is opened with the key: a passphrase that is not the one the remote
	// was sealed with is refused here, not kept to fail every pass after (#153)
	if err := e.loadIndex(); errors.Is(err, errSealed) {
		return nil, ErrWrongPassphrase
	} else if err != nil {
		return nil, err
	}
	// and so is the check, which a remote has from its first key on, before any index: a second device joining with
	// a typo, before the first one's index is there, would otherwise seal under another key and lock it out (#562)
	if err := e.check(made); err != nil {
		return nil, err
	}
	return e, nil
}

// CheckFile holds a short text sealed with the remote's key, put beside the salt by the passphrase that made the
// salt: a passphrase is checked against it when the remote has no index yet to check it against. A remote whose
// salt came before the check has none, and a passphrase there is checked only by the index, as before.
const CheckFile = "_check"

var (
	checkAAD  = []byte("celestine/check/v1")
	checkText = []byte("celestine passphrase check")
)

// check opens the remote's check with the key, and puts one there when it has none and first (the salt made with
// this key) says the key is the remote's first: ErrWrongPassphrase when the check is there and does not open. A key
// that is not known to be the remote's never puts it: a typo would then be the check the right passphrase fails.
func (e *EncRemote) check(first bool) error {
	for range 2 {
		b, err := e.inner.Get(CheckFile)
		if err == nil {
			if out, _, err := e.open(b, checkAAD); err != nil || !bytes.Equal(out, checkText) {
				return ErrWrongPassphrase
			}
			e.checked = true
			return nil
		}
		list, lerr := e.inner.List()
		if lerr != nil {
			return lerr
		}
		if c, ok := list[CheckFile]; ok && !c.Deleted {
			return err // there, and not read: not a reason to put another
		}
		if !first {
			return nil // nothing to check the key against: the index, when there is one, has checked it
		}
		err = e.inner.Put(CheckFile, e.seal(checkText, checkAAD), "")
		if errors.Is(err, ErrConflict) { // put by another device as this one read: read it
			continue
		}
		return err // the first check: what it checks is the next passphrase, not this one
	}
	return ErrConflict
}

// Checked says whether the key is known to be the remote's: it opened the remote's index, or its check. A key that
// made the remote's salt and check is not: it is what it is, with nothing before it to agree with. A passphrase
// entered again replaces the key kept only when it is checked (setSyncPassphrase).
func (e *EncRemote) Checked() bool { return e.indexed || e.checked }

// ErrWrongPassphrase says a passphrase does not open what the remote holds: it is not the one its files were
// sealed with.
var ErrWrongPassphrase = errors.New("the passphrase does not open the remote")

// errSealed says a sealed file did not open with the key.
var errSealed = errors.New("şifre çözülemedi: parola ya da anahtar yanlış")

// ErrIndexRollback says the remote served an index older than one this device has seen, or none where it had one:
// files would be hidden by it, and read as deleted, so the pass takes nothing from it (#148).
var ErrIndexRollback = errors.New("the remote's index is older than one this device has seen")

// ErrRemoteNotYet says the remote holds files but not, for now, the index or the hashes this device has seen: a
// folder copied by another program may bring them late. The pass, or the file, waits for them, missingWait at
// most; past it they are taken for withheld (ErrIndexRollback, ErrFileRollback).
var ErrRemoteNotYet = errors.New("waiting for the remote's index or hashes")

// missingWait is how long the index, or the hashes, may be missing before that is taken for a remote withholding them.
const missingWait = 5 * time.Minute

// ErrFileRollback says the remote served under a file's name bytes that are not its copy: another file's, an older
// copy, or a file deleted since. It is not written over the copy here (#148).
var ErrFileRollback = errors.New("the remote's copy is not this file's latest")

// The labels things are sealed with when labels are on (EncRemote.Labels): what opens under one was put as it.
var (
	indexAAD  = []byte("ryudit/index/v1")
	hashesAAD = []byte("ryudit/hashes/v1")
)

// fileAAD is the label a file is sealed with when labels are on: its path, so that the sealed bytes of one file
// served under the name of another do not open.
func fileAAD(p string) []byte { return []byte("ryudit/file/v1\x00" + p) }

// seal seals b with a nonce of its own in front: under the label aad when labels are on, with none when they are off.
func (e *EncRemote) seal(b, aad []byte) []byte {
	if !e.Labels {
		aad = nil
	}
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, aad)
}

// open opens what seal sealed, under the label aad first, and with none then: what was sealed before labels, or
// with them off, is read too, and unlabelled says so.
func (e *EncRemote) open(b, aad []byte) (out []byte, unlabelled bool, err error) {
	n := e.gcm.NonceSize()
	if len(b) < n {
		return nil, false, errors.New("şifreli dosya kısa")
	}
	if out, err := e.gcm.Open(nil, b[:n], b[n:], aad); err == nil {
		return out, false, nil
	}
	out, err = e.gcm.Open(nil, b[:n], b[n:], nil)
	if err != nil {
		return nil, false, errSealed
	}
	return out, true, nil
}

// short is the part of a hash the hashes and the sync state keep: a quarter of a SHA-256 tells a copy from one
// sealed again, and the bytes of one file from another's.
func short(h string) string { return h[:min(16, len(h))] }

func (e *EncRemote) opaque(p string) string {
	m := hmac.New(sha256.New, e.Key)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

// hashKey is the name a file has in the hashes: its opaque name, short.
func (e *EncRemote) hashKey(p string) string { return short(e.opaque(p)) }

const (
	IndexFile  = "_index"
	HashesFile = "_hashes"
)

// genKey is the entry of the index that holds its generation, a JSON object of a counter by device, as a string:
// the index stays the map of strings a version from before #148 reads, which takes the entry for a name with no
// object (no object is named so), leaves it out of its List, and puts it back as it found it in a union.
const genKey = "_gen"

// A genVector is the generation of the index: by device, how many times it put the index.
type genVector map[string]int64

// below says whether g is older than seen: no counter above seen's, and one below. Neither below the other is
// two writes that did not see each other (a folder copied by another program keeps one of them): not older.
func (g genVector) below(seen genVector) bool {
	lower := false
	for d, n := range g {
		if n > seen[d] {
			return false
		}
	}
	for d, n := range seen {
		if g[d] < n {
			lower = true
		}
	}
	return lower
}

// join is the highest counter of each device in g and o.
func (g genVector) join(o genVector) genVector {
	out := genVector{}
	for d, n := range g {
		out[d] = n
	}
	for d, n := range o {
		out[d] = max(out[d], n)
	}
	return out
}

// readIndex opens a sealed index: its names, its generation, and whether it had one.
func (e *EncRemote) readIndex(raw []byte) (idx map[string]string, gen genVector, numbered bool, err error) {
	plain, _, err := e.open(raw, indexAAD)
	if err != nil {
		return nil, nil, false, err
	}
	idx = map[string]string{}
	if err := json.Unmarshal(plain, &idx); err != nil {
		return nil, nil, false, err
	}
	gen = genVector{}
	g, numbered := idx[genKey]
	if numbered && json.Unmarshal([]byte(g), &gen) != nil {
		gen, numbered = genVector{}, false
	}
	delete(idx, genKey)
	return idx, gen, numbered, nil
}

// checkIndex refuses an index older than the one this device has seen, unless the remote is trusted again. An index
// below what was seen that names something this device never knew is not an older one served again but a write
// that did not see this device's: a version from before #148 carries the generation it read.
func (e *EncRemote) checkIndex(idx map[string]string, gen genVector) error {
	if !gen.below(e.seen) {
		return nil
	}
	for op := range idx {
		if !e.known[short(op)] {
			return nil
		}
	}
	if e.trustIndex {
		e.trusted = true
		return nil
	}
	return fmt.Errorf("%w (generation %v, seen %v)", ErrIndexRollback, gen, e.seen)
}

// onlyItsOwn says whether a listing holds nothing but what the remote keeps for itself: a remote emptied, or never used.
func onlyItsOwn(list map[string]RemoteEntry) bool {
	for n := range list {
		if n != SaltFile && n != HashesFile && n != IndexFile && n != CheckFile {
			return false
		}
	}
	return true
}

func (e *EncRemote) loadIndex() error {
	if e.loaded {
		return nil
	}
	b, err := e.inner.Get(IndexFile)
	if err != nil {
		list, lerr := e.inner.List()
		if lerr != nil {
			return lerr
		}
		if ent, ok := list[IndexFile]; ok && !ent.Deleted {
			return err
		}
		// no index: a remote never used, or emptied, has nothing else either; one that holds files had an index,
		// and one this device saw, withheld, would hide them all
		switch {
		case onlyItsOwn(list): // emptied: its index and hashes start again
			e.seen, e.known, e.hseen = genVector{}, map[string]bool{}, genVector{}
		case len(e.seen) > 0 && e.trustIndex:
			e.trusted = true
		case len(e.seen) > 0:
			if e.indexSince == 0 {
				e.indexSince = e.now.Unix()
			}
			if e.now.Unix()-e.indexSince < int64(missingWait/time.Second) {
				return ErrRemoteNotYet // a folder may bring it late
			}
			return fmt.Errorf("%w (no index)", ErrIndexRollback)
		}
		e.loaded, e.raw, e.numbered = true, "", true
		return nil
	}
	e.indexSince = 0
	idx, gen, numbered, err := e.readIndex(b)
	if err != nil {
		return err
	}
	if err := e.checkIndex(idx, gen); err != nil {
		return err // nothing is taken from it: the pass stops at List
	}
	for op, p := range idx {
		e.idx[op] = p
	}
	e.seen, e.numbered, e.raw = e.seen.join(gen), numbered, HashOf(b)
	e.loaded, e.indexed = true, true
	return nil
}

// HasIndex says whether the remote held a sealed index when it was opened, which the key opened: only then is a
// passphrase known to be the remote's (OpenEnc). A remote with a salt and no index takes any passphrase.
func (e *EncRemote) HasIndex() bool { return e.indexed }

// saveIndex puts the index back, merged with what the remote holds now: another device may have added
// names since this one read it, and an index written from the old copy would drop them (their files would
// then not be seen, and their owner would take them for deleted). Names are only ever added, so a union
// loses nothing; if the index changes again while it is being put, it is read and merged once more. It is put
// first against the index as this remote last read it, with no listing, with this device's counter one above
// any it has seen.
func (e *EncRemote) saveIndex() error {
	for try := 0; try < 8; try++ {
		if try > 0 { // the remote's index is not the one last read: read it again
			e.raw = ""
			list, err := e.inner.List()
			if err != nil {
				return err
			}
			if ent, ok := list[IndexFile]; ok && !ent.Deleted {
				raw, err := e.inner.Get(IndexFile)
				if err != nil {
					return err
				}
				theirs, gen, _, err := e.readIndex(raw)
				if err != nil {
					return err
				}
				if err := e.checkIndex(theirs, gen); err != nil {
					return err
				}
				for op, p := range theirs {
					e.idx[op] = p
				}
				e.seen = e.seen.join(gen)
				e.raw = HashOf(raw) // the hash of the bytes: that of a folder's file and of the Worker's
			}
		}
		next := e.seen.join(nil)
		next[e.device]++
		out := make(map[string]string, len(e.idx)+1)
		for op, p := range e.idx {
			out[op] = p
		}
		gb, _ := json.Marshal(next)
		out[genKey] = string(gb)
		plain, _ := json.Marshal(out)
		sealed := e.seal(plain, indexAAD)
		switch err := e.inner.Put(IndexFile, sealed, e.raw); {
		case err == nil:
			e.seen, e.raw, e.numbered, e.indexed = next, HashOf(sealed), true, true
			e.reput = e.trusted
			return nil
		case errors.Is(err, ErrConflict):
			continue
		default:
			return err
		}
	}
	return ErrConflict
}

// fileHashes is what the hashes keep of a file: the short hash of its bytes ("" when it is not known) and of its
// sealed bytes ("" when it is not known), and whether it was deleted (its last sealed bytes then).
type fileHashes struct {
	plain, sealed string
	deleted       bool
}

// String is how an entry is written in the hashes: "plain:sealed", or "-:sealed" for a file deleted.
func (f fileHashes) String() string {
	if f.deleted {
		return "-:" + f.sealed
	}
	return f.plain + ":" + f.sealed
}

func parseFileHashes(s string) fileHashes {
	plain, sealed, _ := strings.Cut(s, ":")
	if plain == "-" {
		return fileHashes{sealed: sealed, deleted: true}
	}
	return fileHashes{plain: plain, sealed: sealed}
}

// knownHash is what this pass knows of a file for the hashes, and the entry the hashes had for it when it was
// learnt: it goes in only while they still have that one, so that a newer one another device put is kept.
type knownHash struct {
	now fileHashes
	was string
}

// loadHashes reads the remote's hashes, once a pass. Hashes that cannot be read vouch for nothing: they only make
// the checks weaker, never refuse.
func (e *EncRemote) loadHashes() {
	if e.hloaded {
		return
	}
	e.hloaded, e.hraw, e.hashes = true, "", map[string]fileHashes{}
	gen, found := genVector{}, false
	if raw, err := e.inner.Get(HashesFile); err == nil {
		found, e.hraw = true, HashOf(raw)
		if plain, _, err := e.open(raw, hashesAAD); err == nil {
			m := map[string]string{}
			json.Unmarshal(plain, &m)
			json.Unmarshal([]byte(m[genKey]), &gen)
			delete(m, genKey)
			for k, s := range m {
				e.hashes[k] = parseFileHashes(s)
			}
		}
	}
	// hashes older than seen, or none where some were, would let what they vouched against through: nothing is
	// fetched on their word (vouch), and they are not put back, until the user trusts the remote again
	switch {
	case found:
		e.hashesSince = 0
		e.hrolled = gen.below(e.hseen) && !e.trustFiles
	case len(e.hseen) == 0:
	case e.trustFiles: // trusted again: put anew, from what this pass learns
		e.hashesSince = 0
	default:
		if e.hashesSince == 0 {
			e.hashesSince = e.now.Unix()
		}
		e.hwait = e.now.Unix()-e.hashesSince < int64(missingWait/time.Second)
		e.hrolled = !e.hwait
	}
	e.hseen = e.hseen.join(gen)
	e.bySealed = map[string]string{}
	for k, f := range e.hashes {
		if f.sealed != "" {
			e.bySealed[f.sealed] = k
		}
	}
}

// entry is the hashes' entry of the file at key, with what this pass knows better over it.
func (e *EncRemote) entry(key string) (fileHashes, bool) {
	if k, ok := e.mine[key]; ok {
		return k.now, true
	}
	f, ok := e.hashes[key]
	return f, ok
}

// know records what this pass knows of the file at p for the hashes.
func (e *EncRemote) know(p string, f fileHashes) {
	e.loadHashes()
	if e.mine == nil {
		e.mine = map[string]knownHash{}
	}
	key := e.hashKey(p)
	was := ""
	if k, ok := e.mine[key]; ok {
		was = k.was
	} else if old, ok := e.hashes[key]; ok {
		was = old.String()
	}
	e.mine[key] = knownHash{f, was}
	if f.sealed != "" {
		e.bySealed[f.sealed] = key
	}
}

// vouch says whether the bytes of short hash plain, sealed as the bytes of hash sealed, may be taken as the file at
// p. Refused are sealed bytes that were there before: a copy this device held of the file, not its last; the copy
// it or the hashes have of another file (two files exchanged); the last copy of a file deleted since. With labels on,
// a labelled file must also be the one the hashes give. What is taken goes in the hashes.
func (e *EncRemote) vouch(p, plain, sealed string, unlabelled bool) error {
	e.loadHashes()
	key, s := e.hashKey(p), short(sealed)
	// the bytes this device has of the file already: nothing is written over, so whatever copy brought them is
	// taken, and held as the last (a folder back after it was away, its files sent up again meanwhile, has them so)
	if have, ok := e.have[p]; ok && have == plain {
		e.know(p, fileHashes{plain: plain, sealed: s})
		if e.got == nil {
			e.got = map[string]string{}
		}
		e.got[p] = s
		return nil
	}
	if !e.trustFiles {
		if held := e.earlier[p]; len(held) > 0 && slices.Contains(held[:len(held)-1], s) {
			return fmt.Errorf("%w: an older copy", ErrFileRollback)
		}
		if q, ok := e.heldBy[s]; ok && q != p {
			return fmt.Errorf("%w: it is %s", ErrFileRollback, q)
		}
		if k, ok := e.bySealed[s]; ok {
			if f, _ := e.entry(k); f.sealed == s {
				if k != key {
					return fmt.Errorf("%w: it is another file", ErrFileRollback)
				}
				if f.deleted {
					return fmt.Errorf("%w: it was deleted", ErrFileRollback)
				}
			}
		}
		switch {
		case e.hrolled:
			return fmt.Errorf("%w: the remote's hashes are older than this device has seen", ErrFileRollback)
		case e.hwait:
			return ErrRemoteNotYet
		}
		if e.Labels && !unlabelled {
			if f, ok := e.entry(key); !ok || f.deleted || f.plain != plain {
				if e.unvouched == nil {
					e.unvouched = map[string]int64{}
				}
				since := e.unvouched[p]
				if since == 0 {
					since = e.now.Unix()
					e.unvouched[p] = since
				}
				if e.now.Unix()-since < int64(missingWait/time.Second) { // its hashes may yet come
					return ErrRemoteNotYet
				}
				return fmt.Errorf("%w: the hashes do not vouch for it", ErrFileRollback)
			}
		}
	}
	delete(e.unvouched, p)
	e.know(p, fileHashes{plain: plain, sealed: s})
	if e.got == nil {
		e.got = map[string]string{}
	}
	e.got[p] = s
	return nil
}

// saveHashes puts the hashes with what this pass knows better, against the copy it read; on a conflict it reads
// them again, and what another device put since stays. An entry whose file is not on the remote at all, as the pass
// listed it, is left out, and one whose file has a tombstone is kept as deleted.
func (e *EncRemote) saveHashes() error {
	for try := 0; try < 8; try++ {
		if try > 0 {
			e.hloaded = false
			e.loadHashes()
		}
		if e.hrolled || e.hwait {
			return nil // not put over hashes older than seen, nor where they may yet come
		}
		read := map[string]string{}
		out := map[string]string{}
		for k, f := range e.hashes {
			out[k], read[k] = f.String(), f.String()
		}
		for k, m := range e.mine {
			if out[k] == m.was {
				out[k] = m.now.String()
			}
		}
		if e.inner0 != nil && try == 0 { // read again, the hashes may name what was put after the pass listed
			opOf := map[string]string{}
			for op := range e.inner0 {
				opOf[short(op)] = op
			}
			for k, s := range out {
				if _, ours := e.mine[k]; ours {
					continue
				}
				op, there := opOf[k]
				switch f := parseFileHashes(s); {
				case !there && e.hashes[k].String() == s:
					delete(out, k)
				case there && e.inner0[op].Deleted && !f.deleted && f.sealed != "":
					out[k] = fileHashes{sealed: f.sealed, deleted: true}.String()
				}
			}
		}
		if maps.Equal(out, read) && e.hraw != "" && !e.trustFiles {
			e.mine = nil
			return nil // nothing new: a pass that only took what another device put does not put them again
		}
		next := e.hseen.join(nil)
		next[e.device]++
		gb, _ := json.Marshal(next)
		withGen := maps.Clone(out)
		withGen[genKey] = string(gb)
		plain, _ := json.Marshal(withGen)
		sealed := e.seal(plain, hashesAAD)
		switch err := e.inner.Put(HashesFile, sealed, e.hraw); {
		case err == nil:
			e.hraw, e.hseen = HashOf(sealed), next
			e.hashes = map[string]fileHashes{}
			for k, s := range out {
				e.hashes[k] = parseFileHashes(s)
			}
			e.mine = nil
			return nil
		case errors.Is(err, ErrConflict):
			continue
		default:
			return err
		}
	}
	return ErrConflict
}

// beginPass gives the remote what the sync state knows of it: this device's id, the generation of the index and
// its names it has seen, the sealed copies of each file it held, and whether the user trusts the remote again for
// this pass. An index read before (OpenEnc) is read again, under them.
func (e *EncRemote) beginPass(st *SyncState, now time.Time) {
	if st.Device == "" {
		id := make([]byte, 8)
		rand.Read(id)
		st.Device = hex.EncodeToString(id)
	}
	e.device = st.Device
	e.seen = genVector(st.IndexSeen).join(nil)
	e.known = map[string]bool{}
	for i := 0; i+16 <= len(st.IndexNames); i += 16 {
		e.known[st.IndexNames[i:i+16]] = true
	}
	e.earlier, e.heldBy = st.Earlier, map[string]string{}
	for p, list := range st.Earlier {
		for _, s := range list {
			if s != deletedMark {
				e.heldBy[s] = p
			}
		}
	}
	e.trustIndex, e.trustFiles = st.TrustIndex, st.TrustFiles
	e.hseen = genVector(st.HashesSeen).join(nil)
	e.now, e.indexSince, e.hashesSince = now, st.IndexMissingSince, st.HashesMissingSince
	e.unvouched = maps.Clone(st.Unvouched)
	e.have = map[string]string{}
	for p, f := range st.Files {
		if f.Local != "" && f.Remote != "" {
			e.have[p] = short(f.Local)
		}
	}
	if e.loaded {
		e.loaded, e.idx = false, map[string]string{}
	}
}

// endPass puts what the pass learnt, and keeps in the sync state what it saw. The index is put once more when it
// had no generation, or was taken trusted again (its counters then go above what was seen); the hashes when this
// pass knows better. A file the remote listed as this device last synced it has the hash of its bytes here then,
// which goes in the hashes when they have another or none: a library synced before #148 has them all after one
// pass, with nothing sent again. A trust the pass did not use up stays for the next one.
func (e *EncRemote) endPass(st *SyncState) error {
	var err error
	if e.loaded {
		if !e.numbered || e.trusted && !e.reput {
			if ierr := e.saveIndex(); ierr != nil {
				err = fmt.Errorf("%s: %w", IndexFile, ierr)
			}
		}
		e.loadHashes()
		for n, f := range st.Files {
			if f.Local == "" || f.Remote == "" || e.listed[n] != f.Remote {
				continue
			}
			key := e.hashKey(n)
			if _, ours := e.mine[key]; ours {
				continue
			}
			// the remote holds the copy this device last synced: its bytes are the ones here then, and its sealed
			// bytes are the ones the remote lists, which were this device's to put or to fetch
			if want := (fileHashes{plain: short(f.Local), sealed: short(f.Remote)}); e.hashes[key] != want {
				e.know(n, want)
			}
		}
		if len(e.mine) > 0 || e.trustFiles {
			if herr := e.saveHashes(); herr != nil && err == nil {
				err = fmt.Errorf("%s: %w", HashesFile, herr)
			}
		}
		if err == nil {
			st.TrustIndex = false
		}
		st.TrustFiles = false
		names := make([]string, 0, len(e.idx))
		for op := range e.idx {
			names = append(names, short(op))
		}
		sort.Strings(names)
		st.IndexNames = strings.Join(names, "")
	}
	st.IndexMissingSince, st.HashesMissingSince = e.indexSince, e.hashesSince
	st.Unvouched = e.unvouched
	st.IndexSeen, st.HashesSeen = e.seen.join(nil), e.hseen.join(nil)
	st.Earlier = e.held(st)
	return err
}

// deletedMark ends the sealed copies the sync state keeps of a file deleted: each of them is then an older one.
const deletedMark = "-"

// earlierKept is how many sealed copies of a file the sync state keeps at most, and heldKept of all files: past
// it, the longest lists lose their oldest first.
const (
	earlierKept = 256
	heldKept    = 50_000
)

// held is the sealed copies each file had as this device synced it, the one the remote has last: what this pass
// fetched and took, or put, is hashed here, not taken from the remote's word, but for the first copy of a file synced
// before, which the remote lists as this device last synced it. A file the state no longer has ends with deletedMark
// only when it was deleted: by this pass, or with a tombstone the remote lists. One that only went missing (a folder
// copied by another program may bring it late) keeps its last copy as its last, and that copy is taken back.
func (e *EncRemote) held(st *SyncState) map[string][]string {
	out := map[string][]string{}
	add := func(p, s string) {
		if list := out[p]; len(list) == 0 || list[len(list)-1] != s {
			out[p] = append(list, s)
		}
	}
	for p, list := range st.Earlier {
		out[p] = slices.Clone(list)
	}
	for p, f := range st.Files {
		switch {
		case f.Remote == "":
		case e.sent[p] != "" && f.Remote == e.sent[p]:
			add(p, short(e.sent[p]))
		case e.got[p] != "" && f.Remote == e.listed[p]:
			add(p, e.got[p])
		case len(out[p]) == 0 && f.Remote == e.listed[p]:
			add(p, short(f.Remote))
		}
	}
	for p := range out {
		if _, ok := st.Files[p]; !ok && (e.deletedHere[p] || e.inner0[e.opaque(p)].Deleted) {
			add(p, deletedMark)
		}
	}
	lens, total := make([]int, 0, len(out)), 0
	for p, list := range out {
		if len(list) > earlierKept {
			out[p] = list[len(list)-earlierKept:]
		}
		lens = append(lens, len(out[p]))
		total += len(out[p])
	}
	if total > heldKept { // the longest lists are cut to one length, as high as fits
		sort.Ints(lens)
		limit, sum := 0, 0
		for i, n := range lens {
			if sum+n*(len(lens)-i) > heldKept {
				limit = (heldKept - sum) / (len(lens) - i)
				break
			}
			sum += n
		}
		for p, list := range out {
			if len(list) > limit {
				out[p] = list[len(list)-max(limit, 1):]
			}
		}
	}
	return out
}

func (e *EncRemote) List() (map[string]RemoteEntry, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	// an index taken trusted again is put again, with the names here (Announce), before anything is read from it:
	// what it leaves out would read as deleted
	if e.trusted && !e.reput {
		if err := e.saveIndex(); err != nil {
			return nil, err
		}
	}
	e.loadHashes() // before the listing: an entry put after it, of a file put after it, is not taken for a stale one
	raw, err := e.inner.List()
	if err != nil {
		return nil, err
	}
	out := map[string]RemoteEntry{}
	e.listed, e.inner0 = map[string]string{}, raw
	for op, p := range e.idx {
		if r, ok := raw[op]; ok {
			out[p] = r // the hash is of the sealed bytes: it changes at each seal, so the engine keeps its own (see Sync)
			e.listed[p] = r.Hash
		}
	}
	return out, nil
}

// Versions are those of the file the path names, asked of the remote by the name it has there; what comes back is opened here.
func (e *EncRemote) Versions(path string) ([]RemoteVersion, error) {
	v, ok := e.inner.(versioned)
	if !ok {
		return nil, errNoVersions
	}
	return v.Versions(e.opaque(path))
}

func (e *EncRemote) VersionData(path string, at int64) ([]byte, error) {
	v, ok := e.inner.(versioned)
	if !ok {
		return nil, errNoVersions
	}
	b, err := v.VersionData(e.opaque(path), at)
	if err != nil {
		return nil, err
	}
	out, _, err := e.open(b, fileAAD(path)) // a version of the past is older by what it is: nothing vouches for it
	return out, err
}

func (e *EncRemote) Get(name string) ([]byte, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	b, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return nil, err
	}
	out, unlabelled, err := e.open(b, fileAAD(name))
	if errors.Is(err, errSealed) && e.indexed && !e.trustFiles {
		// the key opened the index: a file it does not open under its own name is sealed for another (#148)
		return nil, fmt.Errorf("%w: sealed for another name", ErrFileRollback)
	}
	if err != nil {
		return nil, err
	}
	if err := e.vouch(name, short(HashOf(out)), HashOf(b), unlabelled); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *EncRemote) Put(name string, data []byte, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	if err := e.Announce([]string{name}); err != nil { // its name first, then the object (Announce)
		return err
	}
	sealed := e.seal(data, fileAAD(name))
	if err := e.inner.Put(e.opaque(name), sealed, prev); err != nil {
		return err
	}
	e.know(name, fileHashes{plain: short(HashOf(data)), sealed: short(HashOf(sealed))})
	if e.sent == nil {
		e.sent = map[string]string{}
	}
	e.sent[name] = HashOf(sealed)
	return nil
}

// Announce puts the names in the sealed index, once for all of them, before any of their files is put: an object
// on the remote always has its name in the index, so that another device making a file of the same path meets
// it, and keeps a conflict copy, instead of writing over it as a pass's own leftover (#149). A name in the index
// with no object is harmless: List leaves it out, as if it were not there. A pass announces the files it has at its
// start (syncOnce), so that a first pass of thousands puts the index once, not once a file.
func (e *EncRemote) Announce(names []string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	var added []string
	for _, n := range names {
		if op := e.opaque(n); e.idx[op] != n { // names are never taken out of the index
			e.idx[op] = n
			added = append(added, op)
		}
	}
	if len(added) == 0 {
		return nil
	}
	if err := e.saveIndex(); err != nil {
		// not on the remote: taken back here, or the next Announce would take them for put and skip them
		for _, op := range added {
			delete(e.idx, op)
		}
		return err
	}
	return nil
}

func (e *EncRemote) Delete(name, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	if err := e.inner.Delete(e.opaque(name), prev); err != nil {
		return err
	}
	e.know(name, fileHashes{sealed: short(prev), deleted: true}) // its last sealed bytes, as the pass listed them
	if e.deletedHere == nil {
		e.deletedHere = map[string]bool{}
	}
	e.deletedHere[name] = true
	return nil
}

// ---- the engine ----

// SyncState is what was true at the last sync: for each file, its hash here and the remote's hash then.
type SyncState struct {
	Files map[string]fileSync `json:"files"`
	// AllowRemovals lets this one pass delete here what the remote does not have, past massRemovals (the
	// user said so); it is never kept.
	AllowRemovals bool `json:"-"`
	// Resend sends up again, as new, the files this one pass would hold back (massRemovals): what the guard is
	// for is a remote that lost them, and the copies here are the ones to keep (#147). It is never kept.
	Resend bool `json:"-"`
	// BeforeChange, if set, is called once in a pass, before the first file here is written, removed or moved: a
	// backup of the library is taken then, not before every pass, whether or not it changes anything (#150). It is
	// never kept.
	BeforeChange func() `json:"-"`
	// Held is how many files the last pass held back: they stay held, whatever the next pass counts, until the
	// user says to delete them or to send them up again.
	Held int `json:"held,omitempty"`
	// Seen is each file as it was seen when its hash here was what the last sync had (localFiles).
	Seen map[string]fileSeen `json:"seen,omitempty"`
	// Pristine is the files a new library made for itself (its first collection and page, its journal), by
	// their hash then, until they are first synced. One the remote has too, never synced here and still as it
	// was made, is the remote's, with no conflict: the library is a new device's, and the account's copy is
	// the real one.
	Pristine map[string]string `json:"pristine,omitempty"`
	// Bases keep each page file as it was at the last sync, to merge a page changed on both sides (MergePage); with
	// none, it is kept twice.
	Bases SyncBases `json:"-"`
	// Remote names the remote the state is of (ForRemote): the files it knows were synced with that one.
	Remote string `json:"remote,omitempty"`
	// Counter is the server's change counter as this device's last pass left it, when nothing but that pass's own
	// writes moved it while it went (ExactCounter); 0 when that is not known. Unchanged says the app found the counter
	// at it before this pass: the remote is as the state says, and the pass takes that for its listing, which held
	// every file's name and hash, at every pass (#151). Unchanged is never kept.
	Counter   int64 `json:"counter,omitempty"`
	Unchanged bool  `json:"-"`
	// Device names this device in the generation of a sealed index (EncRemote): made at its first pass, and kept.
	Device string `json:"device,omitempty"`
	// IndexSeen is the generation of the remote's sealed index this device has seen, and IndexNames the short names
	// it had, one after another: an index below it that names nothing new is an older one served again (#148).
	IndexSeen  map[string]int64 `json:"indexSeen,omitempty"`
	IndexNames string           `json:"indexNames,omitempty"`
	// HashesSeen is the generation of the remote's "_hashes" this device has seen: older ones, or none, are not
	// taken at their word. IndexMissingSince and HashesMissingSince are when a pass first found the index, or the
	// hashes, missing where they were, in Unix seconds (0 for not): a folder copied by another program may bring them
	// late, so they are waited for, missingWait at most.
	HashesSeen        map[string]int64 `json:"hashesSeen,omitempty"`
	IndexMissingSince int64            `json:"indexMissingSince,omitempty"`
	// Unvouched is when each labelled file the remote's hashes did not vouch for was first found so (EncRemote.vouch).
	Unvouched          map[string]int64 `json:"unvouched,omitempty"`
	HashesMissingSince int64            `json:"hashesMissingSince,omitempty"`
	// Earlier is, by path, the short hashes of the sealed copies of each file this device synced, the last one
	// last, and deletedMark after them for a file deleted (EncRemote.vouch): the remote serving one of them, but
	// the last, under its name, or one of them under another's, serves bytes that are not that file's now (#148).
	Earlier map[string][]string `json:"earlier,omitempty"`
	// TrustIndex takes, for this one pass, the remote's index as it is, though older than one this device has seen
	// or missing (the user said so): it is put again above what was seen. TrustFiles takes the files the checks
	// would refuse. Each is left set by a pass that could not use it up, and is never kept.
	TrustIndex bool `json:"-"`
	TrustFiles bool `json:"-"`
}

// ForRemote makes st the state of the remote named id, and says whether it forgot what it knew: a state of another
// remote reads every file as synced, and against a new one each would be "unchanged here, gone there", deleted here
// (#146). It starts again as a first pass, where a file on both sides with the same bytes is only recorded. A state
// kept before remotes were named takes the name it is given.
func (st *SyncState) ForRemote(id string) bool {
	if st.Remote == id {
		return false
	}
	old := st.Remote
	st.Remote = id
	if old == "" {
		return false
	}
	st.forget()
	st.IndexSeen, st.IndexNames, st.Earlier = nil, "", nil // another remote's index counts its own generations
	st.HashesSeen, st.IndexMissingSince, st.HashesMissingSince = nil, 0, 0
	st.Counter, st.Unvouched = 0, nil
	return true
}

// asListed is the listing of the remote as this device's last pass left it: each file it synced, with the hash it
// has there.
func (st *SyncState) asListed() map[string]RemoteEntry {
	out := map[string]RemoteEntry{}
	for n, f := range st.Files {
		if f.Remote != "" {
			out[n] = RemoteEntry{Hash: f.Remote}
		}
	}
	return out
}

// MayTakeAsListed says whether a pass may take the state for the remote's listing, the remote being as it was
// (Unchanged): only in a plain pass, nothing to trust again, to remove or to send again, nothing held or waited for,
// and something synced already.
func (st *SyncState) MayTakeAsListed() bool {
	return st.Counter != 0 && !st.TrustIndex && !st.TrustFiles && !st.AllowRemovals && !st.Resend && st.Held == 0 &&
		st.IndexMissingSince == 0 && st.HashesMissingSince == 0 && len(st.Unvouched) == 0 && len(st.asListed()) > 0
}

// forget drops what the state knows of each file and its base; what this library made for itself stays (Pristine).
func (st *SyncState) forget() {
	for n := range st.Files {
		st.dropBase(n)
	}
	st.Files = map[string]fileSync{}
}

// massRemovals is how many files, and a tenth of the library, a pass may delete here for being gone from
// the remote with no tombstone, before it holds them back and says so: a remote that lost its index, or
// a folder not mounted, looks like that, and a deletion is not what it means.
const massRemovals = 20

type fileSync struct {
	Local  string `json:"l"` // hash of the file here at the last sync ("" if it was not there)
	Remote string `json:"r"` // the remote's hash of it then
}

// SyncResult says what a sync did.
type SyncResult struct {
	Pushed, Pulled, Deleted, Removed int      // files sent up, brought down, deleted there, deleted here
	Merged                           int      // pages changed on both sides and merged: written here, and sent up
	Held                             int      // files not deleted here, though the remote has none: too many at once (massRemovals)
	Waiting                          bool     // files not fetched, the remote's hashes waited for (ErrRemoteNotYet)
	Refused                          []string // files not taken from the remote, its copy not being theirs (ErrFileRollback)
	Conflicts                        []string // the files that changed on both sides: ours is kept beside
	Errors                           []string
	// Remote is how many names the remote had as the pass began, deleted ones too: none is an account that was
	// never used (the app gives a new one its welcome page then, and not before).
	Remote int
	// Listed says the remote was listed; not when the state was taken for its listing (SyncCounted). Changes is the
	// server's change counter after the pass, when SyncCounted read it (ChangesKnown).
	Listed       bool
	Changes      int64
	ChangesKnown bool
}

// SyncCounted is SyncOnce with a server that counts its writes exactly (ExactCounter): its counter is read before
// the pass, and when it is what the state's last pass left (SyncState.Counter), nothing but this device wrote since,
// and the pass takes what the state knows for the remote's listing (#151). After a pass with no error, nothing refused
// and nothing waited for, the counter is kept when it moved by this pass's own writes alone, and forgotten
// otherwise: a write of another device while the pass went makes the next one list. r is to be new for the pass:
// its count of its own writes is this pass's. Another remote is synced as SyncOnce does it.
func SyncCounted(fsys FS, root string, r Remote, st *SyncState, skip map[string]bool, now time.Time) (SyncResult, error) {
	ec, ok := r.(ExactCounter)
	if !ok {
		return SyncOnce(fsys, root, r, st, skip, now)
	}
	before, exact, cerr := ec.CountedChanges()
	counted := cerr == nil && exact
	st.Unchanged = counted && st.Counter != 0 && before == st.Counter
	res, err := SyncOnce(fsys, root, r, st, skip, now)
	st.Unchanged = false
	st.Counter = 0
	after, _, aerr := ec.CountedChanges()
	if aerr == nil {
		res.Changes, res.ChangesKnown = after, true
		if counted && err == nil && len(res.Errors) == 0 && !res.Waiting && len(res.Refused) == 0 && after == before+ec.Written() {
			st.Counter = after
		}
	}
	return res, err
}

func (r SyncResult) String() string {
	if r.Pushed+r.Pulled+r.Deleted+r.Removed+r.Held+r.Merged+len(r.Conflicts) == 0 && len(r.Errors) == 0 {
		return "Eşit: değişiklik yok"
	}
	s := fmt.Sprintf("%d gönderildi, %d alındı", r.Pushed, r.Pulled)
	if r.Merged > 0 {
		s += fmt.Sprintf(", %d merged", r.Merged)
	}
	if r.Deleted+r.Removed > 0 {
		s += fmt.Sprintf(", %d silme", r.Deleted+r.Removed)
	}
	if len(r.Conflicts) > 0 {
		s += fmt.Sprintf(", %d çakışma (ikisi de saklandı)", len(r.Conflicts))
	}
	if len(r.Errors) > 0 {
		s += fmt.Sprintf(", %d hata", len(r.Errors))
	}
	if r.Held > 0 {
		s += fmt.Sprintf(", %d silme bekliyor", r.Held)
	}
	return s
}

// fileSeen is a file as it was seen at the last sync: its size and when it was written. What is seen the same
// is the same bytes, and its hash is not made again.
type fileSeen struct {
	Size int64 `json:"s"`
	Mod  int64 `json:"m"`
}

// localFiles lists the files of the tree under root, by path relative to it, with their hashes; the top
// folders in skip are left out. A file seen as st saw it at the last sync has the hash it had then, and is not
// read: a library of thousands was read whole, and hashed, every pass. seen is what is seen now. held is what was
// gone by the time it was read (see heldBy).
func localFiles(fsys FS, root string, skip map[string]bool, st *SyncState) (out map[string]string, seen map[string]fileSeen, held map[string]bool, err error) {
	out, seen, held = map[string]string{}, map[string]fileSeen{}, map[string]bool{}
	// hold marks a file, or a folder (its path and a "/"), that is gone by the time it is read: one renamed or
	// removed while the tree is walked, or caught as a tool writes it. The pass leaves it alone, sending,
	// deleting and writing nothing at its path, and its state stays as it was, so the next pass reads it again
	// and decides (#542). A file the last sync knew keeps its hash here, so the remote is still told its name.
	hold := func(r string) {
		held[r] = true
		if f, ok := st.Files[r]; ok && f.Local != "" && !strings.HasSuffix(r, "/") {
			out[r] = f.Local
		}
	}
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		ents, err := fsys.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) && rel != "" { // a folder removed while the tree is walked
			hold(rel + "/")
			for n := range st.Files {
				if strings.HasPrefix(n, rel+"/") {
					hold(n)
				}
			}
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range ents {
			name := e.Name()
			r := path.Join(rel, name)
			if rel == "" && (skip[name] || name == FormatFile) { // the format is each device's app's to write
				continue
			}
			if strings.EqualFold(name, ".git") || strings.HasSuffix(name, ".tmp~") || IsTempFile(name) || name == SettingsKey {
				continue
			}
			if localTrash(r) { // the trash's folder that is this device's alone
				continue
			}
			if e.IsDir() {
				if err := walk(filepath.Join(dir, name), r); err != nil {
					return err
				}
				continue
			}
			if fi, err := e.Info(); err == nil {
				seen[r] = fileSeen{fi.Size(), fi.ModTime().UnixNano()}
				if f, ok := st.Files[r]; ok && f.Local != "" && st.Seen[r] == seen[r] {
					out[r] = f.Local
					continue
				}
			}
			b, err := fsys.ReadFile(filepath.Join(dir, name))
			if errors.Is(err, fs.ErrNotExist) { // renamed or removed since its folder was read
				delete(seen, r)
				hold(r)
				continue
			}
			if err != nil {
				return err
			}
			out[r] = HashOf(b)
		}
		return nil
	}
	return out, seen, held, walk(root, "")
}

// heldBy reports whether name is one localFiles held, or lies in a folder it held.
func heldBy(held map[string]bool, name string) bool {
	if held[name] {
		return true
	}
	for i := strings.LastIndexByte(name, '/'); i > 0; i = strings.LastIndexByte(name[:i], '/') {
		if held[name[:i+1]] {
			return true
		}
	}
	return false
}

// PristineOf is the files of the library under root by their hash, as SyncState.Pristine has them: what a new
// library made for itself, before anything was written in it.
func PristineOf(fsys FS, root string) (map[string]string, error) {
	local, _, _, err := localFiles(fsys, root, nil, &SyncState{})
	return local, err
}

func conflictName(name string, at time.Time) string {
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext) + " (çakışma " + at.Format("2006-01-02 15-04-05") + ")" + ext
}

// SyncOnce makes one pass: it puts the library under root (in fsys) and the remote in agreement.
func SyncOnce(fsys FS, root string, r Remote, st *SyncState, skip map[string]bool, now time.Time) (SyncResult, error) {
	// a remote whose index is numbered and vouches for its files (EncRemote) takes none older than the last pass
	// saw, and puts what this pass learnt in its index at the end
	sp, sealed := r.(sealedPass)
	if sealed {
		sp.beginPass(st, now)
	}
	if st.BeforeChange != nil {
		fsys = &firstChange{FS: fsys, before: st.BeforeChange}
	}
	res, pushed, err := syncOnce(fsys, root, r, st, skip, now)
	// the remote's hash of what was sent: known from what was put where the remote can tell it (EncRemote), and
	// for the rest read once, after their names are there
	var unknown []string
	for _, n := range pushed {
		if ph, ok := r.(interface{ PutHash(string) (string, bool) }); ok {
			if h, ok := ph.PutHash(n); ok {
				f := st.Files[n]
				f.Remote = h
				st.Files[n] = f
				continue
			}
		}
		unknown = append(unknown, n)
	}
	if len(unknown) > 0 {
		if after, lerr := r.List(); lerr == nil {
			for _, n := range unknown {
				f := st.Files[n]
				f.Remote = after[n].Hash
				st.Files[n] = f
			}
		}
	}
	if sealed {
		if ierr := sp.endPass(st); ierr != nil {
			res.Errors = append(res.Errors, ierr.Error()) // named: the index's, or the hashes'
		}
	}
	return res, err
}

// sealedPass is a remote whose index is numbered and vouches for its files (EncRemote): the sync state keeps what
// each pass saw of them.
type sealedPass interface {
	beginPass(st *SyncState, now time.Time)
	endPass(st *SyncState) error
}

func syncOnce(fsys FS, root string, r Remote, st *SyncState, skip map[string]bool, now time.Time) (res SyncResult, pushed []string, err error) {
	if st.Files == nil {
		st.Files = map[string]fileSync{}
	}
	local, seen, held, err := localFiles(fsys, root, skip, st)
	if err != nil {
		return res, pushed, err
	}
	// a remote that keeps the names apart from the files (EncRemote) is given every name this pass could put
	// before any file goes up, and is listed after: a file of the same path that another device put first is
	// then seen, and met as a conflict (#149). An index that cannot be put (a remote at its quota, the network) does
	// not stop the pass: what comes down and what is deleted still are, and each new file's put, which announces
	// its own name first, fails by itself.
	if an, ok := r.(interface{ Announce([]string) error }); ok {
		names := make([]string, 0, len(local))
		for n := range local {
			names = append(names, n)
		}
		sort.Strings(names)
		if err := an.Announce(names); err != nil {
			res.Errors = append(res.Errors, IndexFile+": "+err.Error())
		}
	}
	var remote map[string]RemoteEntry
	if st.Unchanged && st.MayTakeAsListed() { // nothing but this device wrote since its last pass: no listing (#151)
		remote = st.asListed()
		if l, ok := r.(interface{ listedFrom(map[string]RemoteEntry) }); ok {
			l.listedFrom(remote)
		}
	} else if remote, err = r.List(); err != nil {
		return res, pushed, err
	} else {
		res.Listed = true
	}
	res.Remote = len(remote)
	if len(remote) == 0 { // a remote with nothing, not even a tombstone, never had what the state says was synced
		// with it: a bucket made again, an account opened again. Starting over sends everything up.
		for _, f := range st.Files {
			if f.Remote != "" {
				// the copies held stay: what goes up again is sealed with nonces of its own, never one of them, and a
				// remote that only claimed to be empty may serve them again
				st.forget()
				st.IndexSeen, st.IndexNames, st.HashesSeen = nil, "", nil
				break
			}
		}
	}
	for n := range remote { // a name that leaves the folder is a remote's to write over, or to delete, what is outside the library
		if !SafeName(n) || hasGitSegment(n) {
			delete(remote, n)
			res.Errors = append(res.Errors, n+": "+errUnsafeName.Error())
		}
	}
	for n := range st.Files {
		if !SafeName(n) || hasGitSegment(n) {
			delete(st.Files, n)
		}
	}
	names := map[string]bool{}
	for n := range local {
		names[n] = true
	}
	for n := range remote {
		if !skip[strings.SplitN(n, "/", 2)[0]] && !strings.EqualFold(n, FormatFile) && !localTrash(n) { // the trash's folder that is each device's alone is not the remote's to write or delete
			names[n] = true
		}
	}
	for n := range st.Files { // a collection chosen out keeps its entries for when it is chosen back in, but is not worked
		// on: its files are missing here only because the walk left them out, and would read as deleted here
		if !skip[strings.SplitN(n, "/", 2)[0]] {
			names[n] = true
		}
	}
	for n := range names { // what was gone as it was read is the next pass's (#542)
		if heldBy(held, n) {
			delete(names, n)
		}
	}
	order := make([]string, 0, len(names))
	for n := range names {
		order = append(order, n)
	}
	sort.Strings(order)
	abs := func(n string) string { return filepath.Join(root, filepath.FromSlash(n)) }
	// a server that stops answering mid-pass would cost transferIdle for each file left: after transportStop
	// requests that did not get through, the pass stops, and the next takes up what is left (#155)
	var stopped error
	transport := 0
	fail := func(n string, err error) {
		if errors.Is(err, ErrRemoteNotYet) { // waited for: not an error, and the next pass takes it
			res.Waiting = true
			return
		}
		if isTransport(err) {
			if transport++; transport >= transportStop {
				stopped = err
			}
		}
		res.Errors = append(res.Errors, n+": "+err.Error())
		if errors.Is(err, ErrFileRollback) {
			res.Refused = append(res.Refused, n)
		}
	}
	var gone []string // what the remote has no more: the files to take away from here
	// files the remote has no entry for at all (no tombstone), that are here as at the last pass: they
	// would be deleted here. A few is a deletion elsewhere whose tombstone is gone; a tenth of the library
	// is a remote that lost its list.
	// An entry listed without a hash, not deleted, is not one the pass can read: an object put in the bucket by
	// another tool. Read as gone it would be deleted here, past the guard (#147): it is an error for that name.
	for n, re := range remote {
		if re.Held { // gone as the remote was listed: the next pass reads it again (#566)
			delete(remote, n)
			names[n] = false
			continue
		}
		if !re.Deleted && re.Hash == "" {
			delete(remote, n)
			names[n] = false
			res.Errors = append(res.Errors, n+": listed with no hash, left as it is")
		}
	}
	hold, holdTombs := false, false
	if !st.AllowRemovals {
		gone, tombed, synced := 0, 0, 0
		var goneNames []string
		for _, n := range order {
			if !names[n] {
				continue
			}
			if st.Files[n].Remote != "" {
				synced++
			}
			re, rok := remote[n]
			if rok && !re.Deleted {
				continue
			}
			if lh := local[n]; lh != "" && lh == st.Files[n].Local && st.Files[n].Remote != "" {
				if rok {
					tombed++
				} else {
					gone++
				}
				goneNames = append(goneNames, n)
			}
		}
		// many gone at once, or every file synced before, however few: a remote that lost its list
		hold = gone >= massRemovals && gone*10 >= len(local) || gone > 0 && gone+tombed == synced && synced > 1 || gone > 0 && st.Held > 0
		// every file synced before deleted at once, by tombstones: a remote that says "deleted" of everything is
		// not taken at its word (#147); a library really emptied elsewhere is one button away
		holdTombs = tombed > 0 && gone+tombed == synced && synced > 1 || tombed > 0 && st.Held > 0
		if (hold || holdTombs) && st.Resend { // the copies here are the ones to keep: they go up as new
			for _, n := range goneNames {
				delete(st.Files, n)
				st.dropBase(n)
			}
			hold, holdTombs = false, false
		}
	}
	for _, n := range order {
		if stopped != nil {
			break
		}
		if !names[n] { // listed with no hash: left as it is
			continue
		}
		lh := local[n] // "" when it is not here
		re, rok := remote[n]
		rh := ""
		if rok && !re.Deleted {
			rh = re.Hash
		}
		base := st.Files[n]
		localChanged := lh != base.Local
		remoteChanged := rh != base.Remote
		switch {
		case !localChanged && !remoteChanged:
			continue
		case lh == "" && rh == "": // gone from both: forget it
			delete(st.Files, n)
			st.dropBase(n)
			continue
		case localChanged && !remoteChanged: // ours: up
			if lh == "" {
				if err := r.Delete(n, rh); err != nil {
					fail(n, err)
					continue
				}
				res.Deleted++
				delete(st.Files, n)
				st.dropBase(n)
				continue
			}
			b, err := fsys.ReadFile(abs(n))
			if err != nil {
				fail(n, err)
				continue
			}
			if err := r.Put(n, b, rh); err != nil {
				fail(n, err)
				continue
			}
			res.Pushed++
			st.Files[n] = fileSync{Local: lh}
			st.setBase(n, b)
			pushed = append(pushed, n)
		case !localChanged && remoteChanged: // theirs: down
			if rh == "" {
				if _, there := remote[n]; hold && !there || holdTombs && there {
					res.Held++
					continue
				}
				gone = append(gone, n) // taken away after the pass, when what came down is here: to the trash
				continue
			}
			b, err := r.Get(n)
			if err != nil {
				fail(n, err)
				continue
			}
			if !localIs(fsys, abs(n), lh) { // written here since the pass looked: what is typed is not written over, the next pass sees both changed
				continue
			}
			if err := fsys.MkdirAll(filepath.Dir(abs(n))); err != nil {
				fail(n, err)
				continue
			}
			if err := fsys.WriteFile(abs(n), b); err != nil {
				fail(n, err)
				continue
			}
			res.Pulled++
			st.Files[n] = fileSync{Local: HashOf(b), Remote: rh}
			st.setBase(n, b)
		default: // both changed
			if base == (fileSync{}) && lh != "" && rh != "" && st.Pristine[n] == lh { // a new library's own, untouched: theirs
				b, err := r.Get(n)
				if err != nil {
					fail(n, err)
					continue
				}
				if err := fsys.WriteFile(abs(n), b); err != nil {
					fail(n, err)
					continue
				}
				res.Pulled++
				st.Files[n] = fileSync{Local: HashOf(b), Remote: rh}
				st.setBase(n, b)
				continue
			}
			if lh != "" && rh != "" && lh == localHashOf(r, n, rh, lh) {
				st.Files[n] = fileSync{Local: lh, Remote: rh} // the same bytes on both sides
				continue
			}
			if done, err := mergeBoth(fsys, abs(n), r, st, n, lh, rh); errors.Is(err, errWrittenSince) {
				continue
			} else if err != nil {
				fail(n, err)
				continue
			} else if done {
				res.Merged++
				pushed = append(pushed, n)
				continue
			}
			if rh == "" { // they deleted, we changed: ours stays, and goes up again (no conflict copy of our own file)
				st.Files[n] = fileSync{Local: "", Remote: ""}
				continue
			}
			// theirs stays, ours is kept beside, and goes up as a file of its own on the next pass. Theirs is fetched
			// first: one that is refused (ErrFileRollback) would otherwise leave a new copy of ours at every pass
			b, err := r.Get(n)
			if err != nil {
				fail(n, err)
				continue
			}
			if lh != "" {
				mine, err := fsys.ReadFile(abs(n))
				if err != nil {
					fail(n, err)
					continue
				}
				if err := fsys.WriteFile(abs(conflictName(n, now)), mine); err != nil {
					fail(n, err)
					continue
				}
				res.Conflicts = append(res.Conflicts, n)
			}
			if err := fsys.WriteFile(abs(n), b); err != nil {
				fail(n, err)
				continue
			}
			res.Pulled++
			st.Files[n] = fileSync{Local: HashOf(b), Remote: rh}
			st.setBase(n, b)
		}
	}
	if stopped != nil { // what is done is kept in the state; the rest is the next pass's
		return res, pushed, fmt.Errorf("the server stopped answering: %w", stopped)
	}
	if len(gone) > 0 {
		// what is in the trash, by its bytes: a page thrown away on the other device came down there
		kept := map[string]bool{}
		for n, h := range local {
			if strings.HasPrefix(n, TrashDir+"/") {
				kept[h] = true
			}
		}
		for n, f := range st.Files {
			if strings.HasPrefix(n, TrashDir+"/") && f.Local != "" {
				kept[f.Local] = true
			}
		}
		removed, failed := syncTrash(fsys, root, gone, local, kept, now, false)
		for _, n := range removed {
			res.Removed++
			delete(st.Files, n)
			st.dropBase(n)
		}
		for n, err := range failed {
			fail(n, err)
		}
	}
	for n := range st.Pristine { // synced once, a file is the library's like any other
		if _, ok := st.Files[n]; ok {
			delete(st.Pristine, n)
		}
	}
	st.Seen = map[string]fileSeen{} // a file is kept as seen only when what the sync knows of it is what is there
	for n, s := range seen {
		if f, ok := st.Files[n]; ok && f.Local != "" && f.Local == local[n] {
			st.Seen[n] = s
		}
	}
	st.Held = res.Held // what was held stays held at the next pass, until the user says what it is
	return res, pushed, nil
}

// errWrittenSince says the file was written here since the pass looked at it: the next pass takes it.
var errWrittenSince = errors.New("written since the pass began")

// mergeBoth merges a page file changed on both sides, from its base: the merged page is written here and put on
// the remote. done is false when it is not to be merged (no base, not a page, the same blocks changed): the
// conflict copy is made then. The file written here since the pass looked is left for the next pass.
func mergeBoth(fsys FS, p string, r Remote, st *SyncState, n, lh, rh string) (done bool, err error) {
	if st.Bases == nil || lh == "" || rh == "" || st.Files[n].Local == "" || !keepsBase(n) {
		return false, nil
	}
	base := st.Bases.Base(n)
	if base == nil || HashOf(base) != st.Files[n].Local {
		return false, nil
	}
	mine, err := fsys.ReadFile(p)
	if err != nil || HashOf(mine) != lh {
		return false, err
	}
	theirs, err := r.Get(n)
	if err != nil {
		return false, err
	}
	merged, ok := MergePage(base, mine, theirs)
	if !ok {
		return false, nil
	}
	if !localIs(fsys, p, lh) {
		return false, errWrittenSince
	}
	if err := fsys.WriteFile(p, merged); err != nil {
		return false, err
	}
	// until the merge is on the remote, this is as if theirs had come down and the merge been typed here: the next
	// pass sends it up
	st.Files[n] = fileSync{Local: HashOf(theirs), Remote: rh}
	st.setBase(n, theirs)
	if err := r.Put(n, merged, rh); err != nil {
		return false, err
	}
	st.Files[n] = fileSync{Local: HashOf(merged)}
	st.setBase(n, merged)
	return true, nil
}

// localHashOf is the hash of the remote's bytes, for comparing with ours when the remote's own hash is
// of something else (a sealed file's): the bytes are fetched and hashed.
func localHashOf(r Remote, name, remoteHash, _ string) string {
	if _, ok := r.(*EncRemote); !ok {
		return remoteHash
	}
	b, err := r.Get(name)
	if err != nil {
		return ""
	}
	return HashOf(b)
}

// localIs says whether the file at p is still what a pass saw of it: the bytes of hash lh, or no file when lh
// is "". A pass runs while the window goes on, and a page saved after the pass looked is not written over.
func localIs(fsys FS, p, lh string) bool {
	b, err := fsys.ReadFile(p)
	if lh == "" {
		return errors.Is(err, fs.ErrNotExist)
	}
	return err == nil && HashOf(b) == lh
}

// AllowHTTP lets a manifest or a file come over plain HTTP: only a test does.
var AllowHTTP = false

func HTTPSOnly(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(AllowHTTP && u.Scheme == "http") {
		return errors.New("yalnız HTTPS adresinden güncellenir: " + raw)
	}
	return nil
}

func hasGitSegment(name string) bool {
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.EqualFold(seg, ".git") {
			return true
		}
	}
	return false
}

// SettingsKey is what the settings are kept under in a library's file.
const SettingsKey = "settings"

// transportStop is how many requests of a pass may fail to get through before the pass stops (#155).
const transportStop = 3

// isTransport says whether err is a request that did not get through: no connection, no answer in time, a line
// that went quiet. A refusal of the server, or of this device's checks, is not one.
func isTransport(err error) bool {
	var ue *url.Error
	return errors.As(err, &ue)
}

// firstChange is fsys, calling before once, ahead of the first change it makes: a write, a removal, a move, a folder
// made (SyncState.BeforeChange).
type firstChange struct {
	FS
	before func()
	once   sync.Once
}

func (f *firstChange) changing() { f.once.Do(f.before) }

func (f *firstChange) WriteFile(name string, data []byte) error {
	f.changing()
	return f.FS.WriteFile(name, data)
}
func (f *firstChange) Mkdir(name string) error    { f.changing(); return f.FS.Mkdir(name) }
func (f *firstChange) MkdirAll(name string) error { f.changing(); return f.FS.MkdirAll(name) }
func (f *firstChange) Rename(a, b string) error   { f.changing(); return f.FS.Rename(a, b) }
func (f *firstChange) Remove(name string) error   { f.changing(); return f.FS.Remove(name) }
func (f *firstChange) RemoveAll(name string) error {
	f.changing()
	return f.FS.RemoveAll(name)
}
