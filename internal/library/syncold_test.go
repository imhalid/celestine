package library

// The encrypted remote of the versions before #148, as they were (git show cd21b2a:internal/library/sync.go), but for
// its name: the tests of #148 sync with it, on the remote this version writes, to see that both keep syncing.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// oldEncRemote is EncRemote as a version before #148 has it.
type oldEncRemote struct {
	inner   Remote
	Key     []byte
	gcm     cipher.AEAD
	idx     map[string]string // opaque name → path
	loaded  bool
	indexed bool
	sent    map[string]string
}

func newOldEncRemote(inner Remote, key []byte) *oldEncRemote {
	blk, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(blk)
	return &oldEncRemote{inner: inner, Key: key, gcm: g, idx: map[string]string{}}
}

// PutHash is as the old version had it.
func (e *oldEncRemote) PutHash(name string) (string, bool) {
	h, ok := e.sent[name]
	return h, ok
}

func (e *oldEncRemote) seal(b []byte) []byte {
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, nil)
}

func (e *oldEncRemote) open(b []byte) ([]byte, error) {
	n := e.gcm.NonceSize()
	if len(b) < n {
		return nil, errors.New("şifreli dosya kısa")
	}
	out, err := e.gcm.Open(nil, b[:n], b[n:], nil)
	if err != nil {
		return nil, errSealed
	}
	return out, nil
}

func (e *oldEncRemote) opaque(p string) string {
	m := hmac.New(sha256.New, e.Key)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

func (e *oldEncRemote) loadIndex() error {
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
	e.loaded, e.indexed = true, true
	return json.Unmarshal(plain, &e.idx)
}

// HasIndex says whether the remote held a sealed index when it was opened, which the key opened: only then is a
// passphrase known to be the remote's (OpenEnc). A remote with a salt and no index takes any passphrase.
func (e *oldEncRemote) HasIndex() bool { return e.indexed }

// saveIndex puts the index back, merged with what the remote holds now: another device may have added
// names since this one read it, and an index written from the old copy would drop them (their files would
// then not be seen, and their owner would take them for deleted). Names are only ever added, so a union
// loses nothing; if the index changes again while it is being put, it is read and merged once more.
func (e *oldEncRemote) saveIndex() error {
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

func (e *oldEncRemote) List() (map[string]RemoteEntry, error) {
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
func (e *oldEncRemote) Versions(path string) ([]RemoteVersion, error) {
	v, ok := e.inner.(versioned)
	if !ok {
		return nil, errNoVersions
	}
	return v.Versions(e.opaque(path))
}

func (e *oldEncRemote) VersionData(path string, at int64) ([]byte, error) {
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

func (e *oldEncRemote) Get(name string) ([]byte, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	b, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return nil, err
	}
	return e.open(b)
}

func (e *oldEncRemote) Put(name string, data []byte, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	if err := e.Announce([]string{name}); err != nil { // its name first, then the object (Announce)
		return err
	}
	sealed := e.seal(data)
	if err := e.inner.Put(e.opaque(name), sealed, prev); err != nil {
		return err
	}
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
func (e *oldEncRemote) Announce(names []string) error {
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

func (e *oldEncRemote) Delete(name, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	return e.inner.Delete(e.opaque(name), prev)
}
