package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

func docOf(md string) *richtext.Document { return richtext.NewDocument(parseBody(md)) }

func mdOf(d *richtext.Document) string { return strings.TrimSpace(bodyMarkdown(d.Text())) }

// at is the start of the paragraph whose text is s.
func at(d *richtext.Document, s string) int {
	t := d.Text()
	for p := range t.Paragraphs() {
		ps, pe := t.ParagraphRange(p)
		if t.Slice(ps, pe).String() == s {
			return ps
		}
	}
	return -1
}

func TestBlockRange(t *testing.T) {
	d := docOf("# Bir\n\nmetin\n\n- a\n    - a1\n    - a2\n- b\n\n## İki\n\nson\n")
	tx := d.Text()
	check := func(start, want string) {
		t.Helper()
		s, e := blockRange(tx, at(d, start))
		if got := strings.ReplaceAll(tx.Slice(s, e).String(), "\n", "|"); got != want {
			t.Errorf("block of %q: %q, want %q", start, got, want)
		}
	}
	check("a", "a|a1|a2")
	check("a1", "a1")
	check("b", "b")
	check("İki", "İki|son")
	check("Bir", "Bir|metin|a|a1|a2|b|İki|son")
	check("metin", "metin")
}

func TestMoveBlock(t *testing.T) {
	d := docOf("- a\n    - a1\n- b\n- c\n")
	d.SetSelection(at(d, "b"), at(d, "b"))
	if !moveBlock(d, -1) {
		t.Fatal("did not move")
	}
	if got := mdOf(d); got != "- b\n- a\n    - a1\n- c" {
		t.Fatalf("up: %q", got)
	}
	if _, c := d.Selection(); c != at(d, "b") {
		t.Fatalf("caret %d", c)
	}
	moveBlock(d, 1) // back down past a and its child
	if got := mdOf(d); got != "- a\n    - a1\n- b\n- c" {
		t.Fatalf("down: %q", got)
	}
	moveBlock(d, 1)
	if got := mdOf(d); got != "- a\n    - a1\n- c\n- b" {
		t.Fatalf("down to the end: %q", got)
	}
	if moveBlock(d, 1) {
		t.Fatal("moved past the end")
	}
	d.Undo()
	if got := mdOf(d); got != "- a\n    - a1\n- b\n- c" {
		t.Fatalf("undo: %q", got)
	}
}

func TestMoveBlockTo(t *testing.T) {
	d := docOf("Bir\n\nİki\n\nÜç\n\nDört\n")
	tx := d.Text()
	s, e := blockRange(tx, at(d, "Bir"))
	moveBlockTo(d, s, e, at(d, "Üç"))
	if got := mdOf(d); got != "İki\n\nBir\n\nÜç\n\nDört" {
		t.Fatalf("down: %q", got)
	}
	s, e = blockRange(d.Text(), at(d, "Dört"))
	moveBlockTo(d, s, e, 0)
	if got := mdOf(d); got != "Dört\n\nİki\n\nBir\n\nÜç" {
		t.Fatalf("up from the end: %q", got)
	}
	s, e = blockRange(d.Text(), at(d, "Dört"))
	moveBlockTo(d, s, e, d.Text().Len()+1)
	if got := mdOf(d); got != "İki\n\nBir\n\nÜç\n\nDört" {
		t.Fatalf("to the end: %q", got)
	}
}

func TestDuplicateDeleteTurn(t *testing.T) {
	d := docOf("# Başlık\n\nmetin\n\n- a\n    - a1\n")
	duplicateBlock(d, at(d, "a"))
	if got := mdOf(d); got != "# Başlık\n\nmetin\n\n- a\n    - a1\n- a\n    - a1" {
		t.Fatalf("duplicate: %q", got)
	}
	deleteBlock(d, at(d, "metin"))
	if got := mdOf(d); !strings.HasPrefix(got, "# Başlık\n\n- a") {
		t.Fatalf("delete: %q", got)
	}
	turnInto(d, at(d, "Başlık"), "tasks")
	if got := mdOf(d); !strings.HasPrefix(got, "- [ ] Başlık") {
		t.Fatalf("turn: %q", got)
	}
}

// Ekle… puts in a table and a callout as raw blocks: written as Markdown, read back as they are.
func TestInsertRawBlocks(t *testing.T) {
	for _, r := range rawTemplates {
		a, tt := newTestApp(t, "Önce\n")
		tt.Frame()
		n := a.doc.Text().Len()
		a.doc.SetSelection(n, n)
		tt.Frame()
		a.insertRaw(n, r)
		tt.Frame()
		md := bodyMarkdown(a.doc.Text())
		if !strings.Contains(md, strings.Join(r.lines, "\n")) {
			t.Fatalf("%s: not written as Markdown: %q", r.name, md)
		}
		back := bodyMarkdown(parseBody(md))
		if strings.TrimSpace(back) != strings.TrimSpace(md) {
			t.Fatalf("%s: not read back as it was:\n%q\n%q", r.name, md, back)
		}
	}
}

// A block folded has what is under it held behind a chip; the file has it
// back, and an unfold puts it where it was.
func TestFoldAndUnfold(t *testing.T) {
	src := "# Başlık\n\nBir\n\nİki\n\n# Sonraki\n\n- a\n  - a1\n  - a2\n- b"
	a, tt := newTestApp(t, src+"\n")
	tt.Frame()
	whole := mdOf(a.doc)
	a.fold(at(a.doc, "Başlık"))
	tt.Frame()
	if shown := a.doc.Text().String(); strings.Contains(shown, "Bir") || strings.Contains(shown, "İki") {
		t.Fatalf("not hidden: %q", shown)
	}
	if ch := foldOf(a.doc.Text(), at(a.doc, "Başlık")); ch == nil || ch.lines < 2 {
		t.Fatalf("no fold chip: %+v", ch)
	}
	if got := mdOf(a.doc); got != whole {
		t.Fatalf("the file would change:\n%q\n%q", got, whole)
	}
	// a list item with items under it
	a.fold(at(a.doc, "a"))
	if got := mdOf(a.doc); got != whole {
		t.Fatalf("the file would change with a list folded:\n%q\n%q", got, whole)
	}
	// nested: fold the heading again with the list's fold inside it, then put it all back
	a.toggleFold(at(a.doc, "Sonraki"))
	if got := mdOf(a.doc); got != whole {
		t.Fatalf("nested folds change the file:\n%q\n%q", got, whole)
	}
	for range 3 {
		for _, s := range []string{"Başlık", "Sonraki", "a"} {
			if p := at(a.doc, s); p >= 0 {
				if ch := foldOf(a.doc.Text(), p); ch != nil {
					a.unfold(ch)
				}
			}
		}
	}
	if got := mdOf(a.doc); got != whole || strings.Contains(a.doc.Text().String(), "⋯") {
		t.Fatalf("not back:\n%q", got)
	}
}

// The lines of a block of code are one block: whichever line is asked about, it is the
// whole of it, and its grip is the first line's.
func TestCodeBlockIsOneBlock(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n\n```go\nbir\niki\nüç\n```\n\nSonra\n")
	tt.SetSize(900, 600)
	tt.Frame()
	t0 := a.doc.Text()
	first := at(a.doc, "bir")
	mid := at(a.doc, "iki")
	last := at(a.doc, "üç")
	s0, e0 := blockRange(t0, first)
	s1, e1 := blockRange(t0, mid)
	s2, e2 := blockRange(t0, last)
	if s0 != first || s1 != s0 || s2 != s0 || e0 != e1 || e1 != e2 {
		t.Fatalf("lines of one block: %d-%d, %d-%d, %d-%d", s0, e0, s1, e1, s2, e2)
	}
	if got := t0.Slice(s0, e0).String(); got != "bir\niki\nüç" {
		t.Fatalf("the block: %q", got)
	}
	// the grip of the middle line is the first's
	for _, b := range a.boxes {
		if b.Start == mid {
			a.gripAt(b)
		}
	}
	if a.grip.para != first {
		t.Fatalf("the grip of a middle line is at %d, not the first line %d", a.grip.para, first)
	}
	// deleting from the middle takes the whole block
	deleteBlock(a.doc, mid)
	if got := mdOf(a.doc); got != "Önce\n\nSonra" {
		t.Fatalf("after: %q", got)
	}
}

// A block moved, dragged or copied keeps what its paragraphs are, and so does the one beside it: a divider, a
// task's kind and a code block's language live in paragraph IDs, which Replace does not carry (#127).
func TestBlockOpsKeepIDs(t *testing.T) {
	para := func(text, id string) richtext.Paragraph {
		p := richtext.Paragraph{Style: richtext.ParagraphStyle{ID: id}}
		if text != "" {
			p.Runs = []richtext.Run{{Text: text}}
		}
		return p
	}
	sig := func(d *richtext.Document) string {
		var out []string
		for _, p := range d.Text().Paragraphs() {
			s := ""
			for _, r := range p.Runs {
				s += r.Text
			}
			out = append(out, s+"#"+p.Style.ID)
		}
		return strings.Join(out, " | ")
	}
	star := taskIDPrefix + "star"
	newDoc := func() *richtext.Document {
		return richtext.NewDocument(richtext.TextOf(para("a", ""), para("", hrID), para("yıldız", star), para("b", "")))
	}
	const start = "a# | #celestine:hr | yıldız#celestine:task:star | b#"

	d := newDoc()
	d.SetSelection(at(d, ""), at(d, ""))
	moveBlock(d, -1)
	if got := sig(d); got != "#celestine:hr | a# | yıldız#celestine:task:star | b#" {
		t.Fatalf("divider up: %s", got)
	}
	d = newDoc()
	d.SetSelection(at(d, "yıldız"), at(d, "yıldız"))
	moveBlock(d, 1)
	if got := sig(d); got != "a# | #celestine:hr | b# | yıldız#celestine:task:star" {
		t.Fatalf("task down: %s", got)
	}
	d = newDoc()
	s, e := blockRange(d.Text(), at(d, ""))
	moveBlockTo(d, s, e, d.Text().Len()+1)
	if got := sig(d); got != "a# | yıldız#celestine:task:star | b# | #celestine:hr" {
		t.Fatalf("divider dragged to the end: %s", got)
	}
	d = newDoc()
	s, e = blockRange(d.Text(), at(d, "yıldız"))
	moveBlockTo(d, s, e, 0)
	if got := sig(d); got != "yıldız#celestine:task:star | a# | #celestine:hr | b#" {
		t.Fatalf("task dragged to the top: %s", got)
	}
	d = newDoc()
	duplicateBlock(d, at(d, "yıldız"))
	if got := sig(d); got != "a# | #celestine:hr | yıldız#celestine:task:star | yıldız#celestine:task:star | b#" {
		t.Fatalf("task copied: %s", got)
	}
	if d = newDoc(); sig(d) != start {
		t.Fatal(sig(d))
	}
}
