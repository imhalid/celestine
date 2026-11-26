package morph

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// parityLine is one word's answers from the TypeScript (testdata/parity.ts).
type parityLine struct {
	W       string                `json:"w"`
	IsWord  bool                  `json:"isWord"`
	Analyze []MorphAnalysis       `json:"analyze"`
	Prior   float64               `json:"prior"`
	F1      []FuzzyMorphCandidate `json:"f1"`
	F2      []FuzzyMorphCandidate `json:"f2"`
}

// readMorphBin is $TINYFIX_DATA/morph.bin; the test is skipped when TINYFIX_DATA is unset.
func readMorphBin(t testing.TB) []byte {
	t.Helper()
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		t.Skip("TINYFIX_DATA is not set: the tinyfix-tr data directory with morph.bin")
	}
	b, err := os.ReadFile(filepath.Join(dir, "morph.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// loadMorphology is the morphology of $TINYFIX_DATA/morph.bin; the test is skipped when TINYFIX_DATA is unset.
func loadMorphology(t testing.TB) *Morphology {
	t.Helper()
	m, err := FromBinary(readMorphBin(t))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestParity checks that every word of the golden gets the TypeScript's answers, called in the same order on a
// fresh morphology as testdata/parity.ts called them (the transitions' caches depend on that order, as in the
// TypeScript).
func TestParity(t *testing.T) {
	m := loadMorphology(t)
	f, err := os.Open("testdata/parity.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	words, failures := 0, 0
	for sc.Scan() {
		var want parityLine
		if err := json.Unmarshal(sc.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		if want.Analyze == nil {
			want.Analyze = []MorphAnalysis{}
		}
		for i := range want.Analyze {
			if want.Analyze[i].Morphemes == nil {
				want.Analyze[i].Morphemes = []string{}
			}
		}
		if want.F1 == nil {
			want.F1 = []FuzzyMorphCandidate{}
		}
		if want.F2 == nil {
			want.F2 = []FuzzyMorphCandidate{}
		}
		got := parityLine{W: want.W}
		got.IsWord = m.IsWord(want.W)
		got.Analyze = m.Analyze(want.W)
		got.Prior = m.PriorCount(want.W)
		got.F1 = m.FuzzySearch(want.W, 1, 15)
		got.F2 = m.FuzzySearch(want.W, 2, 40)
		words++
		if !reflect.DeepEqual(got, want) {
			failures++
			if failures <= 20 {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				t.Errorf("%q differs:\n got %s\nwant %s", want.W, g, w)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if words == 0 {
		t.Fatal("the golden has no words")
	}
	t.Logf("%d words compared, %d differ", words, failures)
	if failures > 0 {
		t.Errorf("%d of %d words differ", failures, words)
	}
}
