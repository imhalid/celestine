//go:build !js

package library

import (
	"context"
	"database/sql"
	"encoding/json"
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

	batchMu sync.Mutex // one Batch at a time

	readOnly bool // opened by OpenStoreReadOnly: the derived page_index is neither written nor pruned (#594)
}

// WriteCount returns the count of writes made to the store so far.
func (s *Store) WriteCount() uint64 { return s.Writes.Load() }

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
		`CREATE TABLE IF NOT EXISTS page_index (
			path  TEXT PRIMARY KEY,
			mtime INTEGER NOT NULL,
			size  INTEGER NOT NULL,
			data  BLOB NOT NULL
		) WITHOUT ROWID`,
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

// OpenStoreReadOnly opens an existing SQLite library file read-only without creating tables or changing file mode (#165).
func OpenStoreReadOnly(file string) (*Store, error) {
	db, err := sql.Open("sqlite", sqliteURI(file, "mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(0)"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var ok int
	if err := db.QueryRow(`SELECT 1`).Scan(&ok); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, File: file, readOnly: true}, nil
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

func (s *Store) node(q querier, name string) (dir bool, data []byte, mtime time.Time, err error) {
	if sn := s.Snap.Load(); sn != nil {
		n, ok := sn.nodes[Clean(name)]
		if !ok {
			return false, nil, time.Time{}, pathErr("open", name, fs.ErrNotExist)
		}
		if n.dir || n.held {
			return n.dir, n.data, n.mtime, nil
		}
		// a file the snapshot did not read (a picture, an attachment): from the table
	}
	var ns int64
	var d int
	err = q.QueryRow(`SELECT dir, data, mtime FROM nodes WHERE path = ?`, Clean(name)).Scan(&d, &data, &ns)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, time.Time{}, pathErr("open", name, fs.ErrNotExist)
	}
	return d == 1, data, time.Unix(0, ns), err
}

// kind says whether name is there and whether it is a folder, without reading its data: the checks of a write read
// a page's whole blob for its kind, and a save read it three times (#200).
func (s *Store) kind(q querier, name string) (dir, ok bool) {
	if sn := s.Snap.Load(); sn != nil {
		n, there := sn.nodes[Clean(name)]
		return there && n.dir, there
	}
	var d int
	if err := q.QueryRow(`SELECT dir FROM nodes WHERE path = ?`, Clean(name)).Scan(&d); err != nil {
		return false, false
	}
	return d == 1, true
}

func (s *Store) ReadFile(name string) ([]byte, error) { return s.readFile(s.DB, name) }

func (s *Store) readFile(q querier, name string) ([]byte, error) {
	dir, data, _, err := s.node(q, name)
	if err != nil {
		return nil, err
	}
	if dir {
		return nil, pathErr("read", name, errors.New("is a directory"))
	}
	return append([]byte{}, data...), nil
}

func (s *Store) WriteFile(name string, data []byte) error { return s.writeFile(s.DB, name, data) }

func (s *Store) writeFile(q querier, name string, data []byte) error {
	s.wrote()
	n := Clean(name)
	pdir, there := s.kind(q, path.Dir(n))
	if !there {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if !pdir {
		return pathErr("write", name, errors.New("not a directory"))
	}
	if d, _ := s.kind(q, n); d {
		return pathErr("write", name, errors.New("is a directory"))
	}
	if data == nil {
		data = []byte{}
	}
	if path.Base(n) == PageFileName {
		s.snapshotBefore(q, n, data)
	}
	// the checks above again, in the statement that writes: the sync's goroutine uses the same store, and a folder
	// taken away between them would leave a row with no folder (#143)
	r, err := q.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime)
		SELECT ?1, ?2, 0, ?3, ?4 WHERE EXISTS (SELECT 1 FROM nodes WHERE path = ?2 AND dir = 1)
			AND NOT EXISTS (SELECT 1 FROM nodes WHERE path = ?1 AND dir = 1)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, mtime = excluded.mtime`, n, path.Dir(n), data, time.Now().UnixNano())
	if err != nil {
		return err
	}
	if k, err := r.RowsAffected(); err == nil && k == 0 {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if path.Base(n) == PageFileName {
		_, _ = q.Exec(`DELETE FROM page_index WHERE path = ?`, n)
	}
	return nil
}

// WriteFileKeepTime writes data over the file name and leaves it the time it was last written (TimeKeeper). It keeps
// no version of what was there: what is written so is not the page's text.
func (s *Store) WriteFileKeepTime(name string, data []byte) error {
	return s.writeFileKeepTime(s.DB, name, data)
}

func (s *Store) writeFileKeepTime(q querier, name string, data []byte) error {
	s.wrote()
	if data == nil {
		data = []byte{}
	}
	n := Clean(name)
	r, err := q.Exec(`UPDATE nodes SET data = ? WHERE path = ? AND dir = 0`, data, n)
	if err != nil {
		return err
	}
	if k, err := r.RowsAffected(); err == nil && k == 0 {
		return pathErr("write", name, fs.ErrNotExist)
	}
	if path.Base(n) == PageFileName {
		_, _ = q.Exec(`DELETE FROM page_index WHERE path = ?`, n)
	}
	return nil
}

func (s *Store) Mkdir(name string) error { return s.mkdir(s.DB, name) }

func (s *Store) mkdir(q querier, name string) error {
	s.wrote()
	n := Clean(name)
	if _, _, _, err := s.node(q, n); err == nil {
		return pathErr("mkdir", name, fs.ErrExist)
	}
	if d, _, _, err := s.node(q, path.Dir(n)); err != nil || !d {
		return pathErr("mkdir", name, fs.ErrNotExist)
	}
	_, err := q.Exec(`INSERT INTO nodes (path, parent, dir, data, mtime) VALUES (?, ?, 1, NULL, ?)`, n, path.Dir(n), time.Now().UnixNano())
	return err
}

func (s *Store) MkdirAll(name string) error { return s.mkdirAll(s.DB, name) }

func (s *Store) mkdirAll(q querier, name string) error {
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
		d, _, _, err := s.node(q, chain[i])
		if err == nil {
			if !d {
				return pathErr("mkdir", name, errors.New("not a directory"))
			}
			continue
		}
		if err := s.mkdir(q, chain[i]); err != nil {
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
	return s.rename(s.DB, false, oldName, newName)
}

func (s *Store) rename(q querier, inTx bool, oldName, newName string) error {
	s.wrote()
	from, to := Clean(oldName), Clean(newName)
	if _, there := s.kind(q, from); !there || from == "/" {
		return pathErr("rename", oldName, fs.ErrNotExist)
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to, from+"/") {
		return pathErr("rename", oldName, errors.New("into itself"))
	}
	if _, there := s.kind(q, to); there {
		return pathErr("rename", newName, fs.ErrExist)
	}
	if d, _ := s.kind(q, path.Dir(to)); !d {
		return pathErr("rename", newName, fs.ErrNotExist)
	}
	tx, commit, rollback, err := s.begin(q, inTx)
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
	if _, err := tx.Exec(`UPDATE page_index SET path = ?2 || substr(path, length(?1) + 1) WHERE `+underSQL, from, to); err != nil {
		return err
	}
	return commit()
}

func (s *Store) Remove(name string) error { return s.remove(s.DB, name) }

func (s *Store) remove(q querier, name string) error {
	s.wrote()
	n := Clean(name)
	d, _, _, err := s.node(q, n)
	if err != nil {
		return pathErr("remove", name, fs.ErrNotExist)
	}
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	if d {
		var kids int
		q.QueryRow(`SELECT count(*) FROM nodes WHERE parent = ?`, n).Scan(&kids)
		if kids > 0 {
			return pathErr("remove", name, errors.New("directory not empty"))
		}
	}
	if _, err = q.Exec(`DELETE FROM nodes WHERE path = ?`, n); err != nil {
		return err
	}
	// its copies and cached index go with it: a page made later at the same place would show them as its own (#139)
	if _, err = q.Exec(`DELETE FROM versions WHERE path = ?`, n); err != nil {
		return err
	}
	_, _ = q.Exec(`DELETE FROM page_index WHERE path = ?`, n)
	return nil
}

func (s *Store) RemoveAll(name string) error { return s.removeAll(s.DB, false, name) }

func (s *Store) removeAll(q querier, inTx bool, name string) error {
	s.wrote()
	n := Clean(name)
	if n == "/" {
		return pathErr("remove", name, errors.New("the root"))
	}
	tx, commit, rollback, err := s.begin(q, inTx)
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
	_, _ = tx.Exec(`DELETE FROM page_index WHERE `+underSQL, n)
	return commit()
}

func (s *Store) Stat(name string) (fs.FileInfo, error) { return s.stat(s.DB, name) }

func (s *Store) stat(q querier, name string) (fs.FileInfo, error) {
	n := Clean(name)
	if sn := s.Snap.Load(); sn != nil {
		x, ok := sn.nodes[n]
		if !ok {
			return nil, pathErr("stat", name, fs.ErrNotExist)
		}
		return info{name: path.Base(n), size: x.size, dir: x.dir, mtime: x.mtime}, nil
	}
	var ns, size int64
	var d int
	err := q.QueryRow(`SELECT dir, coalesce(length(data), 0), mtime FROM nodes WHERE path = ?`, n).Scan(&d, &size, &ns) // a folder's data is NULL
	if err != nil {
		return nil, pathErr("stat", name, fs.ErrNotExist)
	}
	return info{name: path.Base(n), size: size, dir: d == 1, mtime: time.Unix(0, ns)}, nil
}

func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) { return s.readDir(s.DB, name) }

func (s *Store) readDir(q querier, name string) ([]fs.DirEntry, error) {
	n := Clean(name)
	if d, _, _, err := s.node(q, n); err != nil || !d {
		return nil, pathErr("readdir", name, fs.ErrNotExist)
	}
	if sn := s.Snap.Load(); sn != nil {
		var out []fs.DirEntry
		for _, p := range sn.kids[n] {
			x := sn.nodes[p]
			out = append(out, info{name: path.Base(p), size: x.size, dir: x.dir, mtime: x.mtime})
		}
		return out, nil
	}
	rows, err := q.Query(`SELECT path, dir, coalesce(length(data), 0), mtime FROM nodes WHERE parent = ? ORDER BY path`, n)
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

// ReadNamedFrom reads the files whose name ends in "/"+base and whose path comes after after, in the order of their
// paths, limit at most, by path, and says the last path read. Reading a library's pages in parts of a query each,
// not a query a page, holds only a part in memory at once, where reading them all held the library's every byte,
// and the store's one connection is let go between parts, for the window's saves (#196).
func (s *Store) ReadNamedFrom(base, after string, limit int) (map[string][]byte, string, error) {
	rows, err := s.DB.Query(`SELECT path, data FROM nodes WHERE dir = 0 AND path LIKE ? AND path > ? ORDER BY path LIMIT ?`, "%/"+base, after, limit)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	last := after
	for rows.Next() {
		var p string
		var data []byte
		if err := rows.Scan(&p, &data); err != nil {
			return nil, after, err
		}
		out[p], last = data, p
	}
	return out, last, rows.Err()
}

// ReadPrewarmFrom reads a page chunk alongside any up-to-date cached derived index rows (matching path, mtime,
// and byte length). When the cache matches, raw node bytes are omitted so cold-start prewarm skips re-parsing
// unchanged pages. On older read-only stores without page_index, it falls back to ReadNamedFrom.
func (s *Store) ReadPrewarmFrom(base, after string, limit int) (map[string]PrewarmRow, string, error) {
	rows, err := s.DB.Query(`SELECT n.path, n.mtime, coalesce(length(n.data), 0),
		CASE WHEN idx.mtime = n.mtime AND idx.size = coalesce(length(n.data), 0) THEN idx.data ELSE NULL END,
		CASE WHEN idx.mtime = n.mtime AND idx.size = coalesce(length(n.data), 0) THEN NULL ELSE n.data END
		FROM nodes n
		LEFT JOIN page_index idx ON idx.path = n.path
		WHERE n.dir = 0 AND n.path LIKE ? AND n.path > ?
		ORDER BY n.path LIMIT ?`, "%/"+base, after, limit)
	if err != nil {
		// Older read-only stores may not have a page_index table; fall back safely without mutating the DB.
		raw, last, rerr := s.ReadNamedFrom(base, after, limit)
		if rerr != nil {
			return nil, last, rerr
		}
		out := make(map[string]PrewarmRow, len(raw))
		for p, data := range raw {
			out[p] = PrewarmRow{Src: data}
		}
		return out, last, nil
	}
	defer rows.Close()
	out := map[string]PrewarmRow{}
	last := after
	for rows.Next() {
		var p string
		var r PrewarmRow
		if err := rows.Scan(&p, &r.MTime, &r.Size, &r.Index, &r.Src); err != nil {
			return nil, after, err
		}
		out[p], last = r, p
	}
	return out, last, rows.Err()
}

// WritePageIndexes persists computed derived page index rows in one transaction. Each insert verifies the node's
// current mtime and size so concurrent writes are never overwritten by stale index data.
func (s *Store) WritePageIndexes(entries []PageIndex) error {
	if len(entries) == 0 || s.readOnly {
		return nil
	}
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO page_index (path, mtime, size, data)
		SELECT ?1, ?2, ?3, ?4
		WHERE EXISTS (SELECT 1 FROM nodes WHERE path = ?1 AND mtime = ?2 AND coalesce(length(data), 0) = ?3)
		ON CONFLICT(path) DO UPDATE SET mtime = excluded.mtime, size = excluded.size, data = excluded.data`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range entries {
		blob, err := json.Marshal(e)
		if err != nil {
			continue
		}
		if _, err := stmt.Exec(Clean(e.Path), e.MTime, e.Size, blob); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PrunePageIndex deletes the page_index rows of paths that are no page in nodes: a row a manual edit of the file, or
// an older client that did not know the table, left behind when it removed or renamed a page (#594). A read-only
// store, and one from before the table, are left as they are. It reports how many rows it deleted.
func (s *Store) PrunePageIndex() (int64, error) {
	if s.readOnly {
		return 0, nil
	}
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	r, err := s.DB.Exec(`DELETE FROM page_index WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.path = page_index.path AND n.dir = 0)`)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
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
	size  int64
	held  bool // data was read: the JSON files, which opening the library reads, not the pictures (#197)
	mtime time.Time
}

// BeginSnapshot reads the table, for what reads it next to be answered from it, until endSnapshot or a write.
func (s *Store) BeginSnapshot() {
	rows, err := s.DB.Query(`SELECT path, parent, dir, CASE WHEN path LIKE '%.json' THEN data END, coalesce(length(data), 0), mtime FROM nodes ORDER BY path`)
	if err != nil {
		return
	}
	defer rows.Close()
	sn := &storeSnap{nodes: map[string]snapNode{}, kids: map[string][]string{}}
	for rows.Next() {
		var p, parent string
		var d int
		var data []byte
		var ns, size int64
		if rows.Scan(&p, &parent, &d, &data, &size, &ns) != nil {
			return
		}
		sn.nodes[p] = snapNode{dir: d == 1, data: data, size: size, held: strings.HasSuffix(p, ".json"), mtime: time.Unix(0, ns)}
		if p != parent {
			sn.kids[parent] = append(sn.kids[parent], p)
		}
	}
	if rows.Err() == nil {
		s.Snap.Store(sn)
	}
}

func (s *Store) EndSnapshot() { s.Snap.Store(nil) }

// Batch runs fn as one transaction on a connection of its own, passing a transaction-scoped FS (tx) whose reads
// and writes run inside that transaction. The store has one connection, so another goroutine calling methods on s
// (e.g. sync) waits for the Batch to end and is never folded into its rollback (#526).
func (s *Store) Batch(fn func(tx FS) error) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	ctx := context.Background()
	c, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	tx := &batchStore{s: s, q: &batchConn{c: c}}
	kept := false
	defer func() { // on a panic in fn too: the connection goes back to the others with no transaction open on it
		if !kept {
			c.ExecContext(ctx, `ROLLBACK`)
			s.wrote()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if _, err := c.ExecContext(ctx, `COMMIT`); err != nil {
		return err
	}
	kept = true
	return nil
}

// sealedBatch marks the store's Batch as one the others wait for (BatchSeals).
func (s *Store) sealedBatch() {}

// batchStore is the transaction-scoped FS passed to Batch callbacks.
type batchStore struct {
	s *Store
	q *batchConn
}

func (b *batchStore) ReadFile(name string) ([]byte, error)     { return b.s.readFile(b.q, name) }
func (b *batchStore) WriteFile(name string, data []byte) error { return b.s.writeFile(b.q, name, data) }
func (b *batchStore) WriteFileKeepTime(name string, data []byte) error {
	return b.s.writeFileKeepTime(b.q, name, data)
}
func (b *batchStore) Mkdir(name string) error    { return b.s.mkdir(b.q, name) }
func (b *batchStore) MkdirAll(name string) error { return b.s.mkdirAll(b.q, name) }
func (b *batchStore) Rename(oldName, newName string) error {
	return b.s.rename(b.q, true, oldName, newName)
}
func (b *batchStore) Remove(name string) error              { return b.s.remove(b.q, name) }
func (b *batchStore) RemoveAll(name string) error           { return b.s.removeAll(b.q, true, name) }
func (b *batchStore) Stat(name string) (fs.FileInfo, error) { return b.s.stat(b.q, name) }
func (b *batchStore) ReadDir(name string) ([]fs.DirEntry, error) {
	return b.s.readDir(b.q, name)
}
func (b *batchStore) Batch(fn func(tx FS) error) error { return fn(b) }
func (b *batchStore) sealedBatch()                     {}
func (b *batchStore) WriteCount() uint64               { return b.s.Writes.Load() }

// batchConn is the connection of an open Batch.
type batchConn struct {
	c *sql.Conn
}

func (b *batchConn) Exec(query string, args ...any) (sql.Result, error) {
	return b.c.ExecContext(context.Background(), query, args...)
}

func (b *batchConn) Query(query string, args ...any) (*sql.Rows, error) {
	return b.c.QueryContext(context.Background(), query, args...)
}

func (b *batchConn) QueryRow(query string, args ...any) *sql.Row {
	return b.c.QueryRowContext(context.Background(), query, args...)
}

// querier is what the store's statements run on: the pool, or a Batch's connection.
type querier interface {
	execer
	Query(query string, args ...any) (*sql.Rows, error)
}

// execer is a transaction, or a Batch's connection.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// begin starts a transaction, or reuses q directly when already inside a Batch transaction.
func (s *Store) begin(q querier, inTx bool) (x execer, done func() error, undo func(), err error) {
	if inTx {
		return q, func() error { return nil }, func() {}, nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, nil, nil, err
	}
	return tx, tx.Commit, func() { tx.Rollback() }, nil
}
