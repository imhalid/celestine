package spell

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// benchParagraph is a paragraph of 100 words with a few mistakes of each kind.
const benchParagraph = "Dün akşam arkadaşlarımla birlikte şehrin eski mahallelerinden birinde uzun bir yürüyüş yaptık. " +
	"Hava cok güzeldi ve sokaklarda herkez dışarıdaydı. Yürürken bir kitapçıya girdik, raflarda yıllardır " +
	"kimsenin dokunmadığı kitaplar vardı. Kitapçının sahibi bize Ankaraya yaptığı yolculukları anlattı, birşey " +
	"sormamıza gerek kalmadan hikayelerini paylaştı. Sonra yakındaki bir kafede oturduk ve hiç bir acelemiz " +
	"olmadan çay içtik. Arkadaşım yeni işinden bahsetti, çok yoruluyormuş ama yaptığı işi seviyormuş. Ben de " +
	"ona geçen hafta okuduğum romanı anlattım, o da okumak istediğini söyledi. Gece geç saatlerde evlerimize " +
	"döndük, yorgunduk ama mutluyduk. Bu tür akşamların daha sık olmasını istiyoruz, gelecek hafta " +
	"buluşuruz ve başka bir semti keşfederiz."

// BenchmarkLoad loads the engine's data (TINYFIX_DATA) and reports the heap it keeps.
func BenchmarkLoad(b *testing.B) {
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		b.Skip("TINYFIX_DATA is not set")
	}
	var before, after runtime.MemStats
	for b.Loop() {
		runtime.GC()
		runtime.ReadMemStats(&before)
		e, err := Load(dir, ModeCareful)
		if err != nil {
			b.Fatal(err)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(e)
	}
	b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc)/(1<<20), "heap-MB")
}

func benchEngine(b *testing.B) *Engine {
	dir := os.Getenv("TINYFIX_DATA")
	if dir == "" {
		b.Skip("TINYFIX_DATA is not set")
	}
	e, err := Load(dir, ModeCareful)
	if err != nil {
		b.Fatal(err)
	}
	return e
}

// benchCheck reports the first, cold call (the morphology's caches empty) and then the warm ones.
func benchCheck(b *testing.B, text string, opts CheckOptions) {
	e := benchEngine(b)
	start := time.Now()
	e.Check(text, opts)
	cold := time.Since(start)
	for b.Loop() {
		e.Check(text, opts)
	}
	b.ReportMetric(float64(cold.Microseconds()), "cold-µs")
}

func BenchmarkCheckSentence(b *testing.B) {
	benchCheck(b, "Bugün cok yorgunum, yarın Ankaraya gidiyorum.", CheckOptions{})
}

func BenchmarkCheckParagraph(b *testing.B) {
	if n := len(strings.Fields(benchParagraph)); n != 100 {
		b.Fatalf("the paragraph has %d words", n)
	}
	benchCheck(b, benchParagraph, CheckOptions{})
}

func BenchmarkCheckParagraphWord(b *testing.B) {
	at := runeIndex(benchParagraph, "herkez")
	benchCheck(b, benchParagraph, CheckOptions{Range: &[2]int{at, at + 6}})
}

func BenchmarkInspectParagraph(b *testing.B) {
	e := benchEngine(b)
	for b.Loop() {
		e.Inspect(benchParagraph, CheckOptions{})
	}
}
