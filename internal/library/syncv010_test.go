package library

// The encrypted remote of v0.1.0, the last release before #148, as it was (git show v0.1.0:internal/library/sync.go),
// but for its name: it put the index's names only at the end of a pass (Flush), which its own SyncOnce called, so a
// test calls it after each pass (v010Pass).

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// v010EncRemote is EncRemote as v0.1.0 has it.
type v010EncRemote struct {
	inner   Remote
	Key     []byte
	gcm     cipher.AEAD
	idx     map[string]string // opaque name → path
	loaded  bool
	pending int
	sent    map[string]string
}

const indexEvery = 200

func newV010EncRemote(inner Remote, key []byte) *v010EncRemote {
	blk, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(blk)
	return &v010EncRemote{inner: inner, Key: key, gcm: g, idx: map[string]string{}}
}

// PutHash is as v0.1.0 had it.
func (e *v010EncRemote) PutHash(name string) (string, bool) {
	h, ok := e.sent[name]
	return h, ok
}

// v010Pass is a pass of v0.1.0: its SyncOnce put the index after the files, and forgot what went up when it could not.
func v010Pass(d *device, r *v010EncRemote) (SyncResult, error) {
	res, err := SyncOnce(OSFS{}, d.root, r, &d.st, nil, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))
	if ferr := r.Flush(); ferr != nil && err == nil {
		err = ferr
	}
	return res, err
}

func (e *v010EncRemote) seal(b []byte) []byte {
	nonce := make([]byte, e.gcm.NonceSize())
	rand.Read(nonce)
	return e.gcm.Seal(nonce, nonce, b, nil)
}

func (e *v010EncRemote) open(b []byte) ([]byte, error) {
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

func (e *v010EncRemote) opaque(p string) string {
	m := hmac.New(sha256.New, e.Key)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

func (e *v010EncRemote) loadIndex() error {
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
func (e *v010EncRemote) saveIndex() error {
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

func (e *v010EncRemote) List() (map[string]RemoteEntry, error) {
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
func (e *v010EncRemote) Versions(path string) ([]RemoteVersion, error) {
	v, ok := e.inner.(versioned)
	if !ok {
		return nil, errNoVersions
	}
	return v.Versions(e.opaque(path))
}

func (e *v010EncRemote) VersionData(path string, at int64) ([]byte, error) {
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

func (e *v010EncRemote) Get(name string) ([]byte, error) {
	if err := e.loadIndex(); err != nil {
		return nil, err
	}
	b, err := e.inner.Get(e.opaque(name))
	if err != nil {
		return nil, err
	}
	return e.open(b)
}

func (e *v010EncRemote) Put(name string, data []byte, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	sealed := e.seal(data)
	err := e.inner.Put(e.opaque(name), sealed, prev)
	if errors.Is(err, ErrConflict) && prev == "" && e.idx[e.opaque(name)] != name {
		// an object a pass left there, its name never put in the index: it is ours, and what is in it may
		// be older than what is sent now. It is written over, by its hash as the remote has it now.
		if list, lerr := e.inner.List(); lerr == nil {
			if ent, ok := list[e.opaque(name)]; ok && !ent.Deleted {
				err = e.inner.Put(e.opaque(name), sealed, ent.Hash)
			}
		}
	}
	if err == nil {
		if e.sent == nil {
			e.sent = map[string]string{}
		}
		e.sent[name] = HashOf(sealed)
	} else if !e.sentBefore(name, data) {
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
func (e *v010EncRemote) Flush() error {
	if e.pending == 0 {
		return nil
	}
	if err := e.saveIndex(); err != nil {
		return err
	}
	e.pending = 0
	return nil
}

func (e *v010EncRemote) Delete(name, prev string) error {
	if err := e.loadIndex(); err != nil {
		return err
	}
	return e.inner.Delete(e.opaque(name), prev)
}

// sentBefore reports whether the remote holds name already with these very bytes, though its index
// does not name it: a pass whose index was not written sent it, and sending it again is done.
func (e *v010EncRemote) sentBefore(name string, data []byte) bool {
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
