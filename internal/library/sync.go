package library

import (
	"bytes"
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
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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

func (d DirRemote) List() (map[string]RemoteEntry, error) {
	out := map[string]RemoteEntry{}
	err := filepath.WalkDir(d.Root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(d.Root, p)
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, tombSuffix) {
			out[strings.TrimSuffix(rel, tombSuffix)] = RemoteEntry{Deleted: true}
			return nil
		}
		if _, dead := out[rel]; !dead {
			out[rel] = RemoteEntry{Hash: HashOf(b)}
		}
		return nil
	})
	// a tombstone beside a file that is there: the later is the tombstone only if the file is gone
	for name, e := range out {
		if e.Deleted {
			if _, err := os.Stat(d.file(name)); err == nil {
				b, _ := os.ReadFile(d.file(name))
				out[name] = RemoteEntry{Hash: HashOf(b)}
			}
		}
	}
	if os.IsNotExist(err) {
		return out, nil
	}
	return out, err
}

const tombSuffix = ".deleted~"

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
//
// with "Authorization: Bearer <the device's token>".
type httpRemote struct {
	base, token string
	client      *http.Client
}

func NewHTTPRemote(base, token string) *httpRemote {
	return &httpRemote{strings.TrimRight(base, "/"), token, &http.Client{Timeout: 60 * time.Second}}
}

func (h *httpRemote) do(method, name string, body []byte, prev *string) (*http.Response, error) {
	if err := HTTPSOnly(h.base); err != nil {
		return nil, err
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
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusPreconditionFailed {
		resp.Body.Close()
		return nil, ErrConflict
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, fmt.Errorf("sunucu %d döndü", resp.StatusCode)
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

func (h *httpRemote) getVersion(u string) ([]byte, error) {
	if err := HTTPSOnly(h.base); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("sunucu %d döndü", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (h *httpRemote) Versions(name string) ([]RemoteVersion, error) {
	b, err := h.getVersion(h.versionURL(name, 0))
	if err != nil {
		return nil, err
	}
	var out []RemoteVersion
	return out, json.Unmarshal(b, &out)
}

func (h *httpRemote) VersionData(name string, at int64) ([]byte, error) {
	return h.getVersion(h.versionURL(name, at))
}

// ---- end-to-end encryption ----

// EncRemote wraps a remote: each file is sealed with AES-256-GCM under a key from the passphrase, and
// is stored under a name that is an HMAC of its path: the other side sees the count and the sizes of
// the files, and nothing else. A small sealed index, "_index", maps the names back to paths. Lose the
// passphrase and the recovery key, and the data is gone: that is what it is for.
type EncRemote struct {
	inner  Remote
	Key    []byte
	gcm    cipher.AEAD
	idx    map[string]string // opaque name → path
	loaded bool
	// pending is how many names were added since the index was last put: it is put every indexEvery, and
	// at the end of a pass (Flush), not at every file, which made a first pass of thousands go as their square
	pending int
}

// indexEvery is how many new names wait before the index is put: what another device sees of a long first
// pass comes in steps of it.
const indexEvery = 200

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
	return &EncRemote{inner: inner, Key: key, gcm: g, idx: map[string]string{}}, nil
}

// OpenEnc opens the remote with a passphrase: the salt is read from it, or made and put there.
func OpenEnc(inner Remote, pass string) (*EncRemote, error) {
	salt, err := inner.Get(SaltFile)
	if err != nil {
		list, lerr := inner.List()
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
	}
	key, err := DeriveKey(pass, salt)
	if err != nil {
		return nil, err
	}
	return NewEncRemote(inner, key)
}

func (e *EncRemote) seal(b []byte) []byte {
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, nil)
}

func (e *EncRemote) open(b []byte) ([]byte, error) {
	n := e.gcm.NonceSize()
	if len(b) < n {
		return nil, errors.New("şifreli dosya kısa")
	}
	out, err := e.gcm.Open(nil, b[:n], b[n:], nil)
	if err != nil {
		return nil, errors.New("şifre çözülemedi: parola ya da anahtar yanlış")
	}
	return out, nil
}

func (e *EncRemote) opaque(p string) string {
	m := hmac.New(sha256.New, e.Key)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

const IndexFile = "_index"

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
		e.loaded = true
		return nil // no index yet: nothing was put
	}
	plain, err := e.open(b)
	if err != nil {
		return err
	}
	e.loaded = true
	return json.Unmarshal(plain, &e.idx)
}

// saveIndex puts the index back, merged with what the remote holds now: another device may have added
// names since this one read it, and an index written from the old copy would drop them (their files would
// then not be seen, and their owner would take them for deleted). Names are only ever added, so a union
// loses nothing; if the index changes again while it is being put, it is read and merged once more.
func (e *EncRemote) saveIndex() error {
	for try := 0; try < 8; try++ {
		prev := ""
		list, err := e.inner.List()
		if err != nil {
			return err
		}
		if ent, ok := list[IndexFile]; ok && !ent.Deleted {
			raw, err := e.inner.Get(IndexFile)
			if err != nil {
				return err
			}
			plain, err := e.open(raw)
			if err != nil {
				return err
			}
			theirs := map[string]string{}
			if err := json.Unmarshal(plain, &theirs); err != nil {
				return err
			}
			for op, p := range theirs {
				e.idx[op] = p
			}
			prev = HashOf(raw) // the hash of the bytes: that of a folder's file and of the Worker's
		}
		plain, _ := json.Marshal(e.idx)
		switch err := e.inner.Put(IndexFile, e.seal(plain), prev); {
		case err == nil:
			return nil
		case errors.Is(err, ErrConflict):
			continue
		default:
			return err
		}
	}
	return ErrConflict
}

func (e *EncRemote) List() (map[string]RemoteEntry, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	raw, err := e.inner.List()
	if err != nil {
		return nil, err
	}
	out := map[string]RemoteEntry{}
	for op, p := range e.idx {
		if r, ok := raw[op]; ok {
			out[p] = r // the hash is of the sealed bytes: it changes at each seal, so the engine keeps its own (see Sync)
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
	return e.open(b)
}

func (e *EncRemote) Get(name string) ([]byte, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	b, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return nil, err
	}
	return e.open(b)
}

func (e *EncRemote) Put(name string, data []byte, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	err := e.inner.Put(e.opaque(name), e.seal(data), prev)
	if errors.Is(err, ErrConflict) && prev == "" && e.idx[e.opaque(name)] != name {
		// an object a pass left there, its name never put in the index: it is ours, and what is in it may
		// be older than what is sent now. It is written over, by its hash as the remote has it now.
		if list, lerr := e.inner.List(); lerr == nil {
			if ent, ok := list[e.opaque(name)]; ok && !ent.Deleted {
				err = e.inner.Put(e.opaque(name), e.seal(data), ent.Hash)
			}
		}
	}
	if err != nil && !e.sentBefore(name, data) {
		return err
	}
	if e.idx[e.opaque(name)] == name {
		return nil // the index had it when it was read, and names are never taken out
	}
	e.idx[e.opaque(name)] = name
	if e.pending++; e.pending < indexEvery {
		return nil
	}
	return e.Flush()
}

// Flush puts the index, if names were added to it since it was last put.
func (e *EncRemote) Flush() error {
	if e.pending == 0 {
		return nil
	}
	if err := e.saveIndex(); err != nil {
		return err
	}
	e.pending = 0
	return nil
}

func (e *EncRemote) Delete(name, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	return e.inner.Delete(e.opaque(name), prev)
}

// ---- the engine ----

// SyncState is what was true at the last sync: for each file, its hash here and the remote's hash then.
type SyncState struct {
	Files map[string]fileSync `json:"files"`
	// AllowRemovals lets this one pass delete here what the remote does not have, past massRemovals (the
	// user said so); it is never kept.
	AllowRemovals bool `json:"-"`
	// Seen is each file as it was seen when its hash here was what the last sync had (localFiles).
	Seen map[string]fileSeen `json:"seen,omitempty"`
	// Pristine is the files a new library made for itself (its first collection and page, its journal), by
	// their hash then, until they are first synced. One the remote has too, never synced here and still as it
	// was made, is the remote's, with no conflict: the library is a new device's, and the account's copy is
	// the real one.
	Pristine map[string]string `json:"pristine,omitempty"`
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
	Held                             int      // files not deleted here, though the remote has none: too many at once (massRemovals)
	Conflicts                        []string // the files that changed on both sides: ours is kept beside
	Errors                           []string
}

func (r SyncResult) String() string {
	if r.Pushed+r.Pulled+r.Deleted+r.Removed+r.Held+len(r.Conflicts) == 0 && len(r.Errors) == 0 {
		return "Eşit: değişiklik yok"
	}
	s := fmt.Sprintf("%d gönderildi, %d alındı", r.Pushed, r.Pulled)
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
// read: a library of thousands was read whole, and hashed, every pass. seen is what is seen now.
func localFiles(fsys FS, root string, skip map[string]bool, st *SyncState) (map[string]string, map[string]fileSeen, error) {
	out := map[string]string{}
	seen := map[string]fileSeen{}
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		ents, err := fsys.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range ents {
			name := e.Name()
			r := path.Join(rel, name)
			if rel == "" && skip[name] {
				continue
			}
			if strings.HasSuffix(name, ".tmp~") || name == SettingsKey {
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
			if err != nil {
				return err
			}
			out[r] = HashOf(b)
		}
		return nil
	}
	return out, seen, walk(root, "")
}

// PristineOf is the files of the library under root by their hash, as SyncState.Pristine has them: what a new
// library made for itself, before anything was written in it.
func PristineOf(fsys FS, root string) (map[string]string, error) {
	local, _, err := localFiles(fsys, root, nil, &SyncState{})
	return local, err
}

func conflictName(name string, at time.Time) string {
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext) + " (çakışma " + at.Format("2006-01-02 15-04-05") + ")" + ext
}

// SyncOnce makes one pass: it puts the library under root (in fsys) and the remote in agreement. What
// the remote keeps back to write at the end (the sealed index's names, encRemote) is written then, an
// error or not.
func SyncOnce(fsys FS, root string, r Remote, st *SyncState, skip map[string]bool, now time.Time) (SyncResult, error) {
	res, pushed, err := syncOnce(fsys, root, r, st, skip, now)
	if f, ok := r.(interface{ Flush() error }); ok {
		if ferr := f.Flush(); ferr != nil {
			// the names of what was sent are not on the remote: what went up is forgotten, and goes up
			// again next pass. Kept as sent, the next pass would read it as deleted there, and delete it here.
			for _, n := range pushed {
				delete(st.Files, n)
			}
			if err == nil {
				err = ferr
			}
			return res, err
		}
	}
	if len(pushed) > 0 { // the remote's hash of what was sent, read once, after its names are there
		if after, lerr := r.List(); lerr == nil {
			for _, n := range pushed {
				f := st.Files[n]
				f.Remote = after[n].Hash
				st.Files[n] = f
			}
		}
	}
	return res, err
}

func syncOnce(fsys FS, root string, r Remote, st *SyncState, skip map[string]bool, now time.Time) (res SyncResult, pushed []string, err error) {
	if st.Files == nil {
		st.Files = map[string]fileSync{}
	}
	local, seen, err := localFiles(fsys, root, skip, st)
	if err != nil {
		return res, pushed, err
	}
	remote, err := r.List()
	if err != nil {
		return res, pushed, err
	}
	for n := range remote { // a name that leaves the folder is a remote's to write over, or to delete, what is outside the library
		if !SafeName(n) {
			delete(remote, n)
			res.Errors = append(res.Errors, n+": "+errUnsafeName.Error())
		}
	}
	for n := range st.Files {
		if !SafeName(n) {
			delete(st.Files, n)
		}
	}
	names := map[string]bool{}
	for n := range local {
		names[n] = true
	}
	for n := range remote {
		if !skip[strings.SplitN(n, "/", 2)[0]] {
			names[n] = true
		}
	}
	for n := range st.Files {
		names[n] = true
	}
	order := make([]string, 0, len(names))
	for n := range names {
		order = append(order, n)
	}
	sort.Strings(order)
	abs := func(n string) string { return filepath.Join(root, filepath.FromSlash(n)) }
	fail := func(n string, err error) { res.Errors = append(res.Errors, n+": "+err.Error()) }
	var gone []string // what the remote has no more: the files to take away from here
	// files the remote has no entry for at all (no tombstone), that are here as at the last pass: they
	// would be deleted here. A few is a deletion elsewhere whose tombstone is gone; a tenth of the library
	// is a remote that lost its list.
	hold := false
	if !st.AllowRemovals {
		gone := 0
		for _, n := range order {
			if _, rok := remote[n]; !rok {
				if lh := local[n]; lh != "" && lh == st.Files[n].Local && st.Files[n].Remote != "" {
					gone++
				}
			}
		}
		hold = gone >= massRemovals && gone*10 >= len(local)
	}
	for _, n := range order {
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
			continue
		case localChanged && !remoteChanged: // ours: up
			if lh == "" {
				if err := r.Delete(n, rh); err != nil {
					fail(n, err)
					continue
				}
				res.Deleted++
				delete(st.Files, n)
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
			pushed = append(pushed, n)
		case !localChanged && remoteChanged: // theirs: down
			if rh == "" {
				if _, there := remote[n]; hold && !there {
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
				continue
			}
			if lh != "" && rh != "" && lh == localHashOf(r, n, rh, lh) {
				st.Files[n] = fileSync{Local: lh, Remote: rh} // the same bytes on both sides
				continue
			}
			// theirs stays, ours is kept beside, and goes up as a file of its own on the next pass
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
			if rh == "" { // they deleted, we changed: ours stays, and goes up again
				st.Files[n] = fileSync{Local: "", Remote: ""}
				continue
			}
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
		}
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
		removed, failed := syncTrash(fsys, root, gone, local, kept, now)
		for _, n := range removed {
			res.Removed++
			delete(st.Files, n)
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
	return res, pushed, nil
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

// sentBefore reports whether the remote holds name already with these very bytes, though its index
// does not name it: a pass whose index was not written sent it, and sending it again is done.
func (e *EncRemote) sentBefore(name string, data []byte) bool {
	if e.idx[e.opaque(name)] == name {
		return false
	}
	raw, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return false
	}
	plain, err := e.open(raw)
	return err == nil && bytes.Equal(plain, data)
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

// SettingsKey is what the settings are kept under in a library's file.
const SettingsKey = "settings"
