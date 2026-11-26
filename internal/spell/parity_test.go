package spell

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"testing"
	"unicode/utf16"
)

// TestParityLexicon checks the lexicon, the known misspellings and the rules against the TypeScript engine's
// answers in testdata/parity-lex.jsonl.gz, which testdata/parity-lex.ts writes. It needs the engine's data
// (lexicon.bin, known.tsv) in the directory TINYFIX_DATA names, and is skipped without it.
func TestParityLexicon(t *testing.T) {
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		t.Skip("TINYFIX_DATA is not set")
	}
	bin, err := os.ReadFile(filepath.Join(dir, "lexicon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	lexicon, err := LexiconFromBinary(bin)
	if err != nil {
		t.Fatal(err)
	}
	knownText, err := os.ReadFile(filepath.Join(dir, "known.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	p := &parity{lexicon: lexicon, known: KnownMisspellingsFromTSV(string(knownText))}
	unigrams, bigrams := lexicon.ToTSV()
	p.tsvLexicon = LexiconFromTSV(unigrams, bigrams, 3, 2)

	f, err := os.Open("testdata/parity-lex.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	lines, checked, failed := 0, 0, 0
	for scanner.Scan() {
		lines++
		var line struct {
			F string `json:"f"`
			K string `json:"k"`
			A []any  `json:"a"`
			R any    `json:"r"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("line %d: %v", lines, err)
		}
		var answers map[string]any
		if line.K != "" {
			answers, _ = line.R.(map[string]any)
		} else {
			answers = map[string]any{line.F: line.R}
		}
		for name, want := range answers {
			checked++
			got, ok := p.answer(line.K, name, line.A, want)
			if !ok {
				failed++
				if failed <= 40 {
					t.Errorf("line %d: no Go probe for %s/%s", lines, line.K, name)
				}
				continue
			}
			if !reflect.DeepEqual(normalize(got), want) {
				failed++
				if failed <= 40 {
					gj, _ := json.Marshal(normalize(got))
					wj, _ := json.Marshal(want)
					t.Errorf("line %d: %s%s(%v):\n  go: %s\n  ts: %s", lines, prefix(line.K), name, line.A, clip(gj), clip(wj))
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines == 0 {
		t.Fatal("the golden file is empty")
	}
	t.Logf("%d lines, %d answers checked, %d differ", lines, checked, failed)
	if failed > 0 {
		t.Errorf("%d of %d answers differ from the TypeScript's", failed, checked)
	}
}

func prefix(kind string) string {
	if kind == "" {
		return ""
	}
	return kind + "."
}

func clip(b []byte) string {
	if len(b) > 400 {
		return string(b[:400]) + "..."
	}
	return string(b)
}

// normalize turns a Go answer into what encoding/json would read back from its JSON: the shape the golden has.
// NaN and the infinities are null, as JSON.stringify writes them.
func normalize(v any) any {
	b, err := json.Marshal(jsonSafe(v))
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

// jsonSafe replaces the floats JSON cannot hold with nil, all through v.
func jsonSafe(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = jsonSafe(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = jsonSafe(e)
		}
		return out
	}
	return v
}

// parity holds what the probes run against.
type parity struct {
	lexicon, tsvLexicon *Lexicon
	known               *KnownMisspellings
	// particleSuffixes and numberSuffixes are the suffixes the grouped probes of particles and numbers try, in the
	// golden's order, which its lines before them give.
	particleSuffixes, numberSuffixes []string
}

// str is the string argument i.
func str(args []any, i int) string { s, _ := args[i].(string); return s }

// opt is a string result that may be missing (undefined in the TypeScript).
func opt(s string, ok bool) any {
	if !ok {
		return nil
	}
	return s
}

func nameEntryJSON(e NameEntry, ok bool) any {
	if !ok {
		return nil
	}
	return map[string]any{"name": e.Name, "count": e.Count, "noApos": e.NoApos}
}

func phrasesJSON(list []PhraseEntry) any {
	out := []any{}
	for _, p := range list {
		m := map[string]any{"words": p.Words, "displayWords": p.DisplayWords, "takesApos": p.TakesApos}
		if p.Context != "" {
			m["context"] = string(p.Context)
		}
		out = append(out, m)
	}
	return out
}

func fixesJSON(fixes []KnownFix) any {
	out := []any{}
	for _, f := range fixes {
		out = append(out, map[string]any{"text": f.Text, "count": f.Count, "joinsNext": f.JoinsNext, "contraction": f.Contraction})
	}
	return out
}

func titleJSON(s TitleSplit, ok bool) any {
	if !ok {
		return nil
	}
	return map[string]any{"head": s.Head, "displayHead": s.DisplayHead, "suffix": s.Suffix, "hadApostrophe": s.HadApostrophe}
}

func headJSON(s HeadSplit, ok bool) any {
	if !ok {
		return nil
	}
	return map[string]any{"head": s.Head, "suffix": s.Suffix, "hadApostrophe": s.HadApostrophe}
}

func calendarJSON(s CalendarSplit, ok bool) any {
	if !ok {
		return nil
	}
	return map[string]any{"stem": s.Stem, "suffix": s.Suffix, "hadApostrophe": s.HadApostrophe}
}

func acronymJSON(a AcronymApostrophe, ok bool) any {
	if !ok {
		return nil
	}
	return map[string]any{"prefix": a.Prefix, "suffix": a.Suffix, "withMark": a.WithMark}
}

// execJSON is what JavaScript's RegExp.prototype.exec gives: the match and its groups, null for a group that did
// not take part, or null for no match.
func execJSON(re *regexp.Regexp, s string) any {
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return nil
	}
	out := make([]any, len(loc)/2)
	for i := range out {
		if loc[2*i] >= 0 {
			out[i] = s[loc[2*i]:loc[2*i+1]]
		}
	}
	return out
}

// parityRegexes are the regular expressions by their TypeScript names, in the golden's order.
var parityRegexes = []*regexp.Regexp{
	Question, Also, NoLocativeAfter, StrongNoLocativeAfter, Finite, NameSuffix, PlaceP3sgSuffix,
	NonApostropheSuffixRE, ProperInflectionalSuffixRE, HeadSuffixRE, NoAposProperSuffixRE,
}

var parityRegexNames = []any{
	"QUESTION", "ALSO", "NO_LOCATIVE_AFTER", "STRONG_NO_LOCATIVE_AFTER", "FINITE", "NAME_SUFFIX", "PLACE_P3SG_SUFFIX",
	"NON_APOSTROPHE_SUFFIX_RE", "PROPER_INFLECTIONAL_SUFFIX_RE", "HEAD_SUFFIX_RE", "NO_APOS_PROPER_SUFFIX_RE",
}

var parityHeadSets = []map[string]bool{InstitutionHeads, GeoHeads, StateHeads, EventHeads, OfficerTitleHeads, SayinTitles, SayinCompoundTail}

var parityHeadSetNames = []any{"INSTITUTION_HEADS", "GEO_HEADS", "STATE_HEADS", "EVENT_HEADS", "OFFICER_TITLE_HEADS", "SAYIN_TITLES", "SAYIN_COMPOUND_TAIL"}

// paritySets are the TypeScript's exported sets by their names: a map where order never matters, a WordSet where
// it can.
var paritySets = map[string]any{
	"R.NEVER_ALONE": NeverAlone, "R.JOIN_FIRST": JoinFirst, "R.JOIN_SECOND": JoinSecond, "R.SOMBAHCEMI_KI": SombahcemiKi,
	"R.TE_TA_SKIP": TeTaSkip, "R.HONORIFIC_TITLES": HonorificTitles, "R.PLACE_APOSTROPHE_HEADS": PlaceApostropheHeads,
	"R.JOINED_ETMEK_STEMS": JoinedEtmekStems, "R.SEPARATE_ETMEK_SHORT": SeparateEtmekShort,
	"R.SEPARATE_ETMEK_BARE_IMPERATIVE": SeparateEtmekBareImperative, "R.TIME_NAMES_NO_START_APOSTROPHE": TimeNamesNoStartApostrophe,
	"R.MUSIC_NOTES": MusicNotes, "R.MUSIC_AFTER": MusicAfter,
	"P.MONTHS": Months, "P.WEEKDAYS": Weekdays, "P.INSTITUTION_HEADS": InstitutionHeads, "P.OFFICER_TITLE_HEADS": OfficerTitleHeads,
	"P.OFFICER_TITLE_MODS": OfficerTitleMods, "P.GENERIC_ALONE_INST_HEADS": GenericAloneInstHeads,
	"P.GENERIC_INST_MODIFIERS": GenericInstModifiers, "P.NON_INST_PREFIX": NonInstPrefix, "P.GEO_HEADS": GeoHeads,
	"P.STATE_HEADS": StateHeads, "P.EVENT_HEADS": EventHeads, "P.DIRECTIONS": Directions, "P.PRE_TITLES_ABBR": PreTitlesAbbr,
	"P.PRE_TITLES_FULL": PreTitlesFull, "P.SAYIN_TITLES": SayinTitles, "P.SAYIN_COMPOUND_FIRST": SayinCompoundFirst,
	"P.SAYIN_COMPOUND_TAIL": SayinCompoundTail, "P.POST_HONORIFICS": PostHonorifics, "P.POST_TITLES_WITH_APOS": PostTitlesWithApos,
	"P.KINSHIP_STEMS": KinshipStems, "P.PERSON_REL_BEFORE": PersonRelBefore, "P.MEDIA_TYPE_STEMS": MediaTypeStems,
	"P.LANGUAGE_NAMES": LanguageNames, "P.NO_APOS_RELIGION_OR_DERIVED": NoAposReligionOrDerived,
	"P.OPTIONAL_APOS_ISM_IYET": OptionalAposIsmIyet, "P.PLURAL_PLACE_KEEP_APOS": PluralPlaceKeepApos,
	"P.DERIV_LOOKING_KEEP_APOS": DerivLookingKeepApos, "P.TITLE_CONJUNCTIONS": TitleConjunctions,
}

// stringList is a JSON list of strings.
func stringList(v any) []string {
	var out []string
	for _, e := range v.([]any) {
		out = append(out, e.(string))
	}
	return out
}

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// answer is the Go answer to a probe: kind is the group of a grouped line ("" for a single probe), name the
// function, args its arguments. want is the golden's answer, which only the probes of a whole set or map read, to
// put their members in its order where order does not matter.
func (p *parity) answer(kind, name string, args []any, want any) (any, bool) {
	switch kind {
	case "w":
		return p.word(name, str(args, 0), str(args, 1))
	case "c":
		w := str(args, 0)
		switch name {
		case "matchAcronymApostrophe":
			return acronymJSON(MatchAcronymApostrophe(w)), true
		case "capTr":
			return CapTr(w), true
		case "foldTurkish":
			return FoldTurkish(w), true
		}
	case "p":
		return p.pair(name, str(args, 0), str(args, 1), str(args, 2))
	case "s":
		return p.stemSuffix(name, str(args, 0), str(args, 1), str(args, 2), str(args, 3))
	case "k":
		base := str(args, 0)
		out := []any{}
		for _, s := range p.particleSuffixes {
			out = append(out, IsValidKnownParticleSuffix(base, s))
		}
		return out, name == "isValidKnownParticleSuffix"
	case "n":
		n := str(args, 0)
		out := []any{}
		for _, s := range p.numberSuffixes {
			a, aok := FixNumberSuffix(n, s, false)
			b, bok := FixNumberSuffix(n, s, true)
			out = append(out, []any{opt(a, aok), opt(b, bok)})
		}
		return out, name == "fixNumberSuffix"
	case "":
		return p.single(name, args, want)
	}
	return nil, false
}

func (p *parity) word(name, w, vowel string) (any, bool) {
	folded := FoldTurkish(w)
	switch name {
	case "count":
		return p.lexicon.Count(w), true
	case "exactCount":
		return p.lexicon.ExactCount(w), true
	case "capitalShare":
		return p.lexicon.CapitalShare(w), true
	case "tsvCount":
		return p.tsvLexicon.Count(w), true
	case "tsvCapitalShare":
		return p.tsvLexicon.CapitalShare(w), true
	case "nameFor50":
		return opt(p.known.NameFor(w, 50)), true
	case "nameEntry":
		return nameEntryJSON(p.known.NameEntry(w, 0)), true
	case "asciiNameEntry":
		return nameEntryJSON(p.known.ASCIINameEntry(folded, 0)), true
	case "acronymFor":
		return opt(p.known.AcronymFor(w)), true
	case "asciiAcronymFor":
		return opt(p.known.ASCIIAcronymFor(folded)), true
	case "givenNameFor":
		return opt(p.known.GivenNameFor(w)), true
	case "surnameFor":
		return opt(p.known.SurnameFor(w)), true
	case "ambigNameFor":
		return opt(p.known.AmbigNameFor(w)), true
	case "phrases":
		return phrasesJSON(p.known.PhrasesStartingWith(w, false)), true
	case "phrasesPlain":
		return phrasesJSON(p.known.PhrasesStartingWith(w, true)), true
	case "find":
		return fixesJSON(p.known.Find(w, "", p.lexicon, false)), true
	case "lastVowel":
		if v := LastVowel(w); v != "" {
			return v, true
		}
		return nil, true
	case "sameHighVowels":
		return SameHighVowels(w), true
	case "isVoicelessFinal":
		return IsVoicelessFinal(w), true
	case "blocked":
		return []any{HasBlockedApostropheStem(w, false, false), HasBlockedApostropheStem(w, false, true),
			HasBlockedApostropheStem(w, true, false), HasBlockedApostropheStem(w, true, true)}, true
	case "fixDisharmoniousApostropheSuffix":
		return opt(FixDisharmoniousApostropheSuffix(w)), true
	case "isPlaceApostropheHead":
		return IsPlaceApostropheHead(w, ""), true
	case "isInverseFrontStem":
		return IsInverseFrontStem(w), true
	case "isKinshipWord":
		return IsKinshipWord(w), true
	case "isMediaTypeWord":
		return IsMediaTypeWord(w), true
	case "splitHonorific":
		return titleJSON(SplitHonorific(w)), true
	case "splitPostTitle":
		return titleJSON(SplitPostTitle(w)), true
	case "splitHeadWord":
		out := []any{}
		for _, set := range parityHeadSets {
			out = append(out, headJSON(SplitHeadWord(w, set)))
		}
		return out, true
	case "splitCalendarWord":
		return []any{calendarJSON(SplitCalendarWord(w, Months)), calendarJSON(SplitCalendarWord(w, Weekdays))}, true
	case "hasVowel":
		return HasVowel(w), true
	case "foldTurkish":
		return folded, true
	case "followsHarmony":
		return FollowsHarmony(w, vowel), true
	case "re":
		out := []any{}
		for _, re := range parityRegexes {
			out = append(out, execJSON(re, w))
		}
		return out, true
	}
	return nil, false
}

func (p *parity) pair(name, x, y, replacement string) (any, bool) {
	switch name {
	case "pair":
		return p.lexicon.Pair(x, y), true
	case "tsvPair":
		return p.tsvLexicon.Pair(x, y), true
	case "find":
		return fixesJSON(p.known.Find(x, y, p.lexicon, false)), true
	case "findPlain":
		return fixesJSON(p.known.Find(x, y, p.lexicon, true)), true
	case "collapsedDoubledLocative":
		return opt(CollapsedDoubledLocative(x, y)), true
	case "isPlaceApostropheHead":
		return IsPlaceApostropheHead(y, x), true
	case "isInstitutionHead":
		return IsInstitutionHead(y, x), true
	case "harmonizeQuestionParticle":
		return opt(HarmonizeQuestionParticle(y, x, false)), true
	case "harmonizeQuestionParticleInverse":
		return opt(HarmonizeQuestionParticle(y, x, true)), true
	case "isAuxiliaryOl":
		return IsAuxiliaryOl(x, y, replacement), true
	}
	return nil, false
}

func (p *parity) stemSuffix(name, stem, suffix, capped, plainSuffix string) (any, bool) {
	isMorph := func(w string) bool { return p.lexicon.Count(w) > 0 }
	switch name {
	case "isValidPlaceApostrophe":
		return IsValidPlaceApostrophe(stem, suffix), true
	case "isValidGenericApostrophe":
		return []any{IsValidGenericApostrophe(stem, suffix, false), IsValidGenericApostrophe(stem, suffix, true)}, true
	case "isPhonologicallyValidProperSuffix":
		return []any{
			IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{}),
			IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{Acronym: true}),
			IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{P3sgHead: true}),
		}, true
	case "harmonizeAsciiSuffix":
		return []any{opt(HarmonizeASCIISuffix(stem, suffix, ProperSuffixOptions{})), opt(HarmonizeASCIISuffix(stem, suffix, ProperSuffixOptions{P3sgHead: true}))}, true
	case "joinWithoutApostrophe":
		return []any{JoinWithoutApostrophe(stem, suffix, isMorph), JoinWithoutApostrophe(capped, suffix, isMorph)}, true
	case "isHarmoniousApostropheMarkChange":
		return []any{
			IsHarmoniousApostropheMarkChange(stem, plainSuffix, suffix, false),
			IsHarmoniousApostropheMarkChange(stem, plainSuffix, suffix, true),
			IsHarmoniousApostropheMarkChange(stem, suffix, plainSuffix, false),
		}, true
	case "isValidKnownParticleSuffix":
		return IsValidKnownParticleSuffix(stem+" "+suffix, suffix), true
	}
	return nil, false
}

func (p *parity) single(name string, args []any, want any) (any, bool) {
	switch name {
	case "lexicon.size":
		return p.lexicon.Size(), true
	case "lexicon.pairCount":
		return p.lexicon.PairCount(), true
	case "lexicon.toTsv":
		u, b := p.lexicon.ToTSV()
		return []any{sha(u), sha(b), len(utf16.Encode([]rune(u))), len(utf16.Encode([]rune(b)))}, true
	case "tsvLexicon.size":
		return p.tsvLexicon.Size(), true
	case "tsvLexicon.pairCount":
		return p.tsvLexicon.PairCount(), true
	case "tsvLexicon.toTsv":
		u, b := p.tsvLexicon.ToTSV()
		return []any{sha(u), sha(b)}, true
	case "known.size":
		return p.known.Size(), true
	case "known.hasProperData":
		return p.known.HasProperData(), true
	case "REGEXES":
		return parityRegexNames, true
	case "HEAD_SETS":
		return parityHeadSetNames, true
	case "particleSuffixes":
		p.particleSuffixes = stringList(want)
		return want, true
	case "numberSuffixes":
		p.numberSuffixes = stringList(want)
		return want, true
	case "set":
		set, ok := paritySets[str(args, 0)]
		if !ok {
			return nil, false
		}
		switch s := set.(type) {
		case WordSet:
			return s.Words(), true
		case map[string]bool:
			// Order never matters for these: the members are compared in the golden's order.
			var words []string
			for _, w := range want.([]any) {
				if s[w.(string)] {
					words = append(words, w.(string))
				}
			}
			if len(words) != len(s) {
				var all []string
				for w := range s {
					all = append(all, w)
				}
				slices.Sort(all)
				return all, true
			}
			return words, true
		}
	case "COMPOUND_VERB_JOINS":
		// A Map in the TypeScript, a map here: compared in the golden's order of keys.
		out := []any{}
		for _, entry := range want.([]any) {
			key := entry.([]any)[0].(string)
			rules := []any{}
			for _, r := range CompoundVerbJoins[key] {
				m := map[string]any{"auxStem": r.AuxStem, "prefix": r.Prefix}
				if r.StrictZeroPair {
					m["strictZeroPair"] = true
				}
				rules = append(rules, m)
			}
			out = append(out, []any{key, rules})
		}
		if len(out) != len(CompoundVerbJoins) {
			return fmt.Sprintf("%d keys", len(CompoundVerbJoins)), true
		}
		return out, true
	case "CANONICAL_PHRASES":
		out := []any{}
		for _, c := range CanonicalPhrases {
			if c.Context != "" {
				out = append(out, []any{c.Display, c.Apos, string(c.Context)})
			} else {
				out = append(out, []any{c.Display, c.Apos})
			}
		}
		return out, true
	case "collapsedDoubledLocative":
		return opt(CollapsedDoubledLocative(str(args, 0), str(args, 1))), true
	case "harmonizeQuestionParticle":
		return opt(HarmonizeQuestionParticle(str(args, 0), str(args, 1), args[2].(bool))), true
	case "isAuxiliaryOl":
		return IsAuxiliaryOl(str(args, 0), str(args, 1), str(args, 2)), true
	case "followsHarmony":
		return FollowsHarmony(str(args, 0), str(args, 1)), true
	case "fixNumberSuffix":
		return opt(FixNumberSuffix(str(args, 0), str(args, 1), args[2].(bool))), true
	}
	return nil, false
}
