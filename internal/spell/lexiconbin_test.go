package spell

import (
	"math"
	"strings"
	"testing"
)

// The tests of tinyfix-tr's src/lexicon-binary.test.ts.

// near reports whether got is within a relative tolerance of want.
func near(got, want, tolerance float64) bool { return math.Abs(got-want)/want < tolerance }

func TestBinaryLexiconRoundTrip(t *testing.T) {
	bytes, err := EncodeBinaryLexicon(EncodeBinaryLexiconOptions{
		Unigrams: []BinaryUnigramInput{
			{"ankara", 50000, 49000},
			{"ankara'ya", 4000, 3900},
			{"bir", 2_500_000, 10000},
			{"çok", 10000, 100},
			{"de", 500000, 1000},
			{"ev", 10000, 100},
			{"evde", 8000, 50},
			{"hiç", 40000, 100},
			{"hiçbir", 20772, 100},
			{"şey", 50000, 100},
			{"şöför", 17, 0},
			{"zeytin", 4, 2},
			{"a", 79, 0},
		},
		Bigrams: []BinaryBigramInput{
			{"bir", "şey", 5325, false},
			{"hiç", "bir", 45, false},
			{"ev", "de", 10, true},
			{"çok", "de", 3, true},
		},
		BloomTiers: []BinaryBloomTierInput{
			{3, 16, []string{"nadirüç", "üçüncüform"}},
			{2, 14, []string{"nadiriki", "ikinciform"}},
		},
		BlockSize:     4,
		ExactCountMax: 80,
		CountBuckets:  256,
		CountMaxVal:   3_000_000,
		CapMax:        50,
	})
	if err != nil {
		t.Fatal(err)
	}
	lex, err := LexiconFromBinary(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if lex.Size() != 17 { // 13 exact + 4 in Bloom tiers
		t.Errorf("size %d, want 17", lex.Size())
	}
	if lex.PairCount() != 4 {
		t.Errorf("pair count %d, want 4", lex.PairCount())
	}
	equal := func(what string, got, want float64) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	about := func(what string, got, want, tolerance float64) {
		t.Helper()
		if !near(got, want, tolerance) {
			t.Errorf("%s = %v, want about %v", what, got, want)
		}
	}

	// Small counts (below 80) are exact.
	equal("count(zeytin)", lex.Count("zeytin"), 4)
	equal("count(şöför)", lex.Count("şöför"), 17)
	equal("count(a)", lex.Count("a"), 79)

	// Large counts are quantized within 5%.
	about("count(ankara)", lex.Count("ankara"), 50000, 0.05)
	about("count(bir)", lex.Count("bir"), 2_500_000, 0.05)
	about("count(ankara'ya)", lex.Count("ankara'ya"), 4000, 0.05)

	// The capital share is in steps of 0.02.
	if math.Abs(lex.CapitalShare("ankara")-0.98) > 0.02 {
		t.Errorf("capitalShare(ankara) = %v", lex.CapitalShare("ankara"))
	}
	if math.Abs(lex.CapitalShare("zeytin")-0.5) > 0.02 {
		t.Errorf("capitalShare(zeytin) = %v", lex.CapitalShare("zeytin"))
	}
	equal("capitalShare(şöför)", lex.CapitalShare("şöför"), 0)
	equal("capitalShare(yokboylekelime)", lex.CapitalShare("yokboylekelime"), 0)

	// Pairs, in both the byFirst and the bySecond partitions.
	about("pair(bir, şey)", lex.Pair("bir", "şey"), 5325, 0.05)
	equal("pair(hiç, bir)", lex.Pair("hiç", "bir"), 45)
	equal("pair(ev, de)", lex.Pair("ev", "de"), 10)
	equal("pair(çok, de)", lex.Pair("çok", "de"), 3)
	equal("pair(ev, şey)", lex.Pair("ev", "şey"), 0)
	equal("pair(bilinmeyen, de)", lex.Pair("bilinmeyen", "de"), 0)

	// The Bloom tiers.
	equal("count(nadirüç)", lex.Count("nadirüç"), 3)
	equal("count(üçüncüform)", lex.Count("üçüncüform"), 3)
	equal("count(nadiriki)", lex.Count("nadiriki"), 2)
	equal("count(ikinciform)", lex.Count("ikinciform"), 2)
	equal("count(kesinlikleyoktur123)", lex.Count("kesinlikleyoktur123"), 0)
	equal("count()", lex.Count(""), 0)

	// ExactCount skips the Bloom tiers (it looks up the targets of typo fixes).
	equal("exactCount(zeytin)", lex.ExactCount("zeytin"), 4)
	about("exactCount(ankara'ya)", lex.ExactCount("ankara'ya"), 4000, 0.05)
	equal("exactCount(nadirüç)", lex.ExactCount("nadirüç"), 0)
	equal("exactCount(nadiriki)", lex.ExactCount("nadiriki"), 0)

	// Apostrophe forms are not probed in the Bloom tiers, only in the exact table.
	equal("count(nadir'üç)", lex.Count("nadir'üç"), 0)
	about("count(ankara'ya)", lex.Count("ankara'ya"), 4000, 0.05)

	// Letters outside the alphabet give 0.
	equal("count(hello123)", lex.Count("hello123"), 0)
	equal("count(test@example)", lex.Count("test@example"), 0)

	// ToTSV gives the exact entries back.
	unigrams, bigrams := lex.ToTSV()
	if !strings.Contains(unigrams, "ankara'ya\t") {
		t.Errorf("unigrams lack ankara'ya:\n%s", unigrams)
	}
	if !strings.Contains(bigrams, "ev de\t10") || !strings.Contains(bigrams, "hiç bir\t45") {
		t.Errorf("bigrams lack ev de or hiç bir:\n%s", bigrams)
	}
}

func TestBinaryLexiconEmptyAndCorrupt(t *testing.T) {
	bytes, err := EncodeBinaryLexicon(EncodeBinaryLexiconOptions{})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := LexiconFromBinary(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Size() != 0 || empty.PairCount() != 0 || empty.Count("ev") != 0 || empty.Pair("ev", "de") != 0 {
		t.Errorf("empty lexicon: size %d, pairs %d, count %v, pair %v", empty.Size(), empty.PairCount(), empty.Count("ev"), empty.Pair("ev", "de"))
	}
	if _, err := LexiconFromBinary([]byte{0, 1, 2, 3, 4}); err == nil || !strings.Contains(err.Error(), "ALX1") {
		t.Errorf("corrupt header: err %v, want one naming ALX1", err)
	}
	// A lexicon cut short is an error, not a panic.
	full, _ := EncodeBinaryLexicon(EncodeBinaryLexiconOptions{Unigrams: []BinaryUnigramInput{{"ev", 10, 0}, {"evde", 5, 0}}})
	for n := 57; n < len(full); n++ {
		if _, err := LexiconFromBinary(full[:n]); err == nil {
			t.Errorf("a lexicon cut to %d of %d bytes was read", n, len(full))
		}
	}
}

func TestJSArithmetic(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{{2.5, 3}, {-2.5, -2}, {0.49999999999999994, 0}, {-0.5, -0}, {1.4, 1}} {
		if got := jsRound(c.in); got != c.want {
			t.Errorf("jsRound(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	for _, c := range []struct {
		in   string
		want float64
	}{{"", 0}, {" 12 ", 12}, {"0x10", 16}, {"1e3", 1000}, {".5", 0.5}, {"5.", 5}, {"-Infinity", math.Inf(-1)}} {
		if got := jsNumber(c.in); got != c.want {
			t.Errorf("jsNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	for _, in := range []string{"abc", "1_000", "inf", "NaN", "0x", "-0x10", "1e"} {
		if got := jsNumber(in); !math.IsNaN(got) {
			t.Errorf("jsNumber(%q) = %v, want NaN", in, got)
		}
	}
	for _, c := range []struct {
		in   float64
		want string
	}{{1, "1"}, {1.5, "1.5"}, {1e21, "1e+21"}, {1e-7, "1e-7"}, {1.5e-7, "1.5e-7"}, {123456789012, "123456789012"}, {math.NaN(), "NaN"}} {
		if got := jsNumberString(c.in); got != c.want {
			t.Errorf("jsNumberString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
