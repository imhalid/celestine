package spell

import (
	"math"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/corrector.test.ts. The tests that need the engine's data read it from the
// directory TINYFIX_DATA names and are skipped without it; the TypeScript runs them with a lexicon built from
// Wikipedia, and they pass with the published lexicon.bin as well. The context model's own tests (its subword
// tokenizer and packing) wait for the model's port; the corrector's use of a model is tested with fake models.

var (
	dataOnce      sync.Once
	dataCorrector *Corrector
)

// testCorrector is the corrector of the engine's data with CAREFUL, shared by the tests as the TypeScript's.
func testCorrector(t *testing.T) *Corrector {
	t.Helper()
	lexicon, known, morphology := loadTestData(t)
	dataOnce.Do(func() { dataCorrector = NewCorrector(lexicon, CAREFUL, known, morphology) })
	return dataCorrector
}

func TestTurkishCasing(t *testing.T) {
	if got := tr.Lower("İSTANBUL ILIK"); got != "istanbul ılık" {
		t.Errorf("Lower = %q", got)
	}
	if got := tr.MatchCase("Cok", "çok"); got != "Çok" {
		t.Errorf("MatchCase = %q", got)
	}
	if got := tr.MatchCase("ISTANBUL", "istanbul"); got != "İSTANBUL" {
		t.Errorf("MatchCase = %q", got)
	}
	if !slices.Contains(tr.MarkVariants("cok", 512), "çok") {
		t.Error("MarkVariants(cok) has no çok")
	}
}

func TestCorrectorFixes(t *testing.T) {
	c := testCorrector(t)
	for _, tc := range correctorFixes {
		if got := c.Correct(tc[0], CorrectOptions{}).Text; got != tc[1] {
			t.Errorf("fixes %q:\n got %q\nwant %q", tc[0], got, tc[1])
		}
	}
	for _, input := range correctorKeeps {
		if got := c.Correct(input, CorrectOptions{}).Text; got != input {
			t.Errorf("keeps %q: got %q", input, got)
		}
	}
	for _, tc := range correctorExtraFixes {
		once := c.Correct(tc[0], CorrectOptions{}).Text
		if once != tc[1] {
			t.Errorf("fixes (v5) %q:\n got %q\nwant %q", tc[0], once, tc[1])
		}
		if twice := c.Correct(once, CorrectOptions{}).Text; twice != once {
			t.Errorf("fixes (v5) %q again: %q", once, twice)
		}
	}
	for _, input := range correctorExtraKeeps {
		if got := c.Correct(input, CorrectOptions{}).Text; got != input {
			t.Errorf("keeps (v5) %q: got %q", input, got)
		}
	}
}

// Spoken-style contractions are a style choice: left alone by default (and not taken for slips), spelled out when
// asked.
func TestCorrectorContractions(t *testing.T) {
	c := testCorrector(t)
	for _, input := range []string{"Burda ne noldu, bilmiyorum.", "Yarın gidecem, sen nerde kaldın?"} {
		if got := c.Correct(input, CorrectOptions{}).Text; got != input {
			t.Errorf("%q: got %q", input, got)
		}
	}
	c.Settings.Contractions = true
	defer func() { c.Settings.Contractions = false }()
	if got := c.Correct("Burda ne oldu, bilmiyorum.", CorrectOptions{}).Text; got != "Burada ne oldu, bilmiyorum." {
		t.Errorf("got %q", got)
	}
	if got := c.Correct("Yarın gidecem, sen nerde kaldın?", CorrectOptions{}).Text; got != "Yarın gideceğim, sen nerede kaldın?" {
		t.Errorf("got %q", got)
	}
	r := c.Correct("Bugün noldu?", CorrectOptions{})
	if len(r.Corrections) == 0 || r.Corrections[0].Replacement != "ne oldu" || r.Corrections[0].Kind != KindContraction {
		t.Errorf("Bugün noldu?: %+v", r.Corrections)
	}
}

func TestCorrectorAutoMarked(t *testing.T) {
	c := testCorrector(t)
	// "Açık" was "Acik" before autocorrect fixed it; the rest of the sentence is still typed without Turkish letters.
	text := "Açık deniz boru hatti "
	if got := c.Correct(text, CorrectOptions{AutoMarked: [][2]int{{0, 4}}}).Text; got != "Açık deniz boru hattı " {
		t.Errorf("auto-marked: got %q", got)
	}
	// Typed by the user with a Turkish keyboard: "hatti" is left alone.
	if got := c.Correct(text, CorrectOptions{}).Text; got != text {
		t.Errorf("typed: got %q", got)
	}
}

func TestCorrectorMisc(t *testing.T) {
	c := testCorrector(t)
	for _, tc := range [][2]string{
		// Two-word listed fixes also match when typed without Turkish letters.
		{"Yarin gelecekmisin, hic bir sey yok.", "Yarın gelecek misin, hiçbir şey yok."},
		{"bir cok insan geldi", "birçok insan geldi"},
		// ol- after a participle is a helper verb, not öl-.
		{"Ona yardim ediyor oluyorduk.", "Ona yardım ediyor oluyorduk."},
		{"Ona yardım ediyor oluyorduk.", "Ona yardım ediyor oluyorduk."},
		// A unique valid mark variant keeps its lower bar when plain restoration also proposes it.
		{"bu cihaz neden duzeltmiyor", "bu cihaz neden düzeltmiyor"},
	} {
		if got := c.Correct(tc[0], CorrectOptions{}).Text; got != tc[1] {
			t.Errorf("%q:\n got %q\nwant %q", tc[0], got, tc[1])
		}
	}
}

func TestCorrectorRuleHelpers(t *testing.T) {
	check := func(what string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	opt := func(s string, ok bool) string {
		if !ok {
			return "<null>"
		}
		return s
	}
	// The known misspellings' particle suffix guard.
	check("kim mi+ş", IsValidKnownParticleSuffix("kim mi", "ş"), false)
	check("var mı+ş", IsValidKnownParticleSuffix("var mı", "ş"), false)
	check("var mı+sın", IsValidKnownParticleSuffix("var mı", "sın"), true)
	check("var mı+ydı", IsValidKnownParticleSuffix("var mı", "ydı"), true)
	check("var mı+sin", IsValidKnownParticleSuffix("var mı", "sin"), false)
	check("ya da+cı", IsValidKnownParticleSuffix("ya da", "cı"), false)
	check("değil ki+m", IsValidKnownParticleSuffix("değil ki", "m"), false)
	check("bir şey+ler", IsValidKnownParticleSuffix("bir şey", "ler"), true)

	// The generic apostrophe stem and phonology guards.
	for stem, want := range map[string]bool{"müslümanlar": true, "japonların": true, "yunanlardan": true, "türklerin": true, "türkçeden": true, "hırvatça": true, "bakanlığı": true, "zeynep": false, "clara": false, "valerin": false} {
		check("blocked "+stem, HasBlockedApostropheStem(stem, false, false), want)
	}
	check("blocked karaca", HasBlockedApostropheStem("karaca", false, false), true)
	check("blocked karaca skip", HasBlockedApostropheStem("karaca", true, false), false)
	check("zeynep'ten", IsValidGenericApostrophe("zeynep", "ten", false), true)
	check("zeynep'den", IsValidGenericApostrophe("zeynep", "den", false), false)
	check("ankara'in", IsValidGenericApostrophe("ankara", "in", false), false)
	check("ankara'nın", IsValidGenericApostrophe("ankara", "nın", false), true)
	check("google'ın", IsValidGenericApostrophe("google", "ın", false), true)
	check("mark'ya", IsValidGenericApostrophe("mark", "ya", false), false)

	// Acronym and mixed-case brand apostrophe detection.
	for typed, want := range map[string]AcronymApostrophe{
		"HEXIMden":  {"HEXIM", "den", "hexım'den"},
		"KDAların":  {"KDA", "ların", "kda'ların"},
		"TLSnin":    {"TLS", "nin", "tls'nin"},
		"MacOSdaki": {"MacOS", "daki", "macos'daki"},
		"OpenAIden": {"OpenAI", "den", "openaı'den"},
	} {
		got, ok := MatchAcronymApostrophe(typed)
		check("acronym "+typed, ok && got == want, true)
	}
	for _, typed := range []string{"AIde", "EVde", "URLs"} {
		_, ok := MatchAcronymApostrophe(typed)
		check("acronym "+typed, ok, false)
	}

	// Apostrophe suffix vowel harmony and disharmony restoration (keeping -ki).
	check("mortimer", IsHarmoniousApostropheMarkChange("mortimer", "i", "ı", false), false)
	check("ravenna", IsHarmoniousApostropheMarkChange("ravenna", "nin", "nın", false), true)
	check("mary", IsHarmoniousApostropheMarkChange("mary", "i", "ı", false), false)
	check("altay", IsHarmoniousApostropheMarkChange("altay", "in", "ın", false), true)
	check("begüm", IsHarmoniousApostropheMarkChange("begüm", "un", "ün", false), true)
	check("alkol", IsHarmoniousApostropheMarkChange("alkol", "un", "ün", true), true)
	check("istanbul", IsHarmoniousApostropheMarkChange("istanbul", "u", "ü", false), false)
	for word, want := range map[string]string{
		"ukrayna'sinda": "ukrayna'sında", "ankara'sindaki": "ankara'sındaki", "kda'larin": "kda'ların",
		"ankara'daki": "<null>", "ukrayna'sında": "<null>", "mortimer'sinda": "<null>",
	} {
		check("disharmony "+word, opt(FixDisharmoniousApostropheSuffix(word)), want)
	}

	// Doubled-locative plural typo detection.
	check("bölgedeler", opt(CollapsedDoubledLocative("bölgedeler", "de")), "bölgelerde")
	check("alandalar", opt(CollapsedDoubledLocative("alandalar", "da")), "alanlarda")
	check("bölgelerde", opt(CollapsedDoubledLocative("bölgelerde", "de")), "<null>")

	// Standalone question particle harmony.
	check("var mi", opt(HarmonizeQuestionParticle("mi", "var", false)), "mı")
	check("hazır misin", opt(HarmonizeQuestionParticle("misin", "hazır", false)), "mısın")
	check("gördü mu", opt(HarmonizeQuestionParticle("mu", "gördü", false)), "mü")
	check("saat mi", opt(HarmonizeQuestionParticle("mi", "saat", true)), "<null>")
	check("alkol mu", opt(HarmonizeQuestionParticle("mu", "alkol", true)), "mü")
	check("oldu mü", opt(HarmonizeQuestionParticle("mü", "oldu", false)), "<null>")

	// TDK place, geographic and state apostrophe head-word rules vs institutions.
	check("denizi", IsPlaceApostropheHead("denizi", ""), true)
	check("cumhuriyeti", IsPlaceApostropheHead("cumhuriyeti", ""), true)
	check("mahallesi", IsPlaceApostropheHead("mahallesi", ""), true)
	check("avrupa birliği", IsPlaceApostropheHead("birliği", "avrupa"), true)
	check("psikiyatri birliği", IsPlaceApostropheHead("birliği", "psikiyatri"), false)
	check("üniversitesi", IsPlaceApostropheHead("üniversitesi", ""), false)
	check("partisi", IsPlaceApostropheHead("partisi", ""), false)
	check("kurumu", IsPlaceApostropheHead("kurumu", ""), false)
	check("denizi'nin", IsValidPlaceApostrophe("denizi", "nin"), true)
	check("mahallesi'nde", IsValidPlaceApostrophe("mahallesi", "nde"), true)
	check("cumhuriyeti'nden", IsValidPlaceApostrophe("cumhuriyeti", "nden"), true)
	check("okyanus'ta", IsValidPlaceApostrophe("okyanus", "ta"), true)
	check("okyanus'da", IsValidPlaceApostrophe("okyanus", "da"), false)
	check("denizi'nın", IsValidPlaceApostrophe("denizi", "nın"), false)
	check("blocked denizi", HasBlockedApostropheStem("denizi", true, false), true)
	check("blocked denizi place", HasBlockedApostropheStem("denizi", true, true), false)
}

// fakeMorphology is a WordValidator of a word list, with analyses when it has them (analyses nil: no analyze).
type fakeMorphology struct {
	words    map[string]bool
	analyses map[string][]WordAnalysis
}

func (m fakeMorphology) IsWord(form string) bool { return m.words[form] }

// fakeAnalyzer is a fakeMorphology that analyzes too.
type fakeAnalyzer struct{ fakeMorphology }

func (m fakeAnalyzer) Analyze(form string) []WordAnalysis { return m.analyses[form] }

func TestCorrectorSyntheticRegressions(t *testing.T) {
	unigrams := strings.Join([]string{
		"ortaokul\t1550\t10", "ortaokulda\t160\t0", "orta\t5000\t100", "okul\t4000\t50", "gözlemci\t500\t20",
		"gözlemciden\t100\t0", "her\t50000\t500", "neyse\t5000\t50", "herneyse\t3792\t0", "ikinizin\t500\t5",
		"ikinizinde\t52\t0", "boyunu\t300\t0", "boyunda\t2000\t0", "giyside\t7\t0", "giysi\t1500\t10", "kitap\t5000\t50",
		"kitapta\t2000\t0", "hayal\t4000\t20", "hayalde\t120\t0", "çok\t80000\t500", "çokta\t80\t0", "yazım\t1182\t91",
		"yazımı\t794\t36", "yazılı\t5480\t108", "weeknd\t150\t130", "weeknd'in\t70\t65", "müslümanlar\t2000\t1800",
		"müslümanlara\t665\t500", "japon\t8000\t7000", "japonların\t400\t360", "yunan\t22000\t17000",
		"yunanlardan\t150\t130", "mortimer'ı\t12\t12", "ravenna'nın\t12\t12", "bağrım\t8\t0", "alveol\t25\t0",
		"yüklemek\t300\t0", "bre\t400\t30", "bölgelerde\t4800\t50", "bölgedeler\t2\t0", "elif\t500\t270",
		"elif'in\t120\t100", "elifin\t80\t3", "bursa\t5000\t3500", "bursa'dan\t500\t450", "ekim\t60000\t54000",
		"ekim'de\t800\t750", "ekimde\t300\t30", "cabası\t8\t0", "çabası\t804\t25", "asil\t1479\t187", "aşil'in\t53\t40",
		"suikastçı\t152\t0", "suikastçi\t84\t0", "var\t50000\t500", "mi\t10000\t10", "mı\t20000\t10", "saat\t10000\t200",
		"gordu\t0\t0", "gördü\t9944\t20", "mu\t10000\t10", "mü\t8000\t10", "miş\t159\t0", "mısın\t5000\t0",
		"gün\t40000\t200", "kim\t20000\t100", "gelecekm\t0\t0", "geleceksin\t5000\t0", "gelecek\t30000\t100",
		"katıldı\t8000\t10", "katıldımı\t0\t0", "katılımı\t5000\t10",
	}, "\n")
	bigrams := strings.Join([]string{"orta okul\t100", "ortaokul da\t2", "var mı\t5000", "saat mi\t40", "gördü mü\t300"}, "\n")
	knownTSV := strings.Join([]string{"kimmi\tkim mi\t0\t3000\tMINED", "varmı\tvar mı\t0\t5000\tMINED"}, "\n")
	morphology := fakeAnalyzer{fakeMorphology{
		words: setOf("ortaokul", "ortaokulda", "her", "neyse", "ikinizin",
			"boyunu", "boyunda", "giyside", "giysi", "kitap", "kitapta",
			"hayal", "hayalde", "hayalden", "çok", "çokta", "yazım", "yazımı", "yazılı", "müslümanlar", "müslümanlara",
			"japon", "japonların", "japonlarının", "yunan", "yunanlardan", "yunanlardandır",
			"cabası", "çabası", "asil", "suikastçı", "yükleyince", "bağrım", "alveol", "alveolüsün",
			"bre", "bölgelerde", "bölgedeler",
			"var", "varde", "varda", "vardan", "saat", "saatte", "saatten", "gördü", "günde", "kimmiş",
			"katıldı", "katılımı", "gelecek", "geleceksin"),
		analyses: map[string][]WordAnalysis{
			"japon":       {{Stem: "japon", Pos: "Noun", Morphemes: []string{"A3sg"}}},
			"japonların":  {{Stem: "japon", Pos: "Noun", Morphemes: []string{"A3pl", "Gen"}}},
			"yunan":       {{Stem: "yunan", Pos: "Noun", Morphemes: []string{"A3sg"}}},
			"yunanlardan": {{Stem: "yunan", Pos: "Noun", Morphemes: []string{"A3pl", "Abl"}}},
			"yükleyince":  {{Stem: "yükle", Pos: "Verb", Morphemes: []string{"When"}}},
			"alveolüsün":  {{Stem: "alveol", Pos: "Noun", Morphemes: []string{"A3sg", "P3sg", "Zero", "Pres", "A2sg"}}},
		},
	}}
	c := NewCorrector(LexiconFromTSV(unigrams, bigrams, 1, 1), CAREFUL, KnownMisspellingsFromTSV(knownTSV), morphology)
	for _, tc := range [][2]string{
		// The known particle suffixation: blocks kimmiş -> kim miş, allows varmısın -> var mısın.
		{"Kimmiş?", "Kimmiş?"},
		{"Varmısın?", "Var mısın?"},
		// Margin-based candidate comparison: ortaokul da -> ortaokulda.
		{"Çocuk ortaokul da okudu.", "Çocuk ortaokulda okudu."},
		// Join guards: two capitalized mid-sentence words, Her neyse, İkinizin de.
		{"Bu Gözlemci Den geldi.", "Bu Gözlemci Den geldi."},
		{"Her neyse tamam.", "Her neyse tamam."},
		{"İkinizin de gelmesi gerek.", "İkinizin de gelmesi gerek."},
		// Morphological particle splits + the typo hijacking guard + the voicing and harmony guards.
		{"Küçük boyunuda aldım.", "Küçük boyunu da aldım."},
		{"Giyside leke vardı.", "Giyside leke vardı."},
		{"Bu kitapda yazılı.", "Bu kitapta yazılı."},
		{"Bu hayalda güzel.", "Bu hayalda güzel."},
		{"Bu çok ta güzel.", "Bu çok ta güzel."},
		{"Güzel yazımmı bu.", "Güzel yazım mı bu."},
		// Apostrophe fixes and guards.
		{"Dün Weekndin şarkısı çaldı.", "Dün Weeknd'in şarkısı çaldı."},
		{"Burada Muslumanlara yardım edildi.", "Burada Müslümanlara yardım edildi."},
		{"Dönemin Japonlarının ve eski Yunanlardandır.", "Dönemin Japonlarının ve eski Yunanlardandır."},
		{"Dün Elifin kitabı geldi.", "Dün Elif'in kitabı geldi."},
		{"Bursadan geldik.", "Bursa'dan geldik."},
		{"Ekimde soğuk olur.", "Ekimde soğuk olur."},
		{"Bu 6 Ekimde oldu.", "Bu 6 Ekim'de oldu."},
		{"13 Ekimde oldu.", "13 Ekim'de oldu."},
		{"1. Ekimde oldu.", "1. Ekimde oldu."},
		{"Veri HEXIMden ve KDAların içinden geldi.", "Veri HEXIM'den ve KDA'ların içinden geldi."},
		{"Güncelleme MacOSdaki sorunu çözdü.", "Güncelleme MacOS'daki sorunu çözdü."},
		{"GÜNde iki kez.", "GÜNde iki kez."},
		// Marks fixes and guards.
		{"Bu işin cabası şudur.", "Bu işin cabası şudur."},
		{"Dün Asil'in ödevi bitti.", "Dün Asil'in ödevi bitti."},
		{"Yazar Mortimer'i anlattı.", "Yazar Mortimer'i anlattı."},
		{"Eski Ravenna'nin duvarları.", "Eski Ravenna'nın duvarları."},
		{"Prens Morza Bagrim burada kaldı.", "Prens Morza Bagrim burada kaldı."},
		{"Bu alveolusun yapısı.", "Bu alveolusun yapısı."},
		{"Bir suikastci yakalandı.", "Bir suikastçı yakalandı."},
		{"Tekrar yukleyince düzeldi.", "Tekrar yükleyince düzeldi."},
		{"Ukrayna'sinda yaşananlar.", "Ukrayna'sında yaşananlar."},
		{"var mi?", "var mı?"},
		{"gordu mu?", "gördü mü?"},
		{"saat mi?", "saat mi?"},
		// The short-stem ki guard, the doubled-locative typo guard and the unsplit harmonized question typo guard.
		{"Bu breki kazandı.", "Bu breki kazandı."},
		{"Başka bölgedelerde görüldü.", "Başka bölgedelerde görüldü."},
		{"Bu bölgelerdede görüldü.", "Bu bölgelerde de görüldü."},
		{"Toplantıya katıldımı?", "Toplantıya katıldı mı?"},
	} {
		if got := c.Correct(tc[0], CorrectOptions{}).Text; got != tc[1] {
			t.Errorf("%q:\n got %q\nwant %q", tc[0], got, tc[1])
		}
	}
	// Idempotence on outputs.
	for _, sample := range []string{"Küçük boyunuda aldım.", "Dün Weekndin şarkısı çaldı.", "Bursadan geldik.", "Veri HEXIMden geldi.", "gordu mu?"} {
		once := c.Correct(sample, CorrectOptions{}).Text
		if twice := c.Correct(once, CorrectOptions{}).Text; twice != once {
			t.Errorf("%q -> %q -> %q", sample, once, twice)
		}
	}
}

// fakeModel is a ContextModel of a scoring function.
type fakeModel struct {
	score func(queries []ContextQuery) [][]float64
	names bool
}

func (m fakeModel) Score(queries []ContextQuery) ([][]float64, error) { return m.score(queries), nil }
func (m fakeModel) SupportsNameContext() bool                         { return m.names }

func TestContextFeaturesAndOffsets(t *testing.T) {
	unigrams := strings.Join([]string{
		"oldu\t5000\t10", "öldü\t2000\t10", "uç\t1000\t5", "üç\t8000\t20", "bende\t10000\t50", "ben\t50000\t200",
		"de\t500000\t1000", "ev\t10000\t100", "evde\t8000\t50", "geldi\t20000\t100", "adam\t5000\t50", "cok\t10\t0",
		"çok\t100000\t500",
	}, "\n")
	analyses := map[string][]WordAnalysis{
		"oldu":  {{Stem: "ol", Pos: "Verb", Morphemes: []string{"Past", "A3sg"}}},
		"öldü":  {{Stem: "öl", Pos: "Verb", Morphemes: []string{"Past", "A3sg"}}},
		"evde":  {{Stem: "ev", Pos: "Noun", Morphemes: []string{"A3sg", "Loc"}}},
		"ev":    {{Stem: "ev", Pos: "Noun", Morphemes: []string{"A3sg"}}},
		"ben":   {{Stem: "ben", Pos: "Pron", Morphemes: []string{"A1sg"}}},
		"bende": {{Stem: "ben", Pos: "Pron", Morphemes: []string{"A1sg", "Loc"}}},
	}
	words := map[string]bool{}
	for w := range analyses {
		words[w] = true
	}
	morphology := fakeAnalyzer{fakeMorphology{words: words, analyses: analyses}}
	lex := LexiconFromTSV(unigrams, "ben de\t15000\nev de\t120", 1, 1)
	bitsOf := NewCorrector(lex, CAREFUL, nil, morphology)

	// The 8-bit morphology feature mask matches scripts/extract-morph-features.ts.
	for word, want := range map[string]int{"oldu": 1 | 2 | 128, "evde": 1 | 4 | 32, "ben": 1 | 16, "yokboyle": 0} {
		if got := bitsOf.morphFeatureBits(word); got != want {
			t.Errorf("morphFeatureBits(%s) = %d, want %d", word, got, want)
		}
	}
	feats := bitsOf.buildContextFeatures([]string{"oldu", "öldü"}, ContextMarks, true, lex, nil, nil)
	if len(feats) != 2 || len(feats[0]) != FeatureDim {
		t.Fatalf("features: %v", feats)
	}
	for _, f := range []struct {
		row, slot int
		want      float64
	}{{0, 1, 0}, {0, 3, 0}, {0, 4, 0}, {1, 4, 1}, {0, 5, 1}, {0, 6, 1}, {0, 10, 1}, {0, 11, 1}, {0, 17, 1}} {
		if feats[f.row][f.slot] != f.want {
			t.Errorf("feature %d of %d = %v, want %v", f.slot, f.row, feats[f.row][f.slot], f.want)
		}
	}
	if !(feats[1][1] < 0) {
		t.Errorf("the rarer candidate's relative log frequency is %v", feats[1][1])
	}

	// Per-regime contextOffsets: marksAscii adds to the context threshold, marksMarked more.
	var seenASCIIFlags []float64
	model := fakeModel{score: func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		for i, q := range queries {
			for j := range q.Candidates {
				seenASCIIFlags = append(seenASCIIFlags, q.Features[j][5])
				// The typed candidate gets 0, the alternative 3.5: clears ASCII (3.0) but not marked (4.0).
				score := 0.0
				if j%2 == 1 {
					score = 3.5
				}
				out[i] = append(out[i], score)
			}
		}
		return out
	}}
	settings := CAREFUL
	settings.Threshold = KindThresholds{}
	for k, v := range CAREFUL.Threshold {
		settings.Threshold[k] = v
	}
	settings.Threshold[KindContext] = 3.0
	settings.ContextOffsets = &ContextOffsets{MarksMarked: 1.0, MarksMarkedOnMarked: 2.5}
	c := NewCorrector(lex, settings, nil, morphology)
	if got := c.CorrectWithModel("Adam oldu.", model, CorrectOptions{}).Text; got != "Adam öldü." {
		t.Errorf("ASCII sentence: %q", got)
	}
	if got := c.CorrectWithModel("Adam çok oldu.", model, CorrectOptions{}).Text; got != "Adam çok oldu." {
		t.Errorf("marked sentence: %q", got)
	}
	if !slices.Contains(seenASCIIFlags, 1) || !slices.Contains(seenASCIIFlags, 0) {
		t.Errorf("ascii flags seen: %v", seenASCIIFlags)
	}
}

func TestContextTypoDisambiguation(t *testing.T) {
	unigrams := strings.Join([]string{
		"ondan\t5000\t100", "hiç\t50000\t200", "her\t100000\t500", "haber\t10000\t100", "alamadım\t2000\t0",
		"film\t8000\t100", "beni\t30000\t100", "çok\t100000\t100", "etkiledi\t1500\t0", "etkili\t11800\t0",
		"elektrik\t5000\t100", "kesintisi\t80\t0", "yüzünden\t8000\t0",
	}, "\n")
	morphology := fakeMorphology{words: setOf("ondan", "hiç", "her", "haber", "alamadım", "film", "beni", "çok", "etkiledi", "etkili", "elektrik", "kesintisi", "yüzünden")}
	c := NewCorrector(LexiconFromTSV(unigrams, "", 1, 1), CAREFUL, nil, morphology)

	// Without context the core picks the more frequent "her" for "hber", and leaves the tied "etkildi" and the
	// rare "kesintis" alone.
	for _, tc := range [][2]string{
		{"Ondan hiç hber alamadım.", "Ondan hiç her alamadım."},
		{"Film beni çok etkildi.", "Film beni çok etkildi."},
		{"Elektrik kesintis yüzünden.", "Elektrik kesintis yüzünden."},
	} {
		if got := c.Correct(tc[0], CorrectOptions{}).Text; got != tc[1] {
			t.Errorf("core %q: %q", tc[0], got)
		}
	}
	cands := c.typoCandidatesFor("hber", &WordContext{TextHasMarks: true}, "", 7)
	var texts []string
	for _, cand := range cands {
		texts = append(texts, cand.text)
	}
	if !slices.Equal(texts, []string{"her", "haber"}) {
		t.Errorf("typoCandidatesFor(hber) = %v", texts)
	}

	var seenLeft []string
	scoreMap := map[string]float64{
		"hber": -2.0, "her": 1.0, "haber": 5.5, // a delta of 4.5 over the core's "her"
		"etkildi": -2.0, "etkili": 1.0, "etkiledi": 4.0, // 6.0 over what was typed, a gap of 3.0
		"kesintis": -1.5, "kesintisi": 4.5, // 6.0 over what was typed
	}
	named := fakeModel{score: func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		for i, q := range queries {
			seenLeft = append(seenLeft, q.Left)
			for _, cand := range q.Candidates {
				out[i] = append(out[i], scoreMap[cand])
			}
		}
		return out
	}}
	run := func(text, want string, options CorrectOptions) {
		t.Helper()
		if got := c.CorrectWithModel(text, named, options).Text; got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
	// The override: the core's wrong "her" -> "haber"; when the model is not confident enough, the core's pick
	// stands.
	run("Ondan hiç hber alamadım.", "Ondan hiç haber alamadım.", CorrectOptions{})
	scoreMap["haber"] = 2.0
	run("Ondan hiç hber alamadım.", "Ondan hiç her alamadım.", CorrectOptions{})
	scoreMap["haber"] = 5.5
	// Ambiguous: the tied "etkildi" -> "etkiledi".
	run("Film beni çok etkildi.", "Film beni çok etkiledi.", CorrectOptions{})
	// Rescue: the rare "kesintis" -> "kesintisi" when confident, left alone when weak.
	run("Elektrik kesintis yüzünden.", "Elektrik kesintisi yüzünden.", CorrectOptions{})
	scoreMap["kesintisi"] = 0.5
	run("Elektrik kesintis yüzünden.", "Elektrik kesintis yüzünden.", CorrectOptions{})

	// The TDK SOMBAHCEMİ words ("Mademki", "illaki") are never split by the context model.
	kiCorrector := NewCorrector(LexiconFromTSV("mademki\t500\t10\nmadem\t500\t10\nillaki\t500\t0\nilla\t500\t0\nki\t50000\t0\n", "", 1, 1), CAREFUL, nil, morphology)
	splitAll := fakeModel{score: func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		for i, q := range queries {
			for j := range q.Candidates {
				out[i] = append(out[i], float64(min(j, 1)*10))
			}
		}
		return out
	}}
	if got := kiCorrector.CorrectWithModel("Mademki geldin, illaki konuşalım.", splitAll, CorrectOptions{}).Text; got != "Mademki geldin, illaki konuşalım." {
		t.Errorf("SOMBAHCEMİ: %q", got)
	}

	// The pause pass: "her" (typed "hber") is judged again with the whole sentence -> "haber".
	pauseText := "Ondan hiç her alamadım."
	herStart := runeIndex(pauseText, "her")
	run(pauseText, "Ondan hiç haber alamadım.", CorrectOptions{PriorTypoOriginals: []PriorTypo{{Start: herStart, End: herStart + 3, Original: "hber"}}})

	// The core-restored context: an earlier core mark fix ("cok" -> "çok") is clean in the left context the model
	// sees.
	seenLeft = nil
	c.CorrectWithModel("Film beni cok etkildi.", named, CorrectOptions{})
	if !slices.ContainsFunc(seenLeft, func(l string) bool { return strings.Contains(l, "çok") }) {
		t.Errorf("no core-restored çok in the left contexts: %q", seenLeft)
	}
}

func TestCorrectorInspect(t *testing.T) {
	c := testCorrector(t)
	userSentence := "ben şimdi mesela bir düzenleme yapıyorum ancak bazen metinleri duzenblemiyor yani çok basit kelime harf hatalarını duzenlemniyor bka burada mesela."
	inspected := c.Inspect(userSentence, CorrectOptions{})
	findFlag := func(flags []SpellFlag, original string) *SpellFlag {
		for i := range flags {
			if flags[i].Original == original {
				return &flags[i]
			}
		}
		return nil
	}
	findFix := func(corrections []Correction, original string) *Correction {
		for i := range corrections {
			if corrections[i].Original == original {
				return &corrections[i]
			}
		}
		return nil
	}
	checkResolved := func(wrong, want string) {
		t.Helper()
		if fixed := findFix(inspected.Corrections, wrong); fixed != nil {
			if fixed.Replacement != want {
				t.Errorf("the fix of %q is %q, want %q", wrong, fixed.Replacement, want)
			}
			return
		}
		flagged := findFlag(inspected.Flags, wrong)
		if flagged == nil {
			t.Errorf("%q is neither fixed nor flagged", wrong)
			return
		}
		if len(flagged.Suggestions) == 0 || flagged.Suggestions[0] != want || len(flagged.Suggestions) > 5 {
			t.Errorf("the suggestions for %q are %q, want %q first", wrong, flagged.Suggestions, want)
		}
	}
	checkResolved("duzenblemiyor", "düzenlemiyor")
	checkResolved("duzenlemniyor", "düzenlemiyor")
	checkResolved("bka", "bak")

	// Proper names, apostrophe forms, acronyms, mixed-case brands, URLs and emails, valid unseen forms, attested OOV
	// noun inflections, and contractions (when off) are not flagged.
	for _, s := range []string{
		"Ankara'ya giderken Ahmet ve Maniwaki hakkında konuştuk.",
		"Maniwaki Kanada'da yer alan küçük bir kasabadır.",
		"TDK ve NATO açıklamaları MacOS ve OpenAI üzerinden paylaşıldı.",
		"Detaylar https://ornek.com/abc adresinde ve test@ornek.com hesabında var.",
		"Yeni özgeçmişimi gönderdim, tripodlar da hazır.",
		"Burda noldu hiç bilmiyorum.",
		"Bazen metinleri düzenlemiyor ama çoğu zaman çalışıyor.",
	} {
		if flags := c.Inspect(s, CorrectOptions{}).Flags; len(flags) != 0 {
			t.Errorf("flags on the clean %q: %+v", s, flags)
		}
	}

	// Sentence-start typos are caught even though capitalized.
	for _, tc := range [][3]string{
		{"Hoxam bu hafta sonu ders var mı?", "Hoxam", "Hocam"},
		{"Tikım bugün çok iyi oynadı.", "Tikım", "Takım"},
		{"Otoparkat boş yer kalmamış.", "Otoparkat", "Otoparkta"},
	} {
		res := c.Inspect(tc[0], CorrectOptions{})
		if fixed := findFix(res.Corrections, tc[1]); fixed != nil {
			if fixed.Replacement != tc[2] {
				t.Errorf("%q fixed to %q", tc[1], fixed.Replacement)
			}
		} else if flagged := findFlag(res.Flags, tc[1]); flagged == nil || flagged.Suggestions[0] != tc[2] {
			t.Errorf("%q not caught: %+v", tc[1], flagged)
		}
	}

	// The reasons: "bka" -> "bak" is rare, the unseen "duzenlemniyor" invalid.
	if f := findFlag(inspected.Flags, "bka"); f == nil || f.Reason != FlagRare {
		t.Errorf("bka: %+v", f)
	}
	if f := findFlag(inspected.Flags, "duzenlemniyor"); f == nil || f.Reason != FlagInvalid {
		t.Errorf("duzenlemniyor: %+v", f)
	}

	// The context model re-ranks the suggestions of a flag left below the auto-fix bar.
	rerank := fakeModel{score: func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		for i, q := range queries {
			for _, cand := range q.Candidates {
				score := 0.0
				switch cand {
				case "düzenleniyor":
					score = 1.1
				case "düzenlemiyor":
					score = -0.1
				}
				out[i] = append(out[i], score)
			}
		}
		return out
	}}
	ctxRes := c.InspectWithModel("Bazen hatalarını duzenlemniyor ama çalışıyor.", rerank, CorrectOptions{})
	if f := findFlag(ctxRes.Flags, "duzenlemniyor"); f == nil || f.Suggestions[0] != "düzenleniyor" {
		t.Errorf("re-ranked flag: %+v, fixes %+v", f, ctxRes.Corrections)
	}
}

func TestCorrectorSentenceCase(t *testing.T) {
	c := testCorrector(t)
	prev := c.Settings
	c.Settings = CAREFUL
	c.Settings.SentenceCase = true
	defer func() { c.Settings = prev }()
	for _, tc := range sentenceCaseCases {
		if got := c.Correct(tc[0], CorrectOptions{}).Text; got != tc[1] {
			t.Errorf("%q:\n got %q\nwant %q", tc[0], got, tc[1])
		}
	}

	// The context model's "name" kind decides an ambiguous proper name.
	var seen [][]string
	nameModel := fakeModel{names: true, score: func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		for i, q := range queries {
			seen = append(seen, q.Candidates)
			for j := range q.Candidates {
				f := q.Features[j]
				isName := f[6] == 0 && f[7] == 0 && f[8] == 0 && f[9] == 0 && f[18] == 0
				score := 0.0
				if isName && f[19] == 1 {
					score = 4.5
				}
				out[i] = append(out[i], score)
			}
		}
		return out
	}}
	if got := c.CorrectWithModel("Hafta sonu özlemle sahilde yürüdük.", nameModel, CorrectOptions{}).Text; got != "Hafta sonu Özlem'le sahilde yürüdük." {
		t.Errorf("name model: %q", got)
	}
	if !slices.ContainsFunc(seen, func(c []string) bool { return len(c) == 2 && c[0] == "özlemle" && c[1] == "Özlem'le" }) {
		t.Errorf("name queries: %q", seen)
	}
}

// TestJSMath checks jsLog and jsPow on values Node computes (Math.log, Math.pow); a sweep of 400,000 logarithms
// and 410,000 powers of the prior's range matched Node's to the bit when they were written.
func TestJSMath(t *testing.T) {
	for _, tc := range []struct{ x, y, want float64 }{
		{23.088682131910968, 0.78, 11.573143359981222},
		{264.34452785004163, 0.82, 96.86785123985646},
		{491970.3619585999, 0.78, 27524.05222941537},
		{3.7089387876472576e-05, 0.78, 0.0003499570438534},
		{678951.6988441448, 0.78, 35386.232436939594},
	} {
		if got := jsPow(tc.x, tc.y); got != tc.want {
			t.Errorf("jsPow(%v, %v) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
	for x, want := range map[float64]float64{2: 0.6931471805599453, 10: 2.302585092994046, 1234567: 14.02623085927966,
		0.0003: -8.111728083308073, 1e-310: -713.8013788281542, 3.5: 1.252762968495368} {
		if got := jsLog(x); got != want {
			t.Errorf("jsLog(%v) = %v, want %v", x, got, want)
		}
	}
	if !math.IsInf(jsLog(0), -1) || !math.IsNaN(jsLog(-1)) {
		t.Error("jsLog(0) or jsLog(-1)")
	}
}

var correctorFixes = [][2]string{
	{"Bugün cok yorgunum.", "Bugün çok yorgunum."},
	{"Yarın Ankaraya gidiyorum.", "Yarın Ankara'ya gidiyorum."},
	{"Birşey söylemek istiyorum ama hiç bir fikrim yok.", "Bir şey söylemek istiyorum ama hiçbir fikrim yok."},
	{"Bu iş biter mi bilmiyorum, sen geldinmi?", "Bu iş biter mi bilmiyorum, sen geldin mi?"},
	{"Yarın sen de gelecekmisin?", "Yarın sen de gelecek misin?"},
	{"Eski sürüm daha iyiydi ama arenasıda doluydu.", "Eski sürüm daha iyiydi ama arenası da doluydu."},
	{"Öyle bir şey diyorki herkes güldü.", "Öyle bir şey diyor ki herkes güldü."},
	{"Oysa ki ben ona söylemiştim.", "Oysaki ben ona söylemiştim."},
	{"Herkez geldi, malesef sen yoktun.", "Herkes geldi, maalesef sen yoktun."},
	{"yapabiliur miyiz bunu", "yapabilir miyiz bunu"},
}

var correctorKeeps = []string{
	"Kitabı bende kaldı, sen de gel.",
	"Odada kimse yoktu, o da gelmedi.",
	"Uç noktada durdu ve dişini sıktı.",
	"Ankaralı arkadaşım Türklerin tarihini anlattı.",
	"Türk Dil Kurumundan bir mektup geldi.",
	"Kitabımı evde unuttum, sevgimi ona anlattım.",
	"Evdeki hesap çarşıya uymaz.",
}

var correctorExtraFixes = [][2]string{
	{"Büyük boyunu kullanıyordum, küçük boyunuda aldım.", "Büyük boyunu kullanıyordum, küçük boyunu da aldım."},
	{"İklim değişikliği yağış düzeninide doğrudan etkiliyor.", "İklim değişikliği yağış düzenini de doğrudan etkiliyor."},
	{"Yüz yüze görüşmelerede yer verilmiştir.", "Yüz yüze görüşmelere de yer verilmiştir."},
	{"Kendime almıştım, çok beğenince kardeşimede bir tane sipariş ettim.", "Kendime almıştım, çok beğenince kardeşime de bir tane sipariş ettim."},
	{"Çocuk ortaokul da okurken müzikle ilgilendi.", "Çocuk ortaokulda okurken müzikle ilgilendi."},
	{"Bu kayalık ta ilginç izler vardı.", "Bu kayalıkta ilginç izler vardı."},
	{"Son güncellemeyi test ortamına yükledinizmi?", "Son güncellemeyi test ortamına yüklediniz mi?"},
	{"Bu mont yeterince sıcak tutarmı diye tereddüt etmiştim.", "Bu mont yeterince sıcak tutar mı diye tereddüt etmiştim."},
	{"Aldığım takım gerçekten güzelmi güzel.", "Aldığım takım gerçekten güzel mi güzel."},
	{"Tasarım revizyonlarını Zeynepten teslim aldık.", "Tasarım revizyonlarını Zeynep'ten teslim aldık."},
	{"Bursadan gönderdiğimiz koli bu sabah teslim edilmiştir.", "Bursa'dan gönderdiğimiz koli bu sabah teslim edilmiştir."},
	{"Ödevin araştırma bölümünü Elifin hazırladığı kaynakçayla birleştirdik.", "Ödevin araştırma bölümünü Elif'in hazırladığı kaynakçayla birleştirdik."},
	{"Bağlantı TLSnin güncel sürümüyle korunuyor.", "Bağlantı TLS'nin güncel sürümüyle korunuyor."},
	{"Uygulamayı silip tekrar yukleyince düzeldi.", "Uygulamayı silip tekrar yükleyince düzeldi."},
	{"Kutunun köşesi biraz ezilmisti.", "Kutunun köşesi biraz ezilmişti."},
	{"Eve dönerken marketten ekmek ve yogurt alabilir misin?", "Eve dönerken marketten ekmek ve yoğurt alabilir misin?"},
	{"Bu dizustu bilgisayari oyun icin alan var mi?", "Bu dizüstü bilgisayarı oyun için alan var mı?"},
	{"Yüzden fazla kişi biliyorumki bunu gördü.", "Yüzden fazla kişi biliyorum ki bunu gördü."},
	{"Ravenna'nin eski yapıları korunmuştur.", "Ravenna'nın eski yapıları korunmuştur."},
	// v7 accuracy rules: marks, equal-margin threshold, ASCII sentence, apostrophe, typo channel, compound verbs, splits
	{"Dün okuldaydi ama hazirliklariyla ilgileniyordu.", "Dün okuldaydı ama hazırlıklarıyla ilgileniyordu."},
	{"Yarin erken gidecegiz ve bu konuyu kapatacaktir.", "Yarın erken gideceğiz ve bu konuyu kapatacaktır."},
	{"Bir dogrunun egrisi ve makasi cizildi.", "Bir doğrunun eğrisi ve makası çizildi."},
	{"Hava isinmadir ama gecenleri unutmadi.", "Hava ısınmadır ama geçenleri unutmadı."},
	{"Isa'nin dogumu ve Istanbul'da yasam.", "İsa'nın doğumu ve İstanbul'da yaşam."},
	{"Bu Odullerinde buyuk basari gosterdi.", "Bu Ödüllerinde büyük başarı gösterdi."},
	{"13 Kasımda toplantı yapıldı.", "13 Kasım'da toplantı yapıldı."},
	{"Dün Karacadan güzel bir haber geldi.", "Dün Karaca'dan güzel bir haber geldi."},
	{"Arkadaşım Discordu ve Arsenalı çok seviyor.", "Arkadaşım Discord'u ve Arsenal'ı çok seviyor."},
	{"Takım Malatyaspora karşı oynadı.", "Takım Malatyaspor'a karşı oynadı."},
	{"Yazar Aclan Beyin evine gitti.", "Yazar Aclan Bey'in evine gitti."},
	{"Güncelleme MacOSdaki ve OpenAIden gelen hatayı giderdi.", "Güncelleme MacOS'daki ve OpenAI'den gelen hatayı giderdi."},
	{"Çok masrflı bir mddesel liste hazırladm.", "Çok masraflı bir maddesel liste hazırladım."},
	{"Yeni çizien rapou zamanpa inceledik.", "Yeni çizilen raporu zamanla inceledik."},
	{"Bu kitap üzerined uzun uzun konuştuk.", "Bu kitap üzerinde uzun uzun konuştuk."},
	{"Çok çalışıyrum ama siparşim henüz gelmedi.", "Çok çalışıyorum ama siparişim henüz gelmedi."},
	{"Bu fikirden vaz geçtim ama zaman kayıp ettim.", "Bu fikirden vazgeçtim ama zaman kaybettim."},
	{"Kendimi iyi his ediyorum, bunu ön gördüm.", "Kendimi iyi hissediyorum, bunu öngördüm."},
	{"Durumu hemen farkettim ama evi terketti.", "Durumu hemen fark ettim ama evi terk etti."},
	{"Başvuru kabuledildi, çalışma devamediyor.", "Başvuru kabul edildi, çalışma devam ediyor."},
	{"Bu durumu siz de hisettiniz mi?", "Bu durumu siz de hissettiniz mi?"},
	{"Haftasonu yurtiçi seyahatimiz git gide uzadı.", "Hafta sonu yurt içi seyahatimiz gitgide uzadı."},
	{"Mağazaya hoşgeldiniz, bizimde haberimiz oldu.", "Mağazaya hoş geldiniz, bizim de haberimiz oldu."},
	{"Çok üzülsede istemesede bunu kabullendi.", "Çok üzülse de istemese de bunu kabullendi."},
	{"Böyle bir ihtimalide göz önünde bulundurduk.", "Böyle bir ihtimali de göz önünde bulundurduk."},
	{"Burası Ankara'dada bilinir, Çin'dirki meşhurdur.", "Burası Ankara'da da bilinir, Çin'dir ki meşhurdur."},
	// v8 user-reported short words, misplaced space, two-error combos, vowel drops, and TDK place-name apostrophes
	{"Dün bne eve geç geldim.", "Dün ben eve geç geldim."},
	{"Bu akşam ane eve geldi.", "Bu akşam anne eve geldi."},
	{"Bugün ann e erken geldi.", "Bugün anne erken geldi."},
	{"Bunu neden yapmıyorsunanlamadim.", "Bunu neden yapmıyorsun anlamadım."},
	// Pronoun + "de" before a verb of the same person is the conjunction; before a third-person verb it is locative.
	{"Dün akşam bende geldim.", "Dün akşam ben de geldim."},
	{"Bizde gittik oraya.", "Biz de gittik oraya."},
	{"Kitap bende kaldı.", "Kitap bende kaldı."},
	// Spoken "bişey" is written "bir şey".
	{"Bişey yazıyorum ama düzeltmiyor.", "Bir şey yazıyorum ama düzeltmiyor."},
	{"hiçbişey olmadı", "hiçbir şey olmadı"},
	// Missed letters and swapped vowels (measured on synthetic chat data, half A tuned / half B checked).
	{"Mesela burada nuler oluyor?", "Mesela burada neler oluyor?"},
	{"Mesela brada neler oluyor?", "Mesela burada neler oluyor?"},
	{"Şuna bir bka.", "Şuna bir bak."},
	{"Bu aralar çok çalşıyorum.", "Bu aralar çok çalışıyorum."},
	// Was kept only to avoid "görürüz" (a dropped "ş"); a Turkish letter is now kept, so the missed "ü" is restored.
	{"Yarın akşam görüşrüz.", "Yarın akşam görüşürüz."},
	{"Kitap bende kaldı.", "Kitap bende kaldı."},
	// A doubled suffix is not a second word: "değildirdir" must not become "değil dırdır".
	{"Bu iyi değildirdir.", "Bu iyi değildirdir."},
	{"Yarın ne yapacağını biliyormusunm?", "Yarın ne yapacağını biliyor musun?"},
	{"Birazdan yanına geliyorm.", "Birazdan yanına geliyorum."},
	{"Bunu tek başıma yapabilirm.", "Bunu tek başıma yapabilirim."},
	{"İstersen yarın eve gidebilirm.", "İstersen yarın eve gidebilirim."},
	{"Yoldayım birazdan gelyorm.", "Yoldayım birazdan geliyorum."},
	{"Bu proje üzerinde çok çalışıyrm.", "Bu proje üzerinde çok çalışıyorum."},
	{"Bu anlaşma ABDde ve ABDnin tarihinde önemlidir.", "Bu anlaşma ABD'de ve ABD'nin tarihinde önemlidir."},
	{"Gemi Marmara Denizinin ortasından geçti.", "Gemi Marmara Denizi'nin ortasından geçti."},
	{"Ankara, Türkiye Cumhuriyetinin başkentidir.", "Ankara, Türkiye Cumhuriyeti'nin başkentidir."},
	{"Ailem Kadıköy Mahallesinde oturuyor.", "Ailem Kadıköy Mahallesi'nde oturuyor."},
	{"Yazın Van Gölünde ve Ağrı Dağının eteklerinde gezdik.", "Yazın Van Gölü'nde ve Ağrı Dağı'nın eteklerinde gezdik."},
	{"Akşam İstiklal Caddesinde yürüyüş yaptık.", "Akşam İstiklal Caddesi'nde yürüyüş yaptık."},
	{"Dugonglar Büyük Okyanusta yaşarlar.", "Dugonglar Büyük Okyanus'ta yaşarlar."},
	{"Belge Osmanlı Devletinin arşivindedir.", "Belge Osmanlı Devleti'nin arşivindedir."},
	{"Ülke Avrupa Birliğine resmen başvurdu.", "Ülke Avrupa Birliği'ne resmen başvurdu."},
}

var correctorExtraKeeps = []string{
	"Kimmiş o kapıdaki?",
	"Her neyse, bunu sonra konuşuruz.",
	"İkinizin de gelmesi gerekiyor.",
	"Giyside küçük bir leke vardı.",
	"Ekimde havalar yavaş yavaş soğur.",
	"Bu işin cabası da var.",
	"Yapay zekâyı kolluyor, ona yardımcı oluyorduk.",
	"Düğmeye basınız lütfen.",
	"Toplantıya Asil'in ekibi de katıldı.",
	"Bu saat mi doğru gösteriyor?",
	"Sırpları ve Müslümanlara yapılan yardımları anlattı.",
	"Hırvatçadan çevrilen metni okuduk.",
	"Ta çocukluğundan beri burada yaşar.",
	"O dönemin Japonlarının etkisi büyüktür.",
	"Bu gelenek eski Yunanlardandır.",
	"Yazar Mortimer'i bu bölümde yeniden anlattı.",
	"Prens Morza Bagrim bu bölgede uzun süre kaldı.",
	"Akciğerdeki alveolusun yapısı mikroskopla incelendi.",
	"Bu breki maçı belirledi.",
	"Aynı durum başka bölgedelerde görüldü.",
	// v7 precision guards
	"Danimarkacada bu sözcük farklı kullanılır.",
	"Macbeth'inde ve Sözlük'ünde aynı tema işlenir.",
	"Fatimi devleti tarih kitaplarında anlatılır.",
	"Dekolmanı ameliyatla tedavi edildi.",
	"Bu işin olmağı zor görünüyor.",
	"İnsan bazen lmre gibi yazımlar görür.",
	// v8 precision & TDK institution guards
	"Ann e mektup göndermek istedi.",
	"Bu ne de güzel, o da geldi.",
	"Dün ben bir an şu ne dedim.",
	"Öğrenci Ankara Üniversitesinde okuyor.",
	"Lider Adalet Partisinin kararını açıkladı.",
	"Yüksek Seçim Kurulunun verileri yayımlandı.",
	"Bu rapor Amerikan Psikiyatri Birliğinin çalışmasıdır.",
	"Mezun olduğu Kabataş Erkek Lisesinin takımında oynadı.",
	"Bu maç Superbowl akşamında izlendi.",
	"Hımm, bu fikri daha önce düşünmemiştim.",
}

var sentenceCaseCases = [][2]string{
	// Multi-word institutions (NO apostrophe per TDK) & exception (Avrupa Birliği'ne)
	{"Başvuruyu türk dil kurumuna gönderdik.", "Başvuruyu Türk Dil Kurumuna gönderdik."},
	{"Dün Türk Dil Kurumu'ndan bir mektup geldi.", "Dün Türk Dil Kurumundan bir mektup geldi."},
	{"Ankara Üniversitesi'nde konferans var.", "Ankara Üniversitesinde konferans var."},
	{"Türkiye Büyük Millet Meclisi'ne sunuldu.", "Türkiye Büyük Millet Meclisine sunuldu."},
	{"Toplantı avrupa birliğine bildirildi.", "Toplantı Avrupa Birliği'ne bildirildi."},
	// Institution officer titles (WITH apostrophe)
	{"Dilekçeyi Türk Dil Kurumu başkanına verdik.", "Dilekçeyi Türk Dil Kurumu Başkanı'na verdik."},
	// Geographic & monument phrases (WITH apostrophe)
	{"Hafta sonu van gölünde yüzdük.", "Hafta sonu Van Gölü'nde yüzdük."},
	{"Dün rumeli hisarında fotoğraf çektik.", "Dün Rumeli Hisarı'nda fotoğraf çektik."},
	{"Kışın doğu anadolu bölgesinde kar yağar.", "Kışın Doğu Anadolu Bölgesi'nde kar yağar."},
	// Wrong-apostrophe removal & softening + -izm/-iyet preservation
	{"Türkçe'yi çok güzel konuşuyor.", "Türkçeyi çok güzel konuşuyor."},
	{"Hristiyanlık'ın tarihi üzerine bir kitap okudu.", "Hristiyanlığın tarihi üzerine bir kitap okudu."},
	{"Ankaralı'lar bu havaya alışıktır.", "Ankaralılar bu havaya alışıktır."},
	{"Türkler'in tarihi çok eskidir.", "Türklerin tarihi çok eskidir."},
	{"Başbakanlık'a dilekçe yazdık.", "Başbakanlığa dilekçe yazdık."},
	{"İslamiyet'e sadakat önemlidir.", "İslamiyet'e sadakat önemlidir."},
	// Misplaced apostrophe relocation
	{"Selinin'kini masada bıraktım.", "Selin'inkini masada bıraktım."},
	{"Türkiyemiz'in geleceği parlaktır.", "Türkiye'mizin geleceği parlaktır."},
	// Number + suffix apostrophe & voicing
	{"Bu bina 1985te inşa edilmiş.", "Bu bina 1985'te inşa edilmiş."},
	{"Rapor 2023de tamamlandı.", "Rapor 2023'te tamamlandı."},
	{"Olay 1985'de yaşanmış.", "Olay 1985'te yaşanmış."},
	{"Bu tarz 80lere çok uygun.", "Bu tarz 80'lere çok uygun."},
	// Titles, honorifics & post-titles
	{"Toplantıyı prof. dr. ahmet yılmaz yönetti.", "Toplantıyı Prof. Dr. Ahmet Yılmaz yönetti."},
	{"Dersi ahmet hocaya sorduk.", "Dersi Ahmet Hoca'ya sorduk."},
	{"Sayın başkana teşekkür ederiz.", "Sayın Başkan'a teşekkür ederiz."},
	// Specific dates, holidays & historical eras
	{"Toplantı 17 aralıkta yapılacak.", "Toplantı 17 Aralık'ta yapılacak."},
	{"Bu eser eski çağın izlerini taşıyor.", "Bu eser Eski Çağ'ın izlerini taşıyor."},
	{"Tören cumhuriyet bayramında yapıldı.", "Tören Cumhuriyet Bayramı'nda yapıldı."},
	// Clean-text protections (must stay completely untouched)
	{"Bay Reede, ofisten aradılar.", "Bay Reede, ofisten aradılar."},
	{"Anna, sen onun en iyi arkadaşısın.", "Anna, sen onun en iyi arkadaşısın."},
	{"Jimi Hendrix sahneye çıktı.", "Jimi Hendrix sahneye çıktı."},
	{"Gabrielle uzun bir yolculuğa çıktı.", "Gabrielle uzun bir yolculuğa çıktı."},
	{"Lynne bu çalışmayı inceledi.", "Lynne bu çalışmayı inceledi."},
	{"Michelin Rehberi ünlü bir ödüldür.", "Michelin Rehberi ünlü bir ödüldür."},
	{"Tam 3 gün arda arda Meksika'da kaldı.", "Tam 3 gün arda arda Meksika'da kaldı."},
	{"Lig'inde yer alan dokuz kulüpten biridir.", "Lig'inde yer alan dokuz kulüpten biridir."},
	{"Hâlâ İngilizce kursuna gidiyor mu?", "Hâlâ İngilizce kursuna gidiyor mu?"},
	{"Öğrencilerine olan sevgisi gözlerinden okunuyordu.", "Öğrencilerine olan sevgisi gözlerinden okunuyordu."},
	{"Canım, yarın görüşürüz.", "Canım, yarın görüşürüz."},
	{"Borcunu zamanında ödemiş.", "Borcunu zamanında ödemiş."},
}
