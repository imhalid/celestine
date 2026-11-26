package morph

// This file is dictionary.ts: the Zemberek .dict parser, the inference of Turkish root attributes, and the stem
// transitions of the dictionary items. It is adapted from Zemberek's TextDictionaryLoader and
// StemTransitionsBase.

import (
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// primaryPosMap is the primary parts of speech of the .dict files, by their names there.
var primaryPosMap = map[string]PrimaryPos{
	"Noun": "Noun", "Adj": "Adj", "Adjective": "Adj", "Verb": "Verb", "Pron": "Pron", "Pronoun": "Pron",
	"Adv": "Adv", "Adverb": "Adv", "Conj": "Conj", "Conjunction": "Conj", "Ques": "Ques", "Question": "Ques",
	"Postp": "Postp", "PostPositive": "Postp", "Det": "Det", "Determiner": "Det", "Num": "Num", "Numeral": "Num",
	"Dup": "Dup", "Duplicator": "Dup", "Interj": "Interj", "Interjection": "Interj", "Punc": "Punc",
	"Punctuation": "Punc",
}

// secondaryPosMap is the secondary parts of speech of the .dict files, by their names there.
var secondaryPosMap = map[string]SecondaryPos{
	"Prop": "ProperNoun", "ProperNoun": "ProperNoun", "Abbrv": "Abbreviation", "Abbreviation": "Abbreviation",
	"Time": "Time", "Pers": "PersonalPron", "PersonalPron": "PersonalPron", "Demons": "DemonstrativePron",
	"DemonstrativePron": "DemonstrativePron", "Quant": "QuantitivePron", "QuantitivePron": "QuantitivePron",
	"Ques": "QuestionPron", "QuestionPron": "QuestionPron", "Reflex": "ReflexivePron",
	"ReflexivePron": "ReflexivePron", "Card": "Card", "Ord": "Ord", "Dist": "Dist", "PCDat": "PCDat",
	"PCAbl": "PCAbl", "PCNom": "PCNom", "PCGen": "PCGen", "PCIns": "PCIns", "PCAcc": "PCAcc",
}

// rootAttrMap is the root attributes of the .dict files, by their names there.
var rootAttrMap = map[string]int{
	"Voicing": raVoicing, "NoVoicing": raNoVoicing, "VoicingOpt": raVoicingOpt, "InverseHarmony": raInverseHarmony,
	"Doubling": raDoubling, "LastVowelDrop": raLastVowelDrop, "CompoundP3sg": raCompoundP3sg,
	"CompoundP3sgRoot": raCompoundP3sgRoot, "NoSuffix": raNoSuffix, "ProgressiveVowelDrop": raProgressiveVowelDrop,
	"Aorist_I": raAoristI, "Aorist_A": raAoristA, "Passive_In": raPassiveIn, "Causative_t": raCausativeT,
	"ImplicitPlural": raImplicitPlural, "ImplicitDative": raImplicitDative, "ImplicitP1sg": raImplicitP1sg,
	"ImplicitP2sg": raImplicitP2sg, "FamilyMember": raFamilyMember, "Reciprocal": raReciprocal,
	"NonReciprocal": raNonReciprocal, "Reflexive": raReflexive, "Dummy": raDummy,
	"NounConsInsert_n": raNounConsInsertN, "Ext": raExt, "Informal": raInformal,
}

// excludedNonTDKLemmas are known non-TDK entries and typos of Zemberek's master-dictionary.dict (mostly from the
// adverb and interjection blocks added by hand at its end), left out so that common joined or colloquial
// misspellings are not taken as valid.
var excludedNonTDKLemmas = stringSet([]string{
	"afedersin", "afedersinin", "ardarda", "birara", "birarada", "çöğunlukla", "elele", "herbiri", "herzaman",
	"ilerde", "keşki", "kolkola", "yanısıra",
})

// makeKey is a dictionary item's lookup key.
func makeKey(rootLemma string, primaryPos PrimaryPos, secondaryPos SecondaryPos) string {
	if secondaryPos == "None" {
		return rootLemma + "_" + primaryPos
	}
	return rootLemma + "_" + primaryPos + "_" + secondaryPos
}

// inferAttributes is attrs with the attributes a root's form implies: for verbs the progressive vowel drop, the
// aorist, the passive and the causative; for other words the voicing of a final stop.
func inferAttributes(root string, primaryPos PrimaryPos, secondaryPos SecondaryPos, attrs int) int {
	out := attrs
	rs := []rune(root)
	n := len(rs)
	if n == 0 {
		return out
	}
	last := rs[n-1]
	var prev rune
	if n > 1 {
		prev = rs[n-2]
	}
	vowels := countVowels(root)

	if primaryPos == "Verb" {
		// The special verbs are handled by their own stem transitions and states.
		if root == "de" || root == "ye" || root == "i" || root == "değil" {
			return out
		}
		// Progressive vowel drop: verb stems ending in a vowel (ara -> ar-ıyor, bekle -> bekl-iyor).
		if isVowel(last) {
			out |= raProgressiveVowelDrop
		}
		// Aorist: one syllable takes -Ar, more take -Ir, unless the entry says otherwise.
		if out&(raAoristA|raAoristI) == 0 {
			if vowels <= 1 && !isVowel(last) {
				out |= raAoristA
			} else {
				out |= raAoristI
			}
		}
		// Passive: stems ending in a vowel or l take -In/-InIl; others take +nIl.
		if isVowel(last) || last == 'l' {
			out |= raPassiveIn
		}
		// Causative -t: stems ending in a vowel, or longer ones ending in r or l.
		if isVowel(last) || vowels > 1 && (last == 'r' || last == 'l') {
			out |= raCausativeT
		}
		return out
	}

	// The voicing of a final consonant (kitap -> kitab, ağaç -> ağac, renk -> reng, psikolog -> psikoloğ). Proper
	// nouns and abbreviations do not soften in writing unless their entry says so.
	if (primaryPos == "Noun" || primaryPos == "Adj" || primaryPos == "Num" || primaryPos == "Pron") &&
		secondaryPos != "ProperNoun" && secondaryPos != "Abbreviation" &&
		out&(raVoicing|raNoVoicing|raVoicingOpt|raDoubling|raInverseHarmony|raNoSuffix|raCompoundP3sg) == 0 {
		endsWithNk := n >= 2 && prev == 'n' && last == 'k'
		endsWithVowelG := n >= 2 && isVowel(prev) && last == 'g'
		if endsWithNk {
			out |= raVoicing
		} else if vowels > 1 && (isVoicelessStop(last) || endsWithVowelG) {
			out |= raVoicing
		}
	}
	return out
}

// pendingRef is an item whose Ref: is resolved after all items are read.
type pendingRef struct {
	item   *dictionaryItem
	refKey string
}

// parseDictionaries reads Zemberek .dict texts into dictionary items. Entries marked A:Informal or A:Ext (noisy,
// colloquial or non-TDK spellings) and P:Punc entries are left out for the spell checker's precision.
func parseDictionaries(texts []string) []*dictionaryItem {
	var items []*dictionaryItem
	byKey := map[string]*dictionaryItem{}
	var pendingRefs []pendingRef

	for _, text := range texts {
		start := 0
		n := len(text)
		for start <= n {
			end := strings.IndexByte(text[start:], '\n')
			if end == -1 {
				end = n
			} else {
				end += start
			}
			lineEnd := end
			if lineEnd > start && text[lineEnd-1] == '\r' {
				lineEnd--
			}
			rawLine := jsTrim(text[start:lineEnd])
			start = end + 1

			if rawLine == "" || rawLine[0] == '#' {
				continue
			}

			bracketIdx := strings.IndexByte(rawLine, '[')
			rawLemma := rawLine
			if bracketIdx != -1 {
				rawLemma = rawLine[:bracketIdx]
			}
			rawLemma = jsTrim(rawLemma)
			if rawLemma == "" {
				continue
			}

			primaryPos := ""
			secondaryPos := "None"
			attrs := raNone
			rawPron, rawRoots, refSpec := "", "", ""
			index := 0

			if bracketIdx != -1 {
				closeIdx := strings.LastIndexByte(rawLine, ']')
				meta := rawLine[bracketIdx+1:]
				if closeIdx > bracketIdx {
					meta = rawLine[bracketIdx+1 : closeIdx]
				}
				for _, seg := range strings.Split(meta, ";") {
					seg = jsTrim(seg)
					if seg == "" {
						continue
					}
					colon := strings.IndexByte(seg, ':')
					if colon == -1 {
						continue
					}
					tag := jsTrim(seg[:colon])
					val := jsTrim(seg[colon+1:])
					switch tag {
					case "P":
						comma := strings.IndexByte(val, ',')
						if comma == -1 {
							if p, ok := primaryPosMap[val]; ok {
								primaryPos = p
							}
						} else {
							if p, ok := primaryPosMap[jsTrim(val[:comma])]; ok {
								primaryPos = p
							}
							if s, ok := secondaryPosMap[jsTrim(val[comma+1:])]; ok {
								secondaryPos = s
							}
						}
					case "A":
						for _, attrName := range strings.Split(val, ",") {
							attrName = jsTrim(attrName)
							if attrName == "" {
								continue
							}
							attrs |= rootAttrMap[attrName]
						}
					case "Pr", "pr":
						rawPron = val
					case "Roots":
						rawRoots = val
					case "Index":
						if parsed, ok := jsParseInt(val); ok {
							index = parsed
						}
					case "Ref":
						refSpec = val
					}
				}
			}

			// Punctuation, informal spellings and the non-TDK [A:Ext] entries are left out.
			if primaryPos == "Punc" || attrs&(raInformal|raExt) != 0 {
				continue
			}

			lowerLemma := tr.Lower(rawLemma)
			normLemma := normalizeCircumflex(lowerLemma)
			if excludedNonTDKLemmas[normLemma] {
				continue
			}

			endsInfinitive := runeLen(normLemma) > 3 && (strings.HasSuffix(normLemma, "mak") || strings.HasSuffix(normLemma, "mek"))
			// The part of speech when the entry gives none.
			if primaryPos == "" {
				if endsInfinitive {
					primaryPos = "Verb"
				} else {
					primaryPos = "Noun"
					firstRaw := firstRune(rawLemma)
					if firstRaw != tr.Lower(firstRaw) {
						secondaryPos = "ProperNoun"
					}
				}
			}

			root := normLemma
			if primaryPos == "Verb" && endsInfinitive {
				root = normLemma[:len(normLemma)-3]
			}
			pronunciation := root
			if rawPron != "" {
				pronunciation = normalizeCircumflex(tr.Lower(rawPron))
			}
			inferredAttrs := inferAttributes(root, primaryPos, secondaryPos, attrs)
			key := makeKey(normLemma, primaryPos, secondaryPos)

			item := &dictionaryItem{
				key:           key,
				lemma:         lowerLemma,
				root:          root,
				pronunciation: pronunciation,
				primaryPos:    primaryPos,
				secondaryPos:  secondaryPos,
				attributes:    inferredAttrs,
				index:         index,
			}
			items = append(items, item)
			lowerKey := tr.Lower(key)
			if _, ok := byKey[lowerKey]; index == 0 && !ok {
				byKey[lowerKey] = item
			}
			if refSpec != "" {
				pendingRefs = append(pendingRefs, pendingRef{item, normalizeCircumflex(tr.Lower(refSpec))})
			}

			// The dummy CompoundP3sgRoot stem of a compound noun with Roots:r1-r2 (zeytinyağı [A:CompoundP3sg;
			// Roots:zeytin-yağ] gives the dummy root "zeytinyağ").
			if inferredAttrs&raCompoundP3sg != 0 && rawRoots != "" && primaryPos == "Noun" {
				dashIdx := strings.IndexByte(rawRoots, '-')
				if dashIdx > 0 && dashIdx < len(rawRoots)-1 {
					compoundRoot := normalizeCircumflex(tr.Lower(rawRoots[:dashIdx] + rawRoots[dashIdx+1:]))
					if compoundRoot != "" && compoundRoot != root {
						items = append(items, &dictionaryItem{
							key:           compoundRoot + "_Noun_CompoundRoot",
							lemma:         lowerLemma,
							root:          compoundRoot,
							pronunciation: compoundRoot,
							primaryPos:    "Noun",
							secondaryPos:  "None",
							attributes:    raCompoundP3sgRoot | raDummy | inferredAttrs&raInverseHarmony,
							referenceItem: item,
						})
					}
				}
			}
		}
	}

	// The Ref: links (the variants of LastVowelDrop words that keep the vowel, and circumflexed variants).
	for _, p := range pendingRefs {
		if direct, ok := byKey[p.refKey]; ok {
			p.item.referenceItem = direct
		} else if fallback, ok := byKey[p.refKey+"_"+tr.Lower(p.item.primaryPos)]; ok {
			p.item.referenceItem = fallback
		}
	}
	return items
}

// modifierAttrMask are the attributes that give a root a second, modified stem.
const modifierAttrMask = raDoubling | raLastVowelDrop | raProgressiveVowelDrop | raInverseHarmony | raVoicing |
	raCompoundP3sg | raCompoundP3sgRoot

// specialRootKeys are the roots with stems of their own (ben/ban, de/di, birbiri/birbir).
var specialRootKeys = stringSet([]string{
	"ben_Pron_PersonalPron", "sen_Pron_PersonalPron", "demek_Verb", "yemek_Verb", "imek_Verb",
	"birbiri_Pron_QuantitivePron", "çoğu_Pron_QuantitivePron", "öbürü_Pron_QuantitivePron",
	"birçoğu_Pron_QuantitivePron",
})

// isNonDroppedLastVowelDropVariant reports whether item is the variant of a LastVowelDrop word that keeps the
// vowel.
func isNonDroppedLastVowelDropVariant(item *dictionaryItem) bool {
	ref := item.referenceItem
	return ref != nil && ref.attributes&raLastVowelDrop != 0 && item.attributes&raLastVowelDrop == 0 &&
		item.root == ref.root
}

// newStem is a stem transition.
func newStem(surface string, item *dictionaryItem, phonetic int, to *morphemeState) *stemTransition {
	return &stemTransition{surface: surface, runes: []rune(surface), item: item, phonetic: phonetic, to: to}
}

// generateSpecialRootTransitions is the stems of a root of specialRootKeys.
func generateSpecialRootTransitions(item *dictionaryItem, graph *morphotacticsGraph) []*stemTransition {
	origAttrs := computePhoneticAttributes(item.pronunciation, 0)
	unmodifiedState := graph.getRootState(item, origAttrs)

	switch item.key {
	case "ben_Pron_PersonalPron", "sen_Pron_PersonalPron":
		modSurface := "san"
		if item.root == "ben" {
			modSurface = "ban"
		}
		return []*stemTransition{
			newStem(item.root, item, origAttrs|paUnmodifiedPronoun, unmodifiedState),
			newStem(modSurface, item, computePhoneticAttributes(modSurface, 0)|paModifiedPronoun, graph.pronPers_Mod_S),
		}
	case "demek_Verb", "yemek_Verb":
		modSurface := "yi"
		if item.root == "de" {
			modSurface = "di"
		}
		return []*stemTransition{
			newStem(item.root, item, origAttrs, graph.vDeYeRoot_S),
			newStem(modSurface, item, computePhoneticAttributes(modSurface, 0), graph.vDeYeRoot_S),
		}
	case "imek_Verb":
		return []*stemTransition{newStem(item.root, item, origAttrs, graph.imekRoot_S)}
	case "birbiri_Pron_QuantitivePron", "çoğu_Pron_QuantitivePron", "öbürü_Pron_QuantitivePron",
		"birçoğu_Pron_QuantitivePron":
		var modSurface string
		switch item.root {
		case "birbiri":
			modSurface = "birbir"
		case "çoğu":
			modSurface = "çok"
		case "öbürü":
			modSurface = "öbür"
		default:
			modSurface = "birçok"
		}
		return []*stemTransition{
			newStem(item.root, item, origAttrs|paUnmodifiedPronoun, graph.pronQuant_S),
			newStem(modSurface, item, computePhoneticAttributes(modSurface, 0)|paModifiedPronoun, graph.pronQuantModified_S),
		}
	}
	return []*stemTransition{newStem(item.root, item, origAttrs, unmodifiedState)}
}

// generateStemTransitions is the stems of a dictionary item: the root, and the modified stem its attributes
// make (kitap and kitab, ağız and ağz, hak and hakk).
func generateStemTransitions(item *dictionaryItem, graph *morphotacticsGraph) []*stemTransition {
	if specialRootKeys[item.key] {
		return generateSpecialRootTransitions(item, graph)
	}

	if item.attributes&modifierAttrMask == 0 {
		phonetic := computePhoneticAttributes(item.pronunciation, 0)
		if isNonDroppedLastVowelDropVariant(item) {
			phonetic |= paExpectsVowel | paCannotTerminate
		}
		return []*stemTransition{newStem(item.root, item, phonetic, graph.getRootState(item, phonetic))}
	}

	modifiedSeq := []rune(item.root)
	originalAttrs := computePhoneticAttributes(item.pronunciation, 0)
	modifiedAttrs := originalAttrs
	var unmodifiedRootState, modifiedRootState *morphemeState

	// Voicing comes before Doubling (ret -> red -> redd, tıp -> tıb -> tıbb, zıt -> zıd -> zıdd).
	if item.attributes&raVoicing != 0 && len(modifiedSeq) > 0 {
		last := modifiedSeq[len(modifiedSeq)-1]
		voiced := voiceChar(last)
		if strings.HasSuffix(string(modifiedSeq), "nk") {
			voiced = 'g'
		}
		if voiced != last {
			modifiedSeq = append(modifiedSeq[:len(modifiedSeq)-1:len(modifiedSeq)-1], voiced)
			modifiedAttrs &^= paLastVoicelessStop | paLastVoiceless
			modifiedAttrs |= paLastVoiced | paExpectsVowel | paCannotTerminate
			originalAttrs |= paExpectsConsonant
		}
	}

	if item.attributes&raDoubling != 0 && len(modifiedSeq) > 0 {
		last := modifiedSeq[len(modifiedSeq)-1]
		modifiedSeq = append(modifiedSeq[:len(modifiedSeq):len(modifiedSeq)], last)
		originalAttrs |= paExpectsConsonant
		modifiedAttrs |= paExpectsVowel | paCannotTerminate
	}

	if item.attributes&raLastVowelDrop != 0 && len(modifiedSeq) >= 2 {
		last := modifiedSeq[len(modifiedSeq)-1]
		if !isVowel(last) {
			modifiedSeq = append(modifiedSeq[:len(modifiedSeq)-2:len(modifiedSeq)-2], last)
			if item.primaryPos != "Verb" {
				originalAttrs |= paExpectsConsonant
			} else {
				unmodifiedRootState = graph.verbLastVowelDropUnmodRoot_S
				modifiedRootState = graph.verbLastVowelDropModRoot_S
			}
			modifiedAttrs |= paExpectsVowel | paCannotTerminate
		}
	}

	if item.attributes&raInverseHarmony != 0 {
		originalAttrs = originalAttrs&^paVowelBack | paVowelFront
		modifiedAttrs = modifiedAttrs&^paVowelBack | paVowelFront
	}

	if item.attributes&raProgressiveVowelDrop != 0 && len(modifiedSeq) > 1 {
		modifiedSeq = modifiedSeq[:len(modifiedSeq)-1]
		if containsVowel(string(modifiedSeq)) {
			modifiedAttrs = computePhoneticAttributes(string(modifiedSeq), 0)
		}
		modifiedAttrs |= paLastLetterDropped
	}

	unmodState := unmodifiedRootState
	if unmodState == nil {
		unmodState = graph.getRootState(item, originalAttrs)
	}
	modState := modifiedRootState
	if modState == nil {
		modState = graph.getRootState(item, modifiedAttrs)
	}

	original := newStem(item.root, item, originalAttrs, unmodState)
	modified := newStem(string(modifiedSeq), item, modifiedAttrs, modState)

	if original.surface == modified.surface && original.phonetic == modified.phonetic && original.to == modified.to {
		return []*stemTransition{original}
	}
	if isNonDroppedLastVowelDropVariant(item) && item.attributes&raVoicing != 0 {
		return []*stemTransition{modified}
	}
	return []*stemTransition{original, modified}
}

// stemIndex is the stems by their surfaces, kept in the order they were first added (the TypeScript Map's
// order, which the fuzzy search's trie follows).
type stemIndex struct {
	stems         map[string][]*stemTransition
	order         []string
	maxStemLength int
}

// buildStemIndex makes the stems of the dictionary items and indexes them by surface.
func buildStemIndex(items []*dictionaryItem, graph *morphotacticsGraph) *stemIndex {
	idx := &stemIndex{stems: make(map[string][]*stemTransition, len(items)+len(items)/4)}
	for _, item := range items {
		if item.root == "" {
			continue
		}
		for _, t := range generateStemTransitions(item, graph) {
			if t.surface == "" {
				continue
			}
			if len(t.runes) > idx.maxStemLength {
				idx.maxStemLength = len(t.runes)
			}
			existing, ok := idx.stems[t.surface]
			if !ok {
				idx.order = append(idx.order, t.surface)
			}
			idx.stems[t.surface] = append(existing, t)
		}
	}
	return idx
}
