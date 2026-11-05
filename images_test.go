package main

import (
	"bytes"
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
	lib, _ := OpenLibraryIn(st, storeRoot)
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

// TestPicturesSwept: a picture no line names is deleted when the page is first
// opened; one that is named, and one of another name, stay.
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
	a.docs = map[*Page]*richtext.Document{}
	a.page = nil
	a.open(a.lib.Collections[0].Pages[0])
	if _, err := os.Stat(orphan); err == nil {
		t.Fatal("the orphan stays")
	}
	if _, err := os.Stat(mine); err != nil {
		t.Fatal("a picture of another name was deleted")
	}
	if _, err := os.Stat(kept[0]); err != nil {
		t.Fatal("a picture in use was deleted")
	}
}
