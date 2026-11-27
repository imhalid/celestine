package spell

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/imhalid/celestine/internal/spell/morph"
	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's pkg/src/engine.ts and the types of pkg/src/types.ts: the package's engine, which wraps
// the corrector in the API an editor uses. The Worker, the ONNX session and the async queue of the TypeScript
// are left out: the Go API is synchronous.

// Mode is how careful the engine is: "careful" or "balanced", a little bolder.
type Mode string

// The modes.
const (
	ModeCareful  Mode = "careful"
	ModeBalanced Mode = "balanced"
)

// The kinds of fix the package tells apart (FixKind).
const (
	// KindTurkishLetters is Turkish letters typed without their marks ("cok" -> "çok").
	KindTurkishLetters = "turkce-karakter"
	// KindApostropheFix is an apostrophe after a proper name ("Ankaraya" -> "Ankara'ya").
	KindApostropheFix = "kesme"
	// KindJoinedSeparate is words written joined or apart ("hiç bir" -> "hiçbir", "birşey" -> "bir şey").
	KindJoinedSeparate = "bitisik-ayri"
	// KindSpelling is a misspelling ("gelyorum" -> "geliyorum").
	KindSpelling = "yazim"
	// KindPunctuation is sentence punctuation and capitals ("merhaba.ben" -> "Merhaba. Ben").
	KindPunctuation = "noktalama"
	// KindContractionFix is a spoken-style contraction spelled out ("noldu" -> "ne oldu"), only when asked for.
	KindContractionFix = "kisaltma"
)

// AllFixKinds are all the kinds of fix.
var AllFixKinds = []string{KindTurkishLetters, KindApostropheFix, KindJoinedSeparate, KindSpelling, KindPunctuation, KindContractionFix}

// DefaultFixKinds are the kinds applied when CheckOptions.Kinds is nil: all but contractions, which are a matter of
// style.
var DefaultFixKinds = []string{KindTurkishLetters, KindApostropheFix, KindJoinedSeparate, KindSpelling, KindPunctuation}

// The scopes of a fix: what it was decided on.
const (
	// ScopeWord is a fix decided on the word alone ("cok" -> "çok", "herkez" -> "herkes", "Ankaraya" ->
	// "Ankara'ya").
	ScopeWord = "kelime"
	// ScopeContext is a fix decided on the words around it too ("hiç bir" -> "hiçbir", "yerler de" -> "yerlerde",
	// "geldinmi?" -> "geldin mi?").
	ScopeContext = "baglam"
)

// Fix is one suggested change to the text. From and To are offsets in runes (the TypeScript's UTF-16 units, which
// are the same for Turkish text).
type Fix struct {
	From, To    int
	Original    string
	Replacement string
	// Kind is one of the Kind* fix kinds ("turkce-karakter", "kesme", ...).
	Kind string
	// Scope is ScopeWord or ScopeContext.
	Scope string
	// Confidence is 0..1, from the corrector's margin.
	Confidence float64
	// ByContext is true when the context model made the decision (only with a model loaded).
	ByContext bool
}

// Flag is a suspicious word left as typed, with up to 5 ranked suggestions.
type Flag struct {
	From, To    int
	Original    string
	Suggestions []string
	// Reason is "invalid" (the morphology rejects it) or "rare" (very rare and very close to a valid word).
	Reason string
}

// InspectResult is what Inspect finds: the fixes and the flagged words.
type InspectResult struct {
	Fixes []Fix
	Flags []Flag
}

// CheckOptions are the options of Check, Fix and Inspect.
type CheckOptions struct {
	// Range, when set, keeps only the fixes that meet [from, to) (runes); the rest of the text is read as context.
	Range *[2]int
	// Kinds, when not nil, keeps only fixes of these kinds; an empty, non-nil Kinds keeps none.
	Kinds []string
	// Mode is the mode for this call only; "" is the engine's.
	Mode Mode
	// AutoMarked are [from, to) ranges autocorrect wrote earlier: their Turkish letters are no evidence that the
	// writer types with a Turkish keyboard, so after "hic" was fixed to "hiç" in "hic bir sey", "sey" is fixed too.
	AutoMarked [][2]int
	// PriorTypoOriginals, with a context model only, are the spans autocorrect changed as typos and what was typed
	// there: the model may pick a better candidate from the whole sentence ("hber" -> "her" -> "haber").
	PriorTypoOriginals []PriorTypo
	// NoContext runs the core pass alone, the context model left out though it is loaded: what a check must answer
	// at once, as a word ends while it is typed, does not wait for the network (Celestine's own, #623).
	NoContext bool
}

// BALANCED are the balanced settings: a little bolder than CAREFUL.
var BALANCED = func() Settings {
	s := CAREFUL
	s.Threshold = KindThresholds{KindMarks: 1.8, KindApostrophe: 2.0, KindSplit: 2.0, KindJoin: 2.0, KindTypo: 2.3, KindKnown: 0.7, KindContext: 2.5, KindContraction: 0.7, KindPunct: 1}
	s.Cost = KindThresholds{KindMarks: 0.4, KindApostrophe: 0.5, KindSplit: 0.4, KindJoin: 0.4, KindTypo: 1.7, KindKnown: 0, KindContext: 0, KindContraction: 0, KindPunct: 0}
	s.Ambiguity = 0.7
	s.KnownWord = 5
	s.KnownMarked = 15
	s.MarkedOverride = 4.5
	s.MinName = 12
	return s
}()

// settingsByMode are the settings of each mode.
var settingsByMode = map[Mode]Settings{ModeCareful: CAREFUL, ModeBalanced: BALANCED}

// settingsFor are a mode's settings for the kinds asked for (nil for all): contractions only when "kisaltma" is
// asked for, sentence capitals and spaces unless "noktalama" is left out.
func settingsFor(mode Mode, kinds map[string]bool) Settings {
	s := settingsByMode[mode]
	s.Contractions = kinds[KindContractionFix]
	s.SentenceCase = kinds == nil || kinds[KindPunctuation]
	return s
}

var (
	fencedCode = regexp.MustCompile("```(?s:.)*?(?:```|$)")
	inlineCode = regexp.MustCompile("(?m)`[^`\n]*(?:`|$)")
)

// findCodeRanges are the Markdown code spans and blocks of text, which are left alone.
func findCodeRanges(text string) [][2]int {
	if !strings.Contains(text, "`") {
		return nil
	}
	var ranges [][2]int
	for _, pattern := range []*regexp.Regexp{fencedCode, inlineCode} {
		for _, m := range findAllRunes(pattern, text) {
			ranges = append(ranges, [2]int{m[0], m[1]})
		}
	}
	return ranges
}

// correctionScope is a correction's scope: joins, splits, context decisions and punctuation look at the words
// around, as does every fix that spans two words ("hiç bir" -> "hiçbir"); the rest look at the word alone.
func correctionScope(c Correction) string {
	if c.Kind == KindJoin || c.Kind == KindSplit || c.Kind == KindContext || c.Kind == KindPunct {
		return ScopeContext
	}
	if strings.Contains(c.Original, " ") {
		return ScopeContext
	}
	return ScopeWord
}

// jsSpaceRun is /\s+/g.
var jsSpaceRun = regexp.MustCompile(`[` + jsSpaceClass + `]+`)

// mapCorrectionKind is the package's kind of an internal correction kind.
func mapCorrectionKind(c Correction) string {
	switch c.Kind {
	case KindMarks:
		return KindTurkishLetters
	case KindApostrophe:
		return KindApostropheFix
	case KindSplit, KindJoin:
		return KindJoinedSeparate
	case KindTypo:
		return KindSpelling
	case KindContraction:
		return KindContractionFix
	case KindPunct:
		return KindPunctuation
	}
	// context and known.
	if strings.Contains(c.Original, " ") != strings.Contains(c.Replacement, " ") {
		return KindJoinedSeparate
	}
	strip := func(value string) string { return jsSpaceRun.ReplaceAllString(tr.Lower(value), "") }
	if strip(c.Original) == strip(c.Replacement) {
		return KindJoinedSeparate
	}
	fold := func(value string) string { return foldTr(tr.Lower(value)) }
	if fold(c.Original) == fold(c.Replacement) {
		return KindTurkishLetters
	}
	return KindSpelling
}

// marginToConfidence turns the corrector's natural-log margin into a confidence between 0 and 1.
func marginToConfidence(margin float64) float64 {
	if !defined(margin) || margin > 1e308 || margin < -1e308 {
		if margin > 0 {
			return 1
		}
		return 0
	}
	raw := 1 / (1 + jsExp(-margin/2))
	return min(1, max(0, jsRound(raw*1000)/1000))
}

// normalizeIgnoredWord is how an ignored word is kept: trimmed and lowercased the Turkish way.
func normalizeIgnoredWord(word string) string { return tr.Lower(jsTrim(word)) }

// Apply writes the fixes into text. Fixes that overlap, or whose Original is not what the text holds at their
// place, are an error, and text is not changed.
func Apply(text string, fixes []Fix) (string, error) {
	if len(fixes) == 0 {
		return text, nil
	}
	ordered := slices.Clone(fixes)
	slices.SortStableFunc(ordered, func(a, b Fix) int { return a.From - b.From })
	rs := []rune(text)
	var output strings.Builder
	cursor := 0
	for _, fix := range ordered {
		if fix.From < 0 || fix.To < fix.From || fix.To > len(rs) {
			return text, fmt.Errorf("spell: a fix's range [%d, %d) is not in the text", fix.From, fix.To)
		}
		if fix.From < cursor {
			return text, fmt.Errorf("spell: fixes overlap (%d < %d)", fix.From, cursor)
		}
		if actual := string(rs[fix.From:fix.To]); actual != fix.Original {
			return text, fmt.Errorf("spell: %q was expected at %d, the text has %q", fix.Original, fix.From, actual)
		}
		output.WriteString(string(rs[cursor:fix.From]))
		output.WriteString(fix.Replacement)
		cursor = fix.To
	}
	output.WriteString(string(rs[cursor:]))
	return output.String(), nil
}

// Engine is the spell checker as an editor uses it: the corrector with a mode, the words to leave alone, and
// (later) the context model. Its methods are safe for concurrent use; they take turns, as the corrector's
// settings and the morphology's caches are shared, and the answers depend on the order of the calls as in the
// TypeScript.
type Engine struct {
	mu        sync.Mutex
	corrector *Corrector
	ignored   map[string]bool
	mode      Mode
	model     ContextModel
}

// Load reads lexicon.bin, morph.bin and known.tsv from dir and builds an engine in mode ("" is careful), with its
// context model on when context.onnx is there too.
func Load(dir string, mode Mode) (*Engine, error) {
	lexBytes, err := os.ReadFile(filepath.Join(dir, "lexicon.bin"))
	if err != nil {
		return nil, err
	}
	lexicon, err := LexiconFromBinary(lexBytes)
	if err != nil {
		return nil, err
	}
	morphBytes, err := os.ReadFile(filepath.Join(dir, "morph.bin"))
	if err != nil {
		return nil, err
	}
	morphology, err := morph.FromBinary(morphBytes)
	if err != nil {
		return nil, err
	}
	knownText, err := os.ReadFile(filepath.Join(dir, "known.tsv"))
	if err != nil {
		return nil, err
	}
	e := NewEngine(lexicon, KnownMisspellingsFromTSV(string(knownText)), morphology, mode)
	// the context model, when it is there: the engine's context on, as the npm package's context: true
	if b, err := os.ReadFile(filepath.Join(dir, "context.onnx")); err == nil {
		model, err := LoadContextModel(b)
		if err != nil {
			return nil, err
		}
		e.SetContextModel(model)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return e, nil
}

// NewEngine is an engine of parts already loaded; known and morphology may be nil, and mode "" is careful.
func NewEngine(lexicon *Lexicon, known *KnownMisspellings, morphology WordValidator, mode Mode) *Engine {
	if mode == "" {
		mode = ModeCareful
	}
	return &Engine{
		corrector: NewCorrector(lexicon, settingsByMode[mode], known, morphology),
		ignored:   map[string]bool{},
		mode:      mode,
	}
}

// HasContext reports whether a context model is loaded.
func (e *Engine) HasContext() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.model != nil
}

// SetContextModel loads a context model, or with nil unloads it.
func (e *Engine) SetContextModel(model ContextModel) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.model = model
}

// SetMode changes the mode.
func (e *Engine) SetMode(mode Mode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mode = mode
	e.corrector.Settings = settingsByMode[mode]
}

// Ignore leaves this word (or phrase) alone from now on.
func (e *Engine) Ignore(word string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if normalized := normalizeIgnoredWord(word); normalized != "" {
		e.ignored[normalized] = true
	}
}

// Unignore checks a word left alone with Ignore again.
func (e *Engine) Unignore(word string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if normalized := normalizeIgnoredWord(word); normalized != "" {
		delete(e.ignored, normalized)
	}
}

// overlapsAny reports whether [start, end) meets one of the ranges.
func overlapsAny(ranges [][2]int, start, end int) bool {
	return slices.ContainsFunc(ranges, func(r [2]int) bool { return start < r[1] && end > r[0] })
}

func (e *Engine) filterCorrections(text string, corrections []Correction, rng *[2]int, kindFilter map[string]bool) []Fix {
	fixes := []Fix{}
	if len(corrections) == 0 {
		return fixes
	}
	codeRanges := findCodeRanges(text)
	lastEnd := -1
	for _, c := range corrections {
		if c.Original == c.Replacement || c.Start < lastEnd {
			continue
		}
		if rng != nil && !(c.Start < rng[1] && c.End > rng[0]) {
			continue
		}
		if len(codeRanges) > 0 && overlapsAny(codeRanges, c.Start, c.End) {
			continue
		}
		if e.ignored[normalizeIgnoredWord(c.Original)] {
			continue
		}
		kind := mapCorrectionKind(c)
		if kindFilter != nil && !kindFilter[kind] {
			continue
		}
		fixes = append(fixes, Fix{
			From:        c.Start,
			To:          c.End,
			Original:    c.Original,
			Replacement: c.Replacement,
			Kind:        kind,
			Confidence:  marginToConfidence(c.Margin),
			Scope:       correctionScope(c),
			ByContext:   c.Kind == KindContext,
		})
		lastEnd = c.End
	}
	return fixes
}

func (e *Engine) filterFlags(text string, rawFlags []SpellFlag, rawCorrections []Correction, rng *[2]int, kindFilter map[string]bool) []Flag {
	flags := []Flag{}
	if len(rawFlags) == 0 || (kindFilter != nil && !kindFilter[KindSpelling]) {
		return flags
	}
	codeRanges := findCodeRanges(text)
	lastEnd := -1
	for _, flag := range rawFlags {
		if flag.Start < lastEnd {
			continue
		}
		if rng != nil && !(flag.Start < rng[1] && flag.End > rng[0]) {
			continue
		}
		if len(codeRanges) > 0 && overlapsAny(codeRanges, flag.Start, flag.End) {
			continue
		}
		if e.ignored[normalizeIgnoredWord(flag.Original)] {
			continue
		}
		if slices.ContainsFunc(rawCorrections, func(c Correction) bool {
			return c.Original != c.Replacement && flag.Start < c.End && flag.End > c.Start
		}) {
			continue
		}
		flags = append(flags, Flag{
			From:        flag.Start,
			To:          flag.End,
			Original:    flag.Original,
			Suggestions: slices.Clone(flag.Suggestions[:min(5, len(flag.Suggestions))]),
			Reason:      string(flag.Reason),
		})
		lastEnd = flag.End
	}
	return flags
}

// unlockedModel wraps a ContextModel so Engine.mu is released while Score runs ONNX inference, allowing fast
// NoContext checks and Ignore calls to proceed without blocking on the neural network.
type unlockedModel struct {
	mu *sync.Mutex
	m  ContextModel
}

func (u unlockedModel) Score(queries []ContextQuery) ([][]float64, error) {
	u.mu.Unlock()
	defer u.mu.Lock()
	return u.m.Score(queries)
}

func (u unlockedModel) SupportsNameContext() bool { return u.m.SupportsNameContext() }

// run is a pass of the corrector over text with the call's settings; ok is false when there is nothing to check.
func (e *Engine) run(text string, opts CheckOptions, collectFlags bool) (result CorrectResult, kindFilter map[string]bool, ok bool) {
	if text == "" {
		return result, nil, false
	}
	if opts.Range != nil && opts.Range[0] >= opts.Range[1] {
		return result, nil, false
	}
	if opts.Kinds != nil {
		kindFilter = make(map[string]bool, len(opts.Kinds))
		for _, k := range opts.Kinds {
			kindFilter[k] = true
		}
		if len(kindFilter) == 0 {
			return result, nil, false
		}
	}
	mode := opts.Mode
	if mode == "" {
		mode = e.mode
	}
	corr := *e.corrector
	corr.Settings = settingsFor(mode, kindFilter)
	options := CorrectOptions{AutoMarked: opts.AutoMarked}
	if e.model != nil && !opts.NoContext {
		options.PriorTypoOriginals = opts.PriorTypoOriginals
		result = corr.runModelPass(text, unlockedModel{mu: &e.mu, m: e.model}, options, collectFlags)
	} else {
		result = corr.runCorePass(text, options, collectFlags)
	}
	return result, kindFilter, true
}

// Check is the fixes for text, in order and not overlapping.
func (e *Engine) Check(text string, opts CheckOptions) []Fix {
	e.mu.Lock()
	defer e.mu.Unlock()
	result, kindFilter, ok := e.run(text, opts, false)
	if !ok {
		return []Fix{}
	}
	return e.filterCorrections(text, result.Corrections, opts.Range, kindFilter)
}

// Inspect is the fixes for text and the words it leaves as typed but flags as likely misspelled.
func (e *Engine) Inspect(text string, opts CheckOptions) InspectResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	result, kindFilter, ok := e.run(text, opts, true)
	if !ok {
		return InspectResult{Fixes: []Fix{}, Flags: []Flag{}}
	}
	return InspectResult{
		Fixes: e.filterCorrections(text, result.Corrections, opts.Range, kindFilter),
		Flags: e.filterFlags(text, result.Flags, result.Corrections, opts.Range, kindFilter),
	}
}

// Fix is text with Check's fixes applied.
func (e *Engine) Fix(text string, opts CheckOptions) string {
	fixed, err := Apply(text, e.Check(text, opts))
	if err != nil { // Check's fixes always fit the text they were made for.
		return text
	}
	return fixed
}
