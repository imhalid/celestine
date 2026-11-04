package main

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// A document with every kind of paragraph the editor lays out differently.
const sample = `# Başlık bir

## Başlık iki

Düz bir paragraf, **kalın** ve _eğik_ sözlerle, ` + "`satır içi kod`" + ` ve bir 😀 ile.

Uzun bir paragraf: bu satır bir sütundan uzun olsun diye yazıldı, öyle ki editör onu birkaç satıra kırsın ve caret alt satırlarda da doğru yerde mi bakılsın; bir kez daha söyleyelim, kırılsın.

- madde
  - iç madde
1. numara
- [ ] görev
- [x] bitmiş görev

> alıntı satırı

` + "```" + `
kod bloğu
ikinci satır
` + "```" + `

Son.
`

func newTestApp(t *testing.T, src string) (*app, *ui.Tester) {
	t.Helper()
	a := newApp(richtext.NewDocument(richtext.ParseMarkdown(src)), "")
	tt := ui.NewTester(a.view, 900, 1400)
	tt.Advance(50 * time.Millisecond)
	return a, tt
}

// TestCaretMatchesEditor puts the caret on every rune of every kind of
// paragraph and checks that ours is where the editor's own caret is.
func TestCaretMatchesEditor(t *testing.T) {
	a, tt := newTestApp(t, sample)
	n := a.doc.Text().Len()
	checked := 0
	for i := 0; i <= n; i++ {
		a.doc.SetSelection(i, i)
		tt.Frame()
		want, ok := tt.TextCaret()
		if !ok {
			t.Fatal("the editor has no caret: is it focused?")
		}
		got, ok := locate(a.doc, a.boxes, a.editorR.W, a.font, a.theme)
		if !ok {
			continue // out of view
		}
		g := got.full
		g.X += a.editorR.X
		g.Y += a.editorR.Y
		if math.Abs(float64(g.X-want.X)) > 0.75 || math.Abs(float64(g.Y-want.Y)) > 0.75 || math.Abs(float64(g.H-want.H)) > 0.75 {
			t.Fatalf("rune %d: ours %+v, the editor's %+v (paragraph style %+v)", i, g, want, a.doc.Text().ParagraphStyleAt(min(i, n-1)))
		}
		checked++
	}
	if checked < n/2 {
		t.Fatalf("only %d of %d places checked", checked, n)
	}
}

// TestMarkdownNeverSeen types Markdown a key at a time: it turns into
// structure, and no mark is left in the text.
func TestMarkdownNeverSeen(t *testing.T) {
	a, tt := newTestApp(t, "")
	keys := func(s string) {
		for _, r := range s {
			if r == '\n' {
				tt.Key(0, ui.KeyEnter)
			} else {
				tt.Type(string(r))
			}
		}
	}
	keys("# Başlık\n- bir **iki** üç\n[] iş\n\n> söz") // an empty item's Enter leaves the list
	tt.Advance(50 * time.Millisecond)
	text := a.doc.Text()
	for _, mark := range []string{"#", "*", "[]", ">"} {
		if strings.Contains(text.String(), mark) {
			t.Fatalf("%q is left in the text: %q", mark, text.String())
		}
	}
	var styles []richtext.ParagraphStyle
	for _, p := range text.Paragraphs() {
		styles = append(styles, p.Style)
	}
	if len(styles) != 4 || styles[0].Heading != 1 || styles[1].List != richtext.Bullets || styles[2].List != richtext.Tasks || !styles[3].Quote {
		t.Fatalf("styles: %+v", styles)
	}
	md := text.Markdown()
	for _, want := range []string{"# Başlık", "- bir **iki** üç", "- [ ] iş", "> söz"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown lacks %q:\n%s", want, md)
		}
	}
}

// TestSaveAndReload writes the file after an edit, and reading it back gives the same text.
func TestSaveAndReload(t *testing.T) {
	path := t.TempDir() + "/not.md"
	a := newApp(richtext.NewDocument(richtext.ParseMarkdown(sample)), path)
	now := time.Now()
	a.clock = func() time.Time { return now }
	tt := ui.NewTester(a.view, 900, 900)
	tt.Type("x")
	now = now.Add(time.Second)
	tt.Advance(time.Second)
	if a.dirty {
		t.Fatalf("not saved (err %q)", a.saveErr)
	}
	b, err := loadDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.Text().Equal(a.doc.Text()) {
		t.Fatalf("reloaded text differs:\n%s\n---\n%s", b.Text().Markdown(), a.doc.Text().Markdown())
	}
	_ = os.Remove(path)
}

// TestUndoGivesMarksBack: ⌘Z right after a shortcut gives back the marks as typed.
func TestUndoGivesMarksBack(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Type("#")
	tt.Type(" ")
	if a.doc.Text().ParagraphStyleAt(0).Heading != 1 {
		t.Fatal("no heading")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if s := a.doc.Text().String(); s != "# " || a.doc.Text().ParagraphStyleAt(0).Heading != 0 {
		t.Fatalf("after undo: %q", s)
	}
}
