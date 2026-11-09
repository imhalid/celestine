package library

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// history is the words of a history as "word:born-gone", a word that came
// as a rewrite marked with r, one that went for others with x.
func history(ws []Word) string {
	var out []string
	for _, w := range ws {
		s := w.Text
		if w.Para() {
			s = "¶"
		}
		s += ":" + strconv.Itoa(w.Born)
		if w.Rewrite {
			s += "r"
		}
		if w.Gone > 0 {
			s += "-" + strconv.Itoa(w.Gone)
			if w.Replaced {
				s += "x"
			}
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

// TestWordHistory: added words keep the version they came in, a rewritten
// word stays beside the one put in its place, and a new paragraph comes
// after the one before it.
func TestWordHistory(t *testing.T) {
	got := history(WordHistory([]string{
		"The cat sat on the mat",
		"The black cat sat on the red mat",
		"The black cat lay on the red rug\n\nNew para here",
	}))
	want := "The:1 black:2 cat:1 sat:1-3x lay:3r on:1 the:1 red:2 mat:1-3x rug:3r ¶:1 New:3 para:3 here:3 ¶:3"
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// TestWordHistoryCut: a cut word stays where it was, gone in the version
// that left it out.
func TestWordHistoryCut(t *testing.T) {
	got := history(WordHistory([]string{"one two three", "one three", "one three four"}))
	if want := "one:1 two:1-2 three:1 four:3 ¶:1"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// TestWordHistoryMoved: a paragraph moved elsewhere keeps the version it
// was written in and is not shown twice.
func TestWordHistoryMoved(t *testing.T) {
	a := "one two three four five six seven"
	b := "alpha beta gamma delta epsilon zeta eta"
	got := history(WordHistory([]string{a + "\n" + b, b + "\n" + a}))
	if strings.Contains(got, "-") || strings.Contains(got, ":2") {
		t.Fatalf("moved text taken for new: %s", got)
	}
	if !strings.HasPrefix(got, "alpha:1") {
		t.Fatalf("not in the reading order of the last version: %s", got)
	}
}

// TestWordHistoryLast: the words of the history not yet gone are the last
// version, and those born by a version and not gone by it are that version.
func TestWordHistoryLast(t *testing.T) {
	texts := []string{
		"Seven worlds of digital art",
		"Seven worlds of digital art: audiences, values\nWhat is digital art?",
		"Seven Worlds of Digital Art: A Visitor's Guide\nWhat is \"digital art\"?\nThe usual answer is simple.",
		"Seven Worlds of Digital Art: A Visitor's Guide\nThe usual answer is simple: art made with digital technologies.",
	}
	ws := WordHistory(texts)
	for v := 1; v <= len(texts); v++ {
		var got []string
		for _, w := range ws {
			if w.Born <= v && (w.Gone == 0 || w.Gone > v) {
				got = append(got, w.Text)
			}
		}
		var want []string
		for _, line := range strings.Split(texts[v-1], "\n") {
			want = append(append(want, strings.Fields(line)...), "\n")
		}
		if g, w := strings.Join(got, " "), strings.Join(want, " "); g != w {
			t.Fatalf("version %d:\n got %q\nwant %q", v, g, w)
		}
	}
}

func TestWordHistoryEmpty(t *testing.T) {
	if ws := WordHistory(nil); ws != nil {
		t.Fatal(ws)
	}
	if got := history(WordHistory([]string{"", "a b"})); got != "a:2 b:2 ¶:2" {
		t.Fatal(got)
	}
}

// TestLastOfDays: the copies of a day make one version, the last of them.
func TestLastOfDays(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.Local) }
	got := LastOfDays([]time.Time{day(2, 9), day(2, 11), day(2, 23), day(4, 8), day(5, 10), day(5, 12)})
	if want := []int{2, 3, 5}; !slices.Equal(got, want) {
		t.Fatalf("%v, not %v", got, want)
	}
	if got := LastOfDays(nil); got != nil {
		t.Fatal(got)
	}
}
