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
