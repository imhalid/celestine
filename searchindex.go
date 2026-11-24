package main

import (
	"math/bits"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// The search index. Every page's text and title, folded (Turkish letters as their ASCII, lower case), is
// cut into pairs of letters, and for each pair there is a row of bits: which pages have it. A word asked
// for is its pairs, and the pages that have all of them are the AND of their rows: 10 000 pages are 157
// machine words a row, so the pages that may have the word are known in microseconds, and only those are
// looked into (a pair is a filter, not a proof: the word itself is looked for in them). The text is read
// once, when the page is new or was written; the library's pages are taken in by a few milliseconds a
// frame after it opens (sxWarm), and a search that comes before that is the old scan.
//
// The pairs are hashed into 65 536 rows, so two pairs may share a row: the rows only ever hold more
// pages than they should, never fewer. A row of few pages is a list of them (sparse), and becomes bits
// past sxSparseMax: a row of bits is as long as the last page that has its pair, and a text of many
// letters (Chinese, Japanese) has pairs in most rows, each met by a few pages: 82 MB of bits at ten
// thousand pages, where the lists are a few (#196).

const (
	sxRows      = 1 << 16
	sxBudget    = 3 * time.Millisecond // of a frame, to take pages in
	sxScoreCap  = 600                  // pages looked into at most, for a broad query with a limit
	sxCountCap  = 10                   // occurrences counted for a score
	sxLetterBit = 36                   // a letter that is not ASCII
	sxSparseMax = 32                   // the pages a row holds as a list, before it becomes bits
)

// sxDoc is a page in the index, as it was read.
type sxDoc struct {
	page   *Page
	coll   *Collection
	title  string
	ftitle string
	fcoll  string
	ftext  string // shared with plainOf's, not copied
	mod    int64
	writes int
	epoch  int
	alive  bool
}

type searchIndex struct {
	docs   []*sxDoc
	rows   [sxRows][]uint64    // the pages that have a pair, as bits; nil while sparse holds them
	sparse [sxRows][]int32     // or as a list, while they are few
	seen   [sxRows / 64]uint64 // the pairs of the page being put in
	cand   []uint64            // the pages a query may find
	one    []uint64            // a sparse row as bits, for a query
	epoch  int
	ready  bool // every page of the library has been taken in once
	queue  []*Page
	qcoll  []*Collection
	qpos   int
	dead   int
	gen    int // counts the times it was made again: a page of an earlier one is not in it
}

func newSearchIndex() *searchIndex { return &searchIndex{} }

// pairRow is the row of the pair a, b.
func pairRow(a, b rune) uint16 {
	h := uint32(a)*0x9E3779B1 ^ uint32(b)*0x85EBCA6B
	return uint16(h >> 16)
}

// put sets (on) or clears the bits of page id for the pairs of s. When setting, x.seen says which
// pairs of the page were set already, so that a pair is looked up once per page, not once a time it comes.
func (x *searchIndex) put(id int, s string, on bool) {
	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			k := pairRow(prev, r)
			if on {
				w, m := k>>6, uint64(1)<<(k&63)
				if x.seen[w]&m == 0 {
					x.seen[w] |= m
					x.set(k, id)
				}
			} else {
				x.unset(k, id)
			}
		}
		prev = r
	}
}

func (x *searchIndex) set(k uint16, id int) {
	b := x.rows[k]
	if b == nil {
		sp := x.sparse[k]
		if slices.Contains(sp, int32(id)) {
			return
		}
		if len(sp) < sxSparseMax {
			x.sparse[k] = append(sp, int32(id))
			return
		}
		for _, o := range sp { // too many for a list: bits from now on
			b = setBit(b, int(o))
		}
		x.sparse[k] = nil
	}
	x.rows[k] = setBit(b, id)
}

// unset clears page id from row k.
func (x *searchIndex) unset(k uint16, id int) {
	if b := x.rows[k]; b != nil {
		if id>>6 < len(b) {
			b[id>>6] &^= 1 << (uint(id) & 63)
		}
		return
	}
	sp := x.sparse[k]
	if i := slices.Index(sp, int32(id)); i >= 0 {
		sp[i] = sp[len(sp)-1]
		x.sparse[k] = sp[:len(sp)-1]
	}
}

// setBit sets bit id of b, made longer as needed.
func setBit(b []uint64, id int) []uint64 {
	for len(b) <= id>>6 {
		b = append(b, 0)
	}
	b[id>>6] |= 1 << (uint(id) & 63)
	return b
}

// take puts page p, of collection c, in the index, or again when it was written since; epoch says it is still there.
func (x *searchIndex) take(a *app, c *Collection, p *Page) {
	id, ok := p.SxID, p.SxOf == x && p.SxGen == x.gen // the page knows its place: no lookup
	var d *sxDoc
	if ok {
		d = x.docs[id]
		d.epoch = x.epoch
		if d.mod == p.Modified.UnixNano() && d.writes == p.Writes && d.title == p.Title && d.coll == c {
			return
		}
		x.put(id, d.ftitle, false)
		x.put(id, d.ftext, false)
	} else {
		id = len(x.docs)
		d = &sxDoc{page: p, alive: true, epoch: x.epoch}
		x.docs = append(x.docs, d)
		p.SxID, p.SxOf, p.SxGen = id, x, x.gen
	}
	d.coll, d.title, d.fcoll = c, p.Title, foldText(c.Name)
	d.mod, d.writes = p.Modified.UnixNano(), p.Writes
	d.ftitle, d.ftext = foldText(p.Title), a.plainOf(p).folded
	clear(x.seen[:])
	x.put(id, d.ftitle, true)
	x.put(id, d.ftext, true)
}

// sweep forgets the pages that were not met in this pass: they are gone from the library.
func (x *searchIndex) sweep() {
	for id, d := range x.docs {
		if d == nil || d.epoch == x.epoch {
			continue
		}
		x.put(id, d.ftitle, false)
		x.put(id, d.ftext, false)
		d.page.SxOf = nil
		x.docs[id] = nil
		x.dead++
	}
}

// sync brings the index up to date with the library, all of it: what changed since the last time is
// read again and nothing else. Fast when little changed, which is always after the first pass.
func (x *searchIndex) sync(a *app) {
	x.epoch++
	var walk func(c *Collection, ps []*Page)
	walk = func(c *Collection, ps []*Page) {
		for _, p := range ps {
			x.take(a, c, p)
			walk(c, p.Children)
		}
	}
	for _, c := range a.lib.Collections {
		walk(c, c.Pages)
	}
	x.sweep()
	if x.dead > 256 && x.dead > len(x.docs)/4 { // a hollow index: made again
		x.reset()
		x.sync(a)
		return
	}
	x.ready = true
}

// reset empties the index, to be made again.
func (x *searchIndex) reset() {
	x.docs, x.dead, x.ready, x.queue, x.qcoll = nil, 0, false, nil, nil
	for i := range x.rows {
		x.rows[i], x.sparse[i] = nil, nil
	}
	x.gen++
}

// warm takes the library's pages in, a few milliseconds of a frame at a time, until all are in; it
// says whether it is done. A page's text is read here, not in the keystroke that asks for it.
func (x *searchIndex) warm(a *app, budget time.Duration) bool {
	if x.ready {
		return true
	}
	start := time.Now()
	if x.queue == nil {
		x.epoch++
		for _, c := range a.lib.Collections {
			for _, p := range c.All() {
				x.queue = append(x.queue, p)
				x.qcoll = append(x.qcoll, c)
			}
		}
		x.qpos = 0
	}
	for n := 0; x.qpos < len(x.queue); x.qpos++ {
		x.take(a, x.qcoll[x.qpos], x.queue[x.qpos])
		if n++; n%8 == 0 && time.Since(start) > budget {
			x.qpos++
			return false
		}
	}
	x.queue, x.qcoll = nil, nil
	x.ready = true
	return true
}

// sxHit is a page the index found, and how well.
type sxHit struct {
	d     *sxDoc
	score int
}

// terms are what every page found has in its title or text: the words and the phrases not under an OR or a NOT.
func (q query) terms() []string { return append(append([]string{}, q.words...), q.phrases...) }

// countCap is how often w is in s, up to max.
func countCap(s, w string, max int) int {
	n := 0
	for n < max {
		i := strings.Index(s, w)
		if i < 0 {
			break
		}
		n++
		s = s[i+len(w):]
	}
	return n
}

// query finds the pages that have everything asked, the best first: the pages the pairs allow are
// looked into for the words themselves, and scored as the scan scored them. With a limit, a query that
// a great many pages answer looks into the ones with the words in their titles first and then the
// newest, up to sxScoreCap: the rest could only be told apart by a count of the words nobody reads.
func (x *searchIndex) query(a *app, pq query, limit int) []sxHit {
	x.sync(a)
	n := len(x.docs)
	nw := (n + 63) / 64
	if cap(x.cand) < nw {
		x.cand = make([]uint64, nw)
	}
	cand := x.cand[:nw]
	for i := range cand {
		cand[i] = ^uint64(0)
	}
	if r := n & 63; r != 0 {
		cand[nw-1] = 1<<uint(r) - 1
	}
	for _, t := range append(pq.terms(), pq.title...) {
		var prev rune = -1
		for _, r := range t {
			if prev >= 0 {
				k := pairRow(prev, r)
				row := x.rows[k]
				if row == nil { // a list: as bits, for the AND
					if cap(x.one) < nw {
						x.one = make([]uint64, nw)
					}
					row = x.one[:nw]
					clear(row)
					for _, id := range x.sparse[k] {
						if int(id)>>6 < nw {
							row[id>>6] |= 1 << (uint(id) & 63)
						}
					}
				}
				for i := range cand {
					if i < len(row) {
						cand[i] &= row[i]
					} else {
						cand[i] = 0
					}
				}
			}
			prev = r
		}
	}
	var ids []int
	for i, w := range cand {
		for w != 0 {
			b := bits.TrailingZeros64(w)
			w &= w - 1
			ids = append(ids, i<<6+b)
		}
	}
	// the newest last-seen pages first: ids grow as pages are met, and a page written again keeps its id,
	// so the order is only a fair guess at recency, which is all a cap needs
	slices.Reverse(ids)
	var out []sxHit
	score := func(d *sxDoc) (int, bool) {
		if d == nil || !d.alive {
			return 0, false
		}
		return a.score(pq.root, qdoc{d.page, d.coll, d.ftitle, d.ftext, d.fcoll})
	}
	broad := limit > 0 && len(ids) > sxScoreCap
	if broad {
		first, rest := ids[:0:0], ids[:0:0]
		for _, id := range ids {
			d := x.docs[id]
			if d == nil {
				continue
			}
			inTitle := false
			for _, w := range pq.terms() {
				if strings.Contains(d.ftitle, w) {
					inTitle = true
					break
				}
			}
			if inTitle {
				first = append(first, id)
			} else {
				rest = append(rest, id)
			}
		}
		ids = append(first, rest...)
	}
	for _, id := range ids {
		if s, ok := score(x.docs[id]); ok {
			out = append(out, sxHit{x.docs[id], s})
			if broad && len(out) >= max(sxScoreCap, limit*20) {
				break
			}
		}
	}
	slices.SortStableFunc(out, func(p, q sxHit) int {
		if p.score != q.score {
			return q.score - p.score
		}
		return q.d.page.Modified.Compare(p.d.page.Modified)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// letterMask has a bit for each letter and digit of the folded s, and one for any other: what a
// string has to have for the letters of a query to be in it in order.
func letterMask(s string) uint64 {
	var m uint64
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z':
			m |= 1 << (c - 'a')
		case c >= '0' && c <= '9':
			m |= 1 << (26 + c - '0')
		case c >= utf8.RuneSelf:
			m |= 1 << sxLetterBit
		}
	}
	return m
}
