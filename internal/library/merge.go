package library

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// A page changed on two devices between syncs is merged, when it can be, instead of being kept twice: the page as
// it was at the last sync (its base) is compared with each side, block by block, and what changed on one side
// only is taken from that side. Blocks added at the same place on both are both kept, ours first. A block changed
// on both sides, or changed on one and deleted on the other, is a conflict, and the page is kept twice as before.
// The meta is merged the same way, key by key.

// maxMergeCells bounds the work of comparing two pages, block by block (the product of their lengths): a page past it
// is not merged, and a conflict keeps it twice.
const maxMergeCells = 4_000_000

// MergePage merges the page files mine and theirs, both changed from base; ok is false when they changed the same
// thing, or one of them is not a page file.
func MergePage(base, mine, theirs []byte) (merged []byte, ok bool) {
	var metas [3]map[string]string
	var bodies [3][]string
	for i, src := range [][]byte{base, mine, theirs} {
		meta, body, err := SplitPageChecked(src)
		if err != nil {
			return nil, false
		}
		var blocks []json.RawMessage
		if err := json.Unmarshal([]byte(body), &blocks); err != nil {
			return nil, false
		}
		for _, b := range blocks {
			var c bytes.Buffer
			if json.Compact(&c, b) != nil {
				return nil, false
			}
			bodies[i] = append(bodies[i], c.String())
		}
		metas[i] = meta
	}
	meta, ok := mergeMeta(metas[0], metas[1], metas[2])
	if !ok {
		return nil, false
	}
	blocks, ok := merge3(bodies[0], bodies[1], bodies[2])
	if !ok {
		return nil, false
	}
	return JoinPage(meta, "["+strings.Join(blocks, ",")+"]"), true
}

// mergeMeta merges the meta key by key.
func mergeMeta(base, mine, theirs map[string]string) (map[string]string, bool) {
	out := map[string]string{}
	keys := map[string]bool{}
	for _, m := range []map[string]string{base, mine, theirs} {
		for k := range m {
			keys[k] = true
		}
	}
	for k := range keys {
		o, oin := base[k]
		a, ain := mine[k]
		b, bin := theirs[k]
		switch {
		case a == o && ain == oin:
			a, ain = b, bin
		case b == o && bin == oin, a == b && ain == bin:
		default:
			return nil, false
		}
		if ain {
			out[k] = a
		}
	}
	return out, true
}

// merge3 merges two sequences changed from base (a three-way merge, diff3's): the stretches between the blocks all
// three have in common are taken from the side that changed them.
func merge3(base, mine, theirs []string) ([]string, bool) {
	if len(base)*(len(mine)+len(theirs)) > maxMergeCells {
		return nil, false
	}
	ma, mb := matchLCS(base, mine), matchLCS(base, theirs)
	var out []string
	i, ja, jb := 0, 0, 0
	for {
		k := i // the next block of the base that both sides kept
		for k < len(base) && (ma[k] < 0 || mb[k] < 0) {
			k++
		}
		ea, eb := len(mine), len(theirs)
		if k < len(base) {
			ea, eb = ma[k], mb[k]
		}
		o, a, b := base[i:k], mine[ja:ea], theirs[jb:eb]
		switch {
		case slices.Equal(a, o):
			out = append(out, b...)
		case slices.Equal(b, o), slices.Equal(a, b):
			out = append(out, a...)
		case len(o) == 0: // added at the same place on both: both are kept
			out = append(append(out, a...), b...)
		default:
			return nil, false
		}
		if k == len(base) {
			return out, true
		}
		out = append(out, base[k])
		i, ja, jb = k+1, ea+1, eb+1
	}
}

// matchLCS is, for each element of a, the index in b it is matched to in a longest common subsequence, or -1.
func matchLCS(a, b []string) []int {
	n, m := len(a), len(b)
	l := make([][]int32, n+1) // l[i][j]: the length of the longest common subsequence of a[i:] and b[j:]
	for i := range l {
		l[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				l[i][j] = l[i+1][j+1] + 1
			case l[i+1][j] >= l[i][j+1]:
				l[i][j] = l[i+1][j]
			default:
				l[i][j] = l[i][j+1]
			}
		}
	}
	out := make([]int, n)
	for i := range out {
		out[i] = -1
	}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[i] == b[j]:
			out[i] = j
			i++
			j++
		case l[i+1][j] >= l[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}
