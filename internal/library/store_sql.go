//go:build !js

package library

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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
}

// OpenStore opens the library file at file, making it when there is none.
func OpenStore(file string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(file)+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(0)")
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
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
	n := Clean(name)
	pdir, _, _, err := s.node(path.Dir(n))
	if err != nil {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if !pdir {
		return pathErr("write", name, errors.New("not a directory"))
	}
	if d, _, _, err := s.node(n); err == nil && d {
		return pathErr("write", name, errors.New("is a directory"))
	}
	if data == nil {
		data = []byte{}
	}
	if path.Base(n) == PageFileName {
		s.snapshotBefore(n, data)
	}
	_, err = s.DB.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime) VALUES (?, ?, 0, ?, ?)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, mtime = excluded.mtime`, n, path.Dir(n), data, time.Now().UnixNano())
	return err
}

func (s *Store) Mkdir(name string) error {
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
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
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
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
func (s *Store) below(dir string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT path FROM nodes WHERE path = ?1 OR substr(path, 1, length(?1) + 1) = ?1 || '/'`, dir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out, rows.Err()
}

func (s *Store) Rename(oldName, newName string) error {
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
	from, to := Clean(oldName), Clean(newName)
	if _, _, _, err := s.node(from); err != nil {
		return pathErr("rename", oldName, fs.ErrNotExist)
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to, from+"/") {
		return pathErr("rename", oldName, errors.New("into itself"))
	}
	if _, _, _, err := s.node(to); err == nil {
		return pathErr("rename", newName, fs.ErrExist)
	}
	if d, _, _, err := s.node(path.Dir(to)); err != nil || !d {
		return pathErr("rename", newName, fs.ErrNotExist)
	}
	all, err := s.below(from)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range all {
		np := to + strings.TrimPrefix(p, from)
		if _, err := tx.Exec(`UPDATE nodes SET path = ?, parent = ? WHERE path = ?`, np, path.Dir(np), p); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE versions SET path = ? WHERE path = ?`, np, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Remove(name string) error {
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
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
	_, err = s.DB.Exec(`DELETE FROM nodes WHERE path = ?`, n)
	return err
}

func (s *Store) RemoveAll(name string) error {
	s.Snap.Store(nil) // a write: what was read at once is no more what is there
	n := Clean(name)
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	all, err := s.below(n)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range all {
		if _, err := tx.Exec(`DELETE FROM nodes WHERE path = ?`, p); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM versions WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	err := s.DB.QueryRow(`SELECT dir, length(data), mtime FROM nodes WHERE path = ?`, n).Scan(&d, &size, &ns)
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
