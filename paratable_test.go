package main

import (
	"testing"
	"time"
)

// paraOfBox gives, for every box, what reading the paragraph by its rune gives: its start, end and style; and
// it still does for boxes laid out before an edit, which the table no longer matches (#312).
func TestParaOfBoxMatchesText(t *testing.T) {
	a, tt := newTestApp(t, sample)
	tt.Advance(100 * time.Millisecond)
	check := func(when string) {
		t.Helper()
		text := a.pane().doc.Text()
		if len(a.pane().boxes) == 0 {
			t.Fatalf("%s: no boxes", when)
		}
		for _, b := range a.pane().boxes {
			if b.Start > text.Len() {
				continue
			}
			s, e, para := a.paraOfBox(a.pane(), b)
			ws, we := text.ParagraphRange(b.Start)
			if s != ws || e != we {
				t.Fatalf("%s: box at %d: %d..%d, the text says %d..%d", when, b.Start, s, e, ws, we)
			}
			if para.Style != text.ParagraphStyleAt(b.Start) {
				t.Fatalf("%s: box at %d: style %+v, the text says %+v", when, b.Start, para.Style, text.ParagraphStyleAt(b.Start))
			}
			var line string
			for _, r := range para.Runs {
				line += r.Text
			}
			if want := text.Slice(ws, we).String(); line != want {
				t.Fatalf("%s: box at %d: %q, the text says %q", when, b.Start, line, want)
			}
		}
	}
	check("laid out")
	// a paragraph put in at the top: the boxes of the last frame start where other paragraphs are now
	a.pane().doc.SetSelection(0, 0)
	typeEach(tt, "Yeni\n")
	check("before the next frame") // the boxes are a frame behind the text
	tt.Advance(100 * time.Millisecond)
	check("after it")
}
