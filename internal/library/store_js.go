//go:build js

package library

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	File     string
	mu       sync.Mutex
	node     map[string]*jnode
	meta     map[string]string // base64
	hold     int
	dirty    bool
	readOnly bool
	// before is the library as it was read, kept under a key of its own before the first write over it: the
	// browser's storage has no versions and no backups, and this is one way back (#237).
	before string
	// writeErr is what the last write said, or nil (WriteError).
	writeErr   error
	retryTimer *time.Timer
	// Writes counts the changes to the files and folders, as the Store of a file has it (store_sql.go).
	Writes atomic.Uint64
}

func storage() js.Value {
	s := js.Global().Get("localStorage")
	if s.IsUndefined() || s.IsNull() {
		return js.Undefined()
	}
	return s
}

// idbCode is the little of IndexedDB the store uses: one object store, a get and a put, both promises. The
// connection is opened once and kept, so that a put made as the page goes away starts its transaction at once,
// before the page is gone (#238).
const idbCode = `(function(){
  if (window.__idb || !window.indexedDB) return;
  let db = null, opening = null;
  const open = () => db ? Promise.resolve(db) : (opening = opening || new Promise((res, rej) => { const r = indexedDB.open('celestine', 1); r.onupgradeneeded = () => r.result.createObjectStore('lib'); r.onsuccess = () => { db = r.result; res(db); }; r.onerror = () => rej(r.error); }));
  const put = (d, k, v) => new Promise((res, rej) => { const t = d.transaction('lib', 'readwrite'); t.objectStore('lib').put(v, k); t.oncomplete = () => res(true); t.onerror = () => rej(t.error); t.onabort = () => rej(t.error); });
  window.__idb = {
    get: async (k) => { const d = await open(); return await new Promise((res, rej) => { const q = d.transaction('lib').objectStore('lib').get(k); q.onsuccess = () => res(q.result === undefined ? null : q.result); q.onerror = () => rej(q.error); }); },
    put: (k, v) => db ? put(db, k, v) : open().then((d) => put(d, k, v)),
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

// readWait is how long a read of the library may take before it is a failure: a library of tens of megabytes on a
// phone takes seconds, and a slow read is not an absent one.
const readWait = 2 * time.Minute

// idbGet reads a key, waiting for the promise (the browser's event loop runs on). found is false when there is
// none; err is set when it could not be read, which is not the same (#237).
func idbGet(h js.Value, key string) (v string, found bool, err error) {
	type res struct {
		v   string
		ok  bool
		err error
	}
	ch := make(chan res, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		switch {
		case len(args) > 0 && args[0].Type() == js.TypeString:
			ch <- res{v: args[0].String(), ok: true}
		case len(args) == 0 || args[0].IsNull() || args[0].IsUndefined():
			ch <- res{}
		default:
			ch <- res{err: errors.New("the library in the browser's storage is not text")}
		}
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := "the browser's storage could not be read"
		if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
			msg += ": " + args[0].Call("toString").String()
		}
		ch <- res{err: errors.New(msg)}
		return nil
	})
	defer then.Release()
	defer catch.Release()
	h.Call("get", key).Call("then", then).Call("catch", catch)
	select {
	case r := <-ch:
		return r.v, r.ok, r.err
	case <-time.After(readWait):
		return "", false, errors.New("the browser's storage did not answer")
	}
}

// OpenStoreReadOnly opens an existing library named file in browser storage read-only without creating a store or writing back.
func OpenStoreReadOnly(file string) (*Store, error) {
	return openStore(file, true)
}

// OpenStore opens the library named file in the browser's storage, making it when there is none. A library that is
// there but cannot be read is an error, never an empty library: that would be written over the real one a moment
// later (#237).
func OpenStore(file string) (*Store, error) {
	return openStore(file, false)
}

// openStore loads the library named file from browser storage; when readOnly is true, missing stores fail and writes never flush.
func openStore(file string, readOnly bool) (*Store, error) {
	s := &Store{File: file, readOnly: readOnly, node: map[string]*jnode{"/": {Dir: true, MTime: time.Now().UnixNano()}}, meta: map[string]string{}}
	raw, found := "", false
	if h := idb(); !h.IsUndefined() {
		var err error
		if raw, found, err = idbGet(h, "celestine:"+file); err != nil {
			return nil, err
		}
	}
	if !found { // a library of before IndexedDB, or a browser without it
		if st := storage(); !st.IsUndefined() {
			if v := st.Call("getItem", "celestine:"+file); v.Type() == js.TypeString {
				raw, found = v.String(), true
			}
		}
	}
	if !found && readOnly {
		return nil, fs.ErrNotExist
	}
	if found {
		var in struct {
			Nodes map[string]*jnode `json:"nodes"`
			Meta  map[string]string `json:"meta"`
		}
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return nil, errors.New("the library in the browser's storage is damaged: " + err.Error())
		}
		if in.Nodes != nil {
			s.node = in.Nodes
		}
		if in.Meta != nil {
			s.meta = in.Meta
		}
		if s.node["/"] == nil {
			s.node["/"] = &jnode{Dir: true, MTime: time.Now().UnixNano()}
		}
		s.before = raw
	}
	if !readOnly {
		// the page going away: what is waiting is written at once
		hide := js.FuncOf(func(this js.Value, args []js.Value) any { s.mu.Lock(); s.flushLocked(); s.mu.Unlock(); return nil })
		js.Global().Get("document").Call("addEventListener", "visibilitychange", hide)
		js.Global().Call("addEventListener", "pagehide", hide)
	}
	return s, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readOnly {
		return nil
	}
	s.dirty = true
	s.flushLocked()
	return nil
}

// persist notes that the library changed: it is written a moment later, once for all the changes of the moment.
func (s *Store) persist() {
	if s.readOnly || s.hold > 0 || s.dirty {
		return
	}
	s.dirty = true
	time.AfterFunc(300*time.Millisecond, func() { s.mu.Lock(); s.flushLocked(); s.mu.Unlock() })
}

// flushLocked writes the library to the browser's storage now (s.mu is held). A write that fails leaves the library
// to be written again, and says so through WriteError (#238).
func (s *Store) flushLocked() {
	if s.readOnly || !s.dirty {
		return
	}
	s.dirty = false
	data, _ := json.Marshal(struct {
		Nodes map[string]*jnode `json:"nodes"`
		Meta  map[string]string `json:"meta"`
	}{s.node, s.meta})
	key := "celestine:" + s.File
	if h := idb(); !h.IsUndefined() {
		if s.before != "" { // once: the library as it was read, before it is first written over
			h.Call("put", key+":before", s.before)
			s.before = ""
		}
		var then, catch js.Func
		release := func() { then.Release(); catch.Release() }
		then = js.FuncOf(func(this js.Value, args []js.Value) any {
			release()
			go func() {
				s.mu.Lock()
				s.writeErr = nil
				if s.retryTimer != nil {
					s.retryTimer.Stop()
					s.retryTimer = nil
				}
				s.mu.Unlock()
			}()
			return nil
		})
		catch = js.FuncOf(func(this js.Value, args []js.Value) any {
			release()
			msg := "the library could not be written to the browser's storage"
			if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
				msg += ": " + args[0].Call("toString").String()
			}
			go func() {
				s.mu.Lock()
				s.writeErr = errors.New(msg)
				s.scheduleRetryLocked()
				s.mu.Unlock()
			}()
			return nil
		})
		h.Call("put", key, string(data)).Call("then", then).Call("catch", catch)
		return
	}
	if st := storage(); !st.IsUndefined() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.writeErr = errors.New("the library could not be written to localStorage")
					s.scheduleRetryLocked()
				}
			}()
			st.Call("setItem", key, string(data)) // over the quota it throws: the library stays in memory (#239)
			s.writeErr = nil
			if s.retryTimer != nil {
				s.retryTimer.Stop()
				s.retryTimer = nil
			}
		}()
	}
}

// scheduleRetryLocked schedules one retry of flushLocked after a browser storage write failure (#239).
func (s *Store) scheduleRetryLocked() {
	if s.retryTimer != nil {
		return
	}
	s.retryTimer = time.AfterFunc(5*time.Second, func() {
		s.mu.Lock()
		s.retryTimer = nil
		s.dirty = true
		s.flushLocked()
		s.mu.Unlock()
	})
}

// WriteError is what the last write to the browser's storage said, or nil: the app shows it, as what is typed is
// then only in memory.
func (s *Store) WriteError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeErr
}

// Fast holds the writes to the browser's storage while a copy goes on, and writes them once after.
func (s *Store) Fast(on bool) {
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

// DeleteMeta takes away what is kept under key.
func (s *Store) DeleteMeta(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.meta[key]; ok {
		delete(s.meta, key)
		s.persist()
	}
	return nil
}

// Snapshot: a browser has no file to copy; the library is exported or synced instead.
func (s *Store) Snapshot(dst string) error {
	return errors.New("a browser has no file to copy: export the library or sync it")
}

func (s *Store) get(name string) (*jnode, error) {
	n, ok := s.node[Clean(name)]
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
	defer s.Writes.Add(1)
	n := Clean(name)
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

// WriteFileKeepTime writes data over the file name and leaves it the time it was last written (TimeKeeper).
func (s *Store) WriteFileKeepTime(name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.Writes.Add(1)
	n := Clean(name)
	cur, ok := s.node[n]
	if !ok || cur.Dir {
		return pathErr("write", name, fs.ErrNotExist)
	}
	s.node[n] = &jnode{Data: base64.StdEncoding.EncodeToString(data), MTime: cur.MTime}
	s.persist()
	return nil
}

func (s *Store) mkdir(name string) error {
	n := Clean(name)
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
	defer s.Writes.Add(1)
	err := s.mkdir(name)
	if err == nil {
		s.persist()
	}
	return err
}

func (s *Store) MkdirAll(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.Writes.Add(1)
	var chain []string
	for p := Clean(name); p != "/"; p = path.Dir(p) {
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
	defer s.Writes.Add(1)
	from, to := Clean(oldName), Clean(newName)
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
	defer s.Writes.Add(1)
	n := Clean(name)
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
	defer s.Writes.Add(1)
	n := Clean(name)
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
	n := Clean(name)
	cur, ok := s.node[n]
	if !ok {
		return nil, pathErr("stat", name, fs.ErrNotExist)
	}
	return info{name: path.Base(n), size: int64(base64.StdEncoding.DecodedLen(len(cur.Data))), dir: cur.Dir, mtime: time.Unix(0, cur.MTime)}, nil
}

func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := Clean(name)
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

// BeginSnapshot and EndSnapshot are the file Store's way to read the table at once while the library opens; the
// browser's library is all in memory already.
func (s *Store) BeginSnapshot() {}
func (s *Store) EndSnapshot()   {}

// ReadNamedFrom reads the files whose name ends in "/"+base and whose path comes after after, in the order of their
// paths, limit at most, and says the last path read (as the file Store does).
func (s *Store) ReadNamedFrom(base, after string, limit int) (map[string][]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var paths []string
	for p, n := range s.node {
		if !n.Dir && strings.HasSuffix(p, "/"+base) && p > after {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	if len(paths) > limit {
		paths = paths[:limit]
	}
	out := map[string][]byte{}
	last := after
	for _, p := range paths {
		b, err := base64.StdEncoding.DecodeString(s.node[p].Data)
		if err != nil {
			return nil, after, err
		}
		out[p], last = b, p
	}
	return out, last, nil
}

// Batch runs fn with the writes to the browser's storage held, and writes them once after: there is one value to
// write, so it is whole or not written.
func (s *Store) Batch(fn func(tx FS) error) error {
	s.Fast(true)
	defer s.Fast(false)
	return fn(s)
}

// ReadPrewarmFrom delegates to ReadNamedFrom on browser storage.
func (s *Store) ReadPrewarmFrom(base, after string, limit int) (map[string]PrewarmRow, string, error) {
	raw, last, err := s.ReadNamedFrom(base, after, limit)
	if err != nil {
		return nil, last, err
	}
	out := make(map[string]PrewarmRow, len(raw))
	for p, data := range raw {
		out[p] = PrewarmRow{Src: data}
	}
	return out, last, nil
}

// WritePageIndexes is a no-op on browser storage.
func (s *Store) WritePageIndexes([]PageIndex) error { return nil }

// PrunePageIndex has nothing to prune in the browser's store, which keeps no page_index.
func (s *Store) PrunePageIndex() (int64, error) { return 0, nil }
