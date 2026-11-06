package main

import (
	"bytes"
	"strings"
	"time"
)

// Version history: a library kept in a file keeps, for each page, what the
// page was - a copy of its file each time it is written over after a few
// minutes of quiet from the last copy - so that an earlier text can be seen,
// set against the present, and put back. A folder library keeps none.

// historyEvery is how far apart the copies of a page are, at the least.
var historyEvery = 5 * time.Minute

// historyKeep is how many copies of a page are kept, the oldest going first.
const historyKeep = 100

// Version is a copy of a page.
type Version struct {
	ID    int64
	At    time.Time // when that text was written
	Size  int
	Label string // a name one gave it: such a copy is kept for good
}

func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
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
	var taken int64
	s.db.QueryRow(`SELECT coalesce(max(taken), 0) FROM versions WHERE path = ?`, n).Scan(&taken)
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

// keep stores data as a version of n written at at, and lets the oldest go
// past historyKeep.
func (s *Store) keep(n string, data []byte, at time.Time) {
	s.db.Exec(`INSERT INTO versions (path, at, taken, data) VALUES (?, ?, ?, ?)`, n, at.UnixNano(), s.clock().UnixNano(), data)
	s.db.Exec(`DELETE FROM versions WHERE path = ?1 AND label = '' AND id NOT IN (SELECT id FROM versions WHERE path = ?1 AND label = '' ORDER BY id DESC LIMIT ?2)`, n, historyKeep)
}

// SnapshotNow keeps the file at name as it is, whatever was kept lately: as
// a page is about to be put back to an earlier text.
func (s *Store) SnapshotNow(name string) {
	n := clean(name)
	if old, mt := s.oldFile(n); old != nil && !blankBody(old) {
		var last []byte
		s.db.QueryRow(`SELECT data FROM versions WHERE path = ? ORDER BY id DESC LIMIT 1`, n).Scan(&last)
		if !bytes.Equal(last, old) {
			s.keep(n, old, mt)
		}
	}
}

// SaveNamed keeps the file at name as it is now, under a name of the user's:
// it is kept whatever was kept lately, and never goes to make room.
func (s *Store) SaveNamed(name, label string) bool {
	n := clean(name)
	old, _ := s.oldFile(n)
	if old == nil {
		return false
	}
	s.db.Exec(`INSERT INTO versions (path, at, taken, label, data) VALUES (?, ?, ?, ?, ?)`, n, s.clock().UnixNano(), s.clock().UnixNano(), label, old)
	return true
}

// Versions are the copies kept of the file at name, the latest first.
func (s *Store) Versions(name string) []Version {
	rows, err := s.db.Query(`SELECT id, at, length(data), label FROM versions WHERE path = ? ORDER BY id DESC`, clean(name))
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
	if s.db.QueryRow(`SELECT data FROM versions WHERE path = ? AND id = ?`, clean(name), id).Scan(&data) != nil {
		return nil, false
	}
	return data, true
}

// blankBody reports whether a page file holds no text and no object.
func blankBody(src []byte) bool {
	_, body := splitPage(src)
	return blankText(body)
}

// historian is a ground that keeps versions of its pages.
type historian interface {
	Versions(name string) []Version
	VersionData(name string, id int64) ([]byte, bool)
	SnapshotNow(name string)
	SaveNamed(name, label string) bool
}

// ---- the difference between a version and the present ----

// diffOp is a line of a difference: both have it, only the version has it
// (it comes back), or only the present (it goes).
type diffOp struct {
	kind  byte // '=', '+', '-'
	text  string
	words []diffOp // set where a line that comes back and one that goes are of a pair: the words of this line, those that differ marked
}

// pairWords sets, for the lines that go and come back side by side, which of
// their words are what differs: the first line that comes back with the first
// that goes, and so on.
func pairWords(ops []diffOp) {
	for i := 0; i < len(ops); {
		if ops[i].kind == '=' {
			i++
			continue
		}
		j := i
		var adds, dels []int
		for j < len(ops) && ops[j].kind != '=' {
			if ops[j].kind == '+' {
				adds = append(adds, j)
			} else {
				dels = append(dels, j)
			}
			j++
		}
		for k := 0; k < len(adds) && k < len(dels); k++ {
			a, d := adds[k], dels[k]
			w := diffLines(strings.Fields(ops[a].text), strings.Fields(ops[d].text))
			var mine, theirs []diffOp
			for _, o := range w {
				switch o.kind {
				case '=':
					mine, theirs = append(mine, o), append(theirs, o)
				case '+':
					mine = append(mine, o)
				default:
					theirs = append(theirs, o)
				}
			}
			ops[a].words, ops[d].words = mine, theirs
		}
		i = j
	}
}

// diffLines sets the lines of the version against those of the present: the
// longest run they share, the rest marked. Long texts are compared by their
// first lines only.
func diffLines(version, now []string) []diffOp {
	const cap = 1500
	if len(version) > cap {
		version = version[:cap]
	}
	if len(now) > cap {
		now = now[:cap]
	}
	n, m := len(version), len(now)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if version[i] == now[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case version[i] == now[j]:
			out = append(out, diffOp{kind: '=', text: version[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, diffOp{kind: '+', text: version[i]})
			i++
		default:
			out = append(out, diffOp{kind: '-', text: now[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffOp{kind: '+', text: version[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffOp{kind: '-', text: now[j]})
	}
	return out
}
