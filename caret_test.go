package main

import (
	"image"
	"math"
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

// testLibrary is a library in a temporary folder with one collection and a page of src.
func testLibrary(t *testing.T, src string) *Library {
	t.Helper()
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Sayfa")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveText(src); err != nil {
		t.Fatal(err)
	}
	return lib
}

func newTestApp(t *testing.T, src string) (*app, *ui.Tester) {
	t.Helper()
	a := newApp(testLibrary(t, src))
	a.noSound = true
	t.Cleanup(a.stamping.Wait) // the journal's days written before the library's folder goes
	tt := ui.NewTester(a.view, 900, 1400)
	a.clipRead = tt.Clipboard // the app reads the clipboard the editor does: the Tester's, not the system's
	tt.Advance(50 * time.Millisecond)
	return a, tt
}

// TestCaretMatchesEditor puts the caret on every rune of every kind of
// paragraph and checks that ours is where the editor's own caret is.
func TestCaretMatchesEditor(t *testing.T) { caretMatches(t, 0) }

// With the letters spaced, the caret is still where the editor's is: the app lays a paragraph out spaced as the
// editor draws it (codecolors.go).
func TestCaretMatchesEditorSpaced(t *testing.T) { caretMatches(t, 3) }

func caretMatches(t *testing.T, spacing int) {
	a, tt := newTestApp(t, sample)
	a.settings.LetterSpacing = spacing
	tt.Frame()
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
	a := newApp(testLibrary(t, sample))
	now := time.Now()
	a.clock = func() time.Time { return now }
	tt := ui.NewTester(a.view, 900, 900)
	tt.Type("x")
	now = now.Add(time.Second)
	tt.Advance(time.Second)
	if a.dirty {
		t.Fatalf("not saved (err %q)", a.saveErr)
	}
	body, err := a.page.Text()
	if err != nil {
		t.Fatal(err)
	}
	if b := parseBody(body); !b.Equal(a.doc.Text()) {
		t.Fatalf("reloaded text differs:\n%s\n---\n%s", b.Markdown(), a.doc.Text().Markdown())
	}
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

// units counts UTF-16 units up to a rune, astral runes as two.
func TestUnits(t *testing.T) {
	line := "a😀b界c"
	for at, want := range []int{0, 1, 3, 4, 5, 6, 6} {
		if got := units(line, at); got != want {
			t.Errorf("units(%q, %d) = %d, want %d", line, at, got, want)
		}
	}
}

// A paragraph of half a megabyte gets no caret of ours (laying it out again for
// each frame took minutes), and a long one has its layout kept while it is as it was.
func TestLongParagraphCaret(t *testing.T) {
	a, tt := newTestApp(t, strings.Repeat("kelime ", 6000)+"\n\nson")
	a.doc.SetSelection(3, 3)
	tt.Frame()
	if _, ok := locate(a.doc, a.boxes, 600, a.font, a.theme); ok {
		t.Fatal("a paragraph over maxCaretParagraph got a caret")
	}
	short, tt2 := newTestApp(t, strings.Repeat("kelime ", 400)+"\n\nson")
	short.doc.SetSelection(3, 3)
	tt2.Frame()
	short.doc.SetSelection(3, 3)
	cb, ok := locate(short.doc, short.boxes, 600, short.font, short.theme)
	if !ok {
		t.Fatal("no caret in a paragraph of 2800 runes")
	}
	l1 := longMemo.l
	cb2, _ := locate(short.doc, short.boxes, 600, short.font, short.theme)
	if longMemo.l != l1 || cb2.r != cb.r {
		t.Fatal("the layout of a long paragraph was made again, the doc as it was")
	}
	short.doc.SetSelection(40, 40)
	if _, ok := locate(short.doc, short.boxes, 600, short.font, short.theme); !ok || longMemo.l != l1 {
		t.Fatal("the caret moving made the layout again")
	}
}

// MyGo's Document.CaretRect is where the editor paints the caret, on every rune as the editor reports it to input
// methods: the find bar reveals its match from it, with no layout of the app's own (#306).
func TestCaretRectMatchesEditor(t *testing.T) {
	a, tt := newTestApp(t, sample)
	tt.Frame()
	n := a.doc.Text().Len()
	checked := 0
	for i := 0; i <= n; i++ {
		a.doc.SetSelection(i, i)
		tt.Frame()
		want, ok := tt.TextCaret()
		if !ok {
			t.Fatal("the editor has no caret: is it focused?")
		}
		got, ok := a.doc.CaretRect()
		if !ok {
			continue // out of view
		}
		got.X += a.editorR.X
		got.Y += a.editorR.Y
		if math.Abs(float64(got.X-want.X)) > 0.75 || math.Abs(float64(got.Y-want.Y)) > 0.75 || math.Abs(float64(got.H-want.H)) > 0.75 {
			t.Fatalf("rune %d: CaretRect %+v, the editor's %+v", i, got, want)
		}
		if cb, ok := locate(a.doc, a.boxes, a.editorR.W, a.font, a.theme); ok {
			g := cb.full
			g.X += a.editorR.X
			g.Y += a.editorR.Y
			if math.Abs(float64(g.X-got.X)) > 0.75 || math.Abs(float64(g.Y-got.Y)) > 0.75 {
				t.Fatalf("rune %d: locate %+v, CaretRect %+v", i, g, got)
			}
		}
		checked++
	}
	if checked < n/2 {
		t.Fatalf("only %d of %d places checked", checked, n)
	}
}

func TestLongParagraphCaretDraws(t *testing.T) {
	prefix := "ilk paragraf\n\n"
	longText := prefix + strings.Repeat("kelime ", 6000) + "\n\nson"
	a, tt := newTestApp(t, longText)
	// Place caret inside the oversized (> maxCaretParagraph) second paragraph.
	wantPara := len([]rune("ilk paragraf\n"))
	caretPos := wantPara + 10
	a.doc.SetSelection(caretPos, caretPos)
	a.setCaret(caretMizan) // block-style caret relies on caretPara and caretTop
	tt.Frame()
	tt.Frame()

	if !a.focused {
		t.Fatal("editor is not focused")
	}
	if a.caretFBReads != 1 {
		t.Fatalf("expected 1 CaretRect fallback evaluation on steady state, got %d", a.caretFBReads)
	}
	if a.caretWin.H <= 0 {
		t.Fatalf("caret not positioned on oversized paragraph: %+v", a.caretWin)
	}
	if a.caretPara != wantPara {
		t.Fatalf("caretPara = %d, want %d", a.caretPara, wantPara)
	}
	if b, ok := a.boxAt(caretPos); !ok || a.caretTop != a.editorR.Y+b.Top {
		t.Fatalf("caretTop = %v, want %v (box ok=%v)", a.caretTop, a.editorR.Y+b.Top, ok)
	}

	// Verify pixels change around caretWin when toggled off via CaretBlink vs painted on.
	cr := a.caretWin
	region := image.Rect(max(0, int(cr.X)-6), max(0, int(cr.Y)-6), int(cr.X+cr.W)+16, int(cr.Y+cr.H)+6)
	withCaret := *tt.Image()
	withCaret.Pix = append([]byte(nil), withCaret.Pix...)

	// Repeated idle/blink frames with unchanged text/caret must reuse cached fallback without relayout.
	a.settings.CaretBlink = 0
	tt.Frame()
	tt.Frame()
	if a.caretFBReads != 1 {
		t.Fatalf("CaretRect fallback re-ran on unchanged frames: reads=%d", a.caretFBReads)
	}

	// Scroll caret out of the visible viewport: offscreen caret must not draw.
	beforeScrollDraws := a.caretDraws
	a.pageScroll.Y = cr.Y + 500
	tt.Frame()
	if a.caretDraws != beforeScrollDraws {
		t.Fatalf("expected no caret draw when scrolled out of view, draws %d -> %d", beforeScrollDraws, a.caretDraws)
	}
	// Reset scroll and move caret back to 0 (outside region) to compare painted caret pixels in region.
	a.pageScroll.Y = 0
	a.doc.SetSelection(0, 0)
	tt.Frame()
	withoutCaret := tt.Image()
	if pixelsDiffer(&withCaret, withoutCaret, region, image.Rectangle{}) < 4 {
		t.Fatalf("expected painted caret pixels inside %v", region)
	}
}

// TestPageOpensWithCaretAtStart: a page read from its file starts with the caret before its first letter, not after
// its last (#454); a page come back to keeps the caret where it was left.
func TestPageOpensWithCaretAtStart(t *testing.T) {
	a, tt := newTestApp(t, sample)
	tt.Frame()
	if _, c := a.doc.Selection(); c != 0 {
		t.Fatalf("the first page opens with the caret at %d of %d", c, a.doc.Text().Len())
	}
	first := a.page
	a.doc.SetSelection(5, 5)
	other, err := first.Coll.NewPage(nil, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if err := other.SaveText("alpha\n\nbeta\n\ngamma"); err != nil {
		t.Fatal(err)
	}
	a.open(other)
	tt.Frame()
	if _, c := a.doc.Selection(); c != 0 {
		t.Fatalf("another page opens with the caret at %d of %d", c, a.doc.Text().Len())
	}
	a.open(first)
	tt.Frame()
	if _, c := a.doc.Selection(); c != 5 {
		t.Fatalf("back to the first page, the caret is at %d, not where it was left (5)", c)
	}
}
