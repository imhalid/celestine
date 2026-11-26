package spell

import (
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/spell/morph"
	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/corrector.ts.

// Correction is one change to the text: replace [Start, End) with Replacement. Offsets are in runes, which for
// Turkish text are the TypeScript's UTF-16 units.
type Correction struct {
	Start       int
	End         int
	Original    string
	Replacement string
	Kind        CandidateKind
	// Margin is how much better the replacement fits than what was typed, in natural-log count units.
	Margin float64
}

// CandidateKind is the kind of a change:
//   - marks: Turkish letters typed without their marks ("cok" for "çok");
//   - apostrophe: a suffix on a name without its apostrophe ("Ankaraya");
//   - split: two words written as one ("birşey", "geldinmi", "diyorki");
//   - join: one word written as two ("hiç bir", "oysa ki", "ev de");
//   - typo: a slip of the fingers that made a word that doesn't exist ("gelyorum");
//   - known: a listed, corpus-checked misspelling ("herkez", "malesef", "hiç bir");
//   - context: chosen by the context model between real words ("bende" / "ben de", "oldu" / "öldü");
//   - contraction: a spoken-style contraction spelled out ("noldu" -> "ne oldu"), only when Settings.Contractions
//     is on;
//   - punct: sentence punctuation and capitals ("merhaba.ben" -> "Merhaba. Ben"), only when Settings.SentenceCase
//     is on.
type CandidateKind string

// The kinds of change.
const (
	KindMarks       CandidateKind = "marks"
	KindApostrophe  CandidateKind = "apostrophe"
	KindSplit       CandidateKind = "split"
	KindJoin        CandidateKind = "join"
	KindTypo        CandidateKind = "typo"
	KindKnown       CandidateKind = "known"
	KindContext     CandidateKind = "context"
	KindContraction CandidateKind = "contraction"
	KindPunct       CandidateKind = "punct"
)

// WordAnalysis is a morphological analysis of a form, as TurkishMorphology.analyze gives it.
type WordAnalysis = morph.MorphAnalysis

// FlagReason is why an untouched word was flagged as a likely misspelling: "invalid" (the morphology rejects it and
// it is unseen) or "rare" (rejected, very rare, and close to a much stronger valid word).
type FlagReason string

// The flag reasons.
const (
	FlagInvalid FlagReason = "invalid"
	FlagRare    FlagReason = "rare"
)

// SpellFlag is a suspicious word left untouched by auto-fix, with ranked suggestions (up to 5).
type SpellFlag struct {
	Start       int
	End         int
	Original    string
	Suggestions []string
	Reason      FlagReason
}

// WordValidator tells whether a lowercase form is a real Turkish word by its suffixes (the morphology). A validator
// may also be a wordAnalyzer, a priorCounter and a fuzzySearcher, as *morph.Morphology is; the corrector uses what
// it has, as the TypeScript's optional methods.
type WordValidator interface {
	IsWord(form string) bool
}

// wordAnalyzer is the optional analyze of a WordValidator.
type wordAnalyzer interface {
	Analyze(form string) []morph.MorphAnalysis
}

// priorCounter is the optional priorCount of a WordValidator.
type priorCounter interface {
	PriorCount(form string) float64
}

// fuzzySearcher is the optional fuzzySearch of a WordValidator.
type fuzzySearcher interface {
	FuzzySearch(form string, maxEdits, maxResults int) []morph.FuzzyMorphCandidate
}

// KindThresholds are a number for each kind of change, as the TypeScript's Record<CandidateKind, number>.
type KindThresholds map[CandidateKind]float64

// ContextOffsets are per-regime additive offsets on top of the context threshold; a NaN field is unset.
type ContextOffsets struct {
	MarksASCII, MarksMarked, MarksMarkedOnMarked, DeDaSplit, DeDaJoin, KiSplit, KiJoin, MiSplit, MiJoin float64
}

// Settings are how careful the corrector is. The fields the TypeScript marks optional are NaN when unset.
type Settings struct {
	// Threshold is the minimum margin per kind of change. Higher is more careful.
	Threshold KindThresholds
	// Cost is the fixed cost of each kind of change, subtracted from the candidate's score.
	Cost KindThresholds
	// Ambiguity is how much the best candidate must beat the runner-up (a different text) by.
	Ambiguity float64
	// KnownWord is the count below which words may be slips of the fingers.
	KnownWord float64
	// Contractions spells out spoken-style contractions ("noldu" -> "ne oldu", "burda" -> "burada"). Off by default:
	// they are often written on purpose. When off, such words are left exactly as typed, not taken for slips.
	Contractions bool
	// SentenceCase adds a missing space after a sentence end and capitalizes the first word of a sentence
	// ("merhaba.ben" -> "Merhaba. Ben"). Off in the bench settings so earlier measurements stay comparable; the
	// package turns it on.
	SentenceCase bool
	// TypoInvalidThreshold is the threshold for a slip-of-the-fingers fix when the typed word is unseen and the
	// morphology rejects it.
	TypoInvalidThreshold float64
	// DeDaJoinCost is the extra cost for joining a word with a following "de/da" ("yerler de" -> "yerlerde").
	DeDaJoinCost float64
	// TypoEndWindow and TypoEndCost: a slip in the last TypoEndWindow letters of a word costs TypoEndCost more, as
	// those letters are usually a suffix.
	TypoEndWindow float64
	TypoEndCost   float64
	// TypoFarExtra is, for an unseen word that Turkish rules reject, the cost of reading a letter as an extra key
	// when it is neither a duplicate nor a keyboard neighbor ("ağldı" is a missed "a" in "ağladı", not a stray "ğ"
	// in "aldı").
	TypoFarExtra float64
	// TypoVowelSwap is, for an unseen invalid word, the cost of one vowel typed for another ("nuler" -> "neler");
	// 0 turns it off.
	TypoVowelSwap float64
	// TypoStemShare is, for an unseen invalid word, the share of its stem's count a rare valid inflected form
	// borrows ("arkadaşlarımla" from "arkadaşlarım"), so long correct forms aren't lost for being rare as a whole;
	// 0 turns it off.
	TypoStemShare float64
	// ShortSwapMinCount is the minimum count for an adjacent-swap fix of an unseen-looking 3-letter word ("bka" ->
	// "bak").
	ShortSwapMinCount float64
	// ContextWord is the count from which real words are judged by the context model when one is loaded.
	ContextWord float64
	// ContextVeto drops a count-based fix when the context model prefers what was typed by at least this much.
	ContextVeto float64
	// ContextTypoOverride is the minimum context-model margin over core's typo pick to switch to a competing valid
	// typo candidate.
	ContextTypoOverride float64
	// ContextTypoAmbigGap is the minimum context-model margin between best and runner-up to resolve an ambiguous
	// untouched typo.
	ContextTypoAmbigGap float64
	// ContextTypoAmbigVsTyped is the minimum context-model margin over the invalid typed word to resolve an
	// ambiguous untouched typo.
	ContextTypoAmbigVsTyped float64
	// ContextTypoRescue is the minimum context-model margin over the invalid typed word to rescue a
	// below-threshold typo candidate.
	ContextTypoRescue float64
	// ContextNameThreshold is the minimum margin for the context model ("name" kind) to disambiguate an ambiguous
	// proper name.
	ContextNameThreshold float64
	// ContextOffsets are per-regime additive offsets on top of Threshold[KindContext]; nil when unset.
	ContextOffsets *ContextOffsets
	// KnownMarked is the count from which a word in text that already uses Turkish letters is taken as meant ("uç"
	// stays "uç")...
	KnownMarked float64
	// MarkedOverride ...unless the marked form is this much more common (log ratio): "cogu" is still "çoğu".
	MarkedOverride float64
	// MinName is how often a name must be seen, mostly capitalized, before a suffix after it gets an apostrophe.
	MinName float64
}

// CAREFUL are the careful settings, the engine's measured defaults.
var CAREFUL = Settings{
	Threshold:               KindThresholds{KindMarks: 2.5, KindApostrophe: 2.5, KindSplit: 2.5, KindJoin: 2.5, KindTypo: 3, KindKnown: 1, KindContext: 3, KindContraction: 1, KindPunct: 1},
	Cost:                    KindThresholds{KindMarks: 0.5, KindApostrophe: 0.5, KindSplit: 0.5, KindJoin: 0.5, KindTypo: 2, KindKnown: 0, KindContext: 0, KindContraction: 0, KindPunct: 0},
	Ambiguity:               1,
	KnownWord:               3,
	Contractions:            false,
	SentenceCase:            false,
	TypoInvalidThreshold:    1.5,
	DeDaJoinCost:            0,
	TypoEndWindow:           2,
	TypoEndCost:             1,
	TypoFarExtra:            3,
	TypoVowelSwap:           1.2,
	TypoStemShare:           0,
	ShortSwapMinCount:       300,
	ContextWord:             5,
	ContextVeto:             6.0,
	ContextTypoOverride:     1.2,
	ContextTypoAmbigGap:     1.25,
	ContextTypoAmbigVsTyped: 1.5,
	ContextTypoRescue:       2.5,
	ContextNameThreshold:    2.5,
	KnownMarked:             20,
	MarkedOverride:          5.5,
	MinName:                 20,
}

// undef is the TypeScript's undefined for the optional numbers of a candidate and the settings.
var undef = math.NaN()

// defined reports whether an optional number is set.
func defined(x float64) bool { return !math.IsNaN(x) }

// or is `x ?? fallback` for an optional number.
func or(x, fallback float64) float64 {
	if defined(x) {
		return x
	}
	return fallback
}

// candidate is a replacement under consideration; threshold and slipCost are undef when unset.
type candidate struct {
	text      string
	kind      CandidateKind
	score     float64
	base      float64
	joinsNext bool
	threshold float64
	slipCost  float64
}

// addCandidate offers a candidate; base, threshold and slipCost are undef to leave them out (base then is the typed
// word's score).
type addCandidate func(text string, kind CandidateKind, score, base, threshold, slipCost float64)

// Token is a piece of the text: a word, a run of white space, or one other character. Offsets are in runes.
type Token struct {
	Text   string
	Start  int
	End    int
	IsWord bool
}

// WordContext is what the corrector knows about where a word sits.
type WordContext struct {
	AtSentenceStart bool
	// TextHasMarks is true when the rest of the text uses Turkish letters, so the writer has them on the keyboard.
	TextHasMarks bool
	// QuestionAhead is true when a question mark comes later in the same sentence.
	QuestionAhead bool
	// PrevWord is the lowercased previous word when separated by a single space (using its corrected form if
	// changed), or "" (undefined).
	PrevWord string
	// PrevIsName is true when the previous word was a mid-sentence capitalized word (likely a proper name).
	PrevIsName bool
	// PrevCapitalized is true when the previous word started with an uppercase letter (including sentence start).
	PrevCapitalized bool
	// NextIsolated is true when the next word token is isolated (not glued to hyphens, digits, dots, etc.).
	NextIsolated bool
	// SentenceHasASCIITurkish is true when another word in the same sentence is unmistakably Turkish typed without
	// marks.
	SentenceHasASCIITurkish bool
}

var (
	voicingPairs         = setOf("dt", "td", "bp", "pb", "cç", "çc", "gk", "kg", "gğ", "ğg")
	finiteVerbMorphemes  = setOf("Prog1", "Prog2", "Past", "Narr", "Fut", "Aor", "Opt", "Desr", "Neces", "Imp")
	nonFiniteDerivations = setOf("PastPart", "NarrPart", "FutPart", "PresPart", "AorPart", "Inf1", "Inf2", "Inf3")
	// pronounDe is a pronoun + "de" written as one word, with the verb person that proves it is the conjunction "de".
	pronounDe = map[string]string{"bende": "A1sg", "sende": "A2sg", "bizde": "A1pl", "sizde": "A2pl"}
	tokenRE   = regexp.MustCompile(`[` + tr.Letters + `]+(?:['’][` + tr.Letters + `]+)?|[` + jsSpaceClass + `]+|(?s:.)`)
	// blockedStemMorphemes are the morphemes that show a stem is already plural, oblique-inflected or derived and
	// must not take a new apostrophe.
	blockedStemMorphemes = setOf("A3pl", "Abl", "Loc", "Ins", "Equ", "Cop", "With", "Without", "Ness", "Agt", "Rel")
	// nominalPersonalAgreement are the 1st/2nd person nominal predicate endings blocked for zero-count unseen mark
	// guesses ("alveolusun" -> "alveolüsün").
	nominalPersonalAgreement = setOf("A1sg", "A2sg", "A1pl", "A2pl")
	// oovNominalSuffix are the productive Turkish nominal suffixes allowed on attested OOV loanword or compound
	// roots missing from Zemberek ("özgeçmişimi", "tripodlar").
	oovNominalSuffix = regexp.MustCompile(`^(?:l[ae]r(?:(?:[ıiuü]m(?:[ıiuü]z)?|[ıiuü]n(?:[ıiuü]z)?|[ıiuü])(?:n?[dt][ae]n?|n?[dt][ae]ki|n?[aeıiuü]|n[ıiuü]n|y?l[ae])?|[dt][ae]n?|[dt][ae]ki|[ıiuü]n|[aeıiuü]|l[ae])?|(?:[ıiuü]m(?:[ıiuü]z)?|[ıiuü]n(?:[ıiuü]z)?|s?[ıiuü])(?:n?[dt][ae]n?|n?[dt][ae]ki|n?[aeıiuü]|n[ıiuü]n|y?l[ae])?|[dt][ae]n?|[dt][ae]ki|n?[ıiuü]n|y?[aeıiuü]|y?l[ae]|l[ıiuü]k?)(?:[dt][ıiuü]r|y?[dt][ıiuü]|y?m[ıiuü]ş)?$`)
	// foreignTaxonomicSuffix are the Latin, scientific or clearly foreign endings that shouldn't be flagged when no
	// close Turkish candidate exists.
	foreignTaxonomicSuffix = regexp.MustCompile(`(?:idae|aceae|inae|formes|ensis|oides|phyllus|flora|saurus|stoma|poda|comulus|ischer|lichkeit|hausen|shire|burgh|stadt|field|worth|ingham)$`)
	// fixedKiWords are the TDK SOMBAHCEMİ words that are always written joined and must never be split by the
	// context model.
	fixedKiWords = setOf("oysaki", "halbuki", "mademki", "meğerki", "illaki", "sanki", "belki", "çünkü")

	qMatchRE         = regexp.MustCompile(`^(.{3,})(m[iıuü](?:y?[iıuü]m|s[iıuü]n|y?[iıuü]z|s[iıuü]n[iıuü]z|d[iıuü]r|d[iıuü]|ym[iıuü]ş|ys[ae]|yd[iıuü]|yken)?)$`)
	vowelRE2         = regexp.MustCompile(`[aeıioöuüâîû]`)
	kiCopulaSuffix   = regexp.MustCompile(`^(?:[dt][ıiuü]r|y?d[ıiuü])$`)
	alsoCaseSuffix   = regexp.MustCompile(`^(?:[dt][ae]n?|n[ıiuü]n|y[aeıiuü]|y?l[ae])$`)
	verbGlueEnd      = regexp.MustCompile(`(?:s[ae]|yor|m[ıiuü]ş|[ae]c[ae]k)$`)
	pluralAccEnd     = regexp.MustCompile(`l[ae]r[ıi]$`)
	derivedLanguage  = regexp.MustCompile(`[cç][ae]$`)
	possessiveGenEnd = regexp.MustCompile(`(?:[ıiuü]n[ıiuü]z|[ıiuü]m[ıiuü]z)[ıiuü]n$`)
	locativeEnd      = regexp.MustCompile(`[dt][ae]$`)
	foreignLetters   = regexp.MustCompile(`[qwxâîû]`)
)

// sentenceEnd reports whether a token ends a sentence (SENTENCE_END).
func sentenceEnd(text string) bool {
	switch text {
	case ".", "!", "?", ":", "\n":
		return true
	}
	return false
}

// isVowel is VOWELS.has on one letter.
func isVowel(r rune) bool { return strings.ContainsRune("aeıioöuü", r) }

// lg is the TypeScript's log: the natural logarithm of count + 1, as Math.log computes it.
func lg(count float64) float64 { return jsLog(count + 1) }

// marginOf is a candidate's margin over what was typed.
func marginOf(c *candidate) float64 { return c.score - c.base }

// orderedMap is a JavaScript Map with string keys: it iterates in insertion order, a key set again keeps its place.
type orderedMap[V any] struct {
	keys []string
	vals map[string]V
}

func newOrderedMap[V any]() *orderedMap[V] { return &orderedMap[V]{vals: map[string]V{}} }

func (m *orderedMap[V]) get(key string) (V, bool) {
	v, ok := m.vals[key]
	return v, ok
}

func (m *orderedMap[V]) set(key string, v V) {
	if _, ok := m.vals[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = v
}

func (m *orderedMap[V]) del(key string) {
	if _, ok := m.vals[key]; !ok {
		return
	}
	delete(m.vals, key)
	m.keys = slices.DeleteFunc(m.keys, func(k string) bool { return k == key })
}

func (m *orderedMap[V]) size() int { return len(m.keys) }

// Tokenize cuts text into Tokens: words (letters, with one apostrophe inside), runs of white space, and single
// other characters.
func Tokenize(text string) []Token {
	matches := findAllRunes(tokenRE, text)
	rs := []rune(text)
	tokens := make([]Token, len(matches))
	for i, m := range matches {
		value := string(rs[m[0]:m[1]])
		tokens[i] = Token{Text: value, Start: m[0], End: m[1], IsWord: unicode.IsLetter(rs[m[0]])}
	}
	return tokens
}

// lastSpaceWord is what follows the last space of value, or value.
func lastSpaceWord(value string) string { return lastWord(value) }

// keepCase copies the typed word's capital letters onto a replacement that only adds spaces, apostrophes or marks.
func keepCase(typed, replacement string) string {
	plain := func(text string) string { return strings.NewReplacer(" ", "", "'", "").Replace(text) }
	if runeLen(plain(tr.Lower(typed))) != runeLen(plain(replacement)) {
		return tr.MatchCase(typed, replacement)
	}
	typedLetters := []rune(strings.NewReplacer(" ", "", "'", "", "’", "").Replace(typed))
	index := 0
	var output strings.Builder
	for _, char := range replacement {
		if char == ' ' || char == '\'' {
			output.WriteRune(char)
			continue
		}
		original := ""
		if index < len(typedLetters) {
			original = string(typedLetters[index])
		}
		index++
		if original != tr.Lower(original) {
			output.WriteString(tr.Upper(string(char)))
		} else {
			output.WriteRune(char)
		}
	}
	return output.String()
}

// foldTr takes the marks off a lowercase word's Turkish letters, circumflexes too.
func foldTr(word string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'ç':
			return 'c'
		case 'ğ':
			return 'g'
		case 'ı', 'î':
			return 'i'
		case 'ö':
			return 'o'
		case 'ş':
			return 's'
		case 'ü', 'û':
			return 'u'
		case 'â':
			return 'a'
		}
		return r
	}, word)
}

// CorrectOptions are the options of a correction pass.
type CorrectOptions struct {
	// AutoMarked are [start, end) ranges whose Turkish letters were put there by autocorrect, not typed by the user.
	// They don't count as evidence that the user types with Turkish letters, so "hic bir sey" still gets "sey"
	// fixed after "hiç" was.
	AutoMarked [][2]int
	// PriorTypoOriginals are the original typed tokens for spans already rewritten by an earlier core typing pass
	// (e.g. "hber" -> "her"), allowing the context pass to re-evaluate the original typo once the full sentence is
	// available.
	PriorTypoOriginals []PriorTypo
}

// PriorTypo is a span rewritten as a typo earlier and what was typed there.
type PriorTypo struct {
	Start, End int
	Original   string
}

// Corrector is the Turkish spell corrector: it decides on each word of a text from the lexicon's counts, the known
// misspellings, the rules of Turkish and the morphology. It is not safe for concurrent use: the morphology's
// caches, which make its answers depend on the order of calls as in the TypeScript, and Settings are shared.
type Corrector struct {
	lexicon    *Lexicon
	known      *KnownMisspellings
	morphology WordValidator
	analyzer   wordAnalyzer
	prior      priorCounter
	fuzzy      fuzzySearcher
	Settings   Settings
}

// NewCorrector is a corrector of the lexicon with settings; known and morphology may be nil.
func NewCorrector(lexicon *Lexicon, settings Settings, known *KnownMisspellings, morphology WordValidator) *Corrector {
	c := &Corrector{lexicon: lexicon, known: known, Settings: settings}
	if morphology != nil && !reflect.ValueOf(morphology).IsZero() {
		c.morphology = morphology
		c.analyzer, _ = morphology.(wordAnalyzer)
		c.prior, _ = morphology.(priorCounter)
		c.fuzzy, _ = morphology.(fuzzySearcher)
	}
	return c
}

// hasMorph is `this.morphology !== undefined`.
func (c *Corrector) hasMorph() bool { return c.morphology != nil }

// analyze is `this.morphology?.analyze?.(form)`: false when there is none to ask.
func (c *Corrector) analyze(form string) ([]WordAnalysis, bool) {
	if c.analyzer == nil {
		return nil, false
	}
	return c.analyzer.Analyze(form), true
}

// isRealWord reports whether a word the lexicon has rarely or never seen is one Turkish suffix rules accept
// ("getirsek", "kitabımı").
func (c *Corrector) isRealWord(word string) bool {
	if c.morphology == nil {
		return false
	}
	return c.morphology.IsWord(word)
}

// hasFrontLocative reports whether a stem takes both front locative and front ablative (filters accidental Dat
// collisions like "varit" -> "varde").
func (c *Corrector) hasFrontLocative(stem string) bool {
	return (c.isRealWord(stem+"de") && c.isRealWord(stem+"den")) || (c.isRealWord(stem+"te") && c.isRealWord(stem+"ten"))
}

// hasBackLocative reports whether a stem takes both back locative and back ablative.
func (c *Corrector) hasBackLocative(stem string) bool {
	return (c.isRealWord(stem+"da") && c.isRealWord(stem+"dan")) || (c.isRealWord(stem+"ta") && c.isRealWord(stem+"tan"))
}

// isDerivedLanguageStem reports whether a -ca/-ce/-ça/-çe stem is derived from a proper or nationality stem
// ("Danimarkaca", "Türkçe", "Hırvatça").
func (c *Corrector) isDerivedLanguageStem(stem string) bool {
	if runeLen(stem) < 5 || !derivedLanguage.MatchString(stem) {
		return false
	}
	analyses, ok := c.analyze(stem)
	if !ok || len(analyses) == 0 {
		return false
	}
	return slices.ContainsFunc(analyses, func(a WordAnalysis) bool {
		return slices.Contains(a.Morphemes, "Equ") && a.Stem != stem && c.lexicon.CapitalShare(a.Stem) >= 0.45
	})
}

// isBlockedApostropheStem reports whether TDK rules keep a stem from taking an apostrophe (with the morphology
// minding -ca/-ce names like "Karaca").
func (c *Corrector) isBlockedApostropheStem(stem string, allowPlaceHead bool) bool {
	if c.hasMorph() {
		if HasBlockedApostropheStem(stem, true, allowPlaceHead) {
			return true
		}
		return c.isDerivedLanguageStem(stem)
	}
	return HasBlockedApostropheStem(stem, false, allowPlaceHead)
}

// isBareNominalRoot reports whether a form has a bare Noun/Adj/Adv root analysis (for separate Noun+etmek compound
// verb splits).
func (c *Corrector) isBareNominalRoot(word string) bool {
	analyses, ok := c.analyze(word)
	if !ok || len(analyses) == 0 {
		return false
	}
	return slices.ContainsFunc(analyses, func(a WordAnalysis) bool {
		return (a.Pos == "Noun" || a.Pos == "Adj" || a.Pos == "Adv") &&
			(len(a.Morphemes) == 0 || (len(a.Morphemes) == 1 && a.Morphemes[0] == "A3sg"))
	})
}

// isVerbFormOf reports whether word is an inflected verb form of auxStem ("et", "ol", "geç", "vur", ...).
func (c *Corrector) isVerbFormOf(word, auxStem string) bool {
	analyses, ok := c.analyze(word)
	if !ok || len(analyses) == 0 {
		return false
	}
	return slices.ContainsFunc(analyses, func(a WordAnalysis) bool { return a.Pos == "Verb" && a.Stem == auxStem })
}

// anyIn reports whether one of the morphemes is in set.
func anyIn(morphemes []string, set map[string]bool) bool {
	return slices.ContainsFunc(morphemes, func(m string) bool { return set[m] })
}

// isFiniteVerb reports whether word has a finite conjugated verb analysis (never part of an OOV noun compound).
func (c *Corrector) isFiniteVerb(word string) bool {
	analyses, ok := c.analyze(word)
	if !ok || len(analyses) == 0 {
		return false
	}
	return slices.ContainsFunc(analyses, func(a WordAnalysis) bool {
		return a.Pos == "Verb" && anyIn(a.Morphemes, finiteVerbMorphemes) && !anyIn(a.Morphemes, nonFiniteDerivations)
	})
}

// isInflectedApostropheStem reports whether the morphology parses a candidate apostrophe stem only as an
// already-inflected or derived form ("japonların", "yunanlardan", "ankaradan", "türkçeden"), never as an
// uninflected bare root. Bare personal names that coincide with lowercase inflected words ("Emre", "Selin",
// "Güler") are kept.
func (c *Corrector) isInflectedApostropheStem(stem string) bool {
	analyses, ok := c.analyze(stem)
	if !ok || len(analyses) == 0 {
		return false
	}
	if slices.ContainsFunc(analyses, func(a WordAnalysis) bool {
		return len(a.Morphemes) == 0 || (len(a.Morphemes) == 1 && a.Morphemes[0] == "A3sg")
	}) {
		return false
	}
	for _, a := range analyses {
		if anyIn(a.Morphemes, blockedStemMorphemes) {
			continue
		}
		if a.Stem != stem && c.lexicon.CapitalShare(a.Stem) >= 0.45 {
			continue
		}
		return false
	}
	return true
}

// isSafeUnseenMarkVariant is an extra morphological and lexical guard for unseen (count 0) unique mark
// restorations on long lowercase words: it keeps genuine Turkish inflections ("yukleyince", "sisirmesinden",
// "dalgakiranlarla") while blocking Latin or foreign terms that accidentally match nominal 1st/2nd person copula
// readings ("alveolusun" -> "alveolüsün").
func (c *Corrector) isSafeUnseenMarkVariant(word, candidate string, context *WordContext) bool {
	analyses, ok := c.analyze(candidate)
	if !ok {
		return true
	}
	if len(analyses) == 0 {
		return false
	}
	wr, cr := []rune(word), []rune(candidate)
	hasNewConsonantMark := false
	for i, ch := range cr {
		if (i >= len(wr) || ch != wr[i]) && (ch == 'ç' || ch == 'ğ' || ch == 'ş') {
			hasNewConsonantMark = true
			break
		}
	}
	return slices.ContainsFunc(analyses, func(a WordAnalysis) bool {
		if slices.Contains(a.Morphemes, "Zero") && anyIn(a.Morphemes, nominalPersonalAgreement) {
			return false
		}
		lemma := a.Stem
		if a.Pos == "Verb" {
			if FollowsHarmony(a.Stem, "a") {
				lemma += "mak"
			} else {
				lemma += "mek"
			}
		}
		if c.lexicon.Count(lemma) < 5 || c.lexicon.CapitalShare(lemma) >= 0.5 {
			return false
		}
		n := runeLen(a.Stem)
		stemChanged := sliceRunes(word, 0, n) != sliceRunes(candidate, 0, n)
		if context.TextHasMarks && a.Pos != "Verb" && !stemChanged && !hasNewConsonantMark {
			return false
		}
		return true
	})
}

// withSentenceRules runs the sentence rules (sentence.go) over the corrections, with the corrector's lexicon,
// names and morphology. collectAmbig and preDecidedAmbig are for the context model's pass; nil without one.
func (c *Corrector) withSentenceRules(text string, corrections []Correction, collectAmbig *[]AmbigCandidateQuery, preDecidedAmbig map[int]string) []Correction {
	var disambiguateAmbig func(q AmbigCandidateQuery) (string, bool)
	if preDecidedAmbig != nil {
		disambiguateAmbig = func(q AmbigCandidateQuery) (string, bool) {
			s, ok := preDecidedAmbig[q.Start]
			return s, ok
		}
	}
	return applySentenceRules(
		text,
		corrections,
		func(lower string) bool { return c.isRealWord(lower) || c.lexicon.ExactCount(lower) >= 20 },
		func(lower string, common bool) (string, bool) {
			if c.known == nil {
				return "", false
			}
			minCount := 0.0
			if common {
				minCount = 1500
			}
			return c.known.NameFor(lower, minCount)
		},
		func(lower string) ([]string, bool) {
			analyses, ok := c.analyze(lower)
			if !ok {
				return nil, false
			}
			stems := make([]string, len(analyses))
			for i, a := range analyses {
				stems[i] = tr.Lower(a.Stem)
			}
			return stems, true
		},
		&SentenceResources{
			Known:             c.known,
			CapitalShare:      c.lexicon.CapitalShare,
			Count:             c.lexicon.Count,
			Pair:              c.lexicon.Pair,
			IsMorphWord:       c.isRealWord,
			DisambiguateAmbig: disambiguateAmbig,
			CollectAmbig:      collectAmbig,
		},
	)
}

// isLowercaseProperForSentence reports whether word (when SentenceCase is on) is an unseen lowercase proper name or
// honorific with an inflectional suffix ("nerminin", "emrenin", "kocaelide", "orhan beyde", "gönül hanımda") that
// the TDK proper-name rules should handle instead of a typo or split fix ("merminin", "emrinin", "bey de").
func (c *Corrector) isLowercaseProperForSentence(word, typed string, context *WordContext) bool {
	if !c.Settings.SentenceCase || c.known == nil || !c.known.HasProperData() {
		return false
	}
	if typed != word || strings.Contains(word, "'") {
		return false
	}
	// Honorific + suffix after a person name ("orhan beyde", "gönül hanımda").
	if context.PrevWord != "" {
		if split, ok := SplitHonorific(word); ok && split.Suffix != "" {
			_, isName := c.known.NameEntry(context.PrevWord, 0)
			_, isGiven := c.known.GivenNameFor(context.PrevWord)
			_, isAmbig := c.known.AmbigNameFor(context.PrevWord)
			if isName || isGiven || isAmbig || context.PrevCapitalized {
				return true
			}
		}
	}
	// An acronym + suffix ("tbmmde", "abye", "natonun") or a proper name + suffix ("nerminin", "kocaelide",
	// "emrenin").
	if !c.isRealWord(word) {
		rs := []rune(word)
		if len(rs) >= 4 {
			for cut := min(len(rs)-1, 8); cut >= 2; cut-- {
				base, suf := string(rs[:cut]), string(rs[cut:])
				if _, ok := c.known.AcronymFor(base); ok && IsPhonologicallyValidProperSuffix(base, suf, ProperSuffixOptions{Acronym: true}) {
					return true
				}
			}
		}
		for cut := len(rs) - 1; cut >= 3; cut-- {
			base, suf := string(rs[:cut]), string(rs[cut:])
			if !ProperInflectionalSuffixRE.MatchString(suf) {
				continue
			}
			_, isName := c.known.NameEntry(base, 0)
			isGiven, isAmbig := false, false
			if !isName {
				_, isGiven = c.known.GivenNameFor(base)
				if !isGiven {
					_, isAmbig = c.known.AmbigNameFor(base)
				}
			}
			if (isName || isGiven || isAmbig) && IsPhonologicallyValidProperSuffix(base, suf, ProperSuffixOptions{}) {
				return true
			}
		}
		// A classic book title stem + suffix ("ince memedini").
		if context.PrevWord == "ince" && strings.HasPrefix(word, "memed") {
			return true
		}
	}
	return false
}

// CorrectResult is a corrected text, the changes made, and (from Inspect) the flagged words.
type CorrectResult struct {
	Text        string
	Corrections []Correction
	Flags       []SpellFlag
}

// Correct is the corrected text and the list of changes made.
func (c *Corrector) Correct(text string, options CorrectOptions) CorrectResult {
	r := c.runCorePass(text, options, false)
	r.Flags = nil
	return r
}

// Inspect is Correct, and also flags every suspicious untouched word (rejected by the morphology, or very rare near
// a much stronger valid word) with up to 5 ranked suggestions.
func (c *Corrector) Inspect(text string, options CorrectOptions) CorrectResult {
	return c.runCorePass(text, options, true)
}

// markPrefix counts the Turkish letters with marks in rs before each index, for asking whether the text outside a
// token has any.
func markPrefix(rs []rune) []int {
	counts := make([]int, len(rs)+1)
	for i, r := range rs {
		counts[i+1] = counts[i]
		if strings.ContainsRune("çğıöşüÇĞİÖŞÜ", r) {
			counts[i+1]++
		}
	}
	return counts
}

// checkResult is check's decision on a word.
type checkResult struct {
	replacement string
	kind        CandidateKind
	margin      float64
	joinsNext   bool
}

// passState is what the passes carry from word to word.
type passState struct {
	atSentenceStart bool
	prevWord        string
	prevIsName      bool
	prevCapitalized bool
}

// wordContext builds the context of the word token at index, as both passes do.
func (c *Corrector) wordContext(tokens []Token, index int, st *passState, textHasMarks bool, marks []int, asciiTyped, sentenceOf []int) (WordContext, *nextWordAt) {
	token := tokens[index]
	next := c.nextWord(tokens, index)
	otherASCII := slices.ContainsFunc(asciiTyped, func(other int) bool { return other != index && sentenceOf[other] == sentenceOf[index] })
	sentenceHasASCIITurkish := !textHasMarks && otherASCII
	context := WordContext{
		AtSentenceStart:         st.atSentenceStart,
		TextHasMarks:            textHasMarks && marks[token.Start]+marks[len(marks)-1]-marks[token.End] > 0 && !otherASCII,
		QuestionAhead:           c.questionAhead(tokens, index),
		PrevWord:                st.prevWord,
		PrevIsName:              st.prevIsName,
		PrevCapitalized:         st.prevCapitalized,
		SentenceHasASCIITurkish: sentenceHasASCIITurkish,
	}
	if next != nil {
		context.NextIsolated = c.isIsolated(tokens, next.index)
	}
	return context, next
}

// nonWord updates the pass state for a token that is not a word.
func (st *passState) nonWord(token Token) {
	if sentenceEnd(token.Text) || strings.Contains(token.Text, "\n") {
		st.atSentenceStart = true
	} else if strings.ContainsAny(token.Text, "0123456789") {
		st.atSentenceStart = false
	}
	if token.Text != " " {
		st.forget()
	}
}

// forget drops what is known about the previous word.
func (st *passState) forget() {
	st.prevWord = ""
	st.prevIsName = false
	st.prevCapitalized = false
}

// untouched records a word left as typed as the previous word.
func (st *passState) untouched(token Token, context *WordContext) {
	st.prevWord = tr.Lower(token.Text)
	st.prevIsName = !context.AtSentenceStart && tr.IsUpperInitial(token.Text)
	st.prevCapitalized = tr.IsUpperInitial(token.Text)
}

// fixed records a fixed word's replacement as the previous word.
func (st *passState) fixed(fix *checkResult, context *WordContext) {
	trailing := lastSpaceWord(fix.replacement)
	st.prevWord = tr.Lower(trailing)
	st.prevIsName = !(context.AtSentenceStart && !strings.Contains(fix.replacement, " ")) && tr.IsUpperInitial(trailing)
	st.prevCapitalized = tr.IsUpperInitial(trailing)
}

func (c *Corrector) runCorePass(text string, options CorrectOptions, collectFlags bool) CorrectResult {
	rs := []rune(text)
	tokens := Tokenize(text)
	corrections := []Correction{}
	flags := []SpellFlag{}
	evidence := tr.StripMarksIn(text, options.AutoMarked)
	textHasMarks := tr.HasTurkishMarks(evidence)
	marks := markPrefix([]rune(evidence))
	asciiTyped, sentenceOf := c.asciiEvidence(Tokenize(evidence))
	st := passState{atSentenceStart: true}
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !token.IsWord {
			st.nonWord(token)
			continue
		}
		context, next := c.wordContext(tokens, index, &st, textHasMarks, marks, asciiTyped, sentenceOf)
		st.atSentenceStart = false
		if !c.isIsolated(tokens, index) {
			st.forget()
			continue
		}
		nextText := ""
		if next != nil {
			nextText = next.token.Text
		}
		fix := c.check(token.Text, &context, nextText)
		if fix == nil {
			if collectFlags {
				if flag := c.detectFlag(token, &context, nextText); flag != nil {
					flags = append(flags, *flag)
				}
			}
			st.untouched(token, &context)
			continue
		}
		end := token.End
		if fix.joinsNext && next != nil {
			end = next.token.End
		}
		corrections = append(corrections, Correction{
			Start:       token.Start,
			End:         end,
			Original:    string(rs[token.Start:end]),
			Replacement: fix.replacement,
			Kind:        fix.kind,
			Margin:      fix.margin,
		})
		st.fixed(fix, &context)
		if fix.joinsNext && next != nil {
			index = next.index
		}
	}
	if c.Settings.SentenceCase {
		corrections = c.withSentenceRules(text, corrections, nil, nil)
	}
	return CorrectResult{Text: applyToRunes(rs, corrections), Corrections: corrections, Flags: flags}
}

// applyToRunes writes the corrections, sorted and not overlapping, into the text.
func applyToRunes(rs []rune, corrections []Correction) string {
	var output strings.Builder
	cursor := 0
	for _, correction := range corrections {
		output.WriteString(runeSlice(rs, cursor, correction.Start))
		output.WriteString(correction.Replacement)
		cursor = correction.End
	}
	output.WriteString(runeSlice(rs, cursor, len(rs)))
	return output.String()
}

// asciiEvidence finds the words that were clearly typed without Turkish letters ("anlamadim", "sey"): unseen or
// nearly so as typed, but common with marks. A sentence with one is treated as typed without Turkish letters, even
// when other words in the text have them, so "hic" next to "anlamadim" still becomes "hiç".
func (c *Corrector) asciiEvidence(tokens []Token) (asciiTyped, sentenceOf []int) {
	sentenceOf = make([]int, len(tokens))
	sentence := 0
	for index, token := range tokens {
		if !token.IsWord {
			if sentenceEnd(token.Text) || strings.Contains(token.Text, "\n") {
				sentence++
			}
			sentenceOf[index] = sentence
			continue
		}
		sentenceOf[index] = sentence
		word := tr.Lower(token.Text)
		if tr.HasTurkishMarks(word) || c.lexicon.Count(word) >= c.Settings.KnownWord || c.isRealWord(word) {
			continue
		}
		if slices.ContainsFunc(tr.MarkVariants(word, 512), func(v string) bool { return c.lexicon.Count(v) >= asciiEvidenceCount }) {
			asciiTyped = append(asciiTyped, index)
		}
	}
	return asciiTyped, sentenceOf
}

// asciiEvidenceCount is how often a word's marked form must be seen for the word typed without marks to count as
// evidence of an ASCII keyboard.
const asciiEvidenceCount = 50

// nextWordAt is the next word and its token index.
type nextWordAt struct {
	token Token
	index int
}

// nextWord is the next word when only a single space separates it from this one.
func (c *Corrector) nextWord(tokens []Token, index int) *nextWordAt {
	if index+2 < len(tokens) && tokens[index+1].Text == " " && tokens[index+2].IsWord {
		return &nextWordAt{token: tokens[index+2], index: index + 2}
	}
	return nil
}

func (c *Corrector) questionAhead(tokens []Token, index int) bool {
	for ahead := index + 1; ahead < len(tokens); ahead++ {
		text := tokens[ahead].Text
		if text == "?" {
			return true
		}
		if sentenceEnd(text) || strings.Contains(text, "\n") {
			return false
		}
	}
	return false
}

// gluedMark reports whether text has one of the characters that glue a word into an address, a handle or code:
// /[@/#_\\\d-]/.
func gluedMark(text string) bool { return strings.ContainsAny(text, "@/#_\\-0123456789") }

// hasLetter is /\p{L}/u.test(text).
func hasLetter(text string) bool { return strings.IndexFunc(text, unicode.IsLetter) >= 0 }

// isIsolated reports whether a word stands alone: words glued to "@", "/", digits or dots between letters are parts
// of addresses, handles or code.
func (c *Corrector) isIsolated(tokens []Token, index int) bool {
	before, after := " ", " "
	if index-1 >= 0 {
		before = tokens[index-1].Text
	}
	if index+1 < len(tokens) {
		after = tokens[index+1].Text
	}
	var afterNext *Token
	if index+2 < len(tokens) {
		afterNext = &tokens[index+2]
	}
	if gluedMark(before) || gluedMark(after) {
		return false
	}
	// Part of a word with letters Turkish doesn't use ("skrælingi", Cyrillic): not ours to judge.
	if hasLetter(before) || hasLetter(after) {
		return false
	}
	if after == "." && afterNext != nil && afterNext.IsWord {
		// When SentenceCase is on, a missing space after a full stop ("gonderdim.bir", "yasiyor.her") is not an
		// address or a domain when neither side is an abbreviation or a TLD and there is no second dot.
		if !c.Settings.SentenceCase {
			return false
		}
		cur := tokens[index]
		curLow := tr.Lower(cur.Text)
		nextLow := tr.Lower(afterNext.Text)
		prevDot := before == "." || (index-2 >= 0 && tokens[index-2].Text == ".")
		nextDot := index+3 < len(tokens) && tokens[index+3].Text == "."
		if prevDot || nextDot || runeLen(cur.Text) < 2 || runeLen(afterNext.Text) < 2 || Abbreviations[curLow] || NotASentenceStart[nextLow] {
			return false
		}
		return true
	}
	return true
}

// check decides on one word, optionally joined with the next (nextTyped, "" when there is none). It is nil to
// leave the word alone.
func (c *Corrector) check(typed string, context *WordContext, nextTyped string) *checkResult {
	if tr.IsAllUpper(typed) || runeLen(typed) < 2 {
		return nil
	}
	word := tr.Lower(typed)
	if c.isLowercaseProperForSentence(word, typed, context) {
		return nil
	}
	typedCount := c.lexicon.Count(word)
	listed, keep := c.knownFix(typed, word, context, nextTyped)
	if keep {
		return nil
	}
	if listed != nil {
		return listed
	}
	if pronoun := c.pronounAlso(typed, word, nextTyped); pronoun != nil {
		return pronoun
	}
	candidates := c.candidates(word, typed, context, typedCount, nextTyped)

	if nextTyped != "" && !tr.IsAllUpper(nextTyped) && !tr.IsUpperInitial(nextTyped) && !(tr.IsUpperInitial(typed) && !context.AtSentenceStart) {
		if joined := c.joinCandidate(word, tr.Lower(nextTyped), typed, context.NextIsolated); joined != nil {
			candidates = append(candidates, joined)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sortByMargin(candidates)
	best := candidates[0]
	var runnerUp *candidate
	for _, cand := range candidates {
		if cand.text != best.text {
			runnerUp = cand
			break
		}
	}
	margin := marginOf(best)
	// A word nobody has seen that Turkish suffix rules also reject is almost surely a slip ("gelyorum"), so a fix for
	// it needs less of a margin. Only for lowercase words: an unseen capitalized word is usually a name ("Kahireden"
	// is "Kahire'den").
	invalidTyped := best.kind == KindTypo && c.hasMorph() && typedCount == 0 && typed == word && !c.isRealWord(word)
	threshold := best.threshold
	if !defined(threshold) {
		if invalidTyped {
			threshold = c.Settings.TypoInvalidThreshold
		} else {
			threshold = c.Settings.Threshold[best.kind]
		}
	}
	if margin < threshold {
		return nil
	}
	if runnerUp != nil && margin-marginOf(runnerUp) < c.Settings.Ambiguity {
		return nil
	}
	if best.kind == KindMarks && context.PrevWord != "" && IsAuxiliaryOl(context.PrevWord, word, best.text) {
		return nil
	}
	original := typed
	if best.joinsNext {
		original = typed + " " + nextTyped
	}
	replacement := keepCase(original, best.text)
	if best.kind == KindTypo {
		replacement = tr.MatchCase(typed, best.text)
	}
	return &checkResult{replacement: replacement, kind: best.kind, margin: margin, joinsNext: best.joinsNext}
}

// jsCompare is a sort comparator's number as Array.prototype.sort reads it: below 0 sorts first, NaN is 0.
func jsCompare(d float64) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}

// sortByMargin sorts candidates by margin, the best first, keeping the order of equals.
func sortByMargin(candidates []*candidate) {
	slices.SortStableFunc(candidates, func(first, second *candidate) int { return jsCompare(marginOf(second) - marginOf(first)) })
}

// pronounAlso is "bende geldim" -> "ben de geldim": a pronoun glued to "de" followed by a finite verb of the same
// person is the conjunction, never the locative ("Kitap bende kaldı" keeps "bende": the verb there is third
// person).
func (c *Corrector) pronounAlso(typed, word, nextTyped string) *checkResult {
	person, ok := pronounDe[word]
	if !ok || nextTyped == "" || c.analyzer == nil || tr.IsUpperInitial(nextTyped) && !tr.IsUpperInitial(typed) {
		return nil
	}
	analyses := c.analyzer.Analyze(tr.Lower(nextTyped))
	last := func(a WordAnalysis) string {
		if len(a.Morphemes) == 0 {
			return ""
		}
		return a.Morphemes[len(a.Morphemes)-1]
	}
	found := false
	for _, a := range analyses {
		if a.Pos == "Verb" && anyIn(a.Morphemes, finiteVerbMorphemes) && !slices.Contains(a.Morphemes, "Zero") && last(a) == person {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	if slices.ContainsFunc(analyses, func(a WordAnalysis) bool { return last(a) == "A3sg" && anyIn(a.Morphemes, finiteVerbMorphemes) }) {
		return nil
	}
	return &checkResult{replacement: sliceRunes(typed, 0, -2) + " " + sliceFrom(typed, -2), kind: KindSplit, margin: 5}
}

// knownFix is a listed misspelling, which wins over everything else when the fix is clearly more common than what
// was typed; keep is true when the word must stay as typed.
func (c *Corrector) knownFix(typed, word string, context *WordContext, nextTyped string) (best *checkResult, keep bool) {
	if c.known == nil || (tr.IsUpperInitial(typed) && !context.AtSentenceStart) {
		return nil, false
	}
	next := ""
	if nextTyped != "" && !tr.IsUpperInitial(nextTyped) {
		next = tr.Lower(nextTyped)
	}
	fixes := c.known.Find(word, next, c.lexicon, !context.TextHasMarks)
	// With contractions off, a single word matching one ("noldu") stays as typed: no other rule may take it for a
	// slip.
	if !c.Settings.Contractions && slices.ContainsFunc(fixes, func(f KnownFix) bool { return f.Contraction && !f.JoinsNext }) {
		return nil, true
	}
	for _, fix := range fixes {
		if fix.Contraction && !c.Settings.Contractions {
			continue
		}
		var base float64
		if fix.JoinsNext {
			base = lg(c.lexicon.Pair(word, next))
		} else {
			base = lg(c.lexicon.Count(word))
		}
		margin := lg(fix.Count) - c.Settings.Cost[KindKnown] - base
		if margin < c.Settings.Threshold[KindKnown] || (best != nil && best.margin >= margin) {
			continue
		}
		original := typed
		if fix.JoinsNext {
			original = typed + " " + nextTyped
		}
		kind := KindKnown
		if fix.Contraction {
			kind = KindContraction
		}
		best = &checkResult{replacement: keepCase(original, fix.Text), kind: kind, margin: margin, joinsNext: fix.JoinsNext}
	}
	return best, false
}

// candidates are the replacements of word worth weighing, with their scores.
func (c *Corrector) candidates(word, typed string, context *WordContext, typedCount float64, nextTyped string) []*candidate {
	cost, knownWord, knownMarked, markedOverride := c.Settings.Cost, c.Settings.KnownWord, c.Settings.KnownMarked, c.Settings.MarkedOverride
	typedScore := lg(typedCount)
	found := newOrderedMap[*candidate]()
	bar := func(kind CandidateKind, threshold float64) float64 { return or(threshold, c.Settings.Threshold[kind]) }
	add := func(text string, kind CandidateKind, score, base, threshold, slipCost float64) {
		if !defined(base) {
			base = typedScore
		}
		if math.IsNaN(score) || math.IsInf(score, 0) || math.IsNaN(base) || math.IsInf(base, 0) || text == word {
			return
		}
		existing, ok := found.get(text)
		// The same fix may come from two rules with different bars (plain mark restoration and the lower bar for a
		// unique morphologically valid variant). Keep the best margin and the lower bar of the two, or the lower
		// bar is lost whenever the other rule saw the same candidate first ("duzeltmiyor" -> "düzeltmiyor").
		sameKind := ok && existing.kind == kind
		if !ok || score-base > marginOf(existing) {
			keepBar := sameKind && bar(existing.kind, existing.threshold) < bar(kind, threshold)
			t := threshold
			if keepBar {
				t = existing.threshold
			}
			found.set(text, &candidate{text: text, kind: kind, score: score, base: base, threshold: t, slipCost: slipCost})
		} else if sameKind && bar(kind, threshold) < bar(existing.kind, existing.threshold) {
			existing.threshold = threshold
		}
	}
	values := func() []*candidate {
		out := make([]*candidate, len(found.keys))
		for i, k := range found.keys {
			out[i] = found.vals[k]
		}
		return out
	}
	capitalized := tr.IsUpperInitial(typed)
	isName := capitalized && !context.AtSentenceStart
	realTyped := c.isRealWord(word)
	hasApostrophe := strings.Contains(word, "'")
	wordStem, wordSuffix := "", ""
	if hasApostrophe {
		wordStem, wordSuffix, _ = strings.Cut(word, "'")
	}

	// In text that already uses Turkish marks, protect real Turkish words ("cabası", "oluyorduk", "basınız") with a
	// high override bar, while relaxing strict protection for low-count lowercase non-words ("yogurt", "yasam").
	realMarked := context.TextHasMarks && realTyped
	relaxStrict := c.hasMorph() && typed == word && !realTyped && typedCount < 70 && c.lexicon.CapitalShare(word) < 0.55
	strict := context.TextHasMarks && (realTyped || (typedCount >= knownMarked && !relaxStrict))
	override := markedOverride
	if realMarked {
		override = markedOverride + 0.5
	}

	variants := tr.MarkVariants(word, 512)
	filterInvalidMarks := c.hasMorph() && !isName && !strict && !realTyped && !hasApostrophe &&
		slices.ContainsFunc(variants, func(v string) bool { return c.lexicon.Count(v) > 0 && c.isRealWord(v) })

	forms := append([]string{word}, variants...)
	typedLowerCount := jsRound(typedCount * (1 - c.lexicon.CapitalShare(word)))
	morphParticleHit := false
	strongASCIIMarkHit := false
	firstRune := func(s string) rune {
		r, n := utf8.DecodeRuneInString(s)
		if n == 0 {
			return -1
		}
		return r
	}
	for _, form := range forms {
		markCost := cost[KindMarks]
		if form == word {
			markCost = 0
		}
		if form != word {
			blockMark := false
			sameApostropheStem := false
			if hasApostrophe {
				formStem, formSuffix := form, ""
				if q := strings.IndexByte(form, '\''); q >= 0 {
					formStem, formSuffix = form[:q], form[q+1:]
				} else {
					formStem, formSuffix = sliceRunes(form, 0, -1), form
				}
				sameApostropheStem = formStem == wordStem
				onlyLeadingCapitalI := firstRune(typed) == 'I' && firstRune(wordStem) == 'ı' && firstRune(formStem) == 'i' && sliceFrom(wordStem, 1) == sliceFrom(formStem, 1)
				if context.TextHasMarks && !sameApostropheStem && !onlyLeadingCapitalI && (c.lexicon.Count(wordStem) >= knownMarked || c.isRealWord(wordStem)) {
					blockMark = true
				}
				if !blockMark && formSuffix != wordSuffix {
					isInverseStem := c.hasMorph() && c.hasFrontLocative(formStem) && !c.hasBackLocative(formStem)
					if !IsHarmoniousApostropheMarkChange(formStem, wordSuffix, formSuffix, isInverseStem) {
						blockMark = true
					}
				}
			}
			if !blockMark && !(filterInvalidMarks && !c.isRealWord(form)) {
				count := c.lexicon.Count(form)
				score := lg(count) - cost[KindMarks]
				formValid := c.hasMorph() && c.isRealWord(form)
				if count > 0 && (!strict || score-typedScore >= override) {
					suffixOnlyApostrophe := hasApostrophe && typedCount == 0 && !strict && count >= 7 && sameApostropheStem
					threshold := undef
					if suffixOnlyApostrophe {
						threshold = math.Max(1.5, c.Settings.Threshold[KindMarks]-1.0)
					}
					// In sentences proven to be typed in ASCII Turkish, an invalid lowercase form ("yuku", "super",
					// "dali") restoring to a valid Turkish word needs less log-frequency gap than in normal marked text.
					if !defined(threshold) && !context.TextHasMarks && context.SentenceHasASCIITurkish && !realTyped {
						if typed == word && formValid && runeLen(word) >= 3 && count >= 10 {
							threshold = math.Max(0.7, c.Settings.Threshold[KindMarks]-1.8)
						} else if typedCount == 0 && count >= 5 && (!capitalized || !hasApostrophe || sameApostropheStem || c.lexicon.CapitalShare(form) >= 0.5) {
							// An unseen word in an ASCII-typed Turkish sentence with an attested mark variant ("Gülruh",
							// "Çapari", "Divanı'ndan").
							threshold = math.Max(1.2, c.Settings.Threshold[KindMarks]-1.3)
						}
					}
					add(form, KindMarks, score, typedScore, threshold, undef)
					// Lowercase-vs-lowercase count comparison when the typed lowercase word's corpus count is
					// dominated by a capitalized proper name ("egrisi", "dali", "makasi", "avi", "karin").
					if !strict && typed == word && c.lexicon.CapitalShare(word) >= 0.5 && formValid {
						candLower := jsRound(count * (1 - c.lexicon.CapitalShare(form)))
						if candLower >= 10 {
							add(form, KindMarks, lg(candLower)-cost[KindMarks], lg(typedLowerCount), threshold, undef)
						}
					}
					if !context.TextHasMarks && !realTyped && formValid && score-typedScore >= or(threshold, c.Settings.Threshold[KindMarks]) {
						strongASCIIMarkHit = true
					}
				}
			}
			if strict {
				continue
			}
		}
		// A real word is never heuristically particle-split (guarded inside addSplits via formIsReal), but strongly
		// attested bigram splits (pair >> typedCount, e.g. "ihtimalide" -> "ihtimali de") still apply.
		hit := c.addSplits(form, context, markCost, isName, capitalized, add)
		if form == word && hit {
			morphParticleHit = true
		}
		if !strings.Contains(form, "'") {
			if isName {
				c.addApostrophes(form, word, typedCount, typedScore, markCost, false, context.PrevIsName, context.PrevCapitalized, context.PrevWord, add)
			} else if capitalized && context.AtSentenceStart {
				c.addApostrophes(form, word, typedCount, typedScore, markCost, true, false, false, "", add)
			}
		}
	}

	// A unique morphologically valid mark restoration for unseen or rare lowercase invalid words ("yukleyince",
	// "ezilmisti", "gidecegiz").
	if c.hasMorph() && !capitalized && !strict && !realTyped && !hasApostrophe && typedCount < 5 && runeLen(word) >= 6 && len(variants) <= 128 {
		var validVariants []string
		for _, v := range variants {
			if c.isRealWord(v) {
				validVariants = append(validVariants, v)
			}
		}
		uniqueThreshold := math.Max(1.5, c.Settings.Threshold[KindMarks]-1.0)
		if len(validVariants) == 1 {
			only := validVariants[0]
			vCount := c.lexicon.Count(only)
			if vCount > 0 {
				score := lg(vCount) - cost[KindMarks]
				if typedCount == 0 {
					score = math.Max(lg(vCount), 2.5) - cost[KindMarks]
				}
				add(only, KindMarks, score, typedScore, uniqueThreshold, undef)
			} else if typedCount == 0 && runeLen(word) >= 8 && !tr.HasTurkishMarks(word) && c.isSafeUnseenMarkVariant(word, only, context) {
				add(only, KindMarks, 2.5-cost[KindMarks], typedScore, uniqueThreshold, undef)
			}
		} else if typedCount == 0 && runeLen(word) >= 7 && len(validVariants) > 1 {
			// When several theoretical morph variants exist ("ısınmadır" vs "ışınmadır", "eşleşmiyorsa" vs
			// "esleşmiyorsa"), pick the one actually attested in the lexicon (count >= 3), if it is the only one.
			var seenValid []string
			for _, v := range validVariants {
				if c.lexicon.Count(v) >= 3 {
					seenValid = append(seenValid, v)
				}
			}
			if len(seenValid) == 1 {
				only := seenValid[0]
				vCount := c.lexicon.Count(only)
				add(only, KindMarks, math.Max(lg(vCount), 2.5)-cost[KindMarks], typedScore, uniqueThreshold, undef)
			}
		}
	}

	// An unambiguous internal vowel disharmony inside an apostrophe suffix ("Ukrayna'sinda" -> "Ukrayna'sında").
	if hasApostrophe && typedCount == 0 && !strict {
		if fixedSuffix, ok := FixDisharmoniousApostropheSuffix(word); ok {
			add(fixedSuffix, KindMarks, math.Max(lg(c.lexicon.Count(fixedSuffix)), 3.0)-cost[KindMarks], typedScore, math.Max(1.5, c.Settings.Threshold[KindMarks]-1.0), undef)
		}
	}

	// Standalone question particle harmony in ASCII-typed sentences ("var mi" -> "var mı", "gördü mu" -> "gördü mü").
	if !context.TextHasMarks && context.PrevWord != "" && !context.PrevIsName && runeLen(context.PrevWord) >= 2 {
		c.addQuestionHarmony(word, context, typedScore, nextTyped, add)
	}

	// All-caps acronyms or mixed-case brands glued to a Turkish suffix ("HEXIMden" -> "HEXIM'den", "ABDde" ->
	// "ABD'de", "MacOSdaki" -> "MacOS'daki").
	if typedCount < 10 && !realTyped && !hasApostrophe {
		if acronym, ok := MatchAcronymApostrophe(typed); ok {
			prefixLower := tr.Lower(acronym.Prefix)
			if c.lexicon.Count(prefixLower) < 500 || c.lexicon.CapitalShare(prefixLower) >= 0.5 || (c.hasMorph() && !c.isRealWord(prefixLower)) {
				add(acronym.WithMark, KindApostrophe, math.Max(3.5, typedScore+c.Settings.Threshold[KindApostrophe]+0.6)-cost[KindApostrophe], typedScore, undef, undef)
			}
		}
	}

	// Two-error and compound recoveries for unseen lowercase invalid words:
	// 1. A question particle + a 1-edit slip ("biliyormusunm?" -> "biliyor musun?").
	// 2. Two glued words with optional missing Turkish marks ("yapmıyorsunanlamadim" -> "yapmıyorsun anlamadım").
	// Never compete with an already-passing single-word mark restoration ("muzikevinde" -> "müzikevinde").
	hasInternalUpper := sliceFrom(typed, 1) != sliceFrom(word, 1)
	hasPassingMark := slices.ContainsFunc(values(), func(cand *candidate) bool {
		return cand.kind == KindMarks && marginOf(cand) >= or(cand.threshold, c.Settings.Threshold[KindMarks])
	})
	if c.hasMorph() && typedCount == 0 && !realTyped && !isName && !hasApostrophe && !hasInternalUpper && !morphParticleHit && !strongASCIIMarkHit && !hasPassingMark {
		if runeLen(word) >= 6 {
			c.addQuestionParticleTypoSplits(word, context, typedScore, add)
		}
		if runeLen(word) >= 10 {
			c.addTwoWordGluedSplits(word, typedScore, add)
		}
	}

	// Short 3-letter typo recovery ("bne" -> "ben", "ane" -> "anne") under strict morphology and frequency guards.
	// Never compete with words that already have a valid Turkish mark variant ("goz" -> "göz", "guc" -> "güç").
	if c.hasMorph() && runeLen(word) == 3 && !realTyped && !isName && !hasApostrophe && !hasInternalUpper && !morphParticleHit && !strongASCIIMarkHit &&
		typedCount <= 120 &&
		(typed == word || (context.AtSentenceStart && c.lexicon.CapitalShare(word) < 0.5)) &&
		!slices.ContainsFunc(variants, func(v string) bool { return c.lexicon.ExactCount(v) >= 50 && c.isRealWord(v) }) {
		c.addShortWordTypos(word, context, typedCount, typedScore, typedLowerCount, nextTyped, add)
	}

	// Slips of the fingers are only considered for rare or unseen words that aren't names. A rare form that Turkish
	// suffix rules accept is a real word, not a slip ("getirsek" is not "getirmek"). Do not let single-edit typos
	// hijack or tie a morphologically proven particle split ("boyunuda" -> "boyunu da") or a clear mark restoration
	// in an ASCII-typed sentence ("gecenleri" -> "geçenleri" vs "geceleri"). A lowercase word that Turkish rules
	// reject and that is only seen as a rare misspelling in the corpus ("brada" 13 times, "burada" 33,000) is still
	// a slip: it may be fixed, but only into a word seen 200 times as often.
	rareInvalidSlip := c.hasMorph() && typed == word && typedCount < 50 && !realTyped && c.lexicon.CapitalShare(word) < 0.5
	if (typedCount < knownWord || rareInvalidSlip) && runeLen(word) >= 4 && !isName && !hasApostrophe && !hasInternalUpper && !realTyped && !morphParticleHit && !strongASCIIMarkHit {
		// Do not let single-edit typos mangle an unsplit harmonized question particle ("olurmu", "yaparmı", "varmı").
		unsplitQuestion := false
		if q := qMatchRE.FindStringSubmatch(word); q != nil {
			unsplitQuestion = FollowsHarmony(q[1], sliceRunes(q[2], 1, 2)) && SameHighVowels(q[2]) && c.isRealWord(q[1]) && c.lexicon.Count(q[1]) >= 5
		}
		if !unsplitQuestion {
			unseenLowerInvalid := c.hasMorph() && typedCount == 0 && typed == word
			prevIsAcc := false
			if unseenLowerInvalid && context.PrevWord != "" {
				prevIsAcc = c.isUnambiguousAccusative(context.PrevWord)
			}
			edits := c.oneEdit(word, unseenLowerInvalid)
			for _, text := range edits.keys {
				slipCost := edits.vals[text]
				count := c.lexicon.ExactCount(text)
				if unseenLowerInvalid && c.Settings.TypoStemShare > 0 && runeLen(text) >= 7 && c.isRealWord(text) {
					count = math.Max(count, c.stemCount(text)*c.Settings.TypoStemShare)
				}
				morphValid := c.hasMorph() && count >= 1 && c.isRealWord(text)
				// For attested-but-rare valid forms ("gönderdim" 20, "beğendim" 24) in long unseen words, allow a
				// capped morphological prior boost (never boosting consonant->vowel substitutions like "göstermdn" ->
				// "göstermen").
				if unseenLowerInvalid && morphValid && runeLen(word) >= 8 && count >= 10 && slipCost <= 1.25 {
					count = math.Max(count, c.effectiveCount(text, "autofix"))
				}
				if count < 1 {
					continue
				}
				if typedCount >= knownWord && (count < 200*typedCount || !c.isRealWord(text)) {
					continue
				}
				// Never rewrite a word into a vowelless interjection or abbreviation ("Hımm" -> "Hmm", "km", "vb").
				if !vowelRE2.MatchString(text) {
					continue
				}
				// A lowercase slip shouldn't rewrite into a mostly-capitalized proper name ("krate" -> "kate", "lmre"
				// -> "emre").
				if typed == word && c.lexicon.CapitalShare(text) >= 0.6 {
					continue
				}
				morphPenalty := 0.0
				if c.hasMorph() && !morphValid {
					morphPenalty = 1.5
				}
				voicePenalty := 0.0
				if prevIsAcc && morphValid && c.isPurelyPassiveVerb(text) {
					voicePenalty = 1.4
				}
				// For long unseen lowercase invalid words, a morphologically valid high-confidence keyboard edit (swap,
				// near-key sub or extra, or missed key) needs less frequency evidence ("gönderdim",
				// "yoğunluklarında").
				threshold := undef
				if unseenLowerInvalid && morphValid && runeLen(word) >= 7 && slipCost <= 1.25 && count >= 20 {
					threshold = 0.6
				}
				add(text, KindTypo, lg(count)-cost[KindTypo]-slipCost-morphPenalty-voicePenalty, typedScore, threshold, slipCost)
			}
			// Principled generative-prior + mark-loss-plus-1-edit recovery for long unseen lowercase invalid words
			// ("duzenblemiyor" -> "düzenlemiyor", "kullandğımda" -> "kullandığımda", "puanlrım" -> "puanlarım"). Only
			// when the 1-edit lexicon search didn't already find a passing fix; it needs length >= 8, a
			// non-substitution edit when combined with mark restoration (prevents "suhenin" -> "şubenin"), and a
			// capped prior (keeps zero-count rare verb forms like "gerektin" from hijacking double omissions like
			// "gerektn").
			if unseenLowerInvalid && runeLen(word) >= 8 && c.fuzzy != nil {
				hasPassing1 := slices.ContainsFunc(values(), func(cand *candidate) bool {
					return marginOf(cand) >= or(cand.threshold, c.Settings.Threshold[cand.kind])
				})
				if !hasPassing1 {
					for _, f := range c.fuzzy.FuzzySearch(word, 1, 15) {
						if f.EditCost > 1.25 {
							continue
						}
						if c.lexicon.CapitalShare(f.Word) >= 0.6 {
							continue
						}
						// When restoring Turkish marks AND editing a letter, require length >= 9 and a length change
						// or a transposition (blocks 2-consonant substitutions like "suhenin" -> "şubenin" while
						// keeping "duzenblemiyor" -> "düzenlemiyor").
						marksChanged := foldTr(f.Word) != f.Word && foldTr(word) != f.Word && runeLen(f.Word) == runeLen(word)
						if marksChanged && f.EditCost > 1.1 {
							continue
						}
						eff := c.effectiveCount(f.Word, "autofix")
						if eff < 20 {
							continue
						}
						voicePenalty := 0.0
						if prevIsAcc && c.isPurelyPassiveVerb(f.Word) {
							voicePenalty = 1.4
						}
						threshold := 1.5
						if runeLen(word) >= 9 && eff >= 28 {
							threshold = 0.6
						}
						add(f.Word, KindTypo, lg(eff)-cost[KindTypo]-f.EditCost-voicePenalty, typedScore, threshold, f.EditCost)
					}
				}
			}
			// A cautious 2-vowel-drop recovery ("gelyorm" -> "geliyorum", "çalışıyrm" -> "çalışıyorum") when the
			// 1-edit search finds nothing strong.
			if unseenLowerInvalid && runeLen(word) >= 6 {
				hasPassing := slices.ContainsFunc(values(), func(cand *candidate) bool {
					return marginOf(cand) >= or(cand.threshold, c.Settings.Threshold[cand.kind])
				})
				if !hasPassing {
					c.addDoubleVowelDrops(word, typedScore, add)
				}
			}
		}
	}
	return values()
}

// runeOr is s[i] as a one-letter string, or "" (undefined) out of range.
func runeOr(rs []rune, i int) string {
	if i < 0 || i >= len(rs) {
		return ""
	}
	return string(rs[i])
}

// addShortWordTypos is the strict 3-letter typo recovery ("bne" -> "ben", "ane" -> "anne"): it needs an invalid
// stem and a valid, very frequent target.
func (c *Corrector) addShortWordTypos(word string, context *WordContext, typedCount, typedScore, typedLowerCount float64, nextTyped string, add addCandidate) {
	nextLower := ""
	if nextTyped != "" && !tr.IsAllUpper(nextTyped) {
		nextLower = tr.Lower(nextTyped)
	}
	// Protect attested foreign or phrase tokens ("van der", "de la", "los angeles").
	if context.PrevWord != "" && c.lexicon.Pair(context.PrevWord, word) > 0 {
		return
	}
	if nextLower != "" && c.lexicon.Pair(word, nextLower) > 0 {
		return
	}

	rs := []rune(word)
	edits := newOrderedMap[float64]()
	put := func(cand string, slip float64) {
		if cand == word {
			return
		}
		if old, ok := edits.get(cand); !ok || old > slip {
			edits.set(cand, slip)
		}
	}
	// Adjacent swaps ("bne" -> "ben").
	swaps := NewWordSet(string([]rune{rs[1], rs[0], rs[2]}), string([]rune{rs[0], rs[2], rs[1]}))
	for _, swap := range swaps.Words() {
		put(swap, 0.7)
	}
	// Single letter insertions (a doubled consonant like "ane" -> "anne" is cheapest; a leading insert is heavily
	// penalized).
	for i := 0; i <= len(rs); i++ {
		before, after := string(rs[:i]), string(rs[i:])
		prev, at := runeOr(rs, i-1), runeOr(rs, i)
		for _, letter := range tr.Alphabet {
			l := string(letter)
			isDouble := l == prev || l == at
			slip := 1.2
			switch {
			case i == 0 && isDouble:
				slip = 1.8
			case i == 0:
				slip = 2.3
			case isDouble:
				slip = 0.8
			}
			put(before+l+after, slip)
		}
	}
	// Near-key substitutions only for completely unseen 3-letter words ("bwn" -> "ben", "vqr" -> "var").
	if typedCount == 0 {
		for i, at := range rs {
			for _, letter := range tr.Alphabet {
				if letter != at && tr.AreNearKeys(letter, at) {
					slip := 1.0
					if i == 0 {
						slip = 2.0
					}
					put(string(rs[:i])+string(letter)+string(rs[i+1:]), slip)
				}
			}
		}
	}

	cost := c.Settings.Cost
	base := math.Min(typedScore, lg(typedLowerCount))
	for _, cand := range edits.keys {
		slip := edits.vals[cand]
		count := c.lexicon.ExactCount(cand)
		// A swap is the commonest 3-letter slip ("bka" -> "bak"): it needs less frequency evidence than an insertion.
		minCount, ratio := 1500.0, 50.0
		if swaps.Has(cand) {
			minCount = math.Min(1500, c.Settings.ShortSwapMinCount)
			if c.Settings.ShortSwapMinCount < 1500 {
				ratio = 20
			}
		}
		if count < minCount || count < typedCount*ratio {
			continue
		}
		if c.lexicon.CapitalShare(cand) >= 0.5 || !c.isRealWord(cand) {
			continue
		}
		pairBonus := 0.0
		if (context.PrevWord != "" && c.lexicon.Pair(context.PrevWord, cand) > 0) || (nextLower != "" && c.lexicon.Pair(cand, nextLower) > 0) {
			pairBonus = 0.8
		}
		add(cand, KindTypo, lg(count)-cost[KindTypo]-slip+pairBonus, base, 1.5, undef)
	}
}

// questionHarmonyLetters are the letters tried in a mistyped question particle.
const questionHarmonyLetters = "ıiuümnsyzdrke"

// addQuestionParticleTypoSplits recovers glued question particles with a 1-edit slip ("biliyormusunm?" ->
// "biliyor musun?").
func (c *Corrector) addQuestionParticleTypoSplits(word string, context *WordContext, typedScore float64, add addCandidate) {
	cost := c.Settings.Cost
	rs := []rune(word)
	for cut := 3; cut <= len(rs)-3; cut++ {
		first := string(rs[:cut])
		sr := rs[cut:]
		second := string(sr)
		if sr[0] != 'm' || len(sr) < 3 || len(sr) > 10 || Question.MatchString(second) {
			continue
		}
		firstCount := c.lexicon.ExactCount(first)
		if firstCount < 10 || c.lexicon.CapitalShare(first) >= 0.5 || !c.isRealWord(first) {
			continue
		}
		// Direct 1-edit candidates on the question particle (without word-final suffix-deletion penalties).
		qEdits := newOrderedMap[float64]()
		putQ := func(cand string, slip float64) {
			if runeLen(cand) < 2 {
				return
			}
			if old, ok := qEdits.get(cand); !ok || old > slip {
				qEdits.set(cand, slip)
			}
		}
		for i := 1; i < len(sr); i++ {
			at := sr[i]
			prev := sr[i-1]
			hasNext := i+1 < len(sr)
			var next rune
			if hasNext {
				next = sr[i+1]
			}
			nearExtra := at == prev || tr.AreNearKeys(at, prev) || (hasNext && (at == next || tr.AreNearKeys(at, next)))
			if nearExtra {
				putQ(string(sr[:i])+string(sr[i+1:]), 0.9)
			}
			if hasNext {
				putQ(string(sr[:i])+string(next)+string(at)+runeSlice(sr, i+2, len(sr)), 0.7)
			}
			for _, ch := range questionHarmonyLetters {
				if ch != at && tr.AreNearKeys(ch, at) {
					putQ(string(sr[:i])+string(ch)+string(sr[i+1:]), 0.9)
				}
				putQ(string(sr[:i])+string(ch)+string(sr[i:]), 1.0)
			}
		}
		for _, cand := range qEdits.keys {
			slip := qEdits.vals[cand]
			checks := []string{cand}
			if !context.TextHasMarks && !tr.HasTurkishMarks(cand) {
				checks = append(checks, tr.MarkVariants(cand, 16)...)
			}
			for _, q := range checks {
				if Question.MatchString(q) && SameHighVowels(q) && FollowsHarmony(first, sliceRunes(q, 1, 2)) && (context.QuestionAhead || runeLen(q) >= 4) {
					markPen := 0.0
					if q != cand {
						markPen = cost[KindMarks]
					}
					add(first+" "+q, KindSplit, lg(firstCount)-cost[KindSplit]-slip-markPen, typedScore, 2.5, undef)
				}
			}
		}
	}
}

// gluedPart is one resolved half of a long glued two-word error.
type gluedPart struct {
	text   string
	count  float64
	marked bool
}

// resolveGluedPart resolves one half of a long glued two-word error, restoring missing Turkish marks if need be.
func (c *Corrector) resolveGluedPart(part string) *gluedPart {
	if runeLen(part) < 4 {
		return nil
	}
	if c.isRealWord(part) {
		count := c.lexicon.ExactCount(part)
		if count >= 100 && c.lexicon.CapitalShare(part) < 0.45 {
			return &gluedPart{text: part, count: count}
		}
	}
	var best *gluedPart
	secondBest := 0.0
	for _, v := range tr.MarkVariants(part, 128) {
		count := c.lexicon.ExactCount(v)
		if count < 100 || c.lexicon.CapitalShare(v) >= 0.45 || !c.isRealWord(v) {
			continue
		}
		if best == nil || count > best.count {
			if best != nil {
				secondBest = math.Max(secondBest, best.count)
			}
			best = &gluedPart{text: v, count: count}
		} else {
			secondBest = math.Max(secondBest, count)
		}
	}
	if best != nil && (secondBest == 0 || best.count >= secondBest*10) {
		return &gluedPart{text: best.text, count: best.count, marked: true}
	}
	return nil
}

// addTwoWordGluedSplits splits two glued words, even with missing Turkish marks ("yapmıyorsunanlamadim" ->
// "yapmıyorsun anlamadım").
func (c *Corrector) addTwoWordGluedSplits(word string, typedScore float64, add addCandidate) {
	cost := c.Settings.Cost
	rs := []rune(word)
	for cut := 4; cut <= len(rs)-4; cut++ {
		// A doubled tail is a repeated suffix ("değil-dirdir"), not a second word ("dırdır").
		tail := rs[cut:]
		if len(tail)%2 == 0 && string(tail[:len(tail)/2]) == string(tail[len(tail)/2:]) {
			continue
		}
		left := c.resolveGluedPart(string(rs[:cut]))
		if left == nil {
			continue
		}
		right := c.resolveGluedPart(string(rs[cut:]))
		if right == nil {
			continue
		}
		// Require a finite verb on either side (never an OOV noun compound) or an attested bigram.
		if !c.isFiniteVerb(left.text) && !c.isFiniteVerb(right.text) && c.lexicon.Pair(left.text, right.text) < 5 {
			continue
		}
		markPen := 0.0
		if left.marked {
			markPen += cost[KindMarks]
		}
		if right.marked {
			markPen += cost[KindMarks]
		}
		score := math.Min(lg(left.count), lg(right.count)) - cost[KindSplit] - markPen - 0.8
		add(left.text+" "+right.text, KindSplit, score, typedScore, 2.5, undef)
	}
}

// doubleDropVowels are the vowels addDoubleVowelDrops puts back.
var doubleDropVowels = []string{"a", "e", "ı", "i", "o", "u", "ü"}

// addDoubleVowelDrops is a cautious 2-vowel-drop recovery between consonant clusters ("gelyorm" -> "geliyorum",
// "çalışıyrm" -> "çalışıyorum").
func (c *Corrector) addDoubleVowelDrops(word string, typedScore float64, add addCandidate) {
	rs := []rune(word)
	var gaps []int
	for i := 1; i < len(rs); i++ {
		if !isVowel(rs[i-1]) && !isVowel(rs[i]) {
			gaps = append(gaps, i)
		}
	}
	if len(gaps) < 2 || len(gaps) > 5 {
		return
	}
	cost := c.Settings.Cost
	for a := 0; a < len(gaps); a++ {
		p1 := gaps[a]
		for b := a + 1; b < len(gaps); b++ {
			p2 := gaps[b]
			s0, s1, s2 := string(rs[:p1]), string(rs[p1:p2]), string(rs[p2:])
			for _, v1 := range doubleDropVowels {
				for _, v2 := range doubleDropVowels {
					cand := s0 + v1 + s1 + v2 + s2
					count := c.lexicon.ExactCount(cand)
					if count < 50 || c.lexicon.CapitalShare(cand) >= 0.5 || !c.isRealWord(cand) {
						continue
					}
					add(cand, KindTypo, lg(count)-cost[KindTypo]-0.8, typedScore, 1.5, 0.8)
				}
			}
		}
	}
}

// addQuestionHarmony restores "mi/mu/misin/..." marks from the preceding word's vowels in ASCII-typed sentences.
func (c *Corrector) addQuestionHarmony(word string, context *WordContext, typedScore float64, nextTyped string, add addCandidate) {
	prev := context.PrevWord
	if prev == "" || MusicNotes[prev] || !Question.MatchString(word) {
		return
	}
	if nextTyped != "" && MusicAfter[tr.Lower(nextTyped)] {
		return
	}
	prevValid := c.lexicon.Count(prev) >= 5
	if c.hasMorph() {
		prevValid = c.isRealWord(prev)
	}
	if !prevValid || c.lexicon.CapitalShare(prev) >= 0.55 {
		return
	}
	frontLoc := c.hasFrontLocative(prev)
	backLoc := c.hasBackLocative(prev)
	if frontLoc && backLoc {
		return
	}
	lexFront := c.lexicon.Count(prev+"de") + c.lexicon.Count(prev+"te")
	lexBack := c.lexicon.Count(prev+"da") + c.lexicon.Count(prev+"ta")
	isInverse := frontLoc || (!c.hasMorph() && lexFront >= 3 && lexFront > lexBack)
	target, ok := HarmonizeQuestionParticle(word, prev, isInverse)
	if !ok {
		return
	}
	typedPair := c.lexicon.Pair(prev, word)
	targetPair := c.lexicon.Pair(prev, target)
	if typedPair > 0 && typedPair >= targetPair {
		return
	}
	hasClue := context.QuestionAhead || targetPair > 0
	if !hasClue {
		if m := Question.FindStringSubmatchIndex(word); m != nil && m[4] >= 0 {
			hasClue = true
		}
	}
	if !hasClue {
		return
	}
	add(target, KindMarks, typedScore+math.Max(c.Settings.Threshold[KindMarks], 3.0), typedScore, undef, undef)
}

// questionParts matches a question particle: its "m" + vowel, and whether a person or tense ending follows.
func questionParts(s string) (head string, hasTail bool, ok bool) {
	m := Question.FindStringSubmatchIndex(s)
	if m == nil {
		return "", false, false
	}
	return s[m[2]:m[3]], m[4] >= 0, true
}

// addSplits offers "birsey" -> "bir şey" from how often the pair is seen, and "geldinmi", "arenasıda", "diyorki"
// from grammar and the stem. It reports whether a morphologically proven particle split was found.
func (c *Corrector) addSplits(form string, context *WordContext, markCost float64, isName, capitalized bool, add addCandidate) bool {
	cost := c.Settings.Cost
	glued := c.lexicon.Count(form)
	formIsReal := c.hasMorph() && c.isRealWord(form)
	isProperAtStart := capitalized && c.lexicon.CapitalShare(form) >= 0.5
	morphParticleHit := false
	rs := []rune(form)
	for cut := 2; cut <= len(rs)-2; cut++ {
		first, second := string(rs[:cut]), string(rs[cut:])
		if strings.Contains(second, "'") || NeverAlone[second] {
			continue
		}
		pair := c.lexicon.Pair(first, second)
		foreignProperSplit := isName && c.hasMorph() && (!c.isRealWord(first) || !c.isRealWord(second))
		if pair > 0 && !foreignProperSplit {
			add(first+" "+second, KindSplit, lg(pair)-cost[KindSplit]-markCost, undef, undef, undef)
		}
		// Glued particles after an apostrophe-inflected proper name ("Çin'dirki", "Ankara'dada", "İstanbul'damı?").
		if strings.Contains(first, "'") {
			if glued == 0 && !formIsReal && c.lexicon.Count(first) > 0 {
				_, suf, _ := strings.Cut(first, "'")
				if second == "ki" && kiCopulaSuffix.MatchString(suf) {
					add(first+" "+second, KindSplit, math.Max(lg(c.lexicon.Count(first)), 3.2)-cost[KindSplit]-markCost, undef, undef, undef)
				} else if Also.MatchString(second) && FollowsHarmony(first, sliceRunes(second, 1, 2)) && alsoCaseSuffix.MatchString(suf) {
					add(first+" "+second, KindSplit, math.Max(lg(c.lexicon.Count(first)), 3.2)-cost[KindSplit]-markCost, undef, undef, undef)
				} else if head, hasTail, ok := questionParts(second); ok && FollowsHarmony(first, sliceRunes(head, 1, 2)) && SameHighVowels(second) && (context.QuestionAhead || hasTail) {
					add(first+" "+second, KindSplit, math.Max(lg(c.lexicon.Count(first)), 3.2)-cost[KindSplit]-markCost, undef, undef, undef)
				}
			}
			continue
		}
		// TDK rule: Noun + etmek compound verbs without a sound dropped or doubled are always written separate
		// ("farkettim" -> "fark ettim", "terketti" -> "terk etti", "kabuledildi" -> "kabul edildi", "devamediyor" ->
		// "devam ediyor"). Guarded by !formIsReal, so genuine joined verbs like "affettim", "hissettim",
		// "kaybettim" are never touched.
		if !formIsReal && !isName && !isProperAtStart && c.hasMorph() &&
			(strings.HasPrefix(second, "et") || strings.HasPrefix(second, "ed")) &&
			!JoinedEtmekStems[first] &&
			((runeLen(second) >= 4 && (runeLen(first) >= 4 || SeparateEtmekShort[first])) || (second == "et" && SeparateEtmekBareImperative[first])) &&
			c.lexicon.Count(first) >= 50 &&
			c.lexicon.CapitalShare(first) < 0.5 &&
			c.isBareNominalRoot(first) &&
			c.isVerbFormOf(second, "et") {
			base := lg(glued)
			targetScore := math.Max(lg(c.lexicon.Count(first)), base+c.Settings.Threshold[KindSplit]+cost[KindSplit]+0.1)
			add(first+" "+second, KindSplit, targetScore-cost[KindSplit]-markCost, base, undef, undef)
			morphParticleHit = true
			continue
		}
		// A morphologically valid word ("giyside", "güveninde", "kitabımı", "evdeki") is never particle-split.
		if formIsReal || glued >= c.Settings.KnownMarked || runeLen(first) < 3 {
			continue
		}
		stem := c.lexicon.Count(first)
		morphProven := c.hasMorph() && c.isRealWord(first)

		if head, hasTail, ok := questionParts(second); ok && FollowsHarmony(first, sliceRunes(head, 1, 2)) && SameHighVowels(second) {
			hasQuestionClue := context.QuestionAhead || hasTail
			if hasQuestionClue {
				if morphProven && (stem > 0 || runeLen(first) >= 6) && (!isName || c.lexicon.CapitalShare(first) < 0.5) && !isProperAtStart {
					score := lg(stem) - cost[KindSplit] - markCost
					if glued == 0 {
						score = math.Max(lg(stem), 3.2) - cost[KindSplit] - markCost
					}
					add(first+" "+second, KindSplit, score, undef, undef, undef)
					if runeLen(first) >= 4 {
						morphParticleHit = true
					}
				} else if stem > 0 && !isName {
					add(first+" "+second, KindSplit, lg(stem)-cost[KindSplit]-markCost-1, undef, undef, undef)
				}
			} else if morphProven && !isName && !isProperAtStart && runeLen(first) >= 4 && glued < c.Settings.KnownWord && stem > 0 && c.lexicon.CapitalShare(first) < 0.5 {
				add(first+" "+second, KindSplit, lg(stem)-cost[KindSplit]-markCost, undef, undef, undef)
				morphParticleHit = true
			}
			continue
		}

		if Also.MatchString(second) && FollowsHarmony(first, sliceRunes(second, 1, 2)) && !isName && !isProperAtStart {
			if capitalized && c.isDerivedLanguageStem(first) {
				continue
			}
			voiced := "ta"
			if second == "de" {
				voiced = "te"
			}
			voicingTypo := c.hasMorph() && IsVoicelessFinal(first) && c.isRealWord(first+voiced)
			inverseHarmonyTypo := c.hasMorph() && second == "da" && c.hasFrontLocative(first)
			collapsed, isCollapsed := CollapsedDoubledLocative(first, second)
			doubledLocativeTypo := isCollapsed &&
				c.lexicon.Count(collapsed) >= 20 &&
				c.lexicon.Count(collapsed) >= math.Max(stem, 1)*10 &&
				(!c.hasMorph() || c.isRealWord(collapsed))
			if voicingTypo || inverseHarmonyTypo || doubledLocativeTypo {
				continue
			}
			if morphProven && (stem > 0 || runeLen(first) >= 6) {
				score := lg(stem) - cost[KindSplit] - markCost
				if glued == 0 {
					score = math.Max(lg(stem), 3.2) - cost[KindSplit] - markCost
				}
				// Morphologically impossible verb or conditional + de/da glues ("üzülsede", "istemesede") with low
				// corpus noise.
				impossibleVerbGlue := glued > 0 && glued < c.Settings.KnownMarked &&
					((pair >= 5 && pair >= glued*3) || (stem >= glued*4 && verbGlueEnd.MatchString(first)))
				threshold := undef
				if impossibleVerbGlue {
					threshold = math.Max(0.7, c.Settings.Threshold[KindSplit]-1.8)
				}
				add(first+" "+second, KindSplit, score, undef, threshold, undef)
				if runeLen(first) >= 4 {
					morphParticleHit = true
				}
			} else if (stem > 0 && NoLocativeAfter.MatchString(first)) || (c.hasMorph() && glued == 0 && runeLen(first) >= 8 && pluralAccEnd.MatchString(first)) {
				score := lg(stem) - cost[KindSplit] - markCost - 1
				if c.hasMorph() && glued == 0 && ((stem >= 3 && StrongNoLocativeAfter.MatchString(first)) || (runeLen(first) >= 8 && pluralAccEnd.MatchString(first))) {
					score = math.Max(lg(stem), 3.2) - cost[KindSplit] - markCost
				}
				add(first+" "+second, KindSplit, score, undef, undef, undef)
			}
			continue
		}

		if second == "ki" && runeLen(first) >= 4 && !isName && !isProperAtStart {
			if morphProven && (stem > 0 || runeLen(first) >= 6) {
				score := lg(stem) - cost[KindSplit] - markCost
				if glued == 0 {
					score = math.Max(lg(stem), 3.2) - cost[KindSplit] - markCost
				}
				add(first+" "+second, KindSplit, score, undef, undef, undef)
				morphParticleHit = true
			} else if stem > 0 && Finite.MatchString(first) {
				add(first+" "+second, KindSplit, lg(stem)-cost[KindSplit]-markCost-1, undef, undef, undef)
			}
		}
	}
	return morphParticleHit
}

// addApostrophes offers "Ankaraya" -> "Ankara'ya" when the form is seen, or when "ankara" is a name and "ya" a
// suffix that takes one.
func (c *Corrector) addApostrophes(form, word string, typedCount, typedScore, markCost float64, atSentenceStart, prevIsName, prevCapitalized bool, prevWord string, add addCandidate) {
	cost, minName := c.Settings.Cost, c.Settings.MinName
	hasMorph := c.hasMorph()
	formCount := c.lexicon.Count(form)
	if form == word {
		formCount = typedCount
	}
	typedCaps := jsRound(typedCount * c.lexicon.CapitalShare(word))
	realWord := c.isRealWord(word)
	rs, wr := []rune(form), []rune(word)
	wordFrom := func(cut int) string { return runeSlice(wr, cut, len(wr)) }
	for cut := 2; cut < len(rs); cut++ {
		stem, suffix := string(rs[:cut]), string(rs[cut:])
		withMark := stem + "'" + suffix
		seen := c.lexicon.Count(withMark)
		placeHead := !atSentenceStart && prevCapitalized && IsPlaceApostropheHead(stem, prevWord)
		blockedDone, blocked := false, false
		isBlocked := func() bool {
			if !blockedDone {
				blocked = c.isBlockedApostropheStem(stem, placeHead) || (!placeHead && c.isInflectedApostropheStem(stem))
				blockedDone = true
			}
			return blocked
		}
		// TDK geographic, place and state P3sg head words after a capitalized name ("Marmara Denizi'nin", "Türkiye
		// Cumhuriyeti'nin", "Kadıköy Mahallesi'nde", "Büyük Okyanus'ta", "Osmanlı Devleti'nin"). Institution head
		// words ("Üniversitesi", "Partisi", "Kurumu") are not among PlaceApostropheHeads.
		if placeHead && markCost == 0 && suffix == wordFrom(cut) && IsValidPlaceApostrophe(stem, suffix) {
			markCap := 0.0
			if seen > 0 {
				markCap = c.lexicon.CapitalShare(withMark)
			}
			seenCaps := jsRound(seen * markCap)
			if seen >= 5 && markCap >= 0.65 && seenCaps >= typedCaps*3 {
				add(withMark, KindApostrophe,
					math.Max(lg(seenCaps), lg(typedCaps)+c.Settings.Threshold[KindApostrophe]+0.5)-cost[KindApostrophe],
					lg(typedCaps), math.Max(1.5, c.Settings.Threshold[KindApostrophe]-1.0), undef)
				continue
			}
			if seen == 0 && typedCaps == 0 {
				add(withMark, KindApostrophe, c.Settings.Threshold[KindApostrophe]+0.5, 0, undef, undef)
				continue
			}
		}
		if seen > 0 {
			markCap := c.lexicon.CapitalShare(withMark)
			stemCap := c.lexicon.CapitalShare(stem)
			if atSentenceStart {
				if markCap >= 0.7 && stemCap >= 0.55 && !TimeNamesNoStartApostrophe[stem] && !isBlocked() {
					add(withMark, KindApostrophe, lg(seen)-cost[KindApostrophe]-markCost, typedScore, undef, undef)
				}
				continue
			}
			// Do not put an apostrophe on a blocked or inflected stem while restoring marks ("Odullerinde" ->
			// "Ödüllerinde").
			if markCost == 0 || !isBlocked() {
				threshold := undef
				if suffix == wordFrom(cut) && markCap >= 0.8 && typedCount == 0 && seen >= 7 && !isBlocked() {
					// An unseen mid-sentence proper name with an attested high-cap apostrophe form ("Discord'u",
					// "Arsenal'ı", "Karaca'dan").
					threshold = math.Max(1.5, c.Settings.Threshold[KindApostrophe]-1.0)
				} else if !realWord && suffix == wordFrom(cut) && markCap >= 0.8 && typedCount > 0 && typedCount <= 5 && seen >= 8 && seen >= typedCount*4 &&
					c.lexicon.Count(stem) >= 20 && stemCap >= 0.55 && !isBlocked() {
					// A non-Turkish word with a tiny typo count in the corpus ("Malatyaspora" 5 vs "Malatyaspor'a" 96,
					// "Hessin" 5 vs 64).
					threshold = math.Max(1.2, c.Settings.Threshold[KindApostrophe]-1.3)
				}
				add(withMark, KindApostrophe, lg(seen)-cost[KindApostrophe]-markCost, typedScore, threshold, undef)
			}
			// Mid-sentence, compare capitalized-mid-sentence counts too so a lowercase homograph ("elifin", "ekimde",
			// "aşilin") does not hide a clear proper-name apostrophe.
			seenCaps := jsRound(seen * markCap)
			if seenCaps >= 10 && seenCaps/seen >= 0.5 && stemCap >= 0.5 && !isBlocked() {
				// Honorific titles after a mid-sentence proper name ("Aclan Bey'in", "Ayşe Hanım'a") take an
				// apostrophe per TDK.
				honorificThreshold := undef
				if prevIsName && HonorificTitles[stem] {
					honorificThreshold = math.Max(1.2, c.Settings.Threshold[KindApostrophe]-1.3)
				}
				add(withMark, KindApostrophe, lg(seenCaps)-cost[KindApostrophe]-markCost, lg(typedCaps), honorificThreshold, undef)
			}
		}
		if atSentenceStart {
			continue
		}
		// The generic rule applies to unseen forms (or tiny Bloom-only non-word noise when attested) of clear names,
		// keeping the suffix as typed so a mark variant cannot compete with itself ("Weeknd'in" vs a fake
		// "Weeknd'ın").
		effectiveUnseen := formCount == 0 || (formCount <= 2 && !realWord && seen >= 7)
		if !effectiveUnseen || runeLen(stem) < 3 || suffix != wordFrom(cut) {
			continue
		}
		stemCount := c.lexicon.Count(stem)
		if stemCount < 5 || !IsValidGenericApostrophe(stem, suffix, hasMorph) || isBlocked() {
			continue
		}
		share := c.lexicon.CapitalShare(stem)
		suffixLen := runeLen(suffix)
		if stemCount >= minName {
			if suffixLen >= 2 && share >= 0.58 {
				add(withMark, KindApostrophe, lg(stemCount)-cost[KindApostrophe]-markCost-0.2, typedScore, undef, undef)
			} else if suffixLen == 1 && share >= 0.75 {
				add(withMark, KindApostrophe, lg(stemCount)-cost[KindApostrophe]-markCost-1.2, typedScore, undef, undef)
			} else if hasMorph && !realWord && !c.isRealWord(stem) && runeLen(stem) >= 4 && stemCount >= 25 && share >= 0.2 && suffixLen >= 3 {
				// An unseen foreign or brand stem with a lower cap share + an unambiguous 3+ letter Turkish suffix
				// ("Calypso'dan", "Fonte'nin", "Equestria'daki").
				add(withMark, KindApostrophe, lg(stemCount)-cost[KindApostrophe]-markCost-0.2, typedScore, undef, undef)
			}
		} else if hasMorph && formCount == 0 && !realWord && !c.isRealWord(stem) && runeLen(stem) >= 4 && share >= 0.55 && suffixLen >= 2 {
			// A rare foreign proper name (count 5..19, high cap share) with a harmonized 2+ or 3+ letter Turkish
			// suffix ("Tevet'te", "Salayev'dir", "Tatikios'un", "Morgane'nın", "Tsvangirai'ye").
			firstSuffixVowel := ""
			for _, ch := range suffix {
				if strings.ContainsRune("aeıioöuü", ch) {
					firstSuffixVowel = string(ch)
					break
				}
			}
			if suffixLen >= 3 || firstSuffixVowel == "" || FollowsHarmony(stem, firstSuffixVowel) {
				add(withMark, KindApostrophe, math.Max(lg(stemCount), 3.2)-cost[KindApostrophe]-markCost-0.2, typedScore, undef, undef)
			}
		}
	}
}

// joinCandidate is "hiç bir" -> "hiçbir", "oysa ki" -> "oysaki", "ev de" -> "evde", "vaz geçtim" -> "vazgeçtim",
// "ann e" -> "anne", or nil.
func (c *Corrector) joinCandidate(word, next, typed string, nextIsolated bool) *candidate {
	if strings.Contains(word, "'") || strings.Contains(next, "'") {
		return nil
	}
	// A misplaced space inside a single word ("ann e" -> "anne"): it needs a lowercase first token (protects the
	// proper name "Ann"), at least one morphologically invalid part (protects "ne de", "o da"), and a frequent valid
	// join.
	if c.hasMorph() && typed == word && nextIsolated && !JoinSecond[next] && !Question.MatchString(next) {
		wInvalid := !c.isRealWord(word)
		nInvalid := !c.isRealWord(next)
		nextLen := runeLen(next)
		if (wInvalid || (nInvalid && nextLen >= 2)) && (nextLen >= 2 || wInvalid) {
			joined := word + next
			if runeLen(joined) >= 4 && c.lexicon.CapitalShare(joined) < 0.5 && c.isRealWord(joined) {
				count := c.lexicon.ExactCount(joined)
				pair := c.lexicon.Pair(word, next)
				if count >= 50 && count >= math.Max(pair, 1)*50 {
					wLower := jsRound(c.lexicon.Count(word) * (1 - c.lexicon.CapitalShare(word)))
					nLower := jsRound(c.lexicon.Count(next) * (1 - c.lexicon.CapitalShare(next)))
					if (!wInvalid || wLower*5 < count) && (!nInvalid || nLower*5 < count) {
						return &candidate{text: joined, kind: KindJoin, score: lg(count) - c.Settings.Cost[KindJoin], base: lg(pair), joinsNext: true, threshold: 2.5, slipCost: undef}
					}
				}
			}
		}
	}
	// TDK joined compound verbs across all conjugations ("vaz geçtim" -> "vazgeçtim", "kayıp ettim" -> "kaybettim",
	// "his ediyorum" -> "hissediyorum", "baş vurdum" -> "başvurdum").
	if verbRules, ok := CompoundVerbJoins[word]; ok && c.hasMorph() {
		for _, rule := range verbRules {
			startsAux := strings.HasPrefix(next, rule.AuxStem) || (rule.AuxStem == "et" && strings.HasPrefix(next, "ed"))
			if !startsAux || !c.isVerbFormOf(next, rule.AuxStem) {
				continue
			}
			joined := rule.Prefix + next
			if !c.isRealWord(joined) {
				continue
			}
			pair := c.lexicon.Pair(word, next)
			if rule.StrictZeroPair && pair > 0 || !rule.StrictZeroPair && pair > 5 {
				continue
			}
			count := c.lexicon.Count(joined)
			if count == 0 {
				continue
			}
			return &candidate{
				text:      joined,
				kind:      KindJoin,
				score:     math.Max(lg(count), lg(pair)+c.Settings.Threshold[KindJoin]+0.5) - c.Settings.Cost[KindJoin],
				base:      lg(pair),
				joinsNext: true,
				threshold: undef,
				slipCost:  undef,
			}
		}
	}
	if word == "her" && next != "hangi" && next != "kes" {
		return nil
	}
	if !JoinFirst[word] && !JoinSecond[next] {
		return nil
	}
	joined := word + next
	count := c.lexicon.Count(joined)
	if count == 0 {
		return nil
	}
	// Lowercase joins must not accidentally form a proper noun ("bu şehr" -> "buşehr", "o dile" -> "odile").
	if c.lexicon.CapitalShare(joined) >= 0.5 {
		return nil
	}
	isSombahcemi := next == "ki" && SombahcemiKi[word]
	// Do not join into a morphologically invalid word ("İkinizin de" -> "İkinizinde", "Her neyse" -> "Herneyse").
	if c.hasMorph() && c.isRealWord(word) && !c.isRealWord(joined) && !isSombahcemi {
		return nil
	}
	also := Also.MatchString(next)
	if also {
		if IsVoicelessFinal(word) || possessiveGenEnd.MatchString(word) {
			return nil
		}
		if !c.hasMorph() && !FollowsHarmony(word, sliceRunes(next, 1, 2)) {
			return nil
		}
	}
	if (next == "te" || next == "ta") && TeTaSkip[word] {
		return nil
	}
	pair := c.lexicon.Pair(word, next)
	// "de/da" after a word is far more often the conjunction in everyday writing than a misplaced locative, so
	// joining it needs a clearer case than other joins.
	extra := 0.0
	if also {
		extra = c.Settings.DeDaJoinCost
	}
	threshold := undef
	if isSombahcemi && pair == 0 {
		threshold = math.Max(1.0, c.Settings.Threshold[KindJoin]-1.5)
	} else if next == "ki" && locativeEnd.MatchString(word) && pair <= 1 && (!c.hasMorph() || c.isRealWord(joined)) {
		threshold = math.Max(1.8, c.Settings.Threshold[KindJoin]-0.7)
	} else if (next == "te" || next == "ta") && pair == 0 && c.joinTeTa(word, joined) {
		threshold = math.Max(2.0, c.Settings.Threshold[KindJoin]-0.5)
	}
	return &candidate{text: joined, kind: KindJoin, score: lg(count) - c.Settings.Cost[KindJoin] - extra, base: lg(pair), joinsNext: true, threshold: threshold, slipCost: undef}
}

// joinTeTa is whether a word + "te/ta" join is backed: the joined word is valid, or without a morphology the word
// ends in a voiceless consonant.
func (c *Corrector) joinTeTa(word, joined string) bool {
	if c.hasMorph() {
		return c.isRealWord(joined)
	}
	return IsVoicelessFinal(word)
}

// stemCount is the count of the longest shorter prefix (4+ letters) that is itself a seen real word:
// "arkadaşlarım" for "arkadaşlarımla".
func (c *Corrector) stemCount(text string) float64 {
	rs := []rune(text)
	for end := len(rs) - 1; end >= 4; end-- {
		stem := string(rs[:end])
		count := c.lexicon.ExactCount(stem)
		if count > 0 && c.isRealWord(stem) {
			return count
		}
	}
	return 0
}

// effectiveCount is an empirical-Bayes effective frequency combining the observed corpus count with the
// morphological generative prior C(lemma, pos) * P(morphemes | pos), so unseen or rare valid inflections
// ("düzenlemiyor", "kullandığımda") rank accurately without inflating bizarre stacked derivations. mode is
// "autofix" or "rank".
func (c *Corrector) effectiveCount(cand, mode string) float64 {
	obs := c.lexicon.ExactCount(cand)
	prior := 0.0
	if c.prior != nil {
		prior = c.prior.PriorCount(cand)
	}
	if prior <= 0 {
		return obs
	}
	if mode == "rank" {
		priorEst := 0.65 * jsPow(prior, 0.78)
		return math.Max(obs, priorEst) + float64(math.Min(obs, priorEst)*0.25)
	}
	// Cap the unseen-form prior in auto-fix so a corpus-absent inflection ("gerektin") cannot pretend to have count
	// 3,000, while strong unseen forms ("düzenlemiyor", "kullandığımda") still pass cleanly.
	rawPrior := 0.5 * jsPow(prior, 0.82)
	cappedPrior := math.Min(120, rawPrior)
	if obs == 0 {
		cappedPrior = math.Min(45, rawPrior)
	}
	return math.Max(obs, cappedPrior)
}

// isUnambiguousAccusative reports whether every morphological reading of word is accusative ("hatalarını",
// "arabayı", "kitabını").
func (c *Corrector) isUnambiguousAccusative(word string) bool {
	analyses, ok := c.analyze(word)
	if !ok || len(analyses) == 0 {
		return false
	}
	for _, a := range analyses {
		if !slices.Contains(a.Morphemes, "Acc") {
			return false
		}
	}
	return true
}

// isPurelyPassiveVerb reports whether every finite verb reading of word is passive or intransitive -lAn
// ("düzenleniyor").
func (c *Corrector) isPurelyPassiveVerb(word string) bool {
	analyses, ok := c.analyze(word)
	if !ok || len(analyses) == 0 {
		return false
	}
	finite := 0
	for _, a := range analyses {
		if a.Pos == "Verb" || anyIn(a.Morphemes, finiteVerbMorphemes) {
			finite++
			if !slices.Contains(a.Morphemes, "Pass") && !slices.Contains(a.Morphemes, "Acquire") {
				return false
			}
		}
	}
	return finite > 0
}

// isAttestedOovInflection reports whether word is a productive, harmonized nominal inflection of a corpus-attested
// noun or loanword root missing from Zemberek's dictionary ("özgeçmişimi" = "özgeçmiş" + "imi", "tripodlar" =
// "tripod" + "lar"). It never holds for roots Zemberek knows, so real Turkish root typos are never shielded.
func (c *Corrector) isAttestedOovInflection(word string) bool {
	rs := []rune(word)
	if !c.hasMorph() || len(rs) < 6 {
		return false
	}
	for cut := len(rs) - 1; cut >= 4; cut-- {
		root, suffix := string(rs[:cut]), string(rs[cut:])
		if !oovNominalSuffix.MatchString(suffix) {
			continue
		}
		if c.lexicon.ExactCount(root) < 12 || c.lexicon.CapitalShare(root) >= 0.5 {
			continue
		}
		if c.isRealWord(root) {
			continue
		}
		// Check consonant assimilation and vowel harmony on the suffix boundary.
		if strings.HasPrefix(suffix, "d") && IsVoicelessFinal(root) {
			continue
		}
		if strings.HasPrefix(suffix, "t") && !IsVoicelessFinal(root) {
			continue
		}
		firstSuffixVowel := ""
		for _, ch := range suffix {
			if isVowel(ch) {
				firstSuffixVowel = string(ch)
				break
			}
		}
		if firstSuffixVowel != "" && !FollowsHarmony(root, firstSuffixVowel) {
			continue
		}
		// Also check that the suffix's own vowels harmonize with each other (except the invariant -ki).
		checkCore := suffix
		if strings.HasSuffix(suffix, "ki") {
			checkCore = suffix[:len(suffix)-2]
		}
		prevV := root
		harmOK := true
		for _, ch := range checkCore {
			if isVowel(ch) {
				if !FollowsHarmony(prevV, string(ch)) {
					harmOK = false
					break
				}
				prevV = string(ch)
			}
		}
		if harmOK {
			return true
		}
	}
	return false
}

// detectFlag is the SpellFlag of an untouched token that should be flagged, or nil.
func (c *Corrector) detectFlag(token Token, context *WordContext, nextTyped string) *SpellFlag {
	if d := c.detectFlagDetailed(token, context, nextTyped); d != nil {
		return &d.flag
	}
	return nil
}

// detectedFlag is a flag with the context model's metadata on its suggestions.
type detectedFlag struct {
	flag       SpellFlag
	meta       []TypoCandidateMeta
	coreScores []float64
}

// rankedSuggestion is a suggestion for a flagged word and how it ranked.
type rankedSuggestion struct {
	text          string
	display       string
	kind          CandidateKind
	editCost      float64
	obsCount      float64
	effCount      float64
	morphValid    bool
	rankScore     float64
	isMarkVariant bool
}

// flagOf builds a detectedFlag from the top suggestions.
func flagOf(token Token, typed string, ranked []rankedSuggestion, reason FlagReason) *detectedFlag {
	selected := ranked[:min(5, len(ranked))]
	suggestions := make([]string, len(selected))
	meta := []TypoCandidateMeta{{}}
	coreScores := make([]float64, len(selected))
	for i, s := range selected {
		suggestions[i] = s.display
		meta = append(meta, TypoCandidateMeta{CoreMargin: s.rankScore, SlipCost: s.editCost, IsMarkVariant: s.isMarkVariant})
		coreScores[i] = s.rankScore
	}
	return &detectedFlag{
		flag:       SpellFlag{Start: token.Start, End: token.End, Original: typed, Suggestions: suggestions, Reason: reason},
		meta:       meta,
		coreScores: coreScores,
	}
}

// detectFlagDetailed is the misspelling detector for words check did not fix. It tells "invalid" (rejected by the
// morphology and unseen) from "rare" (rejected, very rare, and close to a much stronger valid Turkish word), and
// protects proper names, acronyms, foreign and scientific words, apostrophe forms, contractions, and productive
// inflections of attested OOV nouns.
func (c *Corrector) detectFlagDetailed(token Token, context *WordContext, nextTyped string) *detectedFlag {
	typed := token.Text
	if tr.IsAllUpper(typed) || runeLen(typed) < 2 {
		return nil
	}
	word := tr.Lower(typed)
	if strings.Contains(word, "'") {
		return nil
	}
	// Mixed or internal uppercase ("MacOS", "OpenAI", "HAProxy", "iPhone"): a brand or code, not a spelling flag.
	if sliceFrom(typed, 1) != sliceFrom(word, 1) {
		return nil
	}
	capitalized := tr.IsUpperInitial(typed)
	// Mid-sentence capitalized words are proper names.
	if capitalized && !context.AtSentenceStart {
		return nil
	}
	// Respect contractions: when contractions are off, known contractions stay untouched and unflagged.
	if _, keep := c.knownFix(typed, word, context, nextTyped); keep {
		return nil
	}

	hasMorph := c.hasMorph()
	if hasMorph && c.isRealWord(word) {
		return nil
	}

	typedCount := c.lexicon.Count(word)
	// Common words in the corpus (modern loanwords, proper names, terms missing from Zemberek) are not flagged.
	if typedCount >= 50 {
		return nil
	}
	capShare := c.lexicon.CapitalShare(word)
	// Capitalized sentence-start words mostly capitalized in the corpus are proper names. Lowercase words whose
	// corpus occurrences are capitalized proper names ("beek" vs "Beek", "sebz" vs "Sebz") have a very low lowercase
	// count and ARE checked as rare typos.
	if capitalized && typedCount >= 2 && capShare >= 0.45 {
		return nil
	}
	typedLowerCount := jsRound(typedCount * (1 - capShare))
	// Without a morphology, only completely unseen lowercase words that have strong lexicon suggestions are flagged.
	if !hasMorph && (typedCount > 0 || capitalized) {
		return nil
	}

	ranked := c.rankedSuggestionsFor(typed, word, context, typedCount, nextTyped)
	var top *rankedSuggestion
	if len(ranked) > 0 {
		top = &ranked[0]
	}

	// Protect productive harmonized inflections of attested OOV nouns ("özgeçmişimi", "tripodlar",
	// "stabilitesinden") unless a very close high-frequency valid Turkish word competes.
	if typedCount == 0 && hasMorph && c.isAttestedOovInflection(word) {
		strongComp := top != nil && top.morphValid && top.editCost <= 0.85 && top.effCount >= 50
		if !strongComp {
			return nil
		}
	}

	if typedCount == 0 {
		if capitalized {
			// Sentence-start capitalized words (Wikipedia, subtitles) are very often proper names ("Maniwaki",
			// "Ruluka"). Flag them only when there is a plausible valid Turkish candidate ("Hoxam" -> "Hocam", "Tikım"
			// -> "Takım", "Maalledek" -> "Mahalledeki"); capShare up to 0.8 is allowed for morph-valid forms of
			// address ("Hocam").
			if top == nil {
				return nil
			}
			plausibleAtStart := top.kind == KindSplit || top.kind == KindApostrophe ||
				(top.morphValid && c.lexicon.CapitalShare(top.text) < 0.8 &&
					((top.editCost <= 1.2 && top.effCount >= 1) || (top.editCost <= 1.75 && top.effCount >= 5) || (top.editCost <= 1.95 && top.effCount >= 50)))
			if !plausibleAtStart {
				return nil
			}
		} else {
			// A lowercase unseen word rejected by the morphology: suppress foreign, scientific or Latin terms that
			// have no close Turkish candidate ("scapiflora", "neurophyllus", "psychoanalytischer", "milicenton",
			// "dârayî"). A word with Turkish marks ("klasöwe" -> "klasöre") or within 1 edit of an attested Turkish
			// word is kept even when a typo hits q/w/x.
			hasCloseTurkish := top != nil && (top.kind == KindSplit || top.kind == KindMarks ||
				(top.morphValid && ((top.editCost <= 1.75 && (top.effCount >= 0.5 || top.obsCount >= 1)) ||
					(top.editCost <= 1.9 && top.obsCount >= 15 && tr.HasTurkishMarks(word)))))
			if !hasCloseTurkish {
				if foreignLetters.MatchString(word) || foreignTaxonomicSuffix.MatchString(word) {
					return nil
				}
				if len(ranked) == 0 {
					return nil
				}
			}
		}
		return flagOf(token, typed, ranked, FlagInvalid)
	}

	// A rare but seen (0 < typedCount < 50) morph-rejected word is flagged "rare" only when very close to a much
	// stronger valid Turkish word ("bka" -> "bak", "aşam" -> "akşam", "gedi" -> "geldi", "beek" -> "bebek", "sebz" ->
	// "sebze", "Afedersiniz" -> "Affedersiniz").
	if top == nil {
		return nil
	}
	if capitalized {
		// Sentence-start capitalized words already seen in the lexicon ("Yaghi", "Şeyhhasan", "Kilittaşı") are
		// usually proper names or compounds; flag them only when a massive 1-edit Turkish word dominates
		// ("Afedersiniz").
		hugeAtStart := top.morphValid && c.lexicon.CapitalShare(top.text) < 0.5 && top.editCost <= 0.85 && top.effCount >= math.Max(500, typedCount*200)
		if !hugeAtStart {
			return nil
		}
	} else {
		if c.lexicon.CapitalShare(top.text) >= 0.5 {
			return nil
		}
		effectiveTyped := math.Max(typedLowerCount, 1)
		isStrongSplit := top.kind == KindSplit && top.effCount >= math.Max(80, effectiveTyped*20)
		minEff := 80.0
		if typedCount <= 3 {
			minEff = 45
		}
		isCloseTypo := slices.ContainsFunc(ranked[:min(3, len(ranked))], func(cand rankedSuggestion) bool {
			return cand.morphValid && c.lexicon.CapitalShare(cand.text) < 0.5 &&
				((cand.editCost <= 1.25 && cand.effCount >= math.Max(minEff, effectiveTyped*18)) ||
					(typedCount <= 3 && cand.editCost <= 1.65 && cand.effCount >= 200))
		})
		if !isStrongSplit && !isCloseTypo {
			return nil
		}
	}

	// Protect attested phrase tokens in bigrams ("van der", "de la") from rare flagging.
	nextLower := ""
	if nextTyped != "" && !tr.IsAllUpper(nextTyped) {
		nextLower = tr.Lower(nextTyped)
	}
	if typedCount >= 5 && runeLen(word) <= 3 {
		if context.PrevWord != "" && c.lexicon.Pair(context.PrevWord, word) > 0 {
			return nil
		}
		if nextLower != "" && c.lexicon.Pair(word, nextLower) > 0 {
			return nil
		}
	}
	return flagOf(token, typed, ranked, FlagRare)
}

// poolItem is a suggestion under consideration in rankedSuggestionsFor.
type poolItem struct {
	text        string
	kind        CandidateKind
	editCost    float64
	foldedEdits int
	morphValid  bool
	obsCount    float64
	effCount    float64
}

// rankedSuggestionsFor builds and ranks the suggestions for a suspicious word from:
//  1. the rule candidates (marks, splits, apostrophes, short typos, 1-edit lexicon typos, double vowel drops),
//  2. a fast 1-edit mark-folded fuzzy morphological search (unseen valid forms + mark loss + 1 typo),
//  3. a 2-edit one when the 1-edit search yields no strong candidate (double omissions, two typos),
//  4. a score from the empirical-Bayes morphological prior, the Turkish keyboard edit cost, bigram support, and
//     Turkish accusative-passive case-voice agreement.
func (c *Corrector) rankedSuggestionsFor(typed, word string, context *WordContext, typedCount float64, nextTyped string) []rankedSuggestion {
	foldedWord := foldTr(word)
	nextLower := ""
	if nextTyped != "" && !tr.IsAllUpper(nextTyped) {
		nextLower = tr.Lower(nextTyped)
	}
	prevIsAcc := false
	if context.PrevWord != "" {
		prevIsAcc = c.isUnambiguousAccusative(context.PrevWord)
	}

	pool := newOrderedMap[*poolItem]()
	// consider offers a suggestion; knownMorphValid is -1 when unknown (undefined), else 0 or 1.
	consider := func(candText string, kind CandidateKind, editCost float64, knownMorphValid int, foldedEdits int) {
		if candText == "" || candText == word {
			return
		}
		isMulti := strings.Contains(candText, " ")
		if !isMulti {
			if !vowelRE2.MatchString(candText) {
				return
			}
			if typed == word && c.lexicon.CapitalShare(candText) >= 0.65 {
				return
			}
		}
		if existing, ok := pool.get(candText); ok && existing.editCost <= editCost {
			if foldedEdits < existing.foldedEdits {
				existing.foldedEdits = foldedEdits
			}
			return
		}

		var morphValid bool
		switch {
		case knownMorphValid >= 0:
			morphValid = knownMorphValid == 1
		case isMulti:
			morphValid = true
			for _, p := range strings.Split(candText, " ") {
				if !(c.isRealWord(p) || Question.MatchString(p) || Also.MatchString(p) || p == "ki") {
					morphValid = false
					break
				}
			}
		default:
			morphValid = c.hasMorph() && c.isRealWord(candText)
		}
		var obsCount, effCount float64
		if isMulti {
			parts := strings.Split(candText, " ")
			p1, p2 := parts[0], parts[1]
			pairC := c.lexicon.Pair(p1, p2)
			minC := math.Min(c.lexicon.ExactCount(p1), c.lexicon.ExactCount(p2))
			obsCount = math.Max(pairC, minC)
			effCount = math.Max(pairC, math.Min(c.effectiveCount(p1, "rank"), math.Max(c.lexicon.Count(p2), 50)))
		} else {
			obsCount = c.lexicon.ExactCount(candText)
			effCount = obsCount
			if morphValid {
				effCount = c.effectiveCount(candText, "rank")
			}
		}
		// Require either corpus attestation or morphological validity with a non-trivial generative prior.
		if obsCount < 1 && (!morphValid || (c.prior != nil && effCount < 0.03)) {
			return
		}
		pool.set(candText, &poolItem{text: candText, kind: kind, editCost: editCost, foldedEdits: foldedEdits, morphValid: morphValid, obsCount: obsCount, effCount: effCount})
	}

	// 1. The rule and lexicon candidates (marks, splits, apostrophes, short 3-letter typos, 1-edit lexicon typos).
	for _, cand := range c.candidates(word, typed, context, typedCount, nextTyped) {
		if cand.joinsNext {
			continue
		}
		var ec float64
		fe := 1
		switch cand.kind {
		case KindSplit:
			ec = 0.85
		case KindApostrophe:
			ec = 0.75
		default:
			ec = morph.TurkishEditCost(word, cand.text)
			if foldTr(cand.text) == foldedWord {
				fe = 0
			}
		}
		consider(cand.text, cand.kind, ec, -1, fe)
	}

	wr := []rune(word)
	// 2. Short 3-letter words: swaps, insertions and near-key substitutions to valid Turkish words ("bka" -> "bak").
	if len(wr) == 3 && c.hasMorph() {
		for _, swap := range []string{string([]rune{wr[1], wr[0], wr[2]}), string([]rune{wr[0], wr[2], wr[1]})} {
			if c.isRealWord(swap) {
				consider(swap, KindTypo, morph.TurkishEditCost(word, swap), 1, 1)
			}
		}
		for i := 0; i <= 3; i++ {
			before, after := string(wr[:i]), string(wr[i:])
			for _, ch := range tr.Alphabet {
				ins := before + string(ch) + after
				if c.lexicon.ExactCount(ins) >= 20 && c.isRealWord(ins) {
					consider(ins, KindTypo, morph.TurkishEditCost(word, ins), 1, 1)
				}
			}
		}
		for i := 0; i < 3; i++ {
			at := wr[i]
			for _, ch := range tr.Alphabet {
				if ch != at && (tr.AreNearKeys(ch, at) || (isVowel(ch) && isVowel(at))) {
					sub := string(wr[:i]) + string(ch) + string(wr[i+1:])
					if c.lexicon.ExactCount(sub) >= 20 && c.isRealWord(sub) {
						consider(sub, KindTypo, morph.TurkishEditCost(word, sub), 1, 1)
					}
				}
			}
		}
	}

	// 3. A fast 1-edit mark-folded morphological search (unseen valid forms + mark loss + 1 typo).
	if c.fuzzy != nil && len(wr) >= 4 && len(wr) <= 28 {
		for _, f := range c.fuzzy.FuzzySearch(word, 1, 30) {
			kind := KindTypo
			if f.FoldedEdits == 0 {
				kind = KindMarks
			}
			consider(f.Word, kind, f.EditCost, 1, f.FoldedEdits)
		}

		// 4. Escalate to a 2-edit mark-folded morphological search when the 1-edit one doesn't produce a strong
		// high-confidence candidate (or leaves a single candidate on medium words), catching double omissions like
		// "grkli" -> "gerekli", "gerektn" -> "gerçekten", "frmuar" -> "fermuarı", "palıcanlar" -> "patlıcanları",
		// "mahalemize" -> "mahallemizde".
		veryStrongD1, cleanTranspositionD1 := 0, 0
		for _, key := range pool.keys {
			item := pool.vals[key]
			if item.morphValid && item.foldedEdits <= 1 {
				if item.editCost <= 1.1 && item.effCount >= 120 {
					veryStrongD1++
				}
				if item.editCost <= 0.75 && item.effCount >= 1 {
					cleanTranspositionD1++
				}
			}
		}
		if (veryStrongD1 == 0 || (pool.size() < 2 && cleanTranspositionD1 == 0)) && len(wr) >= 4 && len(wr) <= 18 {
			for _, f := range c.fuzzy.FuzzySearch(word, 2, 40) {
				consider(f.Word, KindTypo, f.EditCost, 1, f.FoldedEdits)
			}
		}
	}

	// 5. Direct 1-edit lexicon matches for words with a low count too (non-Zemberek words like "caspius").
	if len(wr) >= 4 && pool.size() < 3 {
		edits := c.oneEdit(word, true)
		for _, cand := range edits.keys {
			slip := edits.vals[cand]
			if c.lexicon.ExactCount(cand) >= 5 {
				consider(cand, KindTypo, math.Min(slip, morph.TurkishEditCost(word, cand)), -1, 1)
			}
		}
	}

	scored := make([]rankedSuggestion, 0, pool.size())
	for _, key := range pool.keys {
		item := pool.vals[key]
		isMulti := strings.Contains(item.text, " ")
		pairBonus := 0.0
		if !isMulti {
			if context.PrevWord != "" && c.lexicon.Pair(context.PrevWord, item.text) > 0 {
				pairBonus += 1.2
			}
			if nextLower != "" && c.lexicon.Pair(item.text, nextLower) > 0 {
				pairBonus += 1.0
			}
		}
		morphPenalty := 0.0
		if !item.morphValid {
			morphPenalty = -1.8
		}
		// Universal Turkish grammar: an unambiguously accusative direct object ("hatalarını") does not govern a
		// purely passive or intransitive verb ("düzenleniyor"), favoring active transitive readings ("düzenlemiyor").
		voicePenalty := 0.0
		if prevIsAcc && !isMulti && item.morphValid && c.isPurelyPassiveVerb(item.text) {
			voicePenalty = -1.6
		}
		// Bayes channel penalty: 1 keyboard edit is far likelier than 2, so 1-edit transpositions and omissions
		// ("göndredim" -> "gönderdim", "kalelmerimi" -> "kalemlerimi") stay first over common 2-edit words while
		// genuine double omissions ("gerektn" -> "gerçekten") still surface.
		extraEditPenalty := 0.0
		if item.foldedEdits >= 2 {
			extraEditPenalty = 1.8
		}
		rankScore := lg(item.effCount) - float64(3.6*item.editCost) - extraEditPenalty + pairBonus + morphPenalty + voicePenalty
		display := keepCase(typed, item.text)
		if item.kind == KindTypo {
			display = tr.MatchCase(typed, item.text)
		}
		scored = append(scored, rankedSuggestion{
			text:          item.text,
			display:       display,
			kind:          item.kind,
			editCost:      item.editCost,
			obsCount:      item.obsCount,
			effCount:      item.effCount,
			morphValid:    item.morphValid,
			rankScore:     rankScore,
			isMarkVariant: !isMulti && foldTr(item.text) == foldedWord,
		})
	}

	slices.SortStableFunc(scored, func(a, b rankedSuggestion) int {
		d := b.rankScore - a.rankScore
		if d == 0 || math.IsNaN(d) {
			d = a.editCost - b.editCost
		}
		return jsCompare(d)
	})

	// Deduplicate by display string, keeping the highest-ranked entry.
	seenDisplay := map[string]bool{}
	deduped := scored[:0:0]
	for _, s := range scored {
		if seenDisplay[s.display] {
			continue
		}
		seenDisplay[s.display] = true
		deduped = append(deduped, s)
	}
	return deduped
}

// oneEdit are the words one edit away, with a cost for edits that are unlikely on a Turkish Q keyboard.
func (c *Corrector) oneEdit(word string, unseenLowerInvalid bool) *orderedMap[float64] {
	edits := newOrderedMap[float64]()
	rs := []rune(word)
	n := len(rs)
	endCost := c.Settings.TypoEndCost
	if unseenLowerInvalid {
		endCost = math.Min(c.Settings.TypoEndCost, 0.5)
	}
	put := func(text string, slip float64, position int, finalDelPenalty float64) {
		// Changing the end of a word often swaps one real suffix for another ("öyküsüz" is not "öyküsü",
		// "benzeyene" is not "benzeyen"), so edits there cost more.
		atEnd := 0.0
		if float64(position) >= float64(n)-c.Settings.TypoEndWindow {
			atEnd = endCost
		}
		// Without a morphology (or for capitalized or seen words), block edits on the last letter of 6+ letter
		// words. With a morphology, on unseen lowercase invalid words, valid inflections are already excluded via
		// !realTyped.
		if !unseenLowerInvalid && n >= 6 && position >= n-1 {
			return
		}
		atStart := 0.0
		if position == 0 {
			atStart = 1.5
		}
		total := slip + atEnd + atStart + finalDelPenalty
		if text == "" {
			return
		}
		if old, ok := edits.get(text); !ok || old > total {
			edits.set(text, total)
		}
	}
	for i := 0; i <= n; i++ {
		before := string(rs[:i])
		hasAt := i < n
		var at rune
		after := ""
		if hasAt {
			at = rs[i]
			after = string(rs[i+1:])
		}
		hasPrev := i > 0
		var previous rune
		if hasPrev {
			previous = rs[i-1]
		}
		if hasAt {
			hasFollowing := i+1 < n
			var following rune
			if hasFollowing {
				following = rs[i+1]
			}
			// An extra key is almost always a duplicate or keyboard neighbor of an adjacent key ("fat-finger").
			nearExtra := (hasPrev && (at == previous || tr.AreNearKeys(at, previous))) ||
				(hasFollowing && (at == following || tr.AreNearKeys(at, following)))
			isFinalNonDupDelete := i == n-1 && !(hasPrev && at == previous)
			// A Turkish letter is typed on purpose; in a word Turkish rules reject it is not a stray key
			// ("çalşıyorum" is a missed "ı" in "çalışıyorum", not an extra "ş" in "çalıyorum").
			deliberate := unseenLowerInvalid && strings.ContainsRune("çğışöü", at) && !(hasPrev && at == previous) && !(hasFollowing && at == following)
			// Deleting a non-duplicate final letter (especially a distant key like "m" after "r" in "geliyorm") risks
			// chopping a real person suffix ("geliyorm" -> "geliyor" instead of "geliyorum").
			finalDelPen := 0.0
			if isFinalNonDupDelete && n >= 6 {
				finalDelPen = 2.5
				if nearExtra {
					finalDelPen = 1.4
				}
			}
			farExtra := 2.3
			if unseenLowerInvalid {
				farExtra = c.Settings.TypoFarExtra
			}
			slip := farExtra
			if deliberate {
				slip = math.Max(farExtra, 5)
			} else if nearExtra {
				slip = 0.9
			}
			put(before+after, slip, i, finalDelPen) // an extra key
			if hasFollowing {
				put(before+string(following)+string(at)+string(rs[i+2:]), 0.7, i, 0) // two keys swapped
			}
		}
		betweenConsonants := hasPrev && hasAt && !isVowel(previous) && !isVowel(at)
		rest := string(rs[i:])
		for _, letter := range tr.Alphabet {
			if hasAt && letter != at {
				switch {
				case tr.AreNearKeys(letter, at):
					put(before+string(letter)+after, 0.8, i, 0)
				case voicingPairs[string(at)+string(letter)]:
					put(before+string(letter)+after, 1.8, i, 0)
				case unseenLowerInvalid && c.Settings.TypoVowelSwap > 0 && isVowel(at) && isVowel(letter):
					put(before+string(letter)+after, c.Settings.TypoVowelSwap, i, 0)
				}
			}
			// Omitting a vowel inside a consonant cluster ("geliyorm", "yapabilirm", "gönderdm") is very common in
			// Turkish.
			insertSlip := 1.0
			if betweenConsonants && isVowel(letter) {
				insertSlip = 0.6
			}
			put(before+string(letter)+rest, insertSlip, i, 0)
		}
	}
	edits.del(word)
	return edits
}

// contestedSpan is a place where the typed span and another real-word reading compete, for the context model.
type contestedSpan struct {
	start, end    int
	typed         string
	alternatives  []string
	kind          ContextKind
	direction     string // "marks", "split", "join" or "typo"
	asciiContext  bool
	typedHasMarks bool
}

// pendingOption is a decision left to the context model.
type pendingOption struct {
	contestedSpan
	// surfaceOriginal is the text now at [start, end) (it differs from typed only for PriorTypoOriginals).
	surfaceOriginal string
	// typoMode is "override", "ambiguous" or "rescue" for a typo, else "".
	typoMode        string
	coreReplacement string
	typoMeta        []TypoCandidateMeta
}

// typoCandidate is a morphologically valid typo or mark candidate of an invalid or rare word.
type typoCandidate struct {
	text          string
	kind          CandidateKind
	margin        float64
	threshold     float64
	slipCost      float64
	isMarkVariant bool
}

// typoCandidatesFor are the morphologically valid typo and mark candidates of an invalid or rare word, sorted by
// core score, with their core margin, threshold, slip cost and whether they only restore marks. The context
// model's pass uses them.
func (c *Corrector) typoCandidatesFor(typed string, context *WordContext, nextTyped string, maxCandidates int) []typoCandidate {
	if !c.hasMorph() || tr.IsAllUpper(typed) || runeLen(typed) < 4 {
		return nil
	}
	word := tr.Lower(typed)
	if strings.Contains(word, "'") || sliceFrom(typed, 1) != sliceFrom(word, 1) {
		return nil
	}
	if tr.IsUpperInitial(typed) && !context.AtSentenceStart {
		return nil
	}
	if c.isRealWord(word) {
		return nil
	}
	if listed, keep := c.knownFix(typed, word, context, nextTyped); listed != nil || keep {
		return nil
	}
	typedCount := c.lexicon.Count(word)
	rareInvalidSlip := typed == word && typedCount < 50 && c.lexicon.CapitalShare(word) < 0.5
	if typedCount >= c.Settings.KnownWord && !rareInvalidSlip {
		return nil
	}
	cands := c.candidates(word, typed, context, typedCount, nextTyped)
	foldedWord := foldTr(word)
	var valid []typoCandidate
	for _, cand := range cands {
		if cand.kind != KindTypo && cand.kind != KindMarks {
			continue
		}
		if strings.Contains(cand.text, " ") || !c.isRealWord(cand.text) || c.lexicon.ExactCount(cand.text) < 1 {
			continue
		}
		invalidTyped := cand.kind == KindTypo && typedCount == 0 && typed == word
		threshold := cand.threshold
		if !defined(threshold) {
			if invalidTyped {
				threshold = c.Settings.TypoInvalidThreshold
			} else {
				threshold = c.Settings.Threshold[cand.kind]
			}
		}
		valid = append(valid, typoCandidate{
			text:          cand.text,
			kind:          cand.kind,
			margin:        marginOf(cand),
			threshold:     threshold,
			slipCost:      or(cand.slipCost, 0),
			isMarkVariant: foldTr(cand.text) == foldedWord,
		})
	}
	slices.SortStableFunc(valid, func(a, b typoCandidate) int { return jsCompare(b.margin - a.margin) })
	return valid[:min(maxCandidates, len(valid))]
}

// splitTail is typed with a space before its last two letters ("bende" -> "ben de").
func splitTail(typed string) string { return sliceRunes(typed, 0, -2) + " " + sliceFrom(typed, -2) }

var questionTail = regexp.MustCompile(`^m[ıiuü]$`)

// contestedSpans are the places where the typed span and another real-word reading compete, for the context model
// to judge.
func (c *Corrector) contestedSpans(token Token, next *Token, context *WordContext) []contestedSpan {
	typed := token.Text
	word := tr.Lower(typed)
	if tr.IsAllUpper(typed) || runeLen(typed) < 2 || (tr.IsUpperInitial(typed) && !context.AtSentenceStart) || sliceFrom(typed, 1) != sliceFrom(word, 1) {
		return nil
	}
	known := func(form string) bool { return c.lexicon.Count(form) >= c.Settings.ContextWord }
	asciiContext := !context.TextHasMarks || context.SentenceHasASCIITurkish
	typedHasMarks := tr.HasTurkishMarks(word)
	nextLow := ""
	if next != nil {
		nextLow = tr.Lower(next.Text)
	}
	var spans []contestedSpan
	span := func(end int, typed string, alternatives []string, kind ContextKind, direction string, ascii bool) {
		spans = append(spans, contestedSpan{start: token.Start, end: end, typed: typed, alternatives: alternatives, kind: kind, direction: direction, asciiContext: ascii, typedHasMarks: typedHasMarks})
	}

	// "oldu" / "öldü", "uç" / "üç": real words that differ only in Turkish marks (question particle harmony is
	// deterministic).
	if known(word) && !strings.Contains(word, "'") && !Question.MatchString(word) {
		var variants []string
		for _, v := range tr.MarkVariants(word, 512) {
			if known(v) {
				variants = append(variants, v)
			}
		}
		slices.SortStableFunc(variants, func(a, b string) int { return jsCompare(c.lexicon.Count(b) - c.lexicon.Count(a)) })
		variants = variants[:min(3, len(variants))]
		for i, v := range variants {
			variants[i] = keepCase(typed, v)
		}
		if len(variants) > 0 {
			span(token.End, typed, variants, ContextMarks, "marks", asciiContext)
		}
	}

	// "bende" / "ben de", "evde" / "ev de", "evdeki" / "evde ki", "sevgimi" / "sevgi mi": a real word that could be
	// two. Never split the TDK-fixed SOMBAHCEMİ words ("oysaki", "illaki"), never split a locative -te/-ta into the
	// conjunction de/da ("ekte" is never "ek de"), and never split Xde before another "de/da" ("bende de").
	if known(word) && runeLen(word) >= 4 && !strings.Contains(word, "'") {
		stem, tail := sliceRunes(word, 0, -2), sliceFrom(word, -2)
		switch {
		case (tail == "de" || tail == "da") && nextLow != "de" && nextLow != "da" && known(stem) && FollowsHarmony(stem, sliceRunes(tail, 1, 2)):
			span(token.End, typed, []string{splitTail(typed)}, ContextDeDa, "split", false)
		case tail == "ki" && !fixedKiWords[word] && !SombahcemiKi[stem] && known(stem):
			span(token.End, typed, []string{splitTail(typed)}, ContextKi, "split", false)
		case questionTail.MatchString(tail) && runeLen(stem) >= 3 && known(stem) && FollowsHarmony(stem, sliceRunes(tail, 1, 2)):
			span(token.End, typed, []string{splitTail(typed)}, ContextMi, "split", false)
		}
	}

	// "ben de" / "bende", "kitap da" / "kitapta", "evde ki" / "evdeki", "sevgi mi" / "sevgimi": two words that could
	// be one.
	if next != nil && !tr.IsUpperInitial(next.Text) && !tr.IsAllUpper(next.Text) && !strings.Contains(word, "'") && known(word) {
		second := nextLow
		both := typed + " " + next.Text
		switch {
		case second == "de" || second == "da":
			gluedD := word + second
			gluedT := ""
			if IsVoicelessFinal(word) {
				if second == "de" {
					gluedT = word + "te"
				} else {
					gluedT = word + "ta"
				}
			}
			glued := ""
			if known(gluedD) {
				glued = gluedD
			} else if gluedT != "" && known(gluedT) {
				glued = gluedT
			}
			if glued != "" {
				span(next.End, both, []string{keepCase(typed, glued)}, ContextDeDa, "join", false)
			}
		case second == "ki" && known(word+"ki"):
			span(next.End, both, []string{typed + next.Text}, ContextKi, "join", false)
		case Question.MatchString(second) && !MusicNotes[word] && known(word+second):
			span(next.End, both, []string{typed + next.Text}, ContextMi, "join", false)
		}
	}
	return spans
}

// priorAwareLexicon is the lexicon as the flag re-ranking shows it to the context model: an unseen or rare valid
// candidate ("düzenlemiyor") counts with the morphology's generative prior, so corpus sparsity doesn't bias the
// count features.
type priorAwareLexicon struct{ c *Corrector }

func (p priorAwareLexicon) Count(form string) float64 {
	raw := p.c.lexicon.Count(form)
	if raw >= 20 || strings.Contains(form, "'") || !p.c.isRealWord(form) {
		return raw
	}
	return math.Max(raw, jsRound(p.c.effectiveCount(form, "rank")))
}

func (p priorAwareLexicon) Pair(first, second string) float64 { return p.c.lexicon.Pair(first, second) }

// overlapsSpan reports whether [aStart, aEnd) and [bStart, bEnd) meet.
func overlapsSpan(aStart, aEnd, bStart, bEnd int) bool { return aStart < bEnd && bStart < aEnd }

// contextSlice is the text around [spanStart, spanEnd) with the decided corrections written in, so the context
// model sees clean neighbors in ASCII or many-typo sentences.
func contextSlice(rs []rune, decided []Correction, spanStart, spanEnd int) (left, right string) {
	leftFrom := max(0, spanStart-180)
	rightTo := min(len(rs), spanEnd+150)
	if len(decided) == 0 {
		return runeSlice(rs, leftFrom, spanStart), runeSlice(rs, spanEnd, rightTo)
	}
	var l, r strings.Builder
	cur := leftFrom
	for _, d := range decided {
		if d.End <= leftFrom {
			continue
		}
		if d.Start >= spanStart {
			break
		}
		if d.Start < cur || d.End > spanStart {
			continue
		}
		l.WriteString(runeSlice(rs, cur, d.Start) + d.Replacement)
		cur = d.End
	}
	l.WriteString(runeSlice(rs, cur, spanStart))
	cur = spanEnd
	for _, d := range decided {
		if d.End <= spanEnd {
			continue
		}
		if d.Start >= rightTo {
			break
		}
		if d.Start < cur || d.End > rightTo {
			continue
		}
		r.WriteString(runeSlice(rs, cur, d.Start) + d.Replacement)
		cur = d.End
	}
	r.WriteString(runeSlice(rs, cur, rightTo))
	return l.String(), r.String()
}

// CorrectWithModel is Correct with the context model judging contested real-word readings ("bende" / "ben de",
// "oldu" / "öldü", "sevgimi" / "sevgi mi") and ambiguous or contested typo candidates ("yllar" -> "yıllar" or
// "yollar", "hber" -> "her" or "haber") in sentence context. With a nil model it is Correct.
func (c *Corrector) CorrectWithModel(text string, model ContextModel, options CorrectOptions) CorrectResult {
	r := c.runModelPass(text, model, options, false)
	r.Flags = nil
	return r
}

// InspectWithModel is Inspect with the context model, which also re-ranks the flags' suggestions in sentence
// context. With a nil model it is Inspect.
func (c *Corrector) InspectWithModel(text string, model ContextModel, options CorrectOptions) CorrectResult {
	return c.runModelPass(text, model, options, true)
}

// alternativeIndex is the index of the alternative that is replacement, ignoring case, or -1.
func alternativeIndex(alternatives []string, replacement string) int {
	low := tr.Lower(replacement)
	return slices.IndexFunc(alternatives, func(alt string) bool { return tr.Lower(alt) == low })
}

// runModelPass is runAsyncPass: the core pass with the context model's decisions. Without a model, or when the
// model fails, it is the core pass.
func (c *Corrector) runModelPass(text string, model ContextModel, options CorrectOptions, collectFlags bool) CorrectResult {
	if model == nil {
		return c.runCorePass(text, options, collectFlags)
	}
	rs := []rune(text)
	tokens := Tokenize(text)
	evidence := tr.StripMarksIn(text, options.AutoMarked)
	textHasMarks := tr.HasTurkishMarks(evidence)
	marks := markPrefix([]rune(evidence))
	asciiTyped, sentenceOf := c.asciiEvidence(Tokenize(evidence))
	priorTypoByStart := map[int]PriorTypo{}
	for _, item := range options.PriorTypoOriginals {
		priorTypoByStart[item.Start] = item
	}
	var decided []Correction
	var pending []pendingOption
	st := passState{atSentenceStart: true}
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !token.IsWord {
			st.nonWord(token)
			continue
		}
		context, next := c.wordContext(tokens, index, &st, textHasMarks, marks, asciiTyped, sentenceOf)
		st.atSentenceStart = false
		if !c.isIsolated(tokens, index) {
			st.forget()
			continue
		}
		var nextToken *Token
		nextText := ""
		if next != nil {
			nextToken = &next.token
			nextText = next.token.Text
		}
		for _, option := range c.contestedSpans(token, nextToken, &context) {
			pending = append(pending, pendingOption{contestedSpan: option, surfaceOriginal: option.typed})
		}
		fix := c.check(token.Text, &context, nextText)
		asciiContext := !context.TextHasMarks || context.SentenceHasASCIITurkish
		alternativesOf := func(original string, cands []typoCandidate) ([]string, []TypoCandidateMeta) {
			alts := make([]string, len(cands))
			meta := []TypoCandidateMeta{{}}
			for i, cand := range cands {
				if cand.kind == KindTypo {
					alts[i] = tr.MatchCase(original, cand.text)
				} else {
					alts[i] = keepCase(original, cand.text)
				}
				meta = append(meta, TypoCandidateMeta{CoreMargin: cand.margin, SlipCost: cand.slipCost, IsMarkVariant: cand.isMarkVariant})
			}
			return alts, meta
		}

		// Re-evaluate a word rewritten as a typo earlier, during live typing (the pause pass's override).
		if priorTypo, ok := priorTypoByStart[token.Start]; ok && priorTypo.End == token.End && fix == nil {
			typoCands := c.typoCandidatesFor(priorTypo.Original, &context, nextText, 7)
			if len(typoCands) >= 2 {
				alts, meta := alternativesOf(priorTypo.Original, typoCands)
				if alternativeIndex(alts, token.Text) >= 0 {
					pending = append(pending, pendingOption{
						contestedSpan: contestedSpan{start: token.Start, end: token.End, typed: priorTypo.Original, alternatives: alts, kind: ContextTypo,
							direction: "typo", asciiContext: asciiContext, typedHasMarks: tr.HasTurkishMarks(priorTypo.Original)},
						surfaceOriginal: token.Text, typoMode: "override", coreReplacement: token.Text, typoMeta: meta,
					})
				}
			}
		}

		// Contested or rescued typo candidates for invalid or rare words.
		if typoCands := c.typoCandidatesFor(token.Text, &context, nextText, 7); len(typoCands) > 0 {
			word := tr.Lower(token.Text)
			typedCount := c.lexicon.Count(word)
			alts, meta := alternativesOf(token.Text, typoCands)
			option := pendingOption{
				contestedSpan: contestedSpan{start: token.Start, end: token.End, typed: token.Text, alternatives: alts, kind: ContextTypo,
					direction: "typo", asciiContext: asciiContext, typedHasMarks: tr.HasTurkishMarks(word)},
				surfaceOriginal: token.Text, typoMeta: meta,
			}
			if fix != nil && !fix.joinsNext && len(typoCands) >= 2 && fix.kind == KindTypo && alternativeIndex(alts, fix.replacement) >= 0 {
				option.typoMode, option.coreReplacement = "override", fix.replacement
				pending = append(pending, option)
			} else if fix == nil {
				top := typoCands[0]
				if len(typoCands) >= 2 && (top.margin >= top.threshold || (typedCount == 0 && top.margin >= 0)) {
					option.typoMode = "ambiguous"
					pending = append(pending, option)
				} else if typedCount == 0 && token.Text == word && top.margin >= -0.5 {
					option.typoMode = "rescue"
					pending = append(pending, option)
				}
			}
		}

		if fix == nil {
			st.untouched(token, &context)
			continue
		}
		end := token.End
		if fix.joinsNext && next != nil {
			end = next.token.End
		}
		decided = append(decided, Correction{Start: token.Start, End: end, Original: string(rs[token.Start:end]), Replacement: fix.replacement, Kind: fix.kind, Margin: fix.margin})
		st.fixed(fix, &context)
		if fix.joinsNext && next != nil {
			index = next.index
		}
	}

	queries := make([]ContextQuery, len(pending))
	for i, option := range pending {
		candidates := append([]string{option.typed}, option.alternatives...)
		left, right := contextSlice(rs, decided, option.start, option.end)
		queries[i] = ContextQuery{Left: left, Right: right, Candidates: candidates,
			Features: c.buildContextFeatures(candidates, option.kind, option.asciiContext, c.lexicon, option.typoMeta, nil)}
	}
	scores, err := model.Score(queries)
	if err != nil || len(scores) != len(queries) {
		return c.runCorePass(text, options, collectFlags)
	}

	typoOverrideBar := or(c.Settings.ContextTypoOverride, 2.5)
	typoAmbigGap := or(c.Settings.ContextTypoAmbigGap, 1.5)
	typoAmbigVsTyped := or(c.Settings.ContextTypoAmbigVsTyped, 2.5)
	typoRescueBar := or(c.Settings.ContextTypoRescue, 4.5)

	// 1. Keep the core fixes, allowing (a) the context veto on count-based joins and splits, and (b) a
	// high-confidence switch between competing valid typo candidates ("hber": core "her" -> context "haber").
	var kept []Correction
	for _, fix := range decided {
		if fix.Kind == KindJoin || fix.Kind == KindSplit {
			index := slices.IndexFunc(pending, func(o pendingOption) bool {
				return o.kind != ContextTypo && o.start == fix.Start && o.end == fix.End && alternativeIndex(o.alternatives, fix.Replacement) >= 0
			})
			if index != -1 {
				row := scores[index]
				alternative := alternativeIndex(pending[index].alternatives, fix.Replacement)
				if row[alternative+1]-row[0] <= -c.Settings.ContextVeto {
					continue
				}
			}
			kept = append(kept, fix)
			continue
		}
		overrideIdx := slices.IndexFunc(pending, func(o pendingOption) bool {
			return o.kind == ContextTypo && o.typoMode == "override" && o.start == fix.Start && o.end == fix.End
		})
		if overrideIdx != -1 {
			option := pending[overrideIdx]
			row := scores[overrideIdx]
			if coreAltIdx := alternativeIndex(option.alternatives, fix.Replacement); coreAltIdx != -1 {
				bestAlt := coreAltIdx
				for i := range option.alternatives {
					if row[i+1] > row[bestAlt+1] {
						bestAlt = i
					}
				}
				delta := row[bestAlt+1] - row[coreAltIdx+1]
				if bestAlt != coreAltIdx && delta >= typoOverrideBar {
					kept = append(kept, Correction{Start: fix.Start, End: fix.End, Original: fix.Original, Replacement: option.alternatives[bestAlt], Kind: KindContext, Margin: delta})
					continue
				}
			}
		}
		kept = append(kept, fix)
	}

	// 2. Where no core fix touches a span (or for PriorTypoOriginals in the pause pass), apply confident context
	// decisions.
	offsets := c.Settings.ContextOffsets
	thresholdFor := func(option pendingOption) float64 {
		base := c.Settings.Threshold[KindContext]
		if offsets == nil {
			return base
		}
		split := option.direction == "split"
		pick := func(ifSplit, ifJoin float64) float64 {
			if split {
				return base + or(ifSplit, 0)
			}
			return base + or(ifJoin, 0)
		}
		switch option.kind {
		case ContextMarks:
			if option.asciiContext {
				return base + or(offsets.MarksASCII, 0)
			}
			if option.typedHasMarks {
				return base + or(offsets.MarksMarkedOnMarked, 3.0)
			}
			return base + or(offsets.MarksMarked, 1.5)
		case ContextDeDa:
			return pick(offsets.DeDaSplit, offsets.DeDaJoin)
		case ContextKi:
			return pick(offsets.KiSplit, offsets.KiJoin)
		}
		return pick(offsets.MiSplit, offsets.MiJoin)
	}
	var judged []Correction
	for index, option := range pending {
		if slices.ContainsFunc(decided, func(fix Correction) bool { return overlapsSpan(fix.Start, fix.End, option.start, option.end) }) {
			continue
		}
		row := scores[index]
		if option.kind == ContextTypo {
			if option.typoMode == "override" && option.coreReplacement != "" {
				// The pause pass's override for a PriorTypoOriginals span not touched in this pass.
				coreAltIdx := alternativeIndex(option.alternatives, option.coreReplacement)
				if coreAltIdx == -1 {
					continue
				}
				bestAlt := coreAltIdx
				for i := range option.alternatives {
					if row[i+1] > row[bestAlt+1] {
						bestAlt = i
					}
				}
				delta := row[bestAlt+1] - row[coreAltIdx+1]
				if bestAlt != coreAltIdx && delta >= typoOverrideBar {
					judged = append(judged, Correction{Start: option.start, End: option.end, Original: option.surfaceOriginal, Replacement: option.alternatives[bestAlt], Kind: KindContext, Margin: delta})
				}
				continue
			}
			best, second := 1, -1
			for i := 2; i < len(row); i++ {
				if row[i] > row[best] {
					second = best
					best = i
				} else if second == -1 || row[i] > row[second] {
					second = i
				}
			}
			vsTyped := row[best] - row[0]
			gap := math.Inf(1)
			if second != -1 {
				gap = row[best] - row[second]
			}
			switch option.typoMode {
			case "ambiguous":
				if vsTyped >= typoAmbigVsTyped && gap >= typoAmbigGap {
					judged = append(judged, Correction{Start: option.start, End: option.end, Original: option.surfaceOriginal, Replacement: option.alternatives[best-1], Kind: KindContext, Margin: math.Min(vsTyped, gap)})
				}
			case "rescue":
				if vsTyped >= typoRescueBar && gap >= typoAmbigGap {
					judged = append(judged, Correction{Start: option.start, End: option.end, Original: option.surfaceOriginal, Replacement: option.alternatives[best-1], Kind: KindContext, Margin: vsTyped})
				}
			}
			continue
		}
		best := 1
		for i := 2; i < len(row); i++ {
			if row[i] > row[best] {
				best = i
			}
		}
		if margin := row[best] - row[0]; margin >= thresholdFor(option) {
			judged = append(judged, Correction{Start: option.start, End: option.end, Original: option.surfaceOriginal, Replacement: option.alternatives[best-1], Kind: KindContext, Margin: margin})
		}
	}
	all := append(kept, judged...)
	slices.SortStableFunc(all, func(a, b Correction) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return jsCompare(b.Margin - a.Margin)
	})
	corrections := []Correction{}
	for _, fix := range all {
		if !slices.ContainsFunc(corrections, func(done Correction) bool { return overlapsSpan(done.Start, done.End, fix.Start, fix.End) }) {
			corrections = append(corrections, fix)
		}
	}
	if c.Settings.SentenceCase {
		corrections = c.sentenceRulesWithModel(text, corrections, model, textHasMarks)
	}
	finalText := applyToRunes(rs, corrections)
	if !collectFlags {
		return CorrectResult{Text: finalText, Corrections: corrections, Flags: []SpellFlag{}}
	}
	flags := c.modelFlags(rs, tokens, corrections, model, textHasMarks, marks, asciiTyped, sentenceOf)
	return CorrectResult{Text: finalText, Corrections: corrections, Flags: flags}
}

// sentenceRulesWithModel runs the sentence rules, letting the context model decide ambiguous proper names
// ("Deniz" / "deniz") when it can.
func (c *Corrector) sentenceRulesWithModel(text string, corrections []Correction, model ContextModel, textHasMarks bool) []Correction {
	if !model.SupportsNameContext() || c.known == nil || !c.known.HasProperData() {
		return c.withSentenceRules(text, corrections, nil, nil)
	}
	collectAmbig := []AmbigCandidateQuery{}
	pass1 := c.withSentenceRules(text, corrections, &collectAmbig, nil)
	if len(collectAmbig) == 0 {
		return pass1
	}
	asciiCtx := !textHasMarks
	queries := make([]ContextQuery, len(collectAmbig))
	for i, q := range collectAmbig {
		candidates := []string{q.CommonCand, q.ProperCand}
		stemCapShare := c.lexicon.CapitalShare(q.Base)
		aposShare := 0.0
		if q.Suffix != "" {
			cApos := c.lexicon.Count(q.Base + "'" + q.Suffix)
			cJoin := c.lexicon.Count(q.Base + q.Suffix)
			aposShare = cApos / (cApos + cJoin + 1)
		}
		meta := []NameCandidateMeta{
			{IsProper: false, StemCapShare: stemCapShare, AposShare: aposShare, IsGiven: q.IsGiven, AtSentenceStart: q.AtStart},
			{IsProper: true, StemCapShare: stemCapShare, AposShare: aposShare, IsGiven: q.IsGiven, AtSentenceStart: q.AtStart},
		}
		queries[i] = ContextQuery{Left: q.LeftRaw, Right: q.RightRaw, Candidates: candidates,
			Features: c.buildContextFeatures(candidates, ContextName, asciiCtx, c.lexicon, nil, meta)}
	}
	nameScores, err := model.Score(queries)
	if err != nil {
		return pass1
	}
	preDecidedAmbig := map[int]string{}
	nameBar := or(c.Settings.ContextNameThreshold, 2.5)
	for i, q := range collectAmbig {
		if i >= len(nameScores) || len(nameScores[i]) < 2 {
			continue
		}
		row := nameScores[i]
		margin := row[1] - row[0]
		if q.HadApostrophe {
			if margin >= 1.5 {
				preDecidedAmbig[q.Start] = q.ProperCand
			} else if -margin >= 2.0 {
				preDecidedAmbig[q.Start] = q.CommonCand
			}
			continue
		}
		// At a sentence start, an overwhelmingly common inflected lowercase Turkish noun ("Savaşa gitme", where
		// "savaşa" is frequent with a capital share of 5% or less) needs a stronger margin to split with an
		// apostrophe.
		effBar := nameBar
		if q.AtStart && q.Suffix != "" {
			wholeLow := q.Base + q.Suffix
			if c.lexicon.Count(wholeLow) >= 1000 && c.lexicon.CapitalShare(wholeLow) <= 0.05 {
				effBar = nameBar + 1.5
			}
		}
		if margin >= effBar {
			preDecidedAmbig[q.Start] = q.ProperCand
		}
	}
	if len(preDecidedAmbig) > 0 {
		return c.withSentenceRules(text, corrections, nil, preDecidedAmbig)
	}
	return pass1
}

// modelFlags flags every suspicious token left untouched by both the core and the context corrections, and
// re-ranks their suggestions in sentence context with one call to the model.
func (c *Corrector) modelFlags(rs []rune, tokens []Token, corrections []Correction, model ContextModel, textHasMarks bool, marks, asciiTyped, sentenceOf []int) []SpellFlag {
	type rawFlag struct {
		detectedFlag
		asciiContext bool
	}
	var rawFlags []rawFlag
	st := passState{atSentenceStart: true}
	corrIdx := 0
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !token.IsWord {
			st.nonWord(token)
			continue
		}
		for corrIdx < len(corrections) && corrections[corrIdx].End <= token.Start {
			corrIdx++
		}
		var overlapping *Correction
		if corrIdx < len(corrections) && overlapsSpan(corrections[corrIdx].Start, corrections[corrIdx].End, token.Start, token.End) {
			overlapping = &corrections[corrIdx]
		}
		ctx, next := c.wordContext(tokens, index, &st, textHasMarks, marks, asciiTyped, sentenceOf)
		st.atSentenceStart = false
		if overlapping != nil {
			st.fixed(&checkResult{replacement: overlapping.Replacement}, &ctx)
			continue
		}
		if !c.isIsolated(tokens, index) {
			st.forget()
			continue
		}
		nextText := ""
		if next != nil {
			nextText = next.token.Text
		}
		if detected := c.detectFlagDetailed(token, &ctx, nextText); detected != nil {
			rawFlags = append(rawFlags, rawFlag{detectedFlag: *detected, asciiContext: !ctx.TextHasMarks || ctx.SentenceHasASCIITurkish})
		}
		st.untouched(token, &ctx)
	}

	// Re-rank multi-suggestion flags in sentence context with one batched call to the model.
	var targets []int
	for i, entry := range rawFlags {
		if len(entry.flag.Suggestions) >= 2 {
			targets = append(targets, i)
		}
	}
	if len(targets) > 0 {
		lexicon := priorAwareLexicon{c}
		queries := make([]ContextQuery, len(targets))
		for qi, i := range targets {
			entry := rawFlags[i]
			candidates := append([]string{entry.flag.Original}, entry.flag.Suggestions...)
			left, right := contextSlice(rs, corrections, entry.flag.Start, entry.flag.End)
			queries[qi] = ContextQuery{Left: left, Right: right, Candidates: candidates,
				Features: c.buildContextFeatures(candidates, ContextTypo, entry.asciiContext, lexicon, entry.meta, nil)}
		}
		if flagScores, err := model.Score(queries); err == nil {
			for qi, i := range targets {
				entry := &rawFlags[i]
				if qi >= len(flagScores) || len(flagScores[qi]) != len(entry.flag.Suggestions)+1 {
					continue
				}
				row := flagScores[qi]
				type scoredSuggestion struct {
					sugg  string
					score float64
				}
				ranked := make([]scoredSuggestion, len(entry.flag.Suggestions))
				for idx, sugg := range entry.flag.Suggestions {
					core := 0.0
					if idx < len(entry.coreScores) {
						core = entry.coreScores[idx]
					}
					// Combine the sentence context score with the core linguistic and edit-cost score.
					ranked[idx] = scoredSuggestion{sugg, row[idx+1] + float64(0.65*core)}
				}
				slices.SortStableFunc(ranked, func(a, b scoredSuggestion) int { return jsCompare(b.score - a.score) })
				for idx, r := range ranked {
					entry.flag.Suggestions[idx] = r.sugg
				}
			}
		}
	}
	flags := make([]SpellFlag, len(rawFlags))
	for i, entry := range rawFlags {
		flags[i] = entry.flag
	}
	return flags
}
