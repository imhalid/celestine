package morph

// The tests of tinyfix-tr's src/morphology/morphology.test.ts. The TypeScript builds its morphology from
// Zemberek's .dict files, which are not in this repository; here the tests that need the whole lexicon read
// morph.bin from TINYFIX_DATA and are skipped without it. The sample-wrong.txt and sample-valid.txt reports are
// not ported, as those lists are not in the TypeScript repository either.

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// checkWords fails for each word whose IsWord is not want.
func checkWords(t *testing.T, m *Morphology, want bool, words ...string) {
	t.Helper()
	for _, w := range words {
		if got := m.IsWord(w); got != want {
			t.Errorf("IsWord(%q) = %v, want %v", w, got, want)
		}
	}
}

func TestLoadsQuickly(t *testing.T) {
	start := time.Now()
	m := loadMorphology(t)
	t.Logf("morph.bin loaded in %v", time.Since(start))
	checkWords(t, m, true, "ev")
}

func TestRequiredForms(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, true,
		"düşünmüştün", "getirsek", "ekmemi", "yatıyorsun", "olmuştun", "yorumumu", "kitabımı", "öyküsüz",
		"araştırtmaya", "açkılanmayan", "gidecekmişsin", "yapamayacaklarımızdan", "evdekiler", "kitapçılardan",
		"ağzımızı", "burnundan", "diyordum", "yiyeceksin",
		"soyumdan", "anlaşayım", "göndertme", "getirdiğimi", "yapmamı", "olduğumu")
	checkWords(t, m, false,
		"seyehat", "herkez", "yalnış", "yanlız", "malesef", "orjinal", "birşey", "herşey", "geliyom", "gelyorum",
		"yapabiliur", "kazanackasın")
}

func TestRejectsCliticsDropsAndBadPhonology(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, false,
		// The attached question particle.
		"geldinmi", "gidecekmisin", "varmı", "yokmu", "değilmi", "biliyormusun",
		// The attached conjunctions.
		"olsada", "yinede", "dahada", "nasılda", "öylemi", "iyiki", "demekki", "tabiki",
		// The colloquial vowel-drop contractions.
		"burda", "şurda", "orda", "içerde", "dışarda", "yukarda", "ilerde",
		// Phonology and morphotactics.
		"kitapı", "kitapım", "kitablar", "kitabta", "evte", "evlar", "masaler", "gitecek", "gitiyor", "yeyen",
		"diyer", "evdekiyi", "evdekide",
		// Modified stems that cannot end a word.
		"kitab", "ağz", "burn", "hakk", "gid")
}

func TestPhonology(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, true,
		// Vowel harmony, InverseHarmony included.
		"evler", "masalar", "gözlerim", "kolumuz", "saatler", "kalpler", "kalbim", "hayallerim",
		// Consonant assimilation (d/t, c/ç).
		"kitapta", "kitaptan", "kitapçı", "evde", "evden", "yaptı", "geldi", "seçti", "gitmişti",
		// Voicing and the NoVoicing exceptions.
		"kitabı", "ağacı", "rengi", "dengi", "çocuğu", "genci", "dermatoloğu", "gidiyor", "ediyor", "hukuku",
		"holdingi",
		// Vowel drop, in nouns and LastVowelDrop verbs.
		"burnu", "ağzı", "oğlum", "şehrimizde", "aklında", "gönlüm", "ömrümüz", "kıvrıldı", "kavruldu",
		// Doubling.
		"hakkı", "hakkımızda", "affına", "tıbbı", "reddi", "zıddı", "hissi",
		// Buffer consonants (y, n, s) and compounds.
		"arabayı", "arabaya", "arabası", "arabasını", "arabanın", "buzdolabına", "buzdolapları", "buzdolabım",
		"anaokulunda",
		// Irregulars (ben/sen, demek/yemek, ne, su).
		"bana", "sana", "benim", "bizim", "bizlerin", "onunla", "bununla", "neyi", "neye", "neyin", "nesini",
		"nesine", "diyor", "diyorum", "yiyor", "yiyecek", "deyince", "yiyince", "deyip", "yiyip", "suyu", "suyun",
		"suyumuz", "akarsuyun")
}

func TestNominalInflection(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, true,
		// The plural, the six possessives, all cases.
		"ev", "evim", "evin", "evi", "evimiz", "eviniz", "evleri", "evimi", "evine", "evinde", "evinden", "evinin",
		"evimle", "evce",
		// -ki (locative, time, genitive) and its pronominal cases and plural.
		"evdeki", "evdekini", "evdekine", "evdekinde", "evdekinden", "evdekinin", "evdekileri", "evdekilerden",
		"dünkü", "dünküler", "yarınki", "benimki", "benimkini", "masanınki",
		// The copula and -ken.
		"öğretmenim", "öğretmensin", "öğretmendir", "öğretmendi", "öğretmenmiş", "öğretmense", "evdeyim",
		"evdeydik", "evdeymiş", "evdeyken", "çocukken", "değilim", "değildi", "değilmiş", "değilse", "değildir",
		"nasılsın", "öyleydi",
		// Derivations: -li, -siz, -lik, -ci, -ce, -cik, -daş, -sal.
		"evli", "evsiz", "insanlık", "güzelliği", "kitapçı", "yavaşça", "insanca", "evcik", "kedicik",
		"meslektaş", "yurttaş", "sınıftaş", "köydeşlerimizden", "odadaş", "tarihsel", "toplumsal", "bilimsel")
}

func TestVerbalInflection(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, true,
		// Tenses, negation, the imperative, ability, compound tenses.
		"geliyorum", "geliyorsun", "geliyor", "geliyoruz", "geliyorsunuz", "geliyorlar", "gelmiyorum", "geleceğim",
		"geleceksin", "gelmeyeceğim", "geldim", "geldin", "geldik", "geldiniz", "geldiler", "gelmedim", "gelmişim",
		"gelmişsin", "gelir", "gelirim", "gelmem", "gelmez", "gelmezsin", "gelmeyiz", "gelmeli", "gelmeliyim",
		"gelse", "gelsem", "gelsek", "gelseydim", "geleyim", "gelelim", "gel", "gelsin", "gelin", "geliniz",
		"gelsinler", "gelebilirim", "gelemem", "gelemiyorum", "gelemeyeceğim", "geliyordum", "gelecektim",
		"gelmiştin", "gelirdi", "gelmezdi", "gelmeliydi", "arıyorum", "bekliyor", "okuyorum",
		// Verbal nouns, participles and converbs with nominal suffixes.
		"gelme", "gelmemi", "gelmek", "gelmekten", "gelmekte", "gelmekle", "gelmekteyim", "geldik", "geldiğim",
		"gelecek", "gelen", "gelenlerden", "gelip", "gelmeyip", "gelerek", "gelmeyerek", "gelince", "gelmeyince",
		"gelmeden", "gelemeden", "geldikten", "yaptıktan", "geldikçe", "gelmedikçe", "gelirken", "geliyorken",
		"gelecekken",
		// Voice: the passive, the causative and its chains, the reflexive, the reciprocal.
		"yapıldı", "yapılıyor", "yazılmış", "aranıyor", "alındı", "bilinen", "silindi", "denildi", "yeniyor",
		"yaptırdı", "yaptırttı", "okuttu", "okutturdu", "araştırttı", "bitirdi", "düşürdü", "yıkandı", "giyindi",
		"tarandı", "görüştük", "yazışıyorlar", "tanıştık")
}

func TestAnalyze(t *testing.T) {
	m := loadMorphology(t)
	for _, s := range []struct{ word, stem, pos string }{
		{"kitabımı", "kitap", "Noun"},
		{"gidecekmişsin", "git", "Verb"},
		{"araştırtmaya", "araştır", "Verb"},
		{"evdekiler", "ev", "Noun"},
		{"diyordum", "de", "Verb"},
		{"ağzımızı", "ağız", "Noun"},
		{"yapamayacaklarımızdan", "yap", "Verb"},
	} {
		analyses := m.Analyze(s.word)
		found := false
		for _, a := range analyses {
			if a.Stem == s.stem && a.Pos == s.pos {
				found = true
				if len(a.Morphemes) == 0 {
					t.Errorf("Analyze(%q): the %s %s analysis has no morphemes", s.word, s.stem, s.pos)
				}
			}
		}
		if !found {
			t.Errorf("Analyze(%q) = %v, want an analysis with stem %q and pos %q", s.word, analyses, s.stem, s.pos)
		}
	}
	for _, w := range []string{"", "seyehat", "herkez"} {
		if got := m.Analyze(w); len(got) != 0 {
			t.Errorf("Analyze(%q) = %v, want none", w, got)
		}
	}
}

func TestCasingAndCircumflexes(t *testing.T) {
	m := loadMorphology(t)
	checkWords(t, m, false, "", "kitâbımı")
	checkWords(t, m, true, "İSTANBULDA", "GİDECEKMİŞSİN", "IŞIKLARDAN", "hükûmetten", "hükümetten", "âcizane")
}

func TestMiniDictionary(t *testing.T) {
	mini := FromDictionaries("ev\nkitap\ngelmek [A:Aorist_I]\n")
	checkWords(t, mini, true, "evdekiler", "kitabımı", "geliyordum")
	checkWords(t, mini, false, "masa")
}

// TestBinaryMatchesText checks that a dictionary written as AMR1 and read back answers as its text does, on
// a small dictionary that has a modified stem, a compound with its dummy root, a Ref:, a pronunciation, an
// index and a circumflex.
func TestBinaryMatchesText(t *testing.T) {
	dict := "ev\nkitap\nhak [A:Doubling]\nağız [A:LastVowelDrop]\nzeytinyağı [A:CompoundP3sg; Roots:zeytin-yağ]\n" +
		"hükûmet\nhükümet [Ref:hükûmet]\nkalp [A:Voicing,InverseHarmony]\nsaat [A:NoVoicing,InverseHarmony]\n" +
		"gelmek [A:Aorist_I]\nyüz [P:Num; Index:2]\nben [P:Pron,Pers]\nde [P:Conj]\ndemek\nAhmet\n"
	text := FromDictionaries(dict)
	items := parseDictionaries([]string{dict})
	b, err := encodeBinaryDictionary(items, nil)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := FromBinary(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"evdekiler", "kitabımı", "hakkı", "hak", "ağzımızı", "ağz", "zeytinyağları",
		"zeytinyağı", "hükûmetten", "hükümetten", "kalbim", "saatler", "geliyordum", "yüzü", "bana", "sana",
		"diyordum", "Ahmet'e", "ahmetler", "kitâbımı"} {
		if got, want := bin.IsWord(w), text.IsWord(w); got != want {
			t.Errorf("IsWord(%q): binary %v, text %v", w, got, want)
		}
		if got, want := bin.Analyze(w), text.Analyze(w); !reflect.DeepEqual(got, want) {
			t.Errorf("Analyze(%q): binary %v, text %v", w, got, want)
		}
	}
}

// TestBinaryRoundTrip checks that morph.bin's items and prior come back the same after they are written again.
func TestBinaryRoundTrip(t *testing.T) {
	b := readMorphBin(t)
	items, prior, err := parseBinaryDictionaryWithPrior(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeBinaryDictionary(items, prior)
	if err != nil {
		t.Fatal(err)
	}
	items2, prior2, err := parseBinaryDictionaryWithPrior(again)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, items2) {
		t.Error("the items differ after a round trip")
	}
	// The quantized probabilities come back the same, being quantized already.
	if !reflect.DeepEqual(prior.lemmaCounts, prior2.lemmaCounts) || !reflect.DeepEqual(prior.chainProbs, prior2.chainProbs) ||
		!reflect.DeepEqual(prior.bigramProbs, prior2.bigramProbs) {
		t.Error("the prior differs after a round trip")
	}
}

func TestAMR2PriorRoundTrip(t *testing.T) {
	miniItems := parseDictionaries([]string{"düzenlemek\nkullanmak\ngerek\n"})
	lemmas := newOrderedFloats()
	lemmas.set("düzenle|Verb", 75000)
	lemmas.set("kullan|Verb", 250000)
	lemmas.set("gerek|Noun", 5000)
	prior := newMorphPrior(lemmas,
		map[string]float64{
			"Verb|Neg+Prog1+A3sg":              0.002,
			"Verb|Pass+Verb+Prog1+A3sg":        0.0007,
			"Verb|PastPart+Noun+A3sg+P1sg+Loc": 0.0005,
		},
		map[string]float64{"^Verb>Neg": 0.08, "Neg>Prog1": 0.12, "Prog1>A3sg": 0.45, "A3sg>$": 0.8},
	)
	b, err := encodeBinaryDictionary(miniItems, prior)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := FromBinary(b)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasPrior() {
		t.Fatal("the AMR2 payload has no prior")
	}
	if !loaded.IsWord("düzenlemiyor") {
		t.Error(`"düzenlemiyor" is not a word`)
	}
	pNeg, pPass := loaded.PriorCount("düzenlemiyor"), loaded.PriorCount("düzenleniyor")
	if pNeg <= 100 || pNeg >= 220 {
		t.Errorf("PriorCount(düzenlemiyor) = %v, want about 150", pNeg)
	}
	if pNeg <= pPass {
		t.Errorf("the active negative (%v) is not above the passive (%v)", pNeg, pPass)
	}
	if got := loaded.PriorCount("kullandığımda"); got <= 80 {
		t.Errorf("PriorCount(kullandığımda) = %v, want above 80", got)
	}
	if got := loaded.PriorCount("duzenblemiyor"); got != 0 {
		t.Errorf("PriorCount(duzenblemiyor) = %v, want 0", got)
	}
}

// hasWord reports whether cands has word, with foldedEdits when it is not negative.
func hasWord(cands []FuzzyMorphCandidate, word string, foldedEdits int) bool {
	for _, c := range cands {
		if c.Word == word && (foldedEdits < 0 || c.FoldedEdits == foldedEdits) {
			return true
		}
	}
	return false
}

func TestFuzzySearch(t *testing.T) {
	m := loadMorphology(t)
	f1 := m.FuzzySearch("duzenblemiyor", 1, 20)
	if !hasWord(f1, "düzenlemiyor", 1) {
		t.Errorf("FuzzySearch(duzenblemiyor, 1) = %v, want düzenlemiyor with one edit", f1)
	}
	// Short stems out of the band never leak into the results.
	for _, c := range f1 {
		if runeLen(c.Word) < 10 {
			t.Errorf("FuzzySearch(duzenblemiyor, 1) has the short candidate %q", c.Word)
		}
	}
	f2 := m.FuzzySearch("duzenlemniyor", 1, 20)
	if !hasWord(f2, "düzenlemiyor", -1) || !hasWord(f2, "düzenleniyor", -1) {
		t.Errorf("FuzzySearch(duzenlemniyor, 1) = %v, want düzenlemiyor and düzenleniyor", f2)
	}
	if got := m.FuzzySearch("kalelmerimi", 1, 20); !hasWord(got, "kalemlerimi", -1) {
		t.Errorf("FuzzySearch(kalelmerimi, 1) = %v, want the swap across the stem's end, kalemlerimi", got)
	}
	if got := m.FuzzySearch("kullandğımda", 1, 20); !hasWord(got, "kullandığımda", -1) {
		t.Errorf("FuzzySearch(kullandğımda, 1) = %v, want kullandığımda", got)
	}
	if got := m.FuzzySearch("grkli", 2, 40); !hasWord(got, "gerekli", -1) {
		t.Errorf("FuzzySearch(grkli, 2) = %v, want gerekli", got)
	}
	if got := m.FuzzySearch("beketiyruz", 2, 40); !hasWord(got, "bekletiyoruz", -1) {
		t.Errorf("FuzzySearch(beketiyruz, 2) = %v, want bekletiyoruz", got)
	}
}

func TestTurkishEditCost(t *testing.T) {
	if got := TurkishEditCost("duzenblemiyor", "düzenlemiyor"); got >= 1.3 {
		t.Errorf("TurkishEditCost(duzenblemiyor, düzenlemiyor) = %v, want below 1.3", got)
	}
	if got := TurkishEditCost("kullandğımda", "kullandığımda"); got >= 0.7 {
		t.Errorf("TurkishEditCost(kullandğımda, kullandığımda) = %v, want below 0.7", got)
	}
}

// TestJSExp checks jsExp against V8's Math.exp at the values the prior takes (the 256 quantized lemma counts and
// probabilities) and at others: the hash of their bits is the one Node 22 gives for the same values.
func TestJSExp(t *testing.T) {
	var h uint64
	add := func(x float64) { h = h*31 + math.Float64bits(x) }
	for q := 0; q < 256; q++ {
		add(dequantizeLemmaCount(byte(q)))
		add(dequantizeProb(byte(q)))
	}
	for i := -3000; i <= 3000; i++ {
		add(jsExp(float64(i) / 100))
	}
	for _, x := range []float64{1, -1, 709.7, -745, 1e-300, 0.3, 0.5, 2, 100, -0.0001} {
		add(jsExp(x))
	}
	if h != 13308407562123717242 {
		t.Errorf("the hash of jsExp's values is %d, not V8's", h)
	}
}
