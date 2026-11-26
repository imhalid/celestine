package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// spansOfPara is paragraph p of a's page as plain spans, an object its own span, as the editor hands them to
// paintSpans.
func spansOfPara(a *app, p int) []ui.Span {
	pn := a.pane()
	t := pn.doc.Text()
	pt := a.paraTableOf(pn, t)
	from, to := pt.start[p], t.Len()
	if p+1 < len(pt.start) {
		to = pt.start[p+1] - 1
	}
	var out []ui.Span
	var run strings.Builder
	for _, r := range []rune(t.Slice(from, to).String()) {
		if r == richtext.ObjectReplacement {
			if run.Len() > 0 {
				out = append(out, ui.Span{Text: run.String()})
				run.Reset()
			}
			out = append(out, ui.Span{Text: string(r), ObjectWidth: 16})
			continue
		}
		run.WriteRune(r)
	}
	if run.Len() > 0 {
		out = append(out, ui.Span{Text: run.String()})
	}
	return out
}

// struck is the words of spans struck through, joined by "|" where a struck piece ends.
func struck(spans []ui.Span) string {
	var b strings.Builder
	was := false
	for _, s := range spans {
		if s.Strikethrough {
			b.WriteString(s.Text)
		} else if was {
			b.WriteString("|")
		}
		was = s.Strikethrough
	}
	return strings.Trim(b.String(), "|")
}

// A task done in the running text has its own words struck and muted, as a task line done has: the words after its
// box up to the next box, or those before it when none come after; a task not done, the words before the first box,
// and the box itself stay as they are; ticked off again, its words are plain (#599).
func TestInlineTaskDoneIsStruck(t *testing.T) {
	for _, c := range []struct{ md, want string }{
		{"Call [x] Ana, [ ] Ben\n", "Ana"},
		{"Call [ ] Ana, [x] Ben\n", "Ben"},
		{"Call [x] Ana, [x] Ben\n", "Ana|Ben"},
		{"Buy milk [x]\n", "Buy milk"},
		{"Buy milk [ ]\n", ""},
		{"Plain words\n", ""},
	} {
		a, tt := newTestApp(t, c.md)
		tt.Frame()
		got := a.inlineDoneSpans(a.pane(), 0, spansOfPara(a, 0))
		if s := struck(got); s != c.want {
			t.Errorf("%q: struck %q, want %q", c.md, s, c.want)
		}
		var all strings.Builder
		for _, s := range got {
			all.WriteString(s.Text)
			if s.Strikethrough && s.ObjectWidth != 0 {
				t.Errorf("%q: a box is struck", c.md)
			}
			if s.Strikethrough && s.Color != a.theme.Muted {
				t.Errorf("%q: %q struck but not muted", c.md, s.Text)
			}
		}
		var want strings.Builder
		for _, s := range spansOfPara(a, 0) {
			want.WriteString(s.Text)
		}
		if all.String() != want.String() {
			t.Errorf("%q: the words changed: %q, want %q", c.md, all.String(), want.String())
		}
	}
	a, tt := newTestApp(t, "Call [x] Ana\n")
	tt.Frame()
	boxes := checksIn(a)
	if len(boxes) != 1 {
		t.Fatalf("%d boxes", len(boxes))
	}
	a.toggleCheckChip(boxes[0])
	tt.Frame()
	if s := struck(a.inlineDoneSpans(a.pane(), 0, spansOfPara(a, 0))); s != "" {
		t.Fatalf("ticked off again, still struck: %q", s)
	}
}
