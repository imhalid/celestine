package spell

import (
	"slices"
	"strings"
	"testing"

	"github.com/imhalid/celestine/internal/spell/morph"
)

// This file is tinyfix-tr's pkg/test/autr.test.ts, contraction.test.ts, and the parts of inspect.test.ts,
// context.test.ts and data-and-worker.test.ts that need neither the Worker nor ONNX nor downloads.

// noPunct are all kinds but punctuation and contractions: the threshold tests work on one word, whose sentence
// capital is not their subject.
var noPunct = []string{KindTurkishLetters, KindApostropheFix, KindJoinedSeparate, KindSpelling}

// engineOfTSV is an engine of TSV texts, as createTdkai(RawData) makes one: no morphology, minCount and minPair 1.
func engineOfTSV(unigrams, bigrams, known string, mode Mode) *Engine {
	return NewEngine(LexiconFromTSV(unigrams, bigrams, 1, 1), KnownMisspellingsFromTSV(known), nil, mode)
}

var testUnigrams = strings.Join([]string{
	"bugün\t8000\t100", "çok\t10000\t100", "yorgunum\t500\t0", "nasıl\t5000\t50", "yarın\t3000\t100",
	"ankara\t50000\t49000", "ankara'ya\t4000\t3900", "gidiyorum\t2000\t0", "sen\t20000\t100", "geldin\t1500\t0",
	"ev\t10000\t100", "de\t500000\t1000", "evde\t8000\t50", "kaldım\t3000\t0", "bir\t100000\t500", "şey\t50000\t100",
	"hiç\t40000\t100", "hiçbir\t20772\t100", "fikrim\t1000\t0", "yok\t30000\t100", "ama\t80000\t200",
	"herkes\t3709\t50", "maalesef\t242\t10", "geldi\t20000\t100", "yapabilir\t15000\t10", "miyiz\t1000\t0",
	"bunu\t25000\t100", "soz\t5\t0", "söz\t80\t0", "uc\t25\t0", "uç\t2500\t0", "nokta\t5000\t0", "değil\t20000\t50",
}, "\n")

var testBigrams = "ev de\t10\nbir şey\t5325\nhiç bir\t45"

var testKnown = strings.Join([]string{
	"# wrong\tright\twrong_count\tright_count\tsection",
	"herkez\therkes\t0\t3709\tMISSPELLINGS",
	"malesef\tmaalesef\t3\t242\tMISSPELLINGS",
	"birşey\tbir şey\t4\t5325\tMISSPELLINGS",
	"hiç bir\thiçbir\t45\t20772\tMISSPELLINGS",
}, "\n")

func testEngine(mode Mode) *Engine { return engineOfTSV(testUnigrams, testBigrams, testKnown, mode) }

// summary is each fix's original, replacement and kind.
func summary(fixes []Fix) [][3]string {
	out := [][3]string{}
	for _, f := range fixes {
		out = append(out, [3]string{f.Original, f.Replacement, f.Kind})
	}
	return out
}

func TestEngineFixesAllKinds(t *testing.T) {
	e := testEngine(ModeCareful)
	text := "Bugün cok yorgunum. Yarın Ankaraya gidiyorum. Birşey yok ama hiç bir fikrim yok. Herkez geldi, malesef yapabiliur miyiz bunu? Sen geldinmi? Ev de kaldım."
	fixes := e.Check(text, CheckOptions{})
	rs := []rune(text)
	for i, fix := range fixes {
		if i > 0 && fixes[i-1].To > fix.From {
			t.Errorf("fixes overlap: %+v %+v", fixes[i-1], fix)
		}
		if string(rs[fix.From:fix.To]) != fix.Original {
			t.Errorf("fix %+v is not at its place", fix)
		}
		if fix.Confidence < 0 || fix.Confidence > 1 || !slices.Contains(AllFixKinds, fix.Kind) {
			t.Errorf("fix %+v", fix)
		}
	}
	byOriginal := map[string]Fix{}
	for _, f := range fixes {
		byOriginal[f.Original] = f
	}
	for _, want := range [][3]string{
		{"cok", "çok", KindTurkishLetters},
		{"Ankaraya", "Ankara'ya", KindApostropheFix},
		{"Birşey", "Bir şey", KindJoinedSeparate},
		{"hiç bir", "hiçbir", KindJoinedSeparate},
		{"geldinmi", "geldin mi", KindJoinedSeparate},
		{"Ev de", "Evde", KindJoinedSeparate},
		{"Herkez", "Herkes", KindSpelling},
		{"malesef", "maalesef", KindSpelling},
		{"yapabiliur", "yapabilir", KindSpelling},
	} {
		if f := byOriginal[want[0]]; f.Replacement != want[1] || f.Kind != want[2] {
			t.Errorf("%s: %+v", want[0], f)
		}
	}
	applied, err := Apply(text, fixes)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Bugün çok yorgunum. Yarın Ankara'ya gidiyorum. Bir şey yok ama hiçbir fikrim yok. Herkes geldi, maalesef yapabilir miyiz bunu? Sen geldin mi? Evde kaldım."; applied != want {
		t.Errorf("applied: %q", applied)
	}
	if fixed := e.Fix(text, CheckOptions{}); fixed != applied {
		t.Errorf("Fix: %q", fixed)
	}
}

func TestEngineKinds(t *testing.T) {
	e := testEngine(ModeCareful)
	text := "Bugün cok yorgunum, Ankaraya birşey ve herkez geldi."
	for kind, want := range map[string][3]string{
		KindTurkishLetters: {"cok", "çok", KindTurkishLetters},
		KindJoinedSeparate: {"birşey", "bir şey", KindJoinedSeparate},
		KindSpelling:       {"herkez", "herkes", KindSpelling},
	} {
		if got := summary(e.Check(text, CheckOptions{Kinds: []string{kind}})); !slices.Equal(got, [][3]string{want}) {
			t.Errorf("%s: %v", kind, got)
		}
	}
	if got := e.Check(text, CheckOptions{Kinds: []string{}}); len(got) != 0 {
		t.Errorf("no kinds: %v", got)
	}
}

func TestEngineRange(t *testing.T) {
	e := testEngine(ModeCareful)
	// A word's range in the second sentence leaves the first sentence's fix out.
	sentence := "Bugün cok yorgunum. Yarın Ankaraya gidiyorum."
	at := runeIndex(sentence, "Ankaraya")
	if got := summary(e.Check(sentence, CheckOptions{Range: &[2]int{at, at + 8}})); !slices.Equal(got, [][3]string{{"Ankaraya", "Ankara'ya", KindApostropheFix}}) {
		t.Errorf("Ankaraya: %v", got)
	}
	// The question particle is read from the context, though the question mark is outside the range.
	question := "Sen geldinmi?"
	at = runeIndex(question, "geldinmi")
	if got := e.Check(question, CheckOptions{Range: &[2]int{at, at + 8}}); len(got) != 1 || got[0].Replacement != "geldin mi" {
		t.Errorf("geldinmi: %v", got)
	}
	// Turkish letters in the rest of the text are read as context too.
	if got := e.Check("uc nokta", CheckOptions{Range: &[2]int{0, 2}}); len(got) != 1 || got[0].Replacement != "Uç" {
		t.Errorf("uc nokta: %v", got)
	}
	if got := e.Check("uc nokta değil", CheckOptions{Range: &[2]int{0, 2}}); len(got) != 1 || got[0].Replacement != "Uc" || got[0].Kind != KindPunctuation {
		t.Errorf("uc nokta değil: %v", got)
	}
	// A two-word join is caught when the second word's range is given.
	joinText := "Ev de kaldım."
	at = runeIndex(joinText, "de")
	if got := e.Check(joinText, CheckOptions{Range: &[2]int{at, at + 2}}); len(got) != 1 || got[0].Original != "Ev de" || got[0].Replacement != "Evde" {
		t.Errorf("Ev de: %v", got)
	}
	// An empty range has no fixes.
	if got := e.Check(sentence, CheckOptions{Range: &[2]int{5, 5}}); len(got) != 0 {
		t.Errorf("empty range: %v", got)
	}
}

func TestEngineModes(t *testing.T) {
	careful, balanced := testEngine(ModeCareful), testEngine(ModeBalanced)
	// "soz" (5) -> "söz" (80): below the careful threshold, above the balanced one.
	opts := CheckOptions{Kinds: noPunct}
	if got := careful.Fix("soz", opts); got != "soz" {
		t.Errorf("careful: %q", got)
	}
	if got := balanced.Fix("soz", opts); got != "söz" {
		t.Errorf("balanced: %q", got)
	}
	// The mode of one call, and SetMode.
	if got := careful.Fix("soz", CheckOptions{Kinds: noPunct, Mode: ModeBalanced}); got != "söz" {
		t.Errorf("one call balanced: %q", got)
	}
	if got := careful.Fix("soz", opts); got != "soz" {
		t.Errorf("careful again: %q", got)
	}
	careful.SetMode(ModeBalanced)
	if got := careful.Fix("soz", opts); got != "söz" {
		t.Errorf("SetMode: %q", got)
	}
}

func TestEngineIgnore(t *testing.T) {
	e := testEngine(ModeCareful)
	e.Ignore("Herkez")
	if got := e.Fix("Herkez ve herkez geldi.", CheckOptions{}); got != "Herkez ve herkez geldi." {
		t.Errorf("ignored: %q", got)
	}
	e.Unignore("herkez")
	if got := e.Fix("Herkez geldi.", CheckOptions{}); got != "Herkes geldi." {
		t.Errorf("unignored: %q", got)
	}
}

func TestEngineLeavesCodeAlone(t *testing.T) {
	e := testEngine(ModeCareful)
	fixes := e.Check("Bugün cok iyiyim ama `cok herkez` ve\n```\ncok herkez\n```\ndokunulmaz.", CheckOptions{})
	if len(fixes) != 1 || fixes[0].From != 6 || fixes[0].Original != "cok" || fixes[0].Replacement != "çok" {
		t.Errorf("fixes: %+v", fixes)
	}
}

func TestApplyErrors(t *testing.T) {
	if _, err := Apply("cok", []Fix{{From: 0, To: 3, Original: "xyz", Replacement: "abc", Kind: KindSpelling, Confidence: 0.9, Scope: ScopeWord}}); err == nil {
		t.Error("a fix whose original is not in the text was applied")
	}
	overlapping := []Fix{
		{From: 0, To: 3, Original: "cok", Replacement: "çok", Kind: KindTurkishLetters, Confidence: 0.9, Scope: ScopeWord},
		{From: 2, To: 5, Original: "k y", Replacement: "k z", Kind: KindSpelling, Confidence: 0.9, Scope: ScopeContext},
	}
	if _, err := Apply("cok y", overlapping); err == nil {
		t.Error("overlapping fixes were applied")
	}
}

func TestEnginePunctuation(t *testing.T) {
	known := strings.Join([]string{testKnown, "ankara\tAnkara\t0\t50000\tNAME", "türkçe\tTürkçe\tnoapos\t40000\tNAME", "türk dil kurumu\tTürk Dil Kurumu\t0\t10000\tPHRASE_NOAPOS"}, "\n")
	e := engineOfTSV(testUnigrams, testBigrams, known, ModeCareful)
	text := "dün türk dil kurumuna geldi.yarın 1985te ankaraya gidiyorum, türkçe'yi sevdik."
	fixes := e.Check(text, CheckOptions{Kinds: []string{KindPunctuation}})
	if len(fixes) == 0 {
		t.Fatal("no fixes")
	}
	for _, f := range fixes {
		if f.Kind != KindPunctuation {
			t.Errorf("fix %+v", f)
		}
	}
	if applied, _ := Apply(text, fixes); applied != "Dün Türk Dil Kurumuna geldi. Yarın 1985'te Ankara'ya gidiyorum, Türkçeyi sevdik." {
		t.Errorf("applied: %q", applied)
	}
	// Without punctuation these changes are not made.
	if got := e.Fix("1985te ankaraya geldik.", CheckOptions{Kinds: noPunct}); got != "1985te ankaraya geldik." {
		t.Errorf("no punctuation: %q", got)
	}
}

var contractionUnigrams = strings.Join([]string{"burada\t5000\t10", "ne\t90000\t100", "oldu\t5000\t0", "bilmiyorum\t2000\t0", "herkes\t3709\t50"}, "\n")

var contractionKnown = strings.Join([]string{
	"herkez\therkes\t0\t3709\tMISSPELLINGS",
	"burda\tburada\t0\t5000\tCONTRACTION",
	"noldu\tne oldu\t0\t800\tCONTRACTION",
}, "\n")

func TestEngineContractions(t *testing.T) {
	if slices.Contains(DefaultFixKinds, KindContractionFix) {
		t.Error("contractions are a default kind")
	}
	e := engineOfTSV(contractionUnigrams, "ne oldu\t800\n", contractionKnown, ModeCareful)
	if got := e.Fix("Burda noldu, herkez bilmiyorum.", CheckOptions{}); got != "Burda noldu, herkes bilmiyorum." {
		t.Errorf("by default: %q", got)
	}
	got := summary(e.Check("Burda noldu?", CheckOptions{Kinds: append(slices.Clone(DefaultFixKinds), KindContractionFix)}))
	if want := [][3]string{{"Burda", "Burada", KindContractionFix}, {"noldu", "ne oldu", KindContractionFix}}; !slices.Equal(got, want) {
		t.Errorf("asked for: %v", got)
	}
}

func TestEngineScopes(t *testing.T) {
	e := engineOfTSV(contractionUnigrams+"\nhiçbir\t9000\t0\nhiç\t9000\t0\nbir\t90000\t0", "ne oldu\t800\n", contractionKnown+"\nhiç bir\thiçbir\t45\t9000\tJOINED_ALWAYS", ModeCareful)
	var got [][3]string
	for _, f := range e.Check("herkez hiç bir şey bilmiyorum", CheckOptions{}) {
		got = append(got, [3]string{f.Original, f.Replacement, f.Scope})
	}
	if want := [][3]string{{"herkez", "Herkes", ScopeWord}, {"hiç bir", "hiçbir", ScopeContext}}; !slices.Equal(got, want) {
		t.Errorf("scopes: %v", got)
	}
}

// binaryBundleEngine is an engine of a binary lexicon and a morphology of dictionary texts.
func binaryBundleEngine(t *testing.T, unigrams []BinaryUnigramInput, dictionary, known string) *Engine {
	t.Helper()
	bin, err := EncodeBinaryLexicon(EncodeBinaryLexiconOptions{Unigrams: unigrams, Bigrams: []BinaryBigramInput{{First: "bir", Second: "şey", Count: 5325}}})
	if err != nil {
		t.Fatal(err)
	}
	lexicon, err := LexiconFromBinary(bin)
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(lexicon, KnownMisspellingsFromTSV(known), morph.FromDictionaries(dictionary), ModeCareful)
}

func TestEngineBinaryAndMorphology(t *testing.T) {
	e := binaryBundleEngine(t, []BinaryUnigramInput{
		{Form: "bugün", Count: 8000, Capitalized: 100}, {Form: "çok", Count: 10000, Capitalized: 100},
		{Form: "getirmek", Count: 5000, Capitalized: 10}, {Form: "boyun", Count: 2000, Capitalized: 10},
		{Form: "boyunu", Count: 300}, {Form: "boyunda", Count: 2000}, {Form: "yüklemek", Count: 3000},
		{Form: "yüklediniz", Count: 50}, {Form: "herkes", Count: 3709, Capitalized: 50}, {Form: "geldi", Count: 20000, Capitalized: 100},
	}, "getirmek [A:Aorist_I]\nboy\nyüklemek\n", "herkez\therkes\t0\t3709\tMISSPELLINGS\n")
	// The morphology keeps the rare but valid "getirsek" (not "getirmek"), and splits "boyunuda" and "yükledinizmi?".
	for _, tc := range [][2]string{
		{"Bugün getirsek olur.", "Bugün getirsek olur."},
		{"Boyunuda aldım, yükledinizmi?", "Boyunu da aldım, yüklediniz mi?"},
		{"Herkez cok geldi.", "Herkes çok geldi."},
	} {
		if got := e.Fix(tc[0], CheckOptions{}); got != tc[1] {
			t.Errorf("%q: %q", tc[0], got)
		}
	}
}

// TestEngineInspect is inspect.test.ts's first test with the engine's data: the TypeScript builds a morphology with a
// made-up generative prior, which the Go morphology does not export a way to build.
func TestEngineInspect(t *testing.T) {
	lexicon, known, morphology := loadTestData(t)
	e := NewEngine(lexicon, known, morphology, ModeCareful)
	text := "Bugün cok herkez hatalarını duzenlemniyor bka Ankara'ya `duzenlemniyor` geldi."
	res := e.Inspect(text, CheckOptions{})
	var fixes [][2]string
	for _, f := range res.Fixes {
		fixes = append(fixes, [2]string{f.Original, f.Replacement})
	}
	if want := [][2]string{{"cok", "çok"}, {"herkez", "herkes"}}; !slices.Equal(fixes, want) {
		t.Errorf("fixes: %v", fixes)
	}
	flagged := func(r InspectResult) []string {
		out := []string{}
		for _, f := range r.Flags {
			out = append(out, f.Original)
		}
		return out
	}
	if got := flagged(res); !slices.Equal(got, []string{"duzenlemniyor", "bka"}) {
		t.Fatalf("flags: %+v", res.Flags)
	}
	rs := []rune(text)
	duzen, bka := res.Flags[0], res.Flags[1]
	if string(rs[duzen.From:duzen.To]) != "duzenlemniyor" || duzen.Reason != "invalid" || !slices.Contains(duzen.Suggestions, "düzenlemiyor") || len(duzen.Suggestions) > 5 {
		t.Errorf("duzenlemniyor: %+v", duzen)
	}
	if string(rs[bka.From:bka.To]) != "bka" || bka.Reason != "rare" || bka.Suggestions[0] != "bak" {
		t.Errorf("bka: %+v", bka)
	}
	// The range keeps only what is in it.
	inRange := e.Inspect(text, CheckOptions{Range: &[2]int{bka.From, bka.To}})
	if len(inRange.Fixes) != 0 || !slices.Equal(flagged(inRange), []string{"bka"}) {
		t.Errorf("range: %+v", inRange)
	}
	// Without "yazim" there are no flags; with it there are.
	onlyMarks := e.Inspect(text, CheckOptions{Kinds: []string{KindTurkishLetters}})
	if len(onlyMarks.Fixes) != 1 || onlyMarks.Fixes[0].Original != "cok" || len(onlyMarks.Flags) != 0 {
		t.Errorf("only marks: %+v", onlyMarks)
	}
	onlySpelling := e.Inspect(text, CheckOptions{Kinds: []string{KindSpelling}})
	if len(onlySpelling.Fixes) != 1 || onlySpelling.Fixes[0].Original != "herkez" || len(onlySpelling.Flags) != 2 {
		t.Errorf("only spelling: %+v", onlySpelling)
	}
	// Ignored words are not flagged, and come back once unignored.
	e.Ignore("DUZENLEMNİYOR")
	if got := flagged(e.Inspect(text, CheckOptions{})); !slices.Equal(got, []string{"bka"}) {
		t.Errorf("ignored: %v", got)
	}
	e.Unignore("duzenlemniyor")
	if got := flagged(e.Inspect(text, CheckOptions{})); len(got) != 2 {
		t.Errorf("unignored: %v", got)
	}
	// AutoMarked works with Inspect as with Check.
	if r := e.Inspect("Bugün cok geldi.", CheckOptions{AutoMarked: [][2]int{{0, 5}}}); len(r.Fixes) == 0 || r.Fixes[0].Replacement != "çok" {
		t.Errorf("auto-marked: %+v", r.Fixes)
	}

	// With a context model, Inspect re-ranks the suggestions in the sentence.
	e.SetContextModel(fakeModel{score: scoreByCandidate(map[string]float64{"düzenleniyor": 1.1, "düzenlemiyor": -0.1}, nil)})
	r := e.Inspect("Bugün hatalarını duzenlemniyor.", CheckOptions{})
	if len(r.Flags) == 0 || r.Flags[0].Original != "duzenlemniyor" || r.Flags[0].Suggestions[0] != "düzenleniyor" {
		t.Errorf("re-ranked: %+v", r.Flags)
	}
}

// scoreByCandidate scores each candidate from a table (0 when missing), recording the candidates of each call.
func scoreByCandidate(table map[string]float64, calls *[][]string) func([]ContextQuery) [][]float64 {
	return func(queries []ContextQuery) [][]float64 {
		out := make([][]float64, len(queries))
		var seen []string
		for i, q := range queries {
			for _, cand := range q.Candidates {
				out[i] = append(out[i], table[cand])
				seen = append(seen, cand)
			}
		}
		if calls != nil {
			*calls = append(*calls, seen)
		}
		return out
	}
}

var contextTestUnigrams = strings.Join([]string{
	"bugün\t8000\t100", "çok\t10000\t100", "adam\t5000\t100", "oldu\t5000\t0", "öldü\t2000\t0", "ben\t50000\t200",
	"de\t500000\t1000", "bende\t10000\t50", "geliyorum\t3000\t0", "herkes\t3709\t50",
}, "\n")

func TestEngineContextModel(t *testing.T) {
	without := engineOfTSV(contextTestUnigrams, "bir şey\t5325\n", "herkez\therkes\t0\t3709\tMISSPELLINGS\n", ModeCareful)
	with := engineOfTSV(contextTestUnigrams, "bir şey\t5325\n", "herkez\therkes\t0\t3709\tMISSPELLINGS\n", ModeCareful)
	var calls [][]string
	with.SetContextModel(fakeModel{score: scoreByCandidate(map[string]float64{"oldu": 0, "öldü": 4.2, "bende": 0, "ben de": 4.5}, &calls)})
	if without.HasContext() || !with.HasContext() {
		t.Error("HasContext")
	}
	// The model is asked only about lowercase words, as in its training (contestedSpans).
	text := "Yarın bende geliyorum, adam oldu."
	if got := without.Fix(text, CheckOptions{}); got != text {
		t.Errorf("without a model: %q", got)
	}
	if got := summary(with.Check(text, CheckOptions{})); !slices.Equal(got, [][3]string{{"bende", "ben de", KindJoinedSeparate}, {"oldu", "öldü", KindTurkishLetters}}) {
		t.Errorf("with a model: %v", got)
	}
	if got := with.Fix(text, CheckOptions{}); got != "Yarın ben de geliyorum, adam öldü." {
		t.Errorf("with a model: %q", got)
	}
	if len(calls) == 0 || !slices.Contains(calls[0], "ben de") || !slices.Contains(calls[0], "öldü") {
		t.Errorf("calls: %q", calls)
	}
	// The kinds and the range filter the context's fixes too.
	if got := with.Check(text, CheckOptions{Kinds: []string{KindJoinedSeparate}}); len(got) != 1 || got[0].Replacement != "ben de" || !got[0].ByContext {
		t.Errorf("kinds: %+v", got)
	}
	at := runeIndex(text, "oldu")
	if got := with.Check(text, CheckOptions{Range: &[2]int{at, at + 4}}); len(got) != 1 || got[0].Replacement != "öldü" {
		t.Errorf("range: %+v", got)
	}
	// Ignored words and inline code are left alone by the context's fixes too.
	if got := with.Fix("Yarın `bende` geliyorum.", CheckOptions{}); got != "Yarın `bende` geliyorum." {
		t.Errorf("code: %q", got)
	}
	with.Ignore("oldu")
	if got := with.Fix(text, CheckOptions{}); got != "Yarın ben de geliyorum, adam oldu." {
		t.Errorf("ignored: %q", got)
	}

	// The context thresholds of careful (3.0) and balanced (2.5).
	e := engineOfTSV(contextTestUnigrams, "bir şey\t5325\n", "herkez\therkes\t0\t3709\tMISSPELLINGS\n", ModeCareful)
	e.SetContextModel(fakeModel{score: scoreByCandidate(map[string]float64{"oldu": 0, "öldü": 2.7}, nil)})
	if got := e.Fix("Adam oldu.", CheckOptions{}); got != "Adam oldu." {
		t.Errorf("careful: %q", got)
	}
	if got := e.Fix("Adam oldu.", CheckOptions{Mode: ModeBalanced}); got != "Adam öldü." {
		t.Errorf("balanced call: %q", got)
	}
	e.SetMode(ModeBalanced)
	if got := e.Fix("Adam oldu.", CheckOptions{}); got != "Adam öldü." {
		t.Errorf("balanced: %q", got)
	}
}
