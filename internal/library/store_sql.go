//go:build !js

package library

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a library in one SQLite file: a table of nodes, folders and files
// by path, and a table of small things kept by name (the settings).
type Store struct {
	DB   *sql.DB
	File string
	Now  func() time.Time          // the clock of the history; the system's when nil
	Snap atomic.Pointer[storeSnap] // the table read at once, while the library opens
	// Writes counts the changes to the files and folders (not to the settings, Meta): the app syncs a little
	// after it moves, so that what was written goes up before long.
	Writes atomic.Uint64

	batchMu  sync.Mutex  // one Batch at a time
	batching atomic.Bool // a Batch's transaction is open on the connection
}

// wrote marks a change to the tree: what was read at once is no more what is there, and Writes moves.
func (s *Store) wrote() {
	s.Snap.Store(nil)
	s.Writes.Add(1)
}

// sqliteURI is the file: URI of the database at file, with query: the characters a URI reads as its own (%, ?, #)
// are escaped, or a folder named "My #1 notes" opens a database named "My " (#137).
func sqliteURI(file, query string) string {
	p := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(filepath.ToSlash(file))
	return "file:" + p + "?" + query
}

// OpenStore opens the library file at file, making it when there is none.
func OpenStore(file string) (*Store, error) {
	// WAL: a reader on the file, the backup's snapshot or the MCP bridge, does not hold a save back (#136)
	db, err := sql.Open("sqlite", sqliteURI(file, "_pragma=busy_timeout(5000)&_pragma=foreign_keys(0)&_pragma=journal_mode(WAL)"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer, one at a time: the tree is small
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			path   TEXT PRIMARY KEY,
			parent TEXT NOT NULL,
			dir    INTEGER NOT NULL,
			data   BLOB,
			mtime  INTEGER NOT NULL
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS nodes_parent ON nodes(parent)`,
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value BLOB) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS versions (id INTEGER PRIMARY KEY AUTOINCREMENT, path TEXT NOT NULL, at INTEGER NOT NULL, taken INTEGER NOT NULL DEFAULT 0, label TEXT NOT NULL DEFAULT '', data BLOB NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS versions_path ON versions(path)`,
		`CREATE INDEX IF NOT EXISTS versions_taken ON versions(taken)`, // the prune of old copies, at each one kept (#200)
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	os.Chmod(file, 0o600)                                                     // the file holds the settings, and in them the keys and tokens: for its owner alone
	db.Exec(`ALTER TABLE versions ADD COLUMN label TEXT NOT NULL DEFAULT ''`) // a file from before names: the column is added (an error when it is there)
	s := &Store{DB: db, File: file}
	if _, err := db.Exec(`INSERT OR IGNORE INTO nodes (path, parent, dir, data, mtime) VALUES (?, '', 1, NULL, ?)`, StoreRoot, time.Now().UnixNano()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// WriteError is nil for a library file: its writes say at once when they fail. The browser's store writes later,
// and keeps what its last write said (store_js.go).
func (s *Store) WriteError() error { return nil }

func (s *Store) Close() error { return s.DB.Close() }

// Meta is the small thing kept under key, nil when there is none.
func (s *Store) Meta(key string) []byte {
	var v []byte
	if s.DB.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v) != nil {
		return nil
	}
	return v
}

func (s *Store) SetMeta(key string, value []byte) error {
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
	_, err := s.DB.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// DeleteMeta takes away what is kept under key.
func (s *Store) DeleteMeta(key string) error {
	s.Snap.Store(nil)
	_, err := s.DB.Exec(`DELETE FROM meta WHERE key = ?`, key)
	return err
}

func (s *Store) node(name string) (dir bool, data []byte, mtime time.Time, err error) {
	if sn := s.Snap.Load(); sn != nil {
		n, ok := sn.nodes[Clean(name)]
		if !ok {
			return false, nil, time.Time{}, pathErr("open", name, fs.ErrNotExist)
		}
		return n.dir, n.data, n.mtime, nil
	}
	var ns int64
	var d int
	err = s.DB.QueryRow(`SELECT dir, data, mtime FROM nodes WHERE path = ?`, Clean(name)).Scan(&d, &data, &ns)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, time.Time{}, pathErr("open", name, fs.ErrNotExist)
	}
	return d == 1, data, time.Unix(0, ns), err
}

// kind says whether name is there and whether it is a folder, without reading its data: the checks of a write read
// a page's whole blob for its kind, and a save read it three times (#200).
func (s *Store) kind(name string) (dir, ok bool) {
	if sn := s.Snap.Load(); sn != nil {
		n, there := sn.nodes[Clean(name)]
		return there && n.dir, there
	}
	var d int
	if err := s.DB.QueryRow(`SELECT dir FROM nodes WHERE path = ?`, Clean(name)).Scan(&d); err != nil {
		return false, false
	}
	return d == 1, true
}

func (s *Store) ReadFile(name string) ([]byte, error) {
	dir, data, _, err := s.node(name)
	if err != nil {
		return nil, err
	}
	if dir {
		return nil, pathErr("read", name, errors.New("is a directory"))
	}
	return append([]byte{}, data...), nil
}

func (s *Store) WriteFile(name string, data []byte) error {
	s.wrote()
	n := Clean(name)
	pdir, there := s.kind(path.Dir(n))
	if !there {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if !pdir {
		return pathErr("write", name, errors.New("not a directory"))
	}
	if d, _ := s.kind(n); d {
		return pathErr("write", name, errors.New("is a directory"))
	}
	if data == nil {
		data = []byte{}
	}
	if path.Base(n) == PageFileName {
		s.snapshotBefore(n, data)
	}
	// the checks above again, in the statement that writes: the sync's goroutine uses the same store, and a folder
	// taken away between them would leave a row with no folder (#143)
	r, err := s.DB.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime)
		SELECT ?1, ?2, 0, ?3, ?4 WHERE EXISTS (SELECT 1 FROM nodes WHERE path = ?2 AND dir = 1)
			AND NOT EXISTS (SELECT 1 FROM nodes WHERE path = ?1 AND dir = 1)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, mtime = excluded.mtime`, n, path.Dir(n), data, time.Now().UnixNano())
	if err != nil {
		return err
	}
	if k, err := r.RowsAffected(); err == nil && k == 0 {
		return pathErr("write", name, fs.ErrNotExist)
	}
	return nil
}

func (s *Store) Mkdir(name string) error {
	s.wrote()
	n := Clean(name)
	if _, _, _, err := s.node(n); err == nil {
		return pathErr("mkdir", name, fs.ErrExist)
	}
	if d, _, _, err := s.node(path.Dir(n)); err != nil || !d {
		return pathErr("mkdir", name, fs.ErrNotExist)
	}
	_, err := s.DB.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime) VALUES (?, ?, 1, NULL, ?)`, n, path.Dir(n), time.Now().UnixNano())
	return err
}

func (s *Store) MkdirAll(name string) error {
	if sn := s.Snap.Load(); sn != nil {
		if n, ok := sn.nodes[Clean(name)]; ok && n.dir {
			return nil // there: nothing is written
		}
	}
	s.wrote()
	n := Clean(name)
	var chain []string
	for p := n; p != "/"; p = path.Dir(p) {
		chain = append(chain, p)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		d, _, _, err := s.node(chain[i])
		if err == nil {
			if !d {
				return pathErr("mkdir", name, errors.New("not a directory"))
			}
			continue
		}
		if err := s.Mkdir(chain[i]); err != nil {
			return err
		}
	}
	return nil
}

// below is the paths under dir, deepest first.
// underSQL is the paths of ?1 and under it, as a range of the key: "/" sorts just before "0", so what starts with
// ?1 + "/" lies between the two. A substr of the path read every row of the table for every move, delete and
// trash (#200).
const underSQL = `(path = ?1 OR (path >= ?1 || '/' AND path < ?1 || '0'))`

func (s *Store) Rename(oldName, newName string) error {
	s.wrote()
	from, to := Clean(oldName), Clean(newName)
	if _, there := s.kind(from); !there || from == "/" {
		return pathErr("rename", oldName, fs.ErrNotExist)
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to, from+"/") {
		return pathErr("rename", oldName, errors.New("into itself"))
	}
	if _, there := s.kind(to); there {
		return pathErr("rename", newName, fs.ErrExist)
	}
	if d, _ := s.kind(path.Dir(to)); !d {
		return pathErr("rename", newName, fs.ErrNotExist)
	}
	tx, commit, rollback, err := s.begin()
	if err != nil {
		return err
	}
	defer rollback()
	// the checks above again, in the transaction: nothing else writes while it holds the one connection (#143)
	var ok int
	if err := tx.QueryRow(`SELECT (SELECT count(*) FROM nodes WHERE path = ?1) = 1
		AND (SELECT count(*) FROM nodes WHERE path = ?2) = 0
		AND (SELECT count(*) FROM nodes WHERE path = ?3 AND dir = 1) = 1`, from, to, path.Dir(to)).Scan(&ok); err != nil {
		return err
	}
	if ok != 1 {
		return pathErr("rename", oldName, errors.New("changed while it was moved"))
	}
	// one statement a table, the tree under it with it: the new names lie outside the old range, so none is met twice
	if _, err := tx.Exec(`UPDATE nodes SET path = ?2 || substr(path, length(?1) + 1),
		parent = CASE WHEN path = ?1 THEN ?3 ELSE ?2 || substr(parent, length(?1) + 1) END WHERE `+underSQL, from, to, path.Dir(to)); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE versions SET path = ?2 || substr(path, length(?1) + 1) WHERE `+underSQL, from, to); err != nil {
		return err
	}
	return commit()
}

func (s *Store) Remove(name string) error {
	s.wrote()
	n := Clean(name)
	d, _, _, err := s.node(n)
	if err != nil {
		return pathErr("remove", name, fs.ErrNotExist)
	}
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	if d {
		var kids int
		s.DB.QueryRow(`SELECT count(*) FROM nodes WHERE parent = ?`, n).Scan(&kids)
		if kids > 0 {
			return pathErr("remove", name, errors.New("directory not empty"))
		}
	}
	if _, err = s.DB.Exec(`DELETE FROM nodes WHERE path = ?`, n); err != nil {
		return err
	}
	// its copies go with it: a page made later at the same place would show them as its own (#139)
	_, err = s.DB.Exec(`DELETE FROM versions WHERE path = ?`, n)
	return err
}

func (s *Store) RemoveAll(name string) error {
	s.wrote()
	n := Clean(name)
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	tx, commit, rollback, err := s.begin()
	if err != nil {
		return err
	}
	defer rollback()
	if _, err := tx.Exec(`DELETE FROM nodes WHERE `+underSQL, n); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM versions WHERE `+underSQL, n); err != nil {
		return err
	}
	return commit()
}

func (s *Store) Stat(name string) (fs.FileInfo, error) {
	n := Clean(name)
	if sn := s.Snap.Load(); sn != nil {
		x, ok := sn.nodes[n]
		if !ok {
			return nil, pathErr("stat", name, fs.ErrNotExist)
		}
		return info{name: path.Base(n), size: int64(len(x.data)), dir: x.dir, mtime: x.mtime}, nil
	}
	var ns, size int64
	var d int
	err := s.DB.QueryRow(`SELECT dir, coalesce(length(data), 0), mtime FROM nodes WHERE path = ?`, n).Scan(&d, &size, &ns) // a folder's data is NULL
	if err != nil {
		return nil, pathErr("stat", name, fs.ErrNotExist)
	}
	return info{name: path.Base(n), size: size, dir: d == 1, mtime: time.Unix(0, ns)}, nil
}

func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) {
	n := Clean(name)
	if d, _, _, err := s.node(n); err != nil || !d {
		return nil, pathErr("readdir", name, fs.ErrNotExist)
	}
	if sn := s.Snap.Load(); sn != nil {
		var out []fs.DirEntry
		for _, p := range sn.kids[n] {
			x := sn.nodes[p]
			out = append(out, info{name: path.Base(p), size: int64(len(x.data)), dir: x.dir, mtime: x.mtime})
		}
		return out, nil
	}
	rows, err := s.DB.Query(`SELECT path, dir, coalesce(length(data), 0), mtime FROM nodes WHERE parent = ? ORDER BY path`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []fs.DirEntry
	for rows.Next() {
		var p string
		var d int
		var size, ns int64
		if err := rows.Scan(&p, &d, &size, &ns); err != nil {
			return nil, err
		}
		out = append(out, info{name: path.Base(p), size: size, dir: d == 1, mtime: time.Unix(0, ns)})
	}
	return out, rows.Err()
}

// Fast turns the wait for each write to the disk off, for a copy of what is still there, and on again.
func (s *Store) Fast(on bool) {
	if on {
		s.DB.Exec(`PRAGMA synchronous = OFF`)
	} else {
		s.DB.Exec(`PRAGMA synchronous = FULL`)
	}
}

// ReadNamed reads, in one pass over the table, every file whose name ends in "/"+base: by its name.
// Opening a library of thousands of pages reads them all at once, not a query each.
func (s *Store) ReadNamed(base string) (map[string][]byte, error) {
	rows, err := s.DB.Query(`SELECT path, data FROM nodes WHERE dir = 0 AND path LIKE ?`, "%/"+base)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var p string
		var data []byte
		if err := rows.Scan(&p, &data); err != nil {
			return nil, err
		}
		out[p] = data
	}
	return out, rows.Err()
}

// storeSnap is the whole table, read in one pass, for opening the library: the folders and pages of a
// library of thousands are read without a query each.
type storeSnap struct {
	nodes map[string]snapNode
	kids  map[string][]string // by parent, in order
}

type snapNode struct {
	dir   bool
	data  []byte
	mtime time.Time
}

// BeginSnapshot reads the table, for what reads it next to be answered from it, until endSnapshot or a write.
func (s *Store) BeginSnapshot() {
	rows, err := s.DB.Query(`SELECT path, parent, dir, data, mtime FROM nodes ORDER BY path`)
	if err != nil {
		return
	}
	defer rows.Close()
	sn := &storeSnap{nodes: map[string]snapNode{}, kids: map[string][]string{}}
	for rows.Next() {
		var p, parent string
		var d int
		var data []byte
		var ns int64
		if rows.Scan(&p, &parent, &d, &data, &ns) != nil {
			return
		}
		sn.nodes[p] = snapNode{dir: d == 1, data: data, mtime: time.Unix(0, ns)}
		if p != parent {
			sn.kids[parent] = append(sn.kids[parent], p)
		}
	}
	if rows.Err() == nil {
		s.Snap.Store(sn)
	}
}

func (s *Store) EndSnapshot() { s.Snap.Store(nil) }

// Batch runs fn as one transaction: the store has one connection, so what fn writes through the store is in it, and
// is kept whole or not at all. Rename and RemoveAll, which make transactions of their own, run inside it.
func (s *Store) Batch(fn func() error) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	if _, err := s.DB.Exec(`BEGIN IMMEDIATE`); err != nil {
		return err
	}
	s.batching.Store(true)
	err := fn()
	s.batching.Store(false)
	if err != nil {
		s.DB.Exec(`ROLLBACK`)
		s.wrote()
		return err
	}
	_, err = s.DB.Exec(`COMMIT`)
	return err
}

// execer is a transaction, or the store's connection inside a Batch.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// begin starts a transaction, or, inside a Batch, goes on in the Batch's: done commits it (or does nothing).
func (s *Store) begin() (x execer, done func() error, undo func(), err error) {
	if s.batching.Load() {
		return s.DB, func() error { return nil }, func() {}, nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, nil, nil, err
	}
	return tx, tx.Commit, func() { tx.Rollback() }, nil
}
