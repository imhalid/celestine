package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
)

// pngBytes is a w by h picture.
func pngBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// pictures counts the pictures in the editor's text, and those with their image.
func pictures(a *app) (n, shown int) {
	t := a.doc.Text()
	for i, r := range []rune(t.String()) {
		if r != richtext.ObjectReplacement {
			continue
		}
		if att := t.AttributesAt(i).Attachment; att != nil {
			if _, ok := att.Data.(*imageData); ok {
				n++
				if att.Image != nil {
					shown++
				}
			}
		}
	}
	return
}

// TestAddImage: a picture is a file beside the page and a line of its
// Markdown, and is shown again when the page is read.
func TestAddImage(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	if err := a.addImage("foto.png", pngBytes(40, 20)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n, shown := pictures(a); n != 1 || shown != 1 {
		t.Fatalf("pictures %d, shown %d", n, shown)
	}
	md := a.pageMarkdown(a.page, a.doc.Text())
	if !strings.Contains(md, "![foto](resim-") || !strings.HasPrefix(md, "metin\n") {
		t.Fatalf("markdown:\n%s", md)
	}
	files, _ := filepath.Glob(filepath.Join(a.page.Dir, "resim-*.png"))
	if len(files) != 1 {
		t.Fatalf("files: %v", files)
	}
	a.save()
	// read again, as another start would
	a.docs = map[*Page]*richtext.Document{}
	a.images = nil
	a.page = nil
	a.open(a.lib.Collections[0].Pages[0])
	tt.Frame()
	if n, shown := pictures(a); n != 1 || shown != 1 {
		t.Fatalf("after reading: pictures %d, shown %d", n, shown)
	}
	if err := a.addImage("x.txt", []byte("hello")); err == nil {
		t.Fatal("a text taken for a picture")
	}
	// a picture whose file is gone stays a line of Markdown
	os.Remove(files[0])
	a.docs = map[*Page]*richtext.Document{}
	a.images = nil
	a.page = nil
	a.open(a.lib.Collections[0].Pages[0])
	tt.Frame()
	if n, shown := pictures(a); n != 1 || shown != 0 {
		t.Fatalf("missing: pictures %d, shown %d", n, shown)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "![foto](resim-") {
		t.Fatalf("a missing picture's line was lost:\n%s", md)
	}
}

// TestImageDropAndPaste: a file dropped on the text and a picture pasted are
// added; a paste of text is still the editor's.
func TestImageDropAndPaste(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	file := filepath.Join(t.TempDir(), "bir.png")
	os.WriteFile(file, pngBytes(30, 30), 0o644)
	r := a.editorR
	if r.W == 0 {
		t.Fatal("no editor")
	}
	tt.DropFiles(r.X+r.W/2, r.Y+r.H/2, file)
	tt.Advance(100 * time.Millisecond)
	if n, _ := pictures(a); n != 1 {
		t.Fatalf("after the drop: %d", n)
	}
	a.clipImage = func() []byte { return pngBytes(10, 10) }
	a.imgOnClip.Store(true)
	tt.Frame() // the key is registered once the flag is up
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Advance(100 * time.Millisecond)
	if n, _ := pictures(a); n != 2 {
		t.Fatalf("after the paste: %d (the key did not come to the app)", n)
	}
}

// TestImageInFile: pictures live in the library's file as well.
func TestImageInFile(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 900, 900)
	tt.Advance(50 * time.Millisecond)
	if err := a.addImage("foto.png", pngBytes(20, 20)); err != nil {
		t.Fatal(err)
	}
	es, _ := st.ReadDir(a.page.Dir)
	found := false
	for _, e := range es {
		found = found || strings.HasPrefix(e.Name(), "resim-")
	}
	if !found {
		t.Fatalf("not in the file: %v", es)
	}
}

// TestPasteTextStaysText: with text on the clipboard, ⌘V is the editor's.
func TestPasteTextStaysText(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	a.clipImage = func() []byte { return pngBytes(10, 10) }
	tt.SetClipboard("yapıştı")
	a.doc.SetSelection(0, 0)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Advance(100 * time.Millisecond)
	if n, _ := pictures(a); n != 0 || !strings.Contains(a.doc.Text().String(), "yapıştı") {
		t.Fatalf("pictures %d, text %q", n, a.doc.Text().String())
	}
}

// TestPicturesSwept: a picture no line names goes to the trash when the page is first opened; one that is named,
// one of another name, and one written in the last minutes stay.
func TestPicturesSwept(t *testing.T) {
	a, _ := newTestApp(t, "metin\n")
	if err := a.addImage("foto.png", pngBytes(20, 20)); err != nil {
		t.Fatal(err)
	}
	a.save()
	kept, _ := filepath.Glob(filepath.Join(a.page.Dir, "resim-*.png"))
	orphan := filepath.Join(a.page.Dir, "resim-20200101-010101.png")
	mine := filepath.Join(a.page.Dir, "benim.png")
	os.WriteFile(orphan, pngBytes(5, 5), 0o644)
	os.WriteFile(mine, pngBytes(5, 5), 0o644)
	old := a.clock().Add(-time.Hour)
	os.Chtimes(orphan, old, old)
	fresh := filepath.Join(a.page.Dir, "resim-20200101-020202.png") // a sync brought it before its page
	os.WriteFile(fresh, pngBytes(5, 5), 0o644)
	os.Chtimes(fresh, a.clock(), a.clock())
	a.docs = map[*Page]*richtext.Document{}
	a.page = nil
	a.open(a.lib.Collections[0].Pages[0])
	if _, err := os.Stat(orphan); err == nil {
		t.Fatal("the orphan stays")
	}
	if got, _ := filepath.Glob(filepath.Join(a.lib.Root, TrashDir, "*", "resim-20200101-010101.png")); len(got) != 1 {
		t.Fatal("the orphan is not in the trash")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("a picture written a moment ago was swept")
	}
	if _, err := os.Stat(mine); err != nil {
		t.Fatal("a picture of another name was deleted")
	}
	if _, err := os.Stat(kept[0]); err != nil {
		t.Fatal("a picture in use was deleted")
	}
}

// TestPictureLightbox: a click on a picture shows it large; a click or Esc puts it away.
func TestPictureLightbox(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1000, 800)
	if err := a.addImage("foto.png", pngBytes(900, 500)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Frame()
	if len(a.picSeen) == 0 {
		t.Fatal("the picture is not drawn")
	}
	h := a.picSeen[0]
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(300 * time.Millisecond)
	if a.lightbox == nil {
		t.Fatal("no lightbox")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.lightbox != nil {
		t.Fatal("Esc leaves it")
	}
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.ClickAt(5, 5)
	tt.Advance(100 * time.Millisecond)
	if a.lightbox != nil {
		t.Fatal("a click leaves it")
	}
}

// A picture that cannot be read yet (a sync has not brought it) is not kept as missing: it is tried again
// after a moment, and shows when it is there.
func TestImageComesLater(t *testing.T) {
	a, _ := newTestApp(t, "x")
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	a.clock = func() time.Time { return now }
	file := filepath.Join(a.page.Dir, "sonra.png")
	if a.loadImage(a.page, file) != nil {
		t.Fatal("a picture that is not there")
	}
	if err := a.page.FS().WriteFile(file, pngBytes(8, 8)); err != nil {
		t.Fatal(err)
	}
	if a.loadImage(a.page, file) != nil {
		t.Fatal("tried again at once, every frame")
	}
	now = now.Add(imageRetry + time.Second)
	if a.loadImage(a.page, file) == nil {
		t.Fatal("the picture that came is not read")
	}
}

// The settings are written for their owner alone.
func TestSettingsPrivate(t *testing.T) {
	a, _ := newTestApp(t, "x")
	dir := t.TempDir()
	a.settingsFile = filepath.Join(dir, "celestine", "settings.json")
	a.settings.SyncKey = "gizli"
	a.saveSettings()
	fi, err := os.Stat(a.settingsFile)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("settings file: %v %v", err, fi)
	}
	out := filepath.Join(dir, "disari.json")
	if err := a.exportSettingsTo(out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	if strings.Contains(string(b), "gizli") || strings.Contains(string(b), a.settings.Token) {
		t.Fatal("a secret is in the export")
	}
	if fi, _ := os.Stat(out); fi.Mode().Perm() != 0o600 {
		t.Fatalf("export mode %v", fi.Mode())
	}
	s, err := OpenStore(filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if fi, _ := os.Stat(filepath.Join(dir, "k.db")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("library file mode %v", fi.Mode())
	}
}

// An unreadable page opens read-only: it says so, nothing is written over its file, and its pictures stay (#125,
// #126). A restore of an earlier copy mends it.
func TestUnreadablePageKept(t *testing.T) {
	for _, bad := range []string{`{"meta":{"title":"Kesik"},"body":[{"r":[{"t":"yar`, `{"meta":{"title":"Yeni"},"body":{"v":2,"blocks":[]}}`} {
		a, _ := newTestApp(t, "ilk hâli\n")
		p := a.page
		a.save()
		pic := filepath.Join(p.Dir, "resim-20200101-010101.png")
		os.WriteFile(pic, pngBytes(5, 5), 0o644)
		old := a.clock().Add(-time.Hour)
		os.Chtimes(pic, old, old)
		os.WriteFile(p.File(), []byte(bad), 0o644)
		a.docs = map[*Page]*richtext.Document{}
		a.page = nil
		a.open(p)
		if p.Unreadable == "" {
			t.Fatalf("%q: not seen as unreadable", bad)
		}
		a.doc.SetText(richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "x"}}}))
		a.dirty = true
		a.save()
		if b, _ := os.ReadFile(p.File()); string(b) != bad {
			t.Fatalf("%q: written over: %s", bad, b)
		}
		if _, err := os.Stat(pic); err != nil {
			t.Fatalf("%q: its picture was swept", bad)
		}
		if err := p.SetTitle("başka"); err == nil {
			if b, _ := os.ReadFile(p.File()); string(b) != bad {
				t.Fatalf("%q: a rename wrote over it", bad)
			}
		}
		os.WriteFile(p.File(), JoinPage(map[string]string{"title": "Kesik"}, `[{"r":[{"t":"onarıldı"}]}]`), 0o644)
		a.takeOutside(p)
		if p.Unreadable != "" || !strings.Contains(a.doc.Text().String(), "onarıldı") {
			t.Fatalf("%q: a mended file is not taken: %q %q", bad, p.Unreadable, a.doc.Text().String())
		}
	}
}

// A page that is not open, made readable again outside the app, is written to again: its values and its place are
// not refused until it is opened (#541).
func TestUnreadablePageMendedClosed(t *testing.T) {
	a, _ := newTestApp(t, "ilk\n")
	p, _ := a.page.Coll.NewPage(nil, "Kapalı")
	p.SaveText("metin\n")
	os.WriteFile(p.File(), []byte(`{"meta":{"title":"Kapalı"},"body":[{"r":[{"t":"yar`), 0o644)
	a.takeOutside(p)
	if p.Unreadable == "" {
		t.Fatal("not seen as unreadable")
	}
	write := `[{"r":[{"t":"yazıldı"}]}]` // what a renumbering or a value writes: the page, through Save
	if err := p.Save(write); !errors.Is(err, library.ErrUnreadable) {
		t.Fatalf("written over an unreadable page: %v", err)
	}
	os.WriteFile(p.File(), JoinPage(map[string]string{"title": "Kapalı"}, `[{"r":[{"t":"onarıldı"}]}]`), 0o644)
	a.takeOutside(p)
	if p.Unreadable != "" {
		t.Fatalf("mended, still refused: %q", p.Unreadable)
	}
	if err := p.Save(write); err != nil {
		t.Fatalf("the mended page: %v", err)
	}
}
