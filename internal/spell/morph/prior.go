package morph

// This file is prior.ts: the generative prior of a valid word form that may never have been seen,
//
//	P_hat(form) = max over the analyses a of form: C_lemma(a.stem, a.pos) * P(a.morphemes | a.pos),
//
// from the corpus probabilities of suffix chains, a smoothed morpheme-bigram backoff for unseen or long chains,
// and the quantized lemma counts of morph.bin's AMR2 block. The TypeScript's buildMorphPriorFromCorpora, which
// makes the prior from corpora when morph.bin is built, is not ported: the app only reads morph.bin.

import (
	"errors"
	"math"
	"sort"
	"strings"
)

// The MST1 block's magic.
var priorMagic = [4]byte{'M', 'S', 'T', '1'}

// primaryPosList is the primary parts of speech by their index in morph.bin.
var primaryPosList = []PrimaryPos{
	"Noun", "Adj", "Verb", "Pron", "Adv", "Conj", "Ques", "Postp", "Det", "Num", "Dup", "Interj", "Punc",
}

const (
	lemmaLogMax      = 17.0 // exp(17) ~ 24M
	probNegLogMax    = 20.0 // exp(-20) ~ 2e-9
	unseenBigramProb = 1e-4
)

// quantizeLemmaCount is a lemma count as a byte, its logarithm in 255 steps.
func quantizeLemmaCount(count float64) byte {
	if count <= 0 {
		return 0
	}
	lg := math.Log(count + 1)
	return byte(max(1, min(255, jsRound(lg/lemmaLogMax*255))))
}

// dequantizeLemmaCount is the lemma count of a quantized byte.
func dequantizeLemmaCount(q byte) float64 {
	if q == 0 {
		return 0
	}
	return jsExp(float64(q)*lemmaLogMax/255) - 1
}

// quantizeProb is a probability as a byte, its negative logarithm in 255 steps.
func quantizeProb(prob float64) byte {
	if prob <= 0 {
		return 255
	}
	nlp := -math.Log(prob)
	if nlp <= 0 {
		return 0
	}
	return byte(max(0, min(255, jsRound(nlp/probNegLogMax*255))))
}

// dequantizeProb is the probability of a quantized byte.
func dequantizeProb(q byte) float64 {
	return jsExp(-float64(q) * probNegLogMax / 255)
}

// orderedFloats is a map from strings to numbers that remembers the order keys were first set in, as a
// JavaScript Map does: the prior sums its lemma counts in that order, and a sum of floats depends on it.
type orderedFloats struct {
	keys   []string
	values map[string]float64
}

// newOrderedFloats is an empty orderedFloats.
func newOrderedFloats() *orderedFloats { return &orderedFloats{values: map[string]float64{}} }

// set sets a key's value, keeping its place when it has one.
func (m *orderedFloats) set(key string, value float64) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// morphPrior estimates the expected corpus frequency of any morphologically valid Turkish word from its lemma
// frequency and the probability of its suffix chain.
type morphPrior struct {
	lemmaCounts map[string]float64
	stemTotals  map[string]float64
	chainProbs  map[string]float64
	bigramProbs map[string]float64
}

// newMorphPrior is a prior of its lemma counts ("stem|pos"), suffix-chain probabilities ("pos|M1+M2") and
// morpheme-bigram probabilities ("prev>next", "^pos" starting a chain and "$" ending it).
func newMorphPrior(lemmaCounts *orderedFloats, chainProbs, bigramProbs map[string]float64) *morphPrior {
	p := &morphPrior{
		lemmaCounts: lemmaCounts.values,
		stemTotals:  map[string]float64{},
		chainProbs:  chainProbs,
		bigramProbs: bigramProbs,
	}
	for _, key := range lemmaCounts.keys {
		bar := strings.IndexByte(key, '|')
		if bar == -1 {
			continue
		}
		stem := key[:bar]
		p.stemTotals[stem] = p.stemTotals[stem] + lemmaCounts.values[key]
	}
	return p
}

// lemmaCount is the total corpus frequency of (stem, pos), with a conservative fallback across parts of speech
// for homographs.
func (p *morphPrior) lemmaCount(stem, pos string) float64 {
	if exact, ok := p.lemmaCounts[stem+"|"+pos]; ok && exact > 0 {
		return exact
	}
	if total, ok := p.stemTotals[stem]; ok {
		return float64(total * 0.2)
	}
	return 0
}

// stemTotal is the corpus frequency of stem across all parts of speech, to order the fuzzy search's stems.
func (p *morphPrior) stemTotal(stem string) float64 { return p.stemTotals[stem] }

// chainProb is P(morphemes | pos): the stored probability of the chain, or the morpheme-bigram backoff.
func (p *morphPrior) chainProb(pos string, morphemes []string) float64 {
	if exact, ok := p.chainProbs[pos+"|"+strings.Join(morphemes, "+")]; ok {
		return exact
	}
	// The smoothed morpheme-bigram backoff for unseen or long suffix chains.
	prob := 1.0
	prev := "^" + pos
	for _, cur := range morphemes {
		prob = float64(prob * p.bigram(prev+">"+cur))
		prev = cur
	}
	prob = float64(prob * p.bigram(prev+">$"))
	// A backed-off chain is damped a little so that an attested chain wins a tie.
	return float64(prob * 0.5)
}

// bigram is a bigram's probability, unseenBigramProb for an unseen one.
func (p *morphPrior) bigram(key string) float64 {
	if v, ok := p.bigramProbs[key]; ok {
		return v
	}
	return unseenBigramProb
}

// expectedCount is the expected corpus count of a valid word form, the best over its analyses; 0 without any.
func (p *morphPrior) expectedCount(analyses []MorphAnalysis) float64 {
	best := 0.0
	for _, a := range analyses {
		lc := p.lemmaCount(a.Stem, a.Pos)
		if lc <= 0 {
			continue
		}
		est := float64(lc * p.chainProb(a.Pos, a.Morphemes))
		if est > best {
			best = est
		}
	}
	return best
}

// encodeMorphPrior writes a prior as an MST1 block aligned with the sorted dictionary items. The TypeScript
// sorts the chains and bigrams with localeCompare; here they are sorted by their bytes, which changes only the
// block's layout, not what it decodes to.
func encodeMorphPrior(p *morphPrior, sortedItems []*dictionaryItem) ([]byte, error) {
	itemCount := len(sortedItems)
	lemmaCol := make([]byte, itemCount)
	for i, item := range sortedItems {
		lemmaCol[i] = quantizeLemmaCount(p.lemmaCounts[item.root+"|"+item.primaryPos])
	}

	symToID := map[string]int{}
	var symList []string
	var overflow error
	intern := func(sym string) byte {
		id, ok := symToID[sym]
		if !ok {
			id = len(symList)
			if id > 255 {
				overflow = errors.New("morph prior: symbol table overflow")
			}
			symToID[sym] = id
			symList = append(symList, sym)
		}
		return byte(id)
	}

	var chainBytes []byte
	chainCount := 0
	for _, key := range sortedKeys(p.chainProbs) {
		bar := strings.IndexByte(key, '|')
		posIdx := indexOf(primaryPosList, key[:bar])
		if posIdx == -1 {
			continue
		}
		rest := key[bar+1:]
		var morphs []string
		if rest != "" {
			morphs = strings.Split(rest, "+")
		}
		if len(morphs) > 255 {
			continue
		}
		chainBytes = append(chainBytes, byte(posIdx), byte(len(morphs)))
		for _, m := range morphs {
			chainBytes = append(chainBytes, intern(m))
		}
		chainBytes = append(chainBytes, quantizeProb(p.chainProbs[key]))
		chainCount++
	}

	var bigramBytes []byte
	for _, key := range sortedKeys(p.bigramProbs) {
		gt := strings.IndexByte(key, '>')
		prev, next := intern(key[:gt]), intern(key[gt+1:])
		bigramBytes = append(bigramBytes, prev, next, quantizeProb(p.bigramProbs[key]))
	}
	if overflow != nil {
		return nil, overflow
	}

	var symBytes []byte
	for _, sym := range symList {
		symBytes = append(symBytes, byte(len(sym)))
		symBytes = append(symBytes, sym...)
	}

	out := append([]byte{}, priorMagic[:]...)
	out = appendUint16(out, uint16(len(symList)))
	out = appendUint32(out, uint32(len(symBytes)))
	out = appendUint32(out, uint32(chainCount))
	out = appendUint32(out, uint32(len(chainBytes)))
	out = appendUint32(out, uint32(len(bigramBytes)/3))
	out = append(out, lemmaCol...)
	out = append(out, symBytes...)
	out = append(out, chainBytes...)
	return append(out, bigramBytes...), nil
}

// sortedKeys is the keys of m in byte order.
func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// indexOf is the index of s in list, -1 when it is not there.
func indexOf(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return -1
}

// errTruncatedPrior is the error of an MST1 block that ends too soon.
var errTruncatedPrior = errors.New("morph: truncated morphology prior block (MST1)")

// listItem is list[i], or "undefined" as JavaScript writes an index out of range into a template string.
func listItem(list []string, i int) string {
	if i < len(list) {
		return list[i]
	}
	return "undefined"
}

// decodeMorphPrior reads the MST1 block at start in b, whose lemma counts are aligned with sortedItems.
func decodeMorphPrior(b []byte, start int, sortedItems []*dictionaryItem) (*morphPrior, error) {
	if len(b)-start < 22 || [4]byte(b[start:start+4]) != priorMagic {
		return nil, errors.New("morph: invalid morphology prior block (MST1 expected)")
	}
	off := start + 4
	symCount := int(le16(b[off:]))
	off += 2
	symByteLen := int(le32(b[off:]))
	off += 4
	chainCount := int(le32(b[off:]))
	off += 4
	chainByteLen := int(le32(b[off:]))
	off += 4
	bigramCount := int(le32(b[off:]))
	off += 4

	itemCount := len(sortedItems)
	if off+itemCount > len(b) {
		return nil, errTruncatedPrior
	}
	lemmaCol := b[off : off+itemCount]
	off += itemCount

	symList := make([]string, symCount)
	symPos := off
	for i := range symList {
		if symPos >= len(b) {
			return nil, errTruncatedPrior
		}
		n := int(b[symPos])
		symPos++
		if symPos+n > len(b) {
			return nil, errTruncatedPrior
		}
		symList[i] = string(b[symPos : symPos+n])
		symPos += n
	}
	off += symByteLen

	chainEnd := off + chainByteLen
	if chainEnd > len(b) {
		return nil, errTruncatedPrior
	}
	chainProbs := map[string]float64{}
	cPos := off
	for i := 0; i < chainCount && cPos < chainEnd; i++ {
		if cPos+2 > len(b) {
			return nil, errTruncatedPrior
		}
		pos := listItem(primaryPosList, int(b[cPos]))
		n := int(b[cPos+1])
		cPos += 2
		if cPos+n+1 > len(b) {
			return nil, errTruncatedPrior
		}
		parts := make([]string, n)
		for j := range parts {
			parts[j] = listItem(symList, int(b[cPos]))
			cPos++
		}
		chainProbs[pos+"|"+strings.Join(parts, "+")] = dequantizeProb(b[cPos])
		cPos++
	}
	off = chainEnd

	if off+bigramCount*3 > len(b) {
		return nil, errTruncatedPrior
	}
	bigramProbs := make(map[string]float64, bigramCount)
	for i := 0; i < bigramCount; i++ {
		prev, next := listItem(symList, int(b[off])), listItem(symList, int(b[off+1]))
		bigramProbs[prev+">"+next] = dequantizeProb(b[off+2])
		off += 3
	}

	lemmaCounts := newOrderedFloats()
	for i, q := range lemmaCol {
		if q == 0 {
			continue
		}
		item := sortedItems[i]
		key := item.root + "|" + item.primaryPos
		val := dequantizeLemmaCount(q)
		if existing, ok := lemmaCounts.values[key]; !ok || val > existing {
			lemmaCounts.set(key, val)
		}
	}
	return newMorphPrior(lemmaCounts, chainProbs, bigramProbs), nil
}
