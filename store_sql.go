//go:build !js

package main

import (
	"database/sql"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a library in one SQLite file: a table of nodes, folders and files
// by path, and a table of small things kept by name (the settings).
type Store struct {
	db   *sql.DB
	File string
	now  func() time.Time // the clock of the history; the system's when nil
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
	db.Exec(`ALTER TABLE versions ADD COLUMN label TEXT NOT NULL DEFAULT ''`) // a file from before names: the column is added (an error when it is there)
	s := &Store{db: db, File: file}
	if _, err := db.Exec(`INSERT OR IGNORE INTO nodes (path, parent, dir, data, mtime) VALUES (?, '', 1, NULL, ?)`, storeRoot, time.Now().UnixNano()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Meta is the small thing kept under key, nil when there is none.
func (s *Store) Meta(key string) []byte {
	var v []byte
	if s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v) != nil {
		return nil
	}
	return v
}

func (s *Store) SetMeta(key string, value []byte) error {
	_, err := s.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *Store) node(name string) (dir bool, data []byte, mtime time.Time, err error) {
	var ns int64
	var d int
	err = s.db.QueryRow(`SELECT dir, data, mtime FROM nodes WHERE path = ?`, clean(name)).Scan(&d, &data, &ns)
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
	n := clean(name)
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
	if path.Base(n) == pageFileName {
		s.snapshotBefore(n, data)
	}
	_, err = s.db.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime) VALUES (?, ?, 0, ?, ?)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, mtime = excluded.mtime`, n, path.Dir(n), data, time.Now().UnixNano())
	return err
}

func (s *Store) Mkdir(name string) error {
	n := clean(name)
	if _, _, _, err := s.node(n); err == nil {
		return pathErr("mkdir", name, fs.ErrExist)
	}
	if d, _, _, err := s.node(path.Dir(n)); err != nil || !d {
		return pathErr("mkdir", name, fs.ErrNotExist)
	}
	_, err := s.db.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime) VALUES (?, ?, 1, NULL, ?)`, n, path.Dir(n), time.Now().UnixNano())
	return err
}

func (s *Store) MkdirAll(name string) error {
	n := clean(name)
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
	rows, err := s.db.Query(`SELECT path FROM nodes WHERE path = ?1 OR substr(path, 1, length(?1) + 1) = ?1 || '/'`, dir)
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
	from, to := clean(oldName), clean(newName)
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
	tx, err := s.db.Begin()
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
	n := clean(name)
	d, _, _, err := s.node(n)
	if err != nil {
		return pathErr("remove", name, fs.ErrNotExist)
	}
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	if d {
		var kids int
		s.db.QueryRow(`SELECT count(*) FROM nodes WHERE parent = ?`, n).Scan(&kids)
		if kids > 0 {
			return pathErr("remove", name, errors.New("directory not empty"))
		}
	}
	_, err = s.db.Exec(`DELETE FROM nodes WHERE path = ?`, n)
	return err
}

func (s *Store) RemoveAll(name string) error {
	n := clean(name)
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	all, err := s.below(n)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
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
	n := clean(name)
	var ns, size int64
	var d int
	err := s.db.QueryRow(`SELECT dir, length(data), mtime FROM nodes WHERE path = ?`, n).Scan(&d, &size, &ns)
	if err != nil {
		return nil, pathErr("stat", name, fs.ErrNotExist)
	}
	return info{name: path.Base(n), size: size, dir: d == 1, mtime: time.Unix(0, ns)}, nil
}

func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) {
	n := clean(name)
	if d, _, _, err := s.node(n); err != nil || !d {
		return nil, pathErr("readdir", name, fs.ErrNotExist)
	}
	rows, err := s.db.Query(`SELECT path, dir, coalesce(length(data), 0), mtime FROM nodes WHERE parent = ? ORDER BY path`, n)
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

// fast turns the wait for each write to the disk off, for a copy of what is still there, and on again.
func (s *Store) fast(on bool) {
	if on {
		s.db.Exec(`PRAGMA synchronous = OFF`)
	} else {
		s.db.Exec(`PRAGMA synchronous = FULL`)
	}
}
