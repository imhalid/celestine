package morph

// This file is index.ts: the morphology's entry point, TurkishMorphology.

import "sync"

const maxPriorCache = 16_384

// Morphology is the Turkish morphology: whether a lowercase Turkish form is valid by Turkish phonology and
// morphotactics, its analyses, its generative prior count, and the valid forms near a typo. Its methods are safe
// for concurrent use; they take turns, as the caches they share are the TypeScript's and change with each call.
type Morphology struct {
	mu         sync.Mutex
	analyzer   *morphologicalAnalyzer
	prior      *morphPrior
	priorCache map[string]float64
	fuzzy      *fuzzyMorphSearch
}

// newMorphology is a morphology of the dictionary items and the prior, which may be nil.
func newMorphology(items []*dictionaryItem, prior *morphPrior) *Morphology {
	graph := newMorphotacticsGraph()
	return &Morphology{
		analyzer:   newMorphologicalAnalyzer(buildStemIndex(items, graph)),
		prior:      prior,
		priorCache: map[string]float64{},
	}
}

// FromDictionaries is a morphology of Zemberek .dict texts, without a prior.
func FromDictionaries(texts ...string) *Morphology {
	return newMorphology(parseDictionaries(texts), nil)
}

// FromBinary is a morphology of a compact binary dictionary (AMR1, or AMR2 with its prior: morph.bin).
func FromBinary(b []byte) (*Morphology, error) {
	items, prior, err := parseBinaryDictionaryWithPrior(b)
	if err != nil {
		return nil, err
	}
	return newMorphology(items, prior), nil
}

// IsWord reports whether form, lowercased the Turkish way, is a valid word by Turkish suffix rules.
func (m *Morphology) IsWord(form string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.analyzer.isWord(form)
}

// Analyze is the analyses of form (stem, part of speech, morphemes), for inspection and tests.
func (m *Morphology) Analyze(form string) []MorphAnalysis {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.analyzer.analyze(form)
}

// HasPrior reports whether the generative prior (stem frequency times suffix-chain probability) is loaded.
func (m *Morphology) HasPrior() bool { return m.prior != nil }

// PriorCount is the expected corpus frequency of a valid Turkish form from its stem frequency and suffix-chain
// probability; 0 without a prior or for an invalid word.
func (m *Morphology) PriorCount(form string) float64 {
	if m.prior == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cached, ok := m.priorCache[form]; ok {
		return cached
	}
	est := m.prior.expectedCount(m.analyzer.analyze(form))
	if len(m.priorCache) >= maxPriorCache {
		clear(m.priorCache)
	}
	m.priorCache[form] = est
	return est
}

// LemmaCount is the corpus frequency of a stem and part of speech together.
func (m *Morphology) LemmaCount(stem, pos string) float64 {
	if m.prior == nil {
		return 0
	}
	return m.prior.lemmaCount(stem, pos)
}

// FuzzySearch is the valid Turkish forms, unseen ones included, within lost Turkish marks and maxEdits (1 or 2)
// edits of form, ordered by edit cost, at most maxResults of them. The TypeScript's defaults are 1 and 40.
func (m *Morphology) FuzzySearch(form string, maxEdits, maxResults int) []FuzzyMorphCandidate {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fuzzy == nil {
		m.fuzzy = newFuzzyMorphSearch(m.analyzer.stemIndex, m.analyzer, m.prior)
	}
	return m.fuzzy.search(form, maxEdits, maxResults)
}
