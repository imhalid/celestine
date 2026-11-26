package spell

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/sentence.ts: sentence punctuation, TDK proper-name capitals, multi-word phrases,
// titles, dates, missing spaces after full stops, and apostrophes put in, taken out or moved. It is careful on clean
// text: abbreviations ("Dr.", "vb."), ordinals and decimals ("3. sınıf", "3.5", "09.15'te"), initials ("A."),
// ellipses ("..."), addresses, e-mails and file names are left as they are. Offsets are in runes, the TypeScript's
// UTF-16 units for Turkish text.

// AmbigCandidateQuery is an unresolved ambiguous proper-name occurrence, passed to an optional disambiguator that
// reads its context.
type AmbigCandidateQuery struct {
	Start         int
	End           int
	Base          string
	DisplayBase   string
	Suffix        string
	HadApostrophe bool
	AtStart       bool
	IsGiven       bool
	CommonCand    string
	ProperCand    string
	LeftRaw       string
	RightRaw      string
}

// SentenceResources are the lexical and morphological resources the corrector gives the full TDK proper-name
// rules. A nil function or pointer is a resource not given.
type SentenceResources struct {
	Known        *KnownMisspellings
	CapitalShare func(lower string) float64
	Count        func(lower string) float64
	Pair         func(first, second string) float64
	IsMorphWord  func(lower string) bool
	// DisambiguateAmbig is the optional disambiguator for ambiguous names (a core linear model or pre-scored
	// neural context): the chosen text, or false to leave the name to the rules.
	DisambiguateAmbig func(query AmbigCandidateQuery) (string, bool)
	// CollectAmbig, when given, receives every unresolved ambiguous name query, for the corrector to score them
	// together with the context model.
	CollectAmbig *[]AmbigCandidateQuery
}

// SentenceToken is a word or a number of the text as the proper-name rules see it, with the form the corrections
// so far give it.
type SentenceToken struct {
	Start    int
	End      int
	Raw      string
	Eff      string
	Low      string
	IsWord   bool
	IsNumber bool
	// SepBefore is the separator between the previous token and this one, without white space.
	SepBefore string
	// GapBefore is the text between the previous token and this one.
	GapBefore string
	// GapAfter is the text between this token and the next.
	GapAfter string
	AtStart  bool
	Locked   bool
}

// Abbreviations are common Turkish (and a few English) abbreviations written with a full stop.
var Abbreviations = setOf(
	"vb", "vs", "vd", "dr", "prof", "doç", "doc", "yrd", "öğr", "ogr", "gör", "gor", "av", "op", "uzm", "örn", "orn",
	"bkz", "bk", "krş", "krs", "yy", "no", "sn", "st", "tel", "cad", "sok", "mah", "apt", "s", "sf", "çev", "cev",
	"haz", "ed", "der", "alm", "ing", "fr", "lat", "ar", "far", "tic", "ltd", "şti", "sti", "aş", "as", "inc", "co",
	"mr", "mrs", "ms", "jr", "sr", "etc", "eg", "ie", "vol", "fig", "ca", "ör", "or", "mö", "bnb", "hz", "san",
	"bul", "blv", "km", "cm", "mm", "kg", "gr", "lt", "ml", "sa", "dk", "mhz", "ghz", "khz", "kb", "mb", "gb",
	"subsp", "sp", "spp", "var", "pav", "nr", "yyl", "ss", "pp", "tr", "ital", "bros",
	"müh", "muh", "mim", "ecz", "dt", "vet", "arb",
)

// NotASentenceStart are file extensions and top-level domains: "dosya.pdf" and "site.com" are not two sentences.
var NotASentenceStart = setOf(
	"com", "net", "org", "edu", "gov", "mil", "int", "io", "ai", "dev", "app", "tr", "uk", "us", "info", "biz", "xyz",
	"html", "htm", "js", "mjs", "ts", "tsx", "jsx", "css", "scss", "json", "md", "txt", "pdf", "doc", "docx", "xls",
	"xlsx", "ppt", "pptx", "odt", "rtf", "png", "jpg", "jpeg", "gif", "svg", "webp", "ico", "mp3", "mp4", "wav", "mov",
	"avi", "mkv", "zip", "rar", "gz", "tar", "exe", "dmg", "apk", "py", "java", "kt", "swift", "rs", "go", "rb", "php",
	"sh", "sql", "xml", "csv", "yml", "yaml", "toml", "lock", "log", "env", "ini", "cfg",
)

// almanVerbalNext are the verbal-noun predicate words that follow a lowercase "alman" ("alman gerek", "alman daha
// iyi olur").
var almanVerbalNext = setOf(
	"gerek", "gerekiyor", "gerekir", "gerekecek", "gerekti", "gerekirdi", "lazım", "şart",
	"zorunlu", "mümkün", "daha", "iyi", "doğru", "yanlış", "saçma", "mantıklı", "için",
	"halde", "durumunda", "mı", "mi", "bile", "da", "de", "olur", "olacak", "önemli",
)

// aleviSectCtx are the religious and social context words near "alevi" that tell it from "alev" + "i" ("its
// flame").
var aleviSectCtx = setOf(
	"sünni", "şii", "bektaşi", "aile", "aileler", "köy", "köyü", "köyde", "mahalle", "inanç",
	"inancı", "kültür", "kültürü", "toplum", "toplumu", "vatandaş", "cemevi", "cem", "dede",
	"mezhep", "mezhebi", "gelenek", "geleneği", "halk", "türkmen", "islam", "müslüman",
)

// humanPossessedNext are the words that show a human possessor right after an ambiguous name's genitive ("Bahar'ın
// annesi", "Kaya'nın masasına").
var humanPossessedNext = setOf(
	"annesi", "annesini", "annesine", "annesinde", "annesinden", "annesinin", "annesiyle",
	"babası", "babasını", "babasına", "babasında", "babasından", "babasının", "babasıyla",
	"ablası", "ablasını", "ablasına", "abisi", "abisini", "abisine", "ağabeyi",
	"kardeşi", "kardeşini", "kardeşine", "kardeşiyle", "eşi", "karısı", "kocası", "oğlu", "kızı",
	"evi", "evini", "evine", "evinde", "evinden", "evinin",
	"ofisi", "ofisini", "ofisine", "ofisinde", "ofisinden",
	"masası", "masasını", "masasına", "masasında", "masasından",
	"arabası", "arabasını", "arabasına", "arabasında", "arabasıyla",
	"telefonu", "telefonunu", "numarası", "numarasını", "düğünü", "düğününe", "düğününü",
	"doğum", "ödevi", "ödevini", "sınav", "sonucu", "şarkıları", "şarkılarını", "romanları",
	"romanlarını", "şiirleri", "şiirlerini", "kitabı", "kitabını", "eserleri", "eserlerini",
	"bisikleti", "bisikletini", "çantası", "çantasını", "defteri", "defterini",
	"kalemi", "gözlüğü", "bilgisayarı", "işi", "işini", "yeni",
)

// singleWordWorks are single-word famous literary titles that take a capital and an apostrophe after an author's
// genitive ("Tanpınar'ın Huzur'unu okumak").
var singleWordWorks = NewWordSet(
	"huzur", "yaban", "eylül", "çalıkuşu", "tutunamayanlar", "sefiller", "dava", "şato",
	"dönüşüm", "veba", "körlük", "simyacı", "satranç", "yabancı", "kumarbaz", "budala",
	"ecinniler", "diriliş", "obligomov", "oblomov", "goriot", "emma", "dracula", "frankenstein",
)

// singleWordPublications are single-word famous Turkish newspapers and magazines before "dergisi" or "gazetesi"
// ("Varlık dergisine").
var singleWordPublications = setOf(
	"varlık", "cumhuriyet", "hürriyet", "milliyet", "sabah", "sözcü", "posta", "zaman",
	"birgün", "evrensel", "karar", "akşam", "güneş", "takvim", "yeniçağ", "aydınlık",
	"notos", "hece", "dergâh", "dergah", "kafa", "bavul", "ot", "uykusuz", "penguen",
	"leman", "atlas", "papirüs", "hisar", "yön", "kadro", "markopaşa", "akbaba", "millî",
)

// commonObjectAfterSurnameVerbs are the verbs after which a common word is never capitalized as a surname after a
// first name.
var commonObjectAfterSurnameVerbs = setOf(
	"yaktı", "yakıyor", "yakmış", "yaksın", "attı", "atıyor", "atmış", "topladı", "topluyor",
	"aldı", "alıyor", "gördü", "görüyor", "yedi", "yiyor", "içti", "içiyor", "açtı", "açıyor",
	"taktı", "takıyor", "giydi", "giyiyor", "sürdü", "sürüyor", "kırdı", "kırıyor", "kesti", "kesiyor",
	"döktü", "döküyor", "çizdi", "çiziyor", "yazdı", "yazıyor", "okudu", "okuyor", "söyledi", "söylüyor",
	"verdi", "veriyor", "tuttu", "tutuyor", "buldu", "buluyor", "dikti", "dikiyor",
)

// veryCommonSurnameCollisions are common Turkish words that are also surnames, as nouns, adjectives or verbs, and
// need extra care after a first name.
var veryCommonSurnameCollisions = setOf(
	"ak", "al", "ay", "can", "er", "gün", "kan", "kır", "ok", "su", "bal", "baş", "dağ", "dal",
	"gök", "gül", "taş", "tok", "ulu", "boz", "sarı", "kara", "uzun", "kısa", "büyük", "küçük",
	"genç", "yeni", "eski", "iyi", "güzel", "doğru", "açık", "kapalı", "sert", "kuru", "özel",
	"temel", "uyar", "uçar", "gezer", "sever", "okur", "keser", "yazar", "bakar", "tutar",
	"gider", "gelir", "bilir", "kalır", "alır", "verir", "eder", "olur", "döner", "düşer",
	"ateş", "çiçek", "güneş", "bulut", "deniz", "toprak", "yıldız", "kaya", "demir", "çelik",
	"altın", "gümüş", "bakır", "tunç", "ipek", "şeker", "ekşi", "tatlı", "kuzu", "kurt",
	"aslan", "kaplan", "kartal", "şahin", "doğan", "ceylan", "tavşan", "koç", "kılıç",
	"kalkan", "bayrak", "sancak", "uçak", "kalem", "kitap", "duman", "rüzgar", "yağmur",
	"bahar", "eylül", "nisan", "mayıs", "ocak", "bayram", "barış", "savaş", "zafer", "umut",
	"sevgi", "özlem", "mutlu", "güven", "erdem", "onur", "ufuk", "huzur", "varlık", "öykü",
	"masal", "rüya", "hayal", "neşe", "sevinç", "kader", "melek", "nur", "pınar", "ırmak",
	"nehir", "dere", "göl", "orman", "çınar", "meşe", "lale", "menekşe", "kiraz", "elmas",
	"inci", "mercan", "yakut", "sedef", "zengin", "fakir", "yalnız", "çalışkan", "sağlam",
	"parlak", "keskin", "önder", "öner", "sayın", "seçkin", "bilgin", "kahraman", "usta",
	"terzi", "çoban", "avcı", "balcı", "bağcı", "yazıcı", "yapıcı", "topçu", "dalgıç",
	"kalaycı", "ekinci", "sütçü", "tütüncü", "çiftçi", "köse", "topal", "koca", "kocaman",
	"torun", "kardeş", "arkadaş", "komşu", "misafir", "yoldaş", "yurt", "yayla", "kural",
	"metin", "saygın", "coşkun", "ergin", "yetkin", "sezer", "güler", "tekin", "şen", "tan",
	"oral", "arslan", "yılmaz", "korkmaz", "durmaz", "sönmez", "dönmez", "ölmez", "solmaz",
	"söylemez", "yüksel", "uyan", "uyanık", "uslu", "yaman", "yanık", "yeşil", "kızıl", "mavi",
	"üstün", "ünlü", "uğurlu", "varol", "vural", "yaşar", "yavuz", "yurtsever", "başar",
	"gündüz", "gürbüz", "kaymak", "pala", "tarhan", "tosun", "yörük", "dursun", "poyraz",
)

// commonGivenAdjectives are common Turkish adjectives, months and nouns among the given names that must not
// trigger alone after a relationship word ("kızım mutlu oldu").
var commonGivenAdjectives = setOf(
	"mutlu", "zeki", "emin", "adil", "aziz", "galip", "halis", "kamil", "kudret", "mahir",
	"metin", "nadir", "nezih", "önder", "sadık", "saygın", "seçkin", "servet", "şeref", "şerif",
	"taşkın", "temel", "ulvi", "yaman", "yücel", "yüksel", "arzu", "duygu", "eylem", "gaye",
	"gizem", "gözde", "imge", "kıymet", "pembe", "seher", "şirin", "yaprak", "yeter", "adalet",
	"bayram", "cuma", "eylül", "nisan", "kasım", "ramazan", "devrim", "savaş", "zafer", "sultan",
	"barış", "umut", "sevgi", "özlem", "güven", "inanç", "huzur", "neşe", "sevinç", "hayal",
	"rüya", "masal", "öykü", "doğa", "ışık", "güneş", "bulut", "yağmur", "toprak", "çiçek",
	"gül", "bahar", "deniz", "can", "kaya", "demir", "çelik", "ateş", "yıldız", "kader",
	"nur", "ipek", "evren", "pınar", "gönül", "aslı", "cihan", "saadet", "defne", "doruk",
)

// fallbackNameSuffixes are the suffixes the minimal fallback (without resources, for unit tests) accepts.
var fallbackNameSuffixes = setOf(strings.Split(
	"a e ya ye ı i u ü yı yi yu yü da de ta te dan den tan ten ın in un ün nın nin nun nün yın yin yun yün "+
		"la le yla yle lı li lu lü ca ce ça çe cı ci cu cü daki deki taki teki dakı dır dir dur dür tır tir tur tür "+
		"sı si su sü sın sin sun sün sına sine "+
		"nda nde ndan nden na ne nı ni nu nü yken iken ydı ydi ydu ydü", " ")...)

var (
	noApostropheName = regexp.MustCompile(`(?:ca|ce|ça|çe|lı|li|lu|lü|cı|ci|cu|cü|çı|çi|çu|çü)$`)
	lowerInitial     = regexp.MustCompile(`^[a-zçğıöşüâîûäëïóúñ]`)
	roman            = regexp.MustCompile(`^[IVXLCDM]+$`)
	// readingCtxRE are the verbs and nouns that show a literary work after "<Author>'ın <Work>".
	readingCtxRE = regexp.MustCompile(`^(?:oku|bitir|yaz|incele|beğen|sev|anlat|kitap|roman|eser|yayım|basıl|çevir|bırak|başla)`)
)

// isLetterRune is JavaScript's /\p{L}/u on one character; r is -1 past the text's ends.
func isLetterRune(r rune) bool { return r >= 0 && unicode.IsLetter(r) }

// isUpperRune is /\p{Lu}/u on one character.
func isUpperRune(r rune) bool { return r >= 0 && unicode.Is(unicode.Lu, r) }

// isWordChar is WORD_CHAR, /[\p{L}\p{N}'’]/u, on one character.
func isWordChar(r rune) bool {
	return r >= 0 && (unicode.IsLetter(r) || unicode.IsNumber(r) || r == '\'' || r == '’')
}

// isJSSpace is /\s/u on one character.
func isJSSpace(r rune) bool { return r >= 0 && jsWhitespace(r) }

// hasUpper is /\p{Lu}/u.test(s).
func hasUpper(s string) bool { return strings.IndexFunc(s, isUpperRune) >= 0 }

// hasNumber is /\p{N}/u.test(s).
func hasNumber(s string) bool { return strings.IndexFunc(s, unicode.IsNumber) >= 0 }

// startsUpper is /^\p{Lu}/u.test(s).
func startsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return s != "" && isUpperRune(r)
}

// runeAt is text[i] as a rune, or -1 (undefined) outside the text.
func runeAt(rs []rune, i int) rune {
	if i < 0 || i >= len(rs) {
		return -1
	}
	return rs[i]
}

// runeSlice is String.prototype.slice over runes.
func runeSlice(rs []rune, start, end int) string {
	n := len(rs)
	clamp := func(i int) int {
		if i < 0 {
			i += n
			if i < 0 {
				i = 0
			}
		}
		if i > n {
			i = n
		}
		return i
	}
	start, end = clamp(start), clamp(end)
	if start >= end {
		return ""
	}
	return string(rs[start:end])
}

// runeIndex is String.prototype.indexOf(sub) in runes, or -1.
func runeIndex(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:i])
}

// capitalize gives word a capital first letter, the Turkish way.
func capitalize(word string) string {
	if word == "" {
		return word
	}
	r, size := utf8.DecodeRuneInString(word)
	return tr.Upper(string(r)) + word[size:]
}

// wordEnd is the end of the word that starts at start.
func wordEnd(rs []rune, start int) int {
	end := start
	for end < len(rs) && isWordChar(rs[end]) {
		end++
	}
	return end
}

// wordStart is the start of the word that ends at end.
func wordStart(rs []rune, end int) int {
	start := end
	for start > 0 && isWordChar(rs[start-1]) {
		start--
	}
	return start
}

// continuesAsAddress reports whether the text after end goes on as an address, a path or code.
func continuesAsAddress(rs []rune, end int) bool {
	next := runeAt(rs, end)
	if next < 0 {
		return false
	}
	if strings.ContainsRune("@/\\:_=#&?%", next) && next != '?' {
		return true
	}
	if next == '?' && isLetterRune(runeAt(rs, end+1)) {
		return true
	}
	return (next == '.' || next == '-') && isWordChar(runeAt(rs, end+1))
}

// precededByAddress reports whether the character before start belongs to an address or a path.
func precededByAddress(rs []rune, start int) bool {
	prev := runeAt(rs, start-1)
	return prev >= 0 && strings.ContainsRune("@/\\:_.#", prev)
}

// endsSentence reports whether the punctuation run [punctStart, punctEnd) ends a sentence.
func endsSentence(rs []rune, punctStart, punctEnd int) bool {
	run := runeSlice(rs, punctStart, punctEnd)
	if strings.Contains(run, "…") || strings.Contains(run, "..") {
		return false
	}
	before := runeAt(rs, punctStart-1)
	if before < 0 || !isWordChar(before) {
		return false
	}
	start := wordStart(rs, punctStart)
	word := runeSlice(rs, start, punctStart)
	if hasNumber(word) {
		return false
	}
	if utf8.RuneCountInString(word) == 1 {
		return false
	}
	if roman.MatchString(word) || hasUpper(sliceFrom(word, 1)) {
		return false
	}
	if Abbreviations[tr.Lower(word)] {
		return false
	}
	if precededByAddress(rs, start) {
		return false
	}
	if !strings.Contains(run, ".") || strings.HasPrefix(run, "!") || strings.HasPrefix(run, "?") {
		capitalized := startsUpper(word)
		if capitalized && !isSentenceStartAt(rs, start) {
			return false
		}
	}
	return true
}

// isSentenceStartAt reports whether a sentence starts at start: only spaces, quotes and brackets lie between it and
// the text's start or a sentence's end.
func isSentenceStartAt(rs []rune, start int) bool {
	index := start - 1
	for index >= 0 && (isJSSpace(rs[index]) || strings.ContainsRune(`"'“‘«([`, rs[index])) {
		index--
	}
	return index < 0 || strings.ContainsRune(".!?…", rs[index])
}

// isMissingSpaceBoundary reports whether a dot, exclamation or question mark glued between two letter runs
// (left[punct]right) is a sentence boundary that lost its space, not a domain, a file name, an abbreviation, a
// camelCase word or an address. Offsets are in runes.
func isMissingSpaceBoundary(text string, leftStart int, left string, punctAt int, rightStart int, right string, isWordOrFixable func(lower string) bool) bool {
	rs := []rune(text)
	rightEnd := rightStart + utf8.RuneCountInString(right)
	if isWordChar(runeAt(rs, leftStart-1)) || precededByAddress(rs, leftStart) {
		return false
	}
	if isWordChar(runeAt(rs, rightEnd)) || continuesAsAddress(rs, rightEnd) {
		return false
	}
	if hasUpper(sliceFrom(left, 1)) || hasUpper(sliceFrom(right, 1)) {
		return false
	}
	leftLower := tr.Lower(left)
	rightLower := tr.Lower(right)
	if Abbreviations[leftLower] || NotASentenceStart[rightLower] {
		return false
	}
	if !isWordOrFixable(leftLower) || !isWordOrFixable(rightLower) {
		return false
	}
	_ = punctAt
	return true
}

// sortByStart is result.sort((a, b) => a.start - b.start): a stable sort by start.
func sortByStart(result []Correction) {
	slices.SortStableFunc(result, func(a, b Correction) int { return a.Start - b.Start })
}

// spliceOut removes result[i].
func spliceOut(result []Correction, i int) []Correction {
	return append(result[:i], result[i+1:]...)
}

var (
	missingSpaceRE  = regexp.MustCompile(`(\p{L}{2,})([.!?])(\p{L}{2,})`)
	sentenceTokenRE = regexp.MustCompile(`\p{L}+(?:['’]\p{L}+)*|[0-9]+`)
	allDigits       = regexp.MustCompile(`^[0-9]+$`)
	adjSpaceRE      = regexp.MustCompile(`^[ \t]+$`)
	adjDotSpaceRE   = regexp.MustCompile(`^\.[ \t]+$`)
	// numberSuffixRE is /(\d+(?:[.:]\d+)?)(')?([a-zçğıöşü]+)(?![\p{L}\p{N}_])/gu without its lookahead, which the
	// loop checks: the letter run is greedy and nothing before it can backtrack into a match, so a match whose next
	// character fails the lookahead is no match at all.
	numberSuffixRE = regexp.MustCompile(`([0-9]+(?:[.:][0-9]+)?)(')?([a-zçğıöşü]+)`)
)

// turkishMarkLetters are /[çğıöşüâîû]/ without the i flag; turkishMarkLettersI are the same with it (ı has no
// capital of its own there, as JavaScript's case-insensitive matching never maps a non-ASCII letter to an ASCII
// one).
const (
	turkishMarkLetters  = "çğıöşüâîû"
	turkishMarkLettersI = "çğıöşüâîûÇĞÖŞÜÂÎÛ"
)

// bareInstitutionOffices are institution heads that are offices on their own ("Başbakanlık'a" -> "Başbakanlığa").
var bareInstitutionOffices = setOf(
	"başbakanlık", "bakanlık", "müdürlük", "başkanlık", "rektörlük", "dekanlık", "valilik", "kaymakamlık",
)

// unambigMixedInstHeads are institution heads that are safe to capitalize in mixed-case text.
var unambigMixedInstHeads = setOf(
	"üniversitesi", "bakanlığı", "valiliği", "kaymakamlığı", "lisesi", "hastanesi", "belediyesi", "pastanesi", "bakkaliyesi",
)

// givenMatch is a token read as a person's given name: its display form, its suffix and whether it had an
// apostrophe.
type givenMatch struct {
	display, suffix string
	hadApos         bool
}

// surnameMatch is a token read as a surname.
type surnameMatch struct {
	display, suffix   string
	hadApos, isCommon bool
}

// shopHeads are the institution heads after which an ambiguous name is a shop's or a school's name ("Yıldız
// pastanesi").
var shopHeads = setOf(
	"pastanesi", "bakkaliyesi", "lokantası", "restoranı", "oteli", "kafesi", "fırını", "eczanesi", "kitabevi",
	"yayınevi", "lisesi", "koleji", "hastanesi", "üniversitesi", "bankası",
)

// anchorInstModifiers are institution modifiers that are proper anchors on their own ("Dokuz Eylül", "Marmara",
// "Ziraat", "Diyanet").
var anchorInstModifiers = setOf(
	"dokuz", "ondokuz", "marmara", "boğaziçi", "uludağ", "osmangazi", "hacettepe", "gazi", "ege", "anadolu", "ziraat",
	"içişleri", "dışişleri", "bankacılık", "sosyal", "bakanlar", "meteoroloji", "radyo", "diyanet",
)

// strictAppositiveHeads are the heads that need a canonical institutional modifier before them.
var strictAppositiveHeads = setOf("şirketi", "kulübü", "grubu", "holdingi")

// appositiveInstMods are the canonical institutional modifiers before strictAppositiveHeads ("Spor Kulübü").
var appositiveInstMods = setOf(
	"spor", "jimnastik", "anonim", "limited", "ticaret", "yatırım", "inşaat", "sanayi", "gençlik", "kültür",
	"denizcilik", "havacılık",
)

// nationalityGenericMod are nationalities that make a two-word institution generic ("Alman kulübü").
var nationalityGenericMod = setOf(
	"alman", "fransız", "ingiliz", "amerikan", "italyan", "ispanyol", "rus", "yunan", "japon",
	"çin", "türk", "osmanlı", "sovyet", "avrupa", "asya", "afrika", "arap", "ermeni", "yahudi",
	"kürt", "rum", "bulgar", "sırp", "macar", "hollanda", "belçika", "isveç", "norveç", "polonya",
)

// genericNatInstHeads are the heads that a nationality before them makes generic.
var genericNatInstHeads = setOf(
	"kulübü", "başkanlığı", "idaresi", "müdürlüğü", "bölümü", "kütüphanesi", "müzesi",
	"mahkemesi", "komitesi", "konseyi", "partisi", "birliği", "federasyonu", "derneği",
	"vakfı", "şirketi", "tiyatrosu", "operası", "hastanesi", "okulu", "fakültesi",
	"merkezi", "dairesi", "şubesi", "ofisi", "yurdu", "elçiliği",
)

// foreignAppositiveHeads are the generic heads an unknown foreign word before them leaves lowercase ("Kurbin
// belediyesinde").
var foreignAppositiveHeads = setOf("belediyesi", "hastanesi", "mahkemesi", "okulu", "müzesi", "kütüphanesi")

// officerRoleFollowBlock are the words after a bare lowercase job that show it is a description, not an office.
var officerRoleFollowBlock = setOf(
	"olan", "olarak", "oldu", "olmuş", "olacak", "idi", "seçildi", "atandı", "görev", "görevini",
)

// geoModifiers are the modifiers a place's name may start with ("antik", "millî", "büyük", "doğu").
var geoModifiers = setOf(
	"antik", "millî", "milli", "tabiat", "büyük", "küçük", "yeni", "eski", "kızıl", "kara", "ak", "beyaz",
	"yeşil", "mavi", "sarı", "orta", "iç", "doğu", "batı", "kuzey", "güney", "kuzeydoğu", "kuzeybatı",
	"güneydoğu", "güneybatı", "yukarı", "aşağı", "birinci", "ikinci", "üçüncü", "meydan", "deniz", "barış",
	"ateşkes", "şehitler", "şehitleri", "sultan", "gazi", "paşa", "mustafa", "kemal", "atatürk", "selçuklu",
	"anadolu", "uluslararası", "devlet",
)

// strictKnownGeoHeads are the place heads that need a strong proper anchor before them.
var strictKnownGeoHeads = setOf(
	"kanalı", "sokağı", "savaşı", "dağları", "mahallesi", "hanı", "koyu", "kemeri", "feneri",
	"istasyonu", "otogarı", "stadı", "arenası", "plajı", "tepesi", "vadisi", "kenti", "hamamı",
	"çarşısı", "çeşmesi", "kilisesi", "otoyolu", "geçidi", "limanı", "iskelesi",
)

// quarterNationalities make "<nationality> mahallesi" a generic quarter ("Yahudi mahallesi").
var quarterNationalities = setOf(
	"yahudi", "ermeni", "rum", "türk", "yunan", "arap", "kürt", "çin", "italyan", "fransız", "alman",
)

// stationKinds make "<kind> istasyonu" a generic station ("tren istasyonu").
var stationKinds = setOf("tren", "metro", "tramvay", "otobüs", "uluslararası")

// surnameFirstTitles are the titles after which a surname alone is a name ("Bay Kaya").
var surnameFirstTitles = setOf("bay", "bayan", "doktor", "avukat", "profesör", "albay", "komiser", "kaptan")

// strongFirstTitles are the titles that make the name after them a person's on their own.
var strongFirstTitles = setOf(
	"bay", "bayan", "mimar", "gazi", "albay", "kaymakam", "hâkim", "hakim", "savcı", "başsavcı", "kaptan",
	"başkomiser", "komiser", "büyükelçi", "başhekim", "cumhurbaşkanı",
)

// rankPrefixes are the first words of two-word ranks ("belediye başkanı", "genel müdür").
var rankPrefixes = setOf(
	"belediye", "genel", "bölüm", "okul", "meclis", "cumhuriyet", "dışişleri", "içişleri", "sağlık", "adalet",
	"maliye", "savunma",
)

// predicateAfterBareTitle are the words after a bare occupational title that make it a predicate ("Ahmet öğretmen
// oldu").
var predicateAfterBareTitle = setOf(
	"oldu", "olmuş", "olacak", "olmak", "olarak", "olur", "oluyor", "olsa", "olsun",
	"gibi", "değil", "mı", "mi", "mu", "mü", "var", "yok", "kalmış", "kaldı",
)

// notNameBeforeHonorific are the words before an honorific that are not names ("genç bey", "güzel hanım").
var notNameBeforeHonorific = setOf(
	"sayın", "genç", "yaşlı", "güzel", "küçük", "büyük", "sevgili", "değerli", "kibar", "efendi",
)

// personalAmbigNames are the common words among the given names that are a person's after a relationship word
// ("arkadaşım umut").
var personalAmbigNames = setOf(
	"melek", "ateş", "umut", "bahar", "deniz", "can", "barış", "gül", "ırmak", "özlem", "sevgi", "defne", "doruk",
	"pınar", "gönül", "aslı", "cihan", "saadet", "çiçek", "yağmur", "toprak", "bulut", "güneş", "yıldız", "kader",
	"nur", "ipek", "evren", "kaya",
)

// dativeNext are the words after a name in the dative that show a human addressee ("Deniz'e mesaj at").
var dativeNext = setOf("mesaj", "söyle", "söyledim", "söyleyin", "gönderdim", "gönder", "ilettim", "sürpriz")

// ambigPlaceNames are the ambiguous place names the disambiguator is asked about.
var ambigPlaceNames = setOf(
	"afyon", "ağrı", "aydın", "bebek", "bodrum", "boğaz", "çeşme", "kartal", "kaş", "kemer", "konak", "mısır",
	"moda", "ordu", "tokat", "tuzla", "uşak", "van",
)

var (
	workCueRE            = regexp.MustCompile(`^(?:oku|bitir|yaz|izle|seyret|roman|kitap|film|dizi|oyun|eser|tiyatro|yazar|yönetmen|tablo|resim|sahne|başyapıt|çevir|baskı|yayım|sinema|şair|ödül|etkilen|beğen|duygulan|bırak|anlat|albüm|şarkı)`)
	specialDayCueRE      = regexp.MustCompile(`^(?:tören|kutlu|kutla|anma|etkinlik|program|konuşma|resmî|resmi|millî|milli|bayram|tebrik|mesaj|hafta|hediye|armağan|yürüyüş|saygı|minnet|mart|mayıs|haziran|şubat|kasım)`)
	sVowelFinalRE        = regexp.MustCompile(`[aeıioöuüâîû]$`)
	eraPossNounRE        = regexp.MustCompile(`(?:s[ıiuü]|l[ae]r[ıi])(?:n[ıiuü]n|nd[ae]n?|n[aeıiuü]|yl[ae])?$`)
	authorGenitiveRE     = regexp.MustCompile(`'(?:ın|in|un|ün|nın|nin|nun|nün)$`)
	fourDigits           = regexp.MustCompile(`^[0-9]{4}$`)
	readingOrWatchingRE  = regexp.MustCompile(`^(?:oku|bitir|yaz|izle|seyret|roman|kitap|film|dizi|oyun|eser|tiyatro)`)
	lowerSuffixLetters   = regexp.MustCompile(`^[a-zçğıöşü]+$`)
	consonantSRE         = regexp.MustCompile(`[bcdfghjklmnpqrtvwxz]s$`)
	pluralCaseRE         = regexp.MustCompile(`^l[ae]r.+`)
	cSuffixRE            = regexp.MustCompile(`^[cçs]`)
	lukEndRE             = regexp.MustCompile(`l[ıiuü]k$`)
	liCiEndRE            = regexp.MustCompile(`(?:l[ıiuü]|[cç][ıiuü])$`)
	larEndRE             = regexp.MustCompile(`l[ae]r$`)
	kiOrCopRE            = regexp.MustCompile(`^(?:ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|ki(?:n[aeıiuü]|nd[ae]n?|n[ıiuü]n)?|yd[ıiuü][mk]?|ym[ıiuü]ş|n|m)$`)
	possRelocRE          = regexp.MustCompile(`(?:m[ıiuü]z|n[ıiuü]z)$`)
	daEndRE              = regexp.MustCompile(`[dt][ae]$`)
	denizCueRE           = regexp.MustCompile(`^(?:gir|yüz|atla|dökül|açıl|bak|in)`)
	gulCueRE             = regexp.MustCompile(`^(?:bahçe|su|reçel|yaprak|diken|koku)`)
	copulaSuffixRE       = regexp.MustCompile(`^[dt][ıiuü]r$`)
	aliyeDativeCueRE     = regexp.MustCompile(`^(?:söyle|ver|gönder|yaz|sor|ilet|mesaj|haber|bak|uğra|telefon|git|gid|gel|dön|bırak|de|da|ulaş)`)
	accusativeEndRE      = regexp.MustCompile(`(?:ini|ını|unu|ünü|yi|yı|yu|yü)$`)
	pluralProperRE       = regexp.MustCompile(`^l[ae]r(?:[ıiuü]n(?:d[ae]n?)?|d[ae]n?|[aeıiuü]|l[ae]|$)`)
	nonSoftSuffixRE      = regexp.MustCompile(`^(?:n?[ıiuü]n|[dt][ae]n?|y?l[ae]|y?[aeıiuü])$`)
	ambigSuffixRE        = regexp.MustCompile(`^(?:n?[ıiuü]n|[dt][ae]n?|[dt][ae]ki|n?[ıiuü]nki|y?l[ae]|y?[aeıiuü])$`)
	genitiveSuffixRE     = regexp.MustCompile(`^(?:n?[ıiuü]n)$`)
	dativeSuffixRE       = regexp.MustCompile(`^y?[ae]$`)
	instrumentalSuffixRE = regexp.MustCompile(`^y?l[ae]$`)
	sameRoomRE           = regexp.MustCompile(`^(?:oda|sınıf|otobüs|okul|ev|apartman|masa|takım|grup)`)
	togetherVerbRE       = regexp.MustCompile(`^(?:hazırl|çalış|git|gid|gel|otur|kal|yap|konuş|yürü)`)
	glossNextRE          = regexp.MustCompile(`^(?:anlamına|anlamında|demek|kelimesi|sözcüğü)$`)
)

// stripCircumflex takes the circumflexes off a word ("Millî" -> "Milli").
func stripCircumflex(s string) string {
	return strings.NewReplacer("â", "a", "î", "i", "û", "u", "Â", "A", "Î", "I", "Û", "U").Replace(s)
}

// applySentenceRules adds missing spaces, TDK proper-name capitals, multi-word phrases, titles, dates, apostrophe
// fixes and sentence-start capitals to corrections (sorted, not overlapping), and returns them all. nameFor and
// stemsOf may be nil, and resources nil for the minimal rules alone.
func applySentenceRules(
	text string,
	corrections []Correction,
	isWord func(lower string) bool,
	nameFor func(lower string, common bool) (string, bool),
	stemsOf func(lower string) ([]string, bool),
	resources *SentenceResources,
) []Correction {
	if nameFor == nil {
		nameFor = func(string, bool) (string, bool) { return "", false }
	}
	if stemsOf == nil {
		stemsOf = func(string) ([]string, bool) { return nil, false }
	}
	rs := []rune(text)
	textLen := len(rs)
	slice := func(start, end int) string { return runeSlice(rs, start, end) }
	result := slices.Clone(corrections)
	covering := func(start, end int) int {
		return slices.IndexFunc(result, func(fix Correction) bool { return fix.Start < end && fix.End > start })
	}
	spaced := map[int]bool{}

	// 1. A missing space after a sentence end between two real words: "merhaba.ben" -> "merhaba. Ben".
	for _, match := range findAllRunes(missingSpaceRE, text) {
		leftStart := match[0]
		left := slice(match[2], match[3])
		punctAt := leftStart + utf8.RuneCountInString(left)
		rightStart := punctAt + 1
		right := slice(match[6], match[7])
		rightEnd := rightStart + utf8.RuneCountInString(right)
		if isWordChar(runeAt(rs, leftStart-1)) || precededByAddress(rs, leftStart) {
			continue
		}
		if isWordChar(runeAt(rs, rightEnd)) || continuesAsAddress(rs, rightEnd) {
			continue
		}
		if hasUpper(sliceFrom(left, 1)) || hasUpper(sliceFrom(right, 1)) {
			continue
		}
		if covering(punctAt, punctAt+1) != -1 {
			continue
		}
		wordFix := covering(rightStart, rightEnd)
		leftFix := covering(leftStart, punctAt)
		asFixed := func(index int, fallback string) string {
			if index != -1 && !strings.Contains(result[index].Replacement, " ") {
				return result[index].Replacement
			}
			return fallback
		}
		leftLower := tr.Lower(asFixed(leftFix, left))
		rightLower := tr.Lower(asFixed(wordFix, right))
		if Abbreviations[leftLower] || NotASentenceStart[rightLower] {
			continue
		}
		if !isWord(leftLower) || !isWord(rightLower) {
			continue
		}
		punct := string(rs[punctAt])
		spaced[punctAt] = true
		if wordFix == -1 {
			result = append(result, Correction{
				Start:       punctAt,
				End:         rightEnd,
				Original:    slice(punctAt, rightEnd),
				Replacement: punct + " " + capitalize(right),
				Kind:        KindPunct,
				Margin:      6,
			})
		} else {
			existing := result[wordFix]
			if existing.Start != rightStart {
				continue
			}
			existing.Start = punctAt
			existing.Original = slice(punctAt, existing.End)
			existing.Replacement = punct + " " + capitalize(existing.Replacement)
			result[wordFix] = existing
		}
	}
	sortByStart(result)

	// The minimal fallback, for unit tests that call without resources.
	if resources == nil || resources.Known == nil || !resources.Known.HasProperData() {
		result = applyMinimalRules(rs, result, spaced, isWord, nameFor, stemsOf)
		result = applySentenceStartCapitals(rs, result, spaced)
		sortByStart(result)
		return result
	}

	// The full TDK proper-name and apostrophe rules, with resources.Known.
	known := resources.Known
	isMorphWord := resources.IsMorphWord
	if isMorphWord == nil {
		isMorphWord = isWord
	}
	capShare := resources.CapitalShare
	if capShare == nil {
		capShare = func(string) float64 { return 0 }
	}
	lexCount := resources.Count
	if lexCount == nil {
		lexCount = func(string) float64 { return 0 }
	}
	lexPair := resources.Pair
	if lexPair == nil {
		lexPair = func(string, string) float64 { return 0 }
	}

	// The words and numbers (without those glued to addresses or code), with their forms after the core's fixes.
	var tokens []*SentenceToken
	for _, m := range findAllRunes(sentenceTokenRE, text) {
		start := m[0]
		end := m[1]
		raw := slice(start, end)
		isNum := allDigits.MatchString(raw)
		if !isNum {
			if precededByAddress(rs, start) {
				continue
			}
			if continuesAsAddress(rs, end) && !spaced[end] {
				continue
			}
			if r := runeAt(rs, end); r >= 0 && (unicode.IsNumber(r) || r == '_') {
				continue
			}
			if r := runeAt(rs, start-1); r >= 0 && (unicode.IsNumber(r) || r == '_') {
				continue
			}
		}
		eff := raw
		lockedByCore := false
		if covIdx := covering(start, end); covIdx != -1 {
			fix := result[covIdx]
			if fix.Start == start && fix.End == end && !strings.Contains(fix.Replacement, " ") {
				eff = fix.Replacement
			} else if spaced[fix.Start] && fix.End == end && strings.Contains(fix.Replacement, " ") {
				eff = fix.Replacement[strings.Index(fix.Replacement, " ")+1:]
			} else {
				// A core split or join over more than one word ("Belliki" -> "Belli ki", "Yarın ki" -> "Yarınki") is
				// never overwritten.
				lockedByCore = true
			}
		}
		var prevTok *SentenceToken
		if len(tokens) > 0 {
			prevTok = tokens[len(tokens)-1]
		}
		var gapBefore string
		if prevTok != nil {
			gapBefore = slice(prevTok.End, start)
			prevTok.GapAfter = gapBefore
		} else {
			gapBefore = slice(0, start)
		}
		sepBefore := jsTrim(gapBefore)
		atStart := isSentenceStartAt(rs, start) ||
			(prevTok != nil && spaced[prevTok.End] && utf8.RuneCountInString(gapBefore) == 1)
		tokens = append(tokens, &SentenceToken{
			Start:     start,
			End:       end,
			Raw:       raw,
			Eff:       eff,
			Low:       tr.Lower(eff),
			IsWord:    !isNum,
			IsNumber:  isNum,
			SepBefore: sepBefore,
			GapBefore: gapBefore,
			GapAfter:  sliceFrom(text, end),
			AtStart:   atStart,
			Locked:    lockedByCore,
		})
	}

	// setToken sets a token's form, merging it with a fix of the core or a missing space already there.
	setToken := func(tok *SentenceToken, nextText string, lock bool) {
		if lock {
			tok.Locked = true
		}
		// A change whose only difference is a curly ’ for a straight ' is never made.
		if nextText == tok.Eff || strings.ReplaceAll(nextText, "'", "’") == tok.Eff {
			return
		}
		tok.Eff = nextText
		tok.Low = tr.Lower(nextText)
		idx := covering(tok.Start, tok.End)
		if idx == -1 {
			if nextText != tok.Raw {
				result = append(result, Correction{Start: tok.Start, End: tok.End, Original: tok.Raw, Replacement: nextText, Kind: KindPunct, Margin: 6})
				sortByStart(result)
			}
			return
		}
		existing := result[idx]
		if existing.Start == tok.Start && existing.End == tok.End {
			if nextText == tok.Raw {
				result = spliceOut(result, idx)
			} else {
				existing.Replacement = nextText
				result[idx] = existing
			}
			return
		}
		if spaced[existing.Start] && existing.End == tok.End {
			if spIdx := strings.Index(existing.Replacement, " "); spIdx != -1 {
				existing.Replacement = existing.Replacement[:spIdx+1] + nextText
				result[idx] = existing
			}
		}
	}

	// adjSpace reports whether tokens a and b are apart by white space alone (no comma, full stop or new line).
	adjSpace := func(a, b *SentenceToken) bool { return adjSpaceRE.MatchString(b.GapBefore) }
	// adjDotSpace reports whether a is followed by "." and spaces and then b ("Prof. Dr. Ahmet").
	adjDotSpace := func(a, b *SentenceToken) bool { return adjDotSpaceRE.MatchString(b.GapBefore) }
	// isCap reports whether a token is written with a capital.
	isCap := func(tok *SentenceToken) bool { return startsUpper(tok.Eff) }
	isAllCaps := func(tok *SentenceToken) bool {
		return utf8.RuneCountInString(tok.Eff) >= 2 && tok.Eff == tr.Upper(tok.Eff) && tok.Eff != tr.Lower(tok.Eff)
	}
	isMixedInnerCap := func(tok *SentenceToken) bool { return !isAllCaps(tok) && hasUpper(sliceFrom(tok.Eff, 1)) }

	// The text has evidence of typing in ASCII: a core fix of marks, or no Turkish letters at all.
	textHasTurkishMarks := strings.ContainsAny(text, turkishMarkLettersI)
	sentenceHasAsciiFixes := slices.ContainsFunc(result, func(r Correction) bool {
		o := tr.Lower(r.Original)
		rep := tr.Lower(r.Replacement)
		return o != rep && FoldTurkish(o) == FoldTurkish(rep)
	})

	// lookupName finds a proper name exactly, or by its unique ASCII-folded form when that is safe.
	lookupName := func(stemLow string, allowAscii bool) (NameEntry, bool) {
		if exact, ok := known.NameEntry(stemLow, 0); ok {
			return exact, true
		}
		if allowAscii && !strings.ContainsAny(stemLow, turkishMarkLetters) && !isMorphWord(stemLow) {
			return known.ASCIINameEntry(stemLow, 0)
		}
		return NameEntry{}, false
	}

	// isCommonWordCollision reports whether a lowercase form is a common word: morphology finds a stem in it other
	// than base.
	isCommonWordCollision := func(wholeLow, baseLow string) bool {
		stems, ok := stemsOf(wholeLow)
		if !ok || len(stems) == 0 {
			return isWord(wholeLow)
		}
		// "belli", "hatayı" and "emrinin" are read through a common word's stem other than base: a collision.
		return slices.ContainsFunc(stems, func(s string) bool { return s != baseLow && s != tr.Lower(baseLow) })
	}

	// resolveGivenName reads tok as a person's first name (a Turkish given name or an ambiguous given name; never a
	// language or a place).
	resolveGivenName := func(tok *SentenceToken) (givenMatch, bool) {
		if !tok.IsWord || isAllCaps(tok) || isMixedInnerCap(tok) {
			return givenMatch{}, false
		}
		low := tok.Low
		lookupPersonFirst := func(s string) (string, bool) {
			if g, ok := known.GivenNameFor(s); ok {
				return g, true
			}
			if ne, ok := known.NameEntry(s, 0); ok && !ne.NoApos && !LanguageNames[s] && !NoAposReligionOrDerived[s] {
				return known.GivenNameFor(s)
			}
			return "", false
		}
		if apos := strings.Index(low, "'"); apos > 0 {
			stem := low[:apos]
			suf := low[apos+1:]
			disp, ok := lookupPersonFirst(stem)
			if !ok {
				if ne, isName := known.NameEntry(stem, 0); !(isName && ne.NoApos) {
					disp, ok = known.NameFor(stem, 0)
				}
			}
			if ok && disp != "" && ProperInflectionalSuffixRE.MatchString(suf) {
				return givenMatch{display: disp, suffix: suf, hadApos: true}, true
			}
			return givenMatch{}, false
		}
		if bare, ok := known.GivenNameFor(low); ok {
			return givenMatch{display: bare}, true
		}
		// A given name with a suffix (never cutting everyday words like "arasında" or "savaşta").
		lowRunes := []rune(low)
		for cut := len(lowRunes) - 1; cut >= 3; cut-- {
			base := string(lowRunes[:cut])
			suf := string(lowRunes[cut:])
			disp, ok := known.GivenNameFor(base)
			if !ok {
				continue
			}
			if isCommonWordCollision(low, base) {
				continue
			}
			if _, isName := known.NameEntry(base, 0); isMorphWord(low) && !isName && lexCount(base+"'"+suf) < math.Max(5, lexCount(low)*2) {
				continue
			}
			if ProperInflectionalSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(tr.Lower(disp), suf, ProperSuffixOptions{}) {
				return givenMatch{display: disp, suffix: suf}, true
			}
			if sentenceHasAsciiFixes || !textHasTurkishMarks {
				if harm, ok := HarmonizeASCIISuffix(tr.Lower(disp), suf, ProperSuffixOptions{}); ok && harm != "" {
					return givenMatch{display: disp, suffix: harm}, true
				}
			}
		}
		if (sentenceHasAsciiFixes || !textHasTurkishMarks) && !strings.ContainsAny(low, turkishMarkLetters) {
			if asc, ok := known.ASCIINameEntry(low, 0); ok && !asc.NoApos {
				if _, isGiven := known.GivenNameFor(tr.Lower(asc.Name)); isGiven {
					return givenMatch{display: asc.Name}, true
				}
			}
		}
		return givenMatch{}, false
	}

	// resolveSurname reads tok as a person's surname, bare or with a suffix.
	resolveSurname := func(tok *SentenceToken, allowCommon bool) (surnameMatch, bool) {
		if !tok.IsWord || isAllCaps(tok) || isMixedInnerCap(tok) {
			return surnameMatch{}, false
		}
		low := tok.Low
		if apos := strings.Index(low, "'"); apos > 0 {
			stem := low[:apos]
			suf := low[apos+1:]
			// A surname only (never known.NameFor, which would take the city in "arda Meksika'da").
			if disp, ok := known.SurnameFor(stem); ok && ProperInflectionalSuffixRE.MatchString(suf) {
				return surnameMatch{display: disp, suffix: suf, hadApos: true, isCommon: veryCommonSurnameCollisions[stem]}, true
			}
			if allowCommon && isCap(tok) && !tok.AtStart && ProperInflectionalSuffixRE.MatchString(suf) {
				return surnameMatch{display: runeSlice([]rune(tok.Eff), 0, utf8.RuneCountInString(low[:apos])), suffix: suf, hadApos: true}, true
			}
			return surnameMatch{}, false
		}
		if dispBare, ok := known.SurnameFor(low); ok {
			isCom := veryCommonSurnameCollisions[low]
			if !isCom || allowCommon {
				return surnameMatch{display: dispBare, isCommon: isCom}, true
			}
		}
		// No apostrophe is made up inside a surname already capitalized mid-sentence ("Fatma Aliye", "Bay Reede",
		// "Robert Belle").
		if isCap(tok) && !tok.AtStart {
			return surnameMatch{}, false
		}
		lowRunes := []rune(low)
		for cut := len(lowRunes) - 1; cut >= 2; cut-- {
			base := string(lowRunes[:cut])
			suf := string(lowRunes[cut:])
			disp, ok := known.SurnameFor(base)
			if !ok {
				continue
			}
			if isCommonWordCollision(low, base) {
				continue
			}
			isCom := veryCommonSurnameCollisions[base]
			if isCom && !allowCommon {
				continue
			}
			if _, isName := known.NameEntry(base, 0); isMorphWord(low) && !isName && capShare(low) < 0.35 && lexCount(low) >= 20 {
				continue
			}
			if ProperInflectionalSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(tr.Lower(disp), suf, ProperSuffixOptions{}) {
				return surnameMatch{display: disp, suffix: suf, isCommon: isCom}, true
			}
			if sentenceHasAsciiFixes || !textHasTurkishMarks {
				if harm, ok := HarmonizeASCIISuffix(tr.Lower(disp), suf, ProperSuffixOptions{}); ok && harm != "" {
					return surnameMatch{display: disp, suffix: harm, isCommon: isCom}, true
				}
			}
		}
		return surnameMatch{}, false
	}

	// buildContextAround is the text left and right of tokens[tokIdx] with the fixes made so far.
	buildContextAround := func(tokIdx int) (leftRaw, rightRaw string) {
		target := tokens[tokIdx]
		leftFrom := max(0, target.Start-140)
		rightTo := min(textLen, target.End+100)
		var lb strings.Builder
		cur := leftFrom
		for k := 0; k < tokIdx; k++ {
			t := tokens[k]
			if t.End <= leftFrom {
				continue
			}
			if t.Start >= target.Start {
				break
			}
			if t.Start < cur {
				continue
			}
			disp := t.Eff
			if t.AtStart && t.IsWord && lowerInitial.MatchString(t.Eff) {
				disp = capitalize(t.Eff)
			}
			lb.WriteString(slice(cur, t.Start) + disp)
			cur = t.End
		}
		lb.WriteString(slice(cur, target.Start))
		var rb strings.Builder
		cur = target.End
		for k := tokIdx + 1; k < len(tokens); k++ {
			t := tokens[k]
			if t.End <= target.End {
				continue
			}
			if t.Start >= rightTo {
				break
			}
			if t.Start < cur {
				continue
			}
			disp := t.Eff
			if t.AtStart && t.IsWord && lowerInitial.MatchString(t.Eff) {
				disp = capitalize(t.Eff)
			}
			rb.WriteString(slice(cur, t.Start) + disp)
			cur = t.End
		}
		rb.WriteString(slice(cur, rightTo))
		return lb.String(), rb.String()
	}

	// formatProper writes a proper name and its suffix by the apostrophe rule.
	formatProper := func(displayBase, suffix string, takesApos bool) string {
		if suffix == "" {
			return displayBase
		}
		if takesApos {
			return displayBase + "'" + suffix
		}
		return displayBase + suffix
	}

	// 2. Multi-word canonical and mined proper phrases (PHRASE_APOS, PHRASE_NOAPOS). They run before the wrong
	// apostrophes and the single words, so that new titles like "Saatleri Ayarlama Enstitüsü'nü", "Beyaz
	// Kale'sini" or "İnce Memed'ini" win.
	hasWorkSentenceCue := slices.ContainsFunc(tokens, func(t *SentenceToken) bool {
		return t.IsWord && workCueRE.MatchString(t.Low)
	})
	hasSpecialDaySentenceCue := false
	for idx, t := range tokens {
		if (t.IsWord && specialDayCueRE.MatchString(t.Low)) ||
			(t.IsNumber && idx+1 < len(tokens) && tokens[idx+1].IsWord && Months.Has(strings.Split(tokens[idx+1].Low, "'")[0])) {
			hasSpecialDaySentenceCue = true
			break
		}
	}
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked {
			continue
		}
		candidates := known.PhrasesStartingWith(tok.Low, true)
		if len(candidates) == 0 {
			continue
		}
		for _, phrase := range candidates {
			n := len(phrase.Words)
			if i+n > len(tokens) {
				continue
			}
			// The words are apart by spaces alone.
			ok := true
			for k := 1; k < n; k++ {
				if !adjSpace(tokens[i+k-1], tokens[i+k]) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			// All but the last word match, exactly or folded to ASCII.
			for k := 0; k < n-1; k++ {
				tw := tokens[i+k].Low
				pw := phrase.Words[k]
				if tw != pw && FoldTurkish(tw) != FoldTurkish(pw) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}

			// The last word matches bare or with an inflectional suffix, with or without an apostrophe.
			lastTok := tokens[i+n-1]
			lastPw := phrase.Words[n-1]
			lastDisp := phrase.DisplayWords[n-1]
			lastLow := lastTok.Low
			apos := strings.Index(lastLow, "'")
			matchedSuffix, matched := "", false
			p3sgOf := func(pw string) bool {
				return sVowelFinalRE.MatchString(pw) && (InstitutionHeads[pw] || GeoHeads[pw] || StateHeads[pw] || EventHeads[pw])
			}

			if lastLow == lastPw || FoldTurkish(lastLow) == FoldTurkish(lastPw) {
				matchedSuffix, matched = "", true
			} else if apos > 0 {
				stem := lastLow[:apos]
				suf := lastLow[apos+1:]
				if (stem == lastPw || FoldTurkish(stem) == FoldTurkish(lastPw)) && (HeadSuffixRE.MatchString(suf) || ProperInflectionalSuffixRE.MatchString(suf)) {
					matchedSuffix, matched = suf, true
				}
			} else {
				// "Beyaz Kalesini" where the core already made "Kalesi'ni": the raw form is tried too.
				rawLow := tr.Lower(lastTok.Raw)
				forms := []string{lastLow}
				if rawLow != lastLow {
					forms = append(forms, rawLow)
				}
				for _, form := range forms {
					if strings.HasPrefix(form, lastPw) && utf8.RuneCountInString(form) > utf8.RuneCountInString(lastPw) {
						suf := form[len(lastPw):]
						p3sg := p3sgOf(lastPw)
						if (ProperInflectionalSuffixRE.MatchString(suf) || HeadSuffixRE.MatchString(suf)) && IsPhonologicallyValidProperSuffix(lastPw, suf, ProperSuffixOptions{P3sgHead: p3sg}) {
							matchedSuffix, matched = suf, true
							break
						}
						if harm, ok := HarmonizeASCIISuffix(lastPw, suf, ProperSuffixOptions{P3sgHead: p3sg}); ok && harm != "" {
							matchedSuffix, matched = harm, true
							break
						}
					} else {
						fForm := FoldTurkish(form)
						fPw := FoldTurkish(lastPw)
						if strings.HasPrefix(fForm, fPw) && utf8.RuneCountInString(fForm) > utf8.RuneCountInString(fPw) {
							suf := sliceFrom(form, utf8.RuneCountInString(lastPw))
							p3sg := p3sgOf(lastPw)
							if (ProperInflectionalSuffixRE.MatchString(suf) || HeadSuffixRE.MatchString(suf)) && IsPhonologicallyValidProperSuffix(lastPw, suf, ProperSuffixOptions{P3sgHead: p3sg}) {
								matchedSuffix, matched = suf, true
								break
							}
							if harm, ok := HarmonizeASCIISuffix(lastPw, suf, ProperSuffixOptions{P3sgHead: p3sg}); ok && harm != "" {
								matchedSuffix, matched = harm, true
								break
							}
						}
					}
				}
			}

			// The core already split a book title wrongly: "Beyaz Kalesini" -> "Beyaz Kalesi'ni".
			if !matched && apos > 0 {
				joined := strings.ReplaceAll(lastLow, "'", "")
				if strings.HasPrefix(joined, lastPw) && utf8.RuneCountInString(joined) > utf8.RuneCountInString(lastPw) {
					suf := joined[len(lastPw):]
					if ProperInflectionalSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(lastPw, suf, ProperSuffixOptions{}) {
						matchedSuffix, matched = suf, true
					}
				}
			}

			if !matched {
				continue
			}

			// The gates of the canonical phrases that are ordinary Turkish prose too.
			switch phrase.Context {
			case PhraseEra:
				// A bare lowercase "eski çağ", "ilk çağ", "yeni çağ" or "yakın çağ" before a noun with a third-person
				// possessive ("eski çağ astronomlarının") is a modifier, not a historical period; standalone periods
				// ("İlk Çağ yazının icadıyla başlar") and inflected ones ("Eski Çağ'ın") get capitals.
				anyCap := false
				for k := 0; k < n; k++ {
					ct := tokens[i+k]
					if isCap(ct) && (!ct.AtStart || k > 0) {
						anyCap = true
					}
				}
				if !anyCap && matchedSuffix == "" {
					var afterEra *SentenceToken
					if i+n < len(tokens) && adjSpace(lastTok, tokens[i+n]) {
						afterEra = tokens[i+n]
					}
					modifiesPossNoun := afterEra != nil && !isCap(afterEra) && eraPossNounRE.MatchString(afterEra.Low)
					if modifiesPossNoun {
						continue
					}
				}
			case PhraseDay:
				// Two-word "<X> Günü" special days ("öğretmenler günü", "anneler günü") need a cue of a ceremony or a
				// celebration, or an inflectional suffix ("Öğretmenler Günü'nde").
				if matchedSuffix == "" && !hasSpecialDaySentenceCue {
					continue
				}
			case PhraseWork:
				// Titles of everyday words ("Kış Uykusu", "Yeni Hayat", "Büyük Umutlar", "Küçük Prens") need a cue of
				// literature or media, an author's genitive right before them, or title words already capitalized.
				prevAuthorGen := i > 0 && adjSpace(tokens[i-1], tok) && isCap(tokens[i-1]) && authorGenitiveRE.MatchString(tokens[i-1].Low)
				allContentCap := true
				for k := 0; k < n; k++ {
					ct := tokens[i+k]
					if !TitleConjunctions[ct.Low] && !isCap(ct) {
						allContentCap = false
						break
					}
				}
				if !hasWorkSentenceCue && !prevAuthorGen && !allContentCap {
					continue
				}
			}

			// The phrase is written as listed, keeping the circumflex already chosen ("Milli" or "Millî") on a word
			// already capitalized.
			for k := 0; k < n-1; k++ {
				curTok := tokens[i+k]
				targetDisp := phrase.DisplayWords[k]
				if isCap(curTok) && stripCircumflex(curTok.Eff) == stripCircumflex(targetDisp) {
					curTok.Locked = true
				} else {
					setToken(curTok, targetDisp, true)
				}
			}
			setToken(lastTok, formatProper(lastDisp, matchedSuffix, phrase.TakesApos), true)
			i += n - 1
			break
		}
	}

	// 3. Dates ("12 haziran 2027 cumartesi", "17 aralıka", "25 ocak ile 8 şubat").
	for i := 0; i < len(tokens)-1; i++ {
		dayTok := tokens[i]
		monthTok := tokens[i+1]
		if !dayTok.IsNumber || !monthTok.IsWord || monthTok.Locked {
			continue
		}
		if !adjSpace(dayTok, monthTok) {
			continue
		}
		// The day is 1 to 31, not an ordinal ("3.") or a decimal ("3.5").
		dayNum := jsNumber(dayTok.Raw)
		if dayNum < 1 || dayNum > 31 {
			continue
		}
		if dayTok.SepBefore == "." || strings.HasSuffix(dayTok.GapBefore, ".") || strings.HasSuffix(dayTok.GapBefore, ",") {
			continue
		}
		mSplit, ok := SplitCalendarWord(monthTok.Low, Months)
		if !ok {
			continue
		}
		setToken(monthTok, formatProper(CapTr(mSplit.Stem), mSplit.Suffix, true), true)

		// An optional year of four digits, then an optional weekday ("12 haziran 2027 cumartesi").
		nextIdx := i + 2
		if nextIdx < len(tokens) && tokens[nextIdx].IsNumber && adjSpace(tokens[nextIdx-1], tokens[nextIdx]) && fourDigits.MatchString(tokens[nextIdx].Raw) {
			nextIdx++
		}
		if nextIdx < len(tokens) && tokens[nextIdx].IsWord && adjSpace(tokens[nextIdx-1], tokens[nextIdx]) {
			wTok := tokens[nextIdx]
			if wSplit, ok := SplitCalendarWord(wTok.Low, Weekdays); ok {
				setToken(wTok, formatProper(CapTr(wSplit.Stem), wSplit.Suffix, true), true)
			}
		}
	}

	// 3b. Numbers with Turkish inflectional or ordinal suffixes ("1985te" -> "1985'te", "2023de" -> "2023'te",
	// "1985'de" -> "1985'te", "19.30da" -> "19.30'da", "2nci" -> "2'nci").
	for _, m := range findAllRunes(numberSuffixRE, text) {
		start := m[0]
		end := m[1]
		if r := runeAt(rs, end); r >= 0 && (unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_') {
			continue // the lookahead (?![\p{L}\p{N}_])
		}
		full := slice(start, end)
		numPart := slice(m[2], m[3])
		hadApos := m[4] >= 0 && m[5] > m[4]
		rawSuf := slice(m[6], m[7])
		if r := runeAt(rs, start-1); precededByAddress(rs, start) || (r >= 0 && (unicode.IsLetter(r) || r == '_')) {
			continue
		}
		if continuesAsAddress(rs, end) && !spaced[end] {
			continue
		}
		if covering(start, end) != -1 {
			continue
		}
		lastInt := numPart
		if strings.Contains(numPart, ".") {
			lastInt = numPart[strings.LastIndex(numPart, ".")+1:]
		} else if strings.Contains(numPart, ":") {
			lastInt = numPart[strings.LastIndex(numPart, ":")+1:]
		}
		fixedSuf, ok := FixNumberSuffix(lastInt, rawSuf, hadApos)
		if !ok || fixedSuf == "" {
			continue
		}
		if rep := numPart + "'" + fixedSuf; rep != full {
			result = append(result, Correction{Start: start, End: end, Original: full, Replacement: rep, Kind: KindPunct, Margin: 6})
		}
	}
	sortByStart(result)

	// 4. Wrong apostrophes taken out or moved (TDK kesme_yanlis W1-W6).
	hasReadingOrWatchingCtx := slices.ContainsFunc(tokens, func(t *SentenceToken) bool {
		return t.IsWord && readingOrWatchingRE.MatchString(t.Low)
	})
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked {
			continue
		}
		if !strings.Contains(tok.Eff, "'") && !strings.Contains(tok.Eff, "’") {
			continue
		}
		// Hyphenated foreign or compound suffixes are kept ("Luffy-san'ın", "U-Bot'larında", "Fatu-Krey'li").
		if strings.HasSuffix(tok.GapBefore, "-") {
			continue
		}

		norm := strings.ReplaceAll(tok.Eff, "’", "'")
		parts := strings.Split(norm, "'")
		if slices.Contains(parts, "") {
			continue
		}

		// W1: more than one apostrophe ("Mehmet'le'ydim" -> "Mehmet'leydim", "Kemal'in'kiler" -> "Kemal'inkiler",
		// "İzmir'li'ler" -> "İzmirliler", "Türk'lük'ün" -> "Türklüğün"). Foreign names with an apostrophe in their
		// stem are kept ("G'Kar'a", "Teal'c'in", "d'Aragon'dan", "Goa'uld'ları").
		if len(parts) >= 3 {
			stem := parts[0]
			rest1 := parts[1]
			restAll := strings.Join(parts[1:], "")
			lowStem := tr.Lower(stem)
			lowRest1 := tr.Lower(rest1)
			isTurkishSuffixMid := utf8.RuneCountInString(stem) >= 3 &&
				lowerSuffixLetters.MatchString(rest1) &&
				(ProperInflectionalSuffixRE.MatchString(lowRest1) || NonApostropheSuffixRE.MatchString(lowRest1))
			if !isTurkishSuffixMid {
				continue
			}
			if NonApostropheSuffixRE.MatchString(lowRest1) || LanguageNames[lowStem] || NoAposReligionOrDerived[lowStem] {
				stemPlus1 := JoinWithoutApostrophe(stem, rest1, isMorphWord)
				var finalJoined string
				if len(parts) == 3 {
					finalJoined = JoinWithoutApostrophe(stemPlus1, parts[2], isMorphWord)
				} else {
					finalJoined = stemPlus1 + strings.Join(parts[2:], "")
				}
				setToken(tok, finalJoined, true)
			} else {
				setToken(tok, stem+"'"+restAll, true)
			}
			continue
		}

		// One apostrophe: stem'suffix.
		stem := parts[0]
		suffix := parts[1]
		lowStem := tr.Lower(stem)
		lowSuffix := tr.Lower(suffix)
		var prevTok *SentenceToken
		if i > 0 && adjSpace(tokens[i-1], tok) {
			prevTok = tokens[i-1]
		}

		// Acronyms in capitals ("TDK'nin", "ÖSYM'nin", "ABD'nin") and foreign names with an apostrophe ("O'Connor",
		// "d'Arc") are kept.
		if utf8.RuneCountInString(stem) >= 2 && stem == tr.Upper(stem) && stem != tr.Lower(stem) {
			continue
		}
		if utf8.RuneCountInString(stem) < 3 || startsUpper(suffix) {
			continue
		}

		// W2: a derivational or plural suffix right after the apostrophe ("İzmir'li", "Tokat'lıymış", "Türk'çe",
		// "Müslüman'lık", "Marks'çı", "Tanrı'sal", "Alman'lar", "Weisse'ler").
		if NonApostropheSuffixRE.MatchString(lowSuffix) {
			joined := JoinWithoutApostrophe(stem, suffix, isMorphWord)
			lowJoined := tr.Lower(joined)
			properDisp, hasProper := "", false
			if stemEntry, ok := known.NameEntry(lowStem, 0); ok {
				properDisp, hasProper = stemEntry.Name, true
			} else if isCap(tok) {
				properDisp, hasProper = stem, true
			}
			unseenJoined := !isMorphWord(lowJoined) && lexCount(lowJoined) == 0
			// Foreign English plurals of a consonant and "s" ("Sharks'ların", "Lakers'ların", "Beatles'ların") and
			// foreign brand plurals with a case suffix and a low capital share ("Volvo'ları") are kept, while the
			// TDK's "Weisse'ler", "Alman'lar" and "Marks'çı" are fixed.
			_, isGiven := known.GivenNameFor(lowStem)
			_, isSurname := known.SurnameFor(lowStem)
			isForeignPluralBrandBlock := unseenJoined &&
				((strings.HasPrefix(lowSuffix, "l") && consonantSRE.MatchString(lowStem)) ||
					(pluralCaseRE.MatchString(lowSuffix) && !isGiven && !isSurname && capShare(lowStem) < 0.6))
			if !isForeignPluralBrandBlock &&
				hasProper && properDisp != "" &&
				(isMorphWord(lowJoined) || lexCount(lowJoined) >= 1 || strings.HasPrefix(lowSuffix, "l") || cSuffixRE.MatchString(lowSuffix)) {
				setToken(tok, JoinWithoutApostrophe(properDisp, suffix, isMorphWord), true)
				continue
			}
		}

		// W3a: a language name before the apostrophe ("Fransızca'da", "Rusça'sı", "almanca'ya", "ispanyolca'dan").
		if LanguageNames[lowStem] {
			setToken(tok, CapTr(lowStem)+suffix, true)
			continue
		}

		// W3b-opt: foreign or Arabic -izm and -iyet religions keep their apostrophe ("İslamiyet'e", "Budizm'in",
		// "Hinduizm'de").
		if OptionalAposIsmIyet[lowStem] {
			if !isCap(tok) {
				setToken(tok, CapTr(lowStem)+"'"+suffix, true)
			} else {
				tok.Locked = true
			}
			continue
		}

		// W3b: a derived religion, ideology or demonym before the apostrophe ("Hristiyanlık'ta" -> "Hristiyanlıkta",
		// "Hristiyanlık'ın" -> "Hristiyanlığın", "Atatürkçülük'ün" -> "Atatürkçülüğün", "Ankaralı'nın" ->
		// "Ankaralının").
		if NoAposReligionOrDerived[lowStem] {
			setToken(tok, JoinWithoutApostrophe(CapTr(lowStem), suffix, isMorphWord), true)
			continue
		}
		existingProperStem, hasExistingProper := known.NameEntry(lowStem, 0)
		if !DerivLookingKeepApos[lowStem] && (!hasExistingProper || existingProperStem.NoApos) {
			aposCount := lexCount(lowStem + "'" + lowSuffix)
			softBase := CapTr(lowStem)
			if isCap(tok) {
				softBase = stem
			}
			softPlain := JoinWithoutApostrophe(softBase, suffix, isMorphWord)
			lowSoftPlain := tr.Lower(softPlain)
			joinCount := math.Max(lexCount(lowStem+lowSuffix), lexCount(lowSoftPlain))
			if lukEndRE.MatchString(lowStem) && utf8.RuneCountInString(lowStem) >= 6 && aposCount <= joinCount {
				root := sliceRunes(lowStem, 0, -3)
				_, rootName := known.NameEntry(root, 0)
				_, rootAmbig := known.AmbigNameFor(root)
				if (rootName || rootAmbig) && isMorphWord(lowSoftPlain) {
					setToken(tok, softPlain, true)
					continue
				}
			}
			if liCiEndRE.MatchString(lowStem) && utf8.RuneCountInString(lowStem) >= 6 && aposCount*3 <= joinCount {
				root := sliceRunes(lowStem, 0, -2)
				_, rootName := known.NameEntry(root, 0)
				_, rootAmbig := known.AmbigNameFor(root)
				if (rootName || rootAmbig) && isMorphWord(lowSoftPlain) {
					setToken(tok, softPlain, true)
					continue
				}
			}
		}

		// W3c: a plural proper noun before the apostrophe ("Japonlar'ın", "Almanlar'ın", "Türkler'in", "Ahmetler'e",
		// "Yılmazlar'ın", "Atatürkçüler'in", "Ankaralılar'ın").
		if larEndRE.MatchString(lowStem) && utf8.RuneCountInString(lowStem) >= 6 && !PluralPlaceKeepApos[lowStem] {
			singular := sliceRunes(lowStem, 0, -3)
			_, sName := known.NameEntry(singular, 0)
			_, sGiven := known.GivenNameFor(singular)
			_, sSurname := known.SurnameFor(singular)
			_, sAmbig := known.AmbigNameFor(singular)
			derivedName := false
			if utf8.RuneCountInString(singular) >= 5 && liCiEndRE.MatchString(singular) {
				_, derivedName = known.NameEntry(sliceRunes(singular, 0, -2), 0)
			}
			if sName || sGiven || sSurname || sAmbig || singular == "alman" || NoAposReligionOrDerived[singular] || derivedName {
				base := CapTr(lowStem)
				if isCap(tok) {
					base = stem
				}
				setToken(tok, base+suffix, true)
				continue
			}
		}

		// W4: an institution head before the apostrophe ("Millî Eğitim Bakanlığı'na", "Ankara Üniversitesi'ne",
		// "Anadolu Ajansı'nın", "Başbakanlık'a" -> "Başbakanlığa"). Titles of books and films that end in a museum,
		// a hotel, a club, an association or an institute are kept ("Masumiyet Müzesi'ni okudum", "Ölü Ozanlar
		// Derneği'ni izledim").
		prevLow := ""
		if prevTok != nil {
			prevLow = prevTok.Low
		}
		if IsInstitutionHead(lowStem, prevLow) {
			workProneHead := lowStem == "müzesi" || lowStem == "oteli" || lowStem == "derneği" || lowStem == "enstitüsü" ||
				lowStem == "çiftliği" || lowStem == "kulübü" || lowStem == "tiyatrosu"
			if workProneHead && hasReadingOrWatchingCtx {
				continue
			}
			// A lowercase institution head loses its apostrophe without a capital forced on it (step 6a decides).
			setToken(tok, JoinWithoutApostrophe(stem, suffix, isMorphWord), isCap(tok))
			continue
		}
		if isCap(tok) && bareInstitutionOffices[lowStem] {
			setToken(tok, JoinWithoutApostrophe(stem, suffix, isMorphWord), true)
			continue
		}

		// W5: an apostrophe misplaced inside a proper name's suffixes ("Selinin'kini" -> "Selin'inkini", "Ahmetin'ki"
		// -> "Ahmet'inki", "İstanbulda'ki" -> "İstanbul'daki", "Türkiyemiz'in" -> "Türkiye'mizin", "Ankarad'an" ->
		// "Ankara'dan"). A valid stem on its own ("Savaşı'nın", "Rüyası'na", "Bayramı'na") or an unknown foreign
		// name ("Jimin'in") is never cut.
		_, stemName := known.NameEntry(lowStem, 0)
		_, stemGiven := known.GivenNameFor(lowStem)
		_, stemSurname := known.SurnameFor(lowStem)
		_, stemAmbig := known.AmbigNameFor(lowStem)
		if !stemName && !stemGiven && !stemSurname && !stemAmbig {
			isKiOrCop := kiOrCopRE.MatchString(lowSuffix)
			isPossReloc := possRelocRE.MatchString(lowStem) || (!isMorphWord(lowStem) && daEndRE.MatchString(lowStem))
			if isKiOrCop || isPossReloc {
				joinedLow := []rune(lowStem + lowSuffix)
				for cut := utf8.RuneCountInString(lowStem) - 1; cut >= 3; cut-- {
					base := string(joinedLow[:cut])
					combinedSuf := string(joinedLow[cut:])
					ne, hasNe := known.NameEntry(base, 0)
					if hasNe && ne.NoApos {
						continue
					}
					disp, hasDisp := "", false
					if hasNe {
						disp, hasDisp = ne.Name, true
					} else if isKiOrCop {
						if disp, hasDisp = known.GivenNameFor(base); !hasDisp {
							disp, hasDisp = known.SurnameFor(base)
						}
					}
					if !hasDisp || disp == "" {
						continue
					}
					if NonApostropheSuffixRE.MatchString(combinedSuf) {
						continue
					}
					if ProperInflectionalSuffixRE.MatchString(combinedSuf) && IsPhonologicallyValidProperSuffix(tr.Lower(disp), combinedSuf, ProperSuffixOptions{}) {
						setToken(tok, disp+"'"+combinedSuf, true)
						break
					}
				}
				if tok.Locked {
					continue
				}
			}
		}

		// W6: a common noun with a wrong apostrophe.
		lowJoined := lowStem + lowSuffix
		softenedJoined := JoinWithoutApostrophe(stem, suffix, isMorphWord)
		lowSoftJoined := tr.Lower(softenedJoined)
		morphValidJoined := isMorphWord(lowJoined) || isMorphWord(lowSoftJoined)
		effJoinedCount := math.Max(lexCount(lowJoined), lexCount(lowSoftJoined))
		if !isCap(tok) {
			// A lowercase stem'suffix.
			// 1) A proper name that is always one gets its capital ("ahmet'e" -> "Ahmet'e", "ankara'dan" ->
			// "Ankara'dan").
			if nameEnt, ok := lookupName(lowStem, sentenceHasAsciiFixes); ok {
				fixedSuf, harmOK := HarmonizeASCIISuffix(tr.Lower(nameEnt.Name), lowSuffix, ProperSuffixOptions{})
				if !harmOK {
					fixedSuf = suffix
				}
				setToken(tok, formatProper(nameEnt.Name, fixedSuf, !nameEnt.NoApos), true)
				continue
			}
			// 2) A Turkish given name or surname with an apostrophe that cannot be a common word ("emre'yi" ->
			// "Emre'yi", "aslı'yı" -> "Aslı'yı").
			personDisp, isPerson := known.GivenNameFor(lowStem)
			if !isPerson {
				personDisp, isPerson = known.SurnameFor(lowStem)
			}
			if isPerson && personDisp != "" && !morphValidJoined {
				setToken(tok, personDisp+"'"+suffix, true)
				continue
			}
			// 2b) An honorific after a person's name ("Mustafa ağa'nın" is step 7d's "Mustafa Ağa'nın"; the apostrophe
			// stays).
			if PostHonorifics.Has(lowStem) && prevTok != nil {
				_, prevGiven := known.GivenNameFor(prevTok.Low)
				_, prevName := known.NameEntry(prevTok.Low, 0)
				if prevGiven || prevName || (isCap(prevTok) && !prevTok.AtStart) {
					continue
				}
			}
			// 3) An ordinary common noun (not ambiguous, not a foreign loan like "single'ı" or "rock'ın") loses its
			// apostrophe, softened ("kitap'ı" -> "kitabı").
			lowAposCount := lexCount(lowStem + "'" + lowSuffix)
			_, ambigStem := known.AmbigNameFor(lowStem)
			if !ambigStem && morphValidJoined && (lowAposCount == 0 || lowAposCount*3 < effJoinedCount) {
				setToken(tok, softenedJoined, true)
				continue
			}
			// 4) An ambiguous lowercase stem'suffix ("deniz'e girmeye" or "deniz'e söyle"): the set phrases of
			// common words are settled here; the rest wait for step 8f, so that step 7 (titles, honorifics and
			// person chains like "dr. şebnem koç'tan") locks surnames first.
			if ambigStem && morphValidJoined {
				var nextTok *SentenceToken
				if i+1 < len(tokens) {
					nextTok = tokens[i+1]
				}
				commonCue := (lowStem == "deniz" && nextTok != nil && denizCueRE.MatchString(nextTok.Low)) ||
					(lowStem == "gül" && nextTok != nil && gulCueRE.MatchString(nextTok.Low))
				if commonCue {
					setToken(tok, softenedJoined, true)
					continue
				}
			}
		} else if tok.AtStart {
			// A common noun capitalized at a sentence's start with a wrong apostrophe ("Babam'ın", "Toplantı'ya",
			// "Ofis'te", "Okul'da", "Kitap'ı" -> "Kitabı"). Proper places, institutions and leagues ("Bebek'te",
			// "Moda'da", "Boğaz'da", "Meclis'te", "Lig'inde") are kept by the count of the apostrophe form (at most
			// 25).
			startAposCount := lexCount(lowStem + "'" + lowSuffix)
			if !stemName && !stemAmbig && !stemGiven && !stemSurname &&
				!Months.Has(lowStem) &&
				!Weekdays.Has(lowStem) &&
				capShare(lowStem) < 0.55 &&
				morphValidJoined &&
				effJoinedCount >= 10 &&
				startAposCount <= 25 &&
				startAposCount*8 < effJoinedCount {
				setToken(tok, softenedJoined, true)
				continue
			}
		}
	}

	// 5. Conjunctions capitalized inside capitalized titles ("Radyo Ve Televizyon", "Leyla İle Mecnun").
	for i := 1; i < len(tokens)-1; i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked || tok.AtStart {
			continue
		}
		if tok.Eff != "Ve" && tok.Eff != "İle" {
			continue
		}
		prev := tokens[i-1]
		next := tokens[i+1]
		if !prev.IsWord || !next.IsWord || !adjSpace(prev, tok) || !adjSpace(tok, next) {
			continue
		}
		if isCap(prev) && !isAllCaps(prev) && isCap(next) && !isAllCaps(next) {
			setToken(tok, tr.Lower(tok.Eff), true)
		}
	}

	// 6. Institutions, places and landmarks, directions and events made of several words.
	// 6a. Institutions ("Boğaziçi üniversitesinde", "Marmara üniversitesine", "Eskişehir Osmangazi
	// üniversitesinde", "Yıldız pastanesinden", "Ankara Üniversitesi siyasal bilgiler fakültesi"). Careful on
	// mixed-case text: when the head is already capitalized ("Yahudi kültür Birliği") or a phrase already has
	// capitalized words before a lowercase tail not in PHRASE_NOAPOS ("Dicle Gençlik kulübü", "Sırp Listesi
	// partisini", "Özel İhtisas komisyonu", "Londra Büyük elçiliği", "PTT genel müdürlüğünde", "Boston Üniversitesi
	// iletişim bölümünden"), no capitals are guessed.
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked {
			continue
		}
		// Lowercase words to the left of a head already capitalized are never guessed ("Yahudi kültür Birliği").
		if isCap(tok) {
			continue
		}
		hSplit, ok := SplitHeadWord(tok.Low, InstitutionHeads)
		if !ok {
			continue
		}
		prev := tokens[i-1]
		if !prev.IsWord || !adjSpace(prev, tok) {
			continue
		}
		if !IsInstitutionHead(hSplit.Head, prev.Low) {
			continue
		}

		// A genitive or inflected token is never an institution's anchor ("Sinan Güler'in polikliniğinde",
		// "Atatürk Lisesinin ...").
		if strings.Contains(prev.Low, "'") || strings.Contains(prev.Raw, "'") || strings.Contains(prev.Raw, "’") {
			continue
		}
		if s, ok := SplitHeadWord(prev.Low, InstitutionHeads); ok && s.Suffix != "" {
			continue
		}
		// A generic village, neighbourhood or hotel room ("Eskimo köy odası") is not a chamber.
		if hSplit.Head == "odası" && (prev.Low == "köy" || prev.Low == "mahalle" || prev.Low == "semt" || prev.Low == "otel") {
			continue
		}
		startIdx := i
		hasProperAnchor := false
		for j := i - 1; j >= max(0, i-6); j-- {
			cur := tokens[j]
			right := tokens[j+1]
			if !cur.IsWord || !adjSpace(cur, right) || strings.Contains(cur.Low, "'") || strings.Contains(cur.Raw, "'") || strings.Contains(cur.Raw, "’") {
				break
			}
			if s, ok := SplitHeadWord(cur.Low, InstitutionHeads); ok && s.Suffix != "" {
				break
			}
			lw := cur.Low
			// A conjunction "ve" or "ile" inside is let in only when something valid is to its left.
			if (lw == "ve" || lw == "ile") && j > 0 && tokens[j-1].IsWord && adjSpace(tokens[j-1], cur) {
				startIdx = j
				continue
			}
			neCur, hasNeCur := known.NameEntry(lw, 0)
			_, isAcronym := known.AcronymFor(lw)
			isNameToken := (hasNeCur && !neCur.NoApos) ||
				isAcronym ||
				(isCap(cur) && !cur.AtStart && !NonInstPrefix[lw]) ||
				(isCap(cur) && cur.AtStart && ((hasNeCur && !neCur.NoApos) || GenericInstModifiers[lw] || capShare(lw) >= 0.45))
			isInstMod := GenericInstModifiers[lw] || InstitutionHeads[lw]
			_, isAmbig := known.AmbigNameFor(lw)
			isAmbigShopName := j == i-1 && isAmbig && shopHeads[hSplit.Head]
			// A strong proper anchor, not a generic modifier like "tıp" or "devlet".
			if isNameToken && !NonInstPrefix[lw] && !GenericInstModifiers[lw] {
				hasProperAnchor = true
				startIdx = j
				continue
			}
			if isAmbigShopName {
				hasProperAnchor = true
				startIdx = j
				continue
			}
			if InstitutionHeads[lw] && (cur.Locked || isCap(cur)) {
				// After a university or institution already found ("Ankara Üniversitesi siyasal bilgiler fakültesi").
				hasProperAnchor = true
				startIdx = j
				break
			}
			if isInstMod {
				// Particular capitalized institution prefixes ("Bankacılık düzenleme ve denetleme kurumunun", "Sosyal
				// Güvenlik").
				if isCap(cur) && !cur.AtStart && !NonInstPrefix[lw] {
					hasProperAnchor = true
				}
				if anchorInstModifiers[lw] {
					hasProperAnchor = true
				}
				startIdx = j
				continue
			}
			break
		}
		// Conjunctions ("ve", "ile") at the front are left out.
		for startIdx < i && (tokens[startIdx].Low == "ve" || tokens[startIdx].Low == "ile") {
			startIdx++
		}
		// Bare generic phrases ("tıp fakültesi", "hukuk fakültesi", "devlet hastanesi", "ilçe belediyesi") and
		// indefinite appositives ("bir Irak hastanesinde", "bir Pakistan askeri mahkemesi", "Sony Mobile şirketi",
		// "Beta Gençlikspor kulübü") are kept.
		if startIdx >= i || !hasProperAnchor {
			continue
		}
		if startIdx > 0 && tokens[startIdx-1].Low == "bir" {
			continue
		}
		if strings.HasSuffix(tokens[startIdx].GapBefore, "-") {
			continue
		}

		// Mixed case: when the writer already capitalized part of an unknown phrase and left its head lowercase
		// ("Dicle Gençlik kulübü", "Sırp Listesi partisini", "Özel İhtisas komisyonu", "Londra Büyük elçiliği", "PTT
		// genel müdürlüğünde", "Boston Üniversitesi iletişim bölümünden", "Eskimo köy odası"), only an unambiguous
		// two-word "<KnownProper> <UnambigOfficialHead>" gets capitals ("Boğaziçi üniversitesinde", "Marmara
		// üniversitesine").
		hasAlreadyCapPrefix := false
		for j := startIdx; j < i; j++ {
			cur := tokens[j]
			_, curName := known.NameEntry(cur.Low, 0)
			if startsUpper(cur.Raw) && (!cur.AtStart || j > startIdx || isAllCaps(cur) || curName) {
				hasAlreadyCapPrefix = true
				break
			}
		}
		if hasAlreadyCapPrefix {
			prevEntry, prevName := known.NameEntry(prev.Low, 0)
			safeTwoWordOfficial := startIdx == i-1 && !isAllCaps(prev) && prevName && !prevEntry.NoApos && unambigMixedInstHeads[hSplit.Head]
			if !safeTwoWordOfficial {
				continue
			}
		}
		if startIdx == i-1 && GenericAloneInstHeads[hSplit.Head] && NonInstPrefix[tokens[startIdx].Low] {
			continue
		}
		// Bare brand appositives ("Sony Mobile şirketi", "Beta Gençlikspor kulübü") get no capitals without a
		// canonical Turkish institutional modifier before them ("Spor Kulübü", "Jimnastik Kulübü", "Anonim
		// Şirketi").
		if strictAppositiveHeads[hSplit.Head] && !appositiveInstMods[prev.Low] {
			continue
		}
		if hSplit.Head == "bölümü" {
			// "Bölümü" is capitalized only after a university, faculty or department chain ("Ankara Üniversitesi ...
			// Bölümü").
			hasUnivAnchor := false
			for j := startIdx; j < i; j++ {
				if l := tokens[j].Low; l == "üniversitesi" || l == "fakültesi" || l == "edebiyatı" {
					hasUnivAnchor = true
					break
				}
			}
			if !hasUnivAnchor {
				continue
			}
		}
		// A two-word "<Nationality> kulübü/başkanlığı/kütüphanesi/..." is generic ("Alman kulübü", "Amerikan
		// başkanlığı").
		if startIdx == i-1 && nationalityGenericMod[tokens[startIdx].Low] && genericNatInstHeads[hSplit.Head] {
			continue
		}
		// Lowercase appositives where an unknown foreign word comes before a generic "belediyesi", "hastanesi",
		// "mahkemesi", "okulu", "müzesi" or "kütüphanesi" ("Kurbin belediyesinde", "Polimer kütüphanesi",
		// "Pontevedra müzesinin") are kept.
		if startIdx == i-1 && foreignAppositiveHeads[hSplit.Head] {
			_, prevName := known.NameEntry(prev.Low, 0)
			_, prevAmbig := known.AmbigNameFor(prev.Low)
			if !prevName && !prevAmbig && !GenericInstModifiers[prev.Low] {
				continue
			}
		}
		// A bare "<generic adjective> bilimler fakültesi" ("siyasi bilimler fakültesi") gets no capitals without a
		// university or a proper anchor.
		if hSplit.Head == "fakültesi" && tokens[startIdx].Low == "bilimler" {
			continue
		}

		for j := startIdx; j < i; j++ {
			cur := tokens[j]
			if cur.Locked {
				continue
			}
			if cur.Low == "ve" || cur.Low == "ile" {
				setToken(cur, cur.Low, true)
			} else {
				ne, hasNe := lookupName(cur.Low, sentenceHasAsciiFixes)
				if acro, ok := known.AcronymFor(cur.Low); ok {
					setToken(cur, acro, true)
				} else if hasNe {
					setToken(cur, ne.Name, true)
				} else {
					setToken(cur, CapTr(cur.Eff), true)
				}
			}
		}
		// An institution's head never takes an apostrophe (TDK).
		setToken(tok, CapTr(hSplit.Head)+hSplit.Suffix, true)
	}

	// 6a-officer. Officer titles of institutions ("Türk Dil Kurumu Başkanı'na", "Ankara Üniversitesi Rektörü'nden",
	// "Emniyet Genel Müdürü'ne", "Sağlık Bakanlığı Sözcüsü'ne", "Mühendislik Fakültesi Dekanı'na"). A personal title
	// after an institution's name takes a capital and an apostrophe as an official office (TDK); a bare lowercase
	// job after a capitalized institution ("Bremen Sanat Müzesi müdürü ressam Carl Horn", "Azerbaycan Millî Meclisi
	// başkanı Resul Guliyev") is left as it is.
	for i := 1; i < len(tokens); i++ {
		prev := tokens[i-1]
		tok := tokens[i]
		if !prev.IsWord || !tok.IsWord || tok.Locked || !adjSpace(prev, tok) {
			continue
		}
		// The token before is a capitalized or locked institution or state head, without an apostrophe or a case.
		prevIsBareInst := (prev.Locked || isCap(prev)) &&
			(InstitutionHeads[prev.Low] || StateHeads[prev.Low] || prev.Low == "belediye" || prev.Low == "meclis")
		if !prevIsBareInst {
			continue
		}

		// An optional modifier of one word ("Genel Müdürü'ne", "Başkan Yardımcısı'na").
		titleIdx := i
		if OfficerTitleMods[tok.Low] && i+1 < len(tokens) && tokens[i+1].IsWord && adjSpace(tok, tokens[i+1]) {
			nextTok := tokens[i+1]
			if nextSplit, ok := SplitHeadWord(nextTok.Low, OfficerTitleHeads); ok {
				var afterNext *SentenceToken
				if i+2 < len(tokens) && adjSpace(nextTok, tokens[i+2]) {
					afterNext = tokens[i+2]
				}
				bareLowerRole := nextSplit.Suffix == "" && !isCap(tok) && !isCap(nextTok) && afterNext != nil &&
					(isCap(afterNext) || PersonRelBefore[afterNext.Low] || PreTitlesFull[afterNext.Low] || PreTitlesAbbr[afterNext.Low] || officerRoleFollowBlock[afterNext.Low])
				if !bareLowerRole {
					setToken(tok, CapTr(tok.Eff), true)
					setToken(nextTok, formatProper(CapTr(nextSplit.Head), nextSplit.Suffix, true), true)
					i++
					continue
				}
			}
		}
		if oSplit, ok := SplitHeadWord(tok.Low, OfficerTitleHeads); ok {
			var afterTok *SentenceToken
			if i+1 < len(tokens) && adjSpace(tok, tokens[i+1]) {
				afterTok = tokens[i+1]
			}
			bareLowerRole := oSplit.Suffix == "" && !isCap(tok) && afterTok != nil &&
				(isCap(afterTok) || PersonRelBefore[afterTok.Low] || PreTitlesFull[afterTok.Low] || PreTitlesAbbr[afterTok.Low] || officerRoleFollowBlock[afterTok.Low])
			if !bareLowerRole {
				setToken(tokens[titleIdx], formatProper(CapTr(oSplit.Head), oSplit.Suffix, true), true)
			}
		}
	}

	// 6b. Places, monuments, landmarks, states and events of up to four words before the head ("Sapanca gölü",
	// "Rumeli hisarında", "Marmara bölgesinde", "Doğu Anadolu bölgesinde", "Efes antik kentinde", "Büyük Menderes
	// nehrinde", "Fatih Sultan Mehmet köprüsünden", "Yedigöller millî parkında", "Kore savaşı").
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked {
			continue
		}
		prev := tokens[i-1]
		if !prev.IsWord || !adjSpace(prev, tok) || strings.Contains(prev.Low, "'") || strings.Contains(prev.Raw, "'") ||
			strings.Contains(prev.Raw, "’") || strings.HasSuffix(prev.GapBefore, "-") {
			continue
		}
		if s, ok := SplitHeadWord(prev.Low, InstitutionHeads); ok && s.Suffix != "" {
			continue
		}
		if isAllCaps(prev) {
			continue
		}

		headSplit, ok := SplitHeadWord(tok.Low, GeoHeads)
		if !ok {
			headSplit, ok = SplitHeadWord(tok.Low, StateHeads)
		}
		if !ok {
			headSplit, ok = SplitHeadWord(tok.Low, EventHeads)
		}
		if !ok {
			continue
		}
		if isCap(tok) && !headSplit.HadApostrophe {
			continue
		}
		// A lowercase predicate with a copula ("Atatürk Lisesinin sokağıdır", "Türkiye'nin en büyük gölüdür") is not a
		// proper name.
		if !isCap(tok) && isCap(prev) && copulaSuffixRE.MatchString(headSplit.Suffix) {
			continue
		}
		// "kenti" only after "antik" ("Efes Antik Kenti", "Patara Antik Kenti").
		if headSplit.Head == "kenti" && prev.Low != "antik" {
			continue
		}
		// "boğazı" or "sokağı" after a person's name ("Ahmet boğazında") is anatomy or a common word, not a place.
		if headSplit.Head == "boğazı" || headSplit.Head == "sokağı" {
			_, prevGiven := known.GivenNameFor(prev.Low)
			_, prevName := known.NameEntry(prev.Low, 0)
			if prevGiven && !prevName {
				continue
			}
		}

		// Up to four tokens to the left that are proper names or modifiers.
		startIdx := i
		hasStrongProperAnchor := false
		hasCapAnchor := false
		for j := i - 1; j >= max(0, i-4); j-- {
			cur := tokens[j]
			right := tokens[j+1]
			if !cur.IsWord || !adjSpace(cur, right) || strings.Contains(cur.Low, "'") || strings.Contains(cur.Raw, "'") ||
				strings.Contains(cur.Raw, "’") || strings.HasSuffix(cur.GapBefore, "-") {
				break
			}
			if s, ok := SplitHeadWord(cur.Low, InstitutionHeads); ok && s.Suffix != "" {
				break
			}
			lw := cur.Low
			if ne, ok := lookupName(lw, sentenceHasAsciiFixes); ok && !ne.NoApos && !NonInstPrefix[lw] {
				hasStrongProperAnchor = true
				startIdx = j
				continue
			}
			if cur.Locked && !NonInstPrefix[lw] {
				hasStrongProperAnchor = true
				startIdx = j
				continue
			}
			if isCap(cur) && !cur.AtStart && !NonInstPrefix[lw] && !isAllCaps(cur) {
				hasCapAnchor = true
				startIdx = j
				continue
			}
			if geoModifiers[lw] {
				if j == i-1 && (lw == "büyük" || lw == "küçük" || lw == "yeni" || lw == "eski" || lw == "devlet" || lw == "uluslararası") {
					// A generic phrase of two words like "büyük nehir", "eski kale" or "yeni köprü" is not started.
					break
				}
				startIdx = j
				continue
			}
			if j == i-1 && cur.AtStart && capShare(lw) >= 0.5 && !NonInstPrefix[lw] {
				hasCapAnchor = true
				startIdx = j
				continue
			}
			break
		}

		if startIdx >= i {
			continue
		}
		if startIdx > 0 && tokens[startIdx-1].Low == "bir" {
			continue
		}
		if strictKnownGeoHeads[headSplit.Head] {
			if !hasStrongProperAnchor {
				continue
			}
		} else if !(hasStrongProperAnchor || hasCapAnchor) {
			continue
		}
		if startIdx == i-1 && NonInstPrefix[prev.Low] {
			continue
		}
		if headSplit.Head == "mahallesi" && startIdx == i-1 && quarterNationalities[prev.Low] {
			continue
		}
		if (headSplit.Head == "istasyonu" || headSplit.Head == "havaalanı" || headSplit.Head == "havalimanı") && startIdx == i-1 && stationKinds[prev.Low] {
			continue
		}

		for j := startIdx; j < i; j++ {
			cur := tokens[j]
			if cur.Locked {
				continue
			}
			if ne, ok := lookupName(cur.Low, sentenceHasAsciiFixes); ok {
				setToken(cur, ne.Name, true)
			} else {
				setToken(cur, CapTr(cur.Eff), true)
			}
		}
		setToken(tok, formatProper(CapTr(headSplit.Head), headSplit.Suffix, true), true)
	}

	// (A blind "<direction> + <place>" step was taken out: official regions like "Batı Anadolu", "Doğu Karadeniz",
	// "Kuzey Amerika", "Güney Kore" and "Orta Asya" are step 2's canonical or corpus-checked phrases, and
	// descriptions like "orta Şili", "güney Şili" or "güney Hint Okyanusu" stay lowercase (TDK).)

	// 7. Titles, ranks, honorifics, kinship, media titles and chains of names.
	// 7a. Chains of abbreviated titles before a person's name ("prof. dr. halil inalcık'ın", "op. dr. sinan
	// güler'in", "dr. öğr. üyesi ahmet kaya", "av. melike sönmez", "dr. şebnem koç'tan", "Dekan doç. dr. barış
	// sezer").
	for i := 0; i < len(tokens)-1; i++ {
		t0 := tokens[i]
		if !t0.IsWord || !PreTitlesAbbr[t0.Low] {
			continue
		}
		// The abbreviated titles in a row ("Prof. Dr.", "Op. Dr.", "Doç. Dr.", "Yrd. Doç. Dr.", "Dr. Öğr.").
		endTitle := i
		for endTitle+1 < len(tokens) && adjDotSpace(tokens[endTitle], tokens[endTitle+1]) && PreTitlesAbbr[tokens[endTitle+1].Low] {
			endTitle++
		}
		// An optional "Üyesi" after "Öğr." ("Dr. Öğr. Üyesi Ahmet Kaya").
		uyesiIdx := -1
		nameIdx := endTitle + 1
		if nameIdx+1 < len(tokens) &&
			(tokens[endTitle].Low == "öğr" || tokens[endTitle].Low == "ogr") &&
			adjDotSpace(tokens[endTitle], tokens[nameIdx]) &&
			tokens[nameIdx].Low == "üyesi" &&
			adjSpace(tokens[nameIdx], tokens[nameIdx+1]) {
			uyesiIdx = nameIdx
			nameIdx = nameIdx + 1
		} else if nameIdx >= len(tokens) || !adjDotSpace(tokens[endTitle], tokens[nameIdx]) {
			continue
		}
		firstTok := tokens[nameIdx]
		g, hasG := resolveGivenName(firstTok)
		if !hasG {
			if s, ok := resolveSurname(firstTok, false); ok {
				g, hasG = givenMatch{display: s.display, suffix: s.suffix, hadApos: s.hadApos}, true
			} else if ne, ok := known.NameEntry(firstTok.Low, 0); ok && !ne.NoApos {
				g, hasG = givenMatch{display: ne.Name}, true
			}
		}
		firstIsPerson := hasG || (isCap(firstTok) && !firstTok.AtStart)
		if !firstIsPerson {
			continue
		}

		// A full rank or "Sayın" right before the chain gets its capital too ("Dekan doç. dr. barış sezer", "Sayın
		// prof. dr.").
		if i > 0 && adjSpace(tokens[i-1], t0) && (PreTitlesFull[tokens[i-1].Low] || tokens[i-1].Low == "sayın") {
			setToken(tokens[i-1], CapTr(tokens[i-1].Eff), true)
		}
		for k := i; k <= endTitle; k++ {
			setToken(tokens[k], CapTr(tokens[k].Eff), true)
		}
		if uyesiIdx != -1 {
			setToken(tokens[uyesiIdx], "Üyesi", true)
		}
		if hasG && !firstTok.Locked {
			setToken(firstTok, formatProper(g.display, g.suffix, true), true)
		}
		// The surname after it gets its capital too ("Melike Sönmez", "Halil İnalcık'ın", "Sinan Güler'in", "Şebnem
		// Koç'tan", "Barış Sezer", "Ece Çınar").
		surIdx := nameIdx + 1
		if surIdx < len(tokens) && adjSpace(firstTok, tokens[surIdx]) && (!hasG || g.suffix == "") {
			surTok := tokens[surIdx]
			if sur, ok := resolveSurname(surTok, true); ok && !surTok.Locked {
				setToken(surTok, formatProper(sur.display, sur.suffix, true), true)
			} else if !surTok.Locked && surTok.IsWord && !isCap(surTok) && strings.Contains(surTok.Low, "'") {
				ap := runeIndex(surTok.Low, "'")
				setToken(surTok, CapTr(sliceRunes(surTok.Eff, 0, ap))+sliceFrom(surTok.Eff, ap), true)
			}
		}
		if uyesiIdx != -1 {
			i = uyesiIdx
		} else {
			i = endTitle
		}
	}

	// 7b. Full titles and ranks before a name ("bayan elif tan", "cumhurbaşkanı ismet inönü'nün", "albay nurettin
	// şahin'in", "Kaymakam sena tunç'a", "Hâkim nermin oral", "Kaptan rüştü alkan", "Başkomiser nazlı ekinci",
	// "mimar sinanın", "Bay Kaya").
	for i := 0; i < len(tokens)-1; i++ {
		titleTok := tokens[i]
		if !titleTok.IsWord || !PreTitlesFull[titleTok.Low] {
			continue
		}
		nextTok := tokens[i+1]
		if !nextTok.IsWord || !adjSpace(titleTok, nextTok) {
			continue
		}
		g, hasG := resolveGivenName(nextTok)
		if !hasG && surnameFirstTitles[titleTok.Low] {
			if s, ok := resolveSurname(nextTok, false); ok {
				g, hasG = givenMatch{display: s.display, suffix: s.suffix, hadApos: s.hadApos}, true
			}
		}
		if !hasG {
			continue
		}
		var surTok *SentenceToken
		if i+2 < len(tokens) && adjSpace(nextTok, tokens[i+2]) {
			surTok = tokens[i+2]
		}
		var sur surnameMatch
		hasSur := false
		if g.suffix == "" && surTok != nil {
			sur, hasSur = resolveSurname(surTok, true)
		}
		_, firstName := known.NameEntry(tr.Lower(g.display), 0)
		strongFirst := firstName || hasSur || strongFirstTitles[titleTok.Low]
		if !strongFirst {
			continue
		}

		// A two-word rank's first word gets its capital too ("belediye başkanı ...", "dışişleri bakanı ...", "genel
		// müdür ...").
		if i > 0 && adjSpace(tokens[i-1], titleTok) && rankPrefixes[tokens[i-1].Low] {
			setToken(tokens[i-1], CapTr(tokens[i-1].Eff), true)
		}
		setToken(titleTok, CapTr(titleTok.Eff), true)
		if !nextTok.Locked {
			setToken(nextTok, formatProper(g.display, g.suffix, true), true)
		}
		if hasSur && surTok != nil && !surTok.Locked {
			setToken(surTok, formatProper(sur.display, sur.suffix, true), true)
		}
	}

	// 7c. The vocative and formal "Sayın" before an official title or a person's name ("Sayın Başkan", "sayın
	// başkan", "Sayın Genel Müdür", "Sayın Belediye Başkanı", "Sayın Başkan'a", "sayın ahmet yılmaz").
	for i := 0; i < len(tokens)-1; i++ {
		sayin := tokens[i]
		role := tokens[i+1]
		if !sayin.IsWord || !role.IsWord || sayin.Low != "sayın" || !adjSpace(sayin, role) {
			continue
		}
		roleSplit, hasRoleSplit := SplitHeadWord(role.Low, SayinTitles)
		if SayinTitles[role.Low] {
			if !sayin.Locked {
				setToken(sayin, "Sayın", true)
			}
			setToken(role, CapTr(role.Eff), true)
			for k := i + 2; k <= min(i+3, len(tokens)-1); k++ {
				if !adjSpace(tokens[k-1], tokens[k]) {
					break
				}
				w := tokens[k].Low
				if SayinTitles[w] || SayinCompoundTail[w] {
					setToken(tokens[k], CapTr(tokens[k].Eff), true)
				} else {
					if tailSplit, ok := SplitHeadWord(w, SayinCompoundTail); ok {
						setToken(tokens[k], formatProper(CapTr(tailSplit.Head), tailSplit.Suffix, true), true)
					}
					break
				}
			}
		} else if hasRoleSplit && roleSplit.Suffix != "" {
			if !sayin.Locked {
				setToken(sayin, "Sayın", true)
			}
			setToken(role, formatProper(CapTr(roleSplit.Head), roleSplit.Suffix, true), true)
		} else if SayinCompoundFirst[role.Low] && i+2 < len(tokens) && adjSpace(role, tokens[i+2]) {
			t2 := tokens[i+2]
			t2Split, hasT2Split := SplitHeadWord(t2.Low, SayinCompoundTail)
			if !hasT2Split {
				t2Split, hasT2Split = SplitHeadWord(t2.Low, SayinTitles)
			}
			if SayinTitles[t2.Low] || SayinCompoundTail[t2.Low] || hasT2Split {
				if !sayin.Locked {
					setToken(sayin, "Sayın", true)
				}
				setToken(role, CapTr(role.Eff), true)
				if hasT2Split && t2Split.Suffix != "" {
					setToken(t2, formatProper(CapTr(t2Split.Head), t2Split.Suffix, true), true)
				} else {
					setToken(t2, CapTr(t2.Eff), true)
					if i+3 < len(tokens) && adjSpace(t2, tokens[i+3]) {
						t3 := tokens[i+3]
						t3Split, hasT3Split := SplitHeadWord(t3.Low, SayinCompoundTail)
						if SayinCompoundTail[t3.Low] {
							setToken(t3, CapTr(t3.Eff), true)
						} else if hasT3Split {
							setToken(t3, formatProper(CapTr(t3Split.Head), t3Split.Suffix, true), true)
						}
					}
				}
			}
		} else {
			// "sayın <PersonName> [Surname]" ("sayın ahmet yılmaz", "sayın ayşe hanım").
			g, hasG := resolveGivenName(role)
			if hasG {
				_, isName := known.NameEntry(tr.Lower(g.display), 0)
				if isName || !commonGivenAdjectives[tr.Lower(g.display)] {
					if !sayin.Locked {
						setToken(sayin, "Sayın", true)
					}
					if !role.Locked {
						setToken(role, formatProper(g.display, g.suffix, true), true)
					}
					if g.suffix == "" && i+2 < len(tokens) && adjSpace(role, tokens[i+2]) {
						surTok := tokens[i+2]
						if sur, ok := resolveSurname(surTok, true); ok && !surTok.Locked {
							setToken(surTok, formatProper(sur.display, sur.suffix, true), true)
						}
					}
				}
			}
		}
	}

	// 7d. Honorifics and titles after a name ("Ayşe hanım", "Mahmut efendiye", "kerem bey", "orhan beyde", "Kâzım
	// karabekir paşanın", "Ahmet hocaya", "Mehmet ustaya", "Ali çavuşun", "Temel kaptana").
	for i := 1; i < len(tokens); i++ {
		honTok := tokens[i]
		if !honTok.IsWord || honTok.Locked {
			continue
		}
		honSplit, hasHon := SplitHonorific(honTok.Low)
		var postTitleSplit TitleSplit
		hasPostTitle := false
		if !hasHon {
			postTitleSplit, hasPostTitle = SplitPostTitle(honTok.Low)
		}
		hSplit := honSplit
		if !hasHon {
			hSplit = postTitleSplit
		}
		if !hasHon && !hasPostTitle {
			continue
		}
		prev := tokens[i-1]
		if !prev.IsWord || !adjSpace(prev, honTok) {
			continue
		}

		// A bare occupational title after a name ("hoca", "usta", "öğretmen", "kaptan", "sultan", "han") is never
		// capitalized as a predicate ("Ahmet öğretmen oldu", "Mehmet usta gibi") or without an anchor.
		if hasPostTitle {
			var afterTok *SentenceToken
			if i+1 < len(tokens) && adjSpace(honTok, tokens[i+1]) {
				afterTok = tokens[i+1]
			}
			if postTitleSplit.Suffix == "" && afterTok != nil && predicateAfterBareTitle[afterTok.Low] {
				continue
			}
			// A person's name must come before the title, without doubt ("köy hocası", "genç öğretmen" and "bir
			// kaptan" are not).
			_, prevGiven := known.GivenNameFor(prev.Low)
			_, prevName := known.NameEntry(prev.Low, 0)
			prevIsStrongPerson := (prevGiven && !commonGivenAdjectives[prev.Low]) || (prevName && prevGiven)
			if !prevIsStrongPerson {
				continue
			}
			// "han" and "şah" need a suffix, a capital before them or a given name.
			if (postTitleSplit.Head == "han" || postTitleSplit.Head == "şah") && postTitleSplit.Suffix == "" && !isCap(prev) && !prevGiven {
				continue
			}
		}

		// A bare "-ı" or "-i" ("Sancak Beyi Berham Bey", "Osmanlı beyi", "Türk hanımı") is an accusative ("Ayşe
		// Hanım'ı", "Kerem Bey'i") only after a real given name and not before one.
		if (hSplit.Suffix == "ı" || hSplit.Suffix == "i") && !hSplit.HadApostrophe {
			if _, ok := known.GivenNameFor(prev.Low); !ok {
				continue
			}
			var afterHon *SentenceToken
			if i+1 < len(tokens) && adjSpace(honTok, tokens[i+1]) {
				afterHon = tokens[i+1]
			}
			if afterHon != nil {
				if _, ok := known.GivenNameFor(afterHon.Low); ok || isCap(afterHon) {
					continue
				}
			}
		}

		// The word before is a person's name (never an occupation at a sentence's start like "Memur bey", "Doktor
		// bey", "Hemşire hanım").
		prevDisp, hasPrevDisp := known.GivenNameFor(prev.Low)
		if !hasPrevDisp {
			prevDisp, hasPrevDisp = known.SurnameFor(prev.Low)
		}
		if !hasPrevDisp {
			prevDisp, hasPrevDisp = known.AmbigNameFor(prev.Low)
		}
		if !hasPrevDisp {
			if ne, ok := known.NameEntry(prev.Low, 0); ok && !ne.NoApos {
				prevDisp, hasPrevDisp = known.NameFor(prev.Low, 0)
			}
		}
		if !hasPrevDisp && isCap(prev) && !prev.AtStart && !NonInstPrefix[prev.Low] && !PreTitlesFull[prev.Low] {
			prevDisp, hasPrevDisp = prev.Eff, true
		}
		if !hasPrevDisp || prevDisp == "" || NonInstPrefix[prev.Low] || PreTitlesFull[prev.Low] {
			continue
		}
		// "sayın bey", "genç bey", "yaşlı hanım", "güzel hanım" and "küçük bey" are not names.
		if notNameBeforeHonorific[prev.Low] {
			continue
		}

		if !prev.Locked {
			setToken(prev, prevDisp, true)
		}
		// A given name before a surname gets its capital too ("Kâzım karabekir paşanın").
		if i >= 2 && adjSpace(tokens[i-2], prev) && !tokens[i-2].Locked {
			p2 := tokens[i-2]
			p2Disp, ok := known.GivenNameFor(p2.Low)
			if !ok {
				p2Disp, ok = known.NameFor(p2.Low, 0)
			}
			_, prevSurname := known.SurnameFor(prev.Low)
			if ok && p2Disp != "" && !NonInstPrefix[p2.Low] && (isCap(p2) || prevSurname) {
				setToken(p2, p2Disp, true)
			}
		}
		setToken(honTok, formatProper(hSplit.DisplayHead, hSplit.Suffix, true), true)
	}

	// 7e. Kinship ("tülay ablama", "zehra halamın", "rıza enişteme", "hatice ninemin"): the kinship word stays
	// lowercase (TDK) and the name before it gets its capital.
	for i := 1; i < len(tokens); i++ {
		kin := tokens[i]
		if !kin.IsWord || !IsKinshipWord(kin.Low) {
			continue
		}
		prev := tokens[i-1]
		if !prev.IsWord || prev.Locked || !adjSpace(prev, kin) {
			continue
		}
		disp, ok := known.GivenNameFor(prev.Low)
		if !ok {
			disp, ok = known.NameFor(prev.Low, 0)
		}
		if ok && disp != "" && !NonInstPrefix[prev.Low] && !commonGivenAdjectives[prev.Low] && prev.Low != "küçük" && prev.Low != "büyük" && prev.Low != "sevgili" && prev.Low != "canım" {
			setToken(prev, disp, true)
		}
	}

	// 7f. A relationship or an occupation before a person's name ("arkadaşım melek", "komşumuz baharın", "kardeşim
	// ateş", "arkadaşım umut", "genç yazar defne arslan", "adı elif demirtaş").
	for i := 0; i < len(tokens)-1; i++ {
		rel := tokens[i]
		if !rel.IsWord || !PersonRelBefore[rel.Low] {
			continue
		}
		nameTok := tokens[i+1]
		if !nameTok.IsWord || !adjSpace(rel, nameTok) {
			continue
		}
		g, ok := resolveGivenName(nameTok)
		if !ok {
			continue
		}
		var surTok *SentenceToken
		if g.suffix == "" && i+2 < len(tokens) && adjSpace(nameTok, tokens[i+2]) {
			surTok = tokens[i+2]
		}
		var sur surnameMatch
		hasSur := false
		if surTok != nil {
			sur, hasSur = resolveSurname(surTok, true)
		}
		// "kızım mutlu oldu", "oğlum zeki bir çocuk" and "yazar metin boyunca" are not names, without a surname after
		// them or a name that is a person's for sure.
		bareLow := tr.Lower(g.display)
		if g.suffix == "" && !hasSur && commonGivenAdjectives[bareLow] && !personalAmbigNames[bareLow] {
			continue
		}
		if !nameTok.Locked {
			setToken(nameTok, formatProper(g.display, g.suffix, true), true)
		}
		if hasSur && surTok != nil && !surTok.Locked {
			setToken(surTok, formatProper(sur.display, sur.suffix, true), true)
		}
	}

	// 7g. Titles of publications ("varlık dergisine") and an author's classic novel ("Tanpınar'ın huzurunu okumak").
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord {
			continue
		}
		prev := tokens[i-1]
		if prev.IsWord && !prev.Locked && adjSpace(prev, tok) && IsMediaTypeWord(tok.Low) {
			if singleWordPublications[prev.Low] {
				setToken(prev, CapTr(prev.Low), true)
			}
		}
		// "<Author>'ın <SingleWordWork>[suffix]" with a cue of reading.
		if prev.IsWord && adjSpace(prev, tok) && isCap(prev) && authorGenitiveRE.MatchString(prev.Low) {
			for _, work := range singleWordWorks.Words() {
				if !strings.HasPrefix(tok.Low, work) || utf8.RuneCountInString(tok.Low) <= utf8.RuneCountInString(work) {
					continue
				}
				suf := tok.Low[len(work):]
				if !ProperInflectionalSuffixRE.MatchString(suf) || !IsPhonologicallyValidProperSuffix(work, suf, ProperSuffixOptions{}) {
					continue
				}
				// A cue of reading or literature elsewhere in the sentence.
				hasReadingCtx := false
				for idx, t := range tokens {
					if idx != i && t.IsWord && readingCtxRE.MatchString(t.Low) {
						hasReadingCtx = true
						break
					}
				}
				if hasReadingCtx {
					setToken(tok, CapTr(work)+"'"+suf, true)
					break
				}
			}
		}
	}

	// 7h. A first name and a surname, and names joined by "ile" or "ve" ("hüseyin çelik", "hatice şahin", "cengiz
	// ateş", "hasan tekin", "erkin bulut", "Elif kayanın", "barış Manço'nun", "yaşar kemalin", "barış mançonun",
	// "yunus emrenin", "ece ile ali", "nur ile yavuz", "aylin ile doruk", "atakan ile ece şen").
	for i := 0; i < len(tokens)-1; i++ {
		t1 := tokens[i]
		t2 := tokens[i+1]
		if !t1.IsWord || !t2.IsWord || !adjSpace(t1, t2) {
			continue
		}

		// Case 1: t1 is a strong, capitalized or unconditional Turkish first name and t2 a surname, bare or with a
		// suffix.
		t1Uncond, hasT1Uncond := known.NameEntry(t1.Low, 0)
		t1Given, hasT1Given := known.GivenNameFor(t1.Low)
		t1StrongFirst := (hasT1Uncond && !t1Uncond.NoApos && hasT1Given) ||
			(hasT1Given && isCap(t1) && !t1.AtStart) ||
			(t1.Locked && hasT1Given)

		if t1StrongFirst && !t2.Locked {
			if sur, ok := resolveSurname(t2, true); ok {
				var nextTok *SentenceToken
				if i+2 < len(tokens) && adjSpace(t2, tokens[i+2]) {
					nextTok = tokens[i+2]
				}
				// Surnames that are common words are kept from noun compounds ("çelik tencere", "güneş gözlüğü") and
				// objects of verbs ("ateş yaktı", "çiçek topladı").
				blocked := false
				if sur.isCommon {
					if nextTok != nil && commonObjectAfterSurnameVerbs[nextTok.Low] {
						blocked = true
					}
					if sur.suffix == "" && nextTok != nil && lexPair(t2.Low, nextTok.Low) >= 25 && !NonInstPrefix[nextTok.Low] {
						blocked = true
					}
					// And when the sentence starts with what may be a common noun ("Bahar geldi").
					if t1.AtStart && !hasT1Uncond {
						blocked = true
					}
				}
				if !blocked {
					if !t1.Locked {
						if hasT1Uncond {
							setToken(t1, t1Uncond.Name, true)
						} else {
							setToken(t1, t1Given, true)
						}
					}
					setToken(t2, formatProper(sur.display, sur.suffix, true), true)
					i++
					continue
				}
			}
		}

		// Case 2: t1 is an ambiguous or known first name and t2 a surname that is not ambiguous or is already
		// capitalized ("barış Manço'nun", "barış mançonun", "erkin bulut", "yaşar kemalin", "yunus emrenin"). Never
		// known.NameEntry(t2) here (it would take the country in "arda Meksika'da"), repeated words ("arda arda"), or
		// participles and verbs before a full name already written ("sona eren Okan Buruk").
		if !t1.Locked && hasT1Given && t1Given != "" && utf8.RuneCountInString(t1.Low) >= 3 && t1.Low != t2.Low && !NonInstPrefix[t1.Low] {
			var prevTok1, nextTok2 *SentenceToken
			if i > 0 && adjSpace(tokens[i-1], t1) {
				prevTok1 = tokens[i-1]
			}
			if i+2 < len(tokens) && adjSpace(t2, tokens[i+2]) {
				nextTok2 = tokens[i+2]
			}
			_, t1IsName := known.NameEntry(t1.Low, 0)
			t1IsAmbigCommon := !t1IsName && isMorphWord(t1.Low)
			// A common phrase or participle before a capitalized full name ("sona eren Okan Buruk").
			phraseBeforeBlocked := t1IsAmbigCommon && prevTok1 != nil && !isCap(prevTok1) && lexPair(prevTok1.Low, t1.Low) >= 20
			beforeFullCapName := t1IsAmbigCommon && isCap(t2) && nextTok2 != nil && isCap(nextTok2) && !isAllCaps(nextTok2)
			var surUnambig surnameMatch
			hasSurUnambig := false
			if !phraseBeforeBlocked && !beforeFullCapName {
				surUnambig, hasSurUnambig = resolveSurname(t2, false)
			}
			t2Base := strings.Split(t2.Low, "'")[0]
			t2AlreadyCapSur := false
			if !phraseBeforeBlocked && !beforeFullCapName && isCap(t2) && !t2.AtStart {
				_, t2Surname := known.SurnameFor(t2Base)
				_, t2Given := known.GivenNameFor(t2Base)
				t2AlreadyCapSur = t2Surname && !(t1IsAmbigCommon && t2Given)
			}
			if hasSurUnambig || t2AlreadyCapSur {
				setToken(t1, t1Given, true)
				if hasSurUnambig && !t2.Locked {
					setToken(t2, formatProper(surUnambig.display, surUnambig.suffix, true), true)
				}
			} else if capShare(t1.Low) >= 0.55 && !t2.Locked {
				// A Turkish first name with a high capital share (at least 0.55: "erkin", "yaşar", "yunus") and a
				// surname.
				if surAny, ok := resolveSurname(t2, true); ok {
					var nextTok *SentenceToken
					if i+2 < len(tokens) && adjSpace(t2, tokens[i+2]) {
						nextTok = tokens[i+2]
					}
					nextBlocks := surAny.isCommon && nextTok != nil &&
						(commonObjectAfterSurnameVerbs[nextTok.Low] ||
							(surAny.suffix == "" && lexPair(t2.Low, nextTok.Low) >= 25 && !NonInstPrefix[nextTok.Low]))
					if !nextBlocks {
						setToken(t1, t1Given, true)
						setToken(t2, formatProper(surAny.display, surAny.suffix, true), true)
					}
				}
			}
		}

		// Case 3: names joined by "ile" or "ve" ("ece ile ali", "nur ile yavuz", "aylin ile doruk", "atakan ile ece
		// şen").
		if i+2 < len(tokens) && (t2.Low == "ile" || t2.Low == "ve") && adjSpace(t2, tokens[i+2]) {
			t3 := tokens[i+2]
			_, t1G := known.GivenNameFor(t1.Low)
			_, t1N := known.NameEntry(t1.Low, 0)
			t1IsPerson := (t1G && (t1N || isCap(t1) || t1.Locked)) || (t1.AtStart && t1G && capShare(t1.Low) >= 0.55)
			_, t3G := known.GivenNameFor(t3.Low)
			_, t3N := known.NameEntry(t3.Low, 0)
			t3IsPerson := t3G && (t3N || isCap(t3) || t3.Locked)
			t1GivenDisp, hasT1GivenDisp := known.GivenNameFor(t1.Low)
			t3GivenDisp, hasT3GivenDisp := known.GivenNameFor(t3.Low)
			if t1IsPerson && hasT3GivenDisp && t3GivenDisp != "" && !t3.Locked {
				var nextTok *SentenceToken
				if i+3 < len(tokens) && adjSpace(t3, tokens[i+3]) {
					nextTok = tokens[i+3]
				}
				// "Ahmet ile barış yaptık" and "Ayşe ile umut doluyuz" are not names.
				_, t3Ambig := known.AmbigNameFor(t3.Low)
				idiomBlock := t3Ambig && nextTok != nil && (commonObjectAfterSurnameVerbs[nextTok.Low] || lexPair(t3.Low, nextTok.Low) >= 40)
				if !idiomBlock {
					if !t1.Locked && hasT1GivenDisp && t1GivenDisp != "" {
						setToken(t1, t1GivenDisp, true)
					}
					setToken(t3, t3GivenDisp, true)
					// An optional surname after the second name ("atakan ile ece şen").
					if nextTok != nil && !nextTok.Locked {
						if sur, ok := resolveSurname(nextTok, true); ok {
							var afterSur *SentenceToken
							if i+4 < len(tokens) && adjSpace(nextTok, tokens[i+4]) {
								afterSur = tokens[i+4]
							}
							surBlocked := sur.isCommon && afterSur != nil &&
								(commonObjectAfterSurnameVerbs[afterSur.Low] || lexPair(nextTok.Low, afterSur.Low) >= 25)
							if !surBlocked {
								setToken(nextTok, formatProper(sur.display, sur.suffix, true), true)
							}
						}
					}
				}
			} else if t3IsPerson && hasT1GivenDisp && t1GivenDisp != "" && !t1.Locked && !NonInstPrefix[t1.Low] {
				setToken(t1, t1GivenDisp, true)
			}
		}
	}

	// 8. Proper names, acronyms and languages of one word, and inflections of ambiguous names that do not soften.
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.IsWord || tok.Locked {
			continue
		}
		if isAllCaps(tok) || isMixedInnerCap(tok) {
			continue
		}

		low := tok.Low
		lowLen := utf8.RuneCountInString(low)
		var prevTok, nextTok *SentenceToken
		if i > 0 {
			prevTok = tokens[i-1]
		}
		if i+1 < len(tokens) {
			nextTok = tokens[i+1]
		}

		// 8-apos: an ambiguous lowercase stem'suffix left from step 4 (W6 #4) until after the titles of step 7.
		if strings.Contains(low, "'") {
			if !isCap(tok) {
				apos := strings.Index(low, "'")
				lowStem := low[:apos]
				lowSuf := low[apos+1:]
				if ambDisp, ok := known.AmbigNameFor(lowStem); ok && ambDisp != "" && lowSuf != "" {
					aposRunes := utf8.RuneCountInString(lowStem)
					softCommon := JoinWithoutApostrophe(sliceRunes(tok.Eff, 0, aposRunes), sliceFrom(tok.Eff, aposRunes+1), isMorphWord)
					leftRaw, rightRaw := buildContextAround(i)
					_, isGiven := known.GivenNameFor(lowStem)
					q := AmbigCandidateQuery{
						Start:         tok.Start,
						End:           tok.End,
						Base:          lowStem,
						DisplayBase:   ambDisp,
						Suffix:        lowSuf,
						HadApostrophe: true,
						AtStart:       tok.AtStart,
						IsGiven:       isGiven,
						CommonCand:    softCommon,
						ProperCand:    ambDisp + "'" + lowSuf,
						LeftRaw:       leftRaw,
						RightRaw:      rightRaw,
					}
					if resources.CollectAmbig != nil {
						*resources.CollectAmbig = append(*resources.CollectAmbig, q)
					}
					if resources.DisambiguateAmbig != nil {
						if decided, ok := resources.DisambiguateAmbig(q); ok && decided != "" {
							setToken(tok, decided, true)
						}
					}
				}
			}
			continue
		}

		// 8a. Rules of their own for "hindistan" (not "hindistan cevizi"), "alman", "alevi" and "aliye".
		if low == "hindistan" && (nextTok == nil || !strings.HasPrefix(nextTok.Low, "ceviz")) {
			setToken(tok, "Hindistan", true)
			continue
		}
		if strings.HasPrefix(low, "hindistan") && lowLen > 9 && (!isCap(tok) || tok.AtStart) {
			hSuf := low[len("hindistan"):]
			if ProperInflectionalSuffixRE.MatchString(hSuf) && IsPhonologicallyValidProperSuffix("hindistan", hSuf, ProperSuffixOptions{}) {
				setToken(tok, "Hindistan'"+hSuf, true)
				continue
			}
		}
		if low == "aliye" && (!isCap(tok) || tok.AtStart) {
			dativeCue := nextTok != nil && aliyeDativeCueRE.MatchString(nextTok.Low)
			if dativeCue {
				setToken(tok, "Ali'ye", true)
				continue
			}
		}
		if low == "alman" {
			verbalNext := nextTok != nil && almanVerbalNext[nextTok.Low]
			accPrev := prevTok != nil && accusativeEndRE.MatchString(prevTok.Low)
			if !verbalNext && !accPrev {
				setToken(tok, "Alman", true)
				continue
			}
		}
		if low == "alevi" {
			hasSectCtx := false
			for idx, t := range tokens {
				if idx != i && max(idx-i, i-idx) <= 4 && aleviSectCtx[t.Low] {
					hasSectCtx = true
					break
				}
			}
			if hasSectCtx {
				setToken(tok, "Alevi", true)
				continue
			}
		}

		// 8b. Acronyms (bare with three or more letters, or with a suffix and two or more: "tbmmde" -> "TBMM'de",
		// "İtüye" -> "İTÜ'ye", "abye" -> "AB'ye", "natonun" -> "NATO'nun").
		if lowLen >= 3 {
			if bareAcro, ok := known.AcronymFor(low); ok && bareAcro != "" && !isCommonWordCollision(low, low) {
				setToken(tok, bareAcro, true)
				continue
			}
		}
		// An acronym with a suffix, lowercase or in sentence case ("İtüye", "Abye", "Tbmmde", "Natonun").
		if lowLen >= 4 {
			matchedAcro := false
			lowRunes := []rune(low)
			for cut := min(lowLen-1, 8); cut >= 2; cut-- {
				base := string(lowRunes[:cut])
				suf := string(lowRunes[cut:])
				acro, ok := known.AcronymFor(base)
				if !ok && cut >= 3 && !isMorphWord(base) {
					acro, ok = known.ASCIIAcronymFor(base)
				}
				if !ok || acro == "" {
					continue
				}
				if isCommonWordCollision(low, base) {
					continue
				}
				if IsPhonologicallyValidProperSuffix(base, suf, ProperSuffixOptions{Acronym: true}) {
					setToken(tok, acro+"'"+suf, true)
					matchedAcro = true
					break
				}
			}
			if matchedAcro {
				continue
			}
		}

		// 8c. A bare proper name that always is one ("ahmet" -> "Ahmet", "istanbul" -> "İstanbul", "türkçe" ->
		// "Türkçe", "macka" -> "Maçka").
		if !isCap(tok) {
			if bareName, ok := lookupName(low, sentenceHasAsciiFixes); ok {
				setToken(tok, bareName.Name, true)
				continue
			}
		}

		// 8d. Such a proper name with a suffix, lowercase mid-sentence or at a sentence's start ("Özgeyi", "Boludan",
		// "Türkiyemizin"). Words already capitalized mid-sentence are the core's addApostrophes'.
		if !isCap(tok) || tok.AtStart {
			matchedName := false
			allowAscii := sentenceHasAsciiFixes || !textHasTurkishMarks
			lowRunes := []rune(low)
			for cut := lowLen - 1; cut >= 3; cut-- {
				base := string(lowRunes[:cut])
				suf := string(lowRunes[cut:])
				// "alman" with a plural or derivational suffix too ("almanlar", "almanların", "alman'sınız").
				var entry NameEntry
				if base == "alman" {
					entry = NameEntry{Name: "Alman", Count: 40000}
				} else {
					var ok bool
					if entry, ok = lookupName(base, allowAscii); !ok {
						continue
					}
				}
				baseLow := tr.Lower(entry.Name)

				// Common words ("belli", "hatayı", "türkü", "hindi", "ıraklık") and foreign names on their own ("Anna",
				// "Paula", "Jimi", "Michelin") are kept, but forms that are mostly the proper name's are not
				// ("taksimde", "murata", where "murat'a" is far more common than "murata").
				aposFreq := lexCount(baseLow + "'" + suf)
				wholeFreq := lexCount(low)
				strongProperApos := (aposFreq >= 25 && aposFreq >= wholeFreq*5) ||
					(!isMorphWord(low) && aposFreq >= 15 && aposFreq >= wholeFreq*2)
				if !strongProperApos && isCommonWordCollision(low, baseLow) && !LanguageNames[baseLow] && !NoAposReligionOrDerived[baseLow] {
					continue
				}

				// Case 1: a language or a proper noun that takes no apostrophe ("türkçeyi", "yunancanın", "almancanın",
				// "gürcücesini", "türkmencenin", "korecesini", "moğolcadaki", "türkçedekilere", "hinduizmde").
				if entry.NoApos || LanguageNames[baseLow] || NoAposReligionOrDerived[baseLow] {
					if NoAposProperSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(baseLow, suf, ProperSuffixOptions{}) {
						setToken(tok, entry.Name+suf, true)
						matchedName = true
						break
					}
					continue
				}

				// Case 2: a derivational or plural suffix on a proper noun takes no apostrophe ("lazların",
				// "selçuklular", "hunlar", "ispanyolların", "rumların", "avrupalılar", "japonlar", "türklerin"). Common
				// derivations whose stem is also a place are kept ("ıraklık" is parallax, not "Iraklık").
				if NonApostropheSuffixRE.MatchString(suf) {
					if lexCount(low) >= 3 && capShare(low) < 0.35 {
						continue
					}
					// The TypeScript's first-vowel test is or-ed with true: only the suffix's start counts.
					okStart := strings.HasPrefix(suf, "l") || cSuffixRE.MatchString(suf)
					if okStart && (isMorphWord(baseLow+suf) || lexCount(baseLow+suf) >= 2 || pluralProperRE.MatchString(suf)) {
						setToken(tok, entry.Name+suf, true)
						matchedName = true
						break
					}
					continue
				}

				// Case 3: an inflectional suffix on a bare proper noun takes an apostrophe ("istanbula" -> "İstanbul'a",
				// "fatmadan" -> "Fatma'dan", "türkiyemizin" -> "Türkiye'mizin", "ahmetinki" -> "Ahmet'inki", "burakin"
				// -> "Burak'ın", "palandokende" -> "Palandöken'de").
				if ProperInflectionalSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(baseLow, suf, ProperSuffixOptions{}) {
					setToken(tok, entry.Name+"'"+suf, true)
					matchedName = true
					break
				}
				if allowAscii {
					if harm, ok := HarmonizeASCIISuffix(baseLow, suf, ProperSuffixOptions{}); ok && harm != "" {
						setToken(tok, entry.Name+"'"+harm, true)
						matchedName = true
						break
					}
				}
			}
			if matchedName {
				continue
			}
		}

		// 8e. Inflections of ambiguous or given names that do not soften or that the morphology rejects: "umutun" ->
		// "Umut'un" (the common word is "umudun"), "umuta" -> "Umut'a" ("umuda"), "kıvança" -> "Kıvanç'a"
		// ("kıvanca"), "emrenin" -> "Emre'nin", "aslıyı" -> "Aslı'yı", "tokata" -> "Tokat'a" ("tokada"), "çiçeke" ->
		// "Çiçek'e" ("çiçeğe"), "ufukun" -> "Ufuk'un" ("ufkun"). Foreign names on their own ("Anna", "Jimi",
		// "Gabrielle", "Lynne", "Michelin") and common nouns with a possessive ("zümrütünü") are kept by the corpus
		// counts and the name lists.
		_, lowGiven := known.GivenNameFor(low)
		_, lowName := known.NameEntry(low, 0)
		_, lowSurname := known.SurnameFor(low)
		_, lowAmbig := known.AmbigNameFor(low)
		if (!isCap(tok) || tok.AtStart) && lowLen >= 4 && !lowGiven && !lowName && !lowSurname && !lowAmbig {
			matchedNonSoft := false
			lowRunes := []rune(low)
			for cut := lowLen - 1; cut >= 3; cut-- {
				base := string(lowRunes[:cut])
				suf := string(lowRunes[cut:])
				ambDisp, isAmbig := known.AmbigNameFor(base)
				if !isAmbig {
					ambDisp, _ = known.GivenNameFor(base)
				}
				if ambDisp == "" {
					continue
				}
				if !isAmbig && !nonSoftSuffixRE.MatchString(suf) {
					continue
				}
				baseLow := tr.Lower(ambDisp)
				wholeFreq := lexCount(low)
				aposFreq := lexCount(baseLow + "'" + suf)
				// Foreign or brand names on their own at a sentence's start ("Anna", "Jimi", "Gabrielle", "Lynne",
				// "Michelin") and words the corpus has are kept, unless the proper form with an apostrophe is far more
				// common.
				if tok.AtStart && isCap(tok) && (aposFreq < 5 || aposFreq < wholeFreq*3) {
					continue
				}
				if wholeFreq >= 5 && aposFreq < wholeFreq*3 {
					continue
				}
				// The morphology rejects the form ("umutun", "emrenin", "tokata", "kıvança"), or the proper form
				// with an apostrophe has it all ("aslıyı").
				nonCommonForm := !isMorphWord(low) || (wholeFreq == 0 && aposFreq >= 5)
				if !nonCommonForm {
					continue
				}
				if ProperInflectionalSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(baseLow, suf, ProperSuffixOptions{}) {
					setToken(tok, ambDisp+"'"+suf, true)
					matchedNonSoft = true
					break
				}
				if sentenceHasAsciiFixes {
					if harm, ok := HarmonizeASCIISuffix(baseLow, suf, ProperSuffixOptions{}); ok && harm != "" {
						setToken(tok, ambDisp+"'"+harm, true)
						matchedNonSoft = true
						break
					}
				}
			}
			if matchedNonSoft {
				continue
			}
		}

		// 8f. Frames that only a person fits, and the optional disambiguator, for ambiguous names.
		if !isCap(tok) || tok.AtStart {
			// An ambiguous name, bare or with a suffix.
			ambBase, ambDisp, ambSuf := "", "", ""
			if d, ok := known.AmbigNameFor(low); ok && d != "" {
				ambBase = low
				ambDisp = d
			} else {
				lowRunes := []rune(low)
				for cut := lowLen - 1; cut >= 3; cut-- {
					base := string(lowRunes[:cut])
					suf := string(lowRunes[cut:])
					disp, ok := known.AmbigNameFor(base)
					if !ok || disp == "" {
						continue
					}
					// First- and third-person possessives ("canım", "gülüm", "sevgisi") and copulas are not cut.
					if ambigSuffixRE.MatchString(suf) && IsPhonologicallyValidProperSuffix(base, suf, ProperSuffixOptions{}) {
						ambBase = base
						ambDisp = disp
						ambSuf = suf
						break
					}
				}
			}
			if ambBase != "" && ambDisp != "" {
				_, ambGiven := known.GivenNameFor(ambBase)
				// A human possessor: "<AmbigGiven>'ın annesi/babası/evi/ofisi/masası...", but not "baharın yeni" or
				// "kaderin yeni".
				if ambGiven && genitiveSuffixRE.MatchString(ambSuf) && nextTok != nil && adjSpace(tok, nextTok) &&
					humanPossessedNext[nextTok.Low] && nextTok.Low != "yeni" {
					setToken(tok, ambDisp+"'"+ambSuf, true)
					continue
				}
				// A human addressee in the dative: "<AmbigGiven>'a/e mesaj at / haber ver / söyle / gönder".
				if ambGiven && dativeSuffixRE.MatchString(ambSuf) && nextTok != nil &&
					(dativeNext[nextTok.Low] ||
						(nextTok.Low == "de" && i+2 < len(tokens) && (tokens[i+2].Low == "gönderdim" || tokens[i+2].Low == "söyle" || tokens[i+2].Low == "yazdım"))) {
					setToken(tok, ambDisp+"'"+ambSuf, true)
					continue
				}
				// A human companion in the instrumental: "<AmbigGiven>'la/le telefonda / buluş / aynı odada / aynı
				// otobüste / aynı sınıfta / birlikte hazırla".
				if ambGiven && instrumentalSuffixRE.MatchString(ambSuf) && nextTok != nil &&
					(nextTok.Low == "telefonda" ||
						strings.HasPrefix(nextTok.Low, "buluş") ||
						(nextTok.Low == "aynı" && i+2 < len(tokens) && sameRoomRE.MatchString(tokens[i+2].Low)) ||
						(nextTok.Low == "birlikte" && i+2 < len(tokens) && togetherVerbRE.MatchString(tokens[i+2].Low))) {
					setToken(tok, ambDisp+"'"+ambSuf, true)
					continue
				}
				// The optional disambiguator (the neural context model), asked only about ambiguous given names and
				// ambiguous places (not about surnames, colours or metals like "al", "altın", "taş"), and not about
				// bare words at a sentence's start (step 9 capitalizes those: no ambiguity).
				isCtxEligible := ambGiven || ambigPlaceNames[ambBase]
				// Quoted glosses (`"güneş" anlamına gelen`) and stems that are also ordinary lowercase words elsewhere
				// in the sentence ("savaşa ... savaş başladıktan sonra") are skipped. A character outside the text is
				// "", which "includes" holds, as in the TypeScript.
				prevCh := runeAt(rs, tok.Start-1)
				nextCh := runeAt(rs, tok.End)
				isQuotedGloss := ((prevCh < 0 || strings.ContainsRune(`"'“‘«`, prevCh)) && (nextCh < 0 || strings.ContainsRune(`"'”’»`, nextCh))) ||
					(nextTok != nil && glossNextRE.MatchString(nextTok.Low))
				sameStemLowerElsewhere := false
				for idx, t := range tokens {
					if idx != i && t.IsWord && !isCap(t) && (t.Low == ambBase || (strings.HasPrefix(t.Low, ambBase) && isMorphWord(t.Low))) {
						sameStemLowerElsewhere = true
						break
					}
				}
				if isCtxEligible && !(tok.AtStart && ambSuf == "") && !isQuotedGloss && !sameStemLowerElsewhere {
					leftRaw, rightRaw := buildContextAround(i)
					commonCand := ambBase
					if ambSuf != "" {
						if tok.AtStart {
							commonCand = CapTr(ambBase + ambSuf)
						} else {
							commonCand = ambBase + ambSuf
						}
					}
					properCand := ambDisp
					if ambSuf != "" {
						properCand = ambDisp + "'" + ambSuf
					}
					q := AmbigCandidateQuery{
						Start:         tok.Start,
						End:           tok.End,
						Base:          ambBase,
						DisplayBase:   ambDisp,
						Suffix:        ambSuf,
						HadApostrophe: false,
						AtStart:       tok.AtStart,
						IsGiven:       ambGiven,
						CommonCand:    commonCand,
						ProperCand:    properCand,
						LeftRaw:       leftRaw,
						RightRaw:      rightRaw,
					}
					if resources.CollectAmbig != nil {
						*resources.CollectAmbig = append(*resources.CollectAmbig, q)
					}
					if resources.DisambiguateAmbig != nil {
						if decided, ok := resources.DisambiguateAmbig(q); ok && decided != "" {
							setToken(tok, decided, true)
							continue
						}
					}
				}
			}
		}
	}

	// 9. Capitals at the start of sentences.
	result = applySentenceStartCapitals(rs, result, spaced)
	sortByStart(result)
	return result
}

var (
	minimalWordRE   = regexp.MustCompile(`\p{L}+(?:['’]\p{L}+)?`)
	previousWordRE  = regexp.MustCompile(`(\p{L}+)[^\p{L}]*$`)
	letterRunRE     = regexp.MustCompile(`\p{L}+`)
	sentenceBreakRE = regexp.MustCompile(`([.!?…]+)(["'”’»)\]]*)([` + jsSpaceClass + `]+)(["'“‘«(\[]*)`)
)

// minimalTitles are the abbreviated titles the minimal rules capitalize before a capitalized name.
var minimalTitles = setOf("dr", "prof", "doç", "doc", "av", "op", "uzm", "yrd", "öğr", "ogr", "gör", "gor", "sn", "mr", "mrs", "ms")

// applyMinimalRules is the fallback for proper names and titles when applySentenceRules is called without
// resources (unit tests). It returns result with its changes.
func applyMinimalRules(
	rs []rune,
	result []Correction,
	spaced map[int]bool,
	isWord func(lower string) bool,
	nameFor func(lower string, common bool) (string, bool),
	stemsOf func(lower string) ([]string, bool),
) []Correction {
	text := string(rs)
	covering := func(start, end int) int {
		return slices.IndexFunc(result, func(fix Correction) bool { return fix.Start < end && fix.End > start })
	}

	for _, match := range findAllRunes(minimalWordRE, text) {
		start := match[0]
		end := match[1]
		word := runeSlice(rs, start, end)
		if !lowerInitial.MatchString(word) || hasUpper(word) {
			continue
		}
		if isWordChar(runeAt(rs, start-1)) || precededByAddress(rs, start) {
			continue
		}
		if continuesAsAddress(rs, end) && !spaced[end] {
			continue
		}
		if r := runeAt(rs, end); r >= 0 && (unicode.IsNumber(r) || r == '_') {
			continue
		}
		if covering(start, end) != -1 {
			continue
		}
		apostrophe := strings.IndexFunc(word, func(r rune) bool { return r == '\'' || r == '’' })
		replacement, hasReplacement := "", false
		if apostrophe > 0 {
			if name, ok := nameFor(word[:apostrophe], false); ok && name != "" {
				replacement, hasReplacement = name+word[apostrophe:], true
			}
		} else {
			replacement, hasReplacement = nameFor(word, false)
			previous, hasPrevious := "", false
			if m := previousWordRE.FindStringSubmatch(runeSlice(rs, max(0, start-40), start)); m != nil {
				previous, hasPrevious = m[1], true
			}
			foreignBefore := hasPrevious && !startsUpper(previous) && !isWord(tr.Lower(previous))
			wordRunes := []rune(word)
			if !(hasReplacement && replacement != "") && len(wordRunes) >= 5 && !foreignBefore {
				for cut := len(wordRunes) - 1; cut >= 3; cut-- {
					rest := string(wordRunes[cut:])
					if !fallbackNameSuffixes[rest] {
						continue
					}
					base := string(wordRunes[:cut])
					name, hasName := nameFor(base, true)
					hasName = hasName && name != ""
					var stems []string
					stemsOK := false
					if hasName {
						stems, stemsOK = stemsOf(word)
					}
					unknown := !stemsOK || len(stems) == 0
					var common bool
					if unknown {
						common = isWord(word)
					} else {
						common = slices.ContainsFunc(stems, func(stem string) bool { return stem != base })
					}
					if common {
						if hasName {
							break
						}
						continue
					}
					if hasName {
						if noApostropheName.MatchString(name) {
							replacement = name + rest
						} else {
							replacement = name + "'" + rest
						}
						hasReplacement = true
						break
					}
				}
			}
		}
		if hasReplacement && replacement != "" && replacement != word {
			result = append(result, Correction{Start: start, End: end, Original: word, Replacement: replacement, Kind: KindPunct, Margin: 6})
		}
	}
	sortByStart(result)

	// /\p{L}+(?=\.\s+\p{L})/gu: a run of letters followed by a full stop, white space and a letter. The run is
	// greedy and its shorter forms end before a letter, so only whole runs can match.
	for _, match := range findAllRunes(letterRunRE, text) {
		start := match[0]
		end := match[1]
		if runeAt(rs, end) != '.' {
			continue
		}
		spaces := 0
		for isJSSpace(runeAt(rs, end+1+spaces)) {
			spaces++
		}
		if spaces == 0 || !isLetterRune(runeAt(rs, end+1+spaces)) {
			continue
		}
		word := runeSlice(rs, start, end)
		if !minimalTitles[word] || isWordChar(runeAt(rs, start-1)) {
			continue
		}
		nextStart := end + 1 + spaces
		nextText := runeSlice(rs, nextStart, len(rs))
		if i := slices.IndexFunc(result, func(fix Correction) bool { return fix.Start == nextStart }); i != -1 {
			nextText = result[i].Replacement
		}
		if !startsUpper(nextText) || covering(start, end) != -1 {
			continue
		}
		result = append(result, Correction{Start: start, End: end, Original: word, Replacement: capitalize(word), Kind: KindPunct, Margin: 6})
	}
	sortByStart(result)
	return result
}

// applySentenceStartCapitals capitalizes the lowercase words that start sentences, merging the capital into a fix
// of one word already at that offset. It returns result with its changes.
func applySentenceStartCapitals(rs []rune, result []Correction, spaced map[int]bool) []Correction {
	text := string(rs)
	covering := func(start, end int) int {
		return slices.IndexFunc(result, func(fix Correction) bool { return fix.Start < end && fix.End > start })
	}
	lead := 0
	for lead < len(rs) && (isJSSpace(rs[lead]) || strings.ContainsRune(`"'“‘«([`, rs[lead])) {
		lead++
	}
	starts := []int{lead}
	for _, match := range findAllRunes(sentenceBreakRE, text) {
		punctStart := match[0]
		punctEnd := match[3]
		if !endsSentence(rs, punctStart, punctEnd) {
			continue
		}
		if match[5] > match[4] {
			continue
		}
		nextStart := match[1]
		// A lowercase parenthesis whose ")" is followed by "." is not given a capital (". (başka bir deyişle,
		// ...).").
		if strings.Contains(runeSlice(rs, match[8], match[9]), "(") {
			closeParen := -1
			for k := nextStart; k < len(rs); k++ {
				if rs[k] == ')' {
					closeParen = k
					break
				}
			}
			if closeParen != -1 && runeAt(rs, closeParen+1) == '.' {
				continue
			}
		}
		starts = append(starts, nextStart)
	}
	for _, start := range starts {
		if r := runeAt(rs, start); r < 0 || !lowerInitial.MatchString(string(r)) {
			continue
		}
		end := wordEnd(rs, start)
		word := runeSlice(rs, start, end)
		if word == "" || hasNumber(word) || hasUpper(word) {
			continue
		}
		if utf8.RuneCountInString(word) == 1 && word != "o" {
			continue
		}
		if !spaced[end] && continuesAsAddress(rs, end) {
			continue
		}
		index := covering(start, end)
		if index == -1 {
			result = append(result, Correction{Start: start, End: end, Original: word, Replacement: capitalize(word), Kind: KindPunct, Margin: 6})
			continue
		}
		existing := result[index]
		if existing.Start == start && lowerInitial.MatchString(existing.Replacement) {
			existing.Replacement = capitalize(existing.Replacement)
			result[index] = existing
		}
	}
	return result
}
