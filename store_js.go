//go:build js

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall/js"
	"time"
)

// The library in a browser: the same tree of folders and files as the Store of a file, kept in memory and
// written to the browser's IndexedDB a moment after a change (a few hundred milliseconds, and when the page
// is hidden), so that it holds as much as the browser allows; where IndexedDB is not there it is localStorage,
// with its five megabytes. It is read once, at the start.
// There are no versions (a historian is what keeps them) and no backups of the file: the browser's storage
// is what there is, and the library syncs to a remote like any other (sync.go).

type jnode struct {
	Dir   bool   `json:"d,omitempty"`
	Data  string `json:"b,omitempty"` // base64
	MTime int64  `json:"t"`
}

// Store is a library kept in the browser.
type Store struct {
	File  string
	mu    sync.Mutex
	node  map[string]*jnode
	meta  map[string]string // base64
	hold  int
	dirty bool
	now   func() time.Time
}

func storage() js.Value {
	s := js.Global().Get("localStorage")
	if s.IsUndefined() || s.IsNull() {
		return js.Undefined()
	}
	return s
}

// idbCode is the little of IndexedDB the store uses: one object store, a get and a put, both promises.
const idbCode = `(function(){
  if (window.__idb || !window.indexedDB) return;
  const open = () => new Promise((res, rej) => { const r = indexedDB.open('ryudit', 1); r.onupgradeneeded = () => r.result.createObjectStore('lib'); r.onsuccess = () => res(r.result); r.onerror = () => rej(r.error); });
  window.__idb = {
    get: async (k) => { const db = await open(); return await new Promise((res, rej) => { const q = db.transaction('lib').objectStore('lib').get(k); q.onsuccess = () => res(q.result === undefined ? null : q.result); q.onerror = () => rej(q.error); }); },
    put: async (k, v) => { const db = await open(); return await new Promise((res, rej) => { const t = db.transaction('lib', 'readwrite'); t.objectStore('lib').put(v, k); t.oncomplete = () => res(true); t.onerror = () => rej(t.error); }); },
  };
})()`

// idb is the IndexedDB helper, or undefined where there is none.
func idb() js.Value {
	js.Global().Call("eval", idbCode)
	h := js.Global().Get("__idb")
	if h.IsUndefined() || h.IsNull() {
		return js.Undefined()
	}
	return h
}

// idbGet reads a key, waiting for the promise (the browser's event loop runs on): ok is false when there is none.
func idbGet(h js.Value, key string) (string, bool) {
	type res struct {
		v  string
		ok bool
	}
	ch := make(chan res, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Type() == js.TypeString {
			ch <- res{args[0].String(), true}
		} else {
			ch <- res{}
		}
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any { ch <- res{}; return nil })
	defer then.Release()
	defer catch.Release()
	h.Call("get", key).Call("then", then).Call("catch", catch)
	select {
	case r := <-ch:
		return r.v, r.ok
	case <-time.After(4 * time.Second):
		return "", false
	}
}

// OpenStore opens the library named file in the browser's storage, making it when there is none.
func OpenStore(file string) (*Store, error) {
	s := &Store{File: file, node: map[string]*jnode{"/": {Dir: true, MTime: time.Now().UnixNano()}}, meta: map[string]string{}}
	raw, found := "", false
	if h := idb(); !h.IsUndefined() {
		raw, found = idbGet(h, "ryudit:"+file)
	}
	if !found { // a library of before IndexedDB, or a browser without it
		if st := storage(); !st.IsUndefined() {
			if v := st.Call("getItem", "ryudit:"+file); v.Type() == js.TypeString {
				raw, found = v.String(), true
			}
		}
	}
	if found {
		var in struct {
			Nodes map[string]*jnode `json:"nodes"`
			Meta  map[string]string `json:"meta"`
		}
		if err := json.Unmarshal([]byte(raw), &in); err == nil && in.Nodes != nil {
			s.node = in.Nodes
			if in.Meta != nil {
				s.meta = in.Meta
			}
			if s.node["/"] == nil {
				s.node["/"] = &jnode{Dir: true, MTime: time.Now().UnixNano()}
			}
		}
	}
	// the page going away: what is waiting is written at once
	hide := js.FuncOf(func(this js.Value, args []js.Value) any { s.mu.Lock(); s.flushLocked(); s.mu.Unlock(); return nil })
	js.Global().Get("document").Call("addEventListener", "visibilitychange", hide)
	js.Global().Call("addEventListener", "pagehide", hide)
	return s, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	s.dirty = true
	s.flushLocked()
	s.mu.Unlock()
	return nil
}

// persist notes that the library changed: it is written a moment later, once for all the changes of the moment.
func (s *Store) persist() {
	if s.hold > 0 || s.dirty {
		return
	}
	s.dirty = true
	time.AfterFunc(300*time.Millisecond, func() { s.mu.Lock(); s.flushLocked(); s.mu.Unlock() })
}

// flushLocked writes the library to the browser's storage now (s.mu is held).
func (s *Store) flushLocked() {
	if !s.dirty {
		return
	}
	s.dirty = false
	data, _ := json.Marshal(struct {
		Nodes map[string]*jnode `json:"nodes"`
		Meta  map[string]string `json:"meta"`
	}{s.node, s.meta})
	key := "ryudit:" + s.File
	if h := idb(); !h.IsUndefined() {
		h.Call("put", key, string(data))
		return
	}
	if st := storage(); !st.IsUndefined() {
		st.Call("setItem", key, string(data)) // over the quota it throws: the library stays in memory
	}
}

// fast holds the writes to the browser's storage while a copy goes on, and writes them once after.
func (s *Store) fast(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		s.hold++
		return
	}
	if s.hold > 0 {
		s.hold--
	}
	s.persist()
}

func (s *Store) Meta(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.meta[key]
	if !ok {
		return nil
	}
	b, _ := base64.StdEncoding.DecodeString(v)
	return b
}

func (s *Store) SetMeta(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[key] = base64.StdEncoding.EncodeToString(value)
	s.persist()
	return nil
}

// Snapshot: a browser has no file to copy; the library is exported or synced instead.
func (s *Store) Snapshot(dst string) error {
	return errors.New("tarayıcıda dosya kopyası yok: dışa aktar ya da eşitle")
}

func (s *Store) get(name string) (*jnode, error) {
	n, ok := s.node[clean(name)]
	if !ok {
		return nil, pathErr("open", name, fs.ErrNotExist)
	}
	return n, nil
}

func (s *Store) ReadFile(name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.get(name)
	if err != nil {
		return nil, err
	}
	if n.Dir {
		return nil, pathErr("read", name, errors.New("is a directory"))
	}
	b, _ := base64.StdEncoding.DecodeString(n.Data)
	return b, nil
}

func (s *Store) WriteFile(name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := clean(name)
	p, ok := s.node[path.Dir(n)]
	if !ok {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if !p.Dir {
		return pathErr("write", name, errors.New("not a directory"))
	}
	if cur, ok := s.node[n]; ok && cur.Dir {
		return pathErr("write", name, errors.New("is a directory"))
	}
	s.node[n] = &jnode{Data: base64.StdEncoding.EncodeToString(data), MTime: time.Now().UnixNano()}
	s.persist()
	return nil
}

func (s *Store) mkdir(name string) error {
	n := clean(name)
	if _, ok := s.node[n]; ok {
		return pathErr("mkdir", name, fs.ErrExist)
	}
	if p, ok := s.node[path.Dir(n)]; !ok || !p.Dir {
		return pathErr("mkdir", name, fs.ErrNotExist)
	}
	s.node[n] = &jnode{Dir: true, MTime: time.Now().UnixNano()}
	return nil
}

func (s *Store) Mkdir(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.mkdir(name)
	if err == nil {
		s.persist()
	}
	return err
}

func (s *Store) MkdirAll(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var chain []string
	for p := clean(name); p != "/"; p = path.Dir(p) {
		chain = append(chain, p)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if n, ok := s.node[chain[i]]; ok {
			if !n.Dir {
				return pathErr("mkdir", name, errors.New("not a directory"))
			}
			continue
		}
		if err := s.mkdir(chain[i]); err != nil {
			return err
		}
	}
	s.persist()
	return nil
}

// under are the paths of dir and all below it.
func (s *Store) under(dir string) []string {
	var out []string
	for p := range s.node {
		if p == dir || strings.HasPrefix(p, dir+"/") {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func (s *Store) Rename(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	from, to := clean(oldName), clean(newName)
	if _, ok := s.node[from]; !ok {
		return pathErr("rename", oldName, fs.ErrNotExist)
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to, from+"/") {
		return pathErr("rename", oldName, errors.New("into itself"))
	}
	if _, ok := s.node[to]; ok {
		return pathErr("rename", newName, fs.ErrExist)
	}
	if p, ok := s.node[path.Dir(to)]; !ok || !p.Dir {
		return pathErr("rename", newName, fs.ErrNotExist)
	}
	for _, p := range s.under(from) {
		s.node[to+strings.TrimPrefix(p, from)] = s.node[p]
		delete(s.node, p)
	}
	s.persist()
	return nil
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := clean(name)
	cur, ok := s.node[n]
	if !ok {
		return pathErr("remove", name, fs.ErrNotExist)
	}
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	if cur.Dir && len(s.under(n)) > 1 {
		return pathErr("remove", name, errors.New("directory not empty"))
	}
	delete(s.node, n)
	s.persist()
	return nil
}

func (s *Store) RemoveAll(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := clean(name)
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	for _, p := range s.under(n) {
		delete(s.node, p)
	}
	s.persist()
	return nil
}

func (s *Store) Stat(name string) (fs.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := clean(name)
	cur, ok := s.node[n]
	if !ok {
		return nil, pathErr("stat", name, fs.ErrNotExist)
	}
	return info{name: path.Base(n), size: int64(base64.StdEncoding.DecodedLen(len(cur.Data))), dir: cur.Dir, mtime: time.Unix(0, cur.MTime)}, nil
}

func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := clean(name)
	if cur, ok := s.node[n]; !ok || !cur.Dir {
		return nil, pathErr("readdir", name, fs.ErrNotExist)
	}
	var out []fs.DirEntry
	for p, cur := range s.node {
		if p != "/" && path.Dir(p) == n {
			out = append(out, info{name: path.Base(p), size: int64(base64.StdEncoding.DecodedLen(len(cur.Data))), dir: cur.Dir, mtime: time.Unix(0, cur.MTime)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

// snapshot and endSnapshot are the file Store's way to read the table at once while the library opens; the
// browser's library is all in memory already.
func (s *Store) snapshot()    {}
func (s *Store) endSnapshot() {}

// ReadNamed reads every file whose name ends in "/"+base, by its path (as the file Store does).
func (s *Store) ReadNamed(base string) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]byte{}
	for p, n := range s.node {
		if n.Dir || !strings.HasSuffix(p, "/"+base) {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(n.Data)
		if err != nil {
			return nil, err
		}
		out[p] = b
	}
	return out, nil
}
