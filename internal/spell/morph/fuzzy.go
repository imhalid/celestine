package morph

// This file is fuzzy.ts: an error-tolerant search over the stem index and the graph. It finds the valid Turkish
// words within 1 or 2 Damerau-Levenshtein edits of a typed word, Turkish marks restored freely, unseen inflected
// forms included ("düzenlemiyor" from "duzenblemiyor" or "duzenlemniyor", "gerekli" from "grkli", "bekletiyoruz"
// from "beketiyruz"). The stems are in a trie folded to ASCII, built once in memory; a banded edit-distance row
// follows each path; a suffix is matched exactly once no edit is left; the stems are tried by distance, length
// and lemma frequency, with a bound on the graph's steps; results are cached.

import (
	"slices"
	"sort"
	"strconv"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// FuzzyMorphCandidate is a valid word the fuzzy search found.
type FuzzyMorphCandidate struct {
	// Word is the morphologically valid lowercase Turkish word.
	Word string `json:"word"`
	// FoldedEdits is the Damerau-Levenshtein distance with the marks folded: 0 for marks restored only, 1 or 2.
	FoldedEdits int `json:"foldedEdits"`
	// EditCost is TurkishEditCost of the typed word and Word.
	EditCost float64 `json:"editCost"`
}

const (
	maxWordLen    = 48
	maxFuzzyCache = 4096
	infDist       = 99
	foldOther     = 26
)

// foldCode is a lowercase Turkish letter (or a circumflexed one) as its ASCII letter's index 0..25; 26 for
// anything else.
func foldCode(r rune) byte {
	switch {
	case r >= 'a' && r <= 'z':
		return byte(r - 'a')
	case r == 'ç':
		return 'c' - 'a'
	case r == 'ğ':
		return 'g' - 'a'
	case r == 'ı', r == 'î':
		return 'i' - 'a'
	case r == 'ö':
		return 'o' - 'a'
	case r == 'ş':
		return 's' - 'a'
	case r == 'ü', r == 'û':
		return 'u' - 'a'
	case r == 'â':
		return 'a' - 'a'
	}
	return foldOther
}

// isPlainVowel reports whether r is one of the eight Turkish vowels.
func isPlainVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'ı', 'i', 'o', 'ö', 'u', 'ü':
		return true
	}
	return false
}

// isTurkishMark reports whether r is a Turkish letter with a mark.
func isTurkishMark(r rune) bool {
	switch r {
	case 'ç', 'ğ', 'ı', 'ö', 'ş', 'ü':
		return true
	}
	return false
}

// isVoicingPair reports whether typing a for b swaps a consonant's voicing.
func isVoicingPair(a, b rune) bool {
	switch [2]rune{a, b} {
	case [2]rune{'d', 't'}, [2]rune{'t', 'd'}, [2]rune{'b', 'p'}, [2]rune{'p', 'b'}, [2]rune{'c', 'ç'},
		[2]rune{'ç', 'c'}, [2]rune{'g', 'k'}, [2]rune{'k', 'g'}, [2]rune{'g', 'ğ'}, [2]rune{'ğ', 'g'}:
		return true
	}
	return false
}

// isMarkRestorePair reports whether typing a for b lost b's Turkish mark.
func isMarkRestorePair(a, b rune) bool {
	switch [2]rune{a, b} {
	case [2]rune{'c', 'ç'}, [2]rune{'g', 'ğ'}, [2]rune{'i', 'ı'}, [2]rune{'o', 'ö'}, [2]rune{'s', 'ş'},
		[2]rune{'u', 'ü'}:
		return true
	}
	return false
}

// isMarkStripPair reports whether typing a for b added a mark b does not have.
func isMarkStripPair(a, b rune) bool {
	switch [2]rune{a, b} {
	case [2]rune{'ç', 'c'}, [2]rune{'ğ', 'g'}, [2]rune{'ı', 'i'}, [2]rune{'ö', 'o'}, [2]rune{'ş', 's'},
		[2]rune{'ü', 'u'}:
		return true
	}
	return false
}

// charSubCost is the cost of typing typedCh where candCh belongs.
func charSubCost(typedCh, candCh rune, typedIndex int) float64 {
	if typedCh == candCh {
		return 0
	}
	if isMarkRestorePair(typedCh, candCh) || typedIndex == 0 && typedCh == 'ı' && candCh == 'i' {
		return 0.35
	}
	if isMarkStripPair(typedCh, candCh) {
		return 1.25
	}
	lead := 0.0
	if typedIndex == 0 {
		lead = 0.45
	}
	switch {
	case tr.AreNearKeys(typedCh, candCh):
		return 0.8 + lead
	case isPlainVowel(typedCh) && isPlainVowel(candCh):
		return 1.1 + lead
	case isVoicingPair(typedCh, candCh):
		return 1.25 + lead
	}
	return 1.8 + lead
}

// deleteCost is the cost of a typed letter that does not belong: less for a doubled letter or one next to a key
// beside it.
func deleteCost(typed []rune, i int) float64 {
	m := len(typed)
	ch := typed[i-1]
	isDup := i > 1 && ch == typed[i-2] || i < m && ch == typed[i]
	isNear := i > 1 && tr.AreNearKeys(ch, typed[i-2]) || i < m && tr.AreNearKeys(ch, typed[i])
	switch {
	case isDup:
		return 0.65
	case isNear:
		return 0.85
	case isTurkishMark(ch):
		return 2.4
	}
	return 1.9
}

// TurkishEditCost is the edit cost from typed to cand on a Turkish Q keyboard, with lost marks and swapped
// letters: lower is closer, about 0.35 for a missing Turkish mark, 0.6-0.95 for one typo, 1.2-1.8 for two.
func TurkishEditCost(typed, cand string) float64 {
	t, c := []rune(typed), []rune(cand)
	m, n := len(t), len(c)
	if m == 0 {
		return float64(n)
	}
	if n == 0 {
		return float64(m)
	}
	if typed == cand {
		return 0
	}

	stride := n + 1
	dp := make([]float64, (m+1)*stride)
	for i := 1; i <= m; i++ {
		first := 0.0
		if i == 1 {
			first = 0.45
		}
		dp[i*stride] = dp[(i-1)*stride] + deleteCost(t, i) + first
	}
	for j := 1; j <= n; j++ {
		insCost := 0.95
		if j > 1 && c[j-1] == c[j-2] {
			insCost = 0.65
		}
		first := 0.0
		if j == 1 {
			first = 0.45
		}
		dp[j] = dp[j-1] + insCost + first
	}

	for i := 1; i <= m; i++ {
		tc := t[i-1]
		hasNext := i < m
		var tNext rune
		if hasNext {
			tNext = t[i]
		}
		first := 0.0
		if i == 1 {
			first = 0.45
		}
		delCost := deleteCost(t, i) + first
		rowOff := i * stride
		prevOff := (i - 1) * stride

		for j := 1; j <= n; j++ {
			cc := c[j-1]
			sub := dp[prevOff+j-1] + charSubCost(tc, cc, i-1)
			del := dp[prevOff+j] + delCost

			// The insertion of cc between typed[i-1] and typed[i], a letter missing from typed.
			betweenConsonants := hasNext && !isPlainVowel(tc) && !isPlainVowel(tNext)
			isDupIns := cc == tc || hasNext && cc == tNext
			insBase := 0.95
			if betweenConsonants && isPlainVowel(cc) {
				insBase = 0.6
			} else if isDupIns {
				insBase = 0.65
			}
			ins := dp[rowOff+j-1] + insBase

			var best float64
			if sub < del {
				if sub < ins {
					best = sub
				} else {
					best = ins
				}
			} else if del < ins {
				best = del
			} else {
				best = ins
			}

			// An adjacent transposition, the swapped letters' marks restored as well.
			if i >= 2 && j >= 2 {
				c1 := charSubCost(t[i-2], cc, i-2)
				c2 := charSubCost(tc, c[j-2], i-1)
				if c1 <= 0.35 && c2 <= 0.35 {
					second := 0.0
					if i == 2 {
						second = 0.35
					}
					trans := dp[(i-2)*stride+j-2] + 0.7 + c1 + c2 + second
					if trans < best {
						best = trans
					}
				}
			}
			dp[rowOff+j] = best
		}
	}
	return dp[m*stride+n]
}

// trieNode is a node of the folded stem trie; its keys and children are in the order they were added.
type trieNode struct {
	keys     []byte
	children []*trieNode
	stems    []*stemTransition
}

// exactNode is a path node of the exact folded suffix search.
type exactNode struct {
	pathNode
	word       string
	derivCount int
}

// fuzzyNode is a path node of the fuzzy search, with the edit-distance rows after its word.
type fuzzyNode struct {
	pathNode
	word       string
	wordLen    int
	row        []byte
	prevRow    []byte
	lastCode   byte
	derivCount int
}

// exceedsLocalRepeat reports whether the path already passed through stateID twice.
func exceedsLocalRepeat(node *pathNode, stateID string) bool {
	count := 0
	for cur := node; cur != nil; cur = cur.prev {
		if cur.state.id == stateID {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// foundWords is the words found and their edit distances, in the order they were first found.
type foundWords struct {
	words []string
	edits map[string]int
}

// add records a word with its distance, keeping the smaller distance and the first place.
func (f *foundWords) add(word string, edits int) {
	prev, ok := f.edits[word]
	if !ok {
		f.words = append(f.words, word)
		f.edits[word] = edits
	} else if edits < prev {
		f.edits[word] = edits
	}
}

// stemHit is a stem within the edit budget of the typed word's beginning.
type stemHit struct {
	st         *stemTransition
	row        []byte
	prevRow    []byte
	lastCode   byte
	bestDist   int
	lemmaScore float64
}

// fuzzyMorphSearch is the fuzzy search over a stem index.
type fuzzyMorphSearch struct {
	analyzer  *morphologicalAnalyzer
	prior     *morphPrior
	root      *trieNode
	dpSlab    []byte
	wordCodes [maxWordLen]byte
	cache     map[string][]FuzzyMorphCandidate
}

// newFuzzyMorphSearch builds the folded stem trie of idx.
func newFuzzyMorphSearch(idx *stemIndex, analyzer *morphologicalAnalyzer, prior *morphPrior) *fuzzyMorphSearch {
	f := &fuzzyMorphSearch{
		analyzer: analyzer,
		prior:    prior,
		root:     &trieNode{},
		dpSlab:   make([]byte, (maxWordLen+6)*(maxWordLen+2)),
		cache:    map[string][]FuzzyMorphCandidate{},
	}
	for _, surface := range idx.order {
		entry := idx.stems[surface]
		rs := entry[0].runes
		if len(rs) == 0 || len(rs) > maxWordLen {
			continue
		}
		node := f.root
		ok := true
		for _, r := range rs {
			code := foldCode(r)
			if code >= foldOther {
				ok = false
				break
			}
			k := slices.Index(node.keys, code)
			if k == -1 {
				next := &trieNode{}
				node.keys = append(node.keys, code)
				node.children = append(node.children, next)
				node = next
			} else {
				node = node.children[k]
			}
		}
		if !ok {
			continue
		}
		node.stems = append(node.stems, entry...)
	}
	return f
}

// search finds the valid Turkish words within maxEdits (1 or 2) folded Damerau-Levenshtein edits of form, each
// verified with isWord, ordered by EditCost, at most maxResults of them (as JavaScript's slice(0, maxResults)
// takes them).
func (f *fuzzyMorphSearch) search(form string, maxEdits, maxResults int) []FuzzyMorphCandidate {
	word := tr.Lower(form)
	wr := []rune(word)
	L := len(wr)
	if L < 3 || L > maxWordLen-4 {
		return []FuzzyMorphCandidate{}
	}

	cacheKey := strconv.Itoa(maxEdits) + ":" + word
	if cached, ok := f.cache[cacheKey]; ok {
		return slices.Clone(cached[:jsSliceEnd(maxResults, len(cached))])
	}

	wCodes := &f.wordCodes
	for i, r := range wr {
		wCodes[i] = foldCode(r)
	}

	stride := L + 1
	slab := f.dpSlab
	// Row 0: 0, 1, 2, ..., L.
	for j := 0; j <= L; j++ {
		slab[j] = byte(j)
	}

	var stemHits []stemHit
	maxStemDepth := min(L+maxEdits, maxWordLen)

	var dfsTrie func(node *trieNode, depth int, lastCode byte)
	dfsTrie = func(node *trieNode, depth int, lastCode byte) {
		rowOff := depth * stride
		if node.stems != nil && depth >= 1 {
			loJ := 0
			if depth > maxEdits {
				loJ = depth - maxEdits
			}
			hiJ := L
			if depth+maxEdits < L {
				hiJ = depth + maxEdits
			}
			best := maxEdits + 1
			for j := loJ; j <= hiJ; j++ {
				if v := int(slab[rowOff+j]); v < best {
					best = v
				}
			}
			if best <= maxEdits && (depth >= 2 || slab[rowOff+1] == 0) {
				rowCopy := slices.Clone(slab[rowOff : rowOff+stride])
				var prevCopy []byte
				if depth > 0 {
					prevCopy = slices.Clone(slab[(depth-1)*stride : depth*stride])
				}
				for _, st := range node.stems {
					lemmaScore := 0.0
					if f.prior != nil {
						lemmaScore = f.prior.stemTotal(st.item.root)
					}
					stemHits = append(stemHits, stemHit{st, rowCopy, prevCopy, lastCode, best, lemmaScore})
				}
			}
		}

		if depth >= maxStemDepth {
			return
		}
		nextDepth := depth + 1
		nextOff := nextDepth * stride
		prevOff := (depth - 1) * stride
		loJ := 1
		if nextDepth > maxEdits+1 {
			loJ = nextDepth - maxEdits - 1
		}
		hiJ := L
		if nextDepth+maxEdits+1 < L {
			hiJ = nextDepth + maxEdits + 1
		}

		for k, code := range node.keys {
			// The whole row starts at infDist, so that a cell out of the band (row[L] of a short stem above all)
			// never reads a stale zero of an earlier search or an unvisited cell.
			for x := nextOff; x < nextOff+stride; x++ {
				slab[x] = infDist
			}
			if loJ == 1 {
				slab[nextOff] = byte(nextDepth)
			}
			minVal := nextDepth
			for j := loJ; j <= hiJ; j++ {
				sub := int(slab[rowOff+j-1])
				if wCodes[j-1] != code {
					sub++
				}
				del := int(slab[rowOff+j]) + 1
				ins := int(slab[nextOff+j-1]) + 1
				v := min3(sub, del, ins)
				if depth >= 1 && j >= 2 && wCodes[j-1] == lastCode && wCodes[j-2] == code {
					if trans := int(slab[prevOff+j-2]) + 1; trans < v {
						v = trans
					}
				}
				slab[nextOff+j] = byte(v)
				if v < minVal {
					minVal = v
				}
			}
			if minVal <= maxEdits {
				dfsTrie(node.children[k], nextDepth, code)
			}
		}
	}
	dfsTrie(f.root, 0, foldOther)

	// The smallest distance first, then the longest stems (so short stems of 2 or 3 letters do not starve the
	// real roots), then the most frequent lemma.
	sort.SliceStable(stemHits, func(i, j int) bool {
		a, b := &stemHits[i], &stemHits[j]
		if a.bestDist != b.bestDist {
			return a.bestDist < b.bestDist
		}
		if len(a.st.runes) != len(b.st.runes) {
			return len(b.st.runes) < len(a.st.runes)
		}
		return b.lemmaScore-a.lemmaScore < 0
	})

	found := foundWords{edits: map[string]int{}}
	maxFstSteps, maxStepsPerStem := 3500, 220
	if maxEdits <= 1 {
		maxFstSteps, maxStepsPerStem = 1500, 160
	}
	virtualInputLen := L + maxEdits + 1
	fstSteps := 0

	// The exact folded suffix expansion, when no edit is left.
	expandExactFolded := func(st *stemTransition, startPosInW, usedEdits, stemLimit int) {
		stack := []*exactNode{{
			pathNode: pathNode{
				stemTransition: st,
				state:          st.to,
				surface:        st.surface,
				pos:            startPosInW,
				inputLength:    L,
				phonetic:       st.phonetic,
			},
			word: st.surface,
		}}
		for len(stack) > 0 && fstSteps < stemLimit {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			fstSteps++
			if node.pos == L && node.state.terminal && node.phonetic&paCannotTerminate == 0 {
				found.add(node.word, usedEdits)
			}
			if node.depth >= 14 {
				continue
			}
			remaining := L - node.pos
			phonetic := node.phonetic
			p := &node.pathNode
			for _, t := range node.state.outgoing {
				isDeriv := t.to.derivative
				if isDeriv && node.derivCount >= 2 {
					continue
				}
				derivCount := node.derivCount
				lastDerivation := node.lastDerivation
				if isDeriv {
					derivCount++
					lastDerivation = t.to
				}
				if !t.hasSurface {
					if t.condition != nil && !t.condition(p) {
						continue
					}
					if node.depth > 4 && exceedsLocalRepeat(p, t.to.id) {
						continue
					}
					stack = append(stack, &exactNode{
						pathNode: pathNode{
							prev:                   p,
							stemTransition:         node.stemTransition,
							state:                  t.to,
							pos:                    node.pos,
							inputLength:            L,
							phonetic:               phonetic,
							depth:                  node.depth + 1,
							hasSurface:             node.hasSurface,
							hasDerivation:          node.hasDerivation || isDeriv,
							currentGroupHasSurface: !isDeriv && node.currentGroupHasSurface,
							lastDerivation:         lastDerivation,
						},
						word:       node.word,
						derivCount: derivCount,
					})
					continue
				}
				if remaining == 0 || phonetic&t.forbiddenPhonetic != 0 {
					continue
				}
				e := resolveSurfaceAndNextPhonetic(t, phonetic)
				if e == nil {
					continue
				}
				sLen := len(e.runes)
				if sLen > remaining {
					continue
				}
				matched := true
				for k, r := range e.runes {
					if foldCode(r) != wCodes[node.pos+k] {
						matched = false
						break
					}
				}
				if !matched {
					continue
				}
				if t.condition != nil && !t.condition(p) {
					continue
				}
				if node.depth > 4 && exceedsLocalRepeat(p, t.to.id) {
					continue
				}
				stack = append(stack, &exactNode{
					pathNode: pathNode{
						prev:                   p,
						stemTransition:         node.stemTransition,
						state:                  t.to,
						surface:                e.surface,
						pos:                    node.pos + sLen,
						inputLength:            L,
						phonetic:               int(e.nextPhonetic),
						depth:                  node.depth + 1,
						hasSurface:             true,
						hasDerivation:          node.hasDerivation || isDeriv,
						currentGroupHasSurface: !isDeriv,
						lastDerivation:         lastDerivation,
					},
					word:       node.word + e.surface,
					derivCount: derivCount,
				})
			}
		}
	}

	// advanceOneCode is the edit-distance row after the candidate takes one more letter, nil when it is out of
	// the budget; it is computed in the slab's scratch row first and copied only when it is kept.
	advanceOneCode := func(prevRow, prevPrevRow []byte, code, prevCode byte, candLen int) []byte {
		loJ := 1
		if candLen > maxEdits+1 {
			loJ = candLen - maxEdits - 1
		}
		hiJ := L
		if candLen+maxEdits+1 < L {
			hiJ = candLen + maxEdits + 1
		}
		scratchOff := (maxStemDepth + 2) * stride
		scratch := slab[scratchOff : scratchOff+stride]
		for x := range scratch {
			scratch[x] = infDist
		}
		if loJ == 1 {
			scratch[0] = byte(candLen)
		}
		minVal := infDist
		if candLen <= maxEdits {
			minVal = candLen
		}
		for j := loJ; j <= hiJ; j++ {
			sub := int(prevRow[j-1])
			if wCodes[j-1] != code {
				sub++
			}
			del := int(prevRow[j]) + 1
			ins := int(scratch[j-1]) + 1
			v := min3(sub, del, ins)
			if prevPrevRow != nil && j >= 2 && prevCode < foldOther && wCodes[j-1] == prevCode && wCodes[j-2] == code {
				if trans := int(prevPrevRow[j-2]) + 1; trans < v {
					v = trans
				}
			}
			scratch[j] = byte(v)
			if v < minVal {
				minVal = v
			}
		}
		if minVal <= maxEdits {
			return slices.Clone(scratch)
		}
		return nil
	}

	for h := 0; h < len(stemHits) && fstSteps < maxFstSteps; h++ {
		hit := &stemHits[h]
		st := hit.st
		stemLimit := min(maxFstSteps, fstSteps+maxStepsPerStem)

		// No edit left: the exact folded suffix expansion, unless a transposition across the stem's end is
		// possible (the stem's last letter is the next typed one, as "kalem|lerimi" for "kalel|merimi").
		if hit.bestDist == maxEdits {
			sLen := len(st.runes)
			loJ := 0
			if sLen > maxEdits {
				loJ = sLen - maxEdits
			}
			hiJ := L
			if sLen+maxEdits < L {
				hiJ = sLen + maxEdits
			}
			canBoundarySwap := false
			if hit.prevRow != nil && hit.lastCode < foldOther {
				for j := max(1, loJ); j <= hiJ && j < L; j++ {
					if int(hit.prevRow[j-1]) < maxEdits && wCodes[j] == hit.lastCode {
						canBoundarySwap = true
						break
					}
				}
			}
			if !canBoundarySwap {
				for j := loJ; j <= hiJ && fstSteps < stemLimit; j++ {
					if int(hit.row[j]) == maxEdits {
						expandExactFolded(st, j, maxEdits, stemLimit)
					}
				}
				continue
			}
		}

		// An edit left (or a transposition across the stem's end): the banded fuzzy search of the graph.
		stack := []*fuzzyNode{{
			pathNode: pathNode{
				stemTransition: st,
				state:          st.to,
				surface:        st.surface,
				pos:            len(st.runes),
				inputLength:    virtualInputLen,
				phonetic:       st.phonetic,
			},
			word:     st.surface,
			wordLen:  len(st.runes),
			row:      hit.row,
			prevRow:  hit.prevRow,
			lastCode: hit.lastCode,
		}}

		for len(stack) > 0 && fstSteps < stemLimit {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			fstSteps++

			endDist := int(node.row[L])
			if endDist <= maxEdits && node.state.terminal && node.phonetic&paCannotTerminate == 0 {
				found.add(node.word, endDist)
			}
			if node.depth >= 12 {
				continue
			}
			phonetic := node.phonetic
			p := &node.pathNode

			for _, t := range node.state.outgoing {
				isDeriv := t.to.derivative
				if isDeriv && node.derivCount >= 2 {
					continue
				}
				derivCount := node.derivCount
				lastDerivation := node.lastDerivation
				if isDeriv {
					derivCount++
					lastDerivation = t.to
				}

				if !t.hasSurface {
					if t.condition != nil && !t.condition(p) {
						continue
					}
					if node.depth > 4 && exceedsLocalRepeat(p, t.to.id) {
						continue
					}
					stack = append(stack, &fuzzyNode{
						pathNode: pathNode{
							prev:                   p,
							stemTransition:         node.stemTransition,
							state:                  t.to,
							pos:                    node.pos,
							inputLength:            virtualInputLen,
							phonetic:               phonetic,
							depth:                  node.depth + 1,
							hasSurface:             node.hasSurface,
							hasDerivation:          node.hasDerivation || isDeriv,
							currentGroupHasSurface: !isDeriv && node.currentGroupHasSurface,
							lastDerivation:         lastDerivation,
						},
						word:       node.word,
						wordLen:    node.wordLen,
						row:        node.row,
						prevRow:    node.prevRow,
						lastCode:   node.lastCode,
						derivCount: derivCount,
					})
					continue
				}

				if phonetic&t.forbiddenPhonetic != 0 {
					continue
				}
				e := resolveSurfaceAndNextPhonetic(t, phonetic)
				if e == nil {
					continue
				}
				newLen := node.wordLen + len(e.runes)
				if newLen > L+maxEdits {
					continue
				}

				curRow, curPrev, curCode, curLen := node.row, node.prevRow, node.lastCode, node.wordLen
				ok := true
				for _, r := range e.runes {
					code := foldCode(r)
					curLen++
					nextRow := advanceOneCode(curRow, curPrev, code, curCode, curLen)
					if nextRow == nil {
						ok = false
						break
					}
					curPrev, curRow, curCode = curRow, nextRow, code
				}
				if !ok {
					continue
				}
				if t.condition != nil && !t.condition(p) {
					continue
				}
				if node.depth > 4 && exceedsLocalRepeat(p, t.to.id) {
					continue
				}
				stack = append(stack, &fuzzyNode{
					pathNode: pathNode{
						prev:                   p,
						stemTransition:         node.stemTransition,
						state:                  t.to,
						surface:                e.surface,
						pos:                    newLen,
						inputLength:            virtualInputLen,
						phonetic:               int(e.nextPhonetic),
						depth:                  node.depth + 1,
						hasSurface:             true,
						hasDerivation:          node.hasDerivation || isDeriv,
						currentGroupHasSurface: !isDeriv,
						lastDerivation:         lastDerivation,
					},
					word:       node.word + e.surface,
					wordLen:    newLen,
					row:        curRow,
					prevRow:    curPrev,
					lastCode:   curCode,
					derivCount: derivCount,
				})
			}
		}
	}

	// Every candidate is verified with the analyzer's isWord and given its Turkish edit cost.
	verified := []FuzzyMorphCandidate{}
	for _, cand := range found.words {
		if cand == word || abs(runeLen(cand)-L) > maxEdits {
			continue
		}
		if !f.analyzer.isWord(cand) {
			continue
		}
		verified = append(verified, FuzzyMorphCandidate{Word: cand, FoldedEdits: found.edits[cand], EditCost: TurkishEditCost(word, cand)})
	}
	sort.SliceStable(verified, func(i, j int) bool {
		a, b := verified[i], verified[j]
		if d := a.EditCost - b.EditCost; d != 0 {
			return d < 0
		}
		return a.FoldedEdits < b.FoldedEdits
	})

	if len(f.cache) >= maxFuzzyCache {
		clear(f.cache)
	}
	f.cache[cacheKey] = verified
	return slices.Clone(verified[:jsSliceEnd(maxResults, len(verified))])
}

// min3 is the smallest of three distances, chosen as the TypeScript's nested conditional chooses it.
func min3(sub, del, ins int) int {
	if sub < del {
		if sub < ins {
			return sub
		}
		return ins
	}
	if del < ins {
		return del
	}
	return ins
}

// abs is the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
