package spell

import (
	"math"
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is the part of tinyfix-tr's src/context.ts the corrector uses: the types of the context model's
// queries and the features it is given. The model itself (its tokenizer and the network) is ported later and
// plugged in through ContextModel; without one the corrector runs its core pass alone.

// ContextKind is the kind of decision a context query asks for.
type ContextKind string

// The context kinds.
const (
	ContextMarks ContextKind = "marks"
	ContextDeDa  ContextKind = "de_da"
	ContextKi    ContextKind = "ki"
	ContextMi    ContextKind = "mi"
	ContextTypo  ContextKind = "typo"
	ContextName  ContextKind = "name"
)

// FeatureDim is the length of a candidate's feature vector.
const FeatureDim = 26

// TypoCandidateMeta is what the core corrector knows of a typo candidate.
type TypoCandidateMeta struct {
	// CoreMargin is the core corrector's score-base margin (0 for the typed word).
	CoreMargin float64
	// SlipCost is the keyboard or edit cost from oneEdit (0 for the typed word or a pure mark variant).
	SlipCost float64
	// IsMarkVariant is true when the candidate differs from what was typed only in Turkish marks.
	IsMarkVariant bool
}

// NameCandidateMeta is what is known of a candidate reading of an ambiguous proper name.
type NameCandidateMeta struct {
	// IsProper is true for the proper-noun candidate ("Deniz", "Bahar'la"), false for the common-word one
	// ("deniz", "baharla").
	IsProper bool
	// StemCapShare is the lexicon's mid-sentence capital share of the base stem (0..1).
	StemCapShare float64
	// AposShare is the share of suffixed occurrences written with an apostrophe, count(base'suf) /
	// (count(base'suf) + count(base+suf) + 1), 0 when bare.
	AposShare float64
	// IsGiven is true when the base stem is a known Turkish given name.
	IsGiven bool
	// AtSentenceStart is true when the token sits at a sentence start.
	AtSentenceStart bool
}

// ContextQuery is one place in the text to judge: the text around it and the spans that could go there.
type ContextQuery struct {
	Left, Right string
	// Candidates are the spans; the first is what was typed.
	Candidates []string
	// Features are the candidates' scalar features (FeatureDim each), or nil.
	Features [][]float64
}

// ContextModel scores how well each candidate span fits between the text on its left and right: one row per
// query, one score per candidate.
type ContextModel interface {
	Score(queries []ContextQuery) ([][]float64, error)
	// SupportsNameContext reports whether the model judges ambiguous proper names (ContextName).
	SupportsNameContext() bool
}

// counter is what the features read from a lexicon: form and pair counts.
type counter interface {
	Count(form string) float64
	Pair(first, second string) float64
}

var (
	possMorphemes   = setOf("P1sg", "P2sg", "P3sg", "P1pl", "P2pl", "P3pl")
	finiteMorphemes = setOf("Prog1", "Prog2", "Past", "Narr", "Fut", "Aor", "Opt", "Desr", "Neces", "Imp")
	funcPos         = setOf("Pron", "Conj", "Postp", "Det")
)

// morphFeatureBits is the 8-bit morphology feature mask of a word (scripts/extract-morph-features.ts).
func (c *Corrector) morphFeatureBits(word string) int {
	if c.morphology == nil {
		return 0
	}
	analyses, ok := c.analyze(word)
	if !ok {
		if c.morphology.IsWord(word) {
			return 1
		}
		return 0
	}
	if len(analyses) == 0 {
		return 0
	}
	bits := 1
	for _, a := range analyses {
		switch {
		case a.Pos == "Verb":
			bits |= 2
		case a.Pos == "Noun":
			bits |= 4
		case a.Pos == "Adj" || a.Pos == "Adv":
			bits |= 8
		case funcPos[a.Pos]:
			bits |= 16
		}
		for _, m := range a.Morphemes {
			switch {
			case m == "Loc" || m == "Abl":
				bits |= 32
			case possMorphemes[m]:
				bits |= 64
			case finiteMorphemes[m]:
				bits |= 128
			}
		}
	}
	return bits
}

// buildContextFeatures is the 26-dim feature vector of each candidate (py/context_data.py's build_features and
// py/build_name_context_data.py).
func (c *Corrector) buildContextFeatures(candidates []string, kind ContextKind, asciiContext bool, lexicon counter, typoMeta []TypoCandidateMeta, nameMeta []NameCandidateMeta) [][]float64 {
	logCounts := make([]float64, len(candidates))
	minLogs := make([]float64, len(candidates))
	for i, cand := range candidates {
		low := tr.Lower(cand)
		if first, second, ok := strings.Cut(low, " "); ok {
			pairC := lexicon.Pair(first, second)
			minW := math.Min(lexicon.Count(first), lexicon.Count(second))
			logCounts[i] = jsLog(pairC + 1)
			minLogs[i] = jsLog(minW + 1)
		} else {
			lc := jsLog(lexicon.Count(low) + 1)
			logCounts[i] = lc
			minLogs[i] = lc
		}
	}
	maxLc := 0.0
	if len(logCounts) > 0 {
		maxLc = math.Inf(-1)
		for _, lc := range logCounts {
			maxLc = math.Max(maxLc, lc)
		}
	}
	typedLow := ""
	if len(candidates) > 0 {
		typedLow = tr.Lower(candidates[0])
	}
	maxCoreMargin := math.Inf(-1)
	if kind == ContextTypo && typoMeta != nil {
		for i := 1; i < len(candidates); i++ {
			m := -10.0
			if i < len(typoMeta) {
				m = typoMeta[i].CoreMargin
			}
			if m > maxCoreMargin {
				maxCoreMargin = m
			}
		}
	}
	if math.IsInf(maxCoreMargin, 0) || math.IsNaN(maxCoreMargin) {
		maxCoreMargin = 0
	}
	clamp := func(lo, x, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
	b2f := func(b bool) float64 {
		if b {
			return 1
		}
		return 0
	}
	rows := make([][]float64, len(candidates))
	for idx, cand := range candidates {
		row := make([]float64, FeatureDim)
		low := tr.Lower(cand)
		head, _, isPair := strings.Cut(low, " ")
		bits := c.morphFeatureBits(head)
		row[0] = logCounts[idx] / 15
		row[1] = (logCounts[idx] - maxLc) / 10
		row[2] = minLogs[idx] / 15
		row[3] = b2f(isPair)
		row[4] = b2f(tr.HasTurkishMarks(cand))
		row[5] = b2f(asciiContext)
		switch kind {
		case ContextMarks:
			row[6] = 1
		case ContextDeDa:
			row[7] = 1
		case ContextKi:
			row[8] = 1
		case ContextMi:
			row[9] = 1
		}
		for b := 0; b < 8; b++ {
			row[10+b] = float64((bits >> b) & 1)
		}
		switch kind {
		case ContextTypo:
			row[18] = 1
			if idx > 0 && idx < len(typoMeta) {
				meta := typoMeta[idx]
				lowLen, typedLen := runeLen(low), runeLen(typedLow)
				row[19] = clamp(-1.5, meta.CoreMargin/10, 1.5)
				row[20] = clamp(-1.5, (meta.CoreMargin-maxCoreMargin)/10, 0)
				row[21] = math.Min(2.0, meta.SlipCost/5)
				row[22] = b2f(lowLen > typedLen)
				row[23] = b2f(lowLen < typedLen)
				row[24] = b2f(lowLen == typedLen && low != typedLow)
				row[25] = b2f(meta.IsMarkVariant)
			}
		case ContextName:
			// Slots 6..9 and 18 stay 0, so the scorer tells the "name" kind apart without a longer vector.
			if idx < len(nameMeta) {
				meta := nameMeta[idx]
				row[19] = b2f(meta.IsProper)
				row[20] = b2f(strings.Contains(cand, "'"))
				row[21] = meta.StemCapShare
				row[22] = meta.AposShare
				row[23] = b2f(meta.IsGiven)
				row[24] = b2f(!meta.IsGiven)
				row[25] = b2f(meta.AtSentenceStart)
			}
		}
		rows[idx] = row
	}
	return rows
}
