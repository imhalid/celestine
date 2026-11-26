package spell

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The cost of the binary lexicon: reading lexicon.bin, the memory it holds, and its lookups. They need the
// engine's data in TINYFIX_DATA.

func lexiconBytes(tb testing.TB) []byte {
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		tb.Skip("TINYFIX_DATA is not set")
	}
	bin, err := os.ReadFile(filepath.Join(dir, "lexicon.bin"))
	if err != nil {
		tb.Fatal(err)
	}
	return bin
}

// benchWords are words to look up: common ones, rare ones, and ones the lexicon lacks.
var benchWords = []string{"bir", "ve", "evde", "kitaplarımızdan", "ankara'ya", "gelmeyecekmişsiniz", "herkez", "xyzzy", "istanbul", "şey"}

func BenchmarkLexiconFromBinary(b *testing.B) {
	bin := lexiconBytes(b)
	for b.Loop() {
		if _, err := LexiconFromBinary(bin); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLexiconCount(b *testing.B) {
	lex, err := LexiconFromBinary(lexiconBytes(b))
	if err != nil {
		b.Fatal(err)
	}
	i := 0
	for b.Loop() {
		lex.Count(benchWords[i%len(benchWords)])
		i++
	}
}

func BenchmarkLexiconPair(b *testing.B) {
	lex, err := LexiconFromBinary(lexiconBytes(b))
	if err != nil {
		b.Fatal(err)
	}
	i := 0
	for b.Loop() {
		lex.Pair(benchWords[i%len(benchWords)], benchWords[(i+1)%len(benchWords)])
		i++
	}
}

// TestLexiconMemory logs the heap a loaded lexicon holds beyond the bytes of the file.
func TestLexiconMemory(t *testing.T) {
	bin := lexiconBytes(t)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	lex, err := LexiconFromBinary(bin)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("lexicon.bin %d bytes; the lexicon holds %d bytes more on the heap; %d forms, %d pairs",
		len(bin), int64(after.HeapAlloc)-int64(before.HeapAlloc), lex.Size(), lex.PairCount())
	runtime.KeepAlive(lex)
}
