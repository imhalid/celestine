package main

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

func hashOf(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// ---- a folder as a remote ----

type dirRemote struct{ root string }

func (d dirRemote) file(name string) string { return filepath.Join(d.root, filepath.FromSlash(name)) }

func (d dirRemote) List() (map[string]RemoteEntry, error) {
	out := map[string]RemoteEntry{}
	err := filepath.WalkDir(d.root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(d.root, p)
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
			out[rel] = RemoteEntry{Hash: hashOf(b)}
		}
		return nil
	})
	// a tombstone beside a file that is there: the later is the tombstone only if the file is gone
	for name, e := range out {
		if e.Deleted {
			if _, err := os.Stat(d.file(name)); err == nil {
				b, _ := os.ReadFile(d.file(name))
				out[name] = RemoteEntry{Hash: hashOf(b)}
			}
		}
	}
	if os.IsNotExist(err) {
		return out, nil
	}
	return out, err
}

const tombSuffix = ".deleted~"

func (d dirRemote) current(name string) (string, error) {
	b, err := os.ReadFile(d.file(name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return hashOf(b), nil
}

func (d dirRemote) Get(name string) ([]byte, error) { return os.ReadFile(d.file(name)) }

func (d dirRemote) Put(name string, data []byte, prev string) error {
	if cur, err := d.current(name); err != nil {
		return err
	} else if cur != prev {
		return ErrConflict
	}
	p := d.file(name)
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

func (d dirRemote) Delete(name, prev string) error {
	if cur, err := d.current(name); err != nil {
		return err
	} else if cur != prev {
		return ErrConflict
	}
	p := d.file(name)
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

func newHTTPRemote(base, token string) *httpRemote {
	return &httpRemote{strings.TrimRight(base, "/"), token, &http.Client{Timeout: 60 * time.Second}}
}

func (h *httpRemote) do(method, name string, body []byte, prev *string) (*http.Response, error) {
	if err := httpsOnly(h.base); err != nil {
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

// ---- end-to-end encryption ----

// encRemote wraps a remote: each file is sealed with AES-256-GCM under a key from the passphrase, and
// is stored under a name that is an HMAC of its path: the other side sees the count and the sizes of
// the files, and nothing else. A small sealed index, "_index", maps the names back to paths. Lose the
// passphrase and the recovery key, and the data is gone: that is what it is for.
type encRemote struct {
	inner  Remote
	key    []byte
	gcm    cipher.AEAD
	idx    map[string]string // opaque name → path
	loaded bool
}

const saltFile = "_salt"

// deriveKey makes the key from a passphrase and the salt that is kept on the remote (made when there is none).
func deriveKey(pass string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, pass, salt, 200_000, 32)
}

// recoveryKey is the key as words to write down: base32, in groups.
func recoveryKey(key []byte) string {
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	var parts []string
	for i := 0; i < len(s); i += 4 {
		parts = append(parts, s[i:min(i+4, len(s))])
	}
	return strings.Join(parts, "-")
}

// keyFromRecovery reads what recoveryKey wrote.
func keyFromRecovery(s string) ([]byte, error) {
	s = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), " ", ""))
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, errors.New("kurtarma anahtarı geçersiz")
	}
	return b, nil
}

func newEncRemote(inner Remote, key []byte) (*encRemote, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &encRemote{inner: inner, key: key, gcm: g, idx: map[string]string{}}, nil
}

// openEnc opens the remote with a passphrase: the salt is read from it, or made and put there.
func openEnc(inner Remote, pass string) (*encRemote, error) {
	salt, err := inner.Get(saltFile)
	if err != nil {
		list, lerr := inner.List()
		if lerr != nil {
			return nil, lerr
		}
		if e, ok := list[saltFile]; ok && !e.Deleted {
			return nil, err
		}
		salt = make([]byte, 16)
		rand.Read(salt)
		if err := inner.Put(saltFile, salt, ""); err != nil {
			return nil, err
		}
	}
	key, err := deriveKey(pass, salt)
	if err != nil {
		return nil, err
	}
	return newEncRemote(inner, key)
}

func (e *encRemote) seal(b []byte) []byte {
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, nil)
}

func (e *encRemote) open(b []byte) ([]byte, error) {
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

func (e *encRemote) opaque(p string) string {
	m := hmac.New(sha256.New, e.key)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

const indexFile = "_index"

func (e *encRemote) loadIndex() error {
	if e.loaded {
		return nil
	}
	b, err := e.inner.Get(indexFile)
	if err != nil {
		list, lerr := e.inner.List()
		if lerr != nil {
			return lerr
		}
		if ent, ok := list[indexFile]; ok && !ent.Deleted {
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

func (e *encRemote) saveIndex() error {
	plain, _ := json.Marshal(e.idx)
	list, err := e.inner.List()
	if err != nil {
		return err
	}
	prev := list[indexFile].Hash
	if list[indexFile].Deleted {
		prev = ""
	}
	return e.inner.Put(indexFile, e.seal(plain), prev)
}

func (e *encRemote) List() (map[string]RemoteEntry, error) {
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

func (e *encRemote) Get(name string) ([]byte, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	b, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return nil, err
	}
	return e.open(b)
}

func (e *encRemote) Put(name string, data []byte, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	if err := e.inner.Put(e.opaque(name), e.seal(data), prev); err != nil {
		return err
	}
	e.idx[e.opaque(name)] = name
	return e.saveIndex()
}

func (e *encRemote) Delete(name, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	return e.inner.Delete(e.opaque(name), prev)
}

// ---- the engine ----

// syncState is what was true at the last sync: for each file, its hash here and the remote's hash then.
type syncState struct {
	Files map[string]fileSync `json:"files"`
}

type fileSync struct {
	Local  string `json:"l"` // hash of the file here at the last sync ("" if it was not there)
	Remote string `json:"r"` // the remote's hash of it then
}

// SyncResult says what a sync did.
type SyncResult struct {
	Pushed, Pulled, Deleted, Removed int      // files sent up, brought down, deleted there, deleted here
	Conflicts                        []string // the files that changed on both sides: ours is kept beside
	Errors                           []string
}

func (r SyncResult) String() string {
	if r.Pushed+r.Pulled+r.Deleted+r.Removed+len(r.Conflicts) == 0 && len(r.Errors) == 0 {
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
	return s
}

// localFiles lists the files of the tree under root, by path relative to it, with their hashes; the top
// folders in skip are left out.
func localFiles(fsys FS, root string, skip map[string]bool) (map[string]string, error) {
	out := map[string]string{}
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
			if strings.HasSuffix(name, ".tmp~") || name == settingsKey {
				continue
			}
			if e.IsDir() {
				if err := walk(filepath.Join(dir, name), r); err != nil {
					return err
				}
				continue
			}
			b, err := fsys.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			out[r] = hashOf(b)
		}
		return nil
	}
	return out, walk(root, "")
}

func conflictName(name string, at time.Time) string {
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext) + " (çakışma " + at.Format("2006-01-02 15-04-05") + ")" + ext
}

// SyncOnce makes one pass: it puts the library under root (in fsys) and the remote in agreement.
func SyncOnce(fsys FS, root string, r Remote, st *syncState, skip map[string]bool, now time.Time) (SyncResult, error) {
	var res SyncResult
	if st.Files == nil {
		st.Files = map[string]fileSync{}
	}
	local, err := localFiles(fsys, root, skip)
	if err != nil {
		return res, err
	}
	remote, err := r.List()
	if err != nil {
		return res, err
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
	var pushed []string // the remote's hash of what was sent is read once, after
	defer func() {
		if len(pushed) == 0 {
			return
		}
		if after, err := r.List(); err == nil {
			for _, n := range pushed {
				f := st.Files[n]
				f.Remote = after[n].Hash
				st.Files[n] = f
			}
		}
	}()
	for _, n := range order {
		if strings.Contains(n, "(çakışma ") && false {
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
				if err := fsys.Remove(abs(n)); err != nil && !os.IsNotExist(err) {
					fail(n, err)
					continue
				}
				res.Removed++
				delete(st.Files, n)
				continue
			}
			b, err := r.Get(n)
			if err != nil {
				fail(n, err)
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
			st.Files[n] = fileSync{Local: hashOf(b), Remote: rh}
		default: // both changed
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
			st.Files[n] = fileSync{Local: hashOf(b), Remote: rh}
		}
	}
	return res, nil
}

// localHashOf is the hash of the remote's bytes, for comparing with ours when the remote's own hash is
// of something else (a sealed file's): the bytes are fetched and hashed.
func localHashOf(r Remote, name, remoteHash, _ string) string {
	if _, ok := r.(*encRemote); !ok {
		return remoteHash
	}
	b, err := r.Get(name)
	if err != nil {
		return ""
	}
	return hashOf(b)
}
