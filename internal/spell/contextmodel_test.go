package spell

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// goldenLines calls fn with each line of a gzipped JSON-lines golden in testdata, decoded into a new T.
func goldenLines[T any](t *testing.T, name string, fn func(line int, v T)) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		var v T
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			t.Fatalf("%s line %d: %v", name, line, err)
		}
		fn(line, v)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}

// The subword tokenizer is the TypeScript's, token for token, on the bench sets' texts and the cases a tokenizer
// gets wrong: apostrophes, capitals, numbers, other languages' letters, a character outside the BMP (#623).
func TestParitySubwords(t *testing.T) {
	failed, lines := 0, 0
	goldenLines(t, "parity-subwords.jsonl.gz", func(line int, v struct {
		T string  `json:"t"`
		S []int64 `json:"s"`
	}) {
		lines++
		if got := encodeSubwords(v.T); !slices.Equal(got, v.S) {
			failed++
			if failed <= 10 {
				t.Errorf("%q:\n  go: %v\n  ts: %v", v.T, got, v.S)
			}
		}
	})
	if failed > 0 || lines < 1000 {
		t.Fatalf("%d of %d texts differ", failed, lines)
	}
}

// The packing is the TypeScript's: [CLS] in the left's segment first, the candidate's and the right's segments
// after, every row as long as the longest (#623, its corrector.test.ts).
func TestPackSubwords(t *testing.T) {
	ids := encodeSubwords("Ankara'ya giderken GOES-15 verilerini inceledik.")
	if len(ids) <= 5 || ids[0] != subCapID || !slices.Contains(ids, subAllCapID) || !slices.Contains(ids, subNumID) {
		t.Fatalf("encoded %v", ids)
	}
	for _, pair := range [][2]string{{"içinde", "için de"}, {"yıllar", "yollar"}, {"oldu", "öldü"}} {
		if slices.Equal(encodeSubwords(pair[0]), encodeSubwords(pair[1])) {
			t.Errorf("%q and %q encoded alike", pair[0], pair[1])
		}
	}
	p := packSubwords([]ContextQuery{{Left: "Ondan hiç ", Right: " alamadım.", Candidates: []string{"hber", "her", "haber"}}})
	if p.n != 3 || p.subwords[0] != subClsID || p.segments[0] != 1 || !slices.Contains(p.segments, 2) || !slices.Contains(p.segments, 3) {
		t.Fatalf("packed %+v", p)
	}
	if len(p.subwords) != p.n*p.seqLen || len(p.features) != p.n*FeatureDim {
		t.Fatalf("sizes %d %d for %d rows of %d", len(p.subwords), len(p.features), p.n, p.seqLen)
	}
}

// loadContextModel is the context model of TINYFIX_DATA, or the test skipped.
func loadContextModel(t *testing.T) *onnxContext {
	t.Helper()
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		t.Skip("TINYFIX_DATA is not set")
	}
	b, err := os.ReadFile(filepath.Join(dir, "context.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadContextModel(b)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*onnxContext)
}

// contextScoreTolerance is how far a score may be from onnxruntime's. The network quantizes its activations to 8 bits
// with a scale of each tensor's own range, so a float32 difference in a last bit (erf, exp, a layer norm's sums,
// which onnxruntime computes with kernels of its own) can move a value to the next of 255 steps, a few hundredths of
// a score after it: onnxruntime itself gives scores up to 0.018 apart with its graph optimized and not.
const contextScoreTolerance = 0.1

// The network run in Go gives onnxruntime's scores on the batches the TypeScript engine made, within the steps of its
// quantization (#623).
func TestContextModelOutputs(t *testing.T) {
	m := loadContextModel(t)
	var worst, sum float64
	batches, rows := 0, 0
	goldenLines(t, "context-batches.jsonl.gz", func(line int, v struct {
		N        int       `json:"n"`
		SeqLen   int       `json:"seqLen"`
		Subwords []int64   `json:"subwords"`
		Segments []int64   `json:"segments"`
		Features []float32 `json:"features"`
		Scores   []float64 `json:"scores"`
	}) {
		batches++
		got, err := m.run(packedSubwords{subwords: v.Subwords, segments: v.Segments, features: v.Features, seqLen: v.SeqLen, n: v.N})
		if err != nil {
			t.Fatalf("batch %d: %v", line, err)
		}
		for i, s := range v.Scores {
			rows++
			d := math.Abs(float64(got[i]) - s)
			worst, sum = max(worst, d), sum+d
			if d > contextScoreTolerance {
				t.Errorf("batch %d row %d: go %v, onnxruntime %v", line, i, got[i], s)
			}
		}
	})
	t.Logf("%d batches, %d rows: the largest difference from onnxruntime %.3g, the mean %.3g", batches, rows, worst, sum/float64(max(rows, 1)))
	if mean := sum / float64(max(rows, 1)); mean > contextScoreTolerance/10 {
		t.Fatalf("the mean difference %.3g: more than the quantization's steps", mean)
	}
}

// contextVerdict is what a check of the engine with its context says of a text, as the golden has it: its fixes and
// its flags.
type contextVerdict struct {
	Fixes []struct {
		From, To                           int
		Original, Replacement, Kind, Scope string
		Confidence                         float64
		ByContext                          bool
	} `json:"fixes"`
	Flags []struct {
		From, To         int
		Original, Reason string
		Suggestions      []string
	} `json:"flags"`
}

// verdictOf is a check's fixes and flags as a contextVerdict.
func verdictOf(fixes []Fix, flags []Flag) contextVerdict {
	var v contextVerdict
	b, _ := json.Marshal(map[string]any{"fixes": fixesWithContext(fixes), "flags": flagsJSONOf(flags)})
	json.Unmarshal(b, &v)
	return v
}

// fixesWithContext are fixes as the context parity's golden has them, whether the context model decided each too.
func fixesWithContext(fixes []Fix) []any {
	out := []any{}
	for _, f := range fixes {
		out = append(out, map[string]any{"from": f.From, "to": f.To, "original": f.Original, "replacement": f.Replacement, "kind": f.Kind, "scope": f.Scope, "confidence": f.Confidence, "byContext": f.ByContext})
	}
	return out
}

// sameVerdict reports whether two checks say the same to the writer: the same words fixed, to the same, of the same
// kind and scope, by the same pass, with a confidence within 0.01; the same words flagged, for the same reason, with
// the same suggestions, in any order. The confidence and the order of nearly tied suggestions come from the
// network's scores, which are onnxruntime's within the steps of its quantization (TestContextModelOutputs).
func sameVerdict(a, b contextVerdict) bool {
	if len(a.Fixes) != len(b.Fixes) || len(a.Flags) != len(b.Flags) {
		return false
	}
	for i, f := range a.Fixes {
		g := b.Fixes[i]
		if f.From != g.From || f.To != g.To || f.Original != g.Original || f.Replacement != g.Replacement ||
			f.Kind != g.Kind || f.Scope != g.Scope || f.ByContext != g.ByContext || math.Abs(f.Confidence-g.Confidence) > 0.01 {
			return false
		}
	}
	for i, f := range a.Flags {
		g := b.Flags[i]
		if f.From != g.From || f.To != g.To || f.Original != g.Original || f.Reason != g.Reason {
			return false
		}
		x, y := slices.Clone(f.Suggestions), slices.Clone(g.Suggestions)
		slices.Sort(x)
		slices.Sort(y)
		if !slices.Equal(x, y) {
			return false
		}
	}
	return true
}

// contextFlips is how many of the inputs checked may say something else than the TypeScript engine with its context
// on: a decision the network's score puts on a threshold, which the steps of its quantization turn. TestParityContext
// says each.
const contextFlips = 0

// contextEvery is which of the golden's inputs TestParityContext checks: every contextEvery-th, the network's runs
// taking some hundreds of milliseconds a paragraph in Go. 1 checks all 31,063.
const contextEvery = 4

// The engine with its context model on says to the writer what the TypeScript engine with context: true does
// (sameVerdict), on the bench sets' inputs, in one engine and in order (#623).
func TestParityContext(t *testing.T) {
	lexicon, known, morphology := loadTestData(t)
	engine := NewEngine(lexicon, known, morphology, ModeCareful)
	engine.SetContextModel(loadContextModel(t))
	lines, exact, close, differ := 0, 0, 0, 0
	goldenLines(t, "parity-context.jsonl.gz", func(line int, v struct {
		In string         `json:"in"`
		I  contextVerdict `json:"i"` // the golden's "k" is check, whose fixes are inspect's
	}) {
		if (line-1)%contextEvery != 0 {
			return
		}
		lines++
		i := engine.Inspect(v.In, CheckOptions{})
		got := verdictOf(i.Fixes, i.Flags)
		switch {
		case reflect.DeepEqual(got, v.I):
			exact++
		case sameVerdict(got, v.I):
			close++
		default:
			differ++
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(v.I)
			t.Logf("line %d %q:\n  go: %s\n  ts: %s", line, v.In, clip(gj), clip(wj))
		}
	})
	t.Logf("%d inputs: %d the same, %d the same but a confidence or a suggestions' order, %d differ", lines, exact, close, differ)
	if differ > contextFlips {
		t.Fatalf("%d inputs say something else than the TypeScript engine with its context, %d allowed", differ, contextFlips)
	}
}
