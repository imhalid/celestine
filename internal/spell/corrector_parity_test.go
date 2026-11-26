package spell

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/imhalid/celestine/internal/spell/morph"
)

// loadTestData reads the engine's data from the directory TINYFIX_DATA names, and skips the test without it.
func loadTestData(t testing.TB) (*Lexicon, *KnownMisspellings, *morph.Morphology) {
	t.Helper()
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		t.Skip("TINYFIX_DATA is not set")
	}
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	lexicon, err := LexiconFromBinary(read("lexicon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	morphology, err := morph.FromBinary(read("morph.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return lexicon, KnownMisspellingsFromTSV(string(read("known.tsv"))), morphology
}

func fixesJSONOf(fixes []Fix) []any {
	out := []any{}
	for _, f := range fixes {
		out = append(out, map[string]any{"from": f.From, "to": f.To, "original": f.Original, "replacement": f.Replacement, "kind": f.Kind, "scope": f.Scope, "confidence": f.Confidence})
	}
	return out
}

func flagsJSONOf(flags []Flag) []any {
	out := []any{}
	for _, f := range flags {
		out = append(out, map[string]any{"from": f.From, "to": f.To, "original": f.Original, "suggestions": f.Suggestions, "reason": f.Reason})
	}
	return out
}

// TestParityCorrector checks the corrector, the sentence rules and the engine against the TypeScript engine's
// answers in testdata/parity-corrector.jsonl.gz, which testdata/parity-corrector.ts writes: every input of
// tinyfix-tr's bench sets, in the same order in one engine, as the morphology's caches make answers depend on the
// order of calls; every 7th line is checked in the balanced mode with every kind, and with a word taken as
// auto-marked, too. It needs the engine's data in the directory TINYFIX_DATA names, and is skipped without it.
func TestParityCorrector(t *testing.T) {
	lexicon, known, morphology := loadTestData(t)
	corrector := NewCorrector(lexicon, CAREFUL, known, morphology)
	engine := NewEngine(lexicon, known, morphology, ModeCareful)

	f, err := os.Open("testdata/parity-corrector.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	lines, answers, failed := 0, 0, 0
	for scanner.Scan() {
		lines++
		var line struct {
			In string          `json:"in"`
			C  map[string]any  `json:"c"`
			I  map[string]any  `json:"i"`
			K  []any           `json:"k"`
			R  *[2]int         `json:"r"`
			KR []any           `json:"kr"`
			KB []any           `json:"kb"`
			A  json.RawMessage `json:"a"`
			KA []any           `json:"ka"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("line %d: %v", lines, err)
		}
		c := corrector.Inspect(line.In, CorrectOptions{})
		corrections := []any{}
		for _, x := range c.Corrections {
			corrections = append(corrections, map[string]any{"start": x.Start, "end": x.End, "original": x.Original, "replacement": x.Replacement, "kind": string(x.Kind), "margin": x.Margin})
		}
		cflags := []any{}
		for _, x := range c.Flags {
			cflags = append(cflags, map[string]any{"from": x.Start, "to": x.End, "original": x.Original, "suggestions": x.Suggestions, "reason": string(x.Reason)})
		}
		inspected := engine.Inspect(line.In, CheckOptions{})
		checked := engine.Check(line.In, CheckOptions{})
		var ranged any
		if line.R != nil {
			ranged = fixesJSONOf(engine.Check(line.In, CheckOptions{Range: line.R}))
		}
		compare := func(what string, got, want any) {
			answers++
			if g := normalize(got); !reflect.DeepEqual(g, normalize(want)) {
				failed++
				if failed <= 30 {
					gj, _ := json.Marshal(g)
					wj, _ := json.Marshal(want)
					t.Errorf("line %d %s %q:\n  go: %s\n  ts: %s", lines, what, line.In, clip(gj), clip(wj))
				}
			}
		}
		compare("corrector", map[string]any{"out": c.Text, "corrections": corrections, "flags": cflags}, line.C)
		compare("inspect", map[string]any{"fixes": fixesJSONOf(inspected.Fixes), "flags": flagsJSONOf(inspected.Flags)}, line.I)
		compare("check", fixesJSONOf(checked), line.K)
		compare("check range", ranged, line.KR)
		if line.A != nil { // every 7th line
			compare("check balanced", fixesJSONOf(engine.Check(line.In, CheckOptions{Mode: ModeBalanced, Kinds: AllFixKinds})), line.KB)
			var marked *[2]int
			if err := json.Unmarshal(line.A, &marked); err != nil {
				t.Fatal(err)
			}
			if marked != nil {
				compare("check auto-marked", fixesJSONOf(engine.Check(line.In, CheckOptions{AutoMarked: [][2]int{*marked}})), line.KA)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines == 0 {
		t.Fatal("the golden file is empty")
	}
	t.Logf("%d lines, %d answers checked, %d differ", lines, answers, failed)
	if failed > 0 {
		t.Errorf("%d of %d answers differ from the TypeScript's", failed, answers)
	}
}
