package morph

// This file is analyzer.ts: the forward search of the graph that validates and analyzes word forms. It is
// adapted from Zemberek's RuleBasedAnalyzer.java, around offsets into the normalized input instead of
// substrings, immutable linked path nodes, the transitions' 64-entry surface caches, and a depth-first search
// from the longest stem that stops at the first accepted path for isWord.

import (
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

const (
	maxRepeatingStateCount = 3
	maxSearchDepth         = 40
	maxCacheEntries        = 65_536
)

// exceedsStateRepeatLimit reports whether the path already passed through the state with nextStateID as often
// as allowed.
func exceedsStateRepeatLimit(node *pathNode, nextStateID string) bool {
	count := 0
	for cur := node; cur != nil; cur = cur.prev {
		if cur.state.id == nextStateID {
			count++
			if count >= maxRepeatingStateCount {
				return true
			}
		}
	}
	return false
}

// lemmaStemOf is the lemma a circumflexed input is checked against: the referenced item's for a Dummy root, and
// without -mak/-mek for a verb.
func lemmaStemOf(st *stemTransition) []rune {
	item := st.item
	if item.attributes&raDummy != 0 && item.referenceItem != nil {
		item = item.referenceItem
	}
	if item.primaryPos == "Verb" && (strings.HasSuffix(item.lemma, "mak") || strings.HasSuffix(item.lemma, "mek")) {
		return []rune(item.lemma[:len(item.lemma)-3])
	}
	return []rune(item.lemma)
}

// matchesCircumflex reports whether the circumflexes of the input are where the stem's lemma has them; a suffix
// never has one.
func matchesCircumflex(st *stemTransition, inputWithCircumflex []rune) bool {
	stemLen := len(st.runes)
	var lemmaStem []rune
	for i, ch := range inputWithCircumflex {
		if ch != 'â' && ch != 'î' && ch != 'û' {
			continue
		}
		if i >= stemLen {
			return false
		}
		if lemmaStem == nil {
			lemmaStem = lemmaStemOf(st)
		}
		if i >= len(lemmaStem) || lemmaStem[i] != ch {
			return false
		}
	}
	return true
}

// makeInitialNode is the first node of a path, at the end of its stem.
func makeInitialNode(stem *stemTransition, inputLength int) *pathNode {
	return &pathNode{
		stemTransition: stem,
		state:          stem.to,
		surface:        stem.surface,
		pos:            len(stem.runes),
		inputLength:    inputLength,
		phonetic:       stem.phonetic,
	}
}

// stepNode is the node after prev takes a transition with its surface.
func stepNode(prev *pathNode, t *suffixTransition, surface string, surfaceLen int, nextPhonetic int) *pathNode {
	isDeriv := t.to.derivative
	hasSurf := surfaceLen > 0
	n := &pathNode{
		prev:                   prev,
		stemTransition:         prev.stemTransition,
		state:                  t.to,
		surface:                surface,
		pos:                    prev.pos + surfaceLen,
		inputLength:            prev.inputLength,
		phonetic:               nextPhonetic,
		depth:                  prev.depth + 1,
		hasSurface:             prev.hasSurface || hasSurf,
		hasDerivation:          prev.hasDerivation || isDeriv,
		currentGroupHasSurface: !isDeriv && (prev.currentGroupHasSurface || hasSurf),
		lastDerivation:         prev.lastDerivation,
	}
	if isDeriv {
		n.lastDerivation = t.to
	}
	return n
}

// pathToAnalysis is the analysis a path that reached the end of the input stands for.
func pathToAnalysis(endNode *pathNode) MorphAnalysis {
	var chain []*pathNode
	for cur := endNode; cur != nil; cur = cur.prev {
		chain = append(chain, cur)
	}
	item := endNode.stemTransition.item
	if item.attributes&raDummy != 0 && item.referenceItem != nil {
		item = item.referenceItem
	}
	morphemes := []string{}
	// The last of the chain is the root's own state; empty Pnon and Nom are noise.
	for i := len(chain) - 2; i >= 0; i-- {
		m := chain[i].state.morpheme
		if m == allMorphemes.nom || m == allMorphemes.pnon {
			continue
		}
		morphemes = append(morphemes, m.id)
	}
	return MorphAnalysis{Stem: item.root, Pos: item.primaryPos, Morphemes: morphemes}
}

// searchInput is a normalized input: its text, its runes, and the byte offset of each rune (and of the end) so
// that its prefixes are looked up without copying.
type searchInput struct {
	text  string
	runes []rune
	offs  []int
}

// newSearchInput is the search input of lower, its circumflexes normalized when it has some.
func newSearchInput(lower string, inputHasCircumflex bool) *searchInput {
	text := lower
	if inputHasCircumflex {
		text = normalizeCircumflex(lower)
	}
	in := &searchInput{text: text, runes: make([]rune, 0, len(text)), offs: make([]int, 0, len(text)+1)}
	for i, r := range text {
		in.runes = append(in.runes, r)
		in.offs = append(in.offs, i)
	}
	in.offs = append(in.offs, len(text))
	return in
}

// prefix is the input's first n runes.
func (in *searchInput) prefix(n int) string { return in.text[:in.offs[n]] }

// morphologicalAnalyzer validates and analyzes word forms over a stem index.
type morphologicalAnalyzer struct {
	stemIndex *stemIndex
	wordCache map[string]bool
}

// newMorphologicalAnalyzer is an analyzer of the stems of idx.
func newMorphologicalAnalyzer(idx *stemIndex) *morphologicalAnalyzer {
	return &morphologicalAnalyzer{stemIndex: idx, wordCache: map[string]bool{}}
}

// isWord reports whether form is a valid Turkish word by the lexicon and the morphotactics, from a bounded cache
// or a depth-first search from the longest stem.
func (a *morphologicalAnalyzer) isWord(form string) bool {
	if form == "" {
		return false
	}
	if cached, ok := a.wordCache[form]; ok {
		return cached
	}
	lower := tr.Lower(form)
	if lower != form {
		if cached, ok := a.wordCache[lower]; ok {
			a.remember(form, cached)
			return cached
		}
	}
	valid := a.checkWordUncached(lower)
	a.remember(lower, valid)
	if lower != form {
		a.remember(form, valid)
	}
	return valid
}

// pushStems pushes the initial nodes of the stems that are the input's first n runes.
func (a *morphologicalAnalyzer) pushStems(stack []*pathNode, in *searchInput, n int, inputHasCircumflex bool, lower []rune) []*pathNode {
	for _, st := range a.stemIndex.stems[in.prefix(n)] {
		if !inputHasCircumflex || matchesCircumflex(st, lower) {
			stack = append(stack, makeInitialNode(st, len(in.runes)))
		}
	}
	return stack
}

// analyze is the distinct analyses of form.
func (a *morphologicalAnalyzer) analyze(form string) []MorphAnalysis {
	if form == "" {
		return []MorphAnalysis{}
	}
	lower := tr.Lower(form)
	if lower == "" {
		return []MorphAnalysis{}
	}
	inputHasCircumflex := hasCircumflex(lower)
	in := newSearchInput(lower, inputHasCircumflex)
	var lowerRunes []rune
	if inputHasCircumflex {
		lowerRunes = []rune(lower)
	}
	inputLen := len(in.runes)
	maxLen := min(inputLen, a.stemIndex.maxStemLength)

	results := []MorphAnalysis{}
	seenKeys := map[string]bool{}
	var stack []*pathNode
	for n := 1; n <= maxLen; n++ {
		stack = a.pushStems(stack, in, n, inputHasCircumflex, lowerRunes)
	}

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.pos == inputLen && node.state.terminal && node.phonetic&paCannotTerminate == 0 {
			analysis := pathToAnalysis(node)
			dedupKey := analysis.Stem + "|" + analysis.Pos + "|" + strings.Join(analysis.Morphemes, "+")
			if !seenKeys[dedupKey] {
				seenKeys[dedupKey] = true
				results = append(results, analysis)
			}
		}
		if node.depth >= maxSearchDepth {
			continue
		}
		stack = a.expandNode(node, in.runes, inputLen, stack)
	}
	return results
}

// remember caches a word's validity, emptying the cache when it is full.
func (a *morphologicalAnalyzer) remember(key string, value bool) {
	if len(a.wordCache) >= maxCacheEntries {
		clear(a.wordCache)
	}
	a.wordCache[key] = value
}

// checkWordUncached searches for a valid analysis of lower, trying the longest stems first so that uninflected
// and lightly inflected words are accepted at once.
func (a *morphologicalAnalyzer) checkWordUncached(lower string) bool {
	if lower == "" {
		return false
	}
	inputHasCircumflex := hasCircumflex(lower)
	in := newSearchInput(lower, inputHasCircumflex)
	var lowerRunes []rune
	if inputHasCircumflex {
		lowerRunes = []rune(lower)
	}
	inputLen := len(in.runes)
	maxLen := min(inputLen, a.stemIndex.maxStemLength)

	var stack []*pathNode
	for n := maxLen; n >= 1; n-- {
		stack = a.pushStems(stack, in, n, inputHasCircumflex, lowerRunes)
		if len(stack) > 0 {
			var accepted bool
			if stack, accepted = a.dfsAccepts(stack, in.runes, inputLen); accepted {
				return true
			}
		}
	}
	return false
}

// dfsAccepts searches from the nodes of stack until a path reaches the end of the input in a terminal state.
func (a *morphologicalAnalyzer) dfsAccepts(stack []*pathNode, input []rune, inputLen int) ([]*pathNode, bool) {
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.pos == inputLen && node.state.terminal && node.phonetic&paCannotTerminate == 0 {
			return stack, true
		}
		if node.depth >= maxSearchDepth {
			continue
		}
		stack = a.expandNode(node, input, inputLen, stack)
	}
	return stack, false
}

// expandNode pushes the steps that can follow node and match the input.
func (a *morphologicalAnalyzer) expandNode(node *pathNode, input []rune, inputLen int, stack []*pathNode) []*pathNode {
	remaining := inputLen - node.pos
	phonetic := node.phonetic
	for _, t := range node.state.outgoing {
		// Empty (epsilon) transitions.
		if !t.hasSurface {
			if t.condition != nil && !t.condition(node) {
				continue
			}
			if node.depth > 8 && exceedsStateRepeatLimit(node, t.to.id) {
				continue
			}
			stack = append(stack, stepNode(node, t, "", 0, phonetic))
			continue
		}
		if remaining == 0 {
			continue
		}
		// The expectation of a vowel or a consonant, checked before the surface.
		if phonetic&t.forbiddenPhonetic != 0 {
			continue
		}
		e := resolveSurfaceAndNextPhonetic(t, phonetic)
		if e == nil {
			continue
		}
		surfLen := len(e.runes)
		if surfLen > remaining {
			continue
		}
		if surfLen > 0 && !runesHavePrefixAt(input, node.pos, e.runes) {
			continue
		}
		if t.condition != nil && !t.condition(node) {
			continue
		}
		if node.depth > 8 && exceedsStateRepeatLimit(node, t.to.id) {
			continue
		}
		stack = append(stack, stepNode(node, t, e.surface, surfLen, int(e.nextPhonetic)))
	}
	return stack
}

// runesHavePrefixAt reports whether input has prefix at pos.
func runesHavePrefixAt(input []rune, pos int, prefix []rune) bool {
	if pos+len(prefix) > len(input) {
		return false
	}
	for i, r := range prefix {
		if input[pos+i] != r {
			return false
		}
	}
	return true
}
