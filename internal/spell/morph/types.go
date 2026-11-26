// Package morph is the Turkish morphology of tinyfix-tr's src/morphology/ in Go (#623): a root-driven
// finite-state analyzer that tells whether a word form is valid Turkish, its analyses, a generative prior of how
// often it is seen, and an error-tolerant search for the valid forms near a typo. It is adapted from Zemberek-NLP
// (Apache 2.0, see NOTICE) through the TypeScript, and answers what the TypeScript answers: strings are indexed
// in runes where the TypeScript indexes UTF-16 units, and every order the TypeScript's maps and sorts keep is kept.
package morph

// This file is types.ts: the shared types and the root attribute bit masks.

// PrimaryPos is a part of speech: "Noun", "Adj", "Verb", "Pron", "Adv", "Conj", "Ques", "Postp", "Det", "Num",
// "Dup", "Interj" or "Punc".
type PrimaryPos = string

// SecondaryPos is a finer part of speech: "None", "ProperNoun", "Abbreviation", "Time", "PersonalPron",
// "DemonstrativePron", "QuantitivePron", "QuestionPron", "ReflexivePron", "Card", "Ord", "Dist", and the
// postpositions' cases "PCDat", "PCAbl", "PCNom", "PCGen", "PCIns", "PCAcc".
type SecondaryPos = string

// The root attribute bit masks (31 bits at most, as the TypeScript's signed 32-bit bitwise operators).
const (
	raNone                 = 0
	raVoicing              = 1 << 0
	raNoVoicing            = 1 << 1
	raVoicingOpt           = 1 << 2
	raInverseHarmony       = 1 << 3
	raDoubling             = 1 << 4
	raLastVowelDrop        = 1 << 5
	raCompoundP3sg         = 1 << 6
	raCompoundP3sgRoot     = 1 << 7
	raNoSuffix             = 1 << 8
	raProgressiveVowelDrop = 1 << 9
	raAoristI              = 1 << 10
	raAoristA              = 1 << 11
	raPassiveIn            = 1 << 12
	raCausativeT           = 1 << 13
	raImplicitPlural       = 1 << 14
	raImplicitDative       = 1 << 15
	raImplicitP1sg         = 1 << 16
	raImplicitP2sg         = 1 << 17
	raFamilyMember         = 1 << 18
	raReciprocal           = 1 << 19
	raNonReciprocal        = 1 << 20
	raReflexive            = 1 << 21
	raDummy                = 1 << 22
	raNounConsInsertN      = 1 << 23
	raExt                  = 1 << 24
	raInformal             = 1 << 25
)

// dictionaryItem is a lexicon entry.
type dictionaryItem struct {
	// key is the lookup key without the homonym index, e.g. "ben_Pron_PersonalPron", "dün_Noun_Time", "değil_Verb".
	key string
	// lemma is the lowercased lemma with its circumflexes, to verify a circumflexed input.
	lemma string
	// root is the normalized root without circumflexes and, for verbs, without -mak/-mek.
	root string
	// pronunciation is the normalized pronunciation, the root when none is given.
	pronunciation string
	primaryPos    PrimaryPos
	secondaryPos  SecondaryPos
	attributes    int
	index         int
	// referenceItem is the item a Dummy compound root ("zeytinyağ" for "zeytinyağı") or a variant stands for.
	referenceItem *dictionaryItem
}

// morpheme is a morpheme of the graph; the graph's states point at the shared ones of allMorphemes.
type morpheme struct {
	id           string
	derivational bool
	pos          PrimaryPos // "" for none
}

// suffixTemplateToken is one token of a suffix template (see tokenizeTemplate).
type suffixTemplateToken struct {
	kind   int
	letter string
}

// pathNode is a step of a search path: an immutable singly linked list from the last step back to the stem.
type pathNode struct {
	prev           *pathNode
	stemTransition *stemTransition
	state          *morphemeState
	surface        string
	// pos is the offset in runes into the normalized search input.
	pos int
	// inputLength is the length of the search input, for the O(1) hasTail check.
	inputLength int
	// phonetic is the PhoneticAttribute bit mask.
	phonetic               int
	depth                  int
	hasSurface             bool
	hasDerivation          bool
	currentGroupHasSurface bool
	lastDerivation         *morphemeState
}

// condition is a transition's condition on the path so far.
type condition func(p *pathNode) bool

// surfaceCacheEntry is a suffix transition's surface for one 6-bit phonetic key, made the first time the key is
// met: until then set is false; a surface that cannot be made has null set.
type surfaceCacheEntry struct {
	set     bool
	null    bool
	surface string
	runes   []rune
	// nextPhonetic is the phonetic bit mask after the surface, computed from the phonetic attributes of the
	// path that first met the key, as the TypeScript's Int32Array keeps it.
	nextPhonetic int32
}

// surfaceCache is the 64-entry cache of a suffix transition, shared with the transitions copied from it.
type surfaceCache [surfaceCacheSize]surfaceCacheEntry

// suffixTransition is an edge of the graph that adds a suffix.
type suffixTransition struct {
	from       *morphemeState
	to         *morphemeState
	template   string
	tokens     []suffixTemplateToken
	lastToken  *suffixTemplateToken
	hasSurface bool
	condition  condition
	// forbiddenPhonetic is PA_EXPECTS_VOWEL or PA_EXPECTS_CONSONANT, the expectation that must not be present.
	forbiddenPhonetic int
	cache             *surfaceCache
}

// morphemeState is a state of the graph.
type morphemeState struct {
	id         string
	morpheme   *morpheme
	terminal   bool
	derivative bool
	posRoot    bool
	outgoing   []*suffixTransition
}

// stemTransition is a stem surface of a dictionary item and the state it enters.
type stemTransition struct {
	surface  string
	runes    []rune
	item     *dictionaryItem
	phonetic int
	to       *morphemeState
}

// MorphAnalysis is an analysis of a word form: its stem, the stem's part of speech, and its morphemes in order
// (empty Pnon and Nom left out), e.g. {kitap Noun [A3sg P1sg Acc]} for "kitabımı".
type MorphAnalysis struct {
	Stem      string   `json:"stem"`
	Pos       string   `json:"pos"`
	Morphemes []string `json:"morphemes"`
}
