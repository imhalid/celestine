package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Images: a picture in a page is a file beside the page's index.md - in the
// library's file, as everything - and a line of the page's Markdown,
// ![alt](resim-….png), as any Markdown tool reads it. The editor shows it as
// a block of its own, as wide as the picture up to maxImageW, or as the page.

const maxImageW = 640

// imageData marks an attachment as a picture of a page (chips are the other
// kind of attachment the app makes): what it was read from.
type imageData struct {
	file    string // its path in the library
	missing bool   // the file is not there, or not a picture
}

var imageTypes = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

// imageExt is the extension of the picture data is - "" when it is none.
func imageExt(data []byte) string {
	if ext, ok := imageTypes[http.DetectContentType(data)]; ok {
		return ext
	}
	head := strings.TrimSpace(string(data[:min(len(data), 512)]))
	if strings.HasPrefix(head, "<svg") || (strings.HasPrefix(head, "<?xml") && strings.Contains(string(data[:min(len(data), 2048)]), "<svg")) {
		return ".svg"
	}
	return ""
}

// imageFile is where the picture the Markdown names is, in p's folder, or
// "" when it names something else (a link out, a path out of the folder).
func imageFile(p *Page, ref string) string {
	if ref == "" || strings.Contains(ref, "://") || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "/") {
		return ""
	}
	if u, err := url.PathUnescape(ref); err == nil {
		ref = u
	}
	if strings.Contains(ref, "..") {
		return ""
	}
	return filepath.Join(p.Dir, filepath.FromSlash(ref))
}

// pictureAttachment is the attachment for a picture of page p: the picture
// itself when the file is there, a labelled box when it is not.
func (a *app) pictureAttachment(p *Page, name, ref string) *richtext.Attachment {
	file := imageFile(p, ref)
	att := &richtext.Attachment{Name: name, URL: ref, Data: &imageData{file: file}}
	if file != "" {
		if src := a.loadImage(p, file); src != nil {
			iw, ih := ui.ImageSize(src)
			if iw > 0 && ih > 0 {
				w := min(iw, maxImageW)
				att.Image, att.Width, att.Height = src, w, w*ih/iw
				att.Draw = func(p *ui.Painter, r ui.Rect) { // its corners rounded
					p.Clip(r, rad(10), func() { p.Image(src, r, ui.Contain) })
					a.picHits = append(a.picHits, picHit{r, src, name})
				}
				return att
			}
		}
	}
	att.Data = &imageData{file: file, missing: true}
	label := name
	if label == "" {
		label = ref
	}
	att.Width, att.Height = 240, 30
	att.Draw = func(p *ui.Painter, r ui.Rect) {
		th := p.Theme()
		p.Fill(r, th.TextMuted.Alpha(0.12), rad(8))
		span := ui.Span{Text: "▫ " + label, Size: 12.5, Color: th.TextMuted}
		_, h := p.MeasureText(r.W-16, span)
		p.RichText(r.X+10, r.Y+max(0, (r.H-h)/2), r.W-16, span)
	}
	return att
}

// imageRetry is how long a picture that could not be read is left alone: one that a sync brings later is
// then there, without the app being started again.
const imageRetry = 3 * time.Second

// loadImage reads and decodes the picture in file; pictures do not change, so
// it is kept once read. One that cannot be read or decoded is not kept: it is tried again after imageRetry.
func (a *app) loadImage(p *Page, file string) ui.ImageSource {
	if src, ok := a.images[file]; ok && src != nil {
		return src
	}
	now := a.clock()
	if at, ok := a.imageMiss[file]; ok && now.Sub(at) < imageRetry {
		return nil
	}
	var src ui.ImageSource
	if data, err := p.fs().ReadFile(file); err == nil {
		src, _ = ui.DecodeImage(data)
	}
	if src == nil {
		if a.imageMiss == nil {
			a.imageMiss = map[string]time.Time{}
		}
		a.imageMiss[file] = now
		return nil
	}
	delete(a.imageMiss, file)
	if a.images == nil {
		a.images = map[string]ui.ImageSource{}
	}
	a.images[file] = src
	return src
}

// withImages gives the pictures of page p, read from its Markdown as
// attachments with a URL only, their picture.
func (a *app) withImages(p *Page, t richtext.Text) richtext.Text {
	s := t.String()
	if !strings.ContainsRune(s, richtext.ObjectReplacement) {
		return t
	}
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] != richtext.ObjectReplacement {
			continue
		}
		attrs := t.AttributesAt(i)
		att := attrs.Attachment
		if att == nil || att.Data != nil || att.URL == "" {
			continue
		}
		attrs.Attachment = a.pictureAttachment(p, att.Name, att.URL)
		t = t.ReplaceString(i, i+1, string(richtext.ObjectReplacement), attrs)
	}
	return t
}

// notImage says a thing is not a picture.
const notImage = "Bu bir resim değil: png, jpg, gif, webp ya da svg olmalı."

// addImage puts the picture in data in the open page, in a paragraph of its
// own after the caret's line: the file is made beside the page, and its
// Markdown follows. It reports why it could not.
func (a *app) addImage(name string, data []byte) error {
	p := a.page
	if p == nil || a.doc == nil {
		return fmt.Errorf("açık sayfa yok")
	}
	ext := imageExt(data)
	if ext == "" {
		return fmt.Errorf("%s", notImage)
	}
	if _, err := ui.DecodeImage(data); err != nil {
		return fmt.Errorf("%s", notImage)
	}
	stem := "resim-" + a.clock().Format("20060102-150405")
	ref := stem + ext
	for i := 2; ; i++ {
		if _, err := p.fs().Stat(filepath.Join(p.Dir, ref)); err != nil {
			break
		}
		ref = fmt.Sprintf("%s-%d%s", stem, i, ext)
	}
	if err := p.fs().WriteFile(filepath.Join(p.Dir, ref), data); err != nil {
		return err
	}
	alt := strings.TrimSuffix(path.Base(filepath.ToSlash(name)), path.Ext(name))
	if strings.HasPrefix(alt, "resim-") || alt == "" || alt == "." {
		alt = "resim"
	}
	a.putBlock(a.pictureAttachment(p, alt, ref))
	return nil
}

// putBlock puts an attachment in a paragraph of its own: in the empty line the
// caret is on, or under the line it is in.
func (a *app) putBlock(att *richtext.Attachment) {
	t := a.doc.Text()
	_, caret := a.doc.Selection()
	ps, pe := t.ParagraphRange(min(caret, t.Len()))
	prefix, at := "", ps
	if pe > ps { // a line with text: it goes under it
		prefix, at = "\n", pe
	}
	last := pe >= t.Len()
	attrs := richtext.Attributes{Attachment: att}
	a.doc.Edit(func(ed *richtext.Edit) {
		ins := prefix + string(richtext.ObjectReplacement)
		if last {
			ins += "\n"
		}
		ed.Replace(at, at, richtext.NewText(ins))
		pos := at + utf8.RuneCountInString(prefix)
		ed.ReplaceString(pos, pos+1, string(richtext.ObjectReplacement), attrs)
		ed.FormatParagraphs(pos, pos, func(st *richtext.ParagraphStyle) { *st = richtext.ParagraphStyle{} })
		ed.Select(pos+2, pos+2)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
}

// addImageFile reads the picture at file and adds it.
func (a *app) addImageFile(file string) {
	data, err := os.ReadFile(file)
	if err == nil {
		err = a.addImage(filepath.Base(file), data)
	}
	if err != nil {
		a.notice = err.Error()
	}
}

// pickImage asks for a picture in a dialog and adds it; the dialog waits
// for the answer off the window's thread.
func (a *app) pickImage() {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "Resim seç",
			Filters: []mygo.FileFilter{{Name: "Resimler", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error { a.addImageFile(paths[0]); return nil })
	}()
}

// pasteImage adds the picture on the clipboard, when there is one and no
// text; it reports whether it did.
func (a *app) pasteImage() bool {
	read := a.clipImage
	if read == nil {
		read = mygo.Clipboard.ReadImage
	}
	data := read()
	if len(data) == 0 {
		return false
	}
	if err := a.addImage("resim", data); err != nil {
		a.notice = err.Error()
	}
	return true
}

// watchClipboard notes, for as long as the app runs, whether a picture is on
// the clipboard with no text: then ⌘V is the app's, to put the picture in;
// with text on it, it is the editor's. The clipboard is asked twice a second,
// for what formats it holds, not for the picture.
func (a *app) watchClipboard() {
	for range time.Tick(500 * time.Millisecond) {
		pic, text := false, false
		for _, f := range mygo.Clipboard.AvailableFormats() {
			f = strings.ToLower(f)
			switch {
			case strings.HasPrefix(f, "image/"), f == "public.png", f == "public.tiff", f == "cf_dib", f == "cf_dibv5", f == "png":
				pic = true
			case strings.HasPrefix(f, "text/"), f == "utf8_string", f == "string", f == "public.utf8-plain-text", f == "nsstringpboardtype", f == "cf_unicodetext":
				text = true
			}
		}
		a.imgOnClip.Store(pic && !text)
	}
}

// ourPicture is the name of a picture the app made.
var ourPicture = regexp.MustCompile(`^resim-\d{8}-\d{6}(-\d+)?\.(png|jpg|gif|webp|svg)$`)

// sweepPictures deletes the pictures the app made in page p's folder that no
// line of body names any more. It is done as a page is first opened in a
// run, when there is no undo to bring a line back; a picture one put there by
// hand, under another name, is left alone.
func (a *app) sweepPictures(p *Page, body string) {
	entries, err := p.fs().ReadDir(p.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && ourPicture.MatchString(e.Name()) && !strings.Contains(body, e.Name()) {
			p.fs().Remove(filepath.Join(p.Dir, e.Name()))
		}
	}
}

// picHit is a picture drawn, where in the window, for a click on it.
type picHit struct {
	r    ui.Rect
	src  ui.ImageSource
	name string
}

// pictureAt is the picture at (x, y) of the window, as the last frame drew it.
func (a *app) pictureAt(x, y float32) *picHit {
	for i := range a.picSeen {
		h := &a.picSeen[i]
		if x >= h.r.X && x <= h.r.X+h.r.W && y >= h.r.Y && y <= h.r.Y+h.r.H {
			return h
		}
	}
	return nil
}

// lightboxView shows the picture clicked at the size the window allows, over
// a dimmed page; a click anywhere or Esc puts it away.
func (a *app) lightboxView(c *ui.Context) {
	lb := a.lightbox
	if lb == nil {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		a.lightbox = nil
		return
	}
	ww, wh := c.Size()
	iw, ih := ui.ImageSize(lb.src)
	if iw <= 0 || ih <= 0 {
		a.lightbox = nil
		return
	}
	k := min(1, min((ww-80)/iw, (wh-120)/ih))
	w, h := iw*k, ih*k
	ui.Overlay(c, func() {
		back := ui.Box(c.Key("lb-back")).Absolute().Left(0).Top(0).Size(ww, wh).Background(ui.Hex("#000000").Alpha(0.72)).Cursor(ui.CursorPointer).Label("Resmi kapat").Role(ui.RoleButton)
		back.Opacity(back.AnimateTo("in", 1, fadeQuick))
		ui.Box(c.Key("lb-img")).Absolute().Left((ww-w)/2).Top((wh-h)/2-14).Size(w, h).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
			p.Clip(r, rad(12), func() { p.Image(lb.src, r, ui.Contain) })
		})
		if lb.name != "" {
			ui.Text(c.Key("lb-name"), lb.name).Absolute().Left(0).Top((wh+h)/2 + 2).Width(ww).TextAlign(ui.Center).FontSize(13).TextColor(ui.Hex("#ffffff").Alpha(0.8)).PassThrough()
		}
		if back.Clicked() {
			a.lightbox = nil
		}
	})
}
