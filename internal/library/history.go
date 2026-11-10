package library

import (
	"strings"
	"time"
)

// Version history: a library kept in a file keeps, for each page, what the
// page was - a copy of its file each time it is written over after a few
// minutes of quiet from the last copy - so that an earlier text can be seen,
// set against the present, and put back. A folder library keeps none.

// historyEvery is how far apart the copies of a page are, at the least.
var historyEvery = 5 * time.Minute

// HistoryDays is how long a copy is kept: a week after it was taken, it
// goes, unless it has a name.
const HistoryDays = 7

// Version is a copy of a page.
type Version struct {
	ID    int64
	At    time.Time // when that text was written
	Size  int
	Label string // a name one gave it: such a copy is kept for good
}

// blankBody reports whether a page file holds no text and no object.
func blankBody(src []byte) bool {
	_, body := SplitPage(src)
	return blankText(body)
}

// Historian is a ground that keeps versions of its pages.
type Historian interface {
	Versions(name string) []Version
	VersionData(name string, id int64) ([]byte, bool)
	SnapshotNow(name string)
	SaveNamed(name, label string) bool
}

// ---- the difference between a version and the present ----

// DiffOp is a line of a difference: both have it, only the version has it
// (it comes back), or only the present (it goes).
type DiffOp struct {
	Kind  byte // '=', '+', '-'
	Text  string
	Words []DiffOp // set where a line that comes back and one that goes are of a pair: the words of this line, those that differ marked
}

// PairWords sets, for the lines that go and come back side by side, which of
// their words are what differs: the first line that comes back with the first
// that goes, and so on.
func PairWords(ops []DiffOp) {
	for i := 0; i < len(ops); {
		if ops[i].Kind == '=' {
			i++
			continue
		}
		j := i
		var adds, dels []int
		for j < len(ops) && ops[j].Kind != '=' {
			if ops[j].Kind == '+' {
				adds = append(adds, j)
			} else {
				dels = append(dels, j)
			}
			j++
		}
		for k := 0; k < len(adds) && k < len(dels); k++ {
			a, d := adds[k], dels[k]
			w := DiffLines(strings.Fields(ops[a].Text), strings.Fields(ops[d].Text))
			var mine, theirs []DiffOp
			for _, o := range w {
				switch o.Kind {
				case '=':
					mine, theirs = append(mine, o), append(theirs, o)
				case '+':
					mine = append(mine, o)
				default:
					theirs = append(theirs, o)
				}
			}
			ops[a].Words, ops[d].Words = mine, theirs
		}
		i = j
	}
}

// DiffLines sets the lines of the version against those of the present: the
// longest run they share, the rest marked. Long texts are compared by their
// first lines only.
func DiffLines(version, now []string) []DiffOp {
	const cap = 1500
	if len(version) > cap {
		version = version[:cap]
	}
	if len(now) > cap {
		now = now[:cap]
	}
	// the lines both start and end with are the same in any longest run: only those between are compared, a copy
	// a few lines from the present costing a few lines, not 1 501 × 1 501 numbers (#199)
	var head, tail []DiffOp
	for len(version) > 0 && len(now) > 0 && version[0] == now[0] {
		head = append(head, DiffOp{Kind: '=', Text: version[0]})
		version, now = version[1:], now[1:]
	}
	for len(version) > 0 && len(now) > 0 && version[len(version)-1] == now[len(now)-1] {
		tail = append(tail, DiffOp{Kind: '=', Text: version[len(version)-1]})
		version, now = version[:len(version)-1], now[:len(now)-1]
	}
	n, m := len(version), len(now)
	w := m + 1
	lcs := make([]int32, (n+1)*w) // one block, not a slice a line
	at := func(i, j int) int32 { return lcs[i*w+j] }
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if version[i] == now[j] {
				lcs[i*w+j] = at(i+1, j+1) + 1
			} else {
				lcs[i*w+j] = max(at(i+1, j), at(i, j+1))
			}
		}
	}
	out := head
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case version[i] == now[j]:
			out = append(out, DiffOp{Kind: '=', Text: version[i]})
			i++
			j++
		case at(i+1, j) >= at(i, j+1):
			out = append(out, DiffOp{Kind: '+', Text: version[i]})
			i++
		default:
			out = append(out, DiffOp{Kind: '-', Text: now[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffOp{Kind: '+', Text: version[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffOp{Kind: '-', Text: now[j]})
	}
	for k := len(tail) - 1; k >= 0; k-- {
		out = append(out, tail[k])
	}
	return out
}
