package spell

import (
	"regexp"
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/rules/turkish-rules.ts: the closed lists and the checks of Turkish spelling the
// corrector applies. Its regular expressions are the TypeScript's as RE2 says them; each one's character classes
// hold whole Turkish letters, which JavaScript (without the u flag) matches as the single UTF-16 units they are.

// Question are the question particle's forms, written apart from the word before them and following its vowels.
var Question = regexp.MustCompile(`^(m[iıuü])(y?[iıuü]m|s[iıuü]n|y?[iıuü]z|s[iıuü]n[iıuü]z|d[iıuü]r|d[iıuü]|ym[iıuü]ş|ys[ae]|yd[iıuü]|yken)?$`)

// Also is the conjunction "de/da".
var Also = regexp.MustCompile(`^(d[ae])$`)

// NoLocativeAfter are the endings of stems a locative can't attach to directly, so "-da/-de" after them must be the
// separate word "da/de". It is the fallback when the morphology is not loaded or does not know the stem.
var NoLocativeAfter = regexp.MustCompile(`(s[ıiuü]|s[ıiuü]n[aeıiuü]|l[ae]r[ıi]n[aeıiuü]|l[ae]r[ae]|m[ae]y[ıi]|yl[ae]|s[ae]l[ae]r|[ıiuü]ğ[ıiuü]|[dt][ae]|n[ıiuü]n|yor|m[ıiuü]ş|[ae]c[ae]k|[dt][ıiuü]|[dt][ıiuü]r|[dt][ıiuü]k|s[ae]|m[ae]l[ıi])$`)

// StrongNoLocativeAfter are the unambiguous non-noun-root endings for stems outside the morphology dictionary
// ("subtropiklerdede"). Bare "-da/-de/-sa/-se" are left out so compound nouns it does not know ("mikromaddede",
// "multisitede") are not split.
var StrongNoLocativeAfter = regexp.MustCompile(`(?:l[ae]rd[ae]|[ıiuü]nd[ae]|s[ıiuü]nd[ae]|s[ıiuü]n[aeıiuü]|l[ae]r[ıi]n[aeıiuü]|l[ae]r[ıi]|l[ae]r[ae]|m[ae]y[ıi]|yl[ae]|s[ae]l[ae]r|[ıiuü]ğ[ıiuü]|yor|m[ıiuü]ş|[ae]c[ae]k|m[ae]l[ıi])$`)

// Finite are the endings of finite verbs and the copula, which "ki" follows as a separate word ("diyor ki",
// "beceridir ki", "öyle ki").
var Finite = regexp.MustCompile(`(yor(?:um|sun|uz|sunuz|lar|du)?|m[ıiuü]ş(?:t[ıiuü]r)?|[ae]c[ae]k|[dt][ıiuü]|[dt][ıiuü]r|[dt][ıiuü]k|[dt][ıiuü]m|[dt][ıiuü]n|m[ae]l[ıi]|s[ae]|[ae]r(?:[ıi]m|l[ae]r)?|[ıiuü]r(?:[ıiuü]m|l[ae]r)?|öyle|böyle|şöyle)$`)

// NeverAlone are the suffixes that never stand alone: splitting one off a word is never a fix ("kmdir",
// "kitaplar").
var NeverAlone = setOf(
	"dir", "dır", "dur", "dür", "tir", "tır", "tur", "tür", "ler", "lar", "dı", "di", "du", "dü", "ti", "tı",
	"tu", "tü", "in", "ın", "un", "ün", "e", "a", "i", "ı", "u", "ü", "le", "la",
)

// JoinFirst are short words people often write apart from a word they belong with ("hiç bir", "bir çok",
// "her hangi").
var JoinFirst = setOf(
	"bir", "her", "hiç", "ya", "hep", "kimi", "öbür", "birbiri", "halbu", "oysa", "madem", "sanki", "meğer",
)

// JoinSecond are suffix-like words that belong on the word before when they are the locative or ablative ("ev de"
// for "evde").
var JoinSecond = setOf("de", "da", "te", "ta", "den", "dan", "ten", "tan", "ki", "deki", "daki", "teki", "taki")

// SombahcemiKi are the traditional SOMBAHÇEMİ forms written joined with "ki" (ambiguous or contested forms like
// "illa ki" left out).
var SombahcemiKi = setOf("oysa", "halbu", "madem", "meğer")

// TeTaSkip are function words after which a separate "te/ta" is almost always a devoiced conjunction ("çok ta" ->
// "çok da") or the adverb "ta".
var TeTaSkip = setOf(
	"çok", "yok", "pek", "hiç", "hep", "artık", "ancak", "fakat", "hem", "daha", "bile", "yine", "gene", "ve",
	"ama", "çünkü", "da", "de",
)

// NameSuffix are the suffixes the generic (unseen-form) apostrophe rule accepts after a clear proper name:
// genitive, locative, ablative, -deki/-daki, instrumental, copula, dative/accusative.
var NameSuffix = regexp.MustCompile(`^(?:n?[ıiuü]n|[dt][ae]|[dt][ae]n|[dt][ae]ki|y?l[ae]|[dt][ıiuü]r|y?[aeıiuü]|yd[ıiuü]|ym[ıiuü]ş)$`)

// acronymSuffix are the suffixes accepted after an all-caps acronym ("HEXIM'den", "KDA'ların", "TLS'nin"), always
// two letters or more, so English "-s" is not one.
var acronymSuffix = regexp.MustCompile(`^(?:l[ae]r(?:[ıiuü]n|[dt][ae]n?|[dt][ae]ki|l[ae]|[aeıiuü])?|n?[ıiuü]n|[dt][ae]n?|[dt][ae]ki|y?l[ae]|y[aeıiuü]|[dt][ıiuü]r)$`)

var acronymWord = regexp.MustCompile(`^([A-ZÇĞİÖŞÜa-zçğıöşü]*[A-ZÇĞİÖŞÜ]{2,})([a-zçğıöşü]{2,})$`)

// HonorificTitles are Turkish honorific titles that take an apostrophe after a proper name ("Aclan Bey'in",
// "Ayşe Hanım'a").
var HonorificTitles = setOf("bey", "hanım", "paşa", "efendi", "ağa")

// PlaceApostropheHeads is the closed list of TDK geographic, place, landmark and state head words (mostly P3sg
// forms) that take an apostrophe after a capitalized proper name ("Marmara Denizi'nin", "Türkiye Cumhuriyeti'nin",
// "Kadıköy Mahallesi'nde", "Büyük Okyanus'ta", "Osmanlı Devleti'nin"). Institution and organization head words
// ("Üniversitesi", "Partisi", "Bakanlığı", "Kurumu", ...) are left out on purpose.
var PlaceApostropheHeads = setOf(
	// Water bodies and coastal geography
	"denizi", "okyanusu", "okyanus", "gölü", "nehri", "ırmağı", "çayı", "deresi", "boğazı", "körfezi", "kanalı",
	"şelalesi", "adası", "yarımadası", "takımadası",
	// Landforms
	"dağı", "ovası", "yaylası", "platosu", "vadisi", "kanyonu", "çölü", "ormanı", "mağarası", "havzası", "kıtası",
	// Urban and administrative places
	"mahallesi", "caddesi", "sokağı", "bulvarı", "meydanı", "eyaleti", "oblastı", "bölgesi",
	// Historical landmarks
	"sarayı", "kalesi", "köprüsü", "kulesi",
	// States and polities
	"cumhuriyeti", "devleti", "imparatorluğu", "krallığı", "prensliği",
)

// PlaceP3sgSuffix are the suffixes after a P3sg vowel-final place or state head word (pronominal buffer "n", "y",
// or the copula).
var PlaceP3sgSuffix = regexp.MustCompile(`^(?:n[ıiuü]n|nd[ae]n?|nd[ae]ki|n[aeıiuü]|yl[ae]|[dt][ıiuü]r|yd[ıiuü]|ym[ıiuü]ş)$`)

// IsPlaceApostropheHead reports whether stem is a TDK geographic, place or state head word, or the explicit TDK
// exception "Avrupa Birliği" (prevWord is the word before, "" for none).
func IsPlaceApostropheHead(stem, prevWord string) bool {
	return PlaceApostropheHeads[stem] || (stem == "birliği" && prevWord == "avrupa")
}

var vowelFinalStem = regexp.MustCompile(`[aeıioöuüâîû]$`)
var bufferedSuffix = regexp.MustCompile(`^(?:n[ıiuü]n|y)`)

// IsValidPlaceApostrophe checks a suffix attached with an apostrophe to a TDK place, geographic or state head
// word.
func IsValidPlaceApostrophe(stem, suffix string) bool {
	if vowelFinalStem.MatchString(stem) {
		if !PlaceP3sgSuffix.MatchString(suffix) {
			return false
		}
	} else {
		if !NameSuffix.MatchString(suffix) {
			return false
		}
		if bufferedSuffix.MatchString(suffix) {
			return false
		}
		if strings.HasPrefix(suffix, "d") && IsVoicelessFinal(stem) {
			return false
		}
		if strings.HasPrefix(suffix, "t") && !IsVoicelessFinal(stem) {
			return false
		}
	}
	if firstVowel := firstOf(suffix, "aeıioöuü"); firstVowel != "" && !FollowsHarmony(stem, firstVowel) {
		return false
	}
	return true
}

// firstOf is the first letter of text that is one of letters, or "".
func firstOf(text, letters string) string {
	for _, r := range text {
		if strings.ContainsRune(letters, r) {
			return string(r)
		}
	}
	return ""
}

// CompoundVerbJoinRule is how a TDK compound verb that is always written joined (for a dropped vowel, a doubled
// consonant, or lexicalization) is joined: the first word becomes prefix before the auxiliary verb auxStem.
type CompoundVerbJoinRule struct {
	AuxStem        string
	Prefix         string
	StrictZeroPair bool
}

// CompoundVerbJoins are the compound verbs written joined in every conjugation, by their separate first word:
// "vaz" + "geçtim" -> "vazgeçtim", "kayıp" + "ediyorum" -> "kaybediyorum", "his" + "ettik" -> "hissettik".
var CompoundVerbJoins = map[string][]CompoundVerbJoinRule{
	"vaz":   {{AuxStem: "geç", Prefix: "vaz"}},
	"baş":   {{AuxStem: "vur", Prefix: "baş", StrictZeroPair: true}},
	"var":   {{AuxStem: "say", Prefix: "var", StrictZeroPair: true}},
	"ön":    {{AuxStem: "gör", Prefix: "ön", StrictZeroPair: true}},
	"alı":   {{AuxStem: "koy", Prefix: "alı"}},
	"devir": {{AuxStem: "al", Prefix: "devr"}, {AuxStem: "et", Prefix: "devr"}},
	"af":    {{AuxStem: "et", Prefix: "aff"}},
	"his":   {{AuxStem: "et", Prefix: "hiss"}},
	"red":   {{AuxStem: "et", Prefix: "redd"}},
	"ret":   {{AuxStem: "et", Prefix: "redd"}},
	"hal":   {{AuxStem: "et", Prefix: "hall"}},
	"zan":   {{AuxStem: "et", Prefix: "zann"}},
	"kayıp": {{AuxStem: "et", Prefix: "kayb"}, {AuxStem: "ol", Prefix: "kayb"}},
	"kayıt": {{AuxStem: "et", Prefix: "kayd"}, {AuxStem: "ol", Prefix: "kayd"}},
	"seyir": {{AuxStem: "et", Prefix: "seyr"}},
	"emir":  {{AuxStem: "et", Prefix: "emr"}},
	"hapis": {{AuxStem: "et", Prefix: "haps"}},
	"nakil": {{AuxStem: "et", Prefix: "nakl"}},
	"keşif": {{AuxStem: "et", Prefix: "keşf"}},
	"sabır": {{AuxStem: "et", Prefix: "sabr"}},
	"şükür": {{AuxStem: "et", Prefix: "şükr"}},
	"bahis": {{AuxStem: "et", Prefix: "bahs"}},
	"fetih": {{AuxStem: "et", Prefix: "feth"}},
	"mahv":  {{AuxStem: "et", Prefix: "mahv"}, {AuxStem: "ol", Prefix: "mahv"}},
	"vaat":  {{AuxStem: "et", Prefix: "vad"}},
	"def":   {{AuxStem: "et", Prefix: "def"}},
	"men":   {{AuxStem: "et", Prefix: "men"}},
}

// JoinedEtmekStems are the stems whose compound with "etmek" changes its sound and is written joined, not split
// ("hisettiniz" -> "hissettiniz").
var JoinedEtmekStems = setOf(
	"af", "his", "red", "ret", "hal", "zan", "kayıp", "kayıt", "seyir", "emir", "hapis", "nakil", "keşif",
	"devir", "sabır", "şükür", "bahis", "fetih", "mahv", "vaat", "def", "men", "kat",
)

// SeparateEtmekShort are the three-letter noun stems that form compound verbs with "etmek" written apart, per TDK.
var SeparateEtmekShort = setOf("hak", "arz", "yok", "söz", "not", "göç", "alt")

// SeparateEtmekBareImperative are the common stems that may be split before a bare two-letter imperative "et"
// ("farket" -> "fark et", "meraket" -> "merak et").
var SeparateEtmekBareImperative = setOf(
	"fark", "terk", "hak", "park", "devam", "dikkat", "merak", "kabul", "iptal", "kontrol", "takip", "yardım",
	"rica", "teşekkür",
)

// TimeNamesNoStartApostrophe are month and weekday names, which alone take no apostrophe at the start of a
// sentence ("Ekimde havalar soğur").
var TimeNamesNoStartApostrophe = setOf(
	"ocak", "şubat", "mart", "nisan", "mayıs", "haziran", "temmuz", "ağustos", "eylül", "ekim", "kasım",
	"aralık", "pazartesi", "salı", "çarşamba", "perşembe", "cuma", "cumartesi", "pazar",
)

// MusicNotes and MusicAfter are the note names and the words after them that keep a standalone "mi" from being
// rewritten to "mı".
var (
	MusicNotes = setOf("do", "re", "mi", "fa", "sol", "la", "si")
	MusicAfter = setOf("majör", "minör", "major", "minor", "bemol", "diyez", "notası", "notasi", "anahtarı", "anahtari")
)

// backVowels, frontVowels and roundVowels are the Turkish vowels by their harmony classes.
const (
	backVowels  = "aıouâû"
	frontVowels = "eiöüî"
	roundVowels = "ouöüû"
)

func isBack(v string) bool  { return len([]rune(v)) == 1 && strings.Contains(backVowels, v) }
func isFront(v string) bool { return len([]rune(v)) == 1 && strings.Contains(frontVowels, v) }
func isRound(v string) bool { return len([]rune(v)) == 1 && strings.Contains(roundVowels, v) }

// LastVowel is the last vowel of word, or "" when it has none.
func LastVowel(word string) string {
	rs := []rune(word)
	for i := len(rs) - 1; i >= 0; i-- {
		if strings.ContainsRune(backVowels+frontVowels, rs[i]) {
			return string(rs[i])
		}
	}
	return ""
}

// FollowsHarmony reports whether a two-way (e/a) or four-way (i/ı/u/ü) vowel follows the stem's last vowel.
func FollowsHarmony(stem, vowel string) bool {
	last := LastVowel(stem)
	if last == "" {
		return true
	}
	back := isBack(last)
	if vowel == "a" || vowel == "e" {
		return back == (vowel == "a")
	}
	return vowel == harmonicHigh(back, isRound(last))
}

// harmonicHigh is the high vowel that follows a back or front, round or unrounded vowel.
func harmonicHigh(back, round bool) string {
	switch {
	case back && round:
		return "u"
	case back:
		return "ı"
	case round:
		return "ü"
	}
	return "i"
}

// SameHighVowels reports whether a particle keeps one high vowel throughout: "misiniz", "mıydı" and "müsün" do;
// "misın" doesn't exist.
func SameHighVowels(particle string) bool {
	first := rune(-1)
	for _, r := range particle {
		if strings.ContainsRune("ıiuü", r) {
			if first == -1 {
				first = r
			} else if r != first {
				return false
			}
		}
	}
	return true
}

var voicelessFinal = regexp.MustCompile(`[fstkçşhp]$`)

// IsVoicelessFinal reports whether a lowercase word ends in a Turkish voiceless consonant (f, s, t, k, ç, ş, h, p).
func IsVoicelessFinal(word string) bool { return voicelessFinal.MatchString(word) }

var (
	blockedP3sg        = regexp.MustCompile(`[^aeıioöuü](s?[ıiuü])$`)
	blockedPlural      = regexp.MustCompile(`l[ae]r$`)
	blockedBackPlural  = regexp.MustCompile(`[aıouâû][^aeıioöuü]*lar(?:ın(?:dan?|a|ı)?|ıyla|dan?|a|la)$`)
	blockedFrontPlural = regexp.MustCompile(`[eiöüî][^aeıioöuü]*ler(?:in(?:den?|e|i)?|iyle|den?|e|le)$`)
	blockedBareCa      = regexp.MustCompile(`[cç][ae]$`)
	blockedCaInflected = regexp.MustCompile(`[cç][ae](?:d[ae]n?|n[ıi]n|y[aeıi]|yl[ae])$`)
)

// HasBlockedApostropheStem reports whether a stem takes no apostrophe by TDK rules in generic or boosted inference:
// possessive-ending institution and place stems ("Cumhuriyetinde", "Bakanlığından"), plural stems and inflected
// plurals ("Türklerin", "Müslümanlara", "Japonların", "Yunanlardan"), and long derived language or manner stems in
// -ca/-ce/-ça/-çe with their inflections ("Hırvatçadan", "Türkçeden"). The TypeScript's defaults for the flags are
// false.
func HasBlockedApostropheStem(stem string, skipBareCa, allowPlaceHead bool) bool {
	if !allowPlaceHead && blockedP3sg.MatchString(stem) {
		return true
	}
	if blockedPlural.MatchString(stem) {
		return true
	}
	if blockedBackPlural.MatchString(stem) {
		return true
	}
	if blockedFrontPlural.MatchString(stem) {
		return true
	}
	n := runeLen(stem)
	if !skipBareCa && n >= 6 && blockedBareCa.MatchString(stem) {
		return true
	}
	if n >= 8 && blockedCaInflected.MatchString(stem) {
		return true
	}
	return false
}

var (
	vowelOrYFinal     = regexp.MustCompile(`[aeıioöuüâîûy]$`)
	vowelInitial      = regexp.MustCompile(`^[aeıiuü]`)
	vowelFinalNotE    = regexp.MustCompile(`[aıioöuüâîû]$`)
	voicelessFinalExt = regexp.MustCompile(`[fstkçşhpcx]$`)
)

// IsValidGenericApostrophe is the phonological and TDK checks for attaching an unseen suffix to a proper name with
// an apostrophe, which turn down impossible splits like "Ankara'in", "Mark'ya", "Zeynep'den" or "Müslümanlar'a".
// The TypeScript's default for skipBareCa is false.
func IsValidGenericApostrophe(stem, suffix string, skipBareCa bool) bool {
	if !NameSuffix.MatchString(suffix) {
		return false
	}
	if HasBlockedApostropheStem(stem, skipBareCa, false) {
		return false
	}
	// The buffer "n" (genitive -nın/-nin/-nun/-nün) and the buffer "y" (-ya/-ye/-yı/-yle/-ydı/-ymış) follow a vowel
	// or "y".
	if bufferedSuffix.MatchString(suffix) && !vowelOrYFinal.MatchString(stem) {
		return false
	}
	// Vowel-initial suffixes follow a consonant, or the silent final "-e" of foreign names ("Google'ın", "Blake'i").
	if vowelInitial.MatchString(suffix) && vowelFinalNotE.MatchString(stem) {
		return false
	}
	// Voicing: a voiceless final takes "t-", a voiced or vowel final "d-", in the locative, ablative and copula.
	if strings.HasPrefix(suffix, "d") && IsVoicelessFinal(stem) {
		return false
	}
	if strings.HasPrefix(suffix, "t") && !voicelessFinalExt.MatchString(stem) {
		return false
	}
	return true
}

// AcronymApostrophe is an acronym glued to a Turkish suffix, and the word with the apostrophe put back.
type AcronymApostrophe struct {
	Prefix, Suffix, WithMark string
}

// MatchAcronymApostrophe matches an all-caps acronym or a mixed-case tech or brand name ending in two or more
// capitals, glued to a Turkish suffix ("HEXIMden", "MacOSdaki").
func MatchAcronymApostrophe(typed string) (AcronymApostrophe, bool) {
	match := acronymWord.FindStringSubmatch(typed)
	if match == nil {
		return AcronymApostrophe{}, false
	}
	prefix, suffix := match[1], match[2]
	if runeLen(prefix) < 3 || !acronymSuffix.MatchString(suffix) {
		return AcronymApostrophe{}, false
	}
	return AcronymApostrophe{Prefix: prefix, Suffix: suffix, WithMark: tr.Lower(prefix) + "'" + suffix}, true
}

var questionVowel = regexp.MustCompile(`^m[iıuü]$`)

// IsValidKnownParticleSuffix guards the suffixing of multi-word particle rules in KnownMisspellings ("kimmi ->
// kim mi", "yada -> ya da"): rules ending in "de", "da", "ki" never take trailing letters ("yadacı" is not
// "ya dacı"); rules ending in "mi/mı/mu/mü" take only suffixes that keep the last word a valid harmonized question
// particle ("varmısın" -> "var mısın", but not "kimmiş" -> "kim miş" or "varmış" -> "var mış").
func IsValidKnownParticleSuffix(baseRight, suffix string) bool {
	space := strings.LastIndexByte(baseRight, ' ')
	if space == -1 {
		return true
	}
	baseLast := baseRight[space+1:]
	if baseLast == "de" || baseLast == "da" || baseLast == "ki" {
		return false
	}
	if questionVowel.MatchString(baseLast) {
		combined := baseLast + suffix
		return Question.MatchString(combined) && SameHighVowels(combined)
	}
	return true
}

var consonantY = regexp.MustCompile(`[^aeıioöuüâîû]y$`)

// IsHarmoniousApostropheMarkChange reports whether every vowel changed inside an apostrophe suffix ("i" -> "ı",
// "u" -> "ü", "o" -> "ö") harmonizes with the vowel before it, starting from the stem's last vowel. It turns down
// "Mortimer'i" -> "Mortimer'ı" and keeps "Ravenna'nin" -> "Ravenna'nın". The TypeScript's default for isInverseStem
// is false.
func IsHarmoniousApostropheMarkChange(formStem, wordSuffix, formSuffix string, isInverseStem bool) bool {
	if wordSuffix == formSuffix {
		return true
	}
	before, after := []rune(wordSuffix), []rune(formSuffix)
	if len(before) != len(after) {
		return true
	}
	for i := range after {
		if before[i] == after[i] {
			continue
		}
		a := string(after[i])
		if !isBack(a) && !isFront(a) {
			continue
		}
		suffixVowel := LastVowel(string(after[:i]))
		var prev string
		inverse := false
		switch {
		case suffixVowel != "":
			prev = suffixVowel
		case consonantY.MatchString(formStem):
			// A final syllabic "-y" in foreign names ("Mary", "Tony", "Lucy", "Murphy") is a front "/i/".
			prev = "i"
		default:
			prev = LastVowel(formStem)
			inverse = isInverseStem
		}
		if prev == "" {
			continue
		}
		back := !inverse && isBack(prev)
		if a == "o" || a == "ö" {
			if back != (a == "o") {
				return false
			}
			continue
		}
		if a != harmonicHigh(back, isRound(prev)) {
			return false
		}
	}
	return true
}

var (
	doubledFront = regexp.MustCompile(`[dt]eler$`)
	doubledBack  = regexp.MustCompile(`[dt]alar$`)
)

// CollapsedDoubledLocative finds a doubled locative typed around the plural suffix ("bölgedelerde" for
// "bölgelerde", "alandalarda" for "alanlarda") and gives the collapsed locative plural.
func CollapsedDoubledLocative(first, second string) (string, bool) {
	if second == "de" && doubledFront.MatchString(first) && runeLen(first) >= 7 {
		return sliceRunes(first, 0, -5) + "lerde", true
	}
	if second == "da" && doubledBack.MatchString(first) && runeLen(first) >= 7 {
		return sliceRunes(first, 0, -5) + "larda", true
	}
	return "", false
}

var validApostropheBackSuffix = regexp.MustCompile(`^(?:(?:lar)?(?:s?ı)?(?:n?ın|n?[dt]an?|n?[dt]aki|n?a|n?ı|y?la)|y[aı])$`)

// FixDisharmoniousApostropheSuffix restores an unambiguous back-vowel harmony inside an apostrophe suffix typed
// without Turkish letters: "ukrayna'sinda" -> "ukrayna'sında", "ankara'sindaki" -> "ankara'sındaki", "kda'larin"
// -> "kda'ların". It never changes a trailing "-ki" and never guesses harmony from an unseen foreign stem.
func FixDisharmoniousApostropheSuffix(word string) (string, bool) {
	quote := strings.IndexByte(word, '\'')
	if quote == -1 || runeLen(word[:quote]) < 2 || quote != strings.LastIndexByte(word, '\'') {
		return "", false
	}
	stem := word[:quote]
	suffix := word[quote+1:]
	keepKi := strings.HasSuffix(suffix, "ki")
	core := suffix
	if keepKi {
		core = suffix[:len(suffix)-2]
	}
	if !strings.Contains(core, "a") || !strings.Contains(core, "i") {
		return "", false
	}
	fixedSuffix := strings.ReplaceAll(core, "i", "ı")
	if keepKi {
		fixedSuffix += "ki"
	}
	if fixedSuffix == suffix || !validApostropheBackSuffix.MatchString(fixedSuffix) {
		return "", false
	}
	if !IsHarmoniousApostropheMarkChange(stem, suffix, fixedSuffix, false) {
		return "", false
	}
	return stem + "'" + fixedSuffix, true
}

// HarmonizeQuestionParticle restores the missing Turkish letters ("i" -> "ı", "u" -> "ü") of a standalone
// question particle ("mi", "mu", "misin", ...) in a sentence typed without them, by the vowel harmony of the word
// before, which may be an inverse-harmony loanword like "saat" or "alkol".
func HarmonizeQuestionParticle(word, prevWord string, isInverseHarmony bool) (string, bool) {
	match := Question.FindStringSubmatch(word)
	if match == nil || !SameHighVowels(word) {
		return "", false
	}
	last := LastVowel(prevWord)
	if last == "" {
		return "", false
	}
	round := isRound(last)
	front := isInverseHarmony || isFront(last)
	want := harmonicHigh(!front, round)
	current := string([]rune(match[1])[1])
	if current == want {
		return "", false
	}
	// Only missing letters are restored (ASCII -> Turkish); letters are never stripped, nor other vowels guessed.
	if !(current == "i" && want == "ı" || current == "u" && want == "ü") {
		return "", false
	}
	target := strings.ReplaceAll(word, current, want)
	if Question.MatchString(target) && SameHighVowels(target) {
		return target, true
	}
	return "", false
}

var participleEnd = regexp.MustCompile(`(?:[ıiuü]?yor|m[ıiuü]ş|[ae]c[ae]k)$`)

// IsAuxiliaryOl reports whether "ol-" is the helper verb after a participle ("yardım ediyor oluyorduk", "gelmiş
// olacak", "gidecek olursa"): there it is never "öl-", however common "ölüyorduk" is on its own.
func IsAuxiliaryOl(prevWord, word, replacement string) bool {
	return strings.HasPrefix(word, "ol") && strings.HasPrefix(replacement, "öl") && participleEnd.MatchString(prevWord)
}
