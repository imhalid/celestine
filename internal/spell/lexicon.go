// Package spell is the Turkish spell checker of the editor, a port of tinyfix-tr (#623): its lexicon of word and
// word-pair counts, its list of known misspellings, its rules of Turkish spelling and proper names, and the corrector
// that uses them. Each file is one of the TypeScript engine's, so the two read side by side; the answers are the
// engine's, which the parity tests check.
package spell

import (
	"math"
	"strings"
)

// This file is tinyfix-tr's src/lexicon.ts.

// Lexicon is word-form and word-pair counts from edited Turkish text. Forms are lowercased the Turkish way;
// "ankara'ya" keeps its apostrophe. It is read from TSV texts or from a binary ALX1 lexicon.
type Lexicon struct {
	counts      map[string]float64
	countOrder  []string
	capitalized map[string]float64
	pairs       map[string]float64
	pairOrder   []string
	binary      *BinaryLexicon
}

// LexiconFromTSV reads a lexicon from its TSV texts: unigrams are "form\tcount\tcapitalizedMidSentence" lines,
// bigrams "first second\tcount" lines. Forms seen fewer than minCount times and pairs fewer than minPair are left
// out (the TypeScript's defaults are 1 and 1).
func LexiconFromTSV(unigrams, bigrams string, minCount, minPair float64) *Lexicon {
	l := &Lexicon{counts: map[string]float64{}, capitalized: map[string]float64{}, pairs: map[string]float64{}}
	for _, line := range strings.Split(unigrams, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		form := fields[0]
		value := math.NaN() // Number(undefined)
		if len(fields) > 1 {
			value = jsNumber(fields[1])
		}
		if value < minCount {
			continue
		}
		if _, ok := l.counts[form]; !ok {
			l.countOrder = append(l.countOrder, form)
		}
		l.counts[form] = value
		if caps := field(fields, 2); caps != "" && caps != "0" {
			l.capitalized[form] = jsNumber(caps)
		}
	}
	for _, line := range strings.Split(bigrams, "\n") {
		if line == "" {
			continue
		}
		var key, count string
		if tab := strings.LastIndexByte(line, '\t'); tab >= 0 {
			key, count = line[:tab], line[tab+1:]
		} else {
			// line.slice(0, -1) and line.slice(0): the whole line is the count, and the key loses its last letter.
			key, count = sliceRunes(line, 0, -1), line
		}
		if value := jsNumber(count); value >= minPair {
			if _, ok := l.pairs[key]; !ok {
				l.pairOrder = append(l.pairOrder, key)
			}
			l.pairs[key] = value
		}
	}
	return l
}

// field is fields[i], or "" past the end (where the TypeScript's destructuring gives undefined).
func field(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}

// LexiconFromBinary reads a compact ALX1 lexicon (lexicon.bin) straight into memory, parsing no TSV. The lexicon
// keeps bytes.
func LexiconFromBinary(bytes []byte) (*Lexicon, error) {
	b, err := NewBinaryLexicon(bytes)
	if err != nil {
		return nil, err
	}
	return &Lexicon{binary: b}, nil
}

// Count is how often form was seen (for a binary lexicon, a rare form in a Bloom tier counts too).
func (l *Lexicon) Count(form string) float64 {
	if l.binary != nil {
		return l.binary.Count(form)
	}
	return l.counts[form]
}

// ExactCount is the exact table's count only: a binary lexicon's protection-only Bloom tiers are skipped, for
// looking up the targets of a fix.
func (l *Lexicon) ExactCount(form string) float64 {
	if l.binary != nil {
		return l.binary.ExactCount(form)
	}
	return l.counts[form]
}

// Pair is how often first was followed by second.
func (l *Lexicon) Pair(first, second string) float64 {
	if l.binary != nil {
		return l.binary.Pair(first, second)
	}
	return l.pairs[first+" "+second]
}

// CapitalShare is the share of form's occurrences written with a capital mid-sentence: near 1 for names like
// "ankara".
func (l *Lexicon) CapitalShare(form string) float64 {
	if l.binary != nil {
		return l.binary.CapitalShare(form)
	}
	count := l.Count(form)
	if count == 0 {
		return 0
	}
	return l.capitalized[form] / count
}

// Size is the number of forms.
func (l *Lexicon) Size() int {
	if l.binary != nil {
		return l.binary.Size()
	}
	return len(l.counts)
}

// PairCount is the number of word pairs.
func (l *Lexicon) PairCount() int {
	if l.binary != nil {
		return l.binary.PairCount()
	}
	return len(l.pairs)
}

// ToTSV is the lexicon as the two TSV texts it was read from, for measuring what a pruned copy would weigh.
func (l *Lexicon) ToTSV() (unigrams, bigrams string) {
	if l.binary != nil {
		return l.binary.ToTSV()
	}
	u := make([]string, len(l.countOrder))
	for i, form := range l.countOrder {
		u[i] = form + "\t" + jsNumberString(l.counts[form]) + "\t" + jsNumberString(l.capitalized[form])
	}
	b := make([]string, len(l.pairOrder))
	for i, pair := range l.pairOrder {
		b[i] = pair + "\t" + jsNumberString(l.pairs[pair])
	}
	return strings.Join(u, "\n"), strings.Join(b, "\n")
}
