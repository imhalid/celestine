package morph

// The benchmarks the editor's use is judged by: the load, and IsWord and FuzzySearch per word as the user types.
// They read morph.bin from TINYFIX_DATA and the words of the parity golden:
//
//	TINYFIX_DATA=… go test ./internal/spell/morph/ -run Footprint -bench . -v

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// goldenWords is the words of the parity golden.
func goldenWords(tb testing.TB) []string {
	tb.Helper()
	f, err := os.Open("testdata/parity.jsonl.gz")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var words []string
	for sc.Scan() {
		var line struct{ W string }
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			tb.Fatal(err)
		}
		if line.W != "" {
			words = append(words, line.W)
		}
	}
	return words
}

// heapInUse is the live heap after a collection.
func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// TestFootprint logs the heap a loaded morphology holds, before and after the fuzzy search's trie is built.
func TestFootprint(t *testing.T) {
	b := readMorphBin(t)
	before := heapInUse()
	m, err := FromBinary(b)
	if err != nil {
		t.Fatal(err)
	}
	loaded := heapInUse()
	m.FuzzySearch("kitaplar", 1, 15)
	withTrie := heapInUse()
	t.Logf("morph.bin %d bytes; loaded: %.1f MB; with the fuzzy trie: %.1f MB", len(b),
		float64(loaded-before)/(1<<20), float64(withTrie-before)/(1<<20))
	runtime.KeepAlive(m)
}

func BenchmarkFromBinary(b *testing.B) {
	data := readMorphBin(b)
	for b.Loop() {
		if _, err := FromBinary(data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIsWordUncached is IsWord without its result cache: the search itself, per word.
func BenchmarkIsWordUncached(b *testing.B) {
	m := loadMorphology(b)
	words := goldenWords(b)
	for i := range words {
		words[i] = tr.Lower(words[i])
	}
	b.ResetTimer()
	i := 0
	for b.Loop() {
		m.analyzer.checkWordUncached(words[i%len(words)])
		i++
	}
}

func BenchmarkIsWordCached(b *testing.B) {
	m := loadMorphology(b)
	words := goldenWords(b)
	for _, w := range words {
		m.IsWord(w)
	}
	b.ResetTimer()
	i := 0
	for b.Loop() {
		m.IsWord(words[i%len(words)])
		i++
	}
}

// benchmarkFuzzy is FuzzySearch per word without its result cache.
func benchmarkFuzzy(b *testing.B, maxEdits, maxResults int) {
	m := loadMorphology(b)
	words := goldenWords(b)
	m.FuzzySearch("kitaplar", 1, 15)
	b.ResetTimer()
	i := 0
	for b.Loop() {
		clear(m.fuzzy.cache)
		m.FuzzySearch(words[i%len(words)], maxEdits, maxResults)
		i++
	}
}

func BenchmarkFuzzySearch1(b *testing.B) { benchmarkFuzzy(b, 1, 15) }
func BenchmarkFuzzySearch2(b *testing.B) { benchmarkFuzzy(b, 2, 40) }
