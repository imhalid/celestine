package morph

// This file is phonetics.ts: Turkish phonological helpers, the phonetic attribute bit masks, and the surfaces of
// suffix templates. It is adapted from Zemberek's TurkishAlphabet, AttributesHelper and SurfaceTransition.

import "strings"

// The phonetic attribute bit masks.
const (
	paLastVowel          = 1 << 0
	paLastConsonant      = 1 << 1
	paVowelFront         = 1 << 2
	paVowelBack          = 1 << 3
	paVowelRound         = 1 << 4
	paVowelUnround       = 1 << 5
	paLastVoiced         = 1 << 6
	paLastVoiceless      = 1 << 7
	paLastVoicelessStop  = 1 << 8
	paFirstVowel         = 1 << 9
	paFirstConsonant     = 1 << 10
	paHasNoVowel         = 1 << 11
	paExpectsVowel       = 1 << 12
	paExpectsConsonant   = 1 << 13
	paCannotTerminate    = 1 << 14
	paLastLetterDropped  = 1 << 15
	paModifiedPronoun    = 1 << 16
	paUnmodifiedPronoun  = 1 << 17
	vowelHarmonyMask     = paVowelFront | paVowelBack | paVowelRound | paVowelUnround
	surfaceCacheSize     = 64
	tokenLetter          = 0
	tokenAVowel          = 1
	tokenIVowel          = 2
	tokenAppend          = 3
	tokenDevoiceFirst    = 4
	tokenLastVoiced      = 5
	tokenLastNotVoiced   = 6
	circumflexCharacters = "âîû"
)

// isVowel reports whether r is a Turkish vowel, circumflexed ones included.
func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'ı', 'i', 'o', 'ö', 'u', 'ü', 'â', 'î', 'û':
		return true
	}
	return false
}

// isFrontVowel reports whether r is a front vowel.
func isFrontVowel(r rune) bool {
	switch r {
	case 'e', 'i', 'ö', 'ü', 'î':
		return true
	}
	return false
}

// isRoundedVowel reports whether r is a rounded vowel.
func isRoundedVowel(r rune) bool {
	switch r {
	case 'o', 'ö', 'u', 'ü', 'û':
		return true
	}
	return false
}

// isVoiceless reports whether r is a voiceless consonant.
func isVoiceless(r rune) bool {
	switch r {
	case 'ç', 'f', 'h', 'k', 'p', 's', 'ş', 't':
		return true
	}
	return false
}

// isVoicelessStop reports whether r is a voiceless stop.
func isVoicelessStop(r rune) bool {
	switch r {
	case 'ç', 'k', 'p', 't':
		return true
	}
	return false
}

// voiceChar is the voiced form of a voiceless stop (and g), r itself for any other letter.
func voiceChar(r rune) rune {
	switch r {
	case 'p':
		return 'b'
	case 'ç':
		return 'c'
	case 't':
		return 'd'
	case 'k', 'g':
		return 'ğ'
	}
	return r
}

// devoiceChar is the voiceless form of b, c, d and g, as a suffix's first letter takes it after a voiceless sound.
func devoiceChar(s string) string {
	switch s {
	case "b":
		return "p"
	case "c":
		return "ç"
	case "d":
		return "t"
	case "g":
		return "k"
	}
	return s
}

// countVowels is the number of vowels in word.
func countVowels(word string) int {
	count := 0
	for _, r := range word {
		if isVowel(r) {
			count++
		}
	}
	return count
}

// containsVowel reports whether word has a vowel.
func containsVowel(word string) bool {
	for _, r := range word {
		if isVowel(r) {
			return true
		}
	}
	return false
}

// hasCircumflex reports whether word has â, î or û.
func hasCircumflex(word string) bool { return strings.ContainsAny(word, circumflexCharacters) }

// normalizeCircumflex is word with â, î and û as a, i and u.
func normalizeCircumflex(word string) string {
	if !hasCircumflex(word) {
		return word
	}
	return circumflexReplacer.Replace(word)
}

// circumflexReplacer writes â, î and û as a, i and u.
var circumflexReplacer = strings.NewReplacer("â", "a", "î", "i", "û", "u")

// surfaceCacheKey packs the 6 phonetic bits that a suffix's surface depends on into 0..63.
func surfaceCacheKey(attrs int) int {
	key := 0
	if attrs&paLastVowel != 0 {
		key |= 1
	}
	if attrs&paVowelBack != 0 {
		key |= 2
	}
	if attrs&paVowelFront != 0 {
		key |= 4
	}
	if attrs&paVowelRound != 0 {
		key |= 8
	}
	if attrs&paVowelUnround != 0 {
		key |= 16
	}
	if attrs&paLastVoiceless != 0 {
		key |= 32
	}
	return key
}

// computePhoneticAttributes is the phonetic bit mask of seq; a seq without vowels (a consonant suffix such as -m,
// -n, -k) takes its vowel harmony from predecessorAttrs.
func computePhoneticAttributes(seq string, predecessorAttrs int) int {
	if seq == "" {
		return predecessorAttrs
	}
	rs := []rune(seq)
	var lastVowel rune
	for i := len(rs) - 1; i >= 0; i-- {
		if isVowel(rs[i]) {
			lastVowel = rs[i]
			break
		}
	}
	first, last := rs[0], rs[len(rs)-1]
	attrs := 0
	if lastVowel != 0 {
		if isVowel(last) {
			attrs |= paLastVowel
		} else {
			attrs |= paLastConsonant
		}
		if isFrontVowel(lastVowel) {
			attrs |= paVowelFront
		} else {
			attrs |= paVowelBack
		}
		if isRoundedVowel(lastVowel) {
			attrs |= paVowelRound
		} else {
			attrs |= paVowelUnround
		}
		if isVowel(first) {
			attrs |= paFirstVowel
		} else {
			attrs |= paFirstConsonant
		}
	} else {
		// Only the vowel harmony comes from the predecessor; the consonant and voicing bits are made anew.
		attrs = predecessorAttrs&vowelHarmonyMask | paLastConsonant | paFirstConsonant | paHasNoVowel
	}
	if isVoiceless(last) {
		attrs |= paLastVoiceless
		if isVoicelessStop(last) {
			attrs |= paLastVoicelessStop
		}
	} else {
		attrs |= paLastVoiced
	}
	return attrs
}

// tokenizeTemplate splits a Zemberek suffix template into tokens:
//   - A: the low vowel of the harmony (a/e), left out at the start after a vowel;
//   - I: the high vowel of the harmony (ı/i/u/ü), left out at the start after a vowel;
//   - +x: a buffer letter x, written only after a vowel;
//   - >x: x devoiced after a voiceless sound;
//   - ~x: a final voiceless stop that expects a consonant next (-AcA~k, -dI~k, -lI~k);
//   - !x: a final voiced stop that expects a vowel next and cannot end the word (-AcA!ğ, -dI!ğ).
func tokenizeTemplate(template string) []suffixTemplateToken {
	rs := []rune(template)
	at := func(i int) string {
		if i < len(rs) {
			return string(rs[i])
		}
		return ""
	}
	var tokens []suffixTemplateToken
	for i := 0; i < len(rs); {
		ch := rs[i]
		i++
		switch ch {
		case '+':
			next := at(i)
			i++
			switch next {
			case "I":
				tokens = append(tokens, suffixTemplateToken{tokenIVowel, ""})
			case "A":
				tokens = append(tokens, suffixTemplateToken{tokenAVowel, ""})
			default:
				tokens = append(tokens, suffixTemplateToken{tokenAppend, next})
			}
		case '>':
			tokens = append(tokens, suffixTemplateToken{tokenDevoiceFirst, at(i)})
			i++
		case '~':
			tokens = append(tokens, suffixTemplateToken{tokenLastVoiced, at(i)})
			i++
		case '!':
			tokens = append(tokens, suffixTemplateToken{tokenLastNotVoiced, at(i)})
			i++
		case 'I':
			tokens = append(tokens, suffixTemplateToken{tokenIVowel, ""})
		case 'A':
			tokens = append(tokens, suffixTemplateToken{tokenAVowel, ""})
		default:
			tokens = append(tokens, suffixTemplateToken{tokenLetter, string(ch)})
		}
	}
	return tokens
}

// generateSurface is the surface of a tokenized suffix template after a predecessor with the phonetic bit mask
// phoneticAttrs; ok is false when the vowel harmony cannot be resolved (a vowelless root without a pronunciation).
func generateSurface(tokens []suffixTemplateToken, phoneticAttrs int) (sb string, ok bool) {
	attrs := phoneticAttrs
	for index, token := range tokens {
		if sb != "" {
			attrs = computePhoneticAttributes(sb, phoneticAttrs)
		}
		switch token.kind {
		case tokenLetter, tokenLastVoiced, tokenLastNotVoiced:
			sb += token.letter
		case tokenAVowel:
			if index == 0 && phoneticAttrs&paLastVowel != 0 {
				break
			}
			switch {
			case attrs&paVowelBack != 0:
				sb += "a"
			case attrs&paVowelFront != 0:
				sb += "e"
			default:
				return "", false
			}
		case tokenIVowel:
			if index == 0 && phoneticAttrs&paLastVowel != 0 {
				break
			}
			switch {
			case attrs&paVowelFront != 0 && attrs&paVowelUnround != 0:
				sb += "i"
			case attrs&paVowelBack != 0 && attrs&paVowelUnround != 0:
				sb += "ı"
			case attrs&paVowelBack != 0 && attrs&paVowelRound != 0:
				sb += "u"
			case attrs&paVowelFront != 0 && attrs&paVowelRound != 0:
				sb += "ü"
			default:
				return "", false
			}
		case tokenAppend:
			if attrs&paLastVowel != 0 {
				sb += token.letter
			}
		case tokenDevoiceFirst:
			if attrs&paLastVoiceless != 0 {
				sb += devoiceChar(token.letter)
			} else {
				sb += token.letter
			}
		}
	}
	return sb, true
}

// computeNextPhoneticAttrs is the phonetic bit mask after a suffix transition writes surface.
func computeNextPhoneticAttrs(surface string, prevAttrs int, lastToken *suffixTemplateToken) int {
	attrs := computePhoneticAttributes(surface, prevAttrs)
	attrs &^= paCannotTerminate
	if lastToken != nil {
		switch lastToken.kind {
		case tokenLastVoiced:
			attrs |= paExpectsConsonant
		case tokenLastNotVoiced:
			attrs |= paExpectsVowel | paCannotTerminate
		}
	}
	return attrs
}

// resolveSurfaceAndNextPhonetic is a transition's surface and the phonetic bit mask after it, from the
// transition's cache when its key was met before (analyzer.ts and fuzzy.ts each have this function).
func resolveSurfaceAndNextPhonetic(t *suffixTransition, phonetic int) *surfaceCacheEntry {
	key := surfaceCacheKey(phonetic)
	e := &t.cache[key]
	if !e.set {
		e.set = true
		generated, ok := generateSurface(t.tokens, phonetic)
		if ok {
			e.surface = generated
			e.runes = []rune(generated)
			e.nextPhonetic = int32(computeNextPhoneticAttrs(generated, phonetic, t.lastToken))
		} else {
			e.null = true
		}
	}
	if e.null {
		return nil
	}
	return e
}
