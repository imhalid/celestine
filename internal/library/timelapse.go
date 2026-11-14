package library

import (
	"slices"
	"strings"
	"time"
)

// The time-lapse of a page: every word ever written in it, across the
// versions kept, in reading order - with the version it came in and the one
// it was taken out in - so that the writing of a text can be shown as it
// went: which words came first, which were added, rewritten or cut.

// Word is a word of a page's history, or the end of a paragraph (Para).
type Word struct {
	Text string // "\n" at the end of a paragraph
	Born int    // the version it first shows in, from 1
	Gone int    // the version it is not in any more; 0 if the last has it
	// Rewrite: it came in place of other words, not into a gap; Replaced:
	// it went for other words, not cut alone.
	Rewrite, Replaced bool
}

// Para reports whether the word is the end of a paragraph.
func (w Word) Para() bool { return w.Text == "\n" }

// movedRun is how many words in a row, cut in one place and written in
// another, are taken for a move: they keep the version they were written in.
const movedRun = 6

// WordHistory is the history of the words of texts, the versions of one
// text from the oldest: each set against the one before it, a word taken
// for the same while it stays among the same neighbours. A line of a text
// is a paragraph.
func WordHistory(texts []string) []Word {
	if len(texts) == 0 {
		return nil
	}
	ids := map[string]int{}
	id := func(t string) int {
		v, ok := ids[t]
		if !ok {
			v = len(ids)
			ids[t] = v
		}
		return v
	}
	tokens := func(text string) []int {
		var out []int
		for _, line := range strings.Split(text, "\n") {
			if f := strings.Fields(line); len(f) > 0 {
				for _, w := range f {
					out = append(out, id(w))
				}
				out = append(out, id("\n"))
			}
		}
		return out
	}
	type word struct {
		tok               int
		born, gone        int
		rewrite, replaced bool
	}
	var all []word
	var live []int
	for _, t := range tokens(texts[0]) {
		live = append(live, len(all))
		all = append(all, word{tok: t, born: 1})
	}
	order := slices.Clone(live)
	for k := 1; k < len(texts); k++ {
		ver, b := k+1, tokens(texts[k])
		a := make([]int, len(live))
		for i, m := range live {
			a[i] = all[m].tok
		}
		ops, _ := matchWords(a, b)
		var next, dels, ins []int
		after := map[int][]int{} // the new words that come right after a word; -1: at the start
		for _, o := range ops {
			if o.tag == '=' {
				next = append(next, live[o.i1:o.i2]...)
				continue
			}
			for _, m := range live[o.i1:o.i2] {
				all[m].gone, all[m].replaced = ver, o.tag == 'r'
				dels = append(dels, m)
			}
			anchor := -1
			switch {
			case o.i2 > o.i1:
				anchor = live[o.i2-1]
			case o.i1 > 0:
				anchor = live[o.i1-1]
			}
			for _, t := range b[o.j1:o.j2] {
				n := len(all)
				all = append(all, word{tok: t, born: ver, rewrite: o.tag == 'r'})
				after[anchor] = append(after[anchor], n)
				next, ins = append(next, n), append(ins, n)
			}
		}
		// the new words go in reading order right after the words they
		// replace or follow, the cut ones staying where they were
		placed := append([]int(nil), after[-1]...)
		for _, m := range order {
			placed = append(placed, m)
			placed = append(placed, after[m]...)
		}
		order = placed
		// a long run cut in one place and written in another was moved
		dt, it := make([]int, len(dels)), make([]int, len(ins))
		for i, m := range dels {
			dt[i] = all[m].tok
		}
		for i, m := range ins {
			it[i] = all[m].tok
		}
		_, blocks := matchWords(dt, it)
		gone := map[int]bool{}
		for _, bl := range blocks {
			if bl.size < movedRun {
				continue
			}
			for x := range bl.size {
				d, n := dels[bl.i+x], ins[bl.j+x]
				all[n].born, all[n].rewrite = all[d].born, all[d].rewrite
				gone[d] = true
			}
		}
		if len(gone) > 0 {
			order = slices.DeleteFunc(order, func(m int) bool { return gone[m] })
		}
		live = next
	}
	name := make([]string, len(ids))
	for w, v := range ids {
		name[v] = w
	}
	out := make([]Word, len(order))
	for i, m := range order {
		w := all[m]
		out[i] = Word{Text: name[w.tok], Born: w.born, Gone: w.gone, Rewrite: w.rewrite, Replaced: w.replaced}
	}
	return out
}

// ---- matching two runs of words ----

// wordOp is a step from one run to the other: a[i1:i2] and b[j1:j2] are the
// same ('='), or a's are replaced by b's ('r'), cut ('d') or b's put in ('i').
type wordOp struct {
	tag            byte
	i1, i2, j1, j2 int
}

// wordBlock is a run both have: a[i:i+size] is b[j:j+size].
type wordBlock struct{ i, j, size int }

// matchWords sets a against b as Python's difflib.SequenceMatcher does,
// without its junk: the longest run they share, then the same on each side
// of it. It gives the steps from a to b and the runs they share.
func matchWords(a, b []int) ([]wordOp, []wordBlock) {
	// What the two share at their start is matched at once: two versions of a page are mostly the same, and only
	// what follows is looked through (#506). Their end is not: a run there would be taken over an earlier one.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	// A word in more than one in a hundred of b's (of 200 or more) does not start a run, as difflib's autojunk
	// has it: each place of "the" in a long page was tried against each of a's, which on a page of thousands of
	// like lines took minutes. A run found is still drawn out over such words on both sides (longest).
	b2j := map[int][]int{}
	for j := pre; j < len(b); j++ {
		b2j[b[j]] = append(b2j[b[j]], j)
	}
	if n := len(b) - pre; n >= 200 {
		for t, js := range b2j {
			if len(js) > n/100+1 {
				delete(b2j, t)
			}
		}
	}
	// prev[j+1] is the length of the run ending at a[i-1] and b[j]; the
	// slices are kept clean between calls by zeroing what was set.
	prev, cur := make([]int, len(b)+1), make([]int, len(b)+1)
	var prevSet, curSet []int
	longest := func(alo, ahi, blo, bhi int) wordBlock {
		best := wordBlock{alo, blo, 0}
		for i := alo; i < ahi; i++ {
			for _, j := range b2j[a[i]] {
				if j < blo {
					continue
				}
				if j >= bhi {
					break
				}
				k := prev[j] + 1
				cur[j+1] = k
				curSet = append(curSet, j+1)
				if k > best.size {
					best = wordBlock{i - k + 1, j - k + 1, k}
				}
			}
			for _, x := range prevSet {
				prev[x] = 0
			}
			prev, cur = cur, prev
			prevSet, curSet = curSet, prevSet[:0]
		}
		for _, x := range prevSet {
			prev[x] = 0
		}
		prevSet = prevSet[:0]
		// nothing but such words: the run the two share at the start of what is left, or at its end
		if best.size == 0 {
			for best.size < ahi-alo && best.size < bhi-blo && a[alo+best.size] == b[blo+best.size] {
				best.size++
			}
			if best.size == 0 {
				k := 0
				for k < ahi-alo && k < bhi-blo && a[ahi-1-k] == b[bhi-1-k] {
					k++
				}
				best = wordBlock{ahi - k, bhi - k, k}
			}
		}
		// over the words that start no run, before it and after it
		for best.size > 0 && best.i > alo && best.j > blo && a[best.i-1] == b[best.j-1] {
			best.i, best.j, best.size = best.i-1, best.j-1, best.size+1
		}
		for best.size > 0 && best.i+best.size < ahi && best.j+best.size < bhi && a[best.i+best.size] == b[best.j+best.size] {
			best.size++
		}
		return best
	}

	var blocks []wordBlock
	if pre > 0 {
		blocks = append(blocks, wordBlock{0, 0, pre})
	}
	queue := [][4]int{{pre, len(a), pre, len(b)}}
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		alo, ahi, blo, bhi := q[0], q[1], q[2], q[3]
		m := longest(alo, ahi, blo, bhi)
		if m.size == 0 {
			continue
		}
		blocks = append(blocks, m)
		if alo < m.i && blo < m.j {
			queue = append(queue, [4]int{alo, m.i, blo, m.j})
		}
		if m.i+m.size < ahi && m.j+m.size < bhi {
			queue = append(queue, [4]int{m.i + m.size, ahi, m.j + m.size, bhi})
		}
	}
	slices.SortFunc(blocks, func(x, y wordBlock) int {
		if x.i != y.i {
			return x.i - y.i
		}
		return x.j - y.j
	})
	// runs that touch are one run
	var merged []wordBlock
	for _, bl := range blocks {
		if n := len(merged); n > 0 && merged[n-1].i+merged[n-1].size == bl.i && merged[n-1].j+merged[n-1].size == bl.j {
			merged[n-1].size += bl.size
			continue
		}
		merged = append(merged, bl)
	}

	var ops []wordOp
	i, j := 0, 0
	for _, bl := range append(merged, wordBlock{len(a), len(b), 0}) {
		var tag byte
		switch {
		case i < bl.i && j < bl.j:
			tag = 'r'
		case i < bl.i:
			tag = 'd'
		case j < bl.j:
			tag = 'i'
		}
		if tag != 0 {
			ops = append(ops, wordOp{tag, i, bl.i, j, bl.j})
		}
		i, j = bl.i+bl.size, bl.j+bl.size
		if bl.size > 0 {
			ops = append(ops, wordOp{'=', bl.i, i, bl.j, j})
		}
	}
	return ops, merged
}

// LastOfDays is, of times from the oldest, the index of the last of each
// day: the versions of a text written at those times, a day a version.
func LastOfDays(times []time.Time) []int {
	var out []int
	for i, t := range times {
		if i+1 < len(times) && sameDay(t, times[i+1]) {
			continue
		}
		out = append(out, i)
	}
	return out
}

// sameDay reports whether a and b are on the same day where the computer is.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}
