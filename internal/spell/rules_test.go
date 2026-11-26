package spell

import "testing"

// The tests of tinyfix-tr's rules alone: "Turkish rule helpers" of src/corrector.test.ts and "proper-name helpers"
// of src/sentence.test.ts.

func TestTurkishRuleHelpers(t *testing.T) {
	is := func(what string, got, want bool) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	str := func(what, got string, ok bool, want string) {
		t.Helper()
		if want == "" && ok {
			t.Errorf("%s = %q, want none", what, got)
		} else if want != "" && (!ok || got != want) {
			t.Errorf("%s = %q (%v), want %q", what, got, ok, want)
		}
	}

	// The particle suffix guard of KnownMisspellings.
	is(`isValidKnownParticleSuffix("kim mi", "ş")`, IsValidKnownParticleSuffix("kim mi", "ş"), false)
	is(`isValidKnownParticleSuffix("var mı", "ş")`, IsValidKnownParticleSuffix("var mı", "ş"), false)
	is(`isValidKnownParticleSuffix("var mı", "sın")`, IsValidKnownParticleSuffix("var mı", "sın"), true)
	is(`isValidKnownParticleSuffix("var mı", "ydı")`, IsValidKnownParticleSuffix("var mı", "ydı"), true)
	is(`isValidKnownParticleSuffix("var mı", "sin")`, IsValidKnownParticleSuffix("var mı", "sin"), false)
	is(`isValidKnownParticleSuffix("ya da", "cı")`, IsValidKnownParticleSuffix("ya da", "cı"), false)
	is(`isValidKnownParticleSuffix("değil ki", "m")`, IsValidKnownParticleSuffix("değil ki", "m"), false)
	is(`isValidKnownParticleSuffix("bir şey", "ler")`, IsValidKnownParticleSuffix("bir şey", "ler"), true)

	// The generic apostrophe's stem and sound guards.
	for _, c := range []struct {
		stem             string
		skipBareCa, want bool
	}{
		{"müslümanlar", false, true}, {"japonların", false, true}, {"yunanlardan", false, true},
		{"türklerin", false, true}, {"türkçeden", false, true}, {"hırvatça", false, true},
		{"karaca", false, true}, {"karaca", true, false}, {"bakanlığı", false, true},
		{"zeynep", false, false}, {"clara", false, false}, {"valerin", false, false},
	} {
		is("hasBlockedApostropheStem("+c.stem+")", HasBlockedApostropheStem(c.stem, c.skipBareCa, false), c.want)
	}
	is(`isValidGenericApostrophe("zeynep", "ten")`, IsValidGenericApostrophe("zeynep", "ten", false), true)
	is(`isValidGenericApostrophe("zeynep", "den")`, IsValidGenericApostrophe("zeynep", "den", false), false)
	is(`isValidGenericApostrophe("ankara", "in")`, IsValidGenericApostrophe("ankara", "in", false), false)
	is(`isValidGenericApostrophe("ankara", "nın")`, IsValidGenericApostrophe("ankara", "nın", false), true)
	is(`isValidGenericApostrophe("google", "ın")`, IsValidGenericApostrophe("google", "ın", false), true)
	is(`isValidGenericApostrophe("mark", "ya")`, IsValidGenericApostrophe("mark", "ya", false), false)

	// Acronyms and mixed-case brand names glued to a suffix.
	for _, c := range []struct {
		typed string
		want  AcronymApostrophe
	}{
		{"HEXIMden", AcronymApostrophe{"HEXIM", "den", "hexım'den"}},
		{"KDAların", AcronymApostrophe{"KDA", "ların", "kda'ların"}},
		{"TLSnin", AcronymApostrophe{"TLS", "nin", "tls'nin"}},
		{"MacOSdaki", AcronymApostrophe{"MacOS", "daki", "macos'daki"}},
		{"OpenAIden", AcronymApostrophe{"OpenAI", "den", "openaı'den"}},
	} {
		if got, ok := MatchAcronymApostrophe(c.typed); !ok || got != c.want {
			t.Errorf("matchAcronymApostrophe(%q) = %+v (%v), want %+v", c.typed, got, ok, c.want)
		}
	}
	for _, typed := range []string{"AIde", "EVde", "URLs"} {
		if got, ok := MatchAcronymApostrophe(typed); ok {
			t.Errorf("matchAcronymApostrophe(%q) = %+v, want none", typed, got)
		}
	}

	// The vowel harmony of apostrophe suffixes, and its restoration (keeping -ki).
	is(`isHarmoniousApostropheMarkChange("mortimer", "i", "ı")`, IsHarmoniousApostropheMarkChange("mortimer", "i", "ı", false), false)
	is(`isHarmoniousApostropheMarkChange("ravenna", "nin", "nın")`, IsHarmoniousApostropheMarkChange("ravenna", "nin", "nın", false), true)
	is(`isHarmoniousApostropheMarkChange("mary", "i", "ı")`, IsHarmoniousApostropheMarkChange("mary", "i", "ı", false), false)
	is(`isHarmoniousApostropheMarkChange("altay", "in", "ın")`, IsHarmoniousApostropheMarkChange("altay", "in", "ın", false), true)
	is(`isHarmoniousApostropheMarkChange("begüm", "un", "ün")`, IsHarmoniousApostropheMarkChange("begüm", "un", "ün", false), true)
	is(`isHarmoniousApostropheMarkChange("alkol", "un", "ün", true)`, IsHarmoniousApostropheMarkChange("alkol", "un", "ün", true), true)
	is(`isHarmoniousApostropheMarkChange("istanbul", "u", "ü", false)`, IsHarmoniousApostropheMarkChange("istanbul", "u", "ü", false), false)
	for _, c := range [][2]string{
		{"ukrayna'sinda", "ukrayna'sında"}, {"ankara'sindaki", "ankara'sındaki"}, {"kda'larin", "kda'ların"},
		{"ankara'daki", ""}, {"ukrayna'sında", ""}, {"mortimer'sinda", ""},
	} {
		got, ok := FixDisharmoniousApostropheSuffix(c[0])
		str("fixDisharmoniousApostropheSuffix("+c[0]+")", got, ok, c[1])
	}

	// The doubled locative around the plural.
	got, ok := CollapsedDoubledLocative("bölgedeler", "de")
	str(`collapsedDoubledLocative("bölgedeler", "de")`, got, ok, "bölgelerde")
	got, ok = CollapsedDoubledLocative("alandalar", "da")
	str(`collapsedDoubledLocative("alandalar", "da")`, got, ok, "alanlarda")
	got, ok = CollapsedDoubledLocative("bölgelerde", "de")
	str(`collapsedDoubledLocative("bölgelerde", "de")`, got, ok, "")

	// The harmony of a standalone question particle.
	for _, c := range []struct {
		word, prev string
		inverse    bool
		want       string
	}{
		{"mi", "var", false, "mı"}, {"misin", "hazır", false, "mısın"}, {"mu", "gördü", false, "mü"},
		{"mi", "saat", true, ""}, {"mu", "alkol", true, "mü"}, {"mü", "oldu", false, ""},
	} {
		got, ok := HarmonizeQuestionParticle(c.word, c.prev, c.inverse)
		str("harmonizeQuestionParticle("+c.word+", "+c.prev+")", got, ok, c.want)
	}

	// The TDK place, geographic and state head words, against institutions.
	is(`isPlaceApostropheHead("denizi")`, IsPlaceApostropheHead("denizi", ""), true)
	is(`isPlaceApostropheHead("cumhuriyeti")`, IsPlaceApostropheHead("cumhuriyeti", ""), true)
	is(`isPlaceApostropheHead("mahallesi")`, IsPlaceApostropheHead("mahallesi", ""), true)
	is(`isPlaceApostropheHead("birliği", "avrupa")`, IsPlaceApostropheHead("birliği", "avrupa"), true)
	is(`isPlaceApostropheHead("birliği", "psikiyatri")`, IsPlaceApostropheHead("birliği", "psikiyatri"), false)
	is(`isPlaceApostropheHead("üniversitesi")`, IsPlaceApostropheHead("üniversitesi", ""), false)
	is(`isPlaceApostropheHead("partisi")`, IsPlaceApostropheHead("partisi", ""), false)
	is(`isPlaceApostropheHead("kurumu")`, IsPlaceApostropheHead("kurumu", ""), false)
	is(`isValidPlaceApostrophe("denizi", "nin")`, IsValidPlaceApostrophe("denizi", "nin"), true)
	is(`isValidPlaceApostrophe("mahallesi", "nde")`, IsValidPlaceApostrophe("mahallesi", "nde"), true)
	is(`isValidPlaceApostrophe("cumhuriyeti", "nden")`, IsValidPlaceApostrophe("cumhuriyeti", "nden"), true)
	is(`isValidPlaceApostrophe("okyanus", "ta")`, IsValidPlaceApostrophe("okyanus", "ta"), true)
	is(`isValidPlaceApostrophe("okyanus", "da")`, IsValidPlaceApostrophe("okyanus", "da"), false)
	is(`isValidPlaceApostrophe("denizi", "nın")`, IsValidPlaceApostrophe("denizi", "nın"), false)
	is(`hasBlockedApostropheStem("denizi", true, false)`, HasBlockedApostropheStem("denizi", true, false), true)
	is(`hasBlockedApostropheStem("denizi", true, true)`, HasBlockedApostropheStem("denizi", true, true), false)
}

func TestProperNameHelpers(t *testing.T) {
	// A number's suffix: the TDK apostrophe and the voicing of its consonant.
	for _, c := range []struct {
		num, suffix string
		hadApos     bool
		want        string
	}{
		{"1985", "te", false, "te"}, {"1985", "de", false, "te"}, {"1985", "de", true, "te"},
		{"1985", "te", true, ""}, // already right
		{"2023", "de", false, "te"}, {"80", "lere", false, "lere"}, {"2", "nci", false, "nci"},
		{"30", "da", false, "da"},
		// Units and short codes.
		{"5", "cm", false, ""}, {"3", "a", false, ""}, {"20", "px", false, ""},
	} {
		got, ok := FixNumberSuffix(c.num, c.suffix, c.hadApos)
		if c.want == "" && ok || c.want != "" && (!ok || got != c.want) {
			t.Errorf("fixNumberSuffix(%q, %q, %v) = %q (%v), want %q", c.num, c.suffix, c.hadApos, got, ok, c.want)
		}
	}

	// The softening when a wrong apostrophe is taken away.
	morphWords := setOf("kitabı", "ağaca", "tutarlılığı")
	isMorph := func(w string) bool { return morphWords[w] }
	for _, c := range [][3]string{
		{"Hristiyanlık", "ın", "Hristiyanlığın"}, {"Atatürkçülük", "ün", "Atatürkçülüğün"},
		{"Başbakanlık", "a", "Başbakanlığa"}, {"kitap", "ı", "kitabı"}, {"ağaç", "a", "ağaca"},
		{"Türkler", "in", "Türklerin"},
	} {
		if got := JoinWithoutApostrophe(c[0], c[1], isMorph); got != c[2] {
			t.Errorf("joinWithoutApostrophe(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
