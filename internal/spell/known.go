package spell

import (
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/known.ts.

// knownRule is a known misspelling and its fix, checked against corpus counts by tinyfix-tr's py/verify_lists.py.
type knownRule struct {
	wrong      string
	right      string
	rightCount float64
	// contraction marks a deliberate spoken-style form ("noldu", "burda"), fixed only when the writer asks for it.
	contraction bool
}

// KnownFix is a fix KnownMisspellings.Find offers.
type KnownFix struct {
	Text string
	// Count is how often the fix is seen, for comparing with what was typed.
	Count float64
	// JoinsNext is true when the rule covers the next word too ("hiç bir" -> "hiçbir").
	JoinsNext bool
	// Contraction is true for spoken-style contractions ("noldu" -> "ne oldu").
	Contraction bool
}

// NameEntry is a proper name of the list.
type NameEntry struct {
	Name  string
	Count float64
	// NoApos is true when suffixes on this proper name take no apostrophe per TDK (languages, derivations,
	// religions like "Türkçe", "Hinduizm").
	NoApos bool
}

// PhraseEntry is a multi-word proper phrase.
type PhraseEntry struct {
	Words        []string
	DisplayWords []string
	// TakesApos is true when inflectional suffixes on the final word take an apostrophe (places, events, works);
	// false for institutions.
	TakesApos bool
	// Context is the gate of a phrase that overlaps with ordinary Turkish prose ("era", "day", "work"), or "".
	Context PhraseContext
}

// acronymEntry is an acronym's capitals and how often it is seen.
type acronymEntry struct {
	upper string
	count float64
}

const maxKnownSuffix = 6

var knownSuffix = regexp.MustCompile(`^[a-zçğıöşü]+$`)

// KnownMisspellings are misspellings people make often enough to list: "herkez" -> "herkes", "birşey" ->
// "bir şey", "hiç bir" -> "hiçbir". A rule also covers the wrong form with a suffix ("herkezin" -> "herkesin") when
// the fixed form with that suffix is a word the lexicon has seen, so "yalnış" never reaches into "yalnızlık".
//
// It also keeps the proper-name tables appended to known.tsv for the browser bundle: NAME, ACRO, GIVEN, SURNAME,
// AMBIG, PHRASE_APOS, PHRASE_NOAPOS.
type KnownMisspellings struct {
	single      map[string]knownRule
	byFirstWord map[string][]knownRule
	// byPlainFirstWord are the two-word rules with Turkish letters, keyed by the first word without them ("hic" for
	// "hiç bir").
	byPlainFirstWord map[string][]knownRule
	stems            []knownRule
	// names are the proper names always written with a capital ("ahmet" -> "Ahmet").
	names map[string]NameEntry
	// asciiNames are the proper names with Turkish letters by their ASCII-folded form ("canakkale" -> "Çanakkale");
	// nil where two names fold alike.
	asciiNames map[string]*NameEntry
	// acronyms are the all-caps acronyms ("tbmm" -> "TBMM", "nato" -> "NATO").
	acronyms map[string]acronymEntry
	// asciiAcronyms are the acronyms with Turkish letters by their folded form ("itu" -> "İTÜ"); nil where two fold
	// alike.
	asciiAcronyms map[string]*acronymEntry
	// givenNames are Turkish given names, ambiguous ones like "deniz", "umut", "defne" included, for person-frame
	// rules.
	givenNames map[string]string
	// surnames are Turkish surnames, ambiguous ones like "çelik", "şahin", "kaya" included.
	surnames map[string]string
	// ambigNames are names that are also ordinary Turkish words, for disambiguating by context.
	ambigNames map[string]string
	// phrasesByFirst are the multi-word proper phrases by their lowercased first word, longest first.
	phrasesByFirst map[string][]PhraseEntry
	// phrasesByPlainFirst are the same by their ASCII-folded first word, for sentences typed without Turkish letters.
	phrasesByPlainFirst map[string][]PhraseEntry
	hasCanonicalPhrases bool
}

func newKnownMisspellings() *KnownMisspellings {
	return &KnownMisspellings{
		single:              map[string]knownRule{},
		byFirstWord:         map[string][]knownRule{},
		byPlainFirstWord:    map[string][]knownRule{},
		names:               map[string]NameEntry{},
		asciiNames:          map[string]*NameEntry{},
		acronyms:            map[string]acronymEntry{},
		asciiAcronyms:       map[string]*acronymEntry{},
		givenNames:          map[string]string{},
		surnames:            map[string]string{},
		ambigNames:          map[string]string{},
		phrasesByFirst:      map[string][]PhraseEntry{},
		phrasesByPlainFirst: map[string][]PhraseEntry{},
	}
}

// KnownMisspellingsFromTSV reads the lines of data/rules/known.tsv: wrong, right, wrong count or flags, right
// count, section.
func KnownMisspellingsFromTSV(text string) *KnownMisspellings {
	known := newKnownMisspellings()
	hasProperSections := false
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		wrong, right, col3, rightCount := field(fields, 0), field(fields, 1), field(fields, 2), field(fields, 3)
		if wrong == "" || right == "" {
			continue
		}
		section := jsTrim(field(fields, 4))
		switch section {
		case "NAME":
			hasProperSections = true
			entry := NameEntry{Name: right, Count: numberOrZero(rightCount, len(fields) > 3), NoApos: strings.Contains(col3, "noapos")}
			known.names[wrong] = entry
			if folded := FoldTurkish(wrong); folded != wrong {
				prev, seen := known.asciiNames[folded]
				if !seen {
					known.asciiNames[folded] = &entry
				} else if prev != nil && prev.Name != entry.Name {
					known.asciiNames[folded] = nil
				}
			}
			continue
		case "ACRO":
			hasProperSections = true
			entry := acronymEntry{upper: right, count: numberOrZero(rightCount, len(fields) > 3)}
			known.acronyms[wrong] = entry
			if folded := FoldTurkish(wrong); folded != wrong {
				prev, seen := known.asciiAcronyms[folded]
				if !seen {
					known.asciiAcronyms[folded] = &entry
				} else if prev != nil && prev.upper != entry.upper {
					known.asciiAcronyms[folded] = nil
				}
			}
			continue
		case "GIVEN":
			hasProperSections = true
			known.givenNames[wrong] = right
			continue
		case "SURNAME":
			hasProperSections = true
			known.surnames[wrong] = right
			continue
		case "AMBIG":
			hasProperSections = true
			known.ambigNames[wrong] = right
			continue
		case "PHRASE_APOS", "PHRASE_NOAPOS":
			hasProperSections = true
			var ctx PhraseContext
			if col3 == "era" || col3 == "day" || col3 == "work" {
				ctx = PhraseContext(col3)
			}
			known.addPhrase(wrong, right, section == "PHRASE_APOS", ctx)
			continue
		}
		count := math.NaN() // Number(undefined)
		if len(fields) > 3 {
			count = jsNumber(rightCount)
		}
		rule := knownRule{wrong: wrong, right: right, rightCount: count, contraction: section == "CONTRACTION"}
		words := strings.Split(wrong, " ")
		switch len(words) {
		case 1:
			known.single[wrong] = rule
			if runeLen(wrong) >= 4 {
				known.stems = append(known.stems, rule)
			}
		case 2:
			known.byFirstWord[words[0]] = append(known.byFirstWord[words[0]], rule)
			plainFirst := plain(words[0])
			if plain(wrong) != wrong {
				known.byPlainFirstWord[plainFirst] = append(known.byPlainFirstWord[plainFirst], rule)
			}
		}
	}
	// The canonical TDK multi-word phrases are always registered when proper-name sections are present.
	if hasProperSections {
		known.ensureCanonicalPhrases()
	}
	slices.SortStableFunc(known.stems, func(a, b knownRule) int { return runeLen(b.wrong) - runeLen(a.wrong) })
	for _, list := range known.phrasesByFirst {
		slices.SortStableFunc(list, func(a, b PhraseEntry) int { return len(b.Words) - len(a.Words) })
	}
	for _, list := range known.phrasesByPlainFirst {
		slices.SortStableFunc(list, func(a, b PhraseEntry) int { return len(b.Words) - len(a.Words) })
	}
	return known
}

// numberOrZero is `Number(text) || 0`, where present is false for a missing field (undefined).
func numberOrZero(text string, present bool) float64 {
	if !present {
		return 0
	}
	v := jsNumber(text)
	if v != v { // NaN
		return 0
	}
	return v
}

// sameWords reports whether a phrase has exactly these words.
func sameWords(p PhraseEntry, words []string) bool { return slices.Equal(p.Words, words) }

func (k *KnownMisspellings) addPhrase(lowerPhrase, displayPhrase string, takesApos bool, context PhraseContext) {
	words := strings.Split(lowerPhrase, " ")
	displayWords := strings.Split(displayPhrase, " ")
	if len(words) < 2 || len(words) != len(displayWords) {
		return
	}
	first := words[0]
	existing, has := k.phrasesByFirst[first]
	dupIdx := -1
	if has {
		dupIdx = slices.IndexFunc(existing, func(p PhraseEntry) bool { return sameWords(p, words) })
	}
	if dupIdx != -1 {
		// A canonical phrase's context may refine an untagged TSV entry.
		if context != "" && existing[dupIdx].Context == "" {
			updated := existing[dupIdx]
			updated.Context = context
			existing[dupIdx] = updated
			if pList, ok := k.phrasesByPlainFirst[FoldTurkish(first)]; ok {
				if pIdx := slices.IndexFunc(pList, func(p PhraseEntry) bool { return sameWords(p, words) }); pIdx != -1 {
					pList[pIdx] = updated
				}
			}
		}
		return
	}
	entry := PhraseEntry{Words: words, DisplayWords: displayWords, TakesApos: takesApos, Context: context}
	k.phrasesByFirst[first] = append(existing, entry)
	if FoldTurkish(lowerPhrase) != lowerPhrase {
		plainFirst := FoldTurkish(first)
		k.phrasesByPlainFirst[plainFirst] = append(k.phrasesByPlainFirst[plainFirst], entry)
	}
}

func (k *KnownMisspellings) ensureCanonicalPhrases() {
	if k.hasCanonicalPhrases {
		return
	}
	k.hasCanonicalPhrases = true
	for _, p := range CanonicalPhrases {
		lower := strings.ReplaceAll(strings.ReplaceAll(p.Display, "I", "ı"), "İ", "i")
		k.addPhrase(strings.ToLower(lower), p.Display, p.Apos, p.Context)
	}
}

// NameFor is the capitalized spelling of a proper name from its lowercase form ("istanbul" -> "İstanbul"), when it
// is listed and seen at least minCount times (the TypeScript's default is 0).
func (k *KnownMisspellings) NameFor(lower string, minCount float64) (string, bool) {
	entry, ok := k.names[lower]
	if !ok || !(entry.Count >= minCount) {
		return "", false
	}
	return entry.Name, true
}

// NameEntry is a proper name's entry, which says too whether TDK forbids apostrophes on it.
func (k *KnownMisspellings) NameEntry(lower string, minCount float64) (NameEntry, bool) {
	entry, ok := k.names[lower]
	if !ok || !(entry.Count >= minCount) {
		return NameEntry{}, false
	}
	return entry, true
}

// ASCIINameEntry is the one proper name whose ASCII-folded form is foldedLower ("canakkale" -> "Çanakkale"); exact
// forms are not in this table.
func (k *KnownMisspellings) ASCIINameEntry(foldedLower string, minCount float64) (NameEntry, bool) {
	entry := k.asciiNames[foldedLower]
	if entry == nil || !(entry.Count >= minCount) {
		return NameEntry{}, false
	}
	return *entry, true
}

// AcronymFor is an acronym in capitals ("tbmm" -> "TBMM").
func (k *KnownMisspellings) AcronymFor(lower string) (string, bool) {
	entry, ok := k.acronyms[lower]
	return entry.upper, ok
}

// ASCIIAcronymFor is the one acronym whose folded form is foldedLower ("itu" -> "İTÜ").
func (k *KnownMisspellings) ASCIIAcronymFor(foldedLower string) (string, bool) {
	if entry := k.asciiAcronyms[foldedLower]; entry != nil {
		return entry.upper, true
	}
	return "", false
}

// GivenNameFor is a known Turkish given name, ambiguous ones like "deniz", "umut", "defne" included.
func (k *KnownMisspellings) GivenNameFor(lower string) (string, bool) {
	name, ok := k.givenNames[lower]
	return name, ok
}

// SurnameFor is a known Turkish surname, ambiguous ones like "çelik", "şahin", "kaya" included.
func (k *KnownMisspellings) SurnameFor(lower string) (string, bool) {
	name, ok := k.surnames[lower]
	return name, ok
}

// AmbigNameFor is a known ambiguous name: a person, place or work that is also a common Turkish noun.
func (k *KnownMisspellings) AmbigNameFor(lower string) (string, bool) {
	name, ok := k.ambigNames[lower]
	return name, ok
}

// PhrasesStartingWith are the multi-word proper phrases whose first word is firstLower, and with allowPlain also
// those whose ASCII-folded first word is its folded form. The list is the lexicon's own; it must not be changed.
func (k *KnownMisspellings) PhrasesStartingWith(firstLower string, allowPlain bool) []PhraseEntry {
	exact := k.phrasesByFirst[firstLower]
	if !allowPlain {
		return exact
	}
	plainList, ok := k.phrasesByPlainFirst[FoldTurkish(firstLower)]
	if !ok {
		return exact
	}
	if exact == nil {
		return plainList
	}
	return append(slices.Clip(exact), plainList...)
}

// HasProperData reports whether the proper-name tables were loaded.
func (k *KnownMisspellings) HasProperData() bool { return len(k.names) > 0 }

// Size is the number of misspelling rules, one-word and two-word.
func (k *KnownMisspellings) Size() int {
	n := len(k.single)
	for _, list := range k.byFirstWord {
		n += len(list)
	}
	return n
}

// Find are the fixes for a lowercased word, and for it with the next word ("" for none) when a two-word rule
// matches. With typedPlain (the sentence is typed without Turkish letters) a two-word rule also matches its plain
// form: "hic bir" -> "hiçbir".
func (k *KnownMisspellings) Find(word, next string, lexicon *Lexicon, typedPlain bool) []KnownFix {
	fixes := []KnownFix{}
	if exact, ok := k.single[word]; ok {
		fixes = append(fixes, KnownFix{Text: exact.right, Count: exact.rightCount, Contraction: exact.contraction})
	} else {
		for _, rule := range k.stems {
			if !strings.HasPrefix(word, rule.wrong) {
				continue
			}
			suffix := word[len(rule.wrong):]
			if runeLen(suffix) > maxKnownSuffix || !knownSuffix.MatchString(suffix) {
				continue
			}
			if !IsValidKnownParticleSuffix(rule.right, suffix) {
				continue
			}
			fixed := rule.right + suffix
			if count := lexicon.Count(lastWord(fixed)); count > 0 {
				fixes = append(fixes, KnownFix{Text: fixed, Count: jsMin(count, rule.rightCount), Contraction: rule.contraction})
				break
			}
		}
	}
	if next != "" {
		for _, rule := range k.byFirstWord[word] {
			second := strings.Split(rule.wrong, " ")[1]
			if next == second {
				fixes = append(fixes, KnownFix{Text: rule.right, Count: rule.rightCount, JoinsNext: true, Contraction: rule.contraction})
			} else if strings.HasPrefix(next, second) && knownSuffix.MatchString(next[len(second):]) && runeLen(next)-runeLen(second) <= maxKnownSuffix {
				suffix := next[len(second):]
				if !IsValidKnownParticleSuffix(rule.right, suffix) {
					continue
				}
				fixed := rule.right + suffix
				if count := lexicon.Count(lastWord(fixed)); count > 0 {
					fixes = append(fixes, KnownFix{Text: fixed, Count: jsMin(count, rule.rightCount), JoinsNext: true, Contraction: rule.contraction})
				}
			}
		}
		if typedPlain && !slices.ContainsFunc(fixes, func(f KnownFix) bool { return f.JoinsNext }) {
			for _, rule := range k.byPlainFirstWord[word] {
				if plain(rule.wrong) == word+" "+next {
					fixes = append(fixes, KnownFix{Text: rule.right, Count: rule.rightCount, JoinsNext: true, Contraction: rule.contraction})
				}
			}
		}
	}
	return fixes
}

// jsMin is JavaScript's Math.min of two numbers: NaN when either is.
func jsMin(a, b float64) float64 {
	if a != a || b != b {
		return a + b
	}
	if a < b {
		return a
	}
	return b
}

// plain is the text without Turkish letters.
func plain(text string) string { return tr.StripMarksIn(text, [][2]int{{0, len(text)}}) }
