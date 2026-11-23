package main

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// pixelsDiffer counts the bytes that differ between a and b, in the rectangle in and outside skip.
func pixelsDiffer(a, b *image.RGBA, in, skip image.Rectangle) int {
	n := 0
	for y := in.Min.Y; y < in.Max.Y; y++ {
		for x := in.Min.X; x < in.Max.X; x++ {
			if image.Pt(x, y).In(skip) {
				continue
			}
			i := a.PixOffset(x, y)
			for k := 0; k < 4; k++ {
				if a.Pix[i+k] != b.Pix[i+k] {
					n++
				}
			}
		}
	}
	return n
}

func fileText(t *testing.T, a *app) string {
	t.Helper()
	b, err := a.page.Body()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// With autosave off nothing is written by itself: ⌘S writes, and so does leaving the page.
func TestAutoSaveOff(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.clock = tt.Now
	a.settings.NoAutoSave = true
	tt.Frame()
	tt.Type("yeni")
	tt.Advance(3 * time.Second)
	if !a.dirty {
		t.Fatal("saved by itself")
	}
	if got := a.savedText(); got != "kaydedilmedi" {
		t.Fatalf("status %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyS)
	tt.Advance(50 * time.Millisecond)
	if a.dirty {
		t.Fatal("⌘S did not save")
	}
	if a.savedText() == "kaydedildi" || a.savedAt.IsZero() {
		t.Fatalf("no time shown: %q", a.savedText())
	}
	// leaving the page keeps what was typed
	tt.Type("daha")
	tt.Advance(100 * time.Millisecond)
	p := a.page
	a.flush()
	if a.dirty {
		t.Fatal("flush did not save")
	}
	if got := fileText(t, a); got == "" || p == nil {
		t.Fatal("nothing in the file")
	}
}

// The delay of the autosave is the setting's.
func TestAutoSaveDelay(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.clock = tt.Now
	a.settings.SaveAfter = 5
	tt.Frame()
	tt.Type("a")
	tt.Advance(2 * time.Second)
	if !a.dirty {
		t.Fatal("saved before the delay")
	}
	tt.Advance(4 * time.Second)
	if a.dirty {
		t.Fatal("not saved after the delay")
	}
}

// ⌥⌘= and ⌥⌘- change the text's size, the View menu puts it back (⌘= and ⌘- zoom the window, winzoom.go).
func TestFontKeys(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	base := a.fontSize()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyEqual)
	tt.Advance(20 * time.Millisecond)
	if a.fontSize() != base+1 {
		t.Fatalf("bigger: %v", a.fontSize())
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyMinus)
	tt.Key(ui.Cmd|ui.Alt, ui.KeyMinus)
	tt.Advance(20 * time.Millisecond)
	if a.fontSize() != base-1 {
		t.Fatalf("smaller: %v", a.fontSize())
	}
	for i := 0; i < 30; i++ {
		a.stepFont(-1)
	}
	if a.fontSize() != minFont {
		t.Fatalf("no floor: %v", a.fontSize())
	}
	a.resetFont() // the View menu's: ⌥⌘0 is the editor's plain text (#535)
	tt.Advance(20 * time.Millisecond)
	if a.fontSize() != base {
		t.Fatalf("reset: %v", a.fontSize())
	}
}

// ⌥⌘0 in a heading makes it plain text, as the block's hints say, and leaves the text's size as it is (#535).
func TestPlainTextKey(t *testing.T) {
	a, tt := newTestApp(t, "# Heading one\n\nbody\n")
	tt.Frame()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyEqual) // a size of its own, which a reset would take away
	tt.Advance(20 * time.Millisecond)
	size := a.fontSize()
	a.doc.SetSelection(3, 3) // in the heading
	tt.Frame()
	if a.doc.Text().ParagraphStyleAt(0).Heading != 1 {
		t.Fatal("the test's page has no heading")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.Key0)
	tt.Advance(20 * time.Millisecond)
	if h := a.doc.Text().ParagraphStyleAt(0).Heading; h != 0 || a.fontSize() != size {
		t.Fatalf("⌥⌘0 in a heading: heading %d, size %v (was %v)", h, a.fontSize(), size)
	}
}

// Paragraph spacing and the first line's indent reach the editor's theme.
func TestParagraphSettings(t *testing.T) {
	a, tt := newTestApp(t, "bir\n\niki\n")
	tt.Frame()
	if a.theme.ParagraphSpacing != defaultParaGap || a.theme.FirstLineIndent != 0 {
		t.Fatalf("defaults: %v %v", a.theme.ParagraphSpacing, a.theme.FirstLineIndent)
	}
	g := 1.2
	a.settings.ParaGap, a.settings.Indent = &g, 1.5
	tt.Frame()
	tt.Frame()
	if a.theme.ParagraphSpacing != 1.2 || a.theme.FirstLineIndent != 1.5 {
		t.Fatalf("set: %v %v", a.theme.ParagraphSpacing, a.theme.FirstLineIndent)
	}
}

// White space is drawn over the text, and not at all when it is not asked for.
func TestWhiteSpaceMarks(t *testing.T) {
	a, tt := newTestApp(t, "bir iki  üç sekme \n\n"+strings.Repeat("dolgu\n\n", 9)+"son")
	a.doc.SetSelection(3, 3)
	a.doc.InsertText("\t") // the page text takes a tab as a space: this one is the editor's own
	a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
	tt.Advance(100 * time.Millisecond)
	tt.Advance(100 * time.Millisecond)
	// the caret and what moves round it are left out: only the marks are looked at
	last := a.boxes[len(a.boxes)-1].Start
	a.doc.SetSelection(last, last)
	tt.Advance(100 * time.Millisecond)
	tt.Advance(100 * time.Millisecond)
	cb, ok := locate(a.doc, a.boxes, a.editorR.W, a.font, a.theme)
	if !ok || cb.r.Y < 100 {
		t.Fatalf("caret not placed: %+v", cb)
	}
	skip := image.Rect(int(a.editorR.X+cb.r.X)-80, int(a.editorR.Y+cb.r.Y)-80, int(a.editorR.X+cb.r.X)+80, int(a.editorR.Y+cb.r.Y+cb.r.H)+80)
	text := image.Rect(int(a.editorR.X), int(a.editorR.Y), int(a.editorR.X+a.editorR.W), int(a.editorR.Y)+120) // the first lines
	snap := func() *image.RGBA {
		tt.Frame() // what the settings changed is painted by a frame
		tt.Advance(40 * time.Millisecond)
		im := *tt.Image()
		im.Pix = append([]byte(nil), im.Pix...)
		return &im
	}
	for name, set := range map[string]func(){
		"spaces": func() { a.settings.ShowSpaces = true },
		"tabs":   func() { a.settings.ShowTabs = true },
		"breaks": func() { a.settings.ShowBreaks = true },
	} {
		a.settings = Settings{}
		off := snap()
		set()
		on := snap()
		if d := pixelsDiffer(off, on, text, skip); d == 0 {
			t.Errorf("%s: nothing drawn (%d bytes differ) in %v skip %v", name, d, text, skip)
		}
		a.settings = Settings{}
		if d := pixelsDiffer(off, snap(), text, skip); d != 0 {
			t.Errorf("%s: marks left behind (%d bytes)", name, d)
		}
	}
}

// The new settings come out of an old file: nothing set means the defaults.
func TestEditorSettingsDefaults(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.settings = Settings{}
	if a.paraGap() != defaultParaGap || a.saveAfter() != 700*time.Millisecond || a.firstIndent() != 0 || a.settings.NoAutoSave {
		t.Fatal("defaults are not the old behaviour")
	}
	f := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(f, []byte(`{"mcp":false,"fontSize":18,"showPath":true}`), 0o644)
	a.settingsFile = f
	a.loadSettings()
	if a.fontSize() != 18 {
		t.Fatal("old file not read")
	}
}

// With white space shown only in the selection, a page with nothing selected has no marks, and one selected across
// the spaces has them there.
func TestWhiteSpaceOnlyInSelection(t *testing.T) {
	a, tt := newTestApp(t, "one two three four\n\n"+strings.Repeat("filler\n\n", 9)+"end")
	end := a.doc.Text().Len()
	a.doc.SetSelection(end, end) // the caret far from the first line
	tt.Advance(100 * time.Millisecond)
	tt.Advance(100 * time.Millisecond)
	first := image.Rect(int(a.editorR.X), int(a.editorR.Y), int(a.editorR.X+a.editorR.W), int(a.editorR.Y)+40)
	snap := func() *image.RGBA {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
		im := *tt.Image()
		im.Pix = append([]byte(nil), im.Pix...)
		return &im
	}
	plain := snap()
	a.settings.ShowSpaces, a.settings.WhiteInSel = true, true
	if n := pixelsDiffer(plain, snap(), first, image.Rectangle{}); n != 0 {
		t.Fatalf("marks with nothing selected: %d", n)
	}
	a.settings.WhiteInSel = false
	if n := pixelsDiffer(plain, snap(), first, image.Rectangle{}); n == 0 {
		t.Fatal("no marks at all: the test sees nothing")
	}
}

// A caret set to blink is shown while it moves, and once still is hidden and shown again; set not to, it stays.
func TestCaretBlink(t *testing.T) {
	a, tt := newTestApp(t, "one two\n")
	a.settings.NoCaretFX = true // a plain caret: nothing else moves round it
	a.doc.SetSelection(3, 3)
	tt.Advance(100 * time.Millisecond)
	box := image.Rect(int(a.editorR.X), int(a.editorR.Y), int(a.editorR.X+a.editorR.W), int(a.editorR.Y)+40)
	snap := func() *image.RGBA {
		im := *tt.Image()
		im.Pix = append([]byte(nil), im.Pix...)
		return &im
	}
	a.settings.CaretBlink = 500
	a.doc.SetSelection(4, 4) // moved: shown
	tt.Frame()
	shown := snap()
	tt.Advance(700 * time.Millisecond) // still past the first half: hidden
	tt.Frame()
	if pixelsDiffer(shown, snap(), box, image.Rectangle{}) == 0 {
		t.Fatal("the caret did not blink off")
	}
	tt.Advance(500 * time.Millisecond) // on again
	tt.Frame()
	if n := pixelsDiffer(shown, snap(), box, image.Rectangle{}); n != 0 {
		t.Fatalf("the caret did not come back: %d", n)
	}
	a.settings.CaretBlink = 0
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if n := pixelsDiffer(shown, snap(), box, image.Rectangle{}); n != 0 {
		t.Fatalf("a caret not set to blink went: %d", n)
	}
}

// Finding in a long page allocates a few times, not at each place of it, and a Vim key that is no word motion does
// not copy the page's runes (#198).
func TestFindAndVimCosts(t *testing.T) {
	a, tt := newTestApp(t, strings.Repeat("a long line of words to find in, ", 3000)+"\n")
	tt.Frame()
	a.find.query = "words"
	allocs := testing.AllocsPerRun(3, func() {
		a.find.ver = 0 // found again
		a.findAll()
	})
	if len(a.find.matches) != 3000 || allocs > 100 {
		t.Fatalf("%d matches, %v allocations a search", len(a.find.matches), allocs)
	}
	a.settings.VimKeys = true
	if !a.vimOn() {
		t.Skip("no normal mode here")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 20 {
		a.vimKey('l', 1)
	}
	runtime.ReadMemStats(&after)
	if per := (after.TotalAlloc - before.TotalAlloc) / 20; per > 50_000 { // the page's runes are 400 KB
		t.Fatalf("a Vim key allocates %d bytes", per)
	}
}
