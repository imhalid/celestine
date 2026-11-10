//go:build !js

package library

import (
	"bytes"
	"time"
)

func (s *Store) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// snapshotBefore keeps what the page file at n holds, before data is
// written over it, unless it is the same or a copy was kept lately.
func (s *Store) snapshotBefore(n string, data []byte) {
	old, mtime := s.oldFile(n)
	if old == nil || bytes.Equal(old, data) || blankBody(old) {
		return
	}
	if _, ob := SplitPage(old); ob != "" { // only the meta changed (an order, a rank): the text is kept already (#138)
		if _, nb := SplitPage(data); nb == ob {
			return
		}
	}
	var taken int64
	s.DB.QueryRow(`SELECT coalesce(max(taken), 0) FROM versions WHERE path = ?`, n).Scan(&taken)
	if taken != 0 && s.clock().Sub(time.Unix(0, taken)) < historyEvery {
		return // a copy of this stretch of writing is kept already
	}
	s.keep(n, old, mtime)
}

// oldFile is the file at n, and when it was written; nil when there is none.
func (s *Store) oldFile(n string) ([]byte, time.Time) {
	d, data, mt, err := s.node(n)
	if err != nil || d {
		return nil, time.Time{}
	}
	return data, mt
}

// maxVersions is how many unnamed copies of one page are kept, at most.
const maxVersions = 200

// keep stores data as a version of n written at at, and lets the copies of
// every page taken more than HistoryDays ago go, but the named ones.
func (s *Store) keep(n string, data []byte, at time.Time) {
	now := s.clock()
	s.DB.Exec(`INSERT INTO versions (path, at, taken, data) VALUES (?, ?, ?, ?)`, n, at.UnixNano(), now.UnixNano(), data)
	s.DB.Exec(`DELETE FROM versions WHERE label = '' AND taken < ?`, now.AddDate(0, 0, -HistoryDays).UnixNano())
	// and no more than maxVersions of a page, the oldest unnamed going first (#139)
	s.DB.Exec(`DELETE FROM versions WHERE path = ?1 AND label = '' AND id NOT IN
		(SELECT id FROM versions WHERE path = ?1 AND label = '' ORDER BY taken DESC LIMIT ?2)`, n, maxVersions)
}

// SnapshotNow keeps the file at name as it is, whatever was kept lately: as
// a page is about to be put back to an earlier text.
func (s *Store) SnapshotNow(name string) {
	n := Clean(name)
	if old, mt := s.oldFile(n); old != nil && !blankBody(old) {
		var last []byte
		s.DB.QueryRow(`SELECT data FROM versions WHERE path = ? ORDER BY id DESC LIMIT 1`, n).Scan(&last)
		if !bytes.Equal(last, old) {
			s.keep(n, old, mt)
		}
	}
}

// SaveNamed keeps the file at name as it is now, under a name of the user's:
// it is kept whatever was kept lately, and never goes to make room.
func (s *Store) SaveNamed(name, label string) bool {
	n := Clean(name)
	old, _ := s.oldFile(n)
	if old == nil {
		return false
	}
	s.DB.Exec(`INSERT INTO versions (path, at, taken, label, data) VALUES (?, ?, ?, ?, ?)`, n, s.clock().UnixNano(), s.clock().UnixNano(), label, old)
	return true
}

// Versions are the copies kept of the file at name, the latest first.
func (s *Store) Versions(name string) []Version {
	rows, err := s.DB.Query(`SELECT id, at, length(data), label FROM versions WHERE path = ? ORDER BY id DESC`, Clean(name))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		var at int64
		if rows.Scan(&v.ID, &at, &v.Size, &v.Label) == nil {
			v.At = time.Unix(0, at)
			out = append(out, v)
		}
	}
	return out
}

// VersionData is what version id of the file at name held.
func (s *Store) VersionData(name string, id int64) ([]byte, bool) {
	var data []byte
	if s.DB.QueryRow(`SELECT data FROM versions WHERE path = ? AND id = ?`, Clean(name), id).Scan(&data) != nil {
		return nil, false
	}
	return data, true
}
